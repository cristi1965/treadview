package api

import (
	"net/http"
	"trading-agents/internal/database"
	"trading-agents/internal/models"

	"github.com/gin-gonic/gin"
)

// GetTrades fetches all trading logs sorted by CreatedAt descending
func (h *Handler) GetTrades(c *gin.Context) {
	var trades []models.Trade
	if err := database.DB.Order("created_at desc").Find(&trades).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, trades)
}

// CreateTrade adds a new trading record to SQLite
func (h *Handler) CreateTrade(c *gin.Context) {
	var trade models.Trade
	if err := c.ShouldBindJSON(&trade); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := database.DB.Create(&trade).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, trade)
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
