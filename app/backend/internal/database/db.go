package database

import (
	"log"
	"path/filepath"
	"trading-agents/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB(dbDir string) {
	dbPath := filepath.Join(dbDir, "trades.db")
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to local SQLite database: %v", err)
	}

	// Auto-migrate tables
	err = DB.AutoMigrate(&models.Trade{}, &models.Event{}, &models.Guru{}, &models.Holding{}, &models.CongressTrade{})
	if err != nil {
		log.Fatalf("Database auto-migration failed: %v", err)
	}
	log.Printf("SQLite database successfully initialized at: %s", dbPath)

	// Seed Whales data
	SeedWhalesData(DB)

	// Seed default macroeconomic events if table is empty
	var count int64
	DB.Model(&models.Event{}).Count(&count)
	if count == 0 {
		defaultEvents := []models.Event{
			{Date: "2026-07-02", Time: "02:00", Name: "美联储 FOMC 利率决议", Stars: 3, Previous: "5.25%-5.50%", Consensus: "5.00%-5.25%"},
			{Date: "2026-07-03", Time: "20:30", Name: "美国 6 月非农就业人数 (NFP)", Stars: 3, Previous: "27.2万", Consensus: "18.5万"},
			{Date: "2026-07-03", Time: "20:30", Name: "美国 6 月失业率", Stars: 2, Previous: "4.0%", Consensus: "4.0%"},
			{Date: "2026-07-08", Time: "20:30", Name: "美国 6 月核心 CPI 年率", Stars: 3, Previous: "3.4%", Consensus: "3.2%"},
		}
		for _, event := range defaultEvents {
			DB.Create(&event)
		}
		log.Println("Seeded default macroeconomic events into database.")
	}
}
