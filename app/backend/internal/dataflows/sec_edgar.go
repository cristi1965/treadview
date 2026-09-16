package dataflows

import (
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const secBaseURL = "https://data.sec.gov"

const secCompanyFactsCacheMaxAge = 24 * time.Hour

var secCacheMu sync.Mutex

type secCompanyFactsCacheEnvelope struct {
	URL         string          `json:"url"`
	FetchedAt   string          `json:"fetchedAt"`
	ContentHash string          `json:"contentHash"`
	Body        json.RawMessage `json:"body"`
}

type secFact struct {
	Start string  `json:"start"`
	End   string  `json:"end"`
	Value float64 `json:"val"`
	Accn  string  `json:"accn"`
	FY    int     `json:"fy"`
	FP    string  `json:"fp"`
	Form  string  `json:"form"`
	Filed string  `json:"filed"`
}

type secCompanyFacts struct {
	CIK        int    `json:"cik"`
	EntityName string `json:"entityName"`
	Facts      map[string]map[string]struct {
		Units map[string][]secFact `json:"units"`
	} `json:"facts"`
}

type secTickerEntry struct {
	CIK    int    `json:"cik_str"`
	Ticker string `json:"ticker"`
	Title  string `json:"title"`
}

// CIKs are stable SEC issuer identifiers, not market observations. These
// product-watchlist seeds were verified against SEC filing-search results; all
// financial facts are still fetched from companyfacts for the requested run.
var verifiedSECIssuerSeeds = map[string]struct {
	CIK  int
	Name string
}{
	"AAPL": {CIK: 320193, Name: "Apple Inc."},
	"NVDA": {CIK: 1045810, Name: "NVIDIA Corp."},
	"MSFT": {CIK: 789019, Name: "Microsoft Corp."},
	"AMZN": {CIK: 1018724, Name: "Amazon.com Inc."},
	"META": {CIK: 1326801, Name: "Meta Platforms, Inc."},
	"TSLA": {CIK: 1318605, Name: "Tesla, Inc."},
}

func enrichFromSEC(client *http.Client, out *StockMetrics) error {
	cik, name, err := lookupSECCIK(client, out.Symbol)
	if err != nil {
		return err
	}
	rawURL := fmt.Sprintf("%s/api/xbrl/companyfacts/CIK%010d.json", secBaseURL, cik)
	body, sourceFetchedAt, sourceTransport, sourceContentHash, err := fetchSECCompanyFacts(client, rawURL, cik)
	if err != nil {
		return err
	}
	var facts secCompanyFacts
	if err := json.Unmarshal(body, &facts); err != nil {
		return fmt.Errorf("parse SEC companyfacts: %w", err)
	}
	if facts.CIK != 0 && facts.CIK != cik {
		return fmt.Errorf("SEC companyfacts CIK mismatch")
	}
	if facts.EntityName != "" {
		name = facts.EntityName
	}
	out.SourceFetchedAt = sourceFetchedAt.UTC().Format(time.RFC3339)
	out.SourceTransport = sourceTransport
	out.SourceContentHash = sourceContentHash
	return applySECFacts(out, cik, name, &facts)
}

func fetchSECCompanyFacts(client *http.Client, rawURL string, cik int) ([]byte, time.Time, string, string, error) {
	if body, fetchedAt, contentHash, err := readSECCompanyFactsCache(rawURL, cik, time.Now().UTC()); err == nil {
		return body, fetchedAt, "verified-cache", contentHash, nil
	}
	body, err := secGet(client, rawURL)
	if err != nil {
		return nil, time.Time{}, "", "", err
	}
	fetchedAt := time.Now().UTC().Truncate(time.Second)
	contentHash, err := validateSECCompanyFactsBody(body, cik)
	if err != nil {
		return nil, time.Time{}, "", "", err
	}
	_ = writeSECCompanyFactsCache(rawURL, fetchedAt, contentHash, body, cik)
	return body, fetchedAt, "network", contentHash, nil
}

func validateSECCompanyFactsBody(body []byte, cik int) (string, error) {
	var identity struct {
		CIK   int                                   `json:"cik"`
		Facts map[string]map[string]json.RawMessage `json:"facts"`
	}
	if err := json.Unmarshal(body, &identity); err != nil {
		return "", fmt.Errorf("parse SEC companyfacts identity: %w", err)
	}
	if identity.CIK != cik || len(identity.Facts["us-gaap"]) == 0 {
		return "", fmt.Errorf("SEC companyfacts identity or us-gaap namespace mismatch")
	}
	sum := sha256.Sum256(body)
	return fmt.Sprintf("sha256:%x", sum[:]), nil
}

func secCompanyFactsCachePath(cik int) (string, error) {
	root := strings.TrimSpace(os.Getenv("TRADINGAGENTS_CACHE_DIR"))
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, ".tradingagents", "cache")
	}
	return filepath.Join(root, "sec-companyfacts", fmt.Sprintf("CIK%010d.cache.json", cik)), nil
}

