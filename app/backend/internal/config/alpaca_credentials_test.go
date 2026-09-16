package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAlpacaCredentialsFromProtectedFile(t *testing.T) {
	t.Setenv("ALPACA_API_KEY_ID", "")
	t.Setenv("ALPACA_API_SECRET_KEY", "")
	path := filepath.Join(t.TempDir(), "alpaca.env")
	if err := os.WriteFile(path, []byte("ALPACA_API_KEY_ID=file-id\nALPACA_API_SECRET_KEY=file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STOCKGOD_ALPACA_CREDENTIALS_FILE", path)
	got, err := LoadAlpacaCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKeyID != "file-id" || got.APISecretKey != "file-secret" {
		t.Fatalf("unexpected credentials: %#v", got)
	}
}

func TestLoadAlpacaCredentialsEnvironmentOverridesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alpaca.env")
	if err := os.WriteFile(path, []byte("ALPACA_API_KEY_ID=file-id\nALPACA_API_SECRET_KEY=file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STOCKGOD_ALPACA_CREDENTIALS_FILE", path)
	t.Setenv("ALPACA_API_KEY_ID", "env-id")
	t.Setenv("ALPACA_API_SECRET_KEY", "env-secret")
	got, err := LoadAlpacaCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKeyID != "env-id" || got.APISecretKey != "env-secret" {
		t.Fatalf("environment did not win: %#v", got)
	}
}

func TestLoadAlpacaCredentialsRejectsPartialEnvironmentPair(t *testing.T) {
	t.Setenv("ALPACA_API_KEY_ID", "env-id")
	t.Setenv("ALPACA_API_SECRET_KEY", "")
	if _, err := LoadAlpacaCredentials(); err == nil {
		t.Fatal("expected partial process credentials to be rejected")
	}
}

func TestLoadAlpacaCredentialsRejectsPartialFilePair(t *testing.T) {
	t.Setenv("ALPACA_API_KEY_ID", "")
	t.Setenv("ALPACA_API_SECRET_KEY", "")
	path := filepath.Join(t.TempDir(), "alpaca.env")
	if err := os.WriteFile(path, []byte("ALPACA_API_KEY_ID=file-id\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STOCKGOD_ALPACA_CREDENTIALS_FILE", path)
	if _, err := LoadAlpacaCredentials(); err == nil {
		t.Fatal("expected partial file credentials to be rejected")
	}
}

func TestLoadAlpacaCredentialsRejectsLoosePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alpaca.env")
	if err := os.WriteFile(path, []byte("ALPACA_API_KEY_ID=id\nALPACA_API_SECRET_KEY=secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STOCKGOD_ALPACA_CREDENTIALS_FILE", path)
	if _, err := LoadAlpacaCredentials(); err == nil {
		t.Fatal("expected loose permissions to be rejected")
	}
}
