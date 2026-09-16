package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"trading-agents/internal/database"
	"trading-agents/internal/models"
)

func setupWhalesTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "whales.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Guru{}, &models.Holding{}, &models.GuruFiling{}, &models.CongressTrade{}); err != nil {
		t.Fatal(err)
	}
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous })
	return db
}

func performWhalesRequest(path string, handler func(*gin.Context)) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, path, nil)
	if id := context.Request.PathValue("id"); id != "" {
		context.Params = gin.Params{{Key: "id", Value: id}}
	}
	handler(context)
	return recorder
}

func TestGetGurusDoesNotAdvertiseLegacyHoldingsWithoutStoredRows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupWhalesTestDB(t)
	guru := models.Guru{
		Name: "Legacy", Slug: "legacy-list", Type: "superinvestor",
		PositionCount: 40, TopStock: "NVDA", TopStockWeight: 15,
	}
	if err := db.Create(&guru).Error; err != nil {
		t.Fatal(err)
	}

	recorder := performWhalesRequest("/api/whales/gurus", (&Handler{}).GetGurus)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var rows []struct {
		Holdings int `json:"holdings"`
		TopStock struct {
			Symbol string `json:"symbol"`
		} `json:"topStock"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Holdings != 0 || rows[0].TopStock.Symbol != "" {
		t.Fatalf("legacy metadata leaked as verified holdings: %+v", rows)
	}
}

func TestGetSmartMoneyConsensusUsesSameCohortAndReportPeriod(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupWhalesTestDB(t)
	gurus := []models.Guru{
		{Name: "A", Slug: "a", Type: "superinvestor", ReportPeriod: "2026-06-30", FilingDate: "2026-08-14", Source: "sec-edgar-13f"},
		{Name: "B", Slug: "b", Type: "superinvestor", ReportPeriod: "2026-06-30", FilingDate: "2026-08-14", Source: "sec-edgar-13f"},
		{Name: "Old", Slug: "old", Type: "superinvestor", ReportPeriod: "2026-03-31", Source: "sec-edgar-13f"},
		{Name: "Fund", Slug: "fund", Type: "fund", ReportPeriod: "2026-06-30"},
	}
	for i := range gurus {
		if err := db.Create(&gurus[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	holdings := []models.Holding{
		{GuruID: gurus[0].ID, ReportPeriod: "2026-06-30", StockSymbol: "NVDA", StockName: "NVIDIA", Weight: 20, Change: "+ Add"},
		{GuruID: gurus[1].ID, ReportPeriod: "2026-06-30", StockSymbol: "NVDA", StockName: "NVIDIA", Weight: 10, Change: "- Trim"},
		{GuruID: gurus[2].ID, ReportPeriod: "2026-03-31", StockSymbol: "NVDA", StockName: "NVIDIA", Weight: 99, Change: "+ Add"},
		{GuruID: gurus[3].ID, ReportPeriod: "2026-06-30", StockSymbol: "NVDA", StockName: "NVIDIA", Weight: 88, Change: "+ Add"},
	}
	if err := db.Create(&holdings).Error; err != nil {
		t.Fatal(err)
	}

	recorder := performWhalesRequest("/api/whales/consensus?type=us_gurus", (&Handler{}).GetSmartMoneyConsensus)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var rows []struct {
		GuruCount    int     `json:"guruCount"`
		AvgWeight    float64 `json:"avgWeight"`
		AddCount     int     `json:"addCount"`
		TrimCount    int     `json:"trimCount"`
		ReportPeriod string  `json:"reportPeriod"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].GuruCount != 2 || rows[0].AvgWeight != 15 || rows[0].AddCount != 1 || rows[0].TrimCount != 1 {
		t.Fatalf("unexpected consensus: %+v", rows)
	}
	if rows[0].ReportPeriod != "2026-06-30" || recorder.Header().Get("X-Data-Time") != "2026-06-30" {
		t.Fatalf("wrong disclosure period: row=%+v header=%q", rows[0], recorder.Header().Get("X-Data-Time"))
	}

	historical := performWhalesRequest("/api/whales/consensus?type=us_gurus&reportPeriod=2026-03-31", (&Handler{}).GetSmartMoneyConsensus)
	if historical.Code != http.StatusOK {
		t.Fatalf("historical status=%d body=%s", historical.Code, historical.Body.String())
	}
	if err := json.Unmarshal(historical.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].GuruCount != 1 || rows[0].ReportPeriod != "2026-03-31" {
		t.Fatalf("historical consensus mixed periods: %+v", rows)
	}
}

