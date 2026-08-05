package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"trading-agents/internal/models"
)

// GetETFSectors handles GET /api/etf/sectors
func (h *Handler) GetETFSectors(c *gin.Context) {
	category := c.Query("category") // Filter by category
	sortBy := c.DefaultQuery("sort", "aum") // Sort by: aum, return1y, return5y, drawdown
	search := c.Query("q") // Search query

	// TODO: Implement real data fetching from database or external API
	// For now, return mock data structure
	sectors := getETFSectorsData()

	// Apply category filter
	if category != "" && category != "all" {
		filtered := []models.ETFSector{}
		for _, s := range sectors {
			if string(s.Category) == category {
				filtered = append(filtered, s)
			}
		}
		sectors = filtered
	}

	// Apply search filter
	if search != "" {
		filtered := []models.ETFSector{}
		searchLower := strings.ToLower(search)
		for _, s := range sectors {
			if strings.Contains(strings.ToLower(s.Name), searchLower) ||
				strings.Contains(strings.ToLower(s.TopPerformer.Ticker), searchLower) {
				filtered = append(filtered, s)
			}
		}
		sectors = filtered
	}

	// Apply sorting
	sectors = sortETFSectors(sectors, sortBy)

	// Count by category
	categoryCounts := make(map[models.ETFCategory]int)
	allSectors := getETFSectorsData()
	for _, s := range allSectors {
		categoryCounts[s.Category]++
	}

	c.JSON(http.StatusOK, models.ETFSectorsResponse{
		Sectors:    sectors,
		Total:      len(sectors),
		Categories: categoryCounts,
	})
}

// GetETFSectorDetail handles GET /api/etf/sectors/:id
func (h *Handler) GetETFSectorDetail(c *gin.Context) {
	sectorID := c.Param("id")

	// TODO: Fetch sector details and list of ETFs in this sector
	sectors := getETFSectorsData()
	
	var sector *models.ETFSector
	for _, s := range sectors {
		if s.ID == sectorID {
			sector = &s
			break
		}
	}

	if sector == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sector not found"})
		return
	}

	// TODO: Fetch ETFs for this sector
	etfs := []models.ETF{} // Placeholder

	c.JSON(http.StatusOK, gin.H{
		"sector": sector,
		"etfs":   etfs,
	})
}

// SearchETFs handles GET /api/etf/search
func (h *Handler) SearchETFs(c *gin.Context) {
	query := c.Query("q")
	
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	// TODO: Implement ETF search
	results := []models.ETF{} // Placeholder

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
	})
}

