package agents

import (
	"context"
	"fmt"

	"trading-agents/internal/llm"
)

// BullResearcher argues the bullish case for the investment.
func BullResearcher(ctx context.Context, client llm.LLMClient, state *AgentState, onEvent func(NodeEvent)) (string, error) {
	emitStart(onEvent, "Bull Researcher")

	context := buildDebateContext(state)
	bearArgument := state.InvestmentDebateState.BearHistory

	prompt := fmt.Sprintf(`You are a Bull Researcher — your job is to argue WHY this investment is a good idea.

%s

Based on the analyst reports above, present a compelling BULLISH case for %s.
Focus on:
- Strong growth catalysts and competitive advantages
- Positive momentum signals from technical analysis
- Favorable fundamental metrics
- Supportive macro/news environment

If the Bear Researcher has already argued:
---
%s
---
Directly counter their key points with specific evidence from the reports.

Be specific, cite data from the reports, and build a persuasive argument.`, context, state.CompanyOfInterest, bearArgument)

	response, err := client.Generate(ctx,
		"You are a financial researcher arguing the bullish case for an investment. Be persuasive but grounded in evidence.",
		prompt, false)
	if err != nil {
		emitError(onEvent, "Bull Researcher", err)
		return "", err
	}

	emitComplete(onEvent, "Bull Researcher", response)
	return response, nil
}

// BearResearcher argues the bearish case against the investment.
func BearResearcher(ctx context.Context, client llm.LLMClient, state *AgentState, onEvent func(NodeEvent)) (string, error) {
	emitStart(onEvent, "Bear Researcher")

	context := buildDebateContext(state)
	bullArgument := state.InvestmentDebateState.BullHistory

	prompt := fmt.Sprintf(`You are a Bear Researcher — your job is to argue WHY this investment is risky or ill-timed.

%s

Based on the analyst reports above, present a compelling BEARISH case against %s.
Focus on:
- Overvaluation risks and stretched multiples
- Negative technical signals and bearish divergences
- Fundamental weaknesses or deteriorating metrics
- Macro headwinds and geopolitical risks

The Bull Researcher has argued:
---
%s
---
Directly counter their key points with specific evidence from the reports.

Be specific, cite data from the reports, and build a persuasive counter-argument.`, context, state.CompanyOfInterest, bullArgument)

	response, err := client.Generate(ctx,
		"You are a financial researcher arguing the bearish case against an investment. Be critical and thorough.",
		prompt, false)
	if err != nil {
		emitError(onEvent, "Bear Researcher", err)
		return "", err
	}

	emitComplete(onEvent, "Bear Researcher", response)
	return response, nil
}

// ResearchManager evaluates the bull/bear debate and produces the investment plan.
func ResearchManager(ctx context.Context, client llm.LLMClient, state *AgentState, onEvent func(NodeEvent)) (string, error) {
	emitStart(onEvent, "Research Manager")

	prompt := fmt.Sprintf(`You are the Research Manager. Review the following debate and analyst reports, then produce an investment recommendation.

## Analyst Reports
- Market Analysis: %s
- Fundamentals Analysis: %s
- Sentiment Analysis: %s
- News Analysis: %s

## Bull Case
%s

## Bear Case
%s

## Your Task
Evaluate both sides and produce a structured investment plan with:
1. **Recommendation**: Exactly one of Buy / Overweight / Hold / Underweight / Sell
2. **Rationale**: Summary of key arguments from both sides and what tipped the balance
3. **Strategic Actions**: Concrete steps for the trader to implement

Reserve Hold only when evidence is genuinely balanced. Otherwise commit to the stronger side.
%s`,
		truncate(state.MarketReport, 3000),
		truncate(state.FundamentalsReport, 3000),
		truncate(state.SentimentReport, 3000),
		truncate(state.NewsReport, 3000),
		state.InvestmentDebateState.BullHistory,
		state.InvestmentDebateState.BearHistory,
		languageInstruction(state),
	)

	response, err := client.Generate(ctx,
		"You are a senior Research Manager making investment recommendations. Be decisive and evidence-based.",
		prompt, true) // use deep thinking model
	if err != nil {
		emitError(onEvent, "Research Manager", err)
		return "", err
	}

	emitComplete(onEvent, "Research Manager", response)
	return response, nil
}

func buildDebateContext(state *AgentState) string {
	return fmt.Sprintf(`## Analyst Reports for %s (Date: %s)

### Market Analysis (Technical)
%s

### Fundamentals Analysis
%s

### Sentiment Analysis
%s

### News Analysis
%s`,
		state.CompanyOfInterest, state.TradeDate,
		truncate(state.MarketReport, 2000),
		truncate(state.FundamentalsReport, 2000),
		truncate(state.SentimentReport, 2000),
		truncate(state.NewsReport, 2000),
	)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... [truncated]"
}
