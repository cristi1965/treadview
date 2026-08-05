package main

import (
	"fmt"
	"os"

	"trading-agents/internal/dataflows"
)

func main() {
	fmt.Println("=== TradingView REST API Test ===")

	tv := dataflows.NewTVRestClient()

	// Test single quote
	fmt.Println("\n--- Single Quote: AAPL ---")
	item, err := tv.GetRealTimeQuote("AAPL")
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Println(dataflows.FormatQuoteForLLM(*item, "AAPL"))
	}

	// Test batch
	fmt.Println("\n--- Batch Quotes: AAPL, NVDA, MSFT, SOXL ---")
	items, err := tv.GetRealTimeQuotes([]string{"AAPL", "NVDA", "MSFT", "SOXL"})
	if err != nil {
		fmt.Printf("❌ Batch error: %v\n", err)
		os.Exit(1)
	}

	for _, it := range items {
		fmt.Printf("  %s: $%.2f (%.2f%%) vol=%.0f\n",
			it.Symbol, it.Price, it.ChangePct, it.Volume)
	}

	fmt.Println("\n--- Formatted for LLM ---")
	fmt.Println(dataflows.FormatQuotesForLLM(items))

	fmt.Println("=== Test Complete ===")
}
