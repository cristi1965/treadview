package market

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"trading-agents/internal/dataflows"
)

func TestParseUsStocksQuotes(t *testing.T) {
	raw := []byte(`{
	  "generated_at": "2026-07-31 03:31 ET",
	  "count": 2,
	  "stocks": [
	    {"sym":"RKLB","name":"Rocket Lab","price":64.68,"pct":10.38,"mcapB":38.69,"vol":18162886},
	    {"sym":"bad","name":"x","price":0,"pct":1,"mcapB":1,"vol":1},
	    {"sym":"nvda","name":"NVIDIA","price":195.04,"pct":2.65,"mcapB":4719.97,"vol":129009718}
	  ]
	}`)

	quotes, generatedAt, err := parseUsStocksQuotes(raw)
	if err != nil {
		t.Fatalf("parseUsStocksQuotes: %v", err)
	}
	if generatedAt != "2026-07-31 03:31 ET" {
		t.Fatalf("generatedAt=%q", generatedAt)
	}
	if len(quotes) != 2 {
		t.Fatalf("len(quotes)=%d want 2", len(quotes))
	}
	rklb, ok := quotes["RKLB"]
	if !ok {
		t.Fatal("missing RKLB")
	}
	if rklb.Price != 64.68 || rklb.Pct != 10.38 {
		t.Fatalf("RKLB=%+v", rklb)
	}
	if _, ok := quotes["NVDA"]; !ok {
		t.Fatal("missing NVDA (should be uppercased)")
	}
}

func TestProviderCircuitBreakerSkipsFailuresAndAllowsRecoveryProbe(t *testing.T) {
	now := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	p := &Provider{now: func() time.Time { return now }}
	for i := 0; i < providerBreakerThreshold; i++ {
		if !p.providerAllowed("tradingview") {
			t.Fatalf("attempt %d unexpectedly blocked", i+1)
		}
		p.providerResult("tradingview", false)
	}
	if p.providerAllowed("tradingview") {
		t.Fatal("circuit breaker did not open after consecutive failures")
	}
	now = now.Add(providerBreakerCooldown)
	if !p.providerAllowed("tradingview") {
		t.Fatal("half-open recovery probe was blocked")
	}
	p.providerResult("tradingview", true)
	if !p.providerAllowed("tradingview") {
		t.Fatal("successful recovery did not close circuit")
	}
}

