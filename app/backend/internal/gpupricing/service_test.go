package gpupricing

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

	"trading-agents/internal/config"
	"trading-agents/internal/database"
	"trading-agents/internal/models"
)

type fakeProvider struct {
	name       string
	configured bool
	mu         sync.Mutex
	quotes     []Quote
	err        error
	started    chan struct{}
	release    chan struct{}
}

func (p *fakeProvider) Name() string     { return p.name }
func (p *fakeProvider) Configured() bool { return p.configured }
func (p *fakeProvider) Fetch(ctx context.Context, _ time.Time) ([]Quote, error) {
	if p.started != nil {
		select {
		case p.started <- struct{}{}:
		default:
		}
	}
	if p.release != nil {
		select {
		case <-p.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return cloneQuotes(p.quotes), p.err
}

func openGPUPriceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "gpu-prices.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.GPUPriceObservation{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func validTestQuote(provider, model, identity string, price float64) Quote {
	return Quote{Provider: provider, GPUModel: model, Product: "cloud", BillingMode: "on-demand", GPUCount: 1,
		Currency: "USD", RawPrice: price, RawUnit: "USD/GPU-hour", PriceUSDPerGPUHour: price,
		SourceURL: "https://official.example/prices", SourceIdentity: identity}
}

func TestServiceRefreshPersistsDeduplicatesFiltersAndReportsPartial(t *testing.T) {
	db := openGPUPriceTestDB(t)
	now := time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC)
	good := &fakeProvider{name: ProviderRunPod, configured: true, quotes: []Quote{validTestQuote(ProviderRunPod, "H100", "h100:secure", 3.49)}}
	bad := &fakeProvider{name: ProviderLambda, configured: true, err: providerError{code: "rate_limited"}}
	unconfigured := &fakeProvider{name: ProviderVast}
	service := NewServiceWithProviders(db, []Provider{good, bad, unconfigured}, func() time.Time { return now })

	result, err := service.Refresh(context.Background())
	if err != nil || result.Updated != 1 || result.Snapshot.Status != StatusPartial || result.Snapshot.Count != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if result.Snapshot.Providers[1].ErrorCode != "rate_limited" || result.Snapshot.Providers[2].Status != StatusUnconfigured {
		t.Fatalf("provider states=%+v", result.Snapshot.Providers)
	}
	if _, err := service.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&models.GPUPriceObservation{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("deduplicated count=%d err=%v", count, err)
	}
	history, err := service.History(context.Background(), HistoryFilter{Provider: "runpod", GPUModel: "h100", Days: 30, Limit: 10})
	if err != nil || history.Count != 1 || history.Items[0].SourceIdentity != "h100:secure" {
		t.Fatalf("history=%+v err=%v", history, err)
	}
}

