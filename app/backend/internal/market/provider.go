package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"trading-agents/internal/config"
	"trading-agents/internal/dataflows"
)

// Quote 是对外的报价结构（/api/quote 的每一只股票）。
// json tag 决定序列化后的字段名，前端按这些名字取值。
type Quote struct {
	Price           float64 `json:"price"`
	Pct             float64 `json:"pct"`
	Session         string  `json:"session"`
	PrevClose       float64 `json:"prevClose"`
	Source          string  `json:"source,omitempty"` // tradingview | yahoo-* | local-us/cn
	Name            string  `json:"name,omitempty"`
	Currency        string  `json:"currency,omitempty"`
	CurrencyCode    string  `json:"currencyCode,omitempty"`
	CurrencySource  string  `json:"currencySource,omitempty"`
	DataTime        string  `json:"dataTime,omitempty"`
	TimeGranularity string  `json:"timeGranularity,omitempty"`
	SourceSession   string  `json:"sourceSession,omitempty"`
	ProviderURL     string  `json:"providerURL,omitempty"`
	ProviderMode    string  `json:"providerMode,omitempty"`
}

// snapshotQuote 对应本地 JSON 快照里 quotes 的一项（字段比 Quote 多市值/成交量）。
type snapshotQuote struct {
	Price           float64 `json:"price"`
	Pct             float64 `json:"pct"`
	Vol             float64 `json:"vol"`
	McapB           float64 `json:"mcapB,omitempty"`
	McapYi          float64 `json:"mcapYi,omitempty"`
	Source          string  `json:"source,omitempty"`
	DataTime        string  `json:"dataTime,omitempty"`
	TimeGranularity string  `json:"timeGranularity,omitempty"`
	Session         string  `json:"session,omitempty"`
	ProviderURL     string  `json:"providerURL,omitempty"`
	ProviderMode    string  `json:"providerMode,omitempty"`
}

// Provider 聚合多数据源。线程安全：并发读快照时用 RWMutex。
type Provider struct {
	alpaca            *dataflows.AlpacaQuoteClient
	tv                *dataflows.TVRestClient
	yf                *dataflows.YFinanceClient
	cn                *dataflows.CNQuoteClient
	http              *http.Client
	mu                sync.RWMutex
	refreshMu         sync.Mutex // serialize snapshot file writes across refresh paths
	cnRefreshMu       sync.Mutex // 串行化 A 股刷新，避免并发写入 a-market.json
	usSnap            map[string]snapshotQuote
	cnSnap            map[string]snapshotQuote
	usLiveSnap        map[string]snapshotQuote // only quotes proven by the current provider refresh
	usSnapshotAt      time.Time                // persisted US snapshot data time, never response time
	cnSnapshotAt      time.Time                // persisted CN snapshot data time, never response time
	liveAt            time.Time                // 最近一次成功 US live 刷新
	usLiveSource      string                   // provider attached to liveAt
	cnLiveAt          time.Time                // 最近一次成功 CN live 刷新
	usRefreshing      atomic.Bool
	cnRefreshing      atomic.Bool
	cnFetch           cnRefreshFetcher
	now               func() time.Time
	usBatchFetch      usRefreshBatchFetcher
	usTVFetch         usRefreshTVFetcher
	usYahooFetch      usRefreshYahooFetcher
	usSnapWrite       usRefreshSnapshotWriter
	usFilePatch       usRefreshFilePatcher
	directYahooFetch  usRefreshYahooFetcher
	directNasdaqFetch func([]string) map[string]dataflows.NasdaqQuote
	alpacaFetch       func([]string) (map[string]dataflows.AlpacaQuote, error)
	providerBreakerMu sync.Mutex
	providerBreakers  map[string]*providerBreaker
}

type providerBreaker struct {
	failures  int
	openUntil time.Time
}

var (
	defaultProvider     *Provider
	defaultProviderOnce sync.Once // 保证进程内只初始化一次（单例）
)

// Default 返回全局 Provider（懒加载）。
func Default() *Provider {
	defaultProviderOnce.Do(func() {
		defaultProvider = New()
		defaultProvider.ReloadSnapshots()
	})
	return defaultProvider
}

// New 创建 Provider（不依赖 stockgod 上游）。
func New() *Provider {
	credentials, err := config.LoadAlpacaCredentials()
	if err != nil {
		fmt.Printf("[WARN] Alpaca credentials unavailable: %v\n", err)
	}
	alpaca := dataflows.NewAlpacaQuoteClient(credentials.APIKeyID, credentials.APISecretKey)
	provider := &Provider{
		alpaca: alpaca,
		tv:     dataflows.NewTVRestClient(),
		yf:     dataflows.NewYFinanceClient(),
		cn:     dataflows.NewCNQuoteClient(),
		http:   &http.Client{Timeout: 12 * time.Second},
		usSnap: map[string]snapshotQuote{},
		cnSnap: map[string]snapshotQuote{},
		now:    time.Now,
		providerBreakers: map[string]*providerBreaker{
			"tradingview": {},
			"yahoo":       {},
		},
	}
	provider.cnFetch = provider.cn.GetQuotes
	provider.alpacaFetch = alpaca.GetQuotes
	return provider
}

