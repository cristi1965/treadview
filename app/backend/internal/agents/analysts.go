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

CRITICAL: Always call get_realtime_quote FIRST to get the current real-time price via TradingView WebSocket.
This ensures your analysis uses the actual current market price, not stale historical data.
Then call get_stock_data to retrieve price data, then use get_indicators with specific indicator names.
Before writing the final report, call get_verified_market_snapshot and treat it as the source of truth.

Write a very detailed report with specific, actionable insights. Append a Markdown table at the end.
%s`, state.TradeDate, state.CompanyOfInterest, state.InstrumentContext, languageInstruction(state))

	toolDefs := dataflows.MarketToolDefs()

	report, err := client.GenerateWithTools(ctx, systemPrompt,
		[]llm.ChatMessage{{Role: "user", Content: fmt.Sprintf(
			"Proceed with your assigned analysis for this workflow. The instrument to analyze is %s and the trade date is %s.",
			state.CompanyOfInterest, state.TradeDate)}},
		toolDefs, false, tools.Execute,
		func(text string) { emitStream(onEvent, "Market Analyst", text) },
	)
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
Include as much detail as possible. Provide specific, actionable insights with supporting evidence.
Append a Markdown table at the end to organize key points.

Use the available tools: get_fundamentals for comprehensive analysis, get_balance_sheet, get_cashflow, and get_income_statement for specific financial statements.

Today's date is %s. The instrument is %s. %s
%s`, state.TradeDate, state.CompanyOfInterest, state.InstrumentContext, languageInstruction(state))

	toolDefs := dataflows.FundamentalsToolDefs()

	report, err := client.GenerateWithTools(ctx, systemPrompt,
		[]llm.ChatMessage{{Role: "user", Content: fmt.Sprintf(
			"Proceed with your assigned analysis. Analyze %s fundamentals as of %s.",
			state.CompanyOfInterest, state.TradeDate)}},
		toolDefs, false, tools.Execute,
		func(text string) { emitStream(onEvent, "Fundamentals Analyst", text) },
	)
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

	systemPrompt := fmt.Sprintf(`You are a researcher tasked with analyzing social media sentiment and public opinion about a company.
Analyze available news data for sentiment signals. Provide a comprehensive sentiment report covering:
1. Source-by-source breakdown with specific evidence
2. Cross-source divergences and alignments
3. Dominant narrative themes
4. Catalysts and risks surfaced by the data
5. A markdown table summarizing key sentiment signals

Rate overall sentiment as: Bullish / Mildly Bullish / Neutral / Mixed / Mildly Bearish / Bearish
Provide an overall score on a 0-10 scale (0=maximally bearish, 5=neutral, 10=maximally bullish).

Today's date is %s. The instrument is %s. %s
%s`, state.TradeDate, state.CompanyOfInterest, state.InstrumentContext, languageInstruction(state))

	toolDefs := dataflows.SentimentToolDefs()

	report, err := client.GenerateWithTools(ctx, systemPrompt,
		[]llm.ChatMessage{{Role: "user", Content: fmt.Sprintf(
			"Proceed with your assigned sentiment analysis. Analyze sentiment for %s as of %s.",
			state.CompanyOfInterest, state.TradeDate)}},
		toolDefs, false, tools.Execute,
		func(text string) { emitStream(onEvent, "Sentiment Analyst", text) },
	)
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
Research and analyze: ticker-specific news, global macro news, insider transactions, macroeconomic indicators, and prediction market data.

Write a comprehensive report covering all these areas with specific, actionable insights.
Append a Markdown table at the end to organize key points.

Today's date is %s. The instrument is %s. %s
%s`, state.TradeDate, state.CompanyOfInterest, state.InstrumentContext, languageInstruction(state))

	toolDefs := dataflows.NewsToolDefs()

	report, err := client.GenerateWithTools(ctx, systemPrompt,
		[]llm.ChatMessage{{Role: "user", Content: fmt.Sprintf(
			"Proceed with your assigned news analysis. Analyze news for %s as of %s.",
			state.CompanyOfInterest, state.TradeDate)}},
		toolDefs, false, tools.Execute,
		func(text string) { emitStream(onEvent, "News Analyst", text) },
	)
	if err != nil {
		emitError(onEvent, "News Analyst", err)
		return "", err
	}

	emitComplete(onEvent, "News Analyst", report)
	return report, nil
}

// Helper functions

func languageInstruction(state *AgentState) string {
	return `
IMPORTANT: You MUST write your final analysis report, markdown table, summaries, and outputs in a strictly bilingual format:
1. First, write the complete content in English (at the top).
2. Insert a divider line "---" (on a new line).
3. Then, write the complete translation/equivalent of the content in Chinese (简体中文) below the divider.
Remember: English must always be on top, and Chinese must always be on the bottom. Keep technical ticker symbols (like NVDA) and common financial ratios (like PE, PEG) in their standard format in both sections.`
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
