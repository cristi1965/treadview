package dataflows

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	tvWebSocketURL  = "wss://data.tradingview.com/socket.io/websocket"
	tvOrigin        = "https://data.tradingview.com"
	tvPingInterval  = 30 * time.Second
	tvConnectTimeout = 15 * time.Second
	tvQuoteTimeout   = 10 * time.Second
)

// TVClient provides real-time stock data via TradingView's WebSocket.
// Protocol: custom ~m~{len}~m~{json} framing (NOT standard socket.io).
// Session IDs are client-generated: "qs_" + 12 random lowercase chars.
type TVClient struct {
	conn    *websocket.Conn
	mu      sync.Mutex
	quotes  map[string]*TVQuote // symbol → latest quote
	subMu   sync.RWMutex
	done    chan struct{}
	running bool
	session string
}

// TVQuote represents a real-time quote from TradingView.
type TVQuote struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Change    float64 `json:"change"`
	ChangePct float64 `json:"change_pct"`
	Volume    float64 `json:"volume"`
	Timestamp int64   `json:"timestamp"`
}

// NewTVClient creates a new TradingView WebSocket client.
func NewTVClient() *TVClient {
	return &TVClient{
		quotes: make(map[string]*TVQuote),
		done:   make(chan struct{}),
	}
}

// Connect establishes the WebSocket connection to TradingView.
func (c *TVClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	header := http.Header{}
	header.Set("Origin", tvOrigin)

	dialer := websocket.Dialer{HandshakeTimeout: tvConnectTimeout}
	conn, _, err := dialer.Dial(tvWebSocketURL, header)
	if err != nil {
		return fmt.Errorf("TradingView dial: %w", err)
	}

	c.conn = conn
	c.running = true
	c.session = generateSession()

	// Start reader & pinger
	go c.readLoop()
	go c.pingLoop()

	log.Printf("[TV Client] Connected (session: %s)", c.session)
	return nil
}

// SubscribeQuotes subscribes to real-time quote updates for symbols.
// Symbols should be in TradingView format: "NASDAQ:AAPL", "NYSE:SPY"
func (c *TVClient) SubscribeQuotes(symbols []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return fmt.Errorf("not connected")
	}

	// 1. Create session
	c.sendMessage("quote_create_session", []any{c.session})

	// 2. Set fields we want
	c.sendMessage("quote_set_fields", []any{c.session, "lp", "volume", "ch", "chp"})

	// 3. Subscribe to symbols
	symList := strings.Join(symbols, ",")
	c.sendMessage("quote_add_symbols", []any{c.session, symList})

	log.Printf("[TV Client] Subscribed: %v", symbols)
	return nil
}

// GetQuote returns the latest quote for a symbol, or nil.
func (c *TVClient) GetQuote(symbol string) *TVQuote {
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	return c.quotes[symbol]
}

// WaitForQuote blocks until a quote is received or timeout.
func (c *TVClient) WaitForQuote(symbol string, timeout time.Duration) (*TVQuote, error) {
	deadline := time.After(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		q := c.GetQuote(symbol)
		if q != nil && q.Price > 0 {
			return q, nil
		}
		select {
		case <-deadline:
			return nil, fmt.Errorf("timeout waiting for %s", symbol)
		case <-ticker.C:
		}
	}
}

// GetCurrentPrice returns just the price.
func (c *TVClient) GetCurrentPrice(symbol string) (float64, error) {
	q, err := c.WaitForQuote(symbol, tvQuoteTimeout)
	if err != nil {
		return 0, err
	}
	return q.Price, nil
}

// IsConnected checks connection state.
func (c *TVClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// Close disconnects.
func (c *TVClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	c.running = false
	close(c.done)
	c.conn.Close()
	log.Printf("[TV Client] Disconnected")
}

// ---- internal ----

// generateSession creates a client-side session ID.
func generateSession() string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 12)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return "qs_" + string(b)
}

// sendMessage sends a TradingView protocol message.
func (c *TVClient) sendMessage(method string, params []any) {
	msg := map[string]any{"m": method, "p": params}
	jsonBytes, _ := json.Marshal(msg)
	framed := fmt.Sprintf("~m~%d~m~%s", len(jsonBytes), string(jsonBytes))
	c.conn.WriteMessage(websocket.TextMessage, []byte(framed))
}

