package market

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"trading-agents/internal/dataflows"
)

const (
	liveRefreshBatchSize     = 30 // TradingView scanner 对 tickers 批量有上限，过大只返回部分
	liveRefreshWorkers       = 8
	liveRefreshBatchFor      = 30 * time.Second
	liveRefreshOverallFor    = 2 * time.Minute
	liveRefreshInterval      = 45 * time.Second
	broadRefreshInterval     = 15 * time.Minute
	liveFreshFor             = 90 * time.Second
	providerBreakerThreshold = 3
	providerBreakerCooldown  = 2 * time.Minute
)

type usRefreshBatchFetcher func(context.Context, []string) map[string]Quote
type usRefreshTVFetcher func([]string) ([]dataflows.TVQuoteData, error)
type usRefreshYahooFetcher func([]string) (map[string]Quote, error)
type usRefreshSnapshotWriter func(map[string]snapshotQuote, time.Time) error
type usRefreshFilePatcher func(map[string]snapshotQuote, time.Time, bool) error
type cnRefreshFetcher func([]string) ([]dataflows.CNQuote, error)

type usRefreshBatchResult struct {
	quotes map[string]Quote
}

var (
	liveLoopOnce sync.Once
	liveRunning  atomic.Bool
)

// PayloadStatus is the provenance used by market payloads and health reporting.
type PayloadStatus struct {
	Source          string
	DataTime        time.Time
	TimeGranularity string
	Stale           bool
	Market          string
	Session         string
	ProviderURL     string
}

// StartLiveRefresh launches a background loop that keeps US quotes fresh via TV/Yahoo.
// Safe to call multiple times; only one loop runs per process.
func (p *Provider) StartLiveRefresh() {
	liveLoopOnce.Do(func() {
		go p.liveLoop()
	})
}

func (p *Provider) liveLoop() {
	liveRunning.Store(true)
	defer liveRunning.Store(false)

	// Keep the core US universe inside the 90-second freshness window. The
	// remaining universe is fetched on demand by /api/quote.
	if n, err := p.RefreshUSQuotes(liveRefreshBatchSize); err != nil {
		log.Printf("[market-live] core US refresh failed: %v", err)
	} else {
		log.Printf("[market-live] core US refresh updated %d symbols", n)
	}
	if n, err := p.RefreshCNQuotes(500); err != nil {
		log.Printf("[market-live-cn] top refresh failed: %v", err)
	} else {
		log.Printf("[market-live-cn] top refresh updated %d symbols", n)
	}
	if n, err := p.RefreshCNPriorityQuotes(); err != nil {
		log.Printf("[market-live-cn] priority refresh failed: %v", err)
	} else if n > 0 {
		log.Printf("[market-live-cn] priority refresh updated %d symbols", n)
	}

	hotTicker := time.NewTicker(liveRefreshInterval)
	broadTicker := time.NewTicker(broadRefreshInterval)
	defer hotTicker.Stop()
	defer broadTicker.Stop()
	for {
		select {
		case <-hotTicker.C:
			_, _ = p.RefreshUSQuotes(liveRefreshBatchSize)
			_, _ = p.RefreshCNQuotes(500)
			_, _ = p.RefreshCNPriorityQuotes()
		case <-broadTicker.C:
			// Broad cycles improve the persisted universe, but partial cycles never
			// advance the complete-snapshot timestamp or become market-wide live data.
			if n, err := p.RefreshUSQuotes(0); err != nil {
				log.Printf("[market-broad] US refresh incomplete: updated=%d err=%v", n, err)
			}
			if n, err := p.RefreshCNQuotes(0); err != nil {
				log.Printf("[market-broad] CN refresh incomplete: updated=%d err=%v", n, err)
			}
		}
	}
}

// LiveUpdatedAt returns when the in-memory US snap was last successfully live-refreshed.
func (p *Provider) LiveUpdatedAt() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.liveAt
}

// USPayloadStatus reports the same source/data-time decision as LiveUSPayload without starting refresh work.
func (p *Provider) USPayloadStatus(now time.Time) PayloadStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	quotes := freshUSLiveSnapshot(p.usLiveSnap, now)
	usingLiveSubset := len(quotes) > 0
	if len(quotes) == 0 {
		quotes = p.usSnap
	}
	status := usPayloadStatus(quotes, p.usLiveSource, p.usSnapshotAt, now)
	if usingLiveSubset {
		status = failClosedOnPartialUSCoverage(status, len(quotes), len(p.usSnap))
	}
	return status
}