func (p *Provider) AlpacaConfigured() bool {
	return p != nil && p.alpaca != nil && p.alpaca.Configured()
}

// ReloadSnapshots 把本地快照装进内存，供 Quotes 最后一档兜底。
// 美股优先 us-stocks.json（与 /api/stocks 同源），再回写 market.json。
func (p *Provider) ReloadSnapshots() {
	us, generatedAt, err := usQuotesFromUsStocks()
	usAt := parseSnapshotTime(generatedAt)
	if err != nil || len(us) == 0 {
		us, usAt = readSnapshotQuotes("market.json")
	} else {
		if timedQuotes, _ := readSnapshotQuotes("market.json"); len(timedQuotes) > 0 {
			overlayMatchingSnapshotProvenance(us, timedQuotes)
		}
		if usAt.IsZero() {
			if _, path, readErr := readUsStocksFile(); readErr == nil {
				usAt = fileModifiedAt(path)
			}
		}
	}
	cn, cnAt := readSnapshotQuotes("a-market.json")
	p.mu.Lock()
	defer p.mu.Unlock()
	if us != nil {
		p.usSnap = us
		p.usSnapshotAt = usAt
	}
	if cn != nil {
		p.cnSnap = cn
		p.cnSnapshotAt = cnAt
	}
}

func overlayMatchingSnapshotProvenance(target, evidence map[string]snapshotQuote) {
	for symbol, quote := range target {
		matched, ok := evidence[symbol]
		if !ok || matched.DataTime == "" || math.Abs(matched.Price-quote.Price) > 1e-8 || math.Abs(matched.Pct-quote.Pct) > 0.01 {
			continue
		}
		quote.Source = matched.Source
		quote.DataTime = matched.DataTime
		quote.TimeGranularity = matched.TimeGranularity
		quote.Session = matched.Session
		quote.ProviderURL = matched.ProviderURL
		quote.ProviderMode = matched.ProviderMode
		target[symbol] = quote
	}
}

