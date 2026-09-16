package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/agents"
	"trading-agents/internal/dataflows"
	"trading-agents/internal/market"
)

// CreateEvidenceOnlyDossier persists a deterministic, source-only research
// package. It never invokes an LLM or represents itself as a 10-Agent run.
func (h *Handler) CreateEvidenceOnlyDossier(c *gin.Context) {
	startedAt := time.Now()
	var req agents.AnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Ticker = strings.ToUpper(strings.TrimSpace(req.Ticker))
	if !safeTickerRe.MatchString(req.Ticker) || !validTradeDate(req.TradeDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ticker and trade_date (YYYY-MM-DD) are required"})
		return
	}
	builder := h.evidenceOnlyBuild
	if builder == nil {
		builder = h.buildEvidenceOnlyFromProviders
	}
	result, preflight, err := builder(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "research_status": agents.ResearchStatusUnavailable})
		return
	}
	if !preflight.Passed || result == nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"research_status": agents.ResearchStatusUnavailable, "preflight": preflight,
			"message": "Evidence-only dossier was not persisted because source evidence failed the PIT gate.",
		})
		return
	}
	result.DurationSecs = time.Since(startedAt).Seconds()
	if err := h.saveResult(result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "research_status": agents.ResearchStatusUnavailable})
		return
	}
	h.mu.Lock()
	h.lastResult = result
	h.results = append(h.results, *result)
	h.mu.Unlock()
	c.Header("X-Research-Status", agents.ResearchStatusEvidenceOnly)
	c.JSON(http.StatusCreated, result)
}

func (h *Handler) buildEvidenceOnlyFromProviders(req agents.AnalysisRequest) (*agents.AnalysisResult, dataflows.ResearchPreflight, error) {
	registry := dataflows.NewToolRegistry(req.Ticker, req.TradeDate, h.config.FREDAPIKey)
	preflight := registry.RunEvidenceOnlyPreflight()
	invocations := registry.EvidenceSnapshot()
	if preflight.Passed {
		if quoteInvocation, ok := evidenceOnlyQuoteInvocationFromProvider(market.Default(), req.Ticker, req.TradeDate, time.Now().UTC()); ok {
			invocations = append(invocations, quoteInvocation)
		}
		preflight = dataflows.AssessEvidenceOnlyPreflight(invocations, req.TradeDate)
	}
	if quarterly, annual := fundamentalPeriodDepth(invocations); preflight.Passed && (quarterly < 5 || annual < 3) {
		preflight.Passed = false
		preflight.Reasons = append(preflight.Reasons, fmt.Sprintf("evidence-only dossier requires five quarterly and three annual filing-bound periods; captured quarterly=%d annual=%d", quarterly, annual))
	}
	if !preflight.Passed {
		return nil, preflight, nil
	}
	result, err := buildEvidenceOnlyResult(req, invocations, h.config.ResultsDir, time.Now().UTC())
	return result, preflight, err
}

type evidenceOnlyQuoteProvider interface {
	Quotes([]string) map[string]market.Quote
	SnapshotQuotes([]string) map[string]market.Quote
}

func evidenceOnlyQuoteInvocationFromProvider(provider evidenceOnlyQuoteProvider, symbol, tradeDate string, completedAt time.Time) (dataflows.ToolInvocation, bool) {
	quotes := provider.Quotes([]string{symbol})
	quote, ok := quotes[symbol]
	if !ok || quote.Price <= 0 || strings.TrimSpace(quote.DataTime) == "" || strings.TrimSpace(quote.ProviderURL) == "" {
		quote, ok = provider.SnapshotQuotes([]string{symbol})[symbol]
	}
	if !ok || quote.Price <= 0 || strings.TrimSpace(quote.DataTime) == "" || strings.TrimSpace(quote.ProviderURL) == "" {
		return dataflows.ToolInvocation{}, false
	}
	payload, err := json.Marshal(struct {
		Symbol string       `json:"symbol"`
		Quote  market.Quote `json:"quote"`
	}{Symbol: symbol, Quote: quote})
	if err != nil {
		return dataflows.ToolInvocation{}, false
	}
	raw := string(payload)
	excerpt := raw
	truncated := false
	if len(excerpt) > 2048 {
		excerpt = excerpt[:2048]
		truncated = true
	}
	return dataflows.ToolInvocation{
		Name: "get_stock_data", Inputs: map[string]string{
			"ticker": symbol, "requested_as_of": tradeDate, "source_mode": "provider-quote",
			"source_session": quote.SourceSession, "time_granularity": quote.TimeGranularity,
		},
		DataTime: quote.DataTime, ObservationTimes: []string{quote.DataTime}, Provider: quote.Source,
		TimeGranularity: quote.TimeGranularity,
		SourceURL:       quote.ProviderURL, CompletedAt: completedAt.Format(time.RFC3339), Status: "captured",
		ContentHash: agents.ContentHash(raw), PayloadExcerpt: excerpt, PayloadTruncated: truncated,
		PayloadSize: int64(len(raw)), MethodVersion: "market-provider-quote-v1", RawPayload: raw,
	}, true
}

