package dataflows

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var numRe = regexp.MustCompile(`[-+]?(?:\d+\.?\d*|\.\d+)`)

// GetStockMetricsLive prefers East Money valuation + Nasdaq ratios (Yahoo crumb is often 429).
func (c *YFinanceClient) GetStockMetricsLive(ticker string) (*StockMetrics, error) {
	ticker = strings.TrimSpace(strings.ToUpper(ticker))
	if ticker == "" {
		return nil, fmt.Errorf("empty ticker")
	}

	out := &StockMetrics{Symbol: ticker, Source: "eastmoney+nasdaq"}

	// 1) Yahoo when crumb works (best completeness)
	if ym, err := c.GetStockMetrics(ticker); err == nil && ym != nil && (ym.TrailingPE > 0 || ym.GrossMargin > 0) {
		ym.Source = "yahoo-quoteSummary"
		return ym, nil
	}

	emErr := enrichFromEastMoney(c.httpClient, out)
	ndErr := enrichFromNasdaq(c.httpClient, out)
	if out.TrailingPE == 0 && out.GrossMargin == 0 && out.Price == 0 && out.Sector == "" {
		return nil, fmt.Errorf("fundamentals unavailable for %s (em=%v nd=%v)", ticker, emErr, ndErr)
	}
	return out, nil
}

