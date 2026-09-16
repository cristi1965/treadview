package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestStaleETFResponsesPreserveKnownSnapshotTime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := `{"updated":"2020-01-02","aumUnit":"usd","sectors":[{"sector":"test","super":"宽基","n":1,"aum":1}],"etfs":[]}`
	if err := os.WriteFile(filepath.Join(root, "data", "etf-analyses.json"), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	h := &Handler{}
	router := gin.New()
	router.GET("/api/etf/sectors", h.GetETFSectors)
	router.GET("/api/etf/sectors/:id", h.GetETFSectorDetail)
	router.GET("/api/etf/search", h.SearchETFs)

	for _, target := range []string{"/api/etf/sectors?mode=historical", "/api/etf/sectors/test?mode=historical", "/api/etf/search?q=SPY&mode=historical"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", target, recorder.Code, recorder.Body.String())
		}
		if got := recorder.Header().Get("X-Data-Time"); got != "2020-01-02" {
			t.Fatalf("%s X-Data-Time=%q", target, got)
		}
		var body map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if recorder.Header().Get("X-Data-Stale") != "true" || recorder.Header().Get("X-Data-Refreshable") != "true" || body["dataMode"] != "historical" {
			t.Fatalf("%s headers=%v body=%+v", target, recorder.Header(), body)
		}
		diagnostics, ok := body["diagnostics"].(map[string]any)
		if !ok || diagnostics["maxAgeSeconds"] != float64(etfAnalysesMaxAge/time.Second) {
			t.Fatalf("%s diagnostics=%+v", target, diagnostics)
		}
	}
}

