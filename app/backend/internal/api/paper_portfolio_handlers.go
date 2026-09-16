package api

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"trading-agents/internal/config"
	"trading-agents/internal/database"
	"trading-agents/internal/dataflows"
	"trading-agents/internal/models"
)

const (
	paperMaxPortfolioExposurePct   = 80.0
	paperMaxSymbolExposurePct      = 25.0
	paperMaxSectorExposurePct      = 40.0
	paperMaxOpenOrderLossPct       = 6.0
	paperMaxOrderADVPercent        = 1.0
	paperUnknownSector             = "unknown"
	paperDefaultRiskPolicyVersion  = "paper-risk-v2"
	paperDefaultMinStopCoveragePct = 100.0
	paperDefaultMaxDailyLossPct    = 3.0
	paperDefaultMaxDrawdownPct     = 12.0
	paperDefaultMaxStressLossPct   = 10.0
)

var paperUniverseLoader = loadUniverseStocks
var paperADVLoader = dataflows.LoadProductionADV

type paperPortfolioLimits struct {
	PolicyVersion           string  `json:"policyVersion"`
	MaxPortfolioExposurePct float64 `json:"maxPortfolioExposurePct"`
	MaxSymbolExposurePct    float64 `json:"maxSymbolExposurePct"`
	MaxSectorExposurePct    float64 `json:"maxSectorExposurePct"`
	MaxOpenOrderLossPct     float64 `json:"maxOpenOrderLossPct"`
	MaxOrderADVPercent      float64 `json:"maxOrderADVPercent"`
	MinStopCoveragePct      float64 `json:"minStopCoveragePct"`
	MaxDailyLossPct         float64 `json:"maxDailyLossPct"`
	MaxDrawdownPct          float64 `json:"maxDrawdownPct"`
	MaxStressLossPct        float64 `json:"maxStressLossPct"`
}

type paperPortfolioSymbolRisk struct {
	Symbol               string     `json:"symbol"`
	Sector               string     `json:"sector"`
	PositionQuantity     int        `json:"positionQuantity"`
	ReservedQuantity     int        `json:"reservedQuantity"`
	StopCoveredQuantity  int        `json:"stopCoveredQuantity"`
	StopCoveragePct      float64    `json:"stopCoveragePct"`
	MarkPrice            *float64   `json:"markPrice,omitempty"`
	MarkSource           string     `json:"markSource,omitempty"`
	MarkTime             *time.Time `json:"markTime,omitempty"`
	ValuationError       string     `json:"valuationError,omitempty"`
	PositionMarketValue  *float64   `json:"positionMarketValue,omitempty"`
	AcceptedBuyNotional  float64    `json:"acceptedBuyNotional"`
	AcceptedSellNotional float64    `json:"acceptedSellNotional"`
	GrossExposure        *float64   `json:"grossExposure,omitempty"`
	ExposurePct          *float64   `json:"exposurePct,omitempty"`
	AverageCost          float64    `json:"averageCost"`
	UnrealizedPnL        *float64   `json:"unrealizedPnL,omitempty"`
	AverageDailyVolume   *float64   `json:"averageDailyVolume,omitempty"`
	PlannedADVPercent    *float64   `json:"plannedADVPercent,omitempty"`
	LiquiditySource      string     `json:"liquiditySource,omitempty"`
	LiquidityError       string     `json:"liquidityError,omitempty"`
	LiquidityComplete    bool       `json:"liquidityComplete"`
}

type paperStressScenario struct {
	Name              string   `json:"name"`
	Kind              string   `json:"kind"`
	Status            string   `json:"status"`
	Assumption        string   `json:"assumption,omitempty"`
	UnknownReason     string   `json:"unknownReason,omitempty"`
	PriceShockPct     float64  `json:"priceShockPct,omitempty"`
	EquityAfter       *float64 `json:"equityAfter,omitempty"`
	PnL               *float64 `json:"pnl,omitempty"`
	LiquidityUsagePct *float64 `json:"liquidityUsagePct,omitempty"`
}

type paperPortfolioSectorRisk struct {
	Sector        string   `json:"sector"`
	GrossExposure *float64 `json:"grossExposure,omitempty"`
	ExposurePct   *float64 `json:"exposurePct,omitempty"`
}

type paperPortfolioCurrencyRisk struct {
	Currency                string                     `json:"currency"`
	InitialCash             float64                    `json:"initialCash"`
	AvailableCash           float64                    `json:"availableCash"`
	ReservedCash            float64                    `json:"reservedCash"`
	PositionMarketValue     *float64                   `json:"positionMarketValue,omitempty"`
	OpenBuyNotional         float64                    `json:"openBuyNotional"`
	OpenSellNotional        float64                    `json:"openSellNotional"`
	GrossExposure           *float64                   `json:"grossExposure,omitempty"`
	TotalEquity             *float64                   `json:"totalEquity,omitempty"`
	GrossExposurePct        *float64                   `json:"grossExposurePct,omitempty"`
	LargestSymbol           string                     `json:"largestSymbol,omitempty"`
	LargestSymbolExposure   *float64                   `json:"largestSymbolExposure,omitempty"`
	LargestSymbolPct        *float64                   `json:"largestSymbolPct,omitempty"`
	LargestSector           string                     `json:"largestSector,omitempty"`
	LargestSectorExposure   *float64                   `json:"largestSectorExposure,omitempty"`
	LargestSectorPct        *float64                   `json:"largestSectorPct,omitempty"`
	PositionQuantity        int                        `json:"positionQuantity"`
	StopCoveredQuantity     int                        `json:"stopCoveredQuantity"`
	StopCoveragePct         float64                    `json:"stopCoveragePct"`
	OpenOrderMaxPlannedLoss float64                    `json:"openOrderMaxPlannedLoss"`
	OpenOrderMaxLossPct     *float64                   `json:"openOrderMaxLossPct,omitempty"`
	ValuationComplete       bool                       `json:"valuationComplete"`
	SectorComplete          bool                       `json:"sectorComplete"`
	LiquidityComplete       bool                       `json:"liquidityComplete"`
	RealizedPnLToday        float64                    `json:"realizedPnLToday"`
	UnrealizedPnL           *float64                   `json:"unrealizedPnL,omitempty"`
	DayStartEquity          *float64                   `json:"dayStartEquity,omitempty"`
	DayStartObservedAt      *time.Time                 `json:"dayStartObservedAt,omitempty"`
	DailyPnL                *float64                   `json:"dailyPnL,omitempty"`
	DailyLossPct            *float64                   `json:"dailyLossPct,omitempty"`
	DailyRiskComplete       bool                       `json:"dailyRiskComplete"`
	PeakEquity              *float64                   `json:"peakEquity,omitempty"`
	PeakEquityAt            *time.Time                 `json:"peakEquityAt,omitempty"`
	PeakDrawdown            *float64                   `json:"peakDrawdown,omitempty"`
	PeakDrawdownPct         *float64                   `json:"peakDrawdownPct,omitempty"`
	StressScenarios         []paperStressScenario      `json:"stressScenarios"`
	Degraded                bool                       `json:"degraded"`
	DegradationReasons      []string                   `json:"degradationReasons,omitempty"`
	UnknownSymbols          []string                   `json:"unknownSymbols"`
	Symbols                 []paperPortfolioSymbolRisk `json:"symbols"`
	Sectors                 []paperPortfolioSectorRisk `json:"sectors"`
}

