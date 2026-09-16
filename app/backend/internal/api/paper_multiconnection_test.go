package api

import (
	"errors"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"trading-agents/internal/database"
	"trading-agents/internal/models"
)

func TestPaperReservationIsSafeAcrossIndependentSQLiteConnections(t *testing.T) {
	path := t.TempDir() + "/paper-concurrency.db"
	db1, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	db2, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db1.AutoMigrate(&models.PaperAccount{}, &models.PaperPosition{}, &models.PaperOrder{}, &models.PaperFill{}, &models.PaperEquityCheckpoint{}, &models.PaperDailyEquityBaseline{}, &models.AuditEvent{}); err != nil {
		t.Fatal(err)
	}
	if err := db1.Create(&models.PaperAccount{Currency: "USD", InitialCash: 10_000, Cash: 10_000, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	reserve := func(db *gorm.DB) {
		<-start
		err := db.Transaction(func(tx *gorm.DB) error {
			account, position, err := loadPaperLedger(tx, "USD", "NVDA")
			if err != nil {
				return err
			}
			return reservePaperOrder(tx, &account, &position, "BUY", 6_000, 0)
		})
		errs <- err
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); reserve(db1) }()
	go func() { defer wg.Done(); reserve(db2) }()
	close(start)
	wg.Wait()
	close(errs)
	successes, failures := 0, 0
	for err := range errs {
		if err == nil {
			successes++
		} else {
			failures++
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("independent connection reservations successes=%d failures=%d", successes, failures)
	}
	var account models.PaperAccount
	if err := db1.Where("currency = ?", "USD").First(&account).Error; err != nil {
		t.Fatal(err)
	}
	if account.Cash != 4_000 || account.ReservedCash != 6_000 || account.Version != 2 {
		t.Fatalf("ledger conservation failed: %+v", account)
	}
}

func TestPaperPartialFillIDIsIdempotentAcrossIndependentSQLiteConnections(t *testing.T) {
	installPaperMulticonnectionRiskSources(t)
	path := t.TempDir() + "/paper-partial-idempotency.db"
	db1, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	db2, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db1.AutoMigrate(&models.PaperAccount{}, &models.PaperPosition{}, &models.PaperOrder{}, &models.PaperFill{}, &models.PaperEquityCheckpoint{}, &models.PaperDailyEquityBaseline{}, &models.AuditEvent{}); err != nil {
		t.Fatal(err)
	}
	if err := db1.Create(&models.PaperAccount{Currency: "USD", InitialCash: 10_000, Cash: 8_999, ReservedCash: 1_001, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	entry, stop := 100.0, 98.0
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	seedPaperDailyBaseline(t, db1, "USD", now, 10_000)
	order := models.PaperOrder{
		ClientOrderID: "two-connection-partial", Environment: paperEnvironment, Symbol: "NVDA",
		Side: "BUY", Market: "US", Currency: "USD", OrderType: "LIMIT", TimeInForce: "GTC",
		ReferencePrice: 100, Entry: &entry, ProtectiveStop: &stop, Quantity: 10, RemainingQty: 10,
		QuoteSource: "tradingview", QuoteTime: now, Status: paperStatusAccepted,
		StatusHistory: []models.PaperOrderStatusEvent{{Status: paperStatusAccepted, At: now}},
		RequestHash:   "two-connection-partial", ReservedCash: 1_001, Version: 1,
		RiskSnapshot: models.PaperRiskSnapshot{InvestableCapital: 10_000, MaxRiskAmount: 150, PlannedNotional: 1_000, MaxNotionalAmount: 1_500, RiskLimitPassed: true},
	}
	if err := db1.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	quote := paperAuthoritativeQuote{Price: 100, Source: "tradingview", At: now}
	type result struct {
		replay bool
		err    error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	fill := func(db *gorm.DB) {
		<-start
		_, replay, _, err := transitionPaperOrderDBWithPolicyQuantity(db, order.ClientOrderID, paperStatusSimulatedFill, quote, now, defaultPaperRiskPolicy(), false, "shared-partial-fill", 4)
		results <- result{replay: replay, err: err}
	}
	go fill(db1)
	go fill(db2)
	close(start)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil || first.replay == second.replay {
		t.Fatalf("concurrent partial results first=%+v second=%+v", first, second)
	}
	var persisted models.PaperOrder
	if err := db1.Where("id = ?", order.ID).First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	var fills int64
	if err := db1.Model(&models.PaperFill{}).Where("paper_order_id = ?", order.ID).Count(&fills).Error; err != nil {
		t.Fatal(err)
	}
	if fills != 1 || persisted.FillQty != 4 || persisted.RemainingQty != 6 || persisted.Status != paperStatusAccepted {
		t.Fatalf("concurrent partial fill duplicated: fills=%d order=%+v", fills, persisted)
	}
}

func TestPaperTerminalTransitionIsSafeAcrossIndependentSQLiteConnections(t *testing.T) {
	installPaperMulticonnectionRiskSources(t)
	path := t.TempDir() + "/paper-terminal.db"
	db1, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	db2, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db1.AutoMigrate(&models.PaperAccount{}, &models.PaperPosition{}, &models.PaperOrder{}, &models.PaperFill{}, &models.PaperEquityCheckpoint{}, &models.PaperDailyEquityBaseline{}, &models.AuditEvent{}); err != nil {
		t.Fatal(err)
	}
	if err := db1.Create(&models.PaperAccount{Currency: "USD", InitialCash: 10_000, Cash: 8_999, ReservedCash: 1_001, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	entry := 100.0
	createdAt := time.Date(2026, 9, 8, 14, 0, 0, 0, time.UTC)
	seedPaperDailyBaseline(t, db1, "USD", createdAt, 10_000)
	order := models.PaperOrder{
		ClientOrderID: "two-connection-terminal", Environment: paperEnvironment, Symbol: "NVDA",
		Side: "BUY", Market: "US", Currency: "USD", OrderType: "LIMIT", TimeInForce: "GTC",
		ReferencePrice: 100, Entry: &entry, Quantity: 10, QuoteSource: "tradingview", QuoteTime: createdAt,
		Status: paperStatusAccepted, StatusHistory: []models.PaperOrderStatusEvent{{Status: paperStatusAccepted, At: createdAt}},
		RequestHash: "two-connection-terminal", ReservedCash: 1_001, Version: 1,
		RiskSnapshot: models.PaperRiskSnapshot{InvestableCapital: 10_000, MaxRiskAmount: 150, PlannedNotional: 1_000, MaxNotionalAmount: 1_500, RiskLimitPassed: true},
	}
	if err := db1.Create(&order).Error; err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	go func() {
		<-start
		_, _, _, err := transitionPaperOrderDB(db1, order.ClientOrderID, paperStatusCancelled, paperAuthoritativeQuote{}, createdAt.Add(time.Minute))
		errs <- err
	}()
	go func() {
		<-start
		quote := paperAuthoritativeQuote{Price: 100, Source: "tradingview", At: createdAt.Add(time.Minute)}
		_, _, _, err := transitionPaperOrderDB(db2, order.ClientOrderID, paperStatusSimulatedFill, quote, createdAt.Add(time.Minute))
		errs <- err
	}()
	close(start)
	first, second := <-errs, <-errs
	if (first == nil) == (second == nil) {
		t.Fatalf("terminal transitions first=%v second=%v; want exactly one winner", first, second)
	}
	loser := first
	if loser == nil {
		loser = second
	}
	if !errors.Is(loser, errPaperStateConflict) {
		t.Fatalf("loser error=%v want state conflict", loser)
	}
	var final models.PaperOrder
	if err := db1.Where("client_order_id = ?", order.ClientOrderID).First(&final).Error; err != nil {
		t.Fatal(err)
	}
	if final.Status != paperStatusCancelled && final.Status != paperStatusSimulatedFill {
		t.Fatalf("unexpected terminal status: %+v", final)
	}
	if len(final.StatusHistory) != 2 || final.Version != 2 {
		t.Fatalf("terminal transition was not singular: %+v", final)
	}
}

func TestProtectiveChildIsIdempotentAcrossIndependentSQLiteConnections(t *testing.T) {
	installPaperMulticonnectionRiskSources(t)
	path := t.TempDir() + "/paper-child-idempotency.db"
	db1, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	db2, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db1.AutoMigrate(&models.PaperAccount{}, &models.PaperPosition{}, &models.PaperOrder{}, &models.PaperFill{}, &models.PaperEquityCheckpoint{}, &models.PaperDailyEquityBaseline{}, &models.AuditEvent{}); err != nil {
		t.Fatal(err)
	}
	if err := db1.Create(&models.PaperAccount{Currency: "USD", InitialCash: 10_000, Cash: 8_999, ReservedCash: 1_001, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	entry, stop, takeProfit := 100.0, 98.0, 106.0
	now := time.Date(2026, 9, 8, 14, 0, 0, 0, time.UTC)
	seedPaperDailyBaseline(t, db1, "USD", now, 10_000)
	parent := models.PaperOrder{
		ClientOrderID: "two-connection-protected", Environment: paperEnvironment, Symbol: "NVDA", Side: "BUY", Market: "US", Currency: "USD",
		OrderType: "LIMIT", TimeInForce: "GTC", ReferencePrice: 100, Entry: &entry, ProtectiveStop: &stop, TakeProfit: &takeProfit, Quantity: 10,
		QuoteSource: "tradingview", QuoteTime: now, Status: paperStatusAccepted,
		StatusHistory: []models.PaperOrderStatusEvent{{Status: paperStatusAccepted, At: now}}, RequestHash: "two-connection-protected", ReservedCash: 1_001, Version: 1,
		RiskSnapshot: models.PaperRiskSnapshot{InvestableCapital: 10_000, MaxLoss: 20, MaxRiskAmount: 150, PlannedNotional: 1_000, MaxNotionalAmount: 1_500, RiskLimitPassed: true},
	}
	if err := db1.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan bool, 2)
	fill := func(db *gorm.DB) {
		<-start
		_, replay, conflict, err := transitionPaperOrderDB(db, parent.ClientOrderID, paperStatusSimulatedFill, paperAuthoritativeQuote{Price: 100, Source: "tradingview", At: now}, now)
		if err != nil {
			t.Errorf("fill: %v (%s)", err, conflict)
		}
		results <- replay
	}
	go fill(db1)
	go fill(db2)
	close(start)
	first, second := <-results, <-results
	if first == second {
		t.Fatalf("replay flags=%v,%v want one transition and one replay", first, second)
	}
	var count int64
	if err := db1.Model(&models.PaperOrder{}).Where("parent_order_id = ? AND status = ?", parent.ID, paperStatusAccepted).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("OCO child count=%d want=2", count)
	}
	var position models.PaperPosition
	if err := db1.Where("currency = ? AND symbol = ?", "USD", "NVDA").First(&position).Error; err != nil {
		t.Fatal(err)
	}
	if position.Quantity != 10 || position.ReservedQuantity != 10 {
		t.Fatalf("position=%+v", position)
	}
}

func TestPaperFillCollectsSlowADVBeforeSQLiteImmediateTransaction(t *testing.T) {
	installPaperMulticonnectionRiskSources(t)
	path := t.TempDir() + "/paper-slow-source.db"
	db1, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	db2, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db1.AutoMigrate(&models.PaperAccount{}, &models.PaperPosition{}, &models.PaperOrder{}, &models.PaperFill{}, &models.PaperEquityCheckpoint{}, &models.PaperDailyEquityBaseline{}, &models.AuditEvent{}); err != nil {
		t.Fatal(err)
	}
	if err := db1.Create(&models.PaperAccount{Currency: "USD", InitialCash: 10_000, Cash: 8_999, ReservedCash: 1_001, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	seedPaperDailyBaseline(t, db1, "USD", now, 10_000)
	entry, stop, takeProfit := 100.0, 98.0, 106.0
	order := models.PaperOrder{
		ClientOrderID: "slow-source-fill", Environment: paperEnvironment, Symbol: "NVDA", Side: "BUY", Market: "US", Currency: "USD",
		OrderType: "LIMIT", TimeInForce: "GTC", ReferencePrice: 100, Entry: &entry, ProtectiveStop: &stop, TakeProfit: &takeProfit, Quantity: 10,
		QuoteSource: "tradingview", QuoteTime: now, Status: paperStatusAccepted, StatusHistory: []models.PaperOrderStatusEvent{{Status: paperStatusAccepted, At: now}},
		RequestHash: "slow-source-fill", ReservedCash: 1_001, Version: 1,
		RiskSnapshot: models.PaperRiskSnapshot{InvestableCapital: 10_000, MaxLoss: 20, MaxRiskAmount: 150, PlannedNotional: 1_000, MaxNotionalAmount: 1_500, RiskLimitPassed: true},
	}
	if err := db1.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	previousADV := paperADVLoader
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	paperADVLoader = func(marketCode string, symbols []string) (map[string]float64, string, error) {
		once.Do(func() { close(entered) })
		<-release
		return previousADV(marketCode, symbols)
	}
	t.Cleanup(func() { paperADVLoader = previousADV })

	fillDone := make(chan error, 1)
	go func() {
		_, _, _, fillErr := transitionPaperOrderDB(db1, order.ClientOrderID, paperStatusSimulatedFill, paperAuthoritativeQuote{Price: 100, Source: "tradingview", At: now}, now)
		fillDone <- fillErr
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("slow ADV loader was not reached")
	}
	writeDone := make(chan error, 1)
	go func() {
		writeDone <- db2.Create(&models.PaperEquityCheckpoint{Currency: "USD", Equity: 10_000, ObservedAt: now}).Error
	}()
	select {
	case writeErr := <-writeDone:
		if writeErr != nil {
			t.Fatalf("independent write failed while ADV loader was blocked: %v", writeErr)
		}
	case <-time.After(time.Second):
		t.Fatal("slow ADV loader held the SQLite write reservation")
	}
	close(release)
	if fillErr := <-fillDone; fillErr != nil {
		t.Fatalf("fill after slow source: %v", fillErr)
	}
}

func installPaperMulticonnectionRiskSources(t *testing.T) {
	t.Helper()
	previousUniverse := paperUniverseLoader
	previousADV := paperADVLoader
	paperUniverseLoader = func(marketCode string) ([]models.Stock, string, error) {
		if marketCode != "us" {
			return nil, "test-universe", nil
		}
		return []models.Stock{{Symbol: "NVDA", Price: 100, Sector: "Technology"}}, "test-universe", nil
	}
	paperADVLoader = func(string, []string) (map[string]float64, string, error) {
		return map[string]float64{"NVDA": 1_000_000}, "test-adv", nil
	}
	t.Cleanup(func() {
		paperUniverseLoader = previousUniverse
		paperADVLoader = previousADV
	})
}

func seedPaperDailyBaseline(t *testing.T, db *gorm.DB, currency string, now time.Time, equity float64) {
	t.Helper()
	if err := db.Create(&models.PaperDailyEquityBaseline{
		Currency: currency, MarketDate: paperMarketDateForCurrency(currency, now), ObservedAt: now.UTC(), Equity: equity,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestPaperOCOLegsCannotBothFillAcrossIndependentSQLiteConnections(t *testing.T) {
	path := t.TempDir() + "/paper-oco-race.db"
	db1, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	db2, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db1.AutoMigrate(&models.PaperAccount{}, &models.PaperPosition{}, &models.PaperOrder{}, &models.PaperFill{}, &models.PaperEquityCheckpoint{}, &models.PaperDailyEquityBaseline{}, &models.AuditEvent{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	stopPrice, takeProfitPrice := 95.0, 105.0
	parent := models.PaperOrder{
		ClientOrderID: "oco-race-parent", Environment: paperEnvironment, Symbol: "NVDA", Side: "BUY", Market: "US", Currency: "USD",
		OrderType: "LIMIT", TimeInForce: "GTC", ReferencePrice: 100, Quantity: 10, FillQty: 10, FillPrice: 100,
		ProtectiveStop: &stopPrice, TakeProfit: &takeProfitPrice, OCOGroupID: "paper-oco-race", ProtectionRemainingQty: 10, ProtectionInitialized: true,
		QuoteSource: "tradingview", QuoteTime: now, Status: paperStatusSimulatedFill, FilledAt: &now,
		StatusHistory: []models.PaperOrderStatusEvent{{Status: paperStatusSimulatedFill, At: now}}, RequestHash: "oco-race-parent", Version: 2,
	}
	if err := db1.Create(&models.PaperAccount{Currency: "USD", InitialCash: 10_000, Cash: 9_000, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db1.Create(&models.PaperPosition{Currency: "USD", Symbol: "NVDA", Quantity: 10, ReservedQuantity: 10, AverageCost: 100, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db1.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	stop := models.PaperOrder{
		ClientOrderID: "oco-race-stop", Environment: paperEnvironment, Symbol: "NVDA", Side: "SELL", Market: "US", Currency: "USD", OrderType: "STOP_MARKET", TimeInForce: "GTC",
		ReferencePrice: 100, TriggerPrice: &stopPrice, ParentOrderID: &parent.ID, OCOGroupID: parent.OCOGroupID, Quantity: 10, ReservedQty: 10,
		QuoteSource: "tradingview", QuoteTime: now, Status: paperStatusAccepted, StatusHistory: []models.PaperOrderStatusEvent{{Status: paperStatusAccepted, At: now}}, RequestHash: "oco-race-stop", Version: 1,
	}
	takeProfit := models.PaperOrder{
		ClientOrderID: "oco-race-tp", Environment: paperEnvironment, Symbol: "NVDA", Side: "SELL", Market: "US", Currency: "USD", OrderType: "LIMIT", TimeInForce: "GTC",
		ReferencePrice: 100, Entry: &takeProfitPrice, ParentOrderID: &parent.ID, OCOGroupID: parent.OCOGroupID, Quantity: 10,
		QuoteSource: "tradingview", QuoteTime: now, Status: paperStatusAccepted, StatusHistory: []models.PaperOrderStatusEvent{{Status: paperStatusAccepted, At: now}}, RequestHash: "oco-race-tp", Version: 1,
	}
	if err := db1.Create(&stop).Error; err != nil {
		t.Fatal(err)
	}
	if err := db1.Create(&takeProfit).Error; err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	fill := func(db *gorm.DB, id string, price float64) {
		<-start
		_, _, _, err := transitionPaperOrderDBWithAudit(db, id, paperStatusSimulatedFill, paperAuthoritativeQuote{Price: price, Source: "tradingview", At: now}, now, true)
		errs <- err
	}
	go fill(db1, stop.ClientOrderID, 94)
	go fill(db2, takeProfit.ClientOrderID, 106)
	close(start)
	first, second := <-errs, <-errs
	if (first == nil) == (second == nil) {
		t.Fatalf("OCO transitions first=%v second=%v; want exactly one winner", first, second)
	}
	loser := first
	if loser == nil {
		loser = second
	}
	if !errors.Is(loser, errPaperStateConflict) {
		t.Fatalf("OCO loser error=%v want state conflict", loser)
	}
	var children []models.PaperOrder
	if err := db1.Where("oco_group_id = ? AND parent_order_id IS NOT NULL", parent.OCOGroupID).Order("id asc").Find(&children).Error; err != nil {
		t.Fatal(err)
	}
	filled, cancelled := 0, 0
	for _, child := range children {
		if child.Status == paperStatusSimulatedFill {
			filled++
		}
		if child.Status == paperStatusCancelled {
			cancelled++
		}
	}
	if filled != 1 || cancelled != 1 {
		t.Fatalf("OCO children=%+v", children)
	}
	var position models.PaperPosition
	if err := db1.Where("currency = ? AND symbol = ?", "USD", "NVDA").First(&position).Error; err != nil {
		t.Fatal(err)
	}
	if position.Quantity != 0 || position.ReservedQuantity != 0 {
		t.Fatalf("OCO race oversold inventory: %+v", position)
	}
	if err := db1.First(&parent, parent.ID).Error; err != nil {
		t.Fatal(err)
	}
	if parent.ProtectionRemainingQty != 0 {
		t.Fatalf("OCO parent capacity=%d want=0", parent.ProtectionRemainingQty)
	}
}

func TestProtectivePartialFillIsIdempotentAcrossIndependentSQLiteConnections(t *testing.T) {
	path := t.TempDir() + "/paper-oco-partial-race.db"
	db1, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	db2, err := database.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db1.AutoMigrate(&models.PaperAccount{}, &models.PaperPosition{}, &models.PaperOrder{}, &models.PaperFill{}, &models.PaperEquityCheckpoint{}, &models.PaperDailyEquityBaseline{}, &models.AuditEvent{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	stopPrice, takeProfitPrice := 95.0, 105.0
	parent := models.PaperOrder{
		ClientOrderID: "oco-partial-race-parent", Environment: paperEnvironment, Symbol: "NVDA", Side: "BUY", Market: "US", Currency: "USD",
		OrderType: "LIMIT", TimeInForce: "GTC", ReferencePrice: 100, Quantity: 10, FillQty: 10, FillPrice: 100,
		ProtectiveStop: &stopPrice, TakeProfit: &takeProfitPrice, OCOGroupID: "paper-oco-partial-race", ProtectionRemainingQty: 10, ProtectionInitialized: true,
		QuoteSource: "tradingview", QuoteTime: now, Status: paperStatusSimulatedFill, FilledAt: &now,
		StatusHistory: []models.PaperOrderStatusEvent{{Status: paperStatusSimulatedFill, At: now}}, RequestHash: "oco-partial-race-parent", Version: 2,
	}
	if err := db1.Create(&models.PaperAccount{Currency: "USD", InitialCash: 10_000, Cash: 9_000, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db1.Create(&models.PaperPosition{Currency: "USD", Symbol: "NVDA", Quantity: 10, ReservedQuantity: 10, AverageCost: 100, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db1.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	stop := models.PaperOrder{
		ClientOrderID: "oco-partial-race-stop", Environment: paperEnvironment, Symbol: "NVDA", Side: "SELL", Market: "US", Currency: "USD", OrderType: "STOP_MARKET", TimeInForce: "GTC",
		ReferencePrice: 100, TriggerPrice: &stopPrice, ParentOrderID: &parent.ID, OCOGroupID: parent.OCOGroupID, Quantity: 10, RemainingQty: 10, ReservedQty: 10,
		QuoteSource: "tradingview", QuoteTime: now, Status: paperStatusAccepted, StatusHistory: []models.PaperOrderStatusEvent{{Status: paperStatusAccepted, At: now}}, RequestHash: "oco-partial-race-stop", Version: 1,
	}
	takeProfit := models.PaperOrder{
		ClientOrderID: "oco-partial-race-tp", Environment: paperEnvironment, Symbol: "NVDA", Side: "SELL", Market: "US", Currency: "USD", OrderType: "LIMIT", TimeInForce: "GTC",
		ReferencePrice: 100, Entry: &takeProfitPrice, ParentOrderID: &parent.ID, OCOGroupID: parent.OCOGroupID, Quantity: 10, RemainingQty: 10,
		QuoteSource: "tradingview", QuoteTime: now, Status: paperStatusAccepted, StatusHistory: []models.PaperOrderStatusEvent{{Status: paperStatusAccepted, At: now}}, RequestHash: "oco-partial-race-tp", Version: 1,
	}
	if err := db1.Create(&stop).Error; err != nil {
		t.Fatal(err)
	}
	if err := db1.Create(&takeProfit).Error; err != nil {
		t.Fatal(err)
	}

	type result struct {
		replay bool
		err    error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	fill := func(db *gorm.DB) {
		<-start
		_, replay, _, err := transitionPaperOrderDBWithPolicyQuantity(db, stop.ClientOrderID, paperStatusSimulatedFill,
			paperAuthoritativeQuote{Price: 94, Source: "tradingview", At: now}, now, defaultPaperRiskPolicy(), false, "oco-partial-shared", 4)
		results <- result{replay: replay, err: err}
	}
	go fill(db1)
	go fill(db2)
	close(start)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil || first.replay == second.replay {
		t.Fatalf("concurrent protective partial results first=%+v second=%+v", first, second)
	}
	if err := db1.First(&stop, stop.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db1.First(&takeProfit, takeProfit.ID).Error; err != nil {
		t.Fatal(err)
	}
	var position models.PaperPosition
	if err := db1.Where("currency = ? AND symbol = ?", "USD", "NVDA").First(&position).Error; err != nil {
		t.Fatal(err)
	}
	var fillCount int64
	if err := db1.Model(&models.PaperFill{}).Where("paper_order_id = ?", stop.ID).Count(&fillCount).Error; err != nil {
		t.Fatal(err)
	}
	if fillCount != 1 || stop.Status != paperStatusAccepted || stop.FillQty != 4 || stop.RemainingQty != 6 || takeProfit.Status != paperStatusAccepted || takeProfit.Quantity != 6 || position.Quantity != 6 || position.ReservedQuantity != 6 {
		t.Fatalf("concurrent protective partial invariant fills=%d stop=%+v sibling=%+v position=%+v", fillCount, stop, takeProfit, position)
	}
}
