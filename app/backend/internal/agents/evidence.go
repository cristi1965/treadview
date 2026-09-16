package agents

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const AnalysisMethodVersion = "tradingagents-go-pipeline-v3-evidence-gated"
const EvidenceOnlyMethodVersion = "evidence-only-dossier-v6"

// ContentHash returns a stable SHA-256 identifier without persisting duplicate raw content.
func ContentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// NewAnalysisAudit initializes a run with the external source classes available to the tool registry.
func NewAnalysisAudit(req AnalysisRequest, outputLanguage, provider, quickModel, deepModel string, maxDebateRounds, maxRiskRounds int, now time.Time) AnalysisAudit {
	createdAt := now.UTC().Format(time.RFC3339)
	ticker := strings.ToUpper(strings.TrimSpace(req.Ticker))
	inputs := map[string]string{
		"ticker": ticker, "trade_date": req.TradeDate, "asset_type": req.AssetType,
		"output_language": outputLanguage, "max_debate_rounds": fmt.Sprint(maxDebateRounds),
		"max_risk_rounds": fmt.Sprint(maxRiskRounds),
	}
	input := strings.Join([]string{ticker, req.TradeDate, req.AssetType, outputLanguage, fmt.Sprint(maxDebateRounds), fmt.Sprint(maxRiskRounds), provider, quickModel, deepModel, AnalysisMethodVersion}, "|")
	audit := AnalysisAudit{
		RunID:         newRunID(now),
		CreatedAt:     createdAt,
		MethodVersion: AnalysisMethodVersion,
		ModelProvider: provider,
		Models: map[string]string{
			"quick": quickModel,
			"deep":  deepModel,
		},
		Inputs:    inputs,
		InputHash: ContentHash(input),
		Evidence:  []Evidence{},
		Claims:    []Claim{},
	}
	for _, source := range []Evidence{
		{ID: "market-data", Kind: "input-source", Source: "TradingView/Yahoo market tools", URL: fmt.Sprintf("https://finance.yahoo.com/quote/%s/history/", ticker), DataTime: "unknown", FetchedAt: createdAt, MethodVersion: "market-tools-v1", Status: "declared"},
		{ID: "fundamentals-data", Kind: "input-source", Source: "Yahoo/EastMoney/Nasdaq fundamentals tools", URL: fmt.Sprintf("https://finance.yahoo.com/quote/%s/key-statistics/", strings.ToUpper(strings.TrimSpace(ticker))), DataTime: "unknown", FetchedAt: createdAt, MethodVersion: "fundamentals-tools-v1", Status: "declared"},
		{ID: "news-data", Kind: "input-source", Source: "Yahoo News/FRED/CBOE news tools", URL: fmt.Sprintf("https://finance.yahoo.com/quote/%s/news/", ticker), DataTime: "unknown", FetchedAt: createdAt, MethodVersion: "news-tools-v1", Status: "declared"},
		{ID: "public-player-data", Kind: "input-source", Source: "StockTwits/X public-player tools", URL: fmt.Sprintf("https://stocktwits.com/symbol/%s", ticker), DataTime: "unknown", FetchedAt: createdAt, MethodVersion: "public-player-tools-v1", Status: "declared"},
	} {
		audit.Evidence = append(audit.Evidence, source)
	}
	return audit
}

// NewEvidenceOnlyAudit declares a no-LLM run without pretending that agent or
// portfolio stages were executed.
func NewEvidenceOnlyAudit(req AnalysisRequest, now time.Time) AnalysisAudit {
	ticker := strings.ToUpper(strings.TrimSpace(req.Ticker))
	inputs := map[string]string{"ticker": ticker, "trade_date": req.TradeDate, "asset_type": req.AssetType, "research_mode": ResearchModeEvidenceOnly}
	contextJSON := "null"
	if req.ResearchContext != nil {
		if raw, err := json.Marshal(req.ResearchContext); err == nil {
			contextJSON = string(raw)
		}
	}
	inputs["research_context"] = contextJSON
	input := strings.Join([]string{ticker, req.TradeDate, req.AssetType, ResearchModeEvidenceOnly, EvidenceOnlyMethodVersion, contextJSON}, "|")
	return AnalysisAudit{
		RunID: newRunID(now), CreatedAt: now.UTC().Format(time.RFC3339), MethodVersion: EvidenceOnlyMethodVersion,
		ModelProvider: "none", Models: map[string]string{}, Inputs: inputs, InputHash: ContentHash(input),
		Evidence: []Evidence{}, Claims: []Claim{},
	}
}

