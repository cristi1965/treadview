package database

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"trading-agents/internal/models"
)

func setupCongressSyncTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "congress.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.CongressTrade{}); err != nil {
		t.Fatal(err)
	}
	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous })
	return db
}

func writeCongressFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "congress.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadCongressLiveJSONRetainsSnapshotWhenNewPayloadHasNoValidRows(t *testing.T) {
	db := setupCongressSyncTestDB(t)
	existing := models.CongressTrade{Politician: "Existing", Party: "Independent", Symbol: "KEEP", Type: "BUY", Date: "2026-08-01"}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}

	path := writeCongressFixture(t, `{"updatedAt":"2026-09-10T00:00:00Z","source":"house-ptr","trades":[{"politician":"","symbol":"","date":""}]}`)
	if got := loadCongressLiveJSON(path); got != 0 {
		t.Fatalf("loaded=%d want=0", got)
	}
	var rows []models.CongressTrade
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Symbol != "KEEP" {
		t.Fatalf("committed snapshot was replaced: %+v", rows)
	}
}

func TestLoadCongressLiveJSONAtomicallyReplacesSnapshotWithValidatedRows(t *testing.T) {
	db := setupCongressSyncTestDB(t)
	if err := db.Create(&models.CongressTrade{Politician: "Old", Party: "Independent", Symbol: "OLD", Type: "BUY", Date: "2026-01-01"}).Error; err != nil {
		t.Fatal(err)
	}

	path := writeCongressFixture(t, `{"updatedAt":"2026-09-10T00:00:00Z","source":"house-ptr","trades":[{"politician":"Jane Doe","party":"Democratic","symbol":"nvda","type":"purchase","amount":"$1,001 - $15,000","date":"2026-09-01","filingDate":"2026-09-05","sourceURL":"https://example.test/filing","filingId":"123"}]}`)
	if got := loadCongressLiveJSON(path); got != 1 {
		t.Fatalf("loaded=%d want=1", got)
	}
	var rows []models.CongressTrade
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Symbol != "NVDA" || rows[0].Politician != "Jane Doe" {
		t.Fatalf("unexpected replacement: %+v", rows)
	}
}

func TestLoadCongressLiveJSONRollsBackWhenInsertFails(t *testing.T) {
	db := setupCongressSyncTestDB(t)
	if err := db.Create(&models.CongressTrade{Politician: "Existing", Party: "Independent", Symbol: "KEEP", Type: "BUY", Date: "2026-08-01"}).Error; err != nil {
		t.Fatal(err)
	}
	callbackName := "test:reject-congress-insert"
	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "congress_trades" {
			tx.AddError(errors.New("forced insert failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callbackName) })

	path := writeCongressFixture(t, `{"updatedAt":"2026-09-10T00:00:00Z","source":"house-ptr","trades":[{"politician":"Jane Doe","party":"Democratic","symbol":"NVDA","type":"BUY","date":"2026-09-01"}]}`)
	if got := loadCongressLiveJSON(path); got != 0 {
		t.Fatalf("loaded=%d want=0", got)
	}
	var rows []models.CongressTrade
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Symbol != "KEEP" {
		t.Fatalf("transaction did not restore prior snapshot: %+v", rows)
	}
}
