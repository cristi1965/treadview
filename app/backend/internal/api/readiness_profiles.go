package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"trading-agents/internal/agents"
	"trading-agents/internal/config"
	"trading-agents/internal/database"
	"trading-agents/internal/dataflows"
	"trading-agents/internal/models"
)

const (
	readinessProfileFull               = "full"
	readinessProfileHistoricalResearch = "historical-research"
	readinessProfilePaperEngine        = "paper-engine"
	readinessProfileLiveTradingData    = "live-trading-data"
)

var readinessProfileNames = []string{
	readinessProfileHistoricalResearch,
	readinessProfilePaperEngine,
	readinessProfileLiveTradingData,
}

type readinessProfileResponse struct {
	Profile    string        `json:"profile"`
	Label      string        `json:"label"`
	Status     string        `json:"status"`
	Ready      bool          `json:"ready"`
	HTTPStatus int           `json:"httpStatus"`
	CheckedAt  string        `json:"checkedAt"`
	Scope      []string      `json:"scope"`
	Checks     []systemCheck `json:"checks"`
	Remedies   []string      `json:"remedies,omitempty"`
	Disclaimer string        `json:"disclaimer"`
}

func (h *Handler) getReadinessProfile(c *gin.Context, profile string) {
	var response readinessProfileResponse
	switch profile {
	case readinessProfileHistoricalResearch:
		response = historicalResearchReadiness(h.config, time.Now().UTC())
	case readinessProfilePaperEngine:
		response = paperEngineReadiness(database.DB, h.paperStopScanner, h.config, time.Now().UTC())
	case readinessProfileLiveTradingData:
		status := h.currentSystemStatus()
		quoteResult := boundedQuotesForRequest([]string{"AAPL", "NVDA"})
		response = liveTradingDataReadinessWithDirectQuote(status, quoteResult, time.Now().UTC())
		setDataFreshness(c, liveReadinessFreshness(response, status, quoteResult))
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":           "unknown readiness profile",
			"allowedProfiles": append([]string{readinessProfileFull}, readinessProfileNames...),
		})
		return
	}
	c.Header("X-Readiness-Profile", response.Profile)
	c.JSON(response.HTTPStatus, response)
}

func newReadinessProfile(profile, label string, scope []string, checks []systemCheck, disclaimer string, checkedAt time.Time) readinessProfileResponse {
	ready := true
	remedies := make([]string, 0)
	seenRemedy := make(map[string]bool)
	for _, check := range checks {
		if check.Status != "ok" {
			ready = false
			if check.Remedy != "" && !seenRemedy[check.Remedy] {
				seenRemedy[check.Remedy] = true
				remedies = append(remedies, check.Remedy)
			}
		}
	}
	status := "ok"
	httpStatus := http.StatusOK
	if !ready {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}
	return readinessProfileResponse{
		Profile: profile, Label: label, Status: status, Ready: ready, HTTPStatus: httpStatus,
		CheckedAt: checkedAt.UTC().Format(time.RFC3339), Scope: scope, Checks: checks,
		Remedies: remedies, Disclaimer: disclaimer,
	}
}

func historicalResearchReadiness(cfg *config.Config, checkedAt time.Time) readinessProfileResponse {
	resultsDir := ""
	if cfg != nil {
		resultsDir = cfg.ResultsDir
	}
	return historicalResearchReadinessFromDir(resultsDir, checkedAt)
}

