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

	"trading-agents/internal/market"

	"github.com/gin-gonic/gin"
)

type cnEtfRecSeed struct {
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Theme    string `json:"theme"`
	Why      string `json:"why"`
	Risk     string `json:"risk"`
	Priority int    `json:"priority"`
}

type cnConvictionSeed struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
	Tag    string `json:"tag"`
	Why    string `json:"why"`
}

type cnPicksFile struct {
	Updated        string             `json:"updated"`
	Disclaimer     string             `json:"disclaimer"`
	EtfRecs        []cnEtfRecSeed     `json:"etf_recs"`
	ConvictionSeed []cnConvictionSeed `json:"conviction_seed"`
}

var (
	cnPicksOnce sync.Once
	cnPicksData cnPicksFile
)

func loadCNPicks() cnPicksFile {
	cnPicksOnce.Do(func() {
		paths := []string{
			filepath.Join("data", "cn-picks.json"),
			filepath.Join("app", "backend", "data", "cn-picks.json"),
		}
		for _, p := range paths {
			raw, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			var f cnPicksFile
			if json.Unmarshal(raw, &f) == nil && len(f.EtfRecs) > 0 {
				cnPicksData = f
				return
			}
		}
		cnPicksData = cnPicksFile{Disclaimer: "非投资建议", EtfRecs: nil}
	})
	return cnPicksData
}

type cnQuoteLite struct {
	Price  float64
	Pct    float64
	McapYi float64
	Vol    float64
}

func loadCNLiveMap() map[string]cnQuoteLite {
	out := map[string]cnQuoteLite{}
	p := market.Default()
	for sym, q := range p.CNQuotesCopy() {
		if q.Price <= 0 {
			continue
		}
		out[sym] = cnQuoteLite{Price: q.Price, Pct: q.Pct, McapYi: q.McapYi, Vol: q.Vol}
	}
	return out
}

// fillMissingCNQuotes pulls Eastmoney quotes for symbols absent from a-market snapshot
// (e.g. recommended ETFs not in the stock universe seed). Uses a short TTL cache
// and upserts hits into cnSnap for subsequent refresh cycles.
func fillMissingCNQuotes(quotes map[string]cnQuoteLite, syms []string) {
	missing := make([]string, 0)
	seen := map[string]struct{}{}
	for _, s := range syms {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := quotes[s]; ok && quotes[s].Price > 0 {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		missing = append(missing, s)
	}
	if len(missing) == 0 {
		return
	}

	// Serve from TTL cache first (high availability under bursty UI loads).
	still := make([]string, 0, len(missing))
	cnQuoteCacheMu.RLock()
	now := time.Now()
	for _, s := range missing {
		if e, ok := cnQuoteCache[s]; ok && now.Before(e.expires) && e.q.Price > 0 {
			quotes[s] = e.q
			continue
		}
		still = append(still, s)
	}
	cnQuoteCacheMu.RUnlock()
	if len(still) == 0 {
		return
	}

	p := market.Default()
	p.EnsureCNSymbols(still)
	fetched, _ := boundedQuotes(still)
	upsert := map[string]market.Quote{}
	cnQuoteCacheMu.Lock()
	for sym, q := range fetched {
		if q.Price <= 0 {
			continue
		}
		lite := cnQuoteLite{Price: q.Price, Pct: q.Pct}
		quotes[sym] = lite
		cnQuoteCache[sym] = cnQuoteCacheEntry{q: lite, expires: now.Add(cnQuoteCacheTTL)}
		upsert[sym] = q
	}
	cnQuoteCacheMu.Unlock()
	p.UpsertCNQuotes(upsert)
}

var (
	cnQuoteCacheMu sync.RWMutex
	cnQuoteCache   = map[string]cnQuoteCacheEntry{}
	cnPriorityMu   sync.Mutex
	cnPriorityTry  time.Time
	cnPriorityOKAt time.Time
)

const cnQuoteCacheTTL = 45 * time.Second

type cnQuoteCacheEntry struct {
	q       cnQuoteLite
	expires time.Time
}

const cnDynamicMaxAge = 7 * 24 * time.Hour

func ensureCNPriorityFresh() bool {
	cnPriorityMu.Lock()
	defer cnPriorityMu.Unlock()
	if !cnPriorityOKAt.IsZero() && time.Since(cnPriorityOKAt) < 2*time.Minute {
		return true
	}
	now := time.Now()
	if !cnPriorityTry.IsZero() && now.Sub(cnPriorityTry) < 15*time.Second {
		return false
	}
	cnPriorityTry = now
	if _, err := market.Default().RefreshCNPriorityQuotes(); err != nil {
		return false
	}
	cnPriorityOKAt = time.Now()
	return true
}

func cnQuoteCoverage(quotes map[string]cnQuoteLite, symbols []string) (quoted, required int) {
	seen := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		required++
		if quote, ok := quotes[symbol]; ok && quote.Price > 0 {
			quoted++
		}
	}
	return quoted, required
}

