package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

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
		_ = database.DB.Where("guru_id = ? AND report_period = ?", guru.ID, guru.ReportPeriod).
			Order("weight desc").Limit(limit).Find(&topHoldings)
		var storedHoldingCount int64
		_ = database.DB.Model(&models.Holding{}).
			Where("guru_id = ? AND report_period = ?", guru.ID, guru.ReportPeriod).
			Count(&storedHoldingCount).Error

		recent := make([]map[string]interface{}, 0, len(topHoldings))
		for _, hold := range topHoldings {
			action := "持有"
			chg := strings.ToLower(hold.Change)
			if strings.Contains(chg, "add") || strings.Contains(chg, "new") || strings.Contains(chg, "加") {
				action = "买入"
			} else if strings.Contains(chg, "sell") || strings.Contains(chg, "减") || strings.Contains(chg, "trim") {
				action = "卖出"
			}
			recent = append(recent, map[string]interface{}{
				"symbol": hold.StockSymbol,
				"name":   hold.StockName,
				"weight": hold.Weight,
				"action": action,
			})
		}

		topStock := map[string]interface{}{
			"symbol":     "",
			"name":       "",
			"percentage": 0.0,
		}
		if len(topHoldings) > 0 {
			topStock = map[string]interface{}{
				"symbol":     topHoldings[0].StockSymbol,
				"name":       topHoldings[0].StockName,
				"percentage": topHoldings[0].Weight,
			}
		}

		res := map[string]interface{}{
			"id":             strconv.FormatUint(uint64(guru.ID), 10),
			"name":           guru.Name,
			"nameEn":         guru.NameEn,
			"slug":           guru.Slug,
			"company":        guru.FundName,
			"avatar":         "",
			"type":           guru.Type,
			"reportPeriod":   guru.ReportPeriod,
			"filingDate":     guru.FilingDate,
			"accession":      guru.Accession,
			"source":         guru.Source,
			"sourceAsOf":     guru.SourceAsOf,
			"sourceURL":      guru.SourceURL,
			"syncedAt":       dataTimeOrEmpty(guru.SyncedAt),
			"stale":          guruDisclosureStale(guru),
			"holdings":       int(storedHoldingCount),
			"topStock":       topStock,
			"recentHoldings": recent,
		}
		responseGurus = append(responseGurus, res)
	}

	setDataFreshness(c, whalesFreshnessForGurus(gurus, "whales-db"))
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

	reportPeriod := strings.TrimSpace(c.Query("reportPeriod"))
	if reportPeriod == "" {
		reportPeriod = guru.ReportPeriod
	}
	var holdings []models.Holding
	if err := database.DB.Where("guru_id = ? AND report_period = ?", guru.ID, reportPeriod).
		Order("weight desc").Find(&holdings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	filing := filingForPeriod(guru, reportPeriod)
	var availablePeriods []string
	_ = database.DB.Model(&models.GuruFiling{}).Where("guru_id = ?", guru.ID).
		Order("report_period desc").Pluck("report_period", &availablePeriods).Error
	if len(availablePeriods) == 0 && reportPeriod != "" {
		availablePeriods = []string{reportPeriod}
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
	consensusCounts := consensusCountsForGuru(guru, reportPeriod)
	for i, hold := range holdings {
		rank := i + 1
		if rank <= 10 {
			topTenConcentration += hold.Weight
		}

		change := strings.ToLower(hold.Change)
		action := "hold"
		actionLabel := "持有"
		if strings.Contains(change, "add") || strings.Contains(change, "加仓") {
			action = "buy"
			actionLabel = "加仓"
			newPositions++
		} else if strings.Contains(change, "减持") || strings.Contains(change, "trim") || strings.Contains(change, "sell") {
			action = "sell"
			actionLabel = "减持"
			reducedPositions++
		} else if strings.Contains(change, "新进") || strings.Contains(change, "new") {
			action = "new"
			actionLabel = "新建仓"
			newPositions++
		}

		mappedHoldings = append(mappedHoldings, FrontendHolding{
			Rank:           rank,
			Symbol:         hold.StockSymbol,
			Name:           hold.StockName,
			ConsensusCount: consensusCounts[strings.ToUpper(strings.TrimSpace(hold.StockSymbol))],
			MarketShare:    hold.Weight,
			Action:         action,
			ActionLabel:    actionLabel,
		})
	}

	if topTenConcentration > 100.0 {
		topTenConcentration = 100.0
	}

	setDataFreshness(c, whalesFreshnessForFiling(filing))
	c.JSON(http.StatusOK, gin.H{
		"name":                guru.Name,
		"nameEn":              guru.NameEn,
		"company":             guru.FundName,
		"reportType":          disclosureReportType(filing.Source),
		"updatedAt":           dataTimeOrEmpty(filing.SyncedAt),
		"reportPeriod":        filing.ReportPeriod,
		"filingDate":          filing.FilingDate,
		"accession":           filing.Accession,
		"source":              filing.Source,
		"sourceAsOf":          filing.SourceAsOf,
		"sourceURL":           filing.SourceURL,
		"syncedAt":            dataTimeOrEmpty(filing.SyncedAt),
		"stale":               filingDisclosureStale(filing),
		"availablePeriods":    availablePeriods,
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
		Source     string `json:"source"`
		FilingDate string `json:"filingDate"`
		SourceURL  string `json:"sourceURL"`
		FilingID   string `json:"filingId"`
		Verified   bool   `json:"verified"`
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
				Source:     sourceOrDefault(t.Source, "unknown"),
				FilingDate: sourceOrDefault(t.FilingDate, "unknown"),
				SourceURL:  sourceOrDefault(t.SourceURL, "unknown"),
				FilingID:   sourceOrDefault(t.FilingID, "unknown"),
				Verified:   congressTradeVerified(t),
			},
			IsHot: t.Politician == "Nancy Pelosi" || t.Politician == "Josh Gottheimer" || t.Politician == "Ro Khanna",
		})
	}

	setDataFreshness(c, congressDisclosureFreshnessMeta())
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
		Action     string `json:"action"`
		Symbol     string `json:"symbol"`
		Amount     string `json:"amount"`
		Date       string `json:"date"`
		Title      string `json:"title"`
		Source     string `json:"source"`
		FilingDate string `json:"filingDate"`
		SourceURL  string `json:"sourceURL"`
		FilingID   string `json:"filingId"`
		Verified   bool   `json:"verified"`
	}
	rows := make([]tradeRow, 0, len(matched))
	for _, t := range matched {
		action := "buy"
		if t.Type == "SELL" {
			action = "sell"
		}
		rows = append(rows, tradeRow{
			Action:     action,
			Symbol:     t.Symbol,
			Amount:     t.Amount,
			Date:       t.Date,
			Title:      t.Title,
			Source:     sourceOrDefault(t.Source, "unknown"),
			FilingDate: sourceOrDefault(t.FilingDate, "unknown"),
			SourceURL:  sourceOrDefault(t.SourceURL, "unknown"),
			FilingID:   sourceOrDefault(t.FilingID, "unknown"),
			Verified:   congressTradeVerified(t),
		})
	}

	setDataFreshness(c, congressDisclosureFreshnessMeta())
	c.JSON(http.StatusOK, gin.H{
		"id":          slug,
		"slug":        slug,
		"name":        displayName,
		"party":       party,
		"state":       state,
		"district":    district,
		"title":       first.Title,
		"tradeCount":  len(rows),
		"latestTrade": rows[0],
		"trades":      rows,
	})
}

