package models

// ETFCategory represents the category of an ETF
type ETFCategory string

const (
	CategoryBroad      ETFCategory = "broad"
	CategoryIndustry   ETFCategory = "industry"
	CategoryTheme      ETFCategory = "theme"
	CategoryFactor     ETFCategory = "factor"
	CategoryBond       ETFCategory = "bond"
	CategoryCommodity  ETFCategory = "commodity"
	CategoryLeveraged  ETFCategory = "leveraged"
	CategoryOther      ETFCategory = "other"
)

// ETFSector represents a sector grouping of ETFs
type ETFSector struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Category ETFCategory `json:"category"`
	ETFCount int         `json:"etfCount"`
	AUM      float64     `json:"aum"` // Total assets under management in USD

	TopPerformer struct {
		Ticker   string  `json:"ticker"`
		Return5Y float64 `json:"return5y"` // 5-year return percentage
	} `json:"topPerformer"`

	MaxDrawdown float64 `json:"maxDrawdown"` // Maximum drawdown percentage (negative)

	// Optional sorting fields
	Return1Y         *float64 `json:"return1y,omitempty"`         // 1-year average return
	Return5Y         *float64 `json:"return5y,omitempty"`         // 5-year average return
	AvgExpenseRatio  *float64 `json:"avgExpenseRatio,omitempty"`  // Average expense ratio
}

// ETFCategoryInfo holds category metadata
type ETFCategoryInfo struct {
	ID    ETFCategory `json:"id"`
	Label string      `json:"label"`
	Count int         `json:"count"`
}

// ETFSectorsResponse is the API response for listing sectors
type ETFSectorsResponse struct {
	Sectors    []ETFSector                `json:"sectors"`
	Total      int                        `json:"total"`
	Categories map[ETFCategory]int        `json:"categories"`
}

// ETF represents an individual ETF
type ETF struct {
	Ticker        string      `json:"ticker"`
	Name          string      `json:"name"`
	Category      ETFCategory `json:"category"`
	Sector        string      `json:"sector"`
	AUM           float64     `json:"aum"`
	ExpenseRatio  float64     `json:"expenseRatio"`
	Return1Y      *float64    `json:"return1y,omitempty"`
	Return5Y      *float64    `json:"return5y,omitempty"`
	MaxDrawdown   float64     `json:"maxDrawdown"`
	YTDReturn     *float64    `json:"ytdReturn,omitempty"`
	Dividend      *float64    `json:"dividend,omitempty"`
	Volume        *int64      `json:"volume,omitempty"`
	InceptionDate string      `json:"inceptionDate,omitempty"`
}
