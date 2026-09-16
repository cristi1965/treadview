package dataflows

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"trading-agents/internal/models"
)

func TestFlashImportance(t *testing.T) {
	cases := []struct {
		title string
		want  int
	}{
		{"Fed holds rates steady as CPI cools", 3},
		{"FOMC minutes show split over September cut", 3},
		{"Trump weighs new tariffs on imported chips", 3},
		{"US nonfarm payrolls rise 142,000 in July", 3},
		{"Nvidia earnings beat as data center revenue soars", 2},
		{"Nasdaq closes at a record high", 2},
		{"Local bakery opens second branch downtown", 1},
	}
	for _, tc := range cases {
		if got := FlashImportance(tc.title); got != tc.want {
			t.Errorf("FlashImportance(%q) = %d, want %d", tc.title, got, tc.want)
		}
	}
}

// "fed" 不应该命中 "FedEx"，整词匹配是重要性判定的前提。
func TestFlashImportanceWordBoundary(t *testing.T) {
	if got := FlashImportance("FedEx cuts delivery forecast"); got == 3 {
		t.Errorf("FedEx should not be treated as Fed news, got importance %d", got)
	}
}

func TestFlashKind(t *testing.T) {
	cases := []struct {
		title, source, want string
	}{
		{"Fed signals patience on rate cuts", "Reuters", "macro"},
		{"Applied Materials beats on revenue, guides lower", "CNBC", "earnings"},
		{"Tesla expands Texas plant", "Bloomberg", "company"},
		{"Wall Street rally stalls", "AP", "market"},
		{"Something entirely unrelated happens", "Blog", "market"},
	}
	for _, tc := range cases {
		if got := FlashKind(tc.title, tc.source); got != tc.want {
			t.Errorf("FlashKind(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
}

// 没有翻译器（无 Key / 建不出 client）时，title 必须是干净的英文原标题，
// 不能出现旧版「关键词｜英文」拼接串，body 也不能塞「关键词：」。
func TestBuildFlashItemsFallsBackToEnglishTitle(t *testing.T) {
	items := BuildFlashItems([]NewsArticle{
		{Title: "Fed cuts rates by 25 basis points", Published: "Fri, 14 Aug 2026 11:00:00 +0000"},
	}, time.Now().UTC())
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	it := items[0]
	if it.Title != "Fed cuts rates by 25 basis points" {
		t.Fatalf("expected raw English headline as fallback title, got %q", it.Title)
	}
	if strings.ContainsAny(it.Title, "｜") || strings.Contains(it.Title, "（英文原文）") {
		t.Fatalf("title must not be a spliced keyword string: %q", it.Title)
	}
	if strings.Contains(it.Body, "关键词") || strings.Contains(it.Body, "宏观·") {
		t.Fatalf("body must not repeat keywords/labels: %q", it.Body)
	}

	// 有翻译器时，title 换成中文，缓存落盘。
	dir := t.TempDir()
	cache := newFlashTitleCache(filepath.Join(dir, "flash-title-cache.json"), 10)
	TranslateFlashTitles(context.Background(), items, stubFlashTranslator{"美联储降息 25 个基点"}, cache)
	if items[0].Title != "美联储降息 25 个基点" {
		t.Fatalf("expected translated title, got %q", items[0].Title)
	}
	if items[0].TitleEn != "Fed cuts rates by 25 basis points" {
		t.Fatalf("titleEn must stay English, got %q", items[0].TitleEn)
	}
}

type stubFlashTranslator struct{ zh string }

func (s stubFlashTranslator) TranslateTitles(_ context.Context, titles []string) ([]string, error) {
	out := make([]string, len(titles))
	for i := range out {
		out[i] = s.zh
	}
	return out, nil
}

type errFlashTranslator struct{ calls int }

func (e *errFlashTranslator) TranslateTitles(context.Context, []string) ([]string, error) {
	e.calls++
	return nil, errors.New("boom")
}

// 翻译失败（超时 / 无 Key / 报错）时保持英文原标题，不写缓存。
func TestTranslateFlashTitlesDegradesOnError(t *testing.T) {
	items := []models.FlashItem{{TitleEn: "Nvidia earnings top estimates", Title: "Nvidia earnings top estimates"}}
	cache := newFlashTitleCache(filepath.Join(t.TempDir(), "c.json"), 10)
	tr := &errFlashTranslator{}
	TranslateFlashTitles(context.Background(), items, tr, cache)
	if items[0].Title != "Nvidia earnings top estimates" {
		t.Fatalf("expected English fallback, got %q", items[0].Title)
	}
	if cache.Len() != 0 {
		t.Fatalf("failed translation must not populate cache, got %d", cache.Len())
	}

	// translator 为 nil（没有 API Key）同样降级，且不 panic。
	TranslateFlashTitles(context.Background(), items, nil, cache)
	if items[0].Title != "Nvidia earnings top estimates" {
		t.Fatalf("nil translator should keep English title, got %q", items[0].Title)
	}
}

// 缓存命中就不再请求 LLM，并且能跨进程从磁盘读回。
func TestFlashTitleCacheHitAndPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flash-title-cache.json")
	cache := newFlashTitleCache(path, 10)
	items := []models.FlashItem{{TitleEn: "Fed holds rates steady"}}
	TranslateFlashTitles(context.Background(), items, stubFlashTranslator{"美联储按兵不动"}, cache)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cache should be flushed to disk: %v", err)
	}

	reloaded := newFlashTitleCache(path, 10)
	zh, ok := reloaded.Get("Fed holds rates steady")
	if !ok || zh != "美联储按兵不动" {
		t.Fatalf("expected cache hit from disk, got %q ok=%v", zh, ok)
	}

	// 命中缓存时不应触碰 translator
	tr := &errFlashTranslator{}
	items2 := []models.FlashItem{{TitleEn: "Fed holds rates steady"}}
	TranslateFlashTitles(context.Background(), items2, tr, reloaded)
	if tr.calls != 0 {
		t.Fatalf("cached title should not call translator, calls=%d", tr.calls)
	}
	if items2[0].Title != "美联储按兵不动" {
		t.Fatalf("expected cached Chinese title, got %q", items2[0].Title)
	}
}

func TestTranslateFlashTitlesReadOnlyDoesNotPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flash-title-cache.json")
	cache := newFlashTitleCache(path, 10)
	items := []models.FlashItem{{TitleEn: "Fed holds rates steady"}}

	TranslateFlashTitlesReadOnly(context.Background(), items, stubFlashTranslator{"美联储按兵不动"}, cache)

	if items[0].Title != "美联储按兵不动" {
		t.Fatalf("expected translated title, got %q", items[0].Title)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("read-only translation persisted cache: %v", err)
	}
}

