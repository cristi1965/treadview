package dataflows

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type nasdaqFinancialTable struct {
	AsOf    string `json:"asOf"`
	Headers struct {
		Value1 string `json:"value1"`
		Value2 string `json:"value2"`
		Value3 string `json:"value3"`
		Value4 string `json:"value4"`
		Value5 string `json:"value5"`
	} `json:"headers"`
	Rows []map[string]string `json:"rows"`
}

type nasdaqFinancialPayload struct {
	Data struct {
		Symbol          string               `json:"symbol"`
		IncomeStatement nasdaqFinancialTable `json:"incomeStatementTable"`
		BalanceSheet    nasdaqFinancialTable `json:"balanceSheetTable"`
		CashFlow        nasdaqFinancialTable `json:"cashFlowTable"`
	} `json:"data"`
	Status struct {
		Code int `json:"rCode"`
	} `json:"status"`
}

// enrichFromNasdaqFinancials is used only after SEC EDGAR is unavailable.
// Nasdaq's page states that all table values are in USD thousands.
func enrichFromNasdaqFinancials(client *http.Client, out *StockMetrics) error {
	var errs []string
	var candidates []*StockMetrics
	for _, frequency := range []int{2, 1} {
		endpoint := fmt.Sprintf("https://api.nasdaq.com/api/company/%s/financials?frequency=%d", url.PathEscape(out.Symbol), frequency)
		body, err := httpGet(client, endpoint, map[string]string{
			"Accept": "application/json", "Origin": "https://www.nasdaq.com", "Referer": "https://www.nasdaq.com/",
		})
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		candidate := &StockMetrics{Symbol: out.Symbol, Name: out.Name, Sector: out.Sector, Industry: out.Industry}
		if err := parseNasdaqFinancials(body, candidate, endpoint, frequency); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		if err := enrichNasdaqFilingMetadata(client, candidate, frequency); err != nil {
			errs = append(errs, err.Error())
			continue
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		return fmt.Errorf("Nasdaq financials unavailable: %s", strings.Join(errs, "; "))
	}
	primary := candidates[0]
	mergedPeriods := make([]FinancialPeriod, 0, 8)
	seen := map[string]bool{}
	mergedLinks := make([]SourceReference, 0, len(candidates)*2)
	seenLinks := map[string]bool{}
	for _, candidate := range candidates {
		for _, period := range candidate.Periods {
			key := period.Frequency + "|" + period.PeriodEnd
			if !seen[key] {
				seen[key] = true
				mergedPeriods = append(mergedPeriods, period)
			}
		}
		for _, link := range candidate.SourceLinks {
			key := link.Source + "|" + link.URL
			if !seenLinks[key] {
				seenLinks[key] = true
				mergedLinks = append(mergedLinks, link)
			}
		}
	}
	sort.SliceStable(mergedPeriods, func(i, j int) bool { return mergedPeriods[i].PeriodEnd > mergedPeriods[j].PeriodEnd })
	primary.Periods = mergedPeriods
	primary.SourceLinks = mergedLinks
	*out = *primary
	return nil
}

func parseNasdaqFinancials(body []byte, out *StockMetrics, endpoint string, frequency int) error {
	if out == nil {
		return fmt.Errorf("nil fundamentals target")
	}
	var payload nasdaqFinancialPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("parse Nasdaq financials: %w", err)
	}
	if payload.Status.Code != 0 && payload.Status.Code != http.StatusOK {
		return fmt.Errorf("Nasdaq financials status %d", payload.Status.Code)
	}
	if payload.Data.Symbol != "" && !strings.EqualFold(payload.Data.Symbol, out.Symbol) {
		return fmt.Errorf("Nasdaq financials symbol mismatch")
	}
	periodRaw := firstNonEmpty(payload.Data.IncomeStatement.Headers.Value2, payload.Data.BalanceSheet.Headers.Value2)
	periodEnd, err := time.Parse("1/2/2006", strings.TrimSpace(periodRaw))
	if err != nil {
		return fmt.Errorf("Nasdaq financials period unavailable: %q", periodRaw)
	}
	frequencyLabel := "annual"
	headerLabel := strings.ToLower(payload.Data.IncomeStatement.Headers.Value1)
	if frequency == 2 || strings.Contains(headerLabel, "quarter") {
		frequencyLabel = "quarterly"
	}
	period := frequencyLabel + ":" + periodEnd.Format("2006-01-02")
	if out.FieldSources == nil {
		out.FieldSources = map[string]FieldProvenance{}
	}
	fields := 0
	apply := func(table nasdaqFinancialTable, labels map[string]func(float64)) {
		for _, row := range table.Rows {
			setter := labels[strings.ToLower(strings.TrimSpace(row["value1"]))]
			if setter == nil {
				continue
			}
			value, ok := parseNasdaqUSDThousands(row["value2"])
			if !ok {
				continue
			}
			setter(value)
			fieldName := nasdaqFinancialFieldName(row["value1"])
			out.FieldSources[fieldName] = FieldProvenance{
				Source: "nasdaq-financials", URL: endpoint, AsOf: periodEnd.Format("2006-01-02"),
				FiscalPeriod: period, Frequency: frequencyLabel, Unit: "USD",
			}
			fields++
		}
	}
	apply(payload.Data.IncomeStatement, map[string]func(float64){
		"total revenue":    func(v float64) { out.TotalRevenue = v },
		"gross profit":     func(v float64) { out.GrossProfit = v },
		"operating income": func(v float64) { out.OperatingIncome = v },
		"net income":       func(v float64) { out.NetIncome = v },
	})
	apply(payload.Data.BalanceSheet, map[string]func(float64){
		"cash and cash equivalents": func(v float64) { out.CashAndEquivalents = v },
		"total assets":              func(v float64) { out.TotalAssets = v },
		"total liabilities":         func(v float64) { out.TotalLiabilities = v },
		"total equity":              func(v float64) { out.StockholdersEquity = v },
	})
	if fields == 0 {
		return fmt.Errorf("Nasdaq financials has no supported fields")
	}
	out.Periods = nasdaqFinancialPeriods(payload, frequencyLabel)
	if len(out.Periods) == 0 {
		return fmt.Errorf("Nasdaq financials has no parseable periods")
	}
	out.Source = "nasdaq-financials"
	out.FiscalPeriod = period
	out.AsOf = periodEnd.Format("2006-01-02")
	out.SourceURL = endpoint
	out.SourceLinks = []SourceReference{{
		Source: "nasdaq-financials", Label: "Nasdaq financials (USD thousands)",
		URL: endpoint,
	}}
	return nil
}