func buildEvidenceOnlyResult(req agents.AnalysisRequest, invocations []dataflows.ToolInvocation, resultsDir string, now time.Time) (*agents.AnalysisResult, error) {
	preflight := dataflows.AssessEvidenceOnlyPreflight(invocations, req.TradeDate)
	if !preflight.Passed {
		return nil, fmt.Errorf("PIT preflight failed: %s", strings.Join(preflight.Reasons, "; "))
	}
	if quarterly, annual := fundamentalPeriodDepth(invocations); quarterly < 5 || annual < 3 {
		return nil, fmt.Errorf("five quarterly and three annual filing-bound financial periods are required; captured quarterly=%d annual=%d", quarterly, annual)
	}
	audit := agents.NewEvidenceOnlyAudit(req, now)
	byName := make(map[string]dataflows.ToolInvocation, len(invocations))
	for _, invocation := range invocations {
		byName[invocation.Name] = invocation
	}

	types := []struct{ name, id, category string }{
		{"get_historical_evidence", "tool-historical", "historical"},
		{"get_fundamentals", "tool-fundamentals", "fundamentals"},
		{"get_news", "tool-news", "news"},
	}
	if quote, ok := byName["get_stock_data"]; ok && optionalQuoteEvidenceUsable(quote) {
		types = append(types, struct{ name, id, category string }{"get_stock_data", "tool-quote-cross-check", "quote_cross_check"})
	}
	facts := make([]agents.EvidenceOnlyFact, 0, len(types))
	evidenceIDs := make([]string, 0, len(types))
	for _, sourceType := range types {
		invocation, ok := byName[sourceType.name]
		if !ok {
			return nil, fmt.Errorf("missing preflight invocation %s", sourceType.name)
		}
		payloadRef, payloadSize, err := dataflows.PersistPayloadArtifact(resultsDir, audit.RunID, sourceType.id, invocation)
		if err != nil {
			return nil, fmt.Errorf("persist %s evidence: %w", sourceType.category, err)
		}
		inputs := make(map[string]string, len(invocation.Inputs)+2)
		for key, value := range invocation.Inputs {
			inputs[key] = value
		}
		if len(invocation.ObservationTimes) > 0 {
			inputs["observation_times"] = strings.Join(invocation.ObservationTimes, ",")
		}
		if len(invocation.DisclosureTimes) > 0 {
			inputs["disclosure_times"] = strings.Join(invocation.DisclosureTimes, ",")
		} else if invocation.DisclosureTime != "" {
			inputs["disclosure_times"] = invocation.DisclosureTime
		}
		evidence := agents.Evidence{
			ID: sourceType.id, Kind: "tool-invocation", Source: invocation.Provider, URL: invocation.SourceURL,
			DataTime: invocation.DataTime, FetchedAt: invocation.CompletedAt, MethodVersion: invocation.MethodVersion,
			TimeGranularity: invocation.TimeGranularity,
			ContentHash:     invocation.ContentHash, PayloadExcerpt: invocation.PayloadExcerpt, PayloadTruncated: invocation.PayloadTruncated,
			PayloadRef: payloadRef, PayloadSize: payloadSize, Status: invocation.Status, Inputs: inputs,
		}
		audit.Evidence = append(audit.Evidence, evidence)
		evidenceIDs = append(evidenceIDs, sourceType.id)
		facts = append(facts, agents.EvidenceOnlyFact{
			Category: sourceType.category, Summary: evidenceOnlyFactSummary(sourceType.category, invocation),
			Provider: invocation.Provider, SourceURL: invocation.SourceURL, DataTime: invocation.DataTime,
			FilingDate: invocation.DisclosureTime, EvidenceIDs: []string{sourceType.id},
		})
	}
	if contextJSON := audit.Inputs["research_context"]; req.ResearchContext != nil && contextJSON != "" && contextJSON != "null" {
		audit.Evidence = append(audit.Evidence, agents.Evidence{
			ID: "input-research-context", Kind: "user-input", Source: "authenticated request body",
			DataTime: now.Format(time.RFC3339), TimeGranularity: "second", FetchedAt: now.Format(time.RFC3339),
			MethodVersion: agents.EvidenceOnlyMethodVersion, ContentHash: agents.ContentHash(contextJSON), PayloadExcerpt: contextJSON, Status: "captured",
			Inputs: map[string]string{"research_context": contextJSON, "requested_trade_date": req.TradeDate},
		})
		evidenceIDs = append(evidenceIDs, "input-research-context")
	}
	dossier, err := synthesizeEvidenceOnlyDossier(facts, byName, req.ResearchContext, req.Ticker)
	if err != nil {
		return nil, err
	}
	audit.RecordStage("evidence-only conclusion", agents.EvidenceOnlyDossierContent(dossier), agents.EvidenceOnlyMethodVersion, evidenceIDs, now)
	result := &agents.AnalysisResult{
		Ticker: req.Ticker, TradeDate: req.TradeDate, ResearchMode: agents.ResearchModeEvidenceOnly,
		Decision: dossier.Conclusion, Dossier: dossier, Audit: audit,
		CompletedAt: now.Format(time.RFC3339),
	}
	if health := agents.ApplyPublicationGate(result); !health.Publishable || result.Status != agents.ResearchStatusEvidenceOnly {
		return nil, fmt.Errorf("evidence-only publication gate rejected dossier: %s", strings.Join(health.Reasons, "; "))
	}
	return result, nil
}

