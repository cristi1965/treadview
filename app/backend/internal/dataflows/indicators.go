package dataflows

import "math"

// calcSMA computes a Simple Moving Average.
func calcSMA(data []float64, period int) float64 {
	if len(data) < period {
		return 0
	}
	sum := 0.0
	for i := len(data) - period; i < len(data); i++ {
		sum += data[i]
	}
	return sum / float64(period)
}

// calcEMA computes an Exponential Moving Average (last value).
func calcEMA(data []float64, period int) float64 {
	if len(data) < period {
		return 0
	}
	multiplier := 2.0 / float64(period+1)
	ema := calcSMA(data[:period], period)
	for i := period; i < len(data); i++ {
		ema = (data[i]-ema)*multiplier + ema
	}
	return ema
}

// calcRSI computes the Relative Strength Index.
func calcRSI(data []float64, period int) float64 {
	if len(data) < period+1 {
		return 50 // neutral default
	}

	gains := 0.0
	losses := 0.0
	for i := 1; i <= period; i++ {
		change := data[i] - data[i-1]
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	for i := period + 1; i < len(data); i++ {
		change := data[i] - data[i-1]
		if change > 0 {
			avgGain = (avgGain*float64(period-1) + change) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + (-change)) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

// calcMACD computes MACD line, signal line, and histogram.
func calcMACD(data []float64) (float64, float64, float64) {
	if len(data) < 26 {
		return 0, 0, 0
	}
	ema12 := calcEMA(data, 12)
	ema26 := calcEMA(data, 26)
	macdLine := ema12 - ema26

	// Build MACD series for signal line
	macdSeries := make([]float64, 0, len(data)-25)
	for i := 26; i <= len(data); i++ {
		e12 := calcEMA(data[:i], 12)
		e26 := calcEMA(data[:i], 26)
		macdSeries = append(macdSeries, e12-e26)
	}

	signal := 0.0
	if len(macdSeries) >= 9 {
		signal = calcEMA(macdSeries, 9)
	}
	histogram := macdLine - signal

	return macdLine, signal, histogram
}

// calcBollinger computes Bollinger Bands (middle, upper, lower).
func calcBollinger(data []float64, period int) (float64, float64, float64) {
	if len(data) < period {
		return 0, 0, 0
	}

	mid := calcSMA(data, period)

	// Standard deviation
	sum := 0.0
	for i := len(data) - period; i < len(data); i++ {
		diff := data[i] - mid
		sum += diff * diff
	}
	stdDev := math.Sqrt(sum / float64(period))

	return mid, mid + 2*stdDev, mid - 2*stdDev
}

// calcATR computes the Average True Range.
func calcATR(bars []HistoricalBar, period int) float64 {
	if len(bars) < period+1 {
		return 0
	}

	var trSum float64
	for i := len(bars) - period; i < len(bars); i++ {
		high := bars[i].High
		low := bars[i].Low
		prevClose := bars[i-1].Close

		tr := math.Max(high-low, math.Max(
			math.Abs(high-prevClose),
			math.Abs(low-prevClose),
		))
		trSum += tr
	}

	return trSum / float64(period)
}

// calcVWMA computes the Volume-Weighted Moving Average.
func calcVWMA(closes []float64, volumes []int64, period int) float64 {
	if len(closes) < period || len(volumes) < period {
		return 0
	}

	var sumPV, sumV float64
	for i := len(closes) - period; i < len(closes); i++ {
		sumPV += closes[i] * float64(volumes[i])
		sumV += float64(volumes[i])
	}
	if sumV == 0 {
		return 0
	}
	return sumPV / sumV
}