func failClosedOnPartialUSCoverage(status PayloadStatus, observed, expected int) PayloadStatus {
	if observed <= 0 || expected <= 0 || observed >= expected {
		return status
	}
	status.Stale = true
	status.Source = "stale-snapshot:us-partial"
	if !status.DataTime.IsZero() {
		status.Source += "@" + formatProviderObservationTime(status.DataTime, status.TimeGranularity)
	}
	return status
}

func freshUSLiveSnapshot(quotes map[string]snapshotQuote, now time.Time) map[string]snapshotQuote {
	if exchangeSessionAt("AAPL", now) == "closed" {
		return cloneSnap(quotes)
	}
	fresh := make(map[string]snapshotQuote, len(quotes))
	for symbol, quote := range quotes {
		observed, err := parseProviderObservationTime(quote.DataTime)
		if err != nil || observed.IsZero() || now.Before(observed) || now.Sub(observed) > liveFreshFor {
			continue
		}
		fresh[symbol] = quote
	}
	return fresh
}

func usPayloadStatus(quotes map[string]snapshotQuote, provider string, fallbackAt, now time.Time) PayloadStatus {
	dataAt, granularity, complete := snapshotObservationTimeWithGranularity(quotes, fallbackAt)
	if complete {
		status := payloadStatus("us", provider, "", dataAt, fallbackAt, now)
		status.TimeGranularity = granularity
		status.Source = sourceWithObservationTime(status.Source, dataAt, granularity)
		return status
	}
	status := payloadStatus("us", provider, "", time.Time{}, dataAt, now)
	status.Source = "stale-snapshot:us-incomplete"
	if !dataAt.IsZero() {
		status.Source += "@" + dataAt.UTC().Format(time.RFC3339)
	}
	return status
}

func snapshotObservationTime(quotes map[string]snapshotQuote, fallbackAt time.Time) (time.Time, bool) {
	observed, _, complete := snapshotObservationTimeWithGranularity(quotes, fallbackAt)
	return observed, complete
}

func snapshotObservationTimeWithGranularity(quotes map[string]snapshotQuote, fallbackAt time.Time) (time.Time, string, bool) {
	if len(quotes) == 0 {
		return fallbackAt, "", false
	}
	oldest := time.Time{}
	granularity := "second"
	complete := true
	for _, quote := range quotes {
		observed, err := parseProviderObservationTime(quote.DataTime)
		if err != nil || observed.IsZero() {
			complete = false
			observed = fallbackAt
		}
		if observed.IsZero() {
			continue
		}
		quoteGranularity := quote.TimeGranularity
		if quoteGranularity == "" && len(quote.DataTime) == len("2006-01-02") {
			quoteGranularity = "date"
		}
		if oldest.IsZero() || observed.Before(oldest) || observed.Equal(oldest) && quoteGranularity == "date" {
			oldest = observed
			granularity = quoteGranularity
		}
	}
	return oldest, granularity, complete && !oldest.IsZero()
}

func sourceWithObservationTime(source string, observed time.Time, granularity string) string {
	if observed.IsZero() || !strings.Contains(source, "@") {
		return source
	}
	return strings.SplitN(source, "@", 2)[0] + "@" + formatProviderObservationTime(observed, granularity)
}

func payloadStatus(kind, provider, providerURL string, liveAt, snapshotAt, now time.Time) PayloadStatus {
	sessionSymbol := "AAPL"
	if kind == "cn" || kind == "a" {
		sessionSymbol = "600519"
	}
	session := exchangeSessionAt(sessionSymbol, now)
	if !liveAt.IsZero() {
		if provider == "" {
			provider = map[string]string{"us": "unknown", "cn": "eastmoney-cn"}[kind]
		}
		if session == "closed" {
			return PayloadStatus{Source: fmt.Sprintf("closed-%s@%s", provider, liveAt.UTC().Format(time.RFC3339)), DataTime: liveAt, Stale: true, Market: kind, Session: session, ProviderURL: providerURL}
		}
		stale := now.Before(liveAt) || now.Sub(liveAt) > liveFreshFor
		if !stale {
			return PayloadStatus{Source: fmt.Sprintf("live-%s@%s", provider, liveAt.UTC().Format(time.RFC3339)), DataTime: liveAt, Market: kind, Session: session, ProviderURL: providerURL}
		}
		return PayloadStatus{Source: fmt.Sprintf("stale-snapshot:%s@%s", kind, liveAt.UTC().Format(time.RFC3339)), DataTime: liveAt, Stale: true, Market: kind, Session: session, ProviderURL: providerURL}
	}
	source := "stale-snapshot:" + kind
	if kind == "us" {
		source = "stale-snapshot:us-stocks"
	} else if kind == "cn" {
		source = "stale-snapshot:a-market"
	}
	if !snapshotAt.IsZero() {
		source += "@" + snapshotAt.UTC().Format(time.RFC3339)
	}
	return PayloadStatus{Source: source, DataTime: snapshotAt, Stale: true, Market: kind, Session: session}
}

