package agents

import (
	"context"
	"fmt"

	"trading-agents/internal/llm"
)

// Trader reads the research plan and analyst reports, then produces a transaction proposal.
func Trader(ctx context.Context, client llm.LLMClient, state *AgentState, onEvent func(NodeEvent)) (string, error) {
	emitStart(onEvent, "Trader")

	prompt := fmt.Sprintf(`You are a market-conditions reviewer. Read the Research Manager's synthesis and analyst reports.

## Research Manager's Investment Plan
%s

## Analyst Reports Summary
- Market (Technical): %s
- Fundamentals: %s

## Your Task
Produce only a conditional observation with source-backed conditions to monitor. No verified position input exists: do not output Buy, Hold, Sell, entry price, stop loss, options, sizing, or transaction instructions.

End with: RESEARCH ACTION: **OBSERVE**
%s
%s`,
		state.InvestmentPlan,
		truncate(state.MarketReport, 1500),
		truncate(state.FundamentalsReport, 1500),
		positionGuardrails(state),
		languageInstruction(state),
	)

	response, err := client.Generate(ctx,
		"You are a source-bound market observer. Return conditions for further research, never an executable trade.",
		prompt, false)
	if err != nil {
		emitError(onEvent, "Trader", err)
		return "", err
	}

	emitComplete(onEvent, "Trader", response)
	return response, nil
}
