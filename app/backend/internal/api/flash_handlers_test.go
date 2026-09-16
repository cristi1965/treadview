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

	"trading-agents/internal/models"
)

func TestLatestFlashObservationUsesNewestItemTime(t *testing.T) {
	items := []models.FlashItem{
		{Time: "2026-09-15T10:00:00Z"},
		{Time: "2026-09-15T11:30:00Z"},
		{Time: "invalid"},
	}
	got, ok := latestFlashObservation(items)
	if !ok || got.Format(time.RFC3339) != "2026-09-15T11:30:00Z" {
		t.Fatalf("latest observation=%s ok=%v", got.Format(time.RFC3339), ok)
	}
}

const flashFixture = `{
  "updatedAt": "2026-08-14T15:20:00Z",
  "items": [
    {"id":"a","time":"2026-08-14T14:00:00Z","title":"重磅宏观","titleEn":"Critical macro","importance":3,"kind":"macro"},
    {"id":"b","time":"2026-08-14T13:00:00Z","title":"财报关注","titleEn":"Notable earnings","importance":2,"kind":"earnings"},
    {"id":"c","time":"2026-08-14T15:00:00Z","title":"普通大盘","titleEn":"Routine market","importance":1,"kind":"market"}
  ]
}`

// chdirWithFlashFixture 切到临时目录并放好 data/flash-live.json，模拟静态降级路径。
func chdirWithFlashFixture(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data", "flash-live.json"), []byte(flashFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

func TestClampFlashLimit(t *testing.T) {
	cases := map[string]int{
		"":       flashDefaultLimit,
		"abc":    flashDefaultLimit,
		"0":      flashDefaultLimit,
		"-5":     flashDefaultLimit,
		"1":      1,
		"60":     60,
		"200":    200,
		"201":    flashMaxLimit,
		"100000": flashMaxLimit,
	}
	for in, want := range cases {
		if got := clampFlashLimit(in); got != want {
			t.Errorf("clampFlashLimit(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseFlashImportance(t *testing.T) {
	cases := map[string]int{
		"": 1, "all": 1, "1": 1, "nonsense": 1,
		"important": 2, "2": 2,
		"critical": 3, "top": 3, "3": 3, "RED": 3,
	}
	for in, want := range cases {
		if got := parseFlashImportance(in); got != want {
			t.Errorf("parseFlashImportance(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseFlashKinds(t *testing.T) {
	if parseFlashKinds("") != nil || parseFlashKinds("all") != nil || parseFlashKinds(" , ") != nil {
		t.Fatal("empty/all/blank kind should mean no filter")
	}
	kinds := parseFlashKinds("macro, Earnings")
	if len(kinds) != 2 || !kinds["macro"] || !kinds["earnings"] {
		t.Fatalf("unexpected kinds: %+v", kinds)
	}
}

func TestFilterFlashItems(t *testing.T) {
	items := []models.FlashItem{
		{ID: "a", Importance: 3, Kind: "macro"},
		{ID: "b", Importance: 2, Kind: "earnings"},
		{ID: "c", Importance: 1, Kind: "market"},
	}
	if got := filterFlashItems(items, 1, nil); len(got) != 3 {
		t.Fatalf("all: expected 3, got %d", len(got))
	}
	if got := filterFlashItems(items, 2, nil); len(got) != 2 {
		t.Fatalf("important: expected 2, got %d", len(got))
	}
	got := filterFlashItems(items, 3, nil)
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("critical: expected only a, got %+v", got)
	}
	byKind := filterFlashItems(items, 1, map[string]bool{"earnings": true})
	if len(byKind) != 1 || byKind[0].ID != "b" {
		t.Fatalf("kind filter failed: %+v", byKind)
	}
	if got := filterFlashItems(items, 3, map[string]bool{"market": true}); len(got) != 0 {
		t.Fatalf("combined filter should be empty, got %+v", got)
	}
}

func TestLoadFlashFromJSONStaticFallback(t *testing.T) {
	chdirWithFlashFixture(t)

	items, updated, ok := loadFlashFromJSON()
	if !ok {
		t.Fatal("expected static file to load")
	}
	if updated != "2026-08-14T15:20:00Z" {
		t.Errorf("updatedAt = %q", updated)
	}
	if len(items) != 3 || items[0].ID != "c" {
		t.Fatalf("items should be sorted newest first, got %+v", items)
	}
}

func TestLoadFlashFromJSONMissing(t *testing.T) {
	old, _ := os.Getwd()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	if _, _, ok := loadFlashFromJSON(); ok {
		t.Fatal("expected miss when no static file exists")
	}
}

// 离线模式（STOCKGOD_FLASH_OFFLINE=true）下走静态降级，验证参数与字段完整性。
func TestGetFlashHandlerOfflineFallback(t *testing.T) {
	chdirWithFlashFixture(t)
	t.Setenv("STOCKGOD_FLASH_OFFLINE", "true")
	gin.SetMode(gin.TestMode)

	call := func(query string) models.FlashResponse {
		t.Helper()
		r := gin.New()
		r.GET("/api/flash", GetFlash)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/flash"+query, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		if src := w.Header().Get("X-Data-Source"); src != "stale-snapshot:flash-live" {
			t.Fatalf("X-Data-Source = %q, want stale-snapshot:flash-live", src)
		}
		if stale := w.Header().Get("X-Data-Stale"); stale != "true" {
			t.Fatalf("X-Data-Stale = %q, want true", stale)
		}
		var resp models.FlashResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		return resp
	}

	all := call("")
	if len(all.Items) != 3 || all.UpdatedAt == "" {
		t.Fatalf("default query should return 3 items, got %+v", all)
	}
	for _, it := range all.Items {
		if it.Title == "" || it.TitleEn == "" {
			t.Fatalf("title/titleEn must be filled: %+v", it)
		}
	}
	if got := call("?limit=1"); len(got.Items) != 1 || got.Items[0].ID != "c" {
		t.Fatalf("limit=1 should return newest item, got %+v", got.Items)
	}
	if got := call("?importance=important"); len(got.Items) != 2 {
		t.Fatalf("importance=important should return 2 items, got %d", len(got.Items))
	}
	if got := call("?kind=macro"); len(got.Items) != 1 || got.Items[0].Kind != "macro" {
		t.Fatalf("kind=macro failed: %+v", got.Items)
	}
	if got := call("?limit=99999"); len(got.Items) != 3 {
		t.Fatalf("oversized limit should clamp, got %d", len(got.Items))
	}
}

func TestCalendarImportanceBackCompat(t *testing.T) {
	if got := calendarImportance(models.MarketEvent{IsImportant: true}); got != 3 {
		t.Errorf("isImportant=true should map to 3, got %d", got)
	}
	if got := calendarImportance(models.MarketEvent{}); got != 1 {
		t.Errorf("isImportant=false should map to 1, got %d", got)
	}
	if got := calendarImportance(models.MarketEvent{Importance: 2, IsImportant: true}); got != 2 {
		t.Errorf("explicit importance should win, got %d", got)
	}
}
