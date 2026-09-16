package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"trading-agents/internal/agents"
	"trading-agents/internal/database"
	"trading-agents/internal/market"
	"trading-agents/internal/models"
)

const (
	paperEnvironment         = "PAPER"
	paperStatusAccepted      = "ACCEPTED"
	paperStatusRejected      = "REJECTED"
	paperStatusCancelled     = "CANCELLED"
	paperStatusSimulatedFill = "SIMULATED_FILLED"

	paperReferenceTolerance = 0.01
	paperSlippageRate       = 0.0005
	paperFeeRate            = 0.0005
	paperMinimumFee         = 1.0
	paperFillRiskAuditTTL   = 15 * time.Minute
)

var (
	errPaperNotFound         = errors.New("paper order not found")
	errPaperStateConflict    = errors.New("paper order state conflict")
	errPaperLedgerConflict   = errors.New("paper account changed concurrently")
	errPaperFillRiskRejected = errors.New("paper BUY fill blocked by current risk policy")
)

type paperOrderRequest struct {
	ClientOrderID  string                   `json:"clientOrderId"`
	ResearchRunID  string                   `json:"researchRunId"`
	ResearchTicker string                   `json:"researchTicker"`
	Environment    string                   `json:"environment"`
	Symbol         string                   `json:"symbol"`
	Side           string                   `json:"side"`
	Market         string                   `json:"market"`
	Currency       string                   `json:"currency"`
	OrderType      string                   `json:"orderType"`
	TimeInForce    string                   `json:"timeInForce"`
	ReferencePrice float64                  `json:"referencePrice"`
	Entry          *float64                 `json:"entry"`
	TriggerPrice   *float64                 `json:"triggerPrice"`
	ProtectiveStop *float64                 `json:"protectiveStop"`
	TakeProfit     *float64                 `json:"takeProfit"`
	LegacyStop     *float64                 `json:"stop"`
	Quantity       int                      `json:"quantity"`
	QuoteSource    string                   `json:"quoteSource"`
	QuoteTime      string                   `json:"quoteTime"`
	RiskSnapshot   models.PaperRiskSnapshot `json:"riskSnapshot"`
}

type paperValidationResponse struct {
	Environment      string                     `json:"environment"`
	Valid            bool                       `json:"valid"`
	Degraded         bool                       `json:"degraded"`
	Warnings         []string                   `json:"warnings,omitempty"`
	RejectionReasons []string                   `json:"rejectionReasons"`
	Portfolio        *paperPortfolioRiskSummary `json:"portfolio,omitempty"`
}

type paperOrderResponse struct {
	Order              models.PaperOrder `json:"order"`
	IdempotentReplay   bool              `json:"idempotentReplay"`
	EquityCheckpointed bool              `json:"equityCheckpointed,omitempty"`
	Disclaimer         string            `json:"disclaimer"`
	Warning            string            `json:"warning,omitempty"`
	Error              string            `json:"error,omitempty"`
	FillRejected       bool              `json:"fillRejected,omitempty"`
	RejectionReasons   []string          `json:"rejectionReasons,omitempty"`
}

type paperFillTransitionRequest struct {
	FillID   string `json:"fillId"`
	Quantity int    `json:"quantity"`
}

type paperAuthoritativeQuote struct {
	Price       float64   `json:"price"`
	Source      string    `json:"source"`
	At          time.Time `json:"observedAt"`
	ProviderURL string    `json:"providerURL,omitempty"`
}

type paperAuthoritativeQuoteSnapshot struct {
	Quotes     map[string]paperAuthoritativeQuote
	Reasons    map[string][]string
	ObservedAt time.Time
}

type paperValidation struct {
	Reasons      []string
	Quote        paperAuthoritativeQuote
	Risk         models.PaperRiskSnapshot
	ReservedCash float64
	ReservedQty  int
}

func normalizePaperOrderRequest(req paperOrderRequest) paperOrderRequest {
	req.ClientOrderID = strings.TrimSpace(req.ClientOrderID)
	req.ResearchRunID = strings.TrimSpace(req.ResearchRunID)
	req.ResearchTicker = strings.ToUpper(strings.TrimSpace(req.ResearchTicker))
	req.Environment = strings.ToUpper(strings.TrimSpace(req.Environment))
	req.Symbol = strings.ToUpper(strings.TrimSpace(req.Symbol))
	req.Side = strings.ToUpper(strings.TrimSpace(req.Side))
	req.Market = strings.ToUpper(strings.TrimSpace(req.Market))
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	req.OrderType = strings.ToUpper(strings.TrimSpace(req.OrderType))
	req.TimeInForce = strings.ToUpper(strings.TrimSpace(req.TimeInForce))
	return req
}

func validatePaperOrderShape(req paperOrderRequest) []string {
	reasons := make([]string, 0)
	if req.Environment != paperEnvironment {
		reasons = append(reasons, "environment must be PAPER")
	}
	if req.ClientOrderID == "" || len(req.ClientOrderID) > 80 {
		reasons = append(reasons, "clientOrderId is required and must be at most 80 characters")
	}
	if len(req.ResearchRunID) > 100 {
		reasons = append(reasons, "researchRunId must be at most 100 characters")
	}
	if req.ResearchTicker != "" && !safeTickerRe.MatchString(req.ResearchTicker) {
		reasons = append(reasons, "researchTicker must be a safe ticker")
	}
	if (req.ResearchRunID == "") != (req.ResearchTicker == "") {
		reasons = append(reasons, "researchRunId and researchTicker must be supplied together")
	}
	if req.Symbol == "" || !safeTickerRe.MatchString(req.Symbol) {
		reasons = append(reasons, "symbol is required and must be a safe ticker")
	}
	if req.Side != "BUY" && req.Side != "SELL" {
		reasons = append(reasons, "side must be BUY or SELL")
	}
	if req.Market != "US" && req.Market != "CN" {
		reasons = append(reasons, "market must be US or CN")
	}
	if req.Symbol != "" {
		expectedMarket := "US"
		if isPaperCNSymbol(req.Symbol) {
			expectedMarket = "CN"
		}
		if req.Market != expectedMarket {
			reasons = append(reasons, "symbol does not match market")
		}
	}
	expectedCurrency := map[string]string{"US": "USD", "CN": "CNY"}[req.Market]
	if expectedCurrency == "" || req.Currency != expectedCurrency {
		reasons = append(reasons, "currency does not match market")
	}
	if req.OrderType != "LIMIT" && req.OrderType != "STOP_MARKET" {
		reasons = append(reasons, "orderType must be LIMIT or STOP_MARKET")
	}
	if req.TimeInForce != "GTC" {
		reasons = append(reasons, "timeInForce must be GTC")
	}
	if req.LegacyStop != nil {
		reasons = append(reasons, "stop is ambiguous and unsupported; use triggerPrice or protectiveStop")
	}
	switch {
	case req.Side == "BUY" && req.OrderType == "LIMIT":
		if req.Entry == nil || *req.Entry <= 0 {
			reasons = append(reasons, "BUY LIMIT requires a positive entry")
		}
		if req.TriggerPrice != nil {
			reasons = append(reasons, "BUY LIMIT triggerPrice must be null")
		}
	case req.Side == "BUY" && req.OrderType == "STOP_MARKET":
		if req.Entry != nil {
			reasons = append(reasons, "BUY STOP_MARKET entry must be null")
		}
		if req.TriggerPrice == nil || *req.TriggerPrice <= 0 {
			reasons = append(reasons, "BUY STOP_MARKET requires a positive triggerPrice")
		}
	case req.Side == "SELL" && req.OrderType == "STOP_MARKET":
		if req.Entry != nil {
			reasons = append(reasons, "SELL STOP_MARKET entry must be null")
		}
		if req.TriggerPrice == nil || *req.TriggerPrice <= 0 {
			reasons = append(reasons, "SELL STOP_MARKET requires a positive triggerPrice")
		}
		if req.ProtectiveStop != nil || req.TakeProfit != nil {
			reasons = append(reasons, "SELL STOP_MARKET is the protective exit; protectiveStop and takeProfit must be null")
		}
	case req.Side == "SELL" && req.OrderType == "LIMIT":
		reasons = append(reasons, "SELL LIMIT is outside the supported paper API scope")
	}
	if req.Side == "BUY" {
		if req.ProtectiveStop == nil || *req.ProtectiveStop <= 0 {
			reasons = append(reasons, "BUY orders require a positive protectiveStop")
		}
		if req.TakeProfit == nil || *req.TakeProfit <= 0 {
			reasons = append(reasons, "BUY orders require a positive takeProfit")
		}
	}
	if req.ReferencePrice <= 0 || req.Quantity <= 0 {
		reasons = append(reasons, "referencePrice and quantity must be positive")
	}
	return reasons
}

func (h *Handler) validatePaperResearchProvenance(req paperOrderRequest) []string {
	if req.ResearchRunID == "" && req.ResearchTicker == "" {
		return nil
	}
	if req.ResearchRunID == "" || req.ResearchTicker == "" {
		return []string{"research run id and ticker must be supplied together"}
	}
	if req.ResearchTicker != req.Symbol {
		return []string{"research ticker must match the Paper order symbol"}
	}
	if h == nil || h.config == nil || strings.TrimSpace(h.config.ResultsDir) == "" {
		return []string{"research run store is unavailable"}
	}
	for _, result := range h.loadResultsRaw() {
		if result.Audit.RunID != req.ResearchRunID {
			continue
		}
		if strings.ToUpper(strings.TrimSpace(result.Ticker)) != req.ResearchTicker {
			return []string{"research run ticker does not match the declared research ticker"}
		}
		health := agents.ApplyPublicationGate(&result)
		if !health.Publishable || (result.Status != agents.ResearchStatusPublished && result.Status != agents.ResearchStatusEvidenceOnly) {
			return []string{"research run is not publication-gated and reviewable"}
		}
		if err := verifyResearchPayloadArtifacts(h.config.ResultsDir, &result); err != nil {
			return []string{"research run evidence is unavailable: " + err.Error()}
		}
		return nil
	}
	return []string{"research run was not found"}
}

