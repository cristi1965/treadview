package api

import (
	"net/http"
	"strconv"
	"strings"
	"trading-agents/internal/database"
	"trading-agents/internal/models"

	"github.com/gin-gonic/gin"
)

// GetGurus fetches all gurus (US Giants or A-Shares) mapped to frontend structures
func (h *Handler) GetGurus(c *gin.Context) {
	guruType := c.Query("type") // frontend: us_gurus | a_share_top | private | hot_money

	var gurus []models.Guru
	query := database.DB

	switch guruType {
	case "us_gurus":
		query = query.Where("type = ?", "superinvestor")
	case "a_share_top":
		query = query.Where("type = ?", "fund")
	case "private":
		query = query.Where("type = ?", "private_fund")
	case "hot_money":
		query = query.Where("type = ?", "hot_money")
	case "":
		query = query.Where("type != ?", "politician")
	default:
		query = query.Where("type = ?", guruType)
	}

	if err := query.Find(&gurus).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Map to frontend-compatible structure, attach top recent holdings for density
	var responseGurus []map[string]interface{}
	for _, guru := range gurus {
		limit := 8
		if guru.Type == "hot_money" || guru.Type == "fund" {
			limit = 12
		}
		var topHoldings []models.Holding
		_ = database.DB.Where("guru_id = ?", guru.ID).Order("weight desc").Limit(limit).Find(&topHoldings)

		recent := make([]map[string]interface{}, 0, len(topHoldings))
		for _, hold := range topHoldings {
			action := "买入"
			chg := strings.ToLower(hold.Change)
			if strings.Contains(chg, "sell") || strings.Contains(chg, "减") || strings.Contains(chg, "trim") {
				action = "卖出"
			} else if strings.Contains(chg, "hold") || strings.Contains(chg, "持有") || chg == "" {
				if guru.Type == "hot_money" || guru.Type == "fund" {
					action = "买入"
				} else {
					action = "持有"
				}
			}
			recent = append(recent, map[string]interface{}{
				"symbol": hold.StockSymbol,
				"name":   hold.StockName,
				"weight": hold.Weight,
				"action": action,
			})
		}

		res := map[string]interface{}{
			"id":       strconv.FormatUint(uint64(guru.ID), 10),
			"name":     guru.Name,
			"nameEn":   guru.NameEn,
			"slug":     guru.Slug,
			"company":  guru.FundName,
			"avatar":   "",
			"type":     guru.Type,
			"holdings": guru.PositionCount,
			"topStock": map[string]interface{}{
				"symbol":     guru.TopStock,
				"name":       guru.TopStock,
				"percentage": guru.TopStockWeight,
			},
			"recentHoldings": recent,
		}
		responseGurus = append(responseGurus, res)
	}

	c.JSON(http.StatusOK, responseGurus)
}

// GetGuruDetails fetches a guru and their holdings mapped to flat frontend WhaleDetail
func (h *Handler) GetGuruDetails(c *gin.Context) {
	idStr := c.Param("id")
	
	var guru models.Guru
	var lookupErr error
	
	// Try parsing as numeric ID first, fallback to slug lookup
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err == nil {
		lookupErr = database.DB.First(&guru, id).Error
	} else {
		lookupErr = database.DB.Where("slug = ?", idStr).First(&guru).Error
	}
	
	if lookupErr != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Guru not found"})
		return
	}

	var holdings []models.Holding
	if err := database.DB.Where("guru_id = ?", guru.ID).Order("weight desc").Find(&holdings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Calculate statistics
	totalHoldings := len(holdings)
	topTenConcentration := 0.0
	newPositions := 0
	reducedPositions := 0
	
	type FrontendHolding struct {
		Rank           int     `json:"rank"`
		Symbol         string  `json:"symbol"`
		Name           string  `json:"name"`
		ConsensusCount int     `json:"consensusCount"`
		MarketShare    float64 `json:"marketShare"`
		Action         string  `json:"action"`
		ActionLabel    string  `json:"actionLabel"`
	}
	
	var mappedHoldings []FrontendHolding
	for i, hold := range holdings {
		rank := i + 1
		if rank <= 10 {
			topTenConcentration += hold.Weight
		}
		
		action := "hold"
		actionLabel := "持有"
		if strings.Contains(hold.Change, "加仓") {
			action = "buy"
			actionLabel = "加仓"
			newPositions++
		} else if strings.Contains(hold.Change, "减持") || strings.Contains(hold.Change, "trim") {
			action = "sell"
			actionLabel = "减持"
			reducedPositions++
		} else if strings.Contains(hold.Change, "新进") || strings.Contains(hold.Change, "new") {
			action = "new"
			actionLabel = "新建仓"
			newPositions++
		}
		
		mappedHoldings = append(mappedHoldings, FrontendHolding{
			Rank:           rank,
			Symbol:         hold.StockSymbol,
			Name:           hold.StockName,
			ConsensusCount: 1,
			MarketShare:    hold.Weight,
			Action:         action,
			ActionLabel:    actionLabel,
		})
	}
	
	if topTenConcentration > 100.0 {
		topTenConcentration = 100.0
	}

	c.JSON(http.StatusOK, gin.H{
		"name":                guru.Name,
		"nameEn":              guru.NameEn,
		"company":             guru.FundName,
		"reportType":          "13F",
		"updatedAt":           guru.CreatedAt.Format("2006-01-02"),
		"totalHoldings":       totalHoldings,
		"topTenConcentration": topTenConcentration,
		"newPositions":        newPositions,
		"reducedPositions":    reducedPositions,
		"holdings":            mappedHoldings,
	})
}

