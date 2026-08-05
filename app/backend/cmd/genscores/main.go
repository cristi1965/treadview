// genscores rebuilds us-panel-summary.json with optional original-seed + LLM tiered scoring.
// Usage (from app/backend):
//
//	go run ./cmd/genscores
//	go run ./cmd/genscores -industry -seed
//	go run ./cmd/genscores -llm -limit 120 -workers 2
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"trading-agents/internal/config"
	"trading-agents/internal/llm"
	"trading-agents/internal/scoring"
)

func main() {
	industry := flag.Bool("industry", true, "fill missing Chinese seg/sub on us-stocks.json")
	seed := flag.Bool("seed", true, "overlay recovered original stockgod panel scores")
	useLLM := flag.Bool("llm", false, "run DeepSeek/configured LLM batch scoring on top-N by mcap")
	llmIndustry := flag.Bool("llm-industry", false, "LLM-refine Chinese industry layers for top-N")
	limit := flag.Int("limit", 150, "max symbols for LLM scoring (mcap desc)")
	batch := flag.Int("batch", 12, "LLM batch size")
	workers := flag.Int("workers", 2, "LLM parallel workers")
	refreshCache := flag.Bool("refresh-cache", false, "ignore llm-panel-cache.json and re-score")
	flag.Parse()

	root := findBackendRoot()
	opt := scoring.RegenerateOptions{
		WriteIndustry: *industry,
		SeedOriginal:  *seed,
		LLMIndustry:   *llmIndustry,
		IndustryLimit: *limit,
		LLM: scoring.LLMOptions{
			Enabled:    *useLLM,
			Limit:      *limit,
			BatchSize:  *batch,
			Workers:    *workers,
			CachePath:  scoring.DefaultLLMCachePath(root),
			SkipCached: *refreshCache,
		},
	}

	if *useLLM || *llmIndustry {
		cfg := config.Load()
		client, err := llm.NewClient(cfg)
		if err != nil {
			log.Fatalf("llm client: %v", err)
		}
		opt.Client = client
		log.Printf("LLM enabled: provider=%s score=%v industry=%v limit=%d batch=%d workers=%d",
			cfg.LLMProvider, *useLLM, *llmIndustry, *limit, *batch, *workers)
	}

	panel, written, err := scoring.RegenerateWith(root, opt)
	if err != nil {
		log.Fatalf("regenerate: %v", err)
	}
	fmt.Printf("panel stocks=%d source=%s\n", panel.Count, panel.Source)
	for _, p := range written {
		fmt.Println("wrote", p)
	}
	for _, sym := range []string{"NVDA", "AAPL", "TSM", "JPM", "PLTR", "SMCI"} {
		if row, ok := panel.Stocks[sym]; ok {
			fmt.Printf("  %s sc=%v div=%d\n", sym, row.SC, row.Div)
		}
	}
}

func findBackendRoot() string {
	candidates := []string{".", "app/backend", filepath.Join("..", "..")}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "go.mod")); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	wd, _ := os.Getwd()
	return wd
}
