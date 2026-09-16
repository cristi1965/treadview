package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"trading-agents/internal/market"
)

func TestBoundedQuotesSharesSameSymbolFlight(t *testing.T) {
	previous := quoteProviderFetch
	previousSecondary := quoteSecondaryFetch
	t.Cleanup(func() { quoteProviderFetch = previous; quoteSecondaryFetch = previousSecondary })
	var calls atomic.Int32
	var secondaryCalls atomic.Int32
	quoteProviderFetch = func(_ *market.Provider, symbols []string) map[string]market.Quote {
		calls.Add(1)
		time.Sleep(100 * time.Millisecond)
		now := time.Now().UTC().Format(time.RFC3339)
		result := make(map[string]market.Quote, len(symbols))
		for _, symbol := range symbols {
			result[symbol] = market.Quote{Price: 100, Source: "fixture-provider", DataTime: now, Session: "regular"}
		}
		return result
	}
	quoteSecondaryFetch = func(_ context.Context, _ *market.Provider, _ []string) map[string]market.Quote {
		secondaryCalls.Add(1)
		return nil
	}
	var wait sync.WaitGroup
	wait.Add(2)
	go func() { defer wait.Done(); _ = boundedQuotesWithMeta([]string{"AAPL", "NVDA"}) }()
	go func() { defer wait.Done(); _ = boundedQuotesWithMeta([]string{"NVDA", "AAPL"}) }()
	wait.Wait()
	if calls.Load() != 1 || secondaryCalls.Load() != 1 {
		t.Fatalf("same symbol set started primary=%d secondary=%d upstream calls", calls.Load(), secondaryCalls.Load())
	}
}

func TestBoundedQuotesAllowsCompleteSecondaryToBeatSlowPrimary(t *testing.T) {
	previous := quoteProviderFetch
	previousSecondary := quoteSecondaryFetch
	previousGrace := quotePrimaryGrace
	t.Cleanup(func() {
		quoteProviderFetch = previous
		quoteSecondaryFetch = previousSecondary
		quotePrimaryGrace = previousGrace
	})
	quotePrimaryGrace = func(*market.Provider) time.Duration { return primaryQuoteGrace }
	release := make(chan struct{})
	quoteProviderFetch = func(_ *market.Provider, _ []string) map[string]market.Quote {
		<-release
		return nil
	}
	observedAt := time.Now().UTC().Add(-time.Second).Format(time.RFC3339)
	quoteSecondaryFetch = func(_ context.Context, _ *market.Provider, _ []string) map[string]market.Quote {
		return map[string]market.Quote{
			"AAPL": {Price: 200, Source: "yahoo", DataTime: observedAt, Session: "regular"},
			"NVDA": {Price: 180, Source: "nasdaq", DataTime: observedAt, Session: "regular"},
		}
	}
	startedAt := time.Now()
	result := boundedQuotesWithMeta([]string{"AAPL", "NVDA"})
	close(release)
	if elapsed := time.Since(startedAt); elapsed >= time.Second {
		t.Fatalf("complete secondary did not win promptly: %s", elapsed)
	}
	if result.timedOut || len(result.quotes) != 2 || result.source != "self-provider:mixed" {
		t.Fatalf("secondary result=%+v", result)
	}
}

func TestBoundedQuotesFreshSecondaryBeatsCompletedStalePrimary(t *testing.T) {
	previous := quoteProviderFetch
	previousSecondary := quoteSecondaryFetch
	t.Cleanup(func() { quoteProviderFetch = previous; quoteSecondaryFetch = previousSecondary })
	now := time.Now().UTC()
	quoteProviderFetch = func(_ *market.Provider, _ []string) map[string]market.Quote {
		return map[string]market.Quote{
			"AAPL": {Price: 315, Source: "yahoo-chart", DataTime: now.Add(-14 * time.Hour).Format(time.RFC3339), Session: "pre"},
		}
	}
	quoteSecondaryFetch = func(_ context.Context, _ *market.Provider, _ []string) map[string]market.Quote {
		time.Sleep(25 * time.Millisecond)
		return map[string]market.Quote{
			"AAPL": {Price: 318, Source: "nasdaq", DataTime: now.Add(-time.Second).Format(time.RFC3339), Session: "pre"},
		}
	}
	result := boundedQuotesWithMeta([]string{"AAPL"})
	if result.timedOut || result.source != "nasdaq" || result.quotes["AAPL"].Price != 318 {
		t.Fatalf("fresh secondary did not beat stale primary: %+v", result)
	}
}

