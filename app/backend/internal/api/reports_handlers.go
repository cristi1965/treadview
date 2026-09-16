package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"trading-agents/internal/models"
)

// GetReports handles GET /api/reports
func (h *Handler) GetReports(c *gin.Context) {
	limit, offset, err := parseReportsPagination(c.Query("limit"), c.Query("offset"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reports, freshness, err := getReports()
	if err != nil {
		staleDataError(c, http.StatusServiceUnavailable, freshness, err.Error())
		return
	}
	setDataFreshness(c, freshness)

	start := offset
	end := offset + limit
	if start > len(reports) {
		start = len(reports)
	}
	if end > len(reports) {
		end = len(reports)
	}

	paginatedReports := reports[start:end]
	hasMore := end < len(reports)

	c.JSON(http.StatusOK, models.ReportsListResponse{
		Reports: paginatedReports,
		Total:   len(reports),
		HasMore: hasMore,
	})
}

func parseReportsPagination(limitValue, offsetValue string) (int, int, error) {
	limit, offset := 20, 0
	if strings.TrimSpace(limitValue) != "" {
		parsed, err := strconv.Atoi(limitValue)
		if err != nil || parsed < 1 || parsed > 100 {
			return 0, 0, fmt.Errorf("limit must be an integer between 1 and 100")
		}
		limit = parsed
	}
	if strings.TrimSpace(offsetValue) != "" {
		parsed, err := strconv.Atoi(offsetValue)
		if err != nil || parsed < 0 || parsed > 1_000_000 {
			return 0, 0, fmt.Errorf("offset must be an integer between 0 and 1000000")
		}
		offset = parsed
	}
	return limit, offset, nil
}

// GetReportByID handles GET /api/reports/:id
func (h *Handler) GetReportByID(c *gin.Context) {
	reportID := c.Param("id")
	reports, freshness, err := getReports()
	if err != nil {
		staleDataError(c, http.StatusServiceUnavailable, freshness, err.Error())
		return
	}
	setDataFreshness(c, freshness)

	for _, report := range reports {
		if report.ID == reportID {
			c.JSON(http.StatusOK, gin.H{"report": report})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Report not found"})
}

// GetMarketCalendar handles GET /api/market/calendar
func (h *Handler) GetMarketCalendar(c *gin.Context) {
	events, freshness, period, err := getMarketEventsFromFile()
	if period != "" {
		c.Header("X-Data-Period", period)
	}
	if err != nil {
		staleDataError(c, http.StatusServiceUnavailable, freshness, err.Error())
		return
	}
	setDataFreshness(c, freshness)

	c.JSON(http.StatusOK, models.MarketCalendarResponse{
		Events: events,
	})
}

type liveReportFile struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Date        string `json:"date"`
	Time        string `json:"time"`
	Summary     string `json:"summary"`
	Content     string `json:"content"`
	PublishedAt string `json:"publishedAt"`
}

func getReports() ([]models.Report, dataFreshnessMeta, error) {
	reports, path, info, ok := loadReportsFromJSON()
	freshness := dataFreshnessMeta{Source: "reports-snapshot", Refreshable: false}
	if !ok {
		freshness.Source = "missing"
		freshness.Stale = true
		freshness.StaleReason = "report snapshot unavailable"
		return nil, freshness, os.ErrNotExist
	}
	sortReports(reports)
	freshness.Source = "reports-snapshot:" + filepath.Base(path)
	freshness.DataTime = latestReportDataTime(reports)
	if info != nil {
		freshness.RefreshedAt = info.ModTime().UTC().Format(time.RFC3339)
	}
	freshness.Stale, freshness.StaleReason = staleIfOlder(freshness.DataTime, 24*time.Hour)
	if freshness.Stale {
		sanitizeStaleReportClaims(reports)
	}
	sanitizeInvalidReportMoverClaims(reports)
	return reports, freshness, nil
}

func loadReportsFromJSON() ([]models.Report, string, os.FileInfo, bool) {
	var data []byte
	var path string
	var info os.FileInfo
	for _, candidate := range []string{
		filepath.Join("data", "reports-live.json"),
		filepath.Join("app", "backend", "data", "reports-live.json"),
	} {
		candidateInfo, err := os.Stat(candidate)
		if err != nil || candidateInfo.IsDir() {
			continue
		}
		candidateData, err := os.ReadFile(candidate)
		if err == nil && len(candidateData) > 0 {
			data, path, info = candidateData, candidate, candidateInfo
			break
		}
	}
	if len(data) == 0 {
		return nil, "", nil, false
	}

	var raw []liveReportFile
	if err := json.Unmarshal(data, &raw); err != nil || len(raw) == 0 {
		return nil, path, info, false
	}

	out := make([]models.Report, 0, len(raw))
	for _, r := range raw {
		rtype := models.ReportTypePremarket
		if r.Type == "postmarket" || r.Type == "post" {
			rtype = models.ReportTypePostmarket
		}
		published := parseReportPublishedAt(r)
		summary := r.Summary
		if summary == "" && len(r.Content) > 80 {
			summary = r.Content[:80] + "…"
		}
		out = append(out, models.Report{
			ID:          r.ID,
			Type:        rtype,
			Title:       r.Title,
			Date:        r.Date,
			Time:        r.Time,
			Summary:     summary,
			Content:     r.Content,
			PublishedAt: published,
		})
	}
	sortReports(out)
	return out, path, info, true
}

func sanitizeStaleReportClaims(reports []models.Report) {
	replacer := strings.NewReplacer(
		"实时行情自动生成", "历史行情快照生成",
		"实时行情快照", "历史行情快照",
		"实时行情", "历史行情",
		"live snapshot", "historical snapshot",
	)
	for i := range reports {
		reports[i].Title = replacer.Replace(reports[i].Title)
		reports[i].Summary = replacer.Replace(reports[i].Summary)
		reports[i].Content = replacer.Replace(reports[i].Content)
	}
}

var (
	reportMoverPattern = regexp.MustCompile(`(?i)\b[A-Z][A-Z0-9.-]{0,11}\s+[+-]?[0-9]+(?:\.[0-9]+)?%\s+@\s+\$?[+-]?[0-9]+(?:\.[0-9]+)?`)
	reportPricePattern = regexp.MustCompile(`@\s+\$?([+-]?[0-9]+(?:\.[0-9]+)?)$`)
)

func sanitizeInvalidReportMoverClaims(reports []models.Report) {
	sanitize := func(value string) string {
		return reportMoverPattern.ReplaceAllStringFunc(value, func(candidate string) string {
			priceMatch := reportPricePattern.FindStringSubmatch(candidate)
			if len(priceMatch) != 2 {
				return candidate
			}
			price, err := strconv.ParseFloat(priceMatch[1], 64)
			if err != nil || price > 0 {
				return candidate
			}
			symbol := strings.Fields(candidate)[0]
			return symbol + " 涨跌数据无效（价格 <= 0，已屏蔽）"
		})
	}
	for index := range reports {
		reports[index].Summary = sanitize(reports[index].Summary)
		reports[index].Content = sanitize(reports[index].Content)
	}
}

func parseReportPublishedAt(r liveReportFile) time.Time {
	if strings.TrimSpace(r.PublishedAt) != "" {
		for _, layout := range []string{
			time.RFC3339,
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04 MST",
			"2006-01-02 15:04:05 MST",
		} {
			if t, err := time.Parse(layout, r.PublishedAt); err == nil {
				return t
			}
		}
	}

	loc := reportETLocation()
	if r.Date != "" && r.Time != "" {
		rawTime := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(r.Time, "ET"), "EDT"))
		rawTime = strings.TrimSpace(strings.TrimSuffix(rawTime, "EST"))
		if t, err := time.ParseInLocation("2006-01-02 15:04", r.Date+" "+rawTime, loc); err == nil {
			return t
		}
	}
	if r.Date != "" {
		if t, err := time.ParseInLocation("2006-01-02", r.Date, loc); err == nil {
			return t
		}
	}
	return time.Time{}
}

func sortReports(reports []models.Report) {
	sort.SliceStable(reports, func(i, j int) bool {
		if reports[i].PublishedAt.Equal(reports[j].PublishedAt) {
			return reports[i].Date > reports[j].Date
		}
		return reports[i].PublishedAt.After(reports[j].PublishedAt)
	})
}

// currentReportSlot is used by static-data freshness checks.
func currentReportSlot() (string, models.ReportType, string, time.Time) {
	loc := reportETLocation()
	now := time.Now().In(loc)
	sessionDate := latestUSBusinessDate(now)
	if now.Weekday() >= time.Monday && now.Weekday() <= time.Friday && now.Hour() < 9 {
		sessionDate = latestUSBusinessDate(now.AddDate(0, 0, -1))
	}
	reportType := models.ReportTypePostmarket
	label := "盘后"
	publishedAt := time.Date(sessionDate.Year(), sessionDate.Month(), sessionDate.Day(), 16, 0, 0, 0, loc)
	if now.Weekday() >= time.Monday && now.Weekday() <= time.Friday && now.Hour() >= 9 && now.Hour() < 16 {
		reportType = models.ReportTypePremarket
		label = "盘中"
		publishedAt = now
	}
	return sessionDate.Format("2006-01-02"), reportType, label, publishedAt
}

func latestUSBusinessDate(t time.Time) time.Time {
	for t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		t = t.AddDate(0, 0, -1)
	}
	return t
}

