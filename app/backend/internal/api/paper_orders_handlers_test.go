package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"trading-agents/internal/agents"
	"trading-agents/internal/config"
	"trading-agents/internal/database"
	"trading-agents/internal/market"
	"trading-agents/internal/models"
)

type paperQuoteFixture struct {
	mu     sync.RWMutex
	prices map[string]float64
	source string
	stale  bool
	calls  [][]string
	now    time.Time
}

func (f *paperQuoteFixture) setPrice(symbol string, price float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prices[strings.ToUpper(symbol)] = price
}

func (f *paperQuoteFixture) remove(symbol string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.prices, strings.ToUpper(symbol))
}

func (f *paperQuoteFixture) result(symbols []string) quoteFetchResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]string(nil), symbols...))
	quotes := make(map[string]market.Quote, len(symbols))
	for _, symbol := range symbols {
		symbol = strings.ToUpper(symbol)
		if price := f.prices[symbol]; price > 0 {
			quotes[symbol] = market.Quote{
				Price: price, Source: f.source, DataTime: f.now.UTC().Format(time.RFC3339),
				TimeGranularity: "second", ProviderURL: "https://quotes.example.test/" + symbol,
			}
		}
	}
	return quoteFetchResult{
		quotes: quotes, source: "self-provider", dataTime: f.now.UTC(), timedOut: f.stale,
	}
}

func (f *paperQuoteFixture) resetCalls() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = nil
}

func (f *paperQuoteFixture) recordedCalls() [][]string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make([][]string, len(f.calls))
	for index := range f.calls {
		result[index] = append([]string(nil), f.calls[index]...)
	}
	return result
}

func setupPaperOrderTestRouter(t *testing.T) (*gin.Engine, *paperQuoteFixture) {
	return setupPaperOrderTestRouterWithConfig(t, nil)
}

func setupPaperOrderTestRouterWithConfig(t *testing.T, cfg *config.Config) (*gin.Engine, *paperQuoteFixture) {
	t.Helper()
	db, err := database.OpenSQLite(t.TempDir() + "/paper-orders.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.PaperOrder{}, &models.PaperFill{}, &models.PaperAccount{}, &models.PaperPosition{}, &models.PaperStopScanAudit{}, &models.PaperEquityCheckpoint{}, &models.PaperDailyEquityBaseline{}, &models.AuditEvent{}); err != nil {
		t.Fatal(err)
	}
	for _, currency := range []string{"USD", "CNY"} {
		if err := db.Create(&models.PaperAccount{Currency: currency, InitialCash: 10_000, Cash: 10_000, Version: 1}).Error; err != nil {
			t.Fatal(err)
		}
	}
	previousDB := database.DB
	database.DB = db
	fixture := &paperQuoteFixture{
		prices: map[string]float64{"NVDA": 100, "300750": 200}, source: "tradingview",
		now: time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC),
	}
	// Normal handler tests model a process whose scanner has already opened the
	// trading day. Scanner-specific tests delete these rows when exercising the
	// first-scan creation path.
	for _, currency := range []string{"USD", "CNY"} {
		baseline := models.PaperDailyEquityBaseline{
			Currency: currency, MarketDate: paperMarketDateForCurrency(currency, fixture.now),
			ObservedAt: fixture.now, Equity: 10_000,
		}
		if err := db.Create(&baseline).Error; err != nil {
			t.Fatal(err)
		}
	}
	previousQuotes := boundedQuotesForRequest
	boundedQuotesForRequest = fixture.result
	previousUniverse := paperUniverseLoader
	previousADV := paperADVLoader
	paperUniverseLoader = func(marketCode string) ([]models.Stock, string, error) {
		if marketCode == "cn" {
			return []models.Stock{{Symbol: "300750", Price: 200, Sector: "新能源"}}, "test-cn", nil
		}
		return []models.Stock{
			{Symbol: "AAPL", Price: 100, Sector: "Technology"},
			{Symbol: "NVDA", Price: 100, Sector: "Technology"},
			{Symbol: "MSFT", Price: 100, Sector: "Technology"},
			{Symbol: "AMZN", Price: 100, Sector: "Technology"},
			{Symbol: "XOM", Price: 100, Sector: "Energy"},
			{Symbol: "JPM", Price: 100, Sector: "Financials"},
			{Symbol: "PFE", Price: 100, Sector: "Health Care"},
			{Symbol: "WMT", Price: 100, Sector: "Consumer Staples"},
			{Symbol: "BA", Price: 100, Sector: "Industrials"},
			{Symbol: "NEE", Price: 100, Sector: "Utilities"},
			{Symbol: "GOOGL", Price: 100, Sector: "Technology"},
			{Symbol: "T", Price: 100, Sector: "Communication Services"},
			{Symbol: "LIN", Price: 100, Sector: "Materials"},
			{Symbol: "PLD", Price: 100, Sector: "Real Estate"},
			{Symbol: "CAT", Price: 100, Sector: "Machinery"},
			{Symbol: "KO", Price: 100, Sector: "Beverages"},
		}, "test-us", nil
	}
	paperADVLoader = func(marketCode string, symbols []string) (map[string]float64, string, error) {
		return map[string]float64{
			"300750": 20_000_000, "AAPL": 50_000_000, "NVDA": 40_000_000, "MSFT": 20_000_000,
			"AMZN": 25_000_000, "XOM": 15_000_000, "JPM": 10_000_000, "PFE": 30_000_000,
			"WMT": 10_000_000, "BA": 8_000_000, "NEE": 7_000_000, "GOOGL": 20_000_000,
			"T": 30_000_000, "LIN": 2_000_000, "PLD": 4_000_000, "CAT": 3_000_000, "KO": 12_000_000,
		}, "test-authoritative-20d-adv", nil
	}
	t.Cleanup(func() {
		database.DB = previousDB
		boundedQuotesForRequest = previousQuotes
		paperUniverseLoader = previousUniverse
		paperADVLoader = previousADV
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := &Handler{config: cfg, paperClock: func() time.Time { return fixture.now }}
	scanner := newPaperStopScanner(database.DB, paperStopScannerOptions{
		Clock:      func() time.Time { return fixture.now },
		MarketOpen: func(market string, _ time.Time) bool { return market == "US" },
	})
	scanner.mu.Lock()
	scanner.running = true
	scanner.mu.Unlock()
	handler.SetPaperStopScanner(scanner)
	router.POST("/api/paper-orders/validate", handler.ValidatePaperOrder)
	router.POST("/api/paper-orders", handler.SubmitPaperOrder)
	router.GET("/api/paper-orders", handler.ListPaperOrders)
	router.GET("/api/paper-orders/portfolio-risk", handler.GetPaperPortfolioRisk)
	router.POST("/api/paper-orders/daily-baseline", handler.EstablishPaperDailyEquityBaseline)
	router.GET("/api/paper-orders/:clientOrderId", handler.GetPaperOrder)
	router.POST("/api/paper-orders/:clientOrderId/cancel", handler.CancelPaperOrder)
	router.POST("/api/paper-orders/:clientOrderId/simulated-fill", handler.SimulatePaperFill)
	return router, fixture
}

func paperOrderPayload(id, symbol string, quantity int, stop float64) map[string]any {
	payload := validPaperOrderPayload(id)
	payload["symbol"] = symbol
	payload["quantity"] = quantity
	payload["protectiveStop"] = stop
	risk := payload["riskSnapshot"].(map[string]any)
	risk["maxLoss"] = float64(quantity) * (100 - stop)
	risk["plannedNotional"] = float64(quantity) * 100
	return payload
}

func protectiveSellPayload(id, symbol string, quantity int, trigger float64) map[string]any {
	payload := validPaperOrderPayload(id)
	payload["symbol"] = symbol
	payload["side"] = "SELL"
	payload["orderType"] = "STOP_MARKET"
	payload["entry"] = nil
	payload["triggerPrice"] = trigger
	payload["protectiveStop"] = nil
	payload["takeProfit"] = nil
	payload["quantity"] = quantity
	risk := payload["riskSnapshot"].(map[string]any)
	risk["maxLoss"] = float64(quantity) * (100 - trigger)
	risk["plannedNotional"] = float64(quantity) * trigger
	return payload
}

func decodePaperPortfolio(t *testing.T, w *httptest.ResponseRecorder) paperPortfolioRiskSummary {
	t.Helper()
	var response paperPortfolioRiskSummary
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode paper portfolio: %v body=%s", err, w.Body.String())
	}
	return response
}

func validPaperOrderPayload(id string) map[string]any {
	return map[string]any{
		"clientOrderId": id, "environment": "PAPER", "symbol": "NVDA", "side": "BUY",
		"market": "US", "currency": "USD", "orderType": "LIMIT", "timeInForce": "GTC",
		"referencePrice": 100.0, "entry": 100.0, "triggerPrice": nil, "protectiveStop": 98.0, "takeProfit": 106.0, "quantity": 10,
		"quoteSource": "client-claim", "quoteTime": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		"riskSnapshot": map[string]any{
			"investableCapital": 10_000.0, "maxLoss": 20.0, "maxRiskAmount": 150.0,
			"plannedNotional": 1_000.0, "maxNotionalAmount": 1_500.0, "riskLimitPassed": true,
		},
	}
}

func paperRequest(t *testing.T, router http.Handler, method, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func decodePaperOrderResponse(t *testing.T, w *httptest.ResponseRecorder) paperOrderResponse {
	t.Helper()
	var response paperOrderResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v body=%s", err, w.Body.String())
	}
	return response
}

func decodePaperValidation(t *testing.T, w *httptest.ResponseRecorder) paperValidationResponse {
	t.Helper()
	var response paperValidationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode validation: %v body=%s", err, w.Body.String())
	}
	return response
}

func TestPaperOrderPersistsResearchProvenance(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 5, 18, 0, 0, 0, time.UTC)
	research, err := buildEvidenceOnlyResult(
		agents.AnalysisRequest{Ticker: "NVDA", TradeDate: "2026-09-05", AssetType: "stock"},
		evidenceOnlyFixtureInvocations(now), root, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	writeResearchFixture(t, filepath.Join(root, "api_results", "NVDA.json"), research)
	router, _ := setupPaperOrderTestRouterWithConfig(t, &config.Config{ResultsDir: root})
	payload := validPaperOrderPayload("paper-research-provenance")
	payload["researchRunId"] = research.Audit.RunID
	payload["researchTicker"] = "nvda"

	created := paperRequest(t, router, http.MethodPost, "/api/paper-orders", payload)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	order := decodePaperOrderResponse(t, created).Order
	if order.ResearchRunID != research.Audit.RunID || order.ResearchTicker != "NVDA" {
		t.Fatalf("provenance not persisted: run=%q ticker=%q", order.ResearchRunID, order.ResearchTicker)
	}

	var persisted models.PaperOrder
	if err := database.DB.Where("client_order_id = ?", order.ClientOrderID).First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.ResearchRunID != order.ResearchRunID || persisted.ResearchTicker != order.ResearchTicker {
		t.Fatalf("database provenance mismatch: %+v", persisted)
	}
}

