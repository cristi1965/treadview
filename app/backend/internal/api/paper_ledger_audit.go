package api

import (
	"fmt"
	"math"
	"strings"

	"gorm.io/gorm"

	"trading-agents/internal/models"
)

type paperLedgerPositionKey struct {
	Currency string
	Symbol   string
}

type paperLedgerFillAggregate struct {
	Count       int
	Quantity    int
	Notional    float64
	Slippage    float64
	Fee         float64
	RealizedPnL float64
	Last        models.PaperFill
}

type paperLedgerReplayPosition struct {
	Quantity    int
	AverageCost float64
}

func verifyPaperLedgerIntegrity(db *gorm.DB, auditedFillIDs map[string]struct{}) string {
	if db == nil {
		return "paper ledger cannot be read"
	}
	// Audit-only deployments and focused middleware tests may not install the
	// Paper subsystem. Once any Paper ledger table exists, however, the complete
	// schema and all invariants below are required.
	if !db.Migrator().HasTable(&models.PaperOrder{}) &&
		!db.Migrator().HasTable(&models.PaperFill{}) &&
		!db.Migrator().HasTable(&models.PaperAccount{}) &&
		!db.Migrator().HasTable(&models.PaperPosition{}) {
		return ""
	}
	var orders []models.PaperOrder
	if err := db.Order("id asc").Find(&orders).Error; err != nil {
		return "paper orders cannot be read: " + err.Error()
	}
	var fills []models.PaperFill
	if err := db.Order("created_at asc, id asc").Find(&fills).Error; err != nil {
		return "paper fills cannot be read: " + err.Error()
	}

	ordersByID := make(map[uint]models.PaperOrder, len(orders))
	aggregates := make(map[uint]*paperLedgerFillAggregate, len(orders))
	unauditedFillID := ""
	for _, order := range orders {
		ordersByID[order.ID] = order
	}
	for _, fill := range fills {
		if auditedFillIDs != nil {
			if _, exists := auditedFillIDs[fill.FillID]; !exists && unauditedFillID == "" {
				unauditedFillID = fill.FillID
			}
		}
		order, exists := ordersByID[fill.PaperOrderID]
		if !exists {
			return fmt.Sprintf("paper fill %s references missing order %d", fill.FillID, fill.PaperOrderID)
		}
		aggregate := aggregates[fill.PaperOrderID]
		if aggregate == nil {
			aggregate = &paperLedgerFillAggregate{}
			aggregates[fill.PaperOrderID] = aggregate
		}
		if fill.Sequence != aggregate.Count+1 {
			return fmt.Sprintf("paper order %s fill sequence mismatch", order.ClientOrderID)
		}
		if fill.Quantity <= 0 || fill.Price <= 0 || fill.Quote.Price <= 0 || strings.TrimSpace(fill.Quote.Source) == "" || fill.Quote.ObservedAt.IsZero() {
			return fmt.Sprintf("paper fill %s execution evidence is invalid", fill.FillID)
		}
		if fill.Fee < 0 || fill.Slippage < 0 || !paperLedgerMoneyEqual(fill.Fee, paperFee(roundPaperMoney(fill.Price*float64(fill.Quantity)))) {
			return fmt.Sprintf("paper fill %s fee mismatch", fill.FillID)
		}
		expectedSlippage := roundPaperMoney(math.Abs(fill.Price-fill.Quote.Price) * float64(fill.Quantity))
		if !paperLedgerMoneyEqual(fill.Slippage, expectedSlippage) {
			return fmt.Sprintf("paper fill %s slippage mismatch", fill.FillID)
		}
		aggregate.Count++
		aggregate.Quantity += fill.Quantity
		aggregate.Notional += fill.Price * float64(fill.Quantity)
		aggregate.Slippage = roundPaperMoney(aggregate.Slippage + fill.Slippage)
		aggregate.Fee = roundPaperMoney(aggregate.Fee + fill.Fee)
		aggregate.RealizedPnL = roundPaperMoney(aggregate.RealizedPnL + fill.RealizedPnL)
		aggregate.Last = fill
	}

	ocoChildren := make(map[string][]models.PaperOrder)
	reservedCash := make(map[string]float64)
	reservedQuantity := make(map[paperLedgerPositionKey]int)
	protectedQuantity := make(map[paperLedgerPositionKey]int)
	for _, order := range orders {
		aggregate := aggregates[order.ID]
		if aggregate == nil {
			aggregate = &paperLedgerFillAggregate{}
		}
		if order.Quantity <= 0 || aggregate.Quantity > order.Quantity {
			return fmt.Sprintf("paper order %s quantity is invalid", order.ClientOrderID)
		}
		expectedRemaining := order.Quantity - aggregate.Quantity
		if order.FillQty != aggregate.Quantity {
			return fmt.Sprintf("paper order %s fill quantity mismatch", order.ClientOrderID)
		}
		if order.RemainingQty != expectedRemaining {
			return fmt.Sprintf("paper order %s remaining quantity mismatch", order.ClientOrderID)
		}
		if aggregate.Count == 0 {
			if !paperLedgerPriceEqual(order.FillPrice, 0) || !paperLedgerMoneyEqual(order.Slippage, 0) || !paperLedgerMoneyEqual(order.Fee, 0) || !paperLedgerMoneyEqual(order.RealizedPnL, 0) || order.FillQuote != nil || order.FilledAt != nil {
				return fmt.Sprintf("paper order %s has fill aggregates without fill rows", order.ClientOrderID)
			}
		} else {
			expectedFillPrice := roundPaperPrice(aggregate.Notional / float64(aggregate.Quantity))
			if !paperLedgerPriceEqual(order.FillPrice, expectedFillPrice) {
				return fmt.Sprintf("paper order %s fill price mismatch", order.ClientOrderID)
			}
			if !paperLedgerMoneyEqual(order.Slippage, aggregate.Slippage) {
				return fmt.Sprintf("paper order %s slippage mismatch", order.ClientOrderID)
			}
			if !paperLedgerMoneyEqual(order.Fee, aggregate.Fee) {
				return fmt.Sprintf("paper order %s fee mismatch", order.ClientOrderID)
			}
			if !paperLedgerMoneyEqual(order.RealizedPnL, aggregate.RealizedPnL) {
				return fmt.Sprintf("paper order %s realized PnL mismatch", order.ClientOrderID)
			}
			if order.FillQuote == nil || !samePaperLedgerQuote(*order.FillQuote, aggregate.Last.Quote) {
				return fmt.Sprintf("paper order %s latest fill quote mismatch", order.ClientOrderID)
			}
		}
		if len(order.StatusHistory) == 0 || order.StatusHistory[len(order.StatusHistory)-1].Status != order.Status {
			return fmt.Sprintf("paper order %s status history mismatch", order.ClientOrderID)
		}
		switch order.Status {
		case paperStatusAccepted:
			if expectedRemaining <= 0 || order.FilledAt != nil {
				return fmt.Sprintf("paper order %s accepted status is inconsistent with fills", order.ClientOrderID)
			}
		case paperStatusSimulatedFill:
			if aggregate.Quantity != order.Quantity || order.FilledAt == nil || !order.FilledAt.Equal(aggregate.Last.CreatedAt) {
				return fmt.Sprintf("paper order %s filled status is inconsistent with fills", order.ClientOrderID)
			}
		case paperStatusCancelled:
			if expectedRemaining <= 0 || order.FilledAt != nil {
				return fmt.Sprintf("paper order %s cancelled status is inconsistent with fills", order.ClientOrderID)
			}
		case paperStatusRejected:
			if aggregate.Count != 0 || expectedRemaining != order.Quantity || order.FilledAt != nil {
				return fmt.Sprintf("paper order %s rejected status is inconsistent with fills", order.ClientOrderID)
			}
		default:
			return fmt.Sprintf("paper order %s has unknown status %s", order.ClientOrderID, order.Status)
		}
		if order.ReservedCash < 0 || order.ReservedQty < 0 || (order.Status != paperStatusAccepted && (order.ReservedCash != 0 || order.ReservedQty != 0)) {
			return fmt.Sprintf("paper order %s reservation/status mismatch", order.ClientOrderID)
		}
		key := paperLedgerPositionKey{Currency: order.Currency, Symbol: order.Symbol}
		if order.Status == paperStatusAccepted {
			if order.Side == "BUY" {
				reservedCash[order.Currency] = roundPaperMoney(reservedCash[order.Currency] + order.ReservedCash)
			} else if order.Side == "SELL" {
				reservedQuantity[key] += order.ReservedQty
			} else {
				return fmt.Sprintf("paper order %s side is invalid", order.ClientOrderID)
			}
		}
		if order.ParentOrderID != nil {
			parent, exists := ordersByID[*order.ParentOrderID]
			if !exists || order.Side != "SELL" || order.OCOGroupID == "" || parent.OCOGroupID != order.OCOGroupID {
				return fmt.Sprintf("paper order %s OCO parent relationship is invalid", order.ClientOrderID)
			}
			ocoChildren[order.OCOGroupID] = append(ocoChildren[order.OCOGroupID], order)
		}
		if order.ParentOrderID == nil && order.ProtectionInitialized {
			if order.ProtectionRemainingQty < 0 || order.ProtectionRemainingQty > order.FillQty || order.OCOGroupID == "" {
				return fmt.Sprintf("paper order %s protection quantity is invalid", order.ClientOrderID)
			}
			protectedQuantity[key] += order.ProtectionRemainingQty
		}
	}

	for groupID, children := range ocoChildren {
		filled := 0
		accepted := 0
		for _, child := range children {
			if child.Status == paperStatusSimulatedFill {
				filled++
			}
			if child.Status == paperStatusAccepted {
				accepted++
			}
		}
		if filled > 1 {
			return fmt.Sprintf("paper OCO group %s has multiple filled siblings", groupID)
		}
		if filled == 1 && accepted > 0 {
			return fmt.Sprintf("paper OCO group %s has active sibling after terminal fill", groupID)
		}
	}
	if unauditedFillID != "" {
		return fmt.Sprintf("paper fill %s evidence is missing from audit chain", unauditedFillID)
	}

	replayedPositions := make(map[paperLedgerPositionKey]paperLedgerReplayPosition)
	cashFlow := make(map[string]float64)
	for _, fill := range fills {
		order := ordersByID[fill.PaperOrderID]
		key := paperLedgerPositionKey{Currency: order.Currency, Symbol: order.Symbol}
		position := replayedPositions[key]
		notional := roundPaperMoney(fill.Price * float64(fill.Quantity))
		if order.Side == "BUY" {
			if !paperLedgerMoneyEqual(fill.RealizedPnL, 0) {
				return fmt.Sprintf("paper fill %s realized PnL mismatch", fill.FillID)
			}
			newQuantity := position.Quantity + fill.Quantity
			position.AverageCost = roundPaperPrice((position.AverageCost*float64(position.Quantity) + fill.Price*float64(fill.Quantity)) / float64(newQuantity))
			position.Quantity = newQuantity
			cashFlow[order.Currency] = roundPaperMoney(cashFlow[order.Currency] - notional - fill.Fee)
		} else if order.Side == "SELL" {
			if position.Quantity < fill.Quantity {
				return fmt.Sprintf("paper fill %s exceeds replayed position", fill.FillID)
			}
			expectedPnL := roundPaperMoney((fill.Price-position.AverageCost)*float64(fill.Quantity) - fill.Fee)
			if !paperLedgerMoneyEqual(fill.RealizedPnL, expectedPnL) {
				return fmt.Sprintf("paper fill %s realized PnL mismatch", fill.FillID)
			}
			position.Quantity -= fill.Quantity
			if position.Quantity == 0 {
				position.AverageCost = 0
			}
			cashFlow[order.Currency] = roundPaperMoney(cashFlow[order.Currency] + notional - fill.Fee)
		} else {
			return fmt.Sprintf("paper order %s side is invalid", order.ClientOrderID)
		}
		replayedPositions[key] = position
	}

	var accounts []models.PaperAccount
	if err := db.Order("currency asc").Find(&accounts).Error; err != nil {
		return "paper accounts cannot be read: " + err.Error()
	}
	accountCurrencies := make(map[string]bool, len(accounts))
	for _, account := range accounts {
		accountCurrencies[account.Currency] = true
		if account.InitialCash < 0 || account.Cash < 0 || account.ReservedCash < 0 {
			return fmt.Sprintf("paper account %s contains negative cash", account.Currency)
		}
		expectedReserved := reservedCash[account.Currency]
		expectedCash := roundPaperMoney(account.InitialCash + cashFlow[account.Currency] - expectedReserved)
		if !paperLedgerMoneyEqual(account.ReservedCash, expectedReserved) {
			return fmt.Sprintf("paper account %s reserved cash mismatch", account.Currency)
		}
		if !paperLedgerMoneyEqual(account.Cash, expectedCash) {
			return fmt.Sprintf("paper account %s account cash mismatch", account.Currency)
		}
	}
	for currency := range cashFlow {
		if !accountCurrencies[currency] {
			return fmt.Sprintf("paper account %s is missing", currency)
		}
	}
	for currency := range reservedCash {
		if !accountCurrencies[currency] {
			return fmt.Sprintf("paper account %s is missing", currency)
		}
	}

	var positions []models.PaperPosition
	if err := db.Order("currency asc, symbol asc").Find(&positions).Error; err != nil {
		return "paper positions cannot be read: " + err.Error()
	}
	actualPositionKeys := make(map[paperLedgerPositionKey]bool, len(positions))
	for _, position := range positions {
		key := paperLedgerPositionKey{Currency: position.Currency, Symbol: position.Symbol}
		actualPositionKeys[key] = true
		expected := replayedPositions[key]
		if position.Quantity != expected.Quantity {
			return fmt.Sprintf("paper position %s/%s position quantity mismatch", position.Currency, position.Symbol)
		}
		if position.ReservedQuantity != reservedQuantity[key] {
			return fmt.Sprintf("paper position %s/%s reserved quantity mismatch", position.Currency, position.Symbol)
		}
		if position.Quantity < 0 || position.ReservedQuantity < 0 || position.ReservedQuantity > position.Quantity {
			return fmt.Sprintf("paper position %s/%s reservation is invalid", position.Currency, position.Symbol)
		}
		if !paperLedgerPriceEqual(position.AverageCost, expected.AverageCost) {
			return fmt.Sprintf("paper position %s/%s average cost mismatch", position.Currency, position.Symbol)
		}
		if protectedQuantity[key] > position.Quantity {
			return fmt.Sprintf("paper position %s/%s protection quantity exceeds position", position.Currency, position.Symbol)
		}
	}
	for key, expected := range replayedPositions {
		if !actualPositionKeys[key] && (expected.Quantity != 0 || reservedQuantity[key] != 0) {
			return fmt.Sprintf("paper position %s/%s is missing", key.Currency, key.Symbol)
		}
	}
	for key, reserved := range reservedQuantity {
		if !actualPositionKeys[key] && reserved != 0 {
			return fmt.Sprintf("paper position %s/%s is missing", key.Currency, key.Symbol)
		}
	}
	return ""
}

func paperLedgerMoneyEqual(left, right float64) bool {
	return math.Abs(left-right) < 0.005
}

func paperLedgerPriceEqual(left, right float64) bool {
	return math.Abs(left-right) < 0.00005
}

func samePaperLedgerQuote(left, right models.PaperFillQuote) bool {
	return paperLedgerPriceEqual(left.Price, right.Price) && left.Source == right.Source && left.ProviderURL == right.ProviderURL && left.ObservedAt.Equal(right.ObservedAt)
}
