package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/config"
	"trading-agents/internal/dataflows"
	"trading-agents/internal/llm"
	"trading-agents/internal/scoring"
)

const panelSummaryFreshFor = 4 * time.Hour
const newsFreshFor = 24 * time.Hour

var (
	yfClient     = dataflows.NewYFinanceClient()
	metricsCache sync.Map // symbol -> cachedMetrics
	newsCache    sync.Map
)

type cachedMetrics struct {
	at   time.Time
	data *dataflows.StockMetrics
}

type cachedNews struct {
	at       time.Time
	data     []dataflows.NewsArticle
	dataTime time.Time
}

// GetFundamentalsData handles GET /api/fundamentals?sym=NVDA
func GetFundamentalsData(c *gin.Context) {
	sym := strings.TrimSpace(strings.ToUpper(c.Query("sym")))
	if sym == "" {
		sym = strings.TrimSpace(strings.ToUpper(c.Query("symbol")))
	}
	if sym == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sym required"})
		return
	}
	if v, ok := metricsCache.Load(sym); ok {
		cached := v.(cachedMetrics)
		if time.Since(cached.at) < 10*time.Minute {
			setDataFreshness(c, fundamentalsFreshness(cached.data, cached.at))
			c.JSON(http.StatusOK, cached.data)
			return
		}
	}
	m, err := yfClient.GetStockMetricsLive(sym)
	if err != nil {
		staleDataError(c, http.StatusBadGateway, dataFreshnessMeta{
			Source:      "yahoo-fundamentals",
			Refreshable: true,
		}, err.Error())
		return
	}
	now := time.Now()
	dataflows.FinalizeStockMetricsEvidence(m, now)
	metricsCache.Store(sym, cachedMetrics{at: now, data: m})
	setDataFreshness(c, fundamentalsFreshness(m, now))
	c.JSON(http.StatusOK, m)
}

func fundamentalsFreshness(metrics *dataflows.StockMetrics, fetchedAt time.Time) dataFreshnessMeta {
	meta := dataFreshnessMeta{Source: "fundamentals", Refreshable: true}
	if metrics == nil {
		meta.DataTime = "unknown"
		meta.Stale = true
		meta.StaleReason = "fundamentals unavailable"
		return meta
	}
	meta.Source = firstNonEmpty(metrics.Source, meta.Source)
	if ts, ok := parseLooseDataTime(metrics.FetchedAt); ok {
		meta.RefreshedAt = ts.UTC().Format(time.RFC3339)
	} else if !fetchedAt.IsZero() {
		meta.RefreshedAt = fetchedAt.UTC().Format(time.RFC3339)
	}
	fiscalOK := strings.TrimSpace(metrics.FiscalPeriod) != "" && !strings.EqualFold(strings.TrimSpace(metrics.FiscalPeriod), "unknown")
	asOf, asOfOK := parseLooseDataTime(metrics.AsOf)
	filingAt, filingOK := parseLooseDataTime(metrics.FilingDate)
	filingMetadataOK := filingOK && strings.TrimSpace(metrics.Accession) != "" && strings.TrimSpace(metrics.SourceURL) != ""
	if !fiscalOK || !asOfOK || !filingMetadataOK {
		meta.DataTime = "unknown"
		meta.Stale = true
		meta.StaleReason = "research filing metadata is unavailable"
		return meta
	}
	meta.DataTime = asOf.UTC().Format(time.RFC3339)
	meta.Stale, meta.StaleReason = staleIfOlder(meta.DataTime, 180*24*time.Hour)
	if filingAt.Before(asOf) {
		meta.Stale = true
		meta.StaleReason = "filing date precedes fiscal period end"
	}
	return meta
}

// GetStockNewsData handles GET /api/news?sym=NVDA
func GetStockNewsData(c *gin.Context) {
	sym := strings.TrimSpace(strings.ToUpper(c.Query("sym")))
	if sym == "" {
		sym = strings.TrimSpace(strings.ToUpper(c.Query("symbol")))
	}
	if sym == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sym required"})
		return
	}
	if v, ok := newsCache.Load(sym); ok {
		cached := v.(cachedNews)
		if time.Since(cached.at) < 15*time.Minute {
			setDataFreshness(c, newsFreshness(cached.dataTime, cached.at))
			c.JSON(http.StatusOK, gin.H{"symbol": sym, "news": cached.data, "source": "yahoo-finance-rss"})
			return
		}
	}
	items, err := dataflows.NewNewsClient().GetTickerNews(sym, 10)
	if err != nil {
		staleDataError(c, http.StatusBadGateway, dataFreshnessMeta{
			Source:      "yahoo-finance-rss",
			Stale:       true,
			Refreshable: true,
		}, err.Error())
		return
	}
	now := time.Now()
	dataTime := latestNewsPublished(items)
	if dataTime.IsZero() {
		staleDataError(c, http.StatusBadGateway, dataFreshnessMeta{
			Source: "yahoo-finance-rss", Stale: true, StaleReason: "news publication time unavailable", Refreshable: true,
		}, "news publication time unavailable")
		return
	}
	newsCache.Store(sym, cachedNews{at: now, data: items, dataTime: dataTime})
	setDataFreshness(c, newsFreshness(dataTime, now))
	c.JSON(http.StatusOK, gin.H{"symbol": sym, "news": items, "source": "yahoo-finance-rss"})
}

