package agents

import (
	"context"
	"fmt"
	"log"

	"trading-agents/internal/dataflows"
	"trading-agents/internal/llm"
)

// MarketAnalyst performs technical analysis using stock data and indicators.
func MarketAnalyst(ctx context.Context, client llm.LLMClient, state *AgentState, tools *dataflows.ToolRegistry, onEvent func(NodeEvent)) (string, error) {
	emitStart(onEvent, "Market Analyst")

	systemPrompt := fmt.Sprintf(`You are a trading assistant tasked with analyzing financial markets.
Your role is to select the most relevant indicators for a given market condition from the following list.
Choose up to 8 indicators that provide complementary insights without redundancy.

Categories:
Moving Averages: close_50_sma, close_200_sma, close_10_ema
MACD: macd, macds, macdh
Momentum: rsi
Volatility: boll, boll_ub, boll_lb, atr
Volume: vwma

Today's date is %s. The instrument is %s. %s

Use the supplied preflight-validated price history as the source of truth. Do not infer unavailable real-time prices or indicators.

Write a detailed source-bound research report. Do not issue an investment action. Append a Markdown table at the end.
%s`, state.TradeDate, state.CompanyOfInterest, state.InstrumentContext, languageInstruction(state))

	payload, _ := tools.PreflightPayload("get_historical_evidence")
	report, err := client.Generate(ctx, systemPrompt, fmt.Sprintf(
		"Analyze %s as of %s using only this preflight-validated market payload:\n\n%s",
		state.CompanyOfInterest, state.TradeDate, truncate(payload, 12000)), false)
	if err != nil {
		emitError(onEvent, "Market Analyst", err)
		return "", err
	}

	emitComplete(onEvent, "Market Analyst", report)
	return report, nil
}

// FundamentalsAnalyst analyzes company financial fundamentals.
func FundamentalsAnalyst(ctx context.Context, client llm.LLMClient, state *AgentState, tools *dataflows.ToolRegistry, onEvent func(NodeEvent)) (string, error) {
	emitStart(onEvent, "Fundamentals Analyst")

	systemPrompt := fmt.Sprintf(`You are a researcher tasked with analyzing fundamental information about a company.
Write a comprehensive report covering: financial documents, company profile, basic financials, and financial history.
Include only detail supported by tool output. State unknown when evidence is unavailable; do not issue an investment action.
Append a Markdown table at the end to organize key points.

Use the supplied fundamentals payload, whose fiscal period was validated before this model call. Do not request or infer a substitute.

Today's date is %s. The instrument is %s. %s
%s`, state.TradeDate, state.CompanyOfInterest, state.InstrumentContext, languageInstruction(state))

	payload, _ := tools.PreflightPayload("get_fundamentals")
	report, err := client.Generate(ctx, systemPrompt, fmt.Sprintf(
		"Analyze %s fundamentals as of %s using only this preflight-validated payload:\n\n%s",
		state.CompanyOfInterest, state.TradeDate, truncate(payload, 12000)), false)
	if err != nil {
		emitError(onEvent, "Fundamentals Analyst", err)
		return "", err
	}

	emitComplete(onEvent, "Fundamentals Analyst", report)
	return report, nil
}

// SentimentAnalyst analyzes market sentiment from social media and news.
func SentimentAnalyst(ctx context.Context, client llm.LLMClient, state *AgentState, tools *dataflows.ToolRegistry, onEvent func(NodeEvent)) (string, error) {
	emitStart(onEvent, "Sentiment Analyst")

	systemPrompt := fmt.Sprintf(`You are a researcher tasked with analyzing captured news sentiment about a company.
Use only the supplied preflight-validated news response. Provide a sentiment report covering:
1. Source-by-source breakdown with specific evidence
2. Cross-article divergences and alignments
3. Dominant narrative themes in the captured articles
4. Catalysts and risks surfaced by the data
5. A markdown table summarizing key sentiment signals

Rate overall sentiment as: Bullish / Mildly Bullish / Neutral / Mixed / Mildly Bearish / Bearish
Provide an overall score on a 0-10 scale (0=maximally bearish, 5=neutral, 10=maximally bullish).

Today's date is %s. The instrument is %s. %s
%s
%s`, state.TradeDate, state.CompanyOfInterest, state.InstrumentContext, positionGuardrails(state), languageInstruction(state))

	payload, _ := tools.PreflightPayload("get_news")
	report, err := client.Generate(ctx, systemPrompt, fmt.Sprintf(
		"Analyze sentiment for %s as of %s using only this preflight-validated news payload:\n\n%s",
		state.CompanyOfInterest, state.TradeDate, truncate(payload, 12000)), false)
	if err != nil {
		emitError(onEvent, "Sentiment Analyst", err)
		return "", err
	}

	emitComplete(onEvent, "Sentiment Analyst", report)
	return report, nil
}

