package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"trading-agents/internal/dataflows"
)

// Quote 是对外的报价结构（/api/quote 的每一只股票）。
// json tag 决定序列化后的字段名，前端按这些名字取值。
type Quote struct {
	Price     float64 `json:"price"`
	Pct       float64 `json:"pct"`
	Session   string  `json:"session"`
	PrevClose float64 `json:"prevClose"`
	Source    string  `json:"source,omitempty"` // tradingview | yahoo-* | local-us/cn
}

// snapshotQuote 对应本地 JSON 快照里 quotes 的一项（字段比 Quote 多市值/成交量）。
type snapshotQuote struct {
	Price  float64 `json:"price"`
	Pct    float64 `json:"pct"`
	Vol    float64 `json:"vol"`
	McapB  float64 `json:"mcapB,omitempty"`
	McapYi float64 `json:"mcapYi,omitempty"`
}

// Provider 聚合多数据源。线程安全：并发读快照时用 RWMutex。
type Provider struct {
	tv     *dataflows.TVRestClient
	yf     *dataflows.YFinanceClient
	http   *http.Client
	mu     sync.RWMutex
	usSnap map[string]snapshotQuote
	cnSnap map[string]snapshotQuote
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
	return &Provider{
		tv:     dataflows.NewTVRestClient(),
		yf:     dataflows.NewYFinanceClient(),
		http:   &http.Client{Timeout: 12 * time.Second},
		usSnap: map[string]snapshotQuote{},
		cnSnap: map[string]snapshotQuote{},
	}
}

// ReloadSnapshots 把本地快照装进内存，供 Quotes 最后一档兜底。
// 美股优先 us-stocks.json（与 /api/stocks 同源），再回写 market.json。
func (p *Provider) ReloadSnapshots() {
	us, _, err := usQuotesFromUsStocks()
	if err != nil || len(us) == 0 {
		us = readSnapshotQuotes("market.json")
	} else {
		_ = writeMarketSnapshotFile(us)
	}
	cn := readSnapshotQuotes("a-market.json")
	p.mu.Lock()
	defer p.mu.Unlock()
	if us != nil {
		p.usSnap = us
	}
	if cn != nil {
		p.cnSnap = cn
	}
}