func congressTradeVerified(trade models.CongressTrade) bool {
	for _, value := range []string{trade.Source, trade.FilingDate, trade.SourceURL, trade.FilingID} {
		value = strings.TrimSpace(value)
		if value == "" || strings.EqualFold(value, "unknown") {
			return false
		}
	}
	url := strings.ToLower(strings.TrimSpace(trade.SourceURL))
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")
}

// GetSmartMoneyConsensus fetches the ranked list of most held stocks by Gurus
func (h *Handler) GetSmartMoneyConsensus(c *gin.Context) {
	type ConsensusResult struct {
		Symbol       string  `json:"symbol"`
		Name         string  `json:"name"`
		GuruCount    int64   `json:"guruCount"`
		AvgWeight    float64 `json:"avgWeight"`
		AddCount     int64   `json:"addCount"`
		TrimCount    int64   `json:"trimCount"`
		ReportPeriod string  `json:"reportPeriod"`
		Source       string  `json:"source"`
	}

	var rawResults []struct {
		StockSymbol string
		StockName   string
		Count       int64
		AvgWeight   float64
		AddCount    int64
		TrimCount   int64
	}
	cohort := whaleCohort(c.Query("type"))
	reportPeriod := strings.TrimSpace(c.Query("reportPeriod"))
	if reportPeriod == "" {
		var latest sql.NullString
		if err := database.DB.Model(&models.Guru{}).Where("type = ? AND report_period != ''", cohort).
			Select("MAX(report_period)").Scan(&latest).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if latest.Valid {
			reportPeriod = strings.TrimSpace(latest.String)
		}
	}
	if reportPeriod == "" {
		setDataFreshness(c, missingWhalesFreshness("whales-db", "cannot calculate consensus: no shared disclosure report period"))
		c.JSON(http.StatusOK, []ConsensusResult{})
		return
	}
	consensusSource, sourceOK := sourceForConsensus(cohort, reportPeriod)
	if !sourceOK {
		setDataFreshness(c, missingWhalesFreshness("whales-db", "cannot calculate consensus: source evidence is missing or inconsistent for the report period"))
		c.JSON(http.StatusOK, []ConsensusResult{})
		return
	}

	err := database.DB.Model(&models.Holding{}).
		Joins("JOIN gurus ON gurus.id = holdings.guru_id").
		Where("gurus.type = ? AND holdings.report_period = ?", cohort, reportPeriod).
		Select(`holdings.stock_symbol, holdings.stock_name,
			count(distinct holdings.guru_id) as count, avg(holdings.weight) as avg_weight,
			count(distinct case when lower(holdings.change) like '%add%' or holdings.change like '%加仓%' or lower(holdings.change) like '%new%' or holdings.change like '%新进%' then holdings.guru_id end) as add_count,
			count(distinct case when lower(holdings.change) like '%trim%' or lower(holdings.change) like '%sell%' or holdings.change like '%减持%' then holdings.guru_id end) as trim_count`).
		Group("holdings.stock_symbol, holdings.stock_name").
		Order("count desc, avg_weight desc").
		Find(&rawResults).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	results := make([]ConsensusResult, len(rawResults))
	for i, r := range rawResults {
		results[i] = ConsensusResult{
			Symbol:       r.StockSymbol,
			Name:         r.StockName,
			GuruCount:    r.Count,
			AvgWeight:    r.AvgWeight,
			AddCount:     r.AddCount,
			TrimCount:    r.TrimCount,
			ReportPeriod: reportPeriod,
			Source:       consensusSource,
		}
	}

	freshness := dataFreshnessMeta{Source: consensusSource, DataTime: reportPeriod, Refreshable: true}
	if len(results) == 0 {
		freshness.Stale = true
		freshness.StaleReason = "no consensus holdings for disclosure period"
	}
	setDataFreshness(c, freshness)
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
		GuruName     string  `json:"guruName"`
		FundName     string  `json:"fundName"`
		Value        string  `json:"value"`
		Shares       string  `json:"shares"`
		Change       string  `json:"change"`
		Weight       float64 `json:"weight"`
		AvatarCode   string  `json:"avatarCode"`
		ReportPeriod string  `json:"reportPeriod"`
		Source       string  `json:"source"`
		SourceAsOf   string  `json:"sourceAsOf"`
		SourceURL    string  `json:"sourceURL"`
		Stale        bool    `json:"stale"`
	}

	var holdings []models.Holding
	if err := database.DB.Model(&models.Holding{}).
		Joins("JOIN gurus ON gurus.id = holdings.guru_id").
		Where("holdings.stock_symbol = ? AND holdings.report_period = gurus.report_period", baseSymbol).
		Find(&holdings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	results := make([]HolderResult, 0, len(holdings))
	matchedGurus := make([]models.Guru, 0, len(holdings))
	for _, hol := range holdings {
		var guru models.Guru
		if err := database.DB.First(&guru, hol.GuruID).Error; err == nil {
			matchedGurus = append(matchedGurus, guru)
			results = append(results, HolderResult{
				GuruName:     guru.Name,
				FundName:     guru.FundName,
				Value:        hol.Value,
				Shares:       hol.Shares,
				Change:       hol.Change,
				Weight:       hol.Weight,
				AvatarCode:   guru.AvatarCode,
				ReportPeriod: guru.ReportPeriod,
				Source:       guru.Source,
				SourceAsOf:   guru.SourceAsOf,
				SourceURL:    guru.SourceURL,
				Stale:        guruDisclosureStale(guru),
			})
		}
	}

	setDataFreshness(c, whalesFreshnessForGurus(matchedGurus, "whales-db"))
	c.JSON(http.StatusOK, results)
}