func TestExchangeSessionUsesMarketTimezoneAndTradingDay(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		at   time.Time
		want string
	}{
		{"weekend", time.Date(2026, 9, 6, 10, 0, 0, 0, loc), "closed"},
		{"premarket", time.Date(2026, 9, 8, 8, 0, 0, 0, loc), "pre"},
		{"regular", time.Date(2026, 9, 8, 10, 0, 0, 0, loc), "regular"},
		{"postmarket", time.Date(2026, 9, 8, 17, 0, 0, 0, loc), "post"},
		{"holiday", time.Date(2026, 9, 7, 10, 0, 0, 0, loc), "closed"},
	}
	for _, tt := range tests {
		if got := exchangeSessionAt("AAPL", tt.at); got != tt.want {
			t.Errorf("%s session=%s want=%s", tt.name, got, tt.want)
		}
	}
	weekday := time.Date(2026, 10, 1, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	if got := normalizedExchangeSession("600519", "closed", weekday); got != "closed" {
		t.Fatalf("authoritative upstream holiday state was overwritten: %s", got)
	}
	cnHoliday := time.Date(2026, 10, 1, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	if got := exchangeSessionAt("600519", cnHoliday); got != "closed" {
		t.Fatalf("CN legal holiday session=%s", got)
	}
	hkHoliday := time.Date(2026, 7, 1, 10, 0, 0, 0, time.FixedZone("HKT", 8*60*60))
	if got := exchangeSessionAt("0700.HK", hkHoliday); got != "closed" {
		t.Fatalf("HK legal holiday session=%s", got)
	}
	hkHalfDayAfternoon := time.Date(2026, 12, 24, 14, 0, 0, 0, time.FixedZone("HKT", 8*60*60))
	if got := exchangeSessionAt("0700.HK", hkHalfDayAfternoon); got != "closed" {
		t.Fatalf("HK half day afternoon session=%s", got)
	}
	outOfCoverage := time.Date(2027, 1, 4, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	if got := exchangeSessionAt("600519", outOfCoverage); got != "closed" {
		t.Fatalf("unmaintained CN calendar must fail closed, got %s", got)
	}
}

func TestParseUsStocksQuotesEmpty(t *testing.T) {
	_, _, err := parseUsStocksQuotes([]byte(`{"stocks":[]}`))
	if err == nil {
		t.Fatal("expected error for empty stocks")
	}
}

func TestSnapshotPayloadPreservesUSGeneratedAtInsteadOfResponseTime(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "app", "frontend", "public", "data", "us-stocks.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"generated_at":"2000-01-01T00:00:00Z","stocks":[{"sym":"AAPL","price":100}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	raw, source, err := SnapshotPayload("us")
	if err != nil {
		t.Fatal(err)
	}
	var payload marketSnapshotPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if payload.Ts != want.UnixMilli() || !strings.Contains(source, want.Format(time.RFC3339)) {
		t.Fatalf("snapshot provenance was refreshed: ts=%d source=%s", payload.Ts, source)
	}
}

func TestLivePayloadMarshalUsesProviderDataTime(t *testing.T) {
	dataAt := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	raw, err := marshalMarketPayload(map[string]snapshotQuote{"AAPL": {Price: 100}}, PayloadStatus{
		Source: "stale-snapshot:us@" + dataAt.Format(time.RFC3339), DataTime: dataAt, Stale: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload marketSnapshotPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Ts != dataAt.UnixMilli() {
		t.Fatalf("live payload used response time: ts=%d want=%d", payload.Ts, dataAt.UnixMilli())
	}
	if payload.Source == "" || payload.DataTime != dataAt.Format(time.RFC3339) || !payload.Stale {
		t.Fatalf("market payload lost structured provenance: %+v", payload)
	}
}

func TestPayloadStatusFailsClosedWhenUSSessionIsClosed(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	closedNow := time.Date(2026, 9, 7, 10, 0, 0, 0, loc)
	providerAt := closedNow.Add(-30 * time.Second)
	status := payloadStatus("us", "tradingview", "https://scanner.tradingview.com/america/scan", providerAt, time.Time{}, closedNow)
	if !status.Stale || status.Session != "closed" || !status.DataTime.Equal(providerAt) || !strings.HasPrefix(status.Source, "closed-tradingview@") {
		t.Fatalf("closed-session payload was presented as live: %+v", status)
	}

	openNow := time.Date(2026, 9, 8, 10, 0, 0, 0, loc)
	status = payloadStatus("us", "tradingview", "https://scanner.tradingview.com/america/scan", openNow.Add(-30*time.Second), time.Time{}, openNow)
	if status.Stale || status.Session != "regular" || !strings.HasPrefix(status.Source, "live-tradingview@") {
		t.Fatalf("fresh open-session payload rejected: %+v", status)
	}
}

func TestLiveCNPayloadExcludesRowsWithoutProviderObservation(t *testing.T) {
	observedAt := time.Date(2026, 9, 8, 7, 30, 0, 0, time.UTC)
	provider := &Provider{
		cnSnap: map[string]snapshotQuote{
			"600519": {Price: 1500, Source: "eastmoney-cn", DataTime: observedAt.Format(time.RFC3339)},
			"000001": {Price: 11.8},
		},
		cnLiveAt: observedAt, cnSnapshotAt: observedAt,
	}
	raw, _, err := provider.LiveCNPayload()
	if err != nil {
		t.Fatal(err)
	}
	var payload marketSnapshotPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Quotes) != 1 || payload.Quotes["600519"].Price <= 0 {
		t.Fatalf("unverified CN rows leaked into live payload: %+v", payload.Quotes)
	}
}

func TestUSPayloadStatusDoesNotPromotePartialRefresh(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, loc)
	oldSnapshot := now.Add(-24 * time.Hour)
	quotes := map[string]snapshotQuote{
		"AAPL": {Price: 230, Source: "tradingview", DataTime: now.Add(-30 * time.Second).UTC().Format(time.RFC3339), Session: "regular"},
		"MSFT": {Price: 500},
	}
	status := usPayloadStatus(quotes, "tradingview", oldSnapshot, now)
	if !status.Stale || !status.DataTime.Equal(oldSnapshot) || !strings.HasPrefix(status.Source, "stale-snapshot:us-incomplete@") {
		t.Fatalf("partial refresh promoted entire market payload: %+v", status)
	}

	quotes["MSFT"] = snapshotQuote{Price: 500, Source: "tradingview", DataTime: now.Add(-45 * time.Second).UTC().Format(time.RFC3339), Session: "regular"}
	status = usPayloadStatus(quotes, "tradingview", oldSnapshot, now)
	if status.Stale || !status.DataTime.Equal(now.Add(-45*time.Second)) {
		t.Fatalf("complete provider-timed payload did not become fresh: %+v", status)
	}
}

func TestUSPayloadStatusDoesNotPromotePartialUniverseCoverage(t *testing.T) {
	dataAt := time.Date(2026, 9, 8, 14, 30, 0, 0, time.UTC)
	status := PayloadStatus{
		Source: "live-tradingview@" + dataAt.Format(time.RFC3339), DataTime: dataAt,
		TimeGranularity: "second", Session: "regular",
	}
	status = failClosedOnPartialUSCoverage(status, 30, 6151)
	if !status.Stale || !strings.HasPrefix(status.Source, "stale-snapshot:us-partial@") {
		t.Fatalf("partial universe coverage was presented as live: %+v", status)
	}

	complete := failClosedOnPartialUSCoverage(PayloadStatus{Source: "live-tradingview", DataTime: dataAt}, 6151, 6151)
	if complete.Stale || complete.Source != "live-tradingview" {
		t.Fatalf("complete universe coverage was degraded: %+v", complete)
	}
}

func TestRefreshUSQuotesUsesBoundedParallelBatchFetchAndMergesCompleteUniverse(t *testing.T) {
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	base := make(map[string]snapshotQuote, liveRefreshBatchSize*(liveRefreshWorkers+1))
	for i := range liveRefreshBatchSize * (liveRefreshWorkers + 1) {
		base[fmt.Sprintf("S%03d", i)] = snapshotQuote{Price: 100, McapB: float64(1000 - i)}
	}
	started := make(chan struct{}, len(base)/liveRefreshBatchSize)
	release := make(chan struct{})
	var active atomic.Int32
	var maximum atomic.Int32
	var patchComplete atomic.Bool
	provider := &Provider{
		usSnap: base, now: func() time.Time { return now },
		usSnapWrite: func(map[string]snapshotQuote, time.Time) error { return nil },
		usFilePatch: func(_ map[string]snapshotQuote, _ time.Time, complete bool) error {
			patchComplete.Store(complete)
			return nil
		},
	}
	provider.usBatchFetch = func(ctx context.Context, symbols []string) map[string]Quote {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			previous := maximum.Load()
			if current <= previous || maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			return nil
		}
		quotes := make(map[string]Quote, len(symbols))
		for _, symbol := range symbols {
			quotes[symbol] = Quote{Price: 101, Source: "tradingview", DataTime: now.Format(time.RFC3339), TimeGranularity: "second"}
		}
		return quotes
	}
	done := make(chan struct {
		updated int
		err     error
	}, 1)
	go func() {
		updated, err := provider.RefreshUSQuotes(0)
		done <- struct {
			updated int
			err     error
		}{updated, err}
	}()
	for range liveRefreshWorkers {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("worker pool did not fill concurrently")
		}
	}
	select {
	case <-started:
		close(release)
		t.Fatalf("batch concurrency exceeded %d workers", liveRefreshWorkers)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	result := <-done
	if result.err != nil || result.updated != len(base) || maximum.Load() != liveRefreshWorkers {
		t.Fatalf("updated=%d err=%v maxConcurrency=%d", result.updated, result.err, maximum.Load())
	}
	if !patchComplete.Load() || !provider.usSnapshotAt.Equal(now) || len(provider.usLiveSnap) != len(base) {
		t.Fatalf("complete merge: complete=%v snapshotAt=%s live=%d", patchComplete.Load(), provider.usSnapshotAt, len(provider.usLiveSnap))
	}
}

func TestRefreshUSQuotesPartialCoverageDoesNotAdvanceCompleteSnapshot(t *testing.T) {
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	old := now.Add(-24 * time.Hour)
	base := make(map[string]snapshotQuote, 60)
	for i := range 60 {
		base[fmt.Sprintf("S%03d", i)] = snapshotQuote{Price: 100, McapB: float64(1000 - i), DataTime: old.Format(time.RFC3339)}
	}
	patchComplete := true
	provider := &Provider{
		usSnap: base, usSnapshotAt: old, now: func() time.Time { return now },
		usSnapWrite: func(map[string]snapshotQuote, time.Time) error { return nil },
		usFilePatch: func(_ map[string]snapshotQuote, _ time.Time, complete bool) error {
			patchComplete = complete
			return nil
		},
		usBatchFetch: func(_ context.Context, symbols []string) map[string]Quote {
			return map[string]Quote{symbols[0]: {Price: 101, Source: "tradingview", DataTime: now.Format(time.RFC3339), TimeGranularity: "second"}}
		},
	}
	updated, err := provider.RefreshUSQuotes(0)
	if err == nil || updated != 2 {
		t.Fatalf("full partial refresh must fail closed: updated=%d err=%v", updated, err)
	}
	if patchComplete || !provider.usSnapshotAt.Equal(old) {
		t.Fatalf("partial refresh advanced complete snapshot: complete=%v snapshotAt=%s", patchComplete, provider.usSnapshotAt)
	}
	status := provider.USPayloadStatus(now)
	if !status.Stale || !strings.HasPrefix(status.Source, "stale-snapshot:us-partial") {
		t.Fatalf("partial refresh presented as complete: %+v", status)
	}
}

func TestRefreshUSBatchFetchUsesOneBoundedYahooRequestForTradingViewGaps(t *testing.T) {
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	yahooCalls := 0
	provider := &Provider{
		now: func() time.Time { return now },
		usTVFetch: func(symbols []string) ([]dataflows.TVQuoteData, error) {
			if len(symbols) != 3 {
				t.Fatalf("TV symbols=%v", symbols)
			}
			return []dataflows.TVQuoteData{{Symbol: "NASDAQ:AAPL", Price: 200, DataTime: now, SourceURL: "https://scanner.tradingview.com"}}, nil
		},
		usYahooFetch: func(symbols []string) (map[string]Quote, error) {
			yahooCalls++
			if strings.Join(symbols, ",") != "MSFT,NVDA" {
				t.Fatalf("Yahoo gap symbols=%v", symbols)
			}
			return map[string]Quote{
				"MSFT": {Price: 500, Source: "yahoo", DataTime: now.Format(time.RFC3339)},
				"NVDA": {Price: 180, Source: "yahoo", DataTime: now.Format(time.RFC3339)},
			}, nil
		},
	}
	quotes := provider.fetchUSRefreshBatch(context.Background(), []string{"AAPL", "MSFT", "NVDA"})
	if yahooCalls != 1 || len(quotes) != 3 || quotes["AAPL"].Source != "tradingview" || quotes["MSFT"].Source != "yahoo" {
		t.Fatalf("calls=%d quotes=%+v", yahooCalls, quotes)
	}
}

func TestRefreshUSBatchFetchStopsFallbackAfterContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	yahooCalls := 0
	provider := &Provider{
		now: time.Now,
		usTVFetch: func([]string) ([]dataflows.TVQuoteData, error) {
			cancel()
			return nil, errors.New("TV unavailable")
		},
		usYahooFetch: func([]string) (map[string]Quote, error) {
			yahooCalls++
			return nil, nil
		},
	}
	if quotes := provider.fetchUSRefreshBatch(ctx, []string{"AAPL"}); len(quotes) != 0 || yahooCalls != 0 {
		t.Fatalf("cancelled batch used fallback: calls=%d quotes=%v", yahooCalls, quotes)
	}
}

