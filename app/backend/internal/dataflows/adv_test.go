package dataflows

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseNasdaqADVRequiresTwentyDistinctVolumeObservations(t *testing.T) {
	rows := make([]string, 0, 20)
	date := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC)
	for day := 1; day <= 20; date = date.AddDate(0, 0, 1) {
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}
		rows = append(rows, fmt.Sprintf(`{"date":"%s","volume":"%d,000"}`, date.Format("01/02/2006"), day))
		day++
	}
	raw := []byte(`{"data":{"tradesTable":{"rows":[` + strings.Join(rows, ",") + `]}},"status":{"rCode":200}}`)
	result, err := parseNasdaqADV(raw, "AAPL", "fixture://historical", time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), 20)
	if err != nil {
		t.Fatal(err)
	}
	if result.Window != 20 || result.Start != "2026-08-03" || result.End != "2026-08-28" || result.AverageDailyVolume != 10_500 {
		t.Fatalf("ADV metadata=%+v", result)
	}
	if result.Source != "nasdaq-historical" || result.RefreshedAt != "2026-09-07T00:00:00Z" {
		t.Fatalf("ADV provenance=%+v", result)
	}
}

func TestParseNasdaqADVRejectsWeekendRows(t *testing.T) {
	rows := make([]string, 0, 20)
	for day := 1; day <= 20; day++ {
		rows = append(rows, fmt.Sprintf(`{"date":"08/%02d/2026","volume":"%d,000"}`, day, day))
	}
	raw := []byte(`{"data":{"tradesTable":{"rows":[` + strings.Join(rows, ",") + `]}},"status":{"rCode":200}}`)
	if _, err := parseNasdaqADV(raw, "AAPL", "fixture", time.Now(), 20); err == nil {
		t.Fatal("weekend rows must not count toward a 20-trading-day ADV window")
	}
}

func TestParseNasdaqADVRejectsSingleDayAndInvalidVolume(t *testing.T) {
	raw := []byte(`{"data":{"tradesTable":{"rows":[{"date":"09/03/2026","volume":"37,225,840"},{"date":"09/02/2026","volume":"--"}]}},"status":{"rCode":200}}`)
	if _, err := parseNasdaqADV(raw, "AAPL", "fixture", time.Now(), 20); err == nil {
		t.Fatal("single-day volume must not be accepted as ADV")
	}
}
