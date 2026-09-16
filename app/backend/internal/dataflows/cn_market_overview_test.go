package dataflows

import (
	"testing"
	"time"
)

func TestFinalizeMarketOverviewUsesOldestProviderObservation(t *testing.T) {
	older := time.Date(2026, 9, 15, 1, 2, 3, 0, time.UTC)
	newer := older.Add(time.Minute)
	overview, ok := finalizeMarketOverview(
		[]MarketIndexItem{{Symbol: "000001", Price: 1, Available: true, Source: "eastmoney", DataTime: newer.Format(time.RFC3339)}},
		[]MarketIndexItem{{Symbol: "SPX", Price: 2, Available: true, Source: "yahoo-chart", DataTime: older.Format(time.RFC3339)}},
		nil,
	)
	if !ok || overview.UpdatedAt != older.Format(time.RFC3339) {
		t.Fatalf("overview=%+v ok=%v", overview, ok)
	}
	if len(overview.USOvernightMapping) != 0 || len(overview.ForwardLayoutPlan) != 0 {
		t.Fatalf("unverified advice leaked into overview: %+v", overview)
	}
}

func TestFinalizeMarketOverviewFailsClosedWithoutProviderObservation(t *testing.T) {
	_, ok := finalizeMarketOverview(
		[]MarketIndexItem{{Symbol: "000001", Price: 1, Available: true, Source: "eastmoney"}},
		nil,
		nil,
	)
	if ok {
		t.Fatal("available value without provider observation must fail closed")
	}
}