func TestPaperOrderRejectsUnverifiableResearchProvenance(t *testing.T) {
	root := t.TempDir()
	router, _ := setupPaperOrderTestRouterWithConfig(t, &config.Config{ResultsDir: root})
	payload := validPaperOrderPayload("paper-missing-research")
	payload["researchRunId"] = "run-does-not-exist"
	payload["researchTicker"] = "NVDA"

	created := paperRequest(t, router, http.MethodPost, "/api/paper-orders", payload)
	if created.Code != http.StatusUnprocessableEntity || !strings.Contains(created.Body.String(), "research run") {
		t.Fatalf("unverifiable research provenance status=%d body=%s", created.Code, created.Body.String())
	}
}

func assertReasonContains(t *testing.T, reasons []string, fragment string) {
	t.Helper()
	for _, reason := range reasons {
		if strings.Contains(reason, fragment) {
			return
		}
	}
	t.Fatalf("reasons %v do not contain %q", reasons, fragment)
}

func TestListPaperOrdersKeepsAllActiveOrdersAndReportsHiddenTerminalRows(t *testing.T) {
	router, fixture := setupPaperOrderTestRouter(t)
	createdAt := fixture.now.Add(-24 * time.Hour)
	base := models.PaperOrder{
		Environment: paperEnvironment, Symbol: "NVDA", Side: "BUY", Market: "US", Currency: "USD",
		OrderType: "LIMIT", TimeInForce: "GTC", Quantity: 1, QuoteSource: "test", QuoteTime: fixture.now,
		RequestHash: "test", RemainingQty: 1, Version: 1,
	}
	activeIDs := map[string]bool{}
	for index := 0; index < 2; index++ {
		order := base
		order.ClientOrderID = fmt.Sprintf("active-%d", index)
		order.CreatedAt = createdAt.Add(time.Duration(index) * time.Second)
		order.Status = paperStatusAccepted
		order.StatusHistory = []models.PaperOrderStatusEvent{{Status: paperStatusAccepted, At: order.CreatedAt}}
		if err := database.DB.Create(&order).Error; err != nil {
			t.Fatal(err)
		}
		activeIDs[order.ClientOrderID] = true
	}
	protected := base
	protected.ClientOrderID = "terminal-parent-with-active-protection"
	protected.CreatedAt = createdAt.Add(2 * time.Second)
	protected.Status = paperStatusSimulatedFill
	protected.FillQty = 1
	protected.RemainingQty = 0
	protected.ProtectionInitialized = true
	protected.ProtectionRemainingQty = 1
	protected.StatusHistory = []models.PaperOrderStatusEvent{{Status: paperStatusSimulatedFill, At: protected.CreatedAt}}
	if err := database.DB.Create(&protected).Error; err != nil {
		t.Fatal(err)
	}
	activeIDs[protected.ClientOrderID] = true

	for index := 0; index < 105; index++ {
		order := base
		order.ClientOrderID = fmt.Sprintf("terminal-%03d", index)
		order.CreatedAt = fixture.now.Add(time.Duration(index) * time.Second)
		order.Status = paperStatusCancelled
		order.StatusHistory = []models.PaperOrderStatusEvent{{Status: paperStatusCancelled, At: order.CreatedAt}}
		if err := database.DB.Create(&order).Error; err != nil {
			t.Fatal(err)
		}
	}

	response := paperRequest(t, router, http.MethodGet, "/api/paper-orders", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Orders              []models.PaperOrder `json:"orders"`
		Total               int64               `json:"total"`
		HiddenTerminalCount int64               `json:"hiddenTerminalCount"`
		Truncated           bool                `json:"truncated"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Total != 108 || payload.HiddenTerminalCount != 5 || !payload.Truncated || len(payload.Orders) != 103 {
		t.Fatalf("unexpected list metadata: total=%d hidden=%d truncated=%t orders=%d", payload.Total, payload.HiddenTerminalCount, payload.Truncated, len(payload.Orders))
	}
	for _, order := range payload.Orders {
		delete(activeIDs, order.ClientOrderID)
	}
	if len(activeIDs) != 0 {
		t.Fatalf("active orders were hidden by terminal cap: %v", activeIDs)
	}
}

func TestPaperOrderValidationUsesAuthoritativeQuoteAndAccount(t *testing.T) {
	router, fixture := setupPaperOrderTestRouter(t)
	payload := validPaperOrderPayload("paper-forged")
	payload["referencePrice"] = 140.0
	risk := payload["riskSnapshot"].(map[string]any)
	risk["investableCapital"] = 1_000_000_000.0
	risk["maxRiskAmount"] = 15_000_000.0
	risk["maxNotionalAmount"] = 150_000_000.0

	response := paperRequest(t, router, http.MethodPost, "/api/paper-orders/validate", payload)
	validation := decodePaperValidation(t, response)
	if validation.Valid {
		t.Fatalf("forged quote and capital must be rejected: %+v", validation)
	}
	assertReasonContains(t, validation.RejectionReasons, "referencePrice deviates")

	fixture.remove("NVDA")
	payload = validPaperOrderPayload("paper-missing")
	response = paperRequest(t, router, http.MethodPost, "/api/paper-orders/validate", payload)
	validation = decodePaperValidation(t, response)
	if validation.Valid {
		t.Fatalf("unknown symbol must be rejected: %+v", validation)
	}
	assertReasonContains(t, validation.RejectionReasons, "authoritative quote unavailable")
}

func TestPaperOrderValidationRejectsStaleAuthoritativeQuote(t *testing.T) {
	router, fixture := setupPaperOrderTestRouter(t)
	fixture.mu.Lock()
	fixture.stale = true
	fixture.source = "stale-snapshot:us"
	fixture.mu.Unlock()

	response := paperRequest(t, router, http.MethodPost, "/api/paper-orders/validate", validPaperOrderPayload("paper-stale"))
	validation := decodePaperValidation(t, response)
	if validation.Valid {
		t.Fatalf("stale authoritative quote must be rejected: %+v", validation)
	}
	assertReasonContains(t, validation.RejectionReasons, "authoritative quote is stale")
}

func TestPaperOrderRejectsInsufficientCashAndPosition(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	if err := database.DB.Model(&models.PaperAccount{}).Where("currency = ?", "USD").Update("cash", 500).Error; err != nil {
		t.Fatal(err)
	}

	buy := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("paper-no-cash"))
	buyOrder := decodePaperOrderResponse(t, buy).Order
	if buy.Code != http.StatusUnprocessableEntity || buyOrder.Status != paperStatusRejected {
		t.Fatalf("insufficient-cash order was not rejected: status=%d order=%+v", buy.Code, buyOrder)
	}
	if !strings.Contains(buyOrder.RejectionReason, "insufficient available paper cash") || buyOrder.ReservedCash != 0 {
		t.Fatalf("cash rejection did not preserve ledger invariants: %+v", buyOrder)
	}

	sellPayload := protectiveSellPayload("paper-no-position", "NVDA", 10, 98)
	sell := paperRequest(t, router, http.MethodPost, "/api/paper-orders", sellPayload)
	sellOrder := decodePaperOrderResponse(t, sell).Order
	if sell.Code != http.StatusUnprocessableEntity || sellOrder.Status != paperStatusRejected {
		t.Fatalf("uncovered sell was not rejected: status=%d order=%+v", sell.Code, sellOrder)
	}
	if !strings.Contains(sellOrder.RejectionReason, "insufficient available paper position") || sellOrder.ReservedQty != 0 {
		t.Fatalf("position rejection did not preserve ledger invariants: %+v", sellOrder)
	}

	var account models.PaperAccount
	if err := database.DB.Where("currency = ?", "USD").First(&account).Error; err != nil {
		t.Fatal(err)
	}
	if account.Cash != 500 || account.ReservedCash != 0 {
		t.Fatalf("rejected orders changed cash ledger: %+v", account)
	}
}

func TestPaperOrderSubmitIsIdempotentAndDetectsConflict(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	payload := validPaperOrderPayload("paper-idempotent")
	created := paperRequest(t, router, http.MethodPost, "/api/paper-orders", payload)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	createdResponse := decodePaperOrderResponse(t, created)
	if createdResponse.Order.Status != paperStatusAccepted || createdResponse.Order.Environment != paperEnvironment {
		t.Fatalf("unexpected accepted response: %+v", createdResponse)
	}
	if createdResponse.Order.ReferencePrice != 100 || createdResponse.Order.QuoteSource != "tradingview" {
		t.Fatalf("order did not persist authoritative quote: %+v", createdResponse.Order)
	}

	replayed := paperRequest(t, router, http.MethodPost, "/api/paper-orders", payload)
	if replayed.Code != http.StatusOK || !decodePaperOrderResponse(t, replayed).IdempotentReplay {
		t.Fatalf("expected idempotent replay, status=%d body=%s", replayed.Code, replayed.Body.String())
	}
	payload["quantity"] = 11
	conflict := paperRequest(t, router, http.MethodPost, "/api/paper-orders", payload)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("expected payload conflict, status=%d body=%s", conflict.Code, conflict.Body.String())
	}
}

func TestPaperOrderLifecycleAndRejectedAuditRecord(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	payload := validPaperOrderPayload("paper-cancel")
	validated := paperRequest(t, router, http.MethodPost, "/api/paper-orders/validate", payload)
	validation := decodePaperValidation(t, validated)
	if !validation.Valid || validation.Environment != paperEnvironment {
		t.Fatalf("unexpected validation: %+v", validation)
	}
	paperRequest(t, router, http.MethodPost, "/api/paper-orders", payload)
	cancelled := paperRequest(t, router, http.MethodPost, "/api/paper-orders/paper-cancel/cancel", nil)
	response := decodePaperOrderResponse(t, cancelled)
	if cancelled.Code != http.StatusOK || response.Order.Status != paperStatusCancelled || len(response.Order.StatusHistory) != 2 {
		t.Fatalf("unexpected cancellation: status=%d response=%+v", cancelled.Code, response)
	}
	var account models.PaperAccount
	if err := database.DB.Where("currency = ?", "USD").First(&account).Error; err != nil || account.Cash != 10_000 || account.ReservedCash != 0 {
		t.Fatalf("cancel did not release reservation: account=%+v err=%v", account, err)
	}
	status := paperRequest(t, router, http.MethodGet, "/api/paper-orders/paper-cancel", nil)
	if got := decodePaperOrderResponse(t, status).Order.Status; status.Code != http.StatusOK || got != paperStatusCancelled {
		t.Fatalf("unexpected status lookup: status=%d orderStatus=%s", status.Code, got)
	}
	invalidFill := paperRequest(t, router, http.MethodPost, "/api/paper-orders/paper-cancel/simulated-fill", nil)
	if invalidFill.Code != http.StatusConflict {
		t.Fatalf("expected terminal-state conflict, status=%d", invalidFill.Code)
	}

	rejectedPayload := validPaperOrderPayload("paper-rejected")
	rejectedPayload["referencePrice"] = 500.0
	rejected := paperRequest(t, router, http.MethodPost, "/api/paper-orders", rejectedPayload)
	rejectedResponse := decodePaperOrderResponse(t, rejected)
	if rejected.Code != http.StatusUnprocessableEntity || rejectedResponse.Order.Status != paperStatusRejected || rejectedResponse.Order.RejectionReason == "" {
		t.Fatalf("unexpected rejection audit record: status=%d response=%+v", rejected.Code, rejectedResponse)
	}
	rejectedReplay := paperRequest(t, router, http.MethodPost, "/api/paper-orders", rejectedPayload)
	if rejectedReplay.Code != http.StatusUnprocessableEntity || !decodePaperOrderResponse(t, rejectedReplay).IdempotentReplay {
		t.Fatalf("rejected idempotent replay changed semantics: status=%d body=%s", rejectedReplay.Code, rejectedReplay.Body.String())
	}

	listed := paperRequest(t, router, http.MethodGet, "/api/paper-orders", nil)
	var list struct {
		Environment string              `json:"environment"`
		Orders      []models.PaperOrder `json:"orders"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &list); err != nil || list.Environment != paperEnvironment || len(list.Orders) != 2 {
		t.Fatalf("unexpected list: err=%v body=%s", err, listed.Body.String())
	}
}

