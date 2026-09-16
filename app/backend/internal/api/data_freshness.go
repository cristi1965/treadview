package api

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const marketQuoteFreshFor = 90 * time.Second

type dataFreshnessMeta struct {
	Source        string   `json:"source"`
	DataTime      string   `json:"dataTime,omitempty"`
	RefreshedAt   string   `json:"refreshedAt"`
	Stale         bool     `json:"stale"`
	StaleReason   string   `json:"staleReason,omitempty"`
	Refreshable   bool     `json:"refreshable"`
	PartialErrors []string `json:"partialErrors,omitempty"`
}

func setDataFreshness(c *gin.Context, meta dataFreshnessMeta) {
	now := time.Now().UTC().Format(time.RFC3339)
	if meta.RefreshedAt == "" {
		meta.RefreshedAt = now
	}
	if meta.Stale && meta.StaleReason == "" {
		meta.StaleReason = "stale snapshot or source unavailable"
	}
	c.Header("Cache-Control", "no-store, max-age=0")
	c.Header("X-Data-Source", meta.Source)
	if meta.DataTime != "" {
		c.Header("X-Data-Time", meta.DataTime)
	}
	c.Header("X-Data-Refreshed-At", meta.RefreshedAt)
	c.Header("X-Data-Stale", strconv.FormatBool(meta.Stale))
	c.Header("X-Data-Refreshable", strconv.FormatBool(meta.Refreshable))
	if meta.StaleReason != "" {
		c.Header("X-Data-Stale-Reason", meta.StaleReason)
	}
	if len(meta.PartialErrors) > 0 {
		c.Header("X-Data-Partial-Errors", strings.Join(meta.PartialErrors, ","))
	}
}

func staleDataError(c *gin.Context, status int, meta dataFreshnessMeta, message string) {
	meta.Stale = true
	if meta.StaleReason == "" {
		meta.StaleReason = message
	}
	setDataFreshness(c, meta)
	c.JSON(status, gin.H{
		"error": message,
		"meta":  meta,
	})
}

func realSnapshotSource(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "stale-snapshot"
	}
	return "stale-snapshot:" + name
}

func sourceIsDisallowedForProduct(source string) bool {
	source = strings.ToLower(source)
	return strings.Contains(source, "mock") || strings.Contains(source, "fallback")
}

func millisDataTime(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.UnixMilli(ts).UTC().Format(time.RFC3339)
}

func parseLooseDataTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	if strings.HasPrefix(value, "ts:") {
		if ms, err := strconv.ParseInt(strings.TrimPrefix(value, "ts:"), 10, 64); err == nil && ms > 0 {
			return time.UnixMilli(ms), true
		}
	}
	if ms, err := strconv.ParseInt(value, 10, 64); err == nil && ms > 1_000_000_000_000 {
		return time.UnixMilli(ms), true
	}
	if strings.HasSuffix(value, " ET") {
		base := strings.TrimSuffix(value, " ET")
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			return time.Time{}, false
		}
		if t, err := time.ParseInLocation("2006-01-02 15:04", base, loc); err == nil {
			return t, true
		}
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02 15:04 MST",
		"2006-01-02 15:04",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, value); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func staleIfOlder(dataTime string, maxAge time.Duration) (bool, string) {
	return staleIfOlderAt(dataTime, maxAge, time.Now())
}

func staleIfOlderAt(dataTime string, maxAge time.Duration, now time.Time) (bool, string) {
	t, ok := parseLooseDataTime(dataTime)
	if !ok {
		return true, "missing or invalid data time"
	}
	age := now.Sub(t)
	if age < -time.Minute {
		return true, "data time is in the future"
	}
	if age > maxAge {
		return true, fmt.Sprintf("data age %.1fh exceeds %.1fh", age.Hours(), maxAge.Hours())
	}
	return false, ""
}

func marketQuoteFreshness(source string, dataTime time.Time, now time.Time) (bool, string) {
	if dataTime.IsZero() {
		return true, "missing quote data time"
	}
	if strings.Contains(strings.ToLower(source), "stale-snapshot") {
		return true, "live quotes unavailable; serving last real snapshot"
	}
	if dataTime.After(now.Add(time.Minute)) {
		return true, "quote data time is in the future"
	}
	age := now.Sub(dataTime)
	if age > marketQuoteFreshFor {
		return true, fmt.Sprintf("quote age %.0fs exceeds %.0fs", age.Seconds(), marketQuoteFreshFor.Seconds())
	}
	return false, ""
}

func dataTimeFromSource(source string) string {
	if idx := strings.LastIndex(source, "@"); idx >= 0 && idx+1 < len(source) {
		value := strings.TrimSpace(source[idx+1:])
		if len(value) == len("2006-01-02") {
			return value
		}
		if t, ok := parseLooseDataTime(value); ok {
			return t.UTC().Format(time.RFC3339)
		}
		return value
	}
	return ""
}

func dataTimeOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