func newsFreshness(dataTime, refreshedAt time.Time) dataFreshnessMeta {
	meta := dataFreshnessMeta{
		Source:      "yahoo-finance-rss",
		DataTime:    dataTimeOrUnknown(dataTime),
		RefreshedAt: refreshedAt.UTC().Format(time.RFC3339),
		Refreshable: true,
	}
	if dataTime.IsZero() {
		meta.Stale = true
		meta.StaleReason = "news publication time unavailable"
		return meta
	}
	age := refreshedAt.Sub(dataTime)
	if age < -time.Minute {
		meta.Stale = true
		meta.StaleReason = "news publication time is in the future"
	} else if age > newsFreshFor {
		meta.Stale = true
		meta.StaleReason = "latest news publication is older than 24h"
	}
	return meta
}

func latestNewsPublished(items []dataflows.NewsArticle) time.Time {
	latest := time.Time{}
	for _, item := range items {
		for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC3339} {
			if parsed, err := time.Parse(layout, strings.TrimSpace(item.Published)); err == nil {
				if parsed.After(latest) {
					latest = parsed
				}
				break
			}
		}
	}
	return latest
}

func dataTimeOrUnknown(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return value.UTC().Format(time.RFC3339)
}

// GetPanelSummary handles GET /api/panel-summary — self-built five-factor scores.
func GetPanelSummary(c *gin.Context) {
	root := findBackendRoot()
	if panel, path, info, ok := loadPanelSummary(root); ok {
		freshness := panelSummaryFreshness(root, panel, info)
		panel = panelForSymbol(panel, root, c.Query("sym"))
		c.Header("X-Panel-Path", path)
		setDataFreshness(c, freshness)
		c.JSON(http.StatusOK, panel)
		return
	}
	staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{
		Source:      "missing:panel-summary",
		Stale:       true,
		StaleReason: "panel snapshot unavailable; use authenticated refresh",
		Refreshable: true,
	}, "panel snapshot unavailable")
}

func panelForSymbol(panel *scoring.PanelFile, root, rawSymbol string) *scoring.PanelFile {
	symbol := strings.ToUpper(strings.TrimSpace(rawSymbol))
	if panel == nil {
		return panel
	}
	panel.EnsureValidationStatus()
	if symbol == "" {
		return panel
	}
	row, ok := panel.Stocks[symbol]
	if !ok {
		return &scoring.PanelFile{
			Order: panel.Order, GeneratedAt: panel.GeneratedAt, Source: panel.Source,
			MethodVersion: panel.MethodVersion, Count: 0, Stocks: map[string]scoring.PanelStock{}, Validation: panel.Validation, MarketSubmodel: panel.MarketSubmodel,
		}
	}
	if universe, _, err := scoring.LoadUsStocks(scoring.DefaultUsStocksPaths(root)...); err == nil {
		for _, stock := range universe.Stocks {
			if strings.EqualFold(stock.Sym, symbol) {
				detail := scoring.ExplainFive(stock, true)
				detail.PanelScores = append([]int{}, row.SC...)
				detail.ReproducesPanel = equalIntScores(detail.FinalScores, row.SC)
				row.Detail = &detail
				break
			}
		}
	}
	methodVersion := panel.MethodVersion
	if methodVersion == "" && strings.Contains(panel.Source, "heuristic") {
		methodVersion = scoring.HeuristicMethodVersion
	}
	return &scoring.PanelFile{
		Order: panel.Order, GeneratedAt: panel.GeneratedAt, Source: panel.Source,
		MethodVersion: methodVersion, Count: 1, Stocks: map[string]scoring.PanelStock{symbol: row}, Validation: panel.Validation, MarketSubmodel: panel.MarketSubmodel,
	}
}

