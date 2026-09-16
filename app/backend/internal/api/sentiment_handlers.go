package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"trading-agents/internal/dataflows"
	"trading-agents/internal/market"
)

type sentimentGauge struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Value      float64 `json:"value"`
	Pct        float64 `json:"pct"`
	Unit       string  `json:"unit"`
	Zone       string  `json:"zone"`
	Tip        string  `json:"tip"`
	Thresholds []int   `json:"thresholds,omitempty"`
	Extra      string  `json:"extra,omitempty"`
	AsOf       string  `json:"asOf,omitempty"`
}

type sentimentPanel struct {
	Label  string           `json:"label"`
	Gauges []sentimentGauge `json:"gauges"`
}

type sentimentSnapshot struct {
	Ts string         `json:"ts"`
	US sentimentPanel `json:"us"`
	CN sentimentPanel `json:"cn"`
}

var (
	sentimentMu    sync.RWMutex
	sentimentCache *sentimentSnapshot
	sentimentAt    time.Time
)

func GetSentiment(c *gin.Context) {
	sentimentMu.RLock()
	if sentimentCache != nil && time.Since(sentimentAt) < 30*time.Second {
		defer sentimentMu.RUnlock()
		cachedDataTime, cachedOK := sentimentSnapshotDataTime(sentimentCache)
		if !cachedOK {
			staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{Source: "sentiment-snapshot+live-overlay", DataTime: "unknown", Refreshable: true}, "sentiment observation time unavailable")
			return
		}
		cachedStale, cachedReason := staleIfOlder(cachedDataTime.Format(time.RFC3339), 24*time.Hour)
		setDataFreshness(c, dataFreshnessMeta{
			Source:      "sentiment-snapshot+live-overlay",
			DataTime:    cachedDataTime.Format(time.RFC3339),
			Stale:       cachedStale,
			StaleReason: cachedReason,
			Refreshable: true,
		})
		c.JSON(http.StatusOK, sentimentCache)
		return
	}
	sentimentMu.RUnlock()

	snap := loadSentimentFromFile()
	if snap == nil {
		staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{
			Source:      "missing",
			Refreshable: true,
		}, "sentiment data not available")
		return
	}
	overlayMacroLive(snap)
	vix, vixErr := dataflows.LatestVIX()
	if vixErr == nil {
		for i := range snap.US.Gauges {
			if snap.US.Gauges[i].ID == "vix" {
				snap.US.Gauges[i].Value = vix.Close
				snap.US.Gauges[i].AsOf = vix.Date.Format("2006-01-02")
				if vix.Prev > 0 {
					snap.US.Gauges[i].Pct = (vix.Close - vix.Prev) / vix.Prev * 100
				}
			}
		}
	}
	fredValues := make(chan struct {
		id    string
		value dataflows.FREDValue
	}, 2)
	for _, item := range []struct{ series, id string }{{"DGS10", "tnx"}, {"DGS2", "t2y"}} {
		go func(series, id string) {
			value, err := dataflows.LatestFREDValue(series)
			if err != nil {
				fredValues <- struct {
					id    string
					value dataflows.FREDValue
				}{id: id}
				return
			}
			fredValues <- struct {
				id    string
				value dataflows.FREDValue
			}{id: id, value: value}
		}(item.series, item.id)
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for range 2 {
		select {
		case item := <-fredValues:
			if item.value.Date.IsZero() {
				continue
			}
			for i := range snap.US.Gauges {
				if snap.US.Gauges[i].ID == item.id {
					snap.US.Gauges[i].Value = item.value.Value
					snap.US.Gauges[i].AsOf = item.value.Date.Format("2006-01-02")
				}
			}
		case <-timer.C:
			returnSentiment(c, snap, vixErr)
			return
		}
	}
	returnSentiment(c, snap, vixErr)
}

