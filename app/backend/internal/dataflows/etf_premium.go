package dataflows

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type QDIIPremium struct {
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	NAV             *float64 `json:"nav"`
	NavDate         string   `json:"navDate"`
	Price           *float64 `json:"price"`
	PricePct        *float64 `json:"pricePct"`
	PremiumPct      *float64 `json:"premiumPct"`
	PriceObservedAt string   `json:"priceObservedAt,omitempty"`
	PriceSource     string   `json:"priceSource,omitempty"`
	Level           string   `json:"level"`
	Index           string   `json:"index"`
	Status          string   `json:"status"`
	StatusReason    string   `json:"statusReason,omitempty"`
}

type QDIIPremiumClient struct {
	http         *http.Client
	fundURL      string
	quoteFetcher func([]string) ([]CNQuote, error)
}

func NewQDIIPremiumClient() *QDIIPremiumClient {
	return &QDIIPremiumClient{
		http:         &http.Client{Timeout: 12 * time.Second},
		fundURL:      "https://fundmobapi.eastmoney.com/FundMNewApi/FundMNFInfo",
		quoteFetcher: NewCNQuoteClient().GetQuotes,
	}
}

var qdiiIndexMap = map[string]string{
	"513100": "nasdaq",
	"159941": "nasdaq",
	"513300": "nasdaq",
	"159501": "nasdaq",
	"159513": "nasdaq",
	"159632": "nasdaq",
	"159696": "nasdaq",
	"159509": "nasdaq",
	"513500": "sp500",
	"161125": "sp500",
	"159612": "sp500",
	"513400": "dow",
}

func getLevel(premium float64) string {
	// 🟠 修复 Bug#3：折价（负值）是买入机会，不应标为"危险"
	// 只有正溢价（场内价高于净值）才是杀溢价风险
	if premium < 0 {
		// 折价：场内价 < 净值，对买方有利
		if premium <= -5 {
			return "safe" // 深度折价 = 好事
		}
		return "safe"
	}
	if premium >= 10 {
		return "extreme"
	}
	if premium >= 5 {
		return "danger"
	}
	if premium >= 2 {
		return "caution"
	}
	return "safe"
}

