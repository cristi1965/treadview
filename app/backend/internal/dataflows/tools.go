package dataflows

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"trading-agents/internal/llm"
)

// ToolRegistry provides the mapping between LLM tool names and their
// actual Go implementations.
// Tool definitions for function calling.
type ToolRegistry struct {
	yfinance   *YFinanceClient
	tvrest     *TVRestClient
	news       *NewsClient
	fred       *FREDClient
	ticker     string
	date       string
	evidenceMu sync.Mutex
	evidence   []ToolInvocation
	cacheMu    sync.RWMutex
	cache      map[string]toolCacheEntry
}

type toolCacheEntry struct {
	output string
	err    error
}

// ToolInvocation is the actual, hash-addressed tool input/output metadata for an analysis run.
type ToolInvocation struct {
	Name             string            `json:"name"`
	Inputs           map[string]string `json:"inputs"`
	DataTime         string            `json:"data_time"`
	TimeGranularity  string            `json:"time_granularity,omitempty"`
	ObservationTimes []string          `json:"observation_times,omitempty"`
	DisclosureTime   string            `json:"disclosure_time,omitempty"`
	DisclosureTimes  []string          `json:"disclosure_times,omitempty"`
	Provider         string            `json:"provider,omitempty"`
	SourceURL        string            `json:"source_url,omitempty"`
	CompletedAt      string            `json:"completed_at"`
	Status           string            `json:"status"`
	ContentHash      string            `json:"content_hash"`
	PayloadExcerpt   string            `json:"payload_excerpt,omitempty"`
	PayloadTruncated bool              `json:"payload_truncated,omitempty"`
	PayloadRef       string            `json:"payload_ref,omitempty"`
	PayloadSize      int64             `json:"payload_size,omitempty"`
	MethodVersion    string            `json:"method_version"`
	RawPayload       string            `json:"-"`
}

// NewToolRegistry creates a registry bound to a specific ticker and date.
func NewToolRegistry(ticker, tradeDate, fredAPIKey string) *ToolRegistry {
	return &ToolRegistry{
		yfinance: NewYFinanceClient(),
		tvrest:   NewTVRestClient(),
		news:     NewNewsClient(),
		fred:     NewFREDClient(fredAPIKey),
		ticker:   ticker,
		date:     tradeDate,
		cache:    map[string]toolCacheEntry{},
	}
}

