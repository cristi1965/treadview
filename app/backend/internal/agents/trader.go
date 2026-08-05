package agents

import (
	"context"
	"fmt"

	"trading-agents/internal/llm"
)

// Trader reads the research plan and analyst reports, then produces a transaction proposal.
func Trader(ctx context.Context, client llm.LLMClient, state *AgentState, onEvent func(NodeEvent)) (string, error) {
	emitStart(onEvent, "Trader")

	prompt := fmt.Sprintf(`You are a Trader. Read the Research Manager's investment plan and the analyst reports, 
then produce a concrete transaction proposal.

## Research Manager's Investment Plan
%s

## Analyst Reports Summary
- Market (Technical): %s
- Fundamentals: %s

## Your Task
Produce a transaction proposal with:
1. **Action**: Exactly one of Buy / Hold / Sell
2. **Reasoning**: 2-4 sentences anchored in the analysts' reports and research plan
3. **Entry Price**: Optional target entry price
4. **Stop Loss**: Optional stop-loss level
5. **Position Sizing**: Optional sizing guidance (e.g., "5%% of portfolio")

End with: FINAL TRANSACTION PROPOSAL: **BUY/HOLD/SELL**
%s`,
		state.InvestmentPlan,
		truncate(state.MarketReport, 1500),
		truncate(state.FundamentalsReport, 1500),
		languageInstruction(state),
	)

	response, err := client.Generate(ctx,
		"You are a professional trader translating investment recommendations into concrete transactions. Be precise and practical.",
		prompt, false)
	if err != nil {
		emitError(onEvent, "Trader", err)
		return "", err
	}

	emitComplete(onEvent, "Trader", response)
	return response, nil
}
