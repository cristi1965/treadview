package scoring

import (
	"math"
	"testing"
)

func TestExplainFiveCapturesReproducibleInputsAndDeterministicScaling(t *testing.T) {
	row := UsStockRow{Sym: "NVDA", Name: "NVIDIA", Price: 120, Pct: 3.2, McapB: 2800, Sector: "Technology", Industry: "Semiconductor", Vol: 90_000_000}
	detail := ExplainFive(row, true)

	if detail.MethodVersion != HeuristicMethodVersion || detail.Inputs.Price != 120 || detail.Inputs.DayPct != 3.2 {
		t.Fatalf("score inputs incomplete: %+v", detail)
	}
	if len(detail.RawScores) != 5 || len(detail.FinalScores) != 5 || detail.DeterministicScaling == "" {
		t.Fatalf("score derivation incomplete: %+v", detail)
	}
	for index, key := range PanelOrder {
		parts := detail.Components[key]
		if len(parts) == 0 {
			t.Fatalf("missing component contributions for %s", key)
		}
		var total float64
		for _, part := range parts {
			total += part.Value
		}
		if got := clampInt(int(math.Round(total)), 5, 95); got != detail.RawScores[index] {
			t.Fatalf("%s components produce %d, raw score is %d", key, got, detail.RawScores[index])
		}
	}
	if !detail.ReproducesPanel || !equalScores(detail.FinalScores, detail.PanelScores) {
		t.Fatalf("fresh heuristic detail must reproduce panel: %+v", detail)
	}
	panel := BuildPanel([]UsStockRow{row})
	if panel.MethodVersion != HeuristicMethodVersion || panel.Stocks["NVDA"].Detail == nil {
		t.Fatalf("panel did not retain method/detail: %+v", panel)
	}
}

func TestLegacyPanelNamingMigratesOnlyWhenScoresReproduce(t *testing.T) {
	raw := []int{55, 55, 55, 55, 55}
	final := ScaleHeuristicDeterministically(raw)
	panel := &PanelFile{Source: "local-heuristic-v2-calibrated", Stocks: map[string]PanelStock{
		"AAPL": {SC: append([]int(nil), final...), Detail: &ScoreDetail{MethodVersion: HeuristicMethodVersion, RawScores: raw, FinalScores: append([]int(nil), final...)}},
		"BAD":  {SC: []int{1, 2, 3, 4, 5}, Detail: &ScoreDetail{MethodVersion: HeuristicMethodVersion, RawScores: raw, FinalScores: append([]int(nil), final...)}},
	}}
	panel.EnsureDeterministicScalingDisclosure()
	if panel.Source != "local-heuristic-v2-deterministic-scaling" || panel.Stocks["AAPL"].Detail.DeterministicScaling != DeterministicScalingDisclosure {
		t.Fatalf("reproducible legacy detail was not migrated: %+v", panel)
	}
	if panel.Stocks["BAD"].Detail.DeterministicScaling != "" {
		t.Fatalf("non-reproducing detail received unsupported disclosure: %+v", panel.Stocks["BAD"])
	}
}

func equalScores(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