func readSECCompanyFactsCache(rawURL string, cik int, now time.Time) ([]byte, time.Time, string, error) {
	path, err := secCompanyFactsCachePath(cik)
	if err != nil {
		return nil, time.Time{}, "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, "", err
	}
	var cached secCompanyFactsCacheEnvelope
	if err := json.Unmarshal(raw, &cached); err != nil {
		return nil, time.Time{}, "", err
	}
	fetchedAt, err := time.Parse(time.RFC3339, cached.FetchedAt)
	if err != nil || cached.URL != rawURL || now.Before(fetchedAt) || now.Sub(fetchedAt) > secCompanyFactsCacheMaxAge {
		return nil, time.Time{}, "", fmt.Errorf("SEC companyfacts cache metadata is invalid or stale")
	}
	contentHash, err := validateSECCompanyFactsBody(cached.Body, cik)
	if err != nil || contentHash != cached.ContentHash {
		return nil, time.Time{}, "", fmt.Errorf("SEC companyfacts cache failed content verification")
	}
	return append([]byte(nil), cached.Body...), fetchedAt, contentHash, nil
}

func writeSECCompanyFactsCache(rawURL string, fetchedAt time.Time, contentHash string, body []byte, cik int) error {
	path, err := secCompanyFactsCachePath(cik)
	if err != nil {
		return err
	}
	envelope, err := json.Marshal(secCompanyFactsCacheEnvelope{
		URL: rawURL, FetchedAt: fetchedAt.UTC().Format(time.RFC3339), ContentHash: contentHash, Body: append(json.RawMessage(nil), body...),
	})
	if err != nil {
		return err
	}
	secCacheMu.Lock()
	defer secCacheMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".companyfacts-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(envelope); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func lookupSECCIK(client *http.Client, ticker string) (int, string, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if seed, ok := verifiedSECIssuerSeeds[ticker]; ok {
		return seed.CIK, seed.Name, nil
	}
	cik, name, searchErr := lookupSECCIKEFTS(client, ticker)
	if searchErr == nil {
		return cik, name, nil
	}
	cik, name, atomErr := lookupSECCIKAtom(client, ticker)
	if atomErr == nil {
		return cik, name, nil
	}
	body, err := secGet(client, "https://www.sec.gov/files/company_tickers.json")
	mapErr := err
	if err == nil {
		var entries map[string]secTickerEntry
		if err := json.Unmarshal(body, &entries); err != nil {
			mapErr = fmt.Errorf("parse SEC ticker map: %w", err)
		} else {
			for _, entry := range entries {
				if strings.EqualFold(entry.Ticker, ticker) && entry.CIK > 0 {
					return entry.CIK, entry.Title, nil
				}
			}
			mapErr = fmt.Errorf("SEC ticker map has no CIK for %s", ticker)
		}
	}
	return 0, "", fmt.Errorf("SEC CIK lookup failed: search: %v; atom: %v; ticker map: %w", searchErr, atomErr, mapErr)
}

