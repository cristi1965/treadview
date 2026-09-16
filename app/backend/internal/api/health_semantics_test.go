package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"trading-agents/internal/config"
)

func TestHealthIsExplicitlyLivenessOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	h := NewHandler(&config.Config{}, nil, NewHub())
	h.HealthCheck(ctx)
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["liveness"] != "ok" || body["dataStatus"] != "not_checked" || body["readinessEndpoint"] != "/api/readiness" {
		t.Fatalf("health conflates process and data health: %v", body)
	}
}
