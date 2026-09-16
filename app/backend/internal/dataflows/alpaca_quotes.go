package dataflows

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const alpacaMarketDataURL = "https://data.alpaca.markets/v2/stocks/snapshots"

type AlpacaQuote struct {
	Symbol     string
	Price      float64
	PrevClose  float64
	ObservedAt time.Time
	Exchange   string
	Feed       string
	SourceURL  string
}

type AlpacaQuoteClient struct {
	keyID  string
	secret string
	feed   string
	http   *http.Client
}

func NewAlpacaQuoteClient(keyID, secret string) *AlpacaQuoteClient {
	return &AlpacaQuoteClient{
		keyID: strings.TrimSpace(keyID), secret: strings.TrimSpace(secret), feed: "iex",
		http: &http.Client{Timeout: 4 * time.Second},
	}
}

func (c *AlpacaQuoteClient) Configured() bool {
	return c != nil && c.keyID != "" && c.secret != ""
}

func (c *AlpacaQuoteClient) GetQuotes(symbols []string) (map[string]AlpacaQuote, error) {
	if !c.Configured() {
		return map[string]AlpacaQuote{}, nil
	}
	wanted := make([]string, 0, len(symbols))
	for _, raw := range symbols {
		if symbol := strings.ToUpper(strings.TrimSpace(raw)); symbol != "" {
			wanted = append(wanted, symbol)
		}
	}
	if len(wanted) == 0 {
		return map[string]AlpacaQuote{}, nil
	}
	query := url.Values{"symbols": {strings.Join(wanted, ",")}, "feed": {c.feed}}
	sourceURL := alpacaMarketDataURL + "?" + query.Encode()
	req, err := http.NewRequest(http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("APCA-API-KEY-ID", c.keyID)
	req.Header.Set("APCA-API-SECRET-KEY", c.secret)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alpaca market data status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	return parseAlpacaSnapshots(body, c.feed, sourceURL)
}

func parseAlpacaSnapshots(body []byte, feed, sourceURL string) (map[string]AlpacaQuote, error) {
	var payload map[string]struct {
		LatestTrade struct {
			Price     float64   `json:"p"`
			Timestamp time.Time `json:"t"`
			Exchange  string    `json:"x"`
		} `json:"latestTrade"`
		PrevDailyBar struct {
			Close float64 `json:"c"`
		} `json:"prevDailyBar"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("alpaca snapshots parse: %w", err)
	}
	out := make(map[string]AlpacaQuote, len(payload))
	for symbol, snapshot := range payload {
		if snapshot.LatestTrade.Price <= 0 || snapshot.LatestTrade.Timestamp.IsZero() {
			continue
		}
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		out[symbol] = AlpacaQuote{
			Symbol: symbol, Price: snapshot.LatestTrade.Price, PrevClose: snapshot.PrevDailyBar.Close,
			ObservedAt: snapshot.LatestTrade.Timestamp.UTC(), Exchange: snapshot.LatestTrade.Exchange,
			Feed: feed, SourceURL: sourceURL,
		}
	}
	return out, nil
}
