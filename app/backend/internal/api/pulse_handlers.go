package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"trading-agents/internal/market"

	"github.com/gin-gonic/gin"
)

type pulseCompany struct {
	Ticker       string   `json:"ticker"`
	Name         string   `json:"name"`
	Layer        string   `json:"layer"`
	Segment      string   `json:"segment,omitempty"`
	Region       string   `json:"region"`
	MarketCapB   float64  `json:"marketCapB"`
	Heat         float64  `json:"heat"`
	Industries   []string `json:"industries"`
	Moat         *int     `json:"moat,omitempty"`
	Pos52        *float64 `json:"pos52,omitempty"`
	Pct          *float64 `json:"pct,omitempty"`
	LivePrice    *float64 `json:"livePrice,omitempty"`
	LiveMcapYi   *float64 `json:"liveMcapYi,omitempty"`
	ValuationPct *float64 `json:"valuationPct,omitempty"`
	Momentum20d  *float64 `json:"momentum20d,omitempty"`
	RSI          *float64 `json:"rsi,omitempty"`
	Sentiment    *float64 `json:"sentiment,omitempty"`
	DataSource   string   `json:"dataSource,omitempty"`
	LiveBar      string   `json:"liveBar,omitempty"`
	QuoteSource  string   `json:"quoteSource,omitempty"`
}

type pulseFile struct {
	GeneratedAt   string         `json:"generated_at"`
	Source        string         `json:"source"`
	SourceLiveBar string         `json:"source_live_bar,omitempty"`
	Count         int            `json:"count"`
	Companies     []pulseCompany `json:"companies"`
}

var (
	pulseMu       sync.RWMutex
	pulseData     pulseFile
	pulseLoadedAt time.Time
	pulsePath     string
)

const pulseCompaniesMaxAge = 24 * time.Hour

func pulseFilePaths() []string {
	return []string{
		filepath.Join("data", "pulse-companies.json"),
		filepath.Join("app", "backend", "data", "pulse-companies.json"),
		filepath.Join("..", "frontend", "public", "data", "pulse-companies.json"),
		filepath.Join("..", "frontend", "dist", "data", "pulse-companies.json"),
	}
}

func loadPulseFile() (pulseFile, error) {
	pulseMu.RLock()
	if len(pulseData.Companies) > 0 && time.Since(pulseLoadedAt) < 30*time.Second {
		defer pulseMu.RUnlock()
		return pulseData, nil
	}
	pulseMu.RUnlock()

	pulseMu.Lock()
	defer pulseMu.Unlock()
	if len(pulseData.Companies) > 0 && time.Since(pulseLoadedAt) < 30*time.Second {
		return pulseData, nil
	}

	bestPath, raw, info, err := newestPulseFile()
	if err != nil {
		return pulseFile{}, err
	}

	var f pulseFile
	if json.Unmarshal(raw, &f) != nil || len(f.Companies) == 0 {
		return pulseFile{}, os.ErrNotExist
	}
	if err := pulseFileFreshness(f, info); err != nil {
		return f, err
	}
	pulseData = f
	pulseLoadedAt = time.Now()
	pulsePath = bestPath
	return pulseData, nil
}

// GetPulseData GET /api/pulse — heatmap universe with live quote overlay.
// ?region=US|CN|ALL  optional filter; default ALL.
func GetPulseData(c *gin.Context) {
	f, err := loadPulseFile()
	historical := err != nil
	if historical || len(f.Companies) == 0 {
		reason := "pulse companies unavailable"
		if err != nil {
			reason = err.Error()
		}
		meta := dataFreshnessMeta{
			Source:      realSnapshotSource(firstNonEmpty(f.Source, "pulse")),
			DataTime:    f.GeneratedAt,
			Refreshable: false,
		}
		mode := strings.ToLower(strings.TrimSpace(c.Query("mode")))
		if len(f.Companies) == 0 || (mode != "best" && mode != "historical") {
			staleDataError(c, http.StatusServiceUnavailable, meta, reason)
			return
		}
	}

	region := strings.ToUpper(strings.TrimSpace(c.Query("region")))
	if region == "" {
		region = "ALL"
	}

	p := market.Default()

	type liveQ struct {
		Price  float64
		Pct    float64
		McapB  float64
		McapYi float64
	}

	needUS := !historical && region != "CN"
	needCN := !historical && (region == "ALL" || region == "CN")

	usQuotes := map[string]liveQ{}
	if needUS {
		for sym, q := range p.USQuotesCopy() {
			if q.Price <= 0 {
				continue
			}
			usQuotes[strings.ToUpper(sym)] = liveQ{Price: q.Price, Pct: q.Pct, McapB: q.McapB}
		}
	}

	cnQuotes := map[string]liveQ{}
	if needCN {
		for sym, q := range p.CNQuotesCopy() {
			if q.Price <= 0 {
				continue
			}
			cnQuotes[sym] = liveQ{Price: q.Price, Pct: q.Pct, McapYi: q.McapYi}
		}
	}

	out := make([]pulseCompany, 0, len(f.Companies))
	liveN := 0
	for _, row := range f.Companies {
		if region != "ALL" && !strings.EqualFold(row.Region, region) {
			continue
		}
		cp := row
		switch strings.ToUpper(cp.Region) {
		case "CN":
			if q, ok := cnQuotes[cp.Ticker]; ok && q.Price > 0 {
				price, pct := q.Price, q.Pct
				cp.LivePrice = &price
				cp.Pct = &pct
				if q.McapYi > 0 {
					yi := q.McapYi
					cp.LiveMcapYi = &yi
					cp.MarketCapB = q.McapYi / 10 // 亿元≈0.1B USD rough for bubble size
				}
				cp.QuoteSource = "live-cn"
				liveN++
			}
		default:
			if q, ok := usQuotes[strings.ToUpper(cp.Ticker)]; ok && q.Price > 0 {
				price, pct := q.Price, q.Pct
				cp.LivePrice = &price
				cp.Pct = &pct
				if q.McapB > 0 {
					cp.MarketCapB = q.McapB
				}
				cp.QuoteSource = "live-us"
				liveN++
			}
		}
		out = append(out, cp)
	}

	freshness := pulseFreshness(f, p, needUS, needCN)
	dataMode := "live"
	degradedReason := ""
	if historical {
		dataMode = "historical"
		degradedReason = err.Error()
		freshness = dataFreshnessMeta{
			Source:      realSnapshotSource(firstNonEmpty(f.Source, "pulse")),
			DataTime:    f.GeneratedAt,
			RefreshedAt: f.GeneratedAt,
			Stale:       true,
			StaleReason: "historical pulse snapshot; not for current decisions",
			Refreshable: false,
		}
	}
	setDataFreshness(c, freshness)
	c.JSON(http.StatusOK, gin.H{
		"generated_at":    f.GeneratedAt,
		"source":          f.Source,
		"source_live_bar": f.SourceLiveBar,
		"refreshed_at":    freshness.RefreshedAt,
		"count":           len(out),
		"live_overlay":    liveN,
		"region":          region,
		"dataMode":        dataMode,
		"degradedReason":  degradedReason,
		"companies":       out,
	})
}

