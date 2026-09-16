package database

import (
	"log"
	"net/url"
	"os"
	"path/filepath"
	"trading-agents/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB(dbDir string) {
	if err := os.MkdirAll(dbDir, 0o700); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}
	dbPath := filepath.Join(dbDir, "trades.db")
	var err error
	DB, err = OpenSQLite(dbPath)
	if err != nil {
		log.Fatalf("Failed to connect to local SQLite database: %v", err)
	}

	// Auto-migrate tables
	err = DB.AutoMigrate(
		&models.Trade{}, &models.PaperOrder{}, &models.PaperFill{}, &models.PaperAccount{}, &models.PaperPosition{}, &models.PaperStopScanAudit{}, &models.PaperEquityCheckpoint{}, &models.PaperDailyEquityBaseline{},
		&models.AuditEvent{}, &models.Event{}, &models.Guru{}, &models.Holding{}, &models.GuruFiling{}, &models.CongressTrade{},
		&models.GPUPriceObservation{},
	)
	if err != nil {
		log.Fatalf("Database auto-migration failed: %v", err)
	}
	seedPaperAccounts(DB)
	backfillPaperOrderRemainingQuantity(DB)
	backfillWhaleDisclosureHistory(DB)
	log.Printf("SQLite database successfully initialized at: %s", dbPath)

	// Seed Whales data
	SeedWhalesData(DB)
	RestoreWhalesSyncStatus(DB)

	// Seed default macroeconomic events if table is empty
	var count int64
	DB.Model(&models.Event{}).Count(&count)
	if count == 0 {
		defaultEvents := []models.Event{
			{Date: "2026-07-02", Time: "02:00", Name: "美联储 FOMC 利率决议", Stars: 3, Previous: "5.25%-5.50%", Consensus: "5.00%-5.25%"},
			{Date: "2026-07-03", Time: "20:30", Name: "美国 6 月非农就业人数 (NFP)", Stars: 3, Previous: "27.2万", Consensus: "18.5万"},
			{Date: "2026-07-03", Time: "20:30", Name: "美国 6 月失业率", Stars: 2, Previous: "4.0%", Consensus: "4.0%"},
			{Date: "2026-07-08", Time: "20:30", Name: "美国 6 月核心 CPI 年率", Stars: 3, Previous: "3.4%", Consensus: "3.2%"},
			{Date: "2026-07-15", Time: "20:30", Name: "美国 6 月零售销售月率", Stars: 2, Previous: "0.1%", Consensus: "0.3%"},
			{Date: "2026-07-16", Time: "22:00", Name: "美国 6 月工业产出", Stars: 1, Previous: "0.4%", Consensus: "0.2%"},
			{Date: "2026-07-22", Time: "22:00", Name: "美国 7 月现房销售", Stars: 1, Previous: "403万", Consensus: "400万"},
			{Date: "2026-07-30", Time: "20:30", Name: "美国 GDP 初值 (季率)", Stars: 3, Previous: "2.0%", Consensus: "2.4%"},
			{Date: "2026-07-31", Time: "20:30", Name: "美国 6 月核心 PCE 年率", Stars: 3, Previous: "2.6%", Consensus: "2.6%"},
			{Date: "2026-08-01", Time: "22:00", Name: "ISM 制造业 PMI", Stars: 2, Previous: "49.0", Consensus: "49.5"},
			{Date: "2026-08-07", Time: "20:30", Name: "美国 7 月非农就业人数 (NFP)", Stars: 3, Previous: "18.5万", Consensus: "16.0万"},
			{Date: "2026-08-12", Time: "20:30", Name: "美国 7 月 CPI 年率", Stars: 3, Previous: "2.7%", Consensus: "2.8%"},
			{Date: "2026-08-21", Time: "20:30", Name: "美国 7 月初请失业金", Stars: 1, Previous: "22.1万", Consensus: "22.5万"},
			{Date: "2026-08-29", Time: "20:30", Name: "美国 7 月核心 PCE", Stars: 3, Previous: "2.6%", Consensus: "2.6%"},
		}
		for _, event := range defaultEvents {
			DB.Create(&event)
		}
		log.Println("Seeded default macroeconomic events into database.")
	}
}

func backfillPaperOrderRemainingQuantity(db *gorm.DB) {
	if err := db.Exec(`UPDATE paper_orders
		SET remaining_qty = CASE WHEN quantity > fill_qty THEN quantity - fill_qty ELSE 0 END
		WHERE remaining_qty = 0 AND status = 'ACCEPTED' AND quantity > fill_qty`).Error; err != nil {
		log.Fatalf("Failed to backfill Paper remaining quantity: %v", err)
	}
}

// OpenSQLite configures SQLite transactions to acquire the write reservation at
// BEGIN. This serializes Paper ledger decisions across independent processes;
// correctness must not depend on an in-process mutex.
func OpenSQLite(dbPath string) (*gorm.DB, error) {
	dsn := url.URL{Scheme: "file", Path: dbPath}
	query := dsn.Query()
	query.Set("_busy_timeout", "5000")
	query.Set("_foreign_keys", "on")
	query.Set("_journal_mode", "WAL")
	query.Set("_txlock", "immediate")
	dsn.RawQuery = query.Encode()
	return gorm.Open(sqlite.Open(dsn.String()), &gorm.Config{})
}

func seedPaperAccounts(db *gorm.DB) {
	const startingCash = 100_000.0
	for _, currency := range []string{"USD", "CNY"} {
		account := models.PaperAccount{Currency: currency}
		if err := db.Where("currency = ?", currency).FirstOrCreate(&account, models.PaperAccount{
			Currency: currency, InitialCash: startingCash, Cash: startingCash, Version: 1,
		}).Error; err != nil {
			log.Fatalf("Failed to initialize %s paper account: %v", currency, err)
		}
	}
}
