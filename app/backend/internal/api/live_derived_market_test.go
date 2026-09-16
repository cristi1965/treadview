package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/market"
)

func TestBuildLiveMacroSnapshotRequiresCompleteProvenance(t *testing.T) {
	at := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	base := macroSnapshot{Series: []macroSeriesItem{
		{Sym: "SPY", Name: "S&P 500", Kind: "index"},
		{Sym: "TLT", Name: "Treasury", Kind: "rate"},
	}}
	quotes := map[string]market.Quote{
		"SPY": {Price: 700, Pct: 1, Source: "tradingview", DataTime: at.Format(time.RFC3339)},
		"TLT": {Price: 90, Pct: -1, Source: "yahoo-chart", DataTime: at.Format(time.RFC3339)},
	}
	live, ok := buildLiveMacroSnapshot(base, quotes, at)
	if !ok || live.Ts != at.UnixMilli() || len(live.Series) != 2 || live.Series[0].Price != 700 {
		t.Fatalf("live=%+v ok=%v", live, ok)
	}
	delete(quotes, "TLT")
	if _, ok := buildLiveMacroSnapshot(base, quotes, at); ok {
		t.Fatal("partial macro quote set must fail closed")
	}
}

func TestMacroPublishesCompleteClosedMarketQuotesAsStaleSnapshot(t *testing.T) {
	root := chdirToTempBackend(t)
	original := fetchMacroQuotes
	t.Cleanup(func() { fetchMacroQuotes = original })
	at := time.Date(2026, 9, 8, 20, 0, 0, 0, time.UTC)
	quotes := make(map[string]market.Quote)
	for _, row := range canonicalMacroTemplate().Series {
		quotes[row.Sym] = market.Quote{
			Price: 100, Pct: 1, Session: "closed", Source: "yahoo-chart",
			DataTime: at.Format(time.RFC3339), TimeGranularity: "second",
		}
	}
	fetchMacroQuotes = func([]string) quoteFetchResult {
		return quoteFetchResult{
			quotes: quotes, dataTime: at, dataTimeLabel: at.Format(time.RFC3339),
			timeGranularity: "second", source: "yahoo-chart",
		}
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/macro", nil)
	GetMacroData(c)
	if w.Code != http.StatusOK || w.Header().Get("X-Data-Stale") != "true" || !strings.HasPrefix(w.Header().Get("X-Data-Source"), "stale-snapshot:macro:") {
		t.Fatalf("status=%d headers=%v body=%s", w.Code, w.Header(), w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(root, "data", "macro.json")); err != nil {
		t.Fatalf("trusted macro snapshot was not persisted: %v", err)
	}
}

func TestMacroSnapshotLoaderSkipsUnpricedWorkingFile(t *testing.T) {
	root := chdirToTempBackend(t)
	mustWrite(t, filepath.Join(root, "data", "macro.json"), `{"series":[{"sym":"SPY","price":0}],"ts":1788600000000}`)
	mustWrite(t, filepath.Join(root, "data", "macro-verified.json"), `{"series":[{"sym":"SPY","name":"S&P 500","kind":"index","price":700,"pct":1}],"ts":1785484161411}`)

	snapshot, ok := loadMacroSnapshotFile()
	if !ok || len(snapshot.Series) != 1 || snapshot.Series[0].Price != 700 || snapshot.Ts != 1785484161411 {
		t.Fatalf("snapshot=%+v ok=%v", snapshot, ok)
	}
	var status systemDataFileStatus
	for _, file := range collectSystemDataFiles(root) {
		if file.ID == "macro" {
			status = file
			break
		}
	}
	if !status.Exists || status.Error != "" || !strings.HasSuffix(status.Path, "macro-verified.json") {
		t.Fatalf("status=%+v", status)
	}
	endpoint := endpointByPath(buildSystemEndpointStatuses(collectSystemDataFiles(root), nil, nil, nil), "/api/macro")
	if endpoint.Status != "stale" || !endpoint.Stale || !endpoint.Refreshable || endpoint.DataTime != "2026-07-31T07:49:21Z" {
		t.Fatalf("endpoint=%+v", endpoint)
	}
}

func TestBuildLiveMoversSnapshotSortsObservedUniverse(t *testing.T) {
	at := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	quotes := make(map[string]market.SnapshotQuote)
	for index := 0; index < 12; index++ {
		symbol := string(rune('A' + index))
		quotes[symbol] = market.SnapshotQuote{Price: float64(index + 1), Pct: float64(index - 6)}
	}
	snapshot, ok := buildLiveMoversSnapshot(quotes, market.PayloadStatus{DataTime: at, Session: "regular"})
	if !ok || snapshot.Session != "regular" || snapshot.Ts != at.UnixMilli() || len(snapshot.Gainers) != 6 || len(snapshot.Losers) != 6 {
		t.Fatalf("snapshot=%+v ok=%v", snapshot, ok)
	}
	if snapshot.Gainers[0].Pct != 5 || snapshot.Losers[0].Pct != -6 {
		t.Fatalf("unexpected ordering: gainers=%+v losers=%+v", snapshot.Gainers, snapshot.Losers)
	}
}

func TestMacroAndMoverStatusesRemainIndependentFromFreshMarket(t *testing.T) {
	root := chdirToTempBackend(t)
	mustWrite(t, root+"/data/macro.json", `{"series":[],"ts":1577970000000}`)
	mustWrite(t, root+"/data/premarket-movers.json", `{"gainers":[],"losers":[],"ts":1577970000000}`)
	status := market.PayloadStatus{Source: "tradingview", DataTime: time.Now().UTC(), Session: "regular"}
	endpoints := buildSystemEndpointStatuses(collectSystemDataFiles(root), &status, nil, nil)
	for _, endpoint := range endpoints {
		if endpoint.Endpoint == "/api/macro" && (endpoint.Source != "stale-snapshot:macro" || !endpoint.Stale) {
			t.Fatalf("macro endpoint=%+v", endpoint)
		}
		if endpoint.Endpoint == "/api/premarket-movers" && (endpoint.Source != "stale-snapshot:premarket-movers" || !endpoint.Stale) {
			t.Fatalf("movers endpoint=%+v", endpoint)
		}
	}
}
