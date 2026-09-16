package scoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"trading-agents/internal/llm"
)

const llmPanelSystemPrompt = `你是 StockGod 风格的「五方独立评分」引擎。
必须严格按五位投资人公开的投资方法论打分，模拟其口径，不是真实本人观点。
分数 0-100 整数；各框架独立，允许且鼓励分歧（div 会自动算）。

评分顺序固定为数组 sc = [buffett, duan, serenity, druckenmiller, sentiment]：

1) buffett（巴菲特 · 价值/护城河）
- 看：生意是否伟大、护城河、可理解、长期现金创造力、安全边际
- 伟大但太贵 → 中高分到中分（观察仓），不是低分；质地差才低分
- 典型：KO/AAPL 类品牌护城河偏高；纯题材/小票偏低

2) duan（段永平 · 商业模式）
- 看：是否「好生意」、模式是否看懂、企业文化/本分、可长期持有
- 价格不是第一位；看懂顶级模式可给高分甚至重仓分
- 典型：平台/消费品牌/干净模式偏高；看不懂的周期/复杂结构偏低

3) serenity（Serenity · 供应链瓶颈 alpha）
- 看：是否有信息差/卡脖子环节/尚未被定价的微观 alpha
- 逻辑对但极度拥挤的明牌龙头（如超级大盘 AI 核心）→ 中低到中分（旁证锚，不是主战场）
- 中盘工业/材料/设备/上游瓶颈 → 更高分

4) druckenmiller（德鲁肯米勒 · 宏观流动性/趋势）
- 看：宏观/产业顺风、动量、能否装大仓、不对称赔率
- 顺风最强载体高分；顺风减弱或无趋势偏低；不太看传统估值

5) sentiment（情绪资金面）
- 看：拥挤度、被动/主动同向、散户亢奋、短期涨幅透支
- 全民共识拥挤 → 低分（警惕）；冰点/错杀 → 高分（逆向区）
- 与生意质量解耦

校准锚（勿照抄，用于口径）：
- NVDA ≈ [62,80,58,84,47]
- AAPL ≈ [72,88,35,60,45]
- TSM ≈ [88,90,72,87,55]
- PLTR ≈ [38,62,45,78,35]
- SMCI ≈ [28,25,58,62,60]
整体分布应偏克制：多数票落在 20-70，避免全体 60+。

只输出 JSON 对象，不要 markdown：
{"stocks":{"SYM":{"sc":[n,n,n,n,n]}}}
`

// LLMOptions controls tiered LLM batch scoring.
type LLMOptions struct {
	Enabled    bool
	Limit      int // max stocks to LLM-score (by mcap desc); 0 = none
	BatchSize  int
	Workers    int
	CachePath  string
	SkipCached bool // if false, reuse cache entries
}

// RegenerateOptions configures panel rebuild.
type RegenerateOptions struct {
	WriteIndustry bool
	SeedOriginal  bool // overlay recovered original panel scores
	LLM           LLMOptions
	LLMIndustry   bool // refine Chinese seg/sub via LLM for top-N
	IndustryLimit int
	Client        llm.LLMClient
}

// DefaultLLMCachePath returns data/llm-panel-cache.json under backend root.
func DefaultLLMCachePath(backendRoot string) string {
	return filepath.Join(backendRoot, "data", "llm-panel-cache.json")
}

// DefaultOriginalPanelPaths candidate paths for stockgod original fixture.
func DefaultOriginalPanelPaths(backendRoot string) []string {
	return []string{
		filepath.Join(backendRoot, "..", "..", "recovered_source", "stockgod-data-store", "fixtures", "data_data_us-panel-summary.json.json"),
		filepath.Join(backendRoot, "..", "frontend", "public", "data", "us-panel-summary.orig.json"),
		filepath.Join("recovered_source", "stockgod-data-store", "fixtures", "data_data_us-panel-summary.json.json"),
	}
}

type llmBatchResponse struct {
	Stocks map[string]struct {
		SC []int `json:"sc"`
	} `json:"stocks"`
}

