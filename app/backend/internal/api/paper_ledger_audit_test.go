package api

import (
	"net/http"
	"strings"
	"testing"

	"trading-agents/internal/database"
	"trading-agents/internal/models"
)

func assertPaperAuditValid(t *testing.T) []models.AuditEvent {
	t.Helper()
	var events []models.AuditEvent
	if err := database.DB.Order("id asc").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, events); !valid {
		t.Fatalf("untampered Paper ledger is invalid: %s", reason)
	}
	return events
}

func submitAndFillPaperOrderForAudit(t *testing.T, clientOrderID string) []models.AuditEvent {
	t.Helper()
	router, _ := setupPaperOrderTestRouter(t)
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload(clientOrderID)); got.Code != http.StatusCreated {
		t.Fatalf("submit status=%d body=%s", got.Code, got.Body.String())
	}
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders/"+clientOrderID+"/simulated-fill", nil); got.Code != http.StatusOK {
		t.Fatalf("fill status=%d body=%s", got.Code, got.Body.String())
	}
	return assertPaperAuditValid(t)
}

func TestVerifyAuditEventsWithEvidenceRejectsPaperLedgerCorruption(t *testing.T) {
	t.Run("partial fill remaining", func(t *testing.T) {
		router, _ := setupPaperOrderTestRouter(t)
		if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("audit-partial")); got.Code != http.StatusCreated {
			t.Fatalf("submit status=%d body=%s", got.Code, got.Body.String())
		}
		if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders/audit-partial/simulated-fill", map[string]any{"fillId": "audit-partial-1", "quantity": 4}); got.Code != http.StatusOK {
			t.Fatalf("partial fill status=%d body=%s", got.Code, got.Body.String())
		}
		events := assertPaperAuditValid(t)
		if err := database.DB.Model(&models.PaperOrder{}).Where("client_order_id = ?", "audit-partial").Update("remaining_qty", 5).Error; err != nil {
			t.Fatal(err)
		}
		if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, events); valid || !strings.Contains(reason, "remaining quantity mismatch") {
			t.Fatalf("tampered partial fill valid=%t reason=%q", valid, reason)
		}
	})

	t.Run("order fill aggregate", func(t *testing.T) {
		events := submitAndFillPaperOrderForAudit(t, "audit-order-aggregate")
		if err := database.DB.Model(&models.PaperOrder{}).Where("client_order_id = ?", "audit-order-aggregate").Update("fill_qty", 9).Error; err != nil {
			t.Fatal(err)
		}
		if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, events); valid || !strings.Contains(reason, "fill quantity mismatch") {
			t.Fatalf("tampered order aggregate valid=%t reason=%q", valid, reason)
		}
	})

	t.Run("position quantity", func(t *testing.T) {
		events := submitAndFillPaperOrderForAudit(t, "audit-position")
		if err := database.DB.Model(&models.PaperPosition{}).Where("currency = ? AND symbol = ?", "USD", "NVDA").Update("quantity", 11).Error; err != nil {
			t.Fatal(err)
		}
		if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, events); valid || !strings.Contains(reason, "position quantity mismatch") {
			t.Fatalf("tampered position valid=%t reason=%q", valid, reason)
		}
	})

	t.Run("position reserved quantity", func(t *testing.T) {
		events := submitAndFillPaperOrderForAudit(t, "audit-position-reserved")
		if err := database.DB.Model(&models.PaperPosition{}).Where("currency = ? AND symbol = ?", "USD", "NVDA").Update("reserved_quantity", 9).Error; err != nil {
			t.Fatal(err)
		}
		if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, events); valid || !strings.Contains(reason, "reserved quantity mismatch") {
			t.Fatalf("tampered reserved position valid=%t reason=%q", valid, reason)
		}
	})

	t.Run("account cash", func(t *testing.T) {
		events := submitAndFillPaperOrderForAudit(t, "audit-account")
		if err := database.DB.Model(&models.PaperAccount{}).Where("currency = ?", "USD").Update("cash", 9_001.0).Error; err != nil {
			t.Fatal(err)
		}
		if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, events); valid || !strings.Contains(reason, "account cash mismatch") {
			t.Fatalf("tampered account valid=%t reason=%q", valid, reason)
		}
	})

	t.Run("account reserved cash", func(t *testing.T) {
		router, _ := setupPaperOrderTestRouter(t)
		if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("audit-account-reserved")); got.Code != http.StatusCreated {
			t.Fatalf("submit status=%d body=%s", got.Code, got.Body.String())
		}
		events := assertPaperAuditValid(t)
		if err := database.DB.Model(&models.PaperAccount{}).Where("currency = ?", "USD").Update("reserved_cash", 1_000.0).Error; err != nil {
			t.Fatal(err)
		}
		if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, events); valid || !strings.Contains(reason, "reserved cash mismatch") {
			t.Fatalf("tampered reserved cash valid=%t reason=%q", valid, reason)
		}
	})

	t.Run("order realized pnl", func(t *testing.T) {
		events := submitAndFillPaperOrderForAudit(t, "audit-pnl")
		if err := database.DB.Model(&models.PaperOrder{}).Where("client_order_id = ?", "audit-pnl").Update("realized_pn_l", 1.0).Error; err != nil {
			t.Fatal(err)
		}
		if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, events); valid || !strings.Contains(reason, "realized PnL mismatch") {
			t.Fatalf("tampered PnL valid=%t reason=%q", valid, reason)
		}
	})
}

