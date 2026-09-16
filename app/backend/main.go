// Package main 是后端可执行程序的入口。
//
// 学习路径建议：
//  1. 先看本文件的组装顺序（数据库 → 配置 → LLM → 编排器 → 路由 → Listen）
//  2. 再打开 internal/api/router.go 看有哪些 HTTP 路径
//  3. 行情相关看 internal/market；股票列表看 internal/api/stocks_handlers.go
//
// 运行（在 app/backend 目录）：
//
//	go build -o /tmp/tradingagents-backend .
//	STOCKGOD_LOCAL_FRONTEND=true STOCKGOD_REPLAY=false /tmp/tradingagents-backend
//	# Evidence-only workstation without background universe refresh:
//	STOCKGOD_DISABLE_LIVE_REFRESH=true /tmp/tradingagents-backend
package main

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"trading-agents/internal/api"
	"trading-agents/internal/config"
	"trading-agents/internal/database"
	"trading-agents/internal/etfrefresh"
	"trading-agents/internal/gpupricing"
	"trading-agents/internal/llm"
	"trading-agents/internal/market"
	"trading-agents/internal/orchestrator"
)

func main() {
	// Lshortfile：日志带上文件名:行号，排错时更好定位
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("TradingAgents Go Backend starting...")

	// 从仓库根或当前目录的 .env + 进程环境变量加载配置
	cfg := config.Load()
	fixtureEnabled, err := api.ConfigurePaperRuntimeFixtureFromEnvironment(cfg)
	if err != nil {
		log.Fatalf("Failed to configure Paper runtime fixture: %v", err)
	}
	gpuFixtureEnabled, err := gpupricing.ConfigureRuntimeFixtureFromEnvironment(cfg)
	if err != nil {
		log.Fatalf("Failed to configure GPU runtime fixture: %v", err)
	}

	// SQLite 默认放在用户数据目录；可用 STOCKGOD_DATABASE_DIR 显式覆盖。
	database.InitDB(cfg.DatabaseDir)
	if resolved, err := api.ReconcilePendingAuditOutcomes(); err != nil {
		log.Fatalf("Failed to reconcile pending audit outcomes: %v", err)
	} else if resolved > 0 {
		log.Printf("Reconciled %d audit outcomes interrupted by the previous process", resolved)
	}
	log.Printf("Config loaded: provider=%s, deep=%s, quick=%s",
		cfg.LLMProvider, cfg.DeepThinkLLM, cfg.QuickThinkLLM)

	// 启动即刷美股 live 缓存：页面刷新读 /api/market 而不是陈旧 us-stocks 价
	if backgroundLiveRefreshEnabled(fixtureEnabled, gpuFixtureEnabled) {
		market.Default().StartLiveRefresh()
	} else if !fixtureEnabled && !gpuFixtureEnabled {
		log.Println("Background live market refresh disabled by STOCKGOD_DISABLE_LIVE_REFRESH")
	} else {
		log.Println("Background live market refresh disabled for temporary acceptance fixture")
	}

	// 工厂模式：按 TRADINGAGENTS_LLM_PROVIDER 选择 Gemini / DeepSeek / OpenAI…
	llmClient, err := llm.NewClient(cfg)
	if err != nil {
		// Fatalf = 打印错误并 os.Exit(1)，适合「没有 LLM 就不要继续」的启动失败
		log.Fatalf("Failed to initialize LLM client: %v", err)
	}
	log.Printf("LLM client initialized: provider=%s", cfg.LLMProvider)

	// 编排多智能体分析（驾驶舱 /api/analysis/* 会用到）
	orch := orchestrator.New(cfg, llmClient)

	processCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	gpuPrices := gpupricing.NewService(cfg, database.DB)
	gpuPrices.Start(processCtx)
	etfRefresh := etfrefresh.NewService(etfrefresh.Config{})
	etfRefresh.Start(processCtx)
	var stopScanner *api.PaperStopScanner
	if !strings.EqualFold(os.Getenv("STOCKGOD_REPLAY"), "true") && !strings.EqualFold(os.Getenv("STOCKGOD_LIVE_MIRROR"), "true") {
		stopScanner = api.NewPaperStopScanner(database.DB, cfg)
		if err := stopScanner.Start(processCtx); err != nil {
			log.Fatalf("Failed to start local Paper stop scanner: %v", err)
		}
	}

	// 注册全部 REST + WebSocket；默认自建数据，不反代源站
	router := api.SetupRouterWithRuntimeContext(processCtx, cfg, orch, stopScanner, gpuPrices, etfRefresh)

	addr := fmt.Sprintf("127.0.0.1:%s", cfg.Port)
	log.Printf("Server listening on %s", addr)
	log.Printf("  REST API: http://localhost:%s/api/health", cfg.Port)
	log.Printf("  WebSocket: ws://localhost:%s/ws", cfg.Port)

	server := &http.Server{
		Addr: addr, Handler: compressStaticAssets(router),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      4 * time.Minute,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	var serveErr error
	select {
	case <-processCtx.Done():
	case serveErr = <-serverErrors:
	}
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if stopScanner != nil {
		if err := stopScanner.Stop(shutdownCtx); err != nil {
			log.Printf("Paper stop scanner shutdown failed: %v", err)
		}
	}
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown failed: %v", err)
	}
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		log.Fatalf("Server failed: %v", serveErr)
	}
}

type gzipStaticResponseWriter struct {
	http.ResponseWriter
	writer *gzip.Writer
}

func (w gzipStaticResponseWriter) WriteHeader(statusCode int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w gzipStaticResponseWriter) Write(data []byte) (int, error) {
	w.Header().Del("Content-Length")
	return w.writer.Write(data)
}

func compressStaticAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.Method != http.MethodGet && r.Method != http.MethodHead) ||
			!strings.HasPrefix(r.URL.Path, "/assets/") ||
			!strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		compressed := gzip.NewWriter(w)
		defer compressed.Close()
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Del("Content-Length")
		next.ServeHTTP(gzipStaticResponseWriter{ResponseWriter: w, writer: compressed}, r)
	})
}

func liveRefreshEnabled() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv("STOCKGOD_DISABLE_LIVE_REFRESH")), "true")
}

func backgroundLiveRefreshEnabled(paperFixtureEnabled, gpuFixtureEnabled bool) bool {
	return !paperFixtureEnabled && !gpuFixtureEnabled && liveRefreshEnabled()
}
