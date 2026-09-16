package models

import "time"

type Trade struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	Symbol        string    `gorm:"size:20;not null" json:"symbol"`
	Direction     string    `gorm:"size:10;not null" json:"direction"` // BUY, SELL
	EntryPrice    float64   `gorm:"not null" json:"entryPrice"`
	ExitPrice     float64   `gorm:"not null" json:"exitPrice"`
	Shares        float64   `gorm:"not null" json:"shares"`
	Pnl           float64   `gorm:"not null" json:"pnl"`
	EmotionScore  int       `gorm:"default:5" json:"emotionScore"`
	Notes         string    `gorm:"type:text" json:"notes"`
	PaperOrderID  string    `gorm:"size:80;index" json:"paperOrderId,omitempty"`
	ResearchRunID string    `gorm:"size:100;index" json:"researchRunId,omitempty"`
	RequestID     string    `gorm:"size:80;uniqueIndex" json:"-"`
}

type PaperOrderStatusEvent struct {
	Status string    `json:"status"`
	At     time.Time `json:"at"`
	Reason string    `json:"reason,omitempty"`
}

type PaperPolicyCheck struct {
	PolicyVersion string    `json:"policyVersion"`
	Reasons       []string  `json:"reasons,omitempty"`
	CheckedAt     time.Time `json:"checkedAt"`
	Passed        bool      `json:"passed"`
}

type PaperRiskSnapshot struct {
	PolicyVersion             string            `json:"policyVersion"`
	PolicyReasons             []string          `json:"policyReasons,omitempty"`
	InvestableCapital         float64           `json:"investableCapital"`
	MaxLoss                   float64           `json:"maxLoss"`
	MaxRiskAmount             float64           `json:"maxRiskAmount"`
	PlannedNotional           float64           `json:"plannedNotional"`
	MaxNotionalAmount         float64           `json:"maxNotionalAmount"`
	PortfolioGrossExposure    float64           `json:"portfolioGrossExposure"`
	PortfolioGrossExposurePct float64           `json:"portfolioGrossExposurePct"`
	MaxPortfolioExposurePct   float64           `json:"maxPortfolioExposurePct"`
	SymbolExposurePct         float64           `json:"symbolExposurePct"`
	MaxSymbolExposurePct      float64           `json:"maxSymbolExposurePct"`
	Sector                    string            `json:"sector"`
	SectorExposurePct         float64           `json:"sectorExposurePct"`
	MaxSectorExposurePct      float64           `json:"maxSectorExposurePct"`
	OpenOrderMaxLoss          float64           `json:"openOrderMaxLoss"`
	OpenOrderMaxLossPct       float64           `json:"openOrderMaxLossPct"`
	MaxOpenOrderLossPct       float64           `json:"maxOpenOrderLossPct"`
	AverageDailyVolume        *float64          `json:"averageDailyVolume,omitempty"`
	PlannedADVPercent         *float64          `json:"plannedADVPercent,omitempty"`
	MaxOrderADVPercent        float64           `json:"maxOrderADVPercent"`
	LiquiditySource           string            `json:"liquiditySource,omitempty"`
	LiquidityComplete         bool              `json:"liquidityComplete"`
	PortfolioDegraded         bool              `json:"portfolioDegraded"`
	PortfolioOffendingSymbols []string          `json:"portfolioOffendingSymbols,omitempty"`
	RiskWarnings              []string          `json:"riskWarnings,omitempty"`
	StopCoveragePct           float64           `json:"stopCoveragePct"`
	MinStopCoveragePct        float64           `json:"minStopCoveragePct"`
	DailyLossPct              float64           `json:"dailyLossPct"`
	MaxDailyLossPct           float64           `json:"maxDailyLossPct"`
	DrawdownPct               float64           `json:"drawdownPct"`
	MaxDrawdownPct            float64           `json:"maxDrawdownPct"`
	StressLossPct             float64           `json:"stressLossPct"`
	MaxStressLossPct          float64           `json:"maxStressLossPct"`
	RiskLimitPassed           bool              `json:"riskLimitPassed"`
	SubmissionPolicyCheck     *PaperPolicyCheck `json:"submissionPolicyCheck,omitempty"`
	FillPolicyCheck           *PaperPolicyCheck `json:"fillPolicyCheck,omitempty"`
}