type paperPortfolioRiskSummary struct {
	Environment     string                       `json:"environment"`
	AccountModel    string                       `json:"accountModel"`
	GeneratedAt     time.Time                    `json:"generatedAt"`
	SnapshotAt      *time.Time                   `json:"snapshotAt,omitempty"`
	SnapshotFresh   bool                         `json:"snapshotFresh"`
	SnapshotReason  string                       `json:"snapshotReason,omitempty"`
	QuoteObservedAt *time.Time                   `json:"quoteObservedAt,omitempty"`
	Projected       bool                         `json:"projected"`
	RiskComplete    bool                         `json:"riskComplete"`
	Degraded        bool                         `json:"degraded"`
	UnknownSymbols  []string                     `json:"unknownSymbols"`
	Sources         []string                     `json:"sources"`
	SourceErrors    []string                     `json:"sourceErrors,omitempty"`
	Limits          paperPortfolioLimits         `json:"limits"`
	Groups          []paperPortfolioCurrencyRisk `json:"groups"`
	BaseCurrency    paperBaseCurrencyRisk        `json:"baseCurrency"`
	Disclaimer      string                       `json:"disclaimer"`
}

type paperBaseCurrencyRisk struct {
	Currency                 string     `json:"currency"`
	Status                   string     `json:"status"`
	TotalEquity              *float64   `json:"totalEquity,omitempty"`
	FXSource                 string     `json:"fxSource,omitempty"`
	FXObservedAt             *time.Time `json:"fxObservedAt,omitempty"`
	Reason                   string     `json:"reason,omitempty"`
	RequiredForCurrentPolicy bool       `json:"requiredForCurrentPolicy"`
}

type paperPortfolioProjection struct {
	Request        paperOrderRequest
	Risk           models.PaperRiskSnapshot
	ReplaceOrderID uint
}

type paperSecurity struct {
	Sector    string
	ADV       *float64
	ADVSource string
}

type paperPositionMark struct {
	Quote paperAuthoritativeQuote
	Error string
}

// paperPortfolioMarketSnapshot contains every external input needed by the
// portfolio evaluator. It is collected before any SQLite write transaction.
type paperPortfolioMarketSnapshot struct {
	CapturedAt      time.Time
	CaptureDuration time.Duration
	CoveredSymbols  map[string]bool
	SecurityMaster  map[string]paperSecurity
	PositionMarks   map[string]paperPositionMark
	Sources         []string
	SourceErrors    []string
	QuoteObservedAt time.Time
	CaptureError    string
}

type paperSymbolAccumulator struct {
	response           paperPortfolioSymbolRisk
	positionValue      float64
	positionValueKnown bool
	grossExposure      float64
}

type paperGroupAccumulator struct {
	response       paperPortfolioCurrencyRisk
	symbols        map[string]*paperSymbolAccumulator
	sectorExposure map[string]float64
}

func defaultPaperRiskPolicy() paperPortfolioLimits {
	return paperPortfolioLimits{
		PolicyVersion:           paperDefaultRiskPolicyVersion,
		MaxPortfolioExposurePct: paperMaxPortfolioExposurePct,
		MaxSymbolExposurePct:    paperMaxSymbolExposurePct,
		MaxSectorExposurePct:    paperMaxSectorExposurePct,
		MaxOpenOrderLossPct:     paperMaxOpenOrderLossPct,
		MaxOrderADVPercent:      paperMaxOrderADVPercent,
		MinStopCoveragePct:      paperDefaultMinStopCoveragePct,
		MaxDailyLossPct:         paperDefaultMaxDailyLossPct,
		MaxDrawdownPct:          paperDefaultMaxDrawdownPct,
		MaxStressLossPct:        paperDefaultMaxStressLossPct,
	}
}

func paperRiskPolicyFromConfig(cfg *config.Config) paperPortfolioLimits {
	policy := defaultPaperRiskPolicy()
	if cfg == nil {
		return policy
	}
	if version := strings.TrimSpace(cfg.PaperRiskPolicyVersion); version != "" {
		policy.PolicyVersion = version
	}
	if cfg.PaperMinStopCoveragePct >= 0 && cfg.PaperMinStopCoveragePct <= 100 && cfg.PaperMinStopCoveragePct != 0 {
		policy.MinStopCoveragePct = cfg.PaperMinStopCoveragePct
	}
	if cfg.PaperMaxDailyLossPct > 0 && cfg.PaperMaxDailyLossPct <= 100 {
		policy.MaxDailyLossPct = cfg.PaperMaxDailyLossPct
	}
	if cfg.PaperMaxDrawdownPct > 0 && cfg.PaperMaxDrawdownPct <= 100 {
		policy.MaxDrawdownPct = cfg.PaperMaxDrawdownPct
	}
	if cfg.PaperMaxStressLossPct > 0 && cfg.PaperMaxStressLossPct <= 100 {
		policy.MaxStressLossPct = cfg.PaperMaxStressLossPct
	}
	return policy
}

// GetPaperPortfolioRisk returns the server-owned paper ledger risk. The router
// should protect this handler with the same read guard as paper accounts.
func (h *Handler) GetPaperPortfolioRisk(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "paper portfolio store unavailable"})
		return
	}
	summary, err := buildPaperPortfolioRiskAt(database.DB, nil, paperRiskPolicyFromConfig(h.config), h.paperNow())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func buildPaperPortfolioRiskAt(tx *gorm.DB, projection *paperPortfolioProjection, policy paperPortfolioLimits, now time.Time) (paperPortfolioRiskSummary, error) {
	snapshot := capturePaperPortfolioMarketSnapshot(tx, projection, now)
	return buildPaperPortfolioRiskFromSnapshotAt(tx, projection, policy, now, snapshot)
}

