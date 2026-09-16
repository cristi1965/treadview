package models

import "time"

type Guru struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
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
	ReportPeriod   string    `gorm:"size:20;index" json:"reportPeriod"`
	FilingDate     string    `gorm:"size:20" json:"filingDate"`
	Accession      string    `gorm:"size:32" json:"accession"`
	Source         string    `gorm:"size:50" json:"source"`
	SourceAsOf     string    `gorm:"size:40" json:"sourceAsOf"`
	SourceURL      string    `gorm:"size:500" json:"sourceURL"`
	SyncedAt       time.Time `json:"syncedAt"`
}

type Holding struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time `json:"createdAt"`
	GuruID       uint      `gorm:"not null;index:idx_holding_guru_period_symbol,priority:1" json:"guruId"`
	ReportPeriod string    `gorm:"size:20;index:idx_holding_guru_period_symbol,priority:2" json:"reportPeriod"`
	StockSymbol  string    `gorm:"size:20;not null;index:idx_holding_guru_period_symbol,priority:3" json:"stockSymbol"`
	StockName    string    `gorm:"size:100" json:"stockName"`
	Value        string    `gorm:"size:50" json:"value"`
	Shares       string    `gorm:"size:50" json:"shares"`
	Change       string    `gorm:"size:20" json:"change"`
	Weight       float64   `json:"weight"`
}

type GuruFiling struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	GuruID       uint      `gorm:"not null;uniqueIndex:idx_guru_filing_period,priority:1" json:"guruId"`
	ReportPeriod string    `gorm:"size:20;not null;uniqueIndex:idx_guru_filing_period,priority:2" json:"reportPeriod"`
	FilingDate   string    `gorm:"size:20" json:"filingDate"`
	Accession    string    `gorm:"size:32" json:"accession"`
	Source       string    `gorm:"size:50" json:"source"`
	SourceAsOf   string    `gorm:"size:40" json:"sourceAsOf"`
	SourceURL    string    `gorm:"size:500" json:"sourceURL"`
	SyncedAt     time.Time `json:"syncedAt"`
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
	Source     string    `gorm:"size:80;not null;default:unknown" json:"source"`
	FilingDate string    `gorm:"size:20;not null;default:unknown" json:"filingDate"`
	SourceURL  string    `gorm:"size:500;not null;default:unknown" json:"sourceURL"`
	FilingID   string    `gorm:"size:100;not null;default:unknown" json:"filingId"`
}
