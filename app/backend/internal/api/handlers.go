package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/agents"
	"trading-agents/internal/config"
	"trading-agents/internal/database"
	"trading-agents/internal/dataflows"
	"trading-agents/internal/etfrefresh"
	"trading-agents/internal/gpupricing"
	"trading-agents/internal/llm"
	"trading-agents/internal/orchestrator"
)

// Handler holds dependencies for HTTP request handlers.
type Handler struct {
	config       *config.Config
	orchestrator *orchestrator.Orchestrator
	hub          *Hub
	gpuPricing   *gpupricing.Service
	etfRefresh   *etfrefresh.Service

	// Running analysis tracking
	mu            sync.Mutex
	runningCtx    context.Context
	runningCancel context.CancelFunc
	isRunning     bool
	isStopping    bool
	runningID     uint64
	lastResult    *agents.AnalysisResult
	results       []agents.AnalysisResult // history

	// Batch analysis
	batchRunning bool
	batchID      string
	batchStatus  batchJobStatus

	paperStopScanner  *PaperStopScanner
	paperClock        func() time.Time
	evidenceOnlyBuild func(agents.AnalysisRequest) (*agents.AnalysisResult, dataflows.ResearchPreflight, error)
	runAnalysis       func(context.Context, agents.AnalysisRequest) (*agents.AnalysisResult, error)
	processCtx        context.Context
	analysisTimeout   time.Duration
	resultLoadErrors  atomic.Int64
}

const defaultAnalysisTimeout = 30 * time.Minute

func (h *Handler) SetPaperStopScanner(scanner *PaperStopScanner) {
	h.paperStopScanner = scanner
}

func (h *Handler) SetETFRefreshService(service *etfrefresh.Service) {
	h.etfRefresh = service
}

// NewHandler creates API handlers.
func NewHandler(cfg *config.Config, orch *orchestrator.Orchestrator, hub *Hub) *Handler {
	return NewHandlerWithGPUPricing(cfg, orch, hub, gpupricing.NewService(cfg, database.DB))
}

func NewHandlerWithGPUPricing(cfg *config.Config, orch *orchestrator.Orchestrator, hub *Hub, gpuPrices *gpupricing.Service) *Handler {
	return &Handler{
		config:       cfg,
		orchestrator: orch,
		hub:          hub,
		gpuPricing:   gpuPrices,
		processCtx:   context.Background(),
	}
}

func (h *Handler) SetProcessContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	h.processCtx = ctx
}

func (h *Handler) newAnalysisContext() (context.Context, context.CancelFunc) {
	parent := h.processCtx
	if parent == nil {
		parent = context.Background()
	}
	timeout := h.analysisTimeout
	if timeout <= 0 {
		timeout = defaultAnalysisTimeout
	}
	return context.WithTimeout(parent, timeout)
}

func (h *Handler) paperNow() time.Time {
	if h != nil && h.paperClock != nil {
		return h.paperClock().UTC()
	}
	if paperRuntimeFixtureEnabled() {
		return paperRuntimeFixtureNow()
	}
	return time.Now().UTC()
}

// StartAnalysis handles POST /api/analysis/start
func (h *Handler) StartAnalysis(c *gin.Context) {
	var req agents.AnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Ticker = strings.ToUpper(strings.TrimSpace(req.Ticker))
	if !safeTickerRe.MatchString(req.Ticker) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticker"})
		return
	}
	if !validTradeDate(req.TradeDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trade_date must be YYYY-MM-DD"})
		return
	}

	h.mu.Lock()
	if h.isRunning || h.batchRunning {
		h.mu.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "Analysis already running"})
		return
	}

	ctx, cancel := h.newAnalysisContext()
	h.runningID++
	runID := h.runningID
	h.runningCtx = ctx
	h.runningCancel = cancel
	h.isRunning = true
	h.isStopping = false
	h.mu.Unlock()

	// Wire up WebSocket broadcasting
	if h.orchestrator != nil {
		h.orchestrator.SetEventHandler(func(event agents.NodeEvent) {
			h.hub.Broadcast(event)
		})
	}

	// Run analysis in background
	go func() {
		defer func() {
			h.mu.Lock()
			if h.runningID == runID {
				h.isRunning = false
				h.isStopping = false
				h.runningCtx = nil
				h.runningCancel = nil
			}
			h.mu.Unlock()
		}()

		runner := h.runAnalysis
		if runner == nil {
			runner = h.orchestrator.RunAnalysis
		}
		result, err := runner(ctx, req)
		if err != nil {
			log.Printf("[API] Analysis error: %v", err)
			result = h.failedAnalysisResult(req, err)
			if saveErr := h.saveResult(result); saveErr != nil {
				log.Printf("[API] Failed to persist analysis failure: %v", saveErr)
			}
			h.mu.Lock()
			h.lastResult = result
			h.results = append(h.results, *result)
			h.mu.Unlock()
			h.hub.Broadcast(agents.NodeEvent{
				Type:    "analysis_error",
				Node:    "System",
				Content: err.Error(),
				Status:  "error",
			})
			return
		}

		// Save result to disk
		if err := h.saveResult(result); err != nil {
			h.hub.Broadcast(agents.NodeEvent{
				Type:    "analysis_warning",
				Node:    "System",
				Content: "result save failed: " + err.Error(),
				Status:  "warning",
			})
		}
		h.mu.Lock()
		h.lastResult = result
		h.results = append(h.results, *result)
		h.mu.Unlock()
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"status":          "started",
		"research_status": "pending_evidence_validation",
		"ticker":          req.Ticker,
		"date":            req.TradeDate,
		"message":         "Analysis started. Results publish only after evidence validation.",
	})
}

