package dataflows

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	eastmoneyUListURL      = "https://push2delay.eastmoney.com/api/qt/ulist.np/get"
	eastmoneyUListFallback = "https://push2.eastmoney.com/api/qt/ulist.np/get"
)

// CNQuote is a live A-share quote from Eastmoney push2.
type CNQuote struct {
	Symbol     string
	Name       string
	Price      float64
	Pct        float64
	Vol        float64
	McapYi     float64 // 亿元
	ObservedAt time.Time
}

// CNQuoteClient fetches A-share quotes via Eastmoney ulist API (no key).
type CNQuoteClient struct {
	http *http.Client
}

func NewCNQuoteClient() *CNQuoteClient {
	return &CNQuoteClient{http: &http.Client{Timeout: 12 * time.Second}}
}

// GetQuotes batches codes like "600519","000001". Max ~80 per request recommended.
func (c *CNQuoteClient) GetQuotes(codes []string) ([]CNQuote, error) {
	secids := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		secids = append(secids, toEastmoneySecID(code))
	}
	if len(secids) == 0 {
		return nil, nil
	}

	q := url.Values{}
	q.Set("fltt", "2")
	q.Set("fields", "f12,f14,f2,f3,f5,f20,f124")
	q.Set("secids", strings.Join(secids, ","))
	qs := q.Encode()

	raw, err := c.fetchUList(eastmoneyUListURL + "?" + qs)
	if err != nil {
		raw, err = c.fetchUList(eastmoneyUListFallback + "?" + qs)
	}
	if err != nil {
		return nil, err
	}

	return parseCNQuotes(raw)
}

func parseCNQuotes(raw []byte) ([]CNQuote, error) {
	var parsed struct {
		Data struct {
			Diff []map[string]any `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("eastmoney parse: %w", err)
	}

	out := make([]CNQuote, 0, len(parsed.Data.Diff))
	for _, row := range parsed.Data.Diff {
		sym := asString(row["f12"])
		price := asFloat(row["f2"])
		if sym == "" || price <= 0 {
			continue
		}
		mcap := asFloat(row["f20"]) // 元
		observedAt := time.Time{}
		if observedUnix := int64(asFloat(row["f124"])); observedUnix > 0 {
			observedAt = time.Unix(observedUnix, 0).UTC()
		}
		out = append(out, CNQuote{
			Symbol:     sym,
			Name:       asString(row["f14"]),
			Price:      price,
			Pct:        asFloat(row["f3"]),
			Vol:        asFloat(row["f5"]),
			McapYi:     mcap / 1e8,
			ObservedAt: observedAt,
		})
	}
	return out, nil
}

func (c *CNQuoteClient) fetchUList(fullURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eastmoney ulist HTTP %d: %s", resp.StatusCode, truncateBytes(raw, 200))
	}
	return raw, nil
}

func toEastmoneySecID(code string) string {
	code = strings.TrimSpace(code)
	if strings.HasPrefix(code, "5") || strings.HasPrefix(code, "6") || strings.HasPrefix(code, "9") {
		return "1." + code // SH
	}
	return "0." + code // SZ
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case json.Number:
		return t.String()
	default:
		return fmt.Sprint(v)
	}
}

func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	default:
		return 0
	}
}

func truncateBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n])
}
