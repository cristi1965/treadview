package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/config"
)

type systemStatusResponse struct {
	Status      string                 `json:"status"`
	CheckedAt   string                 `json:"checkedAt"`
	Runtime     systemRuntimeStatus    `json:"runtime"`
	DataFiles   []systemDataFileStatus `json:"dataFiles"`
	Checks      []systemCheck          `json:"checks"`
	Suggestions []string               `json:"suggestions,omitempty"`
}

type systemRuntimeStatus struct {
	Port              string `json:"port"`
	LLMProvider       string `json:"llmProvider"`
	DeepThinkLLM      string `json:"deepThinkLLM,omitempty"`
	QuickThinkLLM     string `json:"quickThinkLLM,omitempty"`
	LiveMirrorEnabled bool   `json:"liveMirrorEnabled"`
	ReplayEnabled     bool   `json:"replayEnabled"`
	WSClients         int    `json:"wsClients"`
}

type systemDataFileStatus struct {
	ID              string `json:"id"`
	Path            string `json:"path,omitempty"`
	Exists          bool   `json:"exists"`
	SizeBytes       int64  `json:"sizeBytes,omitempty"`
	ModifiedAt      string `json:"modifiedAt,omitempty"`
	GeneratedAt     string `json:"generatedAt,omitempty"`
	Count           int    `json:"count,omitempty"`
	BrandViolations int    `json:"brandViolations,omitempty"`
	Error           string `json:"error,omitempty"`
}

type systemCheck struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Remedy   string `json:"remedy,omitempty"`
	Severity string `json:"severity,omitempty"`
}

type dataFileSpec struct {
	ID              string
	Candidates      []string
	CheckBrandText  bool
	Required        bool
	ExpectedMinimum int
}

// GetSystemStatus handles GET /api/system/status.
func (h *Handler) GetSystemStatus(c *gin.Context) {
	root := findBackendRootForStocks()
	c.JSON(http.StatusOK, collectSystemStatus(root, h.config, h.hub.ClientCount()))
}

func collectSystemStatus(backendRoot string, cfg *config.Config, wsClients int) systemStatusResponse {
	files := collectSystemDataFiles(backendRoot)
	checks := buildSystemChecks(files, cfg)
	status := "ok"
	for _, check := range checks {
		if check.Status != "ok" {
			status = "degraded"
			break
		}
	}

	return systemStatusResponse{
		Status:    status,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Runtime: systemRuntimeStatus{
			Port:              cfg.Port,
			LLMProvider:       cfg.LLMProvider,
			DeepThinkLLM:      cfg.DeepThinkLLM,
			QuickThinkLLM:     cfg.QuickThinkLLM,
			LiveMirrorEnabled: strings.EqualFold(os.Getenv("STOCKGOD_LIVE_MIRROR"), "true"),
			ReplayEnabled:     strings.EqualFold(os.Getenv("STOCKGOD_REPLAY"), "true"),
			WSClients:         wsClients,
		},
		DataFiles:   files,
		Checks:      checks,
		Suggestions: systemStatusSuggestions(checks),
	}
}

func collectSystemDataFiles(backendRoot string) []systemDataFileStatus {
	specs := []dataFileSpec{
		{
			ID: "us-stocks",
			Candidates: []string{
				filepath.Join(backendRoot, "..", "frontend", "public", "data", "us-stocks.json"),
				filepath.Join(backendRoot, "..", "frontend", "dist", "data", "us-stocks.json"),
				filepath.Join(backendRoot, "data", "us-stocks.json"),
			},
			Required: true, ExpectedMinimum: 1,
		},
		{
			ID: "us-panel-summary",
			Candidates: []string{
				filepath.Join(backendRoot, "..", "frontend", "public", "data", "us-panel-summary.json"),
				filepath.Join(backendRoot, "data", "us-panel-summary.json"),
			},
			Required: true, ExpectedMinimum: 1,
		},
		{
			ID: "a-market",
			Candidates: []string{
				filepath.Join(backendRoot, "data", "a-market.json"),
				filepath.Join(backendRoot, "..", "frontend", "public", "data", "a-market.json"),
			},
			Required: true, ExpectedMinimum: 1,
		},
		{
			ID: "market-fallback",
			Candidates: []string{
				filepath.Join(backendRoot, "data", "market.json"),
			},
			Required: false,
		},
		{
			ID: "macro",
			Candidates: []string{
				filepath.Join(backendRoot, "data", "macro.json"),
			},
			Required: false,
		},
		{
			ID: "premarket-movers",
			Candidates: []string{
				filepath.Join(backendRoot, "data", "premarket-movers.json"),
			},
			Required: false,
		},
		{
			ID: "reports-live",
			Candidates: []string{
				filepath.Join(backendRoot, "data", "reports-live.json"),
			},
			CheckBrandText: true, Required: true, ExpectedMinimum: 1,
		},
		{
			ID: "reports-static",
			Candidates: []string{
				filepath.Join(backendRoot, "..", "frontend", "public", "data", "reports.json"),
			},
			CheckBrandText: true, Required: false,
		},
	}

	out := make([]systemDataFileStatus, 0, len(specs))
	for _, spec := range specs {
		out = append(out, inspectSystemDataFile(spec))
	}
	return out
}