func (h *Handler) failedAnalysisResult(req agents.AnalysisRequest, runErr error) *agents.AnalysisResult {
	now := time.Now()
	assetType := strings.TrimSpace(req.AssetType)
	if assetType == "" {
		assetType = "stock"
	}
	req.AssetType = assetType
	audit := agents.NewAnalysisAudit(req, h.config.OutputLanguage, h.config.LLMProvider, h.config.QuickThinkLLM, h.config.DeepThinkLLM, h.config.MaxDebateRounds, h.config.MaxRiskDiscussRounds, now)
	result := &agents.AnalysisResult{
		Ticker: req.Ticker, TradeDate: req.TradeDate, Audit: audit,
		State:       agents.AgentState{CompanyOfInterest: req.Ticker, AssetType: assetType, TradeDate: req.TradeDate, OutputLanguage: h.config.OutputLanguage},
		CompletedAt: now.UTC().Format(time.RFC3339),
	}
	agents.MarkResearchUnavailable(result, "analysis failed: "+runErr.Error())
	result.Message = "Research unavailable: analysis failed: " + runErr.Error()
	return result
}

// StopAnalysis handles POST /api/analysis/stop
func (h *Handler) StopAnalysis(c *gin.Context) {
	h.mu.Lock()
	if !h.isRunning {
		h.mu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"error": "No analysis running"})
		return
	}
	if h.isStopping {
		h.mu.Unlock()
		c.JSON(http.StatusOK, gin.H{"status": "stopping"})
		return
	}
	cancel := h.runningCancel
	h.isStopping = true
	h.mu.Unlock()
	if cancel != nil {
		cancel()
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopping"})
}

// GetStatus handles GET /api/analysis/status
func (h *Handler) GetStatus(c *gin.Context) {
	h.mu.Lock()
	status := "idle"
	if h.isStopping {
		status = "stopping"
	} else if h.isRunning {
		status = "running"
	}

	var lastResult *agents.AnalysisResult
	if h.lastResult != nil {
		lastResult = cloneAnalysisResult(h.lastResult)
	}
	h.mu.Unlock()

	resp := gin.H{
		"status":     status,
		"ws_clients": h.hub.ClientCount(),
	}

	if lastResult != nil {
		agents.ApplyPublicationGate(lastResult)
		resp["last_result"] = lastResult
		resp["research_status"] = lastResult.Status
		c.Header("X-Research-Status", lastResult.Status)
	} else {
		c.Header("X-Research-Status", "none")
	}

	c.JSON(http.StatusOK, resp)
}

