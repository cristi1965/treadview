package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAMarketStaticRouteRequiresFullPerSymbolProvenance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC().Truncate(time.Second)
	oldest := now.Add(-10 * time.Second)

	tests := []struct {
		name       string
		payload    string
		wantStatus int
		wantStale  string
		wantTime   string
	}{
		{
			name: "complete provider timed universe",
			payload: fmt.Sprintf(`{"ts":%d,"count":2,"quotes":{"600519":{"price":1500,"source":"eastmoney-cn","dataTime":%q,"timeGranularity":"second"},"300750":{"price":300,"source":"eastmoney-cn","dataTime":%q,"timeGranularity":"second"}}}`,
				now.UnixMilli(), oldest.Format(time.RFC3339), now.Format(time.RFC3339)),
			wantStatus: http.StatusOK,
			wantStale:  "false",
			wantTime:   oldest.Format(time.RFC3339),
		},
		{
			name: "one row missing provenance",
			payload: fmt.Sprintf(`{"ts":%d,"count":2,"quotes":{"600519":{"price":1500,"source":"eastmoney-cn","dataTime":%q},"300750":{"price":300}}}`,
				now.UnixMilli(), now.Format(time.RFC3339)),
			wantStatus: http.StatusGone,
			wantStale:  "true",
		},
		{
			name: "declared count does not cover universe",
			payload: fmt.Sprintf(`{"ts":%d,"count":2,"quotes":{"600519":{"price":1500,"source":"eastmoney-cn","dataTime":%q}}}`,
				now.UnixMilli(), now.Format(time.RFC3339)),
			wantStatus: http.StatusGone,
			wantStale:  "true",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "data", "a-market.json"), []byte(test.payload), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Chdir(root)

			router := gin.New()
			router.GET("/data/*filepath", func(c *gin.Context) { serveDataFile(c, "") })
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/data/a-market.json", nil))

			if recorder.Code != test.wantStatus || recorder.Header().Get("X-Data-Stale") != test.wantStale {
				t.Fatalf("status=%d stale=%q headers=%v body=%s", recorder.Code, recorder.Header().Get("X-Data-Stale"), recorder.Header(), recorder.Body.String())
			}
			if test.wantTime != "" && recorder.Header().Get("X-Data-Time") != test.wantTime {
				t.Fatalf("data time=%q want=%q", recorder.Header().Get("X-Data-Time"), test.wantTime)
			}
		})
	}
}
