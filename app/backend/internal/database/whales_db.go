package database

import (
	_ "embed"
	"encoding/json"
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
}

type RawInvestor struct {
	Slug          string       `json:"slug"`
	Name          string       `json:"name"`
	NameEn        string       `json:"name_en"`
	Entity        string       `json:"entity"`
	Type          string       `json:"type"`
	Country       string       `json:"country"`
	HoldingsCount int          `json:"holdings_count"`
	Holdings      []RawHolding `json:"holdings"`
}

type RawTrade struct {
	Ticker string `json:"ticker"`
	Side   string `json:"side"`
	Date   string `json:"date"`
	Size   string `json:"size"`
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
	SyncMutex        sync.Mutex
	WhalesSyncing    = false
	WhalesLastSync   = ""
	WhalesSyncStatus = "Idle"
)

func SeedWhalesData(db *gorm.DB) {
	// Seed Gurus if empty or if slug is missing (which requires a re-seed)
	var count int64
	db.Model(&models.Guru{}).Where("slug != ''").Count(&count)
	if count > 0 {
		return
	}

	log.Println("Seeding Whales bootstrap from embedded local investors/members datasets...")
	
	// Delete any old incomplete records to avoid duplicates
	db.Exec("DELETE FROM holdings")
	db.Exec("DELETE FROM gurus")

	// 1. Unmarshal investors
	var rawInvestors []RawInvestor
	if err := json.Unmarshal(investorsJSON, &rawInvestors); err != nil {
		log.Printf("Failed to unmarshal investors: %v", err)
		return
	}

	// 2. Map and Insert Gurus & Holdings
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
		}

		if err := db.Create(&guru).Error; err != nil {
			log.Printf("Failed to create guru %s: %v", guru.Name, err)
			continue
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
			db.Create(&holding)
		}
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
			}
			db.Create(&congressTrade)
		}
	}

	log.Printf("Successfully seeded %d gurus and congress trades from local bootstrap data!", len(rawInvestors))
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