// PaperFillQuote is the provider observation used to price a simulated fill.
// Keeping the raw observation separate from FillPrice makes slippage and fill
// decisions independently reproducible without implying a broker execution.
type PaperFillQuote struct {
	Price       float64   `json:"price"`
	Source      string    `json:"source"`
	ObservedAt  time.Time `json:"observedAt"`
	ProviderURL string    `json:"providerURL,omitempty"`
}

// PaperFill is an append-only execution fact for one Paper order. Order-level
// fill fields remain cumulative projections for backwards-compatible clients.
type PaperFill struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time      `gorm:"index;not null" json:"createdAt"`
	FillID        string         `gorm:"size:100;uniqueIndex;not null" json:"fillId"`
	PaperOrderID  uint           `gorm:"index;not null;uniqueIndex:idx_paper_fill_sequence" json:"paperOrderId"`
	Sequence      int            `gorm:"not null;uniqueIndex:idx_paper_fill_sequence" json:"sequence"`
	Quantity      int            `gorm:"not null" json:"quantity"`
	Price         float64        `gorm:"not null" json:"price"`
	Quote         PaperFillQuote `gorm:"serializer:json;type:text;not null" json:"quote"`
	Slippage      float64        `gorm:"not null;default:0" json:"slippage"`
	Fee           float64        `gorm:"not null;default:0" json:"fee"`
	RealizedPnL   float64        `gorm:"not null;default:0" json:"realizedPnL"`
	PolicyVersion string         `gorm:"size:80" json:"policyVersion,omitempty"`
	PolicyReasons []string       `gorm:"serializer:json;type:text" json:"policyReasons,omitempty"`
}

func (PaperFill) TableName() string { return "paper_fills" }

// PaperOrder is an auditable simulation record. It never represents a broker order.
type PaperOrder struct {
	ID                     uint                    `gorm:"primaryKey" json:"id"`
	CreatedAt              time.Time               `json:"createdAt"`
	UpdatedAt              time.Time               `json:"updatedAt"`
	ClientOrderID          string                  `gorm:"size:80;uniqueIndex;not null" json:"clientOrderId"`
	ResearchRunID          string                  `gorm:"size:100;index" json:"researchRunId,omitempty"`
	ResearchTicker         string                  `gorm:"size:20" json:"researchTicker,omitempty"`
	Environment            string                  `gorm:"size:10;not null" json:"environment"`
	Symbol                 string                  `gorm:"size:20;not null" json:"symbol"`
	Side                   string                  `gorm:"size:8;not null" json:"side"`
	Market                 string                  `gorm:"size:8;not null" json:"market"`
	Currency               string                  `gorm:"size:8;not null" json:"currency"`
	OrderType              string                  `gorm:"size:20;not null" json:"orderType"`
	TimeInForce            string                  `gorm:"size:8;not null" json:"timeInForce"`
	ReferencePrice         float64                 `json:"referencePrice"`
	Entry                  *float64                `json:"entry"`
	TriggerPrice           *float64                `json:"triggerPrice,omitempty"`
	ProtectiveStop         *float64                `json:"protectiveStop,omitempty"`
	TakeProfit             *float64                `json:"takeProfit,omitempty"`
	ParentOrderID          *uint                   `gorm:"index" json:"parentOrderId,omitempty"`
	OCOGroupID             string                  `gorm:"size:80;index" json:"ocoGroupId,omitempty"`
	ProtectionRemainingQty int                     `gorm:"not null;default:0" json:"protectionRemainingQty,omitempty"`
	ProtectionInitialized  bool                    `gorm:"not null;default:false" json:"protectionInitialized,omitempty"`
	Quantity               int                     `json:"quantity"`
	QuoteSource            string                  `gorm:"size:120;not null" json:"quoteSource"`
	QuoteTime              time.Time               `json:"quoteTime"`
	RiskSnapshot           PaperRiskSnapshot       `gorm:"serializer:json;type:text" json:"riskSnapshot"`
	Status                 string                  `gorm:"size:24;index;not null" json:"status"`
	StatusHistory          []PaperOrderStatusEvent `gorm:"serializer:json;type:text" json:"statusHistory"`
	RejectionReason        string                  `gorm:"type:text" json:"rejectionReason,omitempty"`
	RequestHash            string                  `gorm:"size:64;not null" json:"-"`
	ReservedCash           float64                 `gorm:"not null;default:0" json:"reservedCash,omitempty"`
	ReservedQty            int                     `gorm:"not null;default:0" json:"reservedQty,omitempty"`
	FillPrice              float64                 `gorm:"not null;default:0" json:"fillPrice,omitempty"`
	FillQty                int                     `gorm:"not null;default:0" json:"fillQty,omitempty"`
	RemainingQty           int                     `gorm:"not null;default:0" json:"remainingQty"`
	Slippage               float64                 `gorm:"not null;default:0" json:"slippage,omitempty"`
	Fee                    float64                 `gorm:"not null;default:0" json:"fee,omitempty"`
	RealizedPnL            float64                 `gorm:"not null;default:0" json:"realizedPnL"`
	FilledAt               *time.Time              `json:"filledAt,omitempty"`
	FillModel              string                  `gorm:"size:120" json:"fillModel,omitempty"`
	FillQuote              *PaperFillQuote         `gorm:"serializer:json;type:text" json:"fillQuote,omitempty"`
	Fills                  []PaperFill             `gorm:"-" json:"fills,omitempty"`
	Version                uint64                  `gorm:"not null;default:1" json:"version"`
}

