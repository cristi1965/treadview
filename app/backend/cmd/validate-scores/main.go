package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"trading-agents/internal/scoring"
)

func main() {
	input := flag.String("input", "", "JSON array of historical score samples")
	cutoff := flag.String("cutoff", "", "fixed train/test cutoff date (YYYY-MM-DD)")
	output := flag.String("output", "", "optional JSON report path; stdout when empty")
	flag.Parse()
	report := scoring.ValidationReport{
		Status: "unvalidated", Method: "strict-time-split-v1",
		Benchmark: "benchmark-adjusted forward return; majority-direction baseline",
		Reasons:   []string{"historical input and cutoff are required"},
	}
	if *input != "" && *cutoff != "" {
		raw, err := os.ReadFile(*input)
		if err != nil {
			report.Reasons = []string{err.Error()}
		} else {
			var samples []scoring.HistoricalScoreSample
			if err := json.Unmarshal(raw, &samples); err != nil {
				report.Reasons = []string{err.Error()}
			} else {
				report = scoring.ValidateHistoricalScores(samples, *cutoff)
			}
		}
	}
	raw, _ := json.MarshalIndent(report, "", "  ")
	if *output != "" {
		if err := os.WriteFile(*output, raw, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	fmt.Println(string(raw))
}
