package dataflows

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const usInstrumentMasterContractVersion = "stockgod-us-symbol-universe-v1"

type usInstrumentMaster struct {
	ContractVersion string
	DataVersion     string
	Source          string
	Market          string
	AssetClass      string
	Currency        string
	Symbols         map[string]struct{}
	Names           map[string]string
}

func (m *usInstrumentMaster) Contains(symbol string) bool {
	if m == nil {
		return false
	}
	_, ok := m.Symbols[strings.ToUpper(strings.TrimSpace(symbol))]
	return ok
}

func (m *usInstrumentMaster) CurrencySource() string {
	return fmt.Sprintf("instrument-master:%s@%s", m.ContractVersion, m.DataVersion)
}

func (m *usInstrumentMaster) ContainsStock(symbol string) bool {
	return m != nil && m.Market == "US" && m.AssetClass == "stocks" && m.Currency == "USD" && m.Contains(symbol)
}

func loadUSInstrumentMaster() (*usInstrumentMaster, error) {
	paths := []string{
		filepath.Join("..", "frontend", "public", "data", "us-stocks.json"),
		filepath.Join("app", "frontend", "public", "data", "us-stocks.json"),
		filepath.Join("frontend", "public", "data", "us-stocks.json"),
		filepath.Join("data", "us-stocks.json"),
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		master, err := parseUSInstrumentMaster(raw, path)
		if err != nil {
			return nil, err
		}
		return master, nil
	}
	return nil, fmt.Errorf("versioned US instrument master unavailable")
}

func parseUSInstrumentMaster(raw []byte, source string) (*usInstrumentMaster, error) {
	var payload struct {
		GeneratedAt string `json:"generated_at"`
		Count       int    `json:"count"`
		Stocks      []struct {
			Symbol string `json:"sym"`
			Name   string `json:"name"`
		} `json:"stocks"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if payload.GeneratedAt == "" || payload.Count <= 0 || len(payload.Stocks) != payload.Count {
		return nil, fmt.Errorf("US instrument master metadata is incomplete")
	}
	master := &usInstrumentMaster{
		ContractVersion: usInstrumentMasterContractVersion,
		DataVersion:     payload.GeneratedAt, Source: source,
		Market: "US", AssetClass: "stocks", Currency: "USD",
		Symbols: make(map[string]struct{}, len(payload.Stocks)),
		Names:   make(map[string]string, len(payload.Stocks)),
	}
	for _, row := range payload.Stocks {
		symbol := strings.ToUpper(strings.TrimSpace(row.Symbol))
		if symbol == "" {
			return nil, fmt.Errorf("US instrument master contains empty symbol")
		}
		master.Symbols[symbol] = struct{}{}
		if name := strings.TrimSpace(row.Name); name != "" {
			master.Names[symbol] = name
		}
	}
	return master, nil
}

// USInstrumentName returns only the versioned instrument-master name. Missing
// names remain empty rather than being inferred from a ticker.
func USInstrumentName(symbol string) string {
	master, err := loadUSInstrumentMaster()
	if err != nil {
		return ""
	}
	return master.Names[strings.ToUpper(strings.TrimSpace(symbol))]
}