type llmCacheFile struct {
	Updated string                `json:"updated"`
	Source  string                `json:"source"`
	Stocks  map[string]PanelStock `json:"stocks"`
}

// ApplySeed overlays scores from a seed panel (e.g. original site dump).
func ApplySeed(panel *PanelFile, seed *PanelFile) (n int) {
	if panel == nil || seed == nil {
		return 0
	}
	for sym, row := range seed.Stocks {
		if len(row.SC) < 5 {
			continue
		}
		sc := normalizeSC(row.SC)
		panel.Stocks[sym] = PanelStock{SC: sc, Div: Divergence(sc)}
		n++
	}
	return n
}

// SelectLLMUniverse returns top-N symbols by market cap.
func SelectLLMUniverse(stocks []UsStockRow, limit int) []UsStockRow {
	if limit <= 0 {
		return nil
	}
	cp := append([]UsStockRow(nil), stocks...)
	sort.Slice(cp, func(i, j int) bool {
		if cp[i].McapB == cp[j].McapB {
			return cp[i].Vol > cp[j].Vol
		}
		return cp[i].McapB > cp[j].McapB
	})
	if limit > len(cp) {
		limit = len(cp)
	}
	return cp[:limit]
}

// ScoreWithLLM batch-scores stocks via LLM and merges into panel.
func ScoreWithLLM(ctx context.Context, client llm.LLMClient, panel *PanelFile, universe []UsStockRow, opt LLMOptions) (updated int, err error) {
	if client == nil || len(universe) == 0 {
		return 0, nil
	}
	batchSize := opt.BatchSize
	if batchSize <= 0 {
		batchSize = 12
	}
	workers := opt.Workers
	if workers <= 0 {
		workers = 2
	}

	cache := loadLLMCache(opt.CachePath)
	if cache.Stocks == nil {
		cache.Stocks = map[string]PanelStock{}
	}

	pending := make([]UsStockRow, 0, len(universe))
	for _, s := range universe {
		sym := strings.ToUpper(strings.TrimSpace(s.Sym))
		if sym == "" {
			continue
		}
		if !opt.SkipCached {
			if row, ok := cache.Stocks[sym]; ok && len(row.SC) >= 5 {
				sc := normalizeSC(row.SC)
				panel.Stocks[sym] = PanelStock{SC: sc, Div: Divergence(sc)}
				updated++
				continue
			}
		}
		pending = append(pending, s)
	}
	log.Printf("[scoring/llm] universe=%d cached_hit=%d pending=%d batch=%d workers=%d",
		len(universe), updated, len(pending), batchSize, workers)

	if len(pending) == 0 {
		panel.Source = mergeSource(panel.Source, "llm-cache")
		return updated, nil
	}

	batches := chunkStocks(pending, batchSize)
	type job struct {
		idx  int
		rows []UsStockRow
	}
	jobs := make(chan job, len(batches))
	for i, b := range batches {
		jobs <- job{idx: i, rows: b}
	}
	close(jobs)

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		failN    atomic.Int32
		doneN    atomic.Int32
		firstErr error
	)
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for j := range jobs {
				scored, err := scoreBatch(ctx, client, j.rows)
				if err != nil {
					failN.Add(1)
					log.Printf("[scoring/llm] batch %d/%d failed: %v", j.idx+1, len(batches), err)
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					continue
				}
				mu.Lock()
				for sym, row := range scored {
					panel.Stocks[sym] = row
					cache.Stocks[sym] = row
					updated++
				}
				mu.Unlock()
				n := int(doneN.Add(1))
				if n%max(1, len(batches)/10) == 0 || n == len(batches) {
					log.Printf("[scoring/llm] progress batches %d/%d", n, len(batches))
				}
				// gentle pacing for DeepSeek TPM
				time.Sleep(400 * time.Millisecond)
			}
		}()
	}
	wg.Wait()

	cache.Updated = time.Now().UTC().Format(time.RFC3339)
	cache.Source = "llm-batch-v1"
	if opt.CachePath != "" {
		if err := WriteJSON(opt.CachePath, cache); err != nil {
			log.Printf("[scoring/llm] cache write: %v", err)
		}
	}

	panel.Source = mergeSource(panel.Source, "llm-batch-v1")
	panel.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	panel.Count = len(panel.Stocks)

	if updated == 0 && firstErr != nil {
		return 0, firstErr
	}
	if failN.Load() > 0 {
		log.Printf("[scoring/llm] completed with %d failed batches (kept seed/heuristic for those)", failN.Load())
	}
	return updated, nil
}

