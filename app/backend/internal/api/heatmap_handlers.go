package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/models"
)

// HeatmapNode represents a node in the heatmap
type HeatmapNode struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	ChangePercent float64 `json:"changePercent"`
	MarketCap     int64   `json:"marketCap"`
	Sector        string  `json:"sector"`
	AvgScore      float64 `json:"avgScore"`
}

// GetHeatmapData returns all stocks for the heatmap visualization
func GetHeatmapData(c *gin.Context) {
	stocks, source, err := loadUniverseStocks("us")
	if err != nil {
		staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{
			Source:      source,
			Refreshable: true,
		}, err.Error())
		return
	}
	stocks, quoteResult := enrichHeatmapQuotes(stocks)
	nodes := make([]HeatmapNode, 0, len(stocks))
	for _, stock := range stocks {
		if stock.Price <= 0 || stock.MarketCap <= 0 {
			continue
		}
		nodes = append(nodes, HeatmapNode{
			Symbol:        stock.Symbol,
			Name:          stock.Name,
			Price:         stock.Price,
			ChangePercent: stock.ChangePercent,
			MarketCap:     int64(stock.MarketCap),
			Sector:        stock.Sector,
			AvgScore:      stock.AvgScore,
		})
	}
	if len(nodes) == 0 {
		staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{
			Source:      source,
			Refreshable: true,
		}, "heatmap universe has no priced nodes")
		return
	}

	setDataFreshness(c, heatmapFreshness(source, len(nodes), quoteResult, time.Now()))
	c.JSON(http.StatusOK, gin.H{
		"nodes": nodes,
		"total": len(nodes),
	})
}

func enrichHeatmapQuotes(stocks []models.Stock) ([]models.Stock, quoteFetchResult) {
	symbols := make([]string, 0, len(stocks))
	for _, stock := range stocks {
		symbols = append(symbols, stock.Symbol)
	}
	result := boundedQuotesForRequest(symbols)
	out := make([]models.Stock, len(stocks))
	copy(out, stocks)
	for index := range out {
		quote, ok := result.quotes[out[index].Symbol]
		if !ok || quote.Price <= 0 {
			continue
		}
		if out[index].Price > 0 && out[index].MarketCap > 0 {
			out[index].MarketCap *= quote.Price / out[index].Price
		}
		out[index].Price = quote.Price
		out[index].ChangePercent = quote.Pct
		out[index].Change = quote.Price * quote.Pct / 100
	}
	return out, result
}

func heatmapFreshness(universeSource string, nodeCount int, quotes quoteFetchResult, now time.Time) dataFreshnessMeta {
	meta := dataFreshnessMeta{Source: universeSource + "+" + quotes.source, DataTime: "unknown", Refreshable: true}
	oldest, universeOK := parseLooseDataTime(dataTimeFromSource(universeSource))
	if universeOK {
		meta.DataTime = oldest.UTC().Format(time.RFC3339)
	}
	if !quotes.dataTime.IsZero() && (!universeOK || quotes.dataTime.Before(oldest)) {
		oldest = quotes.dataTime
		meta.DataTime = oldest.UTC().Format(time.RFC3339)
	}
	trusted := trustedQuoteCoverage(quotes.quotes)
	if trusted < nodeCount {
		meta.Stale = true
		meta.StaleReason = fmt.Sprintf("live quote coverage is incomplete (%d/%d); remaining values use the universe snapshot", trusted, nodeCount)
	}
	if strings.Contains(strings.ToLower(universeSource), "stale-snapshot") {
		meta.Stale = true
		meta.StaleReason = "heatmap universe is a stale snapshot"
	}
	if !universeOK {
		meta.Stale = true
		meta.StaleReason = "heatmap universe observation time unavailable"
	} else if old, reason := staleIfOlderAt(meta.DataTime, 24*time.Hour, now); old {
		meta.Stale = true
		meta.StaleReason = reason
	}
	if quoteStale, reason := quoteResultFreshness(quotes, now); quoteStale {
		meta.Stale = true
		if meta.StaleReason != "" {
			meta.StaleReason += "; "
		}
		meta.StaleReason += reason
	}
	return meta
}
