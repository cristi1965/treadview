package api

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/dataflows"
)

type ChatMessageReq struct {
	Role    string `json:"role"` // "user" | "assistant" | "system"
	Content string `json:"content"`
}

type ChatAskRequest struct {
	Question       string           `json:"question" binding:"required"`
	Persona        string           `json:"persona"` // "cro" | "quant" | "hot_money" | "macro" | "night_owl" | "general"
	SelectedSymbol string           `json:"selectedSymbol"`
	History        []ChatMessageReq `json:"history"`
	UseDeep        bool             `json:"useDeep"`
}

type ChatAskResponse struct {
	Answer     string         `json:"answer"`
	Persona    string         `json:"persona"`
	Symbol     string         `json:"symbol,omitempty"`
	MarketData map[string]any `json:"marketData,omitempty"`
	Timestamp  int64          `json:"timestamp"`
}

var symbolDetector = regexp.MustCompile(`(?i)\b([0-9]{6}|[A-Z]{1,5})\b`)

// HandleChatAsk handles custom financial trading Q&A with live market context.
func (h *Handler) HandleChatAsk(c *gin.Context) {
	var req ChatAskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Question is required"})
		return
	}

	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Question cannot be empty"})
		return
	}

	llmClient := h.orchestrator.GetLLMClient()
	if llmClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "LLM client not configured or unavailable"})
		return
	}

	// 1. Detect or use symbol
	symbol := strings.ToUpper(strings.TrimSpace(req.SelectedSymbol))
	if symbol == "" {
		matches := symbolDetector.FindAllString(req.Question, -1)
		for _, m := range matches {
			mUpper := strings.ToUpper(m)
			// Filter out common english words
			if mUpper == "THE" || mUpper == "FOR" || mUpper == "AND" || mUpper == "HOW" || mUpper == "CAN" || mUpper == "ETF" || mUpper == "NAV" || mUpper == "BUY" || mUpper == "SELL" {
				continue
			}
			symbol = mUpper
			break
		}
	}

	// 2. Fetch live data for context if symbol detected
	marketCtx := ""
	liveDataMap := map[string]any{}

	if symbol != "" {
		quoteContext, quoteMetadata := buildChatQuoteContext(symbol, boundedQuotesForRequest([]string{symbol}), time.Now())
		marketCtx += quoteContext
		liveDataMap = quoteMetadata

		// If it looks like a QDII ETF, check premium
		if len(symbol) == 6 && (strings.HasPrefix(symbol, "513") || strings.HasPrefix(symbol, "159") || strings.HasPrefix(symbol, "161")) {
			qdiiClient := dataflows.NewQDIIPremiumClient()
			if premiums, err := qdiiClient.GetPremiums(); err == nil {
				for _, p := range premiums {
					if p.Code == symbol && p.Status == "complete" && p.NAV != nil && p.PremiumPct != nil {
						marketCtx += fmt.Sprintf("- 基金净值 (NAV)：%.4f (%s)\n- 当前场内溢价率：+%.2f%%\n- 风险等级：%s (若溢价>5%% 极易杀溢价踩踏)\n",
							*p.NAV, p.NavDate, *p.PremiumPct, p.Level)
						liveDataMap["nav"] = *p.NAV
						liveDataMap["premiumPct"] = *p.PremiumPct
						liveDataMap["level"] = p.Level
						break
					}
				}
			}
		}
	}

	// Also append general market temperature
	globalResult := boundedQuotesForRequest([]string{"NDX", "SPX", "VIX", "TNX"})
	globalStale, _ := quoteResultFreshness(globalResult, time.Now())
	if len(globalResult.quotes) > 0 && !globalStale {
		marketCtx += "\n【全球核心宏观温度】\n"
		if q, ok := globalResult.quotes["NDX"]; ok && q.Price > 0 {
			marketCtx += fmt.Sprintf("- 纳斯达克100 (NDX): %.2f (%.2f%%)\n", q.Price, q.Pct)
		}
		if q, ok := globalResult.quotes["SPX"]; ok && q.Price > 0 {
			marketCtx += fmt.Sprintf("- 标普500 (SPX): %.2f (%.2f%%)\n", q.Price, q.Pct)
		}
		if q, ok := globalResult.quotes["VIX"]; ok && q.Price > 0 {
			marketCtx += fmt.Sprintf("- 恐慌指数 (VIX): %.2f (%.2f%%)\n", q.Price, q.Pct)
		}
	}

	// 3. Craft Persona System Prompt
	systemPrompt := buildPersonaPrompt(req.Persona, marketCtx)

	// 4. Construct Conversation Prompt
	var conversationSb strings.Builder
	if len(req.History) > 0 {
		conversationSb.WriteString("以下是前序对话上下文：\n")
		// Limit to last 6 messages
		start := 0
		if len(req.History) > 6 {
			start = len(req.History) - 6
		}
		for _, msg := range req.History[start:] {
			conversationSb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
		}
		conversationSb.WriteString("\n用户当前最新提问：\n")
	}
	conversationSb.WriteString(req.Question)

	// 5. Generate with LLM
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()

	answer, err := llmClient.Generate(ctx, systemPrompt, conversationSb.String(), req.UseDeep)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AI 生成失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, ChatAskResponse{
		Answer:     answer,
		Persona:    req.Persona,
		Symbol:     symbol,
		MarketData: liveDataMap,
		Timestamp:  time.Now().UnixMilli(),
	})
}