func TestBoundedQuotesRejectsPartialSecondaryAndKeepsPrimaryPriority(t *testing.T) {
	assertSecondaryCannotWin(t, map[string]market.Quote{
		"AAPL": {Price: 200, Source: "yahoo", DataTime: time.Now().UTC().Format(time.RFC3339)},
	})
}

func TestBoundedQuotesRejectsSecondaryWithoutProviderTime(t *testing.T) {
	assertSecondaryCannotWin(t, map[string]market.Quote{
		"AAPL": {Price: 200, Source: "yahoo", DataTime: time.Now().UTC().Format(time.RFC3339)},
		"NVDA": {Price: 180, Source: "nasdaq"},
	})
}

func assertSecondaryCannotWin(t *testing.T, secondary map[string]market.Quote) {
	t.Helper()
	previous := quoteProviderFetch
	previousSecondary := quoteSecondaryFetch
	t.Cleanup(func() { quoteProviderFetch = previous; quoteSecondaryFetch = previousSecondary })
	primaryObservedAt := time.Now().UTC().Add(-time.Second).Format(time.RFC3339)
	quoteProviderFetch = func(_ *market.Provider, _ []string) map[string]market.Quote {
		time.Sleep(75 * time.Millisecond)
		return map[string]market.Quote{
			"AAPL": {Price: 201, Source: "tradingview", DataTime: primaryObservedAt, Session: "regular"},
			"NVDA": {Price: 181, Source: "tradingview", DataTime: primaryObservedAt, Session: "regular"},
		}
	}
	quoteSecondaryFetch = func(_ context.Context, _ *market.Provider, _ []string) map[string]market.Quote { return secondary }
	result := boundedQuotesWithMeta([]string{"AAPL", "NVDA"})
	if result.timedOut || result.source != "tradingview" || result.quotes["AAPL"].Price != 201 || result.quotes["NVDA"].Price != 181 {
		t.Fatalf("untrusted secondary won race: %+v", result)
	}
}

func TestMarketQuoteFreshness(t *testing.T) {
	now := time.Date(2026, time.September, 6, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		source   string
		dataTime time.Time
		stale    bool
		reason   string
	}{
		{name: "fresh live", source: "tradingview", dataTime: now.Add(-30 * time.Second)},
		{name: "expired live", source: "tradingview", dataTime: now.Add(-91 * time.Second), stale: true, reason: "quote age"},
		{name: "snapshot always degraded", source: "stale-snapshot:us", dataTime: now.Add(-10 * time.Second), stale: true, reason: "last real snapshot"},
		{name: "missing provenance", source: "tradingview", stale: true, reason: "missing quote data time"},
		{name: "future provenance", source: "tradingview", dataTime: now.Add(2 * time.Minute), stale: true, reason: "in the future"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stale, reason := marketQuoteFreshness(tt.source, tt.dataTime, now)
			if stale != tt.stale || !strings.Contains(reason, tt.reason) {
				t.Fatalf("stale=%v reason=%q, want stale=%v reason containing %q", stale, reason, tt.stale, tt.reason)
			}
		})
	}
}

func TestQuoteResultFreshness(t *testing.T) {
	now := time.Date(2026, time.September, 6, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		result quoteFetchResult
		stale  bool
		reason string
	}{
		{name: "fresh fetch", result: quoteFetchResult{quotes: map[string]market.Quote{"NVDA": {Price: 230}}, source: "self-provider", dataTime: now.Add(-time.Second)}},
		{name: "stale result", result: quoteFetchResult{quotes: map[string]market.Quote{"NVDA": {Price: 229}}, source: "stale-snapshot", dataTime: now.Add(-time.Second)}, stale: true, reason: "last real snapshot"},
		{name: "timeout", result: quoteFetchResult{quotes: map[string]market.Quote{"NVDA": {Price: 229}}, source: "stale-snapshot", dataTime: now.Add(-time.Hour), timedOut: true}, stale: true, reason: "timed out"},
		{name: "empty result", result: quoteFetchResult{source: "self-provider", dataTime: now}, stale: true, reason: "returned no quotes"},
		{name: "closed session", result: quoteFetchResult{quotes: map[string]market.Quote{"NVDA": {Price: 230, Session: "closed"}}, source: "tradingview", dataTime: now.Add(-time.Second)}, stale: true, reason: "session is closed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stale, reason := quoteResultFreshness(tt.result, now)
			if stale != tt.stale || !strings.Contains(reason, tt.reason) {
				t.Fatalf("stale=%v reason=%q, want stale=%v reason containing %q", stale, reason, tt.stale, tt.reason)
			}
		})
	}
}

