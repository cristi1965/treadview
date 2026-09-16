package dataflows

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestHistoricalEvidenceCalculationsAreReproducible(t *testing.T) {
	rows := make([]string, 0, 21)
	date := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	for index := 0; index < 21; date = date.AddDate(0, 0, 1) {
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}
		price := 100 + index
		rows = append(rows, fmt.Sprintf(`{"date":"%s","open":"$%d","high":"$%d","low":"$%d","close":"$%d","volume":"%d,000"}`,
			date.Format("01/02/2006"), price-1, price+1, price-2, price, index+1))
		index++
	}
	raw := []byte(`{"data":{"symbol":"AAPL","tradesTable":{"rows":[` + strings.Join(rows, ",") + `]}},"status":{"rCode":200}}`)
	evidence, err := parseNasdaqHistoricalEvidence(raw, "AAPL", "https://example.test/history", date, 20)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Status != "complete" || evidence.SampleCount != 21 || evidence.TimeGranularity != "date" {
		t.Fatalf("historical evidence metadata=%+v", evidence)
	}
	calculations := CalculateHistoricalEvidence(evidence)
	if len(calculations) != 6 || calculations[2].Name != "return_20d" || calculations[2].Value == nil || calculations[3].Name != "annualized_volatility" || calculations[3].Value == nil || calculations[3].Inputs["daily_return_count"] != 20 || calculations[5].Value == nil {
		t.Fatalf("historical calculations=%+v", calculations)
	}
	if calculations[5].Inputs["sessions"] != 20 {
		t.Fatalf("20-day ADV used the wrong window: %+v", calculations[5])
	}
	parsed, err := ParseHistoricalEvidence(FormatHistoricalEvidence(evidence))
	if err != nil || parsed.SampleCount != evidence.SampleCount || parsed.Observations[0].Close != 100 {
		t.Fatalf("historical structured round trip=%+v err=%v", parsed, err)
	}
}

func TestHistoricalEvidenceMarksMissingWindowsUnknownAndRejectsFutureRows(t *testing.T) {
	raw := []byte(`{"data":{"symbol":"AAPL","tradesTable":{"rows":[` +
		`{"date":"09/08/2026","open":"$119","high":"$121","low":"$118","close":"$120","volume":"2,000"},` +
		`{"date":"09/04/2026","open":"$99","high":"$101","low":"$98","close":"$100","volume":"1,000"}` +
		`]}},"status":{"rCode":200}}`)
	evidence, err := parseNasdaqHistoricalEvidence(raw, "AAPL", "fixture", time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), 20)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.SampleCount != 1 || evidence.DataTime != "2026-09-04" || evidence.Status != "insufficient_samples" {
		t.Fatalf("future row was retained: %+v", evidence)
	}
	for _, calculation := range CalculateHistoricalEvidence(evidence) {
		if calculation.Status != "unknown" || calculation.Value != nil || calculation.Reason == "" {
			t.Fatalf("insufficient calculation did not remain unknown: %+v", calculation)
		}
	}
}

func TestHistoricalEvidenceRejectsInvalidOHLC(t *testing.T) {
	raw := []byte(`{"data":{"symbol":"AAPL","tradesTable":{"rows":[` +
		`{"date":"09/04/2026","open":"$100","high":"$99","low":"$98","close":"$101","volume":"1,000"},` +
		`{"date":"09/03/2026","open":"$100","high":"$102","low":"$99","close":"$101","volume":"1,000"}` +
		`]}} ,"status":{"rCode":200}}`)
	evidence, err := parseNasdaqHistoricalEvidence(raw, "AAPL", "fixture", time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), 1)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.SampleCount != 1 || evidence.Observations[0].Date != "2026-09-03" {
		t.Fatalf("invalid OHLC row survived: %+v", evidence)
	}
}
