package dataflows

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFundamentalEvidenceSerializesUnavailableMetricsAsNull(t *testing.T) {
	data := FundamentalData{Source: "fixture", Periods: []FinancialPeriod{{
		FiscalPeriod: "quarterly:2026-06-30", Frequency: "quarterly", Form: "10-Q", PeriodEnd: "2026-06-30",
		TotalRevenue: 0, GrossProfit: 45, AvailableFields: []string{"totalRevenue", "grossProfit"},
	}}}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["marketCap"] != nil || decoded["profitMargins"] != nil {
		t.Fatalf("unavailable top-level metric was zero-filled: %s", raw)
	}
	period := decoded["periods"].([]any)[0].(map[string]any)
	if period["totalRevenue"] != float64(0) || period["grossProfit"] != float64(45) || period["netIncome"] != nil {
		t.Fatalf("reported zero/missing period distinction was lost: %s", raw)
	}
}

func TestFinalizeStockMetricsEvidenceSeparatesUnknownDatesFromFetchTime(t *testing.T) {
	fetchedAt := time.Date(2026, 9, 6, 9, 30, 0, 0, time.UTC)
	metrics := &StockMetrics{Symbol: "AAPL", Source: "test"}

	FinalizeStockMetricsEvidence(metrics, fetchedAt)

	if metrics.FiscalPeriod != "unknown" || metrics.AsOf != "unknown" {
		t.Fatalf("missing upstream dates must remain unknown: %+v", metrics)
	}
	if metrics.FetchedAt != "2026-09-06T09:30:00Z" {
		t.Fatalf("unexpected fetchedAt: %s", metrics.FetchedAt)
	}
	if metrics.SourceLinks == nil || metrics.FieldSources == nil {
		t.Fatal("evidence collections must serialize as empty collections, not null")
	}
}

func TestResearchPreflightRequiresMarketFiscalPeriodAndNewsTime(t *testing.T) {
	invocations := []ToolInvocation{
		{Name: "get_stock_data", Status: "captured", Provider: "Yahoo Finance Chart", SourceURL: "https://query1.finance.yahoo.com/chart/AAPL", DataTime: "2026-09-05", ObservationTimes: []string{"2026-09-05"}, RawPayload: "2026-09-05,100"},
		{Name: "get_fundamentals", Status: "captured", Provider: "sec-edgar", SourceURL: "https://data.sec.gov/AAPL", DataTime: "2026-06-30", ObservationTimes: []string{"2026-06-30"}, DisclosureTime: "2026-08-01", RawPayload: "Fiscal Period: FY2026-Q3 | As Of: 2026-06-30 | Filing Date: 2026-08-01"},
		{Name: "get_news", Status: "captured", Provider: "Yahoo Finance RSS", SourceURL: "https://feeds.finance.yahoo.com/AAPL", DataTime: "2026-09-05T12:00:00Z", ObservationTimes: []string{"2026-09-05T12:00:00Z"}, RawPayload: "1. **Headline**\nPublished: 2026-09-05T12:00:00Z"},
	}
	if got := AssessResearchPreflight(invocations, "2026-09-05"); !got.Passed {
		t.Fatalf("valid preflight rejected: %+v", got)
	}
	invocations[1].DataTime = "unknown"
	invocations[1].ObservationTimes = nil
	invocations[1].DisclosureTime = ""
	invocations[1].RawPayload = "Fiscal Period: unknown"
	if got := AssessResearchPreflight(invocations, "2026-09-05"); got.Passed || len(got.Reasons) < 2 {
		t.Fatalf("unknown fiscal period must fail before LLM: %+v", got)
	}
}

