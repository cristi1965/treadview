package dataflows

import "testing"

func TestParseNasdaqQuoteRequiresSourceMetadata(t *testing.T) {
	raw := []byte(`{"data":{"symbol":"AAPL","marketStatus":"Closed","primaryData":{"lastSalePrice":"$234.56","netChange":"1.25","percentageChange":"0.54%","lastTradeTimestamp":"Sep 3, 2026 4:00 PM ET","currency":null,"isRealTime":false}},"status":{"rCode":200}}`)
	quote, err := parseNasdaqQuote(raw, "AAPL", "https://api.nasdaq.com/api/quote/AAPL/info?assetclass=stocks")
	if err != nil {
		t.Fatal(err)
	}
	master, err := parseUSInstrumentMaster([]byte(`{"generated_at":"2026-09-06T20:00:00Z","count":1,"stocks":[{"sym":"AAPL"}]}`), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if quote.Currency != "" || !master.ContainsStock(quote.Symbol) {
		t.Fatalf("Nasdaq null currency must remain separate from master: %+v", quote)
	}
	quote.Currency = "USD"
	quote.CurrencySource = master.CurrencySource()
	if quote.Currency != "USD" || quote.CurrencySource == "" || quote.MarketStatus != "closed" || quote.TradeTime.IsZero() {
		t.Fatalf("missing parsed provenance: %+v", quote)
	}
	if quote.TradeTime.Year() != 2026 || quote.TradeTime.Month() != 9 || quote.TradeTime.Day() != 3 {
		t.Fatalf("trade time=%s", quote.TradeTime)
	}
}

func TestParseNasdaqQuotePreservesDateOnlyGranularity(t *testing.T) {
	raw := []byte(`{"data":{"symbol":"AAPL","marketStatus":"Closed","primaryData":{"lastSalePrice":"$234.56","lastTradeTimestamp":"Sep 3, 2026","currency":"USD","isRealTime":false}},"status":{"rCode":200}}`)
	quote, err := parseNasdaqQuote(raw, "AAPL", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if quote.TimeGranularity != "date" || quote.TradeTime.Format("2006-01-02") != "2026-09-03" {
		t.Fatalf("date-only source gained false precision: %+v", quote)
	}
}

func TestParseNasdaqQuoteFailsClosedWithoutStatusOrTradeTime(t *testing.T) {
	for _, raw := range []string{
		`{"data":{"primaryData":{"lastSalePrice":"234.56","lastTradeTimestamp":"Sep 3, 2026"}},"status":{"rCode":200}}`,
		`{"data":{"marketStatus":"Closed","primaryData":{"lastSalePrice":"$234.56"}},"status":{"rCode":200}}`,
	} {
		if _, err := parseNasdaqQuote([]byte(raw), "AAPL", "fixture"); err == nil {
			t.Fatalf("expected fail-closed parse for %s", raw)
		}
	}
}

func TestParseNasdaqQuoteRejectsSymbolMismatch(t *testing.T) {
	raw := []byte(`{"data":{"symbol":"MSFT","marketStatus":"Closed","primaryData":{"lastSalePrice":"$234.56","lastTradeTimestamp":"Sep 3, 2026 4:00 PM ET"}},"status":{"rCode":200}}`)
	if _, err := parseNasdaqQuote(raw, "AAPL", "fixture"); err == nil {
		t.Fatal("mismatched Nasdaq response symbol must fail closed")
	}
}
