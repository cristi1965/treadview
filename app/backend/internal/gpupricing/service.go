package gpupricing

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"trading-agents/internal/config"
	"trading-agents/internal/models"
)

const (
	defaultRefreshInterval  = 6 * time.Hour
	defaultHistoryRetention = 90 * 24 * time.Hour
	providerTimeout         = 45 * time.Second
	ObservationFreshFor     = 12 * time.Hour
)

type providerFetchResult struct {
	name   string
	quotes []Quote
	err    error
}

// Service is the deep module used by HTTP callers. It owns authenticated
// provider fan-out, validation, current-state caching and durable history.
type Service struct {
	db        *gorm.DB
	providers []Provider
	now       func() time.Time
	interval  time.Duration
	retention time.Duration
	retryBase time.Duration
	retryMax  time.Duration
	staleFor  time.Duration
	testOnly  bool

	mu               sync.RWMutex
	refreshing       bool
	current          map[string][]Quote
	statuses         map[string]ProviderStatus
	nextRefreshAt    time.Time
	schedulerWake    chan struct{}
	schedulerStarted bool
	startOnce        sync.Once
}

func NewService(cfg *config.Config, db *gorm.DB) *Service {
	providers := runtimeFixtureProviders(cfg)
	testOnly := providers != nil
	if providers == nil {
		providers = providersFromConfig(cfg)
	}
	service := NewServiceWithProviders(db, providers, time.Now)
	service.testOnly = testOnly
	if cfg != nil {
		service.applyOperationalConfig(cfg)
	}
	return service
}

func providersFromConfig(cfg *config.Config) []Provider {
	client := &http.Client{
		Timeout: providerTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	var runPodKey, modalID, modalSecret, lambdaKey, vastKey string
	if cfg != nil {
		runPodKey = cfg.RunPodAPIKey
		modalID = cfg.ModalTokenID
		modalSecret = cfg.ModalTokenSecret
		lambdaKey = cfg.LambdaAPIKey
		vastKey = cfg.VastAPIKey
	}
	providers := []Provider{
		&runPodProvider{key: runPodKey, endpoint: runPodSourceURL, http: client},
		&modalProvider{tokenID: modalID, tokenSecret: modalSecret},
		&lambdaProvider{key: lambdaKey, endpoint: lambdaSourceURL, http: client},
		&vastProvider{key: vastKey, endpoint: vastSourceURL, http: client},
	}
	return providers
}

func NewServiceWithProviders(db *gorm.DB, providers []Provider, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	s := &Service{
		db: db, providers: append([]Provider(nil), providers...), now: now,
		interval: defaultRefreshInterval, retention: defaultHistoryRetention,
		retryBase: 15 * time.Minute, retryMax: defaultRefreshInterval, staleFor: ObservationFreshFor,
		current: make(map[string][]Quote), statuses: make(map[string]ProviderStatus), schedulerWake: make(chan struct{}, 1),
	}
	for _, provider := range s.providers {
		name := strings.ToLower(strings.TrimSpace(provider.Name()))
		status := ProviderStatus{Provider: name, Configured: provider.Configured(), Status: StatusUnconfigured}
		if status.Configured {
			status.Status = StatusUnavailable
		}
		s.statuses[name] = status
	}
	s.loadLatest()
	return s
}

// Start keeps the scheduler alive even when no provider is configured. A later
// Reconfigure wakes it so newly supplied credentials take effect immediately.
func (s *Service) Start(ctx context.Context) {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		s.mu.Lock()
		s.schedulerStarted = true
		s.mu.Unlock()
		go s.runScheduler(ctx)
	})
}

// Reconfigure atomically replaces provider clients without exposing credential
// values. It is intentionally rejected while a refresh is using the old set.
func (s *Service) Reconfigure(cfg *config.Config) error {
	if s == nil || cfg == nil {
		return errors.New("GPU pricing reconfigure requires service and config")
	}
	providers := runtimeFixtureProviders(cfg)
	testOnly := providers != nil
	if providers == nil {
		providers = providersFromConfig(cfg)
	}
	return s.reconfigureProvidersWithMode(providers, cfg, testOnly)
}

func (s *Service) reconfigureProviders(providers []Provider, cfg *config.Config) error {
	return s.reconfigureProvidersWithMode(providers, cfg, false)
}

