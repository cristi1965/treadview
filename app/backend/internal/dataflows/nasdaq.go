package dataflows

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// NasdaqQuote is the public quote subset needed by macro and ETF views.
type NasdaqQuote struct {
	Symbol          string
	Price           float64
	Change          float64
	ChangePct       float64
	TradeTime       time.Time
	TimeGranularity string
	MarketStatus    string
	Currency        string
	CurrencySource  string
	AssetClass      string
	SourceURL       string
	IsRealTime      bool
}

type nasdaqClient struct {
	http *http.Client
}

var defaultNasdaq = &nasdaqClient{http: &http.Client{Timeout: 4 * time.Second}}

// NasdaqQuotes fetches a bounded set concurrently. Nasdaq's endpoint is
// symbol-scoped, so this is deliberately small and only used as a fallback.
func NasdaqQuotes(symbols []string) map[string]NasdaqQuote {
	out := make(map[string]NasdaqQuote, len(symbols))
	master, err := loadUSInstrumentMaster()
	if err != nil {
		return out
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, raw := range symbols {
		sym := strings.TrimSpace(strings.ToUpper(raw))
		if sym == "" {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			q, err := defaultNasdaq.quote(sym)
			if err != nil || q.Price <= 0 || !master.ContainsStock(q.Symbol) || q.AssetClass != "stocks" {
				return
			}
			if q.Currency == "" {
				q.Currency = "USD"
				q.CurrencySource = master.CurrencySource()
			}
			mu.Lock()
			out[sym] = q
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

func (c *nasdaqClient) quote(symbol string) (NasdaqQuote, error) {
	u := "https://api.nasdaq.com/api/quote/" + symbol + "/info?assetclass=stocks"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return NasdaqQuote{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return NasdaqQuote{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return NasdaqQuote{}, fmt.Errorf("nasdaq status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return NasdaqQuote{}, err
	}
	return parseNasdaqQuote(body, symbol, u)
}

func parseNasdaqQuote(body []byte, symbol, sourceURL string) (NasdaqQuote, error) {
	var payload struct {
		Data struct {
			Symbol       string `json:"symbol"`
			MarketStatus string `json:"marketStatus"`
			Primary      struct {
				LastSalePrice      string `json:"lastSalePrice"`
				NetChange          string `json:"netChange"`
				PercentageChange   string `json:"percentageChange"`
				LastTradeTimestamp string `json:"lastTradeTimestamp"`
				Currency           string `json:"currency"`
				IsRealTime         bool   `json:"isRealTime"`
			} `json:"primaryData"`
		} `json:"data"`
		Status struct {
			Code int `json:"rCode"`
		} `json:"status"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&payload); err != nil {
		return NasdaqQuote{}, err
	}
	if payload.Status.Code != 0 && payload.Status.Code != http.StatusOK {
		return NasdaqQuote{}, fmt.Errorf("nasdaq payload status %d", payload.Status.Code)
	}
	price := parseMoney(payload.Data.Primary.LastSalePrice)
	change := parseMoney(payload.Data.Primary.NetChange)
	pct := parseMoney(strings.TrimSuffix(payload.Data.Primary.PercentageChange, "%"))
	if price <= 0 {
		return NasdaqQuote{}, fmt.Errorf("nasdaq quote has no price")
	}
	tradeTime, timeGranularity, err := parseNasdaqTradeTime(payload.Data.Primary.LastTradeTimestamp)
	if err != nil {
		return NasdaqQuote{}, err
	}
	status := strings.ToLower(strings.TrimSpace(payload.Data.MarketStatus))
	if status == "" {
		return NasdaqQuote{}, fmt.Errorf("nasdaq quote has no market status")
	}
	currency := strings.ToUpper(strings.TrimSpace(payload.Data.Primary.Currency))
	currencySource := ""
	if currency != "" {
		currencySource = "nasdaq:primaryData.currency"
	}
	if payload.Data.Symbol != "" && !strings.EqualFold(payload.Data.Symbol, symbol) {
		return NasdaqQuote{}, fmt.Errorf("nasdaq quote symbol mismatch")
	}
	return NasdaqQuote{
		Symbol: strings.ToUpper(symbol), Price: price, Change: change, ChangePct: pct,
		TradeTime: tradeTime, TimeGranularity: timeGranularity, MarketStatus: status, Currency: currency,
		CurrencySource: currencySource, AssetClass: "stocks",
		SourceURL: sourceURL, IsRealTime: payload.Data.Primary.IsRealTime,
	}, nil
}

func parseNasdaqTradeTime(value string) (time.Time, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, "", fmt.Errorf("nasdaq quote has no trade time")
	}
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.Time{}, "", err
	}
	trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(value, " ET"), " EST"))
	for _, layout := range []string{
		"Jan 2, 2006 3:04 PM", "Jan 2, 2006 03:04 PM",
		"01/02/2006 03:04 PM", "2006-01-02 15:04:05",
	} {
		if parsed, parseErr := time.ParseInLocation(layout, trimmed, loc); parseErr == nil {
			return parsed, "second", nil
		}
	}
	if parsed, parseErr := time.ParseInLocation("Jan 2, 2006", trimmed, loc); parseErr == nil {
		return parsed, "date", nil
	}
	if parsed, parseErr := time.Parse(time.RFC3339, value); parseErr == nil {
		return parsed, "second", nil
	}
	return time.Time{}, "", fmt.Errorf("unparseable nasdaq trade time %q", value)
}
