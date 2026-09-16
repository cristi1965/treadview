package dataflows

import "testing"

func TestParseNasdaqFinancialsKeepsExplicitPeriodAndFieldSources(t *testing.T) {
	raw := []byte(`{"data":{"symbol":"AAPL","incomeStatementTable":{"headers":{"value1":"Quarterly Ending:","value2":"6/27/2026"},"rows":[{"value1":"Total Revenue","value2":"$109,417,000"},{"value1":"Net Income","value2":"$23,434,000"},{"value1":"Operating Expenses","value2":""}]},"balanceSheetTable":{"headers":{"value1":"Quarterly Ending:","value2":"6/27/2026"},"rows":[{"value1":"Total Assets","value2":"$331,495,000"},{"value1":"Total Equity","value2":"$65,849,000"}]}},"status":{"rCode":200}}`)
	out := &StockMetrics{Symbol: "AAPL"}
	if err := parseNasdaqFinancials(raw, out, "fixture://nasdaq-financials", 2); err != nil {
		t.Fatal(err)
	}
	if out.FiscalPeriod != "quarterly:2026-06-27" || out.AsOf != "2026-06-27" {
		t.Fatalf("period metadata=%+v", out)
	}
	if out.TotalRevenue != 109_417_000_000 || out.NetIncome != 23_434_000_000 || out.TotalAssets != 331_495_000_000 {
		t.Fatalf("USD thousands conversion failed: %+v", out)
	}
	if out.FieldSources["totalRevenue"].Frequency != "quarterly" || out.FieldSources["totalRevenue"].URL != "fixture://nasdaq-financials" {
		t.Fatalf("field source=%+v", out.FieldSources["totalRevenue"])
	}
	if _, ok := out.FieldSources["operatingExpenses"]; ok {
		t.Fatal("unknown/empty field must not be synthesized")
	}
}

func TestParseNasdaqUSDThousandsRejectsUnknownAndPreservesNegative(t *testing.T) {
	for _, value := range []string{"", "--", "N/A"} {
		if _, ok := parseNasdaqUSDThousands(value); ok {
			t.Fatalf("accepted unknown value %q", value)
		}
	}
	if got, ok := parseNasdaqUSDThousands("-$321,000"); !ok || got != -321_000_000 {
		t.Fatalf("negative parse got=%v ok=%v", got, ok)
	}
}

func TestParseNasdaqFinancialsCapturesMultiplePeriods(t *testing.T) {
	raw := []byte(`{"data":{"symbol":"AAPL","incomeStatementTable":{"headers":{"value1":"Quarterly Ending:","value2":"6/27/2026","value3":"3/28/2026"},"rows":[{"value1":"Total Revenue","value2":"$109,417,000","value3":"$95,359,000"}]},"balanceSheetTable":{"headers":{"value1":"Quarterly Ending:","value2":"6/27/2026","value3":"3/28/2026"},"rows":[{"value1":"Total Assets","value2":"$331,495,000","value3":"$330,000,000"}]}} ,"status":{"rCode":200}}`)
	out := &StockMetrics{Symbol: "AAPL"}
	if err := parseNasdaqFinancials(raw, out, "fixture://nasdaq-financials", 2); err != nil {
		t.Fatal(err)
	}
	if len(out.Periods) != 2 || out.Periods[1].PeriodEnd != "2026-03-28" || out.Periods[1].TotalRevenue != 95_359_000_000 {
		t.Fatalf("multi-period financials missing: %+v", out.Periods)
	}
}

func TestParseNasdaqFinancialsAnnualPeriod(t *testing.T) {
	raw := []byte(`{"data":{"symbol":"AAPL","incomeStatementTable":{"headers":{"value1":"Period Ending:","value2":"9/27/2025"},"rows":[{"value1":"Gross Profit","value2":"$195,201,000"}]}} ,"status":{"rCode":200}}`)
	out := &StockMetrics{Symbol: "AAPL"}
	if err := parseNasdaqFinancials(raw, out, "fixture://annual", 1); err != nil {
		t.Fatal(err)
	}
	if out.FiscalPeriod != "annual:2025-09-27" || out.AsOf != "2025-09-27" || out.GrossProfit != 195_201_000_000 {
		t.Fatalf("annual period or value mismatch: %+v", out)
	}
	if source := out.FieldSources["grossProfit"]; source.Frequency != "annual" || source.FiscalPeriod != out.FiscalPeriod || source.Unit != "USD" {
		t.Fatalf("annual field provenance mismatch: %+v", source)
	}
}
