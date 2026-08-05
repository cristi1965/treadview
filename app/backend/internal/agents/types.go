package agents

// AgentState holds the full state of a trading analysis run.
type AgentState struct {
	CompanyOfInterest string `json:"company_of_interest"`
	AssetType         string `json:"asset_type"`
	InstrumentContext string `json:"instrument_context"`
	TradeDate         string `json:"trade_date"`
	OutputLanguage    string `json:"output_language"`

	// Analyst reports
	MarketReport       string `json:"market_report"`
	SentimentReport    string `json:"sentiment_report"`
	NewsReport         string `json:"news_report"`
	FundamentalsReport string `json:"fundamentals_report"`

	// Investment debate
	InvestmentDebateState InvestDebateState `json:"investment_debate_state"`
	InvestmentPlan        string            `json:"investment_plan"`

	// Trader
	TraderInvestmentPlan string `json:"trader_investment_plan"`

	// Risk debate
	RiskDebateState    RiskDebateState `json:"risk_debate_state"`
	FinalTradeDecision string          `json:"final_trade_decision"`

	// Memory
	PastContext string `json:"past_context"`
}

// InvestDebateState tracks the bull/bear research debate.
type InvestDebateState struct {
	BullHistory     string `json:"bull_history"`
	BearHistory     string `json:"bear_history"`
	History         string `json:"history"`
	CurrentResponse string `json:"current_response"`
	JudgeDecision   string `json:"judge_decision"`
	Count           int    `json:"count"`
}

// RiskDebateState tracks the risk management debate.
type RiskDebateState struct {
	AggressiveHistory string `json:"aggressive_history"`
	ConservativeHistory string `json:"conservative_history"`
	NeutralHistory    string `json:"neutral_history"`
	History           string `json:"history"`
	LatestSpeaker     string `json:"latest_speaker"`
	JudgeDecision     string `json:"judge_decision"`
	Count             int    `json:"count"`
}

// NodeEvent is emitted during agent execution for real-time UI updates.
type NodeEvent struct {
	Type      string `json:"type"`
	Node      string `json:"node"`
	Content   string `json:"content"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	Timestamp int64  `json:"timestamp"`
}

// AnalysisRequest is the API input for starting an analysis.
type AnalysisRequest struct {
	Ticker    string `json:"ticker" binding:"required"`
	TradeDate string `json:"trade_date" binding:"required"`
	AssetType string `json:"asset_type"`
}

// AnalysisResult is the final output of a completed analysis.
type AnalysisResult struct {
	Ticker       string     `json:"ticker"`
	TradeDate    string     `json:"trade_date"`
	Decision     string     `json:"decision"`
	State        AgentState `json:"state"`
	CompletedAt  string     `json:"completed_at"`
	DurationSecs float64   `json:"duration_secs"`
}

// ConfigResponse is returned by GET /api/config.
type ConfigResponse struct {
	LLMProvider    string `json:"llm_provider"`
	DeepThinkLLM   string `json:"deep_think_llm"`
	QuickThinkLLM  string `json:"quick_think_llm"`
	OutputLanguage string `json:"output_language"`
	MaxDebateRounds int   `json:"max_debate_rounds"`
	MaxRiskRounds   int   `json:"max_risk_rounds"`
	LLMBackendURL   string `json:"llm_backend_url,omitempty"`
}

// ConfigUpdateRequest is for PUT /api/config.
type ConfigUpdateRequest struct {
	LLMProvider    *string `json:"llm_provider,omitempty"`
	DeepSeekAPIKey *string `json:"deepseek_api_key,omitempty"`
	GoogleAPIKey   *string `json:"google_api_key,omitempty"`
	DeepThinkLLM   *string `json:"deep_think_llm,omitempty"`
	QuickThinkLLM  *string `json:"quick_think_llm,omitempty"`
	LLMBackendURL  *string `json:"llm_backend_url,omitempty"`
	OutputLanguage  *string `json:"output_language,omitempty"`
	MaxDebateRounds *int    `json:"max_debate_rounds,omitempty"`
	MaxRiskRounds   *int    `json:"max_risk_rounds,omitempty"`
}
