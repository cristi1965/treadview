package dataflows

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

var fiscalPeriodPattern = regexp.MustCompile(`(?i)Fiscal Period:\s*([^\s|]+)`)
var newsItemPattern = regexp.MustCompile(`(?m)^\d+\. \*\*`)

const (
	researchPreflightRequestTimeout = 8 * time.Second
	researchEvidenceRequestTimeout  = 35 * time.Second
	researchPreflightTotalBudget    = 25 * time.Second
	researchEvidenceTotalBudget     = 60 * time.Second
	researchMarketMaxAge            = 7 * 24 * time.Hour
	researchNewsMaxAge              = 7 * 24 * time.Hour
	researchFundamentalsMaxAge      = 180 * 24 * time.Hour
)

type ResearchPreflight struct {
	Passed  bool     `json:"passed"`
	Reasons []string `json:"reasons,omitempty"`
}

// RunResearchPreflight captures the minimum source set before any paid model call.
func (r *ToolRegistry) RunResearchPreflight() ResearchPreflight {
	return r.runNamedPreflight([]string{"get_stock_data", "get_fundamentals", "get_news"}, false)
}

// RunEvidenceOnlyPreflight uses dated historical OHLCV as the primary market
// evidence. A current quote is deliberately not required by this contract.
func (r *ToolRegistry) RunEvidenceOnlyPreflight() ResearchPreflight {
	return r.runNamedPreflight([]string{"get_fundamentals", "get_news", "get_historical_evidence"}, true)
}

func (r *ToolRegistry) runNamedPreflight(names []string, requireHistorical bool) ResearchPreflight {
	start := len(r.EvidenceSnapshot())
	// Bound each network client so one unavailable provider cannot turn a
	// fail-closed preflight into minutes of waiting before publication is decided.
	requestTimeout := researchPreflightRequestTimeout
	if requireHistorical {
		requestTimeout = researchEvidenceRequestTimeout
	}
	r.yfinance.httpClient.Timeout = requestTimeout
	r.news.httpClient.Timeout = researchPreflightRequestTimeout
	tasks := make([]func(), 0, len(names))
	var priorityTask func()
	for _, name := range names {
		name := name
		task := func() {
			_, _ = r.Prime(name, map[string]any{"ticker": r.ticker})
		}
		if requireHistorical && name == "get_fundamentals" {
			priorityTask = task
			continue
		}
		tasks = append(tasks, task)
	}
	totalBudget := researchPreflightTotalBudget
	if requireHistorical {
		totalBudget = researchEvidenceTotalBudget
	}
	completed := false
	if priorityTask != nil {
		completed = runPriorityPreflightTasks(totalBudget, priorityTask, tasks)
	} else {
		completed = runPreflightTasks(totalBudget, tasks)
	}
	invocations := r.EvidenceSnapshot()[start:]
	result := AssessResearchPreflight(invocations, r.date)
	if requireHistorical {
		result = AssessEvidenceOnlyPreflight(invocations, r.date)
	}
	if !completed {
		result.Passed = false
		result.Reasons = append(result.Reasons, fmt.Sprintf("research preflight exceeded %.0f second total budget", totalBudget.Seconds()))
	}
	return result
}

// Filing evidence is the only non-substitutable source in the historical
// profile. Complete it before auxiliary fan-out so those requests cannot
// consume the shared network budget or connection pool first.
func runPriorityPreflightTasks(budget time.Duration, priority func(), tasks []func()) bool {
	startedAt := time.Now()
	priority()
	remaining := budget - time.Since(startedAt)
	if remaining <= 0 {
		return false
	}
	return runPreflightTasks(remaining, tasks)
}

func AssessEvidenceOnlyPreflight(invocations []ToolInvocation, tradeDate string) ResearchPreflight {
	result := assessResearchPreflightSources(invocations, tradeDate, []string{"get_fundamentals", "get_news"})
	return assessHistoricalPreflight(invocations, tradeDate, result)
}