func isPaperCNSymbol(symbol string) bool {
	if len(symbol) != 6 {
		return false
	}
	for _, ch := range symbol {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func fetchAuthoritativePaperQuote(symbol string, now time.Time) (paperAuthoritativeQuote, []string) {
	snapshot := fetchAuthoritativePaperQuotes([]string{symbol}, now)
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	return snapshot.Quotes[symbol], snapshot.Reasons[symbol]
}

func fetchAuthoritativePaperQuotes(symbols []string, now time.Time) paperAuthoritativeQuoteSnapshot {
	snapshot := paperAuthoritativeQuoteSnapshot{
		Quotes:  make(map[string]paperAuthoritativeQuote),
		Reasons: make(map[string][]string),
	}
	unique := make(map[string]bool, len(symbols))
	requested := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		if symbol == "" {
			snapshot.Reasons[symbol] = []string{"authoritative quote unavailable for empty symbol"}
			continue
		}
		if !unique[symbol] {
			unique[symbol] = true
			requested = append(requested, symbol)
		}
	}
	if len(requested) == 0 {
		return snapshot
	}
	sort.Strings(requested)
	result := boundedQuotesForRequest(requested)
	return paperQuoteSnapshotFromResult(requested, now, result)
}

func paperQuoteSnapshotFromResult(requested []string, now time.Time, result quoteFetchResult) paperAuthoritativeQuoteSnapshot {
	snapshot := paperAuthoritativeQuoteSnapshot{
		Quotes:  make(map[string]paperAuthoritativeQuote),
		Reasons: make(map[string][]string),
	}
	snapshot.ObservedAt = result.dataTime.UTC()
	stale, staleReason := quoteResultFreshness(result, now)
	resultSource := strings.ToLower(strings.TrimSpace(result.source))
	if resultSource == "" || strings.Contains(resultSource, "unknown") || strings.Contains(resultSource, "stale") || sourceIsDisallowedForProduct(result.source) {
		stale = true
		if staleReason == "" {
			staleReason = "quote source is unknown, stale, mock or fallback"
		}
	}
	if result.dataTime.After(now.Add(30 * time.Second)) {
		stale = true
		staleReason = "quote data time is unreasonably in the future"
	}
	quotesBySymbol := make(map[string]market.Quote, len(result.quotes))
	for key, quote := range result.quotes {
		quotesBySymbol[strings.ToUpper(strings.TrimSpace(key))] = quote
	}
	for _, symbol := range requested {
		quote, ok := quotesBySymbol[symbol]
		source := strings.TrimSpace(quote.Source)
		paperQuote := paperAuthoritativeQuote{
			Price: quote.Price, Source: source, At: result.dataTime.UTC(), ProviderURL: strings.TrimSpace(quote.ProviderURL),
		}
		snapshot.Quotes[symbol] = paperQuote
		if !ok || quote.Price <= 0 {
			snapshot.Reasons[symbol] = []string{"authoritative quote unavailable for symbol"}
			continue
		}
		lowerSource := strings.ToLower(source)
		quoteStale := stale || source == "" || strings.Contains(lowerSource, "unknown") || strings.Contains(lowerSource, "stale") || sourceIsDisallowedForProduct(source) || strings.HasPrefix(strings.ToLower(strings.TrimSpace(quote.ProviderMode)), "display-only")
		quoteStaleReason := staleReason
		if quoteStale && quoteStaleReason == "" {
			quoteStaleReason = "quote source is unknown, stale, display-only, mock or fallback"
		}
		if quoteStale {
			snapshot.Reasons[symbol] = []string{"authoritative quote is stale: " + quoteStaleReason}
		}
	}
	return snapshot
}

func validatePaperOrderAgainstLedger(req paperOrderRequest, quote paperAuthoritativeQuote, account models.PaperAccount, position models.PaperPosition) paperValidation {
	reasons := validatePaperOrderShape(req)
	if quote.Price <= 0 || quote.At.IsZero() || quote.Source == "" {
		reasons = append(reasons, "authoritative quote is incomplete")
	} else if req.ReferencePrice > 0 {
		deviation := math.Abs(req.ReferencePrice-quote.Price) / quote.Price
		if deviation > paperReferenceTolerance {
			reasons = append(reasons, fmt.Sprintf("referencePrice deviates %.2f%% from authoritative quote; limit is %.2f%%", deviation*100, paperReferenceTolerance*100))
		}
	}

	capital := account.Cash
	if account.ID == 0 || account.Cash < 0 || account.ReservedCash < 0 {
		reasons = append(reasons, "paper account is unavailable or invalid")
	}
	if req.Side == "BUY" && capital <= 0 {
		reasons = append(reasons, "paper account has no available cash for BUY")
	}
	plannedExecutionPrice := quote.Price
	if req.Entry != nil {
		plannedExecutionPrice = *req.Entry
	} else if req.TriggerPrice != nil {
		plannedExecutionPrice = *req.TriggerPrice
	}
	lossPerShare := 0.0
	if req.Side == "BUY" {
		if req.ProtectiveStop != nil {
			lossPerShare = plannedExecutionPrice - *req.ProtectiveStop
			if lossPerShare <= 0 {
				reasons = append(reasons, "protectiveStop must be below the planned BUY execution price")
				lossPerShare = 0
			}
		}
		if req.TakeProfit != nil && *req.TakeProfit <= plannedExecutionPrice {
			reasons = append(reasons, "takeProfit must exceed the planned BUY execution price")
		}
		if req.OrderType == "STOP_MARKET" && req.TriggerPrice != nil && *req.TriggerPrice <= quote.Price {
			reasons = append(reasons, "BUY STOP_MARKET triggerPrice must be above the authoritative quote")
		}
	} else if req.Side == "SELL" && req.OrderType == "STOP_MARKET" && req.TriggerPrice != nil {
		if *req.TriggerPrice >= quote.Price {
			reasons = append(reasons, "SELL STOP_MARKET triggerPrice must be below the authoritative quote")
		}
		lossPerShare = math.Max(quote.Price-*req.TriggerPrice, 0)
	}
	risk := models.PaperRiskSnapshot{
		InvestableCapital: capital,
		MaxLoss:           roundPaperMoney(float64(req.Quantity) * lossPerShare),
		MaxRiskAmount:     roundPaperMoney(capital * 0.015),
		PlannedNotional:   roundPaperMoney(float64(req.Quantity) * plannedExecutionPrice),
		MaxNotionalAmount: roundPaperMoney(capital * 0.15),
	}
	clientRisk := req.RiskSnapshot
	if !clientRisk.RiskLimitPassed {
		reasons = append(reasons, "client risk snapshot did not pass")
	}

	validation := paperValidation{Reasons: reasons, Quote: quote, Risk: risk}
	if req.Side == "BUY" {
		validation.ReservedCash = roundPaperMoney(risk.PlannedNotional + paperFee(risk.PlannedNotional))
		if account.Cash+0.01 < validation.ReservedCash {
			validation.Reasons = append(validation.Reasons, "insufficient available paper cash")
		}
	} else if req.Side == "SELL" {
		validation.ReservedQty = req.Quantity
		available := position.Quantity - position.ReservedQuantity
		if position.ID == 0 || available < req.Quantity {
			validation.Reasons = append(validation.Reasons, "insufficient available paper position")
		}
	}
	validation.Risk.RiskLimitPassed = len(validation.Reasons) == 0
	return validation
}

func loadPaperLedger(tx *gorm.DB, currency, symbol string) (models.PaperAccount, models.PaperPosition, error) {
	var account models.PaperAccount
	if err := tx.Where("currency = ?", currency).First(&account).Error; err != nil {
		return account, models.PaperPosition{}, err
	}
	var position models.PaperPosition
	err := tx.Where("currency = ? AND symbol = ?", currency, symbol).First(&position).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	return account, position, err
}

func paperRequestHash(req paperOrderRequest) string {
	raw, _ := json.Marshal(req)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func paperDisclaimer() string {
	return "Paper simulation only. USD and CNY are isolated Paper subaccounts; no unsourced FX total is implied. Automatic protective STOP_MARKET and take-profit SELL LIMIT legs form a local OCO group and trigger only from fresh authoritative quotes observed by this process while it is running. They are not broker-hosted GTC orders. No broker order is sent and no fill is real."
}

func (h *Handler) ValidatePaperOrder(c *gin.Context) {
	var req paperOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req = normalizePaperOrderRequest(req)
	now := h.paperNow()
	quote, quoteReasons := fetchAuthoritativePaperQuote(req.Symbol, now)
	validation := paperValidation{Reasons: validatePaperOrderShape(req), Quote: quote}
	validation.Reasons = append(validation.Reasons, h.validatePaperResearchProvenance(req)...)
	var portfolio *paperPortfolioRiskSummary
	if database.DB == nil {
		validation.Reasons = append(validation.Reasons, quoteReasons...)
		validation.Reasons = append(validation.Reasons, "paper account store unavailable")
	} else {
		account, position, err := loadPaperLedger(database.DB, req.Currency, req.Symbol)
		if err != nil {
			validation.Reasons = append(validation.Reasons, quoteReasons...)
			validation.Reasons = append(validation.Reasons, "paper account unavailable: "+err.Error())
		} else {
			validationPosition := position
			reservationReason := ""
			if req.Side == "SELL" {
				protectedQty, reservationErr := acceptedProtectiveReservation(database.DB, req.Currency, req.Symbol)
				if reservationErr != nil {
					reservationReason = "paper protective reservation unavailable: " + reservationErr.Error()
				} else {
					validationPosition.ReservedQuantity -= protectedQty
				}
			}
			validation = validatePaperOrderAgainstLedger(req, quote, account, validationPosition)
			if reservationReason != "" {
				validation.Reasons = append(validation.Reasons, reservationReason)
			}
			validation.Reasons = append(validation.Reasons, quoteReasons...)
			summary, portfolioErr := buildPaperPortfolioRiskAt(database.DB, &paperPortfolioProjection{Request: req, Risk: validation.Risk}, paperRiskPolicyFromConfig(h.config), now)
			if portfolioErr != nil {
				validation.Reasons = append(validation.Reasons, "paper portfolio risk unavailable: "+portfolioErr.Error())
			} else {
				portfolio = &summary
				applyPaperPortfolioLimits(req, &validation, summary, now, "submission")
			}
		}
	}
	validation.Risk.RiskLimitPassed = len(validation.Reasons) == 0
	c.JSON(http.StatusOK, paperValidationResponse{
		Environment: paperEnvironment, Valid: len(validation.Reasons) == 0,
		Degraded: validation.Risk.PortfolioDegraded, Warnings: validation.Risk.RiskWarnings,
		RejectionReasons: validation.Reasons, Portfolio: portfolio,
	})
}

func (h *Handler) SubmitPaperOrder(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "paper order store unavailable"})
		return
	}
	var req paperOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req = normalizePaperOrderRequest(req)
	if req.ClientOrderID == "" || len(req.ClientOrderID) > 80 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "clientOrderId is required and must be at most 80 characters"})
		return
	}
	hash := paperRequestHash(req)
	now := h.paperNow()
	quote, quoteReasons := fetchAuthoritativePaperQuote(req.Symbol, now)
	portfolioSnapshot := capturePaperPortfolioMarketSnapshot(database.DB, &paperPortfolioProjection{Request: req}, now)

	var order models.PaperOrder
	idempotentReplay := false
	payloadConflict := false
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var existing models.PaperOrder
		lookupErr := tx.Where("client_order_id = ?", req.ClientOrderID).First(&existing).Error
		if lookupErr == nil {
			if existing.RequestHash != hash {
				payloadConflict = true
				return errPaperStateConflict
			}
			order = existing
			idempotentReplay = true
			return nil
		}
		if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}

		account, position, ledgerErr := loadPaperLedger(tx, req.Currency, req.Symbol)
		validation := paperValidation{Reasons: validatePaperOrderShape(req), Quote: quote}
		validation.Reasons = append(validation.Reasons, quoteReasons...)
		if ledgerErr != nil {
			validation.Reasons = append(validation.Reasons, "paper account unavailable: "+ledgerErr.Error())
		} else {
			reservationReason := ""
			if req.Side == "SELL" {
				if protectedQty, reservationErr := acceptedProtectiveReservation(tx, req.Currency, req.Symbol); reservationErr == nil {
					position.ReservedQuantity -= protectedQty
				} else {
					reservationReason = "paper protective reservation unavailable: " + reservationErr.Error()
				}
			}
			validation = validatePaperOrderAgainstLedger(req, quote, account, position)
			if reservationReason != "" {
				validation.Reasons = append(validation.Reasons, reservationReason)
			}
			validation.Reasons = append(validation.Reasons, quoteReasons...)
			policy := paperRiskPolicyFromConfig(h.config)
			summary, portfolioErr := buildPaperPortfolioRiskFromSnapshotAt(tx, &paperPortfolioProjection{Request: req, Risk: validation.Risk}, policy, now, portfolioSnapshot)
			if portfolioErr != nil {
				validation.Reasons = append(validation.Reasons, "paper portfolio risk unavailable: "+portfolioErr.Error())
			} else {
				applyPaperPortfolioLimits(req, &validation, summary, now, "submission")
			}
		}
		validation.Reasons = append(validation.Reasons, h.validatePaperResearchProvenance(req)...)
		validation.Risk.RiskLimitPassed = len(validation.Reasons) == 0
		status := paperStatusAccepted
		reason := ""
		if len(validation.Reasons) > 0 {
			status = paperStatusRejected
			reason = strings.Join(validation.Reasons, "; ")
		} else {
			if req.Side == "SELL" {
				if err := reconcileProtectivePaperOrders(tx, req.Currency, req.Symbol, req.Quantity, now); err != nil {
					return err
				}
				account, position, ledgerErr = loadPaperLedger(tx, req.Currency, req.Symbol)
				if ledgerErr != nil {
					return ledgerErr
				}
			}
			if err := reservePaperOrder(tx, &account, &position, req.Side, validation.ReservedCash, validation.ReservedQty); err != nil {
				return err
			}
		}
		order = models.PaperOrder{
			ClientOrderID: req.ClientOrderID, ResearchRunID: req.ResearchRunID, ResearchTicker: req.ResearchTicker,
			Environment: paperEnvironment, Symbol: req.Symbol,
			Side: req.Side, Market: req.Market, Currency: req.Currency, OrderType: req.OrderType,
			TimeInForce: req.TimeInForce, ReferencePrice: quote.Price, Entry: req.Entry, TriggerPrice: req.TriggerPrice,
			ProtectiveStop: req.ProtectiveStop, TakeProfit: req.TakeProfit,
			Quantity: req.Quantity, QuoteSource: quote.Source, QuoteTime: quote.At,
			RiskSnapshot: validation.Risk, Status: status, RejectionReason: reason, RequestHash: hash,
			ReservedCash: validation.ReservedCash, ReservedQty: validation.ReservedQty, RemainingQty: req.Quantity, Version: 1,
			StatusHistory: []models.PaperOrderStatusEvent{{Status: status, At: now, Reason: reason}},
		}
		if status != paperStatusAccepted {
			order.ReservedCash = 0
			order.ReservedQty = 0
		}
		return tx.Create(&order).Error
	})
	if err != nil {
		if payloadConflict {
			c.JSON(http.StatusConflict, gin.H{"error": "clientOrderId already exists with a different payload"})
			return
		}
		var existing models.PaperOrder
		if lookupErr := database.DB.Where("client_order_id = ?", req.ClientOrderID).First(&existing).Error; lookupErr == nil && existing.RequestHash == hash {
			status := http.StatusOK
			if existing.Status == paperStatusRejected {
				status = http.StatusUnprocessableEntity
			}
			c.JSON(status, paperOrderResponse{Order: existing, IdempotentReplay: true, Disclaimer: paperDisclaimer(), Error: existing.RejectionReason})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	status := http.StatusCreated
	if order.Status == paperStatusRejected {
		status = http.StatusUnprocessableEntity
	} else if idempotentReplay {
		status = http.StatusOK
	}
	c.JSON(status, paperOrderResponse{Order: order, IdempotentReplay: idempotentReplay, Disclaimer: paperDisclaimer(), Error: order.RejectionReason})
}

func reservePaperOrder(tx *gorm.DB, account *models.PaperAccount, position *models.PaperPosition, side string, cash float64, quantity int) error {
	if side == "BUY" {
		if account.Cash+0.01 < cash {
			return errors.New("insufficient available paper cash")
		}
		result := tx.Model(&models.PaperAccount{}).
			Where("id = ? AND version = ? AND cash >= ?", account.ID, account.Version, cash-0.01).
			Updates(map[string]any{
				"cash": account.Cash - cash, "reserved_cash": account.ReservedCash + cash,
				"version": account.Version + 1,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errPaperLedgerConflict
		}
		return nil
	}
	if position.ID == 0 || position.Quantity-position.ReservedQuantity < quantity {
		return errors.New("insufficient available paper position")
	}
	result := tx.Model(&models.PaperPosition{}).
		Where("id = ? AND version = ? AND quantity - reserved_quantity >= ?", position.ID, position.Version, quantity).
		Updates(map[string]any{"reserved_quantity": position.ReservedQuantity + quantity, "version": position.Version + 1})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errPaperLedgerConflict
	}
	return nil
}

func (h *Handler) ListPaperOrders(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "paper order store unavailable"})
		return
	}
	const terminalLimit = 100
	status := strings.ToUpper(strings.TrimSpace(c.Query("status")))
	filtered := database.DB.Model(&models.PaperOrder{})
	if status != "" {
		filtered = filtered.Where("status = ?", status)
	}
	activeCondition := "(status = ? OR (protection_initialized = ? AND protection_remaining_qty > 0))"

	var total int64
	if err := filtered.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var activeOrders []models.PaperOrder
	activeQuery := database.DB.Where(activeCondition, paperStatusAccepted, true)
	if status != "" {
		activeQuery = activeQuery.Where("status = ?", status)
	}
	if err := activeQuery.Find(&activeOrders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var terminalCount int64
	terminalQuery := database.DB.Model(&models.PaperOrder{}).Where("NOT "+activeCondition, paperStatusAccepted, true)
	if status != "" {
		terminalQuery = terminalQuery.Where("status = ?", status)
	}
	if err := terminalQuery.Count(&terminalCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var terminalOrders []models.PaperOrder
	if err := terminalQuery.Order("created_at desc, id desc").Limit(terminalLimit).Find(&terminalOrders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	orders := append(activeOrders, terminalOrders...)
	sort.SliceStable(orders, func(left, right int) bool {
		if orders[left].CreatedAt.Equal(orders[right].CreatedAt) {
			return orders[left].ID > orders[right].ID
		}
		return orders[left].CreatedAt.After(orders[right].CreatedAt)
	})
	for index := range orders {
		if err := hydratePaperOrderFills(database.DB, &orders[index]); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	hiddenTerminalCount := terminalCount - int64(len(terminalOrders))
	c.JSON(http.StatusOK, gin.H{
		"environment": paperEnvironment, "orders": orders, "disclaimer": paperDisclaimer(),
		"total": total, "hiddenTerminalCount": hiddenTerminalCount, "truncated": hiddenTerminalCount > 0,
	})
}

func (h *Handler) GetPaperAccount(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "paper account store unavailable"})
		return
	}
	var accounts []models.PaperAccount
	var positions []models.PaperPosition
	if err := database.DB.Order("currency asc").Find(&accounts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := database.DB.Where("quantity <> 0 OR reserved_quantity <> 0").Order("currency asc, symbol asc").Find(&positions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"environment": paperEnvironment,
		"accounts":    accounts,
		"positions":   positions,
		"disclaimer":  paperDisclaimer(),
	})
}

