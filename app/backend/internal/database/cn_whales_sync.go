package database

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"trading-agents/internal/models"

	"gorm.io/gorm"
)

// RunNonEDGARWhalesRefresh refreshes CN fund / hot_money / private_fund / politician
// from the embedded local corpus, then overlays hot_money seats with East Money 龙虎榜
// when the public API is reachable.
func RunNonEDGARWhalesRefresh() {
	log.Println("[CN-Whales] Refreshing non-EDGAR gurus from local corpus...")
	SyncMutex.Lock()
	WhalesSyncStatus = "Refreshing CN/local whales corpus..."
	SyncMutex.Unlock()

	var rawInvestors []RawInvestor
	if err := json.Unmarshal(investorsJSON, &rawInvestors); err != nil {
		log.Printf("[CN-Whales] parse investors embed: %v", err)
		return
	}

	updated := 0
	for _, rawInv := range rawInvestors {
		switch rawInv.Type {
		case "fund", "hot_money", "private_fund", "politician":
		default:
			continue
		}
		if refreshGuruFromRaw(rawInv) {
			updated++
		}
	}
	log.Printf("[CN-Whales] refreshed %d non-EDGAR gurus from corpus", updated)

	if n, err := refreshHotMoneyFromEastMoney(); err != nil {
		log.Printf("[CN-Whales] East Money hot_money overlay skipped: %v", err)
	} else {
		log.Printf("[CN-Whales] East Money hot_money overlay wrote %d seat holdings", n)
	}
}

func refreshGuruFromRaw(rawInv RawInvestor) bool {
	source := bootstrapSource(rawInv)
	topStock := "—"
	topStockWeight := 0.0
	for _, h := range rawInv.Holdings {
		w := 0.0
		if h.PctOfPortfolio != nil {
			w = *h.PctOfPortfolio
		}
		if w > topStockWeight {
			topStockWeight = w
			topStock = h.Ticker
		}
	}

	holdings := make([]models.Holding, 0, len(rawInv.Holdings))
	for _, h := range rawInv.Holdings {
		symbol := strings.TrimSpace(h.Ticker)
		if symbol == "" {
			continue
		}
		weight := 0.0
		if h.PctOfPortfolio != nil {
			weight = *h.PctOfPortfolio
		}
		change := "Hold"
		switch h.ChangeType {
		case "add":
			change = "+ 加仓"
		case "trim":
			change = "- 减持"
		case "new":
			change = "New 新进"
		}
		holdings = append(holdings, models.Holding{
			StockSymbol: symbol, StockName: h.StockName, Value: "—", Shares: "—", Change: change, Weight: weight,
		})
	}
	if len(holdings) == 0 {
		return false
	}

	var guru models.Guru
	err := DB.Where("slug = ?", rawInv.Slug).First(&guru).Error
	if err != nil {
		avatar := "G"
		parts := strings.Fields(rawInv.NameEn)
		if len(parts) >= 2 {
			avatar = string(parts[0][0]) + string(parts[1][0])
		} else if len(rawInv.Name) > 0 {
			runes := []rune(rawInv.Name)
			avatar = string(runes[0])
		}
		guru = models.Guru{
			Name:           rawInv.Name,
			NameEn:         rawInv.NameEn,
			Slug:           rawInv.Slug,
			Title:          "Investor",
			FundName:       rawInv.Entity,
			AUM:            rawInv.Entity,
			PositionCount:  len(holdings),
			TopStock:       topStock,
			TopStockWeight: topStockWeight,
			AvatarCode:     strings.ToUpper(avatar),
			Type:           rawInv.Type,
			Source:         source,
			SourceAsOf:     strings.TrimSpace(rawInv.LatestPeriod),
			SourceURL:      bootstrapSourceURL(source),
		}
	} else {
		guru.PositionCount = len(holdings)
		guru.TopStock = topStock
		guru.TopStockWeight = topStockWeight
		if rawInv.Entity != "" {
			guru.FundName = rawInv.Entity
		}
		guru.Type = rawInv.Type
		guru.Source = source
		guru.SourceAsOf = strings.TrimSpace(rawInv.LatestPeriod)
		guru.SourceURL = bootstrapSourceURL(source)
	}
	if err := replaceGuruHoldings(DB, &guru, holdings); err != nil {
		log.Printf("[CN-Whales] atomic replacement failed for %s: %v", rawInv.Slug, err)
		return false
	}
	return true
}

func replaceGuruHoldings(db *gorm.DB, guru *models.Guru, holdings []models.Holding) error {
	if guru == nil || strings.TrimSpace(guru.Slug) == "" || len(holdings) == 0 {
		return fmt.Errorf("refusing empty guru holdings replacement")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(guru).Error; err != nil {
			return err
		}
		for i := range holdings {
			holdings[i].GuruID = guru.ID
		}
		if err := tx.Where("guru_id = ?", guru.ID).Delete(&models.Holding{}).Error; err != nil {
			return err
		}
		return tx.Create(&holdings).Error
	})
}

type eastMoneyResp struct {
	Result struct {
		Data []struct {
			SecurityCode   string  `json:"SECURITY_CODE"`
			SecurityName   string  `json:"SECURITY_NAME_ABBR"`
			TradeDate      string  `json:"TRADE_DATE"`
			OperatDeptName string  `json:"OPERATEDEPT_NAME"`
			Buy            float64 `json:"BUY"`
			Sell           float64 `json:"SELL"`
			Net            float64 `json:"NET"`
		} `json:"data"`
	} `json:"result"`
	Success bool `json:"success"`
}