func cnDynamicFreshness(liveAt time.Time, quoted, required int, now time.Time) (dataTime string, stale bool, reason string) {
	if liveAt.IsZero() {
		return "unknown", true, "live CN quote refresh time is unavailable"
	}
	dataTime = liveAt.UTC().Format(time.RFC3339)
	if required <= 0 || quoted < required {
		return dataTime, true, fmt.Sprintf("live CN quote coverage incomplete: %d/%d", quoted, required)
	}
	age := now.Sub(liveAt)
	if age > cnDynamicMaxAge {
		return dataTime, true, fmt.Sprintf("live CN quotes age %.1fh exceeds %.1fh", age.Hours(), cnDynamicMaxAge.Hours())
	}
	return dataTime, false, ""
}

type pulseLite struct {
	Ticker     string
	Name       string
	Heat       float64
	Moat       int
	Industries []string
	Layer      string
	Segment    string
	MarketCapB float64
}

func loadCNPulseLite() []pulseLite {
	f, err := loadPulseFile()
	if err != nil {
		return nil
	}
	out := make([]pulseLite, 0, len(f.Companies)/2)
	for _, c := range f.Companies {
		if !strings.EqualFold(c.Region, "CN") {
			continue
		}
		moat := 0
		if c.Moat != nil {
			moat = *c.Moat
		}
		out = append(out, pulseLite{
			Ticker:     c.Ticker,
			Name:       c.Name,
			Heat:       c.Heat,
			Moat:       moat,
			Industries: c.Industries,
			Layer:      c.Layer,
			Segment:    c.Segment,
			MarketCapB: c.MarketCapB,
		})
	}
	return out
}

