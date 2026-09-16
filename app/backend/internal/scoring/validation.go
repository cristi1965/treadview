package scoring

import (
	"fmt"
	"math"
	"sort"
	"time"
)

type HistoricalScoreSample struct {
	Symbol          string     `json:"symbol"`
	AsOf            string     `json:"as_of"`
	FeatureDataTime string     `json:"feature_data_time"`
	LabelEnd        string     `json:"label_end"`
	Scores          [5]float64 `json:"scores"`
	ForwardReturn   float64    `json:"forward_return"`
	BenchmarkReturn float64    `json:"benchmark_return"`
}

type ValidationMetric struct {
	Factor            string     `json:"factor"`
	SampleCount       int        `json:"sample_count"`
	IC                float64    `json:"ic"`
	IC95              [2]float64 `json:"ic_95"`
	DirectionAccuracy float64    `json:"direction_accuracy"`
	Accuracy95        [2]float64 `json:"accuracy_95"`
}

type ValidationReport struct {
	Status            string             `json:"status"`
	Method            string             `json:"method"`
	Cutoff            string             `json:"cutoff,omitempty"`
	TrainSamples      int                `json:"train_samples"`
	TestSamples       int                `json:"test_samples"`
	Benchmark         string             `json:"benchmark"`
	BenchmarkAccuracy float64            `json:"benchmark_accuracy"`
	Metrics           []ValidationMetric `json:"metrics,omitempty"`
	Reasons           []string           `json:"reasons,omitempty"`
}

// EnsureValidationStatus prevents legacy panel files from implying validation by omission.
func (p *PanelFile) EnsureValidationStatus() {
	if p == nil || p.Validation.Status != "" {
		return
	}
	p.Validation = ValidationReport{
		Status: "unvalidated", Method: "strict-time-split-v1",
		Benchmark: "benchmark-adjusted forward return; majority-direction baseline",
		Reasons:   []string{"legacy panel has no historical validation report"},
	}
}

// ValidateHistoricalScores evaluates only observations after a fixed cutoff and rejects temporal leakage.
func ValidateHistoricalScores(samples []HistoricalScoreSample, cutoff string) ValidationReport {
	report := ValidationReport{
		Status: "unvalidated", Method: "strict-time-split-v1", Cutoff: cutoff,
		Benchmark: "benchmark-adjusted forward return; majority-direction baseline",
	}
	cutoffAt, err := time.Parse("2006-01-02", cutoff)
	if err != nil {
		report.Reasons = []string{"invalid cutoff date"}
		return report
	}
	test := make([]HistoricalScoreSample, 0)
	for index, sample := range samples {
		asOf, asOfErr := time.Parse("2006-01-02", sample.AsOf)
		featureAt, featureErr := time.Parse("2006-01-02", sample.FeatureDataTime)
		labelEnd, labelErr := time.Parse("2006-01-02", sample.LabelEnd)
		if asOfErr != nil || featureErr != nil || labelErr != nil || featureAt.After(asOf) || !labelEnd.After(asOf) {
			report.Reasons = append(report.Reasons, fmt.Sprintf("sample %d has invalid dates or future feature leakage", index))
			continue
		}
		if !asOf.After(cutoffAt) {
			report.TrainSamples++
			continue
		}
		test = append(test, sample)
	}
	report.TestSamples = len(test)
	if len(report.Reasons) > 0 {
		return report
	}
	if len(test) < 30 {
		report.Reasons = []string{"fewer than 30 out-of-sample observations"}
		return report
	}
	positive := 0
	for _, sample := range test {
		if sample.ForwardReturn-sample.BenchmarkReturn >= 0 {
			positive++
		}
	}
	baseRate := float64(positive) / float64(len(test))
	report.BenchmarkAccuracy = math.Max(baseRate, 1-baseRate)
	for factorIndex, factor := range PanelOrder {
		scores := make([]float64, len(test))
		excess := make([]float64, len(test))
		correct := 0
		for i, sample := range test {
			scores[i] = sample.Scores[factorIndex]
			excess[i] = sample.ForwardReturn - sample.BenchmarkReturn
			if (scores[i] >= 50) == (excess[i] >= 0) {
				correct++
			}
		}
		ic := spearman(scores, excess)
		report.Metrics = append(report.Metrics, ValidationMetric{
			Factor: factor, SampleCount: len(test), IC: ic, IC95: fisher95(ic, len(test)),
			DirectionAccuracy: float64(correct) / float64(len(test)), Accuracy95: wilson95(correct, len(test)),
		})
	}
	report.Reasons = []string{"historical metrics were evaluated, but they do not validate the five-factor fundamental methodology or its deterministic scaling"}
	return report
}

func spearman(x, y []float64) float64 {
	return pearson(ranks(x), ranks(y))
}

func ranks(values []float64) []float64 {
	type item struct {
		index int
		value float64
	}
	items := make([]item, len(values))
	for i, value := range values {
		items[i] = item{i, value}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].value < items[j].value })
	out := make([]float64, len(values))
	for start := 0; start < len(items); {
		end := start + 1
		for end < len(items) && items[end].value == items[start].value {
			end++
		}
		rank := (float64(start+1) + float64(end)) / 2
		for i := start; i < end; i++ {
			out[items[i].index] = rank
		}
		start = end
	}
	return out
}

func pearson(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0
	}
	var sx, sy float64
	for i := range x {
		sx += x[i]
		sy += y[i]
	}
	mx, my := sx/float64(len(x)), sy/float64(len(y))
	var covariance, vx, vy float64
	for i := range x {
		dx, dy := x[i]-mx, y[i]-my
		covariance += dx * dy
		vx += dx * dx
		vy += dy * dy
	}
	if vx == 0 || vy == 0 {
		return 0
	}
	return covariance / math.Sqrt(vx*vy)
}

func fisher95(value float64, n int) [2]float64 {
	if n <= 3 || math.Abs(value) >= 1 {
		return [2]float64{value, value}
	}
	z := math.Atanh(value)
	delta := 1.96 / math.Sqrt(float64(n-3))
	return [2]float64{math.Tanh(z - delta), math.Tanh(z + delta)}
}

func wilson95(success, total int) [2]float64 {
	if total == 0 {
		return [2]float64{}
	}
	z := 1.96
	n := float64(total)
	p := float64(success) / n
	denominator := 1 + z*z/n
	center := (p + z*z/(2*n)) / denominator
	margin := z * math.Sqrt((p*(1-p)+z*z/(4*n))/n) / denominator
	return [2]float64{center - margin, center + margin}
}