// refreshHotMoneyFromEastMoney overlays the first hot_money guru seats with latest
// public 龙虎榜 department net-buy rows (best-effort; corpus remains fallback).
func refreshHotMoneyFromEastMoney() (int, error) {
	u := url.URL{
		Scheme: "https",
		Host:   "datacenter-web.eastmoney.com",
		Path:   "/api/data/v1/get",
	}
	q := u.Query()
	q.Set("sortColumns", "NET,TRADE_DATE")
	q.Set("sortTypes", "-1,-1")
	q.Set("pageSize", "100")
	q.Set("pageNumber", "1")
	q.Set("reportName", "RPT_BILLBOARD_DAILYDETAILS")
	q.Set("columns", "ALL")
	q.Set("source", "WEB")
	q.Set("client", "WEB")
	u.RawQuery = q.Encode()

	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 TradingAgents DataSync")
	req.Header.Set("Referer", "https://data.eastmoney.com/")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("eastmoney HTTP %d", resp.StatusCode)
	}

	var parsed eastMoneyResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		// Try alternate report name
		return refreshHotMoneyAlt()
	}
	if !parsed.Success || len(parsed.Result.Data) == 0 {
		return refreshHotMoneyAlt()
	}

	return applyHotMoneyRows(parsed)
}

func refreshHotMoneyAlt() (int, error) {
	// Fallback endpoint used by many open-source scrapers
	endpoint := "https://datacenter-web.eastmoney.com/api/data/v1/get?sortColumns=BILLBOARD_NET_AMT,TRADE_DATE,SECURITY_CODE&sortTypes=-1,-1,-1&pageSize=80&pageNumber=1&reportName=RPT_DAILYBILLBOARD_DETAILS&columns=ALL&source=WEB&client=WEB"
	client := &http.Client{Timeout: 20 * time.Second}
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 TradingAgents DataSync")
	req.Header.Set("Referer", "https://data.eastmoney.com/")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("eastmoney alt HTTP %d", resp.StatusCode)
	}

	var generic map[string]any
	if err := json.Unmarshal(body, &generic); err != nil {
		return 0, err
	}
	ok, _ := generic["success"].(bool)
	if !ok {
		return 0, fmt.Errorf("eastmoney alt success=false")
	}
	result, _ := generic["result"].(map[string]any)
	data, _ := result["data"].([]any)
	if len(data) == 0 {
		return 0, fmt.Errorf("eastmoney alt empty")
	}

	holdings := make([]models.Holding, 0, 40)
	for _, row := range data {
		m, _ := row.(map[string]any)
		code, _ := m["SECURITY_CODE"].(string)
		name, _ := m["SECURITY_NAME_ABBR"].(string)
		if code == "" {
			continue
		}
		net, _ := m["BILLBOARD_NET_AMT"].(float64)
		change := "+ 加仓"
		if net < 0 {
			change = "- 减持"
		}
		holdings = append(holdings, models.Holding{StockSymbol: code, StockName: name, Value: fmt.Sprintf("%.0f", net), Shares: "—", Change: change})
		if len(holdings) >= 40 {
			break
		}
	}
	if len(holdings) == 0 {
		return 0, fmt.Errorf("eastmoney alt contains no valid holdings")
	}

	// Map into a synthetic hot_money guru "lhb-live".
	var guru models.Guru
	if err := DB.Where("slug = ?", "lhb-live").First(&guru).Error; err != nil {
		guru = models.Guru{
			Name:       "龙虎榜席位（实时）",
			NameEn:     "LHB Live",
			Slug:       "lhb-live",
			Title:      "Hot Money",
			FundName:   "东方财富龙虎榜",
			Type:       "hot_money",
			AvatarCode: "LH",
		}
	}
	guru.PositionCount = len(holdings)
	guru.TopStock = holdings[0].StockSymbol
	if err := replaceGuruHoldings(DB, &guru, holdings); err != nil {
		return 0, err
	}
	return len(holdings), nil
}

func applyHotMoneyRows(parsed eastMoneyResp) (int, error) {
	holdings := make([]models.Holding, 0, 40)
	for _, row := range parsed.Result.Data {
		if row.SecurityCode == "" {
			continue
		}
		change := "+ 加仓"
		if row.Net < 0 {
			change = "- 减持"
		}
		holdings = append(holdings, models.Holding{
			StockSymbol: row.SecurityCode, StockName: row.SecurityName, Value: fmt.Sprintf("%.0f", row.Net), Shares: "—", Change: change,
		})
		if len(holdings) >= 40 {
			break
		}
	}
	if len(holdings) == 0 {
		return 0, fmt.Errorf("eastmoney response contains no valid holdings")
	}

	var guru models.Guru
	if err := DB.Where("slug = ?", "lhb-live").First(&guru).Error; err != nil {
		guru = models.Guru{
			Name:       "龙虎榜席位（实时）",
			NameEn:     "LHB Live",
			Slug:       "lhb-live",
			Title:      "Hot Money",
			FundName:   "东方财富龙虎榜",
			Type:       "hot_money",
			AvatarCode: "LH",
		}
	}
	guru.PositionCount = len(holdings)
	guru.TopStock = holdings[0].StockSymbol
	if err := replaceGuruHoldings(DB, &guru, holdings); err != nil {
		return 0, err
	}
	return len(holdings), nil
}
