package agents

import (
	"context"
	"fmt"

	"trading-agents/internal/llm"
)

// AggressiveAnalyst argues for taking more risk for higher returns.
func AggressiveAnalyst(ctx context.Context, client llm.LLMClient, state *AgentState, onEvent func(NodeEvent)) (string, error) {
	emitStart(onEvent, "Aggressive Analyst")

	prompt := fmt.Sprintf(`You are an Aggressive Risk Analyst. You believe in maximizing returns and are comfortable with higher risk.

## Trader's Proposal
%s

## Previous Discussion
%s

Stress-test the upside scenario without proposing a position:
- Which captured evidence supports the upside case
- Which source-backed conditions would strengthen it
- Which material facts remain unknown

Be specific and reference the analyst reports.
%s`,
		state.TraderInvestmentPlan,
		state.RiskDebateState.History,
		sourceDisciplineInstruction(),
	)

	response, err := client.Generate(ctx,
		"You stress-test the upside case but never invent facts or issue a trade.",
		prompt, false)
	if err != nil {
		emitError(onEvent, "Aggressive Analyst", err)
		return "", err
	}

	emitComplete(onEvent, "Aggressive Analyst", response)
	return response, nil
}

// ConservativeAnalyst argues for caution and risk mitigation.
func ConservativeAnalyst(ctx context.Context, client llm.LLMClient, state *AgentState, onEvent func(NodeEvent)) (string, error) {
	emitStart(onEvent, "Conservative Analyst")

	prompt := fmt.Sprintf(`You are a Conservative Risk Analyst. You prioritize capital preservation and downside protection.

## Trader's Proposal
%s

## Previous Discussion
%s

Stress-test the downside scenario without proposing a position:
- Key source-backed downside risks
- Which captured evidence would invalidate the downside case
- Which material facts remain unknown

Be specific and reference the analyst reports.
%s`,
		state.TraderInvestmentPlan,
		state.RiskDebateState.History,
		sourceDisciplineInstruction(),
	)

	response, err := client.Generate(ctx,
		"You are a conservative risk analyst who prioritizes capital preservation above all else.",
		prompt, false)
	if err != nil {
		emitError(onEvent, "Conservative Analyst", err)
		return "", err
	}

	emitComplete(onEvent, "Conservative Analyst", response)
	return response, nil
}

// NeutralAnalyst provides a balanced perspective on risk.
func NeutralAnalyst(ctx context.Context, client llm.LLMClient, state *AgentState, onEvent func(NodeEvent)) (string, error) {
	emitStart(onEvent, "Neutral Analyst")

	prompt := fmt.Sprintf(`You are a Neutral Risk Analyst. You seek balanced, well-calibrated risk management.

## Trader's Proposal
%s

## Aggressive Analyst's View
%s

## Conservative Analyst's View
%s

Provide a balanced synthesis:
- Where each side has valid points
- A conditional synthesis for further observation
- The evidence gaps that prevent execution
- Conditions that would justify refreshing the research

Be specific and evidence-based.
%s`,
		state.TraderInvestmentPlan,
		state.RiskDebateState.AggressiveHistory,
		state.RiskDebateState.ConservativeHistory,
		sourceDisciplineInstruction(),
	)

	response, err := client.Generate(ctx,
		"You are a neutral, source-bound risk analyst. Synthesize evidence gaps and monitoring conditions without investment actions.",
		prompt, false)
	if err != nil {
		emitError(onEvent, "Neutral Analyst", err)
		return "", err
	}

	emitComplete(onEvent, "Neutral Analyst", response)
	return response, nil
}

// PortfolioManager makes the final investment decision.
func PortfolioManager(ctx context.Context, client llm.LLMClient, state *AgentState, onEvent func(NodeEvent)) (string, error) {
	emitStart(onEvent, "Portfolio Manager")

	pastContext := ""
	if state.PastContext != "" {
		pastContext = fmt.Sprintf("\n## Past Lessons & Context\n%s\n", state.PastContext)
	}

	prompt := fmt.Sprintf(`You are the Portfolio Manager making the FINAL investment decision for %s.

## Analyst Reports Summary
- Market (Technical): %s
- Fundamentals: %s
- Sentiment: %s
- News: %s

## Research Team Decision
%s

## Trader's Proposal
%s

## Risk Management Discussion
- Aggressive View: %s
- Conservative View: %s
- Neutral View: %s
%s

## Final Research Output
There is no verified position input. Produce only:
1. **Status**: OBSERVE
2. **Evidence Summary**: What the captured sources establish
3. **Unknowns**: Missing facts that prevent an investment action
4. **Monitoring Conditions**: Conditions for refreshing the research
5. **对照 X / 股票玩家**: Public-player observations, explicitly not the user's holdings
%s
%s`,
		state.CompanyOfInterest,
		truncate(state.MarketReport, 1000),
		truncate(state.FundamentalsReport, 1000),
		truncate(state.SentimentReport, 1000),
		truncate(state.NewsReport, 1000),
		state.InvestmentPlan,
		state.TraderInvestmentPlan,
		truncate(state.RiskDebateState.AggressiveHistory, 1000),
		truncate(state.RiskDebateState.ConservativeHistory, 1000),
		truncate(state.RiskDebateState.NeutralHistory, 1000),
		pastContext,
		positionGuardrails(state),
		languageInstruction(state),
	)

	response, err := client.Generate(ctx,
		"You are a source-bound research reviewer. Output OBSERVE and conditional monitoring only; never issue a trade, stop, option, target, or position instruction.",
		prompt, true) // use deep thinking model
	if err != nil {
		emitError(onEvent, "Portfolio Manager", err)
		return "", err
	}

	emitComplete(onEvent, "Portfolio Manager", response)
	return response, nil
}
