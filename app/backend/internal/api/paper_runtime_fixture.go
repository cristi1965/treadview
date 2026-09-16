package api

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/config"
	"trading-agents/internal/market"
	"trading-agents/internal/models"
)

const (
	paperRuntimeFixtureEnv            = "STOCKGOD_PAPER_RUNTIME_FIXTURE"
	paperRuntimeFixtureAcknowledgment = "temporary-local-acceptance-v1"
	paperRuntimeFixtureSource         = "paper-runtime-fixture"
	paperRuntimeManualBaselineEnv     = "STOCKGOD_PAPER_RUNTIME_MANUAL_BASELINE"
)

var paperRuntimeFixture = struct {
	sync.RWMutex
	enabled    bool
	prices     map[string]float64
	observedAt time.Time
}{prices: map[string]float64{}}

// ConfigurePaperRuntimeFixtureFromEnvironment installs deterministic Paper-only
// data for the runtime acceptance script. It requires an exact acknowledgement,
// an admin token, and a database directory below the OS temporary directory.
// The production default is disabled and exposes no fixture route.
func ConfigurePaperRuntimeFixtureFromEnvironment(cfg *config.Config) (bool, error) {
	value := strings.TrimSpace(os.Getenv(paperRuntimeFixtureEnv))
	if value == "" {
		return false, nil
	}
	if value != paperRuntimeFixtureAcknowledgment {
		return false, fmt.Errorf("%s has an invalid acknowledgement value", paperRuntimeFixtureEnv)
	}
	if cfg == nil || strings.TrimSpace(cfg.AdminToken) == "" {
		return false, errors.New("Paper runtime fixture requires STOCKGOD_ADMIN_TOKEN")
	}
	if strings.EqualFold(os.Getenv("STOCKGOD_REPLAY"), "true") || strings.EqualFold(os.Getenv("STOCKGOD_LIVE_MIRROR"), "true") {
		return false, errors.New("Paper runtime fixture is incompatible with replay or live mirror mode")
	}
	databaseDir := filepath.Clean(strings.TrimSpace(cfg.DatabaseDir))
	temporaryRoot := filepath.Clean(os.TempDir())
	relative, err := filepath.Rel(temporaryRoot, databaseDir)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false, errors.New("Paper runtime fixture requires a dedicated database directory below the OS temporary directory")
	}

	observedAt := time.Date(2026, time.September, 8, 15, 0, 0, 0, time.UTC)
	paperRuntimeFixture.Lock()
	paperRuntimeFixture.enabled = true
	paperRuntimeFixture.observedAt = observedAt
	paperRuntimeFixture.prices = map[string]float64{"NVDA": 100}
	paperRuntimeFixture.Unlock()

	boundedQuotesForRequest = paperRuntimeFixtureQuotes
	paperUniverseLoader = func(string) ([]models.Stock, string, error) {
		return []models.Stock{{Symbol: "NVDA", Price: 100, Sector: "Technology"}}, paperRuntimeFixtureSource, nil
	}
	paperADVLoader = func(string, []string) (map[string]float64, string, error) {
		return map[string]float64{"NVDA": 1_000_000}, paperRuntimeFixtureSource + ":20d-adv", nil
	}
	return true, nil
}

func paperRuntimeFixtureEnabled() bool {
	paperRuntimeFixture.RLock()
	defer paperRuntimeFixture.RUnlock()
	return paperRuntimeFixture.enabled
}

func paperRuntimeManualBaselineEnabled() bool {
	return paperRuntimeFixtureEnabled() && strings.EqualFold(strings.TrimSpace(os.Getenv(paperRuntimeManualBaselineEnv)), "true")
}

func paperRuntimeFixtureNow() time.Time {
	paperRuntimeFixture.RLock()
	defer paperRuntimeFixture.RUnlock()
	return paperRuntimeFixture.observedAt.UTC()
}

func paperRuntimeFixtureQuotes(symbols []string) quoteFetchResult {
	paperRuntimeFixture.RLock()
	defer paperRuntimeFixture.RUnlock()
	quotes := make(map[string]market.Quote, len(symbols))
	for _, requested := range symbols {
		symbol := strings.ToUpper(strings.TrimSpace(requested))
		price := paperRuntimeFixture.prices[symbol]
		if price <= 0 {
			continue
		}
		quotes[symbol] = market.Quote{
			Price: price, Source: paperRuntimeFixtureSource, Session: "regular",
			DataTime: paperRuntimeFixture.observedAt.UTC().Format(time.RFC3339), TimeGranularity: "second",
			ProviderURL: "paper-runtime-fixture://local/" + symbol,
		}
	}
	return quoteFetchResult{
		quotes: quotes, source: paperRuntimeFixtureSource, dataTime: paperRuntimeFixture.observedAt.UTC(),
		dataTimeLabel: paperRuntimeFixture.observedAt.UTC().Format(time.RFC3339), timeGranularity: "second",
	}
}

func (h *Handler) SetPaperRuntimeFixtureQuote(c *gin.Context) {
	if !paperRuntimeFixtureEnabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var request struct {
		Symbol string  `json:"symbol"`
		Price  float64 `json:"price"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid fixture quote"})
		return
	}
	request.Symbol = strings.ToUpper(strings.TrimSpace(request.Symbol))
	if !safeTickerRe.MatchString(request.Symbol) || request.Price <= 0 || math.IsNaN(request.Price) || math.IsInf(request.Price, 0) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fixture quote requires a valid symbol and positive finite price"})
		return
	}
	paperRuntimeFixture.Lock()
	paperRuntimeFixture.prices[request.Symbol] = request.Price
	observedAt := paperRuntimeFixture.observedAt.UTC()
	paperRuntimeFixture.Unlock()
	c.JSON(http.StatusOK, gin.H{
		"environment": paperEnvironment, "symbol": request.Symbol, "price": request.Price,
		"source": paperRuntimeFixtureSource, "observedAt": observedAt, "testOnly": true,
	})
}
