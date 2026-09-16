package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"trading-agents/internal/config"
	"trading-agents/internal/database"
	"trading-agents/internal/models"
)

const (
	defaultPaperStopScanInterval = 5 * time.Second
	defaultPaperStopAuditTTL     = 15 * time.Minute
)

type paperStopScannerOptions struct {
	Clock          func() time.Time
	Quotes         func([]string) quoteFetchResult
	MarketOpen     func(string, time.Time) bool
	Interval       time.Duration
	Policy         paperPortfolioLimits
	PolicyProvider func() paperPortfolioLimits
	AuditTTL       time.Duration
}

type PaperStopScanResult struct {
	StartedAt          time.Time  `json:"startedAt"`
	CompletedAt        time.Time  `json:"completedAt"`
	QuoteObservedAt    *time.Time `json:"quoteObservedAt,omitempty"`
	DailyBaselinesMade int        `json:"dailyBaselinesCreated"`
	Candidates         int        `json:"candidates"`
	Triggered          int        `json:"triggered"`
	Skipped            int        `json:"skipped"`
	ClosedMarket       int        `json:"closedMarket"`
	Errors             int        `json:"errors"`
	EquityCheckpointed bool       `json:"equityCheckpointed"`
	Details            []string   `json:"details,omitempty"`
	auditCandidates    []string
}

type PaperStopScannerStatus struct {
	Running          bool                 `json:"running"`
	Interval         string               `json:"interval"`
	SessionPolicy    string               `json:"sessionPolicy"`
	CalendarCoverage string               `json:"calendarCoverage"`
	AuditRetention   string               `json:"auditRetention"`
	LastResult       *PaperStopScanResult `json:"lastResult,omitempty"`
	RecoveryErrors   []string             `json:"recoveryErrors,omitempty"`
	Disclaimer       string               `json:"disclaimer"`
}

type PaperStopScanner struct {
	db         *gorm.DB
	clock      func() time.Time
	quotes     func([]string) quoteFetchResult
	marketOpen func(string, time.Time) bool
	interval   time.Duration
	policy     func() paperPortfolioLimits
	auditTTL   time.Duration

	mu             sync.RWMutex
	operationMu    sync.Mutex
	running        bool
	lastResult     *PaperStopScanResult
	recoveryErrors []string
	cancel         context.CancelFunc
	done           chan struct{}
}

func NewPaperStopScanner(db *gorm.DB, configs ...*config.Config) *PaperStopScanner {
	var cfg *config.Config
	if len(configs) > 0 {
		cfg = configs[0]
	}
	options := paperStopScannerOptions{PolicyProvider: func() paperPortfolioLimits {
		return paperRiskPolicyFromConfig(cfg)
	}}
	if paperRuntimeFixtureEnabled() {
		options.Clock = paperRuntimeFixtureNow
		options.Interval = time.Hour
	}
	return newPaperStopScanner(db, options)
}

func newPaperStopScanner(db *gorm.DB, options paperStopScannerOptions) *PaperStopScanner {
	if options.Clock == nil {
		options.Clock = time.Now
	}
	if options.Quotes == nil {
		options.Quotes = boundedQuotesForRequest
	}
	if options.MarketOpen == nil {
		options.MarketOpen = paperMarketOpen
	}
	if options.Interval <= 0 {
		options.Interval = defaultPaperStopScanInterval
	}
	if options.PolicyProvider == nil {
		policy := options.Policy
		if policy.PolicyVersion == "" {
			policy = defaultPaperRiskPolicy()
		}
		options.PolicyProvider = func() paperPortfolioLimits { return policy }
	}
	if options.AuditTTL <= 0 {
		options.AuditTTL = defaultPaperStopAuditTTL
	}
	return &PaperStopScanner{
		db: db, clock: options.Clock, quotes: options.Quotes,
		marketOpen: options.MarketOpen, interval: options.Interval, policy: options.PolicyProvider, auditTTL: options.AuditTTL,
	}
}