func equalIntScores(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func loadPanelSummary(root string) (*scoring.PanelFile, string, os.FileInfo, bool) {
	paths := scoring.DefaultPanelPaths(root)
	paths = append(paths,
		filepath.Join(root, "..", "frontend", "dist", "data", "us-panel-summary.json"),
		filepath.Join("app", "frontend", "public", "data", "us-panel-summary.json"),
	)
	path, info, ok := newestExistingFile(paths)
	if !ok {
		return nil, "", nil, false
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, path, info, false
	}
	var panel scoring.PanelFile
	if err := json.Unmarshal(raw, &panel); err != nil || panel.Stocks == nil {
		return nil, path, info, false
	}
	if panel.Order == nil {
		panel.Order = scoring.PanelOrder
	}
	panel.EnsureDeterministicScalingDisclosure()
	panel.EnsureValidationStatus()
	return &panel, path, info, true
}

func panelSummaryFreshness(root string, panel *scoring.PanelFile, info os.FileInfo) dataFreshnessMeta {
	meta := dataFreshnessMeta{Source: "panel-summary+us-stocks", Refreshable: true}
	if info != nil {
		meta.RefreshedAt = info.ModTime().UTC().Format(time.RFC3339)
	}
	if panel == nil {
		meta.Stale = true
		meta.StaleReason = "panel snapshot unavailable"
		return meta
	}
	meta.Source = "panel-summary:" + firstNonEmpty(panel.Source, "unknown") + "+us-stocks"
	panelTime, panelOK := parseLooseDataTime(panel.GeneratedAt)
	universe, _, err := scoring.LoadUsStocks(scoring.DefaultUsStocksPaths(root)...)
	if err != nil || universe == nil {
		meta.DataTime = firstNonEmpty(panel.GeneratedAt, "unknown")
		meta.Stale = true
		meta.StaleReason = "critical us-stocks input unavailable"
		return meta
	}
	universeTime, universeOK := parseLooseDataTime(universe.GeneratedAt)
	if !panelOK || !universeOK {
		meta.DataTime = "unknown"
		meta.Stale = true
		meta.StaleReason = "panel or us-stocks data time is unknown"
		return meta
	}
	oldest := panelTime
	if universeTime.Before(oldest) {
		oldest = universeTime
	}
	meta.DataTime = oldest.UTC().Format(time.RFC3339)
	panelStale, panelReason := staleIfOlder(panel.GeneratedAt, panelSummaryFreshFor)
	inputStale, inputReason := staleIfOlder(universe.GeneratedAt, 24*time.Hour)
	meta.Stale = panelStale || inputStale
	if panelStale {
		meta.StaleReason = "panel: " + panelReason
	}
	if inputStale {
		if meta.StaleReason != "" {
			meta.StaleReason += "; "
		}
		meta.StaleReason += "us-stocks: " + inputReason
	}
	return meta
}

func newestExistingFile(paths []string) (string, os.FileInfo, bool) {
	var (
		bestPath string
		bestInfo os.FileInfo
	)
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if bestInfo == nil || info.ModTime().After(bestInfo.ModTime()) {
			bestPath = p
			bestInfo = info
		}
	}
	if bestPath == "" {
		return "", nil, false
	}
	return bestPath, bestInfo, true
}

// RefreshPanelSummary handles POST /api/panel-summary/refresh
// Query: industry=1, seed=1, llm=1, llm_industry=1, limit=150, refresh_cache=0
func RefreshPanelSummary(c *gin.Context) {
	root := findBackendRoot()
	industry := c.Query("industry") != "0" && c.Query("industry") != "false"
	seed := c.Query("seed") != "0" && c.Query("seed") != "false"
	useLLM := c.Query("llm") == "1" || c.Query("llm") == "true"
	llmIndustry := c.Query("llm_industry") == "1" || c.Query("llm_industry") == "true"
	limit := 150
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	opt := scoring.RegenerateOptions{
		WriteIndustry: industry,
		SeedOriginal:  seed,
		LLMIndustry:   llmIndustry,
		IndustryLimit: limit,
		LLM: scoring.LLMOptions{
			Enabled:    useLLM,
			Limit:      limit,
			BatchSize:  12,
			Workers:    2,
			CachePath:  scoring.DefaultLLMCachePath(root),
			SkipCached: c.Query("refresh_cache") == "1" || c.Query("refresh_cache") == "true",
		},
	}
	if useLLM || llmIndustry {
		cfg := config.Load()
		client, err := llm.NewClient(cfg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "llm: " + err.Error()})
			return
		}
		opt.Client = client
	}

	panel, written, err := scoring.RegenerateWith(root, opt)
	if err != nil {
		staleDataError(c, http.StatusInternalServerError, dataFreshnessMeta{
			Source:      "panel-summary-regenerated",
			Refreshable: true,
		}, err.Error())
		return
	}
	setDataFreshness(c, panelSummaryFreshness(root, panel, nil))
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"count":   panel.Count,
		"source":  panel.Source,
		"written": written,
		"samples": gin.H{
			"NVDA": panel.Stocks["NVDA"],
			"AAPL": panel.Stocks["AAPL"],
			"TSM":  panel.Stocks["TSM"],
		},
	})
}

func findBackendRoot() string {
	candidates := []string{
		".",
		"app/backend",
		filepath.Join("..", "backend"),
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append([]string{wd}, candidates...)
	}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "go.mod")); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	wd, _ := os.Getwd()
	return wd
}
