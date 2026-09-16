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

type CBOEIndexValue struct {
	Date  time.Time
	Close float64
	Prev  float64
}

// LatestVIX reads Cboe's official daily VIX history. It is an end-of-day
// value, so callers must label it as such rather than calling it intraday.
func LatestVIX() (CBOEIndexValue, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://cdn.cboe.com/api/global/us_indices/daily_prices/VIX_History.csv")
	if err != nil {
		return CBOEIndexValue{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return CBOEIndexValue{}, fmt.Errorf("cboe vix status %d", resp.StatusCode)
	}
	reader := csv.NewReader(resp.Body)
	reader.FieldsPerRecord = -1
	var previous CBOEIndexValue
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(row) < 5 || strings.EqualFold(strings.TrimSpace(row[0]), "DATE") {
			continue
		}
		date, err := time.Parse("1/2/2006", strings.TrimSpace(row[0]))
		if err != nil {
			continue
		}
		closeValue, err := strconv.ParseFloat(strings.TrimSpace(row[4]), 64)
		if err != nil || closeValue <= 0 {
			continue
		}
		previous = CBOEIndexValue{Date: date, Close: closeValue, Prev: previous.Close}
	}
	if previous.Close <= 0 {
		return CBOEIndexValue{}, fmt.Errorf("cboe vix history has no valid rows")
	}
	return previous, nil
}