func (c *QDIIPremiumClient) GetPremiums() ([]QDIIPremium, error) {
	fcodes := make([]string, 0, len(qdiiIndexMap))
	for code := range qdiiIndexMap {
		fcodes = append(fcodes, code)
	}
	sort.Strings(fcodes)

	q := url.Values{}
	q.Set("pageIndex", "1")
	q.Set("pageSize", "200")
	q.Set("plat", "Android")
	q.Set("appType", "ttjj")
	q.Set("product", "EFund")
	q.Set("Version", "1")
	q.Set("deviceid", "test")
	q.Set("Ession", "fund")
	q.Set("Fcodes", strings.Join(fcodes, ","))

	reqURL := c.fundURL + "?" + q.Encode()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://fund.eastmoney.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("eastmoney fund read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eastmoney fund HTTP %d: %s", resp.StatusCode, truncateBytes(raw, 200))
	}

	var parsed struct {
		ErrCode    int              `json:"ErrCode"`
		Success    bool             `json:"Success"`
		TotalCount int              `json:"TotalCount"`
		Datas      []map[string]any `json:"Datas"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("eastmoney fund parse: %w", err)
	}

	if !parsed.Success || parsed.ErrCode != 0 {
		return nil, fmt.Errorf("eastmoney fund rejected request: success=%t errCode=%d", parsed.Success, parsed.ErrCode)
	}
	items := ensureQDIIPremiumCoverage(parseQDIIPremiumRows(parsed.Datas), fcodes)
	missingQuotes := make([]string, 0)
	for _, item := range items {
		if item.NAV != nil && item.NavDate != "" && (item.Price == nil || item.PremiumPct == nil || item.PriceObservedAt == "") {
			missingQuotes = append(missingQuotes, item.Code)
		}
	}
	if len(missingQuotes) == 0 || c.quoteFetcher == nil {
		return items, nil
	}
	quotes, quoteErr := c.quoteFetcher(missingQuotes)
	if quoteErr != nil {
		markQDIISupplementFailure(items, missingQuotes, quoteErr)
		return items, nil
	}
	return supplementQDIIPremiums(items, quotes), nil
}

func parseQDIIPremiumRows(rows []map[string]any) []QDIIPremium {
	out := make([]QDIIPremium, 0, len(rows))
	for _, row := range rows {
		code := asString(row["FCODE"])
		if code == "" {
			continue
		}

		idx, ok := qdiiIndexMap[code]
		if !ok {
			idx = "other"
		}

		nav, hasNAV := strictFloat(row["NAV"])
		price, hasPrice := strictFloat(row["NEWPRICE"])
		zjl, hasPremium := strictFloat(row["ZJL"])
		pricePct, hasPricePct := strictFloat(row["CHANGERATIO"])
		navDate := strings.TrimSpace(asString(row["PDATE"]))
		_, hasNavDate := parseQDIINavDate(navDate)
		priceObservedAt, hasPriceObservedAt := parseQDIIPricedAt(asString(row["HQDATE"]))

		item := QDIIPremium{
			Code: code, Name: asString(row["SHORTNAME"]), NavDate: navDate,
			Level: "unknown", Index: idx, Status: "unknown",
		}
		missing := make([]string, 0, 4)
		if hasNAV && nav > 0 {
			item.NAV = floatPointer(nav)
		} else {
			missing = append(missing, "NAV")
		}
		if hasPrice && price > 0 {
			item.Price = floatPointer(price)
			item.PriceSource = "eastmoney:FundMNFInfo"
		} else {
			missing = append(missing, "price")
		}
		if hasPremium {
			premium := -zjl
			item.PremiumPct = floatPointer(premium)
		} else {
			missing = append(missing, "premium")
		}
		if !hasNavDate {
			missing = append(missing, "NAV date")
		}
		if hasPricePct {
			item.PricePct = floatPointer(pricePct)
		}
		if hasPriceObservedAt {
			item.PriceObservedAt = priceObservedAt.Format(time.RFC3339)
		} else {
			missing = append(missing, "price time")
		}
		if len(missing) == 0 {
			item.Status = "complete"
			item.Level = getLevel(*item.PremiumPct)
		} else {
			item.StatusReason = "missing or invalid " + strings.Join(missing, ", ")
		}
		out = append(out, item)
	}
	return out
}

func ensureQDIIPremiumCoverage(items []QDIIPremium, requested []string) []QDIIPremium {
	byCode := make(map[string]QDIIPremium, len(items))
	for _, item := range items {
		byCode[item.Code] = item
	}
	out := make([]QDIIPremium, 0, len(requested))
	for _, code := range requested {
		if item, ok := byCode[code]; ok {
			out = append(out, item)
			continue
		}
		out = append(out, QDIIPremium{Code: code, Index: qdiiIndexMap[code], Level: "unknown", Status: "unknown", StatusReason: "fund endpoint omitted requested code"})
	}
	return out
}

func supplementQDIIPremiums(items []QDIIPremium, quotes []CNQuote) []QDIIPremium {
	byCode := make(map[string]CNQuote, len(quotes))
	for _, quote := range quotes {
		byCode[quote.Symbol] = quote
	}
	for index := range items {
		item := &items[index]
		quote, ok := byCode[item.Code]
		if !ok || quote.Price <= 0 || quote.ObservedAt.IsZero() || item.NAV == nil || *item.NAV <= 0 {
			continue
		}
		premium := (quote.Price/(*item.NAV) - 1) * 100
		item.Price = floatPointer(quote.Price)
		item.PricePct = floatPointer(quote.Pct)
		item.PremiumPct = floatPointer(premium)
		item.PriceObservedAt = quote.ObservedAt.UTC().Format(time.RFC3339)
		item.PriceSource = "eastmoney:push2"
		if _, navOK := parseQDIINavDate(item.NavDate); navOK {
			item.Status = "complete"
			item.Level = getLevel(premium)
			item.StatusReason = ""
		}
	}
	return items
}

func markQDIISupplementFailure(items []QDIIPremium, codes []string, err error) {
	missing := make(map[string]bool, len(codes))
	for _, code := range codes {
		missing[code] = true
	}
	for index := range items {
		if missing[items[index].Code] {
			items[index].StatusReason = strings.TrimSpace(items[index].StatusReason + "; quote supplement failed: " + err.Error())
		}
	}
}

func strictFloat(value any) (float64, bool) {
	if value == nil {
		return 0, false
	}
	var parsed float64
	switch typed := value.(type) {
	case float64:
		parsed = typed
	case float32:
		parsed = float64(typed)
	case int:
		parsed = float64(typed)
	case int64:
		parsed = float64(typed)
	case json.Number:
		value, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		parsed = value
	case string:
		clean := strings.TrimSpace(typed)
		if clean == "" || clean == "-" || clean == "--" || strings.EqualFold(clean, "null") {
			return 0, false
		}
		value, err := strconv.ParseFloat(clean, 64)
		if err != nil {
			return 0, false
		}
		parsed = value
	default:
		return 0, false
	}
	return parsed, !math.IsNaN(parsed) && !math.IsInf(parsed, 0)
}

func floatPointer(value float64) *float64 { return &value }

func parseQDIINavDate(value string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func parseQDIIPricedAt(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if layout == time.RFC3339 {
			if parsed, err := time.Parse(layout, value); err == nil {
				return parsed.UTC(), true
			}
			continue
		}
		location, err := time.LoadLocation("Asia/Shanghai")
		if err != nil {
			return time.Time{}, false
		}
		if parsed, err := time.ParseInLocation(layout, value, location); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

// OldestQDIIPremiumDataTime is known only when every critical input is direct and complete.
func OldestQDIIPremiumDataTime(items []QDIIPremium) (string, bool) {
	if len(items) == 0 {
		return "unknown", false
	}
	oldest := time.Time{}
	for _, item := range items {
		if item.Status != "complete" || item.NAV == nil || item.Price == nil || item.PremiumPct == nil {
			return "unknown", false
		}
		navTime, ok := parseQDIINavDate(item.NavDate)
		if !ok {
			return "unknown", false
		}
		priceTime, ok := parseQDIIPricedAt(item.PriceObservedAt)
		if !ok {
			return "unknown", false
		}
		for _, observedAt := range []time.Time{navTime, priceTime} {
			if oldest.IsZero() || observedAt.Before(oldest) {
				oldest = observedAt
			}
		}
	}
	return oldest.Format(time.RFC3339), true
}

func QDIIPremiumSource(items []QDIIPremium) string {
	hasFund, hasPush := false, false
	for _, item := range items {
		hasFund = hasFund || item.PriceSource == "eastmoney:FundMNFInfo"
		hasPush = hasPush || item.PriceSource == "eastmoney:push2"
	}
	if hasFund && hasPush {
		return "eastmoney:FundMNFInfo+push2"
	}
	if hasPush {
		return "eastmoney:push2"
	}
	return "eastmoney:FundMNFInfo"
}