// Execute runs a named tool with the given arguments.
func (r *ToolRegistry) Execute(name string, args map[string]any) (output string, err error) {
	if entry, ok := r.cachedInvocation(name, args); ok {
		output, err = entry.output, entry.err
		r.recordInvocation(name, args, output, err)
		return output, err
	}
	defer func() { r.recordInvocation(name, args, output, err) }()
	switch name {
	case "get_stock_data":
		ticker := r.argStr(args, "ticker", r.ticker)
		return r.yfinance.GetStockDataFormatted(ticker, r.date)

	case "get_indicators":
		indicators := r.argStr(args, "indicators", "rsi,macd,boll")
		return r.getIndicators(indicators)

	case "get_fundamentals":
		ticker := r.argStr(args, "ticker", r.ticker)
		fund, err := r.yfinance.GetFundamentals(ticker)
		if err != nil {
			return fmt.Sprintf("Fundamentals unavailable for %s: %v", ticker, err), nil
		}
		return formatFundamentals(fund, ticker), nil

	case "get_balance_sheet":
		ticker := r.argStr(args, "ticker", r.ticker)
		return r.yfinance.GetFinancialStatement(ticker, "balanceSheetHistory")

	case "get_cashflow":
		ticker := r.argStr(args, "ticker", r.ticker)
		return r.yfinance.GetFinancialStatement(ticker, "cashflowStatementHistory")

	case "get_income_statement":
		ticker := r.argStr(args, "ticker", r.ticker)
		return r.yfinance.GetFinancialStatement(ticker, "incomeStatementHistory")

	case "get_news":
		ticker := r.argStr(args, "ticker", r.ticker)
		companyName := USInstrumentName(ticker)
		provider := "Yahoo Finance RSS"
		sourceURL := TickerNewsSourceURL(ticker)
		articles, err := r.news.GetTickerNews(ticker, 20)
		if err == nil {
			articles = filterNewsForTradeDate(articles, r.date, researchNewsMaxAge)
		}
		if err != nil || len(articles) == 0 {
			provider = "Google News RSS"
			articles, sourceURL, err = r.news.GetTickerNewsFromGoogle(ticker, companyName, 100)
			if err == nil {
				articles = filterNewsForTradeDate(articles, r.date, researchNewsMaxAge)
			}
		}
		if err != nil {
			return fmt.Sprintf("News unavailable for %s: %v", ticker, err), nil
		}
		if len(articles) == 0 {
			return fmt.Sprintf("News unavailable for %s: no dated articles within the requested point-in-time window", ticker), nil
		}
		review := ReviewTickerNews(ticker, companyName, articles)
		if review.IncludedCount == 0 {
			return fmt.Sprintf("News unavailable for %s: no ticker-relevant dated articles after deterministic review", ticker), nil
		}
		return FormatReviewedNewsForEvidence(review, provider, sourceURL), nil

	case "get_historical_evidence":
		ticker := r.argStr(args, "ticker", r.ticker)
		evidence, err := FetchNasdaqHistoricalEvidence(r.yfinance.httpClient, ticker, r.date)
		if err != nil {
			return fmt.Sprintf("Historical evidence unavailable for %s: %v", ticker, err), nil
		}
		return FormatHistoricalEvidence(evidence), nil

	case "get_x_player_takes":
		ticker := r.argStr(args, "ticker", r.ticker)
		takes, err := r.news.GetXPlayerTakes(ticker, 16)
		if err != nil {
			return fmt.Sprintf("X/player takes unavailable for %s: %v", ticker, err), nil
		}
		return FormatPlayerTakesForLLM(ticker, takes), nil

	case "get_global_news":
		queries := []string{
			"Federal Reserve interest rates inflation",
			"S&P 500 earnings GDP economic outlook",
			"geopolitical risk trade war sanctions",
		}
		articles, err := r.news.GetGlobalNews(queries, 10)
		if err != nil {
			return fmt.Sprintf("Global news unavailable: %v", err), nil
		}
		return FormatNewsForLLM(articles, "Global/Macro News"), nil

	case "get_insider_transactions":
		ticker := r.argStr(args, "ticker", r.ticker)
		return r.yfinance.GetInsiderTransactions(ticker)

	case "get_macro_indicators":
		return r.fred.GetMacroIndicators()

	case "get_prediction_markets":
		return "Prediction market data: currently unavailable in Go backend. Use general market sentiment analysis.", nil

	case "get_verified_market_snapshot":
		return r.getVerifiedSnapshot()

	case "get_realtime_quote":
		ticker := r.argStr(args, "ticker", r.ticker)
		return r.getRealtimeQuote(ticker)

	case "get_realtime_batch":
		symbols := r.argStr(args, "symbols", r.ticker)
		return r.getRealtimeBatch(symbols)

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// Prime executes a required source once and makes the exact response reusable by later LLM tool calls.
func (r *ToolRegistry) Prime(name string, args map[string]any) (string, error) {
	output, err := r.Execute(name, args)
	if isPreflightTool(name) {
		r.cacheMu.Lock()
		r.cache[r.cacheKey(name, args)] = toolCacheEntry{output: output, err: err}
		r.cacheMu.Unlock()
	}
	return output, err
}

func (r *ToolRegistry) cachedInvocation(name string, args map[string]any) (toolCacheEntry, bool) {
	if !isPreflightTool(name) {
		return toolCacheEntry{}, false
	}
	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()
	entry, ok := r.cache[r.cacheKey(name, args)]
	return entry, ok
}

// PreflightPayload returns the exact cached response captured before model execution.
func (r *ToolRegistry) PreflightPayload(name string) (string, bool) {
	entry, ok := r.cachedInvocation(name, map[string]any{"ticker": r.ticker})
	return entry.output, ok && entry.err == nil
}

func (r *ToolRegistry) cacheKey(name string, args map[string]any) string {
	return name + "|" + strings.ToUpper(strings.TrimSpace(r.argStr(args, "ticker", r.ticker)))
}

func isPreflightTool(name string) bool {
	return name == "get_stock_data" || name == "get_fundamentals" || name == "get_news" || name == "get_historical_evidence"
}

// EvidenceSnapshot returns a defensive copy of actual tool invocations in call order.
func (r *ToolRegistry) EvidenceSnapshot() []ToolInvocation {
	r.evidenceMu.Lock()
	defer r.evidenceMu.Unlock()
	return append([]ToolInvocation{}, r.evidence...)
}

func (r *ToolRegistry) recordInvocation(name string, args map[string]any, output string, callErr error) {
	completedAt := time.Now().UTC()
	inputs := make(map[string]string, len(args)+1)
	for key, value := range args {
		inputs[key] = fmt.Sprint(value)
	}
	if _, ok := inputs["ticker"]; !ok && r.ticker != "" {
		inputs["ticker"] = r.ticker
	}
	if r.date != "" {
		inputs["requested_as_of"] = r.date
	}
	dataTime := inferToolDataTime(name, output)
	timeGranularity := inferToolTimeGranularity(name, output)
	provider, sourceURL, disclosureTime := inferToolProvenance(name, output)
	observationTimes := inferToolObservationTimes(name, output)
	disclosureTimes := inferFundamentalDisclosureTimes(name, output)
	status := toolInvocationStatus(output, callErr)
	sum := sha256.Sum256([]byte(output))
	payloadExcerpt, payloadTruncated := reviewablePayloadExcerpt(output)
	invocation := ToolInvocation{
		Name: name, Inputs: inputs, DataTime: dataTime, TimeGranularity: timeGranularity, CompletedAt: completedAt.Format(time.RFC3339),
		Status: status, ContentHash: "sha256:" + hex.EncodeToString(sum[:]), PayloadExcerpt: payloadExcerpt,
		PayloadTruncated: payloadTruncated, PayloadSize: int64(len(output)), MethodVersion: "tool-registry-v4",
		Provider: provider, SourceURL: sourceURL, DisclosureTime: disclosureTime, DisclosureTimes: disclosureTimes, ObservationTimes: observationTimes,
		RawPayload: output,
	}
	r.evidenceMu.Lock()
	r.evidence = append(r.evidence, invocation)
	r.evidenceMu.Unlock()
}

func toolInvocationStatus(output string, callErr error) string {
	if callErr != nil {
		return "error"
	}
	normalized := strings.ToLower(strings.TrimSpace(output))
	for _, prefix := range []string{
		"fundamentals unavailable", "news unavailable", "global news unavailable", "historical evidence unavailable",
		"indicator data unavailable", "verified snapshot unavailable", "quote unavailable",
		"x/player takes unavailable", "prediction market data: currently unavailable",
		"tradingview batch failed", "no trading data available", "insufficient data",
	} {
		if strings.HasPrefix(normalized, prefix) {
			return "unavailable"
		}
	}
	return "captured"
}

func reviewablePayloadExcerpt(output string) (string, bool) {
	const maxPayloadBytes = 2048
	if len(output) <= maxPayloadBytes {
		return output, false
	}
	const marker = "\n...[payload truncated; verify with content_hash]...\n"
	keep := (maxPayloadBytes - len(marker)) / 2
	return output[:keep] + marker + output[len(output)-keep:], true
}

var outputISOTimePattern = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}(?:T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2}))?\b`)

func inferToolDataTime(name, output string) string {
	if strings.TrimSpace(output) == "" {
		return "unknown"
	}
	if name == "get_news" || name == "get_global_news" {
		if latest := latestPublishedTime(output); !latest.IsZero() {
			return latest.UTC().Format(time.RFC3339)
		}
	}
	if name == "get_fundamentals" {
		if value := metadataValue(output, "As Of"); value != "" {
			if parsed, ok := parseToolTime(value); ok {
				return parsed.Format("2006-01-02")
			}
		}
	}
	if name == "get_historical_evidence" {
		if evidence, err := ParseHistoricalEvidence(output); err == nil {
			return evidence.DataTime
		}
	}
	matches := outputISOTimePattern.FindAllString(output, -1)
	times := make([]time.Time, 0, len(matches))
	for _, raw := range matches {
		if parsed, ok := parseToolTime(raw); ok {
			times = append(times, parsed)
		}
	}
	if len(times) == 0 {
		return "unknown"
	}
	selected := times[0]
	for _, candidate := range times[1:] {
		if name == "get_macro_indicators" || name == "get_fundamentals" {
			if candidate.Before(selected) {
				selected = candidate
			}
		} else if candidate.After(selected) {
			selected = candidate
		}
	}
	if selected.Hour() == 0 && selected.Minute() == 0 && selected.Second() == 0 && !strings.Contains(matches[0], "T") {
		return selected.Format("2006-01-02")
	}
	return selected.UTC().Format(time.RFC3339)
}

func inferToolObservationTimes(name, output string) []string {
	observations := make([]string, 0)
	switch name {
	case "get_news", "get_global_news":
		for _, observed := range publishedTimes(output) {
			observations = append(observations, observed.UTC().Format(time.RFC3339))
		}
	case "get_fundamentals":
		for _, value := range metadataValues(output, "As Of") {
			if observed, ok := parseToolTime(value); ok && !containsString(observations, observed.Format("2006-01-02")) {
				observations = append(observations, observed.Format("2006-01-02"))
			}
		}
	case "get_historical_evidence":
		if evidence, err := ParseHistoricalEvidence(output); err == nil {
			for _, observation := range evidence.Observations {
				observations = append(observations, observation.Date)
			}
		}
	default:
		for _, raw := range outputISOTimePattern.FindAllString(output, -1) {
			if observed, ok := parseToolTime(raw); ok {
				observations = append(observations, observed.Format("2006-01-02"))
			}
		}
	}
	return observations
}

func inferToolProvenance(name, output string) (provider, sourceURL, disclosureTime string) {
	provider = metadataValue(output, "Provider")
	if provider == "" {
		provider = metadataValue(output, "Source")
	}
	sourceURL = metadataValue(output, "Source URL")
	if name == "get_fundamentals" {
		disclosureTime = metadataValue(output, "Filing Date")
	}
	return strings.TrimSpace(provider), strings.TrimSpace(sourceURL), strings.TrimSpace(disclosureTime)
}

func inferToolTimeGranularity(name, output string) string {
	if value := metadataValue(output, "Time Granularity"); value != "" {
		return strings.ToLower(strings.TrimSpace(value))
	}
	if name == "get_news" || name == "get_global_news" {
		return "second"
	}
	if name == "get_stock_data" || name == "get_fundamentals" || name == "get_historical_evidence" {
		return "date"
	}
	return "unknown"
}

func metadataValue(output, key string) string {
	values := metadataValues(output, key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func metadataValues(output, key string) []string {
	pattern := regexp.MustCompile(`(?im)(?:^|\|)\s*(?:-\s*)?` + regexp.QuoteMeta(key) + `:\s*([^|\r\n]+)`)
	matches := pattern.FindAllStringSubmatch(output, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 2 {
			value := strings.TrimSpace(match[1])
			if value != "" && !containsString(out, value) {
				out = append(out, value)
			}
		}
	}
	return out
}

func inferFundamentalDisclosureTimes(name, output string) []string {
	if name != "get_fundamentals" {
		return nil
	}
	out := make([]string, 0)
	for _, value := range metadataValues(output, "Filing Date") {
		if parsed, ok := parseToolTime(value); ok {
			formatted := parsed.Format("2006-01-02")
			if !containsString(out, formatted) {
				out = append(out, formatted)
			}
		}
	}
	return out
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func latestPublishedTime(output string) time.Time {
	latest := time.Time{}
	for _, parsed := range publishedTimes(output) {
		if parsed.After(latest) {
			latest = parsed
		}
	}
	return latest
}

func publishedTimes(output string) []time.Time {
	result := make([]time.Time, 0)
	for _, line := range strings.Split(output, "\n") {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), "Published:")
		if !ok {
			continue
		}
		for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC3339} {
			if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
				result = append(result, parsed)
				break
			}
		}
	}
	return result
}

func parseToolTime(value string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

// FetchXPlayerTakes loads public StockTwits + X-mention chatter for the bound ticker.
func (r *ToolRegistry) FetchXPlayerTakes() string {
	takes, err := r.news.GetXPlayerTakes(r.ticker, 16)
	if err != nil {
		return FormatPlayerTakesForLLM(r.ticker, nil)
	}
	return FormatPlayerTakesForLLM(r.ticker, takes)
}

// getIndicators computes technical indicators from stock data.
func (r *ToolRegistry) getIndicators(indicators string) (string, error) {
	bars, err := r.yfinance.GetHistoricalData(r.ticker,
		subtractDays(r.date, 200), r.date)
	if err != nil {
		return fmt.Sprintf("Indicator data unavailable: %v", err), nil
	}

	if len(bars) < 2 {
		return "Insufficient data for indicator calculation", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Technical Indicators for %s (latest observation %s):\n\n", r.ticker, bars[len(bars)-1].Date))

	requested := strings.Split(indicators, ",")
	closes := extractCloses(bars)
	volumes := extractVolumes(bars)

	for _, ind := range requested {
		ind = strings.TrimSpace(ind)
		switch ind {
		case "rsi":
			rsi := calcRSI(closes, 14)
			sb.WriteString(fmt.Sprintf("RSI(14): %.2f\n", rsi))
		case "macd", "macds", "macdh":
			macdLine, signal, hist := calcMACD(closes)
			sb.WriteString(fmt.Sprintf("MACD Line: %.4f\nMACD Signal: %.4f\nMACD Histogram: %.4f\n",
				macdLine, signal, hist))
		case "close_50_sma":
			sma := calcSMA(closes, 50)
			sb.WriteString(fmt.Sprintf("50 SMA: %.2f\n", sma))
		case "close_200_sma":
			sma := calcSMA(closes, 200)
			sb.WriteString(fmt.Sprintf("200 SMA: %.2f\n", sma))
		case "close_10_ema":
			ema := calcEMA(closes, 10)
			sb.WriteString(fmt.Sprintf("10 EMA: %.2f\n", ema))
		case "boll", "boll_ub", "boll_lb":
			mid, upper, lower := calcBollinger(closes, 20)
			sb.WriteString(fmt.Sprintf("Bollinger Mid: %.2f\nBollinger Upper: %.2f\nBollinger Lower: %.2f\n",
				mid, upper, lower))
		case "atr":
			atr := calcATR(bars, 14)
			sb.WriteString(fmt.Sprintf("ATR(14): %.2f\n", atr))
		case "vwma":
			vwma := calcVWMA(closes, volumes, 20)
			sb.WriteString(fmt.Sprintf("VWMA(20): %.2f\n", vwma))
		default:
			sb.WriteString(fmt.Sprintf("%s: not supported\n", ind))
		}
	}

	return sb.String(), nil
}

func (r *ToolRegistry) getVerifiedSnapshot() (string, error) {
	bars, err := r.yfinance.GetHistoricalData(r.ticker,
		subtractDays(r.date, 5), r.date)
	if err != nil {
		return fmt.Sprintf("Verified snapshot unavailable: %v", err), nil
	}

	if len(bars) == 0 {
		return "No trading data available for verified snapshot", nil
	}

	latest := bars[len(bars)-1]
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("VERIFIED MARKET SNAPSHOT for %s (Date: %s)\n", r.ticker, latest.Date))
	sb.WriteString(fmt.Sprintf("Open: %.2f | High: %.2f | Low: %.2f | Close: %.2f | Volume: %d\n",
		latest.Open, latest.High, latest.Low, latest.Close, latest.Volume))

	if len(bars) >= 2 {
		prev := bars[len(bars)-2]
		change := ((latest.Close - prev.Close) / prev.Close) * 100
		sb.WriteString(fmt.Sprintf("Previous Close: %.2f | Change: %.2f%%\n", prev.Close, change))
	}

	return sb.String(), nil
}

// getRealtimeQuote fetches a real-time quote via TradingView REST API.
func (r *ToolRegistry) getRealtimeQuote(ticker string) (string, error) {
	item, err := r.tvrest.GetRealTimeQuote(ticker)
	if err != nil {
		// Fallback to Yahoo on error
		return r.yahooFallbackQuote(ticker)
	}
	return FormatQuoteForLLM(*item, ticker), nil
}

// getRealtimeBatch fetches real-time quotes for multiple symbols at once.
func (r *ToolRegistry) getRealtimeBatch(symbols string) (string, error) {
	symList := strings.Split(symbols, ",")
	tickers := make([]string, 0, len(symList))
	for _, s := range symList {
		tick := strings.TrimSpace(s)
		if tick != "" {
			tickers = append(tickers, tick)
		}
	}

	items, err := r.tvrest.GetRealTimeQuotes(tickers)
	if err != nil {
		return fmt.Sprintf("TradingView batch failed: %v", err), nil
	}

	return FormatQuotesForLLM(items), nil
}

// yahooFallbackQuote gets a quote from Yahoo Finance as fallback.
func (r *ToolRegistry) yahooFallbackQuote(ticker string) (string, error) {
	quote, err := r.yfinance.GetQuote(ticker)
	if err != nil {
		return fmt.Sprintf("Quote unavailable for %s (both TradingView and Yahoo failed): %v", ticker, err), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("QUOTE for %s (via Yahoo Finance - may be delayed):\n\n", ticker))
	sb.WriteString(fmt.Sprintf("Current Price: $%.2f\n", quote.RegularPrice))
	sb.WriteString(fmt.Sprintf("Previous Close: $%.2f\n", quote.PreviousClose))
	sb.WriteString(fmt.Sprintf("Open: $%.2f\n", quote.Open))
	sb.WriteString(fmt.Sprintf("Day High: $%.2f\n", quote.DayHigh))
	sb.WriteString(fmt.Sprintf("Day Low: $%.2f\n", quote.DayLow))
	sb.WriteString(fmt.Sprintf("Volume: %d\n", quote.Volume))
	sb.WriteString(fmt.Sprintf("\n⚠️ Data may be delayed ~15 minutes.\n"))
	return sb.String(), nil
}

// toTVSymbol converts a standard ticker to TradingView exchange-prefixed format.
func toTVSymbol(ticker string) string {
	return ToTVSymbol(ticker)
}

// MarketToolDefs returns the tool definitions for the Market Analyst.
func MarketToolDefs() []llm.ToolDef {
	defs := []llm.ToolDef{
		{
			Name:        "get_stock_data",
			Description: "Get historical OHLCV stock price data in CSV format for technical analysis",
			Parameters: map[string]*llm.SchemaParam{
				"ticker": {Type: "string", Description: "Stock ticker symbol (e.g., NVDA, AAPL)"},
			},
			Required: []string{"ticker"},
		},
		{
			Name:        "get_indicators",
			Description: "Calculate technical indicators from stock data. Available: rsi, macd, macds, macdh, close_50_sma, close_200_sma, close_10_ema, boll, boll_ub, boll_lb, atr, vwma",
			Parameters: map[string]*llm.SchemaParam{
				"indicators": {Type: "string", Description: "Comma-separated list of indicator names to calculate"},
			},
			Required: []string{"indicators"},
		},
		{
			Name:        "get_verified_market_snapshot",
			Description: "Get a verified OHLCV snapshot for the current trading date. Use this as the source of truth.",
			Parameters:  map[string]*llm.SchemaParam{},
		},
		{
			Name:        "get_realtime_quote",
			Description: "Get REAL-TIME stock quote with current price, change, volume, day high/low via TradingView WebSocket (millisecond latency). Always use this for current price instead of historical data.",
			Parameters: map[string]*llm.SchemaParam{
				"ticker": {Type: "string", Description: "Stock ticker symbol (e.g., NVDA, AAPL, SOXL)"},
			},
			Required: []string{"ticker"},
		},
		{
			Name:        "get_realtime_batch",
			Description: "Get real-time quotes for multiple symbols at once. Use this to compare prices or get sector-wide data simultaneously.",
			Parameters: map[string]*llm.SchemaParam{
				"symbols": {Type: "string", Description: "Comma-separated ticker symbols (e.g., 'AAPL,MSFT,NVDA,SOXL')"},
			},
			Required: []string{"symbols"},
		},
	}
	return defs[:3]
}

// FundamentalsToolDefs returns tool definitions for the Fundamentals Analyst.
func FundamentalsToolDefs() []llm.ToolDef {
	defs := []llm.ToolDef{
		{
			Name:        "get_fundamentals",
			Description: "Get comprehensive company fundamentals including profile, financial ratios, and key metrics",
			Parameters: map[string]*llm.SchemaParam{
				"ticker": {Type: "string", Description: "Stock ticker symbol"},
			},
			Required: []string{"ticker"},
		},
		{
			Name:        "get_balance_sheet",
			Description: "Get the company's balance sheet data",
			Parameters: map[string]*llm.SchemaParam{
				"ticker": {Type: "string", Description: "Stock ticker symbol"},
			},
			Required: []string{"ticker"},
		},
		{
			Name:        "get_cashflow",
			Description: "Get the company's cash flow statement data",
			Parameters: map[string]*llm.SchemaParam{
				"ticker": {Type: "string", Description: "Stock ticker symbol"},
			},
			Required: []string{"ticker"},
		},
		{
			Name:        "get_income_statement",
			Description: "Get the company's income statement data",
			Parameters: map[string]*llm.SchemaParam{
				"ticker": {Type: "string", Description: "Stock ticker symbol"},
			},
			Required: []string{"ticker"},
		},
	}
	return defs[:1]
}

// NewsToolDefs returns tool definitions for the News Analyst.
func NewsToolDefs() []llm.ToolDef {
	defs := []llm.ToolDef{
		{
			Name:        "get_news",
			Description: "Get recent news articles for a specific ticker",
			Parameters: map[string]*llm.SchemaParam{
				"ticker": {Type: "string", Description: "Stock ticker symbol"},
			},
			Required: []string{"ticker"},
		},
		{
			Name:        "get_global_news",
			Description: "Get global macroeconomic and market news",
			Parameters:  map[string]*llm.SchemaParam{},
		},
		{
			Name:        "get_insider_transactions",
			Description: "Get insider trading transactions for a ticker",
			Parameters: map[string]*llm.SchemaParam{
				"ticker": {Type: "string", Description: "Stock ticker symbol"},
			},
			Required: []string{"ticker"},
		},
		{
			Name:        "get_macro_indicators",
			Description: "Get key macroeconomic indicators (Fed rate, CPI, unemployment, GDP, treasury yields)",
			Parameters:  map[string]*llm.SchemaParam{},
		},
		{
			Name:        "get_prediction_markets",
			Description: "Get prediction market data for forward-looking sentiment",
			Parameters:  map[string]*llm.SchemaParam{},
		},
	}
	return defs[:1]
}

// SentimentToolDefs returns tool definitions for the Sentiment Analyst.
func SentimentToolDefs() []llm.ToolDef {
	defs := []llm.ToolDef{
		{
			Name:        "get_news",
			Description: "Get recent news articles for sentiment analysis",
			Parameters: map[string]*llm.SchemaParam{
				"ticker": {Type: "string", Description: "Stock ticker symbol"},
			},
			Required: []string{"ticker"},
		},
		{
			Name:        "get_x_player_takes",
			Description: "Get public stock-player posts: StockTwits stream plus X/Twitter mentions indexed by Google News. Use this to contrast retail/trader chatter with the model view. Not official X API.",
			Parameters: map[string]*llm.SchemaParam{
				"ticker": {Type: "string", Description: "Stock ticker symbol"},
			},
			Required: []string{"ticker"},
		},
	}
	return defs[:1]
}

// Helper functions

func formatFundamentals(f *FundamentalData, ticker string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Company Fundamentals for %s:\n\n", ticker))
	sb.WriteString(fmt.Sprintf("Source: %s | Fiscal Period: %s | As Of: %s | Filing Date: %s | Accession: %s\n", f.Source, f.FiscalPeriod, f.AsOf, f.FilingDate, f.Accession))
	if f.SourceURL != "" {
		sb.WriteString(fmt.Sprintf("Source URL: %s\n", f.SourceURL))
	}
	if f.SourceFetchedAt != "" {
		sb.WriteString(fmt.Sprintf("Source Fetched At: %s | Source Transport: %s | Source Content Hash: %s\n", f.SourceFetchedAt, f.SourceTransport, f.SourceContentHash))
	}
	if len(f.Periods) > 0 {
		sb.WriteString("Financial Periods:\n")
		for _, period := range f.Periods {
			sb.WriteString(fmt.Sprintf("- Fiscal Period: %s | Frequency: %s | Form: %s | Period Start: %s | As Of: %s | Filing Date: %s | Accession: %s | Source URL: %s\n", period.FiscalPeriod, period.Frequency, period.Form, period.PeriodStart, period.PeriodEnd, period.FilingDate, period.Accession, period.SourceURL))
			sb.WriteString(fmt.Sprintf("  Values: totalRevenue=%s grossProfit=%s operatingIncome=%s netIncome=%s totalAssets=%s totalLiabilities=%s stockholdersEquity=%s cashAndEquivalents=%s sharesOutstanding=%s\n",
				formatPeriodMetric(period, "totalRevenue", period.TotalRevenue), formatPeriodMetric(period, "grossProfit", period.GrossProfit),
				formatPeriodMetric(period, "operatingIncome", period.OperatingIncome), formatPeriodMetric(period, "netIncome", period.NetIncome),
				formatPeriodMetric(period, "totalAssets", period.TotalAssets), formatPeriodMetric(period, "totalLiabilities", period.TotalLiabilities),
				formatPeriodMetric(period, "stockholdersEquity", period.StockholdersEquity), formatPeriodMetric(period, "cashAndEquivalents", period.CashAndEquivalents),
				formatPeriodMetric(period, "sharesOutstanding", period.SharesOutstanding)))
		}
	}
	if len(f.DerivationInputs) > 0 {
		sb.WriteString("Financial Derivation Inputs (not discrete quarters):\n")
		for _, period := range f.DerivationInputs {
			sb.WriteString(fmt.Sprintf("- Fiscal Period: %s | Frequency: %s | Form: %s | Period Start: %s | As Of: %s | Filing Date: %s | Accession: %s | Source URL: %s\n", period.FiscalPeriod, period.Frequency, period.Form, period.PeriodStart, period.PeriodEnd, period.FilingDate, period.Accession, period.SourceURL))
			sb.WriteString(fmt.Sprintf("  Values: totalRevenue=%s netIncome=%s\n", formatPeriodMetric(period, "totalRevenue", period.TotalRevenue), formatPeriodMetric(period, "netIncome", period.NetIncome)))
		}
	}
	sb.WriteString(fmt.Sprintf("Sector: %s | Industry: %s\n", f.Sector, f.Industry))
	if f.FullTimeEmployees != nil {
		sb.WriteString(fmt.Sprintf("Employees: %d\n", *f.FullTimeEmployees))
	}
	if f.LongBusinessSummary != "" {
		summary := f.LongBusinessSummary
		if len(summary) > 500 {
			summary = summary[:500] + "..."
		}
		sb.WriteString(fmt.Sprintf("Business Summary: %s\n\n", summary))
	}
	sb.WriteString("| Metric | Value |\n|--------|-------|\n")
	writeMetric := func(label, format string, value *float64) {
		if value != nil {
			sb.WriteString(fmt.Sprintf("| %s | "+format+" |\n", label, *value))
		}
	}
	writeMetric("Market Cap", "%.0f", f.MarketCap)
	writeMetric("Enterprise Value", "%.0f", f.EnterpriseValue)
	writeMetric("Profit Margin", "%.4f", f.ProfitMargin)
	writeMetric("Operating Margin", "%.4f", f.OperatingMargin)
	writeMetric("ROE", "%.4f", f.ReturnOnEquity)
	writeMetric("ROA", "%.4f", f.ReturnOnAssets)
	writeMetric("Revenue Growth", "%.4f", f.RevenueGrowth)
	writeMetric("Earnings Growth", "%.4f", f.EarningsGrowth)
	writeMetric("Debt/Equity", "%.2f", f.DebtToEquity)
	writeMetric("Current Ratio", "%.2f", f.CurrentRatio)
	writeMetric("Book Value", "%.2f", f.BookValue)
	writeMetric("Free Cash Flow", "%.0f", f.FreeCashflow)
	writeMetric("Total Revenue", "%.0f", f.TotalRevenue)
	writeMetric("EBITDA", "%.0f", f.EBITDA)
	raw, _ := json.Marshal(f)
	sb.WriteString("Fundamental Evidence JSON: ")
	sb.Write(raw)
	sb.WriteByte('\n')
	return sb.String()
}

func formatPeriodMetric(period FinancialPeriod, field string, value float64) string {
	for _, available := range period.AvailableFields {
		if available == field {
			return fmt.Sprintf("%.0f", value)
		}
	}
	return "unknown"
}

// FormatFundamentalsForEvidence exposes the canonical structured review format
// to fixture builders without duplicating the serialization contract.
func FormatFundamentalsForEvidence(f *FundamentalData, ticker string) string {
	return formatFundamentals(f, ticker)
}

func ParseFundamentalEvidence(output string) (FundamentalData, error) {
	const marker = "Fundamental Evidence JSON: "
	index := strings.Index(output, marker)
	if index < 0 {
		return FundamentalData{}, fmt.Errorf("structured fundamental evidence is missing")
	}
	raw := strings.TrimSpace(output[index+len(marker):])
	var evidence FundamentalData
	if err := json.Unmarshal([]byte(raw), &evidence); err != nil {
		return FundamentalData{}, err
	}
	return evidence, nil
}

func (r *ToolRegistry) argStr(args map[string]any, key, fallback string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return fallback
}

func subtractDays(dateStr string, days int) string {
	t, err := parseDate(dateStr)
	if err != nil {
		return dateStr
	}
	return t.AddDate(0, 0, -days).Format("2006-01-02")
}

func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

func extractCloses(bars []HistoricalBar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.Close
	}
	return out
}

func extractVolumes(bars []HistoricalBar) []int64 {
	out := make([]int64, len(bars))
	for i, b := range bars {
		out[i] = b.Volume
	}
	return out
}
