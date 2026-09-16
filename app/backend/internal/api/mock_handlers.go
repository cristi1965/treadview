package api

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"trading-agents/internal/dataflows"
	"trading-agents/internal/market"

	"github.com/gin-gonic/gin"
)

type MarketQuote struct {
	Price float64 `json:"price"`
	Pct   float64 `json:"pct"`
	Vol   float64 `json:"vol"`
	McapB float64 `json:"mcapB,omitempty"`
}

type AMarketQuote struct {
	Price  float64 `json:"price"`
	Pct    float64 `json:"pct"`
	Vol    float64 `json:"vol"`
	McapYi float64 `json:"mcapYi,omitempty"`
}

type QuoteResponseItem struct {
	Price           float64 `json:"price"`
	Pct             float64 `json:"pct"`
	Session         string  `json:"session"`
	PrevClose       float64 `json:"prevClose"`
	Source          string  `json:"source,omitempty"`
	DataTime        string  `json:"dataTime,omitempty"`
	TimeGranularity string  `json:"timeGranularity,omitempty"`
	SourceSession   string  `json:"sourceSession,omitempty"`
	ProviderURL     string  `json:"providerURL,omitempty"`
	ProviderMode    string  `json:"providerMode,omitempty"`
}

type macroSeriesItem struct {
	Sym   string  `json:"sym"`
	Name  string  `json:"name"`
	Kind  string  `json:"kind"`
	Price float64 `json:"price"`
	Pct   float64 `json:"pct"`
}

type macroSnapshot struct {
	Series []macroSeriesItem `json:"series"`
	Ts     int64             `json:"ts"`
}

func canonicalMacroTemplate() macroSnapshot {
	return macroSnapshot{Series: []macroSeriesItem{
		{Sym: "^TNX", Name: "美国 10 年期国债收益率", Kind: "rate"},
		{Sym: "^IRX", Name: "美国 13 周国债收益率", Kind: "rate"},
		{Sym: "^FVX", Name: "美国 5 年期国债收益率", Kind: "rate"},
		{Sym: "^TYX", Name: "美国 30 年期国债收益率", Kind: "rate"},
		{Sym: "SPY", Name: "标普 500 ETF 代理", Kind: "asset"},
		{Sym: "QQQ", Name: "纳斯达克 100 ETF 代理", Kind: "asset"},
		{Sym: "DIA", Name: "道琼斯 ETF 代理", Kind: "asset"},
		{Sym: "IWM", Name: "罗素 2000 ETF 代理", Kind: "asset"},
		{Sym: "^VIX", Name: "CBOE VIX", Kind: "index"},
		{Sym: "DX-Y.NYB", Name: "美元指数", Kind: "index"},
		{Sym: "GLD", Name: "黄金 ETF 代理", Kind: "asset"},
		{Sym: "USO", Name: "原油 ETF 代理", Kind: "asset"},
		{Sym: "BTC-USD", Name: "比特币", Kind: "asset"},
		{Sym: "ETH-USD", Name: "以太坊", Kind: "asset"},
	}}
}

type moverItem struct {
	Sym     string  `json:"sym"`
	Price   float64 `json:"price"`
	Pct     float64 `json:"pct"`
	PrevPct float64 `json:"prevPct,omitempty"`
}

type moversSnapshot struct {
	Session string      `json:"session"`
	Label   string      `json:"label"`
	Gainers []moverItem `json:"gainers"`
	Losers  []moverItem `json:"losers"`
	Ts      int64       `json:"ts"`
}

var fetchMacroQuotes = fetchMacroYahooQuotes

