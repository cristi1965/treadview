package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitDBCreatesIsolatedDirectoryAndAuditTable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "database")
	previous := DB
	InitDB(dir)
	created := DB
	t.Cleanup(func() {
		if created != nil {
			if sqlDB, err := created.DB(); err == nil {
				if err := sqlDB.Close(); err != nil {
					t.Errorf("close initialized database: %v", err)
				}
			}
		}
		DB = previous
	})
	if _, err := os.Stat(filepath.Join(dir, "trades.db")); err != nil {
		t.Fatalf("database file not created in configured directory: %v", err)
	}
	if !DB.Migrator().HasTable("audit_events") {
		t.Fatal("audit_events table was not migrated")
	}
	if !DB.Migrator().HasTable("gpu_price_observations") {
		t.Fatal("gpu_price_observations table was not migrated")
	}
}
