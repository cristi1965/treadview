package scoring

import (
	"fmt"
	"testing"
)

func TestHistoricalValidationUsesStrictOutOfSampleSplit(t *testing.T) {
	samples := make([]HistoricalScoreSample, 0, 70)
	for i := 0; i < 35; i++ {
		asOf := fmt.Sprintf("2025-01-%02d", i%28+1)
		samples = append(samples, HistoricalScoreSample{Symbol: fmt.Sprintf("T%d", i), AsOf: asOf, FeatureDataTime: asOf, LabelEnd: "2025-02-28", Scores: [5]float64{60, 60, 60, 60, 60}, ForwardReturn: 0.1})
	}
	for i := 0; i < 35; i++ {
		score := float64(20 + i*2)
		excess := float64(i-17) / 100
		asOf := fmt.Sprintf("2026-02-%02d", i%28+1)
		samples = append(samples, HistoricalScoreSample{Symbol: fmt.Sprintf("X%d", i), AsOf: asOf, FeatureDataTime: asOf, LabelEnd: "2026-03-31", Scores: [5]float64{score, score, score, score, score}, ForwardReturn: excess})
	}
	report := ValidateHistoricalScores(samples, "2025-12-31")
	if report.Status != "unvalidated" || report.TrainSamples != 35 || report.TestSamples != 35 || len(report.Metrics) != 5 || len(report.Reasons) == 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Metrics[0].IC < 0.99 || report.Metrics[0].SampleCount != 35 {
		t.Fatalf("out-of-sample metric not computed: %+v", report.Metrics[0])
	}
}

func TestHistoricalValidationRejectsLeakageAndInsufficientSamples(t *testing.T) {
	leaking := []HistoricalScoreSample{{AsOf: "2026-01-02", FeatureDataTime: "2026-01-03", LabelEnd: "2026-01-04"}}
	if report := ValidateHistoricalScores(leaking, "2025-12-31"); report.Status != "unvalidated" || len(report.Reasons) == 0 {
		t.Fatalf("leaking sample accepted: %+v", report)
	}
	validButSmall := []HistoricalScoreSample{{AsOf: "2026-01-02", FeatureDataTime: "2026-01-02", LabelEnd: "2026-01-03"}}
	if report := ValidateHistoricalScores(validButSmall, "2025-12-31"); report.Status != "unvalidated" || report.TestSamples != 1 {
		t.Fatalf("small sample falsely validated: %+v", report)
	}
}

func TestLegacyPanelDefaultsToUnvalidated(t *testing.T) {
	panel := &PanelFile{Stocks: map[string]PanelStock{"AAPL": {SC: []int{1, 2, 3, 4, 5}}}}
	panel.EnsureValidationStatus()
	if panel.Validation.Status != "unvalidated" || len(panel.Validation.Reasons) == 0 {
		t.Fatalf("legacy panel validation status=%+v", panel.Validation)
	}
}
