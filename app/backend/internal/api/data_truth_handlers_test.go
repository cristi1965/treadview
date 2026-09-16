package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"trading-agents/internal/dataflows"
	"trading-agents/internal/market"

	"github.com/gin-gonic/gin"
)

func TestMarketOverviewFailsClosedWithoutAttributableObservation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := fetchMarketOverview
	t.Cleanup(func() { fetchMarketOverview = previous })
	fetchMarketOverview = func() dataflows.ComprehensiveMarketOverview {
		return dataflows.ComprehensiveMarketOverview{CNIndices: []dataflows.MarketIndexItem{{Symbol: "000001", Price: 1, Available: true, Source: "eastmoney"}}}
	}
	router := gin.New()
	router.GET("/api/cn-market-overview", GetMarketOverview)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/cn-market-overview", nil))
	if w.Code != http.StatusServiceUnavailable || w.Header().Get("X-Data-Stale") != "true" || w.Header().Get("X-Data-Time") != "" {
		t.Fatalf("status=%d headers=%v body=%s", w.Code, w.Header(), w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body["error"] == nil {
		t.Fatalf("missing explicit unavailable response: %s", w.Body.String())
	}
}

func TestMarketOverviewFailsClosedWhenOldestObservationIsStale(t *testing.T) {
	previous := fetchMarketOverview
	t.Cleanup(func() { fetchMarketOverview = previous })
	observed := time.Now().UTC().Add(-4 * 24 * time.Hour).Truncate(time.Second)
	fetchMarketOverview = func() dataflows.ComprehensiveMarketOverview {
		return dataflows.ComprehensiveMarketOverview{UpdatedAt: observed.Format(time.RFC3339), CNIndices: []dataflows.MarketIndexItem{{Symbol: "000001", Price: 1, Available: true, Source: "eastmoney", DataTime: observed.Format(time.RFC3339)}}}
	}
	router := gin.New()
	router.GET("/overview", GetMarketOverview)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/overview", nil))
	if w.Code != http.StatusServiceUnavailable || w.Header().Get("X-Data-Stale") != "true" {
		t.Fatalf("status=%d headers=%v body=%s", w.Code, w.Header(), w.Body.String())
	}
}

func TestChatStaleQuoteIsMetadataNotRealtimeFact(t *testing.T) {
	observed := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	context, metadata := buildChatQuoteContext("AAPL", quoteFetchResult{
		quotes: map[string]market.Quote{"AAPL": {
			Price: 200, Pct: 1.5, Currency: "USD", Source: "stale-snapshot:us",
			DataTime: observed.Format(time.RFC3339), Session: "closed",
		}},
		source: "stale-snapshot:provider-observations", dataTime: observed,
	}, time.Now())
	if strings.Contains(context, "【实时标的行情】") || !strings.Contains(context, "不得作为当前实时事实") {
		t.Fatalf("stale context was promoted to realtime: %q", context)
	}
	if metadata["source"] != "stale-snapshot:us" || metadata["dataTime"] != observed.Format(time.RFC3339) || metadata["stale"] != true {
		t.Fatalf("stale provenance lost: %+v", metadata)
	}
}

func TestArenaDegradedQuotesDoNotChangeValueOrRank(t *testing.T) {
	previous := boundedQuotesForRequest
	t.Cleanup(func() { boundedQuotesForRequest = previous })
	observed := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	boundedQuotesForRequest = func([]string) quoteFetchResult {
		return quoteFetchResult{
			quotes: map[string]market.Quote{"AAPL": {Price: 999, Source: "stale-snapshot:us", DataTime: observed.Format(time.RFC3339)}},
			source: "stale-snapshot:provider-observations", dataTime: observed,
		}
	}
	players := []ArenaPlayer{
		{ID: "first", Rank: 1, Cash: 0, Value: 100, Holdings: []ArenaHolding{{Symbol: "AAPL", Shares: 1, Price: 100}}},
		{ID: "second", Rank: 2, Cash: 0, Value: 90, Holdings: []ArenaHolding{{Symbol: "MSFT", Shares: 1, Price: 90}}},
	}
	update := updateArenaQuotes(players, time.Now())
	if !update.Degraded || update.Applied || players[0].Value != 100 || players[0].Holdings[0].Price != 100 || players[0].Rank != 1 {
		t.Fatalf("degraded quote changed ranking inputs: update=%+v players=%+v", update, players)
	}
}

func TestArenaFreshCompleteQuotesApplyAtomically(t *testing.T) {
	previous := boundedQuotesForRequest
	t.Cleanup(func() { boundedQuotesForRequest = previous })
	observed := time.Now().UTC().Add(-time.Second).Truncate(time.Second)
	boundedQuotesForRequest = func([]string) quoteFetchResult {
		return quoteFetchResult{
			quotes: map[string]market.Quote{
				"AAPL": {Price: 110, Source: "nasdaq", DataTime: observed.Format(time.RFC3339), Session: "regular"},
				"MSFT": {Price: 80, Source: "nasdaq", DataTime: observed.Format(time.RFC3339), Session: "regular"},
			},
			source: "nasdaq", dataTime: observed,
		}
	}
	players := []ArenaPlayer{
		{ID: "first", Rank: 1, Value: 100, Holdings: []ArenaHolding{{Symbol: "AAPL", Shares: 1, Cost: 100, Price: 100}}},
		{ID: "second", Rank: 2, Value: 90, Holdings: []ArenaHolding{{Symbol: "MSFT", Shares: 2, Cost: 90, Price: 90}}},
	}
	update := updateArenaQuotes(players, time.Now())
	if update.Degraded || !update.Applied || players[0].Value != 110 || players[1].Value != 160 {
		t.Fatalf("fresh complete quote batch not applied: update=%+v players=%+v", update, players)
	}
}

func TestArenaResponseMarksStaleBatchDegraded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := boundedQuotesForRequest
	t.Cleanup(func() { boundedQuotesForRequest = previous })
	observed := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	boundedQuotesForRequest = func([]string) quoteFetchResult {
		return quoteFetchResult{
			quotes: map[string]market.Quote{}, source: "stale-snapshot:us",
			dataTime: observed, dataTimeLabel: observed.Format(time.RFC3339), timeGranularity: "second", timedOut: true,
		}
	}
	h := &Handler{}
	router := gin.New()
	router.GET("/api/arena", h.GetArena)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/arena?market=cn", nil))
	if w.Code != http.StatusOK || w.Header().Get("X-Data-Stale") != "true" || w.Header().Get("X-Data-Time") != observed.Format(time.RFC3339) || w.Header().Get("X-Data-Time-Granularity") != "second" {
		t.Fatalf("status=%d headers=%v body=%s", w.Code, w.Header(), w.Body.String())
	}
	var body ArenaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || !body.Degraded || body.DegradedReason == "" {
		t.Fatalf("degraded response not explicit: %+v err=%v", body, err)
	}
}