func TestCurrentETFModeRequiresFreshPromotedLastGood(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	updated := time.Now().UTC().Format("2006-01-02")
	payload := `{"updated":"` + updated + `","metadataAsOf":"2026-06-16","source":"yahoo-chart+local-etf-metadata","aumUnit":"usd","n":1,"scope":{"kind":"top-aum","requested":1,"completed":1,"complete":true},"sectors":[{"sector":"test","super":"宽基","n":1,"aum":1}],"etfs":[{"sym":"SPY","name":"SPY","sector":"test","kind":"宽基","aum":1,"expense":0.09,"ret1y":1,"ret5y":2,"mdd":-3}]}`
	if err := os.WriteFile(filepath.Join(root, "data", "etf-analyses-current.json"), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	status := `{"lastAttemptAt":"2026-09-10T01:00:00Z","nextScheduledAt":"2026-09-11T01:00:00Z","lastAttemptStatus":"complete","scopeRequested":12,"scopeCompleted":12}`
	if err := os.WriteFile(filepath.Join(root, "data", "etf-refresh-status.json"), []byte(status), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	router := gin.New()
	router.GET("/api/etf/sectors", (&Handler{}).GetETFSectors)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/etf/sectors?mode=current", nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("X-Data-Stale") == "true" {
		t.Fatalf("status=%d headers=%v body=%s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	diagnostics := body["diagnostics"].(map[string]any)
	if body["dataMode"] != "current" || diagnostics["lastAttemptStatus"] != "complete" || diagnostics["scopeCompleted"] != float64(12) {
		t.Fatalf("body=%+v", body)
	}
}

func TestHistoricalETFModeNeverReadsPromotedCurrent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data", "etf-history"), 0o755); err != nil {
		t.Fatal(err)
	}
	current := `{"updated":"2026-09-10","metadataAsOf":"2026-09-10","source":"current-source","aumUnit":"usd","n":1,"scope":{"kind":"top-aum","requested":1,"completed":1,"complete":true},"sectors":[{"sector":"current","super":"宽基","n":1,"aum":1}],"etfs":[{"sym":"CUR","name":"Current","sector":"current","kind":"宽基","aum":1,"expense":0.1,"ret1y":1,"ret5y":2,"mdd":-3}]}`
	historical := `{"updated":"2026-09-09","metadataAsOf":"2026-09-09","source":"historical-source","aumUnit":"usd","n":1,"scope":{"kind":"top-aum","requested":1,"completed":1,"complete":true},"sectors":[{"sector":"history","super":"宽基","n":1,"aum":1}],"etfs":[{"sym":"HIST","name":"Historical","sector":"history","kind":"宽基","aum":1,"expense":0.1,"ret1y":1,"ret5y":2,"mdd":-3}]}`
	if err := os.WriteFile(filepath.Join(root, "data", "etf-analyses-current.json"), []byte(current), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "etf-history", "2026-09-09.json"), []byte(historical), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	router := gin.New()
	router.GET("/api/etf/sectors", (&Handler{}).GetETFSectors)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/etf/sectors?mode=historical", nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("X-Data-Time") != "2026-09-09" || recorder.Header().Get("X-Data-Stale") != "true" {
		t.Fatalf("status=%d headers=%v body=%s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
	if source := recorder.Header().Get("X-Data-Source"); source == "" || strings.Contains(source, "etf-analyses-current.json") {
		t.Fatalf("historical mode used current source: %q", source)
	}
	var body struct {
		Updated string             `json:"updated"`
		ETFs    []etfAnalysisEntry `json:"etfs"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Updated != "2026-09-09" || len(body.ETFs) != 1 || body.ETFs[0].Sym != "HIST" {
		t.Fatalf("historical body drifted to current: %+v", body)
	}
}

func TestCurrentETFModeRejectsMissingDecisionMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	updated := time.Now().UTC().Format("2006-01-02")
	payload := `{"updated":"` + updated + `","metadataAsOf":"2026-06-16","source":"yahoo-chart+local-etf-metadata","aumUnit":"usd","n":1,"scope":{"kind":"top-aum","requested":1,"completed":1,"complete":true},"sectors":[{"sector":"美股宽基","super":"宽基","n":1,"aum":1}],"etfs":[{"sym":"SPYM","name":"State Street SPDR Portfolio S&P 500 ETF","sector":"美股宽基","kind":"宽基","aum":144091635000,"ret1y":16.23,"ret5y":70.28,"mdd":-25.38}]}`
	if err := os.WriteFile(filepath.Join(root, "data", "etf-analyses-current.json"), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	router := gin.New()
	router.GET("/api/etf/sectors", (&Handler{}).GetETFSectors)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/etf/sectors?mode=current", nil))
	if recorder.Code != http.StatusServiceUnavailable || recorder.Header().Get("X-Data-Stale") != "true" {
		t.Fatalf("missing expense must fail closed: status=%d headers=%v body=%s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
}

func TestCurrentETFModeRejectsIncompletePromotedScope(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	updated := time.Now().UTC().Format("2006-01-02")
	payload := `{"updated":"` + updated + `","metadataAsOf":"2026-06-16","source":"yahoo-chart+local-etf-metadata","aumUnit":"usd","n":1,"scope":{"kind":"top-aum","requested":1,"completed":0,"complete":false},"sectors":[{"sector":"test","super":"宽基","n":1,"aum":1}],"etfs":[]}`
	if err := os.WriteFile(filepath.Join(root, "data", "etf-analyses-current.json"), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	router := gin.New()
	router.GET("/api/etf/sectors", (&Handler{}).GetETFSectors)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/etf/sectors?mode=current", nil))
	if recorder.Code != http.StatusServiceUnavailable || recorder.Header().Get("X-Data-Stale") != "true" {
		t.Fatalf("status=%d headers=%v body=%s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
}

func TestCurrentETFModeDoesNotPromoteLegacyOrFutureSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name, file, updated string
	}{
		{name: "legacy is historical only", file: "etf-analyses.json", updated: time.Now().UTC().Format("2006-01-02")},
		{name: "future current is rejected", file: "etf-analyses-current.json", updated: time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02")},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
				t.Fatal(err)
			}
			payload := `{"updated":"` + test.updated + `","aumUnit":"usd","sectors":[{"sector":"test","super":"宽基","n":1,"aum":1}],"etfs":[]}`
			if err := os.WriteFile(filepath.Join(root, "data", test.file), []byte(payload), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Chdir(root)
			router := gin.New()
			router.GET("/api/etf/sectors", (&Handler{}).GetETFSectors)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/etf/sectors?mode=current", nil))
			if recorder.Code != http.StatusServiceUnavailable || recorder.Header().Get("X-Data-Stale") != "true" {
				t.Fatalf("status=%d headers=%v body=%s", recorder.Code, recorder.Header(), recorder.Body.String())
			}
		})
	}
}

func TestETFModeRejectsAmbiguousLegacyValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/etf/sectors", (&Handler{}).GetETFSectors)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/etf/sectors?mode=best", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
