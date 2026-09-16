package api

import (
	"encoding/json"
	"fmt"
	"math"
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
	"trading-agents/internal/dataflows"
	"trading-agents/internal/market"
	"trading-agents/internal/scoring"
)

func TestEmptyAnalysisHistoryIsNotPublished(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &Handler{config: &config.Config{ResultsDir: t.TempDir()}}
	router := gin.New()
	router.GET("/api/analysis/history", handler.GetHistory)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/analysis/history", nil))
	if w.Code != http.StatusOK || w.Header().Get("X-Research-Status") != "none" || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty history status=%d research=%q body=%s", w.Code, w.Header().Get("X-Research-Status"), w.Body.String())
	}
}

func TestHistoryMergesEqualSizedMemoryAndDiskRunsByRunID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	disk := &agents.AnalysisResult{Ticker: "AAPL", TradeDate: "2026-09-05", CompletedAt: "2026-09-05T18:00:00Z", Audit: agents.AnalysisAudit{RunID: "run-disk"}}
	writeResearchFixture(t, filepath.Join(root, "api_results", "AAPL.json"), disk)
	handler := &Handler{config: &config.Config{ResultsDir: root}, hub: NewHub(), results: []agents.AnalysisResult{
		{Ticker: "NVDA", TradeDate: "2026-09-06", CompletedAt: "2026-09-06T18:00:00Z", Audit: agents.AnalysisAudit{RunID: "run-memory"}},
	}}
	router := gin.New()
	router.GET("/history", handler.GetHistory)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/history", nil))
	var history []agents.AnalysisResult
	if err := json.Unmarshal(w.Body.Bytes(), &history); err != nil || len(history) != 2 {
		t.Fatalf("history did not merge equal-sized sources: err=%v body=%s", err, w.Body.String())
	}
	if history[0].Audit.RunID != "run-memory" || history[1].Audit.RunID != "run-disk" {
		t.Fatalf("merged history order/content=%+v", history)
	}
}

func TestHistoryAvailableRecordIsNotDegradedByFailedArchive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 9, 5, 18, 0, 0, 0, time.UTC)
	available, err := buildEvidenceOnlyResult(agents.AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock"}, evidenceOnlyFixtureInvocations(now), t.TempDir(), now)
	if err != nil {
		t.Fatal(err)
	}
	handler := &Handler{config: &config.Config{ResultsDir: t.TempDir()}, results: []agents.AnalysisResult{
		{Ticker: "OLD", CompletedAt: "2026-08-01T00:00:00Z", Status: agents.ResearchStatusUnavailable},
		*available,
	}}
	router := gin.New()
	router.GET("/history", handler.GetHistory)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/history", nil))
	if w.Code != http.StatusOK || w.Header().Get("X-Research-Status") != agents.ResearchStatusEvidenceOnly {
		t.Fatalf("failed archive polluted current status: code=%d status=%q body=%s", w.Code, w.Header().Get("X-Research-Status"), w.Body.String())
	}
	if got := w.Header().Get("X-Research-Unavailable-Count"); got != "1" {
		t.Fatalf("failed archive count missing: %q", got)
	}
}

func TestEvidenceOnlyDossierPersistsMeasuredDuration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 9, 5, 18, 0, 0, 0, time.UTC)
	root := t.TempDir()
	handler := &Handler{
		config: &config.Config{ResultsDir: root},
		hub:    NewHub(),
		evidenceOnlyBuild: func(req agents.AnalysisRequest) (*agents.AnalysisResult, dataflows.ResearchPreflight, error) {
			time.Sleep(2 * time.Millisecond)
			result, err := buildEvidenceOnlyResult(req, evidenceOnlyFixtureInvocations(now), root, now)
			return result, dataflows.ResearchPreflight{Passed: err == nil}, err
		},
	}
	router := gin.New()
	router.POST("/evidence-only", handler.CreateEvidenceOnlyDossier)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/evidence-only", strings.NewReader(`{"ticker":"AAPL","trade_date":"2026-09-05","asset_type":"stock"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var result agents.AnalysisResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.DurationSecs <= 0 {
		t.Fatalf("duration was not measured: err=%v result=%+v", err, result)
	}
	loaded := handler.loadResultsRaw()
	if len(loaded) != 1 || loaded[0].DurationSecs <= 0 {
		t.Fatalf("measured duration was not persisted: %+v", loaded)
	}
}

func TestPanelForSymbolEnrichesLegacyPanelWithReproducibleDetail(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "backend")
	row := scoring.UsStockRow{Sym: "AAPL", Name: "Apple", Price: 200, Pct: 1.5, McapB: 3000, Sector: "Technology", Industry: "Consumer Electronics", Vol: 50_000_000}
	universe := scoring.UsStocksFile{Count: 1, Stocks: []scoring.UsStockRow{row}}
	writeResearchFixture(t, filepath.Join(base, "frontend", "public", "data", "us-stocks.json"), universe)
	scores := scoring.ExplainFive(row, true).FinalScores
	panel := &scoring.PanelFile{Order: scoring.PanelOrder, Source: "local-heuristic-v2-deterministic-scaling", Stocks: map[string]scoring.PanelStock{
		"AAPL": {SC: scores, Div: scoring.Divergence(scores)},
	}}

	got := panelForSymbol(panel, root, "aapl")
	if got.Count != 1 || got.MethodVersion != scoring.HeuristicMethodVersion || got.Stocks["AAPL"].Detail == nil {
		t.Fatalf("missing symbol evidence detail: %+v", got)
	}
	if got.Validation.Status != "unvalidated" {
		t.Fatalf("legacy panel must explicitly remain unvalidated: %+v", got.Validation)
	}
	if !got.Stocks["AAPL"].Detail.ReproducesPanel {
		t.Fatalf("expected reproducible detail: %+v", got.Stocks["AAPL"].Detail)
	}
}