// GetHistory handles GET /api/analysis/history
func (h *Handler) GetHistory(c *gin.Context) {
	h.mu.Lock()
	serialized, _ := json.Marshal(h.results)
	h.mu.Unlock()
	results := []agents.AnalysisResult{}
	_ = json.Unmarshal(serialized, &results)
	if results == nil {
		results = []agents.AnalysisResult{}
	}

	// Also try to load from disk
	diskResults := h.loadResultsRaw()
	if loadErrors := h.resultLoadErrors.Load(); loadErrors > 0 {
		c.Header("X-Research-History-Errors", strconv.FormatInt(loadErrors, 10))
	}
	merged := make(map[string]agents.AnalysisResult, len(results)+len(diskResults))
	order := make([]string, 0, len(results)+len(diskResults))
	merge := func(items []agents.AnalysisResult) {
		for _, result := range items {
			key := analysisHistoryKey(result)
			if _, exists := merged[key]; !exists {
				order = append(order, key)
			}
			merged[key] = result
		}
	}
	merge(diskResults)
	merge(results)
	results = results[:0]
	for _, key := range order {
		results = append(results, merged[key])
	}

	// Sort by completion time (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].CompletedAt > results[j].CompletedAt
	})
	researchStatus := "none"
	unavailableCount := 0
	for i := range results {
		// History remains an evidence review surface even when a previously saved
		// dossier no longer satisfies the current publication contract. Preserve
		// its facts and gaps while the gate still withdraws every action conclusion.
		dossier := results[i].Dossier
		agents.ApplyPublicationGate(&results[i])
		if results[i].ResearchMode == agents.ResearchModeEvidenceOnly {
			results[i].Dossier = dossier
		}
		if results[i].Status == agents.ResearchStatusUnavailable {
			unavailableCount++
		} else if researchStatus == "none" || researchStatus == agents.ResearchStatusEvidenceOnly {
			researchStatus = results[i].Status
		}
	}
	if researchStatus == "none" && unavailableCount > 0 {
		researchStatus = agents.ResearchStatusUnavailable
	}
	c.Header("X-Research-Status", researchStatus)
	c.Header("X-Research-Unavailable-Count", strconv.Itoa(unavailableCount))

	c.JSON(http.StatusOK, results)
}

func cloneAnalysisResult(result *agents.AnalysisResult) *agents.AnalysisResult {
	if result == nil {
		return nil
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil
	}
	var clone agents.AnalysisResult
	if json.Unmarshal(raw, &clone) != nil {
		return nil
	}
	return &clone
}

func analysisHistoryKey(result agents.AnalysisResult) string {
	if runID := strings.TrimSpace(result.Audit.RunID); runID != "" {
		return "run:" + runID
	}
	return strings.Join([]string{"legacy", result.Ticker, result.TradeDate, result.CompletedAt, result.ResearchMode}, "|")
}

// GetConfig handles GET /api/config
func (h *Handler) GetConfig(c *gin.Context) {
	h.mu.Lock()
	cfg := *h.config
	h.mu.Unlock()
	c.JSON(http.StatusOK, agents.ConfigResponse{
		LLMProvider:     cfg.LLMProvider,
		DeepThinkLLM:    cfg.DeepThinkLLM,
		QuickThinkLLM:   cfg.QuickThinkLLM,
		OutputLanguage:  cfg.OutputLanguage,
		MaxDebateRounds: cfg.MaxDebateRounds,
		MaxRiskRounds:   cfg.MaxRiskDiscussRounds,
		LLMBackendURL:   cfg.LLMBackendURL,
	})
}

// UpdateConfig handles PUT /api/config
func (h *Handler) UpdateConfig(c *gin.Context) {
	var req agents.ConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.isRunning || h.batchRunning {
		c.JSON(http.StatusConflict, gin.H{"error": "模型配置不能在研究运行期间修改"})
		return
	}

	next := *h.config
	applyConfigUpdate(&next, req)
	config.NormalizeRuntimeConfig(&next)

	llmClient, err := llm.NewClient(&next)
	if err != nil {
		llmClient = llm.NewUnavailableClient(err.Error())
	}

	*h.config = next
	h.orchestrator.SetLLMClient(llmClient)

	status := "updated"
	if err != nil {
		status = "updated_with_unavailable_llm"
	}
	c.JSON(http.StatusOK, gin.H{
		"status":          status,
		"llm_provider":    h.config.LLMProvider,
		"deep_think_llm":  h.config.DeepThinkLLM,
		"quick_think_llm": h.config.QuickThinkLLM,
		"llm_backend_url": h.config.LLMBackendURL,
		"runtime_warning": errString(err),
	})
}

// TestConfig validates the currently selected provider, key and model with a real LLM call.
func (h *Handler) TestConfig(c *gin.Context) {
	var req agents.ConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.mu.Lock()
	testCfg := *h.config
	h.mu.Unlock()
	applyConfigUpdate(&testCfg, req)
	config.NormalizeRuntimeConfig(&testCfg)

	llmClient, err := llm.NewClient(&testCfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	start := time.Now()
	testTimeout := 25 * time.Second
	if testCfg.LLMProvider == llm.ProviderDual {
		testTimeout = 90 * time.Second
	}
	testModel := func(useDeep bool) error {
		ctx, cancel := context.WithTimeout(c.Request.Context(), testTimeout)
		defer cancel()
		_, callErr := llm.ProbeSelectedProvider(ctx, llmClient, "Reply with exactly one word: ok", "ok", useDeep)
		return callErr
	}
	if err = testModel(false); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":           err.Error(),
			"llm_provider":    testCfg.LLMProvider,
			"quick_think_llm": testCfg.QuickThinkLLM,
		})
		return
	}
	if testCfg.LLMProvider == llm.ProviderDual {
		if err = testModel(true); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "llm_provider": testCfg.LLMProvider, "deep_think_llm": testCfg.DeepThinkLLM})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":          "ok",
		"llm_provider":    testCfg.LLMProvider,
		"deep_think_llm":  testCfg.DeepThinkLLM,
		"quick_think_llm": testCfg.QuickThinkLLM,
		"latency_ms":      time.Since(start).Milliseconds(),
	})
}