func TestResearchPreflightRejectsFutureStaleAndUndatedEvidence(t *testing.T) {
	base := []ToolInvocation{
		{Name: "get_stock_data", Status: "captured", Provider: "Yahoo", SourceURL: "https://example.test/chart", DataTime: "2026-09-05", ObservationTimes: []string{"2026-09-05"}, RawPayload: "2026-09-05,100"},
		{Name: "get_fundamentals", Status: "captured", Provider: "SEC", SourceURL: "https://example.test/filing", DataTime: "2026-06-30", ObservationTimes: []string{"2026-06-30"}, DisclosureTime: "2026-08-01", RawPayload: "Fiscal Period: FY2026-Q3 | As Of: 2026-06-30 | Filing Date: 2026-08-01"},
		{Name: "get_news", Status: "captured", Provider: "Yahoo RSS", SourceURL: "https://example.test/rss", DataTime: "2026-09-05T12:00:00Z", ObservationTimes: []string{"2026-09-05T12:00:00Z"}, RawPayload: "1. **Headline**\nPublished: 2026-09-05T12:00:00Z"},
	}
	tests := []struct {
		name   string
		mutate func([]ToolInvocation)
		reason string
	}{
		{"future market bar", func(v []ToolInvocation) { v[0].ObservationTimes = []string{"2026-09-06"} }, "future information"},
		{"stale market bar", func(v []ToolInvocation) { v[0].ObservationTimes = []string{"2026-08-01"} }, "age exceeds 7 days"},
		{"stale fundamentals", func(v []ToolInvocation) { v[1].ObservationTimes = []string{"2025-01-01"} }, "age exceeds 180 days"},
		{"future filing", func(v []ToolInvocation) { v[1].DisclosureTime = "2026-09-06" }, "filing date is after"},
		{"missing filing", func(v []ToolInvocation) { v[1].DisclosureTime = "" }, "filing date is missing"},
		{"future news", func(v []ToolInvocation) { v[2].ObservationTimes = []string{"2026-09-06T00:00:00Z"} }, "future information"},
		{"stale news", func(v []ToolInvocation) { v[2].ObservationTimes = []string{"2026-08-01T12:00:00Z"} }, "observation older than 7 days"},
		{"undated news item", func(v []ToolInvocation) { v[2].RawPayload += "\n2. **Undated**" }, "every news item"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invocations := append([]ToolInvocation(nil), base...)
			tt.mutate(invocations)
			got := AssessResearchPreflight(invocations, "2026-09-05")
			if got.Passed || !strings.Contains(strings.Join(got.Reasons, ";"), tt.reason) {
				t.Fatalf("expected %q rejection: %+v", tt.reason, got)
			}
		})
	}
}

func TestResearchPreflightAllowsComparativePeriodsFromSameFiling(t *testing.T) {
	periods := []FinancialPeriod{
		{FiscalPeriod: "FY2026-Q3", Frequency: "quarterly", Form: "10-Q", PeriodStart: "2026-03-29", PeriodEnd: "2026-06-27", FilingDate: "2026-07-31", Accession: "0000320193-26-000020", SourceURL: "https://www.sec.gov/filing"},
		{FiscalPeriod: "FY2026-Q3", Frequency: "quarterly", Form: "10-Q", PeriodStart: "2025-03-30", PeriodEnd: "2025-06-28", FilingDate: "2026-07-31", Accession: "0000320193-26-000020", SourceURL: "https://www.sec.gov/filing"},
	}
	fundamentals := FundamentalData{Source: "sec-edgar-companyfacts", FiscalPeriod: "FY2026-Q3", AsOf: "2026-06-27", FilingDate: "2026-07-31", Accession: periods[0].Accession, SourceURL: periods[0].SourceURL, Periods: periods}
	fundamentalPayload := formatFundamentals(&fundamentals, "AAPL")
	invocations := []ToolInvocation{
		{Name: "get_stock_data", Status: "captured", Provider: "Nasdaq", SourceURL: "https://example.test/history", DataTime: "2026-09-04", ObservationTimes: []string{"2026-09-04"}, RawPayload: "2026-09-04,100"},
		{Name: "get_fundamentals", Status: "captured", Provider: fundamentals.Source, SourceURL: fundamentals.SourceURL, DataTime: fundamentals.AsOf, ObservationTimes: []string{"2026-06-27", "2025-06-28"}, DisclosureTime: fundamentals.FilingDate, DisclosureTimes: []string{fundamentals.FilingDate}, RawPayload: fundamentalPayload},
		{Name: "get_news", Status: "captured", Provider: "Yahoo RSS", SourceURL: "https://example.test/rss", DataTime: "2026-09-05T12:00:00Z", ObservationTimes: []string{"2026-09-05T12:00:00Z"}, RawPayload: "1. **Headline**\nPublished: 2026-09-05T12:00:00Z"},
	}
	if got := AssessResearchPreflight(invocations, "2026-09-05"); !got.Passed {
		t.Fatalf("comparative periods from one filing should retain valid metadata: %+v", got)
	}
}

