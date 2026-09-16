package dataflows

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	yfinanceBaseURL = "https://query1.finance.yahoo.com"
	requestTimeout  = 30 * time.Second
	userAgent       = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

// YFinanceClient fetches stock data from Yahoo Finance.
type YFinanceClient struct {
	httpClient *http.Client
	mu         sync.Mutex
	crumb      string
	crumbAt    time.Time
}

// NewYFinanceClient creates a new Yahoo Finance client with cookie jar for crumb auth.
func NewYFinanceClient() *YFinanceClient {
	return NewYFinanceClientWithTimeout(requestTimeout)
}

// NewYFinanceClientWithTimeout creates a client for latency-sensitive fan-out calls.
func NewYFinanceClientWithTimeout(timeout time.Duration) *YFinanceClient {
	jar, _ := cookiejar.New(nil)
	return &YFinanceClient{
		httpClient: &http.Client{Timeout: timeout, Jar: jar},
	}
}

// StockQuote holds basic quote data.
type StockQuote struct {
	Symbol        string    `json:"symbol"`
	ShortName     string    `json:"shortName"`
	LongName      string    `json:"longName"`
	Currency      string    `json:"currency"`
	RegularPrice  float64   `json:"regularMarketPrice"`
	PreviousClose float64   `json:"regularMarketPreviousClose"`
	Open          float64   `json:"regularMarketOpen"`
	DayHigh       float64   `json:"regularMarketDayHigh"`
	DayLow        float64   `json:"regularMarketDayLow"`
	Volume        int64     `json:"regularMarketVolume"`
	MarketCap     float64   `json:"marketCap"`
	PERatio       float64   `json:"trailingPE"`
	ForwardPE     float64   `json:"forwardPE"`
	DividendYield float64   `json:"dividendYield"`
	FiftyTwoHigh  float64   `json:"fiftyTwoWeekHigh"`
	FiftyTwoLow   float64   `json:"fiftyTwoWeekLow"`
	Beta          float64   `json:"beta"`
	EPS           float64   `json:"trailingEps"`
	ObservedAt    time.Time `json:"-"`
	MarketState   string    `json:"-"`
}

// HistoricalBar represents one OHLCV bar.
type HistoricalBar struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
}

// FundamentalData holds company fundamentals.
type FundamentalData struct {
	Source              string            `json:"source,omitempty"`
	FiscalPeriod        string            `json:"fiscalPeriod,omitempty"`
	AsOf                string            `json:"asOf,omitempty"`
	FilingDate          string            `json:"filingDate,omitempty"`
	Accession           string            `json:"accession,omitempty"`
	SourceURL           string            `json:"sourceURL,omitempty"`
	SourceFetchedAt     string            `json:"sourceFetchedAt,omitempty"`
	SourceContentHash   string            `json:"sourceContentHash,omitempty"`
	SourceTransport     string            `json:"sourceTransport,omitempty"`
	Sector              string            `json:"sector"`
	Industry            string            `json:"industry"`
	FullTimeEmployees   *int              `json:"fullTimeEmployees"`
	Website             string            `json:"website"`
	LongBusinessSummary string            `json:"longBusinessSummary"`
	MarketCap           *float64          `json:"marketCap"`
	EnterpriseValue     *float64          `json:"enterpriseValue"`
	ProfitMargin        *float64          `json:"profitMargins"`
	OperatingMargin     *float64          `json:"operatingMargins"`
	ReturnOnEquity      *float64          `json:"returnOnEquity"`
	ReturnOnAssets      *float64          `json:"returnOnAssets"`
	RevenueGrowth       *float64          `json:"revenueGrowth"`
	EarningsGrowth      *float64          `json:"earningsGrowth"`
	DebtToEquity        *float64          `json:"debtToEquity"`
	CurrentRatio        *float64          `json:"currentRatio"`
	BookValue           *float64          `json:"bookValue"`
	FreeCashflow        *float64          `json:"freeCashflow"`
	TotalRevenue        *float64          `json:"totalRevenue"`
	GrossProfits        *float64          `json:"grossProfits"`
	EBITDA              *float64          `json:"ebitda"`
	Availability        map[string]bool   `json:"availability"`
	Periods             []FinancialPeriod `json:"periods"`
	DerivationInputs    []FinancialPeriod `json:"derivationInputs,omitempty"`
}

