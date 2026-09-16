package api

import (
	"net/http"
	"time"

	"trading-agents/internal/dataflows"

	"github.com/gin-gonic/gin"
)

var fetchMarketOverview = dataflows.FetchComprehensiveMarketOverview

// GetMarketOverview returns comprehensive multi-market indices, fear & greed sentiment gauge, US overnight mapping, and 3-tier forward layout plan
func GetMarketOverview(c *gin.Context) {
	// The overview fans out to multiple public providers. Bound the whole
	// request so one blocked provider cannot keep the first screen pending.
	done := make(chan dataflows.ComprehensiveMarketOverview, 1)
	go func() { done <- fetchMarketOverview() }()
	var data dataflows.ComprehensiveMarketOverview
	select {
	case data = <-done:
	case <-time.After(5 * time.Second):
		staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{
			Source:      "market-overview:provider-observations",
			Refreshable: true,
		}, "market overview live sources timed out")
		return
	}

	available := 0
	for _, item := range data.CNIndices {
		if item.Available {
			available++
		}
	}
	for _, item := range data.USIndices {
		if item.Available {
			available++
		}
	}
	for _, item := range data.GlobalIndices {
		if item.Available {
			available++
		}
	}
	if available == 0 || data.UpdatedAt == "" {
		staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{
			Source:      "market-overview:provider-observations",
			Refreshable: true,
		}, "market overview returned no fully attributable index data")
		return
	}
	observedAt, err := time.Parse(time.RFC3339, data.UpdatedAt)
	if err != nil || observedAt.After(time.Now().UTC().Add(5*time.Minute)) || time.Since(observedAt) > 72*time.Hour {
		staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{
			Source: "market-overview:provider-observations", DataTime: data.UpdatedAt, Refreshable: true,
		}, "market overview oldest provider observation is outside the freshness window")
		return
	}
	setDataFreshness(c, dataFreshnessMeta{
		Source:      "market-overview:provider-observations",
		DataTime:    data.UpdatedAt,
		Refreshable: true,
	})
	c.JSON(http.StatusOK, data)
}