// GetCNPicks GET /api/cn/picks — A-share ETF recs + hot names + high-conviction screen.
func GetCNPicks(c *gin.Context) {
	seed := loadCNPicks()
	priorityFresh := ensureCNPriorityFresh()
	quotes := loadCNLiveMap()
	needQuote := make([]string, 0, len(seed.EtfRecs)+len(seed.ConvictionSeed))
	for _, e := range seed.EtfRecs {
		needQuote = append(needQuote, e.Symbol)
	}
	for _, s := range seed.ConvictionSeed {
		needQuote = append(needQuote, s.Symbol)
	}
	fillMissingCNQuotes(quotes, needQuote)

	pulse := loadCNPulseLite()
	pulseBy := map[string]pulseLite{}
	for _, p := range pulse {
		pulseBy[p.Ticker] = p
	}

	type etfRow struct {
		Symbol   string  `json:"symbol"`
		Name     string  `json:"name"`
		Theme    string  `json:"theme"`
		Why      string  `json:"why"`
		Risk     string  `json:"risk"`
		Priority int     `json:"priority"`
		Price    float64 `json:"price,omitempty"`
		Pct      float64 `json:"pct,omitempty"`
		HasQuote bool    `json:"hasQuote"`
	}
	etfs := make([]etfRow, 0, len(seed.EtfRecs))
	for _, e := range seed.EtfRecs {
		row := etfRow{
			Symbol: e.Symbol, Name: e.Name, Theme: e.Theme, Why: e.Why,
			Risk: e.Risk, Priority: e.Priority,
		}
		if q, ok := quotes[e.Symbol]; ok && q.Price > 0 {
			row.Price = q.Price
			row.Pct = q.Pct
			row.HasQuote = true
		}
		etfs = append(etfs, row)
	}
	sort.SliceStable(etfs, func(i, j int) bool {
		if etfs[i].Priority != etfs[j].Priority {
			return etfs[i].Priority < etfs[j].Priority
		}
		return etfs[i].Symbol < etfs[j].Symbol
	})

	type stockRow struct {
		Symbol     string   `json:"symbol"`
		Name       string   `json:"name"`
		Price      float64  `json:"price,omitempty"`
		Pct        float64  `json:"pct,omitempty"`
		McapYi     float64  `json:"mcapYi,omitempty"`
		Heat       float64  `json:"heat,omitempty"`
		Moat       int      `json:"moat,omitempty"`
		Tag        string   `json:"tag,omitempty"`
		Why        string   `json:"why,omitempty"`
		Layer      string   `json:"layer,omitempty"`
		Industries []string `json:"industries,omitempty"`
		HasQuote   bool     `json:"hasQuote"`
		Rule       string   `json:"rule,omitempty"`
	}

	// Hot: prefer pulse CN with live quotes, rank by |pct| * log(mcap) then heat
	type hotCand struct {
		stockRow
		score float64
	}
	hots := make([]hotCand, 0)
	for _, p := range pulse {
		q, ok := quotes[p.Ticker]
		if !ok || q.Price <= 0 {
			continue
		}
		absPct := q.Pct
		if absPct < 0 {
			absPct = -absPct
		}
		// require meaningful move or high heat
		if absPct < 2 && p.Heat < 70 {
			continue
		}
		mcap := q.McapYi
		if mcap <= 0 {
			mcap = p.MarketCapB * 10
		}
		sc := absPct*3 + p.Heat*0.15
		if mcap > 200 {
			sc += 8
		} else if mcap > 50 {
			sc += 4
		}
		name := p.Name
		if name == "" || name == p.Ticker {
			name = p.Ticker
		}
		tag := "热门"
		if q.Pct >= 5 {
			tag = "强势"
		} else if q.Pct <= -5 {
			tag = "大跌关注"
		}
		hots = append(hots, hotCand{
			stockRow: stockRow{
				Symbol: p.Ticker, Name: name, Price: q.Price, Pct: q.Pct, McapYi: mcap,
				Heat: p.Heat, Moat: p.Moat, Tag: tag, Layer: p.Layer, Industries: p.Industries,
				HasQuote: true, Rule: "脉冲宇宙 ∩ live涨跌幅/热度",
			},
			score: sc,
		})
	}
	sort.Slice(hots, func(i, j int) bool { return hots[i].score > hots[j].score })
	hotOut := make([]stockRow, 0, 12)
	for i := 0; i < len(hots) && i < 12; i++ {
		hotOut = append(hotOut, hots[i].stockRow)
	}

	// High conviction screen from pulse: moat>=4 and heat in [40,72]
	conv := make([]stockRow, 0)
	for _, p := range pulse {
		if p.Moat < 4 {
			continue
		}
		if p.Heat < 40 || p.Heat > 72 {
			continue
		}
		name := p.Name
		if name == "" {
			name = p.Ticker
		}
		row := stockRow{
			Symbol: p.Ticker, Name: name, Heat: p.Heat, Moat: p.Moat,
			Tag: "高信念", Why: "护城河≥4 且过热度处合理区间（非极端泡沫）",
			Layer: p.Layer, Industries: p.Industries,
			Rule: "moat≥4 ∩ heat 40–72",
		}
		if q, ok := quotes[p.Ticker]; ok && q.Price > 0 {
			row.Price = q.Price
			row.Pct = q.Pct
			row.McapYi = q.McapYi
			row.HasQuote = true
		}
		conv = append(conv, row)
	}
	sort.Slice(conv, func(i, j int) bool {
		if conv[i].Moat != conv[j].Moat {
			return conv[i].Moat > conv[j].Moat
		}
		return conv[i].Heat < conv[j].Heat // cooler among strong moats first
	})
	if len(conv) > 15 {
		conv = conv[:15]
	}

	// Seed conviction names (may be outside pulse) — always surface with live if any
	seedConv := make([]stockRow, 0, len(seed.ConvictionSeed))
	for _, s := range seed.ConvictionSeed {
		row := stockRow{Symbol: s.Symbol, Name: s.Name, Tag: s.Tag, Why: s.Why, Rule: "人工精选种子"}
		if p, ok := pulseBy[s.Symbol]; ok {
			row.Heat = p.Heat
			row.Moat = p.Moat
			row.Layer = p.Layer
			row.Industries = p.Industries
			if row.Name == "" {
				row.Name = p.Name
			}
		}
		if q, ok := quotes[s.Symbol]; ok && q.Price > 0 {
			row.Price = q.Price
			row.Pct = q.Pct
			row.McapYi = q.McapYi
			row.HasQuote = true
		}
		seedConv = append(seedConv, row)
	}

	quoted, required := cnQuoteCoverage(quotes, needQuote)
	dataTime, stale, staleReason := cnDynamicFreshness(market.Default().CNLiveUpdatedAt(), quoted, required, time.Now())
	if !priorityFresh {
		stale = true
		staleReason = "priority CN quote refresh did not complete"
	}
	setDataFreshness(c, dataFreshnessMeta{
		Source:      fmt.Sprintf("cn-picks-config@%s+live-quotes", seed.Updated),
		DataTime:    dataTime,
		Stale:       stale,
		StaleReason: staleReason,
		Refreshable: true,
	})
	c.JSON(http.StatusOK, gin.H{
		"disclaimer":    seed.Disclaimer,
		"updated":       seed.Updated,
		"configUpdated": seed.Updated,
		"etf_recs":      etfs,
		"hot":           hotOut,
		"conviction":    conv,
		"seed":          seedConv,
		"rules": gin.H{
			"hot":        "A股脉冲宇宙 ∩ 实时行情；按 |涨跌幅|、热度、市值加权",
			"conviction": "护城河 moat≥4 且过热度 heat∈[40,72]（好生意 + 未极端泡沫）",
			"seed":       "人工维护的高辨识度龙头清单，供对照",
			"etf":        "宽基/科技/主题工具箱；优先流动性与叙事清晰度",
		},
		"counts": gin.H{
			"etf":        len(etfs),
			"hot":        len(hotOut),
			"conviction": len(conv),
			"seed":       len(seedConv),
			"cn_quotes":  len(quotes),
			"cn_pulse":   len(pulse),
		},
	})
}