// Start performs an immediate recovery scan in the background, then continues
// while the process is alive. Persisted ACCEPTED OCO exits are therefore
// recovered after restart, but are never represented as broker-hosted orders.
func (s *PaperStopScanner) Start(parent context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("paper stop scanner database unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return errors.New("paper stop scanner already running")
	}
	ctx, cancel := context.WithCancel(parent)
	s.running = true
	s.cancel = cancel
	s.done = make(chan struct{})
	done := s.done
	go func() {
		defer close(done)
		defer func() {
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}()
		recoveryErrors := s.RecoverProtectiveOrders(ctx)
		s.mu.Lock()
		s.recoveryErrors = append([]string(nil), recoveryErrors...)
		s.mu.Unlock()
		if !paperRuntimeManualBaselineEnabled() {
			s.ScanOnce(ctx)
		}
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.ScanOnce(ctx)
			}
		}
	}()
	return nil
}

// RecoverProtectiveOrders reconstructs missing/shrunk automatic children from
// filled BUY parents and the persisted position ledger after process restart.
func (s *PaperStopScanner) RecoverProtectiveOrders(ctx context.Context) []string {
	if s == nil || s.db == nil {
		return []string{"paper stop scanner database unavailable"}
	}
	type symbolKey struct{ Currency, Symbol string }
	var keys []symbolKey
	if err := s.db.WithContext(ctx).Model(&models.PaperOrder{}).
		Select("DISTINCT currency, symbol").
		Where("environment = ? AND side = ? AND fill_qty > 0 AND protective_stop IS NOT NULL", paperEnvironment, "BUY").
		Scan(&keys).Error; err != nil {
		return []string{"load protective recovery keys: " + err.Error()}
	}
	var failures []string
	for _, key := range keys {
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			now := s.clock().UTC()
			if err := reconcilePaperOCOOrders(tx, key.Currency, key.Symbol, 0, now, "restart-recovery"); err != nil {
				return err
			}
			return appendPaperOCORecoveryAuditTx(tx, key.Currency, key.Symbol, now)
		})
		if err != nil {
			failures = append(failures, key.Currency+":"+key.Symbol+": "+err.Error())
		}
	}
	return failures
}

func (s *PaperStopScanner) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	cancel, done, running := s.cancel, s.done, s.running
	s.mu.RUnlock()
	if !running || cancel == nil || done == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *PaperStopScanner) Status() PaperStopScannerStatus {
	status := PaperStopScannerStatus{SessionPolicy: paperStopSessionPolicy(), CalendarCoverage: paperStopCalendarCoverage(), AuditRetention: paperStopAuditRetention(), Disclaimer: paperDisclaimer()}
	if s == nil {
		return status
	}
	status.Interval = s.interval.String()
	s.mu.RLock()
	defer s.mu.RUnlock()
	status.Running = s.running
	status.RecoveryErrors = append([]string(nil), s.recoveryErrors...)
	if s.lastResult != nil {
		copy := *s.lastResult
		copy.Details = append([]string(nil), s.lastResult.Details...)
		status.LastResult = &copy
	}
	return status
}

func (h *Handler) GetPaperStopScannerStatus(c *gin.Context) {
	if h.paperStopScanner == nil {
		c.JSON(200, PaperStopScannerStatus{Running: false, SessionPolicy: paperStopSessionPolicy(), CalendarCoverage: paperStopCalendarCoverage(), AuditRetention: paperStopAuditRetention(), Disclaimer: paperDisclaimer()})
		return
	}
	c.JSON(200, h.paperStopScanner.Status())
}

