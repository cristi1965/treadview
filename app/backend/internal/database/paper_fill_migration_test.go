package database

import (
	"testing"

	"trading-agents/internal/models"
)

func TestPaperFillMigrationPreservesLegacyAcceptedRemainingQuantity(t *testing.T) {
	db, err := OpenSQLite(t.TempDir() + "/paper-fill-migration.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.PaperOrder{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropColumn(&models.PaperOrder{}, "RemainingQty"); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO paper_orders
		(client_order_id, environment, symbol, side, market, currency, order_type, time_in_force,
		 quantity, fill_qty, quote_source, quote_time, status, request_hash, version)
		VALUES ('legacy-partial', 'PAPER', 'NVDA', 'BUY', 'US', 'USD', 'LIMIT', 'GTC',
		 10, 4, 'legacy-source', CURRENT_TIMESTAMP, 'ACCEPTED', 'legacy-hash', 1)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.PaperOrder{}, &models.PaperFill{}, &models.PaperDailyEquityBaseline{}); err != nil {
		t.Fatalf("additive Paper fill migration failed: %v", err)
	}
	backfillPaperOrderRemainingQuantity(db)
	var order models.PaperOrder
	if err := db.Where("client_order_id = ?", "legacy-partial").First(&order).Error; err != nil {
		t.Fatal(err)
	}
	if order.RemainingQty != 6 {
		t.Fatalf("legacy remaining quantity=%d want=6", order.RemainingQty)
	}
	if !db.Migrator().HasTable(&models.PaperFill{}) || !db.Migrator().HasTable(&models.PaperDailyEquityBaseline{}) {
		t.Fatal("Paper fill or daily baseline additive table is missing")
	}
}
