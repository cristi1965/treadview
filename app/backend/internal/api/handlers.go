package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/agents"
	"trading-agents/internal/config"
	"trading-agents/internal/orchestrator"
)

// Handler holds dependencies for HTTP request handlers.
type Handler struct {
	config       *config.Config
	orchestrator *orchestrator.Orchestrator
	hub          *Hub

	// Running analysis tracking
	mu            sync.Mutex
	runningCtx    context.Context
	runningCancel context.CancelFunc
	isRunning     bool
	lastResult    *agents.AnalysisResult
	results       []agents.AnalysisResult // history
}

// NewHandler creates API handlers.
func NewHandler(cfg *config.Config, orch *orchestrator.Orchestrator, hub *Hub) *Handler {
	return &Handler{
		config:       cfg,
		orchestrator: orch,
		hub:          hub,
	}
}

// StartAnalysis handles POST /api/analysis/start
func (h *Handler) StartAnalysis(c *gin.Context) {
	var req agents.AnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.mu.Lock()
	if h.isRunning {
		h.mu.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "Analysis already running"})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	h.runningCtx = ctx
	h.runningCancel = cancel
	h.isRunning = true
	h.mu.Unlock()

	// Wire up WebSocket broadcasting
	h.orchestrator.SetEventHandler(func(event agents.NodeEvent) {
		h.hub.Broadcast(event)
	})

	// Run analysis in background
	go func() {
		defer func() {
			h.mu.Lock()
			h.isRunning = false
			h.runningCancel = nil
			h.mu.Unlock()
		}()

		result, err := h.orchestrator.RunAnalysis(ctx, req)
		if err != nil {
			log.Printf("[API] Analysis error: %v", err)
			h.hub.Broadcast(agents.NodeEvent{
				Type:    "analysis_error",
				Node:    "System",
				Content: err.Error(),
				Status:  "error",
			})
			return
		}

		h.mu.Lock()
		h.lastResult = result
		h.results = append(h.results, *result)
		h.mu.Unlock()

		// Save result to disk
		h.saveResult(result)
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"status":  "started",
		"ticker":  req.Ticker,
		"date":    req.TradeDate,
		"message": "Analysis started. Connect to /ws for real-time updates.",
	})
}

// StopAnalysis handles POST /api/analysis/stop
func (h *Handler) StopAnalysis(c *gin.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No analysis running"})
		return
	}

	if h.runningCancel != nil {
		h.runningCancel()
	}
	h.orchestrator.Stop()
	h.isRunning = false

	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

// GetStatus handles GET /api/analysis/status
func (h *Handler) GetStatus(c *gin.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	status := "idle"
	if h.isRunning {
		status = "running"
	}

	resp := gin.H{
		"status":     status,
		"ws_clients": h.hub.ClientCount(),
	}

	if h.lastResult != nil {
		resp["last_result"] = h.lastResult
	}

	c.JSON(http.StatusOK, resp)
}

// GetHistory handles GET /api/analysis/history
func (h *Handler) GetHistory(c *gin.Context) {
	h.mu.Lock()
	results := make([]agents.AnalysisResult, len(h.results))
	copy(results, h.results)
	h.mu.Unlock()

	// Also try to load from disk
	diskResults := h.loadResults()
	if len(diskResults) > len(results) {
		results = diskResults
	}

	// Sort by completion time (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].CompletedAt > results[j].CompletedAt
	})

	c.JSON(http.StatusOK, results)
}

// GetConfig handles GET /api/config
func (h *Handler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, agents.ConfigResponse{
		LLMProvider:    h.config.LLMProvider,
		DeepThinkLLM:   h.config.DeepThinkLLM,
		QuickThinkLLM:  h.config.QuickThinkLLM,
		OutputLanguage: h.config.OutputLanguage,
		MaxDebateRounds: h.config.MaxDebateRounds,
		MaxRiskRounds:   h.config.MaxRiskDiscussRounds,
		LLMBackendURL:   h.config.LLMBackendURL,
	})
}

// UpdateConfig handles PUT /api/config
func (h *Handler) UpdateConfig(c *gin.Context) {
	var req agents.ConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.LLMProvider != nil {
		h.config.LLMProvider = *req.LLMProvider
	}
	if req.DeepSeekAPIKey != nil {
		h.config.DeepSeekAPIKey = *req.DeepSeekAPIKey
	}
	if req.GoogleAPIKey != nil {
		h.config.GoogleAPIKey = *req.GoogleAPIKey
	}
	if req.DeepThinkLLM != nil {
		h.config.DeepThinkLLM = *req.DeepThinkLLM
	}
	if req.QuickThinkLLM != nil {
		h.config.QuickThinkLLM = *req.QuickThinkLLM
	}
	if req.LLMBackendURL != nil {
		h.config.LLMBackendURL = *req.LLMBackendURL
	}
	if req.OutputLanguage != nil {
		h.config.OutputLanguage = *req.OutputLanguage
	}
	if req.MaxDebateRounds != nil {
		h.config.MaxDebateRounds = *req.MaxDebateRounds
	}
	if req.MaxRiskRounds != nil {
		h.config.MaxRiskDiscussRounds = *req.MaxRiskRounds
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// HealthCheck handles GET /api/health
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":     "ok",
		"ws_clients": h.hub.ClientCount(),
	})
}

func (h *Handler) saveResult(result *agents.AnalysisResult) {
	dir := filepath.Join(h.config.ResultsDir, "api_results")
	os.MkdirAll(dir, 0755)

	filename := filepath.Join(dir, result.Ticker+"_"+result.TradeDate+".json")
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Printf("[API] Failed to marshal result: %v", err)
		return
	}
	if err := os.WriteFile(filename, data, 0644); err != nil {
		log.Printf("[API] Failed to save result: %v", err)
	}
}

func (h *Handler) loadResults() []agents.AnalysisResult {
	dir := filepath.Join(h.config.ResultsDir, "api_results")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var results []agents.AnalysisResult
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var r agents.AnalysisResult
		if err := json.Unmarshal(data, &r); err == nil {
			results = append(results, r)
		}
	}
	return results
}
