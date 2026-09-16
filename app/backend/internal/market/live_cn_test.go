package market

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"trading-agents/internal/dataflows"
)

func TestRefreshCNQuotesSubsetDoesNotPublishMarketWideFreshness(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	oldLiveAt := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)
	oldSnapshotAt := oldLiveAt.Add(-time.Hour)
	observedAt := oldLiveAt.Add(24 * time.Hour)
	provider := &Provider{
		cnSnap: map[string]snapshotQuote{
			"600519": {Price: 1400, McapYi: 18000, Source: "local-cn", DataTime: "2026-09-10"},
			"300750": {Price: 300, McapYi: 7000, Source: "local-cn", DataTime: "2026-09-10"},
		},
		cnLiveAt: oldLiveAt, cnSnapshotAt: oldSnapshotAt,
	}
	provider.cnFetch = func(symbols []string) ([]dataflows.CNQuote, error) {
		if len(symbols) != 1 || symbols[0] != "600519" {
			t.Fatalf("subset=%v", symbols)
		}
		return []dataflows.CNQuote{{Symbol: "600519", Price: 1500, ObservedAt: observedAt}}, nil
	}

	updated, err := provider.RefreshCNQuotes(1)
	if err != nil || updated != 1 {
		t.Fatalf("updated=%d err=%v", updated, err)
	}
	if !provider.cnLiveAt.Equal(oldLiveAt) || !provider.cnSnapshotAt.Equal(oldSnapshotAt) {
		t.Fatalf("subset advanced global timestamps: live=%s snapshot=%s", provider.cnLiveAt, provider.cnSnapshotAt)
	}
	if provider.cnSnap["600519"].Price != 1500 {
		t.Fatalf("subset did not update target quote: %+v", provider.cnSnap["600519"])
	}
	if _, err := os.Stat(filepath.Join(root, "data", "a-market.json")); !os.IsNotExist(err) {
		t.Fatalf("subset unexpectedly published a-market.json: %v", err)
	}
}

func TestUpsertCNQuotesPreservesPerSymbolProvenanceWithoutPublishingMarketWideFreshness(t *testing.T) {
	oldLiveAt := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)
	oldSnapshotAt := oldLiveAt.Add(-time.Hour)
	observedAt := oldLiveAt.Add(24 * time.Hour)
	provider := &Provider{
		cnSnap: map[string]snapshotQuote{
			"600519": {Price: 1400, Vol: 10, McapYi: 18000, Source: "local-cn", DataTime: "2026-09-10"},
			"300750": {Price: 300, Vol: 20, McapYi: 7000, Source: "local-cn", DataTime: "2026-09-10"},
		},
		cnLiveAt: oldLiveAt, cnSnapshotAt: oldSnapshotAt,
	}

	provider.UpsertCNQuotes(map[string]Quote{
		"600519": {
			Price: 1500, Pct: 1.2, Source: "eastmoney-cn", DataTime: observedAt.Format(time.RFC3339),
			TimeGranularity: "second", SourceSession: "regular", ProviderURL: "https://push2.eastmoney.com",
		},
	})

	if !provider.cnLiveAt.Equal(oldLiveAt) || !provider.cnSnapshotAt.Equal(oldSnapshotAt) {
		t.Fatalf("upsert advanced global timestamps: live=%s snapshot=%s", provider.cnLiveAt, provider.cnSnapshotAt)
	}
	quote := provider.cnSnap["600519"]
	if quote.Source != "eastmoney-cn" || quote.DataTime != observedAt.Format(time.RFC3339) || quote.TimeGranularity != "second" || quote.Session != "regular" || quote.ProviderURL == "" {
		t.Fatalf("upsert discarded per-symbol provenance: %+v", quote)
	}
	if quote.Vol != 10 || quote.McapYi != 18000 {
		t.Fatalf("upsert discarded snapshot fundamentals: %+v", quote)
	}

	provider.UpsertCNQuotes(map[string]Quote{
		"300750": {Price: 999, Source: "local-cn", DataTime: observedAt.Format(time.RFC3339)},
	})
	if quote := provider.cnSnap["300750"]; quote.Price != 300 || quote.Source != "local-cn" || quote.DataTime != "2026-09-10" {
		t.Fatalf("unverified local quote overwrote the snapshot: %+v", quote)
	}
}
