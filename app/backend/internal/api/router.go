package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"trading-agents/internal/config"
	"trading-agents/internal/orchestrator"
)

// SetupRouter 创建 Gin 引擎并挂上全部路由。
//
// 三种模式（互斥，按环境变量早退）：
//  1. STOCKGOD_REPLAY=true      → 只播本地回放 HTML/JSON
//  2. STOCKGOD_LIVE_MIRROR=true → 反代 stockgod.xyz（默认禁止）
//  3. 默认                     → 自建 /api/* + 可选托管 frontend/dist
//
// 新手看路由表即可理解「前端哪个页面打哪个接口」。
func SetupRouter(cfg *config.Config, orch *orchestrator.Orchestrator) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 允许 Vite 开发服与本机直连；wails:// 留给可选桌面壳
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:5173",
			"http://localhost:3000",
			"http://127.0.0.1:5173",
			"http://localhost:8765",
			"http://127.0.0.1:8765",
		},
		AllowOriginFunc: func(origin string) bool {
			return strings.HasPrefix(origin, "wails://") || origin == "null"
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Hub：WebSocket 连接池，分析进度会推给前端
	hub := NewHub()
	// Handler：带 cfg/orch/hub 的方法接收者（多数驾驶舱接口）
	handler := NewHandler(cfg, orch, hub)

	// --- 特殊模式：本地回放 ---
	if strings.ToLower(os.Getenv("STOCKGOD_REPLAY")) == "true" {
		r.GET("/api/health", handler.HealthCheck)
		r.GET("/ws", func(c *gin.Context) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		})
		configureStockGodReplayOnly(r)
		return r
	}

	// --- 特殊模式：源站镜像（仅显式开启）---
	if strings.ToLower(os.Getenv("STOCKGOD_LIVE_MIRROR")) == "true" {
		r.GET("/api/health", handler.HealthCheck)
		r.GET("/ws", func(c *gin.Context) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		})
		configureStockGodMirror(r)
		return r
	}

	r.GET("/ws", func(c *gin.Context) {
		hub.HandleWebSocket(c.Writer, c.Request)
	})

	// /api 分组：下面每一段对应一块产品能力
	api := r.Group("/api", noStoreForDataResponses())
	{
		api.GET("/health", handler.HealthCheck)

		// 驾驶舱：多智能体分析
		analysis := api.Group("/analysis")
		{
			analysis.POST("/start", handler.StartAnalysis)
			analysis.POST("/stop", handler.StopAnalysis)
			analysis.GET("/status", handler.GetStatus)
			analysis.GET("/history", handler.GetHistory)
		}

		api.GET("/config", handler.GetConfig)
		api.PUT("/config", handler.UpdateConfig)
		api.GET("/system/status", handler.GetSystemStatus)

		// 驾驶舱：交易日记 / 事件
		api.GET("/trades", handler.GetTrades)
		api.POST("/trades", handler.CreateTrade)
		api.GET("/stats", handler.GetStats)
		api.GET("/events", handler.GetEvents)
		api.POST("/events", handler.CreateEvent)

		// StockGod：聪明钱 / 国会交易
		api.GET("/whales/gurus", handler.GetGurus)
		api.GET("/whales/gurus/:id", handler.GetGuruDetails)
		api.GET("/whales/congress", handler.GetCongressTrades)
		api.GET("/whales/congress/:slug", handler.GetCongressPolitician)
		api.GET("/whales/consensus", handler.GetSmartMoneyConsensus)
		api.GET("/whales/stock/:symbol", handler.GetStockHolders)
		api.GET("/whales/status", handler.GetWhalesSyncStatus)
		api.POST("/whales/sync", handler.TriggerWhalesSync)

		api.GET("/etf/sectors", handler.GetETFSectors)
		api.GET("/etf/sectors/:id", handler.GetETFSectorDetail)
		api.GET("/etf/search", handler.SearchETFs)

		api.GET("/reports", handler.GetReports)
		api.GET("/reports/:id", handler.GetReportByID)
		api.GET("/market/calendar", handler.GetMarketCalendar)

		api.GET("/arena", handler.GetArena)

		api.GET("/notes/toc", handler.GetNotesTOC)
		api.GET("/notes/art", handler.GetNotesArticle)

		// 扫描页股票宇宙（务必尊重 market=us|cn）
		api.GET("/stocks", handler.GetStocks)
		api.GET("/stocks/search", handler.SearchStocks)

		api.GET("/heatmap", GetHeatmapData)

		// 行情 / 宏观（实现见 mock_handlers.go + market 包）
		api.GET("/premarket-movers", GetPremarketMovers)
		api.GET("/macro", GetMacroData)
		api.GET("/market", GetMarketData)
		api.GET("/a-market", GetAMarketData)
		api.GET("/quote", GetQuoteData)

		api.GET("/panel-summary", GetPanelSummary)
		api.POST("/panel-summary/refresh", RefreshPanelSummary)
		api.GET("/fundamentals", GetFundamentalsData)
		api.GET("/news", GetStockNewsData)
	}

	// 无匹配的前端路由交给 SPA（dist）
	configureFrontend(r)

	return r
}