func historicalResearchReadinessFromDir(resultsDir string, checkedAt time.Time) readinessProfileResponse {
	checks := []systemCheck{
		{ID: "research.pit_price", Status: "fail", Detail: "no complete evidence-only dossier with a PIT primary price from dated historical OHLCV", Remedy: "Create an authenticated evidence-only dossier whose latest completed Nasdaq historical close is at or before the trade date.", Severity: "high"},
		{ID: "research.history", Status: "fail", Detail: "no complete evidence-only dossier with sufficient dated Nasdaq OHLCV history", Remedy: "Capture at least 21 valid dated OHLCV sessions for the dossier trade date.", Severity: "high"},
		{ID: "research.filings", Status: "fail", Detail: "no complete evidence-only dossier with five discrete quarters and three annual filings", Remedy: "Capture five discrete 10-Q quarters and three 10-K annual periods with field-level SEC period, filing date, accession, and source URL available by the trade date.", Severity: "high"},
		{ID: "research.news", Status: "fail", Detail: "no complete evidence-only dossier with a reviewed news source", Remedy: "Run the deterministic ticker-news review and retain its captured source evidence.", Severity: "high"},
		{ID: "research.dossier", Status: "fail", Detail: "no complete evidence-only dossier", Remedy: "Create and retain at least one publication-gated evidence-only dossier.", Severity: "high"},
	}
	bestPassed := -1
	bestPassedChecks := map[string]bool{}
	bestDetails := map[string]string{}
	dir := filepath.Join(strings.TrimSpace(resultsDir), "api_results")
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			raw, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
			if readErr != nil {
				continue
			}
			var result agents.AnalysisResult
			if json.Unmarshal(raw, &result) != nil || result.ResearchMode != agents.ResearchModeEvidenceOnly {
				continue
			}
			passed, details := assessHistoricalResearchResult(resultsDir, &result)
			if len(passed) > bestPassed {
				bestPassed = len(passed)
				bestPassedChecks = passed
				bestDetails = details
			}
			if len(passed) == len(checks) {
				for index := range checks {
					checks[index].Status = "ok"
					checks[index].Detail = details[checks[index].ID]
					checks[index].Remedy = ""
					checks[index].Severity = ""
				}
				break
			}
		}
	}
	if checks[len(checks)-1].Status != "ok" && len(bestDetails) > 0 {
		for index := range checks {
			if bestPassedChecks[checks[index].ID] {
				checks[index].Status = "ok"
				checks[index].Remedy = ""
				checks[index].Severity = ""
			}
			if detail := bestDetails[checks[index].ID]; detail != "" {
				checks[index].Detail = detail
			}
		}
	}
	return newReadinessProfile(
		readinessProfileHistoricalResearch, "Historical research", []string{"PIT primary price from dated Nasdaq historical close", "dated Nasdaq OHLCV history", "filing-bound fundamentals", "reviewed ticker news", "at least one complete evidence-only dossier"}, checks,
		"This profile only proves an auditable historical-research path. It does not imply live market readiness or provide a trade signal.", checkedAt,
	)
}