// 超过上限时按时间淘汰最旧的条目。
func TestFlashTitleCacheEviction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	cache := newFlashTitleCache(path, 3)
	for i := 0; i < 6; i++ {
		cache.Put("headline "+strconv.Itoa(i), "标题"+strconv.Itoa(i))
		time.Sleep(time.Millisecond)
	}
	if err := cache.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if got := cache.Len(); got != 3 {
		t.Fatalf("expected cache pruned to 3, got %d", got)
	}
	if _, ok := cache.Get("headline 5"); !ok {
		t.Error("newest entry should survive eviction")
	}
	if _, ok := cache.Get("headline 0"); ok {
		t.Error("oldest entry should be evicted")
	}
}

func TestFlashTickers(t *testing.T) {
	got := FlashTickers("Nvidia and AMD rally while Intel lags")
	want := map[string]bool{"NVDA": true, "AMD": true, "INTC": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want 3 tickers", got)
	}
	for _, tk := range got {
		if !want[tk] {
			t.Errorf("unexpected ticker %q in %v", tk, got)
		}
	}
	if tk := FlashTickers("Consumer confidence slips"); len(tk) != 0 {
		t.Errorf("expected no tickers, got %v", tk)
	}
}

func TestDedupeFlashItems(t *testing.T) {
	items := []models.FlashItem{
		{ID: "a", TitleEn: "Fed cuts rates!", Title: "美联储降息"},
		{ID: "b", TitleEn: "  fed   cuts rates  ", Title: "美联储降息"},
		{ID: "c", TitleEn: "Nvidia beats estimates", Title: "英伟达超预期"},
		{ID: "d", TitleEn: "", Title: ""},
	}
	out := DedupeFlashItems(items)
	if len(out) != 2 {
		t.Fatalf("expected 2 unique items, got %d (%+v)", len(out), out)
	}
	if out[0].ID != "a" || out[1].ID != "c" {
		t.Fatalf("dedupe should keep first occurrence, got %+v", out)
	}
}