func fetchMacroYahooQuotes(symbols []string) quoteFetchResult {
	client := dataflows.NewYFinanceClientWithTimeout(2500 * time.Millisecond)
	quotes := make(map[string]market.Quote, len(symbols))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, symbol := range symbols {
		symbol := symbol
		wg.Add(1)
		go func() {
			defer wg.Done()
			quote, err := client.GetQuote(symbol)
			if err != nil || quote == nil || quote.RegularPrice <= 0 || quote.ObservedAt.IsZero() {
				return
			}
			pct := 0.0
			if quote.PreviousClose > 0 {
				pct = (quote.RegularPrice - quote.PreviousClose) / quote.PreviousClose * 100
			}
			session := quote.MarketState
			if session == "" {
				session = "unknown"
			}
			mu.Lock()
			quotes[symbol] = market.Quote{
				Price: quote.RegularPrice, Pct: pct, PrevClose: quote.PreviousClose,
				Session: session, SourceSession: session, Source: "yahoo-chart",
				DataTime: quote.ObservedAt.UTC().Format(time.RFC3339), TimeGranularity: "second", CurrencyCode: quote.Currency,
				ProviderURL: "https://query1.finance.yahoo.com/v8/finance/chart/" + url.PathEscape(dataflows.ToYahooSymbol(symbol)),
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return newQuoteFetchResult(market.Default(), quotes, false, time.Now())
}

// GetPremarketMovers handles GET /api/premarket-movers.
func GetPremarketMovers(c *gin.Context) {
	provider := market.Default()
	status := provider.USPayloadStatus(time.Now())
	if !strings.EqualFold(c.Query("mode"), "snapshot") && !status.Stale && status.Session != "closed" {
		if live, ok := buildLiveMoversSnapshot(provider.USQuotesCopy(), status); ok {
			setDataFreshness(c, dataFreshnessMeta{
				Source:      "live-us-movers:" + status.Source,
				DataTime:    dataTimeOrEmpty(status.DataTime),
				Refreshable: true,
			})
			c.JSON(http.StatusOK, live)
			return
		}
	}
	snap, data, ok := loadMoversSnapshotFile()
	if !ok {
		staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{
			Source:      "missing",
			Refreshable: false,
		}, "movers snapshot unavailable")
		return
	}
	setDataFreshness(c, dataFreshnessMeta{
		Source:      realSnapshotSource("premarket-movers"),
		DataTime:    millisDataTime(snap.Ts),
		Stale:       true,
		StaleReason: "read-only movers snapshot; no live refresh was requested",
		Refreshable: false,
	})
	c.Data(http.StatusOK, "application/json", data)
}

func buildLiveMoversSnapshot(quotes map[string]market.SnapshotQuote, status market.PayloadStatus) (moversSnapshot, bool) {
	rows := make([]moverItem, 0, len(quotes))
	for symbol, quote := range quotes {
		if quote.Price <= 0 || math.IsNaN(quote.Pct) || math.IsInf(quote.Pct, 0) {
			continue
		}
		rows = append(rows, moverItem{Sym: symbol, Price: quote.Price, Pct: quote.Pct})
	}
	if len(rows) < 12 || status.DataTime.IsZero() {
		return moversSnapshot{}, false
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Pct > rows[j].Pct })
	limit := 6
	if len(rows) < limit {
		limit = len(rows)
	}
	gainers := append([]moverItem(nil), rows[:limit]...)
	losers := append([]moverItem(nil), rows[len(rows)-limit:]...)
	sort.Slice(losers, func(i, j int) bool { return losers[i].Pct < losers[j].Pct })
	return moversSnapshot{
		Session: status.Session,
		Label:   "实时日涨跌异动",
		Gainers: gainers,
		Losers:  losers,
		Ts:      status.DataTime.UnixMilli(),
	}, true
}

// GetMacroData handles GET /api/macro using current provider quotes when every
// required symbol passes provenance and freshness checks, otherwise the last
// persisted snapshot remains available as an explicitly stale fallback.
func GetMacroData(c *gin.Context) {
	template := canonicalMacroTemplate()
	if !strings.EqualFold(c.Query("mode"), "snapshot") {
		symbols := make([]string, 0, len(template.Series))
		seen := make(map[string]bool, len(template.Series))
		for _, row := range template.Series {
			if !seen[row.Sym] {
				seen[row.Sym] = true
				symbols = append(symbols, row.Sym)
			}
		}
		result := fetchMacroQuotes(symbols)
		stale, staleReason := quoteResultFreshness(result, time.Now())
		if live, complete := buildLiveMacroSnapshot(template, result.quotes, result.dataTime); complete {
			source := "live-macro:" + result.source
			if stale || result.timedOut {
				source = "stale-snapshot:macro:" + result.source
			}
			_ = persistMacroSnapshot(live)
			setDataFreshness(c, dataFreshnessMeta{
				Source:      source,
				DataTime:    firstNonEmpty(result.dataTimeLabel, dataTimeOrEmpty(result.dataTime)),
				Stale:       stale || result.timedOut,
				StaleReason: staleReason,
				Refreshable: true,
			})
			c.JSON(http.StatusOK, live)
			return
		}
	}
	snapshot, ok := loadMacroSnapshotFile()
	if !ok {
		staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{
			Source:      "missing",
			Refreshable: true,
		}, "macro live source unavailable and no real priced snapshot exists")
		return
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal macro snapshot"})
		return
	}
	setDataFreshness(c, dataFreshnessMeta{
		Source:      realSnapshotSource("macro"),
		DataTime:    millisDataTime(snapshot.Ts),
		Stale:       true,
		StaleReason: "live macro source unavailable; serving last real snapshot",
		Refreshable: true,
	})
	c.Data(http.StatusOK, "application/json", raw)
}

