package gpupricing

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"trading-agents/internal/config"
)

func TestConfigureRuntimeFixtureRequiresExplicitSafeTemporaryRuntime(t *testing.T) {
	t.Setenv(RuntimeFixtureEnvironment, RuntimeFixtureAcknowledgement)
	t.Setenv("STOCKGOD_REPLAY", "false")
	t.Setenv("STOCKGOD_LIVE_MIRROR", "false")

	validDatabase := filepath.Join(t.TempDir(), "database")
	if err := os.Mkdir(validDatabase, 0o700); err != nil {
		t.Fatal(err)
	}
	valid := &config.Config{AdminToken: "test-admin", DatabaseDir: validDatabase}
	if enabled, err := ConfigureRuntimeFixtureFromEnvironment(valid); err != nil || !enabled {
		t.Fatalf("valid fixture enabled=%v err=%v", enabled, err)
	}

	for name, mutate := range map[string]func(*config.Config){
		"missing admin token":    func(cfg *config.Config) { cfg.AdminToken = "" },
		"non-temporary database": func(cfg *config.Config) { cfg.DatabaseDir = filepath.Join("testdata", "database") },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := *valid
			mutate(&candidate)
			if enabled, err := ConfigureRuntimeFixtureFromEnvironment(&candidate); err == nil || enabled {
				t.Fatalf("unsafe fixture enabled=%v err=%v", enabled, err)
			}
		})
	}

	t.Run("replay", func(t *testing.T) {
		t.Setenv("STOCKGOD_REPLAY", "true")
		if enabled, err := ConfigureRuntimeFixtureFromEnvironment(valid); err == nil || enabled {
			t.Fatalf("replay fixture enabled=%v err=%v", enabled, err)
		}
	})
	t.Run("live mirror", func(t *testing.T) {
		t.Setenv("STOCKGOD_LIVE_MIRROR", "true")
		if enabled, err := ConfigureRuntimeFixtureFromEnvironment(valid); err == nil || enabled {
			t.Fatalf("live mirror fixture enabled=%v err=%v", enabled, err)
		}
	})
}

func TestRuntimeFixtureUsesRealRefreshPersistenceAndSurvivesReconfigure(t *testing.T) {
	t.Setenv(RuntimeFixtureEnvironment, RuntimeFixtureAcknowledgement)
	t.Setenv("STOCKGOD_REPLAY", "false")
	t.Setenv("STOCKGOD_LIVE_MIRROR", "false")
	databaseDir := filepath.Join(t.TempDir(), "database")
	if err := os.Mkdir(databaseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		AdminToken: "test-admin", DatabaseDir: databaseDir,
		GPUPriceRefreshInterval: time.Hour, GPUPriceHistoryRetention: 24 * time.Hour,
	}
	service := NewService(cfg, openGPUPriceTestDB(t))

	result, err := service.Refresh(context.Background())
	if err != nil || result.Updated != 4 || result.Snapshot.Status != StatusLive || result.Snapshot.Count != 4 {
		t.Fatalf("fixture refresh=%+v err=%v", result, err)
	}
	history, err := service.History(context.Background(), HistoryFilter{Provider: ProviderRunPod, GPUModel: "H100 SXM", Days: 1, Limit: 10})
	if err != nil || history.Count != 1 || history.Items[0].SourceIdentity != "runtime-fixture:runpod:h100" {
		t.Fatalf("fixture history=%+v err=%v", history, err)
	}

	next := *cfg
	if err := service.Reconfigure(&next); err != nil {
		t.Fatal(err)
	}
	snapshot := service.Current()
	if len(snapshot.Providers) != 4 {
		t.Fatalf("reconfigured providers=%+v", snapshot.Providers)
	}
	for _, provider := range snapshot.Providers {
		if !provider.Configured {
			t.Fatalf("fixture provider lost during reconfigure: %+v", provider)
		}
	}
	if refreshed, err := service.Refresh(context.Background()); err != nil || refreshed.Snapshot.Status != StatusLive || refreshed.Snapshot.Count != 4 {
		t.Fatalf("post-reconfigure refresh=%+v err=%v", refreshed, err)
	}
}
