package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/config"
	"trading-agents/internal/database"
	"trading-agents/internal/market"
	"trading-agents/internal/models"
)

func TestPaperRuntimeFixtureIsDisabledByDefault(t *testing.T) {
	t.Setenv(paperRuntimeFixtureEnv, "")
	enabled, err := ConfigurePaperRuntimeFixtureFromEnvironment(&config.Config{})
	if err != nil || enabled {
		t.Fatalf("default fixture state enabled=%t err=%v", enabled, err)
	}
}

func TestPaperRuntimeFixtureRejectsUnacknowledgedEnablement(t *testing.T) {
	t.Setenv(paperRuntimeFixtureEnv, "true")
	enabled, err := ConfigurePaperRuntimeFixtureFromEnvironment(&config.Config{
		AdminToken: "test-token", DatabaseDir: t.TempDir(),
	})
	if err == nil || enabled {
		t.Fatalf("unacknowledged fixture state enabled=%t err=%v", enabled, err)
	}
}

func TestAuthenticatedPaperRuntimeLifecycleUsesTemporaryDatabase(t *testing.T) {
	db, err := database.OpenSQLite(t.TempDir() + "/authenticated-paper-runtime.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.PaperOrder{}, &models.PaperFill{}, &models.PaperAccount{}, &models.PaperPosition{},
		&models.PaperStopScanAudit{}, &models.PaperEquityCheckpoint{}, &models.PaperDailyEquityBaseline{}, &models.AuditEvent{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PaperAccount{Currency: "USD", InitialCash: 10_000, Cash: 10_000, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	seedPaperDailyBaseline(t, db, "USD", now, 10_000)
	prices := map[string]float64{"NVDA": 100}
	previousDB := database.DB
	previousQuotes := boundedQuotesForRequest
	previousUniverse := paperUniverseLoader
	previousADV := paperADVLoader
	database.DB = db
	boundedQuotesForRequest = func(symbols []string) quoteFetchResult {
		quotes := make(map[string]market.Quote, len(symbols))
		for _, symbol := range symbols {
			if price := prices[strings.ToUpper(symbol)]; price > 0 {
				quotes[strings.ToUpper(symbol)] = market.Quote{
					Price: price, Source: "tradingview", DataTime: now.Format(time.RFC3339),
					TimeGranularity: "second", ProviderURL: "https://quotes.example.test/" + strings.ToUpper(symbol),
				}
			}
		}
		return quoteFetchResult{quotes: quotes, source: "self-provider", dataTime: now}
	}
	paperUniverseLoader = func(string) ([]models.Stock, string, error) {
		return []models.Stock{{Symbol: "NVDA", Sector: "Technology"}}, "fixture", nil
	}
	paperADVLoader = func(string, []string) (map[string]float64, string, error) {
		return map[string]float64{"NVDA": 1_000_000}, "fixture-adv", nil
	}
	t.Cleanup(func() {
		database.DB = previousDB
		boundedQuotesForRequest = previousQuotes
		paperUniverseLoader = previousUniverse
		paperADVLoader = previousADV
	})

	cfg := &config.Config{
		AdminToken: "fixture-token", PaperRiskPolicyVersion: "paper-runtime-v1",
		PaperMinStopCoveragePct: 100, PaperMaxDailyLossPct: 3,
		PaperMaxDrawdownPct: 12, PaperMaxStressLossPct: 10,
	}
	handler := &Handler{config: cfg, paperClock: func() time.Time { return now }}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/paper-orders", adminWriteGuard(cfg, "paper-order.submit"), handler.SubmitPaperOrder)
	router.GET("/api/paper-orders", adminReadGuard(cfg), handler.ListPaperOrders)
	router.POST("/api/paper-orders/:clientOrderId/simulated-fill", adminWriteGuard(cfg, "paper-order.simulated-fill"), handler.SimulatePaperFill)
	router.GET("/api/admin/audit/verify", adminReadGuard(cfg), handler.VerifyAuditChain)
	server := httptest.NewServer(router)
	defer server.Close()

	gapPayload := validPaperOrderPayload("runtime-gap-buy-stop")
	gapPayload["orderType"] = "STOP_MARKET"
	gapPayload["entry"] = nil
	gapPayload["triggerPrice"] = 101.0
	gapPayload["takeProfit"] = 130.0
	status, body := paperRuntimeHTTPRequest(t, server.Client(), http.MethodPost, server.URL+"/api/paper-orders", gapPayload, cfg.AdminToken)
	if status != http.StatusCreated {
		t.Fatalf("gap order submit status=%d body=%s", status, body)
	}
	prices["NVDA"] = 115
	status, body = paperRuntimeHTTPRequest(t, server.Client(), http.MethodPost, server.URL+"/api/paper-orders/runtime-gap-buy-stop/simulated-fill", nil, cfg.AdminToken)
	var rejected paperOrderResponse
	if err := json.Unmarshal(body, &rejected); err != nil {
		t.Fatal(err)
	}
	if status != http.StatusUnprocessableEntity || rejected.Order.Status != paperStatusAccepted || rejected.Order.RiskSnapshot.FillPolicyCheck == nil || rejected.Order.RiskSnapshot.FillPolicyCheck.Passed || rejected.Order.ReservedCash <= 0 {
		t.Fatalf("gap risk rejection was not fail-closed: status=%d response=%+v", status, rejected)
	}
	if !strings.Contains(strings.Join(rejected.Order.RiskSnapshot.FillPolicyCheck.Reasons, ";"), "max loss exceeds") {
		t.Fatalf("gap rejection reason missing: %+v", rejected.Order.RiskSnapshot.FillPolicyCheck)
	}

	prices["NVDA"] = 100
	parentPayload := validPaperOrderPayload("runtime-oco-parent")
	parentPayload["quantity"] = 5
	status, body = paperRuntimeHTTPRequest(t, server.Client(), http.MethodPost, server.URL+"/api/paper-orders", parentPayload, cfg.AdminToken)
	if status != http.StatusCreated {
		t.Fatalf("OCO parent submit status=%d body=%s", status, body)
	}
	status, body = paperRuntimeHTTPRequest(t, server.Client(), http.MethodPost, server.URL+"/api/paper-orders/runtime-oco-parent/simulated-fill", nil, cfg.AdminToken)
	var parent paperOrderResponse
	if err := json.Unmarshal(body, &parent); err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK || parent.Order.Status != paperStatusSimulatedFill || parent.Order.Environment != paperEnvironment || !strings.Contains(parent.Disclaimer, "No broker order") {
		t.Fatalf("Paper parent fill contract missing: status=%d response=%+v", status, parent)
	}
	if parent.Order.FillQuote == nil || parent.Order.FillQuote.Price != 100 || parent.Order.FillQuote.Source != "tradingview" ||
		parent.Order.FillQuote.ProviderURL != "https://quotes.example.test/NVDA" || !parent.Order.FillQuote.ObservedAt.Equal(now) {
		t.Fatalf("Paper fill quote evidence missing: %+v", parent.Order.FillQuote)
	}

	status, body = paperRuntimeHTTPRequest(t, server.Client(), http.MethodGet, server.URL+"/api/paper-orders", nil, cfg.AdminToken)
	var listed struct {
		Environment string              `json:"environment"`
		Orders      []models.PaperOrder `json:"orders"`
		Disclaimer  string              `json:"disclaimer"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatal(err)
	}
	var stopID string
	for _, order := range listed.Orders {
		if order.ParentOrderID != nil && *order.ParentOrderID == parent.Order.ID && order.OrderType == "STOP_MARKET" {
			stopID = order.ClientOrderID
		}
	}
	if status != http.StatusOK || listed.Environment != paperEnvironment || stopID == "" || !strings.Contains(listed.Disclaimer, "No broker order") {
		t.Fatalf("automatic Paper OCO children missing: status=%d response=%+v", status, listed)
	}

	prices["NVDA"] = 97
	status, body = paperRuntimeHTTPRequest(t, server.Client(), http.MethodPost, server.URL+"/api/paper-orders/"+stopID+"/simulated-fill", nil, cfg.AdminToken)
	if status != http.StatusOK {
		t.Fatalf("protective Paper SELL fill status=%d body=%s", status, body)
	}
	status, body = paperRuntimeHTTPRequest(t, server.Client(), http.MethodGet, server.URL+"/api/paper-orders", nil, cfg.AdminToken)
	if status != http.StatusOK || json.Unmarshal(body, &listed) != nil {
		t.Fatalf("list after OCO status=%d body=%s", status, body)
	}
	filledChildren, cancelledChildren := 0, 0
	for _, order := range listed.Orders {
		if order.ParentOrderID == nil || *order.ParentOrderID != parent.Order.ID {
			continue
		}
		if order.Status == paperStatusSimulatedFill {
			filledChildren++
		}
		if order.Status == paperStatusCancelled {
			cancelledChildren++
		}
	}
	if filledChildren != 1 || cancelledChildren != 1 {
		t.Fatalf("OCO terminal states filled=%d cancelled=%d orders=%+v", filledChildren, cancelledChildren, listed.Orders)
	}

	status, body = paperRuntimeHTTPRequest(t, server.Client(), http.MethodGet, server.URL+"/api/admin/audit/verify", nil, cfg.AdminToken)
	var audit struct {
		Valid        bool `json:"valid"`
		Count        int  `json:"count"`
		PendingCount int  `json:"pendingCount"`
	}
	if err := json.Unmarshal(body, &audit); err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK || !audit.Valid || audit.Count == 0 || audit.PendingCount != 0 {
		t.Fatalf("authenticated Paper audit chain invalid: status=%d response=%+v", status, audit)
	}
}

func paperRuntimeHTTPRequest(t *testing.T, client *http.Client, method, target string, payload any, token string) (int, []byte) {
	t.Helper()
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatal(err)
		}
	}
	request, err := http.NewRequest(method, target, &body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, raw
}

func TestPaperRuntimeFixtureUsesTemporaryDatabase(t *testing.T) {
	db, err := database.OpenSQLite(t.TempDir() + "/paper-runtime-fixture.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.PaperOrder{}, &models.PaperFill{}, &models.PaperAccount{}, &models.PaperPosition{},
		&models.PaperStopScanAudit{}, &models.PaperEquityCheckpoint{}, &models.PaperDailyEquityBaseline{}, &models.AuditEvent{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PaperAccount{Currency: "USD", InitialCash: 10_000, Cash: 10_000, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	for _, position := range []models.PaperPosition{
		{Currency: "USD", Symbol: "AAPL", Quantity: 5, AverageCost: 90, Version: 1},
		{Currency: "USD", Symbol: "NVDA", Quantity: 10, AverageCost: 90, Version: 1},
	} {
		if err := db.Create(&position).Error; err != nil {
			t.Fatal(err)
		}
	}
	seedPaperDailyBaseline(t, db, "USD", time.Now().UTC(), 10_000)
	previousDB := database.DB
	previousQuotes := boundedQuotesForRequest
	previousUniverse := paperUniverseLoader
	database.DB = db
	boundedQuotesForRequest = func(symbols []string) quoteFetchResult {
		quotes := make(map[string]market.Quote)
		for _, symbol := range symbols {
			switch strings.ToUpper(symbol) {
			case "NVDA":
				quotes["NVDA"] = market.Quote{Price: 100, Source: "tradingview"}
			case "XOM":
				quotes["XOM"] = market.Quote{Price: 100, Source: "tradingview"}
			}
		}
		return quoteFetchResult{quotes: quotes, source: "self-provider", dataTime: time.Now().UTC()}
	}
	paperUniverseLoader = func(string) ([]models.Stock, string, error) {
		return []models.Stock{
			{Symbol: "AAPL", Sector: "Technology"},
			{Symbol: "NVDA", Sector: "Technology"},
			{Symbol: "XOM", Sector: "Energy"},
		}, "fixture", nil
	}
	t.Cleanup(func() {
		database.DB = previousDB
		boundedQuotesForRequest = previousQuotes
		paperUniverseLoader = previousUniverse
	})

	cfg := &config.Config{AdminToken: "fixture-token"}
	handler := &Handler{}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/paper-orders/account", adminReadGuard(cfg), handler.GetPaperAccount)
	router.POST("/api/paper-orders", adminWriteGuard(cfg, "paper-order.submit"), handler.SubmitPaperOrder)

	accountRequest := paperRequestWithBearer(t, router, http.MethodGet, "/api/paper-orders/account", nil, "fixture-token")
	if accountRequest.Code != http.StatusOK || !strings.Contains(accountRequest.Body.String(), "AAPL") || !strings.Contains(accountRequest.Body.String(), "NVDA") {
		t.Fatalf("temporary holdings fixture unavailable: status=%d body=%s", accountRequest.Code, accountRequest.Body.String())
	}
	buy := paperRequestWithBearer(t, router, http.MethodPost, "/api/paper-orders", paperOrderPayload("fixture-buy", "XOM", 5, 98), "fixture-token")
	if buy.Code != http.StatusUnprocessableEntity || !strings.Contains(buy.Body.String(), "paper portfolio risk is incomplete for BUY") {
		t.Fatalf("BUY did not fail closed with 422: status=%d body=%s", buy.Code, buy.Body.String())
	}
	sell := paperRequestWithBearer(t, router, http.MethodPost, "/api/paper-orders", protectiveSellPayload("fixture-sell", "NVDA", 2, 98), "fixture-token")
	if sell.Code != http.StatusCreated || !strings.Contains(sell.Body.String(), `"portfolioDegraded":true`) || !strings.Contains(sell.Body.String(), `"status":"ACCEPTED"`) {
		t.Fatalf("protective SELL did not use disclosed degradation: status=%d body=%s", sell.Code, sell.Body.String())
	}
}

func paperRequestWithBearer(t *testing.T, router http.Handler, method, path string, payload any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, path, &body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