func applyConfigUpdate(cfg *config.Config, req agents.ConfigUpdateRequest) {
	if req.LLMProvider != nil {
		cfg.LLMProvider = strings.TrimSpace(*req.LLMProvider)
	}
	if req.DeepSeekAPIKey != nil {
		cfg.DeepSeekAPIKey = strings.TrimSpace(*req.DeepSeekAPIKey)
	}
	if req.GoogleAPIKey != nil {
		cfg.GoogleAPIKey = strings.TrimSpace(*req.GoogleAPIKey)
	}
	if req.OpenAIAPIKey != nil {
		cfg.OpenAIAPIKey = strings.TrimSpace(*req.OpenAIAPIKey)
		cfg.OpenAICompatAPIKey = strings.TrimSpace(*req.OpenAIAPIKey)
	}
	if req.DeepThinkLLM != nil {
		cfg.DeepThinkLLM = strings.TrimSpace(*req.DeepThinkLLM)
	}
	if req.QuickThinkLLM != nil {
		cfg.QuickThinkLLM = strings.TrimSpace(*req.QuickThinkLLM)
	}
	if req.LLMBackendURL != nil {
		cfg.LLMBackendURL = strings.TrimSpace(*req.LLMBackendURL)
	}
	if req.OutputLanguage != nil {
		cfg.OutputLanguage = strings.TrimSpace(*req.OutputLanguage)
	}
	if req.MaxDebateRounds != nil {
		cfg.MaxDebateRounds = *req.MaxDebateRounds
	}
	if req.MaxRiskRounds != nil {
		cfg.MaxRiskDiscussRounds = *req.MaxRiskRounds
	}
}

// HealthCheck handles GET /api/health
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":            "ok",
		"liveness":          "ok",
		"dataStatus":        "not_checked",
		"readinessEndpoint": "/api/readiness",
		"ws_clients":        h.hub.ClientCount(),
	})
}

func (h *Handler) saveResult(result *agents.AnalysisResult) error {
	if result == nil {
		return fmt.Errorf("cannot save nil analysis result")
	}
	serialized, err := json.Marshal(result)
	if err != nil {
		return err
	}
	var persisted agents.AnalysisResult
	if err := json.Unmarshal(serialized, &persisted); err != nil {
		return err
	}
	agents.ApplyPublicationGate(&persisted)
	dir := filepath.Join(h.config.ResultsDir, "api_results")
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Printf("[API] Failed to create result dir: %v", err)
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}

	safeTicker := filepath.Base(persisted.Ticker)
	safeDate := filepath.Base(persisted.TradeDate)
	safeRunID := filepath.Base(persisted.Audit.RunID)
	if safeRunID == "." || safeRunID == "" {
		safeRunID = "legacy"
	}
	filename := filepath.Join(dir, safeTicker+"_"+safeDate+"_"+safeRunID+".json")
	data, err := json.MarshalIndent(&persisted, "", "  ")
	if err != nil {
		log.Printf("[API] Failed to marshal result: %v", err)
		return err
	}
	temporary, err := os.CreateTemp(dir, ".result-*.tmp")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, filename); err != nil {
		log.Printf("[API] Failed to save result: %v", err)
		return err
	}
	if err := os.Chmod(filename, 0600); err != nil {
		return err
	}
	return nil
}

func (h *Handler) loadResults() []agents.AnalysisResult {
	results := h.loadResultsRaw()
	for i := range results {
		agents.ApplyPublicationGate(&results[i])
	}
	return results
}

func (h *Handler) loadResultsRaw() []agents.AnalysisResult {
	dir := filepath.Join(h.config.ResultsDir, "api_results")
	entries, err := os.ReadDir(dir)
	h.resultLoadErrors.Store(0)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			h.resultLoadErrors.Add(1)
		}
		return nil
	}

	var results []agents.AnalysisResult
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			h.resultLoadErrors.Add(1)
			continue
		}
		var r agents.AnalysisResult
		if err := json.Unmarshal(data, &r); err != nil {
			h.resultLoadErrors.Add(1)
			continue
		}
		results = append(results, r)
	}
	return results
}