func (h *Handler) GetPaperOrder(c *gin.Context) {
	h.respondWithPaperOrder(c, c.Param("clientOrderId"))
}

func (h *Handler) CancelPaperOrder(c *gin.Context) {
	h.transitionPaperOrder(c, c.Param("clientOrderId"), paperStatusCancelled)
}

func (h *Handler) SimulatePaperFill(c *gin.Context) {
	request := paperFillTransitionRequest{}
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Paper fill request"})
			return
		}
	}
	request.FillID = strings.TrimSpace(request.FillID)
	if request.Quantity < 0 || request.Quantity > 0 && request.FillID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "partial fill requires a positive quantity and stable fillId"})
		return
	}
	if request.FillID != "" && !validRequestID.MatchString(request.FillID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fillId must contain only letters, digits, dot, underscore, colon or dash"})
		return
	}
	h.transitionPaperOrderWithFill(c, c.Param("clientOrderId"), paperStatusSimulatedFill, request)
}

func (h *Handler) respondWithPaperOrder(c *gin.Context, clientOrderID string) {
	if database.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "paper order store unavailable"})
		return
	}
	var order models.PaperOrder
	if err := database.DB.Where("client_order_id = ?", clientOrderID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "paper order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := hydratePaperOrderFills(database.DB, &order); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, paperOrderResponse{Order: order, Disclaimer: paperDisclaimer()})
}

