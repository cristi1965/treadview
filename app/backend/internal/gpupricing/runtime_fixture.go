package gpupricing

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"trading-agents/internal/config"
)

const (
	RuntimeFixtureEnvironment     = "STOCKGOD_GPU_RUNTIME_FIXTURE"
	RuntimeFixtureAcknowledgement = "temporary-local-acceptance-v1"
	RuntimeFixtureDataMode        = "acceptance-fixture"
)

// ConfigureRuntimeFixtureFromEnvironment validates the deliberately narrow
// acceptance-only mode before the process opens a database or starts workers.
func ConfigureRuntimeFixtureFromEnvironment(cfg *config.Config) (bool, error) {
	if strings.TrimSpace(os.Getenv(RuntimeFixtureEnvironment)) != RuntimeFixtureAcknowledgement {
		return false, nil
	}
	if cfg == nil || strings.TrimSpace(cfg.AdminToken) == "" {
		return false, errors.New("GPU runtime fixture requires a non-empty admin token")
	}
	if !databaseDirIsTemporary(cfg.DatabaseDir) {
		return false, errors.New("GPU runtime fixture requires STOCKGOD_DATABASE_DIR below the system temporary directory")
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("STOCKGOD_REPLAY")), "true") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("STOCKGOD_LIVE_MIRROR")), "true") {
		return false, errors.New("GPU runtime fixture cannot run in replay or live-mirror mode")
	}
	return true, nil
}

func runtimeFixtureProviders(cfg *config.Config) []Provider {
	enabled, _ := ConfigureRuntimeFixtureFromEnvironment(cfg)
	if !enabled {
		return nil
	}
	return []Provider{
		&runtimeFixtureProvider{name: ProviderRunPod, model: "H100 SXM", price: 3.49},
		&runtimeFixtureProvider{name: ProviderModal, model: "H100 SXM", price: 3.95},
		&runtimeFixtureProvider{name: ProviderLambda, model: "H100 SXM", price: 4.29},
		&runtimeFixtureProvider{name: ProviderVast, model: "H100 SXM", price: 2.30},
	}
}

func databaseDirIsTemporary(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		return false
	}
	absPath, err = filepath.EvalSymlinks(absPath)
	if err != nil {
		return false
	}
	absTemp, err := filepath.Abs(os.TempDir())
	if err != nil {
		return false
	}
	absTemp, err = filepath.EvalSymlinks(absTemp)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(absTemp, absPath)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

type runtimeFixtureProvider struct {
	name  string
	model string
	price float64
}

func (p *runtimeFixtureProvider) Name() string     { return p.name }
func (p *runtimeFixtureProvider) Configured() bool { return true }
func (p *runtimeFixtureProvider) Fetch(_ context.Context, _ time.Time) ([]Quote, error) {
	memory := 80.0
	return []Quote{{
		Provider: p.name, GPUModel: p.model, Product: "acceptance-fixture", BillingMode: "on-demand",
		GPUCount: 1, MemoryGiB: &memory, Region: "test-local", OfferID: p.name + "-h100",
		Availability: "fixture-available", Currency: "USD", RawPrice: p.price,
		RawUnit: "USD/GPU-hour", PriceUSDPerGPUHour: p.price,
		SourceURL:      "https://fixture.invalid/" + p.name,
		SourceIdentity: "runtime-fixture:" + p.name + ":h100",
	}}, nil
}
