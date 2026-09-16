package dataflows

import "testing"

func TestUSInstrumentMasterRequiresVersionAndExactSymbol(t *testing.T) {
	master, err := parseUSInstrumentMaster([]byte(`{"generated_at":"2026-09-06 22:56 ET","count":2,"stocks":[{"sym":"AAPL"},{"sym":"MSFT"}]}`), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if !master.ContainsStock("aapl") || master.ContainsStock("UNKNOWN") || master.Market != "US" || master.AssetClass != "stocks" || master.Currency != "USD" || master.CurrencySource() != "instrument-master:stockgod-us-symbol-universe-v1@2026-09-06 22:56 ET" {
		t.Fatalf("invalid master contract: %+v", master)
	}
	if _, err := parseUSInstrumentMaster([]byte(`{"count":1,"stocks":[{"sym":"AAPL"}]}`), "fixture"); err == nil {
		t.Fatal("missing data version must fail closed")
	}
}
