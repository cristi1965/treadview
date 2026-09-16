package scoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PanelOrder matches frontend SCORE_KEYS index order (duan → duanyongping).
var PanelOrder = []string{"buffett", "duan", "serenity", "druckenmiller", "sentiment"}

type UsStockRow struct {
	Sym      string   `json:"sym"`
	Name     string   `json:"name"`
	Price    float64  `json:"price"`
	Pct      float64  `json:"pct"`
	McapB    float64  `json:"mcapB"`
	Sector   string   `json:"sector"`
	Industry string   `json:"industry,omitempty"`
	Vol      float64  `json:"vol"`
	Country  string   `json:"country,omitempty"`
	Seg      string   `json:"seg,omitempty"`
	Sub      string   `json:"sub,omitempty"`
	Sub2     []string `json:"sub2,omitempty"`
}

type UsStocksFile struct {
	GeneratedAt string       `json:"generated_at,omitempty"`
	Count       int          `json:"count"`
	Stocks      []UsStockRow `json:"stocks"`
}

type PanelStock struct {
	SC     []int        `json:"sc"`
	Div    int          `json:"div"`
	Detail *ScoreDetail `json:"detail,omitempty"`
}

type PanelFile struct {
	Order          []string              `json:"order"`
	GeneratedAt    string                `json:"generated_at,omitempty"`
	Source         string                `json:"source,omitempty"`
	MethodVersion  string                `json:"method_version,omitempty"`
	Count          int                   `json:"count,omitempty"`
	Stocks         map[string]PanelStock `json:"stocks"`
	Validation     ValidationReport      `json:"validation"`
	MarketSubmodel *PriceModelReport     `json:"market_submodel,omitempty"`
}

// ScoreInputs are the exact snapshot fields consumed by the heuristic scorer.
type ScoreInputs struct {
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name"`
	Sector     string  `json:"sector"`
	Industry   string  `json:"industry"`
	Segment    string  `json:"segment"`
	Sub        string  `json:"sub"`
	Price      float64 `json:"price"`
	DayPct     float64 `json:"day_pct"`
	MarketCapB float64 `json:"market_cap_b"`
	Volume     float64 `json:"volume"`
}

// ScoreDetail is sufficient to reproduce the heuristic and deterministic scaling.
type ScoreDetail struct {
	MethodVersion        string                      `json:"method_version"`
	Inputs               ScoreInputs                 `json:"inputs"`
	Components           map[string][]ScoreComponent `json:"components"`
	RawScores            []int                       `json:"raw_scores"`
	FinalScores          []int                       `json:"final_scores"`
	PanelScores          []int                       `json:"panel_scores"`
	ReproducesPanel      bool                        `json:"reproduces_panel"`
	DeterministicScaling string                      `json:"deterministic_scaling"`
}

// ScoreComponent records one deterministic rule contribution to a raw factor score.
type ScoreComponent struct {
	Rule  string  `json:"rule"`
	Value float64 `json:"value"`
}

const HeuristicMethodVersion = "stockgod-five-factor-heuristic-v2"

const DeterministicScalingDisclosure = "deterministic scaling: round(target[i] + (raw[i] - 55) * 0.65), targets=[36.5,37.7,32.8,45.8,41.2], clamp=5..95; not historical calibration"

// BuildPanel computes five-factor scores for every row in the universe.
func BuildPanel(stocks []UsStockRow) *PanelFile {
	return BuildPanelOpts(stocks, true)
}

// BuildPanelOpts computes scores; applyScaling=true applies a fixed, disclosed transform.
func BuildPanelOpts(stocks []UsStockRow, applyScaling bool) *PanelFile {
	src := "local-heuristic-v1"
	if applyScaling {
		src = "local-heuristic-v2-deterministic-scaling"
	}
	out := &PanelFile{
		Order:         append([]string{}, PanelOrder...),
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Source:        src,
		MethodVersion: HeuristicMethodVersion,
		Stocks:        make(map[string]PanelStock, len(stocks)),
		Validation: ValidationReport{
			Status: "unvalidated", Method: "strict-time-split-v1",
			Benchmark: "benchmark-adjusted forward return; majority-direction baseline",
			Reasons:   []string{"no point-in-time historical score and market-cap snapshots with forward benchmark returns were supplied"},
		},
	}
	for _, s := range stocks {
		sym := strings.TrimSpace(strings.ToUpper(s.Sym))
		if sym == "" {
			continue
		}
		detail := ExplainFive(s, applyScaling)
		out.Stocks[sym] = PanelStock{SC: detail.FinalScores, Div: Divergence(detail.FinalScores), Detail: &detail}
	}
	out.Count = len(out.Stocks)
	return out
}

