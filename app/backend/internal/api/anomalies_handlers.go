package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

const marketAnomaliesMaxAge = 24 * time.Hour

// HandleMarketAnomalies returns the latest market anomalies, options IV, dark pool flow, and media transcripts.
func HandleMarketAnomalies(c *gin.Context) {
	data, path, err := loadFreshMarketAnomalies()
	if err != nil {
		staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{
			Source:      path,
			Refreshable: false,
		}, err.Error())
		return
	}

	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Header("X-Data-Path", path)
	setDataFreshness(c, dataFreshnessMeta{
		Source:      "market-anomalies",
		DataTime:    snapshotPayloadDataTime(data),
		Refreshable: false,
	})
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}

func loadFreshMarketAnomalies() ([]byte, string, error) {
	candidates := []string{
		filepath.Join("data", "market-anomalies.json"),
		filepath.Join("app", "backend", "data", "market-anomalies.json"),
		filepath.Join("..", "frontend", "public", "data", "market-anomalies.json"),
		filepath.Join("app", "frontend", "public", "data", "market-anomalies.json"),
		filepath.Join("..", "frontend", "dist", "data", "market-anomalies.json"),
		filepath.Join("app", "frontend", "dist", "data", "market-anomalies.json"),
	}

	var (
		bestPath string
		bestRaw  []byte
		bestInfo os.FileInfo
	)
	for _, p := range candidates {
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
		return nil, "", fmt.Errorf("market anomalies data unavailable")
	}

	var payload struct {
		UpdatedAt string `json:"updatedAt"`
	}
	if err := json.Unmarshal(bestRaw, &payload); err != nil {
		return nil, bestPath, err
	}
	updatedAt, err := time.Parse(time.RFC3339, payload.UpdatedAt)
	if err != nil {
		return nil, bestPath, fmt.Errorf("market anomalies updatedAt invalid: %s", payload.UpdatedAt)
	}
	if time.Since(updatedAt) > marketAnomaliesMaxAge {
		return nil, bestPath, fmt.Errorf("market anomalies stale: updated=%s age=%.1fh max=%.0fh", updatedAt.UTC().Format(time.RFC3339), time.Since(updatedAt).Hours(), marketAnomaliesMaxAge.Hours())
	}
	return bestRaw, bestPath, nil
}