func capturePaperPortfolioMarketSnapshot(db *gorm.DB, projection *paperPortfolioProjection, now time.Time) paperPortfolioMarketSnapshot {
	started := time.Now()
	snapshot := paperPortfolioMarketSnapshot{CapturedAt: now.UTC(), CoveredSymbols: make(map[string]bool)}
	var positions []models.PaperPosition
	var orders []models.PaperOrder
	if err := db.Where("quantity <> 0 OR reserved_quantity <> 0").Order("currency asc, symbol asc").Find(&positions).Error; err != nil {
		snapshot.CaptureError = "load positions for market snapshot: " + err.Error()
		snapshot.CaptureDuration = time.Since(started)
		return snapshot
	}
	if err := db.Where("status = ?", paperStatusAccepted).Order("created_at asc").Find(&orders).Error; err != nil {
		snapshot.CaptureError = "load orders for market snapshot: " + err.Error()
		snapshot.CaptureDuration = time.Since(started)
		return snapshot
	}
	requiredSymbols := paperPortfolioRequiredSymbols(positions, orders, projection)
	for _, symbol := range requiredSymbols {
		snapshot.CoveredSymbols[symbol] = true
	}
	snapshot.SecurityMaster, snapshot.Sources, snapshot.SourceErrors = loadPaperSecurityMaster(requiredSymbols)
	marks, quoteSources, quoteErrors, quoteObservedAt := loadPaperPositionMarks(positions, snapshot.CapturedAt)
	snapshot.PositionMarks = marks
	snapshot.Sources = append(snapshot.Sources, quoteSources...)
	snapshot.SourceErrors = append(snapshot.SourceErrors, quoteErrors...)
	snapshot.QuoteObservedAt = quoteObservedAt
	snapshot.CaptureDuration = time.Since(started)
	sort.Strings(snapshot.Sources)
	sort.Strings(snapshot.SourceErrors)
	return snapshot
}