func readSnapshotQuotes(name string) (map[string]snapshotQuote, time.Time) {
	raw, path, err := readDataFileWithPath(name)
	if err != nil || len(raw) == 0 {
		return nil, time.Time{}
	}
	var payload struct {
		Quotes map[string]snapshotQuote `json:"quotes"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Quotes == nil {
		return nil, time.Time{}
	}
	at := embeddedSnapshotTime(raw)
	if at.IsZero() {
		at = fileModifiedAt(path)
	}
	return payload.Quotes, at
}

// Quotes 批量取价：TradingView → Yahoo → 本地快照。
// 返回 map 的 key 是请求里的原始 symbol；没拿到的符号不会出现在 map 里。
func (p *Provider) Quotes(symbols []string) map[string]Quote {
	out := make(map[string]Quote, len(symbols))
	pending := make([]string, 0, len(symbols))
	for _, s := range symbols {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		pending = append(pending, s)
	}
	if len(pending) == 0 {
		return out
	}

	// ①b A 股：东财 ulist（跳过 TV）
	cnBatch := make([]string, 0, len(pending))
	for _, s := range pending {
		if isCNSymbol(s) {
			cnBatch = append(cnBatch, s)
		}
	}
	const cnBatchSize = 80
	if len(cnBatch) > 0 && p.cn != nil {
		byCode := map[string]dataflows.CNQuote{}
		for i := 0; i < len(cnBatch); i += cnBatchSize {
			j := i + cnBatchSize
			if j > len(cnBatch) {
				j = len(cnBatch)
			}
			items, err := p.cn.GetQuotes(cnBatch[i:j])
			if err != nil {
				continue
			}
			for _, item := range items {
				byCode[item.Symbol] = item
			}
		}
		still := pending[:0]
		for _, s := range pending {
			if item, ok := byCode[s]; ok && item.Price > 0 && !item.ObservedAt.IsZero() {
				prev := item.Price
				if item.Pct != -100 {
					prev = item.Price / (1.0 + item.Pct/100.0)
				}
				session := exchangeSessionAt(s, p.now())
				out[s] = Quote{
					Price:     item.Price,
					Pct:       item.Pct,
					Session:   session,
					PrevClose: prev,
					Source:    "eastmoney-cn",
					Name:      item.Name,
					Currency:  "¥",
					DataTime:  item.ObservedAt.UTC().Format(time.RFC3339), TimeGranularity: "second", SourceSession: exchangeSessionAt(s, item.ObservedAt),
				}
			} else {
				still = append(still, s)
			}
		}
		pending = still
	}

	// ① Alpaca IEX（配置免费 API key 后启用）。IEX-only 会明确写入来源，不冒充 SIP 全市场。
	usBatch := make([]string, 0, len(pending))
	for _, s := range pending {
		if isLikelyUSSymbol(s) {
			usBatch = append(usBatch, s)
		}
	}
	if len(usBatch) > 0 && p.alpaca != nil && p.alpaca.Configured() {
		items, err := p.alpacaFetch(usBatch)
		if err != nil {
			log.Printf("[market/alpaca] quote fetch failed: %v", err)
		}
		if err == nil {
			still := pending[:0]
			for _, s := range pending {
				item, ok := items[strings.ToUpper(s)]
				if !ok || item.Price <= 0 || item.ObservedAt.IsZero() {
					still = append(still, s)
					continue
				}
				pct := float64(0)
				if item.PrevClose > 0 {
					pct = (item.Price/item.PrevClose - 1) * 100
				}
				session := exchangeSessionAt(s, p.now())
				out[s] = Quote{Price: item.Price, Pct: pct, PrevClose: item.PrevClose, Session: session,
					Source: "alpaca-iex", Currency: "$", CurrencyCode: "USD", CurrencySource: "alpaca-snapshot",
					DataTime: item.ObservedAt.UTC().Format(time.RFC3339Nano), TimeGranularity: "nanosecond", SourceSession: session,
					ProviderURL: item.SourceURL, ProviderMode: "iex-only",
				}
			}
			pending = still
		}
	}

	// ② TradingView 仅作无凭据展示补洞；Paper 权威门禁不会接受该来源。
	usBatch = usBatch[:0]
	for _, s := range pending {
		if isLikelyUSSymbol(s) {
			usBatch = append(usBatch, s)
		}
	}
	const tvBatch = 30
	if len(usBatch) > 0 {
		byBase := map[string]dataflows.TVQuoteData{}
		for i := 0; i < len(usBatch); i += tvBatch {
			j := i + tvBatch
			if j > len(usBatch) {
				j = len(usBatch)
			}
			items, err := p.tv.GetRealTimeQuotes(usBatch[i:j])
			if err != nil {
				continue
			}
			for _, item := range items {
				base := item.Symbol
				if idx := strings.LastIndex(base, ":"); idx >= 0 {
					base = base[idx+1:]
				}
				byBase[strings.ToUpper(base)] = item
			}
		}
		still := pending[:0]
		for _, s := range pending {
			if item, ok := byBase[strings.ToUpper(s)]; ok && item.Price > 0 && !item.DataTime.IsZero() {
				pct := item.ChangePct
				prev := item.PrevClose
				if prev <= 0 && pct != -100 {
					prev = item.Price / (1.0 + pct/100.0)
				}
				session := exchangeSessionAt(s, p.now())
				out[s] = Quote{
					Price: item.Price, Pct: pct, Session: session, PrevClose: prev, Source: "tradingview",
					DataTime: item.DataTime.UTC().Format(time.RFC3339), TimeGranularity: "second", SourceSession: session,
					ProviderURL: item.SourceURL, ProviderMode: "display-only:" + item.UpdateMode,
				}
			} else {
				still = append(still, s)
			}
		}
		pending = still
	}

	// ② Yahoo 补洞（含部分非美市场）
	if len(pending) > 0 {
		yahooMap := p.yahooBatch(pending)
		still := pending[:0]
		for _, s := range pending {
			if q, ok := yahooMap[s]; ok {
				out[s] = q
			} else {
				still = append(still, s)
			}
		}
		pending = still
	}

	// ③ Nasdaq 官方页面接口补洞。仅接受带来源成交时间、市场状态和币种证据的响应。
	if len(pending) > 0 {
		usPending := make([]string, 0, len(pending))
		for _, s := range pending {
			if isLikelyUSSymbol(s) {
				usPending = append(usPending, s)
			}
		}
		nasdaqQuotes := dataflows.NasdaqQuotes(usPending)
		still := pending[:0]
		for _, s := range pending {
			q, ok := nasdaqQuotes[strings.ToUpper(s)]
			if !ok || q.Price <= 0 || q.TradeTime.IsZero() || q.MarketStatus == "" || q.Currency == "" {
				still = append(still, s)
				continue
			}
			prev := q.Price - q.Change
			if prev <= 0 && q.ChangePct != -100 {
				prev = q.Price / (1 + q.ChangePct/100)
			}
			out[s] = Quote{
				Price: q.Price, Pct: q.ChangePct, Session: q.MarketStatus, PrevClose: prev,
				Source: "nasdaq", Currency: currencyDisplay(q.Currency), CurrencyCode: q.Currency,
				DataTime: formatProviderObservationTime(q.TradeTime, q.TimeGranularity), TimeGranularity: q.TimeGranularity, SourceSession: q.MarketStatus,
				CurrencySource: q.CurrencySource,
				ProviderURL:    q.SourceURL,
			}
		}
		pending = still
	}

	// ④ 本地快照兜底（绝不编造价格）
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, s := range pending {
		if q, ok := p.usSnap[s]; ok && q.Price > 0 {
			out[s] = snapshotToQuote(q, "stale-snapshot:us")
			continue
		}
		if q, ok := p.cnSnap[s]; ok && q.Price > 0 {
			out[s] = snapshotToQuote(q, "stale-snapshot:cn")
		}
	}
	for symbol, quote := range out {
		quote.Session = normalizedExchangeSession(symbol, quote.Session, p.now())
		out[symbol] = quote
	}
	return out
}

func currencyDisplay(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "USD":
		return "$"
	case "HKD":
		return "HK$"
	case "CNY", "RMB":
		return "¥"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "CAD":
		return "C$"
	default:
		return code
	}
}

// SnapshotQuotes returns the last persisted real quote for the requested
// symbols without making network calls. It is used when a live provider is
// unavailable so callers can respond promptly and label the data as stale.
func (p *Provider) SnapshotQuotes(symbols []string) map[string]Quote {
	out := make(map[string]Quote, len(symbols))
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, raw := range symbols {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		if q, ok := p.usSnap[s]; ok && q.Price > 0 {
			out[s] = snapshotToQuote(q, "stale-snapshot:us")
			continue
		}
		if q, ok := p.cnSnap[s]; ok && q.Price > 0 {
			out[s] = snapshotToQuote(q, "stale-snapshot:cn")
		}
	}
	for symbol, quote := range out {
		quote.Session = normalizedExchangeSession(symbol, quote.Session, p.now())
		out[symbol] = quote
	}
	return out
}

// SecondaryFastQuotes races one Yahoo batch with Nasdaq's bounded symbol
// requests. It never reads local snapshots; callers must fail closed when the
// returned set is incomplete or lacks provider observation time.
func (p *Provider) SecondaryFastQuotes(ctx context.Context, symbols []string) map[string]Quote {
	wanted := make([]string, 0, len(symbols))
	seen := make(map[string]bool, len(symbols))
	for _, raw := range symbols {
		symbol := strings.ToUpper(strings.TrimSpace(raw))
		if symbol == "" || seen[symbol] || !isLikelyUSSymbol(symbol) {
			continue
		}
		seen[symbol] = true
		wanted = append(wanted, symbol)
	}
	if len(wanted) == 0 {
		return map[string]Quote{}
	}
	type directResult struct {
		source string
		quotes map[string]Quote
	}
	results := make(chan directResult, 2)
	go func() {
		yahooSymbols := make([]string, 0, len(wanted))
		back := make(map[string]string, len(wanted))
		for _, symbol := range wanted {
			yahooSymbol := toYahooSymbol(symbol)
			yahooSymbols = append(yahooSymbols, yahooSymbol)
			back[yahooSymbol] = symbol
		}
		fetch := p.directYahooFetch
		if fetch == nil {
			fetch = p.yahooQuoteAPI
		}
		out := make(map[string]Quote, len(wanted))
		if quotes, err := fetch(yahooSymbols); err == nil {
			for yahooSymbol, quote := range quotes {
				if symbol, ok := back[yahooSymbol]; ok {
					out[symbol] = quote
				}
			}
		}
		results <- directResult{source: "yahoo", quotes: out}
	}()
	go func() {
		fetch := p.directNasdaqFetch
		if fetch == nil {
			fetch = dataflows.NasdaqQuotes
		}
		items := fetch(wanted)
		out := make(map[string]Quote, len(items))
		for symbol, item := range items {
			if item.Price <= 0 || item.TradeTime.IsZero() || item.MarketStatus == "" || item.Currency == "" {
				continue
			}
			prev := item.Price - item.Change
			if prev <= 0 && item.ChangePct != -100 {
				prev = item.Price / (1 + item.ChangePct/100)
			}
			out[strings.ToUpper(symbol)] = Quote{
				Price: item.Price, Pct: item.ChangePct, Session: item.MarketStatus, PrevClose: prev,
				Source: "nasdaq", Currency: currencyDisplay(item.Currency), CurrencyCode: item.Currency,
				DataTime: formatProviderObservationTime(item.TradeTime, item.TimeGranularity), TimeGranularity: item.TimeGranularity,
				SourceSession: item.MarketStatus, CurrencySource: item.CurrencySource, ProviderURL: item.SourceURL,
			}
		}
		results <- directResult{source: "nasdaq", quotes: out}
	}()

	merged := make(map[string]Quote, len(wanted))
	for remaining := 2; remaining > 0; remaining-- {
		select {
		case result := <-results:
			for symbol, quote := range result.quotes {
				if _, ok := seen[symbol]; !ok {
					continue
				}
				if existing, ok := merged[symbol]; !ok || quoteIsNewer(quote, existing) {
					merged[symbol] = quote
				}
			}
		case <-ctx.Done():
			return merged
		}
	}
	return merged
}

func quoteIsNewer(candidate, existing Quote) bool {
	if existing.Price <= 0 {
		return true
	}
	candidateAt, candidateErr := parseProviderObservationTime(candidate.DataTime)
	existingAt, existingErr := parseProviderObservationTime(existing.DataTime)
	if candidateErr != nil {
		return false
	}
	return existingErr != nil || candidateAt.After(existingAt)
}

// CachedQuotes returns provider-observed in-memory quotes when available and
// falls back to the persisted real snapshot without performing network I/O.
func (p *Provider) CachedQuotes(symbols []string) map[string]Quote {
	out := make(map[string]Quote, len(symbols))
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, raw := range symbols {
		symbol := strings.TrimSpace(raw)
		if symbol == "" {
			continue
		}
		if quote, ok := p.usLiveSnap[symbol]; ok && quote.Price > 0 {
			source := strings.TrimSpace(quote.Source)
			if source == "" {
				source = "unverified-provider"
			}
			out[symbol] = snapshotToQuote(quote, source)
			continue
		}
		if quote, ok := p.usSnap[symbol]; ok && quote.Price > 0 {
			out[symbol] = snapshotToQuote(quote, "stale-snapshot:us")
			continue
		}
		if quote, ok := p.cnSnap[symbol]; ok && quote.Price > 0 {
			out[symbol] = snapshotToQuote(quote, "stale-snapshot:cn")
		}
	}
	for symbol, quote := range out {
		quote.Session = normalizedExchangeSession(symbol, quote.Session, p.now())
		out[symbol] = quote
	}
	return out
}

// SnapshotUpdatedAt returns the newest persisted snapshot data time.
// It prefers embedded generatedAt/ts and falls back to mtime.
func (p *Provider) SnapshotUpdatedAt() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.usSnapshotAt.After(p.cnSnapshotAt) {
		return p.usSnapshotAt
	}
	return p.cnSnapshotAt
}

// SnapshotDataTime returns the persisted data time for one market.
func (p *Provider) SnapshotDataTime(kind string) time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if kind == "cn" || kind == "a" {
		return p.cnSnapshotAt
	}
	return p.usSnapshotAt
}

func embeddedSnapshotTime(raw []byte) time.Time {
	var payload struct {
		Ts          int64  `json:"ts"`
		GeneratedAt string `json:"generated_at"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return time.Time{}
	}
	if payload.Ts > 0 {
		return time.UnixMilli(payload.Ts)
	}
	if payload.GeneratedAt != "" {
		return parseSnapshotTime(payload.GeneratedAt)
	}
	return time.Time{}
}

func parseSnapshotTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "ts:") {
		if ms, err := strconv.ParseInt(strings.TrimPrefix(value, "ts:"), 10, 64); err == nil && ms > 0 {
			return time.UnixMilli(ms)
		}
	}
	if strings.HasSuffix(value, " ET") {
		if at, err := time.ParseInLocation("2006-01-02 15:04", strings.TrimSuffix(value, " ET"), time.FixedZone("ET", -5*60*60)); err == nil {
			return at
		}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04 MST", "2006-01-02 15:04"} {
		if at, err := time.Parse(layout, value); err == nil {
			return at
		}
	}
	return time.Time{}
}

func fileModifiedAt(path string) time.Time {
	if info, err := os.Stat(path); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

func snapshotToQuote(q snapshotQuote, source string) Quote {
	prev := q.Price
	if q.Pct != -100 {
		prev = q.Price / (1.0 + q.Pct/100.0)
	}
	return Quote{
		Price: q.Price, Pct: q.Pct, Session: q.Session, PrevClose: prev, Source: source,
		DataTime: q.DataTime, TimeGranularity: q.TimeGranularity, SourceSession: q.Session, ProviderURL: q.ProviderURL, ProviderMode: q.ProviderMode,
	}
}

func formatProviderObservationTime(value time.Time, granularity string) string {
	if value.IsZero() {
		return ""
	}
	if granularity == "date" {
		return value.Format("2006-01-02")
	}
	return value.UTC().Format(time.RFC3339)
}

func parseProviderObservationTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if len(value) == len("2006-01-02") {
		return time.Parse("2006-01-02", value)
	}
	return time.Parse(time.RFC3339, value)
}

