package dataflows

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const tvScannerURL = "https://scanner.tradingview.com/america/scan"

// TVRestClient fetches real-time stock data from TradingView's public
// scanner REST API. This is the same API that powers the TradingView
// Technical Analysis widget — no API key required.
type TVRestClient struct {
	httpClient *http.Client
}

// TVQuoteData holds a single quote data point from the scanner.
type TVQuoteData struct {
	Symbol    string
	Price     float64
	Change    float64
	ChangePct float64
	Volume    float64
	High      float64
	Low       float64
	Open      float64
	PrevClose float64
}

// NewTVRestClient creates a TradingView REST API client.
func NewTVRestClient() *TVRestClient {
	return &TVRestClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// GetRealTimeQuote fetches a real-time quote for a single ticker.
func (c *TVRestClient) GetRealTimeQuote(ticker string) (*TVQuoteData, error) {
	items, err := c.GetRealTimeQuotes([]string{ticker})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no quote data for %s", ticker)
	}
	return &items[0], nil
}

// GetRealTimeQuotes fetches real-time quotes for multiple tickers.
func (c *TVRestClient) GetRealTimeQuotes(tickers []string) ([]TVQuoteData, error) {
	// Convert to TradingView format: "NASDAQ:AAPL", "NYSE:SPY"
	tvSymbols := make([]string, len(tickers))
	for i, t := range tickers {
		tvSymbols[i] = toTVSymbol(t)
	}

	// close, change(%), change_abs($), volume, high, low, open, previous_close
	columns := []string{"close", "change", "change_abs", "volume", "high", "low", "open", "previous_close"}

	payload := map[string]any{
		"symbols": map[string]any{
			"tickers": tvSymbols,
			"query":   map[string]any{"types": []string{}},
		},
		"columns": columns,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest("POST", tvScannerURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TradingView API %d: %s", resp.StatusCode, string(raw[:min(300, len(raw))]))
	}

	var result struct {
		TotalCount int `json:"totalCount"`
		Data       []struct {
			Symbol string    `json:"s"`
			Values []float64 `json:"d"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse: %w (body: %s)", err, string(raw[:min(200, len(raw))]))
	}

	log.Printf("[TV REST] Got %d/%d results for %v", len(result.Data), result.TotalCount, tickers)

	// columns: close, change(%), change_abs($), volume, high, low, open, previous_close
	var quotes []TVQuoteData
	for _, entry := range result.Data {
		q := TVQuoteData{Symbol: entry.Symbol}
		v := entry.Values
		if len(v) > 0 {
			q.Price = v[0]
		}
		if len(v) > 1 {
			q.ChangePct = v[1]
		}
		if len(v) > 2 {
			q.Change = v[2]
		}
		if len(v) > 3 {
			q.Volume = v[3]
		}
		if len(v) > 4 {
			q.High = v[4]
		}
		if len(v) > 5 {
			q.Low = v[5]
		}
		if len(v) > 6 {
			q.Open = v[6]
		}
		if len(v) > 7 && v[7] > 0 {
			q.PrevClose = v[7]
		} else if q.Price > 0 && q.ChangePct != -100 {
			q.PrevClose = q.Price / (1.0 + q.ChangePct/100.0)
		}
		quotes = append(quotes, q)
	}

	return quotes, nil
}

// FormatQuoteForLLM formats a single quote as text for LLM consumption.
func FormatQuoteForLLM(q TVQuoteData, ticker string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**REAL-TIME QUOTE: %s** (via TradingView)\n", ticker))
	sb.WriteString(fmt.Sprintf("- Current Price: $%.2f\n", q.Price))
	if q.Change != 0 {
		sb.WriteString(fmt.Sprintf("- Change: $%.2f (%.2f%%)\n", q.Change, q.ChangePct))
	}
	if q.High > 0 {
		sb.WriteString(fmt.Sprintf("- Day High: $%.2f\n", q.High))
	}
	if q.Low > 0 {
		sb.WriteString(fmt.Sprintf("- Day Low: $%.2f\n", q.Low))
	}
	if q.Open > 0 {
		sb.WriteString(fmt.Sprintf("- Open: $%.2f\n", q.Open))
	}
	if q.Volume > 0 {
		sb.WriteString(fmt.Sprintf("- Volume: %.0f\n", q.Volume))
	}
	return sb.String()
}

// FormatQuotesForLLM formats multiple quotes as a markdown table.
func FormatQuotesForLLM(items []TVQuoteData) string {
	var sb strings.Builder
	sb.WriteString("**BATCH REAL-TIME QUOTES** (via TradingView):\n\n")
	sb.WriteString("| Symbol | Price | Change | Change% | Volume |\n")
	sb.WriteString("|--------|-------|--------|---------|--------|\n")
	for _, q := range items {
		sb.WriteString(fmt.Sprintf("| %s | $%.2f | $%.2f | %.2f%% | %.0f |\n",
			q.Symbol, q.Price, q.Change, q.ChangePct, q.Volume))
	}
	return sb.String()
}
