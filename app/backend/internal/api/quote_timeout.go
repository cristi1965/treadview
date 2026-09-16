package api

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"trading-agents/internal/market"
)

const (
	publicQuoteTimeout    = 4 * time.Second
	maxPublicQuoteSymbols = 64
	primaryQuoteGrace     = 150 * time.Millisecond
	alpacaQuoteGrace      = 2 * time.Second
)

type quoteFlight struct {
	done   chan struct{}
	result quoteFetchResult
}

var quoteFlights = struct {
	sync.Mutex
	values map[string]*quoteFlight
}{values: make(map[string]*quoteFlight)}
var boundedQuotesForRequest = boundedQuotesWithMeta
var quoteRequestTimeout = publicQuoteTimeout
var quoteProviderFetch = func(provider *market.Provider, symbols []string) map[string]market.Quote {
	return provider.Quotes(symbols)
}
var quoteSecondaryFetch = func(ctx context.Context, provider *market.Provider, symbols []string) map[string]market.Quote {
	return provider.SecondaryFastQuotes(ctx, symbols)
}
var quotePrimaryGrace = primaryQuoteGraceFor

type quoteFetchResult struct {
	quotes          map[string]market.Quote
	timedOut        bool
	dataTime        time.Time
	dataTimeLabel   string
	timeGranularity string
	source          string
}

func boundedQuotes(symbols []string) (map[string]market.Quote, bool) {
	result := boundedQuotesWithMeta(symbols)
	return result.quotes, result.timedOut
}

func boundedQuotesWithMeta(symbols []string) quoteFetchResult {
	p := market.Default()
	keyParts := append([]string(nil), symbols...)
	for index := range keyParts {
		keyParts[index] = strings.ToUpper(strings.TrimSpace(keyParts[index]))
	}
	sort.Strings(keyParts)
	key := strings.Join(keyParts, ",")
	quoteFlights.Lock()
	if current := quoteFlights.values[key]; current != nil {
		quoteFlights.Unlock()
		select {
		case <-current.done:
			return current.result
		case <-time.After(quoteRequestTimeout):
			return snapshotQuoteFetchResult(p, symbols, true)
		}
	}
	flight := &quoteFlight{done: make(chan struct{})}
	quoteFlights.values[key] = flight
	quoteFlights.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), quoteRequestTimeout)
	defer cancel()
	type quoteCandidate struct {
		primary bool
		result  quoteFetchResult
	}
	primaryFetch := quoteProviderFetch
	secondaryFetch := quoteSecondaryFetch
	done := make(chan quoteCandidate, 2)
	go func() {
		quotes := primaryFetch(p, symbols)
		done <- quoteCandidate{primary: true, result: newQuoteFetchResult(p, quotes, false, time.Now())}
	}()
	go func() {
		quotes := secondaryFetch(ctx, p, symbols)
		done <- quoteCandidate{result: newQuoteFetchResult(p, quotes, false, time.Now())}
	}()

	var freshSecondary *quoteFetchResult
	var stalePrimary *quoteFetchResult
	var staleSecondary *quoteFetchResult
	var grace <-chan time.Time
	remaining := 2
	primaryDone := false
	retainFlight := false
	for remaining > 0 {
		select {
		case candidate := <-done:
			remaining--
			if candidate.primary {
				primaryDone = true
			}
			if !completeTrustedQuoteResult(symbols, candidate.result) {
				if remaining == 0 {
					flight.result = preferredQuoteFallback(freshSecondary, stalePrimary, staleSecondary)
				}
				continue
			}
			stale, _ := quoteResultFreshness(candidate.result, time.Now())
			if stale {
				copy := candidate.result
				if candidate.primary {
					stalePrimary = &copy
				} else {
					staleSecondary = &copy
				}
				if remaining == 0 {
					flight.result = preferredQuoteFallback(freshSecondary, stalePrimary, staleSecondary)
				}
				continue
			}
			if candidate.primary {
				flight.result = candidate.result
				remaining = 0
				continue
			}
			copy := candidate.result
			freshSecondary = &copy
			if remaining == 0 {
				flight.result = copy
				continue
			}
			grace = time.After(quotePrimaryGrace(p))
		case <-grace:
			flight.result = *freshSecondary
			remaining = 0
		case <-ctx.Done():
			flight.result = preferredQuoteFallback(freshSecondary, stalePrimary, staleSecondary)
			retainFlight = !primaryDone
			remaining = 0
		}
	}
	if flight.result.quotes == nil {
		flight.result = snapshotQuoteFetchResult(p, symbols, true)
	}
	result := flight.result
	cleanup := func() {
		close(flight.done)
		quoteFlights.Lock()
		if quoteFlights.values[key] == flight {
			delete(quoteFlights.values, key)
		}
		quoteFlights.Unlock()
	}
	if retainFlight {
		go func() {
			for {
				candidate := <-done
				if candidate.primary {
					cleanup()
					return
				}
			}
		}()
	} else {
		cleanup()
	}
	return result
}

