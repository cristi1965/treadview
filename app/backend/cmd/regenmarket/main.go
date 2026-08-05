// regenmarket rebuilds local market.json from Yahoo Finance using us-stocks.json universe.
// Usage (from app/backend):
//
//	go run ./cmd/regenmarket
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type usStocksFile struct {
	Stocks []struct {
		Sym string `json:"sym"`
	} `json:"stocks"`
}

type marketFile struct {
	Quotes  map[string]quote `json:"quotes"`
	TS      int64            `json:"ts"`
	Count   int              `json:"count"`
	Session string           `json:"session"`
	Source  string           `json:"source"`
}

type quote struct {
	Price float64 `json:"price"`
	Pct   float64 `json:"pct"`
	McapB float64 `json:"mcapB,omitempty"`
	Vol   float64 `json:"vol"`
}

func main() {
	root := findBackendRoot()
	universePath := filepath.Join(root, "..", "frontend", "public", "data", "us-stocks.json")
	if _, err := os.Stat(universePath); err != nil {
		universePath = filepath.Join(root, "data", "us-stocks.json")
	}
	raw, err := os.ReadFile(universePath)
	if err != nil {
		log.Fatalf("read universe: %v", err)
	}
	var uni usStocksFile
	if err := json.Unmarshal(raw, &uni); err != nil {
		log.Fatalf("parse universe: %v", err)
	}
	syms := make([]string, 0, len(uni.Stocks))
	for _, s := range uni.Stocks {
		sym := strings.TrimSpace(s.Sym)
		if sym != "" {
			syms = append(syms, sym)
		}
	}
	log.Printf("universe %d symbols from %s", len(syms), universePath)

	out := marketFile{
		Quotes:  map[string]quote{},
		TS:      time.Now().UnixMilli(),
		Session: "regular",
		Source:  "yahoo-batch",
	}

	client := &http.Client{Timeout: 30 * time.Second}
	const batch = 80
	for i := 0; i < len(syms); i += batch {
		j := i + batch
		if j > len(syms) {
			j = len(syms)
		}
		chunk := syms[i:j]
		got, err := fetchYahooBatch(client, chunk)
		if err != nil {
			log.Printf("batch %d-%d error: %v", i, j, err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		for sym, q := range got {
			out.Quotes[sym] = q
		}
		log.Printf("progress %d/%d (%d quotes)", j, len(syms), len(out.Quotes))
		time.Sleep(200 * time.Millisecond)
	}
	out.Count = len(out.Quotes)

	dest := filepath.Join(root, "data", "market.json")
	blob, _ := json.Marshal(out)
	if err := os.WriteFile(dest, blob, 0o644); err != nil {
		log.Fatalf("write: %v", err)
	}
	log.Printf("wrote %s (%d quotes)", dest, out.Count)
}

func fetchYahooBatch(client *http.Client, syms []string) (map[string]quote, error) {
	u := "https://query1.finance.yahoo.com/v7/finance/quote?symbols=" + url.QueryEscape(strings.Join(syms, ","))
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 TradingAgents regenmarket")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("yahoo HTTP %d: %s", resp.StatusCode, string(body[:min(120, len(body))]))
	}
	var parsed struct {
		QuoteResponse struct {
			Result []struct {
				Symbol             string  `json:"symbol"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				RegularMarketChangePercent float64 `json:"regularMarketChangePercent"`
				RegularMarketVolume float64 `json:"regularMarketVolume"`
				MarketCap          float64 `json:"marketCap"`
			} `json:"result"`
		} `json:"quoteResponse"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := map[string]quote{}
	for _, r := range parsed.QuoteResponse.Result {
		if r.Symbol == "" || r.RegularMarketPrice == 0 {
			continue
		}
		q := quote{
			Price: r.RegularMarketPrice,
			Pct:   r.RegularMarketChangePercent,
			Vol:   r.RegularMarketVolume,
		}
		if r.MarketCap > 0 {
			q.McapB = r.MarketCap / 1e9
		}
		out[r.Symbol] = q
	}
	return out, nil
}

func findBackendRoot() string {
	cwd, _ := os.Getwd()
	for _, p := range []string{cwd, filepath.Join(cwd, "app", "backend")} {
		if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil {
			return p
		}
	}
	return cwd
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