func (s *PaperStopScanner) ScanOnce(ctx context.Context) PaperStopScanResult {
	if s == nil || s.db == nil {
		now := time.Now().UTC()
		return PaperStopScanResult{StartedAt: now, CompletedAt: now, Errors: 1, Details: []string{"paper stop scanner database unavailable"}}
	}
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	result := PaperStopScanResult{StartedAt: s.clock().UTC()}
	policy := s.policy()
	created, baselineErr := s.establishDailyEquityBaselines(ctx, policy, result.StartedAt)
	result.DailyBaselinesMade = created
	if baselineErr != nil {
		result.Errors++
		result.Details = append(result.Details, "establish daily equity baseline: "+baselineErr.Error())
	}
	var orders []models.PaperOrder
	if err := s.db.WithContext(ctx).
		Where("environment = ? AND status = ? AND (order_type = ? OR (order_type = ? AND side = ? AND parent_order_id IS NOT NULL))", paperEnvironment, paperStatusAccepted, "STOP_MARKET", "LIMIT", "SELL").
		Order("id asc").Find(&orders).Error; err != nil {
		result.Errors = 1
		result.Details = append(result.Details, "load accepted automatic exits: "+err.Error())
		return s.completeScan(result)
	}
	result.Candidates = len(orders)
	for _, order := range orders {
		result.auditCandidates = append(result.auditCandidates, fmt.Sprintf("%d:%s:%s:%s", order.ID, order.Side, order.OrderType, order.ClientOrderID))
	}
	openOrders := make([]models.PaperOrder, 0, len(orders))
	symbolSet := make(map[string]bool)
	for _, order := range orders {
		if !s.marketOpen(order.Market, result.StartedAt) {
			result.ClosedMarket++
			result.Details = append(result.Details, order.ClientOrderID+": market closed")
			continue
		}
		openOrders = append(openOrders, order)
		symbolSet[order.Symbol] = true
	}
	if len(openOrders) == 0 {
		return s.completeScan(result)
	}
	symbols := make([]string, 0, len(symbolSet))
	for symbol := range symbolSet {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	quoteResult := s.quotes(symbols)
	snapshot := paperQuoteSnapshotFromResult(symbols, result.StartedAt, quoteResult)
	if !snapshot.ObservedAt.IsZero() {
		observedAt := snapshot.ObservedAt
		result.QuoteObservedAt = &observedAt
	}
	for _, order := range openOrders {
		select {
		case <-ctx.Done():
			result.Errors++
			result.Details = append(result.Details, "scan cancelled: "+ctx.Err().Error())
			return s.completeScan(result)
		default:
		}
		symbol := strings.ToUpper(strings.TrimSpace(order.Symbol))
		quote := snapshot.Quotes[symbol]
		if reasons := snapshot.Reasons[symbol]; len(reasons) > 0 {
			result.Errors++
			result.Details = append(result.Details, order.ClientOrderID+": "+strings.Join(reasons, "; "))
			continue
		}
		if _, reason := simulatedPaperFillPrice(order, quote.Price); reason != "" {
			if strings.Contains(reason, "has not triggered") {
				result.Skipped++
			} else {
				result.Errors++
			}
			result.Details = append(result.Details, order.ClientOrderID+": "+reason)
			continue
		}
		_, replay, conflict, err := transitionPaperOrderDBWithPolicy(s.db.WithContext(ctx), order.ClientOrderID, paperStatusSimulatedFill, quote, result.StartedAt, policy, true)
		if err != nil {
			if errors.Is(err, errPaperStateConflict) || errors.Is(err, errPaperNotFound) || errors.Is(err, errPaperFillRiskRejected) {
				result.Skipped++
			} else {
				result.Errors++
			}
			if conflict == "" {
				conflict = err.Error()
			}
			result.Details = append(result.Details, order.ClientOrderID+": "+conflict)
			continue
		}
		if replay {
			result.Skipped++
		} else {
			result.Triggered++
		}
	}
	if result.QuoteObservedAt != nil {
		summary, err := buildPaperPortfolioRiskAt(s.db.WithContext(ctx), nil, policy, result.StartedAt)
		if err != nil {
			result.Errors++
			result.Details = append(result.Details, "persist equity checkpoint: "+err.Error())
		} else {
			written, checkpointErr := persistPaperEquityCheckpoints(s.db.WithContext(ctx), summary, result.StartedAt)
			if checkpointErr != nil {
				result.Errors++
				result.Details = append(result.Details, "persist equity checkpoint: "+checkpointErr.Error())
			} else {
				result.EquityCheckpointed = written > 0
			}
		}
	}
	return s.completeScan(result)
}

type paperDailyEquityBaselineResponse struct {
	Environment string                          `json:"environment"`
	Currency    string                          `json:"currency"`
	MarketDate  string                          `json:"marketDate"`
	Created     bool                            `json:"created"`
	Baseline    models.PaperDailyEquityBaseline `json:"baseline"`
	Disclaimer  string                          `json:"disclaimer"`
}

// EstablishPaperDailyEquityBaseline asks the local scanner to perform only its
// authoritative create-only baseline step. It never accepts client equity and
// never evaluates or triggers an order.
func (h *Handler) EstablishPaperDailyEquityBaseline(c *gin.Context) {
	if c.Request.ContentLength != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "paper daily baseline accepts no request body; equity is server-computed"})
		return
	}
	currency := strings.ToUpper(strings.TrimSpace(c.Query("currency")))
	marketCode := paperMarketForCurrency(currency)
	if marketCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "currency must be USD or CNY"})
		return
	}
	if database.DB == nil || h.paperStopScanner == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "paper daily baseline scanner is unavailable"})
		return
	}

	scanner := h.paperStopScanner
	now := scanner.clock().UTC()
	marketDate := paperMarketDateForCurrency(currency, now)
	findBaseline := func() (models.PaperDailyEquityBaseline, error) {
		var baseline models.PaperDailyEquityBaseline
		err := database.DB.Where("currency = ? AND market_date = ?", currency, marketDate).First(&baseline).Error
		return baseline, err
	}
	if existing, err := findBaseline(); err == nil {
		c.JSON(http.StatusOK, paperDailyEquityBaselineResponse{
			Environment: paperEnvironment, Currency: currency, MarketDate: marketDate,
			Created: false, Baseline: existing, Disclaimer: paperDisclaimer(),
		})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !scanner.Status().Running {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "paper daily baseline scanner is not running"})
		return
	}
	if !scanner.marketOpen(marketCode, now) {
		c.JSON(http.StatusConflict, gin.H{"error": "paper daily baseline is available only during the covered regular trading session"})
		return
	}

	scanner.operationMu.Lock()
	createdCount, establishErr := scanner.establishDailyEquityBaselines(c.Request.Context(), scanner.policy(), now)
	scanner.operationMu.Unlock()
	if establishErr != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": establishErr.Error()})
		return
	}
	baseline, err := findBaseline()
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "complete authoritative valuation did not establish the requested paper daily baseline"})
		return
	}
	created := createdCount > 0
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, paperDailyEquityBaselineResponse{
		Environment: paperEnvironment, Currency: currency, MarketDate: marketDate,
		Created: created, Baseline: baseline, Disclaimer: paperDisclaimer(),
	})
}

