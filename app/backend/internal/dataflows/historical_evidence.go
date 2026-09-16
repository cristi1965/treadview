package dataflows

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// A 20-session return needs 21 closing observations (20 intervals).
const HistoricalEvidenceMinimumSessions = 21
const HistoricalADVWindow = 20

type HistoricalPriceObservation struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
}

type HistoricalEvidence struct {
	Symbol          string                       `json:"symbol"`
	Source          string                       `json:"source"`
	SourceURL       string                       `json:"source_url"`
	DataTime        string                       `json:"data_time"`
	TimeGranularity string                       `json:"time_granularity"`
	SampleCount     int                          `json:"sample_count"`
	MinimumSessions int                          `json:"minimum_sessions"`
	Status          string                       `json:"status"`
	Observations    []HistoricalPriceObservation `json:"observations"`
}

type HistoricalCalculation struct {
	Name    string             `json:"name"`
	Formula string             `json:"formula"`
	Inputs  map[string]float64 `json:"inputs"`
	Value   *float64           `json:"value,omitempty"`
	Unit    string             `json:"unit,omitempty"`
	Status  string             `json:"status"`
	Reason  string             `json:"reason,omitempty"`
}

func FetchNasdaqHistoricalEvidence(client *http.Client, symbol, tradeDate string) (HistoricalEvidence, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	asOf, err := time.Parse("2006-01-02", strings.TrimSpace(tradeDate))
	if err != nil {
		return HistoricalEvidence{}, fmt.Errorf("invalid historical evidence date: %w", err)
	}
	master, err := loadUSInstrumentMaster()
	if err != nil || !master.ContainsStock(symbol) {
		return HistoricalEvidence{}, fmt.Errorf("symbol absent from versioned US instrument master")
	}
	from := asOf.AddDate(0, 0, -90).Format("2006-01-02")
	endpoint := fmt.Sprintf("https://api.nasdaq.com/api/quote/%s/historical?assetclass=stocks&fromdate=%s&todate=%s&limit=120",
		url.PathEscape(symbol), url.QueryEscape(from), url.QueryEscape(tradeDate))
	body, err := httpGet(client, endpoint, map[string]string{
		"Accept": "application/json", "Origin": "https://www.nasdaq.com", "Referer": "https://www.nasdaq.com/",
	})
	if err != nil {
		return HistoricalEvidence{}, err
	}
	return parseNasdaqHistoricalEvidence(body, symbol, endpoint, asOf, HistoricalEvidenceMinimumSessions)
}