func reportETLocation() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.FixedZone("ET", -4*3600)
	}
	return loc
}

func latestReportDataTime(reports []models.Report) string {
	for _, report := range reports {
		if !report.PublishedAt.IsZero() {
			return report.PublishedAt.UTC().Format(time.RFC3339)
		}
	}
	return ""
}

func marketCalendarTodayET() string {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.FixedZone("ET", -4*3600)
	}
	return time.Now().In(loc).Format("2006-01-02")
}

func getMarketEventsFromFile() ([]models.MarketEvent, dataFreshnessMeta, string, error) {
	today := marketCalendarTodayET()
	events, path, info, err := loadCalendarEventsFromFile()
	freshness := dataFreshnessMeta{Source: "calendar-snapshot", Refreshable: false}
	if err != nil {
		freshness.Source = "missing"
		freshness.DataTime = "unknown"
		freshness.Stale = true
		freshness.StaleReason = "calendar snapshot unavailable"
		return nil, freshness, "", err
	}
	freshness.Source = "calendar-snapshot:" + filepath.Base(path)
	freshness.DataTime = "unknown"
	freshness.RefreshedAt = info.ModTime().UTC().Format(time.RFC3339)
	freshness.Stale = true
	freshness.StaleReason = "calendar snapshot has no provider observation time"
	periodStart, periodEnd := calendarCoverage(events)
	period := ""
	if periodStart != "" && periodEnd != "" {
		period = periodStart + "/" + periodEnd
	}
	if periodStart == "" || periodEnd == "" {
		freshness.Stale = true
		freshness.StaleReason += "; calendar data period is missing or invalid"
	} else if periodEnd < today {
		freshness.Stale = true
		freshness.StaleReason += "; calendar coverage ended at " + periodEnd
	}
	for i := range events {
		events[i].IsToday = events[i].Date == today
		events[i].Importance = calendarImportance(events[i])
	}
	return events, freshness, period, nil
}

