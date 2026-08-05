package main

import (
	"encoding/json"
	"fmt"
	"os"

	"trading-agents/internal/dataflows"
)

func main() {
	sym := "NVDA"
	if len(os.Args) > 1 {
		sym = os.Args[1]
	}
	c := dataflows.NewYFinanceClient()
	m, err := c.GetStockMetricsLive(sym)
	if err != nil {
		fmt.Println("ERR", err)
		os.Exit(1)
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	fmt.Println(string(b))
}