// EnrichIndustry fills missing Chinese seg/sub from English sector/industry.
func EnrichIndustry(stocks []UsStockRow) (filled int) {
	for i := range stocks {
		wasEmpty := stocks[i].Seg == ""
		seg, sub := MapIndustry(stocks[i].Sector, stocks[i].Industry, stocks[i].Name, stocks[i].Seg, stocks[i].Sub)
		if stocks[i].Seg == "" {
			stocks[i].Seg = seg
			if wasEmpty && seg != "" {
				filled++
			}
		}
		if stocks[i].Sub == "" {
			stocks[i].Sub = sub
		}
	}
	return filled
}

// ScoreFive returns [buffett, duan, serenity, druck, sentiment] before deterministic scaling.
func ScoreFive(s UsStockRow) []int {
	return scoreFiveRaw(s)
}

// ExplainFive captures the exact inputs and pre/post-scaling values used by the scorer.
func ExplainFive(s UsStockRow, applyScaling bool) ScoreDetail {
	raw, components := scoreFiveExplained(s)
	final := append([]int{}, raw...)
	scaling := "none"
	if applyScaling {
		final = ScaleHeuristicDeterministically(raw)
		scaling = DeterministicScalingDisclosure
	}
	return ScoreDetail{
		MethodVersion: HeuristicMethodVersion,
		Inputs: ScoreInputs{
			Symbol: strings.ToUpper(strings.TrimSpace(s.Sym)), Name: s.Name, Sector: s.Sector,
			Industry: s.Industry, Segment: s.Seg, Sub: s.Sub, Price: s.Price,
			DayPct: s.Pct, MarketCapB: s.McapB, Volume: s.Vol,
		},
		Components: components, RawScores: raw, FinalScores: final, PanelScores: append([]int{}, final...),
		ReproducesPanel: true, DeterministicScaling: scaling,
	}
}

// EnsureDeterministicScalingDisclosure upgrades legacy naming only when the
// stored raw inputs reproduce both final and panel scores. It never upgrades
// validation status or guesses missing score evidence.
func (p *PanelFile) EnsureDeterministicScalingDisclosure() {
	if p == nil {
		return
	}
	if p.Source == "local-heuristic-v2-calibrated" {
		p.Source = "local-heuristic-v2-deterministic-scaling"
	}
	for symbol, row := range p.Stocks {
		if row.Detail == nil || row.Detail.DeterministicScaling != "" || row.Detail.MethodVersion != HeuristicMethodVersion {
			continue
		}
		expected := ScaleHeuristicDeterministically(row.Detail.RawScores)
		if !samePanelScores(expected, row.Detail.FinalScores) || !samePanelScores(row.Detail.FinalScores, row.SC) {
			continue
		}
		row.Detail.DeterministicScaling = DeterministicScalingDisclosure
		row.Detail.PanelScores = append([]int(nil), row.SC...)
		row.Detail.ReproducesPanel = true
		p.Stocks[symbol] = row
	}
}

