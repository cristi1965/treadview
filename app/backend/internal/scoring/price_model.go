package scoring

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"time"
)

const OHLCVMomentumModelName = "ohlcv-5d-momentum-next-session-v1"

type PriceObservation struct {
	Date  string  `json:"date"`
	Close float64 `json:"close"`
}

type PriceModelMetric struct {
	Name              string     `json:"name"`
	SampleCount       int        `json:"sample_count"`
	IC                float64    `json:"ic"`
	IC95              [2]float64 `json:"ic_95"`
	DirectionAccuracy float64    `json:"direction_accuracy"`
	Accuracy95        [2]float64 `json:"accuracy_95"`
}

type PriceModelReport struct {
	Name          string           `json:"name"`
	Scope         string           `json:"scope"`
	Status        string           `json:"status"`
	Result        string           `json:"result"`
	DatasetHash   string           `json:"dataset_hash,omitempty"`
	Feature       string           `json:"feature"`
	Label         string           `json:"label"`
	Benchmark     string           `json:"benchmark"`
	Cutoff        string           `json:"cutoff,omitempty"`
	TrainSamples  int              `json:"train_samples"`
	TestSamples   int              `json:"test_samples"`
	Baseline      float64          `json:"baseline_direction_accuracy"`
	Metric        PriceModelMetric `json:"metric"`
	Reasons       []string         `json:"reasons,omitempty"`
	FiveFactorUse bool             `json:"validates_five_factor_panel"`
}

type priceModelSample struct {
	AsOf      string  `json:"as_of"`
	LabelEnd  string  `json:"label_end"`
	Momentum5 float64 `json:"momentum_5d"`
	Forward   float64 `json:"forward_return_1d"`
}

// EvaluateOHLCVMomentum performs a fixed, leakage-free evaluation. The latest
// 30 chronological samples are always out-of-sample; no filing or panel factor
// is consumed, so this report cannot validate the five-factor panel.
func EvaluateOHLCVMomentum(observations []PriceObservation) PriceModelReport {
	report := PriceModelReport{
		Name: OHLCVMomentumModelName, Scope: "market-price-only submodel; not the five-factor fundamental panel",
		Status: "unavailable", Result: "not_evaluated", Feature: "close[t] / close[t-5] - 1",
		Label: "close[t+1] / close[t] - 1", Benchmark: "majority next-session direction in out-of-sample set",
		FiveFactorUse: false,
	}
	if len(observations) < 37 {
		report.Reasons = []string{"requires at least 37 ordered observations for a 5-session feature, next-session label, and 30 out-of-sample samples"}
		return report
	}
	samples := make([]priceModelSample, 0, len(observations)-6)
	for i := 5; i+1 < len(observations); i++ {
		prior, current, future := observations[i-5], observations[i], observations[i+1]
		priorAt, priorErr := time.Parse("2006-01-02", prior.Date)
		currentAt, currentErr := time.Parse("2006-01-02", current.Date)
		futureAt, futureErr := time.Parse("2006-01-02", future.Date)
		if priorErr != nil || currentErr != nil || futureErr != nil || !priorAt.Before(currentAt) || !currentAt.Before(futureAt) ||
			prior.Close <= 0 || current.Close <= 0 || future.Close <= 0 || !finitePrice(prior.Close) || !finitePrice(current.Close) || !finitePrice(future.Close) {
			report.Reasons = []string{"observations contain invalid, duplicate, non-chronological, or non-finite values"}
			return report
		}
		samples = append(samples, priceModelSample{AsOf: current.Date, LabelEnd: future.Date, Momentum5: current.Close/prior.Close - 1, Forward: future.Close/current.Close - 1})
	}
	raw, _ := json.Marshal(samples)
	digest := sha256.Sum256(raw)
	report.DatasetHash = "sha256:" + hex.EncodeToString(digest[:])
	split := len(samples) - 30
	report.TrainSamples, report.TestSamples = split, 30
	report.Cutoff = samples[split-1].AsOf
	test := samples[split:]
	features, labels := make([]float64, len(test)), make([]float64, len(test))
	positive, correct := 0, 0
	for i, sample := range test {
		features[i], labels[i] = sample.Momentum5, sample.Forward
		if sample.Forward >= 0 {
			positive++
		}
		if (sample.Momentum5 >= 0) == (sample.Forward >= 0) {
			correct++
		}
	}
	report.Baseline = math.Max(float64(positive)/30, 1-float64(positive)/30)
	ic := spearman(features, labels)
	report.Metric = PriceModelMetric{
		Name: "ohlcv_5d_momentum", SampleCount: 30, IC: ic, IC95: fisher95(ic, 30),
		DirectionAccuracy: float64(correct) / 30, Accuracy95: wilson95(correct, 30),
	}
	report.Status = "evaluated"
	report.Result = "failed"
	if report.Metric.IC95[0] > 0 && report.Metric.Accuracy95[0] > report.Baseline {
		report.Result = "passed"
	} else {
		report.Reasons = []string{"out-of-sample confidence intervals do not establish positive IC and direction accuracy above the majority baseline"}
	}
	return report
}

func finitePrice(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
