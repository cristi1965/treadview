package main

import (
	"fmt"
	"os"
	"time"

	"trading-agents/internal/dataflows"
)

func main() {
	fmt.Println("=== TradingView WebSocket Test ===")

	tv := dataflows.NewTVClient()
	defer tv.Close()

	fmt.Println("Connecting to TradingView WebSocket...")
	if err := tv.Connect(); err != nil {
		fmt.Printf("❌ Connection failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Connected!")

	// Test: subscribe to a few stocks
	symbols := []string{"NASDAQ:AAPL", "NYSE:SPY", "AMEX:SOXL"}
	fmt.Printf("\nSubscribing to: %v\n", symbols)

	if err := tv.SubscribeQuotes(symbols); err != nil {
		fmt.Printf("❌ Subscribe failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Subscribed!")

	fmt.Println("\nWaiting for real-time quotes (up to 10s)...")

	for _, s := range symbols {
		quote, err := tv.WaitForQuote(s, 10*time.Second)
		if err != nil {
			fmt.Printf("  %s: ❌ %v\n", s, err)
			continue
		}
		fmt.Printf("  %s: $%.2f | Change: %.2f%% | Volume: %.0f\n",
			s, quote.Price, quote.ChangePct, quote.Volume)
	}

	// Also test GetCurrentPrice convenience method
	fmt.Println("\n--- Convenience API Test ---")
	if price, err := tv.GetCurrentPrice("NASDAQ:AAPL"); err != nil {
		fmt.Printf("  AAPL: ❌ %v\n", err)
	} else {
		fmt.Printf("  AAPL current price: $%.2f\n", price)
	}

	fmt.Println("\n=== Test Complete ===")
}
