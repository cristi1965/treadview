package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"trading-agents/internal/agents"
	"trading-agents/internal/config"
)

func TestSingleAndBatchAnalysisShareOneRunSlot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	batchBlocked := &Handler{isRunning: true}
	batchRouter := gin.New()
	batchRouter.POST("/batch", batchBlocked.StartBatchAnalysis)
	batchReq := httptest.NewRequest(http.MethodPost, "/batch", bytes.NewBufferString(`{"tickers":["NVDA"],"trade_date":"2026-09-06"}`))
	batchReq.Header.Set("Content-Type", "application/json")
	batchResponse := httptest.NewRecorder()
	batchRouter.ServeHTTP(batchResponse, batchReq)
	if batchResponse.Code != http.StatusConflict {
		t.Fatalf("batch must not enter occupied single slot: status=%d body=%s", batchResponse.Code, batchResponse.Body.String())
	}

	singleBlocked := &Handler{isRunning: true, batchRunning: true}
	singleRouter := gin.New()
	singleRouter.POST("/single", singleBlocked.StartAnalysis)
	singleReq := httptest.NewRequest(http.MethodPost, "/single", bytes.NewBufferString(`{"ticker":"NVDA","trade_date":"2026-09-06"}`))
	singleReq.Header.Set("Content-Type", "application/json")
	singleResponse := httptest.NewRecorder()
	singleRouter.ServeHTTP(singleResponse, singleReq)
	if singleResponse.Code != http.StatusConflict {
		t.Fatalf("single must not enter occupied batch slot: status=%d body=%s", singleResponse.Code, singleResponse.Body.String())
	}
}

func TestStopKeepsRunSlotUntilWorkerExits(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithCancel(context.Background())
	handler := &Handler{hub: NewHub(), isRunning: true, runningCtx: ctx, runningCancel: cancel, runningID: 7}
	router := gin.New()
	router.POST("/stop", handler.StopAnalysis)
	router.GET("/status", handler.GetStatus)

	stopReq := httptest.NewRequest(http.MethodPost, "/stop", nil)
	stopResponse := httptest.NewRecorder()
	router.ServeHTTP(stopResponse, stopReq)
	if stopResponse.Code != http.StatusOK || ctx.Err() == nil {
		t.Fatalf("stop did not cancel active context: status=%d body=%s", stopResponse.Code, stopResponse.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/status", nil)
	statusResponse := httptest.NewRecorder()
	router.ServeHTTP(statusResponse, statusReq)
	if statusResponse.Code != http.StatusOK || !bytes.Contains(statusResponse.Body.Bytes(), []byte(`"status":"stopping"`)) {
		t.Fatalf("run slot released before worker exit: status=%d body=%s", statusResponse.Code, statusResponse.Body.String())
	}
}

func TestAnalysisFailureBecomesPersistentTerminalResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &Handler{
		config:     &config.Config{ResultsDir: t.TempDir(), LLMProvider: "dual", QuickThinkLLM: "quick", DeepThinkLLM: "deep"},
		hub:        NewHub(),
		lastResult: &agents.AnalysisResult{Ticker: "OLD", Status: agents.ResearchStatusPublished},
		runAnalysis: func(context.Context, agents.AnalysisRequest) (*agents.AnalysisResult, error) {
			return nil, errors.New("provider timeout")
		},
	}
	router := gin.New()
	router.POST("/start", handler.StartAnalysis)
	router.GET("/status", handler.GetStatus)

	started := paperRequest(t, router, http.MethodPost, "/start", map[string]any{"ticker": "NVDA", "trade_date": "2026-09-06"})
	if started.Code != http.StatusAccepted {
		t.Fatalf("start status=%d body=%s", started.Code, started.Body.String())
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := httptest.NewRecorder()
		router.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/status", nil))
		var body struct {
			Status     string                 `json:"status"`
			LastResult *agents.AnalysisResult `json:"last_result"`
		}
		if json.Unmarshal(status.Body.Bytes(), &body) == nil && body.Status == "idle" && body.LastResult != nil {
			if body.LastResult.Ticker != "NVDA" || body.LastResult.Status != agents.ResearchStatusUnavailable || !strings.Contains(body.LastResult.Message, "provider timeout") {
				t.Fatalf("failure terminal result=%+v", body.LastResult)
			}
			if len(handler.loadResultsRaw()) != 1 {
				t.Fatal("failure terminal result was not persisted")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("analysis failure did not reach terminal status")
}

func TestBatchFinalStatusPreservesErrorAndDegradation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		withError  bool
		wantStatus string
	}{
		{name: "unavailable", wantStatus: "degraded"},
		{name: "error", withError: true, wantStatus: "error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := &Handler{config: &config.Config{ResultsDir: t.TempDir()}, hub: NewHub()}
			handler.runAnalysis = func(_ context.Context, req agents.AnalysisRequest) (*agents.AnalysisResult, error) {
				if tc.withError && req.Ticker == "MSFT" {
					return nil, errors.New("provider failed")
				}
				return &agents.AnalysisResult{Ticker: req.Ticker, TradeDate: req.TradeDate, Status: agents.ResearchStatusUnavailable}, nil
			}
			router := gin.New()
			router.POST("/batch", handler.StartBatchAnalysis)
			router.GET("/batch/:id", handler.GetBatchAnalysis)
			started := paperRequest(t, router, http.MethodPost, "/batch", map[string]any{"tickers": []string{"NVDA", "MSFT"}, "trade_date": "2026-09-06"})
			if started.Code != http.StatusAccepted {
				t.Fatalf("start status=%d body=%s", started.Code, started.Body.String())
			}
			var startBody struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(started.Body.Bytes(), &startBody); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(time.Second)
			for time.Now().Before(deadline) {
				status := httptest.NewRecorder()
				router.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/batch/"+startBody.ID, nil))
				var job batchJobStatus
				if json.Unmarshal(status.Body.Bytes(), &job) == nil && (job.Status == "done" || job.Status == "degraded" || job.Status == "error" || job.Status == "cancelled") {
					if job.Status != tc.wantStatus {
						t.Fatalf("final status=%q want=%q job=%+v", job.Status, tc.wantStatus, job)
					}
					return
				}
				time.Sleep(time.Millisecond)
			}
			t.Fatal("batch did not finish")
		})
	}
}