// GetWhalesSyncStatus returns the background sync status
func (h *Handler) GetWhalesSyncStatus(c *gin.Context) {
	database.SyncMutex.Lock()
	defer database.SyncMutex.Unlock()

	freshness := dataFreshnessMeta{
		Source:      "whales-sync-status",
		DataTime:    whalesDataTime(true),
		Refreshable: true,
	}
	if freshness.DataTime == "" {
		freshness = missingWhalesFreshness("whales-sync-status", "sync has not completed")
	}
	setDataFreshness(c, freshness)
	c.JSON(http.StatusOK, gin.H{
		"syncing":          database.WhalesSyncing,
		"lastSync":         database.WhalesLastSync,
		"lastSuccessCount": database.WhalesLastSuccessCount,
		"syncStatus":       database.WhalesSyncStatus,
	})
}

// TriggerWhalesSync starts EDGAR / public-disclosure sync only (no stockgod upstream).
func (h *Handler) TriggerWhalesSync(c *gin.Context) {
	if !database.StartEDGARSync() {
		c.JSON(http.StatusConflict, gin.H{"success": false, "status": "already-running", "mode": "edgar-only"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"success": true, "status": "started", "mode": "edgar-only"})
}

func whalesFreshnessMeta(congress bool) dataFreshnessMeta {
	source := "whales-db"
	if congress {
		source = "congress-db"
	}
	dataTime := whalesDataTime(congress)
	if dataTime == "" {
		return missingWhalesFreshness(source, "missing disclosure data time")
	}
	return dataFreshnessMeta{Source: source, DataTime: dataTime, Refreshable: true}
}

// whalesSystemFreshnessMeta conservatively summarizes every Whales read domain.
// It is intentionally separate from /api/system/status so the status handler can
// consume current database evidence without hard-coding one Whales sub-source.
func whalesSystemFreshnessMeta() dataFreshnessMeta {
	if database.DB == nil {
		return missingWhalesFreshness("whales-db+congress-db", "whales database is unavailable")
	}

	var gurus []models.Guru
	if err := database.DB.Where("type != ?", "politician").Order("id asc").Find(&gurus).Error; err != nil {
		return missingWhalesFreshness("whales-db+congress-db", "cannot inspect guru disclosure freshness")
	}
	guruMeta := whalesFreshnessForGurus(gurus, "whales-db")
	congressMeta := congressDisclosureFreshnessMeta()

	sources := make([]string, 0, 4)
	seen := map[string]bool{}
	for _, guru := range gurus {
		source := strings.TrimSpace(guru.Source)
		if source != "" && !seen[source] {
			seen[source] = true
			sources = append(sources, source)
		}
	}
	if !seen[congressMeta.Source] {
		sources = append(sources, congressMeta.Source)
	}
	sort.Strings(sources)
	meta := dataFreshnessMeta{Source: strings.Join(sources, "+"), Refreshable: true}

	reasons := make([]string, 0, 2)
	if guruMeta.Stale {
		reasons = append(reasons, "gurus: "+guruMeta.StaleReason+" (dataTime="+guruMeta.DataTime+")")
	}
	if congressMeta.Stale {
		reasons = append(reasons, "congress: "+congressMeta.StaleReason+" (dataTime="+congressMeta.DataTime+")")
	}
	if len(reasons) > 0 {
		meta.DataTime = "unknown"
		meta.Stale = true
		meta.StaleReason = strings.Join(reasons, "; ")
		return meta
	}

	meta.DataTime = guruMeta.DataTime
	if meta.DataTime == "" || (congressMeta.DataTime != "" && congressMeta.DataTime < meta.DataTime) {
		meta.DataTime = congressMeta.DataTime
	}
	return meta
}

func congressDisclosureFreshnessMeta() dataFreshnessMeta {
	return congressDisclosureFreshnessAt(time.Now().UTC())
}

func congressDisclosureFreshnessAt(now time.Time) dataFreshnessMeta {
	var latest sql.NullString
	if err := database.DB.Model(&models.CongressTrade{}).Select("MAX(date)").Scan(&latest).Error; err != nil || !latest.Valid || strings.TrimSpace(latest.String) == "" {
		return missingWhalesFreshness("congress-db", "missing congressional disclosure data time")
	}
	dataTime := strings.TrimSpace(latest.String)
	meta := dataFreshnessMeta{Source: "congress-db", DataTime: dataTime, Refreshable: true}
	observedAt, ok := parseLooseDataTime(dataTime)
	if !ok {
		meta.Stale = true
		meta.StaleReason = "invalid congressional trade date"
		return meta
	}
	const maximumDisclosureLag = 60 * 24 * time.Hour
	age := now.Sub(observedAt)
	if age > maximumDisclosureLag {
		meta.Stale = true
		meta.StaleReason = fmt.Sprintf("latest congressional trade age %.0fd exceeds 60d", age.Hours()/24)
	}
	return meta
}

func whalesDataTime(congress bool) string {
	if congress && strings.TrimSpace(database.WhalesLastSync) != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", database.WhalesLastSync, time.Local); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
		return database.WhalesLastSync
	}
	return ""
}

