package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"trading-agents/internal/config"
	"trading-agents/internal/database"
	"trading-agents/internal/gpupricing"
	"trading-agents/internal/models"
)

type apiGPUProvider struct {
	name       string
	configured bool
	quotes     []gpupricing.Quote
}

func (p apiGPUProvider) Name() string     { return p.name }
func (p apiGPUProvider) Configured() bool { return p.configured }
func (p apiGPUProvider) Fetch(context.Context, time.Time) ([]gpupricing.Quote, error) {
	return append([]gpupricing.Quote(nil), p.quotes...), nil
}

func openAPIGPUPriceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "gpu-api.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.GPUPriceObservation{}, &models.AuditEvent{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestGPUPriceCurrentAndStatusAreStructuredWhenUnconfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAPIGPUPriceDB(t)
	service := gpupricing.NewServiceWithProviders(db, []gpupricing.Provider{
		apiGPUProvider{name: gpupricing.ProviderRunPod}, apiGPUProvider{name: gpupricing.ProviderModal},
		apiGPUProvider{name: gpupricing.ProviderLambda}, apiGPUProvider{name: gpupricing.ProviderVast},
	}, time.Now)
	handler := NewHandlerWithGPUPricing(&config.Config{}, nil, NewHub(), service)
	router := gin.New()
	router.GET("/api/gpu-prices", handler.GetGPUPrices)
	router.GET("/api/gpu-prices/status", handler.GetGPUPriceStatus)

	for _, path := range []string{"/api/gpu-prices", "/api/gpu-prices/status"} {
		result := httptest.NewRecorder()
		router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, path, nil))
		if result.Code != http.StatusOK || result.Header().Get("X-Data-Stale") != "true" || result.Header().Get("X-Data-Refreshable") != "false" {
			t.Fatalf("%s status=%d headers=%v body=%s", path, result.Code, result.Header(), result.Body.String())
		}
		var body gpupricing.Snapshot
		if err := json.Unmarshal(result.Body.Bytes(), &body); err != nil || body.Status != gpupricing.StatusUnconfigured || len(body.Providers) != 4 || body.Count != 0 {
			t.Fatalf("%s snapshot=%+v err=%v", path, body, err)
		}
	}
}

func TestGPUPriceFixtureIsExplicitlyTestOnlyInCurrentAndHistoryResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(gpupricing.RuntimeFixtureEnvironment, gpupricing.RuntimeFixtureAcknowledgement)
	t.Setenv("STOCKGOD_REPLAY", "false")
	t.Setenv("STOCKGOD_LIVE_MIRROR", "false")
	databaseDir := t.TempDir()
	db, err := database.OpenSQLite(filepath.Join(databaseDir, "gpu-fixture-api.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.GPUPriceObservation{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{AdminToken: "fixture-admin", DatabaseDir: databaseDir}
	service := gpupricing.NewService(cfg, db)
	if result, err := service.Refresh(context.Background()); err != nil || result.Updated != 4 {
		t.Fatalf("fixture refresh=%+v err=%v", result, err)
	}
	handler := NewHandlerWithGPUPricing(cfg, nil, NewHub(), service)
	router := gin.New()
	router.GET("/api/gpu-prices", handler.GetGPUPrices)
	router.GET("/api/gpu-prices/history", handler.GetGPUPriceHistory)
	router.POST("/api/gpu-prices/refresh", handler.RefreshGPUPrices)
	router.POST("/api/gpu-prices/reload-config", handler.ReloadGPUPriceConfig)

	current := httptest.NewRecorder()
	router.ServeHTTP(current, httptest.NewRequest(http.MethodGet, "/api/gpu-prices", nil))
	var snapshot gpupricing.Snapshot
	if err := json.Unmarshal(current.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if current.Header().Get("X-Data-Source") != gpupricing.RuntimeFixtureDataMode || !snapshot.TestOnly || snapshot.DataMode != gpupricing.RuntimeFixtureDataMode {
		t.Fatalf("fixture current headers=%v snapshot=%+v", current.Header(), snapshot)
	}

	historyResponse := httptest.NewRecorder()
	router.ServeHTTP(historyResponse, httptest.NewRequest(http.MethodGet, "/api/gpu-prices/history?days=1", nil))
	var history gpupricing.HistoryResult
	if err := json.Unmarshal(historyResponse.Body.Bytes(), &history); err != nil {
		t.Fatal(err)
	}
	if historyResponse.Header().Get("X-Data-Source") != gpupricing.RuntimeFixtureDataMode || !history.TestOnly || history.DataMode != gpupricing.RuntimeFixtureDataMode {
		t.Fatalf("fixture history headers=%v body=%+v", historyResponse.Header(), history)
	}

	reloadResponse := httptest.NewRecorder()
	router.ServeHTTP(reloadResponse, httptest.NewRequest(http.MethodPost, "/api/gpu-prices/reload-config", nil))
	var reloadBody struct {
		DataMode string `json:"dataMode"`
		TestOnly bool   `json:"testOnly"`
	}
	if err := json.Unmarshal(reloadResponse.Body.Bytes(), &reloadBody); err != nil {
		t.Fatal(err)
	}
	if reloadResponse.Header().Get("X-Data-Source") != gpupricing.RuntimeFixtureDataMode || !reloadBody.TestOnly || reloadBody.DataMode != gpupricing.RuntimeFixtureDataMode {
		t.Fatalf("fixture reload headers=%v body=%s", reloadResponse.Header(), reloadResponse.Body.String())
	}

	refreshResponse := httptest.NewRecorder()
	router.ServeHTTP(refreshResponse, httptest.NewRequest(http.MethodPost, "/api/gpu-prices/refresh", nil))
	var refresh gpupricing.RefreshResult
	if err := json.Unmarshal(refreshResponse.Body.Bytes(), &refresh); err != nil {
		t.Fatal(err)
	}
	if refreshResponse.Header().Get("X-Data-Source") != gpupricing.RuntimeFixtureDataMode || !refresh.Snapshot.TestOnly || refresh.Snapshot.DataMode != gpupricing.RuntimeFixtureDataMode {
		t.Fatalf("fixture refresh headers=%v body=%s", refreshResponse.Header(), refreshResponse.Body.String())
	}
}

func TestGPUPriceProductionFreshnessKeepsOfficialProviderSource(t *testing.T) {
	meta := gpuPriceFreshness(gpupricing.Snapshot{Status: gpupricing.StatusLive})
	if meta.Source != "official-provider-apis" {
		t.Fatalf("production GPU source=%q", meta.Source)
	}
}

func TestGPUPriceHistoryValidatesFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandlerWithGPUPricing(&config.Config{}, nil, NewHub(), gpupricing.NewServiceWithProviders(openAPIGPUPriceDB(t), nil, time.Now))
	router := gin.New()
	router.GET("/api/gpu-prices/history", handler.GetGPUPriceHistory)
	for _, path := range []string{"/api/gpu-prices/history?days=0", "/api/gpu-prices/history?limit=5001", "/api/gpu-prices/history?provider=unknown"} {
		result := httptest.NewRecorder()
		router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, path, nil))
		if result.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", path, result.Code, result.Body.String())
		}
	}
}

func TestGPUPriceHistoryFreshnessHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAPIGPUPriceDB(t)
	service := gpupricing.NewServiceWithProviders(db, nil, time.Now)
	handler := NewHandlerWithGPUPricing(&config.Config{}, nil, NewHub(), service)
	router := gin.New()
	router.GET("/api/gpu-prices/history", handler.GetGPUPriceHistory)

	request := func(path string) *httptest.ResponseRecorder {
		result := httptest.NewRecorder()
		router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, path, nil))
		return result
	}
	empty := request("/api/gpu-prices/history")
	if empty.Code != http.StatusOK || empty.Header().Get("X-Data-Time") != "unknown" || empty.Header().Get("X-Data-Stale") != "true" || empty.Header().Get("X-Data-Stale-Reason") != "no authenticated observations" {
		t.Fatalf("empty history status=%d headers=%v body=%s", empty.Code, empty.Header(), empty.Body.String())
	}

	now := time.Now().UTC().Truncate(time.Second)
	rows := []models.GPUPriceObservation{
		validAPIObservation(gpupricing.ProviderRunPod, "runpod-old", now.Add(-24*time.Hour)),
		validAPIObservation(gpupricing.ProviderLambda, "lambda-recent", now.Add(-time.Hour)),
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	old := request("/api/gpu-prices/history?provider=runpod")
	if old.Header().Get("X-Data-Time") != now.Add(-24*time.Hour).Format(time.RFC3339) || old.Header().Get("X-Data-Stale") != "true" || !strings.Contains(old.Header().Get("X-Data-Stale-Reason"), "freshness window") {
		t.Fatalf("old history headers=%v body=%s", old.Header(), old.Body.String())
	}
	recent := request("/api/gpu-prices/history")
	if recent.Header().Get("X-Data-Time") != now.Add(-time.Hour).Format(time.RFC3339) || recent.Header().Get("X-Data-Stale") != "false" || recent.Header().Get("X-Data-Stale-Reason") != "" {
		t.Fatalf("recent history headers=%v body=%s", recent.Header(), recent.Body.String())
	}
}