func (s *PaperStopScanner) completeScan(result PaperStopScanResult) PaperStopScanResult {
	result.CompletedAt = s.clock().UTC()
	status := "COMPLETED"
	if result.Errors > 0 {
		status = "DEGRADED"
	}
	fingerprint := paperStopScanFingerprint(result)
	if (result.DailyBaselinesMade > 0 || result.Candidates > 0 || result.Errors > 0) && s.shouldPersistScanAudit(fingerprint, result) {
		audit := models.PaperStopScanAudit{
			StartedAt: result.StartedAt, CompletedAt: result.CompletedAt, QuoteObservedAt: result.QuoteObservedAt,
			Status: status, CandidateCount: result.Candidates, TriggeredCount: result.Triggered,
			SkippedCount: result.Skipped, ClosedMarketCount: result.ClosedMarket, ErrorCount: result.Errors,
			EquityCheckpointed: result.EquityCheckpointed, DailyBaselineCount: result.DailyBaselinesMade,
			StateFingerprint: fingerprint, Details: strings.Join(result.Details, "\n"),
		}
		if err := s.db.Create(&audit).Error; err != nil {
			result.Errors++
			result.Details = append(result.Details, "persist scan audit: "+err.Error())
		}
	}
	s.mu.Lock()
	copy := result
	copy.Details = append([]string(nil), result.Details...)
	s.lastResult = &copy
	s.mu.Unlock()
	return result
}

