package database

import (
	"bytes"
	"database/sql"
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"trading-agents/internal/models"

	"gorm.io/gorm"
)

//go:embed superinvestor_ciks.json
var superinvestorCIKsJSON []byte

// ===================== SEC EDGAR 13F Structures =====================

type EDGARSubmissions struct {
	CIK     string `json:"cik"`
	Name    string `json:"name"`
	Filings struct {
		Recent struct {
			AccessionNumber []string `json:"accessionNumber"`
			FilingDate      []string `json:"filingDate"`
			ReportDate      []string `json:"reportDate"`
			Form            []string `json:"form"`
		} `json:"recent"`
	} `json:"filings"`
}

type InformationTable struct {
	XMLName   xml.Name    `xml:"informationTable"`
	InfoTable []InfoTable `xml:"infoTable"`
}

type InfoTable struct {
	NameOfIssuer string `xml:"nameOfIssuer"`
	CusipNum     string `xml:"cusip"`
	Value        int64  `xml:"value"` // thousands of USD in classic 13F; see valueToUSD
	ShrsOrPrnAmt struct {
		SshPrnamt     int64  `xml:"sshPrnamt"`
		SshPrnamtType string `xml:"sshPrnamtType"`
	} `xml:"shrsOrPrnAmt"`
}

// SuperinvestorEntry maps a guru to their SEC EDGAR CIK.
type SuperinvestorEntry struct {
	CIK      string `json:"cik"`
	DBName   string `json:"dbName"`
	FundName string `json:"fundName"`
	NameEn   string `json:"nameEn"`
}

type superinvestorCIKFile struct {
	Entries map[string]SuperinvestorEntry `json:"entries"`
}

// SuperinvestorCIKs is loaded from embedded JSON (slug → EDGAR data).
var SuperinvestorCIKs map[string]SuperinvestorEntry

func init() {
	SuperinvestorCIKs = loadSuperinvestorCIKs()
}

func loadSuperinvestorCIKs() map[string]SuperinvestorEntry {
	var file superinvestorCIKFile
	if err := json.Unmarshal(superinvestorCIKsJSON, &file); err != nil {
		log.Printf("[EDGAR] failed to parse superinvestor_ciks.json: %v", err)
		return map[string]SuperinvestorEntry{}
	}
	out := make(map[string]SuperinvestorEntry, len(file.Entries))
	for slug, e := range file.Entries {
		e.CIK = strings.TrimSpace(e.CIK)
		if len(e.CIK) > 0 && len(e.CIK) < 10 {
			e.CIK = fmt.Sprintf("%010s", e.CIK)
		}
		if e.CIK == "" {
			continue
		}
		out[slug] = e
	}
	log.Printf("[EDGAR] loaded %d superinvestor CIKs", len(out))
	return out
}

// ===================== EDGAR HTTP Client =====================

func edgarClient() *http.Client {
	return &http.Client{Timeout: 45 * time.Second}
}

func edgarRequest(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TradingAgents DataSync contact@tradingagents.app")
	req.Header.Set("Accept", "application/json,application/xml,text/xml,*/*")
	return edgarClient().Do(req)
}

// ===================== SEC EDGAR Sync Logic =====================

func fetchLatest13FAccession(cik string) (accession, filingDate, reportPeriod, dataFileURL string, err error) {
	url := fmt.Sprintf("https://data.sec.gov/submissions/CIK%s.json", cik)
	resp, err := edgarRequest(url)
	if err != nil {
		return "", "", "", "", fmt.Errorf("EDGAR submissions fetch error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", "", fmt.Errorf("EDGAR submissions HTTP %d for CIK %s", resp.StatusCode, cik)
	}

	var subs EDGARSubmissions
	if err := json.NewDecoder(resp.Body).Decode(&subs); err != nil {
		return "", "", "", "", fmt.Errorf("EDGAR submissions decode error: %w", err)
	}

	r := subs.Filings.Recent
	for i, form := range r.Form {
		if form != "13F-HR" {
			continue
		}
		accNum := stringAt(r.AccessionNumber, i)
		if accNum == "" {
			continue
		}
		filingDate := stringAt(r.FilingDate, i)
		reportPeriod := stringAt(r.ReportDate, i)
		accNoDash := strings.ReplaceAll(accNum, "-", "")
		cikInt := strings.TrimLeft(cik, "0")
		indexURL := fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s/%s/%s-index.htm",
			cikInt, accNoDash, accNum)

		xmlFile, err := findInfoTableXML(indexURL, cikInt, accNoDash)
		if err != nil {
			return accNum, filingDate, reportPeriod, "", err
		}
		return accNum, filingDate, reportPeriod, xmlFile, nil
	}
	return "", "", "", "", fmt.Errorf("no 13F-HR found for CIK %s", cik)
}

