package api

import "testing"

func TestIsCNMarket(t *testing.T) {
	cases := map[string]bool{
		"cn": true, "CN": true, " a ": true, "us": false, "": false, "hk": false,
	}
	for in, want := range cases {
		if got := isCNMarket(in); got != want {
			t.Fatalf("isCNMarket(%q)=%v want %v", in, got, want)
		}
	}
}

func TestParseAMarketStocks(t *testing.T) {
	raw := []byte(`{
	  "quotes": {
	    "601398": {"price": 7.27, "pct": 0.5, "vol": 1000, "mcapYi": 20000},
	    "300001": {"price": 32.92, "pct": -0.03, "vol": 221616, "mcapYi": 347.41},
	    "bad": {"price": 0, "pct": 1, "vol": 1, "mcapYi": 1}
	  },
	  "ts": 1783590554081,
	  "count": 3
	}`)
	out, bySym, src, err := parseAMarketStocks(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("len=%d", len(out))
	}
	if out[0].Symbol != "601398" {
		t.Fatalf("expected mcap sort first 601398, got %s", out[0].Symbol)
	}
	if _, ok := bySym["300001"]; !ok {
		t.Fatal("missing 300001")
	}
	if src != "a-market@1783590554081" {
		t.Fatalf("src=%s", src)
	}
}