func (s *Service) reconfigureProvidersWithMode(providers []Provider, cfg *config.Config, testOnly bool) error {
	s.mu.Lock()
	if s.refreshing {
		s.mu.Unlock()
		return ErrRefreshInProgress
	}
	s.providers = providers
	s.testOnly = testOnly
	if cfg != nil {
		s.applyOperationalConfigLocked(cfg)
	}
	nextStatuses := make(map[string]ProviderStatus, len(providers))
	for _, provider := range providers {
		name := normalizedProviderName(provider)
		previous := s.statuses[name]
		previous.Provider = name
		previous.Configured = provider.Configured()
		previous.QuoteCount = len(s.current[name])
		previous.ErrorCode = ""
		if !previous.Configured {
			previous.Status = StatusUnconfigured
			previous.QuoteCount = 0
		} else if previous.LastSuccessAt != "" && len(s.current[name]) > 0 {
			previous.Status = StatusStale
		} else {
			previous.Status = StatusUnavailable
		}
		nextStatuses[name] = previous
	}
	s.statuses = nextStatuses
	s.nextRefreshAt = time.Time{}
	started := s.schedulerStarted
	s.mu.Unlock()
	if started {
		s.wakeScheduler()
	}
	return nil
}

func (s *Service) runScheduler(ctx context.Context) {
	failures := 0
	nextFullRefresh := time.Time{}
	for {
		if ctx.Err() != nil {
			s.setNextRefresh(time.Time{})
			return
		}
		configured := s.configuredCount()
		delay := s.refreshInterval()
		if configured > 0 {
			now := s.now().UTC()
			retryOnly := failures > 0 && !nextFullRefresh.IsZero() && now.Before(nextFullRefresh)
			var result RefreshResult
			var err error
			if retryOnly {
				result, err = s.refresh(ctx, true)
			} else {
				result, err = s.Refresh(ctx)
				nextFullRefresh = s.now().UTC().Add(s.refreshInterval())
			}
			if err != nil || result.Updated == 0 || result.Snapshot.Status != StatusLive {
				failures++
				delay = s.retryDelay(failures)
			} else {
				failures = 0
			}
			if untilFull := nextFullRefresh.Sub(s.now().UTC()); untilFull > 0 && untilFull < delay {
				delay = untilFull
			}
		} else {
			failures = 0
			nextFullRefresh = time.Time{}
		}
		next := s.now().UTC().Add(delay)
		s.setNextRefresh(next)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			s.setNextRefresh(time.Time{})
			return
		case <-s.schedulerWake:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			s.setNextRefresh(time.Time{})
			failures = 0
			nextFullRefresh = time.Time{}
		case <-timer.C:
		}
	}
}

func (s *Service) Refresh(ctx context.Context) (RefreshResult, error) {
	return s.refresh(ctx, false)
}

func (s *Service) refresh(ctx context.Context, retryOnly bool) (RefreshResult, error) {
	if s == nil {
		return RefreshResult{}, nil
	}
	s.mu.Lock()
	if s.refreshing {
		s.mu.Unlock()
		return RefreshResult{Snapshot: s.Current()}, ErrRefreshInProgress
	}
	s.refreshing = true
	s.mu.Unlock()
	defer func() {
		s.setRefreshing(false)
	}()
	observedAt := s.now().UTC().Truncate(time.Second)
	observedAtText := observedAt.Format(time.RFC3339)
	providers := s.providerSnapshot()
	results := make(chan providerFetchResult, len(providers))
	configured := 0
	for _, provider := range providers {
		name := normalizedProviderName(provider)
		if !provider.Configured() {
			s.updateStatus(name, func(status *ProviderStatus) {
				status.Configured = false
				status.Status = StatusUnconfigured
				status.ErrorCode = ""
				status.QuoteCount = 0
			})
			continue
		}
		if retryOnly && !s.providerNeedsRetry(name) {
			continue
		}
		s.updateStatus(name, func(status *ProviderStatus) {
			status.Configured = true
			status.Status = StatusRefreshing
			status.LastAttemptAt = observedAtText
			status.ErrorCode = ""
		})
		configured++
		go func(provider Provider, name string) {
			providerCtx, cancel := context.WithTimeout(ctx, providerTimeout)
			defer cancel()
			quotes, err := provider.Fetch(providerCtx, observedAt)
			valid := make([]Quote, 0, len(quotes))
			for _, quote := range quotes {
				quote.Provider = name
				quote.ObservedAt = observedAtText
				if validQuote(quote) {
					valid = append(valid, quote)
				}
			}
			if err == nil && len(valid) == 0 {
				err = providerError{code: "no_valid_quotes"}
			}
			results <- providerFetchResult{name: name, quotes: valid, err: err}
		}(provider, name)
	}

	updated := 0
	for range configured {
		result := <-results
		if result.err == nil {
			result.err = s.persist(result.quotes, observedAt)
			if result.err != nil {
				result.err = providerError{code: "store_failed"}
			}
		}
		if result.err != nil {
			code := errorCode(result.err)
			s.mu.RLock()
			hasCurrent := len(s.current[result.name]) > 0
			s.mu.RUnlock()
			s.updateStatus(result.name, func(status *ProviderStatus) {
				status.ErrorCode = code
				if hasCurrent {
					status.Status = StatusStale
				} else {
					status.Status = StatusUnavailable
				}
			})
			continue
		}
		s.mu.Lock()
		s.current[result.name] = cloneQuotes(result.quotes)
		status := s.statuses[result.name]
		status.Status = StatusLive
		status.QuoteCount = len(result.quotes)
		status.LastSuccessAt = observedAtText
		status.ErrorCode = ""
		s.statuses[result.name] = status
		s.mu.Unlock()
		updated += len(result.quotes)
	}
	s.setRefreshing(false)
	result := RefreshResult{Snapshot: s.Current(), Updated: updated}
	if err := s.pruneHistory(ctx); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Service) providerNeedsRetry(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := s.statuses[name]
	return status.ErrorCode != "" || status.Status == StatusUnavailable
}