func samePanelScores(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func scoreFiveRaw(s UsStockRow) []int {
	raw, _ := scoreFiveExplained(s)
	return raw
}

func scoreFiveExplained(s UsStockRow) ([]int, map[string][]ScoreComponent) {
	hay := strings.ToLower(s.Sector + " " + s.Industry + " " + s.Seg + " " + s.Sub + " " + s.Name)
	mcap := s.McapB
	pct := s.Pct
	absPct := math.Abs(pct)
	components := map[string][]ScoreComponent{}
	add := func(factor, rule string, target *float64, value float64) {
		*target += value
		components[factor] = append(components[factor], ScoreComponent{Rule: rule, Value: value})
	}

	// --- Buffett: durable economics + valuation caution ---
	buffett := 0.0
	add("buffett", "base", &buffett, 48)
	switch {
	case containsAny(hay, "consumer defensive", "staples", "必需消费", "beverage", "household"):
		add("buffett", "durable-consumer", &buffett, 18)
	case containsAny(hay, "healthcare", "医疗", "insurance", "bank", "financial", "金融"):
		add("buffett", "healthcare-or-financial", &buffett, 14)
	case containsAny(hay, "utilities", "公用", "railroad", "industrial conglomerate"):
		add("buffett", "utility-or-durable-industrial", &buffett, 10)
	case containsAny(hay, "software", "internet", "platform", "semiconductor", "科技"):
		add("buffett", "technology-franchise", &buffett, 8)
	case containsAny(hay, "biotech", "oil", "mining", "crypto", "spac"):
		add("buffett", "cyclical-or-speculative", &buffett, -10)
	}
	if mcap >= 200 {
		add("buffett", "market-cap>=200B", &buffett, 8)
	} else if mcap >= 50 {
		add("buffett", "market-cap>=50B", &buffett, 4)
	} else if mcap > 0 && mcap < 2 {
		add("buffett", "market-cap<2B", &buffett, -12)
	}
	// Crowded mega-growth without margin of safety
	if mcap >= 500 && absPct > 2.5 {
		add("buffett", "mega-cap-high-daily-move", &buffett, -6)
	}
	if absPct > 8 {
		add("buffett", "absolute-daily-move>8pct", &buffett, -8)
	}

	// --- Duan: clean business model / brand / platform ---
	duan := 0.0
	add("duan", "base", &duan, 50)
	switch {
	case containsAny(hay, "software", "internet content", "interactive media", "platform", "consumer electronics"):
		add("duan", "platform-or-consumer-electronics", &duan, 22)
	case containsAny(hay, "semiconductor", "ai算力", "gpu", "fabless"):
		add("duan", "semiconductor-or-ai-compute", &duan, 18)
	case containsAny(hay, "beverage", "restaurant", "apparel", "footwear", "luxury", "brand"):
		add("duan", "consumer-brand", &duan, 14)
	case containsAny(hay, "banks", "insurance", "asset management"):
		add("duan", "financial-franchise", &duan, 6)
	case containsAny(hay, "biotech", "exploration", "coal", "steel"):
		add("duan", "complex-or-cyclical-business", &duan, -12)
	}
	if mcap >= 100 {
		add("duan", "market-cap>=100B", &duan, 6)
	}
	if pct < -6 {
		add("duan", "daily-return<-6pct", &duan, -4)
	}

	// --- Serenity: bottleneck / underfollowed alpha ---
	serenity := 0.0
	add("serenity", "base", &serenity, 52)
	switch {
	case containsAny(hay, "semiconductor equipment", "electronic components", "copper", "equipment", "materials", "industrial machinery", "electrical equipment"):
		add("serenity", "bottleneck-equipment-or-material", &serenity, 18)
	case containsAny(hay, "communication equipment", "electronic manufacturing", "packaging", "memory", "hbm"):
		add("serenity", "supply-chain-bottleneck", &serenity, 14)
	case containsAny(hay, "software—infrastructure", "software - infrastructure", "cloud"):
		add("serenity", "cloud-infrastructure", &serenity, 4)
	}
	if mcap >= 800 {
		add("serenity", "market-cap>=800B", &serenity, -22)
	} else if mcap >= 300 {
		add("serenity", "market-cap>=300B", &serenity, -12)
	} else if mcap >= 5 && mcap <= 80 {
		add("serenity", "market-cap-5B-to-80B", &serenity, 10)
	} else if mcap > 0 && mcap < 1 {
		add("serenity", "market-cap<1B", &serenity, -8)
	}
	if containsAny(hay, "nvidia", "apple", "microsoft", "alphabet", "amazon", "meta platforms", "tesla") {
		add("serenity", "mega-cap-crowding", &serenity, -10)
	}

	// --- Druckenmiller: trend / liquidity / macro beta ---
	druck := 0.0
	add("druckenmiller", "base", &druck, 50)
	add("druckenmiller", "clamp(day_pct*3.2,-22,28)", &druck, clamp(pct*3.2, -22, 28))
	if containsAny(hay, "technology", "communication", "semiconductor", "software", "internet", "科技", "通信") {
		add("druckenmiller", "technology-or-communication", &druck, 8)
	}
	if containsAny(hay, "utilities", "staples", "reit") && pct < 1 {
		add("druckenmiller", "defensive-sector-and-return<1pct", &druck, -6)
	}
	if mcap >= 50 {
		add("druckenmiller", "market-cap>=50B", &druck, 4)
	}
	if s.Vol > 0 && mcap > 0 {
		// crude attention: dollar volume proxy
		dvol := s.Vol * math.Max(s.Price, 1) / 1e9 // ~$B traded
		if dvol > 2 {
			add("druckenmiller", "dollar-volume>2B", &druck, 6)
		} else if dvol < 0.05 {
			add("druckenmiller", "dollar-volume<0.05B", &druck, -8)
		}
	}

	// --- Sentiment: crowding / reverse heat ---
	sent := 0.0
	add("sentiment", "base", &sent, 55)
	if pct >= 6 {
		add("sentiment", "daily-return>=6pct", &sent, -18)
	} else if pct >= 3 {
		add("sentiment", "daily-return>=3pct", &sent, -10)
	} else if pct <= -6 {
		add("sentiment", "daily-return<=-6pct", &sent, 16)
	} else if pct <= -3 {
		add("sentiment", "daily-return<=-3pct", &sent, 8)
	}
	if mcap >= 500 {
		add("sentiment", "market-cap>=500B", &sent, -10)
	}
	if containsAny(hay, "nvidia", "tesla", "coinbase", "microstrategy") {
		add("sentiment", "crowded-symbol", &sent, -8)
	}
	if s.Vol > 80_000_000 && pct > 2 {
		add("sentiment", "volume>80M-and-return>2pct", &sent, -8)
	}
	if absPct < 0.8 && mcap >= 20 {
		add("sentiment", "calm-tape-and-market-cap>=20B", &sent, 4)
	}

	scores := []int{
		clampInt(int(math.Round(buffett)), 5, 95),
		clampInt(int(math.Round(duan)), 5, 95),
		clampInt(int(math.Round(serenity)), 5, 95),
		clampInt(int(math.Round(druck)), 5, 95),
		clampInt(int(math.Round(sent)), 5, 95),
	}
	return scores, components
}

func Divergence(sc []int) int {
	// Original stockgod panel uses score range (max-min), not stdev.
	if len(sc) == 0 {
		return 0
	}
	lo, hi := sc[0], sc[0]
	for _, v := range sc[1:] {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return hi - lo
}

func MapIndustry(sector, industry, name, existingSeg, existingSub string) (seg, sub string) {
	if existingSeg != "" {
		seg = existingSeg
	}
	if existingSub != "" {
		sub = existingSub
	}
	hay := strings.ToLower(sector + " " + industry + " " + name)

	if seg == "" {
		switch {
		case containsAny(hay, "technology", "software", "semiconductor", "information technology"):
			seg = "科技"
		case containsAny(hay, "healthcare", "biotechnology", "drug", "medical", "health care"):
			seg = "医疗"
		case containsAny(hay, "financial", "bank", "insurance", "capital markets", "asset management"):
			seg = "金融"
		case containsAny(hay, "consumer cyclical", "consumer discretionary", "auto", "retail", "restaurant", "apparel"):
			seg = "可选消费"
		case containsAny(hay, "consumer defensive", "consumer staples", "beverage", "packaged foods", "household"):
			seg = "必需消费"
		case containsAny(hay, "industrials", "aerospace", "machinery", "industrial", "transport"):
			seg = "工业"
		case containsAny(hay, "basic materials", "materials", "chemical", "mining", "steel", "copper"):
			seg = "材料"
		case containsAny(hay, "energy", "oil", "gas", "renewable"):
			seg = "能源"
		case containsAny(hay, "utilities", "electric", "gas utilities", "water"):
			seg = "公用事业"
		case containsAny(hay, "real estate", "reit"):
			seg = "地产"
		case containsAny(hay, "communication", "media", "telecom", "entertainment"):
			seg = "通信媒体"
		default:
			seg = "其他"
		}
	}

	if sub == "" {
		switch {
		case containsAny(hay, "semiconductor", "gpu", "chip"):
			sub = "AI算力"
		case containsAny(hay, "software", "saas", "cloud", "application software"):
			sub = "软件"
		case containsAny(hay, "internet", "interactive media", "social"):
			sub = "互联网"
		case containsAny(hay, "biotech", "biotechnology"):
			sub = "生物科技"
		case containsAny(hay, "drug manufacturer", "pharma"):
			sub = "制药"
		case containsAny(hay, "bank"):
			sub = "银行"
		case containsAny(hay, "insurance"):
			sub = "保险"
		case containsAny(hay, "auto manufacturer", "auto makers", "ev"):
			sub = "汽车"
		case containsAny(hay, "oil", "gas", "exploration"):
			sub = "油气"
		case containsAny(hay, "reit", "real estate"):
			sub = "REIT"
		case containsAny(hay, "aerospace", "defense"):
			sub = "航天军工"
		default:
			if industry != "" {
				sub = industry
			} else {
				sub = sector
			}
		}
	}
	return seg, sub
}

// LoadUsStocks reads us-stocks.json from common locations.
func LoadUsStocks(paths ...string) (*UsStocksFile, string, error) {
	tried := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		tried = append(tried, p)
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var f UsStocksFile
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, p, fmt.Errorf("parse %s: %w", p, err)
		}
		return &f, p, nil
	}
	return nil, "", fmt.Errorf("us-stocks.json not found in %v", tried)
}

