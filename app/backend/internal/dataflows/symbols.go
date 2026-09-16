package dataflows

import (
	"fmt"
	"strings"
	"unicode"
)

// MarketRegion represents the geographic/asset market for a stock.
type MarketRegion string

const (
	MarketUS     MarketRegion = "US"
	MarketHK     MarketRegion = "HK"
	MarketCN     MarketRegion = "CN"
	MarketJP     MarketRegion = "JP"
	MarketTW     MarketRegion = "TW"
	MarketUK     MarketRegion = "UK"
	MarketEU     MarketRegion = "EU"
	MarketCrypto MarketRegion = "CRYPTO"
)

// KnownPopularStocks maps shorthand/popular codes to metadata.
var KnownGlobalNames = map[string]string{
	"01810":   "小米集团-W",
	"1810":    "小米集团-W",
	"00700":   "腾讯控股",
	"0700":    "腾讯控股",
	"09988":   "阿里巴巴-W",
	"9988":    "阿里巴巴-W",
	"03690":   "美团-W",
	"3690":    "美团-W",
	"09618":   "京东集团-SW",
	"9618":    "京东集团-SW",
	"00981":   "中芯国际",
	"0981":    "中芯国际",
	"01024":   "快手-W",
	"1024":    "快手-W",
	"09868":   "小鹏汽车-W",
	"9868":    "小鹏汽车-W",
	"02015":   "理想汽车-W",
	"2015":    "理想汽车-W",
	"09866":   "蔚来-SW",
	"9866":    "蔚来-SW",
	"00005":   "汇丰控股",
	"0005":    "汇丰控股",
	"00388":   "香港交易所",
	"0388":    "香港交易所",
	"02269":   "药明生物",
	"01211":   "比亚迪股份",
	"2330.TW": "台积电",
	"7203.T":  "丰田汽车",
	"9984.T":  "软银集团",
	"6758.T":  "索尼",
	"SHEL.L":  "壳牌石油",
	"SAP.DE":  "SAP软件",
	"BTC-USD": "比特币",
	"ETH-USD": "以太坊",
}

// DetectMarket identifies the market exchange for a given symbol.
func DetectMarket(sym string) MarketRegion {
	s := strings.TrimSpace(strings.ToUpper(sym))
	if s == "" {
		return MarketUS
	}

	// Suffix matching
	if strings.HasSuffix(s, ".HK") || strings.HasPrefix(s, "HKEX:") || strings.HasPrefix(s, "HK:") {
		return MarketHK
	}
	if strings.HasSuffix(s, ".SS") || strings.HasSuffix(s, ".SZ") || strings.HasSuffix(s, ".BJ") ||
		strings.HasPrefix(s, "SSE:") || strings.HasPrefix(s, "SZSE:") || strings.HasPrefix(s, "BSE:") ||
		strings.HasPrefix(s, "SH") || strings.HasPrefix(s, "SZ") {
		return MarketCN
	}
	if strings.HasSuffix(s, ".TW") || strings.HasPrefix(s, "TWSE:") {
		return MarketTW
	}
	if strings.HasSuffix(s, ".T") || strings.HasPrefix(s, "TSE:") {
		return MarketJP
	}
	if strings.HasSuffix(s, ".L") || strings.HasPrefix(s, "LSE:") {
		return MarketUK
	}
	if strings.HasSuffix(s, ".DE") || strings.HasPrefix(s, "XETR:") {
		return MarketEU
	}
	if strings.HasSuffix(s, "-USD") || strings.HasSuffix(s, "USDT") || s == "BTC" || s == "ETH" || s == "SOL" {
		return MarketCrypto
	}

	// Pure numeric codes
	if isPureDigits(s) {
		if len(s) == 6 {
			// China A-Shares: 60xxxx, 688xxx, 00xxxx, 30xxxx, 8xxxxx, 4xxxxx
			return MarketCN
		}
		if len(s) <= 5 {
			// Hong Kong HKEX: 01810, 00700, 1810, 700, 9988, 3690
			return MarketHK
		}
	}

	return MarketUS
}

// ToYahooSymbol converts any input symbol into Yahoo Finance format.
func ToYahooSymbol(sym string) string {
	s := strings.TrimSpace(strings.ToUpper(sym))
	if s == "" {
		return s
	}

	// Strip exchange prefixes
	if idx := strings.Index(s, ":"); idx >= 0 {
		prefix := s[:idx]
		code := s[idx+1:]
		switch prefix {
		case "HKEX", "HK":
			return formatHKYahoo(code)
		case "SSE", "SH":
			return code + ".SS"
		case "SZSE", "SZ":
			return code + ".SZ"
		case "BSE", "BJ":
			return code + ".BJ"
		case "TWSE":
			return code + ".TW"
		case "TSE":
			return code + ".T"
		case "LSE":
			return code + ".L"
		case "XETR":
			return code + ".DE"
		case "BINANCE", "COINBASE":
			if strings.HasSuffix(code, "USDT") {
				return strings.TrimSuffix(code, "USDT") + "-USD"
			}
			return code
		default:
			return code
		}
	}

	// Suffixes already present
	if strings.Contains(s, ".") {
		if strings.HasSuffix(s, ".SH") {
			return strings.TrimSuffix(s, ".SH") + ".SS"
		}
		return s
	}

	// Pure numeric
	if isPureDigits(s) {
		if len(s) == 6 {
			// A-Share: Shanghai 60/68/5, Shenzhen 00/30/1, Beijing 8/4/9
			if strings.HasPrefix(s, "6") || strings.HasPrefix(s, "5") {
				return s + ".SS"
			}
			if strings.HasPrefix(s, "8") || strings.HasPrefix(s, "4") || strings.HasPrefix(s, "92") {
				return s + ".BJ"
			}
			return s + ".SZ"
		}
		if len(s) <= 5 {
			return formatHKYahoo(s)
		}
	}

	// Crypto shorthand
	if s == "BTC" || s == "ETH" || s == "SOL" || s == "DOGE" || s == "BNB" {
		return s + "-USD"
	}
	if strings.HasSuffix(s, "USDT") {
		return strings.TrimSuffix(s, "USDT") + "-USD"
	}

	// Standard US Ticker
	return s
}