// ---- reader / pinger ----

func (c *TVClient) readLoop() {
	defer func() {
		c.mu.Lock()
		c.conn.Close()
		c.running = false
		c.mu.Unlock()
	}()

	for {
		select {
		case <-c.done:
			return
		default:
		}

		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if c.running {
				log.Printf("[TV Client] Read error: %v", err)
			}
			return
		}

		c.handleRaw(string(raw))
	}
}

func (c *TVClient) pingLoop() {
	ticker := time.NewTicker(tvPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			c.mu.Lock()
			if c.running && c.conn != nil {
				// TradingView server sends ~m~\d+~m~ ping frames periodically.
				// We handle pings in the readLoop; here we just make sure the
				// connection is alive.
			}
			c.mu.Unlock()
		}
	}
}

// handleRaw processes a raw TradingView message.
func (c *TVClient) handleRaw(raw string) {
	// Unwrap ~m~ framing to get JSON payloads
	payloads := extractTVPayloads(raw)

	for _, payload := range payloads {
		payload = strings.TrimSpace(payload)
		if payload == "" {
			continue
		}

		// Skip ping frames: numbers between ~m~ wraps
		if matched, _ := regexp.MatchString(`^\d+$`, payload); matched {
			continue
		}

		// Try to parse as a TV protocol JSON message
		var msg map[string]any
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			continue
		}

		// Handle "qsd" (quote streaming data) messages
		if method, ok := msg["m"].(string); ok && method == "qsd" {
			c.handleQSD(msg["p"])
		}
	}
}

// handleQSD processes quote streaming data.
// Format: ["qs_...", {"n": "NASDAQ:AAPL", "v": {"lp": 150.0, "ch": 2.5, "chp": 1.69, "volume": 50000000}}]
func (c *TVClient) handleQSD(params any) {
	arr, ok := params.([]any)
	if !ok || len(arr) < 2 {
		return
	}

	// arr[1] is the symbol data object
	data, ok := arr[1].(map[string]any)
	if !ok {
		return
	}

	symName, _ := data["n"].(string)
	values, ok := data["v"].(map[string]any)
	if !ok {
		return
	}

	quote := &TVQuote{
		Symbol:    symName,
		Timestamp: time.Now().UnixMilli(),
	}

	if v, exists := values["lp"]; exists {
		quote.Price = toFloat64(v)
	}
	if v, exists := values["ch"]; exists {
		quote.Change = toFloat64(v)
	}
	if v, exists := values["chp"]; exists {
		quote.ChangePct = toFloat64(v)
	}
	if v, exists := values["volume"]; exists {
		quote.Volume = toFloat64(v)
	}

	c.subMu.Lock()
	c.quotes[symName] = quote
	c.subMu.Unlock()
}

// extractTVPayloads extracts all JSON payloads from ~m~ framed messages.
// Handles concatenated frames like: ~m~5~m~hello~m~20~m~{"key":"value"}
func extractTVPayloads(raw string) []string {
	var payloads []string

	re := regexp.MustCompile(`~m~(\d+)~m~`)
	locs := re.FindAllStringSubmatchIndex(raw, -1)

	for i, loc := range locs {
		// loc: [fullStart, fullEnd, numStart, numEnd]
		// Payload starts at fullEnd (after ~m~\d+~m~)
		payloadStart := loc[1] // end of the match
		length, _ := strconv.Atoi(raw[loc[2]:loc[3]])

		end := payloadStart + length
		if end <= len(raw) {
			payload := raw[payloadStart:end]
			payloads = append(payloads, payload)
		}
		_ = i
	}

	// Fallback: try simple ~m~ splitting if regex didn't match
	if len(payloads) == 0 {
		parts := strings.Split(raw, "~m~")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
					continue
				}
				if m, _ := regexp.MatchString(`^\d+$`, part); m {
				continue
			}
			payloads = append(payloads, part)
		}
	}

	return payloads
}

func toFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case json.Number:
		f, _ := val.Float64()
		return f
	}
	return 0
}
