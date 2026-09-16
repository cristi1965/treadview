package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"trading-agents/internal/database"
	"trading-agents/internal/market"
	"trading-agents/internal/models"
)

func seedAcceptedPaperStop(t *testing.T, db *gorm.DB, id, side string, trigger float64) models.PaperOrder {
	t.Helper()
	reservedCash := 0.0
	reservedQty := 0
	if side == "BUY" {
		reservedCash = 1_001
	} else {
		reservedQty = 10
	}
	risk := models.PaperRiskSnapshot{}
	if side == "BUY" {
		risk = models.PaperRiskSnapshot{
			InvestableCapital: 10_000, MaxRiskAmount: 150,
			PlannedNotional: trigger * 10, MaxNotionalAmount: 1_500, RiskLimitPassed: true,
		}
	}
	order := models.PaperOrder{
		ClientOrderID: id, Environment: paperEnvironment, Symbol: "NVDA", Side: side,
		Market: "US", Currency: "USD", OrderType: "STOP_MARKET", TimeInForce: "GTC",
		ReferencePrice: 100, TriggerPrice: &trigger, Quantity: 10, QuoteSource: "tradingview",
		QuoteTime: time.Date(2026, 9, 8, 14, 0, 0, 0, time.UTC), Status: paperStatusAccepted,
		StatusHistory: []models.PaperOrderStatusEvent{{Status: paperStatusAccepted, At: time.Date(2026, 9, 8, 13, 0, 0, 0, time.UTC)}},
		RequestHash:   id, ReservedCash: reservedCash, ReservedQty: reservedQty, Version: 1, RiskSnapshot: risk,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	return order
}

func TestPaperStopScannerRejectsBuyStopGapAboveCurrentSingleOrderNotionalLimit(t *testing.T) {
	router, fixture := setupPaperOrderTestRouter(t)
	payload := validPaperOrderPayload("scan-gap-notional")
	payload["orderType"] = "STOP_MARKET"
	payload["entry"] = nil
	payload["triggerPrice"] = 101.0
	payload["protectiveStop"] = 100.0
	payload["takeProfit"] = 130.0
	payload["quantity"] = 14
	created := paperRequest(t, router, http.MethodPost, "/api/paper-orders", payload)
	if created.Code != http.StatusCreated {
		t.Fatalf("submit status=%d body=%s", created.Code, created.Body.String())
	}
	fixture.setPrice("NVDA", 108)
	now := fixture.now
	scanner := newPaperStopScanner(database.DB, paperStopScannerOptions{
		Clock: func() time.Time { return now }, MarketOpen: func(string, time.Time) bool { return true },
		Quotes: func([]string) quoteFetchResult {
			return quoteFetchResult{quotes: map[string]market.Quote{"NVDA": {Price: 108, Source: "tradingview"}}, source: "self-provider", dataTime: now}
		},
	})
	result := scanner.ScanOnce(context.Background())
	if result.Triggered != 0 || result.Skipped != 1 || result.Errors != 0 {
		t.Fatalf("gap scan result=%+v", result)
	}
	var order models.PaperOrder
	if err := database.DB.Where("client_order_id = ?", "scan-gap-notional").First(&order).Error; err != nil {
		t.Fatal(err)
	}
	if order.Status != paperStatusAccepted || order.FillQty != 0 || order.RiskSnapshot.FillPolicyCheck == nil {
		t.Fatalf("gap rejection changed order state: %+v", order)
	}
	assertReasonContains(t, order.RiskSnapshot.FillPolicyCheck.Reasons, "planned notional exceeds")
}

func TestPaperStopScannerTriggersOnlyFreshQuotesDuringMarketHours(t *testing.T) {
	_, _ = setupPaperOrderTestRouter(t)
	if err := database.DB.Model(&models.PaperAccount{}).Where("currency = ?", "USD").Updates(map[string]any{
		"cash": 8999.0, "reserved_cash": 1001.0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	seedAcceptedPaperStop(t, database.DB, "scan-buy", "BUY", 105)
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	providerCalls := 0
	scanner := newPaperStopScanner(database.DB, paperStopScannerOptions{
		Clock: func() time.Time { return now },
		Quotes: func(symbols []string) quoteFetchResult {
			mu.Lock()
			providerCalls++
			mu.Unlock()
			return quoteFetchResult{quotes: map[string]market.Quote{"NVDA": {Price: 106, Source: "tradingview"}}, source: "self-provider", dataTime: now}
		},
		MarketOpen: func(string, time.Time) bool { return true },
	})

	result := scanner.ScanOnce(context.Background())
	if result.Triggered != 1 || result.Errors != 0 || !result.EquityCheckpointed {
		t.Fatalf("scan result=%+v", result)
	}
	mu.Lock()
	gotCalls := providerCalls
	mu.Unlock()
	if gotCalls != 1 {
		t.Fatalf("provider calls=%d want=1", gotCalls)
	}
	var order models.PaperOrder
	if err := database.DB.Where("client_order_id = ?", "scan-buy").First(&order).Error; err != nil {
		t.Fatal(err)
	}
	if order.Status != paperStatusSimulatedFill || order.FillPrice <= 0 {
		t.Fatalf("stop was not filled: %+v", order)
	}
	var audits []models.PaperStopScanAudit
	if err := database.DB.Find(&audits).Error; err != nil || len(audits) != 1 || audits[0].TriggeredCount != 1 || !audits[0].EquityCheckpointed {
		t.Fatalf("scan audit missing: audits=%+v err=%v", audits, err)
	}
	var chain []models.AuditEvent
	if err := database.DB.Order("id asc").Find(&chain).Error; err != nil {
		t.Fatal(err)
	}
	if len(chain) != 1 || chain[0].Actor != "system:paper-oco-scanner" || chain[0].PayloadHash == "" || chain[0].OutcomeHash == "" || chain[0].HashVersion != systemAuditHashVersion {
		t.Fatalf("automatic fill audit missing: %+v", chain)
	}
	if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, chain); !valid {
		t.Fatalf("automatic fill audit chain invalid: %s", reason)
	}
	tampered := append([]models.AuditEvent(nil), chain...)
	tampered[0].OutcomeHash = strings.Repeat("0", 64)
	if valid, _, _, _ := verifyAuditEvents(tampered); valid {
		t.Fatal("tampered automatic outcome hash verified")
	}
}

func TestPaperStopScannerCreatesImmutableAuditedDayStartBaselineOnFirstOpenScan(t *testing.T) {
	_, fixture := setupPaperOrderTestRouter(t)
	if err := database.DB.Where("1 = 1").Delete(&models.PaperDailyEquityBaseline{}).Error; err != nil {
		t.Fatal(err)
	}
	now := fixture.now
	scanner := newPaperStopScanner(database.DB, paperStopScannerOptions{
		Clock:      func() time.Time { return now },
		MarketOpen: func(market string, _ time.Time) bool { return market == "US" },
	})

	first := scanner.ScanOnce(context.Background())
	if first.Errors != 0 || first.DailyBaselinesMade != 1 || first.Candidates != 0 {
		t.Fatalf("first open scan=%+v", first)
	}
	var baseline models.PaperDailyEquityBaseline
	if err := database.DB.Where("currency = ? AND market_date = ?", "USD", paperMarketDateForCurrency("USD", now)).First(&baseline).Error; err != nil {
		t.Fatal(err)
	}
	if baseline.Equity != 10_000 || !baseline.ObservedAt.Equal(now.UTC()) {
		t.Fatalf("unexpected day-start baseline: %+v", baseline)
	}
	var cnyCount int64
	if err := database.DB.Model(&models.PaperDailyEquityBaseline{}).Where("currency = ?", "CNY").Count(&cnyCount).Error; err != nil || cnyCount != 0 {
		t.Fatalf("closed CNY market baseline count=%d err=%v", cnyCount, err)
	}

	if err := database.DB.Model(&models.PaperAccount{}).Where("currency = ?", "USD").Update("cash", 9_000.0).Error; err != nil {
		t.Fatal(err)
	}
	now = now.Add(5 * time.Second)
	second := scanner.ScanOnce(context.Background())
	if second.Errors != 0 || second.DailyBaselinesMade != 0 {
		t.Fatalf("second scan=%+v", second)
	}
	var persisted models.PaperDailyEquityBaseline
	if err := database.DB.First(&persisted, baseline.ID).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Equity != baseline.Equity || !persisted.ObservedAt.Equal(baseline.ObservedAt) {
		t.Fatalf("day-start baseline was rewritten: before=%+v after=%+v", baseline, persisted)
	}
	// Restore the deliberately perturbed account before verifying the complete
	// Paper ledger. The baseline assertion above already proves it was immutable.
	if err := database.DB.Model(&models.PaperAccount{}).Where("currency = ?", "USD").Update("cash", 10_000.0).Error; err != nil {
		t.Fatal(err)
	}
	var events []models.AuditEvent
	if err := database.DB.Order("id asc").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Action != "paper-risk.daily-equity-baseline.create" {
		t.Fatalf("baseline audit events=%+v", events)
	}
	if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, events); !valid {
		t.Fatalf("baseline audit chain invalid: %s", reason)
	}
	if !strings.HasPrefix(events[0].Target, paperDailyBaselineAuditTargetPrefix) {
		t.Fatalf("baseline audit target is not evidence-addressable: %q", events[0].Target)
	}
	legacyEvents := append([]models.AuditEvent(nil), events...)
	legacyEvents[0].Target = baseline.Currency + ":" + baseline.MarketDate
	legacyEvents[0].Hash = calculateAuditHash(legacyEvents[0])
	if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, legacyEvents); !valid {
		t.Fatalf("legacy baseline audit target is not evidence-addressable: %s", reason)
	}
	if err := database.DB.Model(&models.PaperDailyEquityBaseline{}).Where("id = ?", baseline.ID).Update("equity", baseline.Equity+1).Error; err != nil {
		t.Fatal(err)
	}
	if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, events); valid || !strings.Contains(reason, "paper daily equity baseline outcome hash mismatch") {
		t.Fatalf("tampered daily baseline valid=%t reason=%q", valid, reason)
	}
	if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, legacyEvents); valid || !strings.Contains(reason, "paper daily equity baseline outcome hash mismatch") {
		t.Fatalf("tampered legacy daily baseline valid=%t reason=%q", valid, reason)
	}
}

