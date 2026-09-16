package orchestrator

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
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
	mu       sync.RWMutex
	onEvent  func(agents.NodeEvent) // callback for real-time UI updates
	cancelFn context.CancelFunc     // to stop a running analysis
	runID    uint64
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
	o.mu.Lock()
	defer o.mu.Unlock()
	o.onEvent = handler
}

func (o *Orchestrator) eventHandler() func(agents.NodeEvent) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.onEvent
}

// GetLLMClient exposes the underlying LLM client for ad-hoc chat and interactive Q&A.
func (o *Orchestrator) GetLLMClient() llm.LLMClient {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.client
}

// SetLLMClient swaps the runtime LLM client after config changes.
func (o *Orchestrator) SetLLMClient(client llm.LLMClient) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.client = client
}

// RunAnalysis executes the full trading agent pipeline.
// It returns the final state and decision, or an error.
func (o *Orchestrator) RunAnalysis(ctx context.Context, req agents.AnalysisRequest) (*agents.AnalysisResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	routeTrace := llm.NewRouteTrace()
	ctx = llm.WithRouteTrace(ctx, routeTrace)
	o.mu.Lock()
	o.runID++
	runID := o.runID
	o.cancelFn = cancel
	o.mu.Unlock()
	defer func() {
		cancel()
		o.mu.Lock()
		if o.runID == runID {
			o.cancelFn = nil
		}
		o.mu.Unlock()
	}()

	client := o.GetLLMClient()
	if client == nil {
		return nil, fmt.Errorf("LLM client not configured or unavailable")
	}
	onEvent := o.eventHandler()
	bufferedEvents := make([]agents.NodeEvent, 0)
	bufferEvent := func(event agents.NodeEvent) {
		bufferedEvents = append(bufferedEvents, event)
	}

	startTime := time.Now()
	ticker := req.Ticker
	tradeDate := req.TradeDate
	assetType := req.AssetType
	if assetType == "" {
		assetType = "stock"
	}
	req.AssetType = assetType
	provider, quickModel, deepModel := auditModels(o.config)
	audit := agents.NewAnalysisAudit(req, o.config.OutputLanguage, provider, quickModel, deepModel, o.config.MaxDebateRounds, o.config.MaxRiskDiscussRounds, startTime)
	toolEvidenceCursor := 0

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

	preflight := tools.RunEvidenceOnlyPreflight()
	preflightEvidence := captureToolEvidence(&audit, tools, &toolEvidenceCursor, o.config.ResultsDir)
	marketEvidenceID := preflightEvidence["get_historical_evidence"]
	fundamentalsEvidenceID := preflightEvidence["get_fundamentals"]
	newsEvidenceID := preflightEvidence["get_news"]
	if !preflight.Passed || marketEvidenceID == "" || fundamentalsEvidenceID == "" || newsEvidenceID == "" {
		result := unavailableResult(ticker, tradeDate, startTime, state, audit)
		reason := strings.Join(preflight.Reasons, "; ")
		if marketEvidenceID == "" || fundamentalsEvidenceID == "" || newsEvidenceID == "" {
			reason = strings.Trim(strings.Join([]string{reason, "required preflight evidence is not persistable and reviewable"}, "; "), "; ")
		}
		agents.MarkResearchUnavailable(result, reason)
		o.emitResearchUnavailable(result)
		return result, nil
	}
	marketSourceEvidence := []string{marketEvidenceID}
	fundamentalsSourceEvidence := []string{fundamentalsEvidenceID}
	newsSourceEvidence := []string{newsEvidenceID}

	// ========== Phase 1: Analyst Team ==========
	o.emitProgress("Phase 1: Analyst Team", 0)

	// 1. Market Analyst (Technical)
	report, err := agents.MarketAnalyst(ctx, client, state, tools, bufferEvent)
	if err != nil {
		return nil, fmt.Errorf("market analyst: %w", err)
	}
	state.MarketReport = report
	marketArtifact := audit.RecordStage("market report", report, quickModel, marketSourceEvidence, time.Now())
	o.emitProgress("Phase 1: Analyst Team", 25)

	// 2. Fundamentals Analyst
	report, err = agents.FundamentalsAnalyst(ctx, client, state, tools, bufferEvent)
	if err != nil {
		return nil, fmt.Errorf("fundamentals analyst: %w", err)
	}
	state.FundamentalsReport = report
	fundamentalsArtifact := audit.RecordStage("fundamentals report", report, quickModel, fundamentalsSourceEvidence, time.Now())
	o.emitProgress("Phase 1: Analyst Team", 50)

	// 3. Sentiment Analyst
	report, err = agents.SentimentAnalyst(ctx, client, state, tools, bufferEvent)
	if err != nil {
		return nil, fmt.Errorf("sentiment analyst: %w", err)
	}
	state.SentimentReport = report
	sentimentArtifact := audit.RecordStage("sentiment report", report, quickModel, newsSourceEvidence, time.Now())
	o.emitProgress("Phase 1: Analyst Team", 75)

	// 4. News Analyst
	report, err = agents.NewsAnalyst(ctx, client, state, tools, bufferEvent)
	if err != nil {
		return nil, fmt.Errorf("news analyst: %w", err)
	}
	state.NewsReport = report
	newsArtifact := audit.RecordStage("news report", report, quickModel, newsSourceEvidence, time.Now())
	o.emitProgress("Phase 1: Analyst Team", 100)
	if marketArtifact == "" || fundamentalsArtifact == "" || sentimentArtifact == "" || newsArtifact == "" {
		result := unavailableResult(ticker, tradeDate, startTime, state, audit)
		agents.MarkResearchUnavailable(result, "one or more required analyst artifacts are missing")
		o.emitResearchUnavailable(result)
		return result, nil
	}
	if health := agents.AssessAudit(audit); !health.Publishable {
		result := unavailableResult(ticker, tradeDate, startTime, state, audit)
		o.emitResearchUnavailable(result)
		return result, nil
	}

	// ========== Phase 2: Research Team (Bull/Bear Debate) ==========
	o.emitProgress("Phase 2: Research Debate", 0)

	maxDebateRounds := o.config.MaxDebateRounds
	for round := 0; round < maxDebateRounds; round++ {
		// Bull Researcher
		bullResponse, err := agents.BullResearcher(ctx, client, state, bufferEvent)
		if err != nil {
			return nil, fmt.Errorf("bull researcher: %w", err)
		}
		state.InvestmentDebateState.BullHistory = bullResponse
		state.InvestmentDebateState.CurrentResponse = "Bull: " + bullResponse
		state.InvestmentDebateState.History += fmt.Sprintf("\n\n--- Bull Round %d ---\n%s", round+1, bullResponse)
		state.InvestmentDebateState.Count++

		// Bear Researcher
		bearResponse, err := agents.BearResearcher(ctx, client, state, bufferEvent)
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
	investmentPlan, err := agents.ResearchManager(ctx, client, state, bufferEvent)
	if err != nil {
		return nil, fmt.Errorf("research manager: %w", err)
	}
	state.InvestmentPlan = investmentPlan
	debateArtifact := audit.RecordStage("research debate", state.InvestmentDebateState.History, quickModel, []string{marketArtifact, fundamentalsArtifact, sentimentArtifact, newsArtifact}, time.Now())
	researchArtifact := audit.RecordStage("research decision", investmentPlan, stageModel(deepModel, routeTrace), []string{debateArtifact}, time.Now())
	state.InvestmentDebateState.JudgeDecision = investmentPlan
	o.emitProgress("Phase 2: Research Debate", 100)

	if !agents.ConditionalObservationSafe(investmentPlan) {
		result := unavailableResult(ticker, tradeDate, startTime, state, audit)
		agents.MarkResearchUnavailable(result, "research synthesis contains position or executable-action language without verified holdings input")
		o.emitResearchUnavailable(result)
		return result, nil
	}

	// The API has no verified holdings input. Stop before trader/risk/portfolio action generation.
	finalDecision := "Conditional observation only: source-backed research is available for review. Refresh the research when an upstream source period changes; no investment action is published without verified holdings input."
	state.FinalTradeDecision = finalDecision
	audit.RecordStage("portfolio decision", finalDecision, "deterministic-observation-v1", []string{researchArtifact}, time.Now())
	state.RiskDebateState.JudgeDecision = finalDecision
	o.emitProgress("Final Conditional Observation", 100)

	// Extract the core decision (BUY/HOLD/SELL)
	decision := "OBSERVE"

	duration := time.Since(startTime).Seconds()
	log.Printf("[ORCH] Analysis complete: %s → %s (%.1fs)", ticker, decision, duration)

	result := &agents.AnalysisResult{
		Ticker:       ticker,
		TradeDate:    tradeDate,
		Decision:     decision,
		State:        *state,
		Audit:        audit,
		CompletedAt:  time.Now().Format(time.RFC3339),
		DurationSecs: duration,
	}
	health := agents.ApplyPublicationGate(result)
	if !health.Publishable {
		o.emitResearchUnavailable(result)
		return result, nil
	}
	if onEvent != nil {
		for _, event := range bufferedEvents {
			onEvent(event)
		}
	}

	o.emit(agents.NodeEvent{
		Type:    "analysis_complete",
		Node:    "System",
		Content: fmt.Sprintf("Analysis complete: %s → %s", ticker, decision),
		Status:  "completed",
	})

	return result, nil
}

func stageModel(configured string, trace *llm.RouteTrace) string {
	if event, ok := trace.LastSuccessful(); ok && strings.TrimSpace(event.Provider) != "" {
		return event.Provider
	}
	return configured
}

func auditModels(cfg *config.Config) (provider, quick, deep string) {
	if cfg.LLMProvider == llm.ProviderDual {
		return "deepseek+gemini", "deepseek/" + cfg.QuickThinkLLM, "gemini/" + cfg.DeepThinkLLM
	}
	return cfg.LLMProvider, cfg.QuickThinkLLM, cfg.DeepThinkLLM
}

func (o *Orchestrator) emitResearchUnavailable(result *agents.AnalysisResult) {
	reasons := strings.Join(result.ResearchHealth.Reasons, "; ")
	o.emit(agents.NodeEvent{
		Type: "analysis_unavailable", Node: "System", Status: "degraded",
		Content: "research_unavailable: " + reasons,
	})
}

func unavailableResult(ticker, tradeDate string, started time.Time, state *agents.AgentState, audit agents.AnalysisAudit) *agents.AnalysisResult {
	result := &agents.AnalysisResult{
		Ticker: ticker, TradeDate: tradeDate, State: *state, Audit: audit,
		CompletedAt: time.Now().Format(time.RFC3339), DurationSecs: time.Since(started).Seconds(),
	}
	agents.ApplyPublicationGate(result)
	return result
}

func captureToolEvidence(audit *agents.AnalysisAudit, tools *dataflows.ToolRegistry, cursor *int, resultsDir string) map[string]string {
	invocations := tools.EvidenceSnapshot()
	if *cursor >= len(invocations) {
		return nil
	}
	ids := make(map[string]string, len(invocations)-*cursor)
	for index, invocation := range invocations[*cursor:] {
		id := fmt.Sprintf("tool-%02d-%s", *cursor+index+1, strings.ReplaceAll(invocation.Name, "_", "-"))
		payloadRef, payloadSize, artifactErr := dataflows.PersistPayloadArtifact(resultsDir, audit.RunID, id, invocation)
		if artifactErr != nil && invocation.Status == "captured" {
			invocation.Status = "unavailable"
		}
		source := invocation.Provider
		if strings.TrimSpace(source) == "" {
			source = invocation.Name
		}
		evidenceInputs := make(map[string]string, len(invocation.Inputs)+2)
		for key, value := range invocation.Inputs {
			evidenceInputs[key] = value
		}
		if invocation.DisclosureTime != "" {
			evidenceInputs["disclosure_time"] = invocation.DisclosureTime
		}
		if len(invocation.ObservationTimes) > 0 {
			evidenceInputs["observation_times"] = strings.Join(invocation.ObservationTimes, ",")
		}
		evidence := agents.Evidence{
			ID: id, Kind: "tool-invocation", Source: source, URL: invocation.SourceURL, DataTime: invocation.DataTime,
			FetchedAt: invocation.CompletedAt, MethodVersion: invocation.MethodVersion,
			ContentHash: invocation.ContentHash, PayloadExcerpt: invocation.PayloadExcerpt,
			PayloadTruncated: invocation.PayloadTruncated, PayloadRef: payloadRef, PayloadSize: payloadSize,
			Status: invocation.Status, Inputs: evidenceInputs,
		}
		audit.Evidence = append(audit.Evidence, evidence)
		if agents.EvidenceCanSupportClaim(evidence) {
			ids[invocation.Name] = id
		}
	}
	*cursor = len(invocations)
	return ids
}

// Stop cancels a running analysis.
func (o *Orchestrator) Stop() {
	o.mu.RLock()
	cancel := o.cancelFn
	o.mu.RUnlock()
	if cancel != nil {
		cancel()
		log.Println("[ORCH] Analysis cancelled")
	}
}

func (o *Orchestrator) emit(event agents.NodeEvent) {
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}
	o.mu.RLock()
	handler := o.onEvent
	o.mu.RUnlock()
	if handler != nil {
		handler(event)
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
