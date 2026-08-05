package market

import (
	"testing"
)

func TestParseUsStocksQuotes(t *testing.T) {
	raw := []byte(`{
	  "generated_at": "2026-07-31 03:31 ET",
	  "count": 2,
	  "stocks": [
	    {"sym":"RKLB","name":"Rocket Lab","price":64.68,"pct":10.38,"mcapB":38.69,"vol":18162886},
	    {"sym":"bad","name":"x","price":0,"pct":1,"mcapB":1,"vol":1},
	    {"sym":"nvda","name":"NVIDIA","price":195.04,"pct":2.65,"mcapB":4719.97,"vol":129009718}
	  ]
	}`)

	quotes, generatedAt, err := parseUsStocksQuotes(raw)
	if err != nil {
		t.Fatalf("parseUsStocksQuotes: %v", err)
	}
	if generatedAt != "2026-07-31 03:31 ET" {
		t.Fatalf("generatedAt=%q", generatedAt)
	}
	if len(quotes) != 2 {
		t.Fatalf("len(quotes)=%d want 2", len(quotes))
	}
	rklb, ok := quotes["RKLB"]
	if !ok {
		t.Fatal("missing RKLB")
	}
	if rklb.Price != 64.68 || rklb.Pct != 10.38 {
		t.Fatalf("RKLB=%+v", rklb)
	}
	if _, ok := quotes["NVDA"]; !ok {
		t.Fatal("missing NVDA (should be uppercased)")
	}
}

func TestParseUsStocksQuotesEmpty(t *testing.T) {
	_, _, err := parseUsStocksQuotes([]byte(`{"stocks":[]}`))
	if err == nil {
		t.Fatal("expected error for empty stocks")
	}
}