func assessHistoricalResearchResult(resultsDir string, result *agents.AnalysisResult) (map[string]bool, map[string]string) {
	passed := make(map[string]bool)
	details := make(map[string]string)
	if result == nil || result.Dossier == nil {
		return passed, details
	}
	if err := verifyResearchPayloadArtifacts(resultsDir, result); err != nil {
		details["research.dossier"] = "candidate dossier source artifact is unavailable: " + err.Error()
		return passed, details
	}
	health := agents.ApplyPublicationGate(result)
	if !health.Publishable || result.Status != agents.ResearchStatusEvidenceOnly || result.Dossier.MethodVersion != agents.EvidenceOnlyMethodVersion {
		details["research.dossier"] = "candidate dossier failed the evidence-only publication gate"
		return passed, details
	}
	passed["research.dossier"] = true
	details["research.dossier"] = fmt.Sprintf("complete %s dossier for %s", result.Dossier.MethodVersion, result.Ticker)

	evidenceByID := make(map[string]agents.Evidence, len(result.Audit.Evidence))
	for _, evidence := range result.Audit.Evidence {
		evidenceByID[evidence.ID] = evidence
	}
	facts := make(map[string]agents.EvidenceOnlyFact, len(result.Dossier.Facts))
	for _, fact := range result.Dossier.Facts {
		facts[fact.Category] = fact
	}
	tradeDate, tradeDateErr := time.Parse("2006-01-02", result.TradeDate)

	historical := result.Dossier.Historical
	if historical != nil && historical.Status == "complete" && historical.MinimumSessions >= 21 && historical.TimeGranularity == "date" && factEvidenceCaptured(facts["historical"], evidenceByID) && tradeDateErr == nil {
		if validHistoricalProfileObservations(historical, tradeDate) {
			passed["research.history"] = true
			details["research.history"] = fmt.Sprintf("%d dated sessions; minimum=%d", historical.SampleCount, historical.MinimumSessions)
			if validPrimaryHistoricalPrice(result, facts["historical"], tradeDate) {
				latest := historical.Observations[len(historical.Observations)-1]
				passed["research.pit_price"] = true
				details["research.pit_price"] = fmt.Sprintf("Nasdaq historical close %.6f at %s is the primary research price", latest.Close, latest.Date)
			}
		}
	}

	if factEvidenceCaptured(facts["fundamentals"], evidenceByID) && tradeDateErr == nil {
		validPeriods := true
		quarterCount, annualCount := 0, 0
		for _, period := range result.Dossier.FinancialPeriods {
			filingDate, err := time.Parse("2006-01-02", period.FilingDate)
			if err != nil || filingDate.After(tradeDate) || period.Accession == "" || period.SourceURL == "" || !validEvidenceFinancialField(period, "totalRevenue") {
				validPeriods = false
				break
			}
			for _, field := range period.AvailableFields {
				if !validEvidenceFinancialField(period, field) {
					validPeriods = false
					break
				}
			}
			if financialPeriodFrequency(period) == "quarterly" {
				quarterCount++
			}
			if financialPeriodFrequency(period) == "annual" {
				annualCount++
			}
		}
		if validPeriods && quarterCount >= 5 && annualCount >= 3 {
			passed["research.filings"] = true
			details["research.filings"] = fmt.Sprintf("%d discrete quarters and %d annual SEC periods with field-level provenance available by trade date", quarterCount, annualCount)
		} else {
			details["research.filings"] = fmt.Sprintf("insufficient proven SEC depth: discrete quarters=%d/5 annual periods=%d/3", quarterCount, annualCount)
		}
	}

	if result.Dossier.NewsReview != nil && factEvidenceCaptured(facts["news"], evidenceByID) && tradeDateErr == nil && dataTimeAtOrBeforeTradeDate(facts["news"].DataTime, tradeDate) {
		if validNewsReviewForProfile(result.Dossier.NewsReview, tradeDate) {
			passed["research.news"] = true
			details["research.news"] = fmt.Sprintf("reviewed=%d included=%d; no item is later than the trade date", result.Dossier.NewsReview.InputCount, result.Dossier.NewsReview.IncludedCount)
		}
	}
	return passed, details
}

func verifyResearchPayloadArtifacts(resultsDir string, result *agents.AnalysisResult) error {
	if result == nil || strings.TrimSpace(result.Audit.RunID) == "" {
		return fmt.Errorf("missing run id")
	}
	for _, evidence := range result.Audit.Evidence {
		if evidence.Kind != "tool-invocation" || evidence.Status != "captured" {
			continue
		}
		raw, err := dataflows.ReadPayloadArtifact(resultsDir, result.Audit.RunID, evidence.ID, evidence.ContentHash)
		if err != nil {
			return fmt.Errorf("%s: %w", evidence.ID, err)
		}
		if evidence.PayloadSize != int64(len(raw)) {
			return fmt.Errorf("%s: payload size mismatch", evidence.ID)
		}
	}
	return nil
}