func TestConcurrentCancelAndFillHaveOneTerminalWinner(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	created := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("paper-race"))
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}

	paths := []string{
		"/api/paper-orders/paper-race/cancel",
		"/api/paper-orders/paper-race/simulated-fill",
	}
	statuses := make(chan int, len(paths))
	var wg sync.WaitGroup
	for _, path := range paths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			statuses <- paperRequest(t, router, http.MethodPost, path, nil).Code
		}(path)
	}
	wg.Wait()
	close(statuses)
	okCount, conflictCount := 0, 0
	for status := range statuses {
		switch status {
		case http.StatusOK:
			okCount++
		case http.StatusConflict:
			conflictCount++
		default:
			t.Fatalf("unexpected concurrent transition status %d", status)
		}
	}
	if okCount != 1 || conflictCount != 1 {
		t.Fatalf("terminal transition results ok=%d conflict=%d", okCount, conflictCount)
	}

	status := paperRequest(t, router, http.MethodGet, "/api/paper-orders/paper-race", nil)
	order := decodePaperOrderResponse(t, status).Order
	if len(order.StatusHistory) != 2 || (order.Status != paperStatusCancelled && order.Status != paperStatusSimulatedFill) {
		t.Fatalf("invalid terminal order: %+v", order)
	}
}

func TestPaperAccountCashAndPositionConservation(t *testing.T) {
	router, fixture := setupPaperOrderTestRouter(t)
	buy := validPaperOrderPayload("paper-buy")
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", buy); got.Code != http.StatusCreated {
		t.Fatalf("buy submit status=%d body=%s", got.Code, got.Body.String())
	}
	filledBuy := paperRequest(t, router, http.MethodPost, "/api/paper-orders/paper-buy/simulated-fill", nil)
	buyResponse := decodePaperOrderResponse(t, filledBuy)
	buyOrder := buyResponse.Order
	if filledBuy.Code != http.StatusOK || buyOrder.FillQty != 10 || buyOrder.FillPrice != 100 || buyOrder.Fee != 1 || buyOrder.FilledAt == nil || !buyResponse.EquityCheckpointed {
		t.Fatalf("unexpected buy fill: status=%d order=%+v", filledBuy.Code, buyOrder)
	}
	if buyOrder.FillQuote == nil || buyOrder.FillQuote.Price != 100 || buyOrder.FillQuote.Source != "tradingview" ||
		!buyOrder.FillQuote.ObservedAt.Equal(fixture.now.UTC()) || buyOrder.FillQuote.ProviderURL != "https://quotes.example.test/NVDA" {
		t.Fatalf("structured fill quote was not returned: %+v", buyOrder.FillQuote)
	}
	var persisted models.PaperOrder
	if err := database.DB.Where("client_order_id = ?", buyOrder.ClientOrderID).First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.FillQuote == nil || !persisted.FillQuote.ObservedAt.Equal(fixture.now.UTC()) || persisted.FillQuote.ProviderURL != buyOrder.FillQuote.ProviderURL {
		t.Fatalf("structured fill quote was not persisted: %+v", persisted.FillQuote)
	}
	var fillAudit models.AuditEvent
	if err := database.DB.Where("action = ?", "paper-order.manual-simulated-fill-record").Order("id desc").First(&fillAudit).Error; err != nil {
		t.Fatalf("structured fill audit missing: %v", err)
	}
	var persistedFill models.PaperFill
	if err := database.DB.Where("paper_order_id = ?", persisted.ID).First(&persistedFill).Error; err != nil {
		t.Fatal(err)
	}
	if fillAudit.Target != paperFillAuditTarget(persistedFill) || fillAudit.PayloadHash != paperSystemAuditHash(persistedFill) {
		t.Fatalf("fill audit does not reference persisted quote: %+v order=%+v", fillAudit, persisted.FillQuote)
	}

	var account models.PaperAccount
	var position models.PaperPosition
	if err := database.DB.Where("currency = ?", "USD").First(&account).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Where("currency = ? AND symbol = ?", "USD", "NVDA").First(&position).Error; err != nil {
		t.Fatal(err)
	}
	if account.Cash != 8_999 || account.ReservedCash != 0 || position.Quantity != 10 || position.ReservedQuantity != 10 {
		t.Fatalf("unexpected account after buy: account=%+v position=%+v", account, position)
	}
	var protectiveChildren []models.PaperOrder
	if err := database.DB.Where("parent_order_id = ? AND status = ?", buyOrder.ID, paperStatusAccepted).Order("order_type desc").Find(&protectiveChildren).Error; err != nil {
		t.Fatalf("automatic OCO children missing: %v", err)
	}
	if len(protectiveChildren) != 2 || protectiveChildren[0].OrderType != "STOP_MARKET" || protectiveChildren[1].OrderType != "LIMIT" || protectiveChildren[0].OCOGroupID == "" || protectiveChildren[0].OCOGroupID != protectiveChildren[1].OCOGroupID || protectiveChildren[0].ReservedQty+protectiveChildren[1].ReservedQty != 10 {
		t.Fatalf("unexpected automatic OCO children: %+v", protectiveChildren)
	}

	sell := protectiveSellPayload("paper-sell", "NVDA", 10, 98)
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", sell); got.Code != http.StatusCreated {
		t.Fatalf("sell submit status=%d body=%s", got.Code, got.Body.String())
	}
	fixture.setPrice("NVDA", 98)
	filledSell := paperRequest(t, router, http.MethodPost, "/api/paper-orders/paper-sell/simulated-fill", nil)
	sellOrder := decodePaperOrderResponse(t, filledSell).Order
	if filledSell.Code != http.StatusOK || sellOrder.FillQty != 10 || sellOrder.FillPrice != 97.951 || sellOrder.Fee != 1 {
		t.Fatalf("unexpected sell fill: status=%d order=%+v", filledSell.Code, sellOrder)
	}
	if err := database.DB.Where("currency = ?", "USD").First(&account).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Where("currency = ? AND symbol = ?", "USD", "NVDA").First(&position).Error; err != nil {
		t.Fatal(err)
	}
	if account.Cash != 9_977.51 || account.ReservedCash != 0 || position.Quantity != 0 || position.ReservedQuantity != 0 {
		t.Fatalf("cash/position conservation failed: account=%+v position=%+v", account, position)
	}
}

func TestPaperPartialFillsAppendLedgerAndResizeOCOProtection(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("paper-partial")); got.Code != http.StatusCreated {
		t.Fatalf("submit status=%d body=%s", got.Code, got.Body.String())
	}
	first := paperRequest(t, router, http.MethodPost, "/api/paper-orders/paper-partial/simulated-fill", map[string]any{"fillId": "partial-1", "quantity": 4})
	firstOrder := decodePaperOrderResponse(t, first).Order
	if first.Code != http.StatusOK || firstOrder.Status != paperStatusAccepted || firstOrder.FillQty != 4 || firstOrder.RemainingQty != 6 || len(firstOrder.Fills) != 1 {
		t.Fatalf("unexpected first partial fill: status=%d order=%+v", first.Code, firstOrder)
	}
	var position models.PaperPosition
	if err := database.DB.Where("currency = ? AND symbol = ?", "USD", "NVDA").First(&position).Error; err != nil {
		t.Fatal(err)
	}
	if position.Quantity != 4 || position.ReservedQuantity != 4 {
		t.Fatalf("partial OCO protection is not aligned: %+v", position)
	}
	var children []models.PaperOrder
	if err := database.DB.Where("parent_order_id = ? AND status = ?", firstOrder.ID, paperStatusAccepted).Find(&children).Error; err != nil {
		t.Fatal(err)
	}
	if len(children) != 2 || children[0].Quantity != 4 || children[1].Quantity != 4 {
		t.Fatalf("partial fill protective children=%+v", children)
	}

	replay := paperRequest(t, router, http.MethodPost, "/api/paper-orders/paper-partial/simulated-fill", map[string]any{"fillId": "partial-1", "quantity": 4})
	replayResponse := decodePaperOrderResponse(t, replay)
	if replay.Code != http.StatusOK || !replayResponse.IdempotentReplay || len(replayResponse.Order.Fills) != 1 {
		t.Fatalf("partial replay was not idempotent: status=%d response=%+v", replay.Code, replayResponse)
	}
	mismatchedReplay := paperRequest(t, router, http.MethodPost, "/api/paper-orders/paper-partial/simulated-fill", map[string]any{"fillId": "partial-1", "quantity": 5})
	if mismatchedReplay.Code != http.StatusConflict || !strings.Contains(mismatchedReplay.Body.String(), "replay quantity") {
		t.Fatalf("mismatched fill replay status=%d body=%s", mismatchedReplay.Code, mismatchedReplay.Body.String())
	}

	second := paperRequest(t, router, http.MethodPost, "/api/paper-orders/paper-partial/simulated-fill", map[string]any{"fillId": "partial-2", "quantity": 6})
	secondOrder := decodePaperOrderResponse(t, second).Order
	if second.Code != http.StatusOK || secondOrder.Status != paperStatusSimulatedFill || secondOrder.FillQty != 10 || secondOrder.RemainingQty != 0 || len(secondOrder.Fills) != 2 {
		t.Fatalf("unexpected completing fill: status=%d order=%+v", second.Code, secondOrder)
	}
	if secondOrder.Fills[0].Sequence != 1 || secondOrder.Fills[1].Sequence != 2 || secondOrder.Fills[0].Quantity != 4 || secondOrder.Fills[1].Quantity != 6 {
		t.Fatalf("invalid immutable fill sequence: %+v", secondOrder.Fills)
	}
	if err := database.DB.Where("currency = ? AND symbol = ?", "USD", "NVDA").First(&position).Error; err != nil {
		t.Fatal(err)
	}
	if position.Quantity != 10 || position.ReservedQuantity != 10 {
		t.Fatalf("completed fill/OCO position mismatch: %+v", position)
	}
}