func scoreBatch(ctx context.Context, client llm.LLMClient, rows []UsStockRow) (map[string]PanelStock, error) {
	var b strings.Builder
	b.WriteString("请为以下美股打五方分。每只只给 sc 五元组，勿解释。\n\n")
	for _, s := range rows {
		fmt.Fprintf(&b, "- %s | %s | sector=%s industry=%s | seg=%s/%s | price=%.2f pct=%.2f%% mcapB=%.1f vol=%.0f country=%s\n",
			s.Sym, trimName(s.Name, 48), s.Sector, s.Industry, s.Seg, s.Sub, s.Price, s.Pct, s.McapB, s.Vol, s.Country)
	}
	b.WriteString("\n返回 JSON: {\"stocks\":{\"SYM\":{\"sc\":[b,d,s,k,e]}}}")

	var resp llmBatchResponse
	// quick model is enough; structured JSON
	if err := client.StructuredGenerate(ctx, llmPanelSystemPrompt, b.String(), false, &resp); err != nil {
		return nil, err
	}
	if len(resp.Stocks) == 0 {
		return nil, fmt.Errorf("empty llm stocks")
	}
	out := make(map[string]PanelStock, len(resp.Stocks))
	for sym, row := range resp.Stocks {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" || len(row.SC) < 5 {
			continue
		}
		sc := normalizeSC(row.SC)
		out[sym] = PanelStock{SC: sc, Div: Divergence(sc)}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid scores in llm response")
	}
	return out, nil
}

func normalizeSC(sc []int) []int {
	out := make([]int, 5)
	for i := 0; i < 5; i++ {
		v := 0
		if i < len(sc) {
			v = sc[i]
		}
		out[i] = clampInt(v, 5, 95)
	}
	return out
}

func loadLLMCache(path string) llmCacheFile {
	var c llmCacheFile
	if path == "" {
		return c
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	_ = json.Unmarshal(raw, &c)
	if c.Stocks == nil {
		c.Stocks = map[string]PanelStock{}
	}
	return c
}

func LoadPanelFile(paths ...string) (*PanelFile, string, error) {
	for _, p := range paths {
		if p == "" {
			continue
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var f PanelFile
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, p, err
		}
		if f.Stocks == nil {
			continue
		}
		f.EnsureDeterministicScalingDisclosure()
		f.EnsureValidationStatus()
		return &f, p, nil
	}
	return nil, "", fmt.Errorf("panel not found")
}

func chunkStocks(rows []UsStockRow, n int) [][]UsStockRow {
	if n <= 0 {
		n = 12
	}
	var out [][]UsStockRow
	for i := 0; i < len(rows); i += n {
		j := i + n
		if j > len(rows) {
			j = len(rows)
		}
		out = append(out, rows[i:j])
	}
	return out
}

func mergeSource(base, add string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return add
	}
	if strings.Contains(base, add) {
		return base
	}
	return base + "+" + add
}

func trimName(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ScaleHeuristicDeterministically applies a fixed transform. It is not fitted
// or validated against historical forward returns.
func ScaleHeuristicDeterministically(sc []int) []int {
	// original means approx: 36.5, 37.7, 32.8, 45.8, 41.2
	targets := []float64{36.5, 37.7, 32.8, 45.8, 41.2}
	// heuristic tends ~55-65; pull toward target with soft affine
	out := make([]int, 5)
	for i := 0; i < 5 && i < len(sc); i++ {
		v := float64(sc[i])
		// map 50→target, keep relative distance with 0.75 compression around target
		cal := targets[i] + (v-55.0)*0.65
		out[i] = clampInt(int(math.Round(cal)), 5, 95)
	}
	return out
}