func readSnapshotQuotes(name string) map[string]snapshotQuote {
	raw, err := os.ReadFile(filepath.Join("data", name))
	if err != nil {
		raw, err = os.ReadFile(filepath.Join("app", "backend", "data", name))
	}
	if err != nil || len(raw) == 0 {
		return nil
	}
	var payload struct {
		Quotes map[string]snapshotQuote `json:"quotes"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Quotes == nil {
		return nil
	}
	return payload.Quotes
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

	// ① TradingView（美股 ticker 优先）
	usBatch := make([]string, 0, len(pending))
	for _, s := range pending {
		if isLikelyUSSymbol(s) {
			usBatch = append(usBatch, s)
		}
	}
	if len(usBatch) > 0 {
		if items, err := p.tv.GetRealTimeQuotes(usBatch); err == nil {
			byBase := map[string]dataflows.TVQuoteData{}
			for _, item := range items {
				base := item.Symbol
				if i := strings.LastIndex(base, ":"); i >= 0 {
					base = base[i+1:]
				}
				byBase[strings.ToUpper(base)] = item
			}
			still := pending[:0]
			for _, s := range pending {
				if item, ok := byBase[strings.ToUpper(s)]; ok && item.Price > 0 {
					pct := item.ChangePct
					prev := item.PrevClose
					if prev <= 0 && pct != -100 {
						prev = item.Price / (1.0 + pct/100.0)
					}
					out[s] = Quote{Price: item.Price, Pct: pct, Session: "regular", PrevClose: prev, Source: "tradingview"}
				} else {
					still = append(still, s)
				}
			}
			pending = still
		}
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

	// ③ 本地快照兜底（绝不编造价格）
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, s := range pending {
		if q, ok := p.usSnap[s]; ok && q.Price > 0 {
			prev := q.Price
			if q.Pct != -100 {
				prev = q.Price / (1.0 + q.Pct/100.0)
			}
			out[s] = Quote{Price: q.Price, Pct: q.Pct, Session: "regular", PrevClose: prev, Source: "local-us"}
			continue
		}
		if q, ok := p.cnSnap[s]; ok && q.Price > 0 {
			prev := q.Price
			if q.Pct != -100 {
				prev = q.Price / (1.0 + q.Pct/100.0)
			}
			out[s] = Quote{Price: q.Price, Pct: q.Pct, Session: "regular", PrevClose: prev, Source: "local-cn"}
		}
	}
	return out
}

func isLikelyUSSymbol(sym string) bool {
	if strings.Contains(sym, ":") {
		return true
	}
	if strings.Contains(sym, ".") { // 0700.HK, 2330.TW, BRK.B edge: allow BRK.B via Yahoo
		parts := strings.Split(sym, ".")
		if len(parts) == 2 {
			suf := strings.ToUpper(parts[1])
			if suf == "HK" || suf == "TW" || suf == "KS" || suf == "SS" || suf == "SZ" || suf == "T" {
				return false
			}
		}
	}
	// pure A-share codes
	if len(sym) == 6 {
		allDigit := true
		for _, r := range sym {
			if r < '0' || r > '9' {
				allDigit = false
				break
			}
		}
		if allDigit {
			return false
		}
	}
	return true
}

func toYahooSymbol(sym string) string {
	s := strings.TrimSpace(sym)
	if s == "" {
		return s
	}
	if strings.Contains(s, ".") || strings.Contains(s, ":") {
		return strings.ReplaceAll(s, ":", ".")
	}
	if len(s) == 6 {
		allDigit := true
		for _, r := range s {
			if r < '0' || r > '9' {
				allDigit = false
				break
			}
		}
		if allDigit {
			if strings.HasPrefix(s, "6") || strings.HasPrefix(s, "5") {
				return s + ".SS"
			}
			return s + ".SZ"
		}
	}
	return s
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
				Symbol                string  `json:"symbol"`
				RegularMarketPrice    float64 `json:"regularMarketPrice"`
				RegularMarketChangePct float64 `json:"regularMarketChangePercent"`
				RegularMarketPreviousClose float64 `json:"regularMarketPreviousClose"`
				MarketState           string  `json:"marketState"`
			} `json:"result"`
		} `json:"quoteResponse"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	out := make(map[string]Quote, len(parsed.QuoteResponse.Result))
	for _, r := range parsed.QuoteResponse.Result {
		if r.RegularMarketPrice <= 0 {
			continue
		}
		session := "regular"
		if r.MarketState != "" {
			session = strings.ToLower(r.MarketState)
		}
		out[r.Symbol] = Quote{
			Price:     r.RegularMarketPrice,
			Pct:       r.RegularMarketChangePct,
			Session:   session,
			PrevClose: r.RegularMarketPreviousClose,
			Source:    "yahoo",
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
	if price <= 0 {
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
	}, nil
}

// SnapshotPayload returns local market snapshot bytes for /api/market or /api/a-market.
// US prefers a live-derived payload from us-stocks.json (same prices as /api/stocks).
func SnapshotPayload(kind string) ([]byte, string, error) {
	if kind == "cn" || kind == "a" {
		raw, err := readDataFile("a-market.json")
		if err != nil {
			return nil, "local-cn-snapshot", err
		}
		return raw, "local-cn-snapshot", nil
	}

	quotes, generatedAt, err := usQuotesFromUsStocks()
	if err == nil && len(quotes) > 0 {
		_ = writeMarketSnapshotFile(quotes)
		raw, marshalErr := json.Marshal(marketSnapshotPayload{
			Quotes:  quotes,
			Ts:      time.Now().UnixMilli(),
			Count:   len(quotes),
			Session: "regular",
		})
		if marshalErr != nil {
			return nil, "", marshalErr
		}
		src := "us-stocks.json"
		if generatedAt != "" {
			src = "us-stocks@" + generatedAt
		}
		return raw, src, nil
	}

	raw, err := readDataFile("market.json")
	if err != nil {
		return nil, "local-us-snapshot", err
	}
	return raw, "local-us-snapshot", nil
}

type marketSnapshotPayload struct {
	Quotes  map[string]snapshotQuote `json:"quotes"`
	Ts      int64                    `json:"ts"`
	Count   int                      `json:"count"`
	Session string                   `json:"session"`
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

func writeMarketSnapshotFile(quotes map[string]snapshotQuote) error {
	payload := marketSnapshotPayload{
		Quotes:  quotes,
		Ts:      time.Now().UnixMilli(),
		Count:   len(quotes),
		Session: "regular",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	paths := []string{
		filepath.Join("data", "market.json"),
		filepath.Join("app", "backend", "data", "market.json"),
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

func readDataFile(name string) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join("data", name))
	if err != nil {
		raw, err = os.ReadFile(filepath.Join("app", "backend", "data", name))
	}
	return raw, err
}
