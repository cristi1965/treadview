package market

import (
	"fmt"
	"log"
	"sort"
	"time"
)

// RefreshCNQuotes pulls A-share prices via Eastmoney into cnSnap (+ optional a-market.json).
func (p *Provider) RefreshCNQuotes(limit int) (int, error) {
	if !p.cnRefreshing.CompareAndSwap(false, true) {
		return 0, fmt.Errorf("CN quote refresh already running")
	}
	defer p.cnRefreshing.Store(false)
	p.mu.RLock()
	base := cloneSnap(p.cnSnap)
	p.mu.RUnlock()
	if len(base) == 0 {
		p.ReloadSnapshots()
		p.mu.RLock()
		base = cloneSnap(p.cnSnap)
		p.mu.RUnlock()
	}
	if len(base) == 0 {
		return 0, fmt.Errorf("empty CN snapshot")
	}
	if p.cnFetch == nil {
		return 0, fmt.Errorf("cn quote client nil")
	}

	baseCount := len(base)
	syms := make([]string, 0, len(base))
	for sym := range base {
		syms = append(syms, sym)
	}
	sort.Slice(syms, func(i, j int) bool {
		return base[syms[i]].McapYi > base[syms[j]].McapYi
	})
	if limit > 0 && limit < len(syms) {
		syms = syms[:limit]
	}

	updated := 0
	oldestObserved := time.Time{}
	const batch = 80
	for i := 0; i < len(syms); i += batch {
		j := i + batch
		if j > len(syms) {
			j = len(syms)
		}
		items, err := p.cnFetch(syms[i:j])
		if err != nil {
			log.Printf("[market-live-cn] batch failed: %v", err)
			continue
		}
		p.cnRefreshMu.Lock()
		p.mu.Lock()
		for _, q := range items {
			if q.Price <= 0 || q.ObservedAt.IsZero() {
				continue
			}
			prev := p.cnSnap[q.Symbol]
			mcap := q.McapYi
			if mcap <= 0 {
				mcap = prev.McapYi
			}
			session := exchangeSessionAt(q.Symbol, q.ObservedAt)
			p.cnSnap[q.Symbol] = snapshotQuote{
				Price:  q.Price,
				Pct:    q.Pct,
				Vol:    q.Vol,
				McapYi: mcap,
				Source: "eastmoney-cn", DataTime: q.ObservedAt.UTC().Format(time.RFC3339), TimeGranularity: "second", Session: session,
			}
			if oldestObserved.IsZero() || q.ObservedAt.Before(oldestObserved) {
				oldestObserved = q.ObservedAt
			}
			updated++
		}
		p.mu.Unlock()
		p.cnRefreshMu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	if updated == 0 {
		return 0, fmt.Errorf("live CN refresh returned no live quotes")
	}
	if updated != len(syms) || oldestObserved.IsZero() {
		return updated, fmt.Errorf("live CN refresh incomplete: %d/%d", updated, len(syms))
	}
	// A hot subset updates individual in-memory quotes, but it is not a
	// market-wide observation. Only a complete universe refresh can advance the
	// global freshness timestamp or publish the combined snapshot to disk.
	if len(syms) != baseCount {
		return updated, nil
	}
	p.cnRefreshMu.Lock()
	p.mu.Lock()
	p.cnLiveAt = oldestObserved
	p.cnSnapshotAt = oldestObserved
	snap := cloneSnap(p.cnSnap)
	p.mu.Unlock()
	writeErr := writeAMarketSnapshotFileAt(snap, oldestObserved)
	p.cnRefreshMu.Unlock()
	if writeErr != nil {
		return updated, writeErr
	}
	return updated, nil
}

// cnPrioritySymbols are toolbox ETFs + conviction seeds that may be absent from a-market.
var cnPrioritySymbols = []string{
	"510300", "510500", "159915", "512480", "515790", "516160", "512100",
	"588000", "159819", "512760",
	"600519", "300750", "688981", "002371", "688041", "300308", "601318", "600036", "002594", "601899",
}

// RefreshCNPriorityQuotes always refreshes the A-share toolbox watchlist (HA for /api/cn/*).
func (p *Provider) RefreshCNPriorityQuotes() (int, error) {
	if p.cnFetch == nil {
		return 0, fmt.Errorf("cn quote client nil")
	}
	p.EnsureCNSymbols(cnPrioritySymbols)
	items, err := p.cnFetch(cnPrioritySymbols)
	if err != nil {
		return 0, err
	}
	updated := 0
	oldestObserved := time.Time{}
	p.cnRefreshMu.Lock()
	p.mu.Lock()
	for _, q := range items {
		if q.Price <= 0 || q.ObservedAt.IsZero() {
			continue
		}
		prev := p.cnSnap[q.Symbol]
		mcap := q.McapYi
		if mcap <= 0 {
			mcap = prev.McapYi
		}
		session := exchangeSessionAt(q.Symbol, q.ObservedAt)
		p.cnSnap[q.Symbol] = snapshotQuote{Price: q.Price, Pct: q.Pct, Vol: q.Vol, McapYi: mcap, Source: "eastmoney-cn", DataTime: q.ObservedAt.UTC().Format(time.RFC3339), TimeGranularity: "second", Session: session}
		if oldestObserved.IsZero() || q.ObservedAt.Before(oldestObserved) {
			oldestObserved = q.ObservedAt
		}
		updated++
	}
	p.mu.Unlock()
	p.cnRefreshMu.Unlock()
	complete := updated == len(cnPrioritySymbols)
	if !complete {
		return updated, fmt.Errorf("priority CN refresh incomplete: %d/%d", updated, len(cnPrioritySymbols))
	}
	return updated, nil
}

// CNLiveUpdatedAt returns last successful CN live refresh time.
func (p *Provider) CNLiveUpdatedAt() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cnLiveAt
}

// CNPayloadStatus reports the same source/data-time decision as LiveCNPayload without starting refresh work.
func (p *Provider) CNPayloadStatus(now time.Time) PayloadStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return payloadStatus("cn", "eastmoney-cn", "", p.cnLiveAt, p.cnSnapshotAt, now)
}

// LiveCNPayload serves /api/a-market from memory (seed or live).
func (p *Provider) LiveCNPayload() ([]byte, string, error) {
	p.mu.RLock()
	liveAt := p.cnLiveAt
	snapshotAt := p.cnSnapshotAt
	snap := cloneSnap(p.cnSnap)
	p.mu.RUnlock()
	if len(snap) == 0 {
		return SnapshotPayload("cn")
	}
	if !liveAt.IsZero() {
		verified := make(map[string]snapshotQuote, len(snap))
		for symbol, quote := range snap {
			observedAt, err := parseProviderObservationTime(quote.DataTime)
			if err == nil && !observedAt.IsZero() && quote.Price > 0 && quote.Source == "eastmoney-cn" {
				verified[symbol] = quote
			}
		}
		if len(verified) != len(snap) {
			liveAt = time.Time{}
		}
		snap = verified
	}

	status := payloadStatus("cn", "eastmoney-cn", "", liveAt, snapshotAt, time.Now())
	raw, err := marshalMarketPayload(snap, status)
	if err != nil {
		return nil, "", err
	}
	return raw, status.Source, nil
}

func writeAMarketSnapshotFileAt(quotes map[string]snapshotQuote, observedAt time.Time) error {
	return writeSnapshotFileAt(quotes, "a-market.json", observedAt)
}