// RefreshUSQuotes pulls live prices for the US universe into memory (and market.json).
// limit<=0 means full universe; otherwise only the top-N by market cap (approx via mcapB).
// Batches are applied immediately so /api/market becomes live after the first chunk.
func (p *Provider) RefreshUSQuotes(limit int) (int, error) {
	if !p.usRefreshing.CompareAndSwap(false, true) {
		return 0, fmt.Errorf("US quote refresh already running")
	}
	defer p.usRefreshing.Store(false)
	return p.refreshUSQuotesUnlocked(limit)
}

func (p *Provider) refreshUSQuotesUnlocked(limit int) (int, error) {
	p.mu.RLock()
	base := cloneSnap(p.usSnap)
	p.mu.RUnlock()
	if len(base) == 0 {
		p.ReloadSnapshots()
		p.mu.RLock()
		base = cloneSnap(p.usSnap)
		p.mu.RUnlock()
	}
	if len(base) == 0 {
		return 0, fmt.Errorf("empty US snapshot; cannot live-refresh")
	}

	syms := make([]string, 0, len(base))
	for sym := range base {
		syms = append(syms, sym)
	}
	sort.Slice(syms, func(i, j int) bool {
		return base[syms[i]].McapB > base[syms[j]].McapB
	})
	if limit > 0 && limit < len(syms) {
		syms = syms[:limit]
	}
	targetSymbols := make(map[string]struct{}, len(syms))
	for _, symbol := range syms {
		targetSymbols[symbol] = struct{}{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), liveRefreshOverallFor)
	defer cancel()
	jobs := make(chan []string, liveRefreshWorkers)
	results := make(chan usRefreshBatchResult, liveRefreshWorkers)
	var workers sync.WaitGroup
	workerCount := min(liveRefreshWorkers, (len(syms)+liveRefreshBatchSize-1)/liveRefreshBatchSize)
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for {
				if ctx.Err() != nil {
					return
				}
				select {
				case <-ctx.Done():
					return
				case chunk, ok := <-jobs:
					if !ok {
						return
					}
					if ctx.Err() != nil {
						return
					}
					batchCtx, batchCancel := context.WithTimeout(ctx, liveRefreshBatchFor)
					live := p.fetchUSRefreshBatch(batchCtx, chunk)
					batchCancel()
					select {
					case results <- usRefreshBatchResult{quotes: live}:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for i := 0; i < len(syms); i += liveRefreshBatchSize {
			j := i + liveRefreshBatchSize
			if j > len(syms) {
				j = len(syms)
			}
			chunk := append([]string(nil), syms[i:j]...)
			select {
			case jobs <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()

	updated := 0
	observedAt := time.Time{}
	for result := range results {
		p.mu.Lock()
		for sym, q := range result.quotes {
			if _, requested := targetSymbols[sym]; !requested {
				continue
			}
			if q.Price <= 0 || strings.HasPrefix(q.Source, "stale-snapshot:") {
				continue
			}
			quoteAt, err := parseProviderObservationTime(q.DataTime)
			if err != nil || quoteAt.IsZero() || quoteAt.After(p.now().Add(time.Minute)) {
				continue
			}
			prev := p.usSnap[sym]
			if prev.Price <= 0 {
				prev = base[sym]
			}
			mcap := prev.McapB
			if prev.Price > 0 && q.Price > 0 && mcap > 0 {
				mcap = mcap * (q.Price / prev.Price)
			}
			current := snapshotQuote{
				Price:           q.Price,
				Pct:             q.Pct,
				Vol:             prev.Vol,
				McapB:           mcap,
				Source:          q.Source,
				DataTime:        formatProviderObservationTime(quoteAt, q.TimeGranularity),
				TimeGranularity: q.TimeGranularity,
				Session:         q.Session,
				ProviderURL:     q.ProviderURL,
				ProviderMode:    q.ProviderMode,
			}
			p.usSnap[sym] = current
			if p.usLiveSnap == nil {
				p.usLiveSnap = make(map[string]snapshotQuote)
			}
			p.usLiveSnap[sym] = current
			updated++
			if quoteAt.After(observedAt) {
				observedAt = quoteAt
			}
			if p.usLiveSource == "" {
				p.usLiveSource = q.Source
			} else if p.usLiveSource != q.Source {
				p.usLiveSource = "mixed"
			}
		}
		if observedAt.After(p.liveAt) {
			p.liveAt = observedAt
		}
		p.mu.Unlock()
	}

	p.mu.Lock()
	final := cloneSnap(p.usSnap)
	finalDataAt, complete := snapshotObservationTime(final, p.usSnapshotAt)
	complete = complete && len(syms) == len(base) && updated == len(syms)
	if complete {
		p.usSnapshotAt = finalDataAt
	}
	p.mu.Unlock()
	p.refreshMu.Lock()
	writeSnapshot := p.usSnapWrite
	if writeSnapshot == nil {
		writeSnapshot = writeMarketSnapshotFile
	}
	patchStocks := p.usFilePatch
	if patchStocks == nil {
		patchStocks = patchUsStocksPrices
	}
	_ = writeSnapshot(final, finalDataAt)
	_ = patchStocks(final, finalDataAt, complete)
	p.refreshMu.Unlock()
	if updated == 0 {
		return 0, fmt.Errorf("live US refresh returned no live quotes")
	}
	if updated != len(syms) {
		return updated, fmt.Errorf("live US refresh incomplete: %d/%d", updated, len(syms))
	}
	if limit <= 0 && !complete {
		return updated, fmt.Errorf("live US full refresh incomplete: %d/%d", updated, len(base))
	}
	return updated, nil
}

func (p *Provider) fetchUSRefreshBatch(ctx context.Context, symbols []string) map[string]Quote {
	if p.usBatchFetch != nil {
		return p.usBatchFetch(ctx, symbols)
	}
	out := make(map[string]Quote, len(symbols))
	requested := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		requested[symbol] = struct{}{}
	}
	tvFetch := p.usTVFetch
	if tvFetch == nil {
		tvFetch = p.tv.GetRealTimeQuotes
	}
	var items []dataflows.TVQuoteData
	var err error
	if p.providerAllowed("tradingview") {
		items, err = tvFetch(symbols)
		p.providerResult("tradingview", err == nil && len(items) > 0)
	}
	if err == nil {
		for _, item := range items {
			base := item.Symbol
			if idx := strings.LastIndex(base, ":"); idx >= 0 {
				base = base[idx+1:]
			}
			base = strings.ToUpper(base)
			if _, ok := requested[base]; !ok {
				continue
			}
			if item.Price <= 0 || item.DataTime.IsZero() {
				continue
			}
			session := exchangeSessionAt(base, p.now())
			out[base] = Quote{Price: item.Price, Pct: item.ChangePct, PrevClose: item.PrevClose, Source: "tradingview", Session: session,
				DataTime: item.DataTime.UTC().Format(time.RFC3339), TimeGranularity: "second", SourceSession: session,
				ProviderURL: item.SourceURL, ProviderMode: "display-only:" + item.UpdateMode}
		}
	}
	if ctx.Err() != nil || len(out) == len(symbols) {
		return out
	}
	yahooSymbols := make([]string, 0, len(symbols)-len(out))
	back := make(map[string]string, len(symbols)-len(out))
	for _, symbol := range symbols {
		if _, ok := out[symbol]; ok {
			continue
		}
		yahooSymbol := toYahooSymbol(symbol)
		yahooSymbols = append(yahooSymbols, yahooSymbol)
		back[yahooSymbol] = symbol
	}
	yahooFetch := p.usYahooFetch
	if yahooFetch == nil {
		yahooFetch = p.yahooQuoteAPI
	}
	if p.providerAllowed("yahoo") {
		yahoo, err := yahooFetch(yahooSymbols)
		p.providerResult("yahoo", err == nil && len(yahoo) > 0)
		if err == nil {
			for yahooSymbol, quote := range yahoo {
				if symbol, ok := back[yahooSymbol]; ok {
					out[symbol] = quote
				}
			}
		}
	}
	return out
}

// providerAllowed implements a small per-provider circuit breaker. A failed
// batch is counted once; after three failures the source is skipped briefly,
// then one half-open request is allowed to test recovery.
func (p *Provider) providerAllowed(name string) bool {
	p.providerBreakerMu.Lock()
	defer p.providerBreakerMu.Unlock()
	if p.providerBreakers == nil {
		p.providerBreakers = make(map[string]*providerBreaker)
	}
	b := p.providerBreakers[name]
	if b == nil {
		b = &providerBreaker{}
		p.providerBreakers[name] = b
	}
	if b.openUntil.IsZero() {
		return true
	}
	if p.now().Before(b.openUntil) {
		return false
	}
	// Half-open: reserve the probe until the next cooldown window.
	b.openUntil = p.now().Add(providerBreakerCooldown)
	return true
}

func (p *Provider) providerResult(name string, success bool) {
	p.providerBreakerMu.Lock()
	defer p.providerBreakerMu.Unlock()
	if p.providerBreakers == nil {
		p.providerBreakers = make(map[string]*providerBreaker)
	}
	b := p.providerBreakers[name]
	if b == nil {
		b = &providerBreaker{}
		p.providerBreakers[name] = b
	}
	if success {
		b.failures = 0
		b.openUntil = time.Time{}
		return
	}
	b.failures++
	if b.failures >= providerBreakerThreshold {
		b.openUntil = p.now().Add(providerBreakerCooldown)
	}
}

// LiveUSPayload returns the current in-memory US quotes without starting refresh work.
func (p *Provider) LiveUSPayload() ([]byte, string, error) {
	p.mu.RLock()
	liveSource := p.usLiveSource
	snapshotAt := p.usSnapshotAt
	snap := freshUSLiveSnapshot(p.usLiveSnap, p.now())
	usingLiveSubset := len(snap) > 0
	expectedCount := len(p.usSnap)
	if len(snap) == 0 {
		snap = cloneSnap(p.usSnap)
	}
	p.mu.RUnlock()

	if len(snap) == 0 {
		return SnapshotPayload("us")
	}

	status := usPayloadStatus(snap, liveSource, snapshotAt, p.now())
	if usingLiveSubset {
		status = failClosedOnPartialUSCoverage(status, len(snap), expectedCount)
	}
	raw, err := marshalMarketPayload(snap, status)
	if err != nil {
		return nil, "", err
	}

	return raw, status.Source, nil
}

func cloneSnap(in map[string]snapshotQuote) map[string]snapshotQuote {
	out := make(map[string]snapshotQuote, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// SnapshotQuote is a lightweight copy of one in-memory market quote.
type SnapshotQuote struct {
	Price  float64
	Pct    float64
	Vol    float64
	McapB  float64
	McapYi float64
}

// USQuotesCopy returns a copy of the US in-memory snap without JSON round-trip.
func (p *Provider) USQuotesCopy() map[string]SnapshotQuote {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[string]SnapshotQuote, len(p.usSnap))
	for k, v := range p.usSnap {
		out[k] = SnapshotQuote{Price: v.Price, Pct: v.Pct, Vol: v.Vol, McapB: v.McapB, McapYi: v.McapYi}
	}
	return out
}

// CNQuotesCopy returns a copy of the CN in-memory snap without JSON round-trip.
func (p *Provider) CNQuotesCopy() map[string]SnapshotQuote {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[string]SnapshotQuote, len(p.cnSnap))
	for k, v := range p.cnSnap {
		out[k] = SnapshotQuote{Price: v.Price, Pct: v.Pct, Vol: v.Vol, McapB: v.McapB, McapYi: v.McapYi}
	}
	return out
}

// patchUsStocksPrices updates price/pct/mcap/vol on disk when the public file is writable.
// Desktop embed FS is read-only; web mode benefits so /data/us-stocks stays aligned.
func patchUsStocksPrices(quotes map[string]snapshotQuote, dataAt time.Time, complete bool) error {
	raw, path, err := readUsStocksFile()
	if err != nil {
		return err
	}
	var file map[string]any
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	stocks, ok := file["stocks"].([]any)
	if !ok {
		return fmt.Errorf("us-stocks stocks array missing")
	}
	changed := 0
	for _, rowAny := range stocks {
		row, ok := rowAny.(map[string]any)
		if !ok {
			continue
		}
		sym, _ := row["sym"].(string)
		sym = strings.ToUpper(strings.TrimSpace(sym))
		q, ok := quotes[sym]
		if !ok || q.Price <= 0 {
			continue
		}
		row["price"] = q.Price
		row["pct"] = q.Pct
		if q.McapB > 0 {
			row["mcapB"] = q.McapB
		}
		if q.Vol > 0 {
			row["vol"] = q.Vol
		}
		changed++
	}
	if changed == 0 {
		return nil
	}
	if complete && !dataAt.IsZero() {
		file["generated_at"] = dataAt.UTC().Format(time.RFC3339)
	}
	file["count"] = len(stocks)
	out, err := json.Marshal(file)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return err
	}
	dist := strings.Replace(path, "public/data/us-stocks.json", "dist/data/us-stocks.json", 1)
	if dist != path {
		_ = os.WriteFile(dist, out, 0o644)
	}
	return nil
}
