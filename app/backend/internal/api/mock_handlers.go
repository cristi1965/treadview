package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"trading-agents/internal/market"

	"github.com/gin-gonic/gin"
)

type MarketQuote struct {
	Price float64 `json:"price"`
	Pct   float64 `json:"pct"`
	Vol   float64 `json:"vol"`
	McapB float64 `json:"mcapB,omitempty"`
}

type AMarketQuote struct {
	Price  float64 `json:"price"`
	Pct    float64 `json:"pct"`
	Vol    float64 `json:"vol"`
	McapYi float64 `json:"mcapYi,omitempty"`
}

type QuoteResponseItem struct {
	Price     float64 `json:"price"`
	Pct       float64 `json:"pct"`
	Session   string  `json:"session"`
	PrevClose float64 `json:"prevClose"`
	Source    string  `json:"source,omitempty"`
}

// GetPremarketMovers handles GET /api/premarket-movers (local snapshot; rebuilt from us-stocks).
func GetPremarketMovers(c *gin.Context) {
	data, err := os.ReadFile(filepath.Join("data", "premarket-movers.json"))
	if err != nil {
		data, err = os.ReadFile(filepath.Join("app", "backend", "data", "premarket-movers.json"))
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read premarket-movers.json"})
		return
	}
	c.Header("X-Data-Source", "us-stocks-movers")
	c.Data(http.StatusOK, "application/json", data)
}

// GetMacroData handles GET /api/macro (local snapshot refreshed from Yahoo).
func GetMacroData(c *gin.Context) {
	data, err := os.ReadFile(filepath.Join("data", "macro.json"))
	if err != nil {
		data, err = os.ReadFile(filepath.Join("app", "backend", "data", "macro.json"))
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read macro.json"})
		return
	}
	c.Header("X-Data-Source", "yahoo-macro-snapshot")
	c.Data(http.StatusOK, "application/json", data)
}

// GetMarketData：GET /api/market — 美股全市场 quotes（优先 us-stocks.json）。
func GetMarketData(c *gin.Context) {
	data, src, err := market.SnapshotPayload("us")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read market.json"})
		return
	}
	c.Header("X-Data-Source", src)
	c.Data(http.StatusOK, "application/json", data)
}

// GetAMarketData：GET /api/a-market — A 股快照。
func GetAMarketData(c *gin.Context) {
	data, src, err := market.SnapshotPayload("cn")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read a-market.json"})
		return
	}
	c.Header("X-Data-Source", src)
	c.Data(http.StatusOK, "application/json", data)
}

// GetQuoteData：GET /api/quote?syms=A,B — 实时价（TV→Yahoo→快照）。
func GetQuoteData(c *gin.Context) {
	symsQuery := c.Query("syms")
	if symsQuery == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "syms parameter is required"})
		return
	}

	syms := strings.Split(symsQuery, ",")
	live := market.Default().Quotes(syms)

	results := make(map[string]QuoteResponseItem, len(live))
	sources := map[string]int{}
	for sym, q := range live {
		results[sym] = QuoteResponseItem{
			Price:     q.Price,
			Pct:       q.Pct,
			Session:   q.Session,
			PrevClose: q.PrevClose,
			Source:    q.Source,
		}
		sources[q.Source]++
	}

	c.Header("X-Data-Source", "self-provider")
	c.JSON(http.StatusOK, gin.H{
		"quotes":  results,
		"ts":      time.Now().UnixNano() / int64(time.Millisecond),
		"sources": sources,
		"missing": missingSymbols(syms, results),
	})
}

func missingSymbols(requested []string, found map[string]QuoteResponseItem) []string {
	var miss []string
	for _, s := range requested {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := found[s]; !ok {
			miss = append(miss, s)
		}
	}
	return miss
}