func (h *Handler) transitionPaperOrder(c *gin.Context, clientOrderID, target string) {
	h.transitionPaperOrderWithFill(c, clientOrderID, target, paperFillTransitionRequest{})
}

func (h *Handler) transitionPaperOrderWithFill(c *gin.Context, clientOrderID, target string, fillRequest paperFillTransitionRequest) {
	if database.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "paper order store unavailable"})
		return
	}
	now := h.paperNow()
	var quote paperAuthoritativeQuote
	if target == paperStatusSimulatedFill {
		var candidate models.PaperOrder
		if err := database.DB.Where("client_order_id = ?", clientOrderID).First(&candidate).Error; err == nil && candidate.Status == paperStatusAccepted {
			if !paperMarketOpen(candidate.Market, now) {
				c.JSON(http.StatusConflict, gin.H{"error": "paper simulated fill is unavailable outside the covered regular trading session"})
				return
			}
			var reasons []string
			quote, reasons = fetchAuthoritativePaperQuote(candidate.Symbol, now)
			if len(reasons) > 0 {
				c.JSON(http.StatusConflict, gin.H{"error": strings.Join(reasons, "; ")})
				return
			}
		}
	}

	fillID := fillRequest.FillID
	if target == paperStatusSimulatedFill && fillID == "" {
		fillID = "manual-full:" + clientOrderID
	}
	order, idempotentReplay, conflictMessage, err := transitionPaperOrderDBWithPolicyQuantity(database.DB, clientOrderID, target, quote, now, paperRiskPolicyFromConfig(h.config), false, fillID, fillRequest.Quantity)
	if err != nil {
		switch {
		case errors.Is(err, errPaperNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "paper order not found"})
		case errors.Is(err, errPaperStateConflict):
			if conflictMessage == "" {
				conflictMessage = "paper order state changed concurrently"
			}
			c.JSON(http.StatusConflict, gin.H{"error": conflictMessage})
		case errors.Is(err, errPaperFillRiskRejected):
			c.JSON(http.StatusUnprocessableEntity, paperOrderResponse{
				Order: order, Disclaimer: paperDisclaimer(), Error: err.Error(), FillRejected: true,
				RejectionReasons: append([]string(nil), order.RiskSnapshot.PolicyReasons...),
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	checkpointed := false
	warning := ""
	if target == paperStatusSimulatedFill && !idempotentReplay {
		summary, checkpointErr := buildPaperPortfolioRiskAt(database.DB, nil, paperRiskPolicyFromConfig(h.config), now)
		if checkpointErr == nil {
			var written int
			written, checkpointErr = persistPaperEquityCheckpoints(database.DB, summary, now)
			checkpointed = written > 0
		}
		if checkpointErr != nil {
			warning = "paper fill completed but equity checkpoint failed: " + checkpointErr.Error()
		}
	}
	c.JSON(http.StatusOK, paperOrderResponse{Order: order, IdempotentReplay: idempotentReplay, EquityCheckpointed: checkpointed, Disclaimer: paperDisclaimer(), Warning: warning})
}

func transitionPaperOrderDB(db *gorm.DB, clientOrderID, target string, quote paperAuthoritativeQuote, now time.Time) (models.PaperOrder, bool, string, error) {
	return transitionPaperOrderDBWithPolicy(db, clientOrderID, target, quote, now, defaultPaperRiskPolicy(), false)
}

func transitionPaperOrderDBWithAudit(db *gorm.DB, clientOrderID, target string, quote paperAuthoritativeQuote, now time.Time, systemAudit bool) (models.PaperOrder, bool, string, error) {
	return transitionPaperOrderDBWithPolicy(db, clientOrderID, target, quote, now, defaultPaperRiskPolicy(), systemAudit)
}

func transitionPaperOrderDBWithPolicy(db *gorm.DB, clientOrderID, target string, quote paperAuthoritativeQuote, now time.Time, policy paperPortfolioLimits, systemAudit bool) (models.PaperOrder, bool, string, error) {
	fillID := ""
	if target == paperStatusSimulatedFill {
		fillID = "full:" + clientOrderID
	}
	return transitionPaperOrderDBWithPolicyQuantity(db, clientOrderID, target, quote, now, policy, systemAudit, fillID, 0)
}

func transitionPaperOrderDBWithPolicyQuantity(db *gorm.DB, clientOrderID, target string, quote paperAuthoritativeQuote, now time.Time, policy paperPortfolioLimits, systemAudit bool, fillID string, requestedFillQty int) (models.PaperOrder, bool, string, error) {
	portfolioSnapshot := paperPortfolioMarketSnapshot{CapturedAt: now.UTC(), CoveredSymbols: map[string]bool{}}
	if target == paperStatusSimulatedFill {
		var candidate models.PaperOrder
		if err := db.Where("client_order_id = ?", clientOrderID).First(&candidate).Error; err == nil && candidate.Status == paperStatusAccepted && candidate.Side == "BUY" {
			portfolioSnapshot = capturePaperPortfolioMarketSnapshot(db, nil, now)
		}
	}
	var order models.PaperOrder
	conflictMessage := ""
	idempotentReplay := false
	fillRiskRejected := false
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("client_order_id = ?", clientOrderID).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errPaperNotFound
			}
			return err
		}
		if target == paperStatusSimulatedFill && fillID != "" {
			var existingFill models.PaperFill
			if err := tx.Where("fill_id = ?", fillID).First(&existingFill).Error; err == nil {
				if existingFill.PaperOrderID != order.ID {
					conflictMessage = "fillId already belongs to another Paper order"
					return errPaperStateConflict
				}
				if requestedFillQty > 0 && existingFill.Quantity != requestedFillQty {
					conflictMessage = "fillId replay quantity does not match the persisted Paper fill"
					return errPaperStateConflict
				}
				idempotentReplay = true
				return nil
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if order.Status == target {
			idempotentReplay = true
			return nil
		}
		if order.Status != paperStatusAccepted {
			conflictMessage = fmt.Sprintf("cannot transition paper order from %s to %s", order.Status, target)
			return errPaperStateConflict
		}
		beforeAutomaticOrder := order
		fillPrice := 0.0
		remainingBefore := effectivePaperRemainingQty(order)
		fillQuantity := 0
		if target == paperStatusSimulatedFill {
			fillQuantity = requestedFillQty
			if fillQuantity == 0 {
				fillQuantity = remainingBefore
			}
			if remainingBefore <= 0 || fillQuantity <= 0 || fillQuantity > remainingBefore {
				conflictMessage = fmt.Sprintf("fill quantity %d exceeds remaining quantity %d", fillQuantity, remainingBefore)
				return errPaperStateConflict
			}
			if reason := paperFillQuoteReason(quote, now); reason != "" {
				conflictMessage = reason
				return errPaperStateConflict
			}
			var reason string
			fillPrice, reason = simulatedPaperFillPrice(order, quote.Price)
			if reason != "" {
				conflictMessage = reason
				return errPaperStateConflict
			}
			if order.Side == "BUY" {
				riskOrder := order
				riskOrder.Quantity = remainingBefore
				risk, reasons := evaluatePaperFillPolicyTx(tx, riskOrder, fillPrice, policy, now, portfolioSnapshot)
				if len(reasons) > 0 {
					order.RiskSnapshot = risk
					if samePaperFillPolicyState(beforeAutomaticOrder.RiskSnapshot.FillPolicyCheck, risk.FillPolicyCheck) &&
						now.UTC().Sub(beforeAutomaticOrder.RiskSnapshot.FillPolicyCheck.CheckedAt) < paperFillRiskAuditTTL {
						conflictMessage = strings.Join(reasons, "; ")
						order.RiskSnapshot = beforeAutomaticOrder.RiskSnapshot
						fillRiskRejected = true
						return nil
					}
					previousVersion := order.Version
					order.Version++
					result := tx.Model(&models.PaperOrder{}).
						Where("id = ? AND status = ? AND version = ?", order.ID, paperStatusAccepted, previousVersion).
						Select("RiskSnapshot", "Version").Updates(&order)
					if result.Error != nil {
						return result.Error
					}
					if result.RowsAffected != 1 {
						return errPaperStateConflict
					}
					conflictMessage = strings.Join(reasons, "; ")
					fillRiskRejected = true
					if systemAudit {
						if err := appendPaperFillRiskRejectionAuditTx(tx, order, quote, now); err != nil {
							return err
						}
					}
					return nil
				}
				order.RiskSnapshot = risk
			}
		}

		account, position, ledgerErr := loadPaperLedger(tx, order.Currency, order.Symbol)
		if ledgerErr != nil {
			return ledgerErr
		}
		var ocoSiblings []models.PaperOrder
		if order.ParentOrderID != nil && order.OCOGroupID != "" {
			var claimErr error
			ocoSiblings, claimErr = claimPaperOCOReservationTx(tx, &order)
			if claimErr != nil {
				return claimErr
			}
		}
		if target == paperStatusCancelled {
			if err := releasePaperReservation(tx, &account, &position, &order); err != nil {
				return err
			}
		} else {
			fill, err := applyPaperFill(tx, &account, &position, &order, fillPrice, quote, fillQuantity, remainingBefore, fillID, now)
			if err != nil {
				conflictMessage = err.Error()
				return err
			}
			order.Fills = append(order.Fills, fill)
		}

		now = now.UTC()
		completedFill := target == paperStatusSimulatedFill && order.RemainingQty == 0
		if target == paperStatusCancelled || completedFill {
			order.Status = target
		} else {
			order.Status = paperStatusAccepted
		}
		historyReason := ""
		if target == paperStatusSimulatedFill && !completedFill {
			historyReason = fmt.Sprintf("partial Paper fill: %d filled, %d remaining", fillQuantity, order.RemainingQty)
		}
		order.StatusHistory = append(order.StatusHistory, models.PaperOrderStatusEvent{Status: order.Status, At: now, Reason: historyReason})
		order.Version++
		if target == paperStatusSimulatedFill {
			if completedFill {
				order.FilledAt = &now
			}
			if order.Side == "BUY" && order.ProtectiveStop != nil {
				order.OCOGroupID = fmt.Sprintf("paper-oco-%d", order.ID)
				order.ProtectionRemainingQty += fillQuantity
				order.ProtectionInitialized = true
			}
		}
		result := tx.Model(&models.PaperOrder{}).
			Where("id = ? AND status = ? AND version = ?", order.ID, paperStatusAccepted, order.Version-1).
			Select("Status", "StatusHistory", "RiskSnapshot", "ReservedCash", "ReservedQty", "FillPrice", "FillQty", "RemainingQty", "Slippage", "Fee", "RealizedPnL", "FilledAt", "FillModel", "FillQuote", "OCOGroupID", "ProtectionRemainingQty", "ProtectionInitialized", "Version").
			Updates(&order)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errPaperStateConflict
		}
		if target == paperStatusSimulatedFill && order.Side == "SELL" {
			if err := consumePaperProtectionCapacity(tx, order.Currency, order.Symbol, fillQuantity, order.ParentOrderID); err != nil {
				return err
			}
		}
		if target == paperStatusCancelled && order.ParentOrderID != nil {
			if err := consumePaperProtectionCapacity(tx, order.Currency, order.Symbol, order.Quantity, order.ParentOrderID); err != nil {
				return err
			}
			if err := appendPaperAutomaticChildAuditTx(tx, "paper-order.auto-child.cancel", "manual-terminal", beforeAutomaticOrder, order, now); err != nil {
				return err
			}
		}
		if order.ParentOrderID != nil && (target == paperStatusCancelled || completedFill) {
			for index := range ocoSiblings {
				if err := cancelAutomaticPaperChildTx(tx, &ocoSiblings[index], now, "OCO sibling reached terminal state", "oco-terminal"); err != nil {
					return err
				}
			}
		} else if order.ParentOrderID != nil && target == paperStatusSimulatedFill {
			if err := reconcilePaperOCOOrders(tx, order.Currency, order.Symbol, 0, now, "oco-partial-fill"); err != nil {
				return err
			}
		}
		if order.ParentOrderID == nil && (target == paperStatusSimulatedFill || target == paperStatusCancelled) {
			if err := reconcileProtectivePaperOrders(tx, order.Currency, order.Symbol, 0, now); err != nil {
				return err
			}
		}
		if target == paperStatusSimulatedFill {
			fill := order.Fills[len(order.Fills)-1]
			payloadHash := paperSystemAuditHash(fill)
			outcomeHash := paperSystemAuditHash(struct {
				Status      string  `json:"status"`
				FillPrice   float64 `json:"fillPrice"`
				FillQty     int     `json:"fillQty"`
				RealizedPnL float64 `json:"realizedPnL"`
			}{order.Status, order.FillPrice, order.FillQty, order.RealizedPnL})
			requestID := "paper-fill-record:" + fill.FillID
			actor := "system:paper-oms"
			action := "paper-order.manual-simulated-fill-record"
			if systemAudit {
				requestID = "paper-trigger:" + fill.FillID
				actor = "system:paper-oco-scanner"
				action = "paper-order.auto-stop-fill"
				if order.OrderType == "LIMIT" && order.ParentOrderID != nil {
					action = "paper-order.auto-take-profit-fill"
				}
			}
			if err := appendCompletedSystemAuditActorTx(tx, requestID, actor, action, paperFillAuditTarget(fill), payloadHash, outcomeHash, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil && fillRiskRejected {
		err = errPaperFillRiskRejected
	}
	if hydrateErr := hydratePaperOrderFills(db, &order); err == nil && hydrateErr != nil {
		err = hydrateErr
	}
	return order, idempotentReplay, conflictMessage, err
}

func paperFillQuoteReason(quote paperAuthoritativeQuote, checkedAt time.Time) string {
	if quote.Price <= 0 || quote.At.IsZero() || strings.TrimSpace(quote.Source) == "" {
		return "authoritative fill quote is incomplete"
	}
	lowerSource := strings.ToLower(strings.TrimSpace(quote.Source))
	if strings.Contains(lowerSource, "unknown") || strings.Contains(lowerSource, "stale") || sourceIsDisallowedForProduct(quote.Source) {
		return "authoritative fill quote source is unknown, stale, mock or fallback"
	}
	if stale, reason := marketQuoteFreshness(quote.Source, quote.At, checkedAt); stale {
		return "authoritative fill quote is stale: " + reason
	}
	return ""
}

func evaluatePaperFillPolicyTx(tx *gorm.DB, order models.PaperOrder, fillPrice float64, policy paperPortfolioLimits, checkedAt time.Time, snapshot paperPortfolioMarketSnapshot) (models.PaperRiskSnapshot, []string) {
	risk := order.RiskSnapshot
	risk.PlannedNotional = roundPaperMoney(fillPrice * float64(order.Quantity))
	if order.ProtectiveStop != nil {
		risk.MaxLoss = roundPaperMoney(math.Max(fillPrice-*order.ProtectiveStop, 0) * float64(order.Quantity))
	}
	req := paperOrderRequestFromOrder(order, fillPrice)
	validation := paperValidation{Risk: risk}
	summary, err := buildPaperPortfolioRiskFromSnapshotAt(tx, &paperPortfolioProjection{
		Request: req, Risk: risk, ReplaceOrderID: order.ID,
	}, policy, checkedAt, snapshot)
	if err != nil {
		validation.Reasons = append(validation.Reasons, "paper portfolio risk unavailable at fill: "+err.Error())
		recordPaperPolicyCheck(&validation.Risk, policy, validation.Reasons, checkedAt, "fill")
	} else {
		if group := findPaperRiskGroup(summary.Groups, order.Currency); group != nil {
			capital := group.AvailableCash + order.ReservedCash
			risk.InvestableCapital = roundPaperMoney(capital)
			risk.MaxRiskAmount = roundPaperMoney(capital * 0.015)
			risk.MaxNotionalAmount = roundPaperMoney(capital * 0.15)
			validation.Risk = risk
		}
		applyPaperPortfolioLimits(req, &validation, summary, checkedAt, "fill")
	}
	validation.Risk.RiskLimitPassed = len(validation.Reasons) == 0
	return validation.Risk, validation.Reasons
}

func paperOrderRequestFromOrder(order models.PaperOrder, referencePrice float64) paperOrderRequest {
	return paperOrderRequest{
		ClientOrderID: order.ClientOrderID, Environment: order.Environment, Symbol: order.Symbol,
		Side: order.Side, Market: order.Market, Currency: order.Currency, OrderType: order.OrderType,
		TimeInForce: order.TimeInForce, ReferencePrice: referencePrice, Entry: order.Entry,
		TriggerPrice: order.TriggerPrice, ProtectiveStop: order.ProtectiveStop, TakeProfit: order.TakeProfit,
		Quantity: order.Quantity, QuoteSource: order.QuoteSource, QuoteTime: order.QuoteTime.UTC().Format(time.RFC3339),
		RiskSnapshot: order.RiskSnapshot,
	}
}

func appendPaperFillRiskRejectionAuditTx(tx *gorm.DB, order models.PaperOrder, quote paperAuthoritativeQuote, now time.Time) error {
	payloadHash := paperSystemAuditHash(struct {
		OrderID uint                    `json:"orderId"`
		Version uint64                  `json:"version"`
		Quote   paperAuthoritativeQuote `json:"quote"`
	}{order.ID, order.Version - 1, quote})
	outcomeHash := paperSystemAuditHash(order.RiskSnapshot.FillPolicyCheck)
	requestID := fmt.Sprintf("paper-trigger-risk:%d:v%d", order.ID, order.Version)
	return appendCompletedSystemAuditActorTx(tx, requestID, "system:paper-oco-scanner", "paper-order.auto-fill-risk-rejected", order.ClientOrderID, payloadHash, outcomeHash, now)
}

func samePaperFillPolicyState(previous, current *models.PaperPolicyCheck) bool {
	if previous == nil || current == nil || previous.PolicyVersion != current.PolicyVersion || previous.Passed != current.Passed {
		return false
	}
	return strings.Join(previous.Reasons, "\x00") == strings.Join(current.Reasons, "\x00")
}

func paperSystemAuditHash(payload any) string {
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func paperFillAuditTarget(fill models.PaperFill) string {
	return "paper-fill:" + fill.FillID
}

func effectivePaperRemainingQty(order models.PaperOrder) int {
	if order.RemainingQty > 0 {
		return order.RemainingQty
	}
	remaining := order.Quantity - order.FillQty
	if remaining < 0 {
		return 0
	}
	return remaining
}

func hydratePaperOrderFills(db *gorm.DB, order *models.PaperOrder) error {
	if db == nil || order == nil || order.ID == 0 {
		return nil
	}
	if err := db.Where("paper_order_id = ?", order.ID).Order("sequence asc, id asc").Find(&order.Fills).Error; err != nil {
		return err
	}
	if order.Status == paperStatusAccepted {
		order.RemainingQty = effectivePaperRemainingQty(*order)
	}
	return nil
}

func paperFillQuoteAuditTarget(order models.PaperOrder) string {
	return "paper-order:" + order.ClientOrderID + "#fillQuote"
}

func paperFillQuoteAuditPayload(order models.PaperOrder) any {
	return paperFillQuoteAuditPayloadAtVersion(order, order.Version)
}

func paperFillQuoteAuditPayloadAtVersion(order models.PaperOrder, version uint64) any {
	return struct {
		OrderID       uint                   `json:"orderId"`
		ClientOrderID string                 `json:"clientOrderId"`
		Version       uint64                 `json:"version"`
		FillQuote     *models.PaperFillQuote `json:"fillQuote"`
	}{order.ID, order.ClientOrderID, version, order.FillQuote}
}

// claimPaperOCOReservationTx moves the OCO group's single inventory reservation
// onto the leg that is about to become terminal. Sibling cancellation and the
// ledger transition then consume or release that reservation exactly once.
func claimPaperOCOReservationTx(tx *gorm.DB, order *models.PaperOrder) ([]models.PaperOrder, error) {
	var siblings []models.PaperOrder
	if err := tx.Where("oco_group_id = ? AND id <> ? AND status = ?", order.OCOGroupID, order.ID, paperStatusAccepted).
		Order("id asc").Find(&siblings).Error; err != nil {
		return nil, err
	}
	reserved := order.ReservedQty
	for index := range siblings {
		reserved += siblings[index].ReservedQty
	}
	if reserved != effectivePaperRemainingQty(*order) {
		return nil, errors.New("paper OCO reservation invariant violated")
	}
	for index := range siblings {
		if siblings[index].ReservedQty == 0 {
			continue
		}
		result := tx.Model(&models.PaperOrder{}).Where("id = ? AND status = ? AND version = ?", siblings[index].ID, paperStatusAccepted, siblings[index].Version).
			Updates(map[string]any{"reserved_qty": 0, "version": siblings[index].Version + 1})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return nil, result.Error
			}
			return nil, errPaperStateConflict
		}
		siblings[index].ReservedQty = 0
		siblings[index].Version++
	}
	order.ReservedQty = reserved
	return siblings, nil
}

func acceptedProtectiveReservation(tx *gorm.DB, currency, symbol string) (int, error) {
	var total int64
	err := tx.Model(&models.PaperOrder{}).
		Where("currency = ? AND symbol = ? AND status = ? AND side = ? AND parent_order_id IS NOT NULL", currency, symbol, paperStatusAccepted, "SELL").
		Select("COALESCE(SUM(reserved_qty), 0)").Scan(&total).Error
	return int(total), err
}

func consumePaperProtectionCapacity(tx *gorm.DB, currency, symbol string, quantity int, preferredParentID *uint) error {
	var parents []models.PaperOrder
	if err := tx.Where("currency = ? AND symbol = ? AND side = ? AND fill_qty > 0 AND protection_initialized = ? AND protection_remaining_qty > 0", currency, symbol, "BUY", true).
		Order("filled_at asc, id asc").Find(&parents).Error; err != nil {
		return err
	}
	if preferredParentID != nil {
		sort.SliceStable(parents, func(left, right int) bool {
			return parents[left].ID == *preferredParentID && parents[right].ID != *preferredParentID
		})
	}
	remaining := quantity
	for index := range parents {
		if remaining == 0 {
			break
		}
		parent := &parents[index]
		consumed := parent.ProtectionRemainingQty
		if consumed > remaining {
			consumed = remaining
		}
		result := tx.Model(&models.PaperOrder{}).
			Where("id = ? AND version = ? AND protection_remaining_qty >= ?", parent.ID, parent.Version, consumed).
			Updates(map[string]any{"protection_remaining_qty": parent.ProtectionRemainingQty - consumed, "version": parent.Version + 1})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return result.Error
			}
			return errPaperStateConflict
		}
		remaining -= consumed
	}
	return nil
}