func validAPIObservation(provider, identity string, observedAt time.Time) models.GPUPriceObservation {
	return models.GPUPriceObservation{
		Provider: provider, SourceIdentity: identity, ObservedAt: observedAt,
		GPUModel: "H100", Product: "cloud", BillingMode: "on-demand", GPUCount: 1,
		Currency: "USD", RawPrice: 3.49, RawUnit: "USD/GPU-hour", PriceUSDPerGPUHour: 3.49,
		SourceURL: "https://official.example/prices",
	}
}

func TestGPUPriceReferenceIsExplicitlyHistoricalAndSeparate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := gpupricing.NewServiceWithProviders(openAPIGPUPriceDB(t), nil, time.Now)
	router := SetupRouterWithDependencies(&config.Config{}, nil, nil, service)

	result := httptest.NewRecorder()
	router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/api/gpu-prices/reference", nil))
	if result.Code != http.StatusOK || result.Header().Get("X-Data-Source") != gpupricing.ReferenceSource || result.Header().Get("X-Data-Time") != gpupricing.ReferenceVerifiedAt || result.Header().Get("X-Data-Stale") != "true" || result.Header().Get("X-Data-Refreshable") != "false" {
		t.Fatalf("status=%d headers=%v body=%s", result.Code, result.Header(), result.Body.String())
	}
	var body gpupricing.ReferenceSnapshot
	if err := json.Unmarshal(result.Body.Bytes(), &body); err != nil || body.Status != "historical" || body.DataMode != "historical" || !body.Stale || body.Count != 22 {
		t.Fatalf("reference=%+v err=%v", body, err)
	}
	if current := service.Current(); current.Count != 0 || len(current.Quotes) != 0 {
		t.Fatalf("reference leaked into dynamic current=%+v", current)
	}
	history, err := service.History(context.Background(), gpupricing.HistoryFilter{Days: 30})
	if err != nil || history.Count != 0 {
		t.Fatalf("reference leaked into dynamic history=%+v err=%v", history, err)
	}
}

func TestGPUPriceRefreshRouteRequiresAdminAndWritesAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAPIGPUPriceDB(t)
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous })
	cfg := &config.Config{AdminToken: "gpu-secret"}
	service := gpupricing.NewServiceWithProviders(db, nil, time.Now)
	router := SetupRouterWithDependencies(cfg, nil, nil, service)

	denied := performAdminRequest(router, http.MethodPost, "/api/gpu-prices/refresh", "")
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("denied status=%d body=%s", denied.Code, denied.Body.String())
	}
	accepted := performAdminRequest(router, http.MethodPost, "/api/gpu-prices/refresh", "Bearer gpu-secret")
	if accepted.Code != http.StatusOK {
		t.Fatalf("accepted status=%d body=%s", accepted.Code, accepted.Body.String())
	}
	var events []models.AuditEvent
	if err := db.Where("action = ?", "gpu-prices.refresh").Order("id asc").Find(&events).Error; err != nil || len(events) != 2 || events[0].Status != "DENIED_MISSING_TOKEN" || events[1].Status != "SUCCEEDED" {
		t.Fatalf("audit events=%+v err=%v", events, err)
	}
}

func TestGPUPriceSystemEndpointPreservesUnconfiguredState(t *testing.T) {
	endpoint := gpuPriceSystemEndpoint(gpupricing.Snapshot{Status: gpupricing.StatusUnconfigured, Providers: []gpupricing.ProviderStatus{{Provider: "runpod", Status: gpupricing.StatusUnconfigured}}})
	if endpoint.Endpoint != "/api/gpu-prices" || endpoint.Status != gpupricing.StatusUnconfigured || !endpoint.Stale || endpoint.Refreshable {
		t.Fatalf("endpoint=%+v", endpoint)
	}
	capability := systemCapabilityFreshness(systemStatusResponse{Status: "ok", Endpoints: []systemEndpointStatus{endpoint}})
	if capability.Stale {
		t.Fatalf("unconfigured GPU pricing degraded core capability freshness: %+v", capability)
	}
}

func TestGPUProviderCredentialsAreNotExposedByConfigEndpoint(t *testing.T) {
	cfg := &config.Config{
		RunPodAPIKey:     "runpod-private",
		ModalTokenID:     "modal-id-private",
		ModalTokenSecret: "modal-secret-private",
		LambdaAPIKey:     "lambda-private",
		VastAPIKey:       "vast-private",
	}
	handler := NewHandlerWithGPUPricing(cfg, nil, NewHub(), nil)
	result := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(result)

	handler.GetConfig(context)

	for _, secret := range []string{cfg.RunPodAPIKey, cfg.ModalTokenID, cfg.ModalTokenSecret, cfg.LambdaAPIKey, cfg.VastAPIKey} {
		if strings.Contains(result.Body.String(), secret) {
			t.Fatalf("config response exposed GPU provider credential: %s", result.Body.String())
		}
	}
}