func validHistoricalProfileObservations(historical *agents.EvidenceHistoricalReview, tradeDate time.Time) bool {
	if historical == nil || historical.SampleCount != len(historical.Observations) || historical.SampleCount < historical.MinimumSessions {
		return false
	}
	uniqueDates := make(map[string]bool, len(historical.Observations))
	previous := time.Time{}
	for _, observation := range historical.Observations {
		date, err := time.Parse("2006-01-02", observation.Date)
		if err != nil || date.After(tradeDate) || date.Weekday() == time.Saturday || date.Weekday() == time.Sunday || uniqueDates[observation.Date] || (!previous.IsZero() && !previous.Before(date)) {
			return false
		}
		if observation.Open <= 0 || observation.High <= 0 || observation.Low <= 0 || observation.Close <= 0 || observation.Volume <= 0 || observation.High < observation.Open || observation.High < observation.Close || observation.High < observation.Low || observation.Low > observation.Open || observation.Low > observation.Close {
			return false
		}
		uniqueDates[observation.Date] = true
		previous = date
	}
	return len(uniqueDates) >= historical.MinimumSessions && historical.EndDate == historical.Observations[len(historical.Observations)-1].Date
}

func validPrimaryHistoricalPrice(result *agents.AnalysisResult, fact agents.EvidenceOnlyFact, tradeDate time.Time) bool {
	if result == nil || result.Dossier == nil || result.Dossier.Historical == nil || len(result.Dossier.Historical.Observations) == 0 {
		return false
	}
	latest := result.Dossier.Historical.Observations[len(result.Dossier.Historical.Observations)-1]
	if fact.DataTime != latest.Date || !dataTimeAtOrBeforeTradeDate(latest.Date, tradeDate) {
		return false
	}
	name := "as_of_research_price:" + latest.Date
	for _, calculation := range result.Dossier.Calculations {
		if calculation.Name == name && calculation.Status == "computed" && calculation.Value != nil && *calculation.Value == latest.Close && len(calculation.EvidenceIDs) == 1 && calculation.EvidenceIDs[0] == "tool-historical" {
			return true
		}
	}
	return false
}

func validNewsReviewForProfile(review *agents.EvidenceNewsReview, tradeDate time.Time) bool {
	if review == nil || review.InputCount <= 0 || review.IncludedCount <= 0 || review.IncludedCount != len(review.Items) {
		return false
	}
	for _, item := range review.Items {
		publishedAt, ok := parseLooseDataTime(item.PublishedAt)
		if strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.URL) == "" || strings.TrimSpace(item.Source) == "" || !ok || !publishedAt.Before(tradeDate.AddDate(0, 0, 1)) {
			return false
		}
	}
	return true
}

func factEvidenceCaptured(fact agents.EvidenceOnlyFact, evidenceByID map[string]agents.Evidence) bool {
	if fact.Category == "" || fact.Provider == "" || fact.SourceURL == "" || fact.DataTime == "" || strings.EqualFold(fact.DataTime, "unknown") || len(fact.EvidenceIDs) == 0 {
		return false
	}
	for _, id := range fact.EvidenceIDs {
		evidence, ok := evidenceByID[id]
		if !ok || evidence.Status != "captured" || evidence.ContentHash == "" || evidence.PayloadRef == "" || evidence.DataTime == "" || strings.EqualFold(evidence.DataTime, "unknown") {
			return false
		}
	}
	return true
}

func dataTimeAtOrBeforeTradeDate(value string, tradeDate time.Time) bool {
	observedAt, ok := parseLooseDataTime(value)
	if !ok {
		return false
	}
	return observedAt.Before(tradeDate.AddDate(0, 0, 1))
}

