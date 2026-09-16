package database

import (
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"trading-agents/internal/models"
)

type legacyAuditEvent struct {
	ID         uint      `gorm:"primaryKey"`
	CreatedAt  time.Time `gorm:"index;not null"`
	RequestID  string    `gorm:"size:80;index;not null"`
	Actor      string    `gorm:"size:40;not null"`
	Action     string    `gorm:"size:80;index;not null"`
	Target     string    `gorm:"size:240;not null"`
	Result     string    `gorm:"size:32;index;not null"`
	HTTPStatus int
	PrevHash   string `gorm:"size:64"`
	Hash       string `gorm:"size:64;uniqueIndex;not null"`
}

func (legacyAuditEvent) TableName() string { return "audit_events" }

func TestAuditEventMigrationPreservesLegacyDuplicatesAndScopesNewUniqueness(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "audit-migration.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close audit migration database: %v", err)
		}
	})
	if err := db.AutoMigrate(&legacyAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	legacy := []legacyAuditEvent{
		{CreatedAt: time.Now(), RequestID: "legacy-reused", Actor: "admin", Action: "write", Target: "/write", Result: "AUTHORIZED", Hash: "legacy-hash-1"},
		{CreatedAt: time.Now(), RequestID: "legacy-reused", Actor: "admin", Action: "write", Target: "/write", Result: "ALLOWED", PrevHash: "legacy-hash-1", Hash: "legacy-hash-2"},
	}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.AuditEvent{}); err != nil {
		t.Fatalf("audit migration failed with legacy duplicate request IDs: %v", err)
	}

	var migrated []models.AuditEvent
	if err := db.Order("id asc").Find(&migrated).Error; err != nil {
		t.Fatal(err)
	}
	if len(migrated) != 2 || migrated[0].HashVersion != 1 || migrated[1].HashVersion != 1 {
		t.Fatalf("legacy rows were not preserved as hash version 1: %+v", migrated)
	}

	first := models.AuditEvent{
		CreatedAt: time.Now(), RequestID: "v2-unique", Actor: "admin", Action: "write", Target: "/write",
		Status: "PENDING", Result: "PENDING", HashVersion: 2, Hash: "v2-hash-1",
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("first v2 audit outcome failed: %v", err)
	}
	duplicate := first
	duplicate.ID = 0
	duplicate.Hash = "v2-hash-2"
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("v2 audit request ID uniqueness was not enforced")
	}
}
