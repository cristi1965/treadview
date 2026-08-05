package orchestrator

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"trading-agents/internal/agents"
	"trading-agents/internal/config"
	"trading-agents/internal/dataflows"
	"trading-agents/internal/llm"
)

// Orchestrator manages the execution of the multi-agent trading analysis pipeline.
// It replaces LangGraph's StateGraph with explicit Go control flow.
type Orchestrator struct {
	config   *config.Config
	client   llm.LLMClient
	onEvent  func(agents.NodeEvent) // callback for real-time UI updates
	cancelFn context.CancelFunc     // to stop a running analysis
}

// New creates an Orchestrator.
func New(cfg *config.Config, client llm.LLMClient) *Orchestrator {
	return &Orchestrator{
		config: cfg,
		client: client,
	}
}

// SetEventHandler sets the callback for real-time node events.
func (o *Orchestrator) SetEventHandler(handler func(agents.NodeEvent)) {
	o.onEvent = handler
}

// RunAnalysis executes the full trading agent pipeline.
// It returns the final state and decision, or an error.
func (o *Orchestrator) RunAnalysis(ctx context.Context, req agents.AnalysisRequest) (*agents.AnalysisResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	o.cancelFn = cancel
	defer cancel()

	startTime := time.Now()
	ticker := req.Ticker
	tradeDate := req.TradeDate
	assetType := req.AssetType
	if assetType == "" {
		assetType = "stock"
	}

	log.Printf("[ORCH] Starting analysis: %s on %s (type: %s)", ticker, tradeDate, assetType)
	o.emit(agents.NodeEvent{Type: "analysis_start", Node: "System", Content: fmt.Sprintf("Starting analysis for %s", ticker)})

	// Create tool registry for data fetching
	tools := dataflows.NewToolRegistry(ticker, tradeDate, o.config.FREDAPIKey)

	// Initialize state
	state := &agents.AgentState{
		CompanyOfInterest: ticker,
		AssetType:         assetType,
		TradeDate:         tradeDate,
		InstrumentContext: fmt.Sprintf("Instrument: %s (Type: %s)", ticker, assetType),
		OutputLanguage:    o.config.OutputLanguage,
	}

	// ========== Phase 1: Analyst Team ==========
	o.emitProgress("Phase 1: Analyst Team", 0)

	// 1. Market Analyst (Technical)
	report, err := agents.MarketAnalyst(ctx, o.client, state, tools, o.onEvent)
	if err != nil {
		return nil, fmt.Errorf("market analyst: %w", err)
	}
	state.MarketReport = report
	o.emitProgress("Phase 1: Analyst Team", 25)

	// 2. Fundamentals Analyst
	report, err = agents.FundamentalsAnalyst(ctx, o.client, state, tools, o.onEvent)
	if err != nil {
		return nil, fmt.Errorf("fundamentals analyst: %w", err)
	}
	state.FundamentalsReport = report
	o.emitProgress("Phase 1: Analyst Team", 50)

	// 3. Sentiment Analyst
	report, err = agents.SentimentAnalyst(ctx, o.client, state, tools, o.onEvent)
	if err != nil {
		return nil, fmt.Errorf("sentiment analyst: %w", err)
	}
	state.SentimentReport = report
	o.emitProgress("Phase 1: Analyst Team", 75)

	// 4. News Analyst
	report, err = agents.NewsAnalyst(ctx, o.client, state, tools, o.onEvent)
	if err != nil {
		return nil, fmt.Errorf("news analyst: %w", err)
	}
	state.NewsReport = report
	o.emitProgress("Phase 1: Analyst Team", 100)

	// ========== Phase 2: Research Team (Bull/Bear Debate) ==========
	o.emitProgress("Phase 2: Research Debate", 0)

	maxDebateRounds := o.config.MaxDebateRounds
	for round := 0; round < maxDebateRounds; round++ {
		// Bull Researcher
		bullResponse, err := agents.BullResearcher(ctx, o.client, state, o.onEvent)
		if err != nil {
			return nil, fmt.Errorf("bull researcher: %w", err)
		}
		state.InvestmentDebateState.BullHistory = bullResponse
		state.InvestmentDebateState.CurrentResponse = "Bull: " + bullResponse
		state.InvestmentDebateState.History += fmt.Sprintf("\n\n--- Bull Round %d ---\n%s", round+1, bullResponse)
		state.InvestmentDebateState.Count++

		// Bear Researcher
		bearResponse, err := agents.BearResearcher(ctx, o.client, state, o.onEvent)
		if err != nil {
			return nil, fmt.Errorf("bear researcher: %w", err)
		}
		state.InvestmentDebateState.BearHistory = bearResponse
		state.InvestmentDebateState.CurrentResponse = "Bear: " + bearResponse
		state.InvestmentDebateState.History += fmt.Sprintf("\n\n--- Bear Round %d ---\n%s", round+1, bearResponse)
		state.InvestmentDebateState.Count++
	}
	o.emitProgress("Phase 2: Research Debate", 60)

	// Research Manager
	investmentPlan, err := agents.ResearchManager(ctx, o.client, state, o.onEvent)
	if err != nil {
		return nil, fmt.Errorf("research manager: %w", err)
	}
	state.InvestmentPlan = investmentPlan
	state.InvestmentDebateState.JudgeDecision = investmentPlan
	o.emitProgress("Phase 2: Research Debate", 100)

	// ========== Phase 3: Trading ==========
	o.emitProgress("Phase 3: Trading", 0)

	traderPlan, err := agents.Trader(ctx, o.client, state, o.onEvent)
	if err != nil {
		return nil, fmt.Errorf("trader: %w", err)
	}
	state.TraderInvestmentPlan = traderPlan
	o.emitProgress("Phase 3: Trading", 100)

	// ========== Phase 4: Risk Management Debate ==========
	o.emitProgress("Phase 4: Risk Management", 0)

	maxRiskRounds := o.config.MaxRiskDiscussRounds
	for round := 0; round < maxRiskRounds; round++ {
		// Aggressive Analyst
		aggResponse, err := agents.AggressiveAnalyst(ctx, o.client, state, o.onEvent)
		if err != nil {
			return nil, fmt.Errorf("aggressive analyst: %w", err)
		}
		state.RiskDebateState.AggressiveHistory = aggResponse
		state.RiskDebateState.LatestSpeaker = "Aggressive"
		state.RiskDebateState.History += fmt.Sprintf("\n\n--- Aggressive Round %d ---\n%s", round+1, aggResponse)
		state.RiskDebateState.Count++

		// Conservative Analyst
		conResponse, err := agents.ConservativeAnalyst(ctx, o.client, state, o.onEvent)
		if err != nil {
			return nil, fmt.Errorf("conservative analyst: %w", err)
		}
		state.RiskDebateState.ConservativeHistory = conResponse
		state.RiskDebateState.LatestSpeaker = "Conservative"
		state.RiskDebateState.History += fmt.Sprintf("\n\n--- Conservative Round %d ---\n%s", round+1, conResponse)
		state.RiskDebateState.Count++

		// Neutral Analyst
		neuResponse, err := agents.NeutralAnalyst(ctx, o.client, state, o.onEvent)
		if err != nil {
			return nil, fmt.Errorf("neutral analyst: %w", err)
		}
		state.RiskDebateState.NeutralHistory = neuResponse
		state.RiskDebateState.LatestSpeaker = "Neutral"
		state.RiskDebateState.History += fmt.Sprintf("\n\n--- Neutral Round %d ---\n%s", round+1, neuResponse)
		state.RiskDebateState.Count++
	}
	o.emitProgress("Phase 4: Risk Management", 60)

	// ========== Phase 5: Portfolio Manager (Final Decision) ==========
	o.emitProgress("Phase 5: Final Decision", 0)

	finalDecision, err := agents.PortfolioManager(ctx, o.client, state, o.onEvent)
	if err != nil {
		return nil, fmt.Errorf("portfolio manager: %w", err)
	}
	state.FinalTradeDecision = finalDecision
	state.RiskDebateState.JudgeDecision = finalDecision
	o.emitProgress("Phase 5: Final Decision", 100)

	// Extract the core decision (BUY/HOLD/SELL)
	decision := extractDecision(finalDecision)

	duration := time.Since(startTime).Seconds()
	log.Printf("[ORCH] Analysis complete: %s → %s (%.1fs)", ticker, decision, duration)

	result := &agents.AnalysisResult{
		Ticker:       ticker,
		TradeDate:    tradeDate,
		Decision:     decision,
		State:        *state,
		CompletedAt:  time.Now().Format(time.RFC3339),
		DurationSecs: duration,
	}

	o.emit(agents.NodeEvent{
		Type:    "analysis_complete",
		Node:    "System",
		Content: fmt.Sprintf("Analysis complete: %s → %s", ticker, decision),
		Status:  "completed",
	})

	return result, nil
}