// NewsAnalyst analyzes news, macro indicators, insider transactions, and prediction markets.
func NewsAnalyst(ctx context.Context, client llm.LLMClient, state *AgentState, tools *dataflows.ToolRegistry, onEvent func(NodeEvent)) (string, error) {
	emitStart(onEvent, "News Analyst")

	systemPrompt := fmt.Sprintf(`You are a researcher tasked with analyzing the current news and world affairs relevant to a company's stock.
Research and analyze only the preflight-validated ticker-specific news. Other source classes remain unknown unless separately captured in a future validated contract.

Write a comprehensive source-bound report. Do not fill gaps from memory or issue an investment action.
Append a Markdown table at the end to organize key points.

Today's date is %s. The instrument is %s. %s
%s`, state.TradeDate, state.CompanyOfInterest, state.InstrumentContext, languageInstruction(state))

	payload, _ := tools.PreflightPayload("get_news")
	report, err := client.Generate(ctx, systemPrompt, fmt.Sprintf(
		"Analyze news for %s as of %s using only this preflight-validated payload:\n\n%s",
		state.CompanyOfInterest, state.TradeDate, truncate(payload, 12000)), false)
	if err != nil {
		emitError(onEvent, "News Analyst", err)
		return "", err
	}

	emitComplete(onEvent, "News Analyst", report)
	return report, nil
}

// Helper functions

func languageInstruction(state *AgentState) string {
	return sourceDisciplineInstruction() + `

IMPORTANT: Always write a bilingual report so the UI can show 中英对照 / English-only / Chinese-only:
1. First, write the complete content in English (at the top).
2. Insert a divider line "---" on its own line.
3. Then write the complete Simplified Chinese equivalent below the divider.
Do this even if the user later filters the view. Keep ticker symbols and ratios (PE, PEG) unchanged in both sections.`
}

func sourceDisciplineInstruction() string {
	return `EVIDENCE DISCIPLINE:
- Use only facts present in tool output or supplied analyst reports. Never fill a missing fact using "well-established knowledge", common knowledge, memory, or model priors.
- If a source, value, period, or timestamp is missing or unavailable, say unknown. Do not estimate or silently substitute it.
- There is no verified position or portfolio input in this workflow. Do not claim that the user owns the instrument and do not advise maintaining, reducing, adding, hedging, or closing a position.
- Express implications as conditional observations only, never as an order, stop-loss, option strategy, position size, or executable recommendation.`
}

func positionGuardrails(state *AgentState) string {
	takes := state.XPlayerTakes
	if takes == "" {
		takes = "(no street/X takes loaded)"
	}
	return fmt.Sprintf(`
## Street / X player takes (contrast, not orders)
%s

## Evidence and position guardrails
No verified position or portfolio input was provided. Public posts saying someone else is holding do not establish the user's holdings.
- Use only captured source evidence. Never fill gaps with "well-established knowledge", common knowledge, memory, or model priors.
- Missing facts remain unknown.
- Do not say Hold, maintain the existing position, trim, add, sell, buy, hedge, set a stop, or use options for this user.
- Produce a conditional observation only, with conditions that would justify fresh research. Do not produce an executable recommendation.`, takes)
}

func emitStart(onEvent func(NodeEvent), node string) {
	log.Printf("[AGENT] Starting: %s", node)
	if onEvent != nil {
		onEvent(NodeEvent{
			Type:   "node_start",
			Node:   node,
			Status: "running",
		})
	}
}

func emitComplete(onEvent func(NodeEvent), node, content string) {
	log.Printf("[AGENT] Completed: %s (%d chars)", node, len(content))
	if onEvent != nil {
		onEvent(NodeEvent{
			Type:    "node_complete",
			Node:    node,
			Content: content,
			Status:  "completed",
		})
	}
}

func emitStream(onEvent func(NodeEvent), node, text string) {
	if onEvent != nil {
		onEvent(NodeEvent{
			Type:    "stream",
			Node:    node,
			Content: text,
		})
	}
}

func emitError(onEvent func(NodeEvent), node string, err error) {
	log.Printf("[AGENT] Error in %s: %v", node, err)
	if onEvent != nil {
		onEvent(NodeEvent{
			Type:    "node_error",
			Node:    node,
			Content: err.Error(),
			Status:  "error",
		})
	}
}