func TestSecondaryFastQuotesRunsYahooAndNasdaqInParallelAndMerges(t *testing.T) {
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	provider := &Provider{
		now: func() time.Time { return now },
		directYahooFetch: func([]string) (map[string]Quote, error) {
			started <- struct{}{}
			<-release
			return map[string]Quote{"AAPL": {Price: 200, Source: "yahoo", DataTime: now.Format(time.RFC3339)}}, nil
		},
		directNasdaqFetch: func([]string) map[string]dataflows.NasdaqQuote {
			started <- struct{}{}
			<-release
			return map[string]dataflows.NasdaqQuote{"NVDA": {
				Symbol: "NVDA", Price: 180, TradeTime: now, TimeGranularity: "second", MarketStatus: "regular",
				Currency: "USD", AssetClass: "stocks", SourceURL: "https://api.nasdaq.com/api/quote/NVDA/info",
			}}
		},
	}
	done := make(chan map[string]Quote, 1)
	go func() { done <- provider.SecondaryFastQuotes(context.Background(), []string{"AAPL", "NVDA"}) }()
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("secondary providers did not start in parallel")
		}
	}
	close(release)
	quotes := <-done
	if len(quotes) != 2 || quotes["AAPL"].Source != "yahoo" || quotes["NVDA"].Source != "nasdaq" || quotes["NVDA"].DataTime == "" {
		t.Fatalf("secondary merged quotes=%+v", quotes)
	}
}