// RecordStage stores a verifiable digest for a generated artifact and links it to upstream evidence.
func (a *AnalysisAudit) RecordStage(stage, content, model string, upstream []string, now time.Time) string {
	if a == nil || strings.TrimSpace(content) == "" {
		return ""
	}
	normalizedStage := strings.ToLower(strings.NewReplacer(" ", "-", "/", "-", "_", "-").Replace(stage))
	index := len(a.Claims) + 1
	artifactID := fmt.Sprintf("artifact-%02d-%s", index, normalizedStage)
	hash := ContentHash(content)
	createdAt := now.UTC().Format(time.RFC3339)
	lineage := assessEvidenceLineage(*a, upstream)
	status := "degraded"
	dataTime := "unknown"
	if lineage.healthy {
		status = "captured"
		dataTime = formatEvidenceTime(lineage.oldest)
	}
	a.Evidence = append(a.Evidence, Evidence{
		ID: artifactID, Kind: "generated-artifact", Source: stage, DataTime: dataTime,
		FetchedAt: createdAt, MethodVersion: a.MethodVersion, ContentHash: hash, Status: status,
	})
	evidenceIDs := make([]string, 0, len(upstream)+1)
	for _, id := range upstream {
		if id != "" {
			evidenceIDs = append(evidenceIDs, id)
		}
	}
	a.Claims = append(a.Claims, Claim{
		ID: fmt.Sprintf("claim-%02d", index), ArtifactID: artifactID, Stage: stage, Summary: stage + " generated output",
		ContentHash: hash, EvidenceIDs: evidenceIDs, DataTime: dataTime, Status: status, Model: model,
		MethodVersion: a.MethodVersion, CreatedAt: createdAt,
	})
	return artifactID
}

type lineageAssessment struct {
	healthy bool
	oldest  time.Time
	reasons []string
}

// EvidenceCanSupportClaim excludes failed attempts and unverifiable payloads from claim support.
func EvidenceCanSupportClaim(e Evidence) bool {
	if e.Kind == "generated-artifact" || e.Status != "captured" || strings.TrimSpace(e.Source) == "" || strings.TrimSpace(e.PayloadExcerpt) == "" || strings.TrimSpace(e.ContentHash) == "" {
		return false
	}
	if e.Kind == "tool-invocation" && (strings.TrimSpace(e.PayloadRef) == "" || e.PayloadSize <= 0) {
		return false
	}
	_, ok := parseEvidenceTime(e.DataTime)
	return ok
}

// AssessAudit reports structural completeness separately from source health.
func AssessAudit(a AnalysisAudit) ResearchHealth {
	return assessAudit(a, "")
}

// AssessAuditForPublication additionally requires a final portfolio-decision claim.
func AssessAuditForPublication(a AnalysisAudit) ResearchHealth {
	return assessAudit(a, "portfolio decision")
}

// AssessAuditForEvidenceOnly requires a deterministic dossier conclusion while
// retaining the same source payload, hash, and lineage requirements.
func AssessAuditForEvidenceOnly(a AnalysisAudit) ResearchHealth {
	return assessAudit(a, "evidence-only conclusion")
}

