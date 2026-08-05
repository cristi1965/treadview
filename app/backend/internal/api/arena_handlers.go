package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"trading-agents/internal/market"

	"github.com/gin-gonic/gin"
)

type ArenaTrade struct {
	Date   string `json:"date"`
	Action string `json:"action"`
	Symbol string `json:"symbol"`
	Detail string `json:"detail"`
}

type ArenaHolding struct {
	Symbol string  `json:"symbol"`
	Name   string  `json:"name"`
	Shares int     `json:"shares"`
	Cost   float64 `json:"cost"`
	Price  float64 `json:"price"`
	Day    float64 `json:"day"`
	Pnl    float64 `json:"pnl"`
	Thesis string  `json:"thesis"`
}

type ArenaPlayer struct {
	ID        string         `json:"id"`
	Rank      int            `json:"rank"`
	Short     string         `json:"short"`
	Name      string         `json:"name"`
	Style     string         `json:"style"`
	Value     float64        `json:"value"`
	ReturnPct float64        `json:"returnPct"`
	Cash      float64        `json:"cash"`
	Holdings  []ArenaHolding `json:"holdings"`
	Trades    []ArenaTrade   `json:"trades"`
}

type ArenaResponse struct {
	Market    string        `json:"market"`
	SettleDate string       `json:"settleDate"`
	Players   []ArenaPlayer `json:"players"`
}

func (h *Handler) GetArena(c *gin.Context) {
	market := c.DefaultQuery("market", "us")

	settleDate := "2026-07-29"
	players := getArenaPlayers(market)
	if market == "us" {
		if loaded, date, ok := loadArenaFromJSON("us"); ok {
			players = loaded
			if date != "" {
				settleDate = date
			}
		}
	}

	// Dynamically update quotes for holdings
	updateArenaQuotes(players)

	// Sort players by total Value and re-rank
	sort.Slice(players, func(i, j int) bool {
		return players[i].Value > players[j].Value
	})
	for i := range players {
		players[i].Rank = i + 1
	}

	c.JSON(http.StatusOK, ArenaResponse{
		Market:     market,
		SettleDate: settleDate,
		Players:    players,
	})
}

func loadArenaFromJSON(market string) ([]ArenaPlayer, string, bool) {
	name := "arena-us.json"
	if market == "cn" {
		name = "arena-cn.json"
	}
	data, err := os.ReadFile(filepath.Join("data", name))
	if err != nil {
		data, err = os.ReadFile(filepath.Join("app", "backend", "data", name))
	}
	if err != nil || len(data) == 0 {
		return nil, "", false
	}
	var payload ArenaResponse
	if err := json.Unmarshal(data, &payload); err != nil || len(payload.Players) == 0 {
		return nil, "", false
	}
	return payload.Players, payload.SettleDate, true
}

func updateArenaQuotes(players []ArenaPlayer) {
	// 1. Gather all unique symbols
	var syms []string
	for i := range players {
		for j := range players[i].Holdings {
			syms = append(syms, strings.TrimSpace(players[i].Holdings[j].Symbol))
		}
	}

	// 2. Fetch quotes dynamically from default provider
	quotes := market.Default().Quotes(syms)

	// 3. Fallback map for special HK/TW/KS stocks
	fallback := map[string]struct{ Price, Pct float64 }{
		"3037.TW":    {Price: 863, Pct: 2.74},
		"2454.TW":    {Price: 3995, Pct: -0.87},
		"2382.TW":    {Price: 377, Pct: 1.07},
		"2308.TW":    {Price: 1885, Pct: -0.26},
		"9888.HK":    {Price: 113.8, Pct: -3.23},
		"9988.HK":    {Price: 113.2, Pct: 5.3},
		"2018.HK":    {Price: 39.02, Pct: 0},
		"1347.HK":    {Price: 190.6, Pct: 2.42},
		"1810.HK":    {Price: 25.5, Pct: 0.79},
		"9698.HK":    {Price: 32.5, Pct: 2.52},
		"02899.HK":   {Price: 29.2, Pct: -1.82},
		"0981.HK":    {Price: 76.95, Pct: 1.52},
		"0700.HK":    {Price: 490, Pct: 2.34},
		"005930.KS":  {Price: 284000, Pct: 2.34},
		"3661.TW":    {Price: 4065, Pct: 0.74},
		"2317.TW":    {Price: 237.5, Pct: 0.21},
		"2301.TW":    {Price: 214, Pct: 0.47},
		"3711.TW":    {Price: 625, Pct: -3.99},
		"000660.KS":  {Price: 2206000, Pct: 6.26},
		"TSM.TW":     {Price: 1045, Pct: 1.46},
		"005930-S.KS":{Price: 215000, Pct: 1.18},
		"KVYO":       {Price: 16.60, Pct: -3.60},
		"PDD":        {Price: 84.74, Pct: 2.68},
		"ELV":        {Price: 416.08, Pct: -0.66},
	}

	for i := range players {
		totalValue := players[i].Cash
		
		for j := range players[i].Holdings {
			h := &players[i].Holdings[j]
			sym := strings.TrimSpace(h.Symbol)
			
			// Try to find the price and day change percentage
			var price, pct float64
			found := false
			
			if quotes != nil {
				if q, ok := quotes[sym]; ok {
					price = q.Price
					pct = q.Pct
					found = true
				}
			}
			
			if !found {
				if q, ok := fallback[sym]; ok {
					price = q.Price
					pct = q.Pct
					found = true
				}
			}
			
			// If found, update price and day
			if found {
				h.Price = price
				h.Day = pct
			}
			
			// Calculate Pnl relative to Cost
			if h.Cost > 0 {
				h.Pnl = (h.Price - h.Cost) / h.Cost * 100.0
			}
			
			// Add to totalValue
			totalValue += float64(h.Shares) * h.Price
		}
		
		players[i].Value = totalValue
		players[i].ReturnPct = (totalValue - 1000000.0) / 1000000.0 * 100.0
	}
}

