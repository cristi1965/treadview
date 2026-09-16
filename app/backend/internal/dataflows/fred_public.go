package dataflows

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type FREDValue struct {
	Date  time.Time
	Value float64
}

// LatestFREDValue uses FRED's public CSV export and does not require an API key.
func LatestFREDValue(seriesID string) (FREDValue, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://fred.stlouisfed.org/graph/fredgraph.csv?id=" + seriesID)
	if err != nil {
		return FREDValue{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return FREDValue{}, fmt.Errorf("fred status %d", resp.StatusCode)
	}
	reader := csv.NewReader(resp.Body)
	reader.FieldsPerRecord = -1
	var latest FREDValue
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(row) < 2 || strings.EqualFold(row[0], "observation_date") {
			continue
		}
		date, err := time.Parse("2006-01-02", strings.TrimSpace(row[0]))
		if err != nil || strings.TrimSpace(row[1]) == "." {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(row[1]), 64)
		if err == nil {
			latest = FREDValue{Date: date, Value: value}
		}
	}
	if latest.Date.IsZero() {
		return FREDValue{}, fmt.Errorf("fred series %s has no observations", seriesID)
	}
	return latest, nil
}