func paperEngineReadiness(db *gorm.DB, scanner *PaperStopScanner, cfg *config.Config, checkedAt time.Time) readinessProfileResponse {
	checks := []systemCheck{
		{ID: "paper.authentication", Status: "fail", Detail: "paper administrator authentication is not configured", Remedy: "Configure STOCKGOD_ADMIN_TOKEN before using authenticated Paper routes.", Severity: "high"},
		{ID: "paper.database", Status: "fail", Detail: "paper SQLite is unavailable", Remedy: "Start the backend with a writable STOCKGOD_DATABASE_DIR and verify SQLite connectivity.", Severity: "high"},
		{ID: "paper.oms_schema", Status: "fail", Detail: "paper OMS schema is unavailable", Remedy: "Run the backend database migrations for Paper account, order, position, checkpoint, scan audit, and audit-chain tables.", Severity: "high"},
		{ID: "paper.audit_chain", Status: "fail", Detail: "paper audit chain is unavailable", Remedy: "Repair the append-only audit chain and resolve pending outcomes before using Paper workflows.", Severity: "high"},
		{ID: "paper.scheduler", Status: "fail", Detail: "paper OCO scheduler is not running", Remedy: "Start the local Paper OCO scanner and resolve any recovery errors.", Severity: "high"},
		{ID: "paper.risk_policy", Status: "fail", Detail: "paper risk policy is invalid", Remedy: "Configure a versioned Paper risk policy with valid stop, daily-loss, drawdown, and stress thresholds.", Severity: "high"},
	}
	if cfg != nil && strings.TrimSpace(cfg.AdminToken) != "" {
		setProfileCheckOK(checks, "paper.authentication", "administrator authentication is configured")
	}
	if db != nil {
		if sqlDB, err := db.DB(); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			err = sqlDB.PingContext(ctx)
			cancel()
			if err == nil {
				setProfileCheckOK(checks, "paper.database", "SQLite connection responds")
			}
		}
		tables := []any{&models.PaperAccount{}, &models.PaperOrder{}, &models.PaperFill{}, &models.PaperPosition{}, &models.PaperEquityCheckpoint{}, &models.PaperDailyEquityBaseline{}, &models.PaperStopScanAudit{}, &models.AuditEvent{}}
		missing := make([]string, 0)
		for _, table := range tables {
			if !db.Migrator().HasTable(table) {
				missing = append(missing, fmt.Sprintf("%T", table))
			}
		}
		if len(missing) == 0 {
			setProfileCheckOK(checks, "paper.oms_schema", "all Paper ledger and audit tables are present")
		} else {
			setProfileCheckDetail(checks, "paper.oms_schema", "missing tables: "+strings.Join(missing, ", "))
		}
		var events []models.AuditEvent
		if err := db.Order("id asc").Find(&events).Error; err == nil {
			valid, brokenAt, reason, pending := verifyAuditEventsWithEvidence(db, events)
			if valid && pending == 0 {
				setProfileCheckOK(checks, "paper.audit_chain", fmt.Sprintf("valid linked audit chain; count=%d pending=0", len(events)))
			} else {
				setProfileCheckDetail(checks, "paper.audit_chain", fmt.Sprintf("valid=%t brokenAt=%d pending=%d reason=%s", valid, brokenAt, pending, reason))
			}
		}
	}
	if scanner != nil {
		status := scanner.Status()
		if status.Running && len(status.RecoveryErrors) == 0 {
			setProfileCheckOK(checks, "paper.scheduler", "running interval="+status.Interval+"; "+status.AuditRetention)
		} else {
			setProfileCheckDetail(checks, "paper.scheduler", fmt.Sprintf("running=%t recoveryErrors=%d", status.Running, len(status.RecoveryErrors)))
		}
	}
	policy := paperRiskPolicyFromConfig(cfg)
	if policy.PolicyVersion != "" && policy.MinStopCoveragePct >= 0 && policy.MinStopCoveragePct <= 100 && policy.MaxDailyLossPct > 0 && policy.MaxDailyLossPct <= 100 && policy.MaxDrawdownPct > 0 && policy.MaxDrawdownPct <= 100 && policy.MaxStressLossPct > 0 && policy.MaxStressLossPct <= 100 {
		setProfileCheckOK(checks, "paper.risk_policy", fmt.Sprintf("version=%s stop>=%.2f%% daily<=%.2f%% drawdown<=%.2f%% stress<=%.2f%%", policy.PolicyVersion, policy.MinStopCoveragePct, policy.MaxDailyLossPct, policy.MaxDrawdownPct, policy.MaxStressLossPct))
	}
	return newReadinessProfile(
		readinessProfilePaperEngine, "Paper engine", []string{"administrator authentication", "SQLite Paper ledger", "OMS schema and CAS persistence", "linked audit chain", "local OCO scheduler", "versioned risk policy"}, checks,
		"Paper-engine readiness is simulation-only. It does not imply live trading data readiness, a broker connection, or a real order/fill.", checkedAt,
	)
}