func (s *Service) Current() Snapshot {
	if s == nil {
		return Snapshot{Status: StatusUnavailable, Providers: []ProviderStatus{}, Quotes: []Quote{}}
	}
	now := s.now().UTC()
	s.mu.RLock()
	refreshing := s.refreshing
	providers := append([]Provider(nil), s.providers...)
	testOnly := s.testOnly
	nextRefreshAt := s.nextRefreshAt
	statuses := make([]ProviderStatus, 0, len(providers))
	quotes := make([]Quote, 0)
	for _, provider := range providers {
		name := normalizedProviderName(provider)
		status := s.statuses[name]
		status.Configured = provider.Configured()
		if !status.Configured {
			status.Status = StatusUnconfigured
			status.QuoteCount = 0
		} else {
			providerQuotes := cloneQuotes(s.current[name])
			quotes = append(quotes, providerQuotes...)
			status.QuoteCount = len(providerQuotes)
			if refreshing && status.Status == StatusRefreshing {
				// Keep the active status.
			} else if status.LastSuccessAt == "" || len(providerQuotes) == 0 {
				status.Status = StatusUnavailable
			} else if observed, err := time.Parse(time.RFC3339, status.LastSuccessAt); err != nil || now.Sub(observed) > s.staleFor || status.ErrorCode != "" {
				status.Status = StatusStale
			} else {
				status.Status = StatusLive
			}
		}
		statuses = append(statuses, status)
	}
	s.mu.RUnlock()
	sort.Slice(quotes, func(i, j int) bool {
		if quotes[i].Provider != quotes[j].Provider {
			return quotes[i].Provider < quotes[j].Provider
		}
		if quotes[i].GPUModel != quotes[j].GPUModel {
			return quotes[i].GPUModel < quotes[j].GPUModel
		}
		return quotes[i].PriceUSDPerGPUHour < quotes[j].PriceUSDPerGPUHour
	})
	status, observedAt := aggregateStatus(statuses, quotes, refreshing)
	nextRefreshText := ""
	if !nextRefreshAt.IsZero() {
		nextRefreshText = nextRefreshAt.UTC().Format(time.RFC3339)
	}
	dataMode := ""
	if testOnly {
		dataMode = RuntimeFixtureDataMode
	}
	return Snapshot{Status: status, DataMode: dataMode, TestOnly: testOnly, ObservedAt: observedAt, NextRefreshAt: nextRefreshText, Refreshing: refreshing, Providers: statuses, Quotes: quotes, Count: len(quotes)}
}