func stringAt(values []string, index int) string {
	if index < 0 || index >= len(values) {
		return ""
	}
	return strings.TrimSpace(values[index])
}

func findInfoTableXML(indexURL, cikInt, accNoDash string) (string, error) {
	resp, err := edgarRequest(indexURL)
	if err != nil {
		return "", fmt.Errorf("index fetch error: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	re := regexp.MustCompile(`/Archives/edgar/data/` + regexp.QuoteMeta(cikInt) + `/` + regexp.QuoteMeta(accNoDash) + `/[^"]+\.xml`)
	matches := re.FindAllString(content, -1)
	for _, m := range matches {
		lower := strings.ToLower(m)
		if strings.Contains(lower, "primary_doc") || strings.Contains(lower, "xslform") {
			continue
		}
		if strings.Contains(lower, "info") || strings.Contains(lower, "holding") || strings.Contains(lower, "form13f") {
			return "https://www.sec.gov" + m, nil
		}
	}
	for _, m := range matches {
		lower := strings.ToLower(m)
		if strings.Contains(lower, "primary_doc") || strings.Contains(lower, "xslform") {
			continue
		}
		return "https://www.sec.gov" + m, nil
	}
	return fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s/%s/infotable.xml", cikInt, accNoDash), nil
}

func parse13FXMLFromURL(xmlURL string) ([]InfoTable, error) {
	resp, err := edgarRequest(xmlURL)
	if err != nil {
		return nil, fmt.Errorf("XML fetch error: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	cleaned := regexp.MustCompile(`<(/?)ns\d+:`).ReplaceAllString(string(body), "<$1")

	var table InformationTable
	if err := xml.Unmarshal([]byte(cleaned), &table); err != nil {
		return nil, fmt.Errorf("XML unmarshal error: %w", err)
	}
	return table.InfoTable, nil
}

// valueToUSD converts a 13F value field to USD.
// Classic schema is thousands of USD; many 2023+ filings already report dollars.
// Heuristic: large books in dollars sum to >= ~1e10; in thousands they sum ~1e7–1e9.
func valueToUSD(values []int64) []float64 {
	var sum int64
	for _, v := range values {
		sum += v
	}
	mult := 1000.0
	if sum >= 10_000_000_000 { // already dollars (e.g. Berkshire ~2.6e11)
		mult = 1
	}
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = float64(v) * mult
	}
	return out
}

func formatUSD(v float64) string {
	switch {
	case v >= 1_000_000_000:
		return fmt.Sprintf("$%.1fB", v/1_000_000_000)
	case v >= 1_000_000:
		return fmt.Sprintf("$%.1fM", v/1_000_000)
	case v >= 1_000:
		return fmt.Sprintf("$%.1fK", v/1_000)
	default:
		return fmt.Sprintf("$%.0f", v)
	}
}

func changeLabel(prevShares, curShares int64, existed bool) string {
	if !existed {
		return "New 新进"
	}
	if curShares > prevShares {
		return "+ 加仓"
	}
	if curShares < prevShares {
		return "- 减持"
	}
	return "Hold"
}

// ===================== SyncFromEDGAR: Main Sync Function =====================

// RunEDGARSync pulls fresh 13F data from SEC EDGAR for all known CIKs.
// Congress trades stay on local seed (House/Senate PTR not wired); no stockgod.
func RunEDGARSync() {
	SyncMutex.Lock()
	if WhalesSyncing {
		SyncMutex.Unlock()
		return
	}
	WhalesSyncing = true
	WhalesSyncStatus = "Syncing with SEC EDGAR (13F)..."
	SyncMutex.Unlock()
	runClaimedEDGARSync()
}

// StartEDGARSync atomically claims the singleton job before returning.
func StartEDGARSync() bool {
	SyncMutex.Lock()
	if WhalesSyncing {
		SyncMutex.Unlock()
		return false
	}
	WhalesSyncing = true
	WhalesSyncStatus = "Syncing with SEC EDGAR (13F)..."
	SyncMutex.Unlock()
	go runClaimedEDGARSync()
	return true
}

func runClaimedEDGARSync() {
	successCount := 0

	defer func() {
		SyncMutex.Lock()
		WhalesSyncing = false
		if successCount > 0 {
			WhalesLastSync = time.Now().Format("2006-01-02 15:04:05")
			WhalesLastSuccessCount = successCount
			WhalesSyncStatus = fmt.Sprintf("SEC EDGAR complete: %d managers", successCount)
		} else {
			WhalesSyncStatus = "SEC EDGAR failed: no complete filing committed"
		}
		SyncMutex.Unlock()
	}()

	if len(SuperinvestorCIKs) == 0 {
		log.Println("[EDGAR] no CIK registry loaded — abort")
		return
	}

	cusipCache := loadCusipCache()
	// Stable order for logs / rate limits. Optional filter: EDGAR_ONLY_SLUGS=warren-buffett,bill-ackman
	only := map[string]bool{}
	if raw := strings.TrimSpace(os.Getenv("EDGAR_ONLY_SLUGS")); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				only[s] = true
			}
		}
	}
	slugs := make([]string, 0, len(SuperinvestorCIKs))
	for slug := range SuperinvestorCIKs {
		if len(only) > 0 && !only[slug] {
			continue
		}
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	if len(slugs) == 0 {
		log.Println("[EDGAR] no slugs selected — abort")
		return
	}
	log.Printf("[EDGAR] Starting SEC EDGAR 13F sync for %d managers...", len(slugs))

	for _, slug := range slugs {
		entry := SuperinvestorCIKs[slug]
		log.Printf("[EDGAR] Fetching 13F for %s (CIK: %s)...", slug, entry.CIK)

		SyncMutex.Lock()
		WhalesSyncStatus = fmt.Sprintf("EDGAR 13F: %s", slug)
		SyncMutex.Unlock()

		accession, filingDate, reportPeriod, xmlURL, err := fetchLatest13FAccession(entry.CIK)
		if err != nil {
			log.Printf("[EDGAR] Error fetching accession for %s: %v", slug, err)
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if strings.TrimSpace(reportPeriod) == "" {
			log.Printf("[EDGAR] Missing report period for %s accession %s; skipping unsafe sync", slug, accession)
			continue
		}
		if xmlURL == "" {
			log.Printf("[EDGAR] No XML URL found for %s", slug)
			continue
		}

		log.Printf("[EDGAR] %s -> accession=%s date=%s", slug, accession, filingDate)

		holdings, err := parse13FXMLFromURL(xmlURL)
		if err != nil {
			log.Printf("[EDGAR] Error parsing 13F XML for %s: %v", slug, err)
			continue
		}
		if len(holdings) == 0 {
			log.Printf("[EDGAR] No holdings found for %s", slug)
			continue
		}

		rawValues := make([]int64, len(holdings))
		for i, h := range holdings {
			rawValues[i] = h.Value
		}
		usdValues := valueToUSD(rawValues)

		var totalUSD float64
		for _, v := range usdValues {
			totalUSD += v
		}

		// Compare with the latest earlier filing while retaining every report period.
		var existingGuru models.Guru
		if err := DB.Where("slug = ?", slug).First(&existingGuru).Error; err != nil {
			_ = DB.Where("fund_name = ? OR name LIKE ?", entry.FundName, "%"+entry.DBName+"%").First(&existingGuru).Error
		}
		prev := map[string]int64{}
		var oldRows []models.Holding
		var previousPeriod sql.NullString
		if existingGuru.ID != 0 {
			_ = DB.Model(&models.Holding{}).
				Where("guru_id = ? AND report_period != '' AND report_period < ?", existingGuru.ID, reportPeriod).
				Select("MAX(report_period)").Scan(&previousPeriod).Error
			if previousPeriod.Valid && strings.TrimSpace(previousPeriod.String) != "" {
				DB.Where("guru_id = ? AND report_period = ?", existingGuru.ID, previousPeriod.String).Find(&oldRows)
			} else {
				// Rows created before report-period support are used once as the legacy baseline.
				DB.Where("guru_id = ? AND report_period = ''", existingGuru.ID).Find(&oldRows)
			}
		}
		for _, row := range oldRows {
			sym := strings.ToUpper(strings.TrimSpace(row.StockSymbol))
			shares, _ := strconv.ParseInt(strings.ReplaceAll(row.Shares, ",", ""), 10, 64)
			prev[sym] = shares
		}

		// Aggregate duplicate tickers (multi-CUSIP / share-class lines).
		type aggRow struct {
			ticker string
			name   string
			usd    float64
			shares int64
		}
		aggs := map[string]*aggRow{}
		order := []string{}
		for i, h := range holdings {
			ticker := resolveTicker(h.CusipNum, h.NameOfIssuer, cusipCache)
			sym := strings.ToUpper(strings.TrimSpace(ticker))
			if sym == "" {
				continue
			}
			if a, ok := aggs[sym]; ok {
				a.usd += usdValues[i]
				a.shares += h.ShrsOrPrnAmt.SshPrnamt
			} else {
				aggs[sym] = &aggRow{
					ticker: ticker,
					name:   titleCase(h.NameOfIssuer),
					usd:    usdValues[i],
					shares: h.ShrsOrPrnAmt.SshPrnamt,
				}
				order = append(order, sym)
			}
		}
		periodHoldings := make([]models.Holding, 0, len(order))
		for _, sym := range order {
			a := aggs[sym]
			weight := 0.0
			if totalUSD > 0 {
				weight = a.usd / totalUSD * 100.0
			}
			prevShares, existed := prev[sym]
			periodHoldings = append(periodHoldings, models.Holding{
				ReportPeriod: reportPeriod,
				StockSymbol:  a.ticker,
				StockName:    a.name,
				Value:        formatUSD(a.usd),
				Shares:       fmt.Sprintf("%d", a.shares),
				Change:       changeLabel(prevShares, a.shares, existed),
				Weight:       weight,
			})
		}
		_, err = saveEDGARDisclosure(DB, slug, entry, holdings, usdValues, totalUSD, accession, filingDate, reportPeriod, xmlURL, periodHoldings)
		if err != nil {
			log.Printf("[EDGAR] Failed committing %s disclosure for %s: %v", slug, reportPeriod, err)
			continue
		}

		successCount++
		log.Printf("[EDGAR] ✓ Updated %s (%s): %d holdings, AUM %s", slug, filingDate, len(holdings), formatUSD(totalUSD))
		time.Sleep(200 * time.Millisecond)
	}

	saveCusipCache(cusipCache)
	RunNonEDGARWhalesRefresh()
	RunCapitolTradesSync()
	log.Printf("[EDGAR] Sync complete: %d/%d gurus updated", successCount, len(slugs))
}

func saveEDGARDisclosure(db *gorm.DB, slug string, entry SuperinvestorEntry, holdings []InfoTable, usdValues []float64, totalUSD float64, accession, filingDate, reportPeriod, sourceURL string, periodHoldings []models.Holding) (models.Guru, error) {
	var committed models.Guru
	err := db.Transaction(func(tx *gorm.DB) error {
		guru, err := upsertGuruFromEDGARDB(tx, slug, entry, holdings, usdValues, totalUSD, accession, filingDate, reportPeriod, sourceURL)
		if err != nil {
			return err
		}
		for i := range periodHoldings {
			periodHoldings[i].GuruID = guru.ID
			periodHoldings[i].ReportPeriod = reportPeriod
		}
		if err := tx.Where("guru_id = ? AND report_period = ?", guru.ID, reportPeriod).Delete(&models.Holding{}).Error; err != nil {
			return err
		}
		if len(periodHoldings) == 0 {
			return fmt.Errorf("refusing to commit empty SEC filing %s", accession)
		}
		if err := tx.Create(&periodHoldings).Error; err != nil {
			return err
		}
		if err := upsertGuruFilingDB(tx, guru); err != nil {
			return err
		}
		committed = guru
		return nil
	})
	return committed, err
}

func upsertGuruFromEDGARDB(db *gorm.DB, slug string, entry SuperinvestorEntry, holdings []InfoTable, usdValues []float64, totalUSD float64, accession, filingDate, reportPeriod, sourceURL string) (models.Guru, error) {
	var topIdx int
	for i := range holdings {
		if usdValues[i] > usdValues[topIdx] {
			topIdx = i
		}
	}
	topTicker := resolveTicker(holdings[topIdx].CusipNum, holdings[topIdx].NameOfIssuer, nil)
	topWeight := 0.0
	if totalUSD > 0 {
		topWeight = usdValues[topIdx] / totalUSD * 100.0
	}

	var guru models.Guru
	err := db.Where("slug = ?", slug).First(&guru).Error
	if err != nil {
		// Fallback: fund name / Chinese name
		_ = db.Where("fund_name = ? OR name LIKE ?", entry.FundName, "%"+entry.DBName+"%").First(&guru).Error
	}

	nameEn := entry.NameEn
	if nameEn == "" {
		nameEn = slugToName(slug)
	}
	displayName := entry.DBName
	if displayName == "" {
		displayName = nameEn
	}

	avatar := "G"
	parts := strings.Fields(nameEn)
	if len(parts) >= 2 {
		avatar = string(parts[0][0]) + string(parts[1][0])
	} else if len(parts) == 1 && len(parts[0]) > 0 {
		avatar = string(parts[0][0])
	}

	if guru.ID == 0 {
		guru = models.Guru{
			Name:           displayName,
			NameEn:         nameEn,
			Slug:           slug,
			Title:          "Superinvestor",
			FundName:       entry.FundName,
			AUM:            formatUSD(totalUSD),
			PositionCount:  len(holdings),
			TopStock:       topTicker,
			TopStockWeight: topWeight,
			AvatarCode:     strings.ToUpper(avatar),
			Type:           "superinvestor",
			ReportPeriod:   reportPeriod,
			FilingDate:     filingDate,
			Accession:      accession,
			Source:         "sec-edgar-13f",
			SourceAsOf:     reportPeriod,
			SourceURL:      sourceURL,
			SyncedAt:       time.Now().UTC(),
		}
		if err := db.Create(&guru).Error; err != nil {
			return models.Guru{}, err
		}
		return guru, nil
	}
	// Persist the pre-update filing so an upgrade does not lose the last known period.
	if err := upsertGuruFilingDB(db, guru); err != nil {
		return models.Guru{}, err
	}

	guru.Slug = slug
	if guru.NameEn == "" {
		guru.NameEn = nameEn
	}
	if entry.FundName != "" {
		guru.FundName = entry.FundName
	}
	guru.AUM = formatUSD(totalUSD)
	guru.PositionCount = len(holdings)
	guru.TopStock = topTicker
	guru.TopStockWeight = topWeight
	if guru.Type == "" {
		guru.Type = "superinvestor"
	}
	guru.ReportPeriod = reportPeriod
	guru.FilingDate = filingDate
	guru.Accession = accession
	guru.Source = "sec-edgar-13f"
	guru.SourceAsOf = reportPeriod
	guru.SourceURL = sourceURL
	guru.SyncedAt = time.Now().UTC()
	if err := db.Save(&guru).Error; err != nil {
		return models.Guru{}, err
	}
	return guru, nil
}

func upsertGuruFilingDB(db *gorm.DB, guru models.Guru) error {
	if guru.ID == 0 || strings.TrimSpace(guru.ReportPeriod) == "" {
		return nil
	}
	filing := models.GuruFiling{
		GuruID: guru.ID, ReportPeriod: guru.ReportPeriod, FilingDate: guru.FilingDate,
		Accession: guru.Accession, Source: guru.Source, SourceAsOf: guru.SourceAsOf,
		SourceURL: guru.SourceURL, SyncedAt: guru.SyncedAt,
	}
	return db.Where("guru_id = ? AND report_period = ?", guru.ID, guru.ReportPeriod).
		Assign(filing).FirstOrCreate(&filing).Error
}

func backfillWhaleDisclosureHistory(db *gorm.DB) {
	var gurus []models.Guru
	if err := db.Where("report_period != ''").Find(&gurus).Error; err != nil {
		log.Printf("Whales disclosure history backfill skipped: %v", err)
		return
	}
	for _, guru := range gurus {
		if err := db.Model(&models.Holding{}).
			Where("guru_id = ? AND report_period = ''", guru.ID).
			Update("report_period", guru.ReportPeriod).Error; err != nil {
			log.Printf("Whales holding-period backfill failed for guru %d: %v", guru.ID, err)
			continue
		}
		if err := upsertGuruFilingDB(db, guru); err != nil {
			log.Printf("Whales filing-history backfill failed for guru %d: %v", guru.ID, err)
		}
	}
}

// ===================== Helpers =====================

func slugToName(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func titleCase(s string) string {
	words := strings.Fields(strings.ToLower(s))
	stopWords := map[string]bool{"inc": true, "llc": true, "lp": true, "ltd": true, "corp": true, "plc": true, "sa": true}
	result := make([]string, 0, len(words))
	for _, w := range words {
		if stopWords[w] {
			continue
		}
		if len(w) > 0 {
			result = append(result, strings.ToUpper(w[:1])+w[1:])
		}
	}
	return strings.Join(result, " ")
}

// ===================== CUSIP → Ticker =====================

var (
	cusipMu       sync.Mutex
	issuerTickers = map[string]string{
		"APPLE INC":              "AAPL",
		"MICROSOFT CORP":         "MSFT",
		"AMAZON COM INC":         "AMZN",
		"ALPHABET INC":           "GOOGL",
		"NVIDIA CORP":            "NVDA",
		"NVIDIA CORPORATION":     "NVDA",
		"META PLATFORMS INC":     "META",
		"TESLA INC":              "TSLA",
		"BERKSHIRE HATHAWAY INC": "BRK.B",
		"JPMORGAN CHASE":         "JPM",
		"VISA INC":               "V",
		"MASTERCARD INC":         "MA",
		"JOHNSON & JOHNSON":      "JNJ",
		"UNITEDHEALTH GROUP INC": "UNH",
		"WALMART INC":            "WMT",
		"EXXON MOBIL CORP":       "XOM",
		"BROADCOM INC":           "AVGO",
		"TAIWAN SEMICONDUCTOR":   "TSM",
		"COSTCO WHOLESALE CORP":  "COST",
		"HOME DEPOT INC":         "HD",
		"PROCTER AND GAMBLE":     "PG",
		"COCA COLA CO":           "KO",
		"BANK OF AMERICA CORP":   "BAC",
		"BANK AMERICA":           "BAC",
		"AMERICAN EXPRESS CO":    "AXP",
		"AMERICAN EXPRESS":       "AXP",
		"OCCIDENTAL PETE":        "OXY",
		"OCCIDENTAL PETROLEUM":   "OXY",
		"CHEVRON CORP":           "CVX",
		"CHEVRON CORPORATION":    "CVX",
	}
)

func cusipCachePath() string {
	candidates := []string{
		filepath.Join("data", "cusip-tickers.json"),
		filepath.Join("app", "backend", "data", "cusip-tickers.json"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(filepath.Dir(p)); err == nil {
			return p
		}
	}
	return candidates[0]
}

func loadCusipCache() map[string]string {
	path := cusipCachePath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	var m map[string]string
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return map[string]string{}
	}
	return m
}

func saveCusipCache(m map[string]string) {
	if len(m) == 0 {
		return
	}
	path := cusipCachePath()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	raw, _ := json.MarshalIndent(m, "", "  ")
	_ = os.WriteFile(path, raw, 0o644)
}

func resolveTicker(cusip, nameOfIssuer string, cache map[string]string) string {
	cusip = strings.ToUpper(strings.TrimSpace(cusip))
	if len(cusip) > 9 {
		cusip = cusip[:9]
	}

	if cache != nil {
		cusipMu.Lock()
		if t, ok := cache[cusip]; ok && t != "" {
			cusipMu.Unlock()
			return t
		}
		cusipMu.Unlock()
	}

	if t := openFIGILookup(cusip); t != "" {
		if cache != nil {
			cusipMu.Lock()
			cache[cusip] = t
			cusipMu.Unlock()
		}
		return t
	}

	upper := strings.ToUpper(strings.TrimSpace(nameOfIssuer))
	for name, ticker := range issuerTickers {
		if strings.Contains(upper, name) {
			if cache != nil && cusip != "" {
				cusipMu.Lock()
				cache[cusip] = ticker
				cusipMu.Unlock()
			}
			return ticker
		}
	}

	// Last resort: compact issuer token (better than empty)
	parts := strings.Fields(upper)
	if len(parts) > 0 {
		tok := regexp.MustCompile(`[^A-Z0-9]`).ReplaceAllString(parts[0], "")
		if len(tok) >= 2 && len(tok) <= 6 {
			return tok
		}
	}
	if cusip != "" {
		return cusip
	}
	return "—"
}

func openFIGILookup(cusip string) string {
	if cusip == "" || len(cusip) < 8 {
		return ""
	}
	payload := []map[string]string{{"idType": "ID_CUSIP", "idValue": cusip}}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.openfigi.com/v3/mapping", bytes.NewReader(body))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var result []struct {
		Data []struct {
			Ticker   string `json:"ticker"`
			ExchCode string `json:"exchCode"`
			Market   string `json:"marketSector"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result) == 0 {
		return ""
	}
	for _, row := range result[0].Data {
		if row.Ticker == "" {
			continue
		}
		ex := strings.ToUpper(row.ExchCode)
		if ex == "US" || ex == "UW" || ex == "UN" || ex == "UA" || ex == "" {
			return row.Ticker
		}
	}
	if len(result[0].Data) > 0 {
		return result[0].Data[0].Ticker
	}
	return ""
}
