package api

import (
	"testing"
	"time"
)

func TestBiasFromSymbols(t *testing.T) {
	buy, sum := biasFromSymbols([]routineSymbolEval{{Action: "buy"}, {Action: "hold"}})
	if buy != "buy" || sum == "" {
		t.Fatalf("want buy bias, got %s %s", buy, sum)
	}
	trim, _ := biasFromSymbols([]routineSymbolEval{{Action: "trim"}, {Action: "trim"}})
	if trim != "trim" {
		t.Fatalf("want trim, got %s", trim)
	}
	mixed, _ := biasFromSymbols([]routineSymbolEval{{Action: "buy"}, {Action: "trim"}})
	if mixed != "mixed" {
		t.Fatalf("want mixed, got %s", mixed)
	}
}

func TestEvalRoutineSymbolBuyWindow(t *testing.T) {
	lte := -1.2
	pause := 2.0
	r := cnRoutineSeed{
		BuyPcts:  []float64{-1, -2.5},
		SellPcts: []float64{5, 10},
		Signal:   cnRoutineSignalSeed{BuyIfDayPctLTE: &lte, PauseBuyIfDayPctGTE: &pause},
	}
	s := cnRoutineSymbolSeed{Symbol: "510300", Name: "沪深300", Role: "core", WeightHint: 35}
	ev := evalRoutineSymbol(r, s, cnQuoteLite{Price: 4.75, Pct: -1.5}, 0, false)
	if !ev.HasQuote || ev.Action != "buy" {
		t.Fatalf("expected buy, got %+v", ev)
	}
	if len(ev.BuyLevels) != 2 || ev.BuyLevels[0].Price <= 0 {
		t.Fatalf("buy levels missing: %+v", ev.BuyLevels)
	}
}

func TestEvalDipSignal(t *testing.T) {
	da := &cnDipAlerts{
		Yellow: &cnDipAlertLevel{DayPct: -1.5, Label: "回调观察", Action: "留意"},
		Orange: &cnDipAlertLevel{DayPct: -2.5, Label: "跳水警告", Action: "检查"},
		Red:    &cnDipAlertLevel{DayPct: -4.0, Label: "恐慌跳水", Action: "加仓"},
	}
	if ds := evalDipSignal(da, -0.5); ds != nil {
		t.Fatalf("expected nil for -0.5%%, got %+v", ds)
	}
	if ds := evalDipSignal(da, -1.5); ds == nil || ds.Level != "yellow" {
		t.Fatalf("expected yellow for -1.5%%, got %+v", ds)
	}
	if ds := evalDipSignal(da, -3.0); ds == nil || ds.Level != "orange" {
		t.Fatalf("expected orange for -3%%, got %+v", ds)
	}
	if ds := evalDipSignal(da, -5.0); ds == nil || ds.Level != "red" {
		t.Fatalf("expected red for -5%%, got %+v", ds)
	}
	if ds := evalDipSignal(nil, -5.0); ds != nil {
		t.Fatalf("expected nil for nil alerts, got %+v", ds)
	}
}

func TestWorstDipLevel(t *testing.T) {
	syms := []routineSymbolEval{
		{Name: "A", DipSignal: &dipSignal{Level: "yellow", Label: "x", Action: "y"}},
		{Name: "B", DipSignal: &dipSignal{Level: "red", Label: "x", Action: "y"}},
		{Name: "C"},
	}
	lv, msg := worstDipLevel(syms)
	if lv != "red" {
		t.Fatalf("expected red, got %s", lv)
	}
	if msg == "" {
		t.Fatal("expected non-empty summary")
	}
	lv2, _ := worstDipLevel([]routineSymbolEval{{Name: "X"}})
	if lv2 != "" {
		t.Fatalf("expected empty, got %s", lv2)
	}
}