func primaryQuoteGraceFor(provider *market.Provider) time.Duration {
	if provider != nil && provider.AlpacaConfigured() {
		return alpacaQuoteGrace
	}
	return primaryQuoteGrace
}

func preferredQuoteFallback(freshSecondary, stalePrimary, staleSecondary *quoteFetchResult) quoteFetchResult {
	if freshSecondary != nil {
		return *freshSecondary
	}
	if stalePrimary != nil {
		return *stalePrimary
	}
	if staleSecondary != nil {
		return *staleSecondary
	}
	return quoteFetchResult{}
}

func completeTrustedQuoteResult(symbols []string, result quoteFetchResult) bool {
	seen := make(map[string]bool, len(symbols))
	for _, raw := range symbols {
		symbol := strings.ToUpper(strings.TrimSpace(raw))
		if symbol == "" || seen[symbol] {
			continue
		}
		seen[symbol] = true
		quote, ok := result.quotes[symbol]
		if !ok || quote.Price <= 0 || strings.TrimSpace(quote.Source) == "" || isStaleQuoteSource(quote.Source) {
			return false
		}
		if _, _, err := parseQuoteObservation(quote.DataTime, quote.TimeGranularity); err != nil {
			return false
		}
	}
	return len(seen) > 0
}

func newQuoteFetchResult(p *market.Provider, quotes map[string]market.Quote, timedOut bool, fetchedAt time.Time) quoteFetchResult {
	_ = p
	_ = fetchedAt // completion time is intentionally not evidence of quote freshness
	result := quoteFetchResult{quotes: quotes, timedOut: timedOut, source: "self-provider"}
	oldestObserved := time.Time{}
	liveSource := ""
	missingProviderTime := false
	for _, quote := range quotes {
		if isStaleQuoteSource(quote.Source) {
			result.source = "stale-snapshot:provider-observations"
		}
		if quote.DataTime != "" {
			observed, granularity, err := parseQuoteObservation(quote.DataTime, quote.TimeGranularity)
			if err == nil && (oldestObserved.IsZero() || observed.Before(oldestObserved) || observed.Equal(oldestObserved) && granularity == "date") {
				oldestObserved = observed
				result.dataTimeLabel = quote.DataTime
				result.timeGranularity = granularity
			} else if err != nil {
				missingProviderTime = true
			}
		} else {
			missingProviderTime = true
		}
		if quote.Source != "" && !isStaleQuoteSource(quote.Source) {
			if liveSource == "" {
				liveSource = quote.Source
			} else if liveSource != quote.Source {
				liveSource = "self-provider:mixed"
			}
		}
	}
	if !oldestObserved.IsZero() {
		result.dataTime = oldestObserved
	}
	if liveSource != "" && !strings.HasPrefix(result.source, "stale-snapshot:") {
		result.source = liveSource
	}
	if missingProviderTime {
		result.source = "unverified-provider-time"
		result.dataTime = time.Time{}
		result.dataTimeLabel = ""
		result.timeGranularity = ""
	}
	return result
}

func parseQuoteObservation(value, declaredGranularity string) (time.Time, string, error) {
	value = strings.TrimSpace(value)
	granularity := strings.TrimSpace(declaredGranularity)
	if len(value) == len("2006-01-02") {
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil || (granularity != "" && granularity != "date") {
			return time.Time{}, "", fmt.Errorf("invalid date-only quote observation")
		}
		return parsed, "date", nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil || granularity == "date" {
		return time.Time{}, "", fmt.Errorf("invalid timestamped quote observation")
	}
	return parsed, "second", nil
}

func snapshotQuoteFetchResult(p *market.Provider, symbols []string, timedOut bool) quoteFetchResult {
	return newQuoteFetchResult(p, p.SnapshotQuotes(symbols), timedOut, time.Time{})
}

func isStaleQuoteSource(source string) bool {
	return len(source) >= len("stale-snapshot") && source[:len("stale-snapshot")] == "stale-snapshot"
}

func quoteResultFreshness(result quoteFetchResult, now time.Time) (bool, string) {
	if result.timedOut {
		return true, "live quote provider timed out; serving last real snapshot"
	}
	if len(result.quotes) == 0 {
		return true, "live quote provider returned no quotes"
	}
	for _, quote := range result.quotes {
		if quote.Session == "closed" {
			return true, "market session is closed; quote is the last provider observation"
		}
	}
	return marketQuoteFreshness(result.source, result.dataTime, now)
}
