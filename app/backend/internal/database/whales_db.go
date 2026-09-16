package database

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"trading-agents/internal/models"

	"gorm.io/gorm"
)

//go:embed investors.json
var investorsJSON []byte

//go:embed members.json
var membersJSON []byte

type RawHolding struct {
	Ticker         string   `json:"ticker"`
	StockName      string   `json:"stock_name"`
	PctOfPortfolio *float64 `json:"pct_of_portfolio"`
	ChangeType     string   `json:"change_type"`
	Period         string   `json:"period"`
	Source         string   `json:"source"`
}

type RawInvestor struct {
	Slug          string       `json:"slug"`
	Name          string       `json:"name"`
	NameEn        string       `json:"name_en"`
	Entity        string       `json:"entity"`
	Type          string       `json:"type"`
	Country       string       `json:"country"`
	LatestPeriod  string       `json:"latest_period"`
	HoldingsCount int          `json:"holdings_count"`
	Holdings      []RawHolding `json:"holdings"`
}

type RawTrade struct {
	Ticker     string `json:"ticker"`
	Side       string `json:"side"`
	Date       string `json:"date"`
	Size       string `json:"size"`
	Source     string `json:"source"`
	FilingDate string `json:"filingDate"`
	SourceURL  string `json:"sourceURL"`
	FilingID   string `json:"filingId"`
}

type RawMember struct {
	Name     string     `json:"name"`
	Party    string     `json:"party"`
	State    string     `json:"state"`
	District string     `json:"district"`
	Trades   []RawTrade `json:"trades"`
}

// Global sync state
var (
	SyncMutex              sync.Mutex
	WhalesSyncing          = false
	WhalesLastSync         = ""
	WhalesSyncStatus       = "Idle"
	WhalesLastSuccessCount = 0
)

// RestoreWhalesSyncStatus rebuilds durable success state from committed SEC
// disclosures. Volatile progress text may reset after a restart; completed
// filing metadata remains the authority.
func RestoreWhalesSyncStatus(db *gorm.DB) {
	if db == nil {
		return
	}
	query := db.Model(&models.Guru{}).
		Where("source = ? AND report_period <> '' AND filing_date <> '' AND accession <> '' AND source_url <> '' AND synced_at IS NOT NULL", "sec-edgar-13f")
	var count int64
	if err := query.Count(&count).Error; err != nil || count == 0 {
		return
	}
	var latest models.Guru
	if err := query.Order("synced_at DESC").First(&latest).Error; err != nil || latest.SyncedAt.IsZero() {
		return
	}
	SyncMutex.Lock()
	WhalesLastSuccessCount = int(count)
	WhalesLastSync = latest.SyncedAt.Local().Format("2006-01-02 15:04:05")
	WhalesSyncStatus = fmt.Sprintf("SEC EDGAR available: %d managers", count)
	SyncMutex.Unlock()
}