func TestBuildFlashItemsSortedAndFilled(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	articles := []NewsArticle{
		{
			Title:       "Nvidia earnings top estimates - Reuters",
			Link:        "https://example.com/nvda",
			Published:   "Thu, 13 Aug 2026 20:00:00 +0000",
			Description: "<p>Data center revenue &amp; margins beat.</p>",
		},
		{
			Title:     "Fed cuts rates by 25 basis points",
			Link:      "https://example.com/fed",
			Published: "Fri, 14 Aug 2026 11:00:00 +0000",
		},
		{Title: "   ", Link: "https://example.com/blank"},
		{Title: "Fed cuts rates by 25 basis points", Link: "https://example.com/fed-dup"},
	}

	items := BuildFlashItems(articles, now)
	if len(items) != 2 {
		t.Fatalf("expected 2 items after dedupe/blank drop, got %d", len(items))
	}
	if items[0].Time < items[1].Time {
		t.Fatalf("items must be sorted newest first: %+v", items)
	}
	for _, it := range items {
		if it.Title == "" || it.TitleEn == "" {
			t.Fatalf("title/titleEn must not be empty: %+v", it)
		}
		if it.ID == "" || it.Kind == "" || it.Importance < 1 || it.Importance > 3 {
			t.Fatalf("malformed item: %+v", it)
		}
		if _, err := time.Parse(time.RFC3339, it.Time); err != nil {
			t.Fatalf("time %q is not RFC3339: %v", it.Time, err)
		}
	}

	nvda := items[1]
	if nvda.Source != "Reuters" {
		t.Errorf("expected source parsed from headline suffix, got %q", nvda.Source)
	}
	if strings.Contains(nvda.TitleEn, "Reuters") {
		t.Errorf("source suffix should be stripped from titleEn: %q", nvda.TitleEn)
	}
	if strings.Contains(nvda.BodyEn, "<p>") || !strings.Contains(nvda.BodyEn, "&") {
		t.Errorf("bodyEn should be HTML-stripped and unescaped: %q", nvda.BodyEn)
	}
}

func TestBuildFlashItemsCapsAtMax(t *testing.T) {
	articles := make([]NewsArticle, FlashMaxItems+40)
	base := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	for i := range articles {
		articles[i] = NewsArticle{
			Title:     "Market update number " + time.Duration(i).String(),
			Published: base.Add(time.Duration(i) * time.Minute).Format(time.RFC1123Z),
		}
	}
	if got := len(BuildFlashItems(articles, base)); got != FlashMaxItems {
		t.Fatalf("expected cap at %d, got %d", FlashMaxItems, got)
	}
}

func TestGetFlashUsesCache(t *testing.T) {
	c := NewFlashClient()
	c.cached = []models.FlashItem{
		{ID: "1", Time: "2026-08-14T12:00:00Z", Title: "一", Importance: 3, Kind: "macro"},
		{ID: "2", Time: "2026-08-14T11:00:00Z", Title: "二", Importance: 1, Kind: "market"},
	}
	c.cachedAt = time.Now()

	all, err := c.GetFlash(0)
	if err != nil || len(all) != 2 {
		t.Fatalf("cache hit should return 2 items, got %d (%v)", len(all), err)
	}
	one, _ := c.GetFlash(1)
	if len(one) != 1 || one[0].ID != "1" {
		t.Fatalf("limit=1 should return newest cached item, got %+v", one)
	}
	// 返回的必须是副本，改它不能污染缓存
	one[0].Title = "改了"
	if c.cached[0].Title != "一" {
		t.Fatal("GetFlash must return a copy, not the cache slice")
	}
}

func TestFlashBodyEnDropsHeadlineEcho(t *testing.T) {
	title := "Stock market today: Dow slips after record high"
	echo := `<a href="https://news.google.com/x">` + title + `</a>&nbsp;&nbsp;<font color="#6f6f6f">Yahoo Finance</font>`
	if got := flashBodyEn(echo, title); got != "" {
		t.Errorf("headline echo should be dropped, got %q", got)
	}
	real := "<p>The index fell 0.3% as consumer sentiment came in at 61.2, the weakest reading since March.</p>"
	got := flashBodyEn(real, title)
	if got == "" || strings.Contains(got, "<p>") {
		t.Errorf("real summary should be kept and stripped, got %q", got)
	}
	if flashBodyEn("", title) != "" {
		t.Error("empty description should stay empty")
	}
}

func TestParseFlashTimeFallback(t *testing.T) {
	fallback := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)
	if got := parseFlashTime("", fallback); !got.Equal(fallback) {
		t.Errorf("empty pubDate should use fallback, got %v", got)
	}
	if got := parseFlashTime("not a date", fallback); !got.Equal(fallback) {
		t.Errorf("unparsable pubDate should use fallback, got %v", got)
	}
	got := parseFlashTime("Thu, 13 Aug 2026 20:00:00 +0000", fallback)
	if got.UTC().Format(time.RFC3339) != "2026-08-13T20:00:00Z" {
		t.Errorf("RFC1123Z parse failed: %v", got)
	}
}