func calendarCoverage(events []models.MarketEvent) (string, string) {
	start, end := "", ""
	for _, event := range events {
		date := strings.TrimSpace(event.Date)
		if _, err := time.Parse("2006-01-02", date); err != nil {
			continue
		}
		if start == "" || date < start {
			start = date
		}
		if end == "" || date > end {
			end = date
		}
	}
	return start, end
}

// calendarImportance 保证老数据也有星级：没写 importance 时由 isImportant 推导
// （true → 3 星重磅，false → 1 星普通）。
func calendarImportance(e models.MarketEvent) int {
	if e.Importance >= 1 && e.Importance <= 3 {
		return e.Importance
	}
	if e.IsImportant {
		return 3
	}
	return 1
}

func loadCalendarEventsFromFile() ([]models.MarketEvent, string, os.FileInfo, error) {
	var lastErr error
	for _, p := range []string{
		filepath.Join("data", "calendar-events.json"),
		filepath.Join("app", "backend", "data", "calendar-events.json"),
	} {
		raw, err := os.ReadFile(p)
		if err != nil {
			lastErr = err
			continue
		}
		var events []models.MarketEvent
		if err := json.Unmarshal(raw, &events); err != nil {
			lastErr = err
			continue
		}
		if len(events) > 0 {
			info, err := os.Stat(p)
			if err != nil {
				lastErr = err
				continue
			}
			return events, p, info, nil
		}
		lastErr = fmt.Errorf("calendar snapshot contains no events")
	}
	if lastErr == nil {
		lastErr = os.ErrNotExist
	}
	return nil, "", nil, lastErr
}