func TestResearchEvidencePayloadRequiresAdminAndVerifiesHash(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	cfg := &config.Config{ResultsDir: root, AdminToken: "secret"}
	payload := "complete captured source\n2026-09-05,100"
	hash := agents.ContentHash(payload)
	invocation := dataflows.ToolInvocation{RawPayload: payload, ContentHash: hash}
	ref, size, err := dataflows.PersistPayloadArtifact(root, "run-1", "tool-1", invocation)
	if err != nil {
		t.Fatal(err)
	}
	result := agents.AnalysisResult{Ticker: "AAPL", TradeDate: "2026-09-05", Audit: agents.AnalysisAudit{
		RunID: "run-1", Evidence: []agents.Evidence{{ID: "tool-1", ContentHash: hash, PayloadRef: ref, PayloadSize: size}},
	}}
	writeResearchFixture(t, filepath.Join(root, "api_results", "AAPL.json"), result)
	handler := &Handler{config: cfg}
	router := gin.New()
	router.GET("/evidence/:runID/:evidenceID", adminReadGuard(cfg), handler.GetResearchEvidencePayload)

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/evidence/run-1/tool-1", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorized.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/evidence/run-1/tool-1", nil)
	request.Header.Set("Authorization", "Bearer secret")
	authorized := httptest.NewRecorder()
	router.ServeHTTP(authorized, request)
	if authorized.Code != http.StatusOK || authorized.Body.String() != payload || authorized.Header().Get("X-Content-Hash") != hash {
		t.Fatalf("status=%d hash=%s body=%q", authorized.Code, authorized.Header().Get("X-Content-Hash"), authorized.Body.String())
	}
}