func whaleCohort(value string) string {
	switch strings.TrimSpace(value) {
	case "us_gurus", "", "all":
		return "superinvestor"
	case "a_share_top":
		return "fund"
	case "private":
		return "private_fund"
	case "hot_money":
		return "hot_money"
	default:
		return strings.TrimSpace(value)
	}
}

func sourceForConsensus(cohort, reportPeriod string) (string, bool) {
	var sources []string
	if err := database.DB.Model(&models.Holding{}).
		Joins("JOIN gurus ON gurus.id = holdings.guru_id").
		Where("gurus.type = ? AND holdings.report_period = ?", cohort, reportPeriod).
		Distinct().Pluck("gurus.source", &sources).Error; err != nil || len(sources) != 1 || strings.TrimSpace(sources[0]) == "" {
		return "", false
	}
	return sources[0], true
}

func disclosureReportType(source string) string {
	if strings.EqualFold(strings.TrimSpace(source), "sec-edgar-13f") {
		return "SEC 13F"
	}
	return "持仓披露"
}

func consensusCountsForGuru(guru models.Guru, reportPeriod string) map[string]int {
	counts := map[string]int{}
	if strings.TrimSpace(guru.Type) == "" || strings.TrimSpace(reportPeriod) == "" || strings.TrimSpace(guru.Source) == "" {
		return counts
	}
	var rows []struct {
		StockSymbol string
		Count       int
	}
	_ = database.DB.Model(&models.Holding{}).
		Joins("JOIN gurus ON gurus.id = holdings.guru_id").
		Where("gurus.type = ? AND gurus.source = ? AND holdings.report_period = ?", guru.Type, guru.Source, reportPeriod).
		Select("holdings.stock_symbol, count(distinct holdings.guru_id) as count").
		Group("holdings.stock_symbol").Scan(&rows)
	for _, row := range rows {
		counts[strings.ToUpper(strings.TrimSpace(row.StockSymbol))] = row.Count
	}
	return counts
}

