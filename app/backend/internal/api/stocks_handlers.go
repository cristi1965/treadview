package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"trading-agents/internal/market"
	"trading-agents/internal/models"
	"trading-agents/internal/scoring"
)

// stocksUniverseCache：进程内缓存，避免每次请求都解析上万行 JSON。
// mu 用 RWMutex：多请求可并行读，写时独占。
type stocksUniverseCache struct {
	mu        sync.RWMutex
	loadedAt  time.Time
	stocks    []models.Stock
	bySymbol  map[string]models.Stock
	source    string
	generated string
}

var liveStocksCache stocksUniverseCache // 美股
var cnStocksCache stocksUniverseCache   // A 股

const stocksCacheTTL = 60 * time.Second

// GetStocks 处理 GET /api/stocks。
//
// 流程：绑定 query → 按 market 加载宇宙 → 过滤/排序 → 分页 → JSON。
// 关键：market=cn 必须走 a-market，不能返回美股列表。
func (h *Handler) GetStocks(c *gin.Context) {
	var req models.StocksListRequest
	// ShouldBindQuery：把 ?market=us&limit=50 填进结构体；失败返回 400
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Market == "" {
		req.Market = "us"
	}
	marketCode, err := normalizeStocksMarket(req.Market)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Market = marketCode
	if req.Sort == "" {
		req.Sort = "marketcap"
	}
	if req.Order == "" {
		req.Order = "desc"
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 || req.Limit > 200 {
		req.Limit = 50
	}

	stocks, meta, err := loadUniverseStocks(req.Market)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if req.Search != "" {
		stocks = filterStocks(stocks, req.Search)
	}

	if req.MinScore != nil || req.MaxScore != nil {
		filtered := make([]models.Stock, 0, len(stocks))
		for _, stock := range stocks {
			if req.MinScore != nil && stock.AvgScore < float64(*req.MinScore) {
				continue
			}
			if req.MaxScore != nil && stock.AvgScore > float64(*req.MaxScore) {
				continue
			}
			filtered = append(filtered, stock)
		}
		stocks = filtered
	}

	stocks = sortStocks(stocks, req.Sort, req.Order)

	total := len(stocks)
	start := (req.Page - 1) * req.Limit
	end := start + req.Limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	page := stocks[start:end]
	liveTimedOut := false
	var usQuotes quoteFetchResult
	if strings.EqualFold(req.Market, "us") {
		page, usQuotes = enrichStocksCachedQuotes(page)
	} else if isCNMarket(req.Market) {
		page, liveTimedOut = enrichStocksLiveQuotesStatus(page)
	}

	// X-Data-Source 方便验收时看清数据来自哪份快照 / live
	metaOut := meta
	dataTime := dataTimeFromSource(meta)
	stale := true
	staleReason := "live quotes unavailable; serving last real snapshot"
	if strings.EqualFold(req.Market, "us") {
		dataTime = usQuotes.dataTimeLabel
		stale, staleReason = quoteResultFreshness(usQuotes, time.Now())
		trustedCount := trustedQuoteCoverage(usQuotes.quotes)
		complete := len(page) == trustedCount && !strings.HasPrefix(usQuotes.source, "stale-snapshot:") && usQuotes.source != "unverified-provider-time"
		if !complete {
			stale = true
			staleReason = fmt.Sprintf("cached provider coverage is incomplete (%d/%d); serving real snapshot values for the remainder", trustedCount, len(page))
		}
		state := "stale-cached"
		if complete && staleReason == "market session is closed; quote is the last provider observation" {
			state = "closed-" + usQuotes.source
		} else if complete && !stale {
			state = "live-" + usQuotes.source
		}
		metaOut = meta + "+" + state
		if dataTime != "" {
			metaOut += "@" + dataTime
		}
	}
	if isCNMarket(req.Market) {
		if at := market.Default().CNLiveUpdatedAt(); !at.IsZero() {
			dataTime = at.UTC().Format(time.RFC3339)
			stale, staleReason = marketQuoteFreshness("live", at, time.Now())
			if stale {
				metaOut = meta + "+stale-live@" + at.UTC().Format(time.RFC3339)
			} else {
				metaOut = meta + "+live@" + at.UTC().Format(time.RFC3339)
			}
		}
	}
	if liveTimedOut {
		metaOut = meta + "+stale-live-timeout"
		dataTime = dataTimeOrEmpty(market.Default().SnapshotUpdatedAt())
		stale = true
		staleReason = "live quote provider timed out; serving last real snapshot"
	}
	if stale && staleReason == "" {
		staleReason = "live quotes unavailable; serving last real snapshot"
	}
	setDataFreshness(c, dataFreshnessMeta{
		Source:      metaOut,
		DataTime:    dataTime,
		Stale:       stale,
		StaleReason: staleReason,
		Refreshable: true,
	})
	c.JSON(http.StatusOK, models.StocksListResponse{
		Stocks:  page,
		Total:   total,
		Page:    req.Page,
		Limit:   req.Limit,
		HasMore: end < total,
	})
}

// SearchStocks handles GET /api/stocks/search
func (h *Handler) SearchStocks(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}
	marketCode, err := normalizeStocksMarket(c.Query("market"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stocks, meta, err := loadUniverseStocks(marketCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	filtered := filterStocks(stocks, query)
	// Rank: exact symbol → prefix symbol → name contains
	qUpper := strings.ToUpper(query)
	sort.SliceStable(filtered, func(i, j int) bool {
		si, sj := strings.ToUpper(filtered[i].Symbol), strings.ToUpper(filtered[j].Symbol)
		score := func(sym, name string) int {
			if sym == qUpper {
				return 0
			}
			if strings.HasPrefix(sym, qUpper) {
				return 1
			}
			if strings.Contains(strings.ToUpper(name), qUpper) {
				return 2
			}
			return 3
		}
		ai, aj := score(si, filtered[i].Name), score(sj, filtered[j].Name)
		if ai != aj {
			return ai < aj
		}
		return filtered[i].MarketCap > filtered[j].MarketCap
	})

	limit := 20
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	setDataFreshness(c, dataFreshnessMeta{
		Source:      meta,
		DataTime:    dataTimeFromSource(meta),
		Stale:       true,
		StaleReason: "search uses last real universe snapshot",
		Refreshable: true,
	})
	c.JSON(http.StatusOK, models.StockSearchResponse{
		Results: filtered,
		Count:   len(filtered),
	})
}

func filterStocks(stocks []models.Stock, query string) []models.Stock {
	searchLower := strings.ToLower(strings.TrimSpace(query))
	if searchLower == "" {
		return stocks
	}
	out := make([]models.Stock, 0, 64)
	for _, stock := range stocks {
		if strings.Contains(strings.ToLower(stock.Symbol), searchLower) ||
			strings.Contains(strings.ToLower(stock.Name), searchLower) ||
			strings.Contains(strings.ToLower(stock.Sector), searchLower) {
			out = append(out, stock)
		}
	}
	return out
}

func sortStocks(stocks []models.Stock, sortBy, order string) []models.Stock {
	sorted := make([]models.Stock, len(stocks))
	copy(sorted, stocks)
	desc := order != "asc"

	sort.SliceStable(sorted, func(i, j int) bool {
		var vi, vj float64
		switch sortBy {
		case "price":
			vi, vj = sorted[i].Price, sorted[j].Price
		case "change":
			vi, vj = sorted[i].ChangePercent, sorted[j].ChangePercent
		case "avgscore":
			vi, vj = sorted[i].AvgScore, sorted[j].AvgScore
		case "volume":
			vi, vj = float64(sorted[i].Volume), float64(sorted[j].Volume)
		default:
			vi, vj = sorted[i].MarketCap, sorted[j].MarketCap
		}
		if vi == vj {
			return sorted[i].Symbol < sorted[j].Symbol
		}
		if desc {
			return vi > vj
		}
		return vi < vj
	})
	return sorted
}

func enrichStocksLiveQuotes(stocks []models.Stock) []models.Stock {
	out, _ := enrichStocksLiveQuotesStatus(stocks)
	return out
}

func enrichStocksCachedQuotes(stocks []models.Stock) ([]models.Stock, quoteFetchResult) {
	if len(stocks) == 0 {
		return stocks, quoteFetchResult{quotes: map[string]market.Quote{}, source: "stale-snapshot:us"}
	}
	symbols := make([]string, 0, len(stocks))
	for _, stock := range stocks {
		symbols = append(symbols, stock.Symbol)
	}
	provider := market.Default()
	quotes := provider.CachedQuotes(symbols)
	result := newQuoteFetchResult(provider, quotes, false, time.Now())
	out := make([]models.Stock, len(stocks))
	copy(out, stocks)
	for index := range out {
		quote, ok := quotes[out[index].Symbol]
		if !ok || quote.Price <= 0 {
			continue
		}
		if out[index].Price > 0 && out[index].MarketCap > 0 {
			out[index].MarketCap *= quote.Price / out[index].Price
		}
		out[index].Price = quote.Price
		out[index].ChangePercent = quote.Pct
		out[index].Change = quote.Price * quote.Pct / 100
	}
	return out, result
}

func trustedQuoteCoverage(quotes map[string]market.Quote) int {
	count := 0
	for _, quote := range quotes {
		if quote.Price <= 0 || isStaleQuoteSource(quote.Source) || strings.TrimSpace(quote.DataTime) == "" {
			continue
		}
		if _, _, err := parseQuoteObservation(quote.DataTime, quote.TimeGranularity); err == nil {
			count++
		}
	}
	return count
}

func enrichStocksLiveQuotesStatus(stocks []models.Stock) ([]models.Stock, bool) {
	if len(stocks) == 0 {
		return stocks, false
	}
	syms := make([]string, 0, len(stocks))
	for _, s := range stocks {
		syms = append(syms, s.Symbol)
	}
	live, timedOut := boundedQuotes(syms)
	if len(live) == 0 {
		return stocks, timedOut
	}
	out := make([]models.Stock, len(stocks))
	copy(out, stocks)
	for i := range out {
		q, ok := live[out[i].Symbol]
		if !ok || q.Price <= 0 {
			continue
		}
		if out[i].Price > 0 && out[i].MarketCap > 0 {
			out[i].MarketCap = out[i].MarketCap * (q.Price / out[i].Price)
		}
		out[i].Price = q.Price
		out[i].ChangePercent = q.Pct
		out[i].Change = q.Price * q.Pct / 100
	}
	return out, timedOut
}

func loadUniverseStocks(marketCode string) ([]models.Stock, string, error) {
	if isCNMarket(marketCode) {
		return loadCNUniverseStocks()
	}
	return loadUSUniverseStocks()
}

func normalizeStocksMarket(market string) (string, error) {
	m := strings.ToLower(strings.TrimSpace(market))
	if m == "" || m == "us" {
		return "us", nil
	}
	if m == "cn" || m == "a" {
		return "cn", nil
	}
	return "", fmt.Errorf("unsupported market %q; supported values: us, cn", market)
}

func isCNMarket(market string) bool {
	m := strings.ToLower(strings.TrimSpace(market))
	return m == "cn" || m == "a"
}

func loadCNUniverseStocks() ([]models.Stock, string, error) {
	cnStocksCache.mu.RLock()
	if time.Since(cnStocksCache.loadedAt) < stocksCacheTTL && len(cnStocksCache.stocks) > 0 {
		out := cnStocksCache.stocks
		src := cnStocksCache.source
		cnStocksCache.mu.RUnlock()
		return out, src, nil
	}
	cnStocksCache.mu.RUnlock()

	cnStocksCache.mu.Lock()
	defer cnStocksCache.mu.Unlock()
	if time.Since(cnStocksCache.loadedAt) < stocksCacheTTL && len(cnStocksCache.stocks) > 0 {
		return cnStocksCache.stocks, cnStocksCache.source, nil
	}

	raw, err := os.ReadFile(filepath.Join("data", "a-market.json"))
	if err != nil {
		raw, err = os.ReadFile(filepath.Join("app", "backend", "data", "a-market.json"))
	}
	if err != nil {
		root := findBackendRootForStocks()
		raw, err = os.ReadFile(filepath.Join(root, "data", "a-market.json"))
	}
	if err != nil {
		return nil, "", err
	}

	out, bySym, src, err := parseAMarketStocks(raw)
	if err != nil {
		return nil, "", err
	}
	cnStocksCache.stocks = out
	cnStocksCache.bySymbol = bySym
	cnStocksCache.source = src
	cnStocksCache.loadedAt = time.Now()
	return out, src, nil
}

func parseAMarketStocks(raw []byte) ([]models.Stock, map[string]models.Stock, string, error) {
	var payload struct {
		Quotes map[string]struct {
			Price  float64 `json:"price"`
			Pct    float64 `json:"pct"`
			Vol    float64 `json:"vol"`
			McapYi float64 `json:"mcapYi"`
		} `json:"quotes"`
		Count int   `json:"count"`
		Ts    int64 `json:"ts"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, nil, "", err
	}
	if len(payload.Quotes) == 0 {
		return nil, nil, "", fmt.Errorf("a-market.json has no quotes")
	}

	out := make([]models.Stock, 0, len(payload.Quotes))
	bySym := make(map[string]models.Stock, len(payload.Quotes))
	for sym, q := range payload.Quotes {
		sym = strings.TrimSpace(sym)
		if sym == "" || q.Price <= 0 {
			continue
		}
		st := models.Stock{
			Symbol:        sym,
			Name:          sym,
			Price:         q.Price,
			Change:        q.Price * q.Pct / 100.0,
			ChangePercent: q.Pct,
			MarketCap:     q.McapYi * 1e8, // 亿元 → 元
			Volume:        int64(q.Vol),
			Sector:        "A股",
		}
		out = append(out, st)
		bySym[sym] = st
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].MarketCap > out[j].MarketCap
	})

	src := "a-market.json"
	if payload.Ts > 0 {
		src = fmt.Sprintf("a-market@%d", payload.Ts)
	}
	return out, bySym, src, nil
}

func loadUSUniverseStocks() ([]models.Stock, string, error) {
	liveStocksCache.mu.RLock()
	if time.Since(liveStocksCache.loadedAt) < stocksCacheTTL && len(liveStocksCache.stocks) > 0 {
		out := liveStocksCache.stocks
		src := liveStocksCache.source
		liveStocksCache.mu.RUnlock()
		return out, src, nil
	}
	liveStocksCache.mu.RUnlock()

	liveStocksCache.mu.Lock()
	defer liveStocksCache.mu.Unlock()
	// double-check
	if time.Since(liveStocksCache.loadedAt) < stocksCacheTTL && len(liveStocksCache.stocks) > 0 {
		return liveStocksCache.stocks, liveStocksCache.source, nil
	}

	root := findBackendRootForStocks()
	us, usPath, err := scoring.LoadUsStocks(append(
		scoring.DefaultUsStocksPaths(root),
		filepath.Join(root, "..", "frontend", "dist", "data", "us-stocks.json"),
	)...)
	if err != nil {
		return nil, "", err
	}

	panel, _, _ := scoring.LoadPanelFile(append(
		scoring.DefaultPanelPaths(root),
		filepath.Join(root, "..", "frontend", "dist", "data", "us-panel-summary.json"),
		filepath.Join(root, "..", "frontend", "public", "data", "us-panel-summary.json"),
	)...)

	order := scoring.PanelOrder
	if panel != nil && len(panel.Order) == 5 {
		order = panel.Order
	}

	out := make([]models.Stock, 0, len(us.Stocks))
	bySym := make(map[string]models.Stock, len(us.Stocks))
	for _, row := range us.Stocks {
		sym := strings.TrimSpace(strings.ToUpper(row.Sym))
		if sym == "" {
			continue
		}
		scores := models.FiveFactorScores{}
		avg := 0.0
		var div *float64
		if panel != nil {
			if ps, ok := panel.Stocks[sym]; ok && len(ps.SC) >= 5 {
				scores = mapPanelScores(order, ps.SC)
				sum := 0
				for _, v := range ps.SC[:5] {
					sum += v
				}
				avg = float64(sum) / 5.0
				d := float64(ps.Div)
				div = &d
			}
		}
		sector := row.Sector
		if sector == "" {
			sector = row.Seg
		}
		st := models.Stock{
			Symbol:        sym,
			Name:          row.Name,
			Price:         row.Price,
			Change:        row.Price * row.Pct / 100.0,
			ChangePercent: row.Pct,
			MarketCap:     row.McapB * 1e9,
			Volume:        int64(row.Vol),
			Sector:        sector,
			Scores:        scores,
			AvgScore:      avg,
			Divergence:    div,
		}
		out = append(out, st)
		bySym[sym] = st
	}

	src := "us-stocks.json"
	if us.GeneratedAt != "" {
		src = "us-stocks@" + us.GeneratedAt
	}
	_ = usPath

	liveStocksCache.stocks = out
	liveStocksCache.bySymbol = bySym
	liveStocksCache.source = src
	liveStocksCache.generated = us.GeneratedAt
	liveStocksCache.loadedAt = time.Now()
	return out, src, nil
}

func mapPanelScores(order []string, sc []int) models.FiveFactorScores {
	out := models.FiveFactorScores{}
	for i, key := range order {
		if i >= len(sc) {
			break
		}
		v := sc[i]
		switch key {
		case "buffett":
			out.Buffett = v
		case "duan", "duanyongping":
			out.Duanyongping = v
		case "serenity":
			out.Serenity = v
		case "druckenmiller":
			out.Druckenmiller = v
		case "sentiment":
			out.Sentiment = v
		}
	}
	return out
}

func findBackendRootForStocks() string {
	candidates := []string{".", "app/backend", "../backend"}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "go.mod")); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	abs, _ := filepath.Abs(".")
	return abs
}