func liveTradingDataReadiness(status systemStatusResponse) readinessProfileResponse {
	checks := make([]systemCheck, 0, len(status.Endpoints))
	for _, endpoint := range status.Endpoints {
		if !isLiveQuoteEndpoint(endpoint.Endpoint) {
			continue
		}
		check := systemCheck{ID: "live." + strings.TrimPrefix(strings.ReplaceAll(endpoint.Endpoint, "/", "."), "."), Status: "ok", Detail: endpoint.Source + " @ " + firstNonEmpty(endpoint.DataTime, "unknown")}
		if endpoint.Session == "closed" && closedEndpointObservationValid(endpoint, parseCheckedAt(status.CheckedAt)) {
			check.Detail = "market closed; no live stream expected; last provider observation " + endpoint.DataTime
		} else if endpoint.Stale || endpoint.Status != "ok" {
			check.Status = "fail"
			check.Detail = endpoint.Status + ": " + firstNonEmpty(endpoint.StaleReason, "source is not live and complete")
			check.Remedy = liveEndpointRemedy(endpoint)
			check.Severity = "high"
		}
		checks = append(checks, check)
	}
	if len(checks) == 0 {
		checks = append(checks, systemCheck{ID: "live.sources", Status: "fail", Detail: "no live data sources were assessed", Remedy: "Expose and assess the authoritative live market, macro, report, ETF/QDII, and disclosure endpoints.", Severity: "high"})
	}
	sort.SliceStable(checks, func(i, j int) bool { return checks[i].ID < checks[j].ID })
	return newReadinessProfile(
		readinessProfileLiveTradingData, "Live quote transport", []string{"provider-timed US market overview", "provider-timed US stock list", "session-aware CN market quotes"}, checks,
		"This profile covers quote transport only. Macro, reports, ETF/QDII, and disclosure freshness remain visible in system health and are not real-time quote feeds.", parseCheckedAt(status.CheckedAt),
	)
}

func liveTradingDataReadinessWithDirectQuote(status systemStatusResponse, result quoteFetchResult, checkedAt time.Time) readinessProfileResponse {
	base := liveTradingDataReadiness(status)
	checks := append([]systemCheck{}, base.Checks...)
	for index := range checks {
		if checks[index].Status != "ok" {
			checks[index].Status = "warning"
			checks[index].Severity = "medium"
		}
	}
	direct := directQuoteReadinessCheck(result, checkedAt)
	checks = append(checks, direct)
	sort.SliceStable(checks, func(i, j int) bool { return checks[i].ID < checks[j].ID })
	ready := direct.Status == "ok"
	httpStatus := http.StatusOK
	profileStatus := "ok"
	if !ready {
		httpStatus = http.StatusServiceUnavailable
		profileStatus = "degraded"
	}
	remedies := make([]string, 0)
	for _, check := range checks {
		if check.Remedy != "" {
			remedies = append(remedies, check.Remedy)
		}
	}
	return readinessProfileResponse{
		Profile: readinessProfileLiveTradingData, Label: "Core live quotes", Status: profileStatus, Ready: ready, HTTPStatus: httpStatus,
		CheckedAt: checkedAt.UTC().Format(time.RFC3339), Scope: []string{"provider-timed direct AAPL/NVDA quotes"}, Checks: checks,
		Remedies: remedies, Disclaimer: "Core quote readiness is independent from market-wide coverage. US overview/list and CN session coverage remain visible as warnings.",
	}
}