func TestSaveResultUsesRunIDToAvoidSameDayOverwrite(t *testing.T) {
	handler := &Handler{config: &config.Config{ResultsDir: t.TempDir()}}
	for _, runID := range []string{"run-one", "run-two"} {
		result := &agents.AnalysisResult{Ticker: "AAPL", TradeDate: "2026-09-06", Audit: agents.AnalysisAudit{RunID: runID}}
		if err := handler.saveResult(result); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(handler.config.ResultsDir, "api_results"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("expected two immutable run files, entries=%d err=%v", len(entries), err)
	}
}

func TestEvidenceOnlyDossierPersistsAndIsNotPublishedAsAgentResearch(t *testing.T) {
	now := time.Date(2026, 9, 5, 18, 0, 0, 0, time.UTC)
	invocations := evidenceOnlyFixtureInvocations(now)
	root := t.TempDir()
	result, err := buildEvidenceOnlyResult(agents.AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock"}, invocations, root, now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != agents.ResearchStatusEvidenceOnly || result.ResearchMode != agents.ResearchModeEvidenceOnly || result.Decision != agents.ResearchDecisionObserve {
		t.Fatalf("dossier status was confused with agent research: %+v", result)
	}
	if result.Dossier == nil || len(result.Dossier.Facts) != 4 || len(result.Dossier.FinancialPeriods) != 8 || result.Dossier.Historical == nil || result.Dossier.NewsReview == nil || result.Dossier.MarketSubmodel == nil || result.Audit.ModelProvider != "none" || len(result.Audit.Models) != 0 {
		t.Fatalf("dossier disclosure or no-LLM metadata missing: %+v", result)
	}
	if result.Dossier.MarketSubmodel.Status != "evaluated" || result.Dossier.MarketSubmodel.TestSamples != 30 || result.Dossier.MarketSubmodel.DatasetHash == "" || result.Dossier.MarketSubmodel.FiveFactorUse {
		t.Fatalf("market-only PIT validation is missing or confused with panel validation: %+v", result.Dossier.MarketSubmodel)
	}
	if result.Dossier.MarketSubmodel.Classification != "experimental_negative_control" || result.Dossier.MarketSubmodel.ProductSignal || result.Dossier.RiskDiagnostics == nil || result.Dossier.RiskDiagnostics.Status != "complete" {
		t.Fatalf("failed model was not separated from product risk diagnostics: model=%+v risk=%+v", result.Dossier.MarketSubmodel, result.Dossier.RiskDiagnostics)
	}
	if result.Dossier.FinancialTrendSummary == nil || result.Dossier.FinancialTrendSummary.Status != "complete" || len(result.Dossier.FinancialTrendSummary.Metrics) != 6 {
		t.Fatalf("annual and quarterly trend summary is incomplete: %+v", result.Dossier.FinancialTrendSummary)
	}
	for _, name := range []string{"gross_margin:2026-06-30", "revenue_qoq:2026-06-30", "return_20d", "annualized_volatility", "maximum_drawdown", "average_daily_volume_20d"} {
		if calculation := findEvidenceCalculation(result.Dossier.Calculations, name); calculation == nil || calculation.Status != "computed" || calculation.Value == nil || calculation.Formula == "" || len(calculation.Inputs) == 0 || len(calculation.EvidenceIDs) == 0 {
			t.Fatalf("calculation %s is not reproducible: %+v", name, calculation)
		}
	}
	if calculation := findEvidenceCalculation(result.Dossier.Calculations, "revenue_yoy:2026-06-30"); calculation == nil || calculation.Status != "computed" || calculation.Value == nil || math.Abs(*calculation.Value-50) > 1e-9 {
		t.Fatalf("comparable proven YoY was not computed: %+v", calculation)
	}
	for _, id := range []string{"tool-historical", "tool-fundamentals", "tool-news", "tool-quote-cross-check"} {
		if _, err := os.Stat(filepath.Join(root, "evidence", result.Audit.RunID, id+".payload.gz")); err != nil {
			t.Fatalf("missing persisted %s payload: %v", id, err)
		}
	}

	handler := &Handler{config: &config.Config{ResultsDir: root}, hub: NewHub()}
	if err := handler.saveResult(result); err != nil {
		t.Fatal(err)
	}
	loaded := handler.loadResults()
	if len(loaded) != 1 || loaded[0].Status != agents.ResearchStatusEvidenceOnly {
		t.Fatalf("history did not retain evidence-only status: %+v", loaded)
	}
	if got := researchEvidenceCheck(handler.config); got.Status != "ok" || !strings.Contains(got.Detail, "evidence-only dossiers=1") {
		t.Fatalf("readiness did not recognize dossier: %+v", got)
	}
	router := gin.New()
	router.GET("/history", handler.GetHistory)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/history", nil))
	if w.Code != http.StatusOK || w.Header().Get("X-Research-Status") != agents.ResearchStatusEvidenceOnly {
		t.Fatalf("history status=%d research=%q body=%s", w.Code, w.Header().Get("X-Research-Status"), w.Body.String())
	}
	var history []agents.AnalysisResult
	if err := json.Unmarshal(w.Body.Bytes(), &history); err != nil || len(history) != 1 || history[0].Dossier == nil || len(history[0].Dossier.Facts) != 4 {
		t.Fatalf("history omitted evidence-only dossier: err=%v body=%s", err, w.Body.String())
	}

	// A legacy/invalid dossier must keep its reviewable evidence in history,
	// while its action conclusion remains withdrawn by the publication gate.
	result.Dossier.ConclusionBasis = nil
	legacyRoot := t.TempDir()
	writeResearchFixture(t, filepath.Join(legacyRoot, "api_results", "AAPL_legacy-invalid.json"), result)
	legacyHandler := &Handler{config: &config.Config{ResultsDir: legacyRoot}, hub: NewHub()}
	legacyRouter := gin.New()
	legacyRouter.GET("/history", legacyHandler.GetHistory)
	w = httptest.NewRecorder()
	legacyRouter.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/history", nil))
	if err := json.Unmarshal(w.Body.Bytes(), &history); err != nil || len(history) != 1 || history[0].Status != agents.ResearchStatusUnavailable || history[0].Decision != agents.ResearchDecisionUnavailable || history[0].Dossier == nil {
		t.Fatalf("history did not preserve failed dossier evidence safely: err=%v body=%s", err, w.Body.String())
	}
}

func TestEvidenceOnlyRejectsShallowFilingHistoryBeforePublication(t *testing.T) {
	now := time.Date(2026, 9, 5, 18, 0, 0, 0, time.UTC)
	invocations := evidenceOnlyFixtureInvocations(now)
	periods := financialFixturePeriods()[:2]
	fundamentals := dataflows.FundamentalData{
		Source: "sec-edgar-companyfacts", FiscalPeriod: periods[0].FiscalPeriod,
		AsOf: periods[0].PeriodEnd, FilingDate: periods[0].FilingDate,
		Accession: periods[0].Accession, SourceURL: periods[0].SourceURL, Periods: periods,
	}
	invocations[1].RawPayload = dataflows.FormatFundamentalsForEvidence(&fundamentals, "AAPL")
	invocations[1].ContentHash = agents.ContentHash(invocations[1].RawPayload)
	invocations[1].PayloadExcerpt = invocations[1].RawPayload
	invocations[1].PayloadSize = int64(len(invocations[1].RawPayload))
	invocations[1].DisclosureTimes = financialFixtureDates(periods, true)

	_, err := buildEvidenceOnlyResult(agents.AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock"}, invocations, t.TempDir(), now)
	if err == nil || !strings.Contains(err.Error(), "five quarterly and three annual") {
		t.Fatalf("shallow filing history was not rejected: %v", err)
	}
}

func findEvidenceCalculation(calculations []agents.EvidenceOnlyCalculation, name string) *agents.EvidenceOnlyCalculation {
	for index := range calculations {
		if calculations[index].Name == name {
			return &calculations[index]
		}
	}
	return nil
}

func TestFinancialCalculationDoesNotTreatMissingFieldAsZero(t *testing.T) {
	revenue := 100.0
	period := agents.EvidenceFinancialPeriod{PeriodEnd: "2026-06-30", TotalRevenue: &revenue, AvailableFields: []string{"totalRevenue"}}
	calculation := financialEvidenceCalculations([]agents.EvidenceFinancialPeriod{period}, nil, nil)[0]
	if calculation.Name != "gross_margin:2026-06-30" || calculation.Status != "unknown" || calculation.Value != nil {
		t.Fatalf("missing gross profit was presented as a zero margin: %+v", calculation)
	}
	if _, present := calculation.Inputs["gross_profit"]; present {
		t.Fatalf("missing field leaked as a zero-valued calculation input: %+v", calculation)
	}
}

func TestFinancialCalculationRejectsYTDAsQuarter(t *testing.T) {
	period := financialFixturePeriod("FY2026-Q3", "quarterly", "10-Q", "2025-10-01", "2026-06-30", "2026-08-01", 330, 90)
	converted, err := evidenceFinancialPeriods([]dataflows.FinancialPeriod{
		period,
		financialFixturePeriod("FY2026-Q2", "quarterly", "10-Q", "2026-01-01", "2026-03-31", "2026-05-01", 110, 22),
	})
	if err != nil {
		t.Fatal(err)
	}
	if converted[0].TotalRevenue != nil || converted[0].NetIncome != nil || containsFinancialField(converted[0].AvailableFields, "totalRevenue") || containsFinancialField(converted[0].AvailableFields, "netIncome") {
		t.Fatalf("YTD values survived the discrete-quarter field gate: %+v", converted[0])
	}
	calculation := findEvidenceCalculation(financialEvidenceCalculations(converted, nil, nil), "revenue_qoq:2026-06-30")
	if calculation == nil || calculation.Status != "unknown" || calculation.Value != nil {
		t.Fatalf("YTD value was used in QoQ: %+v", calculation)
	}
}

func containsFinancialField(fields []string, wanted string) bool {
	for _, field := range fields {
		if field == wanted {
			return true
		}
	}
	return false
}

func TestFinancialCalculationsRequireSameFrequencyAndComputeTTM(t *testing.T) {
	periods := []agents.EvidenceFinancialPeriod{
		agentFinancialFixturePeriod("FY2026-Q3", "2026-04-01", "2026-06-30", 120, 24),
		agentFinancialFixturePeriod("FY2026-Q2", "2026-01-01", "2026-03-31", 110, 22),
		agentFinancialFixturePeriod("FY2026-Q1", "2025-10-01", "2025-12-31", 100, 20),
		agentFinancialFixturePeriod("FY2025-Q4", "2025-07-01", "2025-09-30", 90, 18),
		agentFinancialFixturePeriod("FY2025-Q3", "2025-04-01", "2025-06-30", 80, 16),
	}
	calculations := financialEvidenceCalculations(periods, nil, nil)
	for name, expected := range map[string]float64{"ttm_revenue": 420, "ttm_net_income": 84, "revenue_yoy:2026-06-30": 50, "net_income_yoy:2026-06-30": 50} {
		calculation := findEvidenceCalculation(calculations, name)
		if calculation == nil || calculation.Status != "computed" || calculation.Value == nil || math.Abs(*calculation.Value-expected) > 1e-9 {
			t.Fatalf("%s calculation=%+v expected=%v", name, calculation, expected)
		}
	}
}

func TestFinancialCalculationsDeriveQ4TTMAndFilingBoundValuation(t *testing.T) {
	latest := financialFixturePeriod("misleading-filing-context", "quarterly", "10-Q", "2026-04-01", "2026-06-30", "2026-08-01", 130, 26)
	latest.SharesOutstanding = 10
	latest.AvailableFields = append(latest.AvailableFields, "sharesOutstanding")
	latest.FieldEvidence["sharesOutstanding"] = dataflows.FinancialFieldEvidence{
		Source: "sec-edgar-companyfacts", URL: latest.SourceURL, Unit: "shares", PeriodStart: "2026-07-15", PeriodEnd: "2026-07-15",
		Form: latest.Form, Filed: latest.FilingDate, Accession: latest.Accession, PeriodKind: "instant",
	}
	rawPeriods := []dataflows.FinancialPeriod{
		latest,
		financialFixturePeriod("wrong", "quarterly", "10-Q", "2026-01-01", "2026-03-31", "2026-05-01", 120, 24),
		financialFixturePeriod("wrong", "quarterly", "10-Q", "2025-10-01", "2025-12-31", "2026-02-01", 110, 22),
		financialFixturePeriod("wrong", "annual", "10-K", "2024-10-01", "2025-09-30", "2025-11-01", 400, 80),
	}
	periods, err := evidenceFinancialPeriods(rawPeriods)
	if err != nil {
		t.Fatal(err)
	}
	support, err := evidenceFinancialDerivationInputs([]dataflows.FinancialPeriod{
		financialFixturePeriod("wrong", "year-to-date", "10-Q", "2024-10-01", "2025-06-30", "2025-08-01", 300, 60),
	})
	if err != nil {
		t.Fatal(err)
	}
	price := 20.0
	calculations := financialEvidenceCalculations(periods, support, &price)
	for name, expected := range map[string]float64{
		"derived_fiscal_q4_revenue": 100, "derived_fiscal_q4_net_income": 20,
		"ttm_revenue": 460, "ttm_net_income": 92, "market_cap": 200,
		"price_to_sales": 200.0 / 460.0, "price_to_book": 200.0 / (130.0 * 1.6),
	} {
		calculation := findEvidenceCalculation(calculations, name)
		if calculation == nil || calculation.Status != "computed" || calculation.Value == nil || math.Abs(*calculation.Value-expected) > 1e-9 || len(calculation.Inputs) == 0 {
			t.Fatalf("%s was not reproducibly derived: %+v", name, calculation)
		}
	}
	if periods[0].FiscalPeriod != "quarterly:2026-04-01/2026-06-30" || support[0].FiscalPeriod != "year-to-date:2024-10-01/2025-06-30" {
		t.Fatalf("filing-context labels survived stable period identity conversion: periods=%+v support=%+v", periods, support)
	}
}

func TestEvidenceOnlyFundamentalFactSummaryCountsPayloadPeriods(t *testing.T) {
	fundamentals := dataflows.FundamentalData{
		Source: "sec-edgar-companyfacts", FiscalPeriod: "quarterly:2026-04-01/2026-06-30", AsOf: "2026-06-30",
		FilingDate: "2026-08-01", Accession: "fixture", SourceURL: "https://www.sec.gov/Archives/fixture", Periods: financialFixturePeriods(),
	}
	invocation := dataflows.ToolInvocation{
		RawPayload: dataflows.FormatFundamentalsForEvidence(&fundamentals, "AAPL"), DisclosureTime: fundamentals.FilingDate,
		DisclosureTimes: []string{"2026-08-01", "2026-05-01", "2026-02-01", "2025-11-01"},
	}
	if summary := evidenceOnlyFactSummary("fundamentals", invocation); !strings.Contains(summary, "Captured 8 filing-bound financial periods") {
		t.Fatalf("fact summary counted filing dates instead of payload periods: %q", summary)
	}
}

func TestEvidenceOnlyDossierUsesParsedFilingPeriodsInsteadOfInvocationCount(t *testing.T) {
	now := time.Date(2026, 9, 5, 18, 0, 0, 0, time.UTC)
	invocations := evidenceOnlyFixtureInvocations(now)
	invocations[1].DisclosureTimes = []string{"2026-08-01"}
	invocations[1].ObservationTimes = []string{"2026-06-30"}
	if _, err := buildEvidenceOnlyResult(agents.AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock"}, invocations, t.TempDir(), now); err != nil {
		t.Fatalf("complete parsed filing history was rejected because invocation metadata was shallow: %v", err)
	}
}

func TestEvidenceOnlyResearchContextIsAuditedWithoutInference(t *testing.T) {
	now := time.Date(2026, 9, 5, 18, 0, 0, 0, time.UTC)
	weight := 2.5
	context := &agents.ResearchContext{
		Mandate: "US large-cap quality", Holdings: []agents.ResearchHolding{{Symbol: "AAPL", WeightPct: &weight}},
		Liquidity: "T+2, daily liquidity required", Tax: "taxable account", RiskBudget: "max 5% issuer weight",
	}
	withContext, err := buildEvidenceOnlyResult(agents.AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock", ResearchContext: context}, evidenceOnlyFixtureInvocations(now), t.TempDir(), now)
	if err != nil {
		t.Fatal(err)
	}
	withoutContext := agents.NewEvidenceOnlyAudit(agents.AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock"}, now)
	if withContext.Dossier.ResearchContext == nil || withContext.Dossier.ResearchContext.Mandate != context.Mandate || withContext.Audit.Inputs["research_context"] == "" || withContext.Audit.Inputs["research_context"] == "null" || withContext.Audit.InputHash == withoutContext.InputHash {
		t.Fatalf("research context was not preserved in dossier and input hash: %+v", withContext)
	}
	for _, fragment := range []string{"mandate was not supplied", "Holdings were not supplied", "Liquidity requirements were not supplied", "Tax context was not supplied", "Risk budget was not supplied"} {
		if containsEvidenceGap(withContext.Dossier.Gaps, fragment) {
			t.Fatalf("supplied context was reported missing: %s gaps=%+v", fragment, withContext.Dossier.Gaps)
		}
	}
}

func TestEvidenceOnlyStructuredContextAndScenarioAreAuditable(t *testing.T) {
	now := time.Date(2026, 9, 5, 18, 0, 0, 0, time.UTC)
	invocations := evidenceOnlyFixtureInvocations(now)
	fundamentals, err := dataflows.ParseFundamentalEvidence(invocations[1].RawPayload)
	if err != nil {
		t.Fatal(err)
	}
	shares := 10.0
	fundamentals.Periods[0].SharesOutstanding = shares
	fundamentals.Periods[0].AvailableFields = append(fundamentals.Periods[0].AvailableFields, "sharesOutstanding")
	fundamentals.Periods[0].FieldEvidence["sharesOutstanding"] = dataflows.FinancialFieldEvidence{
		Source: "sec-edgar-companyfacts", URL: fundamentals.Periods[0].SourceURL, Unit: "shares",
		PeriodStart: "2026-07-15", PeriodEnd: "2026-07-15", Form: fundamentals.Periods[0].Form,
		Filed: fundamentals.Periods[0].FilingDate, Accession: fundamentals.Periods[0].Accession, PeriodKind: "instant",
	}
	invocations[1].RawPayload = dataflows.FormatFundamentalsForEvidence(&fundamentals, "AAPL")
	invocations[1].ContentHash = agents.ContentHash(invocations[1].RawPayload)
	invocations[1].PayloadExcerpt = invocations[1].RawPayload
	invocations[1].PayloadSize = int64(len(invocations[1].RawPayload))
	weight, capPct, growth, multiple := 2.5, 5.0, 10.0, 2.0
	context := &agents.ResearchContext{
		Mandate: "US large-cap quality", Holdings: []agents.ResearchHolding{{Symbol: "aapl", WeightPct: &weight}},
		Liquidity: "daily", Tax: "taxable", RiskBudget: "review only", IssuerCapPct: &capPct,
		Scenario: &agents.ResearchScenario{RevenueGrowthPct: &growth, PSMultiple: &multiple},
	}
	result, err := buildEvidenceOnlyResult(agents.AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock", ResearchContext: context}, invocations, t.TempDir(), now)
	if err != nil {
		t.Fatal(err)
	}
	assessment := result.Dossier.ContextAssessment
	if assessment == nil || assessment.Holding.Status != "provided" || assessment.IssuerCap.Status != "provided" || assessment.IssuerCap.HeadroomPct == nil || *assessment.IssuerCap.HeadroomPct != 2.5 || assessment.IssuerCap.WithinCap == nil || !*assessment.IssuerCap.WithinCap {
		t.Fatalf("structured issuer headroom was not computed from explicit inputs: %+v", assessment)
	}
	if assessment.Scenario.Status != "provided" || assessment.Scenario.BaseTTMRevenue == nil || math.Abs(*assessment.Scenario.BaseTTMRevenue-420) > 1e-9 || assessment.Scenario.ImpliedRevenue == nil || math.Abs(*assessment.Scenario.ImpliedRevenue-462) > 1e-9 || assessment.Scenario.ImpliedMarketCap == nil || math.Abs(*assessment.Scenario.ImpliedMarketCap-924) > 1e-9 || assessment.Scenario.ImpliedPrice == nil || math.Abs(*assessment.Scenario.ImpliedPrice-92.4) > 1e-9 || assessment.Scenario.ChangePct == nil {
		t.Fatalf("user scenario was not reproducibly assessed: %+v", assessment.Scenario)
	}
	for _, name := range []string{"scenario_revenue", "scenario_implied_market_cap", "scenario_implied_price", "scenario_change_pct"} {
		calculation := findEvidenceCalculation(result.Dossier.Calculations, name)
		if calculation == nil || calculation.Status != "computed" || calculation.Value == nil || len(calculation.Inputs) == 0 || calculation.InputSources["user_revenue_growth_pct"] == "" && calculation.InputSources["user_ps_multiple"] == "" {
			t.Fatalf("scenario calculation lacks formula inputs or assumption provenance: %s %+v", name, calculation)
		}
	}
	contextEvidence := false
	for _, evidence := range result.Audit.Evidence {
		if evidence.ID == "input-research-context" && evidence.ContentHash != "" && evidence.PayloadExcerpt != "" && evidence.DataTime == now.Format(time.RFC3339) && evidence.Inputs["requested_trade_date"] == "2026-09-05" {
			contextEvidence = true
		}
	}
	if !contextEvidence || !strings.Contains(strings.ToLower(assessment.Scenario.Disclosure), "not a forecast") {
		t.Fatalf("context evidence or scenario disclosure missing: audit=%+v assessment=%+v", result.Audit, assessment)
	}
}

func TestEvidenceOnlyScenarioStaysUnknownWhenEvidenceOrAssumptionIsMissing(t *testing.T) {
	growth := 10.0
	context := &agents.ResearchContext{Scenario: &agents.ResearchScenario{RevenueGrowthPct: &growth}}
	calculations := scenarioEvidenceCalculations(context, nil, nil, nil)
	if len(calculations) != 4 {
		t.Fatalf("explicit partial scenario did not emit reviewable unknown calculations: %+v", calculations)
	}
	for _, calculation := range calculations {
		if calculation.Status != "unknown" || calculation.Value != nil || calculation.Reason == "" {
			t.Fatalf("missing scenario evidence was treated as a value: %+v", calculation)
		}
	}
	assessment := assessResearchContext(context, "AAPL", calculations, nil, nil)
	if assessment.Scenario.Status != "unknown" || assessment.Scenario.Reason == "" {
		t.Fatalf("partial scenario was not explicitly unknown: %+v", assessment.Scenario)
	}
}

type evidenceQuoteProviderFixture struct {
	liveCalls     int
	snapshotCalls int
	live          map[string]market.Quote
	snapshot      map[string]market.Quote
}

func (p *evidenceQuoteProviderFixture) Quotes([]string) map[string]market.Quote {
	p.liveCalls++
	return p.live
}

func (p *evidenceQuoteProviderFixture) SnapshotQuotes([]string) map[string]market.Quote {
	p.snapshotCalls++
	return p.snapshot
}

func TestEvidenceOnlyQuoteColdStartFetchesProviderWithoutQuoteGET(t *testing.T) {
	provider := &evidenceQuoteProviderFixture{live: map[string]market.Quote{"AAPL": {
		Price: 230, Source: "nasdaq", DataTime: "2026-09-03", TimeGranularity: "date",
		ProviderURL: "https://api.nasdaq.com/api/quote/AAPL/info?assetclass=stocks",
	}}}
	invocation, ok := evidenceOnlyQuoteInvocationFromProvider(provider, "AAPL", "2026-09-05", time.Now())
	if !ok || provider.liveCalls != 1 || provider.snapshotCalls != 0 || invocation.DataTime != "2026-09-03" || invocation.TimeGranularity != "date" {
		t.Fatalf("cold-start provider quote was not captured directly: invocation=%+v provider=%+v", invocation, provider)
	}

	provider = &evidenceQuoteProviderFixture{snapshot: map[string]market.Quote{"AAPL": {
		Price: 229, Source: "stale-snapshot:us", DataTime: "2026-09-02", TimeGranularity: "date",
		ProviderURL: "https://api.nasdaq.com/api/quote/AAPL/info?assetclass=stocks",
	}}}
	if _, ok := evidenceOnlyQuoteInvocationFromProvider(provider, "AAPL", "2026-09-05", time.Now()); !ok || provider.liveCalls != 1 || provider.snapshotCalls != 1 {
		t.Fatalf("failed live refresh did not use provider-timed snapshot fallback: %+v", provider)
	}
}

func evidenceOnlyFixtureInvocations(now time.Time) []dataflows.ToolInvocation {
	fundamentals := dataflows.FundamentalData{
		Source: "sec-edgar-companyfacts", FiscalPeriod: "quarterly:2026-06-30", AsOf: "2026-06-30", FilingDate: "2026-08-01", Accession: "0000320193-26-000001", SourceURL: "https://www.sec.gov/Archives/edgar/data/320193/000032019326000001/aapl.htm",
		Periods: financialFixturePeriods(),
	}
	observations := make([]dataflows.HistoricalPriceObservation, 0, 62)
	for day := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC); len(observations) < 62; day = day.AddDate(0, 0, 1) {
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			continue
		}
		price := float64(100 + len(observations))
		observations = append(observations, dataflows.HistoricalPriceObservation{Date: day.Format("2006-01-02"), Open: price - 1, High: price + 1, Low: price - 2, Close: price, Volume: int64(1_000_000 + len(observations)*1000)})
	}
	historical := dataflows.HistoricalEvidence{Symbol: "AAPL", Source: "nasdaq-historical", SourceURL: "https://api.nasdaq.com/api/quote/AAPL/historical?assetclass=stocks", DataTime: observations[len(observations)-1].Date, TimeGranularity: "date", SampleCount: len(observations), MinimumSessions: dataflows.HistoricalEvidenceMinimumSessions, Status: "complete", Observations: observations}
	news := dataflows.ReviewTickerNews("AAPL", "Apple Inc", []dataflows.NewsArticle{{Title: "Apple reports quarterly revenue", Link: "https://www.reuters.com/technology/apple-results", Published: "2026-09-05T12:00:00Z", Source: "Reuters", Description: "AAPL reported quarterly results."}})
	items := []dataflows.ToolInvocation{
		{Name: "get_stock_data", Provider: "nasdaq", SourceURL: "https://api.nasdaq.com/api/quote/AAPL/info?assetclass=stocks", DataTime: historical.DataTime, TimeGranularity: "date", ObservationTimes: []string{historical.DataTime}, RawPayload: fmt.Sprintf(`{"symbol":"AAPL","quote":{"price":%.2f,"dataTime":%q}}`, observations[len(observations)-1].Close, historical.DataTime)},
		{Name: "get_fundamentals", Provider: "sec-edgar-companyfacts", SourceURL: fundamentals.SourceURL, DataTime: "2026-06-30", TimeGranularity: "date", ObservationTimes: financialFixtureDates(fundamentals.Periods, false), DisclosureTime: "2026-08-01", DisclosureTimes: financialFixtureDates(fundamentals.Periods, true), RawPayload: dataflows.FormatFundamentalsForEvidence(&fundamentals, "AAPL")},
		{Name: "get_news", Provider: "Yahoo Finance RSS", SourceURL: dataflows.TickerNewsSourceURL("AAPL"), DataTime: "2026-09-05T12:00:00Z", TimeGranularity: "second", ObservationTimes: []string{"2026-09-05T12:00:00Z"}, RawPayload: dataflows.FormatReviewedNewsForEvidence(news, "Yahoo Finance RSS", dataflows.TickerNewsSourceURL("AAPL"))},
		{Name: "get_historical_evidence", Provider: "nasdaq-historical", SourceURL: historical.SourceURL, DataTime: historical.DataTime, TimeGranularity: "date", ObservationTimes: historicalDates(observations), RawPayload: dataflows.FormatHistoricalEvidence(historical)},
	}
	for index := range items {
		items[index].Status = "captured"
		items[index].CompletedAt = now.Format(time.RFC3339)
		items[index].MethodVersion = "tool-registry-v4"
		items[index].ContentHash = agents.ContentHash(items[index].RawPayload)
		items[index].PayloadExcerpt = items[index].RawPayload
		items[index].PayloadSize = int64(len(items[index].RawPayload))
		items[index].Inputs = map[string]string{"ticker": "AAPL", "requested_as_of": "2026-09-05"}
	}
	return items
}

func TestEvidenceOnlyUsesHistoricalCloseWhenOptionalQuoteConflicts(t *testing.T) {
	base := evidenceOnlyFixtureInvocations(time.Date(2026, 9, 5, 18, 0, 0, 0, time.UTC))
	for _, test := range []struct {
		name     string
		raw      string
		dataTime string
		want     string
	}{
		{"date", `{"symbol":"AAPL","quote":{"price":121,"dataTime":"2026-09-01"}}`, "2026-09-01", "trading date conflict"},
		{"price", fmt.Sprintf(`{"symbol":"AAPL","quote":{"price":999,"dataTime":%q}}`, base[3].DataTime), base[3].DataTime, "latest close conflict"},
	} {
		t.Run(test.name, func(t *testing.T) {
			invocations := append([]dataflows.ToolInvocation(nil), base...)
			invocations[0].RawPayload = test.raw
			invocations[0].ContentHash = agents.ContentHash(test.raw)
			invocations[0].PayloadExcerpt = test.raw
			invocations[0].PayloadSize = int64(len(test.raw))
			invocations[0].DataTime = test.dataTime
			invocations[0].ObservationTimes = []string{test.dataTime}
			result, err := buildEvidenceOnlyResult(agents.AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock"}, invocations, t.TempDir(), time.Now())
			if err != nil {
				t.Fatalf("optional quote conflict blocked the historical dossier: %v", err)
			}
			if !containsEvidenceGap(result.Dossier.Gaps, test.want) {
				t.Fatalf("quote conflict was not disclosed as a gap: %+v", result.Dossier.Gaps)
			}
			price := findEvidenceCalculation(result.Dossier.Calculations, "as_of_research_price:"+base[3].DataTime)
			if price == nil || price.Value == nil || *price.Value != 161 || price.Formula != "latest completed dated Nasdaq historical close" || len(price.EvidenceIDs) != 1 || price.EvidenceIDs[0] != "tool-historical" {
				t.Fatalf("historical close was not retained as the primary research price: %+v", price)
			}
		})
	}
}

func TestEvidenceOnlyDoesNotRequireOptionalQuote(t *testing.T) {
	now := time.Date(2026, 9, 5, 18, 0, 0, 0, time.UTC)
	invocations := evidenceOnlyFixtureInvocations(now)[1:]
	if preflight := dataflows.AssessEvidenceOnlyPreflight(invocations, "2026-09-05"); !preflight.Passed {
		t.Fatalf("historical-primary preflight still required a quote: %+v", preflight)
	}
	if preflight := dataflows.AssessResearchPreflight(invocations, "2026-09-05"); preflight.Passed || !strings.Contains(strings.Join(preflight.Reasons, ";"), "get_stock_data") {
		t.Fatalf("ordinary 10-Agent preflight lost its quote requirement: %+v", preflight)
	}
	result, err := buildEvidenceOnlyResult(agents.AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock"}, invocations, t.TempDir(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Dossier.Facts) != 3 || !containsEvidenceGap(result.Dossier.Gaps, "Optional quote cross-check unavailable") {
		t.Fatalf("missing quote was not retained as a non-blocking gap: %+v", result.Dossier)
	}
}

func containsEvidenceGap(gaps []string, fragment string) bool {
	for _, gap := range gaps {
		if strings.Contains(gap, fragment) {
			return true
		}
	}
	return false
}

func allFinancialFixtureFields() []string {
	return []string{"totalRevenue", "grossProfit", "operatingIncome", "netIncome", "totalAssets", "totalLiabilities", "stockholdersEquity", "cashAndEquivalents"}
}

func financialFixturePeriods() []dataflows.FinancialPeriod {
	return []dataflows.FinancialPeriod{
		financialFixturePeriod("FY2026-Q3", "quarterly", "10-Q", "2026-04-01", "2026-06-30", "2026-08-01", 120, 24),
		financialFixturePeriod("FY2026-Q2", "quarterly", "10-Q", "2026-01-01", "2026-03-31", "2026-05-01", 110, 22),
		financialFixturePeriod("FY2026-Q1", "quarterly", "10-Q", "2025-10-01", "2025-12-31", "2026-02-01", 100, 20),
		financialFixturePeriod("FY2025-Q4", "quarterly", "10-Q", "2025-07-01", "2025-09-30", "2025-11-01", 90, 18),
		financialFixturePeriod("FY2025-Q3", "quarterly", "10-Q", "2025-04-01", "2025-06-30", "2025-08-01", 80, 16),
		financialFixturePeriod("FY2025", "annual", "10-K", "2025-01-01", "2025-12-31", "2026-02-15", 400, 80),
		financialFixturePeriod("FY2024", "annual", "10-K", "2024-01-01", "2024-12-31", "2025-02-15", 360, 72),
		financialFixturePeriod("FY2023", "annual", "10-K", "2023-01-01", "2023-12-31", "2024-02-15", 320, 64),
	}
}

func financialFixturePeriod(label, frequency, form, start, end, filed string, revenue, netIncome float64) dataflows.FinancialPeriod {
	accession := strings.ReplaceAll("fixture-"+form+"-"+end, "-", "")
	url := "https://www.sec.gov/Archives/edgar/data/320193/" + accession + "/index.html"
	period := dataflows.FinancialPeriod{
		FiscalPeriod: label, Frequency: frequency, Form: form, PeriodStart: start, PeriodEnd: end,
		FilingDate: filed, Accession: accession, SourceURL: url, TotalRevenue: revenue,
		GrossProfit: revenue * .5, OperatingIncome: revenue * .3, NetIncome: netIncome,
		TotalAssets: revenue * 4, TotalLiabilities: revenue * 2.4, StockholdersEquity: revenue * 1.6,
		CashAndEquivalents: revenue * .4, AvailableFields: allFinancialFixtureFields(),
		FieldEvidence: map[string]dataflows.FinancialFieldEvidence{},
	}
	for _, field := range period.AvailableFields {
		kind, fieldStart := "instant", end
		if financialFieldKind(field) == "duration" {
			kind, fieldStart = "duration", start
		}
		period.FieldEvidence[field] = dataflows.FinancialFieldEvidence{Source: "sec-edgar-companyfacts", URL: url, Unit: "USD", PeriodStart: fieldStart, PeriodEnd: end, Form: form, Filed: filed, Accession: accession, PeriodKind: kind}
	}
	return period
}

func financialFixtureDates(periods []dataflows.FinancialPeriod, filing bool) []string {
	out := make([]string, 0, len(periods))
	for _, period := range periods {
		if filing {
			out = append(out, period.FilingDate)
		} else {
			out = append(out, period.PeriodEnd)
		}
	}
	return out
}

func agentFinancialFixturePeriod(label, start, end string, revenue, netIncome float64) agents.EvidenceFinancialPeriod {
	period := financialFixturePeriod(label, "quarterly", "10-Q", start, end, "2026-08-01", revenue, netIncome)
	converted, err := evidenceFinancialPeriods([]dataflows.FinancialPeriod{period, financialFixturePeriod("placeholder", "quarterly", "10-Q", "2024-01-01", "2024-03-31", "2024-05-01", 1, 1)})
	if err != nil {
		panic(err)
	}
	for _, candidate := range converted {
		if candidate.PeriodEnd == end {
			return candidate
		}
	}
	panic("fixture conversion failed")
}

func historicalDates(observations []dataflows.HistoricalPriceObservation) []string {
	out := make([]string, 0, len(observations))
	for _, observation := range observations {
		out = append(out, observation.Date)
	}
	return out
}

func TestSaveAndLoadResultFailClosedForLegacyExecutableAdvice(t *testing.T) {
	handler := &Handler{config: &config.Config{ResultsDir: t.TempDir()}}
	result := &agents.AnalysisResult{
		Ticker: "CRWV", TradeDate: "2026-08-14", Decision: "HOLD",
		State: agents.AgentState{InvestmentPlan: "sell 50%", TraderInvestmentPlan: "stop loss 99", FinalTradeDecision: "buy calls"},
	}
	if err := handler.saveResult(result); err != nil {
		t.Fatal(err)
	}
	loaded := handler.loadResults()
	if len(loaded) != 1 || loaded[0].Status != agents.ResearchStatusUnavailable || loaded[0].Decision != agents.ResearchDecisionUnavailable {
		t.Fatalf("legacy result was not marked unavailable: %+v", loaded)
	}
	raw, err := os.ReadFile(filepath.Join(handler.config.ResultsDir, "api_results", "CRWV_2026-08-14_legacy.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"HOLD", "sell 50%", "stop loss", "buy calls"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("persisted unavailable result contains executable advice %q: %s", forbidden, raw)
		}
	}
}

func TestAnalysisStatusExposesUnavailableWithoutAdvice(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &Handler{hub: NewHub(), lastResult: &agents.AnalysisResult{
		Ticker: "CRWV", TradeDate: "2026-08-14", Decision: "HOLD",
		State: agents.AgentState{FinalTradeDecision: "sell with stop loss"},
	}}
	router := gin.New()
	router.GET("/status", handler.GetStatus)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/status", nil))
	if w.Code != http.StatusOK || w.Header().Get("X-Research-Status") != agents.ResearchStatusUnavailable {
		t.Fatalf("status=%d headers=%v body=%s", w.Code, w.Header(), w.Body.String())
	}
	if strings.Contains(w.Body.String(), "HOLD") || strings.Contains(w.Body.String(), "stop loss") {
		t.Fatalf("status exposed blocked advice: %s", w.Body.String())
	}
}

func writeResearchFixture(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
