package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"trading-agents/internal/config"
	"trading-agents/internal/dataflows"
	"trading-agents/internal/llm"
)

func main() {
	fmt.Println("=== TradingView Real-Time Analysis Test ===")
	fmt.Println()

	ticker := "SOXL"
	tradeDate := time.Now().Format("2006-01-02")

	// Load config
	cfg := config.Load()
	cfg.LLMProvider = "deepseek"

	// Create LLM client
	client, err := llm.NewClient(cfg)
	if err != nil {
		fmt.Printf("❌ LLM init failed: %v\n", err)
		os.Exit(1)
	}

	// Create tool registry
	tools := dataflows.NewToolRegistry(ticker, tradeDate, cfg.FREDAPIKey)

	fmt.Printf("Analyzing %s on %s using DeepSeek + TradingView real-time data...\n\n", ticker, tradeDate)

	// Simple prompt that uses the real-time quote tool
	systemPrompt := fmt.Sprintf(`You are a trading assistant. Today is %s. Analyze %s.

First, call get_realtime_quote to get the current real-time price at millisecond latency via TradingView.
Then, provide your analysis.

Write your response in English then Chinese (简体中文), separated by "---".
`, tradeDate, ticker)

	userPrompt := fmt.Sprintf("What is the current price of %s? Is the -17.55%% drop a buying opportunity? Give me specific advice on entry price, stop loss, target, and holding period.", ticker)

	toolDefs := dataflows.MarketToolDefs()

	report, err := client.GenerateWithTools(
		context.Background(),
		systemPrompt,
		[]llm.ChatMessage{{Role: "user", Content: userPrompt}},
		toolDefs,
		true, // use deep thinking
		tools.Execute,
		nil,
	)
	if err != nil {
		fmt.Printf("❌ Analysis failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println(report)
	fmt.Println(strings.Repeat("=", 60))
}
