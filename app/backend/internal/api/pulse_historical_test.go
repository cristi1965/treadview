package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestPulseHistoricalModePreservesStaleSnapshotWithoutLiveOverlay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := `{"generated_at":"2020-01-02T03:04:05Z","source":"captured-pulse","count":1,"companies":[{"ticker":"TEST","name":"Test","layer":"L1","region":"US","marketCapB":1,"heat":50,"industries":["AI"],"livePrice":12.34,"pct":1.2}]}`
	if err := os.WriteFile(filepath.Join(root, "data", "pulse-companies.json"), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	pulseMu.Lock()
	pulseData = pulseFile{}
	pulseLoadedAt = time.Time{}
	pulsePath = ""
	pulseMu.Unlock()

	router := gin.New()
	router.GET("/api/pulse", GetPulseData)

	strict := httptest.NewRecorder()
	router.ServeHTTP(strict, httptest.NewRequest(http.MethodGet, "/api/pulse?region=US", nil))
	if strict.Code != http.StatusServiceUnavailable || strict.Header().Get("X-Data-Time") != "2020-01-02T03:04:05Z" {
		t.Fatalf("strict status=%d time=%q body=%s", strict.Code, strict.Header().Get("X-Data-Time"), strict.Body.String())
	}

	historical := httptest.NewRecorder()
	router.ServeHTTP(historical, httptest.NewRequest(http.MethodGet, "/api/pulse?mode=best&region=US", nil))
	if historical.Code != http.StatusOK || historical.Header().Get("X-Data-Stale") != "true" {
		t.Fatalf("historical status=%d body=%s", historical.Code, historical.Body.String())
	}
	var body struct {
		DataMode    string         `json:"dataMode"`
		LiveOverlay int            `json:"live_overlay"`
		Companies   []pulseCompany `json:"companies"`
	}
	if err := json.Unmarshal(historical.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.DataMode != "historical" || body.LiveOverlay != 0 || len(body.Companies) != 1 || body.Companies[0].LivePrice == nil || *body.Companies[0].LivePrice != 12.34 {
		t.Fatalf("unexpected historical body: %+v", body)
	}
}
