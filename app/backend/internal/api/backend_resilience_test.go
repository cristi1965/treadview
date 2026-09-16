package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"trading-agents/internal/agents"
	"trading-agents/internal/config"
	"trading-agents/internal/database"
	"trading-agents/internal/market"
	"trading-agents/internal/models"
)

func ginTestRouter(method, path string, handler gin.HandlerFunc) *gin.Engine {
	router := gin.New()
	router.Handle(method, path, handler)
	return router
}

func TestSavedResearchResultIsPrivateAndCorruptionIsVisible(t *testing.T) {
	root := t.TempDir()
	h := &Handler{config: &config.Config{ResultsDir: root}, hub: NewHub()}
	result := &agents.AnalysisResult{Ticker: "AAPL", TradeDate: "2026-09-15", Audit: agents.AnalysisAudit{RunID: "run-private"}}
	if err := h.saveResult(result); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "api_results", "AAPL_2026-09-15_run-private.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("result permissions=%#o want 0600", info.Mode().Perm())
	}
	if err := os.WriteFile(filepath.Join(root, "api_results", "corrupt.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	router := ginTestRouter(http.MethodGet, "/history", h.GetHistory)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/history", nil))
	if w.Header().Get("X-Research-History-Errors") != "1" {
		t.Fatalf("corruption not surfaced headers=%v body=%s", w.Header(), w.Body.String())
	}
}

func TestValidateJournalTradeRejectsInvalidFields(t *testing.T) {
	cases := []string{
		`{"symbol":"../x","direction":"BUY","entryPrice":1,"shares":1}`,
		`{"symbol":"AAPL","direction":"HOLD","entryPrice":1,"shares":1}`,
		`{"symbol":"AAPL","direction":"BUY","entryPrice":0,"shares":1}`,
		`{"symbol":"AAPL","direction":"BUY","entryPrice":1,"shares":-1}`,
	}
	for _, raw := range cases {
		var trade journalTradeInput
		if err := json.Unmarshal([]byte(raw), &trade); err != nil {
			t.Fatal(err)
		}
		if err := validateJournalTrade(trade); err == nil {
			t.Fatalf("accepted invalid journal trade: %s", raw)
		}
	}
}

func TestTradeDateRejectsImpossibleCalendarDate(t *testing.T) {
	if validTradeDate("2026-02-30") {
		t.Fatal("impossible calendar date accepted")
	}
}

func TestJournalCreateIsValidatedAndIdempotent(t *testing.T) {
	db := setupAuditTestDB(t)
	if err := db.AutoMigrate(&models.Trade{}); err != nil {
		t.Fatal(err)
	}
	h := &Handler{}
	router := gin.New()
	router.POST("/trades", adminWriteGuard(&config.Config{AdminToken: "secret"}, "paper-journal.create"), h.CreateTrade)
	request := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/trades", strings.NewReader(`{"id":999,"symbol":"aapl","direction":"buy","entryPrice":100,"exitPrice":110,"shares":2,"pnl":999999}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer secret")
		req.Header.Set(requestIDHeader, "journal-idempotent")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	first, second := request(), request()
	if first.Code != http.StatusCreated || second.Code != http.StatusConflict {
		t.Fatalf("statuses first=%d second=%d", first.Code, second.Code)
	}
	var trades []models.Trade
	if err := database.DB.Find(&trades).Error; err != nil || len(trades) != 1 {
		t.Fatalf("trades=%d err=%v", len(trades), err)
	}
	if trades[0].ID == 999 || trades[0].Symbol != "AAPL" || trades[0].Pnl != 20 {
		t.Fatalf("unsafe journal projection: %+v", trades[0])
	}
}

func TestAnalysisContextUsesProcessCancellationAndDeadline(t *testing.T) {
	processCtx, cancelProcess := context.WithCancel(context.Background())
	h := &Handler{processCtx: processCtx}
	ctx, cancel := h.newAnalysisContext()
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("analysis context has no total deadline")
	}
	cancelProcess()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("analysis context ignored process cancellation")
	}
}

func TestQuoteEndpointRejectsOversizedSymbolSet(t *testing.T) {
	syms := make([]string, maxPublicQuoteSymbols+1)
	for i := range syms {
		syms[i] = "AAPL"
	}
	router := ginTestRouter(http.MethodGet, "/quote", GetQuoteData)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/quote?syms="+strings.Join(syms, ","), nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestTimedOutQuoteFlightDoesNotDuplicateUncancelledPrimary(t *testing.T) {
	previousPrimary := quoteProviderFetch
	previousSecondary := quoteSecondaryFetch
	previousTimeout := quoteRequestTimeout
	defer func() {
		quoteProviderFetch = previousPrimary
		quoteSecondaryFetch = previousSecondary
		quoteRequestTimeout = previousTimeout
	}()
	blocked := make(chan struct{})
	var calls atomic.Int32
	quoteRequestTimeout = 20 * time.Millisecond
	quoteProviderFetch = func(_ *market.Provider, _ []string) map[string]market.Quote {
		calls.Add(1)
		<-blocked
		return nil
	}
	quoteSecondaryFetch = func(context.Context, *market.Provider, []string) map[string]market.Quote { return nil }
	_ = boundedQuotesWithMeta([]string{"AAPL"})
	_ = boundedQuotesWithMeta([]string{"AAPL"})
	if calls.Load() != 1 {
		t.Fatalf("uncancelled primary duplicated: calls=%d", calls.Load())
	}
	close(blocked)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		quoteFlights.Lock()
		_, remaining := quoteFlights.values["AAPL"]
		quoteFlights.Unlock()
		if !remaining {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed-out quote flight was not cleaned up")
}

func TestWhalesSyncReportsAlreadyRunning(t *testing.T) {
	database.SyncMutex.Lock()
	previous := database.WhalesSyncing
	database.WhalesSyncing = true
	database.SyncMutex.Unlock()
	t.Cleanup(func() {
		database.SyncMutex.Lock()
		database.WhalesSyncing = previous
		database.SyncMutex.Unlock()
	})
	router := ginTestRouter(http.MethodPost, "/sync", (&Handler{}).TriggerWhalesSync)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/sync", nil))
	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
