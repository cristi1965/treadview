package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/config"
	"trading-agents/internal/gpupricing"
)

func TestReloadGPUPriceConfigNeverReturnsCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "runtime-reload-secret"
	t.Setenv("RUNPOD_API_KEY", secret)
	cfg := &config.Config{GPUCredentialsFile: t.TempDir() + "/missing.json"}
	service := gpupricing.NewServiceWithProviders(openAPIGPUPriceDB(t), nil, time.Now)
	handler := NewHandlerWithGPUPricing(cfg, nil, NewHub(), service)
	router := gin.New()
	router.POST("/reload", handler.ReloadGPUPriceConfig)

	result := httptest.NewRecorder()
	router.ServeHTTP(result, httptest.NewRequest(http.MethodPost, "/reload", nil))
	if result.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
	}
	if strings.Contains(result.Body.String(), secret) || strings.Contains(result.Body.String(), cfg.GPUCredentialsFile) {
		t.Fatalf("reload response leaked credential material: %s", result.Body.String())
	}
	if !strings.Contains(result.Body.String(), `"provider":"runpod"`) || !strings.Contains(result.Body.String(), `"configured":true`) {
		t.Fatalf("reload response omitted provider status: %s", result.Body.String())
	}
}

func TestETFRefreshHandlerRequiresSharedService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(&config.Config{}, nil, NewHub())
	router := gin.New()
	router.POST("/refresh", handler.TriggerETFRefresh)
	result := httptest.NewRecorder()
	router.ServeHTTP(result, httptest.NewRequest(http.MethodPost, "/refresh", nil))
	if result.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
	}
}