func inspectSystemDataFile(spec dataFileSpec) systemDataFileStatus {
	status := systemDataFileStatus{ID: spec.ID}
	var lastErr error
	for _, path := range spec.Candidates {
		info, err := os.Stat(path)
		if err != nil {
			lastErr = err
			continue
		}
		status.Path = path
		status.Exists = true
		status.SizeBytes = info.Size()
		status.ModifiedAt = info.ModTime().UTC().Format(time.RFC3339)
		raw, err := os.ReadFile(path)
		if err != nil {
			status.Error = err.Error()
			return status
		}
		status.GeneratedAt, status.Count, status.Error = inspectJSONMetadata(raw)
		if spec.CheckBrandText {
			status.BrandViolations = strings.Count(string(raw), "我不是股神")
		}
		return status
	}
	if lastErr != nil {
		status.Error = lastErr.Error()
	}
	return status
}

func inspectJSONMetadata(raw []byte) (string, int, string) {
	var anyValue any
	if err := json.Unmarshal(raw, &anyValue); err != nil {
		return "", 0, err.Error()
	}
	switch v := anyValue.(type) {
	case []any:
		return "", len(v), ""
	case map[string]any:
		generated := firstString(v, "generated_at", "generatedAt", "updated", "publishedAt")
		count := firstInt(v, "count", "n")
		if count == 0 {
			for _, key := range []string{"stocks", "quotes", "reports", "series", "gainers", "events"} {
				if arr, ok := v[key].([]any); ok {
					count = len(arr)
					break
				}
				if obj, ok := v[key].(map[string]any); ok {
					count = len(obj)
					break
				}
			}
		}
		if generated == "" {
			if ts := firstInt64(v, "ts"); ts > 0 {
				generated = fmt.Sprintf("ts:%d", ts)
			}
		}
		return generated, count, ""
	default:
		return "", 0, "unsupported json root"
	}
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func firstInt(m map[string]any, keys ...string) int {
	for _, key := range keys {
		if v, ok := m[key].(float64); ok && v > 0 {
			return int(v)
		}
	}
	return 0
}

func firstInt64(m map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if v, ok := m[key].(float64); ok && v > 0 {
			return int64(v)
		}
	}
	return 0
}

func buildSystemChecks(files []systemDataFileStatus, cfg *config.Config) []systemCheck {
	byID := make(map[string]systemDataFileStatus, len(files))
	for _, file := range files {
		byID[file.ID] = file
	}
	checks := []systemCheck{
		fileExistsCheck(byID["us-stocks"], "data.us_stocks", 1, "Run app/backend/cmd/regenmarket or restore app/frontend/public/data/us-stocks.json."),
		fileExistsCheck(byID["a-market"], "data.a_market", 1, "Restore app/backend/data/a-market.json."),
		fileExistsCheck(byID["reports-live"], "data.reports_live", 1, "Restore app/backend/data/reports-live.json."),
	}
	brandViolations := byID["reports-live"].BrandViolations + byID["reports-static"].BrandViolations
	if brandViolations > 0 {
		checks = append(checks, systemCheck{
			ID:       "content.brand",
			Status:   "warn",
			Detail:   fmt.Sprintf("reports contain legacy brand text 我不是股神 x%d", brandViolations),
			Remedy:   "Replace report body brand text with 我不是神.",
			Severity: "medium",
		})
	} else {
		checks = append(checks, systemCheck{ID: "content.brand", Status: "ok", Detail: "no legacy brand text"})
	}
	if strings.EqualFold(os.Getenv("STOCKGOD_LIVE_MIRROR"), "true") {
		checks = append(checks, systemCheck{
			ID:       "mode.live_mirror",
			Status:   "warn",
			Detail:   "STOCKGOD_LIVE_MIRROR=true",
			Remedy:   "Unset STOCKGOD_LIVE_MIRROR unless live mirror mode was explicitly requested.",
			Severity: "high",
		})
	} else {
		checks = append(checks, systemCheck{ID: "mode.live_mirror", Status: "ok", Detail: "live mirror disabled"})
	}
	if strings.TrimSpace(cfg.LLMProvider) == "" {
		checks = append(checks, systemCheck{ID: "llm.provider", Status: "warn", Detail: "LLM provider is empty", Severity: "low"})
	} else {
		checks = append(checks, systemCheck{ID: "llm.provider", Status: "ok", Detail: cfg.LLMProvider})
	}
	return checks
}

func fileExistsCheck(file systemDataFileStatus, id string, minCount int, remedy string) systemCheck {
	if !file.Exists {
		return systemCheck{ID: id, Status: "fail", Detail: "required file missing", Remedy: remedy, Severity: "high"}
	}
	if file.Error != "" {
		return systemCheck{ID: id, Status: "fail", Detail: file.Error, Remedy: remedy, Severity: "high"}
	}
	if minCount > 0 && file.Count < minCount {
		return systemCheck{ID: id, Status: "fail", Detail: fmt.Sprintf("count=%d", file.Count), Remedy: remedy, Severity: "high"}
	}
	return systemCheck{ID: id, Status: "ok", Detail: fmt.Sprintf("count=%d", file.Count)}
}

func systemStatusSuggestions(checks []systemCheck) []string {
	var out []string
	for _, check := range checks {
		if check.Status != "ok" && check.Remedy != "" {
			out = append(out, check.Remedy)
		}
	}
	return out
}
