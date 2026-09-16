package dataflows

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"html"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"trading-agents/internal/models"
)

// 7×24 快讯流（金十风格）的自建聚合层。
//
// 数据来源只用现成的公开 RSS（Google News / Yahoo Finance），不反代 stockgod.xyz。
// 中文标题由 LLM 在聚合阶段批量翻译（带磁盘缓存）；没有 Key / 失败 / 超时时
// 直接用英文原标题，不做关键词拼接。

const (
	flashCacheTTL    = 90 * time.Second
	flashHTTPTimeout = 8 * time.Second
	// FlashMaxItems 是聚合层保留的最大条数，接口层的 limit 不会超过它。
	FlashMaxItems = 200
)

// flashQueries 覆盖宏观、大盘与热门个股三条线。
var flashQueries = []string{
	"Federal Reserve interest rate decision",
	"US inflation CPI PCE data",
	"stock market today S&P 500 Nasdaq",
	"quarterly earnings report beats misses",
	"Nvidia OR Apple OR Tesla OR Microsoft stock",
	"China stocks A-shares Shanghai composite",
}

// FlashClient 聚合快讯并带 90 秒内存缓存（并发安全）。
type FlashClient struct {
	news       *NewsClient
	translator FlashTitleTranslator
	titleCache *flashTitleCache

	mu       sync.Mutex
	cached   []models.FlashItem
	cachedAt time.Time
}

// NewFlashClient 创建带 8s 超时的快讯聚合器（不含翻译器，测试用）。
func NewFlashClient() *FlashClient {
	return &FlashClient{
		news:       &NewsClient{httpClient: &http.Client{Timeout: flashHTTPTimeout}},
		titleCache: newFlashTitleCache(DefaultFlashTitleCachePath(), flashTitleCacheMax),
	}
}

var (
	defaultFlashOnce   sync.Once
	defaultFlashClient *FlashClient
)

// DefaultFlashClient 返回进程级共享实例（缓存才有意义）。
func DefaultFlashClient() *FlashClient {
	defaultFlashOnce.Do(func() {
		defaultFlashClient = NewFlashClient()
		defaultFlashClient.translator = DefaultFlashTitleTranslator()
	})
	return defaultFlashClient
}

// GetFlash 返回按时间倒序的快讯。limit<=0 时返回全部缓存内容。
//
// 缓存命中（90s 内）直接返回；抓取失败但仍有旧缓存时返回旧缓存；
// 既抓不到也没有缓存时返回 nil，由接口层降级到静态文件。
func (c *FlashClient) GetFlash(limit int) ([]models.FlashItem, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.cached) > 0 && time.Since(c.cachedAt) < flashCacheTTL {
		return limitFlashItems(c.cached, limit), nil
	}

	articles, err := c.news.GetGlobalNews(flashQueries, FlashMaxItems*2)
	items := BuildFlashItems(articles, time.Now().UTC())
	if len(items) > 0 {
		// 翻译放在聚合阶段：结果进 90 秒缓存，接口层不会每次都等 LLM。
		TranslateFlashTitlesReadOnly(context.Background(), items, c.translator, c.titleCache)
	}
	if len(items) == 0 {
		if len(c.cached) > 0 {
			return limitFlashItems(c.cached, limit), nil
		}
		return nil, err
	}

	c.cached = items
	c.cachedAt = time.Now()
	return limitFlashItems(items, limit), nil
}

func limitFlashItems(items []models.FlashItem, limit int) []models.FlashItem {
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	out := make([]models.FlashItem, limit)
	copy(out, items[:limit])
	return out
}

// BuildFlashItems 把英文 RSS 条目映射成快讯：翻译标题、判定重要性/类型、去重、按时间倒序。
func BuildFlashItems(articles []NewsArticle, now time.Time) []models.FlashItem {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	items := make([]models.FlashItem, 0, len(articles))
	for _, a := range articles {
		titleEn := strings.TrimSpace(html.UnescapeString(a.Title))
		if titleEn == "" {
			continue
		}
		// Google News 标题常带 " - Reuters" 尾巴，切出来当来源更干净。
		titleEn, sourceFromTitle := splitFlashSource(titleEn)
		source := strings.TrimSpace(a.Source)
		if sourceFromTitle != "" {
			source = sourceFromTitle
		}

		bodyEn := flashBodyEn(a.Description, titleEn)
		importance := FlashImportance(titleEn)
		kind := FlashKind(titleEn, source)

		items = append(items, models.FlashItem{
			ID:      flashID(titleEn),
			Time:    parseFlashTime(a.Published, now).UTC().Format(time.RFC3339),
			Title:   titleEn, // 默认英文原标题，翻译成功后由 TranslateFlashTitles 覆盖
			TitleEn: titleEn,
			// 没有中文摘要就留空：kind / importance 已是结构化字段，
			// 前端自己渲染标签和圆点，不需要在正文里重复一遍。
			Body:       "",
			BodyEn:     bodyEn,
			Importance: importance,
			Kind:       kind,
			Tickers:    FlashTickers(titleEn),
			Source:     source,
			Link:       strings.TrimSpace(a.Link),
		})
	}

	items = DedupeFlashItems(items)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Time != items[j].Time {
			return items[i].Time > items[j].Time
		}
		return items[i].ID < items[j].ID
	})
	if len(items) > FlashMaxItems {
		items = items[:FlashMaxItems]
	}
	return items
}

