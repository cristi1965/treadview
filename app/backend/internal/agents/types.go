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
	XPlayerTakes       string `json:"x_player_takes"`

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
	AggressiveHistory   string `json:"aggressive_history"`
	ConservativeHistory string `json:"conservative_history"`
	NeutralHistory      string `json:"neutral_history"`
	History             string `json:"history"`
	LatestSpeaker       string `json:"latest_speaker"`
	JudgeDecision       string `json:"judge_decision"`
	Count               int    `json:"count"`
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
	Ticker          string           `json:"ticker" binding:"required"`
	TradeDate       string           `json:"trade_date" binding:"required"`
	AssetType       string           `json:"asset_type"`
	ResearchContext *ResearchContext `json:"research_context,omitempty"`
}

// ResearchContext is optional user-supplied portfolio context. Evidence-only
// research records it verbatim for audit and never infers omitted constraints.
type ResearchContext struct {
	Mandate      string            `json:"mandate,omitempty"`
	Holdings     []ResearchHolding `json:"holdings,omitempty"`
	Liquidity    string            `json:"liquidity,omitempty"`
	Tax          string            `json:"tax,omitempty"`
	RiskBudget   string            `json:"risk_budget,omitempty"`
	IssuerCapPct *float64          `json:"issuer_cap_pct,omitempty"`
	Scenario     *ResearchScenario `json:"scenario,omitempty"`
}

// ResearchScenario contains user-supplied assumptions. Evidence-only research
// records them as assumptions and never presents them as forecasts or targets.
type ResearchScenario struct {
	RevenueGrowthPct *float64 `json:"revenue_growth_pct,omitempty"`
	PSMultiple       *float64 `json:"ps_multiple,omitempty"`
}

type ResearchHolding struct {
	Symbol    string   `json:"symbol"`
	Quantity  *float64 `json:"quantity,omitempty"`
	WeightPct *float64 `json:"weight_pct,omitempty"`
}

// AnalysisResult is the final output of a completed analysis.
type AnalysisResult struct {
	Ticker         string               `json:"ticker"`
	TradeDate      string               `json:"trade_date"`
	ResearchMode   string               `json:"research_mode,omitempty"`
	Status         string               `json:"status"`
	Decision       string               `json:"decision"`
	Message        string               `json:"message,omitempty"`
	ResearchHealth ResearchHealth       `json:"research_health"`
	State          AgentState           `json:"state"`
	Audit          AnalysisAudit        `json:"audit"`
	Dossier        *EvidenceOnlyDossier `json:"dossier,omitempty"`
	CompletedAt    string               `json:"completed_at"`
	DurationSecs   float64              `json:"duration_secs"`
}

// EvidenceOnlyDossier is deterministic source synthesis. It is not an agent
// debate and may only conclude OBSERVE or ABSTAIN.
type EvidenceOnlyDossier struct {
	MethodVersion             string                         `json:"method_version"`
	Label                     string                         `json:"label"`
	Disclaimer                string                         `json:"disclaimer"`
	Facts                     []EvidenceOnlyFact             `json:"facts"`
	Calculations              []EvidenceOnlyCalculation      `json:"calculations"`
	FinancialPeriods          []EvidenceFinancialPeriod      `json:"financial_periods"`
	FinancialDerivationInputs []EvidenceFinancialPeriod      `json:"financial_derivation_inputs,omitempty"`
	Historical                *EvidenceHistoricalReview      `json:"historical,omitempty"`
	NewsReview                *EvidenceNewsReview            `json:"news_review,omitempty"`
	MarketSubmodel            *EvidenceMarketSubmodel        `json:"market_submodel,omitempty"`
	ResearchContext           *ResearchContext               `json:"research_context,omitempty"`
	ContextAssessment         *EvidenceContextAssessment     `json:"context_assessment,omitempty"`
	FinancialTrendSummary     *EvidenceFinancialTrendSummary `json:"financial_trend_summary,omitempty"`
	RiskDiagnostics           *EvidenceRiskDiagnostics       `json:"risk_diagnostics,omitempty"`
	Risks                     []string                       `json:"risks"`
	Gaps                      []string                       `json:"gaps"`
	ConclusionBasis           []string                       `json:"conclusion_basis"`
	Conclusion                string                         `json:"conclusion"`
}