func TestPaperRejectsDisplayOnlyQuoteButAcceptsAlpacaIEX(t *testing.T) {
	now := time.Date(2026, time.September, 8, 15, 0, 0, 0, time.UTC)
	displayOnly := paperQuoteSnapshotFromResult([]string{"NVDA"}, now, quoteFetchResult{
		quotes: map[string]market.Quote{"NVDA": {Price: 100, Source: "tradingview", ProviderMode: "display-only:scanner"}},
		source: "tradingview", dataTime: now,
	})
	if len(displayOnly.Reasons["NVDA"]) == 0 {
		t.Fatal("display-only TradingView quote must not authorize a Paper order")
	}
	alpaca := paperQuoteSnapshotFromResult([]string{"NVDA"}, now, quoteFetchResult{
		quotes: map[string]market.Quote{"NVDA": {Price: 100, Source: "alpaca-iex", ProviderMode: "iex-only"}},
		source: "alpaca-iex", dataTime: now,
	})
	if len(alpaca.Reasons["NVDA"]) != 0 {
		t.Fatalf("fresh Alpaca IEX quote should authorize Paper: %v", alpaca.Reasons["NVDA"])
	}
}

func TestGetQuoteDataFreshnessHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	snapshotAt := time.Date(2026, time.September, 5, 7, 30, 0, 0, time.UTC)
	tests := []struct {
		name            string
		result          quoteFetchResult
		wantStale       string
		wantSource      string
		wantDataTime    string
		wantReasonPart  string
		wantPartialPart string
	}{
		{
			name: "live quote",
			result: quoteFetchResult{
				quotes:   map[string]market.Quote{"NVDA": {Price: 230, Source: "tradingview"}},
				dataTime: time.Now(), source: "self-provider",
			},
			wantStale: "false", wantSource: "self-provider",
		},
		{
			name: "snapshot quote keeps provenance time",
			result: quoteFetchResult{
				quotes:   map[string]market.Quote{"NVDA": {Price: 229, Source: "stale-snapshot:us", DataTime: snapshotAt.Format(time.RFC3339)}},
				dataTime: snapshotAt, source: "stale-snapshot:provider-observations",
			},
			wantStale: "true", wantSource: "stale-snapshot:provider-observations", wantDataTime: snapshotAt.Format(time.RFC3339), wantReasonPart: "last real snapshot",
		},
		{
			name:      "timeout and missing symbol",
			result:    quoteFetchResult{quotes: map[string]market.Quote{}, timedOut: true, dataTime: snapshotAt, source: "stale-snapshot:snapshot-batch-time"},
			wantStale: "true", wantSource: "stale-snapshot:snapshot-batch-time", wantDataTime: snapshotAt.Format(time.RFC3339), wantReasonPart: "timed out", wantPartialPart: "missing:NVDA",
		},
	}

	previous := boundedQuotesForRequest
	t.Cleanup(func() { boundedQuotesForRequest = previous })
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boundedQuotesForRequest = func([]string) quoteFetchResult { return tt.result }
			r := gin.New()
			r.GET("/api/quote", GetQuoteData)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/quote?syms=NVDA", nil))

			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if got := w.Header().Get("X-Data-Stale"); got != tt.wantStale {
				t.Errorf("X-Data-Stale=%q want %q", got, tt.wantStale)
			}
			if got := w.Header().Get("X-Data-Source"); got != tt.wantSource {
				t.Errorf("X-Data-Source=%q want %q", got, tt.wantSource)
			}
			if tt.wantDataTime != "" && w.Header().Get("X-Data-Time") != tt.wantDataTime {
				t.Errorf("X-Data-Time=%q want %q", w.Header().Get("X-Data-Time"), tt.wantDataTime)
			}
			if !strings.Contains(w.Header().Get("X-Data-Stale-Reason"), tt.wantReasonPart) {
				t.Errorf("X-Data-Stale-Reason=%q want containing %q", w.Header().Get("X-Data-Stale-Reason"), tt.wantReasonPart)
			}
			if !strings.Contains(w.Header().Get("X-Data-Partial-Errors"), tt.wantPartialPart) {
				t.Errorf("X-Data-Partial-Errors=%q want containing %q", w.Header().Get("X-Data-Partial-Errors"), tt.wantPartialPart)
			}
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if tt.wantDataTime != "" {
				want, err := time.Parse(time.RFC3339, tt.wantDataTime)
				if err != nil || int64(body["ts"].(float64)) != want.UnixMilli() {
					t.Fatalf("body ts=%v must be provider data time %s", body["ts"], tt.wantDataTime)
				}
				if body["dataTime"] != tt.wantDataTime {
					t.Fatalf("body dataTime=%v must match provider data time %s", body["dataTime"], tt.wantDataTime)
				}
			}
		})
	}
}