// FinancialPeriod is one filing-bound financial period. Missing values stay
// omitted; filing metadata must come from an upstream filing index.
type FinancialPeriod struct {
	FiscalPeriod       string                            `json:"fiscalPeriod"`
	Frequency          string                            `json:"frequency,omitempty"`
	Form               string                            `json:"form,omitempty"`
	PeriodStart        string                            `json:"periodStart,omitempty"`
	PeriodEnd          string                            `json:"periodEnd"`
	FilingDate         string                            `json:"filingDate"`
	Accession          string                            `json:"accession"`
	SourceURL          string                            `json:"sourceURL"`
	TotalRevenue       float64                           `json:"totalRevenue,omitempty"`
	GrossProfit        float64                           `json:"grossProfit,omitempty"`
	OperatingIncome    float64                           `json:"operatingIncome,omitempty"`
	NetIncome          float64                           `json:"netIncome,omitempty"`
	TotalAssets        float64                           `json:"totalAssets,omitempty"`
	TotalLiabilities   float64                           `json:"totalLiabilities,omitempty"`
	StockholdersEquity float64                           `json:"stockholdersEquity,omitempty"`
	CashAndEquivalents float64                           `json:"cashAndEquivalents,omitempty"`
	SharesOutstanding  float64                           `json:"sharesOutstanding,omitempty"`
	AvailableFields    []string                          `json:"availableFields,omitempty"`
	FieldEvidence      map[string]FinancialFieldEvidence `json:"fieldEvidence,omitempty"`
}

// FinancialFieldEvidence binds one reported value to the exact filing fact
// used for that period. Duration facts retain their reported start/end;
// instant facts use the same date for start/end.
type FinancialFieldEvidence struct {
	Source      string `json:"source"`
	URL         string `json:"url"`
	Unit        string `json:"unit"`
	PeriodStart string `json:"periodStart"`
	PeriodEnd   string `json:"periodEnd"`
	Form        string `json:"form"`
	Filed       string `json:"filed"`
	Accession   string `json:"accession"`
	PeriodKind  string `json:"periodKind"`
}

// MarshalJSON makes unavailable period metrics explicit nulls. Internal
// collectors retain numeric fields plus AvailableFields so existing dataflow
// consumers can distinguish a legitimate reported zero from absence.
func (p FinancialPeriod) MarshalJSON() ([]byte, error) {
	type nullablePeriod struct {
		FiscalPeriod       string                            `json:"fiscalPeriod"`
		Frequency          string                            `json:"frequency,omitempty"`
		Form               string                            `json:"form,omitempty"`
		PeriodStart        string                            `json:"periodStart,omitempty"`
		PeriodEnd          string                            `json:"periodEnd"`
		FilingDate         string                            `json:"filingDate"`
		Accession          string                            `json:"accession"`
		SourceURL          string                            `json:"sourceURL"`
		TotalRevenue       *float64                          `json:"totalRevenue"`
		GrossProfit        *float64                          `json:"grossProfit"`
		OperatingIncome    *float64                          `json:"operatingIncome"`
		NetIncome          *float64                          `json:"netIncome"`
		TotalAssets        *float64                          `json:"totalAssets"`
		TotalLiabilities   *float64                          `json:"totalLiabilities"`
		StockholdersEquity *float64                          `json:"stockholdersEquity"`
		CashAndEquivalents *float64                          `json:"cashAndEquivalents"`
		SharesOutstanding  *float64                          `json:"sharesOutstanding"`
		AvailableFields    []string                          `json:"availableFields"`
		FieldEvidence      map[string]FinancialFieldEvidence `json:"fieldEvidence,omitempty"`
	}
	return json.Marshal(nullablePeriod{
		FiscalPeriod: p.FiscalPeriod, Frequency: p.Frequency, Form: p.Form, PeriodStart: p.PeriodStart,
		PeriodEnd: p.PeriodEnd, FilingDate: p.FilingDate, Accession: p.Accession, SourceURL: p.SourceURL,
		TotalRevenue: periodJSONValue(p, "totalRevenue", p.TotalRevenue), GrossProfit: periodJSONValue(p, "grossProfit", p.GrossProfit),
		OperatingIncome: periodJSONValue(p, "operatingIncome", p.OperatingIncome), NetIncome: periodJSONValue(p, "netIncome", p.NetIncome),
		TotalAssets: periodJSONValue(p, "totalAssets", p.TotalAssets), TotalLiabilities: periodJSONValue(p, "totalLiabilities", p.TotalLiabilities),
		StockholdersEquity: periodJSONValue(p, "stockholdersEquity", p.StockholdersEquity), CashAndEquivalents: periodJSONValue(p, "cashAndEquivalents", p.CashAndEquivalents),
		SharesOutstanding: periodJSONValue(p, "sharesOutstanding", p.SharesOutstanding),
		AvailableFields:   p.AvailableFields, FieldEvidence: p.FieldEvidence,
	})
}

