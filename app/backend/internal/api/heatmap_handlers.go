package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HeatmapNode represents a node in the heatmap
type HeatmapNode struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	ChangePercent float64 `json:"changePercent"`
	MarketCap     int64   `json:"marketCap"`
	Sector        string  `json:"sector"`
	AvgScore      float64 `json:"avgScore"`
}

// GetHeatmapData returns all stocks for the heatmap visualization
func GetHeatmapData(c *gin.Context) {
	// Mock data - 986 stocks (simulating full market)
	nodes := generateHeatmapNodes()

	c.JSON(http.StatusOK, gin.H{
		"nodes": nodes,
		"total": len(nodes),
	})
}

func generateHeatmapNodes() []HeatmapNode {
	// Mock sectors
	sectors := []string{
		"Technology", "Semiconductors", "Consumer", "Healthcare",
		"Financials", "Energy", "Industrials", "Materials",
		"Real Estate", "Utilities", "Communications", "Transport",
	}

	// Generate nodes (simulating 986 stocks)
	// For demo, we'll generate a representative sample
	nodes := []HeatmapNode{
		// Tech giants
		{"NVDA", "NVIDIA", 194.83, -1.39, 4710000000000, "Semiconductors", 66.2},
		{"AAPL", "Apple", 308.63, 4.84, 4530000000000, "Technology", 60.0},
		{"MSFT", "Microsoft", 568.12, 0.61, 4210000000000, "Technology", 65.4},
		{"GOOGL", "Alphabet", 287.45, -0.85, 3820000000000, "Technology", 58.6},
		{"AMZN", "Amazon", 214.78, 1.23, 2180000000000, "Consumer", 62.8},
		{"META", "Meta", 623.45, -2.14, 1580000000000, "Technology", 54.2},
		{"TSLA", "Tesla", 238.92, 3.67, 756000000000, "Consumer", 48.6},
		{"BRK.B", "Berkshire", 465.23, 0.45, 965000000000, "Financials", 72.4},
		{"TSM", "TSMC", 198.34, -1.12, 1020000000000, "Semiconductors", 68.8},
		{"V", "Visa", 312.67, 0.89, 645000000000, "Financials", 61.2},
		
		// More semiconductors
		{"AMD", "AMD", 178.23, -2.45, 288000000000, "Semiconductors", 58.4},
		{"AVGO", "Broadcom", 1845.67, -1.78, 875000000000, "Semiconductors", 64.2},
		{"INTC", "Intel", 42.56, 1.23, 178000000000, "Semiconductors", 42.8},
		{"QCOM", "Qualcomm", 223.45, 0.67, 251000000000, "Semiconductors", 56.4},
		{"TXN", "Texas Instruments", 198.76, -0.34, 182000000000, "Semiconductors", 59.2},
		
		// Healthcare
		{"UNH", "UnitedHealth", 612.34, 2.1, 568000000000, "Healthcare", 66.8},
		{"JNJ", "Johnson & Johnson", 178.92, 0.45, 432000000000, "Healthcare", 70.2},
		{"LLY", "Eli Lilly", 878.45, 3.21, 834000000000, "Healthcare", 68.4},
		{"ABBV", "AbbVie", 189.23, 1.12, 334000000000, "Healthcare", 62.6},
		{"PFE", "Pfizer", 28.67, 1.4, 162000000000, "Healthcare", 54.2},
		
		// Financials
		{"JPM", "JPMorgan", 234.56, 0.78, 678000000000, "Financials", 68.4},
		{"BAC", "Bank of America", 45.23, 0.34, 356000000000, "Financials", 58.6},
		{"WFC", "Wells Fargo", 67.89, 0.56, 234000000000, "Financials", 56.2},
		{"GS", "Goldman Sachs", 512.34, 1.23, 178000000000, "Financials", 62.8},
		{"MS", "Morgan Stanley", 123.45, 0.89, 156000000000, "Financials", 60.4},
		
		// Energy
		{"XOM", "Exxon Mobil", 112.34, -0.45, 456000000000, "Energy", 64.2},
		{"CVX", "Chevron", 167.89, -0.67, 312000000000, "Energy", 62.8},
		{"COP", "ConocoPhillips", 134.56, 0.34, 156000000000, "Energy", 58.6},
		
		// Consumer
		{"WMT", "Walmart", 78.23, 0.23, 412000000000, "Consumer", 66.4},
		{"PG", "Procter & Gamble", 167.45, 0.45, 398000000000, "Consumer", 68.2},
		{"KO", "Coca-Cola", 64.32, 0.12, 278000000000, "Consumer", 64.8},
		{"PEP", "PepsiCo", 178.90, 0.34, 245000000000, "Consumer", 62.4},
		{"COST", "Costco", 923.45, 1.23, 410000000000, "Consumer", 70.6},
		
		// Industrials
		{"BA", "Boeing", 178.23, -1.45, 108000000000, "Industrials", 48.2},
		{"CAT", "Caterpillar", 412.34, 1.9, 212000000000, "Industrials", 64.8},
		{"HON", "Honeywell", 234.56, 1.6, 156000000000, "Industrials", 62.4},
		{"GE", "General Electric", 178.90, 0.78, 198000000000, "Industrials", 58.6},
		
		// Communications
		{"T", "AT&T", 18.45, -0.34, 132000000000, "Communications", 52.4},
		{"VZ", "Verizon", 42.67, -0.12, 179000000000, "Communications", 54.8},
		{"DIS", "Disney", 112.34, 2.34, 204000000000, "Communications", 56.2},
		{"NFLX", "Netflix", 712.45, 3.45, 312000000000, "Communications", 60.8},
		
		// Materials
		{"LIN", "Linde", 478.90, 0.67, 234000000000, "Materials", 62.4},
		{"APD", "Air Products", 312.34, 0.45, 67000000000, "Materials", 58.6},
		
		// Real Estate
		{"AMT", "American Tower", 223.45, -0.23, 102000000000, "Real Estate", 56.8},
		{"PLD", "Prologis", 134.56, 0.34, 125000000000, "Real Estate", 60.2},
		
		// Utilities
		{"NEE", "NextEra Energy", 78.90, 0.12, 156000000000, "Utilities", 64.2},
		{"DUK", "Duke Energy", 112.34, 0.23, 87000000000, "Utilities", 62.8},
	}

	// Generate additional nodes to reach closer to 986
	// We'll create variations of existing stocks with slightly different values
	additionalNodes := make([]HeatmapNode, 0, 936) // 986 - 50 = 936
	
	for i := 0; i < 936; i++ {
		baseNode := nodes[i%len(nodes)]
		
		// Create variation
		variation := float64(i%100 - 50) / 100.0 // -0.5 to 0.5
		
		additionalNodes = append(additionalNodes, HeatmapNode{
			Symbol:        baseNode.Symbol + "-" + string(rune(65+i%26)), // e.g., AAPL-A, AAPL-B
			Name:          baseNode.Name + " " + string(rune(65+i%26)),
			Price:         baseNode.Price * (1 + variation*0.1),
			ChangePercent: baseNode.ChangePercent * (1 + variation*0.5),
			MarketCap:     int64(float64(baseNode.MarketCap) * (1 + variation*0.2)),
			Sector:        sectors[i%len(sectors)],
			AvgScore:      baseNode.AvgScore * (1 + variation*0.1),
		})
	}

	// Combine base nodes with additional nodes
	allNodes := append(nodes, additionalNodes...)
	
	return allNodes
}