func TestSecondaryFastQuotesNeverFallsBackToSnapshot(t *testing.T) {
	provider := &Provider{
		now:               time.Now,
		usSnap:            map[string]snapshotQuote{"AAPL": {Price: 199, Source: "tradingview", DataTime: time.Now().UTC().Format(time.RFC3339)}},
		directYahooFetch:  func([]string) (map[string]Quote, error) { return nil, errors.New("unavailable") },
		directNasdaqFetch: func([]string) map[string]dataflows.NasdaqQuote { return nil },
	}
	if quotes := provider.SecondaryFastQuotes(context.Background(), []string{"AAPL"}); len(quotes) != 0 {
		t.Fatalf("secondary path exposed snapshot as live: %+v", quotes)
	}
}

func TestSecondaryFastQuotesKeepsNewestProviderObservation(t *testing.T) {
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	provider := &Provider{
		now: func() time.Time { return now },
		directYahooFetch: func([]string) (map[string]Quote, error) {
			return map[string]Quote{"AAPL": {Price: 315, Source: "yahoo", DataTime: now.Add(-14 * time.Hour).Format(time.RFC3339)}}, nil
		},
		directNasdaqFetch: func([]string) map[string]dataflows.NasdaqQuote {
			return map[string]dataflows.NasdaqQuote{"AAPL": {
				Symbol: "AAPL", Price: 318, TradeTime: now.Add(-10 * time.Second), TimeGranularity: "second",
				MarketStatus: "pre", Currency: "USD", AssetClass: "stocks",
			}}
		},
	}
	quotes := provider.SecondaryFastQuotes(context.Background(), []string{"AAPL"})
	if quotes["AAPL"].Source != "nasdaq" || quotes["AAPL"].Price != 318 {
		t.Fatalf("newest provider observation was not selected: %+v", quotes["AAPL"])
	}
}

