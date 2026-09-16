package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/dataflows"
	"trading-agents/internal/models"
)

func TestGetReportsRejectsInvalidPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, query := range []string{"limit=-1", "limit=0", "limit=101", "limit=abc", "offset=-1", "offset=abc", "offset=1000001"} {
		router := gin.New()
		router.GET("/api/reports", (&Handler{}).GetReports)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/reports?"+query, nil))
		if w.Code != http.StatusBadRequest {
			t.Errorf("query=%s status=%d body=%s", query, w.Code, w.Body.String())
		}
	}
}

func TestReportMoverClaimsHideNonPositivePrices(t *testing.T) {
	reports := []models.Report{{
		Summary: "上涨 NCL 300.00% @ 0.00；OK 2.00% @ 12.50；NEG -9.00% @ -1.00。",
		Content: "### 涨幅居前\nNCL 300.00% @ 0.00；OK 2.00% @ 12.50",
	}}
	sanitizeInvalidReportMoverClaims(reports)
	if strings.Contains(reports[0].Summary, "NCL 300.00%") || strings.Contains(reports[0].Summary, "NEG -9.00%") || !strings.Contains(reports[0].Summary, "价格 <= 0") {
		t.Fatalf("invalid mover claims were not hidden: %+v", reports[0])
	}
	if !strings.Contains(reports[0].Summary, "OK 2.00% @ 12.50") {
		t.Fatalf("valid mover claim changed: %+v", reports[0])
	}
}

func TestGetReportsIsReadOnlyAndPropagatesStaleReportTime(t *testing.T) {
	backendRoot := chdirToTempBackend(t)
	reportPath := filepath.Join(backendRoot, "data", "reports-live.json")
	mustWrite(t, reportPath, `[{"id":"old","type":"postmarket","title":"实时行情自动生成","date":"2020-01-02","time":"16:00 ET","summary":"实时行情快照","content":"当前自建行情源自动生成；TradingView/Yahoo/live snapshot","publishedAt":"2020-01-02T16:00:00-05:00"}]`)
	beforeHash, beforeMod := fileFingerprint(t, reportPath)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/reports", (&Handler{}).GetReports)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/reports", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-Data-Time"); got != "2020-01-02T21:00:00Z" {
		t.Fatalf("data time=%q", got)
	}
	if got := w.Header().Get("X-Data-Stale"); got != "true" {
		t.Fatalf("stale=%q", got)
	}
	if strings.Contains(w.Body.String(), "实时行情") || strings.Contains(w.Body.String(), "live snapshot") {
		t.Fatalf("stale report still claims live data: %s", w.Body.String())
	}
	afterHash, afterMod := fileFingerprint(t, reportPath)
	if beforeHash != afterHash || !beforeMod.Equal(afterMod) {
		t.Fatalf("GET mutated report snapshot: hash %s -> %s, mtime %s -> %s", beforeHash, afterHash, beforeMod, afterMod)
	}
}

func TestGetMarketCalendarUsesSnapshotTimeAndCoverage(t *testing.T) {
	backendRoot := chdirToTempBackend(t)
	calendarPath := filepath.Join(backendRoot, "data", "calendar-events.json")
	mustWrite(t, calendarPath, `[{"id":"event-1","date":"2099-09-30","title":"FOMC","type":"macro"}]`)
	snapshotTime := time.Date(2020, 9, 1, 8, 0, 0, 0, time.UTC)
	if err := os.Chtimes(calendarPath, snapshotTime, snapshotTime); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/market/calendar", (&Handler{}).GetMarketCalendar)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/market/calendar", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-Data-Time"); got != "unknown" {
		t.Fatalf("calendar without provider observation time must be unknown, got=%q", got)
	}
	if got := w.Header().Get("X-Data-Refreshed-At"); got != snapshotTime.Format(time.RFC3339) {
		t.Fatalf("snapshot file time belongs in refreshedAt, got=%q", got)
	}
	if got := w.Header().Get("X-Data-Period"); got != "2099-09-30/2099-09-30" {
		t.Fatalf("data period=%q", got)
	}
	if got := w.Header().Get("X-Data-Stale"); got != "true" {
		t.Fatalf("old calendar snapshot stale=%q", got)
	}
	if got := w.Header().Get("X-Data-Source"); !strings.Contains(got, "calendar-events.json") {
		t.Fatalf("source=%q", got)
	}
}

