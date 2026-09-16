package dataflows

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const productionADVWindow = 20

type DailyVolumeObservation struct {
	Date   string `json:"date"`
	Volume int64  `json:"volume"`
}

type ADVResult struct {
	Symbol             string                   `json:"symbol"`
	AverageDailyVolume float64                  `json:"averageDailyVolume"`
	Window             int                      `json:"window"`
	Start              string                   `json:"start"`
	End                string                   `json:"end"`
	Source             string                   `json:"source"`
	SourceURL          string                   `json:"sourceURL"`
	RefreshedAt        string                   `json:"refreshedAt"`
	Observations       []DailyVolumeObservation `json:"observations"`
}

type advCacheEntry struct {
	at     time.Time
	result ADVResult
}

var nasdaqADVCache sync.Map

// FetchNasdaqADV20 computes ADV from 20 distinct daily Nasdaq historical rows.
// It never substitutes the current quote's single-day volume.
func FetchNasdaqADV20(client *http.Client, symbol string, now time.Time) (ADVResult, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	master, err := loadUSInstrumentMaster()
	if err != nil || !master.ContainsStock(symbol) {
		return ADVResult{}, fmt.Errorf("symbol absent from versioned US instrument master")
	}
	if cached, ok := nasdaqADVCache.Load(symbol); ok {
		entry := cached.(advCacheEntry)
		if now.Sub(entry.at) < 15*time.Minute {
			return entry.result, nil
		}
	}
	from := now.AddDate(0, 0, -60).Format("2006-01-02")
	to := now.Format("2006-01-02")
	endpoint := fmt.Sprintf("https://api.nasdaq.com/api/quote/%s/historical?assetclass=stocks&fromdate=%s&todate=%s&limit=80",
		url.PathEscape(symbol), url.QueryEscape(from), url.QueryEscape(to))
	body, err := httpGet(client, endpoint, map[string]string{
		"Accept": "application/json", "Origin": "https://www.nasdaq.com", "Referer": "https://www.nasdaq.com/",
	})
	if err != nil {
		return ADVResult{}, err
	}
	result, err := parseNasdaqADV(body, symbol, endpoint, now, productionADVWindow)
	if err != nil {
		return ADVResult{}, err
	}
	nasdaqADVCache.Store(symbol, advCacheEntry{at: now, result: result})
	return result, nil
}

func parseNasdaqADV(body []byte, symbol, endpoint string, refreshedAt time.Time, window int) (ADVResult, error) {
	var payload struct {
		Data struct {
			TradesTable struct {
				Rows []struct {
					Date   string `json:"date"`
					Volume string `json:"volume"`
				} `json:"rows"`
			} `json:"tradesTable"`
		} `json:"data"`
		Status struct {
			Code int `json:"rCode"`
		} `json:"status"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ADVResult{}, err
	}
	if payload.Status.Code != 0 && payload.Status.Code != http.StatusOK {
		return ADVResult{}, fmt.Errorf("Nasdaq historical status %d", payload.Status.Code)
	}
	if window < 20 {
		return ADVResult{}, fmt.Errorf("ADV window must contain at least 20 trading days")
	}
	seen := map[string]bool{}
	observations := make([]DailyVolumeObservation, 0, len(payload.Data.TradesTable.Rows))
	for _, row := range payload.Data.TradesTable.Rows {
		date, err := time.Parse("01/02/2006", strings.TrimSpace(row.Date))
		if err != nil {
			continue
		}
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}
		dateKey := date.Format("2006-01-02")
		if seen[dateKey] {
			continue
		}
		volume, err := strconv.ParseInt(strings.ReplaceAll(strings.TrimSpace(row.Volume), ",", ""), 10, 64)
		if err != nil || volume <= 0 {
			continue
		}
		seen[dateKey] = true
		observations = append(observations, DailyVolumeObservation{Date: dateKey, Volume: volume})
	}
	sort.Slice(observations, func(i, j int) bool { return observations[i].Date < observations[j].Date })
	if len(observations) < window {
		return ADVResult{}, fmt.Errorf("Nasdaq historical has %d valid volume observations; need %d", len(observations), window)
	}
	observations = observations[len(observations)-window:]
	var total int64
	for _, observation := range observations {
		total += observation.Volume
	}
	return ADVResult{
		Symbol: symbol, AverageDailyVolume: float64(total) / float64(window), Window: window,
		Start: observations[0].Date, End: observations[len(observations)-1].Date,
		Source: "nasdaq-historical", SourceURL: endpoint,
		RefreshedAt: refreshedAt.UTC().Format(time.RFC3339), Observations: observations,
	}, nil
}

// LoadProductionADVResults fetches only requested symbols and preserves the
// complete auditable 20-session window for each result.
func LoadProductionADVResults(marketCode string, symbols []string) (map[string]ADVResult, error) {
	if !strings.EqualFold(strings.TrimSpace(marketCode), "us") {
		return nil, fmt.Errorf("authoritative 20-day ADV source unavailable for market %s", marketCode)
	}
	unique := make([]string, 0, len(symbols))
	seen := map[string]bool{}
	for _, raw := range symbols {
		symbol := strings.ToUpper(strings.TrimSpace(raw))
		if symbol != "" && !seen[symbol] {
			seen[symbol] = true
			unique = append(unique, symbol)
		}
	}
	if len(unique) == 0 {
		return map[string]ADVResult{}, nil
	}
	type item struct {
		symbol string
		result ADVResult
		err    error
	}
	results := make(chan item, len(unique))
	gate := make(chan struct{}, 4)
	now := time.Now()
	var wg sync.WaitGroup
	for _, symbol := range unique {
		wg.Add(1)
		go func(symbol string) {
			defer wg.Done()
			gate <- struct{}{}
			defer func() { <-gate }()
			result, err := FetchNasdaqADV20(&http.Client{Timeout: 12 * time.Second}, symbol, now)
			results <- item{symbol: symbol, result: result, err: err}
		}(symbol)
	}
	wg.Wait()
	close(results)
	out := make(map[string]ADVResult, len(unique))
	var failures []string
	for item := range results {
		if item.err != nil {
			failures = append(failures, item.symbol+": "+item.err.Error())
			continue
		}
		out[item.symbol] = item.result
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		return nil, fmt.Errorf("ADV incomplete: %s", strings.Join(failures, "; "))
	}
	return out, nil
}

// LoadProductionADV adapts the auditable results to the portfolio risk loader.
func LoadProductionADV(marketCode string, symbols []string) (map[string]float64, string, error) {
	results, err := LoadProductionADVResults(marketCode, symbols)
	if err != nil {
		return nil, "nasdaq-historical:20-trading-days", err
	}
	out := make(map[string]float64, len(results))
	for symbol, result := range results {
		out[symbol] = result.AverageDailyVolume
	}
	return out, "nasdaq-historical:20-trading-days", nil
}
