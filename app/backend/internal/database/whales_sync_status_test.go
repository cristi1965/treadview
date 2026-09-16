package database

import (
	"path/filepath"
	"testing"
	"time"

	"trading-agents/internal/models"
)

func TestRestoreWhalesSyncStatusUsesCommittedSECDisclosures(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "whales-status.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Guru{}); err != nil {
		t.Fatal(err)
	}
	latest := time.Date(2026, 8, 15, 12, 0, 0, 0, time.Local)
	rows := []models.Guru{
		{Name: "Verified", Slug: "verified", Source: "sec-edgar-13f", ReportPeriod: "2026-06-30", FilingDate: "2026-08-14", Accession: "0001", SourceURL: "https://www.sec.gov/Archives/1", SyncedAt: latest},
		{Name: "Bootstrap", Slug: "bootstrap", Source: "bootstrap", ReportPeriod: "unknown"},
		{Name: "Incomplete", Slug: "incomplete", Source: "sec-edgar-13f", ReportPeriod: "2026-06-30", SyncedAt: latest.Add(time.Hour)},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	SyncMutex.Lock()
	previousLast, previousStatus, previousCount := WhalesLastSync, WhalesSyncStatus, WhalesLastSuccessCount
	WhalesLastSync, WhalesSyncStatus, WhalesLastSuccessCount = "", "Idle", 0
	SyncMutex.Unlock()
	t.Cleanup(func() {
		SyncMutex.Lock()
		WhalesLastSync, WhalesSyncStatus, WhalesLastSuccessCount = previousLast, previousStatus, previousCount
		SyncMutex.Unlock()
	})

	RestoreWhalesSyncStatus(db)
	if WhalesLastSuccessCount != 1 || WhalesLastSync != latest.Format("2006-01-02 15:04:05") || WhalesSyncStatus != "SEC EDGAR available: 1 managers" {
		t.Fatalf("last=%q status=%q count=%d", WhalesLastSync, WhalesSyncStatus, WhalesLastSuccessCount)
	}
}