// GetCongressTrades fetches transactions by US Congress members, grouped by politician
func (h *Handler) GetCongressTrades(c *gin.Context) {
	partyFilter := c.Query("party")
	tradeType := c.Query("type")

	var trades []models.CongressTrade
	query := database.DB.Order("date desc")

	if partyFilter != "" {
		gormParty := partyFilter
		if partyFilter == "democrat" {
			gormParty = "Democratic"
		} else if partyFilter == "republican" {
			gormParty = "Republican"
		}
		query = query.Where("party = ?", gormParty)
	}
	if tradeType != "" {
		query = query.Where("type = ?", strings.ToUpper(tradeType))
	}

	if err := query.Find(&trades).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type LatestTrade struct {
		Action     string `json:"action"`
		Symbol     string `json:"symbol"`
		SymbolName string `json:"symbolName"`
		Amount     string `json:"amount"`
		Date       string `json:"date"`
	}
	type FrontendMember struct {
		ID          string      `json:"id"`
		Name        string      `json:"name"`
		Party       string      `json:"party"` // "D" or "R"
		State       string      `json:"state"`
		District    string      `json:"district"`
		LatestTrade LatestTrade `json:"latestTrade"`
		IsHot       bool        `json:"isHot"`
	}

	var members []FrontendMember
	seenPoliticians := make(map[string]bool)

	for _, t := range trades {
		if seenPoliticians[t.Politician] {
			continue
		}
		seenPoliticians[t.Politician] = true

		party := "R"
		if t.Party == "Democratic" {
			party = "D"
		}

		action := "buy"
		if t.Type == "SELL" {
			action = "sell"
		}

		state := ""
		district := t.District
		parts := strings.Split(t.District, "-")
		if len(parts) == 2 {
			state = parts[0]
			district = parts[1]
		} else {
			state = t.District
		}

		members = append(members, FrontendMember{
			ID:       strconv.FormatUint(uint64(t.ID), 10),
			Name:     t.Politician,
			Party:    party,
			State:    state,
			District: district,
			LatestTrade: LatestTrade{
				Action:     action,
				Symbol:     t.Symbol,
				SymbolName: t.Symbol,
				Amount:     t.Amount,
				Date:       t.Date,
			},
			IsHot: t.Politician == "Nancy Pelosi" || t.Politician == "Josh Gottheimer" || t.Politician == "Ro Khanna",
		})
	}

	c.JSON(http.StatusOK, members)
}

func slugifyPoliticianName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}

// GetCongressPolitician handles GET /api/whales/congress/:slug — politician secondary page.
func (h *Handler) GetCongressPolitician(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}

	var trades []models.CongressTrade
	if err := database.DB.Order("date desc").Find(&trades).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var matched []models.CongressTrade
	var displayName string
	for _, t := range trades {
		if slugifyPoliticianName(t.Politician) == slug || strings.EqualFold(t.Politician, slug) {
			matched = append(matched, t)
			if displayName == "" {
				displayName = t.Politician
			}
		}
	}
	if len(matched) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "politician not found", "slug": slug})
		return
	}

	first := matched[0]
	party := "R"
	if first.Party == "Democratic" {
		party = "D"
	}
	state := first.District
	district := ""
	parts := strings.Fields(first.District)
	if len(parts) >= 1 {
		state = parts[0]
	}
	if len(parts) >= 2 {
		district = parts[1]
	}

	type tradeRow struct {
		Action string `json:"action"`
		Symbol string `json:"symbol"`
		Amount string `json:"amount"`
		Date   string `json:"date"`
		Title  string `json:"title"`
	}
	rows := make([]tradeRow, 0, len(matched))
	for _, t := range matched {
		action := "buy"
		if t.Type == "SELL" {
			action = "sell"
		}
		rows = append(rows, tradeRow{
			Action: action,
			Symbol: t.Symbol,
			Amount: t.Amount,
			Date:   t.Date,
			Title:  t.Title,
		})
	}

	c.Header("X-Data-Source", "congress-db")
	c.JSON(http.StatusOK, gin.H{
		"id":         slug,
		"slug":       slug,
		"name":       displayName,
		"party":      party,
		"state":      state,
		"district":   district,
		"title":      first.Title,
		"tradeCount": len(rows),
		"latestTrade": rows[0],
		"trades":     rows,
	})
}