func isLikelyUSSymbol(sym string) bool {
	return dataflows.DetectMarket(sym) == dataflows.MarketUS
}

func isCNSymbol(sym string) bool {
	return dataflows.DetectMarket(sym) == dataflows.MarketCN
}

func toYahooSymbol(sym string) string {
	return dataflows.ToYahooSymbol(sym)
}

func (p *Provider) yahooBatch(symbols []string) map[string]Quote {
	out := make(map[string]Quote, len(symbols))
	yahooSyms := make([]string, 0, len(symbols))
	back := map[string]string{} // yahoo -> original
	for _, s := range symbols {
		ys := toYahooSymbol(s)
		yahooSyms = append(yahooSyms, ys)
		back[ys] = s
	}

	// chunk to keep URL short
	for i := 0; i < len(yahooSyms); i += 40 {
		end := i + 40
		if end > len(yahooSyms) {
			end = len(yahooSyms)
		}
		chunk := yahooSyms[i:end]
		part, err := p.yahooQuoteAPI(chunk)
		if err != nil {
			// per-symbol chart fallback
			for _, ys := range chunk {
				if q, err := p.yahooChart(ys); err == nil {
					out[back[ys]] = q
				}
			}
			continue
		}
		for ys, q := range part {
			if orig, ok := back[ys]; ok {
				out[orig] = q
			}
		}
	}
	return out
}