// configureStockGodReplayOnly serves captured replay HTML/JSON only — no live upstream.
func configureStockGodReplayOnly(r *gin.Engine) {
	replay := loadStockGodReplay()
	serve := func(c *gin.Context) {
		if replay != nil && replay.serve(c) {
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "stockgod live upstream disabled; replay miss",
			"path":  c.Request.URL.Path,
		})
	}
	r.GET("/", serve)
	r.HEAD("/", serve)
	r.NoRoute(func(c *gin.Context) {
		if c.Request.URL.Path == "/api/health" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		serve(c)
	})
}

func configureStockGodMirror(r *gin.Engine) {
	target, _ := url.Parse("https://stockgod.xyz")
	replay := loadStockGodReplay()
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Header.Set("Host", target.Host)
		req.Header.Set("Origin", target.String())
		req.Header.Set("Referer", target.String()+"/")
		req.Header.Set("Accept-Encoding", "identity")
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("Content-Encoding")
		resp.Header.Del("Content-Length")
		resp.Header.Del("Content-Security-Policy")
		resp.Header.Del("X-Frame-Options")
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		http.Error(w, "StockGod live mirror unavailable: "+err.Error(), http.StatusBadGateway)
	}

	mirror := func(c *gin.Context) {
		if replay != nil && replay.serve(c) {
			return
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	}

	r.GET("/", mirror)
	r.HEAD("/", mirror)
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/api/health" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		mirror(c)
	})
}

type stockGodReplay struct {
	root                string
	pages               map[string]stockGodReplayEntry
	responses           map[string]stockGodReplayEntry
	normalizedResponses map[string]stockGodReplayEntry
}

type stockGodReplayEntry struct {
	File        string `json:"file"`
	Status      int    `json:"status"`
	ContentType string `json:"contentType"`
}

func loadStockGodReplay() *stockGodReplay {
	if strings.ToLower(os.Getenv("STOCKGOD_REPLAY")) != "true" {
		return nil
	}

	root := os.Getenv("STOCKGOD_REPLAY_DIR")
	if root == "" {
		root = findStockGodReplayDir()
	}
	if root == "" {
		return nil
	}

	manifestPath := filepath.Join(root, "manifest.json")
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var manifest struct {
		Pages     map[string]stockGodReplayEntry `json:"pages"`
		Responses map[string]stockGodReplayEntry `json:"responses"`
	}
	if err := json.NewDecoder(file).Decode(&manifest); err != nil {
		return nil
	}

	replay := &stockGodReplay{
		root:                root,
		pages:               manifest.Pages,
		responses:           manifest.Responses,
		normalizedResponses: map[string]stockGodReplayEntry{},
	}
	for key, entry := range manifest.Responses {
		if normalized := normalizeReplayKey(key); normalized != key {
			replay.normalizedResponses[normalized] = entry
		}
	}
	return replay
}