func TestProtectiveSellPartialFillsResizeSiblingUntilCompletion(t *testing.T) {
	router, fixture := setupPaperOrderTestRouter(t)
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("partial-exit-parent")); got.Code != http.StatusCreated {
		t.Fatalf("submit status=%d body=%s", got.Code, got.Body.String())
	}
	parent := decodePaperOrderResponse(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders/partial-exit-parent/simulated-fill", nil)).Order
	var stop, sibling models.PaperOrder
	if err := database.DB.Where("parent_order_id = ? AND order_type = ?", parent.ID, "STOP_MARKET").First(&stop).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Where("parent_order_id = ? AND order_type = ?", parent.ID, "LIMIT").First(&sibling).Error; err != nil {
		t.Fatal(err)
	}
	fixture.setPrice("NVDA", 97)
	first := paperRequest(t, router, http.MethodPost, "/api/paper-orders/"+stop.ClientOrderID+"/simulated-fill", map[string]any{"fillId": "exit-partial-1", "quantity": 4})
	firstOrder := decodePaperOrderResponse(t, first).Order
	if first.Code != http.StatusOK || firstOrder.Status != paperStatusAccepted || firstOrder.FillQty != 4 || firstOrder.RemainingQty != 6 || len(firstOrder.Fills) != 1 {
		t.Fatalf("protective partial fill status=%d order=%+v", first.Code, firstOrder)
	}
	if err := database.DB.First(&sibling, sibling.ID).Error; err != nil {
		t.Fatal(err)
	}
	if sibling.Status != paperStatusAccepted || sibling.Quantity != 6 || sibling.FillQty != 0 || sibling.RemainingQty != 6 {
		t.Fatalf("OCO sibling did not track remaining protection: %+v", sibling)
	}
	var position models.PaperPosition
	if err := database.DB.Where("currency = ? AND symbol = ?", "USD", "NVDA").First(&position).Error; err != nil {
		t.Fatal(err)
	}
	if position.Quantity != 6 || position.ReservedQuantity != 6 {
		t.Fatalf("partial protective exit ledger=%+v", position)
	}
	if err := database.DB.First(&parent, parent.ID).Error; err != nil {
		t.Fatal(err)
	}
	if parent.ProtectionRemainingQty != 6 {
		t.Fatalf("parent protection remaining=%d want=6", parent.ProtectionRemainingQty)
	}

	replay := decodePaperOrderResponse(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders/"+stop.ClientOrderID+"/simulated-fill", map[string]any{"fillId": "exit-partial-1", "quantity": 4}))
	if !replay.IdempotentReplay || replay.Order.FillQty != 4 || len(replay.Order.Fills) != 1 {
		t.Fatalf("protective partial replay=%+v", replay)
	}
	completed := paperRequest(t, router, http.MethodPost, "/api/paper-orders/"+stop.ClientOrderID+"/simulated-fill", map[string]any{"fillId": "exit-partial-2", "quantity": 6})
	completedOrder := decodePaperOrderResponse(t, completed).Order
	if completed.Code != http.StatusOK || completedOrder.Status != paperStatusSimulatedFill || completedOrder.FillQty != 10 || completedOrder.RemainingQty != 0 || len(completedOrder.Fills) != 2 {
		t.Fatalf("protective completion status=%d order=%+v", completed.Code, completedOrder)
	}
	if err := database.DB.First(&sibling, sibling.ID).Error; err != nil {
		t.Fatal(err)
	}
	if sibling.Status != paperStatusCancelled {
		t.Fatalf("completed OCO leg did not cancel sibling: %+v", sibling)
	}
	if err := database.DB.Where("currency = ? AND symbol = ?", "USD", "NVDA").First(&position).Error; err != nil {
		t.Fatal(err)
	}
	if position.Quantity != 0 || position.ReservedQuantity != 0 {
		t.Fatalf("completed protective exit ledger=%+v", position)
	}
	var chain []models.AuditEvent
	if err := database.DB.Order("id asc").Find(&chain).Error; err != nil {
		t.Fatal(err)
	}
	if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, chain); !valid {
		t.Fatalf("protective partial audit chain invalid: %s", reason)
	}
}

func TestAuditVerifyFailsWhenPersistedPaperFillIsTampered(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("paper-fill-tamper")); got.Code != http.StatusCreated {
		t.Fatalf("submit status=%d body=%s", got.Code, got.Body.String())
	}
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders/paper-fill-tamper/simulated-fill", nil); got.Code != http.StatusOK {
		t.Fatalf("fill status=%d body=%s", got.Code, got.Body.String())
	}
	var events []models.AuditEvent
	if err := database.DB.Order("id asc").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, events); !valid {
		t.Fatalf("untampered fill evidence invalid: %s", reason)
	}
	if err := database.DB.Exec("UPDATE paper_fills SET price = price + 1 WHERE fill_id = ?", "manual-full:paper-fill-tamper").Error; err != nil {
		t.Fatal(err)
	}
	if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, events); valid || !strings.Contains(reason, "payload hash mismatch") {
		t.Fatalf("tampered fill evidence valid=%t reason=%q", valid, reason)
	}
}

func TestAuditVerifyResolvesLegacyFillQuoteReferenceAcrossLaterOrderVersions(t *testing.T) {
	router, fixture := setupPaperOrderTestRouter(t)
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("paper-legacy-fill-audit")); got.Code != http.StatusCreated {
		t.Fatalf("submit status=%d body=%s", got.Code, got.Body.String())
	}
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders/paper-legacy-fill-audit/simulated-fill", nil); got.Code != http.StatusOK {
		t.Fatalf("fill status=%d body=%s", got.Code, got.Body.String())
	}
	var order models.PaperOrder
	if err := database.DB.Where("client_order_id = ?", "paper-legacy-fill-audit").First(&order).Error; err != nil {
		t.Fatal(err)
	}
	payloadHash := paperSystemAuditHash(paperFillQuoteAuditPayload(order))
	if err := appendCompletedSystemAuditActorTx(database.DB, "legacy-fill-audit", "system:test", "paper-order.legacy-fill", paperFillQuoteAuditTarget(order), payloadHash, paperSystemAuditHash("ok"), fixture.now); err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Model(&models.PaperOrder{}).Where("id = ?", order.ID).Update("version", order.Version+1).Error; err != nil {
		t.Fatal(err)
	}
	var events []models.AuditEvent
	if err := database.DB.Order("id asc").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, events); !valid {
		t.Fatalf("legacy fill evidence failed after legitimate order version advance: %s", reason)
	}
	tamperedQuote := *order.FillQuote
	tamperedQuote.Price++
	if err := database.DB.Model(&models.PaperOrder{}).Where("id = ?", order.ID).
		Select("FillQuote").Updates(&models.PaperOrder{FillQuote: &tamperedQuote}).Error; err != nil {
		t.Fatal(err)
	}
	if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, events); valid || !strings.Contains(reason, "legacy paper fill quote payload hash mismatch") {
		t.Fatalf("tampered legacy fill quote valid=%t reason=%q", valid, reason)
	}
}

func TestPaperDailyLossUsesDayStartEquityIncludingUnrealizedPnL(t *testing.T) {
	cfg := &config.Config{PaperRiskPolicyVersion: "daily-equity-v13", PaperMaxDailyLossPct: 0.5}
	router, fixture := setupPaperOrderTestRouterWithConfig(t, cfg)
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("paper-day-start")); got.Code != http.StatusCreated {
		t.Fatalf("submit status=%d body=%s", got.Code, got.Body.String())
	}
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders/paper-day-start/simulated-fill", nil); got.Code != http.StatusOK {
		t.Fatalf("fill status=%d body=%s", got.Code, got.Body.String())
	}
	fixture.setPrice("NVDA", 90)
	summary := decodePaperPortfolio(t, paperRequest(t, router, http.MethodGet, "/api/paper-orders/portfolio-risk", nil))
	group := findPaperRiskGroup(summary.Groups, "USD")
	if group == nil || !group.DailyRiskComplete || group.DayStartEquity == nil || *group.DayStartEquity != 10_000 || group.DailyPnL == nil || *group.DailyPnL != -101 || group.DailyLossPct == nil || *group.DailyLossPct != 1.01 {
		t.Fatalf("day-start/unrealized daily risk is incorrect: %+v", group)
	}
	blocked := validPaperOrderPayload("paper-daily-loss-block")
	blocked["referencePrice"] = 90.0
	blocked["entry"] = 90.0
	blocked["protectiveStop"] = 88.0
	risk := blocked["riskSnapshot"].(map[string]any)
	risk["maxLoss"] = 20.0
	risk["plannedNotional"] = 900.0
	response := paperRequest(t, router, http.MethodPost, "/api/paper-orders", blocked)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "daily equity loss") {
		t.Fatalf("daily equity loss did not fail BUY closed: status=%d body=%s", response.Code, response.Body.String())
	}
	var baselines int64
	if err := database.DB.Model(&models.PaperDailyEquityBaseline{}).Where("currency = ?", "USD").Count(&baselines).Error; err != nil {
		t.Fatal(err)
	}
	if baselines != 1 {
		t.Fatalf("daily baseline must be immutable per market date; rows=%d", baselines)
	}
}