// formatHKYahoo turns 01810 -> 1810.HK, 00700 -> 0700.HK, 00005 -> 0005.HK
func formatHKYahoo(code string) string {
	code = strings.TrimSpace(code)
	if strings.HasSuffix(code, ".HK") {
		return code
	}
	// Yahoo accepts 4-digit format: e.g. 1810.HK, 0700.HK, 9988.HK, 0005.HK
	if len(code) == 5 && strings.HasPrefix(code, "0") {
		code = code[1:]
	} else if len(code) < 4 {
		code = fmt.Sprintf("%04s", code)
	}
	return code + ".HK"
}

// ToTVSymbol converts any input symbol into TradingView exchange:symbol format.
func ToTVSymbol(sym string) string {
	s := strings.TrimSpace(strings.ToUpper(sym))
	if s == "" {
		return s
	}

	// Already formatted
	if strings.Contains(s, ":") {
		return s
	}

	// Suffix based
	if strings.HasSuffix(s, ".HK") {
		code := strings.TrimSuffix(s, ".HK")
		return "HKEX:" + cleanHKTVCode(code)
	}
	if strings.HasSuffix(s, ".SS") || strings.HasSuffix(s, ".SH") {
		code := strings.TrimSuffix(strings.TrimSuffix(s, ".SS"), ".SH")
		return "SSE:" + code
	}
	if strings.HasSuffix(s, ".SZ") {
		code := strings.TrimSuffix(s, ".SZ")
		return "SZSE:" + code
	}
	if strings.HasSuffix(s, ".BJ") {
		code := strings.TrimSuffix(s, ".BJ")
		return "BSE:" + code
	}
	if strings.HasSuffix(s, ".TW") {
		code := strings.TrimSuffix(s, ".TW")
		return "TWSE:" + code
	}
	if strings.HasSuffix(s, ".T") {
		code := strings.TrimSuffix(s, ".T")
		return "TSE:" + code
	}
	if strings.HasSuffix(s, ".L") {
		code := strings.TrimSuffix(s, ".L")
		return "LSE:" + code
	}
	if strings.HasSuffix(s, ".DE") {
		code := strings.TrimSuffix(s, ".DE")
		return "XETR:" + code
	}
	if strings.HasSuffix(s, "-USD") {
		code := strings.TrimSuffix(s, "-USD")
		return "BINANCE:" + code + "USDT"
	}
	if strings.HasSuffix(s, "USDT") {
		return "BINANCE:" + s
	}

	// Pure numeric
	if isPureDigits(s) {
		if len(s) == 6 {
			if strings.HasPrefix(s, "6") || strings.HasPrefix(s, "5") {
				return "SSE:" + s
			}
			if strings.HasPrefix(s, "8") || strings.HasPrefix(s, "4") || strings.HasPrefix(s, "92") {
				return "BSE:" + s
			}
			return "SZSE:" + s
		}
		if len(s) <= 5 {
			return "HKEX:" + cleanHKTVCode(s)
		}
	}

	// Common NYSE & AMEX lists
	nyseTickers := map[string]bool{
		"TSM": true, "BABA": true, "NIO": true, "XPEV": true, "LI": true,
		"PLTR": true, "SNOW": true, "SPOT": true, "DIS": true, "JPM": true,
		"V": true, "MA": true, "UNH": true, "HD": true, "PG": true, "BRK.A": true,
		"BRK.B": true, "KO": true, "PFE": true, "LLY": true, "NKE": true,
		"WMT": true, "BAC": true, "CRM": true, "ORCL": true, "IBM": true,
	}
	if nyseTickers[s] {
		return "NYSE:" + s
	}

	amexTickers := map[string]bool{
		"SPY": true, "IVV": true, "SOXL": true, "SOXS": true, "TQQQ": true,
		"SQQQ": true, "SPXL": true, "SPXS": true, "TMF": true, "TMV": true,
		"UPRO": true, "SPXU": true, "TNA": true, "TZA": true, "QLD": true,
		"QID": true, "SSO": true, "SDS": true, "GLD": true, "SLV": true,
		"USO": true, "UNG": true, "KOLD": true, "BOIL": true, "UVXY": true,
	}
	if amexTickers[s] {
		return "AMEX:" + s
	}

	// Default to NASDAQ
	return "NASDAQ:" + s
}

func cleanHKTVCode(code string) string {
	c := strings.TrimLeft(code, "0")
	if c == "" {
		return "1"
	}
	return c
}

func isPureDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