func returnSentiment(c *gin.Context, snap *sentimentSnapshot, vixErr error) {
	dataTime, dataTimeOK := sentimentSnapshotDataTime(snap)
	if !dataTimeOK {
		staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{Source: realSnapshotSource("sentiment"), DataTime: "unknown", Refreshable: true}, "sentiment observation time unavailable")
		return
	}
	snapshotStale, reason := staleIfOlder(dataTime.Format(time.RFC3339), 24*time.Hour)
	if snapshotStale && vixErr != nil {
		staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{
			Source:      realSnapshotSource("sentiment"),
			DataTime:    dataTime.Format(time.RFC3339),
			StaleReason: reason,
			Refreshable: true,
		}, "sentiment snapshot is too old and live Cboe VIX is unavailable")
		return
	}

	sentimentMu.Lock()
	sentimentCache = snap
	sentimentAt = time.Now()
	sentimentMu.Unlock()

	setDataFreshness(c, dataFreshnessMeta{
		Source: func() string {
			if vixErr == nil {
				return "cboe-vix+sentiment-snapshot"
			}
			return "sentiment-snapshot+live-overlay"
		}(),
		DataTime: dataTime.Format(time.RFC3339),
		Stale:    snapshotStale,
		StaleReason: func() string {
			if snapshotStale && vixErr == nil {
				return "VIX refreshed from Cboe; remaining sentiment gauges use an older snapshot"
			}
			if vixErr != nil {
				return "only snapshot/live overlay available; Cboe VIX refresh failed"
			}
			return reason
		}(),
		Refreshable: true,
	})
	c.JSON(http.StatusOK, snap)
}

func sentimentSnapshotDataTime(snap *sentimentSnapshot) (time.Time, bool) {
	if snap == nil {
		return time.Time{}, false
	}
	oldest, ok := parseLooseDataTime(snap.Ts)
	if !ok {
		return time.Time{}, false
	}
	for _, panel := range []sentimentPanel{snap.US, snap.CN} {
		for _, gauge := range panel.Gauges {
			if strings.TrimSpace(gauge.AsOf) == "" {
				continue
			}
			if observedAt, parsed := parseLooseDataTime(gauge.AsOf); parsed && observedAt.Before(oldest) {
				oldest = observedAt
			}
		}
	}
	return oldest.UTC(), true
}

func loadSentimentFromFile() *sentimentSnapshot {
	for _, p := range []string{
		filepath.Join("data", "sentiment-snapshot.json"),
		filepath.Join("app", "backend", "data", "sentiment-snapshot.json"),
	} {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var snap sentimentSnapshot
		if err := json.Unmarshal(raw, &snap); err != nil {
			continue
		}
		return &snap
	}
	return nil
}

func overlayMacroLive(snap *sentimentSnapshot) {
	p := market.Default()
	if p == nil {
		return
	}
	quotes := p.USQuotesCopy()
	if len(quotes) == 0 {
		return
	}

	symMap := map[string]string{
		"^VIX":     "vix",
		"^TNX":     "tnx",
		"DX-Y.NYB": "dxy",
		"GC=F":     "gold",
	}
	for sym, id := range symMap {
		q, ok := quotes[sym]
		if !ok || q.Price == 0 {
			continue
		}
		for i := range snap.US.Gauges {
			if snap.US.Gauges[i].ID == id {
				snap.US.Gauges[i].Value = q.Price
				snap.US.Gauges[i].Pct = q.Pct
				snap.US.Gauges[i].Zone = autoZone(id, q.Price)
			}
		}
	}
}

func autoZone(id string, v float64) string {
	switch id {
	case "vix":
		if v < 15 {
			return "complacent"
		} else if v < 20 {
			return "neutral"
		} else if v < 30 {
			return "elevated"
		}
		return "panic"
	case "tnx":
		if v > 5.0 {
			return "restrictive"
		} else if v > 4.5 {
			return "elevated"
		}
		return "neutral"
	case "dxy":
		if v > 105 {
			return "strong"
		} else if v < 100 {
			return "weak"
		}
		return "neutral"
	default:
		return "neutral"
	}
}