func TestNewQuoteFetchResultUsesOldestReturnedObservation(t *testing.T) {
	newer := time.Date(2026, 9, 5, 20, 0, 0, 0, time.UTC)
	older := newer.Add(-24 * time.Hour)
	result := newQuoteFetchResult(market.New(), map[string]market.Quote{
		"AAPL": {Price: 230, Source: "nasdaq", DataTime: newer.Format(time.RFC3339)},
		"NVDA": {Price: 220, Source: "stale-snapshot:us", DataTime: older.Format(time.RFC3339)},
	}, false, newer.Add(time.Hour))
	if !result.dataTime.Equal(older) || result.source != "stale-snapshot:provider-observations" {
		t.Fatalf("mixed result must use oldest object observation: %+v", result)
	}
}

func TestPrimaryQuoteGraceWaitsForConfiguredAlpaca(t *testing.T) {
	t.Setenv("STOCKGOD_ALPACA_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "missing.env"))
	t.Setenv("ALPACA_API_KEY_ID", "paper-id")
	t.Setenv("ALPACA_API_SECRET_KEY", "paper-secret")
	if got := primaryQuoteGraceFor(market.New()); got != alpacaQuoteGrace {
		t.Fatalf("configured Alpaca grace=%s want %s", got, alpacaQuoteGrace)
	}
	t.Setenv("ALPACA_API_KEY_ID", "")
	t.Setenv("ALPACA_API_SECRET_KEY", "")
	if got := primaryQuoteGraceFor(market.New()); got != primaryQuoteGrace {
		t.Fatalf("unconfigured grace=%s want %s", got, primaryQuoteGrace)
	}
}

func TestNewQuoteFetchResultFailsClosedWhenReturnedObjectHasNoObservation(t *testing.T) {
	observed := time.Date(2026, 9, 5, 20, 0, 0, 0, time.UTC)
	result := newQuoteFetchResult(market.New(), map[string]market.Quote{
		"AAPL": {Price: 230, Source: "nasdaq", DataTime: observed.Format(time.RFC3339)},
		"NVDA": {Price: 220, Source: "stale-snapshot:us"},
	}, false, observed.Add(time.Hour))
	if !result.dataTime.IsZero() || result.source != "unverified-provider-time" {
		t.Fatalf("missing object observation must invalidate aggregate time: %+v", result)
	}
}

func TestSetDataFreshnessDoesNotInventDataTime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setDataFreshness(c, dataFreshnessMeta{Source: "unknown", Stale: true})
	if got := w.Header().Get("X-Data-Time"); got != "" {
		t.Fatalf("X-Data-Time=%q; missing provenance must stay missing", got)
	}
}

func TestNewQuoteFetchResultUsesNasdaqSourceObservationTime(t *testing.T) {
	observed := time.Date(2026, 9, 3, 20, 0, 0, 0, time.UTC)
	result := newQuoteFetchResult(market.New(), map[string]market.Quote{
		"AAPL": {Price: 234.56, Source: "nasdaq", DataTime: observed.Format(time.RFC3339)},
	}, false, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	if result.source != "nasdaq" || !result.dataTime.Equal(observed) {
		t.Fatalf("source observation lost: %+v", result)
	}
}

func TestGetQuoteDataPreservesDateOnlyTopLevelTime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := boundedQuotesForRequest
	t.Cleanup(func() { boundedQuotesForRequest = previous })
	boundedQuotesForRequest = func([]string) quoteFetchResult {
		return newQuoteFetchResult(market.New(), map[string]market.Quote{"AAPL": {
			Price: 230, Source: "nasdaq", DataTime: "2026-09-03", TimeGranularity: "date",
		}}, false, time.Now())
	}
	router := gin.New()
	router.GET("/api/quote", GetQuoteData)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/quote?syms=AAPL", nil))
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Header().Get("X-Data-Time") != "2026-09-03" || w.Header().Get("X-Data-Time-Granularity") != "date" || body["dataTime"] != "2026-09-03" || body["timeGranularity"] != "date" || body["ts"].(float64) != 0 {
		t.Fatalf("date-only source gained false top-level precision: headers=%v body=%v", w.Header(), body)
	}
}

func TestNewQuoteFetchResultDoesNotUseCompletionTimeAsDataTime(t *testing.T) {
	result := newQuoteFetchResult(market.New(), map[string]market.Quote{
		"AAPL": {Price: 234.56, Source: "tradingview"},
	}, false, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	if !result.dataTime.IsZero() || result.source != "unverified-provider-time" {
		t.Fatalf("completion time leaked into quote provenance: %+v", result)
	}
}