func assessAudit(a AnalysisAudit, requiredFinalStage string) ResearchHealth {
	health := ResearchHealth{StructureStatus: "complete", SourceStatus: "healthy", DataTime: "unknown"}
	structural := make([]string, 0)
	sourceReasons := make([]string, 0)
	if strings.TrimSpace(a.RunID) == "" || strings.TrimSpace(a.InputHash) == "" || len(a.Inputs) == 0 {
		structural = append(structural, "missing run metadata")
	}
	evidenceIDs := make(map[string]bool, len(a.Evidence))
	evidenceByID := make(map[string]Evidence, len(a.Evidence))
	sourceUnavailable := false
	sourceDegraded := false
	for _, e := range a.Evidence {
		id := strings.TrimSpace(e.ID)
		if id == "" || evidenceIDs[id] {
			structural = append(structural, "missing or duplicate evidence id")
			continue
		}
		evidenceIDs[id] = true
		evidenceByID[id] = e
		if strings.TrimSpace(e.Kind) == "" || strings.TrimSpace(e.Source) == "" || strings.TrimSpace(e.Status) == "" || strings.TrimSpace(e.MethodVersion) == "" {
			structural = append(structural, "incomplete evidence metadata: "+id)
		}
		if e.Kind == "generated-artifact" || e.Status == "declared" {
			continue
		}
		switch e.Status {
		case "error", "unavailable":
			sourceUnavailable = true
			sourceReasons = append(sourceReasons, "source attempt "+e.Status+": "+id)
		case "captured":
			if !EvidenceCanSupportClaim(e) {
				sourceDegraded = true
				sourceReasons = append(sourceReasons, "captured source is not reviewable: "+id)
			}
		default:
			sourceDegraded = true
			sourceReasons = append(sourceReasons, "unknown source status: "+id)
		}
	}
	if len(a.Claims) == 0 {
		structural = append(structural, "no claims")
	}
	finalFound := false
	claimIDs := make(map[string]bool, len(a.Claims))
	artifactClaims := make(map[string]bool, len(a.Claims))
	for _, claim := range a.Claims {
		if requiredFinalStage != "" && strings.EqualFold(strings.TrimSpace(claim.Stage), requiredFinalStage) {
			finalFound = true
		}
		artifact, artifactExists := evidenceByID[claim.ArtifactID]
		claimStructurallyValid := strings.TrimSpace(claim.ID) != "" && !claimIDs[claim.ID] && strings.TrimSpace(claim.ArtifactID) != "" && !artifactClaims[claim.ArtifactID] && strings.TrimSpace(claim.ContentHash) != "" && len(claim.EvidenceIDs) > 0 && artifactExists && artifact.Kind == "generated-artifact" && artifact.ContentHash == claim.ContentHash
		if !claimStructurallyValid {
			structural = append(structural, "incomplete claim metadata: "+claim.ID)
		}
		if strings.TrimSpace(claim.ID) != "" && !claimIDs[claim.ID] {
			claimIDs[claim.ID] = true
		}
		if strings.TrimSpace(claim.ArtifactID) != "" && !artifactClaims[claim.ArtifactID] {
			artifactClaims[claim.ArtifactID] = true
		}
		if len(claim.EvidenceIDs) == 0 {
			continue
		}
		lineage := assessEvidenceLineage(a, claim.EvidenceIDs)
		if !lineage.healthy {
			sourceReasons = append(sourceReasons, lineage.reasons...)
			continue
		}
		if !artifactExists || claim.Status != "captured" || !sameEvidenceTime(claim.DataTime, lineage.oldest) || artifact.Status != "captured" || !sameEvidenceTime(artifact.DataTime, lineage.oldest) {
			sourceReasons = append(sourceReasons, "generated health inheritance mismatch: "+claim.ID)
			continue
		}
		if health.DataTime == "unknown" || (!lineage.oldest.IsZero() && evidenceTimeBefore(lineage.oldest, health.DataTime)) {
			health.DataTime = formatEvidenceTime(lineage.oldest)
		}
	}
	if requiredFinalStage != "" && !finalFound {
		structural = append(structural, "missing "+requiredFinalStage+" claim")
	}
	if len(structural) > 0 {
		health.StructureStatus = "invalid"
	}
	if sourceUnavailable {
		health.SourceStatus = "unavailable"
	} else if sourceDegraded || len(sourceReasons) > 0 {
		health.SourceStatus = "degraded"
	}
	health.Reasons = uniqueEvidenceReasons(append(structural, sourceReasons...))
	health.Publishable = health.StructureStatus == "complete" && health.SourceStatus == "healthy" && health.DataTime != "unknown"
	return health
}

