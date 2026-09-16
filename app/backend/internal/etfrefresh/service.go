package etfrefresh

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"trading-agents/internal/dataflows"
)

const (
	DefaultScopeLimit = 24
	DefaultInterval   = 24 * time.Hour
	minimumFiveYBars  = 1000
)

type HistoryFetcher interface {
	GetHistoricalData(ticker, startDate, endDate string) ([]dataflows.HistoricalBar, error)
}

type Config struct {
	BackendRoot string
	ScopeLimit  int
	Interval    time.Duration
	Now         func() time.Time
	Fetcher     HistoryFetcher
}

type Service struct {
	root       string
	scopeLimit int
	interval   time.Duration
	now        func() time.Time
	fetcher    HistoryFetcher
	runMu      sync.Mutex
	trigger    chan struct{}
}

type RefreshStatus struct {
	LastAttemptAt     string `json:"lastAttemptAt"`
	NextScheduledAt   string `json:"nextScheduledAt"`
	LastAttemptStatus string `json:"lastAttemptStatus"`
	LastAttemptError  string `json:"lastAttemptError,omitempty"`
	ScopeRequested    int    `json:"scopeRequested"`
	ScopeCompleted    int    `json:"scopeCompleted"`
}

type analysesFile struct {
	Updated      string                 `json:"updated"`
	AUMUnit      string                 `json:"aumUnit"`
	N            int                    `json:"n"`
	Supers       map[string]int         `json:"supers"`
	Sectors      []sectorEntry          `json:"sectors"`
	ETFs         []analysisEntry        `json:"etfs"`
	Source       string                 `json:"source,omitempty"`
	SourceURL    string                 `json:"sourceURL,omitempty"`
	MetadataAsOf string                 `json:"metadataAsOf,omitempty"`
	Scope        map[string]interface{} `json:"scope,omitempty"`
}

type sectorEntry struct {
	Sector string  `json:"sector"`
	Super  string  `json:"super"`
	N      int     `json:"n"`
	AUM    float64 `json:"aum"`
}

type analysisEntry struct {
	Sym     string   `json:"sym"`
	Name    string   `json:"name"`
	AUM     float64  `json:"aum,omitempty"`
	Expense *float64 `json:"expense,omitempty"`
	Ret1Y   *float64 `json:"ret1y,omitempty"`
	Ret5Y   *float64 `json:"ret5y,omitempty"`
	MDD     *float64 `json:"mdd,omitempty"`
	Sector  string   `json:"sector"`
	Kind    string   `json:"kind"`
	Verdict string   `json:"verdict,omitempty"`
}

func NewService(cfg Config) *Service {
	root := strings.TrimSpace(cfg.BackendRoot)
	if root == "" {
		root = resolveBackendRoot()
	}
	limit := cfg.ScopeLimit
	if limit <= 0 {
		limit = DefaultScopeLimit
	}
	interval := cfg.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	fetcher := cfg.Fetcher
	if fetcher == nil {
		fetcher = dataflows.NewYFinanceClientWithTimeout(20 * time.Second)
	}
	return &Service{root: root, scopeLimit: limit, interval: interval, now: now, fetcher: fetcher, trigger: make(chan struct{}, 1)}
}

// Start runs one refresh immediately and then once per configured interval.
func (s *Service) Start(ctx context.Context) {
	go func() {
		s.runAndLog(ctx)
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runAndLog(ctx)
			case <-s.trigger:
				s.runAndLog(ctx)
			}
		}
	}()
}