func pulseFreshness(f pulseFile, provider *market.Provider, needUS, needCN bool) dataFreshnessMeta {
	meta := dataFreshnessMeta{
		Source:      "pulse-snapshot",
		DataTime:    f.GeneratedAt,
		RefreshedAt: f.GeneratedAt,
		Refreshable: true,
	}
	oldest, ok := parseLooseDataTime(f.GeneratedAt)
	if !ok {
		meta.DataTime = "unknown"
		meta.Stale = true
		meta.StaleReason = "pulse generated_at is unknown"
		return meta
	}
	statuses := make([]market.PayloadStatus, 0, 2)
	if needUS {
		statuses = append(statuses, provider.USPayloadStatus(time.Now()))
	}
	if needCN {
		statuses = append(statuses, provider.CNPayloadStatus(time.Now()))
	}
	sources := []string{"pulse-snapshot"}
	for _, status := range statuses {
		sources = append(sources, status.Source)
		if status.DataTime.IsZero() {
			meta.DataTime = "unknown"
			meta.Stale = true
			meta.StaleReason = "quote data time is unknown"
			continue
		}
		if status.DataTime.Before(oldest) {
			oldest = status.DataTime
		}
		if status.Stale {
			meta.Stale = true
			meta.StaleReason = "one or more quote inputs are stale"
		}
	}
	meta.Source = strings.Join(sources, "+")
	if meta.DataTime != "unknown" {
		meta.DataTime = oldest.UTC().Format(time.RFC3339)
	}
	if stale, reason := staleIfOlder(f.GeneratedAt, pulseCompaniesMaxAge); stale {
		meta.Stale = true
		meta.StaleReason = "pulse: " + reason
	}
	return meta
}

func newestPulseFile() (string, []byte, os.FileInfo, error) {
	var (
		bestPath string
		bestRaw  []byte
		bestInfo os.FileInfo
	)
	for _, p := range pulseFilePaths() {
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			continue
		}
		raw, err := os.ReadFile(p)
		if err != nil || len(raw) == 0 {
			continue
		}
		if bestInfo == nil || info.ModTime().After(bestInfo.ModTime()) {
			bestPath = p
			bestRaw = raw
			bestInfo = info
		}
	}
	if bestPath == "" {
		return "", nil, nil, os.ErrNotExist
	}
	return bestPath, bestRaw, bestInfo, nil
}

func pulseFileFreshness(f pulseFile, info os.FileInfo) error {
	var ts time.Time
	if strings.TrimSpace(f.GeneratedAt) != "" {
		parsed, err := time.Parse(time.RFC3339, f.GeneratedAt)
		if err != nil {
			return fmt.Errorf("pulse companies generated_at invalid: %s", f.GeneratedAt)
		}
		ts = parsed
	} else if info != nil {
		ts = info.ModTime()
	}
	if ts.IsZero() {
		return fmt.Errorf("pulse companies freshness missing")
	}
	if age := time.Since(ts); age > pulseCompaniesMaxAge {
		return fmt.Errorf("pulse companies stale: updated=%s age=%.1fh max=%.0fh", ts.UTC().Format(time.RFC3339), age.Hours(), pulseCompaniesMaxAge.Hours())
	}
	return nil
}