func periodJSONValue(period FinancialPeriod, field string, value float64) *float64 {
	for _, available := range period.AvailableFields {
		if available == field {
			copy := value
			return &copy
		}
	}
	return nil
}

// StockMetrics is the frontend-facing fundamentals card payload.
type StockMetrics struct {
	Symbol             string                     `json:"symbol"`
	Name               string                     `json:"name,omitempty"`
	Sector             string                     `json:"sector,omitempty"`
	Industry           string                     `json:"industry,omitempty"`
	Price              float64                    `json:"price,omitempty"`
	TrailingPE         float64                    `json:"trailingPE,omitempty"`
	ForwardPE          float64                    `json:"forwardPE,omitempty"`
	PriceToSales       float64                    `json:"priceToSales,omitempty"`
	EnterpriseToEbit   float64                    `json:"enterpriseToEbitda,omitempty"`
	PEG                float64                    `json:"peg,omitempty"`
	PriceToBook        float64                    `json:"priceToBook,omitempty"`
	GrossMargin        float64                    `json:"grossMargin,omitempty"`
	ProfitMargin       float64                    `json:"profitMargin,omitempty"`
	ROE                float64                    `json:"roe,omitempty"`
	RevenueGrowth      float64                    `json:"revenueGrowth,omitempty"`
	DividendYield      float64                    `json:"dividendYield,omitempty"`
	Beta               float64                    `json:"beta,omitempty"`
	TargetMeanPrice    float64                    `json:"targetMeanPrice,omitempty"`
	Recommendation     string                     `json:"recommendation,omitempty"`
	FiftyTwoHigh       float64                    `json:"fiftyTwoWeekHigh,omitempty"`
	FiftyTwoLow        float64                    `json:"fiftyTwoWeekLow,omitempty"`
	FiftyTwoPos        float64                    `json:"fiftyTwoWeekPos,omitempty"`
	TotalRevenue       float64                    `json:"totalRevenue,omitempty"`
	GrossProfit        float64                    `json:"grossProfit,omitempty"`
	OperatingIncome    float64                    `json:"operatingIncome,omitempty"`
	NetIncome          float64                    `json:"netIncome,omitempty"`
	TotalAssets        float64                    `json:"totalAssets,omitempty"`
	TotalLiabilities   float64                    `json:"totalLiabilities,omitempty"`
	StockholdersEquity float64                    `json:"stockholdersEquity,omitempty"`
	CashAndEquivalents float64                    `json:"cashAndEquivalents,omitempty"`
	Source             string                     `json:"source"`
	FiscalPeriod       string                     `json:"fiscalPeriod"`
	AsOf               string                     `json:"asOf"`
	FilingDate         string                     `json:"filingDate,omitempty"`
	Accession          string                     `json:"accession,omitempty"`
	SourceURL          string                     `json:"sourceURL,omitempty"`
	FetchedAt          string                     `json:"fetchedAt"`
	SourceFetchedAt    string                     `json:"sourceFetchedAt,omitempty"`
	SourceContentHash  string                     `json:"sourceContentHash,omitempty"`
	SourceTransport    string                     `json:"sourceTransport,omitempty"`
	SourceLinks        []SourceReference          `json:"sourceLinks"`
	FieldSources       map[string]FieldProvenance `json:"fieldSources"`
	Periods            []FinancialPeriod          `json:"periods"`
	DerivationInputs   []FinancialPeriod          `json:"derivationInputs,omitempty"`
}

// SourceReference is a human-verifiable upstream page used by a payload.
type SourceReference struct {
	Source string `json:"source"`
	Label  string `json:"label"`
	URL    string `json:"url"`
}

