package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadUsesExplicitStockGodDatabaseDir(t *testing.T) {
	explicit := filepath.Join(t.TempDir(), "isolated-db")
	t.Setenv("STOCKGOD_DATABASE_DIR", explicit)
	cfg := Load()
	if cfg.DatabaseDir != explicit {
		t.Fatalf("DatabaseDir=%q want %q", cfg.DatabaseDir, explicit)
	}
}

func TestDefaultDatabaseDirIsOutsideWorkingDirectory(t *testing.T) {
	t.Setenv("STOCKGOD_DATABASE_DIR", "")
	dir := defaultDatabaseDir()
	if dir == "" || dir == "." || !strings.HasSuffix(dir, "TradingAgents") {
		t.Fatalf("unexpected default database directory %q", dir)
	}
	workingDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if absDir == workingDir || strings.HasPrefix(absDir, workingDir+string(filepath.Separator)) {
		t.Fatalf("default database directory must not be inside repository: %q", absDir)
	}
}