func persistMacroSnapshot(snapshot macroSnapshot) error {
	if len(snapshot.Series) == 0 || snapshot.Ts <= 0 {
		return nil
	}
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	dir := "data"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "macro-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.Write(raw); err == nil {
		err = tmp.Close()
	} else {
		_ = tmp.Close()
	}
	if err != nil {
		return err
	}
	return os.Rename(tmpName, filepath.Join(dir, "macro.json"))
}

func buildLiveMacroSnapshot(base macroSnapshot, quotes map[string]market.Quote, dataTime time.Time) (macroSnapshot, bool) {
	if len(base.Series) == 0 || dataTime.IsZero() {
		return macroSnapshot{}, false
	}
	live := macroSnapshot{Series: make([]macroSeriesItem, 0, len(base.Series)), Ts: dataTime.UnixMilli()}
	for _, row := range base.Series {
		quote, ok := quotes[row.Sym]
		if !ok || quote.Price <= 0 || strings.TrimSpace(quote.Source) == "" || strings.TrimSpace(quote.DataTime) == "" {
			return macroSnapshot{}, false
		}
		row.Price = quote.Price
		row.Pct = quote.Pct
		live.Series = append(live.Series, row)
	}
	return live, true
}

// GetMarketData：GET /api/market — 美股全市场 quotes（内存 live 优先，后台持续刷 TV）。
func GetMarketData(c *gin.Context) {
	data, src, err := market.Default().LiveUSPayload()
	if err != nil {
		staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{
			Source:      src,
			Refreshable: true,
		}, "live US market unavailable and no real snapshot exists")
		return
	}
	stale := strings.HasPrefix(src, "stale-snapshot:") || strings.HasPrefix(src, "closed-") || snapshotPayloadStale(data)
	reason := marketPayloadStaleReason(src, stale)
	setDataFreshness(c, dataFreshnessMeta{
		Source:      src,
		DataTime:    firstNonEmpty(dataTimeFromSource(src), snapshotPayloadDataTime(data)),
		Stale:       stale,
		StaleReason: reason,
		Refreshable: true,
	})
	if granularity := snapshotPayloadTimeGranularity(data); granularity != "" {
		c.Header("X-Data-Time-Granularity", granularity)
	}
	c.Data(http.StatusOK, "application/json", data)
}

func snapshotPayloadStale(data []byte) bool {
	var payload struct {
		Stale bool `json:"stale"`
	}
	return json.Unmarshal(data, &payload) == nil && payload.Stale
}

func marketPayloadStaleReason(source string, stale bool) string {
	if !stale {
		return ""
	}
	if strings.Contains(source, "partial") || strings.Contains(source, "incomplete") {
		return "market universe is only partially provider-timed; serving as a stale snapshot"
	}
	if strings.HasPrefix(source, "closed-") {
		return "market session closed; serving the last completed provider snapshot"
	}
	return "live market unavailable; serving last real snapshot"
}

