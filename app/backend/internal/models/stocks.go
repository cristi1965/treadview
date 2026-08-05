package models

// Stock represents a stock in the scan list
type Stock struct {
	Symbol      string  `json:"symbol"`      // Stock ticker
	Name        string  `json:"name"`        // Company name
	Price       float64 `json:"price"`       // Current price
	Change      float64 `json:"change"`      // Price change
	ChangePercent float64 `json:"changePercent"` // Change percentage
	MarketCap   float64 `json:"marketCap"`   // Market capitalization
	Volume      int64   `json:"volume"`      // Trading volume
	Sector      string  `json:"sector"`      // Industry sector
	
	// Five-factor scores (0-100)
	Scores FiveFactorScores `json:"scores"`
	AvgScore float64        `json:"avgScore"` // Average of 5 scores
	
	// Post-analysis metrics
	PostAnalysisChange *float64 `json:"postAnalysisChange,omitempty"` // Change after analysis
	Divergence         *float64 `json:"divergence,omitempty"`          // Score divergence (variance)
	
	// Watchlist status
	IsWatched bool `json:"isWatched"`
}

// FiveFactorScores represents the five independent evaluation dimensions
type FiveFactorScores struct {
	Buffett        int `json:"buffett"`        // 巴菲特维度 (0-100)
	Duanyongping   int `json:"duanyongping"`   // 段永平维度 (0-100)
	Serenity       int `json:"serenity"`       // Serenity维度 (0-100)
	Druckenmiller  int `json:"druckenmiller"`  // 德鲁肯米勒维度 (0-100)
	Sentiment      int `json:"sentiment"`      // 情绪资金面维度 (0-100)
}

// StocksListRequest represents query parameters for stock list
type StocksListRequest struct {
	Market    string  `form:"market"`    // Market: us, cn, hk
	Sort      string  `form:"sort"`      // Sort field: marketcap, price, change, avgscore
	Order     string  `form:"order"`     // Sort order: asc, desc
	Page      int     `form:"page"`      // Page number (1-indexed)
	Limit     int     `form:"limit"`     // Items per page
	Search    string  `form:"q"`         // Search query
	MinScore  *int    `form:"minScore"`  // Minimum avg score filter
	MaxScore  *int    `form:"maxScore"`  // Maximum avg score filter
}

// StocksListResponse is the API response for stock listing
type StocksListResponse struct {
	Stocks []Stock `json:"stocks"`
	Total  int     `json:"total"`
	Page   int     `json:"page"`
	Limit  int     `json:"limit"`
	HasMore bool   `json:"hasMore"`
}

// StockSearchResponse is the response for stock search
type StockSearchResponse struct {
	Results []Stock `json:"results"`
	Count   int     `json:"count"`
}