func initializeLegacyProtectionCapacities(tx *gorm.DB, currency, symbol string, positionQuantity int) error {
	var initializedTotal int64
	if err := tx.Model(&models.PaperOrder{}).
		Where("currency = ? AND symbol = ? AND side = ? AND fill_qty > 0 AND protection_initialized = ?", currency, symbol, "BUY", true).
		Select("COALESCE(SUM(protection_remaining_qty), 0)").Scan(&initializedTotal).Error; err != nil {
		return err
	}
	remaining := positionQuantity - int(initializedTotal)
	if remaining < 0 {
		remaining = 0
	}
	var legacy []models.PaperOrder
	if err := tx.Where("currency = ? AND symbol = ? AND side = ? AND status = ? AND protective_stop IS NOT NULL AND protection_initialized = ?", currency, symbol, "BUY", paperStatusSimulatedFill, false).
		Order("filled_at desc, id desc").Find(&legacy).Error; err != nil {
		return err
	}
	for index := range legacy {
		parent := &legacy[index]
		capacity := parent.FillQty
		if capacity > remaining {
			capacity = remaining
		}
		if capacity < 0 {
			capacity = 0
		}
		result := tx.Model(&models.PaperOrder{}).Where("id = ? AND version = ? AND protection_initialized = ?", parent.ID, parent.Version, false).
			Updates(map[string]any{"protection_remaining_qty": capacity, "protection_initialized": true, "version": parent.Version + 1})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return result.Error
			}
			return errPaperStateConflict
		}
		remaining -= capacity
	}
	return nil
}