func (s *PaperStopScanner) shouldPersistScanAudit(fingerprint string, result PaperStopScanResult) bool {
	if result.Triggered > 0 {
		return true
	}
	var latest models.PaperStopScanAudit
	err := s.db.Order("completed_at desc, id desc").First(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true
	}
	if err != nil || latest.StateFingerprint != fingerprint {
		return true
	}
	return result.CompletedAt.Sub(latest.CompletedAt) >= s.auditTTL
}

func paperStopScanFingerprint(result PaperStopScanResult) string {
	return paperSystemAuditHash(struct {
		Candidates   []string `json:"candidates"`
		CandidateN   int      `json:"candidateCount"`
		Skipped      int      `json:"skipped"`
		ClosedMarket int      `json:"closedMarket"`
		Errors       int      `json:"errors"`
	}{result.auditCandidates, result.Candidates, result.Skipped, result.ClosedMarket, result.Errors})
}

// establishDailyEquityBaselines captures market data before opening the
// immediate SQLite transaction, then creates each open currency account's
// first complete equity observation exactly once for that market date.
func (s *PaperStopScanner) establishDailyEquityBaselines(ctx context.Context, policy paperPortfolioLimits, now time.Time) (int, error) {
	var accounts []models.PaperAccount
	if err := s.db.WithContext(ctx).Order("currency asc").Find(&accounts).Error; err != nil {
		return 0, err
	}
	needed := make(map[string]string)
	for _, account := range accounts {
		market := paperMarketForCurrency(account.Currency)
		if market == "" || !s.marketOpen(market, now) {
			continue
		}
		marketDate := paperMarketDateForCurrency(account.Currency, now)
		var count int64
		if err := s.db.WithContext(ctx).Model(&models.PaperDailyEquityBaseline{}).
			Where("currency = ? AND market_date = ?", account.Currency, marketDate).Count(&count).Error; err != nil {
			return 0, err
		}
		if count == 0 {
			needed[account.Currency] = marketDate
		}
	}
	if len(needed) == 0 {
		return 0, nil
	}

	// This may call quote providers and must remain outside the write transaction.
	snapshot := capturePaperPortfolioMarketSnapshot(s.db.WithContext(ctx), nil, now)
	createdCount := 0
	missing := []string{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		summary, err := buildPaperPortfolioRiskFromSnapshotAt(tx, nil, policy, now, snapshot)
		if err != nil {
			return err
		}
		eligible := summary
		eligible.Groups = eligible.Groups[:0]
		for _, group := range summary.Groups {
			if _, ok := needed[group.Currency]; ok {
				eligible.Groups = append(eligible.Groups, group)
			}
		}
		created, err := ensurePaperDailyEquityBaselinesTx(tx, eligible, now)
		if err != nil {
			return err
		}
		for _, baseline := range created {
			if err := appendPaperDailyBaselineAuditTx(tx, baseline); err != nil {
				return err
			}
		}
		createdCount = len(created)
		for currency, marketDate := range needed {
			var count int64
			if err := tx.Model(&models.PaperDailyEquityBaseline{}).
				Where("currency = ? AND market_date = ?", currency, marketDate).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				missing = append(missing, currency+":"+marketDate)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return createdCount, fmt.Errorf("complete valuation unavailable for %s", strings.Join(missing, ","))
	}
	return createdCount, nil
}

func paperMarketForCurrency(currency string) string {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "USD":
		return "US"
	case "CNY":
		return "CN"
	default:
		return ""
	}
}

