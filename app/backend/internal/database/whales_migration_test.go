package database

import (
	"encoding/xml"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"trading-agents/internal/models"
)

type legacyGuruSchema struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"size:100;not null"`
	Slug string `gorm:"size:100;uniqueIndex"`
}

type legacyCongressTradeSchema struct {
	ID         uint   `gorm:"primaryKey"`
	Politician string `gorm:"size:100;not null"`
	Party      string `gorm:"size:50;not null"`
	Symbol     string `gorm:"size:20;not null"`
	Type       string `gorm:"size:20;not null"`
	Date       string `gorm:"size:20;not null"`
}

func (legacyCongressTradeSchema) TableName() string { return "congress_trades" }

func (legacyGuruSchema) TableName() string { return "gurus" }

func openWhalesMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "whales.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close whales migration database: %v", err)
		}
	})
	return db
}

func TestCongressTradeProvenanceAutoMigratesLegacyRowsAsUnknown(t *testing.T) {
	db := openWhalesMigrationTestDB(t)
	if err := db.AutoMigrate(&legacyCongressTradeSchema{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&legacyCongressTradeSchema{Politician: "Legacy", Party: "Unknown", Symbol: "TEST", Type: "BUY", Date: "2020-01-01"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.CongressTrade{}); err != nil {
		t.Fatal(err)
	}
	var trade models.CongressTrade
	if err := db.First(&trade).Error; err != nil {
		t.Fatal(err)
	}
	if trade.Source != "unknown" || trade.FilingDate != "unknown" || trade.SourceURL != "unknown" || trade.FilingID != "unknown" {
		t.Fatalf("legacy provenance must remain explicitly unknown: %+v", trade)
	}
}

func TestGuruDisclosureFieldsAutoMigrateLegacyTable(t *testing.T) {
	db := openWhalesMigrationTestDB(t)
	if err := db.AutoMigrate(&legacyGuruSchema{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&legacyGuruSchema{Name: "Legacy", Slug: "legacy"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Guru{}, &models.Holding{}, &models.GuruFiling{}); err != nil {
		t.Fatal(err)
	}

	migrator := db.Migrator()
	for _, field := range []string{"report_period", "filing_date", "accession", "source", "source_as_of", "source_url", "synced_at"} {
		if !migrator.HasColumn("gurus", field) {
			t.Fatalf("missing migrated column %q", field)
		}
	}
	if !migrator.HasColumn("holdings", "report_period") || !migrator.HasTable("guru_filings") {
		t.Fatal("holding period or filing-history table was not migrated")
	}

	var migrated models.Guru
	if err := db.Where("slug = ?", "legacy").First(&migrated).Error; err != nil {
		t.Fatal(err)
	}
	if migrated.Name != "Legacy" {
		t.Fatalf("legacy row changed during migration: %+v", migrated)
	}
}

func TestSeedWhalesDataLabelsDataromaWithoutSECFilingMetadata(t *testing.T) {
	db := openWhalesMigrationTestDB(t)
	if err := db.AutoMigrate(&models.Guru{}, &models.Holding{}, &models.GuruFiling{}, &models.CongressTrade{}); err != nil {
		t.Fatal(err)
	}

	SeedWhalesData(db)

	var guru models.Guru
	if err := db.Where("source = ?", "dataroma").First(&guru).Error; err != nil {
		t.Fatal(err)
	}
	if guru.SourceAsOf == "" || guru.SourceURL != "https://www.dataroma.com/" {
		t.Fatalf("bootstrap provenance missing: %+v", guru)
	}
	if guru.ReportPeriod != "" || guru.FilingDate != "" || guru.Accession != "" {
		t.Fatalf("bootstrap snapshot fabricated SEC filing metadata: %+v", guru)
	}
}

func TestSaveEDGARDisclosureCommitsProvenanceAndHoldingsTogether(t *testing.T) {
	db := openWhalesMigrationTestDB(t)
	if err := db.AutoMigrate(&models.Guru{}, &models.Holding{}, &models.GuruFiling{}); err != nil {
		t.Fatal(err)
	}

	var table InformationTable
	fixture := `<informationTable><infoTable><nameOfIssuer>APPLE INC</nameOfIssuer><cusip>037833100</cusip><value>1000</value><shrsOrPrnAmt><sshPrnamt>10</sshPrnamt><sshPrnamtType>SH</sshPrnamtType></shrsOrPrnAmt></infoTable></informationTable>`
	if err := xml.Unmarshal([]byte(fixture), &table); err != nil {
		t.Fatal(err)
	}
	entry := SuperinvestorEntry{CIK: "0001067983", DBName: "Test Manager", FundName: "Test Fund", NameEn: "Test Manager"}
	rows := []models.Holding{{StockSymbol: "AAPL", StockName: "Apple Inc", Shares: "10", Weight: 100}}
	sourceURL := "https://www.sec.gov/Archives/edgar/data/1067983/example.xml"
	guru, err := saveEDGARDisclosure(db, "test-manager", entry, table.InfoTable, []float64{1000}, 1000, "0001067983-26-000001", "2026-05-15", "2026-03-31", sourceURL, rows)
	if err != nil {
		t.Fatal(err)
	}
	if guru.ReportPeriod != "2026-03-31" || guru.FilingDate != "2026-05-15" || guru.Accession != "0001067983-26-000001" || guru.SourceURL != sourceURL {
		t.Fatalf("SEC provenance not persisted: %+v", guru)
	}
	var filing models.GuruFiling
	if err := db.Where("guru_id = ? AND report_period = ?", guru.ID, "2026-03-31").First(&filing).Error; err != nil {
		t.Fatal(err)
	}
	if filing.Source != "sec-edgar-13f" || filing.SourceURL != sourceURL {
		t.Fatalf("filing provenance mismatch: %+v", filing)
	}
	var holdingCount int64
	db.Model(&models.Holding{}).Where("guru_id = ? AND report_period = ?", guru.ID, "2026-03-31").Count(&holdingCount)
	if holdingCount != 1 {
		t.Fatalf("holding count=%d", holdingCount)
	}

	if err := db.Exec(`CREATE TRIGGER fail_new_period BEFORE INSERT ON holdings WHEN NEW.report_period = '2026-06-30' BEGIN SELECT RAISE(FAIL, 'forced holding failure'); END;`).Error; err != nil {
		t.Fatal(err)
	}
	_, err = saveEDGARDisclosure(db, "test-manager", entry, table.InfoTable, []float64{2000}, 2000, "0001067983-26-000002", "2026-08-14", "2026-06-30", "https://www.sec.gov/Archives/edgar/data/1067983/failed.xml", rows)
	if err == nil {
		t.Fatal("expected forced holding failure")
	}
	var after models.Guru
	if err := db.First(&after, guru.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.ReportPeriod != "2026-03-31" || after.Accession != "0001067983-26-000001" || after.SourceURL != sourceURL {
		t.Fatalf("guru metadata escaped failed transaction: %+v", after)
	}
	var failedFilings, failedHoldings int64
	db.Model(&models.GuruFiling{}).Where("guru_id = ? AND report_period = ?", guru.ID, "2026-06-30").Count(&failedFilings)
	db.Model(&models.Holding{}).Where("guru_id = ? AND report_period = ?", guru.ID, "2026-06-30").Count(&failedHoldings)
	if failedFilings != 0 || failedHoldings != 0 {
		t.Fatalf("failed period leaked: filings=%d holdings=%d", failedFilings, failedHoldings)
	}
}

func TestBackfillWhaleDisclosureHistoryPreservesCurrentPeriod(t *testing.T) {
	db := openWhalesMigrationTestDB(t)
	if err := db.AutoMigrate(&models.Guru{}, &models.Holding{}, &models.GuruFiling{}); err != nil {
		t.Fatal(err)
	}
	guru := models.Guru{
		Name: "Existing", Slug: "existing", Type: "superinvestor", ReportPeriod: "2026-03-31",
		FilingDate: "2026-05-15", Accession: "old", Source: "sec-edgar-13f",
	}
	if err := db.Create(&guru).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Holding{GuruID: guru.ID, StockSymbol: "OLD", Weight: 50}).Error; err != nil {
		t.Fatal(err)
	}

	backfillWhaleDisclosureHistory(db)

	var holding models.Holding
	if err := db.Where("guru_id = ?", guru.ID).First(&holding).Error; err != nil {
		t.Fatal(err)
	}
	if holding.ReportPeriod != guru.ReportPeriod {
		t.Fatalf("holding report period=%q", holding.ReportPeriod)
	}
	var filing models.GuruFiling
	if err := db.Where("guru_id = ? AND report_period = ?", guru.ID, guru.ReportPeriod).First(&filing).Error; err != nil {
		t.Fatal(err)
	}
	if filing.Accession != "old" {
		t.Fatalf("filing not preserved: %+v", filing)
	}
}
