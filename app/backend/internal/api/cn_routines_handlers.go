package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"trading-agents/internal/market"
)

type cnRoutineSymbolSeed struct {
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name"`
	Role       string  `json:"role"`
	WeightHint float64 `json:"weight_hint"`
}

type cnRoutineSignalSeed struct {
	BuyIfDayPctLTE           *float64 `json:"buy_if_day_pct_lte"`
	PauseBuyIfDayPctGTE      *float64 `json:"pause_buy_if_day_pct_gte"`
	SatelliteOnlyIfDayPctLTE *float64 `json:"satellite_only_if_day_pct_lte"`
	MaxHeat                  *float64 `json:"max_heat"`
	PreferCool               bool     `json:"prefer_cool"`
}

type cnDipAlertLevel struct {
	DayPct float64 `json:"day_pct"`
	Label  string  `json:"label"`
	Action string  `json:"action"`
}

type cnDipAlerts struct {
	Yellow *cnDipAlertLevel `json:"yellow,omitempty"`
	Orange *cnDipAlertLevel `json:"orange,omitempty"`
	Red    *cnDipAlertLevel `json:"red,omitempty"`
}

type cnRoutineSeed struct {
	ID            string                `json:"id"`
	Name          string                `json:"name"`
	Style         string                `json:"style"`
	Horizon       string                `json:"horizon"`
	Risk          string                `json:"risk"`
	Why           string                `json:"why"`
	Symbols       []cnRoutineSymbolSeed `json:"symbols"`
	BuyPcts       []float64             `json:"buy_pcts"`
	SellPcts      []float64             `json:"sell_pcts"`
	TrimFractions []float64             `json:"trim_fractions"`
	StopPct       *float64              `json:"stop_pct"`
	Signal        cnRoutineSignalSeed   `json:"signal"`
	Playbook      []string              `json:"playbook"`
	DipAlerts     *cnDipAlerts          `json:"dip_alerts,omitempty"`
}

type cnRoutinesFile struct {
	Updated    string `json:"updated"`
	Disclaimer string `json:"disclaimer"`
	Profile    struct {
		ID                  string   `json:"id"`
		Name                string   `json:"name"`
		Audience            string   `json:"audience"`
		CashFloorPct        float64  `json:"cash_floor_pct"`
		CoreEtfTargetPct    float64  `json:"core_etf_target_pct"`
		SatelliteMaxPct     float64  `json:"satellite_max_pct"`
		SingleNameMaxPct    float64  `json:"single_name_max_pct"`
		ThemeStockMaxPct    float64  `json:"theme_stock_max_pct"`
		MaxDeployPerWeekPct float64  `json:"max_deploy_per_week_pct"`
		Rules               []string `json:"rules"`
	} `json:"profile"`
	Routines []cnRoutineSeed `json:"routines"`
}

var (
	cnRoutinesOnce sync.Once
	cnRoutinesData cnRoutinesFile
)

func loadCNRoutines() cnRoutinesFile {
	cnRoutinesOnce.Do(func() {
		paths := []string{
			filepath.Join("data", "cn-routines.json"),
			filepath.Join("app", "backend", "data", "cn-routines.json"),
		}
		for _, p := range paths {
			raw, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			var f cnRoutinesFile
			if json.Unmarshal(raw, &f) == nil && len(f.Routines) > 0 {
				cnRoutinesData = f
				return
			}
		}
		cnRoutinesData = cnRoutinesFile{
			Disclaimer: "非投资建议",
		}
	})
	return cnRoutinesData
}

func roundPrice(p float64) float64 {
	if p >= 100 {
		return math.Round(p*100) / 100
	}
	if p >= 10 {
		return math.Round(p*1000) / 1000
	}
	return math.Round(p*10000) / 10000
}

type routineLevel struct {
	Pct   float64 `json:"pct"`
	Price float64 `json:"price"`
	Note  string  `json:"note"`
}

type dipSignal struct {
	Level  string `json:"level"`
	Label  string `json:"label"`
	Action string `json:"action"`
}