func assessHistoricalPreflight(invocations []ToolInvocation, tradeDate string, result ResearchPreflight) ResearchPreflight {
	var history *ToolInvocation
	for index := range invocations {
		if invocations[index].Name == "get_historical_evidence" {
			history = &invocations[index]
			break
		}
	}
	if history == nil {
		result.Passed = false
		result.Reasons = append(result.Reasons, "missing required source: get_historical_evidence")
		return result
	}
	if history.Status != "captured" || strings.TrimSpace(history.RawPayload) == "" || history.Provider == "" || history.SourceURL == "" {
		result.Passed = false
		result.Reasons = append(result.Reasons, "historical evidence is unavailable or lacks provider provenance")
		return result
	}
	parsedHistory, err := ParseHistoricalEvidence(history.RawPayload)
	if err != nil || parsedHistory.SourceURL != history.SourceURL || parsedHistory.DataTime != history.DataTime || parsedHistory.TimeGranularity != "date" || parsedHistory.SampleCount != len(parsedHistory.Observations) {
		result.Passed = false
		result.Reasons = append(result.Reasons, "historical evidence payload and invocation provenance are inconsistent")
		return result
	}
	if ticker := strings.ToUpper(strings.TrimSpace(history.Inputs["ticker"])); ticker != "" && ticker != parsedHistory.Symbol {
		result.Passed = false
		result.Reasons = append(result.Reasons, "historical evidence symbol does not match requested ticker")
		return result
	}
	tradeDay, ok := parseToolTime(tradeDate)
	if !ok {
		result.Passed = false
		return result
	}
	cutoff := tradeDay.Add(24 * time.Hour)
	if len(history.ObservationTimes) == 0 {
		result.Passed = false
		result.Reasons = append(result.Reasons, "historical evidence has no dated observations")
		return result
	}
	for _, value := range history.ObservationTimes {
		observed, parsed := parseToolTime(value)
		if !parsed || !observed.Before(cutoff) {
			result.Passed = false
			result.Reasons = append(result.Reasons, "historical evidence contains invalid or future observations")
			break
		}
	}
	latest, parsed := parseToolTime(history.DataTime)
	if !parsed || cutoff.Sub(latest) > researchMarketMaxAge {
		result.Passed = false
		result.Reasons = append(result.Reasons, "historical evidence latest observation is too old relative to tradeDate")
	}
	return result
}