// DedupeFlashItems 按归一化后的英文标题去重，保留先出现的一条。
func DedupeFlashItems(items []models.FlashItem) []models.FlashItem {
	seen := make(map[string]bool, len(items))
	out := make([]models.FlashItem, 0, len(items))
	for _, it := range items {
		key := normalizeFlashTitle(it.TitleEn)
		if key == "" {
			key = normalizeFlashTitle(it.Title)
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, it)
	}
	return out
}

// --- 重要性 / 类型 / 标的 ---

// 命中即 3 星（重磅红）：货币政策、通胀就业主数据、地缘与关税。
var flashCriticalWords = []string{
	"fed", "federal reserve", "fomc", "powell", "rate decision", "rate cut", "rate hike",
	"cpi", "pce", "nfp", "nonfarm", "payrolls", "jobs report", "inflation",
	"tariff", "trade war", "war", "recession", "shutdown", "debt ceiling",
}

// 命中即 2 星（关注）：财报、大科技、指数级/大宗商品词。
var flashNotableWords = []string{
	"earnings", "results", "guidance", "revenue", "profit", "buyback", "dividend",
	"nvidia", "apple", "microsoft", "amazon", "alphabet", "google", "meta", "tesla",
	"broadcom", "amd", "intel", "netflix", "openai", "tsmc", "palantir",
	"sp500", "nasdaq", "dow", "russell", "wall street", "stocks", "futures",
	"treasury", "yield", "yields", "oil", "opec", "gold", "bitcoin",
	"gdp", "jobless claims", "retail sales", "ism", "pmi", "ipo",
	"merger", "acquisition", "upgrade", "downgrade", "layoffs",
}

// FlashImportance 返回 1 普通 / 2 关注 / 3 重磅。
func FlashImportance(titleEn string) int {
	padded := padFlashText(titleEn)
	if containsAnyWord(padded, flashCriticalWords) {
		return 3
	}
	if containsAnyWord(padded, flashNotableWords) {
		return 2
	}
	return 1
}

var (
	flashMacroWords = []string{
		"fed", "federal reserve", "fomc", "powell", "rate cut", "rate hike", "rate decision",
		"interest rate", "cpi", "pce", "ppi", "inflation", "gdp", "payrolls", "nonfarm", "nfp",
		"jobless claims", "unemployment", "jobs report", "retail sales", "ism", "pmi",
		"tariff", "trade war", "treasury", "yield", "yields", "dollar", "opec",
		"ecb", "boj", "bank of japan", "debt ceiling", "shutdown", "recession",
	}
	flashEarningsWords = []string{
		"earnings", "results", "guidance", "eps", "revenue", "profit", "quarter", "quarterly", "outlook",
	}
	flashMarketWords = []string{
		"sp500", "nasdaq", "dow", "russell", "wall street", "stocks", "futures", "index",
		"rally", "selloff", "record high", "volatility", "vix", "bitcoin", "gold",
	}
)

// FlashKind 判定 macro | earnings | company | market。
// 宏观词优先（Fed 财报周也归宏观），其次财报词，再看是否点名了个股，兜底 market。
func FlashKind(titleEn, source string) string {
	padded := padFlashText(titleEn + " " + source)
	switch {
	case containsAnyWord(padded, flashMacroWords):
		return "macro"
	case containsAnyWord(padded, flashEarningsWords):
		return "earnings"
	case len(FlashTickers(titleEn)) > 0:
		return "company"
	case containsAnyWord(padded, flashMarketWords):
		return "market"
	default:
		return "market"
	}
}

