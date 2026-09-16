package dataflows

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type MarketIndexItem struct {
	Symbol    string  `json:"symbol"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Change    float64 `json:"change"`
	ChangePct float64 `json:"changePct"`
	TurnoverB float64 `json:"turnoverB"`
	Market    string  `json:"market"`
	Role      string  `json:"role"`
	Available bool    `json:"available"`
	Source    string  `json:"source,omitempty"`
	DataTime  string  `json:"dataTime,omitempty"`
	Error     string  `json:"error,omitempty"`
}

type MarketSentimentGauge struct {
	FearGreedScore   int     `json:"fearGreedScore"`
	FearGreedLabel   string  `json:"fearGreedLabel"`
	FearGreedLevel   string  `json:"fearGreedLevel"`
	VIX              float64 `json:"vix"`
	VixChangePct     float64 `json:"vixChangePct"`
	UpCount          int     `json:"upCount"`
	DownCount        int     `json:"downCount"`
	FlatCount        int     `json:"flatCount"`
	LimitUpCount     int     `json:"limitUpCount"`
	LimitDownCount   int     `json:"limitDownCount"`
	TotalTurnoverB   float64 `json:"totalTurnoverB"`
	NorthboundFlowB  float64 `json:"northboundFlowB"`
	TurnoverStatus   string  `json:"turnoverStatus"`
	SentimentSummary string  `json:"sentimentSummary"`
}

type ComprehensiveMarketOverview struct {
	UpdatedAt          string               `json:"updatedAt"`
	CNIndices          []MarketIndexItem    `json:"cnIndices"`
	USIndices          []MarketIndexItem    `json:"usIndices"`
	GlobalIndices      []MarketIndexItem    `json:"globalIndices"`
	Sentiment          MarketSentimentGauge `json:"sentiment"`
	USOvernightMapping []any                `json:"usOvernightMapping"`
	ForwardLayoutPlan  []any                `json:"forwardLayoutPlan"`
}

type EastmoneyIndexResponse struct {
	Data struct {
		Diff []struct {
			F2   float64 `json:"f2"`
			F3   float64 `json:"f3"`
			F4   float64 `json:"f4"`
			F6   float64 `json:"f6"`
			F12  string  `json:"f12"`
			F14  string  `json:"f14"`
			F124 int64   `json:"f124"`
		} `json:"diff"`
	} `json:"data"`
}

// FetchComprehensiveMarketOverview aggregates only provider-observed index values.
func FetchComprehensiveMarketOverview() ComprehensiveMarketOverview {
	cn := fetchEastmoneyIndices()
	us := fetchYahooIndices([]MarketIndexItem{
		{Symbol: "NDX", Name: "纳指 100 (NDX)", Market: "us", Role: "科技龙头指数"}, {Symbol: "SPX", Name: "标普 500 (SPX)", Market: "us", Role: "美国大盘基准"}, {Symbol: "IXIC", Name: "纳指综合 (IXIC)", Market: "us", Role: "纳斯达克综合指数"},
		{Symbol: "DJI", Name: "道琼斯 (DJI)", Market: "us", Role: "美国工业指数"}, {Symbol: "VIX", Name: "恐慌指数 (VIX)", Market: "us", Role: "期权隐含波动率"}, {Symbol: "TNX", Name: "10年期美债收益率 (TNX)", Market: "us", Role: "美国长期利率"},
	})
	global := fetchYahooIndices([]MarketIndexItem{
		{Symbol: "N225", Name: "日经 225 (Nikkei)", Market: "global", Role: "日本大盘指数"}, {Symbol: "JPY=X", Name: "美元/日元 (USD/JPY)", Market: "global", Role: "美元日元汇率"}, {Symbol: "KS11", Name: "韩国综合 (KOSPI)", Market: "global", Role: "韩国大盘指数"},
		{Symbol: "HSI", Name: "恒生指数 (HSI)", Market: "hk", Role: "香港大盘指数"}, {Symbol: "HSTECH", Name: "恒生科技 (HSTECH)", Market: "hk", Role: "香港科技指数"}, {Symbol: "DX-Y.NYB", Name: "美元指数 (DXY)", Market: "global", Role: "美元指数"},
	})
	overview, _ := finalizeMarketOverview(cn, us, global)
	return overview
}

func fetchEastmoneyIndices() []MarketIndexItem {
	fallback := []MarketIndexItem{{Symbol: "000001", Name: "上证指数", Market: "cn"}, {Symbol: "399001", Name: "深证成指", Market: "cn"}, {Symbol: "399006", Name: "创业板指", Market: "cn"}, {Symbol: "000688", Name: "科创50", Market: "cn"}, {Symbol: "899050", Name: "北证50", Market: "cn"}}
	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest(http.MethodGet, "https://push2.eastmoney.com/api/qt/ulist.np/get?fltt=2&fields=f2,f3,f4,f6,f12,f14,f124&secids=1.000001,0.399001,0.399006,1.000688,0.899050", nil)
	if err != nil {
		return fallback
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return fallback
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fallback
	}
	var payload EastmoneyIndexResponse
	if json.NewDecoder(resp.Body).Decode(&payload) != nil {
		return fallback
	}
	out := make([]MarketIndexItem, 0, len(payload.Data.Diff))
	for _, row := range payload.Data.Diff {
		if row.F2 <= 0 || row.F124 <= 0 {
			continue
		}
		out = append(out, MarketIndexItem{Symbol: row.F12, Name: row.F14, Price: row.F2, Change: row.F4, ChangePct: row.F3, TurnoverB: row.F6 / 1e8, Market: "cn", Available: true, Source: "eastmoney", DataTime: time.Unix(row.F124, 0).UTC().Format(time.RFC3339)})
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func finalizeMarketOverview(cn, us, global []MarketIndexItem) (ComprehensiveMarketOverview, bool) {
	o := ComprehensiveMarketOverview{CNIndices: cn, USIndices: us, GlobalIndices: global, USOvernightMapping: []any{}, ForwardLayoutPlan: []any{}, Sentiment: MarketSentimentGauge{SentimentSummary: "涨跌家数、资金流和推断性情绪未接入可靠来源，暂不展示。"}}
	oldest := time.Time{}
	available := 0
	for _, group := range [][]MarketIndexItem{cn, us, global} {
		for _, item := range group {
			if !item.Available {
				continue
			}
			available++
			observed, err := time.Parse(time.RFC3339, item.DataTime)
			if err != nil || strings.TrimSpace(item.Source) == "" {
				return o, false
			}
			if oldest.IsZero() || observed.Before(oldest) {
				oldest = observed
			}
			if item.Symbol == "VIX" {
				o.Sentiment.VIX = item.Price
				o.Sentiment.VixChangePct = item.ChangePct
			}
		}
	}
	if available == 0 || oldest.IsZero() {
		return o, false
	}
	o.UpdatedAt = oldest.UTC().Format(time.RFC3339)
	return o, true
}

func fetchYahooIndices(items []MarketIndexItem) []MarketIndexItem {
	symbols := map[string]string{"NDX": "^NDX", "SPX": "^GSPC", "IXIC": "^IXIC", "DJI": "^DJI", "VIX": "^VIX", "TNX": "^TNX", "N225": "^N225", "JPY=X": "JPY=X", "KS11": "^KS11", "HSI": "^HSI", "HSTECH": "^HSTECH", "DX-Y.NYB": "DX-Y.NYB"}
	client := NewYFinanceClientWithTimeout(2 * time.Second)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := range items {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			query, ok := symbols[items[i].Symbol]
			if !ok {
				mu.Lock()
				items[i].Error = "未配置可靠行情代码"
				mu.Unlock()
				return
			}
			quote, err := client.GetQuote(query)
			mu.Lock()
			defer mu.Unlock()
			if err != nil || quote == nil || quote.RegularPrice <= 0 || quote.ObservedAt.IsZero() {
				items[i].Error = fmt.Sprintf("实时行情不可用: %v", err)
				return
			}
			change := quote.RegularPrice - quote.PreviousClose
			pct := 0.0
			if quote.PreviousClose > 0 {
				pct = change / quote.PreviousClose * 100
			}
			items[i].Price = quote.RegularPrice
			items[i].Change = change
			items[i].ChangePct = pct
			items[i].Available = true
			items[i].Source = "yahoo-chart"
			items[i].DataTime = quote.ObservedAt.UTC().Format(time.RFC3339)
		}()
	}
	wg.Wait()
	return items
}
