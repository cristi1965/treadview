package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAdminTokenUsesEnvironmentBeforeFile(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "admin-token")
	if err := os.WriteFile(tokenFile, []byte("file-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STOCKGOD_ADMIN_TOKEN_FILE", tokenFile)
	t.Setenv("STOCKGOD_ADMIN_TOKEN", "environment-token")
	if got := loadAdminToken(); got != "environment-token" {
		t.Fatalf("admin token precedence mismatch: %q", got)
	}
}

func TestLoadAdminTokenReadsConfiguredFileAndRejectsEmptyFile(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "admin-token")
	t.Setenv("STOCKGOD_ADMIN_TOKEN", "")
	t.Setenv("STOCKGOD_ADMIN_TOKEN_FILE", tokenFile)
	if err := os.WriteFile(tokenFile, []byte("  file-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := loadAdminToken(); got != "file-token" {
		t.Fatalf("configured admin token file was not loaded: %q", got)
	}
	if err := os.WriteFile(tokenFile, []byte(" \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := loadAdminToken(); got != "" {
		t.Fatalf("empty admin token file enabled writes: %q", got)
	}
}