func buildChatQuoteContext(symbol string, result quoteFetchResult, now time.Time) (string, map[string]any) {
	metadata := map[string]any{}
	quote, ok := result.quotes[symbol]
	if !ok || quote.Price <= 0 {
		return "", metadata
	}
	stale, reason := quoteResultFreshness(result, now)
	metadata["price"] = quote.Price
	metadata["pct"] = quote.Pct
	metadata["currency"] = quote.Currency
	metadata["source"] = quote.Source
	metadata["dataTime"] = quote.DataTime
	metadata["stale"] = stale
	if reason != "" {
		metadata["staleReason"] = reason
	}
	heading := "【实时标的行情】"
	guard := ""
	if stale {
		heading = "【非实时参考行情】"
		guard = "- 此行情已降级，不得作为当前实时事实或交易点位依据。\n"
	}
	context := fmt.Sprintf("\n%s\n%s- 标的代码：%s\n- 观测价格：%.3f %s\n- 观测涨跌幅：%.2f%%\n- 昨日收盘价：%.3f\n- 数据来源：%s\n- 数据时间：%s\n",
		heading, guard, symbol, quote.Price, quote.Currency, quote.Pct, quote.PrevClose, quote.Source, quote.DataTime)
	return context, metadata
}

func buildPersonaPrompt(persona string, marketCtx string) string {
	base := `你是 NEMO「我不是神」实战金融交易 AI 导师系统，专为解决用户的真实股票、ETF、宏观与交易决策痛点而生。

【核心行为准则】
1. 严禁空洞废话或教科书式套话，必须给出清晰、果断、有具体量化点位的战术指导。
2. 任何建议必须考虑「风险第一、生存第一」：严格遵循 1:3 盈亏比、坚硬止损、拒绝盲目扛单。
3. 当标的处于 QDII 高溢价（>5%）时，必须严厉警告杀溢价风险，禁止无脑补仓。
4. 所有交易参数、条件单和执行推演只能用于本系统 Paper 模拟；不得指导用户向券商或交易所提交真实订单。
5. 回答采用结构化清晰 Markdown：
   - 🎯【核心战术结论】（加仓 / 减仓 / 观望 / 止损 / 条件单挂单）
   - 📊【量化点位参考】（建议买入价、坚硬止损价、目标止盈位）
   - 🧠【逻辑与多情景推演】（乐观 Bull / 中性 Base / 悲观 Bear）
   - 🛡️【风控红线与纪律】
`

	switch strings.ToLower(persona) {
	case "cro":
		base += `
【当前专家角色：🛡️ 首席风控官 (Chief Risk Officer)】
- 风格：极度冷酷、克制、严谨，把保住本金作为绝对最高法则。
- 重点：先算如果跌了最多亏多少、单笔亏损是否超过总资金 1.5%、止损点位定在哪里、拒绝一切侥幸与赌徒心理。
`
	case "quant":
		base += `
【当前专家角色：🧮 华尔街量化策略师 (Quant Hedge Fund Strategist)】
- 风格：以数学期望 EV、胜率与赔率分布、非对称盈亏比、均值回归为核心逻辑。
- 重点：分析动量、均线支撑阻力、波动率挤压、资金流向，给出最符合数学优势的期望值解法。
`
	case "hot_money":
		base += `
【当前专家角色：🗡️ A 股顶级游资操盘手 (Hot Money Master)】
- 风格：狼性、敏锐、聚焦核心主线与隔夜美股映射。
- 重点：聚焦主流板块（CPO/算力/AI/芯片）、识别是主力吸筹还是出货、开盘竞价强弱分时、超短线快进快出，绝不恋战杂毛。
`
	case "macro":
		base += `
【当前专家角色：🌐 全球宏观对冲基金经理 (Global Macro Portfolio Manager)】
- 风格：大格局、大周期、跨资产穿透。
- 重点：分析美联储利率路径、美债收益率曲线、美元/日元套息交易、地缘政治与大宗商品轮动对权益市场的波及影响。
`
	case "night_owl":
		base += `
【当前专家角色：🌙 不盯盘自动化战术导师 (Night-Owl Execution Mentor)】
- 风格：专为白天上班/打零工、无法实时盯盘的交易者定制。
- 重点：指导如何在晚上做完功课后，在本系统 Paper 工作台生成限价、触发与保护止损模拟计划；不连接券商，不执行真实交易。
`
	default:
		base += `
【当前专家角色：🤖 NEMO 综合全能交易导师】
- 融合风控、量化、宏观与实操战术，给予全方位的交易指导。
`
	}

	if marketCtx != "" {
		base += fmt.Sprintf("\n【系统注入的市场数据（严格遵守其中的来源与时效标记）】\n%s\n", marketCtx)
	}

	return base
}
