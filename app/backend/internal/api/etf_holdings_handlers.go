package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

type etfHoldingRow struct {
	Symbol string  `json:"symbol"`
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

type etfHoldingsEntry struct {
	Name     string          `json:"name"`
	Venue    string          `json:"venue"`
	Holdings []etfHoldingRow `json:"holdings"`
}

type etfHoldingsFile struct {
	Updated string                      `json:"updated"`
	Note    string                      `json:"note"`
	ETFs    map[string]etfHoldingsEntry `json:"etfs"`
}

var (
	etfHoldingsOnce sync.Once
	etfHoldingsData etfHoldingsFile
)

func loadETFHoldings() etfHoldingsFile {
	etfHoldingsOnce.Do(func() {
		paths := []string{
			filepath.Join("data", "etf-holdings.json"),
			filepath.Join("app", "backend", "data", "etf-holdings.json"),
			filepath.Join("..", "frontend", "public", "data", "etf-holdings.json"),
		}
		for _, p := range paths {
			raw, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			var f etfHoldingsFile
			if json.Unmarshal(raw, &f) == nil && len(f.ETFs) > 0 {
				etfHoldingsData = f
				return
			}
		}
		etfHoldingsData = etfHoldingsFile{ETFs: map[string]etfHoldingsEntry{}}
	})
	return etfHoldingsData
}

func etfHoldingsFreshness(data etfHoldingsFile) (dataFreshnessMeta, string) {
	stale, reason := staleIfOlder(data.Updated, etfAnalysesMaxAge)
	mode := "current"
	if stale {
		mode = "historical"
	}
	return dataFreshnessMeta{
		Source: "etf-holdings-seed", DataTime: data.Updated, Stale: stale,
		StaleReason: reason, Refreshable: false,
	}, mode
}

// GetETFHoldings GET /api/etf/:sym/holdings
func GetETFHoldings(c *gin.Context) {
	sym := strings.ToUpper(strings.TrimSpace(c.Param("sym")))
	data := loadETFHoldings()
	entry, ok := data.ETFs[sym]
	if !ok {
		// try as-is for CN numeric codes
		entry, ok = data.ETFs[c.Param("sym")]
		sym = c.Param("sym")
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "holdings not found", "sym": sym})
		return
	}
	meta, mode := etfHoldingsFreshness(data)
	setDataFreshness(c, meta)
	c.JSON(http.StatusOK, gin.H{
		"sym":            sym,
		"name":           entry.Name,
		"venue":          entry.Venue,
		"holdings":       entry.Holdings,
		"updated":        data.Updated,
		"dataMode":       mode,
		"degradedReason": meta.StaleReason,
	})
}

// CompareETFs GET /api/etf/compare?syms=SPY,QQQ
func CompareETFs(c *gin.Context) {
	raw := strings.TrimSpace(c.Query("syms"))
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "syms required"})
		return
	}
	parts := strings.Split(raw, ",")
	data := loadETFHoldings()
	type side struct {
		Sym      string          `json:"sym"`
		Name     string          `json:"name"`
		Venue    string          `json:"venue"`
		Holdings []etfHoldingRow `json:"holdings"`
	}
	sides := make([]side, 0, len(parts))
	overlap := map[string]int{}
	for _, p := range parts {
		sym := strings.ToUpper(strings.TrimSpace(p))
		entry, ok := data.ETFs[sym]
		if !ok {
			entry, ok = data.ETFs[strings.TrimSpace(p)]
			sym = strings.TrimSpace(p)
		}
		if !ok {
			continue
		}
		sides = append(sides, side{Sym: sym, Name: entry.Name, Venue: entry.Venue, Holdings: entry.Holdings})
		for _, h := range entry.Holdings {
			overlap[strings.ToUpper(h.Symbol)]++
		}
	}
	shared := make([]string, 0)
	for sym, n := range overlap {
		if n >= 2 {
			shared = append(shared, sym)
		}
	}
	sort.Strings(shared)
	meta, mode := etfHoldingsFreshness(data)
	setDataFreshness(c, meta)
	c.JSON(http.StatusOK, gin.H{
		"etfs":           sides,
		"shared":         shared,
		"count":          len(sides),
		"updated":        data.Updated,
		"dataMode":       mode,
		"degradedReason": meta.StaleReason,
	})
}

// GetETFOwners GET /api/etf/owners?symbol=NVDA (also accepts ?sym=)
func GetETFOwners(c *gin.Context) {
	symbol := strings.ToUpper(strings.TrimSpace(c.Query("symbol")))
	if symbol == "" {
		symbol = strings.ToUpper(strings.TrimSpace(c.Query("sym")))
	}
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol required"})
		return
	}
	data := loadETFHoldings()
	type owner struct {
		ETF    string  `json:"etf"`
		Name   string  `json:"name"`
		Venue  string  `json:"venue"`
		Weight float64 `json:"weight"`
	}
	owners := make([]owner, 0)
	for etf, entry := range data.ETFs {
		for _, h := range entry.Holdings {
			if strings.EqualFold(h.Symbol, symbol) {
				owners = append(owners, owner{ETF: etf, Name: entry.Name, Venue: entry.Venue, Weight: h.Weight})
			}
		}
	}
	sort.Slice(owners, func(i, j int) bool { return owners[i].Weight > owners[j].Weight })
	meta, mode := etfHoldingsFreshness(data)
	setDataFreshness(c, meta)
	c.JSON(http.StatusOK, gin.H{
		"symbol": symbol, "owners": owners, "count": len(owners),
		"updated": data.Updated, "dataMode": mode, "degradedReason": meta.StaleReason,
	})
}