func TestCachedQuotesPrefersObservedMemoryWithoutNetwork(t *testing.T) {
	at := time.Date(2026, 9, 8, 20, 0, 0, 0, time.UTC)
	provider := &Provider{
		usLiveSnap: map[string]snapshotQuote{"AAPL": {Price: 230, Source: "tradingview", DataTime: at.Format(time.RFC3339), TimeGranularity: "second"}},
		usSnap: map[string]snapshotQuote{
			"AAPL": {Price: 220, DataTime: "2026-09-07"},
			"MSFT": {Price: 500, DataTime: "2026-09-07", TimeGranularity: "date"},
		},
		now: func() time.Time { return at.Add(time.Hour) },
	}
	quotes := provider.CachedQuotes([]string{"AAPL", "MSFT"})
	if quotes["AAPL"].Price != 230 || quotes["AAPL"].Source != "tradingview" {
		t.Fatalf("live cached quote=%+v", quotes["AAPL"])
	}
	if quotes["MSFT"].Price != 500 || quotes["MSFT"].Source != "stale-snapshot:us" {
		t.Fatalf("snapshot cached quote=%+v", quotes["MSFT"])
	}
}

func TestFreshUSLiveSnapshotExcludesProviderStaleRows(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 8, 10, 0, 0, 0, loc)
	quotes := map[string]snapshotQuote{
		"AAPL": {Price: 230, DataTime: now.Add(-30 * time.Second).UTC().Format(time.RFC3339)},
		"OLD":  {Price: 10, DataTime: now.Add(-20 * time.Minute).UTC().Format(time.RFC3339)},
	}
	fresh := freshUSLiveSnapshot(quotes, now)
	if len(fresh) != 1 || fresh["AAPL"].Price != 230 {
		t.Fatalf("fresh subset=%+v", fresh)
	}
}