func TestGetSmartMoneyConsensusHandlesMissingReportPeriod(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupWhalesTestDB(t)

	recorder := performWhalesRequest("/api/whales/consensus?type=us_gurus", (&Handler{}).GetSmartMoneyConsensus)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "[]" {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("X-Data-Stale"); got != "true" {
		t.Fatalf("X-Data-Stale=%q", got)
	}
	if got := recorder.Header().Get("X-Data-Time"); got != "unknown" {
		t.Fatalf("X-Data-Time=%q", got)
	}
	if got := recorder.Header().Get("X-Data-Stale-Reason"); got == "" {
		t.Fatal("missing explanation for empty consensus")
	}
}

func TestGetSmartMoneyConsensusRejectsMixedOrMissingSourceEvidence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupWhalesTestDB(t)
	gurus := []models.Guru{
		{Name: "Verified", Slug: "verified", Type: "superinvestor", ReportPeriod: "2026-06-30", Source: "sec-edgar-13f"},
		{Name: "Unknown", Slug: "unknown", Type: "superinvestor", ReportPeriod: "2026-06-30"},
	}
	for i := range gurus {
		if err := db.Create(&gurus[i]).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&models.Holding{GuruID: gurus[i].ID, ReportPeriod: "2026-06-30", StockSymbol: "NVDA", Weight: 10}).Error; err != nil {
			t.Fatal(err)
		}
	}

	recorder := performWhalesRequest("/api/whales/consensus?type=us_gurus", (&Handler{}).GetSmartMoneyConsensus)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "[]" {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("X-Data-Stale-Reason"); got == "" {
		t.Fatal("missing explanation for inconsistent source evidence")
	}
}

