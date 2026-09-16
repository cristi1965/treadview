package api

import (
	"testing"

	"trading-agents/internal/database"
	"trading-agents/internal/models"
)

func TestPaperFillQuoteMigrationPreservesLegacyOrders(t *testing.T) {
	db, err := database.OpenSQLite(t.TempDir() + "/legacy-paper-order.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.PaperOrder{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropColumn(&models.PaperOrder{}, "FillQuote"); err != nil {
		t.Fatal(err)
	}
	const legacyID = "legacy-fill-quote"
	if err := db.Exec(`INSERT INTO paper_orders
		(client_order_id, environment, symbol, side, market, currency, order_type, time_in_force,
		 quote_source, quote_time, status, request_hash, version)
		VALUES (?, 'PAPER', 'NVDA', 'BUY', 'US', 'USD', 'LIMIT', 'GTC', 'legacy-source',
		 CURRENT_TIMESTAMP, 'ACCEPTED', 'legacy-hash', 1)`, legacyID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.PaperOrder{}); err != nil {
		t.Fatalf("additive PaperOrder migration failed: %v", err)
	}
	if !db.Migrator().HasColumn(&models.PaperOrder{}, "FillQuote") {
		t.Fatal("fill_quote column was not added")
	}
	var migrated models.PaperOrder
	if err := db.Where("client_order_id = ?", legacyID).First(&migrated).Error; err != nil {
		t.Fatalf("legacy order was not preserved: %v", err)
	}
	if migrated.FillQuote != nil {
		t.Fatalf("legacy order should have no synthetic fill evidence: %+v", migrated.FillQuote)
	}
}