func TestServiceRestoresLatestAndHistoryAcrossDatabaseRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "gpu-prices-restart.db")
	open := func() *gorm.DB {
		db, err := database.OpenSQLite(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.AutoMigrate(&models.GPUPriceObservation{}); err != nil {
			t.Fatal(err)
		}
		return db
	}
	now := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	firstDB := open()
	provider := &fakeProvider{name: ProviderRunPod, configured: true, quotes: []Quote{validTestQuote(ProviderRunPod, "H100", "restart-h100", 3.49)}}
	first := NewServiceWithProviders(firstDB, []Provider{provider}, func() time.Time { return now })
	if result, err := first.Refresh(context.Background()); err != nil || result.Snapshot.Count != 1 {
		t.Fatalf("initial refresh=%+v err=%v", result, err)
	}
	if sqlDB, err := firstDB.DB(); err != nil {
		t.Fatal(err)
	} else if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	secondDB := open()
	t.Cleanup(func() {
		if sqlDB, err := secondDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	restarted := NewServiceWithProviders(secondDB, []Provider{provider}, func() time.Time { return now.Add(13 * time.Hour) })
	snapshot := restarted.Current()
	if snapshot.Status != StatusStale || snapshot.Count != 1 || snapshot.Quotes[0].SourceIdentity != "restart-h100" {
		t.Fatalf("restored current=%+v", snapshot)
	}
	history, err := restarted.History(context.Background(), HistoryFilter{Provider: ProviderRunPod, Days: 30, Limit: 10})
	if err != nil || history.Count != 1 || history.Items[0].SourceIdentity != "restart-h100" {
		t.Fatalf("restored history=%+v err=%v", history, err)
	}
}

func TestServiceKeepsLastSuccessExplicitlyStaleAfterFailure(t *testing.T) {
	db := openGPUPriceTestDB(t)
	now := time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC)
	provider := &fakeProvider{name: ProviderRunPod, configured: true, quotes: []Quote{validTestQuote(ProviderRunPod, "H100", "h100", 3.49)}}
	service := NewServiceWithProviders(db, []Provider{provider}, func() time.Time { return now })
	if _, err := service.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	provider.mu.Lock()
	provider.err = providerError{code: "unauthorized"}
	provider.quotes = nil
	provider.mu.Unlock()
	now = now.Add(time.Hour)
	result, err := service.Refresh(context.Background())
	if err != nil || result.Snapshot.Status != StatusStale || result.Snapshot.Count != 1 || result.Snapshot.Providers[0].ErrorCode != "unauthorized" {
		t.Fatalf("stale fallback=%+v err=%v", result, err)
	}
}

func TestServiceRefreshFetchesConfiguredProvidersInParallel(t *testing.T) {
	db := openGPUPriceTestDB(t)
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	providers := []Provider{
		&fakeProvider{name: ProviderRunPod, configured: true, quotes: []Quote{validTestQuote(ProviderRunPod, "H100", "runpod-h100", 3.49)}, started: started, release: release},
		&fakeProvider{name: ProviderModal, configured: true, quotes: []Quote{validTestQuote(ProviderModal, "H100", "modal-h100", 3.95)}, started: started, release: release},
		&fakeProvider{name: ProviderLambda, configured: true, quotes: []Quote{validTestQuote(ProviderLambda, "H100", "lambda-h100", 4.29)}, started: started, release: release},
		&fakeProvider{name: ProviderVast, configured: true, quotes: []Quote{validTestQuote(ProviderVast, "H100", "vast-h100", 2.30)}, started: started, release: release},
	}
	service := NewServiceWithProviders(db, providers, time.Now)
	done := make(chan RefreshResult, 1)
	go func() {
		result, _ := service.Refresh(context.Background())
		done <- result
	}()

	deadline := time.After(750 * time.Millisecond)
	for range providers {
		select {
		case <-started:
		case <-deadline:
			close(release)
			t.Fatal("configured providers did not all start within one parallel upstream window")
		}
	}
	releasedAt := time.Now()
	close(release)
	select {
	case result := <-done:
		if elapsed := time.Since(releasedAt); elapsed > 750*time.Millisecond {
			t.Fatalf("parallel refresh took too long after providers were released: %s", elapsed)
		}
		if result.Updated != 4 || result.Snapshot.Status != StatusLive || result.Snapshot.Count != 4 {
			t.Fatalf("parallel result=%+v", result)
		}
	case <-time.After(750 * time.Millisecond):
		t.Fatal("parallel refresh did not finish after all providers completed")
	}
}

func TestServiceRefreshPropagatesParentCancellationToAllProviders(t *testing.T) {
	db := openGPUPriceTestDB(t)
	started := make(chan struct{}, 4)
	blocked := make(chan struct{})
	providers := []Provider{
		&fakeProvider{name: ProviderRunPod, configured: true, started: started, release: blocked},
		&fakeProvider{name: ProviderModal, configured: true, started: started, release: blocked},
		&fakeProvider{name: ProviderLambda, configured: true, started: started, release: blocked},
		&fakeProvider{name: ProviderVast, configured: true, started: started, release: blocked},
	}
	service := NewServiceWithProviders(db, providers, time.Now)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan RefreshResult, 1)
	go func() {
		result, _ := service.Refresh(ctx)
		done <- result
	}()
	for range providers {
		select {
		case <-started:
		case <-time.After(750 * time.Millisecond):
			cancel()
			t.Fatal("providers did not start before cancellation")
		}
	}
	cancel()
	select {
	case result := <-done:
		if result.Snapshot.Status != StatusUnavailable || result.Snapshot.Refreshing {
			t.Fatalf("cancelled result=%+v", result)
		}
	case <-time.After(750 * time.Millisecond):
		t.Fatal("parent cancellation did not stop provider refreshes")
	}
}