func TestGetGuruDetailsUsesRealConsensusCount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupWhalesTestDB(t)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	gurus := []models.Guru{
		{Name: "A", Slug: "a", Type: "superinvestor", ReportPeriod: "2026-06-30", FilingDate: "2026-08-14", Accession: "abc", Source: "sec-edgar-13f", SourceURL: "https://www.sec.gov/example", SyncedAt: now},
		{Name: "B", Slug: "b", Type: "superinvestor", ReportPeriod: "2026-06-30", Source: "sec-edgar-13f"},
		{Name: "Fund", Slug: "fund", Type: "fund", ReportPeriod: "2026-06-30"},
	}
	for i := range gurus {
		if err := db.Create(&gurus[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&models.GuruFiling{
		GuruID: gurus[0].ID, ReportPeriod: "2026-06-30", FilingDate: "2026-08-14",
		Accession: "abc", Source: "sec-edgar-13f", SourceURL: "https://www.sec.gov/example", SyncedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	for _, guru := range gurus {
		change := ""
		if guru.Slug == "a" {
			change = "+ Add"
		}
		if err := db.Create(&models.Holding{GuruID: guru.ID, ReportPeriod: "2026-06-30", StockSymbol: "NVDA", StockName: "NVIDIA", Weight: 10, Change: change}).Error; err != nil {
			t.Fatal(err)
		}
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/whales/gurus/a", nil)
	context.Params = gin.Params{{Key: "id", Value: "a"}}
	(&Handler{}).GetGuruDetails(context)
	var detail struct {
		ReportPeriod string `json:"reportPeriod"`
		FilingDate   string `json:"filingDate"`
		ReportType   string `json:"reportType"`
		SourceURL    string `json:"sourceURL"`
		Stale        bool   `json:"stale"`
		NewPositions int    `json:"newPositions"`
		Holdings     []struct {
			ConsensusCount int    `json:"consensusCount"`
			Action         string `json:"action"`
		} `json:"holdings"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.ReportPeriod != "2026-06-30" || detail.FilingDate != "2026-08-14" || detail.ReportType != "SEC 13F" || detail.SourceURL != "https://www.sec.gov/example" || detail.Stale {
		t.Fatalf("unexpected metadata: %+v", detail)
	}
	if detail.NewPositions != 1 || len(detail.Holdings) != 1 || detail.Holdings[0].ConsensusCount != 2 || detail.Holdings[0].Action != "buy" {
		t.Fatalf("unexpected same-cohort consensus: %+v", detail.Holdings)
	}
}

func TestDisclosureReportTypeFollowsSource(t *testing.T) {
	if got := disclosureReportType("sec-edgar-13f"); got != "SEC 13F" {
		t.Fatalf("SEC report type=%q", got)
	}
	if got := disclosureReportType("cn-fund-db"); got != "持仓披露" {
		t.Fatalf("non-SEC report type=%q", got)
	}
}

func TestGetGuruDetailsCanReadHistoricalPeriodWithoutMixingCurrentHoldings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupWhalesTestDB(t)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	guru := models.Guru{
		Name: "A", Slug: "a-history", Type: "superinvestor", ReportPeriod: "2026-06-30",
		FilingDate: "2026-08-14", Accession: "new", Source: "sec-edgar-13f", SyncedAt: now,
	}
	if err := db.Create(&guru).Error; err != nil {
		t.Fatal(err)
	}
	filings := []models.GuruFiling{
		{GuruID: guru.ID, ReportPeriod: "2026-06-30", FilingDate: "2026-08-14", Accession: "new", Source: "sec-edgar-13f", SyncedAt: now},
		{GuruID: guru.ID, ReportPeriod: "2026-03-31", FilingDate: "2026-05-15", Accession: "old", Source: "sec-edgar-13f", SyncedAt: now.Add(-90 * 24 * time.Hour)},
	}
	if err := db.Create(&filings).Error; err != nil {
		t.Fatal(err)
	}
	rows := []models.Holding{
		{GuruID: guru.ID, ReportPeriod: "2026-06-30", StockSymbol: "NEW", Weight: 60},
		{GuruID: guru.ID, ReportPeriod: "2026-03-31", StockSymbol: "OLD", Weight: 40},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/whales/gurus/a-history?reportPeriod=2026-03-31", nil)
	context.Params = gin.Params{{Key: "id", Value: "a-history"}}
	(&Handler{}).GetGuruDetails(context)
	var detail struct {
		ReportPeriod     string   `json:"reportPeriod"`
		Accession        string   `json:"accession"`
		AvailablePeriods []string `json:"availablePeriods"`
		Holdings         []struct {
			Symbol string `json:"symbol"`
		} `json:"holdings"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.ReportPeriod != "2026-03-31" || detail.Accession != "old" || len(detail.Holdings) != 1 || detail.Holdings[0].Symbol != "OLD" {
		t.Fatalf("historical filing mixed with current data: %+v", detail)
	}
	if len(detail.AvailablePeriods) != 2 {
		t.Fatalf("available periods=%v", detail.AvailablePeriods)
	}
}

func TestGetGuruDetailsMarksLegacyDisclosureStale(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupWhalesTestDB(t)
	guru := models.Guru{Name: "Legacy", Slug: "legacy", Type: "superinvestor"}
	if err := db.Create(&guru).Error; err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/whales/gurus/legacy", nil)
	context.Params = gin.Params{{Key: "id", Value: "legacy"}}
	(&Handler{}).GetGuruDetails(context)
	var detail struct {
		Stale        bool   `json:"stale"`
		ReportPeriod string `json:"reportPeriod"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if !detail.Stale || detail.ReportPeriod != "" {
		t.Fatalf("legacy disclosure must be unknown/stale: %+v", detail)
	}
	if got := recorder.Header().Get("X-Data-Stale"); got != "true" {
		t.Fatalf("X-Data-Stale=%q", got)
	}
	if got := recorder.Header().Get("X-Data-Time"); got != "unknown" {
		t.Fatalf("X-Data-Time=%q", got)
	}
}

func TestGetWhalesSyncStatusMarksMissingSyncAsUnknown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousLastSync := database.WhalesLastSync
	database.WhalesLastSync = ""
	t.Cleanup(func() { database.WhalesLastSync = previousLastSync })

	recorder := performWhalesRequest("/api/whales/sync-status", (&Handler{}).GetWhalesSyncStatus)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("X-Data-Time"); got != "unknown" {
		t.Fatalf("X-Data-Time=%q", got)
	}
	if got := recorder.Header().Get("X-Data-Stale"); got != "true" {
		t.Fatalf("X-Data-Stale=%q", got)
	}
}

func TestWhalesSystemFreshnessAggregatesDynamicSourcesConservatively(t *testing.T) {
	db := setupWhalesTestDB(t)
	gurus := []models.Guru{
		{Name: "Third Party", Slug: "third-party", Type: "superinvestor", Source: "dataroma", SourceAsOf: "24 Apr 2026"},
		{Name: "Fund", Slug: "fund-source", Type: "fund", Source: "fund_report", SourceAsOf: "2025Q4"},
	}
	if err := db.Create(&gurus).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CongressTrade{
		Politician: "Representative", Party: "Independent", Symbol: "TEST", Type: "BUY", Date: "2026-08-01",
	}).Error; err != nil {
		t.Fatal(err)
	}

	meta := whalesSystemFreshnessMeta()
	if meta.Source != "congress-db+dataroma+fund_report" {
		t.Fatalf("aggregate source must follow database contents: %+v", meta)
	}
	if !meta.Stale || meta.DataTime != "unknown" || !strings.Contains(meta.StaleReason, "dataTime=24 Apr 2026") {
		t.Fatalf("missing filing period must degrade aggregate freshness: %+v", meta)
	}
}

func TestCongressDisclosureFreshnessRejectsOldTradeDates(t *testing.T) {
	db := setupWhalesTestDB(t)
	if err := db.Create(&models.CongressTrade{Politician: "Representative", Symbol: "TEST", Date: "2026-06-11"}).Error; err != nil {
		t.Fatal(err)
	}

	stale := congressDisclosureFreshnessAt(time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC))
	if !stale.Stale || stale.DataTime != "2026-06-11" || !strings.Contains(stale.StaleReason, "exceeds 60d") {
		t.Fatalf("old congressional disclosures were presented as fresh: %+v", stale)
	}

	fresh := congressDisclosureFreshnessAt(time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC))
	if fresh.Stale || fresh.DataTime != "2026-06-11" {
		t.Fatalf("recent congressional disclosure was rejected: %+v", fresh)
	}
}

func TestWhalesSystemFreshnessUsesOldestCompleteDisclosureDate(t *testing.T) {
	db := setupWhalesTestDB(t)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	if err := db.Create(&models.Guru{
		Name: "Verified", Slug: "verified-system", Type: "superinvestor", ReportPeriod: "2026-06-30",
		FilingDate: "2026-08-14", Accession: "accession", Source: "sec-edgar-13f",
		SourceURL: "https://www.sec.gov/example", SyncedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.CongressTrade{
		Politician: "Representative", Party: "Independent", Symbol: "TEST", Type: "BUY", Date: "2026-08-01",
	}).Error; err != nil {
		t.Fatal(err)
	}

	meta := whalesSystemFreshnessMeta()
	if meta.Stale || meta.Source != "congress-db+sec-edgar-13f" || meta.DataTime != "2026-06-30" {
		t.Fatalf("unexpected complete aggregate freshness: %+v", meta)
	}
}

func TestGuruSourceAsOfComparisonParsesDisplayFormats(t *testing.T) {
	for _, value := range []string{"2026-04-23", "24 Apr 2026", "Apr 25, 2026", "Q2 2026", "2026Q3", "2026/10/01"} {
		if _, ok := parseGuruSourceAsOf(value); !ok {
			t.Errorf("expected parsable sourceAsOf %q", value)
		}
	}
	gurus := []models.Guru{
		{Type: "fund", Source: "fund_report", SourceAsOf: "Q4 2025"},
		{Type: "fund", Source: "fund_report", SourceAsOf: "24 Apr 2026"},
		{Type: "fund", Source: "fund_report", SourceAsOf: "2026Q1"},
	}
	meta := whalesFreshnessForGurus(gurus, "whales-db")
	if meta.DataTime != "24 Apr 2026" {
		t.Fatalf("sourceAsOf must use chronological comparison, got %+v", meta)
	}
}

func TestGetCongressTradesPreservesDisclosureProvenance(t *testing.T) {
	db := setupWhalesTestDB(t)
	trade := models.CongressTrade{
		Politician: "Jane Doe", Party: "Democratic", District: "CA-01", Symbol: "TEST", Type: "BUY",
		Amount: "$1,001 - $15,000", Date: "2026-08-01", Source: "house-ptr", FilingDate: "2026-08-14",
		SourceURL: "https://disclosures-clerk.house.gov/public_disc/ptr-pdfs/2026/123.pdf", FilingID: "123",
	}
	if err := db.Create(&trade).Error; err != nil {
		t.Fatal(err)
	}
	recorder := performWhalesRequest("/api/whales/congress", (&Handler{}).GetCongressTrades)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var members []struct {
		LatestTrade struct {
			Source, FilingDate, SourceURL, FilingID string
			Verified                                bool
		} `json:"latestTrade"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &members); err != nil || len(members) != 1 {
		t.Fatalf("decode congress response: members=%+v err=%v", members, err)
	}
	latest := members[0].LatestTrade
	if latest.Source != trade.Source || latest.FilingDate != trade.FilingDate || latest.SourceURL != trade.SourceURL || latest.FilingID != trade.FilingID || !latest.Verified {
		t.Fatalf("congress provenance lost: %+v", latest)
	}
	if congressTradeVerified(models.CongressTrade{Source: "unknown", FilingDate: "unknown", SourceURL: "unknown", FilingID: "unknown"}) {
		t.Fatal("legacy bootstrap row must not be signal eligible")
	}
}
