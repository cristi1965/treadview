package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/agents"
	"trading-agents/internal/config"
	"trading-agents/internal/database"
	"trading-agents/internal/market"
	"trading-agents/internal/models"
)

func TestHistoricalResearchReadinessRequiresOneCompletePITDossier(t *testing.T) {
	now := time.Date(2026, 9, 5, 18, 0, 0, 0, time.UTC)
	root := t.TempDir()
	result, err := buildEvidenceOnlyResult(
		agents.AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock"},
		evidenceOnlyFixtureInvocations(now)[1:], root, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	writeResearchFixture(t, filepath.Join(root, "api_results", "AAPL.json"), result)

	profile := historicalResearchReadiness(&config.Config{ResultsDir: root}, now)
	if !profile.Ready || profile.Status != "ok" || profile.HTTPStatus != http.StatusOK || len(profile.Checks) != 5 {
		t.Fatalf("historical profile not ready: %+v", profile)
	}
	for _, check := range profile.Checks {
		if check.Status != "ok" || check.Remedy != "" {
			t.Fatalf("historical check did not pass cleanly: %+v", check)
		}
	}
	if profileCheckByID(profile.Checks, "research.pit_price").Status != "ok" {
		t.Fatalf("dated historical close was not accepted as the PIT research price: %+v", profile.Checks)
	}

	result.Dossier.FinancialPeriods[0].FilingDate = "2026-09-06"
	writeResearchFixture(t, filepath.Join(root, "api_results", "AAPL.json"), result)
	profile = historicalResearchReadiness(&config.Config{ResultsDir: root}, now)
	if profile.Ready || profile.HTTPStatus != http.StatusServiceUnavailable || profileCheckByID(profile.Checks, "research.filings").Status != "fail" {
		t.Fatalf("post-trade-date filing did not degrade historical profile: %+v", profile)
	}

	result, err = buildEvidenceOnlyResult(
		agents.AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock"},
		evidenceOnlyFixtureInvocations(now)[1:], root, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	delete(result.Dossier.FinancialPeriods[0].FieldEvidence, "totalRevenue")
	writeResearchFixture(t, filepath.Join(root, "api_results", "AAPL.json"), result)
	profile = historicalResearchReadiness(&config.Config{ResultsDir: root}, now)
	if profile.Ready || profileCheckByID(profile.Checks, "research.filings").Status != "fail" {
		t.Fatalf("missing field-level provenance did not degrade historical profile: %+v", profile)
	}
}

func TestHistoricalResearchProfileRejectsDuplicateDatesAndEmptyNews(t *testing.T) {
	now := time.Date(2026, 9, 5, 18, 0, 0, 0, time.UTC)
	result, err := buildEvidenceOnlyResult(
		agents.AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock"},
		evidenceOnlyFixtureInvocations(now), t.TempDir(), now,
	)
	if err != nil {
		t.Fatal(err)
	}
	tradeDate, err := time.Parse("2006-01-02", result.TradeDate)
	if err != nil {
		t.Fatal(err)
	}
	if !validHistoricalProfileObservations(result.Dossier.Historical, tradeDate) || !validNewsReviewForProfile(result.Dossier.NewsReview, tradeDate) {
		t.Fatal("valid evidence-only fixture did not satisfy historical profile inputs")
	}

	historical := *result.Dossier.Historical
	historical.Observations = append([]agents.EvidenceHistoricalObservation(nil), historical.Observations...)
	historical.Observations[1].Date = historical.Observations[0].Date
	if validHistoricalProfileObservations(&historical, tradeDate) {
		t.Fatal("duplicate historical dates were accepted as independent sessions")
	}

	news := *result.Dossier.NewsReview
	news.IncludedCount = 0
	news.Items = nil
	if validNewsReviewForProfile(&news, tradeDate) {
		t.Fatal("empty news review was accepted as reviewed news evidence")
	}
}

func TestHistoricalResearchReadinessRejectsMissingPayloadArtifact(t *testing.T) {
	now := time.Date(2026, 9, 5, 18, 0, 0, 0, time.UTC)
	root := t.TempDir()
	result, err := buildEvidenceOnlyResult(
		agents.AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock"},
		evidenceOnlyFixtureInvocations(now), root, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	writeResearchFixture(t, filepath.Join(root, "api_results", "AAPL.json"), result)
	if err := os.Remove(filepath.Join(root, "evidence", result.Audit.RunID, "tool-historical.payload.gz")); err != nil {
		t.Fatal(err)
	}
	profile := historicalResearchReadiness(&config.Config{ResultsDir: root}, now)
	if profile.Ready || profile.HTTPStatus != http.StatusServiceUnavailable || profileCheckByID(profile.Checks, "research.dossier").Status != "fail" {
		t.Fatalf("missing source artifact still passed readiness: %+v", profile)
	}
}

func TestPaperEngineReadinessChecksDatabaseAuditSchedulerAndPolicy(t *testing.T) {
	db, err := database.OpenSQLite(t.TempDir() + "/paper-readiness.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.PaperAccount{}, &models.PaperOrder{}, &models.PaperFill{}, &models.PaperPosition{},
		&models.PaperEquityCheckpoint{}, &models.PaperDailyEquityBaseline{}, &models.PaperStopScanAudit{}, &models.AuditEvent{},
	); err != nil {
		t.Fatal(err)
	}
	scanner := newPaperStopScanner(db, paperStopScannerOptions{})
	scanner.mu.Lock()
	scanner.running = true
	scanner.mu.Unlock()
	cfg := &config.Config{
		AdminToken:             "secret",
		PaperRiskPolicyVersion: "paper-risk-test", PaperMinStopCoveragePct: 100,
		PaperMaxDailyLossPct: 3, PaperMaxDrawdownPct: 12, PaperMaxStressLossPct: 10,
	}
	profile := paperEngineReadiness(db, scanner, cfg, time.Now())
	if !profile.Ready || profile.HTTPStatus != http.StatusOK {
		t.Fatalf("paper engine profile not ready: %+v", profile)
	}
	if detail := profileCheckByID(profile.Checks, "paper.risk_policy").Detail; !strings.Contains(detail, "paper-risk-test") {
		t.Fatalf("policy version missing from profile: %s", detail)
	}

	withoutAuth := *cfg
	withoutAuth.AdminToken = ""
	profile = paperEngineReadiness(db, scanner, &withoutAuth, time.Now())
	if profile.Ready || profileCheckByID(profile.Checks, "paper.authentication").Status != "fail" {
		t.Fatalf("paper engine passed without configured authentication: %+v", profile)
	}

	scanner.mu.Lock()
	scanner.running = false
	scanner.mu.Unlock()
	profile = paperEngineReadiness(db, scanner, cfg, time.Now())
	if profile.Ready || profile.HTTPStatus != http.StatusServiceUnavailable || profileCheckByID(profile.Checks, "paper.scheduler").Status != "fail" {
		t.Fatalf("stopped scheduler did not degrade profile: %+v", profile)
	}
}

func TestLiveTradingDataReadinessDoesNotInheritResearchOrPaperHealth(t *testing.T) {
	status := systemStatusResponse{
		Status: "degraded", CheckedAt: "2026-09-07T10:00:00Z",
		Endpoints: []systemEndpointStatus{
			{Domain: "US行情", Endpoint: "/api/market", Source: "tradingview", DataTime: "2026-09-07T09:59:00Z", Status: "ok"},
			{Domain: "宏观", Endpoint: "/api/macro", Source: "stale-snapshot:macro", DataTime: "2026-09-05T00:00:00Z", Stale: true, Status: "stale", StaleReason: "snapshot expired"},
			{Domain: "Notes", Endpoint: "/api/notes/*", Source: "local-notes", Status: "local-authored"},
		},
	}
	profile := liveTradingDataReadiness(status)
	if !profile.Ready || profile.HTTPStatus != http.StatusOK || len(profile.Checks) != 1 || len(profile.Remedies) != 0 {
		t.Fatalf("non-trading research data incorrectly degraded the live quote profile: %+v", profile)
	}
	if !strings.Contains(profile.Disclaimer, "quote transport only") {
		t.Fatalf("cross-profile boundary is not explicit: %s", profile.Disclaimer)
	}
}

func TestLiveTradingDataReadinessFailsForQuoteEndpointsOnly(t *testing.T) {
	status := systemStatusResponse{
		Status: "degraded", CheckedAt: "2026-09-08T14:00:00Z",
		Endpoints: []systemEndpointStatus{
			{Domain: "US行情", Endpoint: "/api/market", Source: "stale-snapshot:us", Status: "stale", Stale: true, StaleReason: "quote expired"},
			{Domain: "盘前异动", Endpoint: "/api/premarket-movers", Source: "stale-snapshot:movers", Status: "stale", Stale: true},
			{Domain: "报告", Endpoint: "/api/reports", Source: "stale-snapshot:reports", Status: "stale", Stale: true},
		},
	}
	profile := liveTradingDataReadiness(status)
	if profile.Ready || profile.HTTPStatus != http.StatusServiceUnavailable || len(profile.Checks) != 1 {
		t.Fatalf("quote degradation was not isolated correctly: %+v", profile)
	}
	if got := profile.Checks[0].ID; got != "live.api.market" {
		t.Fatalf("unexpected live quote check %q", got)
	}
}

func TestDirectQuoteReadinessRequiresBothFreshProviderTimedSymbols(t *testing.T) {
	now := time.Date(2026, 9, 8, 14, 0, 0, 0, time.UTC)
	fresh := quoteFetchResult{
		quotes: map[string]market.Quote{
			"AAPL": {Price: 220, Source: "yahoo-chart", DataTime: now.Add(-20 * time.Second).Format(time.RFC3339), Session: "regular"},
			"NVDA": {Price: 180, Source: "yahoo-chart", DataTime: now.Add(-15 * time.Second).Format(time.RFC3339), Session: "regular"},
		},
		source: "yahoo-chart", dataTime: now.Add(-20 * time.Second), dataTimeLabel: now.Add(-20 * time.Second).Format(time.RFC3339),
	}
	if check := directQuoteReadinessCheck(fresh, now); check.Status != "ok" {
		t.Fatalf("fresh direct quote probe failed: %+v", check)
	}
	fresh.quotes["NVDA"] = market.Quote{}
	if check := directQuoteReadinessCheck(fresh, now); check.Status != "fail" {
		t.Fatalf("missing direct quote provenance passed: %+v", check)
	}
}

func TestLiveTradingDataReadinessSeparatesCoreQuotesFromCoverageWarnings(t *testing.T) {
	now := time.Date(2026, 9, 8, 14, 0, 0, 0, time.UTC)
	status := systemStatusResponse{CheckedAt: now.Format(time.RFC3339), Endpoints: []systemEndpointStatus{
		{Domain: "US行情", Endpoint: "/api/market", Source: "stale-snapshot:us", Status: "stale", Stale: true, StaleReason: "partial coverage"},
		{Domain: "A股行情", Endpoint: "/api/a-market", Source: "stale-snapshot:cn", Status: "stale", Stale: true, StaleReason: "market closed", Session: "closed"},
	}}
	quotes := quoteFetchResult{
		quotes: map[string]market.Quote{
			"AAPL": {Price: 220, Source: "nasdaq", DataTime: now.Add(-20 * time.Second).Format(time.RFC3339), Session: "regular"},
			"NVDA": {Price: 180, Source: "nasdaq", DataTime: now.Add(-15 * time.Second).Format(time.RFC3339), Session: "regular"},
		},
		source: "nasdaq", dataTime: now.Add(-20 * time.Second), dataTimeLabel: now.Add(-20 * time.Second).Format(time.RFC3339),
	}
	profile := liveTradingDataReadinessWithDirectQuote(status, quotes, now)
	if !profile.Ready || profile.HTTPStatus != http.StatusOK || profile.Status != "ok" {
		t.Fatalf("fresh core quotes should pass independently: %+v", profile)
	}
	if check := profileCheckByID(profile.Checks, "live.api.market"); check.Status != "warning" {
		t.Fatalf("market-wide degradation should remain visible as warning: %+v", check)
	}
}

func TestClosedEndpointRequiresExplicitRecentProviderObservation(t *testing.T) {
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	valid := systemEndpointStatus{Source: "closed-eastmoney-cn@2026-09-08T07:00:00Z", DataTime: "2026-09-08T07:00:00Z", Session: "closed"}
	if !closedEndpointObservationValid(valid, now) {
		t.Fatal("recent provider-timed closed observation rejected")
	}
	valid.Source = "stale-snapshot:cn@2026-09-08T07:00:00Z"
	if closedEndpointObservationValid(valid, now) {
		t.Fatal("unverified closed snapshot passed")
	}
}

func TestReadinessProfileQueryUsesIndependentHTTPStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &Handler{config: &config.Config{ResultsDir: t.TempDir()}}
	router := gin.New()
	router.GET("/api/readiness", handler.GetDataReadiness)

	degraded := httptest.NewRecorder()
	router.ServeHTTP(degraded, httptest.NewRequest(http.MethodGet, "/api/readiness?profile=historical-research", nil))
	if degraded.Code != http.StatusServiceUnavailable || degraded.Header().Get("X-Readiness-Profile") != readinessProfileHistoricalResearch || !strings.Contains(degraded.Body.String(), `"profile":"historical-research"`) {
		t.Fatalf("profile HTTP semantics missing: status=%d headers=%v body=%s", degraded.Code, degraded.Header(), degraded.Body.String())
	}

	unknown := httptest.NewRecorder()
	router.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/api/readiness?profile=not-real", nil))
	if unknown.Code != http.StatusBadRequest || !strings.Contains(unknown.Body.String(), readinessProfileFull) || !strings.Contains(unknown.Body.String(), readinessProfileLiveTradingData) {
		t.Fatalf("unknown profile contract missing: status=%d body=%s", unknown.Code, unknown.Body.String())
	}
}

func profileCheckByID(checks []systemCheck, id string) systemCheck {
	for _, check := range checks {
		if check.ID == id {
			return check
		}
	}
	return systemCheck{}
}