func optionalQuoteEvidenceUsable(invocation dataflows.ToolInvocation) bool {
	return invocation.Status == "captured" && strings.TrimSpace(invocation.Provider) != "" && strings.TrimSpace(invocation.SourceURL) != "" &&
		strings.TrimSpace(invocation.DataTime) != "" && strings.TrimSpace(invocation.RawPayload) != "" && invocation.ContentHash == agents.ContentHash(invocation.RawPayload)
}

func evidenceOnlyQuoteCrossCheckGap(byName map[string]dataflows.ToolInvocation) string {
	quoteInvocation, quoteOK := byName["get_stock_data"]
	historyInvocation, historyOK := byName["get_historical_evidence"]
	if !quoteOK {
		return "Optional quote cross-check unavailable; the dated Nasdaq historical close remains the sole primary research price evidence."
	}
	if !historyOK {
		return "Optional quote cross-check cannot be compared because primary historical evidence is missing."
	}
	var quotePayload struct {
		Quote struct {
			Price    float64 `json:"price"`
			DataTime string  `json:"dataTime"`
		} `json:"quote"`
	}
	if err := json.Unmarshal([]byte(quoteInvocation.RawPayload), &quotePayload); err != nil || quotePayload.Quote.Price <= 0 || quotePayload.Quote.DataTime == "" {
		return "Optional quote cross-check payload is invalid; the dated Nasdaq historical close remains the sole primary research price evidence."
	}
	history, err := dataflows.ParseHistoricalEvidence(historyInvocation.RawPayload)
	if err != nil || len(history.Observations) == 0 {
		return "Optional quote cross-check cannot be compared because primary historical evidence is invalid."
	}
	latest := history.Observations[len(history.Observations)-1]
	note := fmt.Sprintf(" Quote provider=%s and historical provider=%s; this cross-check is optional and is not treated as independent primary market evidence.", quoteInvocation.Provider, historyInvocation.Provider)
	if quotePayload.Quote.DataTime != latest.Date {
		return fmt.Sprintf("Optional quote cross-check trading date conflict: quote=%s history=%s; research price remains historical close %.6f.%s", quotePayload.Quote.DataTime, latest.Date, latest.Close, note)
	}
	tolerance := math.Max(0.01, math.Abs(latest.Close)*0.0001)
	if math.Abs(quotePayload.Quote.Price-latest.Close) > tolerance {
		return fmt.Sprintf("Optional quote cross-check latest close conflict: quote=%.6f history=%.6f tolerance=%.6f; research price remains historical close.%s", quotePayload.Quote.Price, latest.Close, tolerance, note)
	}
	return fmt.Sprintf("Optional quote cross-check matched the dated historical close for %s at %.6f.%s", latest.Date, latest.Close, note)
}