func nasdaqFinancialPeriods(payload nasdaqFinancialPayload, frequencyLabel string) []FinancialPeriod {
	headers := payload.Data.IncomeStatement.Headers
	if headers.Value2 == "" {
		headers = payload.Data.BalanceSheet.Headers
	}
	columns := []struct {
		key  string
		date string
	}{{"value2", headers.Value2}, {"value3", headers.Value3}, {"value4", headers.Value4}, {"value5", headers.Value5}}
	out := make([]FinancialPeriod, 0, len(columns))
	for _, column := range columns {
		periodEnd, err := time.Parse("1/2/2006", strings.TrimSpace(column.date))
		if err != nil {
			continue
		}
		form := "10-K"
		if frequencyLabel == "quarterly" {
			form = "10-Q"
		}
		period := FinancialPeriod{FiscalPeriod: frequencyLabel + ":" + periodEnd.Format("2006-01-02"), Frequency: frequencyLabel, Form: form, PeriodEnd: periodEnd.Format("2006-01-02")}
		applyPeriodRows := func(table nasdaqFinancialTable) {
			for _, row := range table.Rows {
				value, ok := parseNasdaqUSDThousands(row[column.key])
				if !ok {
					continue
				}
				switch strings.ToLower(strings.TrimSpace(row["value1"])) {
				case "total revenue":
					period.TotalRevenue = value
					period.AvailableFields = append(period.AvailableFields, "totalRevenue")
				case "gross profit":
					period.GrossProfit = value
					period.AvailableFields = append(period.AvailableFields, "grossProfit")
				case "operating income":
					period.OperatingIncome = value
					period.AvailableFields = append(period.AvailableFields, "operatingIncome")
				case "net income":
					period.NetIncome = value
					period.AvailableFields = append(period.AvailableFields, "netIncome")
				case "cash and cash equivalents":
					period.CashAndEquivalents = value
					period.AvailableFields = append(period.AvailableFields, "cashAndEquivalents")
				case "total assets":
					period.TotalAssets = value
					period.AvailableFields = append(period.AvailableFields, "totalAssets")
				case "total liabilities":
					period.TotalLiabilities = value
					period.AvailableFields = append(period.AvailableFields, "totalLiabilities")
				case "total equity":
					period.StockholdersEquity = value
					period.AvailableFields = append(period.AvailableFields, "stockholdersEquity")
				}
			}
		}
		applyPeriodRows(payload.Data.IncomeStatement)
		applyPeriodRows(payload.Data.BalanceSheet)
		applyPeriodRows(payload.Data.CashFlow)
		seenFields := map[string]bool{}
		uniqueFields := period.AvailableFields[:0]
		for _, field := range period.AvailableFields {
			if !seenFields[field] {
				seenFields[field] = true
				uniqueFields = append(uniqueFields, field)
			}
		}
		period.AvailableFields = uniqueFields
		out = append(out, period)
	}
	return out
}

func parseNasdaqUSDThousands(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "--" || strings.EqualFold(value, "n/a") {
		return 0, false
	}
	negative := strings.HasPrefix(value, "-") || strings.HasPrefix(value, "($")
	cleaned := strings.NewReplacer("$", "", ",", "", "-", "", "(", "", ")", "").Replace(value)
	parsed, err := strconv.ParseFloat(strings.TrimSpace(cleaned), 64)
	if err != nil {
		return 0, false
	}
	if negative {
		parsed = -parsed
	}
	return parsed * 1000, true
}

func nasdaqFinancialFieldName(label string) string {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "total revenue":
		return "totalRevenue"
	case "gross profit":
		return "grossProfit"
	case "operating income":
		return "operatingIncome"
	case "net income":
		return "netIncome"
	case "cash and cash equivalents":
		return "cashAndEquivalents"
	case "total assets":
		return "totalAssets"
	case "total liabilities":
		return "totalLiabilities"
	case "total equity":
		return "stockholdersEquity"
	default:
		return ""
	}
}