func (s *stockGodReplay) serve(c *gin.Context) bool {
	if c.Request.Method == http.MethodOptions {
		c.Status(http.StatusNoContent)
		return true
	}
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.Request.Method != http.MethodPost {
		return false
	}

	key := c.Request.URL.RequestURI()
	entry, ok := s.responses[key]
	if !ok {
		entry, ok = s.normalizedResponses[normalizeReplayKey(key)]
	}
	if !ok {
		entry, ok = s.pages[key]
	}
	if !ok {
		entry, ok = s.pages[c.Request.URL.Path]
	}
	if !ok && c.Request.Method == http.MethodPost && isReplayPostFallbackPath(c.Request.URL.Path) {
		getKey := c.Request.URL.RequestURI()
		entry, ok = s.responses[getKey]
		if !ok {
			entry, ok = s.normalizedResponses[normalizeReplayKey(getKey)]
		}
	}
	if !ok || entry.File == "" {
		if s.serveMissingStaticChunk(c) {
			return true
		}
		return false
	}

	fullPath := filepath.Join(s.root, filepath.Clean(entry.File))
	if !strings.HasPrefix(fullPath, filepath.Clean(s.root)+string(os.PathSeparator)) {
		return false
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return false
	}
	defer file.Close()

	contentType := entry.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	status := entry.Status
	if status == 0 {
		status = http.StatusOK
	}

	c.Header("Content-Type", contentType)
	c.Header("X-StockGod-Replay", "hit")
	if c.Request.Method == http.MethodPost {
		c.Header("X-StockGod-Replay-Method-Fallback", "GET")
	}
	if strings.HasPrefix(contentType, "text/html") {
		c.Header("Clear-Site-Data", "\"cache\"")
	}
	c.Status(status)
	if c.Request.Method == http.MethodHead {
		return true
	}
	if strings.HasPrefix(contentType, "text/html") {
		bodyBytes, err := io.ReadAll(file)
		if err != nil {
			return true
		}
		body := strings.ReplaceAll(string(bodyBytes), ".css?dpl=", ".css?sgcss=20260711&dpl=")
		_, _ = c.Writer.Write([]byte(body))
		return true
	}
	_, _ = io.Copy(c.Writer, file)
	return true
}

func (s *stockGodReplay) serveMissingStaticChunk(c *gin.Context) bool {
	path := c.Request.URL.Path
	if !strings.HasPrefix(path, "/_next/static/chunks/") {
		return false
	}

	contentType := ""
	body := ""
	switch {
	case strings.HasSuffix(path, ".js"):
		contentType = "application/javascript; charset=utf-8"
		body = "\n"
	case strings.HasSuffix(path, ".css"):
		contentType = "text/css; charset=utf-8"
		cssPath := filepath.Join(s.root, "static", "fallback.css")
		if file, err := os.Open(cssPath); err == nil {
			defer file.Close()
			c.Header("Content-Type", contentType)
			c.Header("Cache-Control", "no-store")
			c.Header("X-StockGod-Replay", "static-fallback-css")
			c.Status(http.StatusOK)
			if c.Request.Method == http.MethodHead {
				return true
			}
			_, _ = io.Copy(c.Writer, file)
			return true
		}
		body = "\n"
	default:
		return false
	}

	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "no-store")
	c.Header("X-StockGod-Replay", "static-fallback")
	c.Status(http.StatusOK)
	if c.Request.Method == http.MethodHead {
		return true
	}
	_, _ = c.Writer.Write([]byte(body))
	return true
}

func isReplayPostFallbackPath(path string) bool {
	return strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/data/") || path == "/" || strings.HasPrefix(path, "/stock/") || strings.HasPrefix(path, "/whales/")
}

