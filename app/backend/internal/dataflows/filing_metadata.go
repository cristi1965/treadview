package dataflows

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type filingMetadata struct {
	PeriodEnd  string
	FilingDate string
	Accession  string
	SourceURL  string
	Source     string
}

type secSubmissionsPayload struct {
	CIK     string   `json:"cik"`
	Tickers []string `json:"tickers"`
	Filings struct {
		Recent struct {
			AccessionNumber []string `json:"accessionNumber"`
			FilingDate      []string `json:"filingDate"`
			ReportDate      []string `json:"reportDate"`
			Form            []string `json:"form"`
		} `json:"recent"`
	} `json:"filings"`
}

type nasdaqFilingsPayload struct {
	Data struct {
		Symbol string `json:"symbol"`
		Rows   []struct {
			FormType string `json:"formType"`
			Filed    string `json:"filed"`
			Period   string `json:"period"`
			View     struct {
				HTMLLink string `json:"htmlLink"`
			} `json:"view"`
		} `json:"rows"`
	} `json:"data"`
	Status struct {
		Code int `json:"rCode"`
	} `json:"status"`
}

func enrichNasdaqFilingMetadata(client *http.Client, out *StockMetrics, frequency int) error {
	if out == nil || out.Symbol == "" || out.AsOf == "" {
		return fmt.Errorf("fundamentals period is incomplete")
	}
	periodEnds := make([]string, 0, len(out.Periods)+1)
	for _, period := range out.Periods {
		periodEnds = append(periodEnds, period.PeriodEnd)
	}
	if len(periodEnds) == 0 {
		periodEnds = append(periodEnds, out.AsOf)
	}

	metadata, secErr := fetchSECSubmissionFilings(client, out.Symbol, periodEnds, frequency)
	if len(metadata) == 0 {
		var nasdaqErr error
		metadata, nasdaqErr = fetchNasdaqFilings(client, out.Symbol, periodEnds, frequency)
		if len(metadata) == 0 {
			return fmt.Errorf("verified filing metadata unavailable (SEC=%v; Nasdaq=%v)", secErr, nasdaqErr)
		}
	}
	return applyFilingMetadata(out, metadata)
}

func fetchSECSubmissionFilings(client *http.Client, symbol string, periodEnds []string, frequency int) (map[string]filingMetadata, error) {
	cik, _, err := lookupSECCIK(client, symbol)
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s/submissions/CIK%010d.json", secBaseURL, cik)
	body, err := secGet(client, endpoint)
	if err != nil {
		return nil, err
	}
	return parseSECSubmissionFilings(body, symbol, cik, periodEnds, frequency)
}

func parseSECSubmissionFilings(body []byte, symbol string, cik int, periodEnds []string, frequency int) (map[string]filingMetadata, error) {
	var payload secSubmissionsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse SEC submissions: %w", err)
	}
	if payload.CIK != "" {
		parsedCIK, err := parseSECCIK(payload.CIK)
		if err != nil || parsedCIK != cik {
			return nil, fmt.Errorf("SEC submissions CIK mismatch")
		}
	}
	if len(payload.Tickers) > 0 && !containsFold(payload.Tickers, symbol) {
		return nil, fmt.Errorf("SEC submissions symbol mismatch")
	}
	wanted := stringSet(periodEnds)
	form := filingFormForFrequency(frequency)
	recent := payload.Filings.Recent
	out := map[string]filingMetadata{}
	for i, reportDate := range recent.ReportDate {
		if !wanted[reportDate] || stringAt(recent.Form, i) != form {
			continue
		}
		accession := stringAt(recent.AccessionNumber, i)
		filed := stringAt(recent.FilingDate, i)
		if accession == "" || filed == "" {
			continue
		}
		out[reportDate] = filingMetadata{
			PeriodEnd: reportDate, FilingDate: filed, Accession: accession,
			SourceURL: secFilingIndexURL(cik, accession), Source: "sec-edgar-submissions",
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("SEC submissions has no %s matching requested periods", form)
	}
	return out, nil
}