func filingForPeriod(guru models.Guru, reportPeriod string) models.GuruFiling {
	var filing models.GuruFiling
	if reportPeriod != "" {
		_ = database.DB.Where("guru_id = ? AND report_period = ?", guru.ID, reportPeriod).First(&filing).Error
	}
	if filing.ID == 0 && reportPeriod == guru.ReportPeriod {
		filing = models.GuruFiling{
			GuruID: guru.ID, ReportPeriod: guru.ReportPeriod, FilingDate: guru.FilingDate,
			Accession: guru.Accession, Source: guru.Source, SourceAsOf: guru.SourceAsOf,
			SourceURL: guru.SourceURL, SyncedAt: guru.SyncedAt,
		}
	}
	return filing
}

func whalesFreshnessForFiling(filing models.GuruFiling) dataFreshnessMeta {
	if filingDisclosureStale(filing) {
		return missingWhalesFreshness(sourceOrDefault(filing.Source, "whales-db"), "incomplete disclosure metadata")
	}
	return dataFreshnessMeta{
		Source: filing.Source, DataTime: filing.ReportPeriod,
		RefreshedAt: dataTimeOrEmpty(filing.SyncedAt), Refreshable: true,
	}
}

func filingDisclosureStale(filing models.GuruFiling) bool {
	if !strings.EqualFold(strings.TrimSpace(filing.Source), "sec-edgar-13f") {
		return true
	}
	return strings.TrimSpace(filing.ReportPeriod) == "" ||
		strings.TrimSpace(filing.FilingDate) == "" ||
		strings.TrimSpace(filing.Accession) == "" ||
		strings.TrimSpace(filing.SourceURL) == "" ||
		strings.TrimSpace(filing.Source) == "" || filing.SyncedAt.IsZero()
}