func TestEstablishPaperDailyEquityBaselineIsServerComputedAndIdempotent(t *testing.T) {
	router, fixture := setupPaperOrderTestRouter(t)
	if err := database.DB.Where("currency = ?", "USD").Delete(&models.PaperDailyEquityBaseline{}).Error; err != nil {
		t.Fatal(err)
	}

	created := paperRequest(t, router, http.MethodPost, "/api/paper-orders/daily-baseline?currency=USD", nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("create baseline status=%d body=%s", created.Code, created.Body.String())
	}
	var response paperDailyEquityBaselineResponse
	if err := json.Unmarshal(created.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Created || response.Baseline.Equity != 10_000 || response.Baseline.Currency != "USD" || !response.Baseline.ObservedAt.Equal(fixture.now) {
		t.Fatalf("unexpected baseline response: %+v", response)
	}

	replayed := paperRequest(t, router, http.MethodPost, "/api/paper-orders/daily-baseline?currency=USD", nil)
	if replayed.Code != http.StatusOK || !strings.Contains(replayed.Body.String(), `"created":false`) {
		t.Fatalf("idempotent baseline status=%d body=%s", replayed.Code, replayed.Body.String())
	}
	var count int64
	if err := database.DB.Model(&models.PaperDailyEquityBaseline{}).Where("currency = ?", "USD").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("baseline rows=%d err=%v", count, err)
	}

	rejected := paperRequest(t, router, http.MethodPost, "/api/paper-orders/daily-baseline?currency=USD", map[string]any{"equity": 1})
	if rejected.Code != http.StatusBadRequest || !strings.Contains(rejected.Body.String(), "server-computed") {
		t.Fatalf("client equity status=%d body=%s", rejected.Code, rejected.Body.String())
	}
}