type dipPriceLine struct {
	Level     string  `json:"level"`
	Label     string  `json:"label"`
	Action    string  `json:"action"`
	DayPct    float64 `json:"day_pct"`
	Price     float64 `json:"price"`
	Gap       float64 `json:"gap"`
	Triggered bool    `json:"triggered"`
}

type routineSymbolEval struct {
	Symbol     string         `json:"symbol"`
	Name       string         `json:"name"`
	Role       string         `json:"role"`
	WeightHint float64        `json:"weight_hint"`
	Price      float64        `json:"price,omitempty"`
	Pct        float64        `json:"pct,omitempty"`
	Heat       float64        `json:"heat,omitempty"`
	HasQuote   bool           `json:"hasQuote"`
	Action     string         `json:"action"`
	Reason     string         `json:"reason"`
	BuyLevels  []routineLevel `json:"buy_levels"`
	SellLevels []routineLevel `json:"sell_levels"`
	StopPrice  *float64       `json:"stop_price,omitempty"`
	DipSignal  *dipSignal     `json:"dip_signal,omitempty"`
	DipLines   []dipPriceLine `json:"dip_lines,omitempty"`
}

type routineEval struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Style        string              `json:"style"`
	Horizon      string              `json:"horizon"`
	Risk         string              `json:"risk"`
	Why          string              `json:"why"`
	Playbook     []string            `json:"playbook"`
	TodayBias    string              `json:"today_bias"`
	TodaySummary string              `json:"today_summary"`
	DipLevel     string              `json:"dip_level,omitempty"`
	DipSummary   string              `json:"dip_summary,omitempty"`
	Symbols      []routineSymbolEval `json:"symbols"`
}

func evalDipSignal(dip *cnDipAlerts, pct float64) *dipSignal {
	if dip == nil {
		return nil
	}
	if dip.Red != nil && pct <= dip.Red.DayPct {
		return &dipSignal{Level: "red", Label: dip.Red.Label, Action: dip.Red.Action}
	}
	if dip.Orange != nil && pct <= dip.Orange.DayPct {
		return &dipSignal{Level: "orange", Label: dip.Orange.Label, Action: dip.Orange.Action}
	}
	if dip.Yellow != nil && pct <= dip.Yellow.DayPct {
		return &dipSignal{Level: "yellow", Label: dip.Yellow.Label, Action: dip.Yellow.Action}
	}
	return nil
}

func buildDipLines(dip *cnDipAlerts, price, pct float64) []dipPriceLine {
	if dip == nil || price <= 0 {
		return nil
	}
	refPrice := price / (1.0 + pct/100.0)
	type entry struct {
		level string
		a     *cnDipAlertLevel
	}
	items := []entry{
		{"yellow", dip.Yellow},
		{"orange", dip.Orange},
		{"red", dip.Red},
	}
	var lines []dipPriceLine
	for _, e := range items {
		if e.a == nil {
			continue
		}
		triggerPrice := roundPrice(refPrice * (1.0 + e.a.DayPct/100.0))
		gap := 0.0
		if price > 0 {
			gap = math.Round((price-triggerPrice)/price*10000) / 100
		}
		lines = append(lines, dipPriceLine{
			Level:     e.level,
			Label:     e.a.Label,
			Action:    e.a.Action,
			DayPct:    e.a.DayPct,
			Price:     triggerPrice,
			Gap:       gap,
			Triggered: pct <= e.a.DayPct,
		})
	}
	return lines
}

var dipLevelRank = map[string]int{"yellow": 1, "orange": 2, "red": 3}

func worstDipLevel(syms []routineSymbolEval) (string, string) {
	worst := ""
	var msgs []string
	for _, s := range syms {
		if s.DipSignal == nil {
			continue
		}
		if dipLevelRank[s.DipSignal.Level] > dipLevelRank[worst] {
			worst = s.DipSignal.Level
		}
		msgs = append(msgs, fmt.Sprintf("%s(%s): %s", s.Name, s.DipSignal.Label, s.DipSignal.Action))
	}
	if worst == "" {
		return "", ""
	}
	return worst, strings.Join(msgs, "；")
}