// reconcileProtectivePaperOrders makes automatic child reservations exactly fit
// inventory not already reserved by explicit SELL orders. It runs inside the
// caller's immediate SQLite transaction, so child rows and ledger reservation
// move together across processes.
func reconcileProtectivePaperOrders(tx *gorm.DB, currency, symbol string, additionalManualReservation int, now time.Time) error {
	return reconcilePaperOCOOrders(tx, currency, symbol, additionalManualReservation, now, "ledger-reconcile")
}

func reconcilePaperOCOOrders(tx *gorm.DB, currency, symbol string, additionalManualReservation int, now time.Time, origin string) error {
	var position models.PaperPosition
	if err := tx.Where("currency = ? AND symbol = ?", currency, symbol).First(&position).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if err := initializeLegacyProtectionCapacities(tx, currency, symbol, position.Quantity); err != nil {
		return err
	}
	var children []models.PaperOrder
	if err := tx.Where("currency = ? AND symbol = ? AND side = ? AND parent_order_id IS NOT NULL AND status = ?", currency, symbol, "SELL", paperStatusAccepted).
		Order("parent_order_id asc, id asc").Find(&children).Error; err != nil {
		return err
	}
	currentChildReserved := 0
	for _, child := range children {
		currentChildReserved += child.ReservedQty
	}
	manualReserved := position.ReservedQuantity - currentChildReserved + additionalManualReservation
	if manualReserved < 0 {
		return errors.New("paper protective reservation invariant violated")
	}
	target := position.Quantity - manualReserved
	if target < 0 {
		return errors.New("insufficient paper position after explicit SELL reservations")
	}

	var parents []models.PaperOrder
	if err := tx.Where("currency = ? AND symbol = ? AND side = ? AND fill_qty > 0 AND protective_stop IS NOT NULL AND protection_initialized = ? AND protection_remaining_qty > 0", currency, symbol, "BUY", true).
		Order("filled_at asc, id asc").Find(&parents).Error; err != nil {
		return err
	}
	byParentAndKind := make(map[string]*models.PaperOrder, len(children))
	var duplicateChildren []*models.PaperOrder
	for index := range children {
		if children[index].ParentOrderID == nil {
			continue
		}
		key := fmt.Sprintf("%d:%s", *children[index].ParentOrderID, automaticPaperChildKind(children[index]))
		if _, exists := byParentAndKind[key]; exists {
			duplicateChildren = append(duplicateChildren, &children[index])
			continue
		}
		byParentAndKind[key] = &children[index]
	}
	desiredTotal := 0
	for index := range parents {
		parent := &parents[index]
		desired := parent.ProtectionRemainingQty
		if desired > target-desiredTotal {
			desired = target - desiredTotal
		}
		if desired < 0 {
			desired = 0
		}
		desiredTotal += desired
		for _, kind := range []string{"stop", "take-profit"} {
			key := fmt.Sprintf("%d:%s", parent.ID, kind)
			child := byParentAndKind[key]
			delete(byParentAndKind, key)
			legAvailable := (kind == "stop" && parent.ProtectiveStop != nil) || (kind == "take-profit" && parent.TakeProfit != nil)
			if desired == 0 || !legAvailable {
				if child != nil {
					if err := cancelAutomaticPaperChildTx(tx, child, now, "OCO capacity removed after position/reservation change", origin); err != nil {
						return err
					}
				}
				continue
			}
			if child == nil {
				if err := createAutomaticPaperChildTx(tx, parent, kind, desired, now, origin); err != nil {
					return err
				}
				continue
			}
			reservedQty := 0
			if kind == "stop" || parent.ProtectiveStop == nil {
				reservedQty = desired
			}
			desiredQuantity := child.FillQty + desired
			if child.Quantity != desiredQuantity || child.RemainingQty != desired || child.ReservedQty != reservedQty {
				before := *child
				child.Quantity = desiredQuantity
				child.RemainingQty = desired
				child.ReservedQty = reservedQty
				child.Version++
				child.StatusHistory = append(child.StatusHistory, models.PaperOrderStatusEvent{Status: paperStatusAccepted, At: now, Reason: "OCO capacity resized to current unreserved position"})
				result := tx.Model(&models.PaperOrder{}).Where("id = ? AND status = ? AND version = ?", child.ID, paperStatusAccepted, child.Version-1).
					Select("Quantity", "RemainingQty", "ReservedQty", "StatusHistory", "Version").Updates(child)
				if result.Error != nil || result.RowsAffected != 1 {
					if result.Error != nil {
						return result.Error
					}
					return errPaperStateConflict
				}
				if err := appendPaperAutomaticChildAuditTx(tx, "paper-order.auto-child.resize", origin, before, *child, now); err != nil {
					return err
				}
			}
		}
	}
	for _, child := range byParentAndKind {
		if err := cancelAutomaticPaperChildTx(tx, child, now, "OCO child exceeds current position or parent capability", origin); err != nil {
			return err
		}
	}
	for _, child := range duplicateChildren {
		if err := cancelAutomaticPaperChildTx(tx, child, now, "duplicate OCO child removed", origin); err != nil {
			return err
		}
	}
	delta := desiredTotal - currentChildReserved
	if delta != 0 {
		newReserved := position.ReservedQuantity + delta
		if newReserved < 0 || newReserved > position.Quantity {
			return errors.New("paper protective reservation would exceed position")
		}
		result := tx.Model(&models.PaperPosition{}).Where("id = ? AND version = ?", position.ID, position.Version).
			Updates(map[string]any{"reserved_quantity": newReserved, "version": position.Version + 1})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return result.Error
			}
			return errPaperLedgerConflict
		}
	}
	return nil
}