// GetSmartMoneyConsensus fetches the ranked list of most held stocks by Gurus
func (h *Handler) GetSmartMoneyConsensus(c *gin.Context) {
	type ConsensusResult struct {
		Symbol    string  `json:"symbol"`
		Name      string  `json:"name"`
		GuruCount int64   `json:"guruCount"`
		AvgWeight float64 `json:"avgWeight"`
		Change    string  `json:"change"`
	}

	var rawResults []struct {
		StockSymbol string
		StockName   string
		Count       int64
		AvgWeight   float64
	}

	// Query GORM to calculate counts and averages
	err := database.DB.Model(&models.Holding{}).
		Select("stock_symbol, stock_name, count(distinct(guru_id)) as count, avg(weight) as avg_weight").
		Group("stock_symbol, stock_name").
		Order("count desc, avg_weight desc").
		Find(&rawResults).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	results := make([]ConsensusResult, len(rawResults))
	for i, r := range rawResults {
		// Mock changes based on symbol to make it look realistic
		change := "+0.0%"
		if r.StockSymbol == "NVDA" {
			change = "+45.4%"
		} else if r.StockSymbol == "MSFT" {
			change = "+7.2%"
		} else if r.StockSymbol == "AAPL" {
			change = "-1.2%"
		}

		results[i] = ConsensusResult{
			Symbol:    r.StockSymbol,
			Name:      r.StockName,
			GuruCount: r.Count,
			AvgWeight: r.AvgWeight,
			Change:    change,
		}
	}

	c.JSON(http.StatusOK, results)
}

// GetStockHolders fetches all gurus holding a specific stock symbol (stripping any simulated suffix like -A)
func (h *Handler) GetStockHolders(c *gin.Context) {
	symbol := c.Param("symbol")

	// Strip simulated suffixes (e.g. "NVDA-A" -> "NVDA") to align queries
	baseSymbol := symbol
	if idx := strings.Index(symbol, "-"); idx != -1 {
		baseSymbol = symbol[:idx]
	}

	type HolderResult struct {
		GuruName   string  `json:"guruName"`
		FundName   string  `json:"fundName"`
		Value      string  `json:"value"`
		Shares     string  `json:"shares"`
		Change     string  `json:"change"`
		Weight     float64 `json:"weight"`
		AvatarCode string  `json:"avatarCode"`
	}

	var holdings []models.Holding
	if err := database.DB.Where("stock_symbol = ?", baseSymbol).Find(&holdings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	results := make([]HolderResult, 0, len(holdings))
	for _, hol := range holdings {
		var guru models.Guru
		if err := database.DB.First(&guru, hol.GuruID).Error; err == nil {
			results = append(results, HolderResult{
				GuruName:   guru.Name,
				FundName:   guru.FundName,
				Value:      hol.Value,
				Shares:     hol.Shares,
				Change:     hol.Change,
				Weight:     hol.Weight,
				AvatarCode: guru.AvatarCode,
			})
		}
	}

	c.JSON(http.StatusOK, results)
}

// GetWhalesSyncStatus returns the background sync status
func (h *Handler) GetWhalesSyncStatus(c *gin.Context) {
	database.SyncMutex.Lock()
	defer database.SyncMutex.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"syncing":    database.WhalesSyncing,
		"lastSync":   database.WhalesLastSync,
		"syncStatus": database.WhalesSyncStatus,
	})
}

// TriggerWhalesSync starts EDGAR / public-disclosure sync only (no stockgod upstream).
func (h *Handler) TriggerWhalesSync(c *gin.Context) {
	go func() {
		database.RunEDGARSync()
	}()
	c.JSON(http.StatusOK, gin.H{"success": true, "mode": "edgar-only"})
}
