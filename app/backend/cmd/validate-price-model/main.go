// validate-price-model evaluates the market-only OHLCV submodel. It never
// changes the five-factor validation status.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"trading-agents/internal/scoring"
)

func main() {
	input := flag.String("input", "", "JSON []PriceObservation or evidence-only result")
	panelPath := flag.String("panel", "", "optional panel JSON to attach the separate market_submodel report")
	flag.Parse()
	if *input == "" {
		fmt.Fprintln(os.Stderr, "-input is required")
		os.Exit(2)
	}
	raw, err := os.ReadFile(*input)
	if err != nil {
		fatal(err)
	}
	var observations []scoring.PriceObservation
	if err := json.Unmarshal(raw, &observations); err != nil || len(observations) == 0 {
		var result struct {
			Dossier struct {
				Historical struct {
					Observations []scoring.PriceObservation `json:"observations"`
				} `json:"historical"`
			} `json:"dossier"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			fatal(err)
		}
		observations = result.Dossier.Historical.Observations
	}
	report := scoring.EvaluateOHLCVMomentum(observations)
	if *panelPath != "" {
		panelRaw, err := os.ReadFile(*panelPath)
		if err != nil {
			fatal(err)
		}
		var panel scoring.PanelFile
		if err := json.Unmarshal(panelRaw, &panel); err != nil {
			fatal(err)
		}
		panel.EnsureValidationStatus()
		panel.MarketSubmodel = &report
		updated, _ := json.MarshalIndent(panel, "", "  ")
		if err := os.WriteFile(*panelPath, append(updated, '\n'), 0o644); err != nil {
			fatal(err)
		}
	}
	output, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(output))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