func sourceOrDefault(source, fallback string) string {
	if strings.TrimSpace(source) == "" {
		return fallback
	}
	return source
}

func whalesFreshnessForGurus(gurus []models.Guru, fallbackSource string) dataFreshnessMeta {
	latestPeriod := ""
	latestSourceAsOf := ""
	latestSourceAsOfTime := time.Time{}
	latestSync := time.Time{}
	source := ""
	incomplete := len(gurus) == 0
	for _, guru := range gurus {
		if guru.ReportPeriod > latestPeriod {
			latestPeriod = guru.ReportPeriod
		}
		if parsed, ok := parseGuruSourceAsOf(guru.SourceAsOf); ok && (latestSourceAsOfTime.IsZero() || parsed.After(latestSourceAsOfTime)) {
			latestSourceAsOfTime = parsed
			latestSourceAsOf = guru.SourceAsOf
		}
		if guru.SyncedAt.After(latestSync) {
			latestSync = guru.SyncedAt
		}
		if source == "" && guru.Source != "" {
			source = guru.Source
		}
		if guru.Type == "superinvestor" && guruDisclosureStale(guru) {
			incomplete = true
		}
	}
	if source == "" {
		source = fallbackSource
	}
	if latestPeriod == "" {
		meta := missingWhalesFreshness(source, "third-party snapshot; SEC filing report period is unavailable")
		if latestSourceAsOf != "" {
			meta.DataTime = latestSourceAsOf
		}
		return meta
	}
	if incomplete {
		return dataFreshnessMeta{
			Source: source, DataTime: latestPeriod, RefreshedAt: dataTimeOrEmpty(latestSync),
			Stale: true, StaleReason: "incomplete disclosure metadata", Refreshable: true,
		}
	}
	return dataFreshnessMeta{
		Source: source, DataTime: latestPeriod, RefreshedAt: dataTimeOrEmpty(latestSync), Refreshable: true,
	}
}

func parseGuruSourceAsOf(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	if parsed, ok := parseLooseDataTime(value); ok {
		return parsed, true
	}
	upper := strings.ToUpper(value)
	for _, format := range []struct {
		pattern string
		yearPos int
		qPos    int
	}{
		{pattern: `^([0-9]{4})[- ]?Q([1-4])$`, yearPos: 1, qPos: 2},
		{pattern: `^Q([1-4])[- ]?([0-9]{4})$`, yearPos: 2, qPos: 1},
	} {
		matches := regexp.MustCompile(format.pattern).FindStringSubmatch(upper)
		if len(matches) == 3 {
			year, yearErr := strconv.Atoi(matches[format.yearPos])
			quarter, quarterErr := strconv.Atoi(matches[format.qPos])
			if yearErr == nil && quarterErr == nil {
				return time.Date(year, time.Month(quarter*3+1), 0, 0, 0, 0, 0, time.UTC), true
			}
		}
	}
	for _, layout := range []string{"2 Jan 2006", "Jan 2, 2006", "January 2, 2006", "2006/01/02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func guruDisclosureStale(guru models.Guru) bool {
	if !strings.EqualFold(strings.TrimSpace(guru.Source), "sec-edgar-13f") {
		return true
	}
	return strings.TrimSpace(guru.ReportPeriod) == "" ||
		strings.TrimSpace(guru.FilingDate) == "" ||
		strings.TrimSpace(guru.Accession) == "" ||
		strings.TrimSpace(guru.SourceURL) == "" ||
		strings.TrimSpace(guru.Source) == "" || guru.SyncedAt.IsZero()
}

func missingWhalesFreshness(source, reason string) dataFreshnessMeta {
	return dataFreshnessMeta{Source: source, DataTime: "unknown", Stale: true, StaleReason: reason, Refreshable: true}
}
