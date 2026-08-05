package models

import "time"

type Guru struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	CreatedAt      time.Time `json:"createdAt"`
	Name           string    `gorm:"size:100;not null" json:"name"`
	NameEn         string    `gorm:"size:100" json:"nameEn"`
	Slug           string    `gorm:"size:100;uniqueIndex" json:"slug"`
	Title          string    `gorm:"size:100" json:"title"`
	FundName       string    `gorm:"size:100" json:"fundName"`
	AUM            string    `gorm:"size:50" json:"aum"`
	PositionCount  int       `json:"positionCount"`
	TopStock       string    `gorm:"size:20" json:"topStock"`
	TopStockWeight float64   `json:"topStockWeight"`
	AvatarCode     string    `gorm:"size:10" json:"avatarCode"`
	Type           string    `gorm:"size:50" json:"type"` // us_gurus, a_share_top
}

type Holding struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time `json:"createdAt"`
	GuruID      uint      `gorm:"not null" json:"guruId"`
	StockSymbol string    `gorm:"size:20;not null" json:"stockSymbol"`
	StockName   string    `gorm:"size:100" json:"stockName"`
	Value       string    `gorm:"size:50" json:"value"`
	Shares      string    `gorm:"size:50" json:"shares"`
	Change      string    `gorm:"size:20" json:"change"`
	Weight      float64   `json:"weight"`
}

type CongressTrade struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CreatedAt  time.Time `json:"createdAt"`
	Politician string    `gorm:"size:100;not null" json:"politician"`
	Title      string    `gorm:"size:100" json:"title"`
	Party      string    `gorm:"size:50;not null" json:"party"` // Democratic, Republican
	District   string    `gorm:"size:50" json:"district"`
	Symbol     string    `gorm:"size:20;not null" json:"symbol"`
	Type       string    `gorm:"size:20;not null" json:"type"` // BUY, SELL
	Amount     string    `gorm:"size:50" json:"amount"`
	Date       string    `gorm:"size:20;not null" json:"date"`
}