func (s *Service) History(ctx context.Context, filter HistoryFilter) (HistoryResult, error) {
	if filter.Days <= 0 {
		filter.Days = 30
	}
	if filter.Days > 3650 {
		filter.Days = 3650
	}
	if filter.Limit <= 0 {
		filter.Limit = 1000
	}
	if filter.Limit > 5000 {
		filter.Limit = 5000
	}
	if s == nil || s.db == nil {
		return HistoryResult{Items: []Quote{}, Days: filter.Days}, nil
	}
	s.mu.RLock()
	testOnly := s.testOnly
	s.mu.RUnlock()
	var rows []models.GPUPriceObservation
	query := s.db.WithContext(ctx).Where("observed_at >= ?", s.now().UTC().Add(-time.Duration(filter.Days)*24*time.Hour)).Order("observed_at desc, id desc").Limit(filter.Limit)
	if provider := strings.ToLower(strings.TrimSpace(filter.Provider)); provider != "" {
		query = query.Where("provider = ?", provider)
	}
	if model := strings.TrimSpace(filter.GPUModel); model != "" {
		query = query.Where("LOWER(gpu_model) LIKE ?", "%"+strings.ToLower(model)+"%")
	}
	if err := query.Find(&rows).Error; err != nil {
		return HistoryResult{}, err
	}
	items := make([]Quote, 0, len(rows))
	for _, row := range rows {
		items = append(items, quoteFromModel(row))
	}
	dataMode := ""
	if testOnly {
		dataMode = RuntimeFixtureDataMode
	}
	return HistoryResult{Items: items, Count: len(items), Days: filter.Days, DataMode: dataMode, TestOnly: testOnly}, nil
}

func (s *Service) updateStatus(name string, update func(*ProviderStatus)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.statuses[name]
	update(&status)
	s.statuses[name] = status
}

func (s *Service) setRefreshing(refreshing bool) {
	s.mu.Lock()
	s.refreshing = refreshing
	s.mu.Unlock()
}

func (s *Service) persist(quotes []Quote, observedAt time.Time) error {
	if s.db == nil {
		return gorm.ErrInvalidDB
	}
	rows := make([]models.GPUPriceObservation, 0, len(quotes))
	for _, quote := range quotes {
		rows = append(rows, modelFromQuote(quote, observedAt))
	}
	return s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

func (s *Service) loadLatest() {
	if s.db == nil || !s.db.Migrator().HasTable(&models.GPUPriceObservation{}) {
		return
	}
	var rows []models.GPUPriceObservation
	if err := s.db.Order("observed_at desc, id desc").Limit(10000).Find(&rows).Error; err != nil {
		return
	}
	latest := map[string]time.Time{}
	for _, row := range rows {
		name := strings.ToLower(row.Provider)
		if at, ok := latest[name]; ok && !row.ObservedAt.Equal(at) {
			continue
		}
		latest[name] = row.ObservedAt
		s.current[name] = append(s.current[name], quoteFromModel(row))
	}
	for name, observedAt := range latest {
		status := s.statuses[name]
		status.LastSuccessAt = observedAt.UTC().Format(time.RFC3339)
		status.QuoteCount = len(s.current[name])
		if status.Configured {
			status.Status = StatusStale
		}
		s.statuses[name] = status
	}
}

func (s *Service) configuredCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, provider := range s.providers {
		if provider.Configured() {
			count++
		}
	}
	return count
}

func (s *Service) providerSnapshot() []Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Provider(nil), s.providers...)
}

func (s *Service) applyOperationalConfig(cfg *config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applyOperationalConfigLocked(cfg)
}

func (s *Service) applyOperationalConfigLocked(cfg *config.Config) {
	if cfg.GPUPriceRefreshInterval > 0 {
		s.interval = cfg.GPUPriceRefreshInterval
	}
	if cfg.GPUPriceHistoryRetention > 0 {
		s.retention = cfg.GPUPriceHistoryRetention
	}
	s.retryBase = minDuration(15*time.Minute, s.interval)
	s.retryMax = s.interval
}

func (s *Service) refreshInterval() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.interval
}

func (s *Service) retryDelay(failures int) time.Duration {
	s.mu.RLock()
	base, maximum := s.retryBase, s.retryMax
	s.mu.RUnlock()
	if base <= 0 {
		base = time.Minute
	}
	if maximum < base {
		maximum = base
	}
	delay := base
	for attempt := 1; attempt < failures && delay < maximum; attempt++ {
		if delay > maximum/2 {
			return maximum
		}
		delay *= 2
	}
	return minDuration(delay, maximum)
}