func assessEvidenceLineage(a AnalysisAudit, ids []string) lineageAssessment {
	result := lineageAssessment{healthy: len(ids) > 0}
	evidenceByID := make(map[string]Evidence, len(a.Evidence))
	claimByArtifact := make(map[string]Claim, len(a.Claims))
	for _, e := range a.Evidence {
		evidenceByID[e.ID] = e
	}
	for _, claim := range a.Claims {
		claimByArtifact[claim.ArtifactID] = claim
	}
	var walk func(string, map[string]bool) (time.Time, bool)
	walk = func(id string, stack map[string]bool) (time.Time, bool) {
		id = strings.TrimSpace(id)
		if stack[id] {
			result.reasons = append(result.reasons, "cycle in evidence lineage: "+id)
			return time.Time{}, false
		}
		e, ok := evidenceByID[id]
		if !ok {
			result.reasons = append(result.reasons, "missing evidence: "+id)
			return time.Time{}, false
		}
		if e.Kind != "generated-artifact" {
			if !EvidenceCanSupportClaim(e) {
				result.reasons = append(result.reasons, "unhealthy source evidence: "+id)
				return time.Time{}, false
			}
			parsed, _ := parseEvidenceTime(e.DataTime)
			return parsed, true
		}
		claim, ok := claimByArtifact[id]
		if !ok || claim.ContentHash != e.ContentHash || len(claim.EvidenceIDs) == 0 || e.Status != "captured" {
			result.reasons = append(result.reasons, "generated artifact has no verifiable claim: "+id)
			return time.Time{}, false
		}
		next := make(map[string]bool, len(stack)+1)
		for key, value := range stack {
			next[key] = value
		}
		next[id] = true
		oldest := time.Time{}
		for _, upstreamID := range claim.EvidenceIDs {
			candidate, healthy := walk(upstreamID, next)
			if !healthy {
				return time.Time{}, false
			}
			if oldest.IsZero() || candidate.Before(oldest) {
				oldest = candidate
			}
		}
		if !sameEvidenceTime(e.DataTime, oldest) || !sameEvidenceTime(claim.DataTime, oldest) || claim.Status != "captured" {
			result.reasons = append(result.reasons, "generated health inheritance mismatch: "+id)
			return time.Time{}, false
		}
		return oldest, !oldest.IsZero()
	}
	for _, id := range ids {
		candidate, healthy := walk(id, map[string]bool{})
		if !healthy {
			result.healthy = false
			continue
		}
		if result.oldest.IsZero() || candidate.Before(result.oldest) {
			result.oldest = candidate
		}
	}
	if result.oldest.IsZero() {
		result.healthy = false
	}
	result.reasons = uniqueEvidenceReasons(result.reasons)
	return result
}

// ApplyPublicationGate strips all generated advice when the final evidence lineage is not publishable.
func ApplyPublicationGate(result *AnalysisResult) ResearchHealth {
	if result == nil {
		return ResearchHealth{StructureStatus: "invalid", SourceStatus: "degraded", DataTime: "unknown", Reasons: []string{"nil result"}}
	}
	if result.ResearchMode == ResearchModeEvidenceOnly {
		health := AssessAuditForEvidenceOnly(result.Audit)
		result.Audit.Health = health
		result.ResearchHealth = health
		if health.Publishable {
			if reason := evidenceOnlyOutputViolation(result); reason != "" {
				return MarkResearchUnavailable(result, reason)
			}
			result.Status = ResearchStatusEvidenceOnly
			result.Decision = result.Dossier.Conclusion
			result.Message = "Evidence-only research dossier; not investment advice and not a 10-Agent analysis."
			return health
		}
		return MarkResearchUnavailable(result, "")
	}
	health := AssessAuditForPublication(result.Audit)
	result.Audit.Health = health
	result.ResearchHealth = health
	if health.Publishable {
		if reason := publishedOutputViolation(result); reason != "" {
			return MarkResearchUnavailable(result, reason)
		}
		result.Status = ResearchStatusPublished
		return health
	}
	return MarkResearchUnavailable(result, "")
}

