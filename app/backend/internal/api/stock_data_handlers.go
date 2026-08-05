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
	"trading-agents/internal/market"
	"trading-agents/internal/scoring"
)

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
	at   time.Time
	data []dataflows.NewsItem
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
			c.JSON(http.StatusOK, cached.data)
			return
		}
	}
	m, err := yfClient.GetStockMetricsLive(sym)
	if err != nil {
		// 兼容性兜底：当 Yahoo Finance 基本面拉取失败时，尝试获取实时报价进行最小补洞以防页面空白或崩溃
		live := market.Default().Quotes([]string{sym})
		if q, ok := live[sym]; ok && q.Price > 0 {
			fallback := &dataflows.StockMetrics{
				Symbol: sym,
				Name:   sym,
				Price:  q.Price,
				Source: "quote-fallback-compat",
			}
			metricsCache.Store(sym, cachedMetrics{at: time.Now(), data: fallback})
			c.JSON(http.StatusOK, fallback)
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "symbol": sym})
		return
	}
	metricsCache.Store(sym, cachedMetrics{at: time.Now(), data: m})
	c.JSON(http.StatusOK, m)
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
			c.JSON(http.StatusOK, gin.H{"symbol": sym, "news": cached.data, "source": "yahoo-search"})
			return
		}
	}
	items, err := yfClient.GetNews(sym, 10)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "symbol": sym})
		return
	}
	newsCache.Store(sym, cachedNews{at: time.Now(), data: items})
	c.JSON(http.StatusOK, gin.H{"symbol": sym, "news": items, "source": "yahoo-search"})
}

// GetPanelSummary handles GET /api/panel-summary — self-built five-factor scores.
func GetPanelSummary(c *gin.Context) {
	root := findBackendRoot()
	paths := scoring.DefaultPanelPaths(root)
	paths = append(paths,
		filepath.Join(root, "..", "frontend", "dist", "data", "us-panel-summary.json"),
		filepath.Join("app", "frontend", "public", "data", "us-panel-summary.json"),
	)
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var panel scoring.PanelFile
		if err := json.Unmarshal(raw, &panel); err != nil {
			continue
		}
		if panel.Order == nil {
			panel.Order = scoring.PanelOrder
		}
		c.Header("X-Panel-Path", p)
		c.JSON(http.StatusOK, panel)
		return
	}
	// On-demand generate if missing
	panel, written, err := scoring.Regenerate(root, false)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.Header("X-Panel-Path", strings.Join(written, ","))
	c.JSON(http.StatusOK, panel)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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
