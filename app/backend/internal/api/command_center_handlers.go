package api

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"trading-agents/internal/database"
	"trading-agents/internal/models"

	"github.com/gin-gonic/gin"
)

type journalTradeInput struct {
	Symbol        string  `json:"symbol"`
	Direction     string  `json:"direction"`
	EntryPrice    float64 `json:"entryPrice"`
	ExitPrice     float64 `json:"exitPrice"`
	Shares        float64 `json:"shares"`
	EmotionScore  int     `json:"emotionScore"`
	Notes         string  `json:"notes"`
	PaperOrderID  string  `json:"paperOrderId"`
	ResearchRunID string  `json:"researchRunId"`
}

func validateJournalTrade(input journalTradeInput) error {
	input.Symbol = strings.ToUpper(strings.TrimSpace(input.Symbol))
	if !safeTickerRe.MatchString(input.Symbol) {
		return fmt.Errorf("invalid symbol")
	}
	direction := strings.ToUpper(strings.TrimSpace(input.Direction))
	if direction != "BUY" && direction != "SELL" {
		return fmt.Errorf("direction must be BUY or SELL")
	}
	if input.EntryPrice <= 0 || input.ExitPrice <= 0 || math.IsNaN(input.EntryPrice) || math.IsNaN(input.ExitPrice) || math.IsInf(input.EntryPrice, 0) || math.IsInf(input.ExitPrice, 0) {
		return fmt.Errorf("entryPrice and exitPrice must be finite and positive")
	}
	if input.Shares <= 0 || math.IsNaN(input.Shares) || math.IsInf(input.Shares, 0) {
		return fmt.Errorf("shares must be finite and positive")
	}
	if input.EmotionScore < 0 || input.EmotionScore > 10 {
		return fmt.Errorf("emotionScore must be between 0 and 10")
	}
	if len([]rune(input.Notes)) > 4000 || len(input.PaperOrderID) > 80 || len(input.ResearchRunID) > 100 {
		return fmt.Errorf("journal text or reference is too long")
	}
	return nil
}

type paperTradeRecord struct {
	models.Trade
	Environment string `json:"environment"`
}

func asPaperTradeRecord(trade models.Trade) paperTradeRecord {
	return paperTradeRecord{Trade: trade, Environment: paperEnvironment}
}

// GetTrades fetches all trading logs sorted by CreatedAt descending
func (h *Handler) GetTrades(c *gin.Context) {
	var trades []models.Trade
	if err := database.DB.Order("created_at desc").Find(&trades).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	records := make([]paperTradeRecord, 0, len(trades))
	for _, trade := range trades {
		records = append(records, asPaperTradeRecord(trade))
	}
	c.JSON(http.StatusOK, records)
}

// CreateTrade adds a new trading record to SQLite
func (h *Handler) CreateTrade(c *gin.Context) {
	var input journalTradeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateJournalTrade(input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	input.Symbol = strings.ToUpper(strings.TrimSpace(input.Symbol))
	input.Direction = strings.ToUpper(strings.TrimSpace(input.Direction))
	pnl := (input.ExitPrice - input.EntryPrice) * input.Shares
	if input.Direction == "SELL" {
		pnl = -pnl
	}
	trade := models.Trade{
		Symbol: input.Symbol, Direction: input.Direction, EntryPrice: input.EntryPrice, ExitPrice: input.ExitPrice,
		Shares: input.Shares, Pnl: pnl, EmotionScore: input.EmotionScore, Notes: strings.TrimSpace(input.Notes),
		PaperOrderID: strings.TrimSpace(input.PaperOrderID), ResearchRunID: strings.TrimSpace(input.ResearchRunID),
		RequestID: c.Writer.Header().Get(requestIDHeader),
	}

	if err := database.DB.Create(&trade).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, asPaperTradeRecord(trade))
}

// GetStats calculates win rate and total profit/loss
func (h *Handler) GetStats(c *gin.Context) {
	var trades []models.Trade
	if err := database.DB.Find(&trades).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var totalPnL float64
	var wins float64
	var total float64

	for _, t := range trades {
		totalPnL += t.Pnl
		total++
		if t.Pnl > 0 {
			wins++
		}
	}

	winRate := 0.0
	if total > 0 {
		winRate = wins / total
	}

	c.JSON(http.StatusOK, gin.H{
		"environment": paperEnvironment,
		"totalPnL":    totalPnL,
		"winRate":     winRate,
		"totalTrades": total,
	})
}

// GetEvents fetches macroeconomic events sorted by date ascending
func (h *Handler) GetEvents(c *gin.Context) {
	var events []models.Event
	if err := database.DB.Order("date asc, time asc").Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, events)
}

// CreateEvent inserts a new macro event
func (h *Handler) CreateEvent(c *gin.Context) {
	var event models.Event
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := database.DB.Create(&event).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, event)
}