func TestBuildDipLines(t *testing.T) {
	da := &cnDipAlerts{
		Yellow: &cnDipAlertLevel{DayPct: -1.5, Label: "观察", Action: "留意"},
		Orange: &cnDipAlertLevel{DayPct: -3.0, Label: "警告", Action: "检查"},
		Red:    &cnDipAlertLevel{DayPct: -5.0, Label: "恐慌", Action: "加仓"},
	}
	lines := buildDipLines(da, 4.95, -1.0)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0].Level != "yellow" || lines[0].Triggered {
		t.Fatalf("yellow should not be triggered at -1%%: %+v", lines[0])
	}
	if lines[0].Price <= 0 || lines[0].Gap <= 0 {
		t.Fatalf("yellow line price/gap invalid: %+v", lines[0])
	}
	lines2 := buildDipLines(da, 4.75, -5.0)
	if !lines2[2].Triggered {
		t.Fatalf("red should be triggered at -5%%: %+v", lines2[2])
	}
	if lines2[0].Triggered != true {
		t.Fatalf("yellow should also be triggered at -5%%: %+v", lines2[0])
	}
	if buildDipLines(nil, 5.0, 0) != nil {
		t.Fatal("nil alerts should return nil")
	}
}

func TestEvalRoutineSymbolDipSignal(t *testing.T) {
	lte := -1.2
	r := cnRoutineSeed{
		BuyPcts:  []float64{-1},
		SellPcts: []float64{5},
		Signal:   cnRoutineSignalSeed{BuyIfDayPctLTE: &lte},
		DipAlerts: &cnDipAlerts{
			Yellow: &cnDipAlertLevel{DayPct: -1.5, Label: "回调", Action: "留意"},
			Red:    &cnDipAlertLevel{DayPct: -4.0, Label: "恐慌", Action: "止损"},
		},
	}
	s := cnRoutineSymbolSeed{Symbol: "510300", Name: "沪深300", Role: "core"}
	ev := evalRoutineSymbol(r, s, cnQuoteLite{Price: 4.75, Pct: -2.0}, 0, false)
	if ev.DipSignal == nil || ev.DipSignal.Level != "yellow" {
		t.Fatalf("expected yellow dip signal for -2%%, got %+v", ev.DipSignal)
	}
	ev2 := evalRoutineSymbol(r, s, cnQuoteLite{Price: 4.75, Pct: -0.5}, 0, false)
	if ev2.DipSignal != nil {
		t.Fatalf("expected no dip signal for -0.5%%, got %+v", ev2.DipSignal)
	}
}

func TestFillMissingCNQuotesCache(t *testing.T) {
	cnQuoteCacheMu.Lock()
	cnQuoteCache["999999"] = cnQuoteCacheEntry{
		q:       cnQuoteLite{Price: 1.23, Pct: -0.5},
		expires: time.Now().Add(time.Minute),
	}
	cnQuoteCacheMu.Unlock()
	out := map[string]cnQuoteLite{}
	fillMissingCNQuotes(out, []string{"999999"})
	if out["999999"].Price != 1.23 {
		t.Fatalf("cache miss: %+v", out["999999"])
	}
}

func TestCNDynamicFreshnessUsesQuoteTimeNotRuleVersion(t *testing.T) {
	now := time.Date(2026, 9, 8, 2, 0, 0, 0, time.UTC)
	liveAt := now.Add(-2 * time.Minute)

	dataTime, stale, reason := cnDynamicFreshness(liveAt, 4, 4, now)
	if stale || reason != "" {
		t.Fatalf("fresh complete quotes should be live: stale=%v reason=%q", stale, reason)
	}
	if dataTime != liveAt.Format(time.RFC3339) {
		t.Fatalf("data time must come from quotes: got %q want %q", dataTime, liveAt.Format(time.RFC3339))
	}
}

func TestCNDynamicFreshnessFailsClosedWithoutCompleteLiveQuotes(t *testing.T) {
	now := time.Date(2026, 9, 8, 2, 0, 0, 0, time.UTC)

	if dataTime, stale, reason := cnDynamicFreshness(time.Time{}, 4, 4, now); !stale || dataTime != "unknown" || reason == "" {
		t.Fatalf("missing live timestamp must fail closed: time=%q stale=%v reason=%q", dataTime, stale, reason)
	}
	if _, stale, reason := cnDynamicFreshness(now.Add(-time.Minute), 3, 4, now); !stale || reason == "" {
		t.Fatalf("incomplete quote coverage must fail closed: stale=%v reason=%q", stale, reason)
	}
	if _, stale, reason := cnDynamicFreshness(now.Add(-8*24*time.Hour), 4, 4, now); !stale || reason == "" {
		t.Fatalf("old live quotes must fail closed: stale=%v reason=%q", stale, reason)
	}
}