func SeedWhalesData(db *gorm.DB) {
	var rawInvestors []RawInvestor
	if err := json.Unmarshal(investorsJSON, &rawInvestors); err != nil {
		log.Printf("Failed to unmarshal investors: %v", err)
		return
	}

	// Seed Gurus if empty or if slug is missing (which requires a re-seed)
	var count int64
	db.Model(&models.Guru{}).Where("slug != ''").Count(&count)
	if count > 0 {
		backfillBootstrapSources(db, rawInvestors)
		return
	}

	log.Println("Seeding Whales bootstrap from embedded local investors/members datasets...")
	if err := db.Transaction(func(tx *gorm.DB) error {
		// Delete any old incomplete records to avoid duplicates.
		if err := tx.Exec("DELETE FROM holdings").Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM gurus").Error; err != nil {
			return err
		}

		for _, rawInv := range rawInvestors {
			// Calculate top stock & weight
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

			// Extract AvatarCode
			avatar := "G"
			parts := strings.Fields(rawInv.NameEn)
			if len(parts) > 0 {
				if len(parts) >= 2 {
					avatar = string(parts[0][0]) + string(parts[1][0])
				} else {
					avatar = string(parts[0][0])
					if len(parts[0]) > 1 {
						avatar += string(parts[0][1])
					}
				}
			} else if len(rawInv.Name) > 0 {
				// Chinese character avatar, take first rune
				runes := []rune(rawInv.Name)
				avatar = string(runes[0])
			}
			avatar = strings.ToUpper(avatar)

			aum := "—"
			if rawInv.Entity != "" {
				aum = rawInv.Entity
			}

			source := bootstrapSource(rawInv)
			guru := models.Guru{
				Name:           rawInv.Name,
				NameEn:         rawInv.NameEn,
				Slug:           rawInv.Slug,
				Title:          "Investor",
				FundName:       rawInv.Entity,
				AUM:            aum,
				PositionCount:  rawInv.HoldingsCount,
				TopStock:       topStock,
				TopStockWeight: topStockWeight,
				AvatarCode:     avatar,
				Type:           rawInv.Type,
				Source:         source,
				SourceAsOf:     strings.TrimSpace(rawInv.LatestPeriod),
				SourceURL:      bootstrapSourceURL(source),
			}

			if err := tx.Create(&guru).Error; err != nil {
				return err
			}

			// Insert Holdings
			for _, h := range rawInv.Holdings {
				weight := 0.0
				if h.PctOfPortfolio != nil {
					weight = *h.PctOfPortfolio
				}
				change := "Hold"
				if h.ChangeType == "add" {
					change = "+ 加仓"
				} else if h.ChangeType == "trim" {
					change = "- 减持"
				} else if h.ChangeType == "new" {
					change = "New 新进"
				}

				holding := models.Holding{
					GuruID:      guru.ID,
					StockSymbol: h.Ticker,
					StockName:   h.StockName,
					Value:       "—",
					Shares:      "—",
					Change:      change,
					Weight:      weight,
				}
				if err := tx.Create(&holding).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		log.Printf("Failed to seed whales bootstrap transaction: %v", err)
		return
	}

	// 3. Unmarshal members
	var rawMembers []RawMember
	if err := json.Unmarshal(membersJSON, &rawMembers); err != nil {
		log.Printf("Failed to unmarshal members: %v", err)
		return
	}

	// 4. Map and Insert CongressTrades
	for _, member := range rawMembers {
		party := "Democratic"
		if member.Party == "R" {
			party = "Republican"
		}

		title := "Representative"
		if len(member.District) > 0 && !strings.Contains(member.District, "0") && !strings.Contains(member.District, "1") && !strings.Contains(member.District, "2") && !strings.Contains(member.District, "3") && !strings.Contains(member.District, "4") && !strings.Contains(member.District, "5") && !strings.Contains(member.District, "6") && !strings.Contains(member.District, "7") && !strings.Contains(member.District, "8") && !strings.Contains(member.District, "9") {
			title = "Senator"
		}

		for _, trade := range member.Trades {
			side := "BUY"
			if strings.ToLower(trade.Side) == "sell" {
				side = "SELL"
			}

			amount := trade.Size
			if amount == "" {
				amount = "$1,000 - $15,000"
			}

			// Clean up size format e.g. "$$15K-$50K" to "$15K-$50K"
			amount = strings.Replace(amount, "$$", "$", -1)

			congressTrade := models.CongressTrade{
				Politician: member.Name,
				Title:      title,
				Party:      party,
				District:   member.State + " " + member.District,
				Symbol:     trade.Ticker,
				Type:       side,
				Amount:     amount,
				Date:       trade.Date,
				Source:     congressFieldOrUnknown(trade.Source),
				FilingDate: congressFieldOrUnknown(trade.FilingDate),
				SourceURL:  congressFieldOrUnknown(trade.SourceURL),
				FilingID:   congressFieldOrUnknown(trade.FilingID),
			}
			db.Create(&congressTrade)
		}
	}

	log.Printf("Successfully seeded %d gurus and congress trades from local bootstrap data!", len(rawInvestors))
}

func congressFieldOrUnknown(value string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return "unknown"
}

func congressSourceOrUnknown(rowSource, fileSource string) string {
	if strings.TrimSpace(rowSource) != "" {
		return congressFieldOrUnknown(rowSource)
	}
	return congressFieldOrUnknown(fileSource)
}

func bootstrapSource(rawInv RawInvestor) string {
	for _, holding := range rawInv.Holdings {
		if source := strings.ToLower(strings.TrimSpace(holding.Source)); source != "" {
			return source
		}
	}
	return "local-bootstrap"
}

func bootstrapSourceURL(source string) string {
	if strings.EqualFold(strings.TrimSpace(source), "dataroma") {
		return "https://www.dataroma.com/"
	}
	return ""
}

func backfillBootstrapSources(db *gorm.DB, rawInvestors []RawInvestor) {
	for _, rawInv := range rawInvestors {
		source := bootstrapSource(rawInv)
		updates := map[string]interface{}{
			"source":       source,
			"source_as_of": strings.TrimSpace(rawInv.LatestPeriod),
			"source_url":   bootstrapSourceURL(source),
		}
		// Only known embedded rows without a verified filing are bootstrap data.
		if err := db.Model(&models.Guru{}).
			Where("slug = ? AND report_period = '' AND accession = ''", rawInv.Slug).
			Updates(updates).Error; err != nil {
			log.Printf("Failed to backfill bootstrap source for %s: %v", rawInv.Slug, err)
		}
	}
}

// RunBackgroundSync is deprecated: stockgod upstream is disabled.
// Use RunEDGARSync for public 13F / disclosure updates.
func RunBackgroundSync() {
	log.Println("RunBackgroundSync skipped: stockgod upstream disabled; use RunEDGARSync")
	SyncMutex.Lock()
	WhalesSyncStatus = "stockgod sync disabled — use EDGAR"
	WhalesLastSync = time.Now().Format("2006-01-02 15:04:05")
	SyncMutex.Unlock()
}

func insertRawHolding(guruID uint, h RawHolding) {
	weight := 0.0
	if h.PctOfPortfolio != nil {
		weight = *h.PctOfPortfolio
	}
	change := "Hold"
	if h.ChangeType == "add" {
		change = "+ 加仓"
	} else if h.ChangeType == "trim" {
		change = "- 减持"
	} else if h.ChangeType == "new" {
		change = "New 新进"
	}

	holding := models.Holding{
		GuruID:      guruID,
		StockSymbol: h.Ticker,
		StockName:   h.StockName,
		Value:       "—",
		Shares:      "—",
		Change:      change,
		Weight:      weight,
	}
	DB.Create(&holding)
}