func paperPortfolioRequiredSymbols(positions []models.PaperPosition, orders []models.PaperOrder, projection *paperPortfolioProjection) []string {
	seen := make(map[string]bool, len(positions)+len(orders)+1)
	for _, position := range positions {
		seen[strings.ToUpper(strings.TrimSpace(position.Symbol))] = true
	}
	for _, order := range orders {
		seen[strings.ToUpper(strings.TrimSpace(order.Symbol))] = true
	}
	if projection != nil {
		seen[strings.ToUpper(strings.TrimSpace(projection.Request.Symbol))] = true
	}
	delete(seen, "")
	symbols := make([]string, 0, len(seen))
	for symbol := range seen {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	return symbols
}

func buildPaperPortfolioRiskFromSnapshotAt(tx *gorm.DB, projection *paperPortfolioProjection, policy paperPortfolioLimits, now time.Time, snapshot paperPortfolioMarketSnapshot) (paperPortfolioRiskSummary, error) {
	summary := paperPortfolioRiskSummary{
		Environment: paperEnvironment, AccountModel: "currency-isolated-paper-subaccounts", GeneratedAt: now.UTC(), Projected: projection != nil,
		RiskComplete: true, SnapshotFresh: true, Limits: policy, Disclaimer: paperDisclaimer(),
		UnknownSymbols: []string{}, Sources: []string{}, Groups: []paperPortfolioCurrencyRisk{},
		BaseCurrency: paperBaseCurrencyRisk{
			Currency: "CNY", Status: "unknown", RequiredForCurrentPolicy: false,
			Reason: "USD and CNY are isolated Paper subaccounts; no cross-currency total is computed without a sourced, observed-at, fresh FX quote, and current BUY hard limits are evaluated independently per currency",
		},
	}
	var accounts []models.PaperAccount
	var positions []models.PaperPosition
	var orders []models.PaperOrder
	var realizedSellFills []struct {
		CreatedAt   time.Time
		Currency    string
		Market      string
		RealizedPnL float64
	}
	if err := tx.Order("currency asc").Find(&accounts).Error; err != nil {
		return summary, err
	}
	if err := tx.Where("quantity <> 0 OR reserved_quantity <> 0").Order("currency asc, symbol asc").Find(&positions).Error; err != nil {
		return summary, err
	}
	if err := tx.Where("status = ?", paperStatusAccepted).Order("created_at asc").Find(&orders).Error; err != nil {
		return summary, err
	}
	if err := tx.Table("paper_fills AS fills").
		Select("fills.created_at, orders.currency, orders.market, fills.realized_pn_l").
		Joins("JOIN paper_orders AS orders ON orders.id = fills.paper_order_id").
		Where("orders.environment = ? AND orders.side = ? AND fills.created_at >= ? AND fills.created_at <= ?", paperEnvironment, "SELL", now.UTC().Add(-26*time.Hour), now.UTC()).
		Scan(&realizedSellFills).Error; err != nil {
		return summary, err
	}

	requiredSymbols := paperPortfolioRequiredSymbols(positions, orders, projection)
	snapshotAt := snapshot.CapturedAt.UTC()
	if !snapshotAt.IsZero() {
		summary.SnapshotAt = &snapshotAt
	}
	summary.SnapshotReason = paperPortfolioSnapshotReason(snapshot, requiredSymbols, summary.GeneratedAt)
	summary.SnapshotFresh = summary.SnapshotReason == ""
	if !summary.SnapshotFresh {
		summary.RiskComplete = false
		summary.Degraded = true
	}
	if !snapshot.QuoteObservedAt.IsZero() {
		quoteObservedAt := snapshot.QuoteObservedAt.UTC()
		summary.QuoteObservedAt = &quoteObservedAt
	}
	securityMaster := snapshot.SecurityMaster
	positionMarks := snapshot.PositionMarks
	summary.Sources = append([]string(nil), snapshot.Sources...)
	summary.SourceErrors = append([]string(nil), snapshot.SourceErrors...)
	if summary.SnapshotReason != "" {
		summary.SourceErrors = append(summary.SourceErrors, summary.SnapshotReason)
	}
	sort.Strings(summary.SourceErrors)
	groups := make(map[string]*paperGroupAccumulator, len(accounts))
	for _, account := range accounts {
		groups[account.Currency] = &paperGroupAccumulator{
			response: paperPortfolioCurrencyRisk{
				Currency: account.Currency, InitialCash: account.InitialCash, AvailableCash: account.Cash,
				ReservedCash: account.ReservedCash, ValuationComplete: true, SectorComplete: true, LiquidityComplete: true,
				UnknownSymbols: []string{}, Symbols: []paperPortfolioSymbolRisk{}, Sectors: []paperPortfolioSectorRisk{}, StressScenarios: []paperStressScenario{},
			},
			symbols: make(map[string]*paperSymbolAccumulator), sectorExposure: make(map[string]float64),
		}
	}
	ensureGroup := func(currency string) *paperGroupAccumulator {
		if group := groups[currency]; group != nil {
			return group
		}
		group := &paperGroupAccumulator{
			response: paperPortfolioCurrencyRisk{Currency: currency, ValuationComplete: true, SectorComplete: true, LiquidityComplete: true, UnknownSymbols: []string{}, Symbols: []paperPortfolioSymbolRisk{}, Sectors: []paperPortfolioSectorRisk{}, StressScenarios: []paperStressScenario{}},
			symbols:  make(map[string]*paperSymbolAccumulator), sectorExposure: make(map[string]float64),
		}
		groups[currency] = group
		return group
	}
	ensureSymbol := func(group *paperGroupAccumulator, symbol string) *paperSymbolAccumulator {
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		if row := group.symbols[symbol]; row != nil {
			return row
		}
		security, ok := securityMaster[symbol]
		sector := paperUnknownSector
		if ok && strings.TrimSpace(security.Sector) != "" {
			sector = security.Sector
		}
		row := &paperSymbolAccumulator{response: paperPortfolioSymbolRisk{Symbol: symbol, Sector: sector}}
		if ok && security.ADV != nil && *security.ADV > 0 {
			row.response.AverageDailyVolume = paperFloat(*security.ADV)
			row.response.LiquiditySource = security.ADVSource
			row.response.LiquidityComplete = true
		} else {
			row.response.LiquidityError = "authoritative average daily volume unavailable"
		}
		group.symbols[symbol] = row
		return row
	}

	for _, position := range positions {
		group := ensureGroup(position.Currency)
		row := ensureSymbol(group, position.Symbol)
		row.response.PositionQuantity += position.Quantity
		row.response.ReservedQuantity += position.ReservedQuantity
		row.response.AverageCost = position.AverageCost
		group.response.PositionQuantity += position.Quantity
		mark := positionMarks[strings.ToUpper(strings.TrimSpace(position.Symbol))]
		row.response.MarkSource = mark.Quote.Source
		if !mark.Quote.At.IsZero() {
			markTime := mark.Quote.At
			row.response.MarkTime = &markTime
		}
		if mark.Error == "" && mark.Quote.Price > 0 {
			row.positionValueKnown = true
			row.response.MarkPrice = paperFloat(mark.Quote.Price)
			row.positionValue += float64(position.Quantity) * mark.Quote.Price
		} else {
			row.response.ValuationError = mark.Error
			group.response.ValuationComplete = false
		}
	}
	for _, fill := range realizedSellFills {
		if paperSameTradingDay(fill.Market, fill.CreatedAt, summary.GeneratedAt) {
			group := ensureGroup(fill.Currency)
			group.response.RealizedPnLToday = roundPaperMoney(group.response.RealizedPnLToday + fill.RealizedPnL)
		}
	}
	for _, order := range orders {
		if projection != nil && projection.ReplaceOrderID != 0 && order.ID == projection.ReplaceOrderID {
			continue
		}
		quantity := effectivePaperRemainingQty(order)
		notional := order.RiskSnapshot.PlannedNotional
		maxLoss := order.RiskSnapshot.MaxLoss
		if order.Quantity > 0 && quantity != order.Quantity {
			notional = roundPaperMoney(notional * float64(quantity) / float64(order.Quantity))
			maxLoss = roundPaperMoney(maxLoss * float64(quantity) / float64(order.Quantity))
		}
		if order.ParentOrderID != nil && order.Side == "SELL" {
			quantity = order.ReservedQty
			if order.Quantity > 0 {
				notional = roundPaperMoney(notional * float64(quantity) / float64(order.Quantity))
				maxLoss = roundPaperMoney(maxLoss * float64(quantity) / float64(order.Quantity))
			}
		}
		if quantity > 0 {
			applyPaperOpenOrder(ensureGroup(order.Currency), ensureSymbol, order.Symbol, order.Side, order.OrderType, quantity, notional, maxLoss)
		}
	}
	if projection != nil {
		req := projection.Request
		group := ensureGroup(req.Currency)
		applyPaperOpenOrder(group, ensureSymbol, req.Symbol, req.Side, req.OrderType, req.Quantity, projection.Risk.PlannedNotional, projection.Risk.MaxLoss)
	}

	unknown := make(map[string]bool)
	currencies := make([]string, 0, len(groups))
	for currency := range groups {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	for _, currency := range currencies {
		group := groups[currency]
		finalizePaperPortfolioGroup(group, unknown)
		if group.response.TotalEquity != nil && group.response.ValuationComplete {
			if err := applyPaperPeakAndStress(tx, &group.response, summary.GeneratedAt); err != nil {
				group.response.Degraded = true
				group.response.DegradationReasons = append(group.response.DegradationReasons, "equity checkpoint unavailable: "+err.Error())
			}
		}
		summary.Groups = append(summary.Groups, group.response)
		if !group.response.ValuationComplete || !group.response.SectorComplete || !group.response.LiquidityComplete {
			summary.RiskComplete = false
			summary.Degraded = true
		}
	}
	for symbol := range unknown {
		summary.UnknownSymbols = append(summary.UnknownSymbols, symbol)
	}
	sort.Strings(summary.UnknownSymbols)
	return summary, nil
}

func paperPortfolioSnapshotReason(snapshot paperPortfolioMarketSnapshot, requiredSymbols []string, now time.Time) string {
	if snapshot.CaptureError != "" {
		return "paper market snapshot unavailable: " + snapshot.CaptureError
	}
	if snapshot.CapturedAt.IsZero() {
		return "paper market snapshot capture time is unavailable"
	}
	if snapshot.CaptureDuration > marketQuoteFreshFor {
		return fmt.Sprintf("paper market snapshot collection took %.0fs and exceeded %.0fs", snapshot.CaptureDuration.Seconds(), marketQuoteFreshFor.Seconds())
	}
	if snapshot.CapturedAt.After(now.Add(30 * time.Second)) {
		return "paper market snapshot capture time is in the future"
	}
	if now.Sub(snapshot.CapturedAt) > marketQuoteFreshFor {
		return fmt.Sprintf("paper market snapshot age %.0fs exceeds %.0fs", now.Sub(snapshot.CapturedAt).Seconds(), marketQuoteFreshFor.Seconds())
	}
	for _, symbol := range requiredSymbols {
		if !snapshot.CoveredSymbols[symbol] {
			return "paper market snapshot does not cover latest ledger symbol " + symbol
		}
	}
	return ""
}

func loadPaperSecurityMaster(requiredSymbols []string) (map[string]paperSecurity, []string, []string) {
	master := make(map[string]paperSecurity)
	var sources []string
	var sourceErrors []string
	requiredByMarket := map[string][]string{"us": {}, "cn": {}}
	seenRequired := map[string]bool{}
	for _, raw := range requiredSymbols {
		symbol := strings.ToUpper(strings.TrimSpace(raw))
		if symbol == "" || seenRequired[symbol] {
			continue
		}
		seenRequired[symbol] = true
		marketCode := "us"
		if isPaperCNSymbol(symbol) {
			marketCode = "cn"
		}
		requiredByMarket[marketCode] = append(requiredByMarket[marketCode], symbol)
	}
	for _, marketCode := range []string{"us", "cn"} {
		if len(requiredByMarket[marketCode]) == 0 {
			continue
		}
		stocks, source, err := paperUniverseLoader(marketCode)
		if err != nil {
			sourceErrors = append(sourceErrors, marketCode+": "+err.Error())
			continue
		}
		sources = append(sources, marketCode+":"+source)
		advBySymbol := map[string]float64{}
		advSource := ""
		if len(requiredByMarket[marketCode]) > 0 {
			var advErr error
			advBySymbol, advSource, advErr = paperADVLoader(marketCode, requiredByMarket[marketCode])
			if advErr != nil {
				sourceErrors = append(sourceErrors, marketCode+" ADV: "+advErr.Error())
			} else if advSource != "" {
				sources = append(sources, marketCode+"-adv:"+advSource)
			}
		}
		for _, stock := range stocks {
			symbol := strings.ToUpper(strings.TrimSpace(stock.Symbol))
			if symbol == "" {
				continue
			}
			sector := strings.TrimSpace(stock.Sector)
			if sector == "" || strings.EqualFold(sector, "A股") {
				sector = paperUnknownSector
			}
			security := paperSecurity{Sector: sector}
			if adv := advBySymbol[symbol]; adv > 0 {
				security.ADV = paperFloat(adv)
				security.ADVSource = advSource
			}
			master[symbol] = security
		}
	}
	sort.Strings(sources)
	sort.Strings(sourceErrors)
	return master, sources, sourceErrors
}

func loadPaperPositionMarks(positions []models.PaperPosition, now time.Time) (map[string]paperPositionMark, []string, []string, time.Time) {
	marks := make(map[string]paperPositionMark)
	var sources []string
	var errorsBySymbol []string
	symbols := make([]string, 0, len(positions))
	for _, position := range positions {
		if position.Quantity == 0 {
			continue
		}
		symbol := strings.ToUpper(strings.TrimSpace(position.Symbol))
		if _, exists := marks[symbol]; exists {
			continue
		}
		marks[symbol] = paperPositionMark{}
		symbols = append(symbols, symbol)
	}
	snapshot := fetchAuthoritativePaperQuotes(symbols, now)
	for _, symbol := range symbols {
		quote := snapshot.Quotes[symbol]
		reasons := snapshot.Reasons[symbol]
		if len(reasons) > 0 {
			reason := strings.Join(reasons, "; ")
			marks[symbol] = paperPositionMark{Quote: quote, Error: reason}
			errorsBySymbol = append(errorsBySymbol, symbol+": "+reason)
			continue
		}
		marks[symbol] = paperPositionMark{Quote: quote}
		sources = append(sources, "quote:"+symbol+":"+quote.Source+"@"+quote.At.UTC().Format(time.RFC3339))
	}
	sort.Strings(sources)
	sort.Strings(errorsBySymbol)
	return marks, sources, errorsBySymbol, snapshot.ObservedAt
}

func applyPaperOpenOrder(
	group *paperGroupAccumulator,
	ensureSymbol func(*paperGroupAccumulator, string) *paperSymbolAccumulator,
	symbol, side, orderType string,
	quantity int,
	notional, maxLoss float64,
) {
	row := ensureSymbol(group, symbol)
	if side == "BUY" {
		row.response.AcceptedBuyNotional += notional
		group.response.OpenBuyNotional += notional
	} else {
		row.response.AcceptedSellNotional += notional
		group.response.OpenSellNotional += notional
		if orderType == "STOP_MARKET" {
			row.response.StopCoveredQuantity += quantity
		}
	}
	group.response.OpenOrderMaxPlannedLoss += maxLoss
}

func finalizePaperPortfolioGroup(group *paperGroupAccumulator, unknown map[string]bool) {
	symbolNames := make([]string, 0, len(group.symbols))
	positionValue := 0.0
	grossExposure := 0.0
	unrealizedPnL := 0.0
	group.response.StopCoveredQuantity = 0
	for symbol := range group.symbols {
		symbolNames = append(symbolNames, symbol)
	}
	sort.Strings(symbolNames)
	for _, symbol := range symbolNames {
		row := group.symbols[symbol]
		if row.response.PositionQuantity > 0 && !row.positionValueKnown {
			group.response.ValuationComplete = false
			unknown[symbol] = true
			group.response.UnknownSymbols = append(group.response.UnknownSymbols, symbol)
			reason := row.response.ValuationError
			if reason == "" {
				reason = "authoritative position quote is unavailable"
			}
			group.response.DegradationReasons = append(group.response.DegradationReasons, symbol+": "+reason)
		}
		if row.response.Sector == paperUnknownSector {
			group.response.SectorComplete = false
			unknown[symbol] = true
			if !containsPaperSymbol(group.response.UnknownSymbols, symbol) {
				group.response.UnknownSymbols = append(group.response.UnknownSymbols, symbol)
			}
			group.response.DegradationReasons = append(group.response.DegradationReasons, symbol+": authoritative sector is unknown")
		}
		if !row.response.LiquidityComplete && (row.response.PositionQuantity > 0 || row.response.AcceptedBuyNotional > 0) {
			group.response.LiquidityComplete = false
			unknown[symbol] = true
			if !containsPaperSymbol(group.response.UnknownSymbols, symbol) {
				group.response.UnknownSymbols = append(group.response.UnknownSymbols, symbol)
			}
			group.response.DegradationReasons = append(group.response.DegradationReasons, symbol+": authoritative average daily volume unavailable")
		}
		if row.response.PositionQuantity == 0 || row.positionValueKnown {
			positionValue += row.positionValue
			if row.response.PositionQuantity != 0 {
				row.response.PositionMarketValue = paperFloat(row.positionValue)
				rowPnL := roundPaperMoney((paperValue(row.response.MarkPrice) - row.response.AverageCost) * float64(row.response.PositionQuantity))
				row.response.UnrealizedPnL = paperFloat(rowPnL)
				unrealizedPnL += rowPnL
			}
			row.grossExposure = row.positionValue + row.response.AcceptedBuyNotional
			row.response.GrossExposure = paperFloat(row.grossExposure)
			grossExposure += row.grossExposure
			if row.response.Sector != paperUnknownSector {
				group.sectorExposure[row.response.Sector] += row.grossExposure
			}
		}
		if row.response.PositionQuantity > 0 {
			covered := row.response.StopCoveredQuantity
			if covered > row.response.PositionQuantity {
				covered = row.response.PositionQuantity
			}
			row.response.StopCoveredQuantity = covered
			group.response.StopCoveredQuantity += covered
			row.response.StopCoveragePct = roundPaperPct(float64(covered) / float64(row.response.PositionQuantity) * 100)
		}
	}
	group.response.Degraded = !group.response.ValuationComplete || !group.response.SectorComplete || !group.response.LiquidityComplete
	if group.response.PositionQuantity > 0 {
		group.response.StopCoveragePct = roundPaperPct(float64(group.response.StopCoveredQuantity) / float64(group.response.PositionQuantity) * 100)
	}
	if group.response.ValuationComplete {
		totalEquity := group.response.AvailableCash + group.response.ReservedCash + positionValue
		group.response.PositionMarketValue = paperFloat(roundPaperMoney(positionValue))
		group.response.TotalEquity = paperFloat(roundPaperMoney(totalEquity))
		group.response.GrossExposure = paperFloat(roundPaperMoney(grossExposure))
		group.response.UnrealizedPnL = paperFloat(roundPaperMoney(unrealizedPnL))
		if totalEquity > 0 {
			group.response.GrossExposurePct = paperFloat(roundPaperPct(grossExposure / totalEquity * 100))
			group.response.OpenOrderMaxLossPct = paperFloat(roundPaperPct(group.response.OpenOrderMaxPlannedLoss / totalEquity * 100))
			for _, symbol := range symbolNames {
				row := group.symbols[symbol]
				if row.response.GrossExposure != nil {
					row.response.ExposurePct = paperFloat(roundPaperPct(*row.response.GrossExposure / totalEquity * 100))
					if group.response.LargestSymbolExposure == nil || *row.response.GrossExposure > *group.response.LargestSymbolExposure {
						group.response.LargestSymbol = symbol
						group.response.LargestSymbolExposure = paperFloat(roundPaperMoney(*row.response.GrossExposure))
						group.response.LargestSymbolPct = paperFloat(roundPaperPct(*row.response.GrossExposure / totalEquity * 100))
					}
				}
			}
		}
	}
	for _, symbol := range symbolNames {
		group.response.Symbols = append(group.response.Symbols, group.symbols[symbol].response)
	}
	sectors := make([]string, 0, len(group.sectorExposure))
	for sector := range group.sectorExposure {
		sectors = append(sectors, sector)
	}
	sort.Strings(sectors)
	for _, sector := range sectors {
		exposure := roundPaperMoney(group.sectorExposure[sector])
		row := paperPortfolioSectorRisk{Sector: sector, GrossExposure: paperFloat(exposure)}
		if group.response.TotalEquity != nil && *group.response.TotalEquity > 0 {
			row.ExposurePct = paperFloat(roundPaperPct(exposure / *group.response.TotalEquity * 100))
			if group.response.LargestSectorExposure == nil || exposure > *group.response.LargestSectorExposure {
				group.response.LargestSector = sector
				group.response.LargestSectorExposure = paperFloat(exposure)
				group.response.LargestSectorPct = paperFloat(*row.ExposurePct)
			}
		}
		group.response.Sectors = append(group.response.Sectors, row)
	}
}

func paperSameTradingDay(market string, left, right time.Time) bool {
	locationName := map[string]string{"US": "America/New_York", "CN": "Asia/Shanghai"}[strings.ToUpper(market)]
	location, err := time.LoadLocation(locationName)
	if err != nil {
		return false
	}
	return left.In(location).Format("2006-01-02") == right.In(location).Format("2006-01-02")
}

func applyPaperPeakAndStress(tx *gorm.DB, group *paperPortfolioCurrencyRisk, now time.Time) error {
	if group.TotalEquity == nil {
		return nil
	}
	current := *group.TotalEquity
	var checkpoint models.PaperEquityCheckpoint
	err := tx.Where("currency = ?", group.Currency).Order("equity desc, observed_at asc").First(&checkpoint).Error
	peak := current
	peakAt := now.UTC()
	if err == nil && checkpoint.Equity > peak {
		peak = checkpoint.Equity
		peakAt = checkpoint.ObservedAt
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	group.PeakEquity = paperFloat(roundPaperMoney(peak))
	group.PeakEquityAt = &peakAt
	drawdown := roundPaperMoney(math.Max(peak-current, 0))
	group.PeakDrawdown = paperFloat(drawdown)
	if peak > 0 {
		group.PeakDrawdownPct = paperFloat(roundPaperPct(drawdown / peak * 100))
	}
	marketDate := paperMarketDateForCurrency(group.Currency, now)
	var baseline models.PaperDailyEquityBaseline
	if err := tx.Where("currency = ? AND market_date = ?", group.Currency, marketDate).First(&baseline).Error; err == nil {
		group.DayStartEquity = paperFloat(roundPaperMoney(baseline.Equity))
		observedAt := baseline.ObservedAt.UTC()
		group.DayStartObservedAt = &observedAt
		dailyPnL := roundPaperMoney(current - baseline.Equity)
		group.DailyPnL = paperFloat(dailyPnL)
		dailyLossPct := 0.0
		if baseline.Equity > 0 && dailyPnL < 0 {
			dailyLossPct = roundPaperPct(-dailyPnL / baseline.Equity * 100)
		}
		group.DailyLossPct = paperFloat(dailyLossPct)
		group.DailyRiskComplete = true
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	positionValue := paperValue(group.PositionMarketValue)
	for _, shock := range []float64{-5, -10} {
		pnl := roundPaperMoney(positionValue * shock / 100)
		equityAfter := roundPaperMoney(current + pnl)
		group.StressScenarios = append(group.StressScenarios, paperStressScenario{
			Name: fmt.Sprintf("uniform price shock %.0f%%", shock), Kind: "market", Status: "known",
			Assumption: "all marked positions move by the stated percentage with no liquidity adjustment", PriceShockPct: shock,
			EquityAfter: paperFloat(equityAfter), PnL: paperFloat(pnl),
		})
	}
	if group.SectorComplete && group.LargestSectorExposure != nil {
		const shock = -15.0
		pnl := roundPaperMoney(*group.LargestSectorExposure * shock / 100)
		group.StressScenarios = append(group.StressScenarios, paperStressScenario{
			Name: "largest sector concentration shock", Kind: "sector-concentration", Status: "known",
			Assumption: "largest identified sector falls 15%; other sectors are unchanged", PriceShockPct: shock,
			EquityAfter: paperFloat(roundPaperMoney(current + pnl)), PnL: paperFloat(pnl),
		})
	} else if group.SectorComplete {
		group.StressScenarios = append(group.StressScenarios, paperStressScenario{
			Name: "largest sector concentration shock", Kind: "sector-concentration", Status: "known",
			Assumption: "no identified sector exposure is present", PriceShockPct: -15,
			EquityAfter: paperFloat(roundPaperMoney(current)), PnL: paperFloat(0),
		})
	} else {
		group.StressScenarios = append(group.StressScenarios, paperStressScenario{
			Name: "largest sector concentration shock", Kind: "sector-concentration", Status: "unknown",
			UnknownReason: "authoritative sector exposure is incomplete",
		})
	}
	if group.LiquidityComplete {
		maxUsagePct := 0.0
		for _, symbol := range group.Symbols {
			if symbol.PositionQuantity <= 0 || symbol.AverageDailyVolume == nil || *symbol.AverageDailyVolume <= 0 {
				continue
			}
			usagePct := float64(symbol.PositionQuantity) / (*symbol.AverageDailyVolume * 0.5) * 100
			if usagePct > maxUsagePct {
				maxUsagePct = usagePct
			}
		}
		group.StressScenarios = append(group.StressScenarios, paperStressScenario{
			Name: "liquidity volume shock", Kind: "liquidity", Status: "known",
			Assumption:        "available daily volume falls to 50% of authoritative ADV; reports maximum full-position participation only",
			LiquidityUsagePct: paperFloat(roundPaperPct(maxUsagePct)),
		})
	} else {
		group.StressScenarios = append(group.StressScenarios, paperStressScenario{
			Name: "liquidity volume shock", Kind: "liquidity", Status: "unknown",
			UnknownReason: "authoritative average daily volume is incomplete",
		})
	}
	return nil
}

func paperMarketDateForCurrency(currency string, now time.Time) string {
	locationName := "UTC"
	if strings.EqualFold(currency, "USD") {
		locationName = "America/New_York"
	} else if strings.EqualFold(currency, "CNY") {
		locationName = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(locationName)
	if err != nil {
		location = time.UTC
	}
	return now.In(location).Format("2006-01-02")
}

func ensurePaperDailyEquityBaselinesTx(tx *gorm.DB, summary paperPortfolioRiskSummary, observedAt time.Time) ([]models.PaperDailyEquityBaseline, error) {
	created := make([]models.PaperDailyEquityBaseline, 0, len(summary.Groups))
	for _, group := range summary.Groups {
		if group.TotalEquity == nil || !group.ValuationComplete {
			continue
		}
		marketDate := paperMarketDateForCurrency(group.Currency, observedAt)
		var existing models.PaperDailyEquityBaseline
		err := tx.Where("currency = ? AND market_date = ?", group.Currency, marketDate).First(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		baseline := models.PaperDailyEquityBaseline{
			Currency: group.Currency, MarketDate: marketDate, ObservedAt: observedAt.UTC(), Equity: roundPaperMoney(*group.TotalEquity),
		}
		if err := tx.Create(&baseline).Error; err != nil {
			return nil, err
		}
		created = append(created, baseline)
	}
	return created, nil
}

func persistPaperEquityCheckpoints(tx *gorm.DB, summary paperPortfolioRiskSummary, observedAt time.Time) (int, error) {
	written := 0
	err := tx.Transaction(func(checkpointTx *gorm.DB) error {
		for _, group := range summary.Groups {
			if group.TotalEquity == nil || !group.ValuationComplete {
				continue
			}
			checkpoint := models.PaperEquityCheckpoint{
				ObservedAt: observedAt.UTC(), Currency: group.Currency, Equity: *group.TotalEquity,
			}
			if err := checkpointTx.Create(&checkpoint).Error; err != nil {
				return err
			}
			written++
		}
		return nil
	})
	return written, err
}

func applyPaperPortfolioLimits(req paperOrderRequest, validation *paperValidation, summary paperPortfolioRiskSummary, checkedAt time.Time, phase string) {
	policyReasonStart := len(validation.Reasons)
	policy := summary.Limits
	validation.Risk.PolicyVersion = policy.PolicyVersion
	validation.Risk.PolicyReasons = nil
	defer func() {
		reasons := append([]string(nil), validation.Reasons[policyReasonStart:]...)
		recordPaperPolicyCheck(&validation.Risk, policy, reasons, checkedAt, phase)
	}()
	validation.Risk.MaxPortfolioExposurePct = paperMaxPortfolioExposurePct
	validation.Risk.MaxSymbolExposurePct = paperMaxSymbolExposurePct
	validation.Risk.MaxSectorExposurePct = paperMaxSectorExposurePct
	validation.Risk.MaxOpenOrderLossPct = paperMaxOpenOrderLossPct
	validation.Risk.MaxOrderADVPercent = paperMaxOrderADVPercent
	validation.Risk.MinStopCoveragePct = policy.MinStopCoveragePct
	validation.Risk.MaxDailyLossPct = policy.MaxDailyLossPct
	validation.Risk.MaxDrawdownPct = policy.MaxDrawdownPct
	validation.Risk.MaxStressLossPct = policy.MaxStressLossPct
	policyReject := func(reason string) {
		validation.Reasons = append(validation.Reasons, reason)
	}
	if req.Side == "BUY" {
		if validation.Risk.MaxLoss > validation.Risk.MaxRiskAmount+0.01 {
			policyReject("max loss exceeds server paper-account risk budget")
		}
		if validation.Risk.PlannedNotional > validation.Risk.MaxNotionalAmount+0.01 {
			policyReject("planned notional exceeds server paper-account limit")
		}
	}
	group := findPaperRiskGroup(summary.Groups, req.Currency)
	if group == nil {
		policyReject("paper portfolio total equity is unavailable")
		return
	}
	validation.Risk.PortfolioDegraded = group.Degraded
	validation.Risk.PortfolioOffendingSymbols = append([]string(nil), group.UnknownSymbols...)
	validation.Risk.RiskWarnings = append([]string(nil), group.DegradationReasons...)
	if group.GrossExposure != nil {
		validation.Risk.PortfolioGrossExposure = *group.GrossExposure
	}
	validation.Risk.PortfolioGrossExposurePct = paperValue(group.GrossExposurePct)
	validation.Risk.OpenOrderMaxLoss = group.OpenOrderMaxPlannedLoss
	validation.Risk.OpenOrderMaxLossPct = paperValue(group.OpenOrderMaxLossPct)
	if req.Side != "BUY" {
		return
	}
	if !summary.SnapshotFresh {
		policyReject(summary.SnapshotReason)
		return
	}
	if !group.ValuationComplete || !group.SectorComplete || !group.LiquidityComplete {
		policyReject("paper portfolio risk is incomplete for BUY: " + strings.Join(group.DegradationReasons, "; "))
		return
	}
	if group.TotalEquity == nil || *group.TotalEquity <= 0 || group.GrossExposure == nil {
		policyReject("paper portfolio total equity is unavailable")
		return
	}
	if !group.DailyRiskComplete || group.DayStartEquity == nil || group.DayStartObservedAt == nil {
		policyReject("paper daily equity baseline is unavailable for the current trading day; BUY requires the scanner-established baseline")
		return
	}
	validation.Risk.StopCoveragePct = group.StopCoveragePct
	validation.Risk.DrawdownPct = paperValue(group.PeakDrawdownPct)
	validation.Risk.DailyLossPct = paperValue(group.DailyLossPct)
	for _, scenario := range group.StressScenarios {
		if scenario.PnL == nil || *scenario.PnL >= 0 {
			continue
		}
		lossPct := roundPaperPct(-*scenario.PnL / *group.TotalEquity * 100)
		if lossPct > validation.Risk.StressLossPct {
			validation.Risk.StressLossPct = lossPct
		}
	}
	if group.PositionQuantity > 0 && validation.Risk.StopCoveragePct+0.01 < policy.MinStopCoveragePct {
		policyReject(fmt.Sprintf("paper stop coverage %.2f%% is below policy minimum %.2f%%", validation.Risk.StopCoveragePct, policy.MinStopCoveragePct))
	}
	if validation.Risk.DailyLossPct > policy.MaxDailyLossPct+0.01 {
		policyReject(fmt.Sprintf("paper daily equity loss %.2f%% exceeds policy maximum %.2f%%", validation.Risk.DailyLossPct, policy.MaxDailyLossPct))
	}
	if validation.Risk.DrawdownPct > policy.MaxDrawdownPct+0.01 {
		policyReject(fmt.Sprintf("paper peak drawdown %.2f%% exceeds policy maximum %.2f%%", validation.Risk.DrawdownPct, policy.MaxDrawdownPct))
	}
	if validation.Risk.StressLossPct > policy.MaxStressLossPct+0.01 {
		policyReject(fmt.Sprintf("paper stress loss %.2f%% exceeds policy maximum %.2f%%", validation.Risk.StressLossPct, policy.MaxStressLossPct))
	}
	if validation.Risk.PortfolioGrossExposurePct > paperMaxPortfolioExposurePct+0.01 {
		policyReject("projected paper portfolio gross exposure exceeds 80% of total equity")
	}
	if validation.Risk.OpenOrderMaxLossPct > paperMaxOpenOrderLossPct+0.01 {
		policyReject("projected open-order max planned loss exceeds 6% of total equity")
	}
	for _, symbol := range group.Symbols {
		if symbol.Symbol != req.Symbol {
			continue
		}
		validation.Risk.SymbolExposurePct = paperValue(symbol.ExposurePct)
		validation.Risk.Sector = symbol.Sector
		validation.Risk.AverageDailyVolume = symbol.AverageDailyVolume
		validation.Risk.LiquiditySource = symbol.LiquiditySource
		validation.Risk.LiquidityComplete = symbol.LiquidityComplete
		if symbol.AverageDailyVolume == nil || *symbol.AverageDailyVolume <= 0 {
			policyReject("authoritative average daily volume is unavailable for BUY")
		} else {
			advPct := roundPaperPct(float64(req.Quantity) / *symbol.AverageDailyVolume * 100)
			validation.Risk.PlannedADVPercent = paperFloat(advPct)
			symbol.PlannedADVPercent = paperFloat(advPct)
			if advPct > paperMaxOrderADVPercent+0.0001 {
				policyReject("planned quantity exceeds 1% of authoritative average daily volume")
			}
		}
		if symbol.Sector == paperUnknownSector {
			policyReject("authoritative sector is unknown for symbol")
		}
		if validation.Risk.SymbolExposurePct > paperMaxSymbolExposurePct+0.01 {
			policyReject("projected single-symbol exposure exceeds 25% of total equity")
		}
		break
	}
	for _, sector := range group.Sectors {
		if sector.Sector == validation.Risk.Sector {
			validation.Risk.SectorExposurePct = paperValue(sector.ExposurePct)
			if validation.Risk.SectorExposurePct > paperMaxSectorExposurePct+0.01 {
				policyReject("projected sector exposure exceeds 40% of total equity")
			}
			break
		}
	}
}

func recordPaperPolicyCheck(risk *models.PaperRiskSnapshot, policy paperPortfolioLimits, reasons []string, checkedAt time.Time, phase string) {
	risk.PolicyVersion = policy.PolicyVersion
	risk.PolicyReasons = append([]string(nil), reasons...)
	check := &models.PaperPolicyCheck{
		PolicyVersion: policy.PolicyVersion,
		Reasons:       append([]string(nil), reasons...),
		CheckedAt:     checkedAt.UTC(),
		Passed:        len(reasons) == 0,
	}
	if phase == "fill" {
		risk.FillPolicyCheck = check
	} else {
		risk.SubmissionPolicyCheck = check
	}
}

func findPaperRiskGroup(groups []paperPortfolioCurrencyRisk, currency string) *paperPortfolioCurrencyRisk {
	for index := range groups {
		if groups[index].Currency == currency {
			return &groups[index]
		}
	}
	return nil
}

func paperFloat(value float64) *float64 {
	copy := value
	return &copy
}

func paperValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func roundPaperPct(value float64) float64 {
	return roundPaperPrice(value)
}

func containsPaperSymbol(symbols []string, target string) bool {
	for _, symbol := range symbols {
		if symbol == target {
			return true
		}
	}
	return false
}