// Stop cancels a running analysis.
func (o *Orchestrator) Stop() {
	if o.cancelFn != nil {
		o.cancelFn()
		log.Println("[ORCH] Analysis cancelled")
	}
}

func (o *Orchestrator) emit(event agents.NodeEvent) {
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}
	if o.onEvent != nil {
		o.onEvent(event)
	}
}

func (o *Orchestrator) emitProgress(phase string, progress int) {
	o.emit(agents.NodeEvent{
		Type:     "progress",
		Node:     phase,
		Progress: progress,
	})
}

// extractDecision pulls the BUY/HOLD/SELL signal from the Portfolio Manager's text.
func extractDecision(text string) string {
	upper := strings.ToUpper(text)

	// Look for common patterns
	patterns := []struct {
		search string
		result string
	}{
		{"RATING**: BUY", "BUY"},
		{"RATING**: OVERWEIGHT", "BUY"},
		{"RATING**: SELL", "SELL"},
		{"RATING**: UNDERWEIGHT", "SELL"},
		{"RATING**: HOLD", "HOLD"},
		{"RATING: BUY", "BUY"},
		{"RATING: OVERWEIGHT", "BUY"},
		{"RATING: SELL", "SELL"},
		{"RATING: UNDERWEIGHT", "SELL"},
		{"RATING: HOLD", "HOLD"},
		{"FINAL TRANSACTION PROPOSAL: **BUY**", "BUY"},
		{"FINAL TRANSACTION PROPOSAL: **SELL**", "SELL"},
		{"FINAL TRANSACTION PROPOSAL: **HOLD**", "HOLD"},
	}

	for _, p := range patterns {
		if strings.Contains(upper, strings.ToUpper(p.search)) {
			return p.result
		}
	}

	// Fallback: count keywords
	buyCount := strings.Count(upper, "BUY") + strings.Count(upper, "OVERWEIGHT")
	sellCount := strings.Count(upper, "SELL") + strings.Count(upper, "UNDERWEIGHT")
	holdCount := strings.Count(upper, "HOLD")

	if buyCount > sellCount && buyCount > holdCount {
		return "BUY"
	}
	if sellCount > buyCount && sellCount > holdCount {
		return "SELL"
	}
	return "HOLD"
}