func TestPaperRealizedPnLTodayUsesIndividualSellFillTradingDays(t *testing.T) {
	router, fixture := setupPaperOrderTestRouter(t)
	fixture.mu.Lock()
	fixture.now = time.Date(2026, 9, 9, 15, 0, 0, 0, time.UTC)
	now := fixture.now
	fixture.mu.Unlock()

	yesterday := now.Add(-24 * time.Hour)
	completedAt := now
	completed := models.PaperOrder{
		ClientOrderID: "cross-day-completed-sell", Environment: paperEnvironment, Symbol: "NVDA",
		Side: "SELL", Market: "US", Currency: "USD", OrderType: "LIMIT", TimeInForce: "GTC",
		Quantity: 2, FillQty: 2, RemainingQty: 0, RealizedPnL: 30, Status: paperStatusSimulatedFill,
		FilledAt: &completedAt, RequestHash: "cross-day-completed-sell", Version: 3,
	}
	if err := database.DB.Create(&completed).Error; err != nil {
		t.Fatal(err)
	}
	partial := models.PaperOrder{
		ClientOrderID: "today-partial-sell", Environment: paperEnvironment, Symbol: "NVDA",
		Side: "SELL", Market: "US", Currency: "USD", OrderType: "LIMIT", TimeInForce: "GTC",
		Quantity: 2, FillQty: 1, RemainingQty: 1, RealizedPnL: 7, Status: paperStatusAccepted,
		RequestHash: "today-partial-sell", Version: 2,
	}
	if err := database.DB.Create(&partial).Error; err != nil {
		t.Fatal(err)
	}
	quote := models.PaperFillQuote{Price: 110, Source: "test", ObservedAt: now}
	fills := []models.PaperFill{
		{CreatedAt: yesterday, FillID: "cross-day-yesterday", PaperOrderID: completed.ID, Sequence: 1, Quantity: 1, Price: 105, Quote: quote, RealizedPnL: 10},
		{CreatedAt: now, FillID: "cross-day-today", PaperOrderID: completed.ID, Sequence: 2, Quantity: 1, Price: 110, Quote: quote, RealizedPnL: 20},
		{CreatedAt: now, FillID: "partial-today", PaperOrderID: partial.ID, Sequence: 1, Quantity: 1, Price: 107, Quote: quote, RealizedPnL: 7},
	}
	if err := database.DB.Create(&fills).Error; err != nil {
		t.Fatal(err)
	}

	summary := decodePaperPortfolio(t, paperRequest(t, router, http.MethodGet, "/api/paper-orders/portfolio-risk", nil))
	usd := findPaperRiskGroup(summary.Groups, "USD")
	if usd == nil || usd.RealizedPnLToday != 27 {
		t.Fatalf("realized P&L must use today's individual SELL fills, not terminal order totals: %+v", usd)
	}
}

func TestManualSellResizesAndCancellationRestoresAutomaticProtection(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("protected-parent")); got.Code != http.StatusCreated {
		t.Fatalf("buy submit status=%d body=%s", got.Code, got.Body.String())
	}
	filled := paperRequest(t, router, http.MethodPost, "/api/paper-orders/protected-parent/simulated-fill", nil)
	parent := decodePaperOrderResponse(t, filled).Order
	if filled.Code != http.StatusOK {
		t.Fatalf("buy fill status=%d body=%s", filled.Code, filled.Body.String())
	}
	manualPayload := protectiveSellPayload("manual-exit", "NVDA", 4, 98)
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", manualPayload); got.Code != http.StatusCreated {
		t.Fatalf("manual SELL status=%d body=%s", got.Code, got.Body.String())
	}
	var children []models.PaperOrder
	if err := database.DB.Where("parent_order_id = ? AND status = ?", parent.ID, paperStatusAccepted).Order("id asc").Find(&children).Error; err != nil {
		t.Fatal(err)
	}
	if len(children) != 2 || children[0].Quantity != 6 || children[1].Quantity != 6 || children[0].ReservedQty+children[1].ReservedQty != 6 {
		t.Fatalf("OCO children did not yield one reservation to manual SELL: %+v", children)
	}
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders/manual-exit/cancel", nil); got.Code != http.StatusOK {
		t.Fatalf("manual cancel status=%d body=%s", got.Code, got.Body.String())
	}
	for index := range children {
		if err := database.DB.First(&children[index], children[index].ID).Error; err != nil {
			t.Fatal(err)
		}
	}
	if children[0].Status != paperStatusAccepted || children[1].Status != paperStatusAccepted || children[0].Quantity != 10 || children[1].Quantity != 10 || children[0].ReservedQty+children[1].ReservedQty != 10 {
		t.Fatalf("OCO coverage was not restored: %+v", children)
	}
	var position models.PaperPosition
	if err := database.DB.Where("currency = ? AND symbol = ?", "USD", "NVDA").First(&position).Error; err != nil {
		t.Fatal(err)
	}
	if position.ReservedQuantity != 10 || position.ReservedQuantity > position.Quantity {
		t.Fatalf("reservation invariant failed: %+v", position)
	}
	var chain []models.AuditEvent
	if err := database.DB.Order("id asc").Find(&chain).Error; err != nil {
		t.Fatal(err)
	}
	resizeAudits := 0
	for _, event := range chain {
		if event.Action == "paper-order.auto-child.resize" {
			resizeAudits++
			if event.HashVersion != systemAuditHashVersion || event.PayloadHash == "" || event.OutcomeHash == "" {
				t.Fatalf("resize audit is not complete v3: %+v", event)
			}
		}
	}
	if resizeAudits != 4 {
		t.Fatalf("resize audit count=%d want=4", resizeAudits)
	}
	if valid, _, reason, _ := verifyAuditEvents(chain); !valid {
		t.Fatalf("resize audit chain invalid: %s", reason)
	}
}

func TestCancellingAutomaticOCOLegCancelsBothWithVerifiableV3Audits(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("cancel-oco-parent")); got.Code != http.StatusCreated {
		t.Fatalf("buy submit status=%d body=%s", got.Code, got.Body.String())
	}
	parent := decodePaperOrderResponse(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders/cancel-oco-parent/simulated-fill", nil)).Order
	var takeProfit models.PaperOrder
	if err := database.DB.Where("parent_order_id = ? AND order_type = ? AND status = ?", parent.ID, "LIMIT", paperStatusAccepted).First(&takeProfit).Error; err != nil {
		t.Fatal(err)
	}
	response := paperRequest(t, router, http.MethodPost, "/api/paper-orders/"+takeProfit.ClientOrderID+"/cancel", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("cancel OCO status=%d body=%s", response.Code, response.Body.String())
	}
	var children []models.PaperOrder
	if err := database.DB.Where("parent_order_id = ?", parent.ID).Order("id asc").Find(&children).Error; err != nil {
		t.Fatal(err)
	}
	if len(children) != 2 || children[0].Status != paperStatusCancelled || children[1].Status != paperStatusCancelled {
		t.Fatalf("OCO cancellation did not terminate both legs: %+v", children)
	}
	var position models.PaperPosition
	if err := database.DB.Where("currency = ? AND symbol = ?", "USD", "NVDA").First(&position).Error; err != nil {
		t.Fatal(err)
	}
	if position.Quantity != 10 || position.ReservedQuantity != 0 {
		t.Fatalf("OCO cancellation inventory mismatch: %+v", position)
	}
	var chain []models.AuditEvent
	if err := database.DB.Order("id asc").Find(&chain).Error; err != nil {
		t.Fatal(err)
	}
	cancelAudits := 0
	for _, event := range chain {
		if event.Action == "paper-order.auto-child.cancel" && event.HashVersion == systemAuditHashVersion && event.PayloadHash != "" && event.OutcomeHash != "" {
			cancelAudits++
		}
	}
	if cancelAudits != 2 {
		t.Fatalf("v3 child cancellation audits=%d want=2", cancelAudits)
	}
	if valid, _, reason, _ := verifyAuditEvents(chain); !valid {
		t.Fatalf("OCO cancellation audit chain invalid: %s", reason)
	}
}