// Helper function to get ETF sectors data
// TODO: Replace with actual database queries or API calls
func getETFSectorsData() []models.ETFSector {
	return []models.ETFSector{
		{
			ID:       "tech",
			Name:     "科技",
			Category: models.CategoryIndustry,
			ETFCount: 67,
			AUM:      296000000000,
			TopPerformer: struct {
				Ticker   string  `json:"ticker"`
				Return5Y float64 `json:"return5y"`
			}{Ticker: "PTF", Return5Y: 176},
			MaxDrawdown: -77,
			Return1Y:    floatPtr(42),
			Return5Y:    floatPtr(98),
		},
		{
			ID:       "real-estate",
			Name:     "房地产",
			Category: models.CategoryIndustry,
			ETFCount: 65,
			AUM:      130000000000,
			TopPerformer: struct {
				Ticker   string  `json:"ticker"`
				Return5Y float64 `json:"return5y"`
			}{Ticker: "IDGT", Return5Y: 70},
			MaxDrawdown: -55,
			Return1Y:    floatPtr(12),
			Return5Y:    floatPtr(31),
		},
		{
			ID:       "semiconductor",
			Name:     "半导体",
			Category: models.CategoryIndustry,
			ETFCount: 13,
			AUM:      128000000000,
			TopPerformer: struct {
				Ticker   string  `json:"ticker"`
				Return5Y float64 `json:"return5y"`
			}{Ticker: "SMH", Return5Y: 398},
			MaxDrawdown: -46,
			Return1Y:    floatPtr(65),
			Return5Y:    floatPtr(180),
		},
		{
			ID:       "financial",
			Name:     "金融/银行",
			Category: models.CategoryIndustry,
			ETFCount: 39,
			AUM:      95000000000,
			TopPerformer: struct {
				Ticker   string  `json:"ticker"`
				Return5Y float64 `json:"return5y"`
			}{Ticker: "SPXN", Return5Y: 86},
			MaxDrawdown: -57,
			Return1Y:    floatPtr(28),
			Return5Y:    floatPtr(48),
		},
		{
			ID:       "industrial-defense",
			Name:     "工业/国防",
			Category: models.CategoryIndustry,
			ETFCount: 25,
			AUM:      83000000000,
			TopPerformer: struct {
				Ticker   string  `json:"ticker"`
				Return5Y float64 `json:"return5y"`
			}{Ticker: "PRN", Return5Y: 146},
			MaxDrawdown: -44,
			Return1Y:    floatPtr(33),
			Return5Y:    floatPtr(72),
		},
		{
			ID:       "healthcare",
			Name:     "医疗健康",
			Category: models.CategoryIndustry,
			ETFCount: 30,
			AUM:      76000000000,
			TopPerformer: struct {
				Ticker   string  `json:"ticker"`
				Return5Y float64 `json:"return5y"`
			}{Ticker: "SPXV", Return5Y: 86},
			MaxDrawdown: -53,
			Return1Y:    floatPtr(18),
			Return5Y:    floatPtr(45),
		},
		{
			ID:       "energy",
			Name:     "能源",
			Category: models.CategoryIndustry,
			ETFCount: 32,
			AUM:      75000000000,
			TopPerformer: struct {
				Ticker   string  `json:"ticker"`
				Return5Y float64 `json:"return5y"`
			}{Ticker: "RSPG", Return5Y: 114},
			MaxDrawdown: -45,
			Return1Y:    floatPtr(24),
			Return5Y:    floatPtr(57),
		},
		{
			ID:       "utilities",
			Name:     "公用事业",
			Category: models.CategoryIndustry,
			ETFCount: 11,
			AUM:      39000000000,
			TopPerformer: struct {
				Ticker   string  `json:"ticker"`
				Return5Y float64 `json:"return5y"`
			}{Ticker: "UTES", Return5Y: 93},
			MaxDrawdown: -28,
			Return1Y:    floatPtr(19),
			Return5Y:    floatPtr(46),
		},
		{
			ID:       "communications",
			Name:     "通信",
			Category: models.CategoryIndustry,
			ETFCount: 21,
			AUM:      37000000000,
			TopPerformer: struct {
				Ticker   string  `json:"ticker"`
				Return5Y float64 `json:"return5y"`
			}{Ticker: "XTL", Return5Y: 119},
			MaxDrawdown: -66,
			Return1Y:    floatPtr(21),
			Return5Y:    floatPtr(61),
		},
		{
			ID:       "consumer-discretionary",
			Name:     "可选消费",
			Category: models.CategoryIndustry,
			ETFCount: 15,
			AUM:      27000000000,
			TopPerformer: struct {
				Ticker   string  `json:"ticker"`
				Return5Y float64 `json:"return5y"`
			}{Ticker: "RTH", Return5Y: 54},
			MaxDrawdown: -71,
			Return1Y:    floatPtr(16),
			Return5Y:    floatPtr(33),
		},
		{
			ID:       "consumer-staples",
			Name:     "必需消费",
			Category: models.CategoryIndustry,
			ETFCount: 10,
			AUM:      27000000000,
			TopPerformer: struct {
				Ticker   string  `json:"ticker"`
				Return5Y float64 `json:"return5y"`
			}{Ticker: "VDC", Return5Y: 26},
			MaxDrawdown: -24,
			Return1Y:    floatPtr(9),
			Return5Y:    floatPtr(22),
		},
		{
			ID:       "materials",
			Name:     "材料",
			Category: models.CategoryIndustry,
			ETFCount: 18,
			AUM:      22000000000,
			TopPerformer: struct {
				Ticker   string  `json:"ticker"`
				Return5Y float64 `json:"return5y"`
			}{Ticker: "XME", Return5Y: 154},
			MaxDrawdown: -86,
			Return1Y:    floatPtr(18),
			Return5Y:    floatPtr(69),
		},
		{
			ID:       "biotech",
			Name:     "生物科技",
			Category: models.CategoryIndustry,
			ETFCount: 10,
			AUM:      14000000000,
			TopPerformer: struct {
				Ticker   string  `json:"ticker"`
				Return5Y float64 `json:"return5y"`
			}{Ticker: "BBP", Return5Y: 63},
			MaxDrawdown: -80,
			Return1Y:    floatPtr(14),
			Return5Y:    floatPtr(28),
		},
		{
			ID:       "software-cloud",
			Name:     "软件/云",
			Category: models.CategoryIndustry,
			ETFCount: 14,
			AUM:      13000000000,
			TopPerformer: struct {
				Ticker   string  `json:"ticker"`
				Return5Y float64 `json:"return5y"`
			}{Ticker: "IGPT", Return5Y: 87},
			MaxDrawdown: -77,
			Return1Y:    floatPtr(29),
			Return5Y:    floatPtr(52),
		},
		// Add more sectors as needed
	}
}

// Helper function to sort ETF sectors
func sortETFSectors(sectors []models.ETFSector, sortBy string) []models.ETFSector {
	sorted := make([]models.ETFSector, len(sectors))
	copy(sorted, sectors)

	switch sortBy {
	case "return1y":
		// Sort by 1-year return descending
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				val1 := float64(0)
				val2 := float64(0)
				if sorted[i].Return1Y != nil {
					val1 = *sorted[i].Return1Y
				}
				if sorted[j].Return1Y != nil {
					val2 = *sorted[j].Return1Y
				}
				if val2 > val1 {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
	case "return5y":
		// Sort by 5-year return descending
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				val1 := float64(0)
				val2 := float64(0)
				if sorted[i].Return5Y != nil {
					val1 = *sorted[i].Return5Y
				}
				if sorted[j].Return5Y != nil {
					val2 = *sorted[j].Return5Y
				}
				if val2 > val1 {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
	case "drawdown":
		// Sort by max drawdown ascending (least negative first)
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[j].MaxDrawdown > sorted[i].MaxDrawdown {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
	default: // "aum"
		// Sort by AUM descending
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[j].AUM > sorted[i].AUM {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
	}

	return sorted
}

func floatPtr(f float64) *float64 {
	return &f
}