func enrichFromEastMoney(client *http.Client, out *StockMetrics) error {
	// 105=NASDAQ, 106=NYSE, 107=AMEX on East Money push2
	var lastErr error
	for _, board := range []int{105, 106, 107} {
		u := fmt.Sprintf("https://push2.eastmoney.com/api/qt/stock/get?secid=%d.%s&fields=f57,f58,f43,f170,f162,f163,f164,f167,f116,f117,f191",
			board, url.QueryEscape(out.Symbol))
		body, err := httpGet(client, u, map[string]string{
			"Referer": "https://quote.eastmoney.com/",
		})
		if err != nil {
			lastErr = err
			continue
		}
		var resp struct {
			RC   int            `json:"rc"`
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil || resp.Data == nil {
			lastErr = fmt.Errorf("eastmoney parse board=%d: %w", board, err)
			continue
		}
		if name, ok := resp.Data["f58"].(string); ok && out.Name == "" {
			out.Name = name
		}
		if p := emScaled(resp.Data["f43"], 1000); p > 0 {
			out.Price = p
		}
		// f164 ≈ PE TTM * 100; f163 static PE; f167 PB * 100
		if pe := emScaled(resp.Data["f164"], 100); pe > 0 {
			out.TrailingPE = pe
		} else if pe := emScaled(resp.Data["f163"], 100); pe > 0 {
			out.TrailingPE = pe
		}
		if pb := emScaled(resp.Data["f167"], 100); pb > 0 {
			out.PriceToBook = pb
		}
		if out.Price > 0 || out.TrailingPE > 0 || out.PriceToBook > 0 {
			return nil
		}
		lastErr = fmt.Errorf("eastmoney empty board=%d", board)
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("eastmoney miss")
}

func enrichFromNasdaq(client *http.Client, out *StockMetrics) error {
	hdr := map[string]string{
		"Accept":  "application/json",
		"Origin":  "https://www.nasdaq.com",
		"Referer": "https://www.nasdaq.com/",
	}

	// Summary: sector / industry / target / yield / 52w
	sumURL := fmt.Sprintf("https://api.nasdaq.com/api/quote/%s/summary?assetclass=stocks", url.PathEscape(out.Symbol))
	if body, err := httpGet(client, sumURL, hdr); err == nil {
		var resp struct {
			Data struct {
				SummaryData map[string]struct {
					Value string `json:"value"`
				} `json:"summaryData"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &resp) == nil {
			sd := resp.Data.SummaryData
			if v := sd["Sector"].Value; v != "" && out.Sector == "" {
				out.Sector = v
			}
			if v := sd["Industry"].Value; v != "" && out.Industry == "" {
				out.Industry = v
			}
			if v := sd["OneYrTarget"].Value; v != "" && out.TargetMeanPrice == 0 {
				out.TargetMeanPrice = parseMoney(v)
			}
			if v := sd["Yield"].Value; v != "" && out.DividendYield == 0 {
				out.DividendYield = parsePercentFraction(v)
			}
			if v := sd["FiftTwoWeekHighLow"].Value; v != "" {
				hi, lo := parseRange(v)
				if out.FiftyTwoHigh == 0 {
					out.FiftyTwoHigh = hi
				}
				if out.FiftyTwoLow == 0 {
					out.FiftyTwoLow = lo
				}
			}
			if v := sd["MarketCap"].Value; v != "" && out.Name == "" {
				// keep as-is; market cap not stored on StockMetrics
			}
		}
	}

	infoURL := fmt.Sprintf("https://api.nasdaq.com/api/quote/%s/info?assetclass=stocks", url.PathEscape(out.Symbol))
	if body, err := httpGet(client, infoURL, hdr); err == nil {
		var resp struct {
			Data struct {
				CompanyName string `json:"companyName"`
				PrimaryData struct {
					LastSalePrice string `json:"lastSalePrice"`
				} `json:"primaryData"`
				KeyStats struct {
					FiftyTwoWeekHighLow struct {
						Value string `json:"value"`
					} `json:"fiftyTwoWeekHighLow"`
				} `json:"keyStats"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &resp) == nil {
			if out.Name == "" {
				out.Name = resp.Data.CompanyName
			}
			if out.Price == 0 {
				out.Price = parseMoney(resp.Data.PrimaryData.LastSalePrice)
			}
			if out.FiftyTwoHigh == 0 || out.FiftyTwoLow == 0 {
				hi, lo := parseRange(resp.Data.KeyStats.FiftyTwoWeekHighLow.Value)
				if out.FiftyTwoHigh == 0 {
					out.FiftyTwoHigh = hi
				}
				if out.FiftyTwoLow == 0 {
					out.FiftyTwoLow = lo
				}
			}
		}
	}

	finURL := fmt.Sprintf("https://api.nasdaq.com/api/company/%s/financials?frequency=1", url.PathEscape(out.Symbol))
	if body, err := httpGet(client, finURL, hdr); err == nil {
		var resp struct {
			Data struct {
				FinancialRatiosTable struct {
					Rows []map[string]string `json:"rows"`
				} `json:"financialRatiosTable"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &resp) == nil {
			for _, row := range resp.Data.FinancialRatiosTable.Rows {
				label := strings.ToLower(strings.TrimSpace(row["value1"]))
				val := row["value2"]
				switch label {
				case "gross margin":
					out.GrossMargin = parsePercentFraction(val)
				case "profit margin":
					out.ProfitMargin = parsePercentFraction(val)
				case "after tax roe", "roe":
					out.ROE = parsePercentFraction(val)
				case "operating margin":
					// unused field on StockMetrics; skip
				}
			}
		}
	}

	tpURL := fmt.Sprintf("https://api.nasdaq.com/api/analyst/%s/targetprice", url.PathEscape(out.Symbol))
	if body, err := httpGet(client, tpURL, hdr); err == nil {
		var resp struct {
			Data struct {
				ConsensusOverview struct {
					PriceTarget float64 `json:"priceTarget"`
					Buy         int     `json:"buy"`
					Hold        int     `json:"hold"`
					Sell        int     `json:"sell"`
				} `json:"consensusOverview"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &resp) == nil {
			if out.TargetMeanPrice == 0 {
				out.TargetMeanPrice = resp.Data.ConsensusOverview.PriceTarget
			}
			b, h, s := resp.Data.ConsensusOverview.Buy, resp.Data.ConsensusOverview.Hold, resp.Data.ConsensusOverview.Sell
			if out.Recommendation == "" && (b+h+s) > 0 {
				switch {
				case b >= (h+s)*2:
					out.Recommendation = "strong_buy"
				case b > h && b > s:
					out.Recommendation = "buy"
				case s > b:
					out.Recommendation = "sell"
				default:
					out.Recommendation = "hold"
				}
			}
		}
	}

	if out.FiftyTwoHigh > out.FiftyTwoLow && out.FiftyTwoLow > 0 && out.Price > 0 {
		out.FiftyTwoPos = (out.Price - out.FiftyTwoLow) / (out.FiftyTwoHigh - out.FiftyTwoLow) * 100
	}
	return nil
}

func httpGet(client *http.Client, rawURL string, extra map[string]string) ([]byte, error) {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return body, nil
}

func emScaled(v any, div float64) float64 {
	f := anyFloat(v)
	if f == 0 || div == 0 {
		return 0
	}
	return f / div
}

func anyFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	case int:
		return float64(t)
	case int64:
		return float64(t)
	default:
		return 0
	}
}

func parseMoney(s string) float64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	s = strings.TrimPrefix(s, "$")
	if s == "" || strings.EqualFold(s, "n/a") {
		return 0
	}
	m := numRe.FindString(s)
	f, _ := strconv.ParseFloat(m, 64)
	return f
}

func parsePercentFraction(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "n/a") {
		return 0
	}
	m := numRe.FindString(s)
	f, _ := strconv.ParseFloat(m, 64)
	if strings.Contains(s, "%") || f > 1.5 {
		return f / 100
	}
	return f
}

func parseRange(s string) (high, low float64) {
	// "164.07 - 236.54" or "164.07 - 236.54"
	parts := strings.Split(s, "-")
	if len(parts) < 2 {
		return 0, 0
	}
	a := parseMoney(parts[0])
	b := parseMoney(parts[len(parts)-1])
	if a > b {
		return a, b
	}
	return b, a
}