func TestVerifyAuditEventsWithEvidenceRejectsFilledOCOSibling(t *testing.T) {
	router, fixture := setupPaperOrderTestRouter(t)
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders", validPaperOrderPayload("audit-oco-parent")); got.Code != http.StatusCreated {
		t.Fatalf("submit status=%d body=%s", got.Code, got.Body.String())
	}
	parent := decodePaperOrderResponse(t, paperRequest(t, router, http.MethodPost, "/api/paper-orders/audit-oco-parent/simulated-fill", nil)).Order
	var stop, sibling models.PaperOrder
	if err := database.DB.Where("parent_order_id = ? AND order_type = ?", parent.ID, "STOP_MARKET").First(&stop).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Where("parent_order_id = ? AND order_type = ?", parent.ID, "LIMIT").First(&sibling).Error; err != nil {
		t.Fatal(err)
	}
	fixture.setPrice("NVDA", 97)
	if got := paperRequest(t, router, http.MethodPost, "/api/paper-orders/"+stop.ClientOrderID+"/simulated-fill", nil); got.Code != http.StatusOK {
		t.Fatalf("stop fill status=%d body=%s", got.Code, got.Body.String())
	}
	events := assertPaperAuditValid(t)

	var stopFill models.PaperFill
	if err := database.DB.Where("paper_order_id = ?", stop.ID).First(&stopFill).Error; err != nil {
		t.Fatal(err)
	}
	forgedFill := stopFill
	forgedFill.ID = 0
	forgedFill.FillID = "forged-filled-oco-sibling"
	forgedFill.PaperOrderID = sibling.ID
	if err := database.DB.Create(&forgedFill).Error; err != nil {
		t.Fatal(err)
	}
	now := stopFill.CreatedAt
	sibling.Status = paperStatusSimulatedFill
	sibling.StatusHistory = append(sibling.StatusHistory, models.PaperOrderStatusEvent{Status: paperStatusSimulatedFill, At: now})
	sibling.FillQty = sibling.Quantity
	sibling.RemainingQty = 0
	sibling.FillPrice = stopFill.Price
	sibling.Slippage = stopFill.Slippage
	sibling.Fee = stopFill.Fee
	sibling.RealizedPnL = stopFill.RealizedPnL
	sibling.FilledAt = &now
	sibling.FillModel = "forged"
	sibling.FillQuote = &stopFill.Quote
	if err := database.DB.Model(&models.PaperOrder{}).Where("id = ?", sibling.ID).
		Select("Status", "StatusHistory", "FillQty", "RemainingQty", "FillPrice", "Slippage", "Fee", "RealizedPnL", "FilledAt", "FillModel", "FillQuote").Updates(&sibling).Error; err != nil {
		t.Fatal(err)
	}
	if valid, _, reason, _ := verifyAuditEventsWithEvidence(database.DB, events); valid || !strings.Contains(reason, "multiple filled siblings") {
		t.Fatalf("filled OCO siblings valid=%t reason=%q", valid, reason)
	}
}

func TestPaperLedgerIntegrityRequiresAuditEvidenceForEveryFill(t *testing.T) {
	submitAndFillPaperOrderForAudit(t, "audit-missing-fill-evidence")
	if reason := verifyPaperLedgerIntegrity(database.DB, map[string]struct{}{}); !strings.Contains(reason, "evidence is missing from audit chain") {
		t.Fatalf("unaudited fill was not rejected: %q", reason)
	}
}