func lookupSECCIKEFTS(client *http.Client, ticker string) (int, string, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if ticker == "" {
		return 0, "", fmt.Errorf("empty SEC ticker")
	}
	values := url.Values{
		"q": {ticker}, "category": {"custom"}, "forms": {"10-K,10-Q"},
		"from": {"0"}, "size": {"20"},
	}
	body, err := secGet(client, "https://efts.sec.gov/LATEST/search-index?"+values.Encode())
	if err != nil {
		return 0, "", err
	}
	var response struct {
		Hits struct {
			Hits []struct {
				Source struct {
					CIKs         []string `json:"ciks"`
					DisplayNames []string `json:"display_names"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, "", fmt.Errorf("parse SEC filing search: %w", err)
	}
	marker := "(" + ticker + ")"
	for _, hit := range response.Hits.Hits {
		for index, displayName := range hit.Source.DisplayNames {
			if !strings.Contains(strings.ToUpper(displayName), marker) || index >= len(hit.Source.CIKs) {
				continue
			}
			cik, err := parseSECCIK(hit.Source.CIKs[index])
			if err != nil || cik <= 0 {
				continue
			}
			name := strings.TrimSpace(strings.SplitN(displayName, "  (", 2)[0])
			return cik, name, nil
		}
	}
	return 0, "", fmt.Errorf("SEC filing search has no exact ticker match for %s", ticker)
}

func lookupSECCIKAtom(client *http.Client, ticker string) (int, string, error) {
	values := url.Values{"action": {"getcompany"}, "CIK": {ticker}, "owner": {"exclude"}, "output": {"atom"}}
	body, err := secGet(client, "https://www.sec.gov/cgi-bin/browse-edgar?"+values.Encode())
	if err != nil {
		return 0, "", err
	}
	var feed struct {
		CompanyInfo struct {
			CIK  string `xml:"cik"`
			Name string `xml:"conformed-name"`
		} `xml:"company-info"`
	}
	if err := xml.Unmarshal(body, &feed); err != nil {
		return 0, "", fmt.Errorf("parse SEC company lookup: %w", err)
	}
	cik, err := parseSECCIK(feed.CompanyInfo.CIK)
	if err != nil || cik <= 0 {
		return 0, "", fmt.Errorf("SEC company lookup has no valid CIK for %s", ticker)
	}
	return cik, strings.TrimSpace(feed.CompanyInfo.Name), nil
}

func applySECFacts(out *StockMetrics, cik int, name string, payload *secCompanyFacts) error {
	if out == nil || payload == nil {
		return fmt.Errorf("nil SEC fundamentals payload")
	}
	concepts := payload.Facts["us-gaap"]
	anchor, ok := latestSECDurationFact(concepts, []string{
		"RevenueFromContractWithCustomerExcludingAssessedTax", "Revenues", "SalesRevenueNet",
	})
	if !ok {
		anchor, ok = latestSECFact(concepts, []string{"Assets"})
	}
	if !ok || anchor.Accn == "" || anchor.End == "" || anchor.Filed == "" || anchor.FP == "" {
		return fmt.Errorf("SEC companyfacts has no complete 10-K/10-Q anchor")
	}
	period := secPeriodIdentity(secFactFrequency(anchor), anchor.Start, anchor.End)
	sourceURL := secFilingIndexURL(cik, anchor.Accn)
	out.Name = name
	out.Source = "sec-edgar-companyfacts"
	out.FiscalPeriod = period
	out.AsOf = anchor.End
	out.FilingDate = anchor.Filed
	out.Accession = anchor.Accn
	out.SourceURL = sourceURL
	if out.FieldSources == nil {
		out.FieldSources = map[string]FieldProvenance{}
	}
	out.SourceLinks = append(out.SourceLinks, SourceReference{Source: out.Source, Label: "SEC EDGAR filing", URL: sourceURL})

	type target struct {
		field string
		tags  []string
		set   func(float64)
	}
	targets := []target{
		{"totalRevenue", []string{"RevenueFromContractWithCustomerExcludingAssessedTax", "Revenues", "SalesRevenueNet"}, func(v float64) { out.TotalRevenue = v }},
		{"netIncome", []string{"NetIncomeLoss", "ProfitLoss"}, func(v float64) { out.NetIncome = v }},
		{"totalAssets", []string{"Assets"}, func(v float64) { out.TotalAssets = v }},
		{"totalLiabilities", []string{"Liabilities"}, func(v float64) { out.TotalLiabilities = v }},
		{"stockholdersEquity", []string{"StockholdersEquity", "StockholdersEquityIncludingPortionAttributableToNoncontrollingInterest"}, func(v float64) { out.StockholdersEquity = v }},
	}
	for _, item := range targets {
		fact, found := secFactForPeriod(concepts, item.tags, anchor, item.field != "totalAssets" && item.field != "totalLiabilities" && item.field != "stockholdersEquity")
		if !found {
			continue
		}
		item.set(fact.Value)
		periodStart := fact.Start
		if item.field == "totalAssets" || item.field == "totalLiabilities" || item.field == "stockholdersEquity" {
			periodStart = fact.End
		}
		out.FieldSources[item.field] = FieldProvenance{
			Source: out.Source, URL: sourceURL, AsOf: fact.End, FiscalPeriod: period,
			FilingDate: fact.Filed, Accession: fact.Accn, PeriodStart: periodStart,
			Frequency: secFactFrequency(fact), Form: fact.Form, Unit: "USD",
		}
	}
	if len(out.FieldSources) == 0 {
		return fmt.Errorf("SEC filing contains no supported facts")
	}
	out.Periods = secFinancialPeriods(cik, concepts, payload.Facts["dei"])
	out.DerivationInputs = secFinancialDerivationInputs(cik, concepts)
	return nil
}

func secFinancialPeriods(cik int, concepts map[string]struct {
	Units map[string][]secFact `json:"units"`
}, shareConcepts map[string]struct {
	Units map[string][]secFact `json:"units"`
}) []FinancialPeriod {
	anchors := make([]secFact, 0)
	for _, tag := range []string{"RevenueFromContractWithCustomerExcludingAssessedTax", "Revenues", "SalesRevenueNet"} {
		anchors = append(anchors, concepts[tag].Units["USD"]...)
	}
	valid := anchors[:0]
	for _, fact := range anchors {
		if validSECDurationFact(fact) {
			valid = append(valid, fact)
		}
	}
	sort.SliceStable(valid, func(i, j int) bool {
		if valid[i].End == valid[j].End {
			if valid[i].Filed == valid[j].Filed && valid[i].Form == valid[j].Form {
				if valid[i].Form == "10-Q" {
					return valid[i].Start > valid[j].Start
				}
				return valid[i].Start < valid[j].Start
			}
			return valid[i].Filed > valid[j].Filed
		}
		return valid[i].End > valid[j].End
	})
	seen := map[string]bool{}
	out := make([]FinancialPeriod, 0, 8)
	quarterCount, annualCount := 0, 0
	for _, anchor := range valid {
		frequency := "quarterly"
		limit := 5
		if anchor.Form == "10-K" {
			frequency, limit = "annual", 3
		}
		if (frequency == "quarterly" && quarterCount >= limit) || (frequency == "annual" && annualCount >= limit) {
			continue
		}
		key := anchor.Form + "|" + anchor.End
		if seen[key] {
			continue
		}
		seen[key] = true
		period := FinancialPeriod{
			FiscalPeriod: secPeriodIdentity(frequency, anchor.Start, anchor.End),
			Frequency:    frequency, Form: anchor.Form, PeriodStart: anchor.Start,
			PeriodEnd: anchor.End, FilingDate: anchor.Filed, Accession: anchor.Accn,
			SourceURL: secFilingIndexURL(cik, anchor.Accn), TotalRevenue: anchor.Value, AvailableFields: []string{"totalRevenue"},
			FieldEvidence: map[string]FinancialFieldEvidence{"totalRevenue": secFinancialFieldEvidence(cik, anchor, "duration", "USD")},
		}
		for _, field := range []struct {
			name string
			tags []string
			set  func(float64)
		}{
			{"grossProfit", []string{"GrossProfit"}, func(value float64) { period.GrossProfit = value }},
			{"operatingIncome", []string{"OperatingIncomeLoss"}, func(value float64) { period.OperatingIncome = value }},
			{"netIncome", []string{"NetIncomeLoss", "ProfitLoss"}, func(value float64) { period.NetIncome = value }},
			{"totalAssets", []string{"Assets"}, func(value float64) { period.TotalAssets = value }},
			{"totalLiabilities", []string{"Liabilities"}, func(value float64) { period.TotalLiabilities = value }},
			{"stockholdersEquity", []string{"StockholdersEquity", "StockholdersEquityIncludingPortionAttributableToNoncontrollingInterest"}, func(value float64) { period.StockholdersEquity = value }},
			{"cashAndEquivalents", []string{"CashAndCashEquivalentsAtCarryingValue", "CashCashEquivalentsRestrictedCashAndRestrictedCashEquivalents"}, func(value float64) { period.CashAndEquivalents = value }},
		} {
			durationMetric := field.name == "grossProfit" || field.name == "operatingIncome" || field.name == "netIncome"
			if fact, present := secFactForPeriod(concepts, field.tags, anchor, durationMetric); present {
				field.set(fact.Value)
				period.AvailableFields = append(period.AvailableFields, field.name)
				kind := "instant"
				if durationMetric {
					kind = "duration"
				}
				period.FieldEvidence[field.name] = secFinancialFieldEvidence(cik, fact, kind, "USD")
			}
		}
		if shares, present := secSharesForFiling(shareConcepts, anchor); present {
			period.SharesOutstanding = shares.Value
			period.AvailableFields = append(period.AvailableFields, "sharesOutstanding")
			period.FieldEvidence["sharesOutstanding"] = secFinancialFieldEvidence(cik, shares, "instant", "shares")
		}
		out = append(out, period)
		if frequency == "quarterly" {
			quarterCount++
		} else {
			annualCount++
		}
		if quarterCount >= 5 && annualCount >= 3 {
			break
		}
	}
	return out
}

func secPeriodIdentity(frequency, start, end string) string {
	frequency = strings.ToLower(strings.TrimSpace(frequency))
	if frequency == "" {
		frequency = "unknown"
	}
	if strings.TrimSpace(start) == "" {
		return frequency + ":" + end
	}
	return frequency + ":" + start + "/" + end
}

func secFinancialFieldEvidence(cik int, fact secFact, kind, unit string) FinancialFieldEvidence {
	start := fact.Start
	if kind == "instant" {
		start = fact.End
	}
	return FinancialFieldEvidence{
		Source: "sec-edgar-companyfacts", URL: secFilingIndexURL(cik, fact.Accn), Unit: unit,
		PeriodStart: start, PeriodEnd: fact.End, Form: fact.Form, Filed: fact.Filed,
		Accession: fact.Accn, PeriodKind: kind,
	}
}

func secSharesForFiling(concepts map[string]struct {
	Units map[string][]secFact `json:"units"`
}, anchor secFact) (secFact, bool) {
	var matches []secFact
	anchorEnd, anchorEndErr := time.Parse("2006-01-02", anchor.End)
	for _, tag := range []string{"EntityCommonStockSharesOutstanding", "CommonStockSharesOutstanding"} {
		for _, fact := range concepts[tag].Units["shares"] {
			observed, observedErr := time.Parse("2006-01-02", fact.End)
			if fact.Accn == anchor.Accn && fact.Form == anchor.Form && fact.Filed == anchor.Filed && fact.Start == "" && fact.Value > 0 && anchorEndErr == nil && observedErr == nil {
				days := int(observed.Sub(anchorEnd).Hours() / 24)
				if days >= 0 && days <= 60 {
					matches = append(matches, fact)
				}
			}
		}
	}
	if len(matches) == 0 {
		return secFact{}, false
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].End > matches[j].End })
	return matches[0], true
}

// secFinancialDerivationInputs retains reported Q1-Q3 YTD facts as supporting
// evidence. They are never labeled or counted as discrete quarters.
func secFinancialDerivationInputs(cik int, concepts map[string]struct {
	Units map[string][]secFact `json:"units"`
}) []FinancialPeriod {
	anchors := make([]secFact, 0)
	for _, tag := range []string{"RevenueFromContractWithCustomerExcludingAssessedTax", "Revenues", "SalesRevenueNet"} {
		anchors = append(anchors, concepts[tag].Units["USD"]...)
	}
	valid := anchors[:0]
	for _, fact := range anchors {
		if fact.Form != "10-Q" || fact.Start == "" || fact.End == "" || fact.Filed == "" || fact.Accn == "" {
			continue
		}
		start, startErr := time.Parse("2006-01-02", fact.Start)
		end, endErr := time.Parse("2006-01-02", fact.End)
		if startErr != nil || endErr != nil {
			continue
		}
		days := int(end.Sub(start).Hours()/24) + 1
		if days >= 230 && days <= 300 {
			valid = append(valid, fact)
		}
	}
	sortSECFacts(valid)
	seen := map[string]bool{}
	out := make([]FinancialPeriod, 0, 4)
	for _, anchor := range valid {
		key := anchor.Start + "|" + anchor.End
		if seen[key] {
			continue
		}
		seen[key] = true
		period := FinancialPeriod{
			FiscalPeriod: secPeriodIdentity("year-to-date", anchor.Start, anchor.End), Frequency: "year-to-date",
			Form: anchor.Form, PeriodStart: anchor.Start, PeriodEnd: anchor.End, FilingDate: anchor.Filed,
			Accession: anchor.Accn, SourceURL: secFilingIndexURL(cik, anchor.Accn), TotalRevenue: anchor.Value,
			AvailableFields: []string{"totalRevenue"},
			FieldEvidence:   map[string]FinancialFieldEvidence{"totalRevenue": secFinancialFieldEvidence(cik, anchor, "duration", "USD")},
		}
		if fact, present := secFactForPeriod(concepts, []string{"NetIncomeLoss", "ProfitLoss"}, anchor, true); present {
			period.NetIncome = fact.Value
			period.AvailableFields = append(period.AvailableFields, "netIncome")
			period.FieldEvidence["netIncome"] = secFinancialFieldEvidence(cik, fact, "duration", "USD")
		}
		out = append(out, period)
		if len(out) == 4 {
			break
		}
	}
	return out
}

func latestSECDurationFact(concepts map[string]struct {
	Units map[string][]secFact `json:"units"`
}, tags []string) (secFact, bool) {
	var candidates []secFact
	for _, tag := range tags {
		for _, fact := range concepts[tag].Units["USD"] {
			if validSECDurationFact(fact) {
				candidates = append(candidates, fact)
			}
		}
	}
	if len(candidates) == 0 {
		return secFact{}, false
	}
	sortSECFacts(candidates)
	return candidates[0], true
}

func validSECDurationFact(fact secFact) bool {
	if (fact.Form != "10-K" && fact.Form != "10-Q") || fact.Start == "" || fact.End == "" || fact.Filed == "" || fact.Accn == "" || fact.FP == "" {
		return false
	}
	start, startErr := time.Parse("2006-01-02", fact.Start)
	end, endErr := time.Parse("2006-01-02", fact.End)
	if startErr != nil || endErr != nil || !start.Before(end) {
		return false
	}
	days := int(end.Sub(start).Hours()/24) + 1
	if fact.Form == "10-Q" {
		return days >= 70 && days <= 110
	}
	return days >= 330 && days <= 400
}

func secFactFrequency(fact secFact) string {
	if fact.Form == "10-Q" {
		return "quarterly"
	}
	if fact.Form == "10-K" {
		return "annual"
	}
	return "unknown"
}

func sortSECFacts(facts []secFact) {
	sort.SliceStable(facts, func(i, j int) bool {
		if facts[i].End == facts[j].End {
			if facts[i].Filed == facts[j].Filed && facts[i].Form == facts[j].Form {
				if facts[i].Form == "10-Q" {
					return facts[i].Start > facts[j].Start
				}
				return facts[i].Start < facts[j].Start
			}
			return facts[i].Filed > facts[j].Filed
		}
		return facts[i].End > facts[j].End
	})
}

func latestSECFact(concepts map[string]struct {
	Units map[string][]secFact `json:"units"`
}, tags []string) (secFact, bool) {
	var candidates []secFact
	for _, tag := range tags {
		candidates = append(candidates, concepts[tag].Units["USD"]...)
	}
	valid := candidates[:0]
	for _, fact := range candidates {
		if (fact.Form == "10-K" || fact.Form == "10-Q") && fact.End != "" && fact.Filed != "" && fact.Accn != "" && fact.FP != "" {
			valid = append(valid, fact)
		}
	}
	if len(valid) == 0 {
		return secFact{}, false
	}
	sortSECFacts(valid)
	return valid[0], true
}

func secFactForPeriod(concepts map[string]struct {
	Units map[string][]secFact `json:"units"`
}, tags []string, anchor secFact, durationMetric bool) (secFact, bool) {
	var matches []secFact
	for _, tag := range tags {
		for _, fact := range concepts[tag].Units["USD"] {
			if fact.Accn != anchor.Accn || fact.End != anchor.End || fact.Form != anchor.Form || fact.Filed != anchor.Filed {
				continue
			}
			if durationMetric && fact.Start != anchor.Start {
				continue
			}
			if !durationMetric && fact.Start != "" {
				continue
			}
			matches = append(matches, fact)
		}
	}
	if len(matches) == 0 {
		return secFact{}, false
	}
	return matches[0], true
}

func secFilingIndexURL(cik int, accession string) string {
	compact := strings.ReplaceAll(accession, "-", "")
	return fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%d/%s/%s-index.html", cik, compact, url.PathEscape(accession))
}

func secGet(client *http.Client, rawURL string) ([]byte, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	agent := strings.TrimSpace(os.Getenv("SEC_EDGAR_USER_AGENT"))
	if agent == "" {
		agent = "TradingAgents/1.0 research-contact=admin@localhost"
	}
	req.Header.Set("User-Agent", agent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SEC HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	return body, nil
}

func parseSECCIK(value string) (int, error) {
	return strconv.Atoi(strings.TrimLeft(value, "0"))
}
