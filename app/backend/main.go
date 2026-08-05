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
package main

import (
	"fmt"
	"log"

	"trading-agents/internal/api"
	"trading-agents/internal/config"
	"trading-agents/internal/database"
	"trading-agents/internal/llm"
	"trading-agents/internal/orchestrator"
)

func main() {
	// Lshortfile：日志带上文件名:行号，排错时更好定位
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("TradingAgents Go Backend starting...")

	// SQLite：交易日记 / whales 等本地持久化（文件通常为 trades.db）
	database.InitDB(".")

	// 从仓库根或当前目录的 .env + 进程环境变量加载配置
	cfg := config.Load()
	log.Printf("Config loaded: provider=%s, deep=%s, quick=%s",
		cfg.LLMProvider, cfg.DeepThinkLLM, cfg.QuickThinkLLM)

	// 工厂模式：按 TRADINGAGENTS_LLM_PROVIDER 选择 Gemini / DeepSeek / OpenAI…
	llmClient, err := llm.NewClient(cfg)
	if err != nil {
		// Fatalf = 打印错误并 os.Exit(1)，适合「没有 LLM 就不要继续」的启动失败
		log.Fatalf("Failed to initialize LLM client: %v", err)
	}
	log.Printf("LLM client initialized: provider=%s", cfg.LLMProvider)

	// 编排多智能体分析（驾驶舱 /api/analysis/* 会用到）
	orch := orchestrator.New(cfg, llmClient)

	// 注册全部 REST + WebSocket；默认自建数据，不反代源站
	router := api.SetupRouter(cfg, orch)

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Server listening on %s", addr)
	log.Printf("  REST API: http://localhost:%s/api/health", cfg.Port)
	log.Printf("  WebSocket: ws://localhost:%s/ws", cfg.Port)

	// 阻塞在这里直到进程退出；失败通常是端口被占用
	if err := router.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