func TestClosedProtectionCapacityIsNotReusedByLaterBuy(t *testing.T) {
	router, fixture := setupPaperOrderTestRouter(t)
	firstPayload := validPaperOrderPayload("first-cycle-buy")
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", firstPayload); got.Code != http.StatusCreated {
		t.Fatalf("first buy: %s", got.Body.String())
	}
	first := decodePaperOrderResponse(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders/first-cycle-buy/simulated-fill", nil)).Order
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", protectiveSellPayload("first-cycle-close", "NVDA", 10, 98)); got.Code != http.StatusCreated {
		t.Fatalf("close submit: %s", got.Body.String())
	}
	fixture.setPrice("NVDA", 98)
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders/first-cycle-close/simulated-fill", nil); got.Code != http.StatusOK {
		t.Fatalf("close fill: %s", got.Body.String())
	}
	fixture.setPrice("NVDA", 100)
	secondPayload := validPaperOrderPayload("second-cycle-buy")
	secondPayload["protectiveStop"] = 90.0
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", secondPayload); got.Code != http.StatusCreated {
		t.Fatalf("second buy: %s", got.Body.String())
	}
	second := decodePaperOrderResponse(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders/second-cycle-buy/simulated-fill", nil)).Order
	var active []models.PaperOrder
	if err := database.DB.Where("status = ? AND parent_order_id IS NOT NULL", paperStatusAccepted).Find(&active).Error; err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 {
		t.Fatalf("later position inherited stale protection: first=%+v second=%+v children=%+v", first, second, active)
	}
	for _, child := range active {
		if child.ParentOrderID == nil || *child.ParentOrderID != second.ID {
			t.Fatalf("later position inherited stale protection: first=%+v second=%+v children=%+v", first, second, active)
		}
	}
	var activeStop models.PaperOrder
	if err := database.DB.Where("parent_order_id = ? AND status = ? AND order_type = ?", second.ID, paperStatusAccepted, "STOP_MARKET").First(&activeStop).Error; err != nil || activeStop.TriggerPrice == nil || *activeStop.TriggerPrice != 90 {
		t.Fatalf("later position inherited stale protection: first=%+v second=%+v children=%+v", first, second, active)
	}
	if err := database.DB.First(&first, first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if first.ProtectionRemainingQty != 0 || !first.ProtectionInitialized {
		t.Fatalf("first protection capacity not consumed: %+v", first)
	}
}

func TestPaperBuyFailsClosedWhenAuthoritativeADVIsUnavailable(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	previousADV := paperADVLoader
	previousUniverse := paperUniverseLoader
	paperADVLoader = func(string, []string) (map[string]float64, string, error) {
		return nil, "", errors.New("no reviewed ADV artifact")
	}
	paperUniverseLoader = func(string) ([]models.Stock, string, error) {
		return []models.Stock{{Symbol: "NVDA"}}, "test-missing-sector", nil
	}
	defer func() {
		paperADVLoader = previousADV
		paperUniverseLoader = previousUniverse
	}()
	response := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("missing-adv"))
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing ADV status=%d body=%s", response.Code, response.Body.String())
	}
	order := decodePaperOrderResponse(t, response).Order
	if !strings.Contains(order.RejectionReason, "average daily volume") || order.RiskSnapshot.AverageDailyVolume != nil {
		t.Fatalf("missing ADV was not explicit: %+v", order)
	}
	if err := database.DB.Create(&models.PaperPosition{Currency: "USD", Symbol: "NVDA", Quantity: 10, AverageCost: 100, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	portfolio := decodePaperPortfolio(t, paperRequest(t, router, http.MethodGet, "/api/paper-orders/portfolio-risk", nil))
	group := findPaperRiskGroup(portfolio.Groups, "USD")
	if group == nil {
		t.Fatal("USD stress group missing")
	}
	statuses := map[string]string{}
	for _, scenario := range group.StressScenarios {
		statuses[scenario.Kind] = scenario.Status
	}
	if statuses["sector-concentration"] != "unknown" || statuses["liquidity"] != "unknown" {
		t.Fatalf("missing data was fabricated in stress scenarios: %+v", group.StressScenarios)
	}
}

func TestPaperPortfolioReportsVerifiablePnLPeakDrawdownAndStress(t *testing.T) {
	router, fixture := setupPaperOrderTestRouter(t)
	if err := database.DB.Create(&models.PaperPosition{Currency: "USD", Symbol: "NVDA", Quantity: 10, AverageCost: 100, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	fixture.setPrice("NVDA", 110)
	high := decodePaperPortfolio(t, paperRequest(t, router, http.MethodGet, "/api/paper-orders/portfolio-risk", nil))
	usdHigh := findPaperRiskGroup(high.Groups, "USD")
	if usdHigh == nil || usdHigh.PeakEquity == nil || usdHigh.UnrealizedPnL == nil || *usdHigh.UnrealizedPnL != 100 {
		t.Fatalf("high-water metrics missing: %+v", usdHigh)
	}
	if written, err := persistPaperEquityCheckpoints(database.DB, high, high.GeneratedAt); err != nil || written != 2 {
		t.Fatalf("explicit high-water checkpoint failed: written=%d err=%v", written, err)
	}
	fixture.setPrice("NVDA", 90)
	low := decodePaperPortfolio(t, paperRequest(t, router, http.MethodGet, "/api/paper-orders/portfolio-risk", nil))
	usdLow := findPaperRiskGroup(low.Groups, "USD")
	if usdLow == nil || usdLow.PeakDrawdown == nil || *usdLow.PeakDrawdown != 200 || len(usdLow.StressScenarios) != 4 {
		t.Fatalf("drawdown/stress metrics missing: %+v", usdLow)
	}
	if usdLow.StressScenarios[2].Kind != "sector-concentration" || usdLow.StressScenarios[2].Status != "known" || usdLow.StressScenarios[3].Kind != "liquidity" || usdLow.StressScenarios[3].LiquidityUsagePct == nil {
		t.Fatalf("sector/liquidity stress metrics missing: %+v", usdLow.StressScenarios)
	}
}

func TestPaperPortfolioRiskGETIsPureRead(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	if err := database.DB.Where("1 = 1").Delete(&models.PaperDailyEquityBaseline{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&models.PaperPosition{Currency: "USD", Symbol: "NVDA", Quantity: 10, AverageCost: 100, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		response := paperRequest(t, router, http.MethodGet, "/api/paper-orders/portfolio-risk", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %d status=%d body=%s", index, response.Code, response.Body.String())
		}
	}
	var checkpoints int64
	if err := database.DB.Model(&models.PaperEquityCheckpoint{}).Count(&checkpoints).Error; err != nil {
		t.Fatal(err)
	}
	if checkpoints != 0 {
		t.Fatalf("portfolio risk GET wrote %d equity checkpoints", checkpoints)
	}
	var dailyBaselines int64
	if err := database.DB.Model(&models.PaperDailyEquityBaseline{}).Count(&dailyBaselines).Error; err != nil {
		t.Fatal(err)
	}
	if dailyBaselines != 0 {
		t.Fatalf("portfolio risk GET wrote %d daily equity baselines", dailyBaselines)
	}
}

func TestManualPaperFillUsesCoveredRegularSessionCalendar(t *testing.T) {
	router, fixture := setupPaperOrderTestRouter(t)
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("calendar-fill")); got.Code != http.StatusCreated {
		t.Fatalf("submit status=%d body=%s", got.Code, got.Body.String())
	}

	fixture.mu.Lock()
	fixture.now = time.Date(2026, 9, 7, 14, 0, 0, 0, time.UTC) // Labor Day, 10:00 ET.
	fixture.mu.Unlock()
	closed := paperRequest(t, router, http.MethodPost, "/api/paper-orders/calendar-fill/simulated-fill", nil)
	if closed.Code != http.StatusConflict || !strings.Contains(closed.Body.String(), "outside the covered regular trading session") {
		t.Fatalf("holiday fill status=%d body=%s", closed.Code, closed.Body.String())
	}
	var order models.PaperOrder
	if err := database.DB.Where("client_order_id = ?", "calendar-fill").First(&order).Error; err != nil {
		t.Fatal(err)
	}
	if order.Status != paperStatusAccepted {
		t.Fatalf("holiday fill changed order state: %+v", order)
	}

	fixture.mu.Lock()
	fixture.now = time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	fixture.mu.Unlock()
	open := paperRequest(t, router, http.MethodPost, "/api/paper-orders/calendar-fill/simulated-fill", nil)
	response := decodePaperOrderResponse(t, open)
	if open.Code != http.StatusOK || response.Order.Status != paperStatusSimulatedFill || !response.EquityCheckpointed {
		t.Fatalf("regular-session fill status=%d response=%+v", open.Code, response)
	}
}

func TestPaperRiskPolicyHardGatesAreVersionedAndAuditable(t *testing.T) {
	tests := []struct {
		name     string
		fragment string
		prepare  func(*testing.T, *paperQuoteFixture)
		config   config.Config
	}{
		{
			name: "minimum stop coverage", fragment: "stop coverage",
			prepare: func(t *testing.T, _ *paperQuoteFixture) {
				if err := database.DB.Create(&models.PaperPosition{Currency: "USD", Symbol: "NVDA", Quantity: 10, AverageCost: 100, Version: 1}).Error; err != nil {
					t.Fatal(err)
				}
			},
			config: config.Config{PaperRiskPolicyVersion: "test-policy-v8", PaperMinStopCoveragePct: 100},
		},
		{
			name: "maximum daily loss", fragment: "daily equity loss",
			prepare: func(t *testing.T, fixture *paperQuoteFixture) {
				baseline := models.PaperDailyEquityBaseline{Currency: "USD", MarketDate: paperMarketDateForCurrency("USD", fixture.now), ObservedAt: fixture.now, Equity: 10_400}
				if err := database.DB.Model(&models.PaperDailyEquityBaseline{}).
					Where("currency = ? AND market_date = ?", baseline.Currency, baseline.MarketDate).
					Updates(map[string]any{"observed_at": baseline.ObservedAt, "equity": baseline.Equity}).Error; err != nil {
					t.Fatal(err)
				}
			},
			config: config.Config{PaperRiskPolicyVersion: "test-policy-v8", PaperMaxDailyLossPct: 1},
		},
		{
			name: "maximum drawdown", fragment: "peak drawdown",
			prepare: func(t *testing.T, fixture *paperQuoteFixture) {
				if err := database.DB.Create(&models.PaperEquityCheckpoint{ObservedAt: fixture.now.Add(-time.Hour), Currency: "USD", Equity: 12_000}).Error; err != nil {
					t.Fatal(err)
				}
			},
			config: config.Config{PaperRiskPolicyVersion: "test-policy-v8", PaperMaxDrawdownPct: 10},
		},
		{
			name: "maximum stress loss", fragment: "stress loss",
			prepare: func(*testing.T, *paperQuoteFixture) {},
			config:  config.Config{PaperRiskPolicyVersion: "test-policy-v8", PaperMaxStressLossPct: 1},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router, fixture := setupPaperOrderTestRouterWithConfig(t, &test.config)
			test.prepare(t, fixture)
			response := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("policy-"+strings.ReplaceAll(test.name, " ", "-")))
			orderResponse := decodePaperOrderResponse(t, response)
			if response.Code != http.StatusUnprocessableEntity || orderResponse.Order.Status != paperStatusRejected {
				t.Fatalf("policy gate status=%d response=%+v", response.Code, orderResponse)
			}
			if orderResponse.Order.RiskSnapshot.PolicyVersion != "test-policy-v8" {
				t.Fatalf("policy version missing: %+v", orderResponse.Order.RiskSnapshot)
			}
			assertReasonContains(t, orderResponse.Order.RiskSnapshot.PolicyReasons, test.fragment)
		})
	}
}

func TestPaperBuyFillRechecksCurrentPolicyAndKeepsAcceptedReservation(t *testing.T) {
	cfg := &config.Config{PaperRiskPolicyVersion: "submit-v9", PaperMaxDailyLossPct: 10}
	router, fixture := setupPaperOrderTestRouterWithConfig(t, cfg)
	created := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("fill-recheck"))
	if created.Code != http.StatusCreated {
		t.Fatalf("submit status=%d body=%s", created.Code, created.Body.String())
	}
	createdOrder := decodePaperOrderResponse(t, created).Order
	if createdOrder.RiskSnapshot.SubmissionPolicyCheck == nil || createdOrder.RiskSnapshot.SubmissionPolicyCheck.PolicyVersion != "submit-v9" || !createdOrder.RiskSnapshot.SubmissionPolicyCheck.Passed {
		t.Fatalf("submission policy check missing: %+v", createdOrder.RiskSnapshot)
	}

	cfg.PaperRiskPolicyVersion = "fill-v9"
	cfg.PaperMaxDailyLossPct = 1
	if err := database.DB.Model(&models.PaperDailyEquityBaseline{}).
		Where("currency = ? AND market_date = ?", "USD", paperMarketDateForCurrency("USD", fixture.now)).
		Update("equity", 10_200.0).Error; err != nil {
		t.Fatal(err)
	}

	rejected := paperRequest(t, router, http.MethodPost, "/api/paper-orders/fill-recheck/simulated-fill", nil)
	if rejected.Code != http.StatusUnprocessableEntity {
		t.Fatalf("fill recheck status=%d body=%s", rejected.Code, rejected.Body.String())
	}
	response := decodePaperOrderResponse(t, rejected)
	if response.Order.Status != paperStatusAccepted || response.Order.FillQty != 0 || response.Order.RiskSnapshot.FillPolicyCheck == nil {
		t.Fatalf("rejected fill changed terminal state: %+v", response.Order)
	}
	check := response.Order.RiskSnapshot.FillPolicyCheck
	if check.PolicyVersion != "fill-v9" || check.Passed || !check.CheckedAt.Equal(fixture.now.UTC()) {
		t.Fatalf("current fill policy check missing: %+v", check)
	}
	assertReasonContains(t, check.Reasons, "daily equity loss")
	if response.Order.RiskSnapshot.SubmissionPolicyCheck == nil || response.Order.RiskSnapshot.SubmissionPolicyCheck.PolicyVersion != "submit-v9" {
		t.Fatalf("submission check was overwritten: %+v", response.Order.RiskSnapshot)
	}
	var account models.PaperAccount
	if err := database.DB.Where("currency = ?", "USD").First(&account).Error; err != nil {
		t.Fatal(err)
	}
	if account.Cash != 8_999 || account.ReservedCash != 1_001 {
		t.Fatalf("risk rejection released or consumed reservation: %+v", account)
	}
}

func TestManualPaperBuyStopGapRechecksCurrentSingleOrderLossLimit(t *testing.T) {
	router, fixture := setupPaperOrderTestRouter(t)
	payload := validPaperOrderPayload("manual-gap-loss")
	payload["orderType"] = "STOP_MARKET"
	payload["entry"] = nil
	payload["triggerPrice"] = 101.0
	payload["protectiveStop"] = 98.0
	payload["takeProfit"] = 130.0
	created := paperRequest(t, router, http.MethodPost, "/api/paper-orders", payload)
	if created.Code != http.StatusCreated {
		t.Fatalf("submit status=%d body=%s", created.Code, created.Body.String())
	}
	accepted := decodePaperOrderResponse(t, created).Order
	fixture.setPrice("NVDA", 115)
	rejected := paperRequest(t, router, http.MethodPost, "/api/paper-orders/manual-gap-loss/simulated-fill", nil)
	if rejected.Code != http.StatusUnprocessableEntity {
		t.Fatalf("gap fill status=%d body=%s", rejected.Code, rejected.Body.String())
	}
	response := decodePaperOrderResponse(t, rejected)
	if response.Order.Status != paperStatusAccepted || response.Order.FillQty != 0 || response.Order.RiskSnapshot.FillPolicyCheck == nil {
		t.Fatalf("gap rejection changed order state: %+v", response.Order)
	}
	assertReasonContains(t, response.Order.RiskSnapshot.FillPolicyCheck.Reasons, "max loss exceeds")
	if response.Order.RiskSnapshot.MaxLoss <= response.Order.RiskSnapshot.MaxRiskAmount || response.Order.RiskSnapshot.PlannedNotional <= accepted.RiskSnapshot.PlannedNotional {
		t.Fatalf("fill risk was not recalculated from gap price: before=%+v after=%+v", accepted.RiskSnapshot, response.Order.RiskSnapshot)
	}
	var account models.PaperAccount
	if err := database.DB.Where("currency = ?", "USD").First(&account).Error; err != nil {
		t.Fatal(err)
	}
	if account.Cash != 8_989 || account.ReservedCash != 1_011 {
		t.Fatalf("gap rejection consumed or released reservation: %+v", account)
	}
}

func TestPaperFillPolicyFailsClosedOnStaleOrIncompleteExternalSnapshot(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*testing.T, *paperPortfolioMarketSnapshot, *paperQuoteFixture)
		reason string
	}{
		{
			name: "snapshot expired",
			mutate: func(_ *testing.T, snapshot *paperPortfolioMarketSnapshot, fixture *paperQuoteFixture) {
				snapshot.CapturedAt = fixture.now.Add(-2 * marketQuoteFreshFor)
			},
			reason: "snapshot age",
		},
		{
			name: "latest ledger symbol missing",
			mutate: func(t *testing.T, _ *paperPortfolioMarketSnapshot, fixture *paperQuoteFixture) {
				fixture.setPrice("XOM", 100)
				if err := database.DB.Create(&models.PaperPosition{Currency: "USD", Symbol: "XOM", Quantity: 1, AverageCost: 100, Version: 1}).Error; err != nil {
					t.Fatal(err)
				}
			},
			reason: "does not cover latest ledger symbol XOM",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			router, fixture := setupPaperOrderTestRouter(t)
			created := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("snapshot-check"))
			if created.Code != http.StatusCreated {
				t.Fatalf("submit status=%d body=%s", created.Code, created.Body.String())
			}
			snapshot := capturePaperPortfolioMarketSnapshot(database.DB, nil, fixture.now)
			test.mutate(t, &snapshot, fixture)
			var reasons []string
			if err := database.DB.Transaction(func(tx *gorm.DB) error {
				var order models.PaperOrder
				if err := tx.Where("client_order_id = ?", "snapshot-check").First(&order).Error; err != nil {
					return err
				}
				_, reasons = evaluatePaperFillPolicyTx(tx, order, 100, defaultPaperRiskPolicy(), fixture.now, snapshot)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			assertReasonContains(t, reasons, test.reason)
		})
	}
}