func automaticPaperChildKind(order models.PaperOrder) string {
	if order.OrderType == "LIMIT" && order.ParentOrderID != nil {
		return "take-profit"
	}
	return "stop"
}

func createAutomaticPaperChildTx(tx *gorm.DB, parent *models.PaperOrder, kind string, quantity int, now time.Time, origin string) error {
	if quantity <= 0 || (kind == "stop" && parent.ProtectiveStop == nil) || (kind == "take-profit" && parent.TakeProfit == nil) {
		return nil
	}
	var revision int64
	orderType := "STOP_MARKET"
	clientPrefix := "auto-stop"
	if kind == "take-profit" {
		orderType = "LIMIT"
		clientPrefix = "auto-take-profit"
	}
	if err := tx.Model(&models.PaperOrder{}).Where("parent_order_id = ? AND order_type = ?", parent.ID, orderType).Count(&revision).Error; err != nil {
		return err
	}
	clientID := fmt.Sprintf("%s-%d-%d", clientPrefix, parent.ID, revision+1)
	oco := parent.OCOGroupID
	if oco == "" {
		oco = fmt.Sprintf("paper-oco-%d", parent.ID)
	}
	price := parent.ProtectiveStop
	reservedQty := quantity
	maxLoss := 0.0
	if kind == "stop" {
		maxLoss = roundPaperMoney(math.Max(parent.FillPrice-*parent.ProtectiveStop, 0) * float64(quantity))
	} else {
		price = parent.TakeProfit
		reservedQty = 0
	}
	child := models.PaperOrder{
		ClientOrderID: clientID, Environment: paperEnvironment, Symbol: parent.Symbol, Side: "SELL",
		ResearchRunID: parent.ResearchRunID, ResearchTicker: parent.ResearchTicker,
		Market: parent.Market, Currency: parent.Currency, OrderType: orderType, TimeInForce: "GTC",
		ReferencePrice: parent.FillPrice, ParentOrderID: &parent.ID, OCOGroupID: oco,
		Quantity: quantity, RemainingQty: quantity, QuoteSource: parent.QuoteSource, QuoteTime: parent.QuoteTime,
		RiskSnapshot: models.PaperRiskSnapshot{InvestableCapital: parent.RiskSnapshot.InvestableCapital, MaxLoss: maxLoss, PlannedNotional: roundPaperMoney(*price * float64(quantity)), RiskLimitPassed: true, LiquidityComplete: true},
		Status:       paperStatusAccepted, RequestHash: paperSystemAuditHash(struct {
			Parent   uint   `json:"parent"`
			Kind     string `json:"kind"`
			Revision int64  `json:"revision"`
		}{parent.ID, kind, revision + 1}),
		ReservedQty: reservedQty, Version: 1,
		StatusHistory: []models.PaperOrderStatusEvent{{Status: paperStatusAccepted, At: now.UTC(), Reason: "automatic " + kind + " OCO child created from simulated BUY fill"}},
	}
	if kind == "stop" {
		child.TriggerPrice = price
	} else {
		child.Entry = price
	}
	if err := tx.Create(&child).Error; err != nil {
		return err
	}
	return appendPaperAutomaticChildAuditTx(tx, "paper-order.auto-child.create", origin, models.PaperOrder{}, child, now)
}

func cancelAutomaticPaperChildTx(tx *gorm.DB, child *models.PaperOrder, now time.Time, reason, origin string) error {
	before := *child
	child.Status = paperStatusCancelled
	child.ReservedQty = 0
	child.Version++
	child.StatusHistory = append(child.StatusHistory, models.PaperOrderStatusEvent{Status: paperStatusCancelled, At: now.UTC(), Reason: reason})
	result := tx.Model(&models.PaperOrder{}).Where("id = ? AND status = ? AND version = ?", child.ID, paperStatusAccepted, child.Version-1).
		Select("Status", "ReservedQty", "StatusHistory", "Version").Updates(child)
	if result.Error != nil || result.RowsAffected != 1 {
		if result.Error != nil {
			return result.Error
		}
		return errPaperStateConflict
	}
	return appendPaperAutomaticChildAuditTx(tx, "paper-order.auto-child.cancel", origin, before, *child, now)
}

func appendPaperAutomaticChildAuditTx(tx *gorm.DB, action, origin string, before, after models.PaperOrder, now time.Time) error {
	target := after.ClientOrderID
	if target == "" {
		target = before.ClientOrderID
	}
	var sequence int64
	if err := tx.Model(&models.AuditEvent{}).Where("action = ? AND target = ?", action, target).Count(&sequence).Error; err != nil {
		return err
	}
	payloadHash := paperSystemAuditHash(struct {
		Origin      string `json:"origin"`
		OrderID     uint   `json:"orderId"`
		Version     uint64 `json:"version"`
		Quantity    int    `json:"quantity"`
		ReservedQty int    `json:"reservedQty"`
		Status      string `json:"status"`
	}{origin, before.ID, before.Version, before.Quantity, before.ReservedQty, before.Status})
	outcomeHash := paperSystemAuditHash(struct {
		OrderID     uint   `json:"orderId"`
		Version     uint64 `json:"version"`
		Quantity    int    `json:"quantity"`
		ReservedQty int    `json:"reservedQty"`
		Status      string `json:"status"`
		OCOGroupID  string `json:"ocoGroupId"`
	}{after.ID, after.Version, after.Quantity, after.ReservedQty, after.Status, after.OCOGroupID})
	requestID := fmt.Sprintf("paper-oco:%d:v%d:%d", after.ID, after.Version, sequence+1)
	return appendCompletedSystemAuditActorTx(tx, requestID, "system:paper-oco", action, target, payloadHash, outcomeHash, now)
}

func appendPaperOCORecoveryAuditTx(tx *gorm.DB, currency, symbol string, now time.Time) error {
	var children []models.PaperOrder
	if err := tx.Where("currency = ? AND symbol = ? AND side = ? AND parent_order_id IS NOT NULL AND status = ?", currency, symbol, "SELL", paperStatusAccepted).
		Order("parent_order_id asc, order_type asc, id asc").Find(&children).Error; err != nil {
		return err
	}
	type childSnapshot struct {
		ID          uint   `json:"id"`
		ParentID    uint   `json:"parentId"`
		OrderType   string `json:"orderType"`
		OCOGroupID  string `json:"ocoGroupId"`
		Quantity    int    `json:"quantity"`
		ReservedQty int    `json:"reservedQty"`
		Version     uint64 `json:"version"`
	}
	snapshots := make([]childSnapshot, 0, len(children))
	for _, child := range children {
		if child.ParentOrderID == nil {
			continue
		}
		snapshots = append(snapshots, childSnapshot{
			ID: child.ID, ParentID: *child.ParentOrderID, OrderType: child.OrderType,
			OCOGroupID: child.OCOGroupID, Quantity: child.Quantity,
			ReservedQty: child.ReservedQty, Version: child.Version,
		})
	}
	target := currency + ":" + symbol
	var sequence int64
	if err := tx.Model(&models.AuditEvent{}).Where("action = ? AND target = ?", "paper-order.oco-recovery", target).Count(&sequence).Error; err != nil {
		return err
	}
	payloadHash := paperSystemAuditHash(struct {
		Currency string `json:"currency"`
		Symbol   string `json:"symbol"`
		Origin   string `json:"origin"`
	}{currency, symbol, "restart-recovery"})
	outcomeHash := paperSystemAuditHash(struct {
		Children []childSnapshot `json:"children"`
	}{snapshots})
	requestID := fmt.Sprintf("paper-oco-recovery:%s:%s:%d", currency, symbol, sequence+1)
	return appendCompletedSystemAuditActorTx(tx, requestID, "system:paper-oco", "paper-order.oco-recovery", target, payloadHash, outcomeHash, now)
}

