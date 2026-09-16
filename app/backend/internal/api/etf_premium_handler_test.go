package api

import (
	"testing"
	"time"

	"trading-agents/internal/dataflows"
)

func TestQDIIPremiumFreshnessUsesOldestCriticalInput(t *testing.T) {
	nav, price, premium := 1.0, 1.1, 10.0
	items := []dataflows.QDIIPremium{
		{Code: "513100", NAV: &nav, Price: &price, PremiumPct: &premium, NavDate: "2026-09-06", PriceObservedAt: "2026-09-07T02:00:00Z", Status: "complete"},
		{Code: "513300", NAV: &nav, Price: &price, PremiumPct: &premium, NavDate: "2026-09-04", PriceObservedAt: "2026-09-07T02:00:00Z", Status: "complete"},
	}
	meta, status := qdiiPremiumFreshness(items, time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC))
	if status != "live" || meta.Stale || meta.DataTime != "2026-09-04T00:00:00Z" {
		t.Fatalf("status=%q meta=%+v", status, meta)
	}

	meta, status = qdiiPremiumFreshness(items, time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC))
	if status != "stale" || !meta.Stale || meta.DataTime != "2026-09-04T00:00:00Z" {
		t.Fatalf("status=%q meta=%+v", status, meta)
	}
}

func TestQDIIPremiumFreshnessMarksIncompleteInputUnknown(t *testing.T) {
	items := []dataflows.QDIIPremium{{Code: "513100", NavDate: "2026-09-06", Status: "unknown"}}
	meta, status := qdiiPremiumFreshness(items, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	if status != "unknown" || !meta.Stale || meta.DataTime != "unknown" || len(meta.PartialErrors) == 0 {
		t.Fatalf("status=%q meta=%+v", status, meta)
	}
}