// RefreshMarketData：POST /api/market/refresh — 主动拉一波实时价。
// ?top=400 只刷市值前 N（默认 400）；?full=1 刷全宇宙（较慢）。
func RefreshMarketData(c *gin.Context) {
	if c.Request.Method != http.MethodPost {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "market refresh requires POST"})
		return
	}
	p := market.Default()
	p.StartLiveRefresh()

	full := strings.EqualFold(c.Query("full"), "1") || strings.EqualFold(c.Query("full"), "true")
	limit := 400
	if full {
		limit = 0
	} else if v := strings.TrimSpace(c.Query("top")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	wait := strings.EqualFold(c.Query("wait"), "1") || strings.EqualFold(c.Query("wait"), "true")
	if c.Query("wait") == "" {
		wait = true
	}
	if !wait {
		go func() {
			if _, err := p.RefreshUSQuotes(limit); err != nil {
				log.Printf("[market-refresh] async US failed: %v", err)
			}
			if _, err := p.RefreshCNQuotes(limit); err != nil {
				log.Printf("[market-refresh] async CN failed: %v", err)
			}
		}()
		c.Header("X-Data-Source", "refresh-queued")
		setDataFreshness(c, dataFreshnessMeta{
			Source:      "refresh-queued",
			DataTime:    dataTimeOrEmpty(p.LiveUpdatedAt()),
			Refreshable: true,
		})
		c.JSON(http.StatusAccepted, gin.H{
			"status": "queued",
			"limit":  limit,
			"liveAt": p.LiveUpdatedAt(),
		})
		return
	}

	nUS, errUS := p.RefreshUSQuotes(limit)
	nCN, errCN := p.RefreshCNQuotes(limit)
	if errUS != nil && errCN != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": errUS.Error(), "cnError": errCN.Error()})
		return
	}
	partialErrors := []string{}
	if errUS != nil {
		partialErrors = append(partialErrors, "us:"+errUS.Error())
	}
	if errCN != nil {
		partialErrors = append(partialErrors, "cn:"+errCN.Error())
	}
	setDataFreshness(c, dataFreshnessMeta{
		Source:        "live-refreshed",
		DataTime:      dataTimeOrEmpty(p.LiveUpdatedAt()),
		Stale:         len(partialErrors) > 0,
		StaleReason:   marketRefreshStaleReason(errUS, errCN),
		Refreshable:   true,
		PartialErrors: partialErrors,
	})
	c.JSON(http.StatusOK, gin.H{
		"status":    marketRefreshStatus(errUS, errCN),
		"updated":   nUS,
		"updatedCN": nCN,
		"limit":     limit,
		"liveAt":    p.LiveUpdatedAt(),
		"usError":   errString(errUS),
		"cnError":   errString(errCN),
	})
}

func marketRefreshStatus(errUS, errCN error) string {
	if errUS != nil || errCN != nil {
		return "partial"
	}
	return "ok"
}

func marketRefreshStaleReason(errUS, errCN error) string {
	if errUS == nil && errCN == nil {
		return ""
	}
	return "one or more market domains could not be refreshed"
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// GetAMarketData：GET /api/a-market — A 股 quotes（内存 live 优先）。
func GetAMarketData(c *gin.Context) {
	data, src, err := market.Default().LiveCNPayload()
	if err != nil {
		staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{
			Source:      src,
			Refreshable: true,
		}, "live CN market unavailable and no real snapshot exists")
		return
	}
	stale := strings.HasPrefix(src, "stale-snapshot:") || strings.HasPrefix(src, "closed-")
	reason := ""
	if strings.HasPrefix(src, "closed-") {
		reason = "market session closed; serving the last provider observation"
	} else if stale {
		reason = "live CN market unavailable; serving last real snapshot"
	}
	setDataFreshness(c, dataFreshnessMeta{
		Source:      src,
		DataTime:    firstNonEmpty(dataTimeFromSource(src), snapshotPayloadDataTime(data)),
		Stale:       stale,
		StaleReason: reason,
		Refreshable: true,
	})
	c.Data(http.StatusOK, "application/json", data)
}