func releasePaperReservation(tx *gorm.DB, account *models.PaperAccount, position *models.PaperPosition, order *models.PaperOrder) error {
	if order.ReservedCash > 0 {
		if account.ReservedCash+0.01 < order.ReservedCash {
			return errors.New("paper cash reservation invariant violated")
		}
		result := tx.Model(&models.PaperAccount{}).Where("id = ? AND version = ?", account.ID, account.Version).
			Updates(map[string]any{
				"cash": account.Cash + order.ReservedCash, "reserved_cash": account.ReservedCash - order.ReservedCash,
				"version": account.Version + 1,
			})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return result.Error
			}
			return errPaperLedgerConflict
		}
		order.ReservedCash = 0
	}
	if order.ReservedQty > 0 {
		if position.ID == 0 || position.ReservedQuantity < order.ReservedQty {
			return errors.New("paper position reservation invariant violated")
		}
		result := tx.Model(&models.PaperPosition{}).Where("id = ? AND version = ?", position.ID, position.Version).
			Updates(map[string]any{"reserved_quantity": position.ReservedQuantity - order.ReservedQty, "version": position.Version + 1})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return result.Error
			}
			return errPaperLedgerConflict
		}
		order.ReservedQty = 0
	}
	return nil
}

func simulatedPaperFillPrice(order models.PaperOrder, marketPrice float64) (float64, string) {
	if marketPrice <= 0 {
		return 0, "authoritative fill quote unavailable"
	}
	price := marketPrice
	if order.Side == "BUY" {
		price *= 1 + paperSlippageRate
	} else {
		price *= 1 - paperSlippageRate
	}
	if order.OrderType == "LIMIT" && order.Entry != nil {
		if order.Side == "BUY" {
			if marketPrice > *order.Entry {
				return 0, "buy limit is not marketable at the authoritative quote"
			}
			price = math.Min(price, *order.Entry)
		} else {
			if marketPrice < *order.Entry {
				return 0, "sell limit is not marketable at the authoritative quote"
			}
			price = math.Max(price, *order.Entry)
		}
	} else if order.OrderType == "STOP_MARKET" {
		if order.TriggerPrice == nil || *order.TriggerPrice <= 0 {
			return 0, "stop-market trigger price is unavailable"
		}
		if order.Side == "BUY" && marketPrice < *order.TriggerPrice {
			return 0, "buy stop has not triggered"
		}
		if order.Side == "SELL" && marketPrice > *order.TriggerPrice {
			return 0, "sell stop has not triggered"
		}
	}
	return roundPaperPrice(price), ""
}

func applyPaperFill(tx *gorm.DB, account *models.PaperAccount, position *models.PaperPosition, order *models.PaperOrder, fillPrice float64, quote paperAuthoritativeQuote, fillQuantity, remainingBefore int, fillID string, now time.Time) (models.PaperFill, error) {
	fill := models.PaperFill{}
	if fillQuantity <= 0 || remainingBefore <= 0 || fillQuantity > remainingBefore {
		return fill, errors.New("invalid paper fill quantity")
	}
	notional := roundPaperMoney(fillPrice * float64(fillQuantity))
	fee := paperFee(notional)
	slippage := roundPaperMoney(math.Abs(fillPrice-quote.Price) * float64(fillQuantity))
	fillRealizedPnL := 0.0
	if order.Side == "BUY" {
		reserved := order.ReservedCash
		if fillQuantity < remainingBefore {
			reserved = roundPaperMoney(order.ReservedCash * float64(fillQuantity) / float64(remainingBefore))
		}
		if account.ReservedCash+0.01 < reserved {
			return fill, errors.New("paper cash reservation invariant violated")
		}
		actualCost := notional + fee
		additional := math.Max(0, actualCost-reserved)
		refund := math.Max(0, reserved-actualCost)
		if account.Cash+0.01 < additional {
			return fill, errors.New("insufficient available paper cash at fill")
		}
		accountResult := tx.Model(&models.PaperAccount{}).Where("id = ? AND version = ?", account.ID, account.Version).
			Updates(map[string]any{
				"cash": account.Cash - additional + refund, "reserved_cash": account.ReservedCash - reserved,
				"version": account.Version + 1,
			})
		if accountResult.Error != nil || accountResult.RowsAffected != 1 {
			if accountResult.Error != nil {
				return fill, accountResult.Error
			}
			return fill, errPaperLedgerConflict
		}
		if position.ID == 0 {
			position = &models.PaperPosition{
				Currency: order.Currency, Symbol: order.Symbol, Quantity: fillQuantity,
				AverageCost: fillPrice, Version: 1,
			}
			if err := tx.Create(position).Error; err != nil {
				return fill, err
			}
		} else {
			newQuantity := position.Quantity + fillQuantity
			averageCost := roundPaperPrice((position.AverageCost*float64(position.Quantity) + fillPrice*float64(fillQuantity)) / float64(newQuantity))
			positionResult := tx.Model(&models.PaperPosition{}).Where("id = ? AND version = ?", position.ID, position.Version).
				Updates(map[string]any{"quantity": newQuantity, "average_cost": averageCost, "version": position.Version + 1})
			if positionResult.Error != nil || positionResult.RowsAffected != 1 {
				if positionResult.Error != nil {
					return fill, positionResult.Error
				}
				return fill, errPaperLedgerConflict
			}
		}
		order.ReservedCash = roundPaperMoney(math.Max(0, order.ReservedCash-reserved))
	} else {
		reserved := fillQuantity
		if order.ReservedQty < reserved {
			reserved = order.ReservedQty
		}
		availableForLegacy := position.Quantity - position.ReservedQuantity
		if position.ID == 0 || position.Quantity < fillQuantity || (reserved == 0 && availableForLegacy < fillQuantity) || position.ReservedQuantity < reserved {
			return fill, errors.New("insufficient available paper position at fill")
		}
		newQuantity := position.Quantity - fillQuantity
		newReserved := position.ReservedQuantity - reserved
		averageCost := position.AverageCost
		if newQuantity == 0 {
			averageCost = 0
		}
		positionResult := tx.Model(&models.PaperPosition{}).Where("id = ? AND version = ?", position.ID, position.Version).
			Updates(map[string]any{
				"quantity": newQuantity, "reserved_quantity": newReserved, "average_cost": averageCost,
				"version": position.Version + 1,
			})
		if positionResult.Error != nil || positionResult.RowsAffected != 1 {
			if positionResult.Error != nil {
				return fill, positionResult.Error
			}
			return fill, errPaperLedgerConflict
		}
		proceeds := notional - fee
		fillRealizedPnL = roundPaperMoney((fillPrice-position.AverageCost)*float64(fillQuantity) - fee)
		accountResult := tx.Model(&models.PaperAccount{}).Where("id = ? AND version = ?", account.ID, account.Version).
			Updates(map[string]any{"cash": account.Cash + proceeds, "version": account.Version + 1})
		if accountResult.Error != nil || accountResult.RowsAffected != 1 {
			if accountResult.Error != nil {
				return fill, accountResult.Error
			}
			return fill, errPaperLedgerConflict
		}
		order.ReservedQty -= reserved
	}
	quoteRecord := models.PaperFillQuote{
		Price: roundPaperPrice(quote.Price), Source: strings.TrimSpace(quote.Source),
		ObservedAt: quote.At.UTC(), ProviderURL: strings.TrimSpace(quote.ProviderURL),
	}
	previousQty := order.FillQty
	newFillQty := previousQty + fillQuantity
	order.FillPrice = roundPaperPrice((order.FillPrice*float64(previousQty) + fillPrice*float64(fillQuantity)) / float64(newFillQty))
	order.FillQty = newFillQty
	order.RemainingQty = remainingBefore - fillQuantity
	order.Slippage = roundPaperMoney(order.Slippage + slippage)
	order.Fee = roundPaperMoney(order.Fee + fee)
	order.RealizedPnL = roundPaperMoney(order.RealizedPnL + fillRealizedPnL)
	order.FillModel = "multi-fill-v2: append-only fills, authoritative quotes, 5bps adverse slippage, fee=max(1,5bps notional), limit-price protection"
	order.FillQuote = &quoteRecord
	var count int64
	if err := tx.Model(&models.PaperFill{}).Where("paper_order_id = ?", order.ID).Count(&count).Error; err != nil {
		return fill, err
	}
	policyVersion := order.RiskSnapshot.PolicyVersion
	policyReasons := append([]string(nil), order.RiskSnapshot.PolicyReasons...)
	if order.RiskSnapshot.FillPolicyCheck != nil {
		policyVersion = order.RiskSnapshot.FillPolicyCheck.PolicyVersion
		policyReasons = append([]string(nil), order.RiskSnapshot.FillPolicyCheck.Reasons...)
	}
	fill = models.PaperFill{
		CreatedAt: now.UTC(), FillID: fillID, PaperOrderID: order.ID, Sequence: int(count) + 1,
		Quantity: fillQuantity, Price: fillPrice, Quote: quoteRecord, Slippage: slippage, Fee: fee,
		RealizedPnL: fillRealizedPnL, PolicyVersion: policyVersion, PolicyReasons: policyReasons,
	}
	if err := tx.Create(&fill).Error; err != nil {
		return models.PaperFill{}, err
	}
	return fill, nil
}

func paperFee(notional float64) float64 {
	return roundPaperMoney(math.Max(paperMinimumFee, notional*paperFeeRate))
}

func roundPaperMoney(value float64) float64 {
	return math.Round(value*100) / 100
}

func roundPaperPrice(value float64) float64 {
	return math.Round(value*10_000) / 10_000
}