// MarkResearchUnavailable is the single fail-closed path for evidence and output-policy failures.
func MarkResearchUnavailable(result *AnalysisResult, reason string) ResearchHealth {
	if result == nil {
		return ResearchHealth{StructureStatus: "invalid", SourceStatus: "degraded", DataTime: "unknown", Reasons: []string{"nil result"}}
	}
	preservedMessage := ""
	if result.Status == ResearchStatusUnavailable {
		preservedMessage = strings.TrimSpace(result.Message)
	}
	health := result.ResearchHealth
	if health.StructureStatus == "" {
		if result.ResearchMode == ResearchModeEvidenceOnly {
			health = AssessAuditForEvidenceOnly(result.Audit)
		} else {
			health = AssessAuditForPublication(result.Audit)
		}
	}
	if reason != "" {
		if health.SourceStatus == "" || health.SourceStatus == "healthy" {
			health.SourceStatus = "degraded"
		}
		health.Publishable = false
		health.Reasons = uniqueEvidenceReasons(append(health.Reasons, reason))
	}
	result.Audit.Health = health
	result.ResearchHealth = health
	result.Status = ResearchStatusUnavailable
	result.Decision = ResearchDecisionUnavailable
	result.Message = "Research unavailable: source evidence is incomplete or unhealthy; no investment action is published."
	if preservedMessage != "" {
		result.Message = preservedMessage
	}
	result.State = AgentState{
		CompanyOfInterest: result.State.CompanyOfInterest,
		AssetType:         result.State.AssetType,
		InstrumentContext: result.State.InstrumentContext,
		TradeDate:         result.State.TradeDate,
		OutputLanguage:    result.State.OutputLanguage,
	}
	result.Dossier = nil
	return health
}

// EvidenceOnlyDossierContent is the canonical content hashed by its claim.
func EvidenceOnlyDossierContent(dossier *EvidenceOnlyDossier) string {
	if dossier == nil {
		return ""
	}
	raw, err := json.Marshal(dossier)
	if err != nil {
		return ""
	}
	return string(raw)
}

func evidenceOnlyOutputViolation(result *AnalysisResult) string {
	if result.Dossier == nil || result.Dossier.MethodVersion != EvidenceOnlyMethodVersion || len(result.Dossier.Facts) < 3 || len(result.Dossier.FinancialPeriods) < 2 || result.Dossier.Historical == nil || result.Dossier.NewsReview == nil || result.Dossier.MarketSubmodel == nil || len(result.Dossier.Calculations) == 0 || len(result.Dossier.ConclusionBasis) == 0 {
		return "evidence-only dossier is incomplete"
	}
	requiredFacts := map[string]bool{"historical": false, "fundamentals": false, "news": false}
	for _, fact := range result.Dossier.Facts {
		if _, required := requiredFacts[fact.Category]; required {
			requiredFacts[fact.Category] = true
		}
	}
	for category, present := range requiredFacts {
		if !present {
			return "evidence-only dossier is missing required fact: " + category
		}
	}
	for _, calculation := range result.Dossier.Calculations {
		if calculation.Name == "" || calculation.Formula == "" || len(calculation.EvidenceIDs) == 0 || (calculation.Status != "computed" && calculation.Status != "unknown") || (calculation.Status == "computed" && calculation.Value == nil) || (calculation.Status == "unknown" && calculation.Reason == "") {
			return "evidence-only dossier contains an incomplete calculation"
		}
	}
	if result.Dossier.Conclusion != ResearchDecisionObserve && result.Dossier.Conclusion != ResearchDecisionAbstain {
		return "evidence-only conclusion must be OBSERVE or ABSTAIN"
	}
	if !strings.Contains(strings.ToLower(result.Dossier.Disclaimer), "not investment advice") || !strings.Contains(strings.ToLower(result.Dossier.Label), "evidence-only") {
		return "evidence-only dossier disclosure is incomplete"
	}
	wantedHash := ContentHash(EvidenceOnlyDossierContent(result.Dossier))
	for _, claim := range result.Audit.Claims {
		if strings.EqualFold(claim.Stage, "evidence-only conclusion") && claim.ContentHash == wantedHash {
			return ""
		}
	}
	return "evidence-only dossier does not match its claim hash"
}

