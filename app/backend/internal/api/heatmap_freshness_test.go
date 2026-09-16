package api

import (
	"strings"
	"testing"
	"time"

	"trading-agents/internal/market"
)

func TestHeatmapFreshnessUsesOldestInputAndMarksPartialOverlayStale(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	result := quoteFetchResult{
		quotes: map[string]market.Quote{
			"AAPL": {Price: 230, Source: "yahoo", DataTime: now.Add(-time.Minute).Format(time.RFC3339)},
		},
		source:   "yahoo",
		dataTime: now.Add(-time.Minute),
	}
	meta := heatmapFreshness("us-stocks@2026-09-15 06:00 ET", 2, result, now)
	if !meta.Stale || !strings.Contains(meta.StaleReason, "incomplete") {
		t.Fatalf("partial overlay must be stale: %+v", meta)
	}
	if meta.DataTime != "2026-09-15T10:00:00Z" {
		t.Fatalf("heatmap data time must be oldest input observation, got=%q", meta.DataTime)
	}
}
