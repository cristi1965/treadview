package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadGPUProviderCredentialsFromEnvironment(t *testing.T) {
	t.Setenv("RUNPOD_API_KEY", "runpod")
	t.Setenv("MODAL_TOKEN_ID", "modal-id")
	t.Setenv("MODAL_TOKEN_SECRET", "modal-secret")
	t.Setenv("LAMBDA_API_KEY", "lambda")
	t.Setenv("VAST_API_KEY", "vast")
	cfg := Load()
	if cfg.RunPodAPIKey != "runpod" || cfg.ModalTokenID != "modal-id" || cfg.ModalTokenSecret != "modal-secret" || cfg.LambdaAPIKey != "lambda" || cfg.VastAPIKey != "vast" {
		t.Fatalf("GPU provider credentials were not loaded")
	}
}

func TestLoadGPUProviderCredentialsIgnoresRepositoryDotEnv(t *testing.T) {
	clearGPUCredentialEnvironment(t)
	root := t.TempDir()
	t.Setenv("STOCKGOD_GPU_CREDENTIALS_FILE", filepath.Join(root, "missing-gpu-credentials.json"))
	backend := filepath.Join(root, "app", "backend")
	if err := os.MkdirAll(backend, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("RUNPOD_API_KEY=dotenv-must-not-load\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(backend)
	cfg := Load()
	if cfg.RunPodAPIKey != "" {
		t.Fatalf("repository .env leaked into GPU credentials: %q", cfg.RunPodAPIKey)
	}
}

func TestReloadGPUProviderCredentialsUsesProtectedExternalFileAndEnvironmentOverrides(t *testing.T) {
	clearGPUCredentialEnvironment(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "gpu-provider-credentials.json")
	contents := `{"runpod_api_key":"file-runpod","modal_token_id":"file-modal-id","modal_token_secret":"file-modal-secret","lambda_api_key":"file-lambda","vast_api_key":"file-vast"}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RUNPOD_API_KEY", "env-runpod")
	cfg := &Config{GPUCredentialsFile: path}
	if err := ReloadGPUProviderCredentials(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.RunPodAPIKey != "env-runpod" || cfg.ModalTokenID != "file-modal-id" || cfg.ModalTokenSecret != "file-modal-secret" || cfg.LambdaAPIKey != "file-lambda" || cfg.VastAPIKey != "file-vast" {
		t.Fatalf("unexpected resolved credentials: runpod=%q modal-id=%q lambda=%q vast=%q", cfg.RunPodAPIKey, cfg.ModalTokenID, cfg.LambdaAPIKey, cfg.VastAPIKey)
	}
}

func TestReloadGPUProviderCredentialsRejectsUnsafeFilesWithoutLeakingContents(t *testing.T) {
	clearGPUCredentialEnvironment(t)
	for _, test := range []struct {
		name string
		mode os.FileMode
		body string
	}{
		{name: "permissions", mode: 0o644, body: `{"runpod_api_key":"permission-secret"}`},
		{name: "schema", mode: 0o600, body: `{"runpod_api_key":"schema-secret","unexpected":true}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "credentials.json")
			if err := os.WriteFile(path, []byte(test.body), test.mode); err != nil {
				t.Fatal(err)
			}
			cfg := &Config{GPUCredentialsFile: path, RunPodAPIKey: "old-secret"}
			err := ReloadGPUProviderCredentials(cfg)
			if err == nil || strings.Contains(err.Error(), "secret") || cfg.RunPodAPIKey != "" {
				t.Fatalf("err=%v runpod=%q", err, cfg.RunPodAPIKey)
			}
		})
	}
}

func TestReloadGPUProviderCredentialsRejectsOversizedFile(t *testing.T) {
	clearGPUCredentialEnvironment(t)
	path := filepath.Join(t.TempDir(), "credentials.json")
	if err := os.WriteFile(path, []byte(`{"runpod_api_key":"`+strings.Repeat("x", maxGPUCredentialsFileSize)+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ReloadGPUProviderCredentials(&Config{GPUCredentialsFile: path}); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("oversized file err=%v", err)
	}
}

func TestReloadGPUProviderCredentialsRejectsRepositoryAndSymlinkPaths(t *testing.T) {
	clearGPUCredentialEnvironment(t)
	repositoryFile := filepath.Join("testdata", "gpu-provider-credentials.json")
	if err := os.MkdirAll(filepath.Dir(repositoryFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repositoryFile, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(repositoryFile) })
	if err := ReloadGPUProviderCredentials(&Config{GPUCredentialsFile: repositoryFile}); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("repository path err=%v", err)
	}

	external := filepath.Join(t.TempDir(), "credentials.json")
	link := filepath.Join(t.TempDir(), "credentials-link.json")
	if err := os.WriteFile(external, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, link); err != nil {
		t.Fatal(err)
	}
	if err := ReloadGPUProviderCredentials(&Config{GPUCredentialsFile: link}); err == nil || !strings.Contains(err.Error(), "non-symlink") {
		t.Fatalf("symlink err=%v", err)
	}
}

func TestLoadGPUOperationalDurations(t *testing.T) {
	clearGPUCredentialEnvironment(t)
	t.Setenv("STOCKGOD_GPU_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("STOCKGOD_GPU_REFRESH_INTERVAL", "45m")
	t.Setenv("STOCKGOD_GPU_HISTORY_RETENTION", "120h")
	cfg := Load()
	if cfg.GPUPriceRefreshInterval != 45*time.Minute || cfg.GPUPriceHistoryRetention != 120*time.Hour {
		t.Fatalf("refresh=%s retention=%s", cfg.GPUPriceRefreshInterval, cfg.GPUPriceHistoryRetention)
	}
}

func clearGPUCredentialEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"RUNPOD_API_KEY", "MODAL_TOKEN_ID", "MODAL_TOKEN_SECRET", "LAMBDA_API_KEY", "VAST_API_KEY"} {
		t.Setenv(key, "")
	}
}