func (p *Provider) yahooQuoteAPI(yahooSymbols []string) (map[string]Quote, error) {
	u := "https://query1.finance.yahoo.com/v7/finance/quote?symbols=" + url.QueryEscape(strings.Join(yahooSymbols, ","))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo quote status %d", resp.StatusCode)
	}

	var parsed struct {
		QuoteResponse struct {
			Result []struct {
				Symbol                     string  `json:"symbol"`
				ShortName                  string  `json:"shortName"`
				LongName                   string  `json:"longName"`
				RegularMarketPrice         float64 `json:"regularMarketPrice"`
				RegularMarketChangePct     float64 `json:"regularMarketChangePercent"`
				RegularMarketPreviousClose float64 `json:"regularMarketPreviousClose"`
				MarketState                string  `json:"marketState"`
				Currency                   string  `json:"currency"`
				RegularMarketTime          int64   `json:"regularMarketTime"`
			} `json:"result"`
		} `json:"quoteResponse"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	out := make(map[string]Quote, len(parsed.QuoteResponse.Result))
	for _, r := range parsed.QuoteResponse.Result {
		if r.RegularMarketPrice <= 0 || r.RegularMarketTime <= 0 {
			continue
		}
		session := "regular"
		if r.MarketState != "" {
			session = strings.ToLower(r.MarketState)
		}
		name := r.ShortName
		if name == "" {
			name = r.LongName
		}
		cur := "$"
		if strings.EqualFold(r.Currency, "HKD") || strings.HasSuffix(r.Symbol, ".HK") {
			cur = "HK$"
		} else if strings.EqualFold(r.Currency, "CNY") || strings.EqualFold(r.Currency, "RMB") {
			cur = "¥"
		} else if strings.EqualFold(r.Currency, "TWD") {
			cur = "NT$"
		} else if strings.EqualFold(r.Currency, "JPY") {
			cur = "JP¥"
		} else if strings.EqualFold(r.Currency, "GBP") {
			cur = "£"
		} else if strings.EqualFold(r.Currency, "EUR") {
			cur = "€"
		}
		out[r.Symbol] = Quote{
			Price:           r.RegularMarketPrice,
			Pct:             r.RegularMarketChangePct,
			Session:         session,
			PrevClose:       r.RegularMarketPreviousClose,
			Source:          "yahoo",
			Name:            name,
			Currency:        cur,
			CurrencyCode:    r.Currency,
			DataTime:        time.Unix(r.RegularMarketTime, 0).UTC().Format(time.RFC3339),
			TimeGranularity: "second",
			SourceSession:   session,
			ProviderURL:     u,
			ProviderMode:    "display-only",
		}
	}
	return out, nil
}

func (p *Provider) yahooChart(yahooSymbol string) (Quote, error) {
	u := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=10d", url.PathEscape(yahooSymbol))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return Quote{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := p.http.Do(req)
	if err != nil {
		return Quote{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode != http.StatusOK {
		return Quote{}, fmt.Errorf("chart status %d", resp.StatusCode)
	}
	var parsed struct {
		Chart struct {
			Result []struct {
				Meta struct {
					RegularMarketPrice float64 `json:"regularMarketPrice"`
					PreviousClose      float64 `json:"previousClose"`
					ChartPreviousClose float64 `json:"chartPreviousClose"`
					MarketState        string  `json:"marketState"`
					RegularMarketTime  int64   `json:"regularMarketTime"`
					Currency           string  `json:"currency"`
				} `json:"meta"`
				Indicators struct {
					Quote []struct {
						Close []*float64 `json:"close"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || len(parsed.Chart.Result) == 0 {
		return Quote{}, fmt.Errorf("chart parse failed")
	}
	res := parsed.Chart.Result[0]
	meta := res.Meta
	price := meta.RegularMarketPrice

	// Prefer last two valid daily closes — chartPreviousClose is often wrong vs prior session.
	var closes []float64
	if len(res.Indicators.Quote) > 0 {
		for _, c := range res.Indicators.Quote[0].Close {
			if c != nil && *c > 0 {
				closes = append(closes, *c)
			}
		}
	}
	if len(closes) > 0 {
		price = closes[len(closes)-1]
	}
	if price <= 0 || meta.RegularMarketTime <= 0 {
		return Quote{}, fmt.Errorf("no price")
	}

	prev := 0.0
	if len(closes) >= 2 {
		prev = closes[len(closes)-2]
	}
	if prev <= 0 && meta.PreviousClose > 0 {
		prev = meta.PreviousClose
	}
	if prev <= 0 && meta.ChartPreviousClose > 0 {
		prev = meta.ChartPreviousClose
	}

	pct := 0.0
	if prev > 0 {
		pct = (price - prev) / prev * 100
	}
	session := "regular"
	if meta.MarketState != "" {
		session = strings.ToLower(meta.MarketState)
	}
	return Quote{
		Price:     price,
		Pct:       pct,
		Session:   session,
		PrevClose: prev,
		Source:    "yahoo-chart",
		Currency:  currencyDisplay(meta.Currency), CurrencyCode: meta.Currency,
		DataTime: time.Unix(meta.RegularMarketTime, 0).UTC().Format(time.RFC3339), TimeGranularity: "second",
		SourceSession: session, ProviderURL: u, ProviderMode: "display-only",
	}, nil
}