// flashCompanyTickers 把公司名/别名映射到代码，用于 tickers 与中文标题。
var flashCompanyTickers = []struct{ name, ticker string }{
	{"nvidia", "NVDA"}, {"apple", "AAPL"}, {"microsoft", "MSFT"}, {"amazon", "AMZN"},
	{"alphabet", "GOOGL"}, {"google", "GOOGL"}, {"meta", "META"}, {"tesla", "TSLA"},
	{"broadcom", "AVGO"}, {"amd", "AMD"}, {"intel", "INTC"}, {"netflix", "NFLX"},
	{"palantir", "PLTR"}, {"walmart", "WMT"}, {"boeing", "BA"}, {"jpmorgan", "JPM"},
	{"coinbase", "COIN"}, {"micron", "MU"}, {"tsmc", "TSM"}, {"alibaba", "BABA"},
	{"super micro", "SMCI"}, {"eli lilly", "LLY"}, {"exxon", "XOM"}, {"disney", "DIS"},
}

// FlashTickers 从标题里抽出相关代码（公司名映射 + 已知代码直出）。
func FlashTickers(titleEn string) []string {
	padded := padFlashText(titleEn)
	seen := map[string]bool{}
	var out []string
	add := func(t string) {
		if t == "" || seen[t] {
			return
		}
		seen[t] = true
		out = append(out, t)
	}
	for _, m := range flashCompanyTickers {
		if containsWord(padded, m.name) {
			add(m.ticker)
		}
	}
	for _, m := range flashCompanyTickers {
		if containsWord(padded, strings.ToLower(m.ticker)) {
			add(m.ticker)
		}
	}
	for _, etf := range []string{"SPY", "QQQ", "IWM", "TLT", "GLD", "DIA"} {
		if containsWord(padded, strings.ToLower(etf)) {
			add(etf)
		}
	}
	return out
}

// --- 正文 ---

// flashBodyEn 清洗 RSS 摘要。Google News 的 description 往往只是「标题 + 来源」的
// 回声，这种情况直接留空，免得前端出现两行一样的内容。
func flashBodyEn(description, titleEn string) string {
	body := stripFlashHTML(description)
	if body == "" {
		return ""
	}
	normBody, normTitle := normalizeFlashTitle(body), normalizeFlashTitle(titleEn)
	if normTitle != "" && strings.Contains(normBody, normTitle) && len(normBody) < len(normTitle)+60 {
		return ""
	}
	return truncateFlashBody(body, flashBodyMaxRunes)
}

const flashBodyMaxRunes = 120

func truncateFlashBody(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return strings.TrimSpace(string(r[:maxRunes])) + "…"
}

// --- 小工具 ---

var (
	flashNonWord   = regexp.MustCompile(`[^a-z0-9&\s]+`)
	flashMultiSpc  = regexp.MustCompile(`\s+`)
	flashHTMLTags  = regexp.MustCompile(`<[^>]*>`)
	flashSourceTag = regexp.MustCompile(`\s+-\s+([^-]{2,40})$`)
)

// padFlashText 归一化成 " word word " 形式，便于用整词匹配。
func padFlashText(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "s&p 500", "sp500")
	s = strings.ReplaceAll(s, "s&p500", "sp500")
	s = strings.ReplaceAll(s, "non-farm", "nonfarm")
	s = flashNonWord.ReplaceAllString(s, " ")
	s = flashMultiSpc.ReplaceAllString(s, " ")
	return " " + strings.TrimSpace(s) + " "
}

// containsWord 做整词匹配，并容忍一个复数 s（避免 "fed" 命中 "fedex"）。
func containsWord(padded, word string) bool {
	if word == "" {
		return false
	}
	return strings.Contains(padded, " "+word+" ") || strings.Contains(padded, " "+word+"s ")
}

func containsAnyWord(padded string, words []string) bool {
	for _, w := range words {
		if containsWord(padded, w) {
			return true
		}
	}
	return false
}

func normalizeFlashTitle(s string) string {
	return strings.TrimSpace(padFlashText(s))
}

func flashID(titleEn string) string {
	sum := sha1.Sum([]byte(normalizeFlashTitle(titleEn)))
	return "flash-" + hex.EncodeToString(sum[:])[:12]
}

func splitFlashSource(title string) (string, string) {
	m := flashSourceTag.FindStringSubmatch(title)
	if m == nil {
		return title, ""
	}
	return strings.TrimSpace(strings.TrimSuffix(title, m[0])), strings.TrimSpace(m[1])
}

func stripFlashHTML(s string) string {
	s = flashHTMLTags.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = flashMultiSpc.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	if len(s) > 400 {
		s = s[:400] + "…"
	}
	return s
}

var flashTimeLayouts = []string{
	time.RFC1123Z,
	time.RFC1123,
	time.RFC3339,
	"Mon, 02 Jan 2006 15:04:05 MST",
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"2006-01-02T15:04:05Z",
	"2006-01-02 15:04:05",
}

func parseFlashTime(raw string, fallback time.Time) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	for _, layout := range flashTimeLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return fallback
}
