package scoring

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"trading-agents/internal/llm"
)

const llmIndustrySystemPrompt = `你是美股行业分层标注器，服务 StockGod 热力图的中文行业分层。
为每只股票给出：
- seg: 一级行业，只能从以下选择：科技, 医疗, 金融, 可选消费, 必需消费, 工业, 材料, 能源, 公用事业, 地产, 通信媒体, 其他
- sub: 二级主题（短中文，如 AI算力/软件/半导体设备/银行/生物科技/汽车/油气…）
- sub2: 0-2 个附加标签数组（可空）

只输出 JSON：
{"stocks":{"NVDA":{"seg":"科技","sub":"AI算力","sub2":["软件"]}}}
`

type llmIndustryResponse struct {
	Stocks map[string]struct {
		Seg  string   `json:"seg"`
		Sub  string   `json:"sub"`
		Sub2 []string `json:"sub2"`
	} `json:"stocks"`
}

// EnrichIndustryWithLLM refines Chinese seg/sub for a universe (usually top-N).
func EnrichIndustryWithLLM(ctx context.Context, client llm.LLMClient, stocks []UsStockRow, limit, batchSize int) (updated int, err error) {
	if client == nil {
		return 0, fmt.Errorf("nil llm client")
	}
	universe := SelectLLMUniverse(stocks, limit)
	if len(universe) == 0 {
		return 0, nil
	}
	if batchSize <= 0 {
		batchSize = 20
	}
	bySym := map[string]*UsStockRow{}
	for i := range stocks {
		bySym[strings.ToUpper(stocks[i].Sym)] = &stocks[i]
	}

	batches := chunkStocks(universe, batchSize)
	for bi, batch := range batches {
		select {
		case <-ctx.Done():
			return updated, ctx.Err()
		default:
		}
		var b strings.Builder
		b.WriteString("标注以下股票的中文行业分层：\n")
		for _, s := range batch {
			fmt.Fprintf(&b, "- %s | %s | sector=%s industry=%s | current_seg=%s sub=%s\n",
				s.Sym, trimName(s.Name, 40), s.Sector, s.Industry, s.Seg, s.Sub)
		}
		var resp llmIndustryResponse
		if err := client.StructuredGenerate(ctx, llmIndustrySystemPrompt, b.String(), false, &resp); err != nil {
			log.Printf("[scoring/industry-llm] batch %d failed: %v", bi+1, err)
			continue
		}
		for sym, row := range resp.Stocks {
			sym = strings.ToUpper(strings.TrimSpace(sym))
			target := bySym[sym]
			if target == nil {
				continue
			}
			if seg := strings.TrimSpace(row.Seg); seg != "" {
				target.Seg = seg
			}
			if sub := strings.TrimSpace(row.Sub); sub != "" {
				target.Sub = sub
			}
			if len(row.Sub2) > 0 {
				target.Sub2 = row.Sub2
			}
			updated++
		}
		log.Printf("[scoring/industry-llm] batch %d/%d updated_total=%d", bi+1, len(batches), updated)
		time.Sleep(350 * time.Millisecond)
	}
	return updated, nil
}