type EvidenceMarketSubmodel struct {
	Name           string                    `json:"name"`
	Scope          string                    `json:"scope"`
	Status         string                    `json:"status"`
	Result         string                    `json:"result"`
	DatasetHash    string                    `json:"dataset_hash,omitempty"`
	Feature        string                    `json:"feature"`
	Label          string                    `json:"label"`
	Benchmark      string                    `json:"benchmark"`
	Cutoff         string                    `json:"cutoff,omitempty"`
	TrainSamples   int                       `json:"train_samples"`
	TestSamples    int                       `json:"test_samples"`
	Baseline       float64                   `json:"baseline_direction_accuracy"`
	Metric         EvidenceMarketModelMetric `json:"metric"`
	Reasons        []string                  `json:"reasons,omitempty"`
	FiveFactorUse  bool                      `json:"validates_five_factor_panel"`
	Classification string                    `json:"classification"`
	ProductSignal  bool                      `json:"product_signal"`
}

type EvidenceContextAssessment struct {
	Inputs      EvidenceContextInputs    `json:"inputs"`
	Holding     EvidenceContextHolding   `json:"holding"`
	IssuerCap   EvidenceContextIssuerCap `json:"issuer_cap"`
	Scenario    EvidenceContextScenario  `json:"scenario"`
	Limitations []string                 `json:"limitations"`
}

