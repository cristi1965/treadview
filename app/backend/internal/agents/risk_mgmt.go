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

Argue for a MORE aggressive position:
- Why the risk/reward is favorable
- Why a larger position size is appropriate
- Why the stop-loss can be wider or unnecessary
- Historical precedents where bold positions paid off

Be specific and reference the analyst reports.`,
		state.TraderInvestmentPlan,
		state.RiskDebateState.History,
	)

	response, err := client.Generate(ctx,
		"You are an aggressive risk analyst who favors bold positions with high conviction trades.",
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

Argue for a MORE conservative approach:
- Key downside risks that haven't been adequately addressed
- Why position sizing should be smaller
- Why tighter stop-losses are needed
- Hedging strategies to consider

Be specific and reference the analyst reports.`,
		state.TraderInvestmentPlan,
		state.RiskDebateState.History,
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
- A practical middle-ground recommendation
- Specific risk parameters (position size, stop-loss, take-profit)
- How to scale in/out based on market conditions

Be specific and evidence-based.`,
		state.TraderInvestmentPlan,
		state.RiskDebateState.AggressiveHistory,
		state.RiskDebateState.ConservativeHistory,
	)

	response, err := client.Generate(ctx,
		"You are a neutral risk analyst who synthesizes aggressive and conservative views into practical recommendations.",
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

## Your Final Decision
Produce the FINAL portfolio decision with:
1. **Rating**: Exactly one of Buy / Overweight / Hold / Underweight / Sell
2. **Executive Summary**: Concise action plan (entry, sizing, risk levels, time horizon) — 2-4 sentences
3. **Investment Thesis**: Detailed reasoning anchored in the evidence above
4. **Price Target**: Optional target price
5. **Time Horizon**: Optional holding period recommendation
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
		languageInstruction(state),
	)

	response, err := client.Generate(ctx,
		"You are a senior Portfolio Manager making the final investment decision. Be authoritative and decisive.",
		prompt, true) // use deep thinking model
	if err != nil {
		emitError(onEvent, "Portfolio Manager", err)
		return "", err
	}

	emitComplete(onEvent, "Portfolio Manager", response)
	return response, nil
}