func (s *Service) pruneHistory(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	s.mu.RLock()
	retention := s.retention
	s.mu.RUnlock()
	if retention <= 0 {
		return nil
	}
	cutoff := s.now().UTC().Add(-retention)
	return s.db.WithContext(ctx).Where("observed_at < ?", cutoff).Delete(&models.GPUPriceObservation{}).Error
}

func (s *Service) setNextRefresh(at time.Time) {
	s.mu.Lock()
	s.nextRefreshAt = at
	s.mu.Unlock()
}

func (s *Service) wakeScheduler() {
	select {
	case s.schedulerWake <- struct{}{}:
	default:
	}
}

func normalizedProviderName(provider Provider) string {
	return strings.ToLower(strings.TrimSpace(provider.Name()))
}

func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}

func aggregateStatus(statuses []ProviderStatus, quotes []Quote, refreshing bool) (string, string) {
	configured, live, stale, unavailable := 0, 0, 0, 0
	for _, status := range statuses {
		if !status.Configured {
			continue
		}
		configured++
		switch status.Status {
		case StatusLive:
			live++
		case StatusStale:
			stale++
		case StatusUnavailable:
			unavailable++
		}
	}
	observedAt := oldestObservedAt(quotes)
	if configured == 0 {
		return StatusUnconfigured, observedAt
	}
	if refreshing && live+stale+unavailable == 0 {
		return StatusRefreshing, observedAt
	}
	if live == configured {
		return StatusLive, observedAt
	}
	if live > 0 || stale > 0 && unavailable > 0 {
		return StatusPartial, observedAt
	}
	if stale > 0 {
		return StatusStale, observedAt
	}
	return StatusUnavailable, observedAt
}

func oldestObservedAt(quotes []Quote) string {
	oldest := time.Time{}
	for _, quote := range quotes {
		observed, err := time.Parse(time.RFC3339, quote.ObservedAt)
		if err == nil && (oldest.IsZero() || observed.Before(oldest)) {
			oldest = observed
		}
	}
	if oldest.IsZero() {
		return ""
	}
	return oldest.UTC().Format(time.RFC3339)
}

func cloneQuotes(input []Quote) []Quote { return append([]Quote(nil), input...) }

func modelFromQuote(quote Quote, observedAt time.Time) models.GPUPriceObservation {
	return models.GPUPriceObservation{
		Provider: quote.Provider, SourceIdentity: quote.SourceIdentity, ObservedAt: observedAt,
		GPUModel: quote.GPUModel, Product: quote.Product, BillingMode: quote.BillingMode, GPUCount: quote.GPUCount,
		MemoryGiB: quote.MemoryGiB, Region: quote.Region, OfferID: quote.OfferID, Availability: quote.Availability,
		Currency: quote.Currency, RawPrice: quote.RawPrice, RawUnit: quote.RawUnit, PriceUSDPerGPUHour: quote.PriceUSDPerGPUHour,
		InstanceTotalUSDPerHour: quote.InstanceTotalUSDPerHour, ComputeUSDPerHour: quote.ComputeUSDPerHour, StorageUSDPerHour: quote.StorageUSDPerHour,
		BandwidthUpUSDPerTB: quote.BandwidthUpUSDPerTB, BandwidthDownUSDPerTB: quote.BandwidthDownUSDPerTB, SourceURL: quote.SourceURL,
	}
}

func quoteFromModel(row models.GPUPriceObservation) Quote {
	return Quote{
		Provider: row.Provider, SourceIdentity: row.SourceIdentity, ObservedAt: row.ObservedAt.UTC().Format(time.RFC3339),
		GPUModel: row.GPUModel, Product: row.Product, BillingMode: row.BillingMode, GPUCount: row.GPUCount,
		MemoryGiB: row.MemoryGiB, Region: row.Region, OfferID: row.OfferID, Availability: row.Availability,
		Currency: row.Currency, RawPrice: row.RawPrice, RawUnit: row.RawUnit, PriceUSDPerGPUHour: row.PriceUSDPerGPUHour,
		InstanceTotalUSDPerHour: row.InstanceTotalUSDPerHour, ComputeUSDPerHour: row.ComputeUSDPerHour, StorageUSDPerHour: row.StorageUSDPerHour,
		BandwidthUpUSDPerTB: row.BandwidthUpUSDPerTB, BandwidthDownUSDPerTB: row.BandwidthDownUSDPerTB, SourceURL: row.SourceURL,
	}
}