// DefaultUsStocksPaths returns candidate paths relative to cwd / backend root.
func DefaultUsStocksPaths(backendRoot string) []string {
	return []string{
		filepath.Join(backendRoot, "..", "frontend", "public", "data", "us-stocks.json"),
		filepath.Join(backendRoot, "data", "us-stocks.json"),
		filepath.Join("app", "frontend", "public", "data", "us-stocks.json"),
		filepath.Join("frontend", "public", "data", "us-stocks.json"),
		"data/us-stocks.json",
	}
}

// DefaultPanelPaths returns write targets for panel summary.
func DefaultPanelPaths(backendRoot string) []string {
	return []string{
		filepath.Join(backendRoot, "..", "frontend", "public", "data", "us-panel-summary.json"),
		filepath.Join(backendRoot, "data", "us-panel-summary.json"),
	}
}

// WriteJSON writes pretty JSON to path, creating parent dirs.
func WriteJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o644)
}

// Regenerate rebuilds panel (+ optional industry fill) and writes outputs.
func Regenerate(backendRoot string, writeIndustry bool) (*PanelFile, []string, error) {
	return RegenerateWith(backendRoot, RegenerateOptions{
		WriteIndustry: writeIndustry,
		SeedOriginal:  true,
	})
}

// RegenerateWith rebuilds panel with seed / LLM tiered scoring options.
func RegenerateWith(backendRoot string, opt RegenerateOptions) (*PanelFile, []string, error) {
	f, src, err := LoadUsStocks(DefaultUsStocksPaths(backendRoot)...)
	if err != nil {
		return nil, nil, err
	}
	if opt.WriteIndustry {
		_ = EnrichIndustry(f.Stocks)
	}
	if opt.LLMIndustry && opt.Client != nil {
		lim := opt.IndustryLimit
		if lim <= 0 {
			lim = opt.LLM.Limit
		}
		if lim <= 0 {
			lim = 100
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		n, err := EnrichIndustryWithLLM(ctx, opt.Client, f.Stocks, lim, 20)
		cancel()
		if err != nil {
			log.Printf("[scoring] industry LLM: %v", err)
		} else {
			log.Printf("[scoring] industry LLM updated %d rows", n)
		}
	}
	if opt.WriteIndustry || opt.LLMIndustry {
		f.GeneratedAt = time.Now().Format("2006-01-02 15:04 ET")
		f.Count = len(f.Stocks)
		if err := WriteJSON(src, f); err != nil {
			return nil, nil, fmt.Errorf("write us-stocks: %w", err)
		}
	}

	panel := BuildPanelOpts(f.Stocks, true)

	if opt.SeedOriginal {
		if seed, seedPath, err := LoadPanelFile(DefaultOriginalPanelPaths(backendRoot)...); err == nil {
			n := ApplySeed(panel, seed)
			panel.Source = mergeSource(panel.Source, "seed-orig")
			log.Printf("[scoring] seeded %d scores from %s", n, seedPath)
		} else {
			log.Printf("[scoring] original panel seed skipped: %v", err)
		}
	}

	if opt.LLM.Enabled && opt.Client != nil && opt.LLM.Limit > 0 {
		if opt.LLM.CachePath == "" {
			opt.LLM.CachePath = DefaultLLMCachePath(backendRoot)
		}
		universe := SelectLLMUniverse(f.Stocks, opt.LLM.Limit)
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
		defer cancel()
		n, err := ScoreWithLLM(ctx, opt.Client, panel, universe, opt.LLM)
		if err != nil {
			log.Printf("[scoring] LLM scoring partial/error: %v (updated=%d)", err, n)
			if n == 0 {
				return nil, nil, fmt.Errorf("llm scoring failed: %w", err)
			}
		} else {
			log.Printf("[scoring] LLM updated %d scores", n)
		}
	}

	panel.Count = len(panel.Stocks)
	panel.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	written := make([]string, 0, 2)
	for _, p := range DefaultPanelPaths(backendRoot) {
		if err := WriteJSON(p, panel); err != nil {
			continue
		}
		written = append(written, p)
	}
	if len(written) == 0 {
		return panel, written, fmt.Errorf("failed to write panel files")
	}
	return panel, written, nil
}

func containsAny(hay string, keys ...string) bool {
	for _, k := range keys {
		if strings.Contains(hay, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
