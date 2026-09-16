package models

import "time"

// ReportType represents the type of market report
type ReportType string

const (
	ReportTypePremarket  ReportType = "premarket"  // 盘前
	ReportTypePostmarket ReportType = "postmarket" // 收盘
)

// Report represents a market report
type Report struct {
	ID          string     `json:"id"`
	Type        ReportType `json:"type"`
	Title       string     `json:"title"`
	Date        string     `json:"date"`        // ISO date string
	Time        string     `json:"time"`        // e.g., "16:00 ET"
	Summary     string     `json:"summary"`     // Brief summary for collapsed view
	Content     string     `json:"content"`     // Full content in Markdown
	PublishedAt time.Time  `json:"publishedAt"`
}

// MarketEventType represents the type of market event
type MarketEventType string

const (
	EventTypeMacro    MarketEventType = "macro"    // 宏观数据
	EventTypeEarnings MarketEventType = "earnings" // 财报
	EventTypePolicy   MarketEventType = "policy"   // 政策
	EventTypeHoliday  MarketEventType = "holiday"  // 休市
)

// EarningsDetail represents earnings report details
type EarningsDetail struct {
	Ticker      string  `json:"ticker"`
	CompanyName string  `json:"companyName"`
	ExpectedEPS float64 `json:"expectedEPS"`
	MarketCap   string  `json:"marketCap"` // e.g., "$193B"
	Time        string  `json:"time"`      // "premarket" or "aftermarket"
}

// MarketEvent represents a calendar event
type MarketEvent struct {
	ID              string           `json:"id"`
	Date            string           `json:"date"`      // ISO date string
	DayOfWeek       string           `json:"dayOfWeek"` // e.g., "周五"
	IsToday         bool             `json:"isToday,omitempty"`
	Type            MarketEventType  `json:"type"`
	IsImportant     bool             `json:"isImportant,omitempty"` // 🔴 重磅
	Title           string           `json:"title"`
	Description     string           `json:"description,omitempty"`
	RelatedTickers  []string         `json:"relatedTickers,omitempty"`
	EarningsDetails []EarningsDetail `json:"earnings,omitempty"`

	// Jin10-style economic calendar columns.
	Time       string `json:"time,omitempty"`       // "08:30" in ET
	Country    string `json:"country,omitempty"`    // US / CN / EU / JP / UK / DE
	Importance int    `json:"importance,omitempty"` // 1..3 stars
	Previous   string `json:"previous,omitempty"`   // 前值
	Forecast   string `json:"forecast,omitempty"`   // 预期
	Actual     string `json:"actual,omitempty"`     // 公布值; empty = 未公布
}

// FlashItem is one entry of the 7x24 newsflash feed.
type FlashItem struct {
	ID         string   `json:"id"`
	Time       string   `json:"time"`             // RFC3339 UTC
	Title      string   `json:"title"`            // 中文标题
	TitleEn    string   `json:"titleEn,omitempty"`
	Body       string   `json:"body,omitempty"`
	BodyEn     string   `json:"bodyEn,omitempty"`
	Importance int      `json:"importance"`       // 1 普通 / 2 关注 / 3 重磅(红)
	Kind       string   `json:"kind"`             // macro | earnings | company | market | calendar
	Tickers    []string `json:"tickers,omitempty"`
	Source     string   `json:"source,omitempty"`
	Link       string   `json:"link,omitempty"`
}

// FlashResponse is the response for the newsflash feed.
type FlashResponse struct {
	Items     []FlashItem `json:"items"`
	UpdatedAt string      `json:"updatedAt"`
}

// ReportsListResponse is the response for listing reports
type ReportsListResponse struct {
	Reports []Report `json:"reports"`
	Total   int      `json:"total"`
	HasMore bool     `json:"hasMore"`
}

// MarketCalendarResponse is the response for market calendar
type MarketCalendarResponse struct {
	Events []MarketEvent `json:"events"`
}