func fetchNasdaqFilings(client *http.Client, symbol string, periodEnds []string, frequency int) (map[string]filingMetadata, error) {
	endpoint := fmt.Sprintf("https://api.nasdaq.com/api/company/%s/sec-filings?limit=50&filingtype=%s", url.PathEscape(symbol), url.QueryEscape(filingFormForFrequency(frequency)))
	body, err := httpGet(client, endpoint, map[string]string{
		"Accept": "application/json", "Origin": "https://www.nasdaq.com", "Referer": "https://www.nasdaq.com/",
	})
	if err != nil {
		return nil, err
	}
	return parseNasdaqFilings(body, symbol, periodEnds, frequency)
}

func parseNasdaqFilings(body []byte, symbol string, periodEnds []string, frequency int) (map[string]filingMetadata, error) {
	var payload nasdaqFilingsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse Nasdaq SEC filings: %w", err)
	}
	if payload.Status.Code != 0 && payload.Status.Code != http.StatusOK {
		return nil, fmt.Errorf("Nasdaq SEC filings status %d", payload.Status.Code)
	}
	if payload.Data.Symbol != "" && !strings.EqualFold(payload.Data.Symbol, symbol) {
		return nil, fmt.Errorf("Nasdaq SEC filings symbol mismatch")
	}
	wanted := stringSet(periodEnds)
	form := filingFormForFrequency(frequency)
	out := map[string]filingMetadata{}
	for _, row := range payload.Data.Rows {
		if row.FormType != form {
			continue
		}
		periodAt, periodErr := time.Parse("01/02/2006", strings.TrimSpace(row.Period))
		filedAt, filedErr := time.Parse("01/02/2006", strings.TrimSpace(row.Filed))
		if periodErr != nil || filedErr != nil || !wanted[periodAt.Format("2006-01-02")] {
			continue
		}
		parsedURL, err := url.Parse(row.View.HTMLLink)
		if err != nil || parsedURL.Scheme != "https" || !strings.EqualFold(parsedURL.Query().Get("symbol"), symbol) {
			continue
		}
		ref := strings.TrimSpace(parsedURL.Query().Get("ref"))
		if ref == "" {
			continue
		}
		periodEnd := periodAt.Format("2006-01-02")
		out[periodEnd] = filingMetadata{
			PeriodEnd: periodEnd, FilingDate: filedAt.Format("2006-01-02"),
			Accession: "nasdaq-ref:" + ref, SourceURL: row.View.HTMLLink, Source: "nasdaq-sec-filings",
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("Nasdaq SEC filings has no %s matching requested periods", form)
	}
	return out, nil
}

func applyFilingMetadata(out *StockMetrics, metadata map[string]filingMetadata) error {
	primary, ok := metadata[out.AsOf]
	if !ok {
		return fmt.Errorf("filing metadata does not match primary period %s", out.AsOf)
	}
	out.FilingDate = primary.FilingDate
	out.Accession = primary.Accession
	out.SourceURL = primary.SourceURL
	verifiedPeriods := make([]FinancialPeriod, 0, len(out.Periods))
	for _, period := range out.Periods {
		filing, ok := metadata[period.PeriodEnd]
		if !ok {
			continue
		}
		period.FilingDate = filing.FilingDate
		period.Accession = filing.Accession
		period.SourceURL = filing.SourceURL
		verifiedPeriods = append(verifiedPeriods, period)
	}
	out.Periods = verifiedPeriods
	for name, source := range out.FieldSources {
		if source.AsOf != out.AsOf {
			continue
		}
		source.FilingDate = primary.FilingDate
		source.Accession = primary.Accession
		out.FieldSources[name] = source
	}
	out.SourceLinks = append(out.SourceLinks, SourceReference{Source: primary.Source, Label: "Matched regulatory filing", URL: primary.SourceURL})
	return nil
}

func filingFormForFrequency(frequency int) string {
	if frequency == 2 {
		return "10-Q"
	}
	return "10-K"
}

func containsFold(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(wanted)) {
			return true
		}
	}
	return false
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out[value] = true
		}
	}
	return out
}

func stringAt(values []string, index int) string {
	if index < 0 || index >= len(values) {
		return ""
	}
	return strings.TrimSpace(values[index])
}