func runPreflightTasks(budget time.Duration, tasks []func()) bool {
	var wg sync.WaitGroup
	wg.Add(len(tasks))
	for _, task := range tasks {
		task := task
		go func() {
			defer wg.Done()
			task()
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(budget):
		return false
	}
}

// AssessResearchPreflight applies source contracts without weakening publication evidence rules.
func AssessResearchPreflight(invocations []ToolInvocation, tradeDate string) ResearchPreflight {
	return assessResearchPreflightSources(invocations, tradeDate, []string{"get_stock_data", "get_fundamentals", "get_news"})
}

func assessResearchPreflightSources(invocations []ToolInvocation, tradeDate string, requiredSources []string) ResearchPreflight {
	byName := make(map[string]ToolInvocation, len(invocations))
	for _, invocation := range invocations {
		byName[invocation.Name] = invocation
	}
	reasons := make([]string, 0)
	tradeDay, tradeDateOK := parseToolTime(strings.TrimSpace(tradeDate))
	if !tradeDateOK || len(strings.TrimSpace(tradeDate)) != len("2006-01-02") {
		reasons = append(reasons, "tradeDate is missing or invalid")
	}
	cutoff := tradeDay.Add(24 * time.Hour)
	for _, name := range requiredSources {
		invocation, ok := byName[name]
		if !ok {
			reasons = append(reasons, "missing required source: "+name)
			continue
		}
		if invocation.Status != "captured" {
			reasons = append(reasons, fmt.Sprintf("required source %s is %s", name, invocation.Status))
		}
		if invocation.DataTime == "unknown" || strings.TrimSpace(invocation.DataTime) == "" {
			reasons = append(reasons, "required source has no parseable data time: "+name)
		}
		if strings.TrimSpace(invocation.RawPayload) == "" {
			reasons = append(reasons, "required source has no reviewable payload: "+name)
		}
		if strings.TrimSpace(invocation.Provider) == "" {
			reasons = append(reasons, "required source has no provider: "+name)
		}
		if strings.TrimSpace(invocation.SourceURL) == "" {
			reasons = append(reasons, "required source has no provider URL: "+name)
		}
		if tradeDateOK {
			maxAge := researchMarketMaxAge
			if name == "get_fundamentals" {
				maxAge = researchFundamentalsMaxAge
			} else if name == "get_news" {
				maxAge = researchNewsMaxAge
			}
			validatePointInTime(name, invocation, cutoff, maxAge, &reasons)
		}
	}
	if fundamentals, ok := byName["get_fundamentals"]; ok {
		match := fiscalPeriodPattern.FindStringSubmatch(fundamentals.RawPayload)
		if len(match) < 2 || strings.EqualFold(strings.TrimSpace(match[1]), "unknown") {
			reasons = append(reasons, "fundamentals fiscal period is unknown")
		}
		disclosure, ok := parseToolTime(fundamentals.DisclosureTime)
		if !ok {
			reasons = append(reasons, "fundamentals filing date is missing or invalid")
		} else if tradeDateOK && !disclosure.Before(cutoff) {
			reasons = append(reasons, "fundamentals filing date is after tradeDate")
		}
		for _, value := range fundamentals.DisclosureTimes {
			periodDisclosure, parsed := parseToolTime(value)
			if !parsed {
				reasons = append(reasons, "fundamentals period filing date is invalid")
				continue
			}
			if tradeDateOK && !periodDisclosure.Before(cutoff) {
				reasons = append(reasons, "fundamentals period filing date is after tradeDate")
			}
		}
		if evidence, err := ParseFundamentalEvidence(fundamentals.RawPayload); err == nil && len(evidence.Periods) > 0 {
			for _, period := range evidence.Periods {
				if strings.TrimSpace(period.PeriodEnd) == "" || strings.TrimSpace(period.FilingDate) == "" ||
					strings.TrimSpace(period.Accession) == "" || strings.TrimSpace(period.SourceURL) == "" ||
					strings.TrimSpace(period.Form) == "" || strings.TrimSpace(period.Frequency) == "" {
					reasons = append(reasons, "every fundamentals period must have filing metadata")
					break
				}
			}
		} else if len(fundamentals.DisclosureTimes) > 0 && len(fundamentals.DisclosureTimes) != len(invocationObservationTimes(fundamentals)) {
			reasons = append(reasons, "every fundamentals period must have filing metadata")
		}
	}
	if news, ok := byName["get_news"]; ok {
		items := len(newsItemPattern.FindAllString(news.RawPayload, -1))
		if items == 0 || len(invocationObservationTimes(news)) != items {
			reasons = append(reasons, "every news item must have a parseable published time")
		}
	}
	return ResearchPreflight{Passed: len(reasons) == 0, Reasons: reasons}
}

func validatePointInTime(name string, invocation ToolInvocation, cutoff time.Time, maxAge time.Duration, reasons *[]string) {
	observations := invocationObservationTimes(invocation)
	if len(observations) == 0 {
		*reasons = append(*reasons, "required source has no point-in-time observations: "+name)
		return
	}
	latest := time.Time{}
	for _, observed := range observations {
		if !observed.Before(cutoff) {
			*reasons = append(*reasons, "required source contains future information after tradeDate: "+name)
			return
		}
		if observed.After(latest) {
			latest = observed
		}
		if name == "get_news" && cutoff.Sub(observed) > maxAge {
			*reasons = append(*reasons, fmt.Sprintf("required source %s contains an observation older than %d days", name, int(maxAge.Hours()/24)))
			return
		}
	}
	if cutoff.Sub(latest) > maxAge {
		*reasons = append(*reasons, fmt.Sprintf("required source %s age exceeds %d days relative to tradeDate", name, int(maxAge.Hours()/24)))
	}
}

func invocationObservationTimes(invocation ToolInvocation) []time.Time {
	values := invocation.ObservationTimes
	if len(values) == 0 {
		values = inferToolObservationTimes(invocation.Name, invocation.RawPayload)
	}
	observations := make([]time.Time, 0, len(values))
	for _, value := range values {
		if parsed, ok := parseToolTime(value); ok {
			observations = append(observations, parsed)
		}
	}
	return observations
}