func TestPaperSellProtectiveExitBypassesBuyRiskGate(t *testing.T) {
	cfg := &config.Config{PaperRiskPolicyVersion: "sell-exit-v9", PaperMaxDailyLossPct: 1}
	router, fixture := setupPaperOrderTestRouterWithConfig(t, cfg)
	if err := database.DB.Create(&models.PaperPosition{Currency: "USD", Symbol: "NVDA", Quantity: 10, AverageCost: 100, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	filledAt := fixture.now
	if err := database.DB.Create(&models.PaperOrder{
		ClientOrderID: "sell-exit-loss", Environment: paperEnvironment, Symbol: "XOM", Side: "SELL", Market: "US", Currency: "USD",
		OrderType: "STOP_MARKET", TimeInForce: "GTC", Quantity: 1, QuoteSource: "tradingview", QuoteTime: filledAt,
		Status: paperStatusSimulatedFill, StatusHistory: []models.PaperOrderStatusEvent{{Status: paperStatusSimulatedFill, At: filledAt}},
		RequestHash: "sell-exit-loss", RealizedPnL: -500, FilledAt: &filledAt, Version: 2,
	}).Error; err != nil {
		t.Fatal(err)
	}
	created := paperRequest(t, router, http.MethodPost, "/api/paper-orders", protectiveSellPayload("sell-exit", "NVDA", 10, 98))
	if created.Code != http.StatusCreated || decodePaperOrderResponse(t, created).Order.Status != paperStatusAccepted {
		t.Fatalf("protective SELL submit status=%d body=%s", created.Code, created.Body.String())
	}
	fixture.setPrice("NVDA", 97)
	filled := paperRequest(t, router, http.MethodPost, "/api/paper-orders/sell-exit/simulated-fill", nil)
	if filled.Code != http.StatusOK {
		t.Fatalf("protective SELL fill status=%d body=%s", filled.Code, filled.Body.String())
	}
	order := decodePaperOrderResponse(t, filled).Order
	if order.Status != paperStatusSimulatedFill || order.RiskSnapshot.FillPolicyCheck != nil {
		t.Fatalf("BUY policy blocked or decorated protective SELL: %+v", order)
	}
}

func TestPaperPortfolioRiskAggregatesReservationsExposureAndStopCoverage(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("paper-risk-buy")); got.Code != http.StatusCreated {
		t.Fatalf("buy submit status=%d body=%s", got.Code, got.Body.String())
	}
	beforeFill := decodePaperPortfolio(t, paperRequest(t, router, http.MethodGet, "/api/paper-orders/portfolio-risk", nil))
	usd := findPaperRiskGroup(beforeFill.Groups, "USD")
	if usd == nil || usd.TotalEquity == nil || *usd.TotalEquity != 10_000 || usd.ReservedCash != 1_001 || usd.GrossExposure == nil || *usd.GrossExposure != 1_000 {
		t.Fatalf("unexpected reserved portfolio: %+v", usd)
	}

	filled := paperRequest(t, router, http.MethodPost, "/api/paper-orders/paper-risk-buy/simulated-fill", nil)
	if filled.Code != http.StatusOK {
		t.Fatalf("fill status=%d body=%s", filled.Code, filled.Body.String())
	}
	stop := protectiveSellPayload("paper-risk-stop", "NVDA", 10, 98)
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", stop); got.Code != http.StatusCreated || decodePaperOrderResponse(t, got).Order.Status != paperStatusAccepted {
		t.Fatalf("stop submit status=%d body=%s", got.Code, got.Body.String())
	}
	afterStop := decodePaperPortfolio(t, paperRequest(t, router, http.MethodGet, "/api/paper-orders/portfolio-risk", nil))
	usd = findPaperRiskGroup(afterStop.Groups, "USD")
	if usd == nil || usd.PositionQuantity != 10 || usd.StopCoveredQuantity != 10 || usd.StopCoveragePct != 100 || usd.OpenSellNotional != 980 || usd.OpenOrderMaxPlannedLoss != 20 {
		t.Fatalf("unexpected protected portfolio: %+v", usd)
	}
	if len(usd.Symbols) != 1 || usd.Symbols[0].Sector != "Technology" || usd.Symbols[0].StopCoveragePct != 100 {
		t.Fatalf("unexpected symbol risk: %+v", usd.Symbols)
	}
}