func TestRunPreflightTasksRunsConcurrentlyAndHonorsBudget(t *testing.T) {
	started := time.Now()
	completed := runPreflightTasks(200*time.Millisecond, []func(){
		func() { time.Sleep(40 * time.Millisecond) },
		func() { time.Sleep(40 * time.Millisecond) },
		func() { time.Sleep(40 * time.Millisecond) },
	})
	if !completed {
		t.Fatal("expected concurrent tasks to complete within budget")
	}
	if elapsed := time.Since(started); elapsed >= 100*time.Millisecond {
		t.Fatalf("preflight tasks appear serialized: %s", elapsed)
	}

	started = time.Now()
	completed = runPreflightTasks(20*time.Millisecond, []func(){
		func() { time.Sleep(100 * time.Millisecond) },
	})
	if completed {
		t.Fatal("expected preflight budget to expire")
	}
	if elapsed := time.Since(started); elapsed >= 80*time.Millisecond {
		t.Fatalf("budget did not bound preflight latency: %s", elapsed)
	}
}

func TestRunPriorityPreflightTasksCompletesPriorityBeforeFanout(t *testing.T) {
	priorityDone := make(chan struct{})
	auxiliarySawPriority := make(chan bool, 2)
	completed := runPriorityPreflightTasks(200*time.Millisecond, func() {
		close(priorityDone)
	}, []func(){
		func() { _, open := <-priorityDone; auxiliarySawPriority <- !open },
		func() { _, open := <-priorityDone; auxiliarySawPriority <- !open },
	})
	if !completed {
		t.Fatal("priority preflight should complete within budget")
	}
	for range 2 {
		if !<-auxiliarySawPriority {
			t.Fatal("auxiliary task started before priority evidence completed")
		}
	}
}

func TestPayloadArtifactRoundTripIsHashVerifiedAndPrivate(t *testing.T) {
	payload := "complete upstream response\n2026-09-05,100"
	sum := sha256.Sum256([]byte(payload))
	hash := "sha256:" + hex.EncodeToString(sum[:])
	root := t.TempDir()
	ref, size, err := PersistPayloadArtifact(root, "run-1", "tool-1", ToolInvocation{RawPayload: payload, ContentHash: hash})
	if err != nil || size != int64(len(payload)) || ref != "evidence/run-1/tool-1.payload.gz" {
		t.Fatalf("persist ref=%q size=%d err=%v", ref, size, err)
	}
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(ref)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("artifact permission=%v", info.Mode().Perm())
	}
	raw, err := ReadPayloadArtifact(root, "run-1", "tool-1", hash)
	if err != nil || string(raw) != payload {
		t.Fatalf("round trip=%q err=%v", raw, err)
	}
	if _, err := ReadPayloadArtifact(root, "../escape", "tool-1", hash); err == nil {
		t.Fatal("unsafe artifact path accepted")
	}
}

func TestAddMetricSourceRecordsFieldLevelProvenanceAndDeduplicatesLinks(t *testing.T) {
	metrics := &StockMetrics{Symbol: "AAPL"}
	addMetricSource(metrics, "nasdaq", "Nasdaq financials", "https://example.test/aapl", "2026-06-30", "roe", "grossMargin")
	addMetricSource(metrics, "nasdaq", "Nasdaq financials", "https://example.test/aapl", "2026-06-30", "profitMargin")
	FinalizeStockMetricsEvidence(metrics, time.Now())

	if len(metrics.SourceLinks) != 1 {
		t.Fatalf("expected one deduplicated source link, got %d", len(metrics.SourceLinks))
	}
	if got := metrics.FieldSources["roe"]; got.Source != "nasdaq" || got.AsOf != "2026-06-30" {
		t.Fatalf("unexpected roe provenance: %+v", got)
	}
	if _, ok := metrics.FieldSources["profitMargin"]; !ok {
		t.Fatal("second call must still add field provenance")
	}
	if metrics.Source != "nasdaq" {
		t.Fatalf("payload source must be derived from successful sources: %s", metrics.Source)
	}
}

