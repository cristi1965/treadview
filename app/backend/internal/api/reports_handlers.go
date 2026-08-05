package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"trading-agents/internal/models"
)

// GetReports handles GET /api/reports
func (h *Handler) GetReports(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	reports := getMockReports()

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

// GetReportByID handles GET /api/reports/:id
func (h *Handler) GetReportByID(c *gin.Context) {
	reportID := c.Param("id")
	reports := getMockReports()

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
	events := getMockMarketEvents()

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

func getMockReports() []models.Report {
	if reports, ok := loadReportsFromJSON(); ok {
		return reports
	}
	return fallbackReports()
}

func loadReportsFromJSON() ([]models.Report, bool) {
	data, err := os.ReadFile(filepath.Join("data", "reports-live.json"))
	if err != nil {
		data, err = os.ReadFile(filepath.Join("app", "backend", "data", "reports-live.json"))
	}
	if err != nil || len(data) == 0 {
		return nil, false
	}

	var raw []liveReportFile
	if err := json.Unmarshal(data, &raw); err != nil || len(raw) == 0 {
		return nil, false
	}

	out := make([]models.Report, 0, len(raw))
	for _, r := range raw {
		rtype := models.ReportTypePremarket
		if r.Type == "postmarket" || r.Type == "post" {
			rtype = models.ReportTypePostmarket
		}
		published := time.Now()
		if r.PublishedAt != "" {
			if t, err := time.Parse(time.RFC3339, r.PublishedAt); err == nil {
				published = t
			} else if t, err := time.Parse("2006-01-02T15:04:05Z", r.PublishedAt); err == nil {
				published = t
			}
		} else if r.Date != "" {
			if t, err := time.Parse("2006-01-02", r.Date); err == nil {
				published = t
			}
		}
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
	return out, true
}

func fallbackReports() []models.Report {
	now := time.Now()
	return []models.Report{
		{
			ID:          "report-20260708-pre",
			Type:        models.ReportTypePremarket,
			Title:       "盘前看点 · 07-08(开盘前 1h)",
			Date:        "2026-07-08",
			Time:        "08:30 ET",
			Summary:     "AI 主线未结束,但拥挤交易需要降温",
			Content:     "盘前数据仅供参考 · 非投资建议",
			PublishedAt: now.Add(-time.Hour * 2),
		},
	}
}

func marketCalendarTodayET() string {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.FixedZone("ET", -4*3600)
	}
	return time.Now().In(loc).Format("2006-01-02")
}

func getMockMarketEvents() []models.MarketEvent {
	today := marketCalendarTodayET()
	events := []models.MarketEvent{
		{
			ID: "event-macro-0729", Date: "2026-07-29", DayOfWeek: "周三",
			Type: models.EventTypeMacro, IsImportant: true,
			Title: "美联储官员讲话 · 美债拍卖", Description: "利率预期与流动性",
			RelatedTickers: []string{"TLT", "XLF", "SPY"},
		},
		{
			ID: "event-earnings-0729", Date: "2026-07-29", DayOfWeek: "周三",
			Type: models.EventTypeEarnings, IsImportant: true,
			Title: "MSFT · META · SBUX 财报窗口", Description: "盘后科技与消费验证",
			RelatedTickers: []string{"MSFT", "META", "SBUX", "PYPL"},
			EarningsDetails: []models.EarningsDetail{
				{Ticker: "MSFT", CompanyName: "Microsoft Corporation", ExpectedEPS: 3.37, MarketCap: "$3.1T", Time: "aftermarket"},
				{Ticker: "META", CompanyName: "Meta Platforms Inc.", ExpectedEPS: 5.87, MarketCap: "$1.4T", Time: "aftermarket"},
				{Ticker: "SBUX", CompanyName: "Starbucks Corporation", ExpectedEPS: 0.72, MarketCap: "$95B", Time: "aftermarket"},
				{Ticker: "PYPL", CompanyName: "PayPal Holdings Inc.", ExpectedEPS: 1.18, MarketCap: "$72B", Time: "aftermarket"},
			},
		},
		{
			ID: "event-macro-0730", Date: "2026-07-30", DayOfWeek: "周四",
			Type: models.EventTypeMacro, IsImportant: true,
			Title: "美国 GDP(初值) · 每周初请失业金", Description: "08:30 ET · 增长与就业交叉验证",
			RelatedTickers: []string{"SPY", "QQQ", "IWM"},
		},
		{
			ID: "event-earnings-0730", Date: "2026-07-30", DayOfWeek: "周四",
			Type: models.EventTypeEarnings,
			Title: "MA · AMGN · NOW 财报", Description: "支付·医药·软件",
			RelatedTickers: []string{"MA", "AMGN", "NOW", "EA"},
			EarningsDetails: []models.EarningsDetail{
				{Ticker: "MA", CompanyName: "Mastercard Inc.", ExpectedEPS: 3.82, MarketCap: "$460B", Time: "premarket"},
				{Ticker: "AMGN", CompanyName: "Amgen Inc.", ExpectedEPS: 5.11, MarketCap: "$160B", Time: "aftermarket"},
				{Ticker: "NOW", CompanyName: "ServiceNow Inc.", ExpectedEPS: 3.92, MarketCap: "$170B", Time: "aftermarket"},
				{Ticker: "EA", CompanyName: "Electronic Arts Inc.", ExpectedEPS: 1.05, MarketCap: "$40B", Time: "aftermarket"},
			},
		},
		{
			ID: "event-macro-0731", Date: "2026-07-31", DayOfWeek: "周五",
			Type: models.EventTypeMacro, IsImportant: true,
			Title: "核心 PCE · 芝加哥 PMI", Description: "通胀主线数据",
			RelatedTickers: []string{"TLT", "XLU", "XLP"},
		},
		{
			ID: "event-earnings-0731", Date: "2026-07-31", DayOfWeek: "周五",
			Type: models.EventTypeEarnings, IsImportant: true,
			Title: "AAPL · AMZN · INTC 财报周末前", Description: "大盘权重集中验证",
			RelatedTickers: []string{"AAPL", "AMZN", "INTC", "COIN"},
			EarningsDetails: []models.EarningsDetail{
				{Ticker: "AAPL", CompanyName: "Apple Inc.", ExpectedEPS: 1.42, MarketCap: "$3.0T", Time: "aftermarket"},
				{Ticker: "AMZN", CompanyName: "Amazon.com Inc.", ExpectedEPS: 1.25, MarketCap: "$2.1T", Time: "aftermarket"},
				{Ticker: "INTC", CompanyName: "Intel Corporation", ExpectedEPS: -0.02, MarketCap: "$95B", Time: "aftermarket"},
				{Ticker: "COIN", CompanyName: "Coinbase Global Inc.", ExpectedEPS: 1.10, MarketCap: "$55B", Time: "aftermarket"},
			},
		},
		{
			ID: "event-macro-0803", Date: "2026-08-03", DayOfWeek: "周一",
			Type: models.EventTypeMacro,
			Title: "ISM 制造业 · 建筑支出", Description: "周期敏感数据",
			RelatedTickers: []string{"XLI", "CAT", "DE"},
		},
		{
			ID: "event-earnings-0804", Date: "2026-08-04", DayOfWeek: "周二",
			Type: models.EventTypeEarnings, IsImportant: true,
			Title: "AMD · Super Micro · Palantir 窗口", Description: "AI 硬件与应用交叉",
			RelatedTickers: []string{"AMD", "SMCI", "PLTR", "AVGO"},
			EarningsDetails: []models.EarningsDetail{
				{Ticker: "AMD", CompanyName: "Advanced Micro Devices", ExpectedEPS: 0.92, MarketCap: "$220B", Time: "aftermarket"},
				{Ticker: "SMCI", CompanyName: "Super Micro Computer", ExpectedEPS: 0.68, MarketCap: "$28B", Time: "aftermarket"},
				{Ticker: "PLTR", CompanyName: "Palantir Technologies", ExpectedEPS: 0.09, MarketCap: "$55B", Time: "aftermarket"},
			},
		},
		{
			ID: "event-macro-0805", Date: "2026-08-05", DayOfWeek: "周三",
			Type: models.EventTypeMacro, IsImportant: true,
			Title: "ADP 就业 · 贸易差额", Description: "非农前哨",
			RelatedTickers: []string{"XLF", "JPM", "BAC"},
		},
		{
			ID: "event-earnings-0805", Date: "2026-08-05", DayOfWeek: "周三",
			Type: models.EventTypeEarnings,
			Title: "DIS · MCD · CVX 财报", Description: "消费与能源",
			RelatedTickers: []string{"DIS", "MCD", "CVX", "XOM"},
			EarningsDetails: []models.EarningsDetail{
				{Ticker: "DIS", CompanyName: "The Walt Disney Company", ExpectedEPS: 1.35, MarketCap: "$190B", Time: "aftermarket"},
				{Ticker: "MCD", CompanyName: "McDonald's Corporation", ExpectedEPS: 3.15, MarketCap: "$210B", Time: "premarket"},
				{Ticker: "CVX", CompanyName: "Chevron Corporation", ExpectedEPS: 2.10, MarketCap: "$280B", Time: "premarket"},
			},
		},
		{
			ID: "event-macro-0806", Date: "2026-08-06", DayOfWeek: "周四",
			Type: models.EventTypeMacro, IsImportant: true,
			Title: "初请失业金 · 成屋销售", Description: "就业与地产温度计",
			RelatedTickers: []string{"IYR", "XHB", "LEN"},
		},
		{
			ID: "event-earnings-0806", Date: "2026-08-06", DayOfWeek: "周四",
			Type: models.EventTypeEarnings, IsImportant: true,
			Title: "ARM · Uber · Shopify", Description: "成长股验证周",
			RelatedTickers: []string{"ARM", "UBER", "SHOP", "SQ"},
			EarningsDetails: []models.EarningsDetail{
				{Ticker: "ARM", CompanyName: "Arm Holdings plc", ExpectedEPS: 0.35, MarketCap: "$140B", Time: "aftermarket"},
				{Ticker: "UBER", CompanyName: "Uber Technologies Inc.", ExpectedEPS: 0.55, MarketCap: "$150B", Time: "aftermarket"},
				{Ticker: "SHOP", CompanyName: "Shopify Inc.", ExpectedEPS: 0.28, MarketCap: "$110B", Time: "before"},
			},
		},
		{
			ID: "event-macro-0807", Date: "2026-08-07", DayOfWeek: "周五",
			Type: models.EventTypeMacro, IsImportant: true,
			Title: "非农就业 · 失业率", Description: "08:30 ET · 本周最重磅宏观",
			RelatedTickers: []string{"SPY", "QQQ", "TLT", "DXY"},
		},
		{
			ID: "event-policy-0811", Date: "2026-08-11", DayOfWeek: "周二",
			Type: models.EventTypePolicy,
			Title: "美国 CPI(7月)", Description: "通胀粘性再确认",
			RelatedTickers: []string{"TLT", "XLU", "XLP", "QQQ"},
		},
	}
	for i := range events {
		events[i].IsToday = events[i].Date == today
	}
	return events
}