func TestServiceRejectsConcurrentRefresh(t *testing.T) {
	db := openGPUPriceTestDB(t)
	provider := &fakeProvider{name: ProviderRunPod, configured: true, quotes: []Quote{validTestQuote(ProviderRunPod, "H100", "h100", 3.49)}, started: make(chan struct{}, 1), release: make(chan struct{})}
	service := NewServiceWithProviders(db, []Provider{provider}, time.Now)
	done := make(chan error, 1)
	go func() { _, err := service.Refresh(context.Background()); done <- err }()
	<-provider.started
	if _, err := service.Refresh(context.Background()); !errors.Is(err, ErrRefreshInProgress) {
		t.Fatalf("concurrent err=%v", err)
	}
	close(provider.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestServiceDoesNotExposeHistoricalCurrentWhenCredentialRemoved(t *testing.T) {
	db := openGPUPriceTestDB(t)
	now := time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC)
	row := modelFromQuote(validTestQuote(ProviderRunPod, "H100", "h100", 3.49), now)
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithProviders(db, []Provider{&fakeProvider{name: ProviderRunPod, configured: false}}, func() time.Time { return now })
	snapshot := service.Current()
	if snapshot.Status != StatusUnconfigured || snapshot.Count != 0 || len(snapshot.Quotes) != 0 {
		t.Fatalf("removed credential exposed current quote: %+v", snapshot)
	}
	history, err := service.History(context.Background(), HistoryFilter{Days: 30})
	if err != nil || history.Count != 1 {
		t.Fatalf("durable history missing: %+v err=%v", history, err)
	}
}

func TestServiceReconfigureReplacesProviderConfiguration(t *testing.T) {
	service := NewServiceWithProviders(openGPUPriceTestDB(t), nil, time.Now)
	cfg := &config.Config{
		RunPodAPIKey: "runpod-secret", VastAPIKey: "vast-secret",
		GPUPriceRefreshInterval: 45 * time.Minute, GPUPriceHistoryRetention: 30 * 24 * time.Hour,
	}
	if err := service.Reconfigure(cfg); err != nil {
		t.Fatal(err)
	}
	snapshot := service.Current()
	configured := map[string]bool{}
	for _, status := range snapshot.Providers {
		configured[status.Provider] = status.Configured
	}
	if !configured[ProviderRunPod] || !configured[ProviderVast] || configured[ProviderModal] || configured[ProviderLambda] {
		t.Fatalf("provider configuration=%v", configured)
	}
	if service.interval != 45*time.Minute || service.retention != 30*24*time.Hour {
		t.Fatalf("interval=%s retention=%s", service.interval, service.retention)
	}
}

func TestSchedulerSurvivesUnconfiguredStartupAndRefreshesAfterReconfigure(t *testing.T) {
	service := NewServiceWithProviders(openGPUPriceTestDB(t), []Provider{&fakeProvider{name: ProviderRunPod}}, time.Now)
	service.interval = 100 * time.Millisecond
	service.retryBase = 10 * time.Millisecond
	service.retryMax = 100 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.Start(ctx)
	waitForCondition(t, time.Second, func() bool { return service.Current().NextRefreshAt != "" }, "unconfigured scheduler did not publish next refresh")

	var calls atomic.Int32
	provider := &countingProvider{fakeProvider: fakeProvider{
		name: ProviderRunPod, configured: true,
		quotes: []Quote{validTestQuote(ProviderRunPod, "H100", "hot-reload", 3.49)},
	}, calls: &calls}
	if err := service.reconfigureProviders([]Provider{provider}, nil); err != nil {
		t.Fatal(err)
	}
	waitForCondition(t, time.Second, func() bool { return calls.Load() > 0 && service.Current().Status == StatusLive }, "reconfigured scheduler did not refresh")
	if service.Current().NextRefreshAt == "" {
		t.Fatal("scheduler did not publish the next refresh after success")
	}
}

func TestServiceRefreshPrunesObservationsOutsideRetention(t *testing.T) {
	db := openGPUPriceTestDB(t)
	now := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	old := modelFromQuote(validTestQuote(ProviderRunPod, "A100", "old", 1.5), now.Add(-49*time.Hour))
	recent := modelFromQuote(validTestQuote(ProviderRunPod, "A100", "recent", 1.6), now.Add(-47*time.Hour))
	if err := db.Create(&[]models.GPUPriceObservation{old, recent}).Error; err != nil {
		t.Fatal(err)
	}
	provider := &fakeProvider{name: ProviderRunPod, configured: true, quotes: []Quote{validTestQuote(ProviderRunPod, "H100", "current", 3.49)}}
	service := NewServiceWithProviders(db, []Provider{provider}, func() time.Time { return now })
	service.retention = 48 * time.Hour
	if _, err := service.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	var identities []string
	if err := db.Model(&models.GPUPriceObservation{}).Order("source_identity").Pluck("source_identity", &identities).Error; err != nil {
		t.Fatal(err)
	}
	if len(identities) != 2 || identities[0] != "current" || identities[1] != "recent" {
		t.Fatalf("retained identities=%v", identities)
	}
}

func TestSchedulerRetriesPartialProviderFailureBeforeFullInterval(t *testing.T) {
	db := openGPUPriceTestDB(t)
	goodCalls := &atomic.Int32{}
	badCalls := &atomic.Int32{}
	good := &countingProvider{
		fakeProvider: fakeProvider{name: ProviderRunPod, configured: true, quotes: []Quote{validTestQuote(ProviderRunPod, "H100", "scheduler-good", 3.49)}},
		calls:        goodCalls,
	}
	bad := &countingProvider{
		fakeProvider: fakeProvider{name: ProviderLambda, configured: true, err: providerError{code: "rate_limited"}},
		calls:        badCalls,
	}
	service := NewServiceWithProviders(db, []Provider{good, bad}, time.Now)
	service.interval = time.Second
	service.retryBase = 20 * time.Millisecond
	service.retryMax = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	service.Start(ctx)

	waitForCondition(t, 500*time.Millisecond, func() bool {
		return badCalls.Load() >= 2
	}, "partial provider failure did not trigger bounded retry before the full interval")
	if goodCalls.Load() != 1 {
		t.Fatalf("partial retry fetched successful provider %d times; want exactly once", goodCalls.Load())
	}
	if snapshot := service.Current(); snapshot.Status != StatusPartial {
		t.Fatalf("partial scheduler snapshot=%+v", snapshot)
	}
}

func TestRetryDelayIsExponentialAndBounded(t *testing.T) {
	service := NewServiceWithProviders(nil, nil, time.Now)
	service.retryBase = 10 * time.Millisecond
	service.retryMax = 40 * time.Millisecond
	want := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond}
	for index, expected := range want {
		if actual := service.retryDelay(index + 1); actual != expected {
			t.Fatalf("failure %d delay=%s want=%s", index+1, actual, expected)
		}
	}
}

type countingProvider struct {
	fakeProvider
	calls *atomic.Int32
}

func (p *countingProvider) Fetch(ctx context.Context, observedAt time.Time) ([]Quote, error) {
	p.calls.Add(1)
	return p.fakeProvider.Fetch(ctx, observedAt)
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal(message)
}