func normalizeReplayKey(rawKey string) string {
	parsed, err := url.ParseRequestURI(rawKey)
	if err != nil || parsed.RawQuery == "" {
		return rawKey
	}

	query := parsed.Query()
	if _, ok := query["_rsc"]; !ok {
		return rawKey
	}
	query.Set("_rsc", "*")

	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		values := query[key]
		sort.Strings(values)
		for _, value := range values {
			if key == "_rsc" {
				parts = append(parts, "_rsc=*")
				continue
			}
			parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(value))
		}
	}
	return parsed.Path + "?" + strings.Join(parts, "&")
}

func findStockGodReplayDir() string {
	candidates := []string{
		"../../recovered_source/stockgod-replay",
		"../recovered_source/stockgod-replay",
		"recovered_source/stockgod-replay",
	}

	for _, candidate := range candidates {
		if fileExists(filepath.Join(candidate, "manifest.json")) {
			if absolute, err := filepath.Abs(candidate); err == nil {
				return absolute
			}
			return candidate
		}
	}
	return ""
}

func configureFrontend(r *gin.Engine) {
	distDir := findFrontendDist()
	if distDir == "" {
		r.GET("/", func(c *gin.Context) {
			c.String(http.StatusOK, "Frontend build not found. Run `npm run build` in app/frontend, or open the Vite dev server on http://127.0.0.1:5173/.")
		})
		return
	}

	r.StaticFS("/assets", http.Dir(filepath.Join(distDir, "assets")))
	dataDir := filepath.Join(distDir, "data")
	if fileExists(filepath.Join(dataDir, "reports-latest.json")) {
		r.GET("/data/*filepath", noStoreForDataResponses(), func(c *gin.Context) {
			c.FileFromFS(c.Param("filepath"), http.Dir(dataDir))
		})
	}
	r.StaticFile("/logo.png", filepath.Join(distDir, "logo.png"))
	r.StaticFile("/favicon.ico", filepath.Join(distDir, "favicon.ico"))

	r.GET("/", func(c *gin.Context) {
		serveIndexWithDynamicTitle(c, distDir)
	})

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") || path == "/api" || path == "/ws" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		if filePath := filepath.Join(distDir, filepath.Clean(path)); fileExists(filePath) {
			c.File(filePath)
			return
		}

		serveIndexWithDynamicTitle(c, distDir)
	})
}

func noStoreForDataResponses() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Cache-Control", "no-store, max-age=0")
		c.Writer.Header().Set("Pragma", "no-cache")
		c.Next()
	}
}