// Trigger queues one refresh on the service-owned scheduler without making an
// HTTP request wait for the bounded multi-symbol upstream run.
func (s *Service) Trigger() bool {
	if s == nil {
		return false
	}
	select {
	case s.trigger <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *Service) runAndLog(ctx context.Context) {
	if err := s.RunOnce(ctx); err != nil {
		log.Printf("[etf-refresh] refresh failed without replacing last-good: %v", err)
	}
}

// RunOnce promotes a new current file only when every ETF in the bounded scope
// has enough provider-dated history to compute all advertised metrics.
func (s *Service) RunOnce(ctx context.Context) error {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	now := s.now().UTC()
	status := RefreshStatus{
		LastAttemptAt: now.Format(time.RFC3339), NextScheduledAt: now.Add(s.interval).Format(time.RFC3339),
		LastAttemptStatus: "running",
	}
	_ = s.writeStatus(status)

	input, err := s.loadUniverse()
	if err != nil {
		return s.fail(status, err)
	}
	selected := selectScope(input.ETFs, s.scopeLimit)
	status.ScopeRequested = len(selected)
	if len(selected) == 0 {
		return s.fail(status, fmt.Errorf("ETF refresh scope is empty"))
	}
	if err := validateDecisionMetadata(selected); err != nil {
		return s.fail(status, err)
	}

	start := now.AddDate(-6, 0, -7).Format("2006-01-02")
	end := now.AddDate(0, 0, 1).Format("2006-01-02")
	latestCommon := ""
	histories := make(map[string][]dataflows.HistoricalBar, len(selected))
	for index := range selected {
		if err := ctx.Err(); err != nil {
			return s.fail(status, err)
		}
		bars, fetchErr := s.fetcher.GetHistoricalData(selected[index].Sym, start, end)
		if fetchErr != nil {
			return s.fail(status, fmt.Errorf("%s history: %w", selected[index].Sym, fetchErr))
		}
		latest := latestValidBarDate(bars)
		if latest == "" {
			return s.fail(status, fmt.Errorf("%s history has no valid dated prices", selected[index].Sym))
		}
		histories[selected[index].Sym] = bars
		if latestCommon == "" || latest < latestCommon {
			latestCommon = latest
		}
	}
	latestObserved, parseErr := time.Parse("2006-01-02", latestCommon)
	if parseErr != nil || latestObserved.After(now) || now.Sub(latestObserved) > 7*24*time.Hour {
		return s.fail(status, fmt.Errorf("ETF history latest common observation is not current: %s", latestCommon))
	}
	for index := range selected {
		bars := histories[selected[index].Sym]
		aligned := make([]dataflows.HistoricalBar, 0, len(bars))
		for _, bar := range bars {
			if bar.Date <= latestCommon {
				aligned = append(aligned, bar)
			}
		}
		oneYear, fiveYear, drawdown, latest, calcErr := calculateMetrics(aligned)
		if calcErr != nil || latest != latestCommon {
			return s.fail(status, fmt.Errorf("%s aligned history at %s is incomplete: %v", selected[index].Sym, latestCommon, calcErr))
		}
		selected[index].Ret1Y = &oneYear
		selected[index].Ret5Y = &fiveYear
		selected[index].MDD = &drawdown
		status.ScopeCompleted++
	}

	output := buildOutput(input, selected, latestCommon)
	if err := s.writeCurrent(output); err != nil {
		return s.fail(status, err)
	}
	status.LastAttemptStatus = "complete"
	status.LastAttemptError = ""
	return s.writeStatus(status)
}

func latestValidBarDate(bars []dataflows.HistoricalBar) string {
	latest := ""
	for _, bar := range bars {
		if bar.Close > 0 && bar.Date > latest {
			latest = bar.Date
		}
	}
	return latest
}

func resolveBackendRoot() string {
	cwd, _ := os.Getwd()
	for _, candidate := range []string{cwd, filepath.Join(cwd, "app", "backend")} {
		if _, err := os.Stat(filepath.Join(candidate, "go.mod")); err == nil {
			return candidate
		}
	}
	return cwd
}

func (s *Service) fail(status RefreshStatus, err error) error {
	status.LastAttemptStatus = "failed"
	status.LastAttemptError = err.Error()
	if statusErr := s.writeStatus(status); statusErr != nil {
		return fmt.Errorf("%v; write diagnostics: %w", err, statusErr)
	}
	return err
}

func (s *Service) loadUniverse() (analysesFile, error) {
	paths := []string{
		filepath.Join(s.root, "..", "frontend", "public", "data", "etf-analyses.json"),
		filepath.Join(s.root, "data", "etf-analyses.json"),
		filepath.Join(s.root, "data", "etf-analyses-current.json"),
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var input analysesFile
		if err := json.Unmarshal(raw, &input); err != nil {
			return analysesFile{}, fmt.Errorf("parse ETF universe %s: %w", path, err)
		}
		if len(input.ETFs) == 0 {
			continue
		}
		if input.AUMUnit == "usd_thousands" {
			for i := range input.ETFs {
				input.ETFs[i].AUM *= 1000
			}
			input.AUMUnit = "usd"
		}
		if input.AUMUnit != "usd" {
			return analysesFile{}, fmt.Errorf("ETF universe has unsupported aumUnit %q", input.AUMUnit)
		}
		return input, nil
	}
	return analysesFile{}, fmt.Errorf("ETF universe file not found")
}

func selectScope(entries []analysisEntry, limit int) []analysisEntry {
	selected := append([]analysisEntry(nil), entries...)
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].AUM == selected[j].AUM {
			return selected[i].Sym < selected[j].Sym
		}
		return selected[i].AUM > selected[j].AUM
	})
	if limit < len(selected) {
		selected = selected[:limit]
	}
	return selected
}

func validateDecisionMetadata(entries []analysisEntry) error {
	for _, entry := range entries {
		if strings.TrimSpace(entry.Sym) == "" || strings.TrimSpace(entry.Name) == "" || strings.TrimSpace(entry.Sector) == "" || strings.TrimSpace(entry.Kind) == "" || entry.AUM <= 0 || entry.Expense == nil || *entry.Expense < 0 {
			return fmt.Errorf("ETF decision metadata incomplete for %s", entry.Sym)
		}
	}
	return nil
}

