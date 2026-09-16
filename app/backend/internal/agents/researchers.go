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

Be specific and cite data from the reports. Unknown facts must remain unknown.
%s`, context, state.CompanyOfInterest, bearArgument, sourceDisciplineInstruction())

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

Be specific and cite data from the reports. Unknown facts must remain unknown.
%s`, context, state.CompanyOfInterest, bullArgument, sourceDisciplineInstruction())

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

	prompt := fmt.Sprintf(`You are the Research Manager. Review the following debate and analyst reports, then produce a source-bound research synthesis.

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
Evaluate both sides and produce a structured research synthesis with:
1. **Research Status**: Supported or Insufficient Evidence
2. **Rationale**: Source-bound arguments from both sides
3. **Monitoring Conditions**: New evidence that would change the assessment

No verified holding input exists. Do not output Buy, Sell, Hold, Overweight, Underweight, position changes, stops, options, or executable actions. End with a conditional observation only.
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
		"You are a source-bound Research Manager. Unknown facts remain unknown; output conditional observations, never investment actions.",
		prompt, true) // use deep thinking model
	if err != nil {
		emitError(onEvent, "Research Manager", err)
		return "", err
	}
	if !ConditionalObservationSafe(response) {
		response, err = client.Generate(ctx,
			"Rewrite this as descriptive research only. Preserve evidence and uncertainty. Do not tell the reader what to do, and do not repeat or explain this restriction.",
			fmt.Sprintf(`Rewrite the synthesis below using only these headings:
1. Research Status
2. Evidence Rationale
3. Monitoring Conditions

Describe favorable evidence as supportive and adverse evidence as cautionary. Use purely descriptive language. End with a conditional observation only.

Synthesis to rewrite:
%s
%s`, response, languageInstruction(state)),
			false)
		if err != nil {
			emitError(onEvent, "Research Manager", err)
			return "", err
		}
	}
	if !ConditionalObservationSafe(response) {
		response = `1. Research Status: Evidence review complete; the detailed synthesis was withheld because its wording did not satisfy the non-executable publication contract.
2. Evidence Rationale: Dated market history, filing-bound fundamentals, reviewed news, and opposing research artifacts remain available in the audit record.
3. Monitoring Conditions: Refresh this observation when a source period changes or new source evidence becomes available.
Conditional observation only.`
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