func serveIndexWithDynamicTitle(c *gin.Context, distDir string) {
	indexPath := filepath.Join(distDir, "index.html")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	html := string(content)
	path := c.Request.URL.Path
	pageTitle := "我不是股神 · Not a Stock God"

	if path == "/" {
		pageTitle = "我不是股神 · Not a Stock God"
	} else if path == "/scan" {
		pageTitle = "全市场扫描 · 美股+A股五方判读 · 我不是股神"
	} else if path == "/etf" {
		pageTitle = "ETF · 板块业绩 | 我不是股神"
	} else if path == "/whales" {
		pageTitle = "聪明钱 · 名人持仓 + 国会交易 | 我不是股神"
	} else if strings.HasPrefix(path, "/whales/") {
		id := strings.TrimPrefix(path, "/whales/")
		gurus := map[string]string{
			"howard-marks":          "霍华德·马克斯(Oaktree Capital Management)",
			"warren-buffett":        "沃伦·巴菲特(Berkshire Hathaway)",
			"michael-burry":         "迈克尔·贝里(Scion Asset Management)",
			"bill-gates":            "比尔·盖茨(Bill & Melinda Gates Foundation)",
			"charlie-munger":        "查理·芒格(Daily Journal)",
			"li-lu":                 "李录(Himalaya Capital)",
			"charlie-munger-estate": "查理·芒格遗产",
			"bill-ackman":           "比尔·阿克曼(Pershing Square)",
			"mohnish-pabrai":        "莫尼什·帕伯莱(Pabrai Investment Funds)",
			"guy-spier":             "盖伊·斯皮尔(Aquamarine Capital)",
			"david-tepper":          "大卫·泰珀(Appaloosa Management)",
			"ray-dalio":             "雷·达里奥(Bridgewater Associates)",
			"ken-griffin":           "肯·格里芬(Citadel Advisors)",
			"steve-cohen":           "史蒂夫·科恩(Point72 Asset Management)",
			"stanley-druckenmiller": "斯坦利·德鲁肯米勒(Duquesne Family Office)",
			"chase-coleman":         "蔡斯·科尔曼(Tiger Global Management)",
			"jim-simons":            "吉姆·西蒙斯(Renaissance Technologies)",
			"daniel-loeb":           "丹尼尔·勒布(Third Point)",
			"nelson-peltz":          "尼尔森·佩尔茨(Trian Fund Management)",
			"carl-icahn":            "卡尔·伊坎(Icahn Enterprises)",
		}
		name := "聪明钱"
		if displayName, ok := gurus[id]; ok {
			name = displayName
		} else {
			name = strings.Title(strings.ReplaceAll(id, "-", " "))
		}
		pageTitle = name + "的 13F 持仓 · 我不是股神"
	} else if path == "/arena" {
		pageTitle = "五神对决 · 段永平/巴菲特/Serenity/德鲁肯米勒/情绪 虚拟盘 · 我不是股神"
	} else if path == "/reports" {
		pageTitle = "盘报 · 盘前看点 + 收盘复盘 · 我不是股神"
	} else if path == "/portfolio" {
		pageTitle = "我不是股神 · Not a Stock God"
	} else if strings.HasPrefix(path, "/stock/") {
		sym := strings.ToUpper(strings.TrimPrefix(path, "/stock/"))
		companies := map[string]string{
			"NVDA":  "NVIDIA Corporation",
			"AAPL":  "Apple Inc.",
			"MSFT":  "Microsoft Corporation",
			"AMZN":  "Amazon.com Inc.",
			"GOOGL": "Alphabet Inc.",
			"GOOG":  "Alphabet Inc.",
			"META":  "Meta Platforms Inc.",
			"TSLA":  "Tesla Inc.",
			"AVGO":  "Broadcom Inc.",
			"LLY":   "Eli Lilly and Company",
		}
		comp := companies[sym]
		if comp != "" {
			pageTitle = comp + "(" + sym + ")股价、估值与机构持仓分析 | 我不是股神"
		} else {
			pageTitle = sym + "股价、估值与机构持仓分析 | 我不是股神"
		}
	} else if path == "/about" {
		pageTitle = "关于 / 方法论 · About | 我不是股神"
	} else if path == "/terms" {
		pageTitle = "服务条款 · Terms | 我不是股神"
	} else if path == "/privacy" {
		pageTitle = "隐私政策 · Privacy | 我不是股神"
	} else if path == "/how-to-buy" {
		pageTitle = "如何买美股 · 最低门槛路径 | 我不是股神"
	}

	titleStart := strings.Index(html, "<title>")
	titleEnd := strings.Index(html, "</title>")
	if titleStart != -1 && titleEnd != -1 && titleEnd > titleStart {
		html = html[:titleStart+7] + pageTitle + html[titleEnd:]
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}

func findFrontendDist() string {
	candidates := []string{
		"../frontend/dist",
		"app/frontend/dist",
		"./frontend/dist",
	}

	for _, candidate := range candidates {
		if fileExists(filepath.Join(candidate, "index.html")) {
			if absolute, err := filepath.Abs(candidate); err == nil {
				return absolute
			}
			return candidate
		}
	}

	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
