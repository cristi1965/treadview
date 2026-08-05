package dataflows

import (
	"fmt"
	"strings"
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
	}
}

// Execute runs a named tool with the given arguments.
func (r *ToolRegistry) Execute(name string, args map[string]any) (string, error) {
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
		articles, err := r.news.GetTickerNews(ticker, 20)
		if err != nil {
			return fmt.Sprintf("News unavailable for %s: %v", ticker, err), nil
		}
		return FormatNewsForLLM(articles, fmt.Sprintf("News for %s", ticker)), nil

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
	sb.WriteString(fmt.Sprintf("Technical Indicators for %s (as of %s):\n\n", r.ticker, r.date))

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
// Uses TradingView's symbol-search API to determine the correct exchange.
func toTVSymbol(ticker string) string {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))

	// Already formatted
	if strings.Contains(ticker, ":") {
		return ticker
	}

	// Known exchange mappings for common tickers
	amexTickers := map[string]bool{
		"SOXL": true, "SOXS": true, "TQQQ": true, "SQQQ": true,
		"SPXL": true, "SPXS": true, "TMF": true, "TMV": true,
		"UPRO": true, "SPXU": true, "TNA": true, "TZA": true,
		"QLD": true, "QID": true, "SSO": true, "SDS": true,
		"UDOW": true, "SDOW": true, "TECL": true, "TECS": true,
		"MIDU": true, "MIDD": true, "URE": true, "SRS": true,
		"DRN": true, "DRV": true, "CURE": true, "DRIP": true,
		"USLV": true, "DSLV": true, "AGQ": true, "ZSL": true,
		"UGL": true, "GLL": true, "YINN": true, "YANG": true,
		"EDC": true, "EDZ": true, "ERX": true, "ERY": true,
		"FAS": true, "FAZ": true, "LABU": true, "LABD": true,
		"NUGT": true, "DUST": true, "JNUG": true, "JDST": true,
		"BZQ": true, "DGP": true, "DZZ": true, "GLD": true,
		"SLV": true, "USO": true, "UNG": true, "UCO": true,
		"BOIL": true, "KOLD": true, "SPY": true, "IVV": true,
		"SH": true, "RSP": true,
		// VIX-linked
		"UVXY": true, "SVXY": true, "VIXY": true, "VXX": true,
	}
	if amexTickers[ticker] {
		return "AMEX:" + ticker
	}

	// Default to NASDAQ (works for most common stocks)
	return "NASDAQ:" + ticker
}

// MarketToolDefs returns the tool definitions for the Market Analyst.
func MarketToolDefs() []llm.ToolDef {
	return []llm.ToolDef{
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
}

// FundamentalsToolDefs returns tool definitions for the Fundamentals Analyst.
func FundamentalsToolDefs() []llm.ToolDef {
	return []llm.ToolDef{
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
}

// NewsToolDefs returns tool definitions for the News Analyst.
func NewsToolDefs() []llm.ToolDef {
	return []llm.ToolDef{
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
}

// SentimentToolDefs returns tool definitions for the Sentiment Analyst.
func SentimentToolDefs() []llm.ToolDef {
	return []llm.ToolDef{
		{
			Name:        "get_news",
			Description: "Get recent news articles for sentiment analysis",
			Parameters: map[string]*llm.SchemaParam{
				"ticker": {Type: "string", Description: "Stock ticker symbol"},
			},
			Required: []string{"ticker"},
		},
	}
}


// Helper functions

func formatFundamentals(f *FundamentalData, ticker string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Company Fundamentals for %s:\n\n", ticker))
	sb.WriteString(fmt.Sprintf("Sector: %s | Industry: %s\n", f.Sector, f.Industry))
	sb.WriteString(fmt.Sprintf("Employees: %d\n", f.FullTimeEmployees))
	if f.LongBusinessSummary != "" {
		summary := f.LongBusinessSummary
		if len(summary) > 500 {
			summary = summary[:500] + "..."
		}
		sb.WriteString(fmt.Sprintf("Business Summary: %s\n\n", summary))
	}
	sb.WriteString("| Metric | Value |\n|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Market Cap | %.0f |\n", f.MarketCap))
	sb.WriteString(fmt.Sprintf("| Enterprise Value | %.0f |\n", f.EnterpriseValue))
	sb.WriteString(fmt.Sprintf("| Profit Margin | %.2f%% |\n", f.ProfitMargin*100))
	sb.WriteString(fmt.Sprintf("| Operating Margin | %.2f%% |\n", f.OperatingMargin*100))
	sb.WriteString(fmt.Sprintf("| ROE | %.2f%% |\n", f.ReturnOnEquity*100))
	sb.WriteString(fmt.Sprintf("| ROA | %.2f%% |\n", f.ReturnOnAssets*100))
	sb.WriteString(fmt.Sprintf("| Revenue Growth | %.2f%% |\n", f.RevenueGrowth*100))
	sb.WriteString(fmt.Sprintf("| Earnings Growth | %.2f%% |\n", f.EarningsGrowth*100))
	sb.WriteString(fmt.Sprintf("| Debt/Equity | %.2f |\n", f.DebtToEquity))
	sb.WriteString(fmt.Sprintf("| Current Ratio | %.2f |\n", f.CurrentRatio))
	sb.WriteString(fmt.Sprintf("| Book Value | %.2f |\n", f.BookValue))
	sb.WriteString(fmt.Sprintf("| Free Cash Flow | %.0f |\n", f.FreeCashflow))
	sb.WriteString(fmt.Sprintf("| Total Revenue | %.0f |\n", f.TotalRevenue))
	sb.WriteString(fmt.Sprintf("| EBITDA | %.0f |\n", f.EBITDA))
	return sb.String()
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