// ConditionalObservationSafe rejects execution and assumed-holdings language in the no-position workflow.
func ConditionalObservationSafe(content string) bool {
	normalized := strings.ToLower(strings.TrimSpace(content))
	if normalized == "" {
		return false
	}
	for _, forbidden := range []string{
		"stop loss", "stop-loss", "position size", "existing position", "call option", "put option",
		"买入", "卖出", "持有", "增持", "减持", "止损", "仓位", "期权",
	} {
		if strings.Contains(normalized, forbidden) {
			return false
		}
	}
	for _, word := range strings.FieldsFunc(normalized, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	}) {
		if word == "buy" || word == "sell" || word == "hold" || word == "overweight" || word == "underweight" {
			return false
		}
	}
	return true
}

func publishedOutputViolation(result *AnalysisResult) string {
	stageContent := map[string]string{
		"public-player snapshot": result.State.XPlayerTakes,
		"market report":          result.State.MarketReport,
		"fundamentals report":    result.State.FundamentalsReport,
		"sentiment report":       result.State.SentimentReport,
		"news report":            result.State.NewsReport,
		"research debate":        result.State.InvestmentDebateState.History,
		"research decision":      result.State.InvestmentPlan,
		"portfolio decision":     result.State.FinalTradeDecision,
	}
	for _, claim := range result.Audit.Claims {
		if content, ok := stageContent[strings.ToLower(strings.TrimSpace(claim.Stage))]; ok {
			if strings.TrimSpace(content) == "" || ContentHash(content) != claim.ContentHash {
				return "published output does not match claim hash: " + claim.Stage
			}
		}
	}
	if !ConditionalObservationSafe(result.State.InvestmentPlan) || !ConditionalObservationSafe(result.State.FinalTradeDecision) {
		return "final research output contains position or executable-action language without verified holdings input"
	}
	return ""
}

func parseEvidenceTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04 MST", "2006-01-02 15:04", "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func formatEvidenceTime(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	if value.Hour() == 0 && value.Minute() == 0 && value.Second() == 0 && value.Nanosecond() == 0 {
		return value.Format("2006-01-02")
	}
	return value.UTC().Format(time.RFC3339)
}

func evidenceTimeBefore(candidate time.Time, existing string) bool {
	parsed, ok := parseEvidenceTime(existing)
	return !ok || candidate.Before(parsed)
}

func sameEvidenceTime(value string, expected time.Time) bool {
	parsed, ok := parseEvidenceTime(value)
	return ok && parsed.Equal(expected)
}

func uniqueEvidenceReasons(reasons []string) []string {
	out := make([]string, 0, len(reasons))
	seen := map[string]bool{}
	for _, reason := range reasons {
		if reason = strings.TrimSpace(reason); reason != "" && !seen[reason] {
			seen[reason] = true
			out = append(out, reason)
		}
	}
	return out
}

func newRunID(now time.Time) string {
	var entropy [6]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return fmt.Sprintf("run-%d", now.UTC().UnixNano())
	}
	return fmt.Sprintf("run-%s-%s", now.UTC().Format("20060102T150405.000Z"), hex.EncodeToString(entropy[:]))
}
