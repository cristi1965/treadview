package dataflows

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const fredBaseURL = "https://api.stlouisfed.org/fred"

// FREDClient fetches macroeconomic data from the Federal Reserve Economic Data API.
type FREDClient struct {
	apiKey     string
	httpClient *http.Client
}

// NewFREDClient creates a new FRED client.
func NewFREDClient(apiKey string) *FREDClient {
	return &FREDClient{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// MacroIndicators holds key macroeconomic indicators.
type MacroIndicators struct {
	FedFundsRate     string `json:"fed_funds_rate"`
	CPI              string `json:"cpi"`
	UnemploymentRate string `json:"unemployment_rate"`
	GDP              string `json:"gdp"`
	TenYearYield     string `json:"ten_year_yield"`
	VIX              string `json:"vix"`
}

// seriesMap maps indicator names to their FRED series IDs.
var seriesMap = map[string]string{
	"fed_funds_rate":  "FEDFUNDS",
	"cpi":             "CPIAUCSL",
	"unemployment":    "UNRATE",
	"gdp":             "GDP",
	"ten_year_yield":  "DGS10",
	"vix":             "VIXCLS",
	"sp500":           "SP500",
	"housing_starts":  "HOUST",
	"industrial_prod": "INDPRO",
	"retail_sales":    "RSXFS",
	"consumer_sent":   "UMCSENT",
}

// GetMacroIndicators fetches key macro indicators and formats them for LLM consumption.
func (c *FREDClient) GetMacroIndicators() (string, error) {
	if c.apiKey == "" {
		return "Macro indicators unavailable: FRED_API_KEY not configured. " +
			"Key indicators can be summarized from public knowledge: " +
			"Federal Funds Rate, CPI, Unemployment Rate, GDP growth, 10-Year Treasury Yield.", nil
	}

	indicators := []struct {
		Name     string
		SeriesID string
	}{
		{"Federal Funds Rate", "FEDFUNDS"},
		{"CPI (Consumer Price Index)", "CPIAUCSL"},
		{"Unemployment Rate", "UNRATE"},
		{"Real GDP", "GDP"},
		{"10-Year Treasury Yield", "DGS10"},
		{"VIX (Volatility Index)", "VIXCLS"},
	}

	var sb strings.Builder
	sb.WriteString("## Macroeconomic Indicators (Latest Available Data)\n\n")
	sb.WriteString("| Indicator | Latest Value | Date |\n")
	sb.WriteString("|-----------|-------------|------|\n")

	for _, ind := range indicators {
		value, date, err := c.getLatestObservation(ind.SeriesID)
		if err != nil {
			sb.WriteString(fmt.Sprintf("| %s | N/A | Error: %v |\n", ind.Name, err))
			continue
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", ind.Name, value, date))
	}

	return sb.String(), nil
}

func (c *FREDClient) getLatestObservation(seriesID string) (string, string, error) {
	u := fmt.Sprintf("%s/series/observations?series_id=%s&api_key=%s&file_type=json&sort_order=desc&limit=1",
		fredBaseURL, seriesID, c.apiKey)

	resp, err := c.httpClient.Get(u)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("FRED API returned %d", resp.StatusCode)
	}

	var result struct {
		Observations []struct {
			Date  string `json:"date"`
			Value string `json:"value"`
		} `json:"observations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	if len(result.Observations) == 0 {
		return "", "", fmt.Errorf("no observations for %s", seriesID)
	}

	obs := result.Observations[0]
	return obs.Value, obs.Date, nil
}