func evalRoutineSymbol(r cnRoutineSeed, s cnRoutineSymbolSeed, q cnQuoteLite, heat float64, hasHeat bool) routineSymbolEval {
	row := routineSymbolEval{
		Symbol: s.Symbol, Name: s.Name, Role: s.Role, WeightHint: s.WeightHint,
		BuyLevels: []routineLevel{}, SellLevels: []routineLevel{},
		Action: "wait", Reason: "暂无有效报价，先观察",
	}
	if q.Price <= 0 {
		return row
	}
	row.Price = q.Price
	row.Pct = q.Pct
	row.HasQuote = true
	if hasHeat {
		row.Heat = heat
	}

	for i, pct := range r.BuyPcts {
		note := "分批买"
		if i == 0 {
			note = "轻仓首买/加仓"
		} else if i == len(r.BuyPcts)-1 {
			note = "深跌加仓（仍受周额度约束）"
		}
		row.BuyLevels = append(row.BuyLevels, routineLevel{
			Pct: pct, Price: roundPrice(q.Price * (1.0 + pct/100.0)), Note: note,
		})
	}
	for i, pct := range r.SellPcts {
		note := "减仓"
		if i < len(r.TrimFractions) {
			note = "减仓约 " + trimPctLabel(r.TrimFractions[i])
		}
		row.SellLevels = append(row.SellLevels, routineLevel{
			Pct: pct, Price: roundPrice(q.Price * (1.0 + pct/100.0)), Note: note,
		})
	}
	if r.StopPct != nil {
		sp := roundPrice(q.Price * (1.0 + *r.StopPct/100.0))
		row.StopPrice = &sp
	}

	row.DipSignal = evalDipSignal(r.DipAlerts, q.Pct)
	row.DipLines = buildDipLines(r.DipAlerts, q.Price, q.Pct)

	sig := r.Signal
	role := strings.ToLower(s.Role)

	// Hard cool-off for hot names
	if sig.MaxHeat != nil && hasHeat && heat > *sig.MaxHeat {
		row.Action = "wait"
		row.Reason = "过热度偏高，按套路暂不新开/不加仓"
		return row
	}

	// Satellite / theme: only buy on deeper dips if configured
	if (role == "satellite" || role == "theme") && sig.SatelliteOnlyIfDayPctLTE != nil {
		if q.Pct > *sig.SatelliteOnlyIfDayPctLTE {
			if sig.PauseBuyIfDayPctGTE != nil && q.Pct >= *sig.PauseBuyIfDayPctGTE {
				row.Action = "trim"
				row.Reason = "卫星/主题偏强，今日只考虑按卖点减仓，不加仓"
				return row
			}
			row.Action = "hold"
			row.Reason = "卫星/主题未到回调买点，持有观望"
			return row
		}
	}

	if sig.PauseBuyIfDayPctGTE != nil && q.Pct >= *sig.PauseBuyIfDayPctGTE {
		row.Action = "trim"
		row.Reason = "当日偏强，暂停买入；可对照卖出网格考虑减仓"
		return row
	}

	if sig.BuyIfDayPctLTE != nil && q.Pct <= *sig.BuyIfDayPctLTE {
		row.Action = "buy"
		row.Reason = "当日回调进入买入窗口；按网格分批，遵守周投入上限"
		return row
	}

	// Near first buy grid?
	if len(row.BuyLevels) > 0 && q.Price <= row.BuyLevels[0].Price*1.002 {
		row.Action = "buy"
		row.Reason = "接近第一买点，可按定额轻仓执行"
		return row
	}

	row.Action = "hold"
	row.Reason = "未触发极端信号：继续定投节奏或持有，不追高"
	return row
}

func trimPctLabel(f float64) string {
	return fmt.Sprintf("%.0f%%", f*100)
}