func TestGetPanelSummaryIsReadOnlyAndUsesOldestInputTime(t *testing.T) {
	backendRoot := chdirToTempBackend(t)
	projectRoot := filepath.Dir(backendRoot)
	panelPath := filepath.Join(projectRoot, "frontend", "public", "data", "us-panel-summary.json")
	universePath := filepath.Join(projectRoot, "frontend", "public", "data", "us-stocks.json")
	mustWrite(t, panelPath, `{"generated_at":"2026-09-06T10:00:00Z","source":"local-heuristic-v2-calibrated","method_version":"v2","count":1,"stocks":{"RKLB":{"sc":[1,2,3,4,5],"div":4}}}`)
	mustWrite(t, universePath, `{"generated_at":"2020-01-02T20:00:00Z","count":1,"stocks":[{"sym":"RKLB","price":10,"pct":1,"mcapB":1,"vol":100}]}`)
	beforeHash, beforeMod := fileFingerprint(t, panelPath)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/panel-summary", GetPanelSummary)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/panel-summary?sym=RKLB", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-Data-Time"); got != "2020-01-02T20:00:00Z" {
		t.Fatalf("data time=%q", got)
	}
	if got := w.Header().Get("X-Data-Stale"); got != "true" {
		t.Fatalf("stale=%q", got)
	}
	if got := w.Header().Get("X-Data-Source"); !strings.Contains(got, "us-stocks") {
		t.Fatalf("source does not identify critical input: %q", got)
	}
	afterHash, afterMod := fileFingerprint(t, panelPath)
	if beforeHash != afterHash || !beforeMod.Equal(afterMod) {
		t.Fatalf("GET mutated panel snapshot: hash %s -> %s, mtime %s -> %s", beforeHash, afterHash, beforeMod, afterMod)
	}
}

func TestGetPanelSummaryDoesNotGenerateMissingSnapshot(t *testing.T) {
	backendRoot := chdirToTempBackend(t)
	projectRoot := filepath.Dir(backendRoot)
	mustWrite(t, filepath.Join(projectRoot, "frontend", "public", "data", "us-stocks.json"), `{"generated_at":"2026-09-06T10:00:00Z","count":1,"stocks":[{"sym":"RKLB","price":10}]}`)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/panel-summary", GetPanelSummary)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/panel-summary", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	for _, path := range []string{
		filepath.Join(projectRoot, "frontend", "public", "data", "us-panel-summary.json"),
		filepath.Join(backendRoot, "data", "us-panel-summary.json"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("GET generated %s", path)
		}
	}
}

func TestFundamentalsFreshnessTreatsUnknownPeriodsAsStale(t *testing.T) {
	fetchedAt := time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)
	meta := fundamentalsFreshness(&dataflows.StockMetrics{
		Source: "eastmoney+nasdaq", FiscalPeriod: "unknown", AsOf: "unknown", FetchedAt: fetchedAt.Format(time.RFC3339),
	}, fetchedAt)
	if !meta.Stale || meta.DataTime != "unknown" {
		t.Fatalf("meta=%+v", meta)
	}
	if meta.RefreshedAt != fetchedAt.Format(time.RFC3339) {
		t.Fatalf("fetch time must only be refreshedAt: %+v", meta)
	}
}

func TestFundamentalsFreshnessUsesSECFilingPeriodEnd(t *testing.T) {
	meta := fundamentalsFreshness(&dataflows.StockMetrics{
		Source: "sec-edgar-companyfacts", FiscalPeriod: "FY2026-Q3", AsOf: "2026-06-27", FilingDate: "2026-08-01",
		Accession: "0000320193-26-000079", SourceURL: "https://www.sec.gov/Archives/edgar/data/320193/000032019326000079/filing-index.html",
	}, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	if meta.DataTime != "2026-06-27T00:00:00Z" {
		t.Fatalf("dataTime=%q", meta.DataTime)
	}
}

func TestMacroAndMoversGETsAreReadOnlyStaleSnapshots(t *testing.T) {
	backendRoot := chdirToTempBackend(t)
	macroPath := filepath.Join(backendRoot, "data", "macro.json")
	moversPath := filepath.Join(backendRoot, "data", "premarket-movers.json")
	mustWrite(t, macroPath, `{"series":[{"sym":"SPY","price":100,"pct":1}],"ts":1577970000000}`)
	mustWrite(t, moversPath, `{"session":"regular","label":"snapshot","gainers":[{"sym":"RKLB","price":10,"pct":1}],"losers":[],"ts":1577970000000}`)
	macroHash, macroMod := fileFingerprint(t, macroPath)
	moversHash, moversMod := fileFingerprint(t, moversPath)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/macro", GetMacroData)
	router.GET("/api/premarket-movers", GetPremarketMovers)
	for _, endpoint := range []string{"/api/macro?mode=snapshot", "/api/premarket-movers?mode=snapshot"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, endpoint, nil))
		if w.Code != http.StatusOK || w.Header().Get("X-Data-Stale") != "true" || w.Header().Get("X-Data-Time") != "2020-01-02T13:00:00Z" {
			t.Fatalf("%s status=%d headers=%v body=%s", endpoint, w.Code, w.Header(), w.Body.String())
		}
	}

	afterMacroHash, afterMacroMod := fileFingerprint(t, macroPath)
	afterMoversHash, afterMoversMod := fileFingerprint(t, moversPath)
	if macroHash != afterMacroHash || !macroMod.Equal(afterMacroMod) || moversHash != afterMoversHash || !moversMod.Equal(afterMoversMod) {
		t.Fatal("GET mutated macro or movers snapshot")
	}
}