func TestPaperPortfolioHardLimitsUseAccumulatedLedger(t *testing.T) {
	t.Run("unknown authoritative sector", func(t *testing.T) {
		router, fixture := setupPaperOrderTestRouter(t)
		fixture.setPrice("META", 100)
		response := decodePaperOrderResponse(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders", paperOrderPayload("unknown-sector", "META", 10, 98)))
		if response.Order.Status != paperStatusRejected || !strings.Contains(response.Order.RejectionReason, "authoritative sector is unknown") {
			t.Fatalf("unknown sector was not hard-blocked: %+v", response.Order)
		}
	})

	t.Run("filled position plus accepted order", func(t *testing.T) {
		router, fixture := setupPaperOrderTestRouter(t)
		fixture.setPrice("NVDA", 100)
		first := decodePaperOrderResponse(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders", paperOrderPayload("position-1", "NVDA", 15, 98)))
		if first.Order.Status != paperStatusAccepted {
			t.Fatalf("initial order rejected: %+v", first.Order)
		}
		if filled := paperRequest(t, router, http.MethodPost, "/api/paper-orders/position-1/simulated-fill", nil); filled.Code != http.StatusOK {
			t.Fatalf("initial fill status=%d body=%s", filled.Code, filled.Body.String())
		}
		second := decodePaperOrderResponse(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders", paperOrderPayload("position-2", "NVDA", 11, 98)))
		if second.Order.Status != paperStatusRejected || !strings.Contains(second.Order.RejectionReason, "single-symbol exposure") {
			t.Fatalf("position plus order concentration not rejected: %+v", second.Order)
		}
	})

	t.Run("single symbol concentration", func(t *testing.T) {
		router, fixture := setupPaperOrderTestRouter(t)
		fixture.setPrice("NVDA", 100)
		for index, id := range []string{"symbol-1", "symbol-2"} {
			response := decodePaperOrderResponse(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders", paperOrderPayload(id, "NVDA", 13, 98)))
			if index == 0 && response.Order.Status != paperStatusAccepted {
				t.Fatalf("first order rejected: %+v", response.Order)
			}
			if index == 1 && (response.Order.Status != paperStatusRejected || !strings.Contains(response.Order.RejectionReason, "single-symbol exposure")) {
				t.Fatalf("concentrated order not rejected: %+v", response.Order)
			}
		}
	})

	t.Run("sector concentration", func(t *testing.T) {
		router, fixture := setupPaperOrderTestRouter(t)
		for _, symbol := range []string{"MSFT", "AMZN", "GOOGL"} {
			fixture.setPrice(symbol, 100)
		}
		setup := []struct {
			symbol   string
			quantity int
		}{{"NVDA", 15}, {"MSFT", 12}, {"AMZN", 10}}
		for _, item := range setup {
			response := decodePaperOrderResponse(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders", paperOrderPayload("sector-"+item.symbol, item.symbol, item.quantity, 98)))
			if response.Order.Status != paperStatusAccepted {
				t.Fatalf("setup order %s rejected: %+v", item.symbol, response.Order)
			}
		}
		fourth := decodePaperOrderResponse(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders", paperOrderPayload("sector-GOOGL", "GOOGL", 9, 98)))
		if fourth.Order.Status != paperStatusRejected || !strings.Contains(fourth.Order.RejectionReason, "sector exposure") {
			t.Fatalf("sector limit did not reject: %+v", fourth.Order)
		}
	})

	t.Run("cumulative open loss", func(t *testing.T) {
		router, fixture := setupPaperOrderTestRouter(t)
		symbols := []string{"NVDA", "XOM", "JPM", "PFE", "WMT", "BA"}
		for _, symbol := range symbols {
			fixture.setPrice(symbol, 100)
		}
		for index, symbol := range symbols {
			response := decodePaperOrderResponse(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders", paperOrderPayload("loss-"+symbol, symbol, 3, 60)))
			if index < 5 && response.Order.Status != paperStatusAccepted {
				t.Fatalf("order %d rejected early: %+v", index, response.Order)
			}
			if index == 5 && (response.Order.Status != paperStatusRejected || !strings.Contains(response.Order.RejectionReason, "open-order max planned loss")) {
				t.Fatalf("cumulative loss did not reject: %+v", response.Order)
			}
		}
	})

	t.Run("gross exposure", func(t *testing.T) {
		router, fixture := setupPaperOrderTestRouter(t)
		symbols := []string{"NVDA", "XOM", "JPM", "PFE", "WMT", "BA", "NEE", "T", "LIN", "PLD", "CAT", "KO"}
		quantities := []int{15, 12, 10, 9, 8, 6, 5, 5, 4, 3, 3, 2}
		for _, symbol := range symbols {
			fixture.setPrice(symbol, 100)
		}
		for index, symbol := range symbols {
			response := decodePaperOrderResponse(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders", paperOrderPayload("gross-"+symbol, symbol, quantities[index], 98)))
			if index < len(symbols)-1 && response.Order.Status != paperStatusAccepted {
				t.Fatalf("order %d rejected early: %+v", index, response.Order)
			}
			if index == len(symbols)-1 && (response.Order.Status != paperStatusRejected || !strings.Contains(response.Order.RejectionReason, "gross exposure")) {
				t.Fatalf("gross exposure did not reject: %+v", response.Order)
			}
		}
	})
}

func TestPaperStopMarketDirectionAndSeparatedProtectionFields(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	buyStop := validPaperOrderPayload("buy-stop")
	buyStop["orderType"] = "STOP_MARKET"
	buyStop["entry"] = nil
	buyStop["triggerPrice"] = 101.0
	buyStop["protectiveStop"] = 98.0
	if validation := decodePaperValidation(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders/validate", buyStop)); !validation.Valid {
		t.Fatalf("BUY stop above market should validate: %+v", validation)
	}
	buyStop["triggerPrice"] = 99.0
	if validation := decodePaperValidation(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders/validate", buyStop)); validation.Valid || !strings.Contains(strings.Join(validation.RejectionReasons, ";"), "above the authoritative quote") {
		t.Fatalf("BUY stop below market was not rejected: %+v", validation)
	}

	if err := database.DB.Create(&models.PaperPosition{Currency: "USD", Symbol: "NVDA", Quantity: 10, AverageCost: 90, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Model(&models.PaperAccount{}).Where("currency = ?", "USD").Update("cash", 0).Error; err != nil {
		t.Fatal(err)
	}
	sellStop := protectiveSellPayload("sell-stop", "NVDA", 2, 98)
	if validation := decodePaperValidation(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders/validate", sellStop)); !validation.Valid {
		t.Fatalf("SELL stop below market should validate: %+v", validation)
	}
	sellStop["triggerPrice"] = 101.0
	if validation := decodePaperValidation(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders/validate", sellStop)); validation.Valid || !strings.Contains(strings.Join(validation.RejectionReasons, ";"), "below the authoritative quote") {
		t.Fatalf("SELL stop above market was not rejected: %+v", validation)
	}

	legacy := validPaperOrderPayload("legacy-stop")
	legacy["stop"] = 98.0
	if validation := decodePaperValidation(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders/validate", legacy)); validation.Valid || !strings.Contains(strings.Join(validation.RejectionReasons, ";"), "stop is ambiguous") {
		t.Fatalf("legacy ambiguous stop was not rejected: %+v", validation)
	}
}

func TestPaperPortfolioFailsBuyClosedButAllowsDegradedProtectiveSell(t *testing.T) {
	router, fixture := setupPaperOrderTestRouter(t)
	fixture.setPrice("AAPL", 100)
	fixture.setPrice("MSFT", 100)
	if err := database.DB.Create(&models.PaperPosition{Currency: "USD", Symbol: "AAPL", Quantity: 10, AverageCost: 90, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&models.PaperPosition{Currency: "USD", Symbol: "NVDA", Quantity: 10, AverageCost: 90, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&models.PaperPosition{Currency: "USD", Symbol: "MSFT", Quantity: 10, AverageCost: 90, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	freshQuotes := boundedQuotesForRequest
	boundedQuotesForRequest = func(symbols []string) quoteFetchResult {
		result := freshQuotes(symbols)
		if _, ok := result.quotes["AAPL"]; ok {
			result.quotes["AAPL"] = market.Quote{Price: 100, Source: "fallback:test"}
		}
		delete(result.quotes, "MSFT")
		return result
	}

	summary := decodePaperPortfolio(t, paperRequest(t, router, http.MethodGet, "/api/paper-orders/portfolio-risk", nil))
	if summary.BaseCurrency.Status != "unknown" || summary.BaseCurrency.TotalEquity != nil || summary.BaseCurrency.RequiredForCurrentPolicy {
		t.Fatalf("unsourced cross-currency total must remain explicitly unknown: %+v", summary.BaseCurrency)
	}
	if summary.AccountModel != "currency-isolated-paper-subaccounts" || !strings.Contains(summary.BaseCurrency.Reason, "isolated Paper subaccounts") {
		t.Fatalf("currency-isolated Paper contract missing: %+v", summary)
	}
	usd := findPaperRiskGroup(summary.Groups, "USD")
	if !summary.Degraded || usd == nil || !usd.Degraded || !containsPaperSymbol(usd.UnknownSymbols, "AAPL") || !containsPaperSymbol(usd.UnknownSymbols, "MSFT") {
		t.Fatalf("fallback holding was not identified: summary=%+v usd=%+v", summary, usd)
	}
	if len(usd.Symbols) != 3 || usd.Symbols[0].ValuationError == "" || usd.Symbols[1].ValuationError == "" {
		t.Fatalf("offending valuation reason missing: %+v", usd.Symbols)
	}

	buy := paperRequest(t, router, http.MethodPost, "/api/paper-orders", paperOrderPayload("degraded-buy", "XOM", 5, 98))
	buyOrder := decodePaperOrderResponse(t, buy).Order
	if buy.Code != http.StatusUnprocessableEntity || buyOrder.Status != paperStatusRejected || !containsPaperSymbol(buyOrder.RiskSnapshot.PortfolioOffendingSymbols, "AAPL") || !containsPaperSymbol(buyOrder.RiskSnapshot.PortfolioOffendingSymbols, "MSFT") {
		t.Fatalf("BUY did not fail closed on degraded holding: status=%d order=%+v", buy.Code, buyOrder)
	}

	sell := paperRequest(t, router, http.MethodPost, "/api/paper-orders", protectiveSellPayload("degraded-sell", "NVDA", 2, 98))
	sellOrder := decodePaperOrderResponse(t, sell).Order
	if sell.Code != http.StatusCreated || sellOrder.Status != paperStatusAccepted || !sellOrder.RiskSnapshot.PortfolioDegraded || !containsPaperSymbol(sellOrder.RiskSnapshot.PortfolioOffendingSymbols, "AAPL") || len(sellOrder.RiskSnapshot.RiskWarnings) == 0 {
		t.Fatalf("protective SELL did not preserve degraded disclosure: status=%d order=%+v", sell.Code, sellOrder)
	}
}

func TestPaperPortfolioUsesOneBatchQuoteSnapshot(t *testing.T) {
	router, fixture := setupPaperOrderTestRouter(t)
	fixture.setPrice("AAPL", 101)
	fixture.setPrice("MSFT", 202)
	for _, position := range []models.PaperPosition{
		{Currency: "USD", Symbol: "AAPL", Quantity: 2, AverageCost: 90, Version: 1},
		{Currency: "USD", Symbol: "MSFT", Quantity: 3, AverageCost: 190, Version: 1},
	} {
		if err := database.DB.Create(&position).Error; err != nil {
			t.Fatal(err)
		}
	}
	fixture.resetCalls()

	summary := decodePaperPortfolio(t, paperRequest(t, router, http.MethodGet, "/api/paper-orders/portfolio-risk", nil))
	calls := fixture.recordedCalls()
	if len(calls) != 1 {
		t.Fatalf("portfolio quote provider calls=%d want=1 calls=%v", len(calls), calls)
	}
	sort.Strings(calls[0])
	if strings.Join(calls[0], ",") != "AAPL,MSFT" {
		t.Fatalf("portfolio batch symbols=%v", calls[0])
	}
	if summary.QuoteObservedAt == nil || summary.QuoteObservedAt.IsZero() {
		t.Fatalf("missing shared quote observed time: %+v", summary)
	}
	usd := findPaperRiskGroup(summary.Groups, "USD")
	if usd == nil || len(usd.Symbols) != 2 || usd.Symbols[0].MarkTime == nil || usd.Symbols[1].MarkTime == nil || !usd.Symbols[0].MarkTime.Equal(*usd.Symbols[1].MarkTime) {
		t.Fatalf("position marks do not share one observed time: %+v", usd)
	}
}

func TestConcurrentPaperOrdersPreserveReservationAndConcentrationLimits(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	statuses := make(chan models.PaperOrder, 2)
	var wg sync.WaitGroup
	for _, id := range []string{"concurrent-1", "concurrent-2"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			response := paperRequest(t, router, http.MethodPost, "/api/paper-orders", paperOrderPayload(id, "NVDA", 13, 98))
			statuses <- decodePaperOrderResponse(t, response).Order
		}(id)
	}
	wg.Wait()
	close(statuses)
	accepted, rejected := 0, 0
	for order := range statuses {
		if order.Status == paperStatusAccepted {
			accepted++
		} else if order.Status == paperStatusRejected && strings.Contains(order.RejectionReason, "single-symbol exposure") {
			rejected++
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatalf("concurrent decisions accepted=%d rejected=%d", accepted, rejected)
	}
	var account models.PaperAccount
	if err := database.DB.Where("currency = ?", "USD").First(&account).Error; err != nil {
		t.Fatal(err)
	}
	if account.Cash+account.ReservedCash != account.InitialCash || account.ReservedCash != 1_301 {
		t.Fatalf("reservation conservation failed: %+v", account)
	}
}