// GetQuoteData：GET /api/quote?syms=A,B — 实时价（TV→Yahoo→快照）。
func GetQuoteData(c *gin.Context) {
	symsQuery := c.Query("syms")
	if symsQuery == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "syms parameter is required"})
		return
	}

	syms := strings.Split(symsQuery, ",")
	if len(syms) > maxPublicQuoteSymbols {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many symbols"})
		return
	}
	for _, symbol := range syms {
		if !safeTickerRe.MatchString(strings.ToUpper(strings.TrimSpace(symbol))) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid symbol"})
			return
		}
	}
	fetchResult := boundedQuotesForRequest(syms)
	live := fetchResult.quotes

	results := make(map[string]QuoteResponseItem, len(live))
	sources := map[string]int{}
	for sym, q := range live {
		results[sym] = QuoteResponseItem{
			Price:           q.Price,
			Pct:             q.Pct,
			Session:         q.Session,
			PrevClose:       q.PrevClose,
			Source:          q.Source,
			DataTime:        q.DataTime,
			TimeGranularity: q.TimeGranularity,
			SourceSession:   q.SourceSession,
			ProviderURL:     q.ProviderURL,
			ProviderMode:    q.ProviderMode,
		}
		sources[q.Source]++
	}

	partialErrors := []string{}
	if missing := missingSymbols(syms, results); len(missing) > 0 {
		partialErrors = append(partialErrors, "missing:"+strings.Join(missing, "|"))
	}
	stale, staleReason := quoteResultFreshness(fetchResult, time.Now())
	if fetchResult.timedOut {
		partialErrors = append(partialErrors, "live-timeout: serving last real snapshot")
	}
	dataTimeLabel := fetchResult.dataTimeLabel
	timeGranularity := fetchResult.timeGranularity
	if dataTimeLabel == "" && !fetchResult.dataTime.IsZero() {
		dataTimeLabel = fetchResult.dataTime.UTC().Format(time.RFC3339)
		timeGranularity = "second"
	}
	setDataFreshness(c, dataFreshnessMeta{
		Source:        fetchResult.source,
		DataTime:      dataTimeLabel,
		Stale:         stale,
		StaleReason:   staleReason,
		Refreshable:   true,
		PartialErrors: partialErrors,
	})
	dataTimestamp := int64(0)
	if !fetchResult.dataTime.IsZero() && timeGranularity != "date" {
		dataTimestamp = fetchResult.dataTime.UnixMilli()
	}
	if timeGranularity != "" {
		c.Header("X-Data-Time-Granularity", timeGranularity)
	}
	c.JSON(http.StatusOK, gin.H{
		"quotes":          results,
		"ts":              dataTimestamp,
		"dataTime":        dataTimeLabel,
		"timeGranularity": timeGranularity,
		"refreshedAt":     time.Now().UTC().Format(time.RFC3339),
		"sources":         sources,
		"missing":         missingSymbols(syms, results),
	})
}

func missingSymbols(requested []string, found map[string]QuoteResponseItem) []string {
	var miss []string
	for _, s := range requested {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := found[s]; !ok {
			miss = append(miss, s)
		}
	}
	return miss
}

func loadMacroSnapshotFile() (macroSnapshot, bool) {
	for _, path := range []string{
		filepath.Join("data", "macro.json"),
		filepath.Join("data", "macro-verified.json"),
		filepath.Join("app", "backend", "data", "macro.json"),
		filepath.Join("app", "backend", "data", "macro-verified.json"),
		filepath.Join("..", "frontend", "public", "data", "macro.json"),
		filepath.Join("app", "frontend", "public", "data", "macro.json"),
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var snap macroSnapshot
		if err := json.Unmarshal(raw, &snap); err != nil || len(snap.Series) == 0 || snap.Ts <= 0 {
			continue
		}
		priced := 0
		for _, row := range snap.Series {
			if row.Price > 0 {
				priced++
			}
		}
		if priced == len(snap.Series) {
			return snap, true
		}
	}
	return macroSnapshot{}, false
}

func loadMoversSnapshotFile() (moversSnapshot, []byte, bool) {
	for _, path := range []string{
		filepath.Join("data", "premarket-movers.json"),
		filepath.Join("app", "backend", "data", "premarket-movers.json"),
		filepath.Join("..", "frontend", "public", "data", "premarket-movers.json"),
		filepath.Join("app", "frontend", "public", "data", "premarket-movers.json"),
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var snap moversSnapshot
		if err := json.Unmarshal(raw, &snap); err != nil || snap.Ts <= 0 || len(snap.Gainers)+len(snap.Losers) == 0 {
			continue
		}
		return snap, raw, true
	}
	return moversSnapshot{}, nil, false
}

func snapshotPayloadDataTime(raw []byte) string {
	var payload struct {
		Ts          int64  `json:"ts"`
		GeneratedAt string `json:"generated_at"`
		Updated     string `json:"updated"`
		UpdatedAt   string `json:"updatedAt"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	if payload.Ts > 0 {
		return millisDataTime(payload.Ts)
	}
	if t, ok := parseLooseDataTime(payload.GeneratedAt); ok {
		return t.UTC().Format(time.RFC3339)
	}
	if t, ok := parseLooseDataTime(payload.Updated); ok {
		return t.UTC().Format(time.RFC3339)
	}
	if t, ok := parseLooseDataTime(payload.UpdatedAt); ok {
		return t.UTC().Format(time.RFC3339)
	}
	return ""
}

func snapshotPayloadTimeGranularity(raw []byte) string {
	var payload struct {
		TimeGranularity string `json:"timeGranularity"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return ""
	}
	return strings.TrimSpace(payload.TimeGranularity)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