// FieldProvenance identifies where and when an individual metric came from.
type FieldProvenance struct {
	Source       string `json:"source"`
	URL          string `json:"url"`
	AsOf         string `json:"asOf"`
	FiscalPeriod string `json:"fiscalPeriod,omitempty"`
	FilingDate   string `json:"filingDate,omitempty"`
	Accession    string `json:"accession,omitempty"`
	PeriodStart  string `json:"periodStart,omitempty"`
	Frequency    string `json:"frequency,omitempty"`
	Form         string `json:"form,omitempty"`
	Unit         string `json:"unit,omitempty"`
}

// FinalizeStockMetricsEvidence makes unavailable dates explicit and records fetch time separately.
func FinalizeStockMetricsEvidence(m *StockMetrics, fetchedAt time.Time) {
	if m == nil {
		return
	}
	if m.FiscalPeriod == "" {
		m.FiscalPeriod = "unknown"
	}
	if m.AsOf == "" {
		m.AsOf = "unknown"
	}
	m.FetchedAt = fetchedAt.UTC().Format(time.RFC3339)
	if m.SourceLinks == nil {
		m.SourceLinks = []SourceReference{}
	}
	if m.FieldSources == nil {
		m.FieldSources = map[string]FieldProvenance{}
	}
	if len(m.SourceLinks) > 0 {
		seen := map[string]bool{}
		sources := make([]string, 0, len(m.SourceLinks))
		for _, link := range m.SourceLinks {
			if link.Source != "" && !seen[link.Source] {
				seen[link.Source] = true
				sources = append(sources, link.Source)
			}
		}
		m.Source = strings.Join(sources, "+")
	}
}

func addMetricSource(m *StockMetrics, source, label, sourceURL, asOf string, fields ...string) {
	if m.FieldSources == nil {
		m.FieldSources = map[string]FieldProvenance{}
	}
	if asOf == "" {
		asOf = "unknown"
	}
	for _, field := range fields {
		m.FieldSources[field] = FieldProvenance{Source: source, URL: sourceURL, AsOf: asOf}
	}
	for _, link := range m.SourceLinks {
		if link.URL == sourceURL {
			return
		}
	}
	m.SourceLinks = append(m.SourceLinks, SourceReference{Source: source, Label: label, URL: sourceURL})
}

// NewsItem is one related headline.
type NewsItem struct {
	Title     string `json:"title"`
	Source    string `json:"source"`
	Link      string `json:"link,omitempty"`
	Published int64  `json:"published,omitempty"`
}

func (c *YFinanceClient) ensureCrumb() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.crumb != "" && time.Since(c.crumbAt) < 30*time.Minute {
		return nil
	}
	// Warm cookies (fc.yahoo.com often 404 but still sets A1/A3)
	for _, warm := range []string{"https://fc.yahoo.com", "https://finance.yahoo.com/"} {
		req, err := http.NewRequest("GET", warm, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", userAgent)
		resp, err := c.httpClient.Do(req)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}
	crumbReq, err := http.NewRequest("GET", yfinanceBaseURL+"/v1/test/getcrumb", nil)
	if err != nil {
		return err
	}
	crumbReq.Header.Set("User-Agent", userAgent)
	crumbResp, err := c.httpClient.Do(crumbReq)
	if err != nil {
		return fmt.Errorf("getcrumb: %w", err)
	}
	defer crumbResp.Body.Close()
	body, err := io.ReadAll(crumbResp.Body)
	if err != nil {
		return err
	}
	crumb := strings.TrimSpace(string(body))
	if crumb == "" || strings.Contains(crumb, "Unauthorized") || strings.HasPrefix(crumb, "{") || crumbResp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid yahoo crumb: %s", truncate(crumb, 80))
	}
	c.crumb = crumb
	c.crumbAt = time.Now()
	return nil
}

func (c *YFinanceClient) invalidateCrumb() {
	c.mu.Lock()
	c.crumb = ""
	c.crumbAt = time.Time{}
	c.mu.Unlock()
}