func getArenaPlayers(market string) []ArenaPlayer {
	if market == "cn" {
		return []ArenaPlayer{
			{
				ID: "sentiment", Rank: 1, Short: "情", Name: "情绪资金面", Style: "盘口轮动 · 快进快出", Value: 1077350, ReturnPct: 7.74, Cash: 592880,
				Holdings: []ArenaHolding{
					{Symbol: "300059", Name: "东方财富", Shares: 25000, Cost: 20.13, Price: 20.13, Day: -0.79, Pnl: 0, Thesis: "创业板情绪风向标,券商交易额放量时的第一弹性。"},
					{Symbol: "300033", Name: "同花顺", Shares: 2200, Cost: 225.77, Price: 225.77, Day: -4.42, Pnl: 0, Thesis: "软件与金融终端双重龙头,盘面极其亢奋时适合顺势而为。"},
				},
				Trades: []ArenaTrade{
					{Date: "2026-07-02", Action: "买入", Symbol: "300059", Detail: "25,000 股 @ $20.13 — 券商情绪龙头。"},
				},
			},
		}
	}

	// Default US market players matching the exact settle positions of live site
	return []ArenaPlayer{
		{
			ID: "sentiment", Rank: 1, Short: "情", Name: "情绪资金面", Style: "盘口轮动 · 快进快出", Value: 1057011, ReturnPct: 5.70, Cash: 413042,
			Holdings: []ArenaHolding{
				{Symbol: "PDD", Name: "PDD Holdings Inc.", Shares: 1961, Cost: 82.39, Price: 82.39, Day: -0.16, Pnl: 0, Thesis: "中概情绪在地板上、人人喊政治风险,可南向和聪明钱在偷偷捡,这种又冷又有钱进的盘我喜欢反向埋伏。"},
				{Symbol: "ELV", Name: "Elevance Health Inc.", Shares: 386, Cost: 417.89, Price: 417.89, Day: 0.41, Pnl: 0, Thesis: "盘口一片悲观、卖方连环下调、股价从天花板腰斩到情绪冰点——这种被嫌弃的大盘蓝筹,正是逆向资金悄悄捡的时候。"},
				{Symbol: "KVYO", Name: "Klaviyo Inc. Series A", Shares: 9562, Cost: 16.90, Price: 16.90, Day: 3.30, Pnl: 0, Thesis: "盘口情绪在冰点——破发、远低于 IPO、散户没人提,但 Shopify 拿着大头筹码且机构在底部慢慢吸,这是被嫌弃的冷门。"},
				{Symbol: "CBOE", Name: "Cboe Global Markets Inc.", Shares: 598, Cost: 264.98, Price: 264.98, Day: 2.45, Pnl: 0, Thesis: "盘口上它是机构和被动资金的稳定持仓,没有散户狂热、也没人恐慌抛,资金面温和偏正。"},
			},
			Trades: []ArenaTrade{
				{Date: "2026-07-08", Action: "买入", Symbol: "CBOE", Detail: "598 股 @ $264.98 — 指数盘前走弱时波动率对冲承接稳。"},
				{Date: "2026-07-08", Action: "卖出", Symbol: "AUGO", Detail: "2,385 股 @ $59.82 — -11.7% 触止损认赔。"},
				{Date: "2026-07-07", Action: "卖出", Symbol: "CACI", Detail: "319 股 @ $497.36 — 资金面撤退,防线告破离场。"},
				{Date: "2026-07-02", Action: "买入", Symbol: "PDD", Detail: "1,961 股 @ $82.39 — 极致嫌弃区逆向埋伏。"},
			},
		},
		{
			ID: "duan", Rank: 2, Short: "段", Name: "段永平", Style: "本分 · 极度集中", Value: 1009391, ReturnPct: 0.94, Cash: 40172,
			Holdings: []ArenaHolding{
				{Symbol: "AAPL", Name: "Apple Inc.", Shares: 823, Cost: 291.58, Price: 291.58, Day: 0.88, Pnl: 0, Thesis: "苹果的商业模式和文化我看懂了,生态黏性 + 用户体验本分到极致,这种生意我不在乎贵一点。"},
				{Symbol: "TSM", Name: "Taiwan Semiconductor Manufacturing", Shares: 587, Cost: 408.75, Price: 408.75, Day: 1.02, Pnl: 0, Thesis: "商业模式好到极致——别人替它做研发养客户,它只管把制程做到全世界第一。"},
				{Symbol: "COST", Name: "Costco Wholesale Corporation", Shares: 244, Cost: 983.37, Price: 983.37, Day: 0.59, Pnl: 0, Thesis: "最本分的生意,省下的钱全还给消费者、自己只赚会员费。"},
				{Symbol: "VRSN", Name: "VeriSign Inc.", Shares: 833, Cost: 288.09, Price: 288.09, Day: 2.95, Pnl: 0, Thesis: "什么都不用做就能躺赚的行业,完美的商业模式。"},
			},
			Trades: []ArenaTrade{
				{Date: "2026-06-10", Action: "买入", Symbol: "AAPL", Detail: "823 股 @ $291.58 — 本分生意的典范。"},
			},
		},
		{
			ID: "buffett", Rank: 3, Short: "巴", Name: "巴菲特", Style: "价值 · 护城河 · 长持", Value: 1007411, ReturnPct: 0.74, Cash: 158389,
			Holdings: []ArenaHolding{
				{Symbol: "ACGL", Name: "Arch Capital Group Ltd.", Shares: 1314, Cost: 91.31, Price: 91.31, Day: -0.82, Pnl: 0, Thesis: "纪律极其严明的承保机器,穿越周期的优秀代表。"},
				{Symbol: "TSM", Name: "Taiwan Semiconductor Manufacturing", Shares: 293, Cost: 408.75, Price: 408.75, Day: 1.02, Pnl: 0, Thesis: "独步天下的晶圆制造龙头。"},
				{Symbol: "META", Name: "Meta Platforms Inc.", Shares: 210, Cost: 577.22, Price: 577.22, Day: -2.02, Pnl: 0, Thesis: "极强流量广告现金牛为 AI 研发输血。"},
			},
			Trades: []ArenaTrade{
				{Date: "2026-07-02", Action: "买入", Symbol: "ACGL", Detail: "1,314 股 @ $91.31 — 高质量的承保复利载体。"},
			},
		},
		{
			ID: "druckenmiller", Rank: 4, Short: "德", Name: "德鲁肯米勒", Style: "宏观趋势 · 动量", Value: 994428, ReturnPct: -0.56, Cash: 145343,
			Holdings: []ArenaHolding{
				{Symbol: "LLY", Name: "Eli Lilly and Company", Shares: 110, Cost: 1107.45, Price: 1107.45, Day: -1.60, Pnl: 0, Thesis: "肥胖症大赛道爆发是宏观动量大趋势。"},
				{Symbol: "GEV", Name: "GE Vernova Inc.", Shares: 119, Cost: 1109.73, Price: 1109.73, Day: -1.87, Pnl: 0, Thesis: "AI 电力基建扩容引发电网革命。"},
				{Symbol: "LRCX", Name: "Lam Research Corporation", Shares: 357, Cost: 321.80, Price: 321.80, Day: -10.19, Pnl: 0, Thesis: "先进封装及先进制程设备趋势未止。"},
			},
			Trades: []ArenaTrade{
				{Date: "2026-07-02", Action: "买入", Symbol: "LLY", Detail: "110 股 @ $1107.45 — 踩中大趋势继续配置。"},
			},
		},
		{
			ID: "serenity", Rank: 5, Short: "S", Name: "Serenity", Style: "瓶颈狙击 · 带止损", Value: 943414, ReturnPct: -5.66, Cash: 88920,
			Holdings: []ArenaHolding{
				{Symbol: "NVDA", Name: "NVIDIA Corporation", Shares: 980, Cost: 201.12, Price: 201.12, Day: -0.39, Pnl: 0, Thesis: "芯片端依然供不应求,若短线破位需要止损。"},
				{Symbol: "VST", Name: "Vistra Corp.", Shares: 804, Cost: 158.63, Price: 158.63, Day: -0.58, Pnl: 0, Thesis: "AI 算力的尽头是电力,把握瓶颈要素资产。"},
			},
			Trades: []ArenaTrade{
				{Date: "2026-07-02", Action: "卖出", Symbol: "SMCI", Detail: "跌破阻力带触发止损。"},
			},
		},
	}
}