// PaperDailyEquityBaseline is the first complete authoritative equity observed
// by the Paper scanner during the currency account's regular market session.
// The unique currency/date key and create-only workflow make it immutable.
type PaperDailyEquityBaseline struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Currency   string    `gorm:"size:8;not null;uniqueIndex:idx_paper_daily_equity" json:"currency"`
	MarketDate string    `gorm:"size:10;not null;uniqueIndex:idx_paper_daily_equity" json:"marketDate"`
	ObservedAt time.Time `gorm:"index;not null" json:"observedAt"`
	Equity     float64   `gorm:"not null" json:"equity"`
}

// PaperEquityCheckpoint preserves a server-computed equity observation used to
// derive peak-to-current drawdown. A checkpoint is written only from a complete
// authoritative portfolio valuation.
type PaperEquityCheckpoint struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ObservedAt time.Time `gorm:"index;not null" json:"observedAt"`
	Currency   string    `gorm:"size:8;index;not null" json:"currency"`
	Equity     float64   `gorm:"not null" json:"equity"`
}

// PaperAccount is the server-owned cash ledger for one paper currency.
type PaperAccount struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	Currency     string    `gorm:"size:8;uniqueIndex;not null" json:"currency"`
	InitialCash  float64   `gorm:"not null" json:"initialCash"`
	Cash         float64   `gorm:"not null" json:"cash"`
	ReservedCash float64   `gorm:"not null;default:0" json:"reservedCash"`
	Version      uint64    `gorm:"not null;default:1" json:"version"`
}

// PaperPosition is the server-owned inventory and reservation ledger.
type PaperPosition struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	Currency         string    `gorm:"size:8;uniqueIndex:idx_paper_position;not null" json:"currency"`
	Symbol           string    `gorm:"size:20;uniqueIndex:idx_paper_position;not null" json:"symbol"`
	Quantity         int       `gorm:"not null;default:0" json:"quantity"`
	ReservedQuantity int       `gorm:"not null;default:0" json:"reservedQuantity"`
	AverageCost      float64   `gorm:"not null;default:0" json:"averageCost"`
	Version          uint64    `gorm:"not null;default:1" json:"version"`
}

// PaperStopScanAudit records one local Paper OCO exit scan. It is operational
// evidence only and does not represent broker acknowledgement or exchange state.
type PaperStopScanAudit struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	StartedAt          time.Time  `gorm:"index;not null" json:"startedAt"`
	CompletedAt        time.Time  `gorm:"index;not null" json:"completedAt"`
	QuoteObservedAt    *time.Time `json:"quoteObservedAt,omitempty"`
	Status             string     `gorm:"size:24;index;not null" json:"status"`
	CandidateCount     int        `json:"candidateCount"`
	TriggeredCount     int        `json:"triggeredCount"`
	SkippedCount       int        `json:"skippedCount"`
	ClosedMarketCount  int        `json:"closedMarketCount"`
	ErrorCount         int        `json:"errorCount"`
	EquityCheckpointed bool       `json:"equityCheckpointed"`
	DailyBaselineCount int        `json:"dailyBaselineCount"`
	StateFingerprint   string     `gorm:"size:64;index" json:"stateFingerprint,omitempty"`
	Details            string     `gorm:"type:text" json:"details,omitempty"`
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
