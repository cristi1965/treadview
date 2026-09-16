package dataflows

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestParseQDIIPremiumRowsRequiresDirectCriticalInputs(t *testing.T) {
	rows := []map[string]any{
		{
			"FCODE": "513100", "SHORTNAME": "complete", "NAV": "1.25", "PDATE": "2026-09-06",
			"NEWPRICE": "1.30", "ZJL": "-4", "CHANGERATIO": "0.5", "HQDATE": "2026-09-07 10:00:00",
		},
		{
			"FCODE": "513300", "SHORTNAME": "missing premium", "NAV": "1.10", "PDATE": "2026-09-05",
			"NEWPRICE": "1.20", "ZJL": "--", "HQDATE": "2026-09-07 10:00:00",
		},
		{
			"FCODE": "513500", "SHORTNAME": "missing price", "NAV": "1.10", "PDATE": "2026-09-04",
			"NEWPRICE": nil, "ZJL": "-3", "HQDATE": "2026-09-07 10:00:00",
		},
	}

	items := parseQDIIPremiumRows(rows)
	if len(items) != 3 {
		t.Fatalf("items=%d", len(items))
	}
	if items[0].Status != "complete" || items[0].PremiumPct == nil || *items[0].PremiumPct != 4 {
		t.Fatalf("complete row=%+v", items[0])
	}
	if items[1].Status != "unknown" || items[1].PremiumPct != nil || items[1].Level != "unknown" {
		t.Fatalf("missing premium must stay unknown: %+v", items[1])
	}
	if items[2].Status != "unknown" || items[2].Price != nil || items[2].Level != "unknown" {
		t.Fatalf("missing price must not be reconstructed: %+v", items[2])
	}
	if _, known := OldestQDIIPremiumDataTime(items); known {
		t.Fatal("mixed complete and unknown items must have unknown aggregate time")
	}
}

func TestOldestQDIIPremiumDataTimeUsesOldestNAVDate(t *testing.T) {
	items := parseQDIIPremiumRows([]map[string]any{
		{"FCODE": "513100", "NAV": 1.0, "PDATE": "2026-09-06", "NEWPRICE": 1.1, "ZJL": -10.0, "HQDATE": "2026-09-07 10:00:00"},
		{"FCODE": "513300", "NAV": 2.0, "PDATE": "2026-09-04", "NEWPRICE": 2.1, "ZJL": -5.0, "HQDATE": "2026-09-07 10:00:00"},
	})
	got, known := OldestQDIIPremiumDataTime(items)
	if !known || got != "2026-09-04T00:00:00Z" {
		t.Fatalf("dataTime=%q known=%v", got, known)
	}
}

func TestSupplementQDIIPremiumUsesObservedExchangeQuote(t *testing.T) {
	items := parseQDIIPremiumRows([]map[string]any{{
		"FCODE": "161125", "SHORTNAME": "S&P 500 LOF", "NAV": 3.1685, "PDATE": "2026-09-07",
	}})
	observedAt := time.Date(2026, 9, 9, 3, 23, 36, 0, time.UTC)
	items = supplementQDIIPremiums(items, []CNQuote{{Symbol: "161125", Price: 3.237, Pct: -0.06, ObservedAt: observedAt}})
	if len(items) != 1 || items[0].Status != "complete" || items[0].PriceSource != "eastmoney:push2" || items[0].PriceObservedAt != observedAt.Format(time.RFC3339) {
		t.Fatalf("supplemented item=%+v", items)
	}
	want := (3.237/3.1685 - 1) * 100
	if items[0].PremiumPct == nil || math.Abs(*items[0].PremiumPct-want) > 1e-9 {
		t.Fatalf("premium=%v want=%f", items[0].PremiumPct, want)
	}
}

func TestEnsureQDIIPremiumCoverageKeepsOmittedCodeUnknown(t *testing.T) {
	items := ensureQDIIPremiumCoverage([]QDIIPremium{{Code: "513100", Status: "complete"}}, []string{"513100", "161125"})
	if len(items) != 2 || items[1].Code != "161125" || items[1].Status != "unknown" || !strings.Contains(items[1].StatusReason, "omitted") {
		t.Fatalf("coverage=%+v", items)
	}
}

func TestOldestQDIIPremiumDataTimeIncludesPriceObservation(t *testing.T) {
	nav, price, premium := 1.0, 1.1, 10.0
	items := []QDIIPremium{{Code: "513100", NAV: &nav, Price: &price, PremiumPct: &premium, NavDate: "2026-09-06", PriceObservedAt: "2026-09-05T15:00:00Z", Status: "complete"}}
	got, known := OldestQDIIPremiumDataTime(items)
	if !known || got != "2026-09-05T15:00:00Z" {
		t.Fatalf("dataTime=%q known=%v", got, known)
	}
}