func biasFromSymbols(syms []routineSymbolEval) (string, string) {
	buy, trim, hold := 0, 0, 0
	for _, s := range syms {
		switch s.Action {
		case "buy":
			buy++
		case "trim":
			trim++
		case "hold":
			hold++
		}
	}
	switch {
	case buy > 0 && trim == 0:
		return "buy", "今日偏「分批买入」窗口（仍受现金缓冲与周额度约束）"
	case trim > 0 && buy == 0:
		return "trim", "今日偏「减仓/兑现」窗口，不宜追高加仓"
	case buy > 0 && trim > 0:
		return "mixed", "标的分化：回调的分批买，大涨的对照卖点减"
	case hold > 0:
		return "hold", "无强信号：维持既定定投/持有，不因无聊而交易"
	default:
		return "wait", "数据不足或过热约束，先观望"
	}
}

// GetCNRoutines GET /api/cn/routines — steady playbooks with mechanical buy/sell levels.
func GetCNRoutines(c *gin.Context) {
	file := loadCNRoutines()
	priorityFresh := ensureCNPriorityFresh()
	quotes := loadCNLiveMap()
	need := make([]string, 0)
	for _, r := range file.Routines {
		for _, s := range r.Symbols {
			need = append(need, s.Symbol)
		}
	}
	fillMissingCNQuotes(quotes, need)

	pulse := loadCNPulseLite()
	heatBy := map[string]float64{}
	for _, p := range pulse {
		heatBy[p.Ticker] = p.Heat
	}

	evals := make([]routineEval, 0, len(file.Routines))
	actionCount := map[string]int{}
	for _, r := range file.Routines {
		syms := make([]routineSymbolEval, 0, len(r.Symbols))
		for _, s := range r.Symbols {
			q := quotes[s.Symbol]
			h, ok := heatBy[s.Symbol]
			ev := evalRoutineSymbol(r, s, q, h, ok)
			syms = append(syms, ev)
			actionCount[ev.Action]++
		}
		bias, summary := biasFromSymbols(syms)
		dipLv, dipMsg := worstDipLevel(syms)
		evals = append(evals, routineEval{
			ID: r.ID, Name: r.Name, Style: r.Style, Horizon: r.Horizon, Risk: r.Risk,
			Why: r.Why, Playbook: r.Playbook, TodayBias: bias, TodaySummary: summary,
			DipLevel: dipLv, DipSummary: dipMsg, Symbols: syms,
		})
	}

	// Rank: prioritize routines with actionable buy today, then hold, then high risk last for display
	sort.SliceStable(evals, func(i, j int) bool {
		rank := func(b string) int {
			switch b {
			case "buy":
				return 0
			case "mixed":
				return 1
			case "hold":
				return 2
			case "trim":
				return 3
			default:
				return 4
			}
		}
		ri, rj := rank(evals[i].TodayBias), rank(evals[j].TodayBias)
		if ri != rj {
			return ri < rj
		}
		return evals[i].ID < evals[j].ID
	})

	quoted, required := cnQuoteCoverage(quotes, need)
	dataTime, stale, staleReason := cnDynamicFreshness(market.Default().CNLiveUpdatedAt(), quoted, required, time.Now())
	if !priorityFresh {
		stale = true
		staleReason = "priority CN quote refresh did not complete"
	}
	setDataFreshness(c, dataFreshnessMeta{
		Source:      fmt.Sprintf("cn-routines-config@%s+live-quotes", file.Updated),
		DataTime:    dataTime,
		Stale:       stale,
		StaleReason: staleReason,
		Refreshable: true,
	})
	c.JSON(http.StatusOK, gin.H{
		"disclaimer":    file.Disclaimer,
		"updated":       file.Updated,
		"configUpdated": file.Updated,
		"profile":       file.Profile,
		"routines":      evals,
		"today": gin.H{
			"actions": actionCount,
			"note":    "买卖点以「现价」为锚的机械网格；换日会随报价漂移。用于提醒，不保证收益。",
		},
	})
}
