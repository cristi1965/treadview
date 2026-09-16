package api

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"trading-agents/internal/dataflows"

	"github.com/gin-gonic/gin"
)

const qdiiPremiumFreshFor = 72 * time.Hour
const qdiiPremiumCacheFor = 20 * time.Second

var qdiiPremiumCache struct {
	sync.Mutex
	items    []dataflows.QDIIPremium
	err      error
	loadedAt time.Time
}

func loadQDIIPremiums(now time.Time) ([]dataflows.QDIIPremium, error) {
	qdiiPremiumCache.Lock()
	defer qdiiPremiumCache.Unlock()
	if !qdiiPremiumCache.loadedAt.IsZero() && now.Sub(qdiiPremiumCache.loadedAt) < qdiiPremiumCacheFor {
		return append([]dataflows.QDIIPremium(nil), qdiiPremiumCache.items...), qdiiPremiumCache.err
	}
	items, err := dataflows.NewQDIIPremiumClient().GetPremiums()
	qdiiPremiumCache.items = append([]dataflows.QDIIPremium(nil), items...)
	qdiiPremiumCache.err = err
	qdiiPremiumCache.loadedAt = time.Now().UTC()
	return append([]dataflows.QDIIPremium(nil), items...), err
}

// GetQDIIPremiums returns observed QDII ETF premium/discount data with input freshness.
func GetQDIIPremiums(c *gin.Context) {
	items, err := loadQDIIPremiums(time.Now().UTC())
	if err != nil {
		staleDataError(c, http.StatusBadGateway, dataFreshnessMeta{
			Source:      "eastmoney:FundMNFInfo",
			Refreshable: true,
		}, err.Error())
		return
	}
	meta, status := qdiiPremiumFreshness(items, time.Now().UTC())
	setDataFreshness(c, meta)
	c.JSON(http.StatusOK, gin.H{
		"items": items, "count": len(items), "status": status,
	})
}

func qdiiPremiumFreshness(items []dataflows.QDIIPremium, now time.Time) (dataFreshnessMeta, string) {
	meta := dataFreshnessMeta{Source: dataflows.QDIIPremiumSource(items), Refreshable: true}
	dataTime, known := dataflows.OldestQDIIPremiumDataTime(items)
	if !known {
		meta.DataTime = "unknown"
		meta.Stale = true
		meta.StaleReason = "one or more QDII items have missing price, NAV, premium, NAV date, or price time"
		meta.PartialErrors = []string{"incomplete QDII critical inputs"}
		return meta, "unknown"
	}
	meta.DataTime = dataTime
	parsed, ok := parseLooseDataTime(dataTime)
	if !ok {
		meta.DataTime = "unknown"
		meta.Stale = true
		meta.StaleReason = "invalid QDII critical input time"
		return meta, "unknown"
	}
	age := now.Sub(parsed)
	if age > qdiiPremiumFreshFor {
		meta.Stale = true
		meta.StaleReason = "oldest QDII critical input is " + strings.TrimSpace(dataTime)
		return meta, "stale"
	}
	return meta, "live"
}
