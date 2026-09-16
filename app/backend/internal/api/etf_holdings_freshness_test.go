package api

import "testing"

func TestETFHoldingsFreshnessMarksOldSeedHistorical(t *testing.T) {
	meta, mode := etfHoldingsFreshness(etfHoldingsFile{Updated: "2000-01-01"})
	if !meta.Stale || meta.StaleReason == "" || mode != "historical" {
		t.Fatalf("old holdings must be historical and stale: meta=%+v mode=%q", meta, mode)
	}
}
