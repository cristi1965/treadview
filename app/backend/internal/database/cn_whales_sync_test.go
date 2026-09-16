package database

import (
	"errors"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"trading-agents/internal/models"
)

func setupCNWhalesSyncTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cn-whales.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Guru{}, &models.Holding{}); err != nil {
		t.Fatal(err)
	}
	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous })
	return db
}

func TestReplaceGuruHoldingsRejectsEmptyReplacement(t *testing.T) {
	db := setupCNWhalesSyncTestDB(t)
	guru := models.Guru{Name: "Existing", Slug: "existing", Type: "fund"}
	if err := db.Create(&guru).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Holding{GuruID: guru.ID, StockSymbol: "KEEP"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := replaceGuruHoldings(db, &guru, nil); err == nil {
		t.Fatal("empty replacement was accepted")
	}
	var count int64
	if err := db.Model(&models.Holding{}).Where("guru_id = ? AND stock_symbol = ?", guru.ID, "KEEP").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("prior holding was not retained: count=%d err=%v", count, err)
	}
}

func TestReplaceGuruHoldingsRollsBackDeleteAndMetadataWhenInsertFails(t *testing.T) {
	db := setupCNWhalesSyncTestDB(t)
	guru := models.Guru{Name: "Existing", Slug: "existing-rollback", Type: "fund", TopStock: "KEEP", PositionCount: 1}
	if err := db.Create(&guru).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Holding{GuruID: guru.ID, StockSymbol: "KEEP"}).Error; err != nil {
		t.Fatal(err)
	}
	callbackName := "test:reject-holding-insert"
	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "holdings" {
			tx.AddError(errors.New("forced insert failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callbackName) })

	guru.TopStock = "NEW"
	guru.PositionCount = 2
	err := replaceGuruHoldings(db, &guru, []models.Holding{{StockSymbol: "NEW"}})
	if err == nil {
		t.Fatal("forced insert failure was ignored")
	}
	var stored models.Guru
	if err := db.First(&stored, guru.ID).Error; err != nil {
		t.Fatal(err)
	}
	var rows []models.Holding
	if err := db.Where("guru_id = ?", guru.ID).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if stored.TopStock != "KEEP" || stored.PositionCount != 1 || len(rows) != 1 || rows[0].StockSymbol != "KEEP" {
		t.Fatalf("transaction did not preserve prior state: guru=%+v holdings=%+v", stored, rows)
	}
}
