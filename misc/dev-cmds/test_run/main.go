package main

import (
	"context"
	"fmt"
	"log"

	"trading-agents/internal/agents"
	"trading-agents/internal/config"
	"trading-agents/internal/llm"
	"trading-agents/internal/orchestrator"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Go backend integration test...")

	cfg := config.Load()
	if cfg.GoogleAPIKey == "" {
		log.Fatal("GOOGLE_API_KEY is empty in configuration. Please check your .env file.")
	}

	log.Printf("Using Deep LLM: %s, Quick LLM: %s", cfg.DeepThinkLLM, cfg.QuickThinkLLM)

	client, err := llm.NewGeminiClient(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize Gemini client: %v", err)
	}

	orch := orchestrator.New(cfg, client)
	orch.SetEventHandler(func(event agents.NodeEvent) {
		if event.Type == "node_start" {
			log.Printf("[FLOW] Node %s started...", event.Node)
		} else if event.Type == "node_complete" {
			log.Printf("[FLOW] Node %s completed.", event.Node)
			if event.Node == "Portfolio Manager" {
				fmt.Printf("\n=== PORTFOLIO MANAGER DECISION ===\n%s\n==================================\n", event.Content)
			}
		} else if event.Type == "progress" {
			log.Printf("[FLOW] Progress: %s (%d%%)", event.Node, event.Progress)
		} else if event.Type == "analysis_error" {
			log.Printf("[ERROR] Node %s: %s", event.Node, event.Content)
		}
	})

	req := agents.AnalysisRequest{
		Ticker:    "NVDA",
		TradeDate: "2024-05-10",
		AssetType: "stock",
	}

	log.Println("Triggering orchestrator run for NVDA on 2024-05-10...")
	res, err := orch.RunAnalysis(context.Background(), req)
	if err != nil {
		log.Fatalf("Analysis failed: %v", err)
	}

	fmt.Printf("\n🎉 SUCCESS: Code runs perfectly! Final decision: %s (Duration: %.2f seconds)\n", 
		res.Decision, res.DurationSecs)
}