func calculateMetrics(input []dataflows.HistoricalBar) (float64, float64, float64, string, error) {
	bars := make([]dataflows.HistoricalBar, 0, len(input))
	for _, bar := range input {
		if bar.Date != "" && bar.Close > 0 {
			bars = append(bars, bar)
		}
	}
	sort.SliceStable(bars, func(i, j int) bool { return bars[i].Date < bars[j].Date })
	if len(bars) < minimumFiveYBars {
		return 0, 0, 0, "", fmt.Errorf("only %d valid sessions; need %d", len(bars), minimumFiveYBars)
	}
	latestDate, err := time.Parse("2006-01-02", bars[len(bars)-1].Date)
	if err != nil {
		return 0, 0, 0, "", fmt.Errorf("invalid latest session date")
	}
	oneYearIndex, ok := indexAtOrBefore(bars, latestDate.AddDate(-1, 0, 0))
	if !ok {
		return 0, 0, 0, "", fmt.Errorf("one-year baseline unavailable")
	}
	fiveYearIndex, ok := indexAtOrBefore(bars, latestDate.AddDate(-5, 0, 0))
	if !ok {
		return 0, 0, 0, "", fmt.Errorf("five-year baseline unavailable")
	}
	last := bars[len(bars)-1].Close
	oneYearBase := bars[oneYearIndex].Close
	fiveYearBase := bars[fiveYearIndex].Close
	if oneYearBase <= 0 || fiveYearBase <= 0 {
		return 0, 0, 0, "", fmt.Errorf("invalid return baseline")
	}
	peak := bars[fiveYearIndex].Close
	maxDrawdown := 0.0
	for _, bar := range bars[fiveYearIndex:] {
		peak = math.Max(peak, bar.Close)
		drawdown := (bar.Close/peak - 1) * 100
		maxDrawdown = math.Min(maxDrawdown, drawdown)
	}
	return round((last/oneYearBase - 1) * 100), round((last/fiveYearBase - 1) * 100), round(maxDrawdown), bars[len(bars)-1].Date, nil
}

func indexAtOrBefore(bars []dataflows.HistoricalBar, target time.Time) (int, bool) {
	targetLabel := target.Format("2006-01-02")
	for index := len(bars) - 1; index >= 0; index-- {
		if bars[index].Date <= targetLabel {
			return index, true
		}
	}
	return 0, false
}

func buildOutput(input analysesFile, entries []analysisEntry, updated string) analysesFile {
	superBySector := make(map[string]string, len(input.Sectors))
	for _, sector := range input.Sectors {
		superBySector[sector.Sector] = sector.Super
	}
	sectorMap := map[string]*sectorEntry{}
	supers := map[string]int{}
	for _, entry := range entries {
		sector := sectorMap[entry.Sector]
		if sector == nil {
			sector = &sectorEntry{Sector: entry.Sector, Super: superBySector[entry.Sector]}
			sectorMap[entry.Sector] = sector
		}
		sector.N++
		sector.AUM += entry.AUM
		supers[sector.Super]++
	}
	sectors := make([]sectorEntry, 0, len(sectorMap))
	for _, sector := range sectorMap {
		sectors = append(sectors, *sector)
	}
	sort.Slice(sectors, func(i, j int) bool { return sectors[i].AUM > sectors[j].AUM })
	metadataAsOf := input.MetadataAsOf
	if metadataAsOf == "" {
		metadataAsOf = input.Updated
	}
	return analysesFile{
		Updated: updated, AUMUnit: "usd", N: len(entries), Supers: supers, Sectors: sectors, ETFs: entries,
		Source: "yahoo-chart+local-etf-metadata", SourceURL: "https://query1.finance.yahoo.com/v8/finance/chart/",
		MetadataAsOf: metadataAsOf,
		Scope:        map[string]interface{}{"kind": "top-aum", "requested": len(entries), "completed": len(entries), "complete": true},
	}
}

func (s *Service) writeCurrent(output analysesFile) error {
	path := filepath.Join(s.root, "data", "etf-analyses-current.json")
	archive := filepath.Join(s.root, "data", "etf-history", output.Updated+".json")
	if err := atomicWriteJSON(archive, output); err != nil {
		return fmt.Errorf("archive ETF last-good: %w", err)
	}
	if err := atomicWriteJSON(path, output); err != nil {
		return fmt.Errorf("promote ETF last-good: %w", err)
	}
	return nil
}

func (s *Service) writeStatus(status RefreshStatus) error {
	return atomicWriteJSON(filepath.Join(s.root, "data", "etf-refresh-status.json"), status)
}

func atomicWriteJSON(path string, value interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".etf-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func round(value float64) float64 {
	return math.Round(value*100) / 100
}