type EvidenceContextInput struct {
	Status string `json:"status"`
	Value  string `json:"value,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type EvidenceContextInputs struct {
	Mandate    EvidenceContextInput `json:"mandate"`
	Liquidity  EvidenceContextInput `json:"liquidity"`
	Tax        EvidenceContextInput `json:"tax"`
	RiskBudget EvidenceContextInput `json:"risk_budget"`
	Holdings   EvidenceContextInput `json:"holdings"`
}

type EvidenceContextHolding struct {
	Symbol      string   `json:"symbol"`
	Quantity    *float64 `json:"quantity,omitempty"`
	WeightPct   *float64 `json:"weight_pct,omitempty"`
	Status      string   `json:"status"`
	Reason      string   `json:"reason,omitempty"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

type EvidenceContextIssuerCap struct {
	CapPct      *float64 `json:"cap_pct,omitempty"`
	HeadroomPct *float64 `json:"headroom_pct,omitempty"`
	WithinCap   *bool    `json:"within_cap,omitempty"`
	Status      string   `json:"status"`
	Reason      string   `json:"reason,omitempty"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

type EvidenceContextScenario struct {
	Status            string   `json:"status"`
	RevenueGrowthPct  *float64 `json:"revenue_growth_pct,omitempty"`
	PSMultiple        *float64 `json:"ps_multiple,omitempty"`
	BaseTTMRevenue    *float64 `json:"base_ttm_revenue,omitempty"`
	ImpliedRevenue    *float64 `json:"implied_revenue,omitempty"`
	ImpliedMarketCap  *float64 `json:"implied_market_cap,omitempty"`
	SharesOutstanding *float64 `json:"shares_outstanding,omitempty"`
	ImpliedPrice      *float64 `json:"implied_price,omitempty"`
	CurrentPrice      *float64 `json:"current_price,omitempty"`
	ChangePct         *float64 `json:"change_pct,omitempty"`
	EvidenceIDs       []string `json:"evidence_ids"`
	Reason            string   `json:"reason,omitempty"`
	Disclosure        string   `json:"disclosure"`
}

type EvidenceFinancialTrendSummary struct {
	Status  string                         `json:"status"`
	Metrics []EvidenceFinancialTrendMetric `json:"metrics"`
}

type EvidenceFinancialTrendMetric struct {
	Label           string   `json:"label"`
	CalculationName string   `json:"calculation_name"`
	Status          string   `json:"status"`
	Value           *float64 `json:"value,omitempty"`
	Unit            string   `json:"unit,omitempty"`
	Reason          string   `json:"reason,omitempty"`
}

type EvidenceRiskDiagnostics struct {
	Status     string                         `json:"status"`
	Disclosure string                         `json:"disclosure"`
	Metrics    []EvidenceFinancialTrendMetric `json:"metrics"`
}

type EvidenceMarketModelMetric struct {
	Name              string     `json:"name"`
	SampleCount       int        `json:"sample_count"`
	IC                float64    `json:"ic"`
	IC95              [2]float64 `json:"ic_95"`
	DirectionAccuracy float64    `json:"direction_accuracy"`
	Accuracy95        [2]float64 `json:"accuracy_95"`
}

type EvidenceOnlyFact struct {
	Category    string   `json:"category"`
	Summary     string   `json:"summary"`
	Provider    string   `json:"provider"`
	SourceURL   string   `json:"source_url"`
	DataTime    string   `json:"data_time"`
	FilingDate  string   `json:"filing_date,omitempty"`
	EvidenceIDs []string `json:"evidence_ids"`
}

// EvidenceOnlyCalculation records a deterministic fact transform. Inputs are
// the exact values used by Formula; unknown results retain a reason and no value.
type EvidenceOnlyCalculation struct {
	Name         string             `json:"name"`
	Formula      string             `json:"formula"`
	Inputs       map[string]float64 `json:"inputs"`
	Value        *float64           `json:"value,omitempty"`
	Unit         string             `json:"unit,omitempty"`
	Status       string             `json:"status"`
	Reason       string             `json:"reason,omitempty"`
	EvidenceIDs  []string           `json:"evidence_ids"`
	InputSources map[string]string  `json:"input_sources,omitempty"`
}

type EvidenceFinancialPeriod struct {
	FiscalPeriod       string                            `json:"fiscal_period"`
	Frequency          string                            `json:"frequency,omitempty"`
	Form               string                            `json:"form,omitempty"`
	PeriodStart        string                            `json:"period_start,omitempty"`
	PeriodEnd          string                            `json:"period_end"`
	FilingDate         string                            `json:"filing_date"`
	Accession          string                            `json:"accession"`
	SourceURL          string                            `json:"source_url"`
	TotalRevenue       *float64                          `json:"total_revenue"`
	GrossProfit        *float64                          `json:"gross_profit"`
	OperatingIncome    *float64                          `json:"operating_income"`
	NetIncome          *float64                          `json:"net_income"`
	TotalAssets        *float64                          `json:"total_assets"`
	TotalLiabilities   *float64                          `json:"total_liabilities"`
	StockholdersEquity *float64                          `json:"stockholders_equity"`
	CashAndEquivalents *float64                          `json:"cash_and_equivalents"`
	SharesOutstanding  *float64                          `json:"shares_outstanding"`
	AvailableFields    []string                          `json:"available_fields"`
	FieldEvidence      map[string]EvidenceFinancialField `json:"field_evidence"`
}

type EvidenceFinancialField struct {
	Source      string `json:"source"`
	SourceURL   string `json:"source_url"`
	Unit        string `json:"unit"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	Form        string `json:"form"`
	FilingDate  string `json:"filing_date"`
	Accession   string `json:"accession"`
	PeriodKind  string `json:"period_kind"`
}

type EvidenceHistoricalObservation struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
}

type EvidenceHistoricalReview struct {
	SampleCount     int                             `json:"sample_count"`
	MinimumSessions int                             `json:"minimum_sessions"`
	StartDate       string                          `json:"start_date,omitempty"`
	EndDate         string                          `json:"end_date,omitempty"`
	TimeGranularity string                          `json:"time_granularity"`
	Status          string                          `json:"status"`
	Observations    []EvidenceHistoricalObservation `json:"observations"`
}

type EvidenceNewsItem struct {
	Title         string `json:"title"`
	URL           string `json:"url"`
	PublishedAt   string `json:"published_at"`
	Source        string `json:"source"`
	SourceTier    string `json:"source_tier"`
	ContentType   string `json:"content_type"`
	Theme         string `json:"theme"`
	RelevanceRule string `json:"relevance_rule"`
}

type EvidenceNewsReview struct {
	InputCount           int                `json:"input_count"`
	IncludedCount        int                `json:"included_count"`
	ExcludedLowRelevance int                `json:"excluded_low_relevance"`
	DuplicatesRemoved    int                `json:"duplicates_removed"`
	Rules                []string           `json:"rules"`
	Items                []EvidenceNewsItem `json:"items"`
}

