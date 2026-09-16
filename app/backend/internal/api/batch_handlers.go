package api

import (
	"context"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"trading-agents/internal/agents"
)

var safeTickerRe = regexp.MustCompile(`^[A-Z0-9._-]{1,20}$`)
var safeDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func validTradeDate(value string) bool {
	if !safeDateRe.MatchString(value) {
		return false
	}
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

type batchAnalysisRequest struct {
	Tickers   []string `json:"tickers"`
	TradeDate string   `json:"trade_date"`
	AssetType string   `json:"asset_type"`
}

type batchJobStatus struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"` // queued|running|done|error
	Tickers     []string  `json:"tickers"`
	Done        []string  `json:"done"`
	Unavailable []string  `json:"unavailable,omitempty"`
	Current     string    `json:"current,omitempty"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// StartBatchAnalysis POST /api/analysis/batch — queue up to 10 tickers sequentially.
func (h *Handler) StartBatchAnalysis(c *gin.Context) {
	var req batchAnalysisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tickers := make([]string, 0, len(req.Tickers))
	seen := map[string]bool{}
	for _, t := range req.Tickers {
		t = strings.ToUpper(strings.TrimSpace(t))
		if t == "" || seen[t] || !safeTickerRe.MatchString(t) {
			continue
		}
		seen[t] = true
		tickers = append(tickers, t)
	}
	if len(tickers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tickers required (alphanumeric only)"})
		return
	}
	if len(tickers) > 10 {
		tickers = tickers[:10]
	}
	if req.TradeDate == "" {
		req.TradeDate = time.Now().Format("2006-01-02")
	}
	if !validTradeDate(req.TradeDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trade_date must be YYYY-MM-DD"})
		return
	}
	if req.AssetType == "" {
		req.AssetType = "stock"
	}

	h.mu.Lock()
	if h.batchRunning || h.isRunning {
		h.mu.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "analysis slot already running"})
		return
	}
	ctx, cancel := h.newAnalysisContext()
	h.runningID++
	runID := h.runningID
	id := time.Now().Format("20060102-150405")
	h.batchID = id
	h.batchRunning = true
	h.isRunning = true
	h.isStopping = false
	h.runningCtx = ctx
	h.runningCancel = cancel
	h.batchStatus = batchJobStatus{
		ID: id, Status: "queued", Tickers: tickers, Done: []string{}, CreatedAt: time.Now(),
	}
	h.mu.Unlock()

	if h.orchestrator != nil {
		h.orchestrator.SetEventHandler(func(event agents.NodeEvent) {
			h.hub.Broadcast(event)
		})
	}

	go h.runBatch(ctx, runID, tickers, req.TradeDate, req.AssetType)

	c.JSON(http.StatusAccepted, gin.H{"status": "started", "id": id, "tickers": tickers})
}

// GetBatchAnalysis GET /api/analysis/batch/:id
func (h *Handler) GetBatchAnalysis(c *gin.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := c.Param("id")
	if id != "" && h.batchID != "" && id != h.batchID {
		c.JSON(http.StatusNotFound, gin.H{"error": "batch not found"})
		return
	}
	c.JSON(http.StatusOK, h.batchStatus)
}

func (h *Handler) runBatch(ctx context.Context, runID uint64, tickers []string, tradeDate, assetType string) {
	defer func() {
		h.mu.Lock()
		if h.runningID == runID {
			h.batchRunning = false
			h.isRunning = false
			h.isStopping = false
			h.runningCtx = nil
			h.runningCancel = nil
			switch {
			case h.batchStatus.Status == "cancelled":
			case h.batchStatus.Error != "":
				h.batchStatus.Status = "error"
			case len(h.batchStatus.Unavailable) > 0:
				h.batchStatus.Status = "degraded"
			default:
				h.batchStatus.Status = "done"
			}
		}
		h.mu.Unlock()
	}()

	for _, ticker := range tickers {
		if err := ctx.Err(); err != nil {
			h.mu.Lock()
			h.batchStatus.Status = "cancelled"
			h.batchStatus.Error = err.Error()
			h.mu.Unlock()
			return
		}
		h.mu.Lock()
		h.batchStatus.Status = "running"
		h.batchStatus.Current = ticker
		h.mu.Unlock()

		runner := h.runAnalysis
		if runner == nil {
			runner = h.orchestrator.RunAnalysis
		}
		result, err := runner(ctx, agents.AnalysisRequest{
			Ticker: ticker, TradeDate: tradeDate, AssetType: assetType,
		})

		if err != nil {
			log.Printf("[API] batch %s error: %v", ticker, err)
			h.mu.Lock()
			h.batchStatus.Error = err.Error()
			if ctx.Err() != nil {
				h.batchStatus.Status = "cancelled"
				h.mu.Unlock()
				return
			}
			h.mu.Unlock()
			h.hub.Broadcast(agents.NodeEvent{Type: "batch_item_error", Node: "System", Content: ticker + ": " + err.Error(), Status: "error"})
		} else {
			agents.ApplyPublicationGate(result)
			if err := h.saveResult(result); err != nil {
				log.Printf("[API] batch %s save failed: %v", ticker, err)
			}
			h.mu.Lock()
			h.lastResult = result
			h.results = append(h.results, *result)
			var event agents.NodeEvent
			if result.Status == agents.ResearchStatusUnavailable {
				h.batchStatus.Unavailable = append(h.batchStatus.Unavailable, ticker)
				h.batchStatus.Status = "degraded"
				event = agents.NodeEvent{Type: "batch_item_unavailable", Node: "System", Content: ticker + ": research unavailable", Status: "degraded"}
			} else {
				h.batchStatus.Done = append(h.batchStatus.Done, ticker)
				event = agents.NodeEvent{Type: "batch_item_done", Node: "System", Content: ticker, Status: "done"}
			}
			h.mu.Unlock()
			h.hub.Broadcast(event)
		}
	}
}