func TestReloadSnapshotsKeepsUSAndCNTimesSeparateWithMtimeFallback(t *testing.T) {
	root := t.TempDir()
	usPath := filepath.Join(root, "app", "frontend", "public", "data", "us-stocks.json")
	cnPath := filepath.Join(root, "data", "a-market.json")
	for path, content := range map[string]string{
		usPath: `{"stocks":[{"sym":"AAPL","price":100}]}`,
		cnPath: `{"ts":946771200000,"quotes":{"600519":{"price":1000}}}`,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	usMtime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(usPath, usMtime, usMtime); err != nil {
		t.Fatal(err)
	}
	previous, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	p := New()
	p.ReloadSnapshots()
	if !p.SnapshotDataTime("us").Equal(usMtime) {
		t.Fatalf("US snapshot time=%s want mtime=%s", p.SnapshotDataTime("us"), usMtime)
	}
	if p.SnapshotDataTime("cn").Equal(p.SnapshotDataTime("us")) || p.SnapshotDataTime("cn").IsZero() {
		t.Fatalf("CN snapshot time not independently retained: us=%s cn=%s", p.SnapshotDataTime("us"), p.SnapshotDataTime("cn"))
	}
	raw, source, err := SnapshotPayload("us")
	if err != nil {
		t.Fatal(err)
	}
	var payload marketSnapshotPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Ts != usMtime.UnixMilli() || !strings.Contains(source, usMtime.Format(time.RFC3339)) {
		t.Fatalf("mtime fallback not shared by payload/source: ts=%d source=%s", payload.Ts, source)
	}
}

func TestOverlayMatchingSnapshotProvenanceRequiresSameQuote(t *testing.T) {
	target := map[string]snapshotQuote{
		"AAPL": {Price: 200, Pct: 1},
		"NVDA": {Price: 100, Pct: 2},
	}
	evidence := map[string]snapshotQuote{
		"AAPL": {Price: 200, Pct: 1, Source: "nasdaq", DataTime: "2026-09-03T20:00:00Z", ProviderURL: "https://api.nasdaq.com/aapl"},
		"NVDA": {Price: 101, Pct: 2, Source: "nasdaq", DataTime: "2026-09-03T20:00:00Z", ProviderURL: "https://api.nasdaq.com/nvda"},
	}
	overlayMatchingSnapshotProvenance(target, evidence)
	if target["AAPL"].DataTime == "" || target["AAPL"].ProviderURL == "" {
		t.Fatalf("matching quote lost provenance: %+v", target["AAPL"])
	}
	if target["NVDA"].DataTime != "" {
		t.Fatalf("mismatched quote inherited wrong provenance: %+v", target["NVDA"])
	}
}