func directQuoteReadinessCheck(result quoteFetchResult, checkedAt time.Time) systemCheck {
	fail := func(detail string) systemCheck {
		return systemCheck{ID: "live.api.quote", Status: "fail", Detail: detail, Remedy: "Restore provider-timed direct quotes for both AAPL and NVDA before enabling quote-dependent workflows.", Severity: "high"}
	}
	if result.timedOut {
		return fail("direct AAPL/NVDA quote provider timed out")
	}
	for _, symbol := range []string{"AAPL", "NVDA"} {
		quote, ok := result.quotes[symbol]
		if !ok || quote.Price <= 0 || strings.TrimSpace(quote.DataTime) == "" || isStaleQuoteSource(quote.Source) {
			return fail("direct quote is missing trusted price provenance for " + symbol)
		}
	}
	allClosed := true
	for _, quote := range result.quotes {
		if quote.Session != "closed" {
			allClosed = false
			break
		}
	}
	if allClosed {
		if result.dataTime.IsZero() || result.dataTime.After(checkedAt.Add(time.Minute)) || checkedAt.Sub(result.dataTime) > 10*24*time.Hour {
			return fail("closed-session direct quotes lack a recent provider observation")
		}
		return systemCheck{ID: "live.api.quote", Status: "ok", Detail: "market closed; direct AAPL/NVDA provider observation " + result.dataTimeLabel}
	}
	if stale, reason := quoteResultFreshness(result, checkedAt); stale {
		return fail(reason)
	}
	return systemCheck{ID: "live.api.quote", Status: "ok", Detail: result.source + " @ " + result.dataTimeLabel}
}

func closedEndpointObservationValid(endpoint systemEndpointStatus, checkedAt time.Time) bool {
	if !strings.HasPrefix(endpoint.Source, "closed-") {
		return false
	}
	observed, ok := parseLooseDataTime(endpoint.DataTime)
	return ok && !observed.After(checkedAt.Add(time.Minute)) && checkedAt.Sub(observed) <= 10*24*time.Hour
}

func liveReadinessFreshness(profile readinessProfileResponse, status systemStatusResponse, result quoteFetchResult) dataFreshnessMeta {
	meta := dataFreshnessMeta{Source: "readiness-profile:" + readinessProfileLiveTradingData, Refreshable: true, Stale: !profile.Ready}
	oldest := result.dataTime
	for _, endpoint := range status.Endpoints {
		if !isLiveQuoteEndpoint(endpoint.Endpoint) {
			continue
		}
		if parsed, ok := parseLooseDataTime(endpoint.DataTime); ok && (oldest.IsZero() || parsed.Before(oldest)) {
			oldest = parsed
		}
	}
	meta.DataTime = dataTimeOrEmpty(oldest)
	if !profile.Ready {
		meta.StaleReason = strings.Join(profile.Remedies, "; ")
	}
	return meta
}

func isLiveQuoteEndpoint(endpoint string) bool {
	switch endpoint {
	case "/api/market", "/api/stocks?market=us", "/api/a-market":
		return true
	default:
		return false
	}
}

func liveEndpointRemedy(endpoint systemEndpointStatus) string {
	if endpoint.Refreshable {
		return "Refresh " + endpoint.Domain + " from its authoritative provider and retain a source data time before relying on it."
	}
	return "Restore a current authoritative source for " + endpoint.Domain + "; a readable snapshot alone is not live readiness."
}

func parseCheckedAt(value string) time.Time {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed
	}
	return time.Now().UTC()
}

func setProfileCheckOK(checks []systemCheck, id, detail string) {
	for index := range checks {
		if checks[index].ID == id {
			checks[index].Status = "ok"
			checks[index].Detail = detail
			checks[index].Remedy = ""
			checks[index].Severity = ""
			return
		}
	}
}

func setProfileCheckDetail(checks []systemCheck, id, detail string) {
	for index := range checks {
		if checks[index].ID == id {
			checks[index].Detail = detail
			return
		}
	}
}