func parseNasdaqHistoricalEvidence(body []byte, symbol, endpoint string, asOf time.Time, minimum int) (HistoricalEvidence, error) {
	var payload struct {
		Data struct {
			Symbol      string `json:"symbol"`
			TradesTable struct {
				Rows []struct {
					Date   string `json:"date"`
					Open   string `json:"open"`
					High   string `json:"high"`
					Low    string `json:"low"`
					Close  string `json:"close"`
					Volume string `json:"volume"`
				} `json:"rows"`
			} `json:"tradesTable"`
		} `json:"data"`
		Status struct {
			Code int `json:"rCode"`
		} `json:"status"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return HistoricalEvidence{}, fmt.Errorf("parse Nasdaq historical evidence: %w", err)
	}
	if payload.Status.Code != 0 && payload.Status.Code != http.StatusOK {
		return HistoricalEvidence{}, fmt.Errorf("Nasdaq historical status %d", payload.Status.Code)
	}
	if payload.Data.Symbol != "" && !strings.EqualFold(payload.Data.Symbol, symbol) {
		return HistoricalEvidence{}, fmt.Errorf("Nasdaq historical symbol mismatch")
	}
	seen := map[string]bool{}
	observations := make([]HistoricalPriceObservation, 0, len(payload.Data.TradesTable.Rows))
	for _, row := range payload.Data.TradesTable.Rows {
		date, err := time.Parse("01/02/2006", strings.TrimSpace(row.Date))
		if err != nil || date.After(asOf) || date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}
		dateKey := date.Format("2006-01-02")
		if seen[dateKey] {
			continue
		}
		openValue := parseMoney(row.Open)
		highValue := parseMoney(row.High)
		lowValue := parseMoney(row.Low)
		closeValue := parseMoney(row.Close)
		volume, volumeErr := strconv.ParseInt(strings.ReplaceAll(strings.TrimSpace(row.Volume), ",", ""), 10, 64)
		if openValue <= 0 || highValue <= 0 || lowValue <= 0 || closeValue <= 0 || volumeErr != nil || volume <= 0 ||
			highValue < math.Max(openValue, closeValue) || lowValue > math.Min(openValue, closeValue) || highValue < lowValue {
			continue
		}
		seen[dateKey] = true
		observations = append(observations, HistoricalPriceObservation{
			Date: dateKey, Open: openValue, High: highValue, Low: lowValue,
			Close: closeValue, Volume: volume,
		})
	}
	sort.Slice(observations, func(i, j int) bool { return observations[i].Date < observations[j].Date })
	if len(observations) == 0 {
		return HistoricalEvidence{}, fmt.Errorf("Nasdaq historical returned no valid dated OHLCV observations")
	}
	status := "complete"
	if len(observations) < minimum {
		status = "insufficient_samples"
	}
	return HistoricalEvidence{
		Symbol: symbol, Source: "nasdaq-historical", SourceURL: endpoint,
		DataTime: observations[len(observations)-1].Date, TimeGranularity: "date",
		SampleCount: len(observations), MinimumSessions: minimum, Status: status, Observations: observations,
	}, nil
}

func CalculateHistoricalEvidence(evidence HistoricalEvidence) []HistoricalCalculation {
	rows := evidence.Observations
	calculations := make([]HistoricalCalculation, 0, 6)
	for _, horizon := range []int{1, 5, 20} {
		calculation := HistoricalCalculation{
			Name: fmt.Sprintf("return_%dd", horizon), Formula: "(end_close / start_close - 1) * 100",
			Inputs: map[string]float64{}, Unit: "percent", Status: "unknown",
			Reason: fmt.Sprintf("requires %d closing observations", horizon+1),
		}
		if len(rows) >= horizon+1 {
			start := rows[len(rows)-1-horizon]
			end := rows[len(rows)-1]
			if start.Close > 0 {
				value := roundEvidence((end.Close/start.Close-1)*100, 6)
				calculation.Inputs = map[string]float64{"start_close": start.Close, "end_close": end.Close, "sessions": float64(horizon)}
				calculation.Value, calculation.Status, calculation.Reason = &value, "computed", ""
			}
		}
		calculations = append(calculations, calculation)
	}

	volatility := HistoricalCalculation{
		Name: "annualized_volatility", Formula: "sample_stddev(daily_close_return) * sqrt(252) * 100",
		Inputs: map[string]float64{}, Unit: "percent", Status: "unknown", Reason: "requires at least 20 valid daily returns",
	}
	if len(rows) >= HistoricalEvidenceMinimumSessions {
		returns := make([]float64, 0, len(rows)-1)
		for i := 1; i < len(rows); i++ {
			returns = append(returns, rows[i].Close/rows[i-1].Close-1)
		}
		mean := 0.0
		for _, value := range returns {
			mean += value
		}
		mean /= float64(len(returns))
		variance := 0.0
		for _, value := range returns {
			variance += (value - mean) * (value - mean)
		}
		variance /= float64(len(returns) - 1)
		value := roundEvidence(math.Sqrt(variance)*math.Sqrt(252)*100, 6)
		volatility.Inputs = map[string]float64{"daily_return_count": float64(len(returns)), "annualization_sessions": 252}
		volatility.Value, volatility.Status, volatility.Reason = &value, "computed", ""
	}
	calculations = append(calculations, volatility)

	drawdown := HistoricalCalculation{
		Name: "maximum_drawdown", Formula: "min(close / prior_running_peak_close - 1) * 100",
		Inputs: map[string]float64{}, Unit: "percent", Status: "unknown", Reason: "requires at least two closing observations",
	}
	if len(rows) >= 2 {
		peak := rows[0].Close
		minimumDrawdown := 0.0
		for _, row := range rows[1:] {
			if row.Close > peak {
				peak = row.Close
			}
			if peak > 0 {
				minimumDrawdown = math.Min(minimumDrawdown, row.Close/peak-1)
			}
		}
		value := roundEvidence(minimumDrawdown*100, 6)
		drawdown.Inputs = map[string]float64{"close_observation_count": float64(len(rows))}
		drawdown.Value, drawdown.Status, drawdown.Reason = &value, "computed", ""
	}
	calculations = append(calculations, drawdown)

	adv := HistoricalCalculation{
		Name: "average_daily_volume_20d", Formula: "sum(volume over latest 20 sessions) / 20",
		Inputs: map[string]float64{}, Unit: "shares", Status: "unknown", Reason: "requires 20 valid volume observations",
	}
	if len(rows) >= HistoricalADVWindow {
		window := rows[len(rows)-HistoricalADVWindow:]
		total := int64(0)
		for _, row := range window {
			total += row.Volume
		}
		value := roundEvidence(float64(total)/HistoricalADVWindow, 6)
		adv.Inputs = map[string]float64{"volume_sum": float64(total), "sessions": HistoricalADVWindow}
		adv.Value, adv.Status, adv.Reason = &value, "computed", ""
	}
	calculations = append(calculations, adv)
	return calculations
}

func roundEvidence(value float64, digits int) float64 {
	scale := math.Pow10(digits)
	return math.Round(value*scale) / scale
}

func FormatHistoricalEvidence(evidence HistoricalEvidence) string {
	raw, _ := json.Marshal(evidence)
	return fmt.Sprintf("Provider: %s\nSource URL: %s\nData Time: %s\nTime Granularity: %s\nHistorical Evidence JSON: %s\n",
		evidence.Source, evidence.SourceURL, evidence.DataTime, evidence.TimeGranularity, raw)
}

func ParseHistoricalEvidence(output string) (HistoricalEvidence, error) {
	const marker = "Historical Evidence JSON: "
	index := strings.Index(output, marker)
	if index < 0 {
		return HistoricalEvidence{}, fmt.Errorf("structured historical evidence is missing")
	}
	raw := strings.TrimSpace(output[index+len(marker):])
	var evidence HistoricalEvidence
	if err := json.Unmarshal([]byte(raw), &evidence); err != nil {
		return HistoricalEvidence{}, err
	}
	return evidence, nil
}