// SnapshotPayload returns local market snapshot bytes for /api/market or /api/a-market.
// US prefers a live-derived payload from us-stocks.json (same prices as /api/stocks).
func SnapshotPayload(kind string) ([]byte, string, error) {
	if kind == "cn" || kind == "a" {
		raw, path, err := readDataFileWithPath("a-market.json")
		if err != nil {
			return nil, "stale-snapshot:cn", err
		}
		dataAt := embeddedSnapshotTime(raw)
		if dataAt.IsZero() {
			dataAt = fileModifiedAt(path)
		}
		raw, err = ensureSnapshotPayloadTime(raw, dataAt)
		if err != nil {
			return nil, "stale-snapshot:cn", err
		}
		src := "stale-snapshot:cn"
		if !dataAt.IsZero() {
			src += "@" + dataAt.UTC().Format(time.RFC3339)
		}
		return raw, src, nil
	}

	quotes, generatedAt, err := usQuotesFromUsStocks()
	if err == nil && len(quotes) > 0 {
		dataAt := parseSnapshotTime(generatedAt)
		if dataAt.IsZero() {
			if _, path, readErr := readUsStocksFile(); readErr == nil {
				dataAt = fileModifiedAt(path)
			}
		}
		raw, marshalErr := json.Marshal(marketSnapshotPayload{
			Quotes:  quotes,
			Ts:      millisOrZero(dataAt),
			Count:   len(quotes),
			Session: exchangeSessionAt("AAPL", time.Now()),
		})
		if marshalErr != nil {
			return nil, "", marshalErr
		}
		src := "stale-snapshot:us-stocks"
		if !dataAt.IsZero() {
			src += "@" + dataAt.UTC().Format(time.RFC3339)
		}
		return raw, src, nil
	}

	raw, path, err := readDataFileWithPath("market.json")
	if err != nil {
		return nil, "local-us-snapshot", err
	}
	src := "stale-snapshot:us"
	dataAt := embeddedSnapshotTime(raw)
	if dataAt.IsZero() {
		dataAt = fileModifiedAt(path)
	}
	raw, err = ensureSnapshotPayloadTime(raw, dataAt)
	if err != nil {
		return nil, "stale-snapshot:us", err
	}
	if !dataAt.IsZero() {
		src += "@" + dataAt.UTC().Format(time.RFC3339)
	}
	return raw, src, nil
}

func millisOrZero(at time.Time) int64 {
	if at.IsZero() {
		return 0
	}
	return at.UnixMilli()
}

func ensureSnapshotPayloadTime(raw []byte, dataAt time.Time) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if !dataAt.IsZero() {
		payload["ts"] = dataAt.UnixMilli()
	}
	if quotes, ok := payload["quotes"].(map[string]any); ok {
		for symbol := range quotes {
			payload["session"] = exchangeSessionAt(symbol, time.Now())
			break
		}
	}
	return json.Marshal(payload)
}

func marshalMarketPayload(quotes map[string]snapshotQuote, status PayloadStatus) ([]byte, error) {
	dataTimestamp := millisOrZero(status.DataTime)
	if status.TimeGranularity == "date" {
		dataTimestamp = 0
	}
	return json.Marshal(marketSnapshotPayload{
		Quotes: quotes, Ts: dataTimestamp, Count: len(quotes), Session: status.Session,
		Source: status.Source, DataTime: formatProviderObservationTime(status.DataTime, status.TimeGranularity), TimeGranularity: status.TimeGranularity, Stale: status.Stale, ProviderURL: status.ProviderURL,
	})
}

type marketSnapshotPayload struct {
	Quotes          map[string]snapshotQuote `json:"quotes"`
	Ts              int64                    `json:"ts"`
	Count           int                      `json:"count"`
	Session         string                   `json:"session"`
	Source          string                   `json:"source,omitempty"`
	DataTime        string                   `json:"dataTime,omitempty"`
	TimeGranularity string                   `json:"timeGranularity,omitempty"`
	Stale           bool                     `json:"stale"`
	ProviderURL     string                   `json:"providerURL,omitempty"`
}

type usStocksFileLite struct {
	GeneratedAt string `json:"generated_at"`
	Stocks      []struct {
		Sym   string  `json:"sym"`
		Price float64 `json:"price"`
		Pct   float64 `json:"pct"`
		McapB float64 `json:"mcapB"`
		Vol   float64 `json:"vol"`
	} `json:"stocks"`
}

func usQuotesFromUsStocks() (map[string]snapshotQuote, string, error) {
	raw, path, err := readUsStocksFile()
	if err != nil {
		return nil, "", err
	}
	out, generatedAt, err := parseUsStocksQuotes(raw)
	if err != nil {
		return nil, path, fmt.Errorf("%s: %w", path, err)
	}
	return out, generatedAt, nil
}

// parseUsStocksQuotes converts us-stocks.json bytes into the /api/market quote map.
func parseUsStocksQuotes(raw []byte) (map[string]snapshotQuote, string, error) {
	var file usStocksFileLite
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, "", fmt.Errorf("parse us-stocks: %w", err)
	}
	out := make(map[string]snapshotQuote, len(file.Stocks))
	for _, row := range file.Stocks {
		sym := strings.TrimSpace(strings.ToUpper(row.Sym))
		if sym == "" || row.Price <= 0 {
			continue
		}
		out[sym] = snapshotQuote{
			Price: row.Price,
			Pct:   row.Pct,
			Vol:   row.Vol,
			McapB: row.McapB,
		}
	}
	if len(out) == 0 {
		return nil, "", fmt.Errorf("no quotes in us-stocks payload")
	}
	return out, file.GeneratedAt, nil
}