func evidenceOnlyFactSummary(category string, invocation dataflows.ToolInvocation) string {
	switch category {
	case "historical":
		return fmt.Sprintf("Captured %d dated Nasdaq OHLCV observations; latest completed close at %s is the primary research price.", len(invocation.ObservationTimes), invocation.DataTime)
	case "quote_cross_check":
		return fmt.Sprintf("Captured an optional quote cross-check at %s; it is not required and is not presented as independent from the Nasdaq historical source.", invocation.DataTime)
	case "fundamentals":
		periodCount := len(invocation.DisclosureTimes)
		if fundamentals, err := dataflows.ParseFundamentalEvidence(invocation.RawPayload); err == nil {
			periodCount = len(fundamentals.Periods)
		}
		return fmt.Sprintf("Captured %d filing-bound financial periods; latest filing date %s.", periodCount, invocation.DisclosureTime)
	case "news":
		return fmt.Sprintf("Captured %d timestamped news items; latest publication %s.", len(invocation.ObservationTimes), invocation.DataTime)
	default:
		return "Captured source evidence."
	}
}

func fundamentalPeriodDepth(invocations []dataflows.ToolInvocation) (quarterly, annual int) {
	for _, invocation := range invocations {
		if invocation.Name == "get_fundamentals" {
			fundamentals, err := dataflows.ParseFundamentalEvidence(invocation.RawPayload)
			if err != nil {
				return 0, 0
			}
			seen := make(map[string]bool)
			for _, period := range fundamentals.Periods {
				frequency := strings.ToLower(strings.TrimSpace(period.Frequency))
				key := frequency + ":" + period.PeriodStart + ":" + period.PeriodEnd + ":" + period.Accession
				if seen[key] {
					continue
				}
				seen[key] = true
				switch frequency {
				case "quarterly":
					quarterly++
				case "annual":
					annual++
				}
			}
			return quarterly, annual
		}
	}
	return 0, 0
}

// GetResearchEvidencePayload returns an exact, hash-verified source payload to authenticated administrators.
func (h *Handler) GetResearchEvidencePayload(c *gin.Context) {
	runID := c.Param("runID")
	evidenceID := c.Param("evidenceID")
	for _, result := range h.loadResults() {
		if result.Audit.RunID != runID {
			continue
		}
		for _, evidence := range result.Audit.Evidence {
			if evidence.ID != evidenceID || evidence.PayloadRef == "" {
				continue
			}
			raw, err := dataflows.ReadPayloadArtifact(h.config.ResultsDir, runID, evidenceID, evidence.ContentHash)
			if err != nil {
				c.JSON(http.StatusConflict, gin.H{"error": "evidence artifact is missing or failed hash verification"})
				return
			}
			if evidence.PayloadSize != int64(len(raw)) {
				c.JSON(http.StatusConflict, gin.H{"error": "evidence artifact size mismatch"})
				return
			}
			c.Header("X-Content-Hash", evidence.ContentHash)
			c.Header("X-Payload-Size", strconv.FormatInt(evidence.PayloadSize, 10))
			c.Data(http.StatusOK, "text/plain; charset=utf-8", raw)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "research evidence payload not found"})
}