// Evidence is one independently identifiable input or generated artifact in a run.
type Evidence struct {
	ID               string            `json:"id"`
	Kind             string            `json:"kind"`
	Source           string            `json:"source"`
	URL              string            `json:"url,omitempty"`
	DataTime         string            `json:"data_time"`
	TimeGranularity  string            `json:"time_granularity,omitempty"`
	FetchedAt        string            `json:"fetched_at"`
	MethodVersion    string            `json:"method_version"`
	ContentHash      string            `json:"content_hash,omitempty"`
	PayloadExcerpt   string            `json:"payload_excerpt,omitempty"`
	PayloadTruncated bool              `json:"payload_truncated,omitempty"`
	PayloadRef       string            `json:"payload_ref,omitempty"`
	PayloadSize      int64             `json:"payload_size,omitempty"`
	Status           string            `json:"status"`
	Inputs           map[string]string `json:"inputs,omitempty"`
}

// Claim binds a generated stage artifact to the evidence identifiers used for review.
type Claim struct {
	ID            string   `json:"id"`
	ArtifactID    string   `json:"artifact_id"`
	Stage         string   `json:"stage"`
	Summary       string   `json:"summary"`
	ContentHash   string   `json:"content_hash"`
	EvidenceIDs   []string `json:"evidence_ids"`
	DataTime      string   `json:"data_time"`
	Status        string   `json:"status"`
	Model         string   `json:"model"`
	MethodVersion string   `json:"method_version"`
	CreatedAt     string   `json:"created_at"`
}

// AnalysisAudit is the structured, machine-readable evidence chain persisted with a result.
type AnalysisAudit struct {
	RunID         string            `json:"run_id"`
	CreatedAt     string            `json:"created_at"`
	MethodVersion string            `json:"method_version"`
	ModelProvider string            `json:"model_provider"`
	Models        map[string]string `json:"models"`
	Inputs        map[string]string `json:"inputs"`
	InputHash     string            `json:"input_hash"`
	Evidence      []Evidence        `json:"evidence"`
	Claims        []Claim           `json:"claims"`
	Health        ResearchHealth    `json:"health"`
}

const (
	ResearchStatusPublished     = "published"
	ResearchStatusEvidenceOnly  = "evidence_only"
	ResearchStatusUnavailable   = "research_unavailable"
	ResearchDecisionUnavailable = "research_unavailable"
	ResearchModeEvidenceOnly    = "evidence-only"
	ResearchDecisionObserve     = "OBSERVE"
	ResearchDecisionAbstain     = "ABSTAIN"
)

// ResearchHealth separates document integrity from the health of source evidence.
type ResearchHealth struct {
	StructureStatus string   `json:"structure_status"`
	SourceStatus    string   `json:"source_status"`
	Publishable     bool     `json:"publishable"`
	DataTime        string   `json:"data_time"`
	Reasons         []string `json:"reasons,omitempty"`
}

// ConfigResponse is returned by GET /api/config.
type ConfigResponse struct {
	LLMProvider     string `json:"llm_provider"`
	DeepThinkLLM    string `json:"deep_think_llm"`
	QuickThinkLLM   string `json:"quick_think_llm"`
	OutputLanguage  string `json:"output_language"`
	MaxDebateRounds int    `json:"max_debate_rounds"`
	MaxRiskRounds   int    `json:"max_risk_rounds"`
	LLMBackendURL   string `json:"llm_backend_url,omitempty"`
}

// ConfigUpdateRequest is for PUT /api/config.
type ConfigUpdateRequest struct {
	LLMProvider     *string `json:"llm_provider,omitempty"`
	DeepSeekAPIKey  *string `json:"deepseek_api_key,omitempty"`
	GoogleAPIKey    *string `json:"google_api_key,omitempty"`
	OpenAIAPIKey    *string `json:"openai_api_key,omitempty"`
	DeepThinkLLM    *string `json:"deep_think_llm,omitempty"`
	QuickThinkLLM   *string `json:"quick_think_llm,omitempty"`
	LLMBackendURL   *string `json:"llm_backend_url,omitempty"`
	OutputLanguage  *string `json:"output_language,omitempty"`
	MaxDebateRounds *int    `json:"max_debate_rounds,omitempty"`
	MaxRiskRounds   *int    `json:"max_risk_rounds,omitempty"`
}