func appendPaperDailyBaselineAuditTx(tx *gorm.DB, baseline models.PaperDailyEquityBaseline) error {
	payloadHash := paperSystemAuditHash(paperDailyBaselineAuditPayload(baseline))
	outcomeHash := paperSystemAuditHash(paperDailyBaselineAuditOutcome(baseline))
	return appendCompletedSystemAuditActorTx(tx,
		"paper-day-start:"+baseline.Currency+":"+baseline.MarketDate,
		"system:paper-stop-scanner", paperDailyBaselineAuditAction,
		paperDailyBaselineAuditTarget(baseline), payloadHash, outcomeHash, baseline.ObservedAt)
}

func paperDailyBaselineAuditTarget(baseline models.PaperDailyEquityBaseline) string {
	return fmt.Sprintf("%s%d", paperDailyBaselineAuditTargetPrefix, baseline.ID)
}

func paperDailyBaselineAuditPayload(baseline models.PaperDailyEquityBaseline) any {
	return struct {
		Currency   string `json:"currency"`
		MarketDate string `json:"marketDate"`
		Source     string `json:"source"`
	}{baseline.Currency, baseline.MarketDate, "paper-stop-scanner"}
}

func paperDailyBaselineAuditOutcome(baseline models.PaperDailyEquityBaseline) any {
	return struct {
		ID         uint      `json:"id"`
		ObservedAt time.Time `json:"observedAt"`
		Equity     float64   `json:"equity"`
	}{baseline.ID, baseline.ObservedAt.UTC(), baseline.Equity}
}

func paperMarketOpen(market string, now time.Time) bool {
	market = strings.ToUpper(strings.TrimSpace(market))
	locationName := map[string]string{"US": "America/New_York", "CN": "Asia/Shanghai"}[market]
	if locationName == "" {
		return false
	}
	location, err := time.LoadLocation(locationName)
	if err != nil {
		return false
	}
	local := now.In(location)
	if local.Weekday() == time.Saturday || local.Weekday() == time.Sunday {
		return false
	}
	date := local.Format("2006-01-02")
	if local.Year() != 2026 || paperExchangeClosed2026[market][date] {
		return false
	}
	minute := local.Hour()*60 + local.Minute()
	switch market {
	case "US":
		closeMinute := 16 * 60
		if paperUSEarlyClose2026[date] {
			closeMinute = 13 * 60
		}
		return minute >= 9*60+30 && minute < closeMinute
	case "CN":
		return (minute >= 9*60+30 && minute < 11*60+30) || (minute >= 13*60 && minute < 15*60)
	default:
		return false
	}
}

func paperStopSessionPolicy() string {
	return "official 2026 NYSE/SSE regular-session calendar; dates outside calendar coverage fail closed"
}

func paperStopCalendarCoverage() string {
	return "NYSE and SSE regular sessions for calendar year 2026 only; all other dates are closed"
}

func paperStopAuditRetention() string {
	return "state-change + 15m TTL: candidate/error scans are durable only when state changes or the retention TTL expires; idle scans update in-memory lastResult only"
}

// Calendar dates come from the official NYSE 2026 holiday calendar and SSE
// 2026 closure notice. Unsupported years deliberately remain closed until an
// reviewed calendar is added.
var paperExchangeClosed2026 = map[string]map[string]bool{
	"US": {
		"2026-01-01": true, "2026-01-19": true, "2026-02-16": true,
		"2026-04-03": true, "2026-05-25": true, "2026-06-19": true,
		"2026-07-03": true, "2026-09-07": true, "2026-11-26": true,
		"2026-12-25": true,
	},
	"CN": {
		"2026-01-01": true, "2026-01-02": true,
		"2026-02-16": true, "2026-02-17": true, "2026-02-18": true,
		"2026-02-19": true, "2026-02-20": true, "2026-02-23": true,
		"2026-04-06": true,
		"2026-05-01": true, "2026-05-04": true, "2026-05-05": true,
		"2026-06-19": true, "2026-09-25": true,
		"2026-10-01": true, "2026-10-02": true, "2026-10-05": true,
		"2026-10-06": true, "2026-10-07": true,
	},
}

var paperUSEarlyClose2026 = map[string]bool{
	"2026-11-27": true,
	"2026-12-24": true,
}
