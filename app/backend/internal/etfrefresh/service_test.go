package etfrefresh

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"trading-agents/internal/dataflows"
)

type fakeHistoryFetcher struct {
	bars   map[string][]dataflows.HistoricalBar
	errors map[string]error
}

func (f fakeHistoryFetcher) GetHistoricalData(symbol, _, _ string) ([]dataflows.HistoricalBar, error) {
	if err := f.errors[symbol]; err != nil {
		return nil, err
	}
	return f.bars[symbol], nil
}

func TestRunOncePromotesOnlyCompleteBoundedScope(t *testing.T) {
	root := t.TempDir()
	writeUniverse(t, root)
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	bars := validHistory(now)
	service := NewService(Config{
		BackendRoot: root, ScopeLimit: 2, Now: func() time.Time { return now },
		Fetcher: fakeHistoryFetcher{bars: map[string][]dataflows.HistoricalBar{"AAA": bars, "BBB": bars}},
	})
	if err := service.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	var current analysesFile
	readJSON(t, filepath.Join(root, "data", "etf-analyses-current.json"), &current)
	if current.N != 2 || len(current.ETFs) != 2 || current.Updated != now.Format("2006-01-02") {
		t.Fatalf("current=%+v", current)
	}
	if current.Source != "yahoo-chart+local-etf-metadata" || current.ETFs[0].Ret1Y == nil || current.ETFs[0].Ret5Y == nil || current.ETFs[0].MDD == nil {
		t.Fatalf("current provenance or metrics missing: %+v", current)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "etf-history", current.Updated+".json")); err != nil {
		t.Fatalf("archive missing: %v", err)
	}
	var status RefreshStatus
	readJSON(t, filepath.Join(root, "data", "etf-refresh-status.json"), &status)
	if status.LastAttemptStatus != "complete" || status.ScopeRequested != 2 || status.ScopeCompleted != 2 || status.NextScheduledAt == "" {
		t.Fatalf("status=%+v", status)
	}
}

func TestRunOnceFailurePreservesLastGoodAndRecordsDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeUniverse(t, root)
	currentPath := filepath.Join(root, "data", "etf-analyses-current.json")
	before, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	completeHistory := validHistory(now)
	service := NewService(Config{
		BackendRoot: root, ScopeLimit: 2, Now: func() time.Time { return now },
		Fetcher: fakeHistoryFetcher{
			bars: map[string][]dataflows.HistoricalBar{"AAA": completeHistory, "BBB": completeHistory[len(completeHistory)-100:]},
		},
	})
	if err := service.RunOnce(context.Background()); err == nil {
		t.Fatal("expected incomplete scope to fail")
	}
	after, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("failed refresh replaced last-good")
	}
	var status RefreshStatus
	readJSON(t, filepath.Join(root, "data", "etf-refresh-status.json"), &status)
	if status.LastAttemptStatus != "failed" || status.ScopeRequested != 2 || status.ScopeCompleted != 1 || status.LastAttemptError == "" {
		t.Fatalf("status=%+v", status)
	}
}

func TestRunOnceRejectsMissingDecisionMetadataBeforePromotion(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	currentPath := filepath.Join(root, "data", "etf-analyses-current.json")
	missingExpense := `{"updated":"2026-09-09","aumUnit":"usd","n":1,"sectors":[{"sector":"Broad","super":"宽基","n":1,"aum":300}],"etfs":[{"sym":"SPYM","name":"State Street SPDR Portfolio S&P 500 ETF","aum":300,"sector":"Broad","kind":"宽基"}]}`
	if err := os.WriteFile(currentPath, []byte(missingExpense), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	service := NewService(Config{
		BackendRoot: root, ScopeLimit: 1, Now: func() time.Time { return now },
		Fetcher: fakeHistoryFetcher{bars: map[string][]dataflows.HistoricalBar{"SPYM": validHistory(now)}},
	})

	if err := service.RunOnce(context.Background()); err == nil {
		t.Fatal("missing expense metadata must stop promotion")
	}
	after, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != missingExpense {
		t.Fatal("failed metadata validation replaced current file")
	}
	var status RefreshStatus
	readJSON(t, filepath.Join(root, "data", "etf-refresh-status.json"), &status)
	if status.LastAttemptStatus != "failed" || status.ScopeCompleted != 0 || status.LastAttemptError == "" {
		t.Fatalf("status=%+v", status)
	}
}

func writeUniverse(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	input := analysesFile{
		Updated: "2026-06-16", AUMUnit: "usd", N: 3,
		Sectors: []sectorEntry{{Sector: "Broad", Super: "宽基", N: 3, AUM: 600}},
		ETFs: []analysisEntry{
			{Sym: "AAA", Name: "A", AUM: 300, Expense: float64Ptr(0.03), Sector: "Broad", Kind: "宽基"},
			{Sym: "BBB", Name: "B", AUM: 200, Expense: float64Ptr(0), Sector: "Broad", Kind: "宽基"},
			{Sym: "CCC", Name: "C", AUM: 100, Expense: float64Ptr(0.05), Sector: "Broad", Kind: "宽基"},
		},
	}
	if err := atomicWriteJSON(filepath.Join(root, "data", "etf-analyses-current.json"), input); err != nil {
		t.Fatal(err)
	}
}

func float64Ptr(value float64) *float64 {
	return &value
}

func validHistory(now time.Time) []dataflows.HistoricalBar {
	start := now.AddDate(-6, 0, 0)
	bars := make([]dataflows.HistoricalBar, 0, 2200)
	for day := 0; day <= 6*366; day++ {
		date := start.AddDate(0, 0, day)
		if date.After(now) {
			break
		}
		bars = append(bars, dataflows.HistoricalBar{Date: date.Format("2006-01-02"), Close: 100 + float64(day)/100})
	}
	return bars
}

func readJSON(t *testing.T, path string, target interface{}) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
}