func TestEstablishPaperDailyEquityBaselineRejectsClosedMarket(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	if err := database.DB.Where("currency = ?", "CNY").Delete(&models.PaperDailyEquityBaseline{}).Error; err != nil {
		t.Fatal(err)
	}
	response := paperRequest(t, router, http.MethodPost, "/api/paper-orders/daily-baseline?currency=CNY", nil)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "regular trading session") {
		t.Fatalf("closed-market baseline status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestPaperBuyDoesNotCreateLateDailyBaselineButSellExitRemainsAvailable(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	if err := database.DB.Where("1 = 1").Delete(&models.PaperDailyEquityBaseline{}).Error; err != nil {
		t.Fatal(err)
	}
	buy := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("missing-day-start-buy"))
	if buy.Code != http.StatusUnprocessableEntity || !strings.Contains(buy.Body.String(), "scanner-established baseline") {
		t.Fatalf("BUY without scanner baseline status=%d body=%s", buy.Code, buy.Body.String())
	}
	var baselineCount int64
	if err := database.DB.Model(&models.PaperDailyEquityBaseline{}).Count(&baselineCount).Error; err != nil || baselineCount != 0 {
		t.Fatalf("BUY created late baseline count=%d err=%v", baselineCount, err)
	}
	if err := database.DB.Create(&models.PaperPosition{Currency: "USD", Symbol: "NVDA", Quantity: 2, AverageCost: 100, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	sell := paperRequest(t, router, http.MethodPost, "/api/paper-orders", protectiveSellPayload("baseline-free-sell", "NVDA", 2, 98))
	if sell.Code != http.StatusCreated || decodePaperOrderResponse(t, sell).Order.Status != paperStatusAccepted {
		t.Fatalf("protective SELL was blocked status=%d body=%s", sell.Code, sell.Body.String())
	}
}

func TestPaperStopScannerIdleScansOnlyUpdateStatus(t *testing.T) {
	_, _ = setupPaperOrderTestRouter(t)
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	scanner := newPaperStopScanner(database.DB, paperStopScannerOptions{Clock: func() time.Time { return now }})

	for index := 0; index < 3; index++ {
		result := scanner.ScanOnce(context.Background())
		if result.Candidates != 0 || result.Errors != 0 {
			t.Fatalf("idle scan %d result=%+v", index, result)
		}
		now = now.Add(5 * time.Second)
	}
	var auditCount int64
	if err := database.DB.Model(&models.PaperStopScanAudit{}).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 0 {
		t.Fatalf("idle scans wrote %d durable audits", auditCount)
	}
	status := scanner.Status()
	if status.LastResult == nil || !status.LastResult.CompletedAt.Equal(now.Add(-5*time.Second)) {
		t.Fatalf("idle status was not updated: %+v", status)
	}
	if !strings.Contains(status.AuditRetention, "idle scans update in-memory") {
		t.Fatalf("audit retention semantics missing: %+v", status)
	}
}

func TestPaperStopScannerDeduplicatesUnchangedCandidateAuditUntilTTL(t *testing.T) {
	_, _ = setupPaperOrderTestRouter(t)
	if err := database.DB.Model(&models.PaperAccount{}).Where("currency = ?", "USD").Updates(map[string]any{
		"cash": 8999.0, "reserved_cash": 1001.0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	seedAcceptedPaperStop(t, database.DB, "scan-waiting", "BUY", 105)
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	scanner := newPaperStopScanner(database.DB, paperStopScannerOptions{
		Clock: func() time.Time { return now }, MarketOpen: func(string, time.Time) bool { return true }, AuditTTL: time.Minute,
		Quotes: func([]string) quoteFetchResult {
			return quoteFetchResult{quotes: map[string]market.Quote{"NVDA": {Price: 100, Source: "tradingview"}}, source: "self-provider", dataTime: now}
		},
	})

	for index := 0; index < 3; index++ {
		result := scanner.ScanOnce(context.Background())
		if result.Candidates != 1 || result.Skipped != 1 || result.Triggered != 0 || result.Errors != 0 {
			t.Fatalf("unchanged scan %d result=%+v", index, result)
		}
		now = now.Add(5 * time.Second)
	}
	var count int64
	if err := database.DB.Model(&models.PaperStopScanAudit{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("unchanged candidate wrote %d audits want=1", count)
	}
	now = now.Add(time.Minute)
	scanner.ScanOnce(context.Background())
	if err := database.DB.Model(&models.PaperStopScanAudit{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("TTL renewal wrote %d audits want=2", count)
	}
	if !strings.Contains(scanner.Status().AuditRetention, "state-change + 15m TTL") {
		t.Fatalf("audit retention contract missing: %+v", scanner.Status())
	}
}

func TestPaperStopScannerRechecksBuyRiskAndDeduplicatesRejectionAudit(t *testing.T) {
	_, _ = setupPaperOrderTestRouter(t)
	if err := database.DB.Model(&models.PaperAccount{}).Where("currency = ?", "USD").Updates(map[string]any{
		"cash": 8999.0, "reserved_cash": 1001.0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	seedAcceptedPaperStop(t, database.DB, "scan-risk-reject", "BUY", 105)
	if err := database.DB.Model(&models.PaperDailyEquityBaseline{}).
		Where("currency = ? AND market_date = ?", "USD", paperMarketDateForCurrency("USD", now)).
		Updates(map[string]any{"observed_at": now, "equity": 10_500}).Error; err != nil {
		t.Fatal(err)
	}
	policy := defaultPaperRiskPolicy()
	policy.PolicyVersion = "scanner-fill-v9"
	policy.MaxDailyLossPct = 1
	scanner := newPaperStopScanner(database.DB, paperStopScannerOptions{
		Clock: func() time.Time { return now }, MarketOpen: func(string, time.Time) bool { return true }, Policy: policy,
		Quotes: func([]string) quoteFetchResult {
			return quoteFetchResult{quotes: map[string]market.Quote{"NVDA": {Price: 106, Source: "tradingview"}}, source: "self-provider", dataTime: now}
		},
	})

	for index := 0; index < 2; index++ {
		result := scanner.ScanOnce(context.Background())
		if result.Triggered != 0 || result.Skipped != 1 || result.Errors != 0 {
			t.Fatalf("risk-rejected scan %d result=%+v", index, result)
		}
		now = now.Add(5 * time.Second)
	}
	var order models.PaperOrder
	if err := database.DB.Where("client_order_id = ?", "scan-risk-reject").First(&order).Error; err != nil {
		t.Fatal(err)
	}
	if order.Status != paperStatusAccepted || order.FillQty != 0 || order.RiskSnapshot.FillPolicyCheck == nil || order.RiskSnapshot.FillPolicyCheck.PolicyVersion != "scanner-fill-v9" {
		t.Fatalf("automatic BUY risk rejection was not persisted: %+v", order)
	}
	assertReasonContains(t, order.RiskSnapshot.FillPolicyCheck.Reasons, "daily equity loss")
	var scanAudits, systemAudits int64
	if err := database.DB.Model(&models.PaperStopScanAudit{}).Count(&scanAudits).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Model(&models.AuditEvent{}).Where("action = ?", "paper-order.auto-fill-risk-rejected").Count(&systemAudits).Error; err != nil {
		t.Fatal(err)
	}
	if scanAudits != 1 || systemAudits != 1 {
		t.Fatalf("unchanged automatic rejection audits scan=%d system=%d want=1,1", scanAudits, systemAudits)
	}
}

func TestPaperStopScannerFillsTakeProfitAndAtomicallyCancelsStop(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	if got := paperRequest(t, router, "POST", "/api/paper-orders", validPaperOrderPayload("oco-take-profit-parent")); got.Code != 201 {
		t.Fatalf("buy submit status=%d body=%s", got.Code, got.Body.String())
	}
	parent := decodePaperOrderResponse(t, paperRequest(t, router, "POST", "/api/paper-orders/oco-take-profit-parent/simulated-fill", nil)).Order
	var stop, takeProfit models.PaperOrder
	if err := database.DB.Where("parent_order_id = ? AND order_type = ?", parent.ID, "STOP_MARKET").First(&stop).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Where("parent_order_id = ? AND order_type = ?", parent.ID, "LIMIT").First(&takeProfit).Error; err != nil {
		t.Fatal(err)
	}
	if stop.OCOGroupID == "" || stop.OCOGroupID != takeProfit.OCOGroupID || stop.ReservedQty+takeProfit.ReservedQty != parent.Quantity {
		t.Fatalf("invalid OCO reservation: stop=%+v takeProfit=%+v", stop, takeProfit)
	}
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	scanner := newPaperStopScanner(database.DB, paperStopScannerOptions{
		Clock: func() time.Time { return now }, MarketOpen: func(string, time.Time) bool { return true },
		Quotes: func([]string) quoteFetchResult {
			return quoteFetchResult{quotes: map[string]market.Quote{"NVDA": {Price: *takeProfit.Entry + 1, Source: "tradingview"}}, source: "self-provider", dataTime: now}
		},
	})
	result := scanner.ScanOnce(context.Background())
	if result.Triggered != 1 || result.Skipped != 1 || result.Errors != 0 || !result.EquityCheckpointed {
		t.Fatalf("take-profit scan=%+v", result)
	}
	if err := database.DB.First(&stop, stop.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.First(&takeProfit, takeProfit.ID).Error; err != nil {
		t.Fatal(err)
	}
	if takeProfit.Status != paperStatusSimulatedFill || stop.Status != paperStatusCancelled {
		t.Fatalf("OCO terminal states stop=%s takeProfit=%s", stop.Status, takeProfit.Status)
	}
	var position models.PaperPosition
	if err := database.DB.Where("currency = ? AND symbol = ?", "USD", "NVDA").First(&position).Error; err != nil {
		t.Fatal(err)
	}
	if position.Quantity != 0 || position.ReservedQuantity != 0 {
		t.Fatalf("OCO fill did not conserve inventory: %+v", position)
	}
	var chain []models.AuditEvent
	if err := database.DB.Order("id asc").Find(&chain).Error; err != nil {
		t.Fatal(err)
	}
	actions := map[string]int{}
	for _, event := range chain {
		if event.HashVersion == systemAuditHashVersion {
			actions[event.Action]++
		}
	}
	if actions["paper-order.auto-child.create"] != 2 || actions["paper-order.auto-child.cancel"] < 1 || actions["paper-order.auto-take-profit-fill"] != 1 {
		t.Fatalf("v3 OCO audit coverage=%v", actions)
	}
	if valid, _, reason, _ := verifyAuditEvents(chain); !valid {
		t.Fatalf("v3 OCO audit chain invalid: %s", reason)
	}
}

func TestPaperStopScannerSkipsClosedOrStaleAndRecoversOnStart(t *testing.T) {
	_, _ = setupPaperOrderTestRouter(t)
	if err := database.DB.Model(&models.PaperAccount{}).Where("currency = ?", "USD").Updates(map[string]any{
		"cash": 8999.0, "reserved_cash": 1001.0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	seedAcceptedPaperStop(t, database.DB, "scan-recover", "BUY", 105)
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	marketOpen := false
	stale := false
	scanner := newPaperStopScanner(database.DB, paperStopScannerOptions{
		Clock: func() time.Time { return now }, Interval: time.Hour,
		Quotes: func([]string) quoteFetchResult {
			dataTime := now
			if stale {
				dataTime = now.Add(-2 * marketQuoteFreshFor)
			}
			return quoteFetchResult{quotes: map[string]market.Quote{"NVDA": {Price: 106, Source: "tradingview"}}, source: "self-provider", dataTime: dataTime}
		},
		MarketOpen: func(string, time.Time) bool { return marketOpen },
	})
	if result := scanner.ScanOnce(context.Background()); result.Triggered != 0 || result.ClosedMarket != 1 {
		t.Fatalf("closed-market scan=%+v", result)
	}
	marketOpen, stale = true, true
	if result := scanner.ScanOnce(context.Background()); result.Triggered != 0 || result.Errors != 1 {
		t.Fatalf("stale scan=%+v", result)
	}
	stale = false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := scanner.Start(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		var order models.PaperOrder
		if err := database.DB.Where("client_order_id = ?", "scan-recover").First(&order).Error; err == nil && order.Status == paperStatusSimulatedFill {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("startup recovery did not scan persisted ACCEPTED stop")
		}
		time.Sleep(10 * time.Millisecond)
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := scanner.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	if scanner.Status().Running {
		t.Fatal("scanner still running after Stop")
	}
}

func TestPaperStopScannerRecoveryRebuildsMissingProtectiveChild(t *testing.T) {
	_, _ = setupPaperOrderTestRouter(t)
	stop := 95.0
	filledAt := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	parent := models.PaperOrder{
		ClientOrderID: "legacy-filled-parent", Environment: paperEnvironment, Symbol: "NVDA", Side: "BUY", Market: "US", Currency: "USD",
		OrderType: "LIMIT", TimeInForce: "GTC", ReferencePrice: 100, ProtectiveStop: &stop, Quantity: 8, FillQty: 8, FillPrice: 100,
		QuoteSource: "tradingview", QuoteTime: filledAt, Status: paperStatusSimulatedFill, FilledAt: &filledAt,
		StatusHistory: []models.PaperOrderStatusEvent{{Status: paperStatusSimulatedFill, At: filledAt}}, RequestHash: "legacy-filled-parent", Version: 2,
	}
	if err := database.DB.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&models.PaperPosition{Currency: "USD", Symbol: "NVDA", Quantity: 8, AverageCost: 100, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	scanner := newPaperStopScanner(database.DB, paperStopScannerOptions{Clock: func() time.Time { return filledAt }})
	if failures := scanner.RecoverProtectiveOrders(context.Background()); len(failures) != 0 {
		t.Fatalf("recovery failures=%v", failures)
	}
	if failures := scanner.RecoverProtectiveOrders(context.Background()); len(failures) != 0 {
		t.Fatalf("idempotent recovery failures=%v", failures)
	}
	var children []models.PaperOrder
	if err := database.DB.Where("parent_order_id = ? AND status = ?", parent.ID, paperStatusAccepted).Find(&children).Error; err != nil {
		t.Fatal(err)
	}
	if len(children) != 1 || children[0].ReservedQty != 8 {
		t.Fatalf("recovered children=%+v", children)
	}
	var chain []models.AuditEvent
	if err := database.DB.Order("id asc").Find(&chain).Error; err != nil {
		t.Fatal(err)
	}
	actions := map[string]int{}
	for _, event := range chain {
		actions[event.Action]++
		if event.HashVersion != systemAuditHashVersion || event.PayloadHash == "" || event.OutcomeHash == "" {
			t.Fatalf("recovery audit is not complete v3: %+v", event)
		}
	}
	if actions["paper-order.auto-child.create"] != 1 || actions["paper-order.oco-recovery"] != 2 {
		t.Fatalf("recovery audit coverage=%v", actions)
	}
	if valid, _, reason, _ := verifyAuditEvents(chain); !valid {
		t.Fatalf("recovery audit chain invalid: %s", reason)
	}
}

func TestPaperStopScannerAndCancelHaveSingleTerminalState(t *testing.T) {
	router, _ := setupPaperOrderTestRouter(t)
	if err := database.DB.Create(&models.PaperPosition{Currency: "USD", Symbol: "NVDA", Quantity: 10, ReservedQuantity: 10, AverageCost: 90, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	seedAcceptedPaperStop(t, database.DB, "scan-cancel-race", "SELL", 95)
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	scanner := newPaperStopScanner(database.DB, paperStopScannerOptions{
		Clock: func() time.Time { return now }, MarketOpen: func(string, time.Time) bool { return true },
		Quotes: func([]string) quoteFetchResult {
			return quoteFetchResult{quotes: map[string]market.Quote{"NVDA": {Price: 94, Source: "tradingview"}}, source: "self-provider", dataTime: now}
		},
	})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); scanner.ScanOnce(context.Background()) }()
	go func() {
		defer wg.Done()
		paperRequest(t, router, "POST", "/api/paper-orders/scan-cancel-race/cancel", nil)
	}()
	wg.Wait()
	var order models.PaperOrder
	if err := database.DB.Where("client_order_id = ?", "scan-cancel-race").First(&order).Error; err != nil {
		t.Fatal(err)
	}
	if order.Status != paperStatusCancelled && order.Status != paperStatusSimulatedFill {
		t.Fatalf("unexpected terminal state: %+v", order)
	}
	if len(order.StatusHistory) != 2 {
		t.Fatalf("multiple terminal transitions recorded: %+v", order.StatusHistory)
	}
}

func TestPaperMarketOpenUsesCoveredExchangeCalendar(t *testing.T) {
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		market string
		at     time.Time
		open   bool
	}{
		{name: "US labor day closed", market: "US", at: time.Date(2026, 9, 7, 10, 0, 0, 0, newYork)},
		{name: "US next regular day open", market: "US", at: time.Date(2026, 9, 8, 10, 0, 0, 0, newYork), open: true},
		{name: "US early close enforced", market: "US", at: time.Date(2026, 11, 27, 14, 0, 0, 0, newYork)},
		{name: "CN national holiday closed", market: "CN", at: time.Date(2026, 10, 5, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))},
		{name: "unsupported year fails closed", market: "US", at: time.Date(2027, 9, 8, 10, 0, 0, 0, newYork)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := paperMarketOpen(test.market, test.at); got != test.open {
				t.Fatalf("paperMarketOpen(%s, %s)=%v want=%v", test.market, test.at, got, test.open)
			}
		})
	}
}
