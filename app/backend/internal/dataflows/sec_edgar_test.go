package dataflows

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type secRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn secRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestLookupSECCIKUsesExactOfficialSearchMatch(t *testing.T) {
	client := &http.Client{Transport: secRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `{"hits":{"hits":[{"_source":{"ciks":["0001002125"],"display_names":["ADVANCED LIGHTING TECHNOLOGIES INC  (CIK 0001002125)"]}},{"_source":{"ciks":["0000320193"],"display_names":["EXACT ISSUER INC  (EXCT)  (CIK 0000320193)"]}}]}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	cik, name, err := lookupSECCIK(client, "exct")
	if err != nil || cik != 320193 || name != "EXACT ISSUER INC" {
		t.Fatalf("official search lookup failed cik=%d name=%q err=%v", cik, name, err)
	}
}

func TestSECCompanyFactsUsesFreshHashVerifiedCache(t *testing.T) {
	t.Setenv("TRADINGAGENTS_CACHE_DIR", t.TempDir())
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if fail {
			http.Error(writer, "upstream unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(writer, `{"cik":320193,"facts":{"us-gaap":{"Assets":{}}}}`)
	}))
	defer server.Close()

	firstBody, fetchedAt, transport, contentHash, err := fetchSECCompanyFacts(server.Client(), server.URL, 320193)
	if err != nil || transport != "network" || fetchedAt.IsZero() || contentHash == "" {
		t.Fatalf("network fetch did not create verified cache: transport=%q hash=%q err=%v", transport, contentHash, err)
	}
	fail = true
	secondBody, cachedAt, transport, cachedHash, err := fetchSECCompanyFacts(server.Client(), server.URL, 320193)
	if err != nil || transport != "verified-cache" || !cachedAt.Equal(fetchedAt) || cachedHash != contentHash || string(secondBody) != string(firstBody) {
		t.Fatalf("verified cache was not reused: transport=%q hash=%q fetchedAt=%s err=%v", transport, cachedHash, cachedAt, err)
	}

	path, err := secCompanyFactsCachePath(320193)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"url":"tampered"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := readSECCompanyFactsCache(server.URL, 320193, time.Now().UTC()); err == nil {
		t.Fatal("tampered cache must fail closed")
	}
}

func TestLookupSECCIKUsesVerifiedSeedWithoutNetwork(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: secRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader("unexpected")), Header: make(http.Header)}, nil
	})}
	cik, name, err := lookupSECCIK(client, " AAPL ")
	if err != nil || cik != 320193 || name != "Apple Inc." || calls != 0 {
		t.Fatalf("verified seed lookup failed cik=%d name=%q calls=%d err=%v", cik, name, calls, err)
	}
}

func TestLookupSECCIKUsesOfficialAtomBeforeBulkMap(t *testing.T) {
	bulkMapCalls := 0
	client := &http.Client{Transport: secRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		status := http.StatusOK
		body := `<feed><company-info><cik>0000320193</cik><conformed-name>Apple Inc.</conformed-name></company-info></feed>`
		if request.URL.Path == "/files/company_tickers.json" {
			bulkMapCalls++
			status, body = http.StatusServiceUnavailable, `unavailable`
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	cik, name, err := lookupSECCIK(client, "ATOM")
	if err != nil || cik != 320193 || name != "Apple Inc." || bulkMapCalls != 0 {
		t.Fatalf("official Atom fallback failed cik=%d name=%q err=%v", cik, name, err)
	}
}

func TestApplySECFactsUsesSingleFilingAndLeavesUnknownOmitted(t *testing.T) {
	raw := []byte(`{
      "cik":320193,"entityName":"Apple Inc.","facts":{"us-gaap":{
        "RevenueFromContractWithCustomerExcludingAssessedTax":{"units":{"USD":[
          {"start":"2026-03-29","end":"2026-06-27","val":90000000000,"accn":"0000320193-26-000079","fy":2026,"fp":"Q3","form":"10-Q","filed":"2026-08-01"}
        ]}},
        "NetIncomeLoss":{"units":{"USD":[
          {"start":"2026-03-29","end":"2026-06-27","val":20000000000,"accn":"0000320193-26-000079","fy":2026,"fp":"Q3","form":"10-Q","filed":"2026-08-01"},
          {"start":"2026-03-29","end":"2026-06-27","val":999,"accn":"other","fy":2026,"fp":"Q3","form":"10-Q","filed":"2026-08-02"}
        ]}}
      }}}
    `)
	var payload secCompanyFacts
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	out := &StockMetrics{Symbol: "AAPL", FieldSources: map[string]FieldProvenance{}}
	if err := applySECFacts(out, 320193, "Apple Inc.", &payload); err != nil {
		t.Fatal(err)
	}
	if out.FiscalPeriod != "quarterly:2026-03-29/2026-06-27" || out.AsOf != "2026-06-27" || out.FilingDate != "2026-08-01" || out.Accession != "0000320193-26-000079" {
		t.Fatalf("bad filing metadata: %+v", out)
	}
	if out.TotalRevenue != 90000000000 || out.NetIncome != 20000000000 || out.TotalAssets != 0 {
		t.Fatalf("facts were mixed across filings or unknown was fabricated: %+v", out)
	}
	if out.FieldSources["netIncome"].Accession != out.Accession {
		t.Fatalf("field provenance mismatch: %+v", out.FieldSources)
	}
}

func TestSECFilingPeriodChoosesDiscreteQuarterAndMatchingFieldDuration(t *testing.T) {
	raw := []byte(`{
      "cik":320193,"entityName":"Apple Inc.","facts":{"us-gaap":{
        "RevenueFromContractWithCustomerExcludingAssessedTax":{"units":{"USD":[
          {"start":"2025-09-28","end":"2026-06-27","val":300,"accn":"a1","fy":2026,"fp":"Q3","form":"10-Q","filed":"2026-07-31"},
          {"start":"2026-03-29","end":"2026-06-27","val":110,"accn":"a1","fy":2026,"fp":"Q3","form":"10-Q","filed":"2026-07-31"}
        ]}},
        "NetIncomeLoss":{"units":{"USD":[
          {"start":"2025-09-28","end":"2026-06-27","val":90,"accn":"a1","fy":2026,"fp":"Q3","form":"10-Q","filed":"2026-07-31"},
          {"start":"2026-03-29","end":"2026-06-27","val":30,"accn":"a1","fy":2026,"fp":"Q3","form":"10-Q","filed":"2026-07-31"}
        ]}}
      }}}`)
	var payload secCompanyFacts
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	out := &StockMetrics{Symbol: "AAPL", FieldSources: map[string]FieldProvenance{}}
	if err := applySECFacts(out, 320193, "Apple Inc.", &payload); err != nil {
		t.Fatal(err)
	}
	if out.TotalRevenue != 110 || out.NetIncome != 30 || len(out.Periods) != 1 || out.Periods[0].PeriodStart != "2026-03-29" || out.Periods[0].Frequency != "quarterly" {
		t.Fatalf("YTD and discrete-quarter facts were mixed: %+v periods=%+v", out, out.Periods)
	}
	for _, field := range []string{"totalRevenue", "netIncome"} {
		evidence := out.Periods[0].FieldEvidence[field]
		if evidence.PeriodKind != "duration" || evidence.PeriodStart != "2026-03-29" || evidence.PeriodEnd != "2026-06-27" || evidence.Form != "10-Q" || evidence.Filed != "2026-07-31" || evidence.Accession != "a1" || evidence.URL == "" {
			t.Fatalf("field %s lacks exact filing provenance: %+v", field, evidence)
		}
	}
}

func TestSECFinancialPeriodsCapsAtFiveDiscreteQuartersAndThreeAnnuals(t *testing.T) {
	concepts := map[string]struct {
		Units map[string][]secFact `json:"units"`
	}{"RevenueFromContractWithCustomerExcludingAssessedTax": {Units: map[string][]secFact{"USD": {}}}}
	revenue := concepts["RevenueFromContractWithCustomerExcludingAssessedTax"]
	quarters := []secFact{
		{Start: "2026-04-01", End: "2026-06-30", Value: 120, Accn: "q5", FY: 2026, FP: "Q3", Form: "10-Q", Filed: "2026-08-01"},
		{Start: "2026-01-01", End: "2026-03-31", Value: 110, Accn: "q4", FY: 2026, FP: "Q2", Form: "10-Q", Filed: "2026-05-01"},
		{Start: "2025-10-01", End: "2025-12-31", Value: 100, Accn: "q3", FY: 2026, FP: "Q1", Form: "10-Q", Filed: "2026-02-01"},
		{Start: "2025-07-01", End: "2025-09-30", Value: 90, Accn: "q2", FY: 2025, FP: "Q4", Form: "10-Q", Filed: "2025-11-01"},
		{Start: "2025-04-01", End: "2025-06-30", Value: 80, Accn: "q1", FY: 2025, FP: "Q3", Form: "10-Q", Filed: "2025-08-01"},
		{Start: "2025-01-01", End: "2025-03-31", Value: 70, Accn: "old", FY: 2025, FP: "Q2", Form: "10-Q", Filed: "2025-05-01"},
	}
	// This nine-month YTD value shares the latest filing but must never become a quarter.
	quarters = append(quarters, secFact{Start: "2025-10-01", End: "2026-06-30", Value: 330, Accn: "q5", FY: 2026, FP: "Q3", Form: "10-Q", Filed: "2026-08-01"})
	annuals := []secFact{
		{Start: "2025-01-01", End: "2025-12-31", Value: 400, Accn: "a3", FY: 2025, FP: "FY", Form: "10-K", Filed: "2026-02-15"},
		{Start: "2024-01-01", End: "2024-12-31", Value: 360, Accn: "a2", FY: 2024, FP: "FY", Form: "10-K", Filed: "2025-02-15"},
		{Start: "2023-01-01", End: "2023-12-31", Value: 320, Accn: "a1", FY: 2023, FP: "FY", Form: "10-K", Filed: "2024-02-15"},
		{Start: "2022-01-01", End: "2022-12-31", Value: 280, Accn: "a0", FY: 2022, FP: "FY", Form: "10-K", Filed: "2023-02-15"},
	}
	revenue.Units["USD"] = append(quarters, annuals...)
	concepts["RevenueFromContractWithCustomerExcludingAssessedTax"] = revenue
	periods := secFinancialPeriods(320193, concepts, nil)
	quarterCount, annualCount := 0, 0
	identities := map[string]bool{}
	for _, period := range periods {
		if period.FiscalPeriod != secPeriodIdentity(period.Frequency, period.PeriodStart, period.PeriodEnd) || identities[period.FiscalPeriod] {
			t.Fatalf("period identity is misleading or unstable: %+v", period)
		}
		identities[period.FiscalPeriod] = true
		if period.Frequency == "quarterly" {
			quarterCount++
		} else if period.Frequency == "annual" {
			annualCount++
		}
		if period.Frequency == "quarterly" && period.PeriodEnd == "2026-06-30" && period.TotalRevenue != 120 {
			t.Fatalf("YTD value was used as a discrete quarter: %+v", period)
		}
		if evidence := period.FieldEvidence["totalRevenue"]; evidence.Accession != period.Accession || evidence.PeriodStart != period.PeriodStart || evidence.PeriodEnd != period.PeriodEnd || evidence.PeriodKind != "duration" {
			t.Fatalf("period lacks field-level filing binding: %+v", period)
		}
	}
	if quarterCount != 5 || annualCount != 3 || len(periods) != 8 {
		t.Fatalf("unexpected period depth quarters=%d annuals=%d periods=%+v", quarterCount, annualCount, periods)
	}
}

func TestSECFinancialPeriodsBindsSharesToSameFiling(t *testing.T) {
	concepts := map[string]struct {
		Units map[string][]secFact `json:"units"`
	}{
		"RevenueFromContractWithCustomerExcludingAssessedTax": {Units: map[string][]secFact{"USD": {
			{Start: "2026-03-29", End: "2026-06-27", Value: 100, Accn: "filing-a", FY: 2026, FP: "Q3", Form: "10-Q", Filed: "2026-07-31"},
			{Start: "2025-03-30", End: "2025-06-28", Value: 90, Accn: "filing-a", FY: 2026, FP: "Q3", Form: "10-Q", Filed: "2026-07-31"},
		}}},
	}
	shares := map[string]struct {
		Units map[string][]secFact `json:"units"`
	}{
		"EntityCommonStockSharesOutstanding": {Units: map[string][]secFact{"shares": {
			{End: "2026-07-17", Value: 14_594_180_000, Accn: "filing-a", Form: "10-Q", Filed: "2026-07-31"},
			{End: "2026-07-18", Value: 999, Accn: "other-filing", Form: "10-Q", Filed: "2026-07-31"},
		}}},
	}
	periods := secFinancialPeriods(320193, concepts, shares)
	if len(periods) != 2 || periods[0].SharesOutstanding != 14_594_180_000 || periods[0].FieldEvidence["sharesOutstanding"].Unit != "shares" || periods[0].FieldEvidence["sharesOutstanding"].Accession != "filing-a" || periods[1].SharesOutstanding != 0 {
		t.Fatalf("shares were not bound to the same filing: %+v", periods)
	}
}

func TestSECFinancialDerivationInputsRetainYTDWithoutCallingItAQuarter(t *testing.T) {
	concepts := map[string]struct {
		Units map[string][]secFact `json:"units"`
	}{
		"RevenueFromContractWithCustomerExcludingAssessedTax": {Units: map[string][]secFact{"USD": {
			{Start: "2024-09-29", End: "2025-06-28", Value: 300, Accn: "q3", FY: 2025, FP: "Q3", Form: "10-Q", Filed: "2025-08-01"},
			{Start: "2025-03-30", End: "2025-06-28", Value: 110, Accn: "q3", FY: 2025, FP: "Q3", Form: "10-Q", Filed: "2025-08-01"},
		}}},
		"NetIncomeLoss": {Units: map[string][]secFact{"USD": {{Start: "2024-09-29", End: "2025-06-28", Value: 60, Accn: "q3", FY: 2025, FP: "Q3", Form: "10-Q", Filed: "2025-08-01"}}}},
	}
	inputs := secFinancialDerivationInputs(320193, concepts)
	if len(inputs) != 1 || inputs[0].Frequency != "year-to-date" || inputs[0].FiscalPeriod != "year-to-date:2024-09-29/2025-06-28" || inputs[0].TotalRevenue != 300 || inputs[0].NetIncome != 60 {
		t.Fatalf("reported YTD derivation evidence was lost or mislabeled: %+v", inputs)
	}
	formatted := FormatFundamentalsForEvidence(&FundamentalData{Periods: inputs, DerivationInputs: inputs}, "AAPL")
	if !containsString(inferFundamentalDisclosureTimes("get_fundamentals", formatted), "2025-08-01") || !containsString(inferToolObservationTimes("get_fundamentals", formatted), "2025-06-28") {
		t.Fatalf("YTD derivation evidence was omitted from PIT timestamps: %s", formatted)
	}
}

func TestSECRealAAPLDepth(t *testing.T) {
	if os.Getenv("STOCKGOD_TEST_SEC_REAL") != "1" {
		t.Skip("set STOCKGOD_TEST_SEC_REAL=1 for the read-only SEC integration check")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	body, err := secGet(client, "https://data.sec.gov/api/xbrl/companyfacts/CIK0000320193.json")
	if err != nil {
		t.Fatalf("real SEC AAPL companyfacts unavailable: %v", err)
	}
	var payload secCompanyFacts
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("parse real SEC AAPL companyfacts: %v", err)
	}
	out := &StockMetrics{Symbol: "AAPL", FieldSources: map[string]FieldProvenance{}}
	if err := applySECFacts(out, 320193, payload.EntityName, &payload); err != nil {
		t.Fatalf("apply real SEC AAPL companyfacts: %v", err)
	}
	quarterCount, annualCount := 0, 0
	for _, period := range out.Periods {
		if period.FiscalPeriod != secPeriodIdentity(period.Frequency, period.PeriodStart, period.PeriodEnd) {
			t.Fatalf("real AAPL period has unstable identity: %+v", period)
		}
		if period.Frequency == "quarterly" {
			quarterCount++
		} else if period.Frequency == "annual" {
			annualCount++
		}
		for _, field := range period.AvailableFields {
			evidence, ok := period.FieldEvidence[field]
			if !ok || evidence.Accession != period.Accession || evidence.Filed != period.FilingDate || evidence.Form != period.Form || evidence.URL == "" || (field != "sharesOutstanding" && evidence.PeriodEnd != period.PeriodEnd) {
				t.Fatalf("real AAPL field %s lacks exact provenance: period=%+v evidence=%+v", field, period, evidence)
			}
		}
	}
	if quarterCount != 5 || annualCount != 3 {
		t.Fatalf("real SEC AAPL depth quarters=%d annuals=%d periods=%+v", quarterCount, annualCount, out.Periods)
	}
	if len(out.Periods) == 0 || out.Periods[0].SharesOutstanding <= 0 || len(out.DerivationInputs) == 0 {
		t.Fatalf("real SEC AAPL lacks filing-bound shares or YTD derivation evidence: periods=%+v derivation=%+v", out.Periods, out.DerivationInputs)
	}
}

func TestSECRealAAPLLookupAndEnrichment(t *testing.T) {
	if os.Getenv("STOCKGOD_TEST_SEC_REAL") != "1" {
		t.Skip("set STOCKGOD_TEST_SEC_REAL=1 for the read-only SEC integration check")
	}
	client := &http.Client{Timeout: 20 * time.Second}
	out := &StockMetrics{Symbol: "AAPL", FieldSources: map[string]FieldProvenance{}}
	if err := enrichFromSEC(client, out); err != nil {
		t.Fatalf("real SEC AAPL lookup/enrichment unavailable: %v", err)
	}
	quarterCount, annualCount := 0, 0
	for _, period := range out.Periods {
		if period.Frequency == "quarterly" {
			quarterCount++
		} else if period.Frequency == "annual" {
			annualCount++
		}
	}
	if quarterCount != 5 || annualCount != 3 || out.Source != "sec-edgar-companyfacts" {
		t.Fatalf("real SEC AAPL enrichment depth/source mismatch quarters=%d annuals=%d source=%q", quarterCount, annualCount, out.Source)
	}
}

func TestSECRealAAPLFundamentalsPath(t *testing.T) {
	if os.Getenv("STOCKGOD_TEST_SEC_REAL") != "1" {
		t.Skip("set STOCKGOD_TEST_SEC_REAL=1 for the read-only SEC integration check")
	}
	client := NewYFinanceClientWithTimeout(35 * time.Second)
	metrics, err := client.GetStockMetricsLive("AAPL")
	if err != nil {
		t.Fatalf("real AAPL fundamentals path failed: %v", err)
	}
	quarterCount, annualCount := 0, 0
	for _, period := range metrics.Periods {
		if period.Frequency == "quarterly" {
			quarterCount++
		} else if period.Frequency == "annual" {
			annualCount++
		}
	}
	if metrics.Source != "sec-edgar-companyfacts" || quarterCount != 5 || annualCount != 3 {
		t.Fatalf("real fundamentals path degraded source=%q quarters=%d annuals=%d", metrics.Source, quarterCount, annualCount)
	}
}