func TestToolRegistryCapturesActualInvocationMetadata(t *testing.T) {
	registry := NewToolRegistry("AAPL", "2026-09-05", "")
	_, _ = registry.Execute("unknown_tool", map[string]any{"window": "1y"})

	evidence := registry.EvidenceSnapshot()
	if len(evidence) != 1 || evidence[0].Status != "error" || evidence[0].DataTime != "unknown" {
		t.Fatalf("unexpected invocation evidence: %+v", evidence)
	}
	if evidence[0].Inputs["ticker"] != "AAPL" || evidence[0].Inputs["requested_as_of"] != "2026-09-05" || evidence[0].ContentHash == "" {
		t.Fatalf("invocation inputs/hash missing: %+v", evidence[0])
	}
}

func TestToolInvocationStatusDoesNotTreatNewsWordsAsTransportFailure(t *testing.T) {
	if got := toolInvocationStatus("Published: 2026-09-05T12:00:00Z\nCompany failed to meet estimates", nil); got != "captured" {
		t.Fatalf("ordinary article wording status=%s", got)
	}
	if got := toolInvocationStatus("News unavailable for AAPL: timeout", nil); got != "unavailable" {
		t.Fatalf("controlled failure status=%s", got)
	}
}

func TestReviewablePayloadExcerptPreservesSmallOutputAndBoundsLargeOutput(t *testing.T) {
	if got, truncated := reviewablePayloadExcerpt("source payload"); got != "source payload" || truncated {
		t.Fatalf("small payload=%q truncated=%v", got, truncated)
	}
	large := strings.Repeat("a", 10_000)
	got, truncated := reviewablePayloadExcerpt(large)
	if !truncated || len(got) > 2048 || !strings.Contains(got, "content_hash") {
		t.Fatalf("large payload len=%d truncated=%v", len(got), truncated)
	}
}

func TestInferToolDataTimeUsesResponseObservationsNotCompletionTime(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{"get_stock_data", "Date,Open,High,Low,Close,Volume\n2026-09-03,1,2,1,2,100\n2026-09-05,2,3,2,3,200\n", "2026-09-05"},
		{"get_verified_market_snapshot", "VERIFIED MARKET SNAPSHOT for AAPL (Date: 2026-09-04)\n", "2026-09-04"},
		{"get_macro_indicators", "| CPI | 100 | 2026-08-01 |\n| VIX | 15 | 2026-09-03 |\n", "2026-08-01"},
		{"get_news", "Published: Fri, 04 Sep 2026 13:05:00 +0000\n", "2026-09-04T13:05:00Z"},
		{"get_fundamentals", "Source: yahoo | Fiscal Period: 2025-12-31 | As Of: 2026-09-04T20:00:00Z\n", "2026-09-04"},
	}
	for _, tt := range tests {
		if got := inferToolDataTime(tt.name, tt.output); got != tt.want {
			t.Errorf("%s dataTime=%q want %q", tt.name, got, tt.want)
		}
	}
	if got := inferToolDataTime("get_realtime_quote", "Current Price: $100"); got != "unknown" {
		t.Fatalf("timestamp-free realtime response must remain unknown, got %q", got)
	}
}

func TestInferToolProvenanceSeparatesProviderURLAndDisclosure(t *testing.T) {
	provider, sourceURL, disclosure := inferToolProvenance("get_fundamentals", "Source: sec-edgar | Fiscal Period: FY2026-Q3 | As Of: 2026-06-30 | Filing Date: 2026-08-01\nSource URL: https://data.sec.gov/submissions/CIK.json\n")
	if provider != "sec-edgar" || sourceURL != "https://data.sec.gov/submissions/CIK.json" || disclosure != "2026-08-01" {
		t.Fatalf("structured provenance lost: provider=%q url=%q disclosure=%q", provider, sourceURL, disclosure)
	}
}

func TestRecordInvocationPersistsStructuredSourceMetadata(t *testing.T) {
	registry := NewToolRegistry("AAPL", "2026-09-05", "")
	registry.recordInvocation("get_fundamentals", map[string]any{"ticker": "AAPL"}, "Source: sec-edgar | Fiscal Period: FY2026-Q3 | As Of: 2026-06-30 | Filing Date: 2026-08-01\nSource URL: https://data.sec.gov/companyfacts.json\n", nil)
	evidence := registry.EvidenceSnapshot()
	if len(evidence) != 1 || evidence[0].Provider != "sec-edgar" || evidence[0].SourceURL == "" || evidence[0].DisclosureTime != "2026-08-01" || len(evidence[0].ObservationTimes) != 1 {
		t.Fatalf("structured invocation evidence missing: %+v", evidence)
	}
}