func readUsStocksFile() ([]byte, string, error) {
	candidates := []string{
		filepath.Join("..", "frontend", "public", "data", "us-stocks.json"),
		filepath.Join("app", "frontend", "public", "data", "us-stocks.json"),
		filepath.Join("frontend", "public", "data", "us-stocks.json"),
		filepath.Join("data", "us-stocks.json"),
		filepath.Join("app", "backend", "data", "us-stocks.json"),
		filepath.Join("..", "frontend", "dist", "data", "us-stocks.json"),
	}
	var tried []string
	for _, p := range candidates {
		raw, err := os.ReadFile(p)
		if err != nil {
			tried = append(tried, p)
			continue
		}
		return raw, p, nil
	}
	return nil, "", fmt.Errorf("us-stocks.json not found in %v", tried)
}

func writeMarketSnapshotFile(quotes map[string]snapshotQuote, dataAt time.Time) error {
	return writeSnapshotFileAt(quotes, "market.json", dataAt)
}

func writeSnapshotFileAt(quotes map[string]snapshotQuote, filename string, dataAt time.Time) error {
	if dataAt.IsZero() {
		return fmt.Errorf("refusing to write %s without data time", filename)
	}
	sessionSymbol := "AAPL"
	if filename == "a-market.json" {
		sessionSymbol = "600519"
	}
	granularity := "second"
	if observed, observedGranularity, complete := snapshotObservationTimeWithGranularity(quotes, dataAt); complete && observed.Equal(dataAt) && observedGranularity != "" {
		granularity = observedGranularity
	}
	dataTimestamp := dataAt.UnixMilli()
	if granularity == "date" {
		dataTimestamp = 0
	}
	payload := marketSnapshotPayload{
		Quotes:          quotes,
		Ts:              dataTimestamp,
		Count:           len(quotes),
		Session:         exchangeSessionAt(sessionSymbol, dataAt),
		DataTime:        formatProviderObservationTime(dataAt, granularity),
		TimeGranularity: granularity,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	paths := []string{
		filepath.Join("data", filename),
		filepath.Join("app", "backend", "data", filename),
		filepath.Join("..", "frontend", "public", "data", filename),
		filepath.Join("app", "frontend", "public", "data", filename),
		filepath.Join("..", "frontend", "dist", "data", filename),
		filepath.Join("app", "frontend", "dist", "data", filename),
	}
	var lastErr error
	wrote := false
	for _, p := range paths {
		if _, err := os.Stat(filepath.Dir(p)); err != nil {
			continue
		}
		if err := os.WriteFile(p, raw, 0o644); err != nil {
			lastErr = err
			continue
		}
		wrote = true
	}
	if !wrote {
		return lastErr
	}
	return nil
}

// UpsertCNQuotes merges live CN quotes into the in-memory snapshot so later
// RefreshCNQuotes / Quotes can serve them without re-fetching every time.
func (p *Provider) UpsertCNQuotes(quotes map[string]Quote) {
	if len(quotes) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cnSnap == nil {
		p.cnSnap = map[string]snapshotQuote{}
	}
	for sym, q := range quotes {
		observedAt, err := parseProviderObservationTime(q.DataTime)
		source := strings.ToLower(strings.TrimSpace(q.Source))
		if q.Price <= 0 || source == "" || strings.HasPrefix(source, "local-") || strings.HasPrefix(source, "stale-snapshot:") || strings.HasPrefix(source, "closed-") || err != nil || observedAt.IsZero() {
			continue
		}
		prev := p.cnSnap[sym]
		session := strings.TrimSpace(q.SourceSession)
		if session == "" {
			session = strings.TrimSpace(q.Session)
		}
		p.cnSnap[sym] = snapshotQuote{
			Price:           q.Price,
			Pct:             q.Pct,
			Vol:             prev.Vol,
			McapYi:          prev.McapYi,
			Source:          q.Source,
			DataTime:        formatProviderObservationTime(observedAt, q.TimeGranularity),
			TimeGranularity: q.TimeGranularity,
			Session:         session,
			ProviderURL:     q.ProviderURL,
			ProviderMode:    q.ProviderMode,
		}
	}
}

// EnsureCNSymbols guarantees symbols exist in cnSnap (even with price 0) so
// live refresh batches include recommended ETFs outside the stock universe.
func (p *Provider) EnsureCNSymbols(syms []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cnSnap == nil {
		p.cnSnap = map[string]snapshotQuote{}
	}
	for _, s := range syms {
		s = strings.TrimSpace(s)
		if s == "" || !isCNSymbol(s) {
			continue
		}
		if _, ok := p.cnSnap[s]; ok {
			continue
		}
		p.cnSnap[s] = snapshotQuote{}
	}
}

func readDataFileWithPath(name string) ([]byte, string, error) {
	for _, path := range []string{filepath.Join("data", name), filepath.Join("app", "backend", "data", name)} {
		raw, err := os.ReadFile(path)
		if err == nil {
			return raw, path, nil
		}
	}
	return nil, "", fmt.Errorf("%s not found", name)
}