func TestGetNotesArticleDoesNotCreateCacheDirectory(t *testing.T) {
	backendRoot := chdirToTempBackend(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/notes/art", (&Handler{}).GetNotesArticle)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/notes/art?id=missing", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	for _, path := range []string{
		filepath.Join(backendRoot, "data", "notes-cache"),
		filepath.Join(backendRoot, "app", "backend", "data", "notes-cache"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("GET created notes cache directory %s", path)
		}
	}
}

func TestNotesFreshnessUsesStoredContentTime(t *testing.T) {
	backendRoot := chdirToTempBackend(t)
	dataDir := filepath.Join(backendRoot, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tocPath := filepath.Join(dataDir, "notes-toc.json")
	articlePath := filepath.Join(dataDir, "notes-articles.json")
	mustWrite(t, tocPath, `{"name":"Notes","tagline":"Local","sections":[],"updatedAt":1704067200000}`)
	mustWrite(t, articlePath, `{"intro":"stored article"}`)
	articleTime := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(articlePath, articleTime, articleTime); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.GET("/api/notes/toc", (&Handler{}).GetNotesTOC)
	router.GET("/api/notes/art", (&Handler{}).GetNotesArticle)
	router.GET("/api/notes/search", (&Handler{}).GetNotesSearch)
	tocResponse := httptest.NewRecorder()
	router.ServeHTTP(tocResponse, httptest.NewRequest(http.MethodGet, "/api/notes/toc", nil))
	articleResponse := httptest.NewRecorder()
	router.ServeHTTP(articleResponse, httptest.NewRequest(http.MethodGet, "/api/notes/art?id=intro", nil))
	searchResponse := httptest.NewRecorder()
	router.ServeHTTP(searchResponse, httptest.NewRequest(http.MethodGet, "/api/notes/search?q=stored", nil))

	if tocResponse.Code != http.StatusOK || tocResponse.Header().Get("X-Data-Time") != "2024-01-01T00:00:00Z" {
		t.Fatalf("toc freshness must come from stored content: status=%d dataTime=%q body=%s", tocResponse.Code, tocResponse.Header().Get("X-Data-Time"), tocResponse.Body.String())
	}
	if articleResponse.Code != http.StatusOK || articleResponse.Header().Get("X-Data-Time") != articleTime.Format(time.RFC3339) {
		t.Fatalf("article freshness must come from file mtime: status=%d dataTime=%q body=%s", articleResponse.Code, articleResponse.Header().Get("X-Data-Time"), articleResponse.Body.String())
	}
	if searchResponse.Code != http.StatusOK || !strings.Contains(searchResponse.Body.String(), `"id":"intro"`) || searchResponse.Header().Get("X-Data-Time") != articleTime.Format(time.RFC3339) {
		t.Fatalf("body search must return matching article with content time: status=%d dataTime=%q body=%s", searchResponse.Code, searchResponse.Header().Get("X-Data-Time"), searchResponse.Body.String())
	}
}

func TestRefreshMarketDataRejectsGETBeforeStartingRefresh(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/market/refresh", RefreshMarketData)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/market/refresh", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestMarketRefreshStatusDoesNotReportPartialSuccessAsOK(t *testing.T) {
	upstreamErr := fmt.Errorf("upstream unavailable")
	for _, tc := range []struct {
		name       string
		usErr      error
		cnErr      error
		wantStatus string
		wantStale  bool
	}{
		{name: "all successful", wantStatus: "ok"},
		{name: "US failed", usErr: upstreamErr, wantStatus: "partial", wantStale: true},
		{name: "CN failed", cnErr: upstreamErr, wantStatus: "partial", wantStale: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := marketRefreshStatus(tc.usErr, tc.cnErr); got != tc.wantStatus {
				t.Fatalf("status=%q want=%q", got, tc.wantStatus)
			}
			if got := marketRefreshStaleReason(tc.usErr, tc.cnErr) != ""; got != tc.wantStale {
				t.Fatalf("stale reason present=%v want=%v", got, tc.wantStale)
			}
		})
	}
}

func chdirToTempBackend(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	backendRoot := filepath.Join(root, "backend")
	mustWrite(t, filepath.Join(backendRoot, "go.mod"), "module read-only-test\n")
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(backendRoot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	return backendRoot
}

func fileFingerprint(t *testing.T, path string) (string, time.Time) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), info.ModTime()
}

func decodeJSONBody(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatal(err)
	}
}
