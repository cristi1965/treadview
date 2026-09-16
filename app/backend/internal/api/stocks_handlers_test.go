package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"trading-agents/internal/market"
)

func TestTrustedQuoteCoverageRequiresProviderAndObservationTime(t *testing.T) {
	at := time.Date(2026, 9, 9, 7, 0, 0, 0, time.UTC).Format(time.RFC3339)
	quotes := map[string]market.Quote{
		"AAPL": {Price: 230, Source: "tradingview", DataTime: at, TimeGranularity: "second"},
		"MSFT": {Price: 500, Source: "stale-snapshot:us", DataTime: at, TimeGranularity: "second"},
		"NVDA": {Price: 220, Source: "tradingview"},
	}
	if got := trustedQuoteCoverage(quotes); got != 1 {
		t.Fatalf("coverage=%d want 1", got)
	}
}

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

func TestNormalizeStocksMarket(t *testing.T) {
	cases := map[string]string{
		"":    "us",
		"us":  "us",
		"CN":  "cn",
		" a ": "cn",
	}
	for in, want := range cases {
		got, err := normalizeStocksMarket(in)
		if err != nil {
			t.Fatalf("normalizeStocksMarket(%q) unexpected error: %v", in, err)
		}
		if got != want {
			t.Fatalf("normalizeStocksMarket(%q)=%q want %q", in, got, want)
		}
	}
	if _, err := normalizeStocksMarket("hk"); err == nil {
		t.Fatal("normalizeStocksMarket(\"hk\") expected error")
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

func TestSearchStocksRejectsUnsupportedMarket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet, "/api/stocks/search?q=nvda&market=hk", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	(&Handler{}).SearchStocks(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want %d", w.Code, http.StatusBadRequest)
	}
}
