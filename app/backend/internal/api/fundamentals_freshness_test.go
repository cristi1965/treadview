package api

import (
	"strings"
	"testing"
	"time"

	"trading-agents/internal/dataflows"
)

func TestFundamentalsFreshnessRequiresVerifiedFilingMetadata(t *testing.T) {
	metrics := &dataflows.StockMetrics{Source: "nasdaq-financials", FiscalPeriod: "quarterly:2026-06-27", AsOf: "2026-06-27"}
	meta := fundamentalsFreshness(metrics, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	if !meta.Stale || meta.DataTime != "unknown" || !strings.Contains(meta.StaleReason, "filing metadata") {
		t.Fatalf("missing filing metadata was treated as fresh: %+v", meta)
	}
	metrics.FilingDate = "2026-07-31"
	metrics.Accession = "0000320193-26-000079"
	metrics.SourceURL = "https://www.sec.gov/Archives/edgar/data/320193/000032019326000079/filing-index.html"
	meta = fundamentalsFreshness(metrics, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	if meta.DataTime != "2026-06-27T00:00:00Z" || strings.Contains(meta.StaleReason, "filing metadata") {
		t.Fatalf("verified filing metadata was not recognized: %+v", meta)
	}
}
