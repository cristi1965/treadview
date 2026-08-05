package models

import "time"

type Trade struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	Symbol       string    `gorm:"size:20;not null" json:"symbol"`
	Direction    string    `gorm:"size:10;not null" json:"direction"` // BUY, SELL
	EntryPrice   float64   `gorm:"not null" json:"entryPrice"`
	ExitPrice    float64   `gorm:"not null" json:"exitPrice"`
	Shares       float64   `gorm:"not null" json:"shares"`
	Pnl          float64   `gorm:"not null" json:"pnl"`
	EmotionScore int       `gorm:"default:5" json:"emotionScore"`
	Notes        string    `gorm:"type:text" json:"notes"`
}

type Event struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	Date      string    `gorm:"size:20;not null" json:"date"`
	Time      string    `gorm:"size:10" json:"time"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	Stars     int       `gorm:"default:1" json:"stars"` // 1-3
	Previous  string    `gorm:"size:50" json:"previous"`
	Consensus string    `gorm:"size:50" json:"consensus"`
}
