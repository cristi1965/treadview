package scoring

import (
	"testing"
	"time"
)

func TestOHLCVMomentumEvaluationIsTimeSplitHashedAndSeparateFromPanel(t *testing.T) {
	rows := make([]PriceObservation, 0, 80)
	price := 100.0
	date := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 80; i++ {
		price *= 1.01
		rows = append(rows, PriceObservation{Date: date.AddDate(0, 0, i).Format("2006-01-02"), Close: price})
	}
	report := EvaluateOHLCVMomentum(rows)
	if report.Status != "evaluated" || report.TestSamples != 30 || report.TrainSamples != 44 || report.DatasetHash == "" || report.FiveFactorUse {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Metric.SampleCount != 30 || report.Metric.DirectionAccuracy != 1 {
		t.Fatalf("metric is not reproducible: %+v", report.Metric)
	}
}

func TestOHLCVMomentumRejectsInsufficientOrDuplicateInput(t *testing.T) {
	if report := EvaluateOHLCVMomentum(make([]PriceObservation, 20)); report.Status != "unavailable" {
		t.Fatalf("insufficient input accepted: %+v", report)
	}
	rows := make([]PriceObservation, 40)
	date := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	for i := range rows {
		rows[i] = PriceObservation{Date: date.AddDate(0, 0, i).Format("2006-01-02"), Close: 100 + float64(i)}
	}
	rows[10].Date = rows[9].Date
	if report := EvaluateOHLCVMomentum(rows); report.Status != "unavailable" {
		t.Fatalf("duplicate PIT observation accepted: %+v", report)
	}
}