// GetQuote fetches real-time quote for a ticker via chart (no crumb).
func (c *YFinanceClient) GetQuote(ticker string) (*StockQuote, error) {
	origTicker := ticker
	ticker = ToYahooSymbol(ticker)
	u := fmt.Sprintf("%s/v8/finance/chart/%s?interval=1d&range=5d",
		yfinanceBaseURL, url.PathEscape(ticker))
	body, err := c.doRequest(u, false)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Chart struct {
			Result []struct {
				Meta struct {
					Symbol              string  `json:"symbol"`
					ShortName           string  `json:"shortName"`
					LongName            string  `json:"longName"`
					Currency            string  `json:"currency"`
					RegularMarketPrice  float64 `json:"regularMarketPrice"`
					ChartPreviousClose  float64 `json:"chartPreviousClose"`
					RegularMarketVolume int64   `json:"regularMarketVolume"`
					RegularMarketTime   int64   `json:"regularMarketTime"`
					MarketState         string  `json:"marketState"`
					FiftyTwoWeekHigh    float64 `json:"fiftyTwoWeekHigh"`
					FiftyTwoWeekLow     float64 `json:"fiftyTwoWeekLow"`
				} `json:"meta"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse quote response: %w", err)
	}
	if len(resp.Chart.Result) == 0 {
		return nil, fmt.Errorf("no quote data for %s", origTicker)
	}
	m := resp.Chart.Result[0].Meta
	return &StockQuote{
		Symbol:        firstNonEmpty(m.Symbol, origTicker),
		ShortName:     m.ShortName,
		LongName:      m.LongName,
		Currency:      m.Currency,
		RegularPrice:  m.RegularMarketPrice,
		PreviousClose: m.ChartPreviousClose,
		Volume:        m.RegularMarketVolume,
		FiftyTwoHigh:  m.FiftyTwoWeekHigh,
		FiftyTwoLow:   m.FiftyTwoWeekLow,
		ObservedAt:    time.Unix(m.RegularMarketTime, 0).UTC(),
		MarketState:   strings.ToLower(m.MarketState),
	}, nil
}

// GetHistoricalData fetches OHLCV bars for a date range.
func (c *YFinanceClient) GetHistoricalData(ticker, startDate, endDate string) ([]HistoricalBar, error) {
	ticker = ToYahooSymbol(ticker)
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start date: %w", err)
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end date: %w", err)
	}

	u := historicalDataURL(ticker, start, end)

	body, err := c.doRequest(u, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Chart struct {
			Result []struct {
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Open   []float64 `json:"open"`
						High   []float64 `json:"high"`
						Low    []float64 `json:"low"`
						Close  []float64 `json:"close"`
						Volume []int64   `json:"volume"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse chart response: %w", err)
	}

	if len(resp.Chart.Result) == 0 || len(resp.Chart.Result[0].Timestamp) == 0 {
		return nil, fmt.Errorf("no historical data for %s", ticker)
	}

	r := resp.Chart.Result[0]
	q := r.Indicators.Quote[0]
	var bars []HistoricalBar
	for i, ts := range r.Timestamp {
		t := time.Unix(ts, 0).UTC()
		bar := HistoricalBar{Date: t.Format("2006-01-02")}
		if i < len(q.Open) {
			bar.Open = q.Open[i]
		}
		if i < len(q.High) {
			bar.High = q.High[i]
		}
		if i < len(q.Low) {
			bar.Low = q.Low[i]
		}
		if i < len(q.Close) {
			bar.Close = q.Close[i]
		}
		if i < len(q.Volume) {
			bar.Volume = q.Volume[i]
		}
		bars = append(bars, bar)
	}
	return bars, nil
}

func historicalDataURL(ticker string, start, end time.Time) string {
	return fmt.Sprintf("%s/v8/finance/chart/%s?period1=%d&period2=%d&interval=1d",
		yfinanceBaseURL, url.PathEscape(ToYahooSymbol(ticker)), start.Unix(), end.Unix())
}

// GetFundamentals fetches company profile and financial fundamentals.
func (c *YFinanceClient) GetFundamentals(ticker string) (*FundamentalData, error) {
	m, err := c.GetStockMetricsLive(ticker)
	if err != nil {
		return nil, err
	}
	return &FundamentalData{
		Source:            m.Source,
		FiscalPeriod:      m.FiscalPeriod,
		AsOf:              m.AsOf,
		FilingDate:        m.FilingDate,
		Accession:         m.Accession,
		SourceURL:         m.SourceURL,
		SourceFetchedAt:   m.SourceFetchedAt,
		SourceContentHash: m.SourceContentHash,
		SourceTransport:   m.SourceTransport,
		Sector:            m.Sector,
		Industry:          m.Industry,
		ProfitMargin:      stockMetricPointer(m, "profitMargin", m.ProfitMargin),
		ReturnOnEquity:    stockMetricPointer(m, "roe", m.ROE),
		RevenueGrowth:     stockMetricPointer(m, "revenueGrowth", m.RevenueGrowth),
		TotalRevenue:      stockMetricPointer(m, "totalRevenue", m.TotalRevenue),
		Availability:      stockMetricAvailability(m),
		Periods:           append([]FinancialPeriod(nil), m.Periods...),
		DerivationInputs:  append([]FinancialPeriod(nil), m.DerivationInputs...),
	}, nil
}

func stockMetricPointer(metrics *StockMetrics, field string, value float64) *float64 {
	if metrics == nil {
		return nil
	}
	if _, ok := metrics.FieldSources[field]; !ok {
		return nil
	}
	copy := value
	return &copy
}

func stockMetricAvailability(metrics *StockMetrics) map[string]bool {
	out := map[string]bool{}
	if metrics == nil {
		return out
	}
	for field := range metrics.FieldSources {
		out[field] = true
	}
	return out
}

// GetStockMetrics fetches valuation + quality metrics via crumb-authenticated quoteSummary.
func (c *YFinanceClient) GetStockMetrics(ticker string) (*StockMetrics, error) {
	origTicker := ticker
	ticker = ToYahooSymbol(strings.TrimSpace(strings.ToUpper(ticker)))
	modules := "summaryDetail,defaultKeyStatistics,financialData,assetProfile,price"
	body, err := c.quoteSummary(ticker, modules)
	if err != nil {
		return nil, err
	}

	var resp struct {
		QuoteSummary struct {
			Result []struct {
				AssetProfile *struct {
					Sector   string `json:"sector"`
					Industry string `json:"industry"`
				} `json:"assetProfile"`
				SummaryDetail *struct {
					TrailingPE       yahooRaw `json:"trailingPE"`
					ForwardPE        yahooRaw `json:"forwardPE"`
					PriceToSales     yahooRaw `json:"priceToSalesTrailing12Months"`
					DividendYield    yahooRaw `json:"dividendYield"`
					FiftyTwoWeekHigh yahooRaw `json:"fiftyTwoWeekHigh"`
					FiftyTwoWeekLow  yahooRaw `json:"fiftyTwoWeekLow"`
					Beta             yahooRaw `json:"beta"`
				} `json:"summaryDetail"`
				DefaultKeyStatistics *struct {
					TrailingPE        yahooRaw `json:"trailingPE"`
					ForwardPE         yahooRaw `json:"forwardPE"`
					PegRatio          yahooRaw `json:"pegRatio"`
					PriceToBook       yahooRaw `json:"priceToBook"`
					EnterpriseToEbit  yahooRaw `json:"enterpriseToEbitda"`
					Beta              yahooRaw `json:"beta"`
					LastFiscalYearEnd yahooRaw `json:"lastFiscalYearEnd"`
				} `json:"defaultKeyStatistics"`
				FinancialData *struct {
					CurrentPrice      yahooRaw `json:"currentPrice"`
					TargetMeanPrice   yahooRaw `json:"targetMeanPrice"`
					RecommendationKey string   `json:"recommendationKey"`
					GrossMargins      yahooRaw `json:"grossMargins"`
					ProfitMargins     yahooRaw `json:"profitMargins"`
					ReturnOnEquity    yahooRaw `json:"returnOnEquity"`
					RevenueGrowth     yahooRaw `json:"revenueGrowth"`
				} `json:"financialData"`
				Price *struct {
					ShortName          string   `json:"shortName"`
					LongName           string   `json:"longName"`
					RegularMarketPrice yahooRaw `json:"regularMarketPrice"`
					RegularMarketTime  int64    `json:"regularMarketTime"`
				} `json:"price"`
			} `json:"result"`
			Error *struct {
				Description string `json:"description"`
			} `json:"error"`
		} `json:"quoteSummary"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse quoteSummary: %w", err)
	}
	if len(resp.QuoteSummary.Result) == 0 {
		if resp.QuoteSummary.Error != nil {
			return nil, fmt.Errorf("yahoo: %s", resp.QuoteSummary.Error.Description)
		}
		return nil, fmt.Errorf("no fundamental data for %s", origTicker)
	}
	r := resp.QuoteSummary.Result[0]
	out := &StockMetrics{Symbol: origTicker, Source: "yahoo-quoteSummary"}
	sourceURL := fmt.Sprintf("https://finance.yahoo.com/quote/%s/key-statistics/", url.PathEscape(ticker))
	if r.AssetProfile != nil {
		out.Sector = r.AssetProfile.Sector
		out.Industry = r.AssetProfile.Industry
	}
	if r.Price != nil {
		out.Name = firstNonEmpty(r.Price.LongName, r.Price.ShortName)
		out.Price = r.Price.RegularMarketPrice.F()
		if r.Price.RegularMarketTime > 0 {
			out.AsOf = time.Unix(r.Price.RegularMarketTime, 0).UTC().Format(time.RFC3339)
		}
	}
	if r.FinancialData != nil {
		if out.Price == 0 {
			out.Price = r.FinancialData.CurrentPrice.F()
		}
		out.GrossMargin = r.FinancialData.GrossMargins.F()
		out.ProfitMargin = r.FinancialData.ProfitMargins.F()
		out.ROE = r.FinancialData.ReturnOnEquity.F()
		out.RevenueGrowth = r.FinancialData.RevenueGrowth.F()
		out.TargetMeanPrice = r.FinancialData.TargetMeanPrice.F()
		out.Recommendation = r.FinancialData.RecommendationKey
	}
	if r.SummaryDetail != nil {
		out.TrailingPE = r.SummaryDetail.TrailingPE.F()
		out.ForwardPE = r.SummaryDetail.ForwardPE.F()
		out.PriceToSales = r.SummaryDetail.PriceToSales.F()
		out.DividendYield = r.SummaryDetail.DividendYield.F()
		out.FiftyTwoHigh = r.SummaryDetail.FiftyTwoWeekHigh.F()
		out.FiftyTwoLow = r.SummaryDetail.FiftyTwoWeekLow.F()
		out.Beta = r.SummaryDetail.Beta.F()
	}
	if r.DefaultKeyStatistics != nil {
		if out.TrailingPE == 0 {
			out.TrailingPE = r.DefaultKeyStatistics.TrailingPE.F()
		}
		if out.ForwardPE == 0 {
			out.ForwardPE = r.DefaultKeyStatistics.ForwardPE.F()
		}
		out.PEG = r.DefaultKeyStatistics.PegRatio.F()
		out.PriceToBook = r.DefaultKeyStatistics.PriceToBook.F()
		out.EnterpriseToEbit = r.DefaultKeyStatistics.EnterpriseToEbit.F()
		if out.Beta == 0 {
			out.Beta = r.DefaultKeyStatistics.Beta.F()
		}
		if fiscalEnd := int64(r.DefaultKeyStatistics.LastFiscalYearEnd.F()); fiscalEnd > 0 {
			out.FiscalPeriod = time.Unix(fiscalEnd, 0).UTC().Format("2006-01-02")
		}
	}
	if out.FiftyTwoHigh > out.FiftyTwoLow && out.FiftyTwoLow > 0 && out.Price > 0 {
		out.FiftyTwoPos = (out.Price - out.FiftyTwoLow) / (out.FiftyTwoHigh - out.FiftyTwoLow) * 100
	}
	addMetricSource(out, "yahoo-quoteSummary", "Yahoo Finance key statistics", sourceURL, out.AsOf,
		"name", "sector", "industry", "price", "trailingPE", "forwardPE", "priceToSales",
		"enterpriseToEbitda", "peg", "priceToBook", "grossMargin", "profitMargin", "roe",
		"revenueGrowth", "dividendYield", "beta", "targetMeanPrice", "recommendation",
		"fiftyTwoWeekHigh", "fiftyTwoWeekLow", "fiftyTwoWeekPos")
	FinalizeStockMetricsEvidence(out, time.Now())
	return out, nil
}

// GetNews fetches related headlines via Yahoo search (no crumb).
func (c *YFinanceClient) GetNews(ticker string, limit int) ([]NewsItem, error) {
	ticker = ToYahooSymbol(strings.TrimSpace(strings.ToUpper(ticker)))
	if limit <= 0 {
		limit = 8
	}
	u := fmt.Sprintf("%s/v1/finance/search?q=%s&quotesCount=1&newsCount=%d",
		yfinanceBaseURL, url.QueryEscape(ticker), limit)
	body, err := c.doRequest(u, false)
	if err != nil {
		return nil, err
	}
	var resp struct {
		News []struct {
			Title               string `json:"title"`
			Publisher           string `json:"publisher"`
			Link                string `json:"link"`
			ProviderPublishTime int64  `json:"providerPublishTime"`
		} `json:"news"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse news: %w", err)
	}
	out := make([]NewsItem, 0, len(resp.News))
	for _, n := range resp.News {
		if strings.TrimSpace(n.Title) == "" {
			continue
		}
		out = append(out, NewsItem{
			Title:     n.Title,
			Source:    firstNonEmpty(n.Publisher, "Yahoo Finance"),
			Link:      n.Link,
			Published: n.ProviderPublishTime,
		})
	}
	return out, nil
}

// GetFinancialStatement fetches a specific financial statement type.
func (c *YFinanceClient) GetFinancialStatement(ticker, stmtType string) (string, error) {
	ticker = ToYahooSymbol(ticker)
	body, err := c.quoteSummary(ticker, stmtType)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// GetStockDataFormatted returns stock data as a formatted string for LLM consumption.
func (c *YFinanceClient) GetStockDataFormatted(ticker, tradeDate string) (string, error) {
	end, err := time.Parse("2006-01-02", tradeDate)
	if err != nil {
		return "", err
	}
	start := end.AddDate(0, -2, 0)

	bars, err := c.GetHistoricalData(ticker, start.Format("2006-01-02"), tradeDate)
	if err != nil {
		return "", fmt.Errorf("historical data: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Stock Data for %s (last %d trading days):\n", ticker, len(bars)))
	sb.WriteString("Provider: Yahoo Finance Chart\n")
	sb.WriteString(fmt.Sprintf("Source URL: %s\n", historicalDataURL(ticker, start, end)))
	sb.WriteString("Date,Open,High,Low,Close,Volume\n")
	for _, bar := range bars {
		sb.WriteString(fmt.Sprintf("%s,%.2f,%.2f,%.2f,%.2f,%d\n",
			bar.Date, bar.Open, bar.High, bar.Low, bar.Close, bar.Volume))
	}
	return sb.String(), nil
}

// GetInsiderTransactions returns insider transaction data.
func (c *YFinanceClient) GetInsiderTransactions(ticker string) (string, error) {
	ticker = ToYahooSymbol(ticker)
	body, err := c.quoteSummary(ticker, "insiderTransactions")
	if err != nil {
		return fmt.Sprintf("Insider transactions unavailable for %s: %v", ticker, err), nil
	}
	return string(body), nil
}

func (c *YFinanceClient) quoteSummary(ticker, modules string) ([]byte, error) {
	if err := c.ensureCrumb(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	crumb := c.crumb
	c.mu.Unlock()
	u := fmt.Sprintf("%s/v10/finance/quoteSummary/%s?modules=%s&crumb=%s",
		yfinanceBaseURL, url.PathEscape(ticker), url.QueryEscape(modules), url.QueryEscape(crumb))
	body, err := c.doRequest(u, true)
	if err != nil && strings.Contains(err.Error(), "401") {
		c.invalidateCrumb()
		if err2 := c.ensureCrumb(); err2 != nil {
			return nil, err
		}
		c.mu.Lock()
		crumb = c.crumb
		c.mu.Unlock()
		u = fmt.Sprintf("%s/v10/finance/quoteSummary/%s?modules=%s&crumb=%s",
			yfinanceBaseURL, url.PathEscape(ticker), url.QueryEscape(modules), url.QueryEscape(crumb))
		return c.doRequest(u, true)
	}
	return body, err
}

func (c *YFinanceClient) doRequest(rawURL string, auth bool) ([]byte, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json,text/plain,*/*")
	if auth {
		req.Header.Set("Origin", "https://finance.yahoo.com")
		req.Header.Set("Referer", "https://finance.yahoo.com/")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		log.Printf("[YFinance] %s returned %d: %s", redactCrumb(rawURL), resp.StatusCode, snippet)
		return nil, fmt.Errorf("HTTP %d from Yahoo Finance", resp.StatusCode)
	}
	return body, nil
}

func redactCrumb(u string) string {
	if i := strings.Index(u, "crumb="); i >= 0 {
		return u[:i] + "crumb=***"
	}
	return u
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
