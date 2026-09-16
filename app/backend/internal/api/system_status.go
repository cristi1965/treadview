package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/agents"
	"trading-agents/internal/config"
	"trading-agents/internal/gpupricing"
	"trading-agents/internal/llm"
	"trading-agents/internal/market"
)

type systemStatusResponse struct {
	Status      string                 `json:"status"`
	CheckedAt   string                 `json:"checkedAt"`
	Runtime     systemRuntimeStatus    `json:"runtime"`
	DataFiles   []systemDataFileStatus `json:"dataFiles"`
	Endpoints   []systemEndpointStatus `json:"endpoints"`
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
	AlpacaConfigured  bool   `json:"alpacaConfigured"`
	USQuoteFeed       string `json:"usQuoteFeed"`
	CNQuoteFeed       string `json:"cnQuoteFeed"`
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

type systemEndpointStatus struct {
	Domain      string `json:"domain"`
	Endpoint    string `json:"endpoint"`
	Source      string `json:"source"`
	DataTime    string `json:"dataTime,omitempty"`
	Stale       bool   `json:"stale"`
	StaleReason string `json:"staleReason,omitempty"`
	Refreshable bool   `json:"refreshable"`
	Status      string `json:"status"`
	Session     string `json:"session,omitempty"`
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
	response := h.currentSystemStatus()
	setDataFreshness(c, systemCapabilityFreshness(response))
	c.JSON(http.StatusOK, response)
}

// GetDataReadiness is the data-bearing readiness probe. Unlike /api/health it
// returns 503 whenever required sources or freshness checks are degraded.
func (h *Handler) GetDataReadiness(c *gin.Context) {
	profile := strings.ToLower(strings.TrimSpace(c.Query("profile")))
	if profile != "" && profile != readinessProfileFull {
		h.getReadinessProfile(c, profile)
		return
	}
	response := h.currentSystemStatus()
	setDataFreshness(c, systemCapabilityFreshness(response))
	statusCode := http.StatusOK
	if response.Status != "ok" {
		statusCode = http.StatusServiceUnavailable
	}
	c.JSON(statusCode, gin.H{
		"status": response.Status, "dataStatus": response.Status,
		"checkedAt": response.CheckedAt, "endpoints": response.Endpoints,
		"checks": response.Checks, "suggestions": response.Suggestions,
	})
}

func (h *Handler) currentSystemStatus() systemStatusResponse {
	root := findBackendRootForStocks()
	now := time.Now()
	marketStatus := market.Default().USPayloadStatus(now)
	cnMarketStatus := market.Default().CNPayloadStatus(now)
	whalesStatus := whalesSystemFreshnessMeta()
	qdiiStatus := dataFreshnessMeta{Source: "eastmoney:FundMNFInfo", DataTime: "unknown", Stale: true, StaleReason: "open QDII tools to check current premiums", Refreshable: true}
	response := collectSystemStatusWithFreshness(root, h.config, h.hub.ClientCount(), &marketStatus, &whalesStatus, &qdiiStatus, &cnMarketStatus)
	var client llm.LLMClient
	if h.orchestrator != nil {
		client = h.orchestrator.GetLLMClient()
	}
	if reason, unavailable := llm.UnavailableReason(client); unavailable {
		for index := range response.Checks {
			if response.Checks[index].ID == "llm.provider" {
				response.Checks[index].Status = "warn"
				response.Checks[index].Severity = "high"
				response.Checks[index].Detail = "runtime unavailable: " + reason
				response.Checks[index].Remedy = "Configure and test the selected LLM provider before starting agent research."
				break
			}
		}
		response.Status = "degraded"
	}
	gpuSnapshot := h.gpuPriceSnapshot()
	gpuEndpoint := gpuPriceSystemEndpoint(gpuSnapshot)
	response.Endpoints = append(response.Endpoints, gpuEndpoint)
	return response
}

func systemCapabilityFreshness(response systemStatusResponse) dataFreshnessMeta {
	meta := dataFreshnessMeta{Source: "core-capability-checks", Stale: response.Status != "ok"}
	if meta.Stale {
		meta.StaleReason = "one or more core capability checks failed"
	}
	return meta
}

func gpuPriceSystemEndpoint(snapshot gpupricing.Snapshot) systemEndpointStatus {
	freshness := gpuPriceFreshness(snapshot)
	status := snapshot.Status
	if status == "" {
		status = gpupricing.StatusUnavailable
	}
	return systemEndpointStatus{
		Domain: "GPU算力", Endpoint: "/api/gpu-prices", Source: freshness.Source,
		DataTime: freshness.DataTime, Stale: freshness.Stale, StaleReason: freshness.StaleReason,
		Refreshable: freshness.Refreshable, Status: status,
	}
}

func collectSystemStatus(backendRoot string, cfg *config.Config, wsClients int) systemStatusResponse {
	whalesStatus := missingWhalesFreshness("whales-db", "runtime disclosure freshness not injected")
	qdiiStatus := dataFreshnessMeta{Source: "eastmoney:FundMNFInfo", DataTime: "unknown", Stale: true, StaleReason: "runtime QDII freshness not injected", Refreshable: true}
	return collectSystemStatusWithFreshness(backendRoot, cfg, wsClients, nil, &whalesStatus, &qdiiStatus)
}

func collectSystemStatusWithFreshness(backendRoot string, cfg *config.Config, wsClients int, usMarket *market.PayloadStatus, whales, qdii *dataFreshnessMeta, cnMarkets ...*market.PayloadStatus) systemStatusResponse {
	files := collectSystemDataFiles(backendRoot)
	endpoints := buildSystemEndpointStatuses(files, usMarket, whales, qdii, cnMarkets...)
	checks := buildSystemChecks(files, cfg)
	status := "ok"
	for _, check := range checks {
		if check.Status != "ok" {
			status = "degraded"
			break
		}
	}
	alpacaConfigured := market.Default().AlpacaConfigured()
	usQuoteFeed := "public web sources (experimental)"
	if alpacaConfigured {
		usQuoteFeed = "Alpaca IEX (free, not SIP)"
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
			AlpacaConfigured:  alpacaConfigured,
			USQuoteFeed:       usQuoteFeed,
			CNQuoteFeed:       "Eastmoney public endpoint (experimental)",
		},
		DataFiles:   files,
		Endpoints:   endpoints,
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
			ID: "market-snapshot",
			Candidates: []string{
				filepath.Join(backendRoot, "data", "market.json"),
			},
			Required: false,
		},
		{
			ID: "macro",
			Candidates: []string{
				filepath.Join(backendRoot, "data", "macro.json"),
				filepath.Join(backendRoot, "data", "macro-verified.json"),
			},
			Required: false,
		},
		{
			ID: "etf-analyses",
			Candidates: []string{
				filepath.Join(backendRoot, "data", "etf-analyses-current.json"),
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

func buildSystemEndpointStatuses(files []systemDataFileStatus, usMarket *market.PayloadStatus, whales, qdii *dataFreshnessMeta, cnMarkets ...*market.PayloadStatus) []systemEndpointStatus {
	byID := make(map[string]systemDataFileStatus, len(files))
	for _, file := range files {
		byID[file.ID] = file
	}
	usEndpoint := marketEndpointFromFile(byID["market-snapshot"])
	if usMarket != nil {
		usEndpoint = endpointFromMarketStatus("US行情", "/api/market", *usMarket)
	}
	cnEndpoint := endpointFromFile("A股行情", "/api/a-market", "a-market", byID["a-market"], true, 90*time.Second)
	if len(cnMarkets) > 0 && cnMarkets[0] != nil {
		cnEndpoint = endpointFromMarketStatus("A股行情", "/api/a-market", *cnMarkets[0])
	}
	whalesEndpoint := endpointFromFreshness("Whales", "/api/whales/*", dataFreshnessMeta{Source: "whales-db", DataTime: "unknown", Stale: true, StaleReason: "missing disclosure data time", Refreshable: true})
	if whales != nil {
		whalesEndpoint = endpointFromFreshness("Whales", "/api/whales/*", *whales)
	}
	qdiiEndpoint := qdiiEndpointFromFreshness(dataFreshnessMeta{Source: "eastmoney:FundMNFInfo", DataTime: "unknown", Stale: true, StaleReason: "QDII premium freshness unavailable", Refreshable: true})
	if qdii != nil {
		qdiiEndpoint = qdiiEndpointFromFreshness(*qdii)
	}
	usStocksEndpoint := endpointFromFile("US列表/热力图", "/api/stocks?market=us", "us-stocks", byID["us-stocks"], true, 90*time.Second)
	if usMarket != nil {
		usStocksEndpoint = endpointFromMarketStatus("US列表/热力图", "/api/stocks?market=us", *usMarket)
	}
	macroEndpoint := readOnlySnapshotEndpoint("宏观", "/api/macro", "stale-snapshot:macro", byID["macro"])
	if macroEndpoint.Status != "unavailable" {
		macroEndpoint.Refreshable = true
		macroEndpoint.StaleReason = "live macro source unavailable; serving last real snapshot"
		if observedAt, ok := parseLooseDataTime(macroEndpoint.DataTime); ok {
			macroEndpoint.DataTime = observedAt.UTC().Format(time.RFC3339)
		}
	}
	moversEndpoint := readOnlySnapshotEndpoint("异动", "/api/premarket-movers", "stale-snapshot:premarket-movers", byID["premarket-movers"])
	return []systemEndpointStatus{
		usEndpoint,
		usStocksEndpoint,
		cnEndpoint,
		panelEndpointFromFiles(byID["us-panel-summary"], byID["us-stocks"]),
		macroEndpoint,
		moversEndpoint,
		reportsEndpointFromFile(byID["reports-live"]),
		optionalEndpointFromFile("ETF", "/api/etf/sectors", "etf-analyses", byID["etf-analyses"], 7*24*time.Hour),
		qdiiEndpoint,
		whalesEndpoint,
		{Domain: "Notes", Endpoint: "/api/notes/*", Source: "local-notes", Refreshable: false, Status: "local-authored"},
	}
}

func qdiiEndpointFromFreshness(freshness dataFreshnessMeta) systemEndpointStatus {
	endpoint := endpointFromFreshness("QDII 溢价", "/api/etf/premiums", freshness)
	if strings.TrimSpace(freshness.DataTime) == "" || strings.EqualFold(strings.TrimSpace(freshness.DataTime), "unknown") {
		endpoint.DataTime = "unknown"
		endpoint.Stale = true
		endpoint.Status = "unknown"
	}
	return endpoint
}

func reportsEndpointFromFile(file systemDataFileStatus) systemEndpointStatus {
	endpoint := systemEndpointStatus{
		Domain: "盘报", Endpoint: "/api/reports", Source: "reports-snapshot:reports-live.json", DataTime: file.GeneratedAt, Refreshable: false, Status: "ok",
	}
	if !file.Exists || file.Error != "" {
		endpoint.Stale = true
		endpoint.Status = "unavailable"
		endpoint.StaleReason = firstNonEmpty(file.Error, "report snapshot unavailable")
		return endpoint
	}
	endpoint.Stale, endpoint.StaleReason = staleIfOlder(file.GeneratedAt, 24*time.Hour)
	if endpoint.Stale {
		endpoint.Status = "stale"
	}
	return endpoint
}

func panelEndpointFromFiles(panel, universe systemDataFileStatus) systemEndpointStatus {
	endpoint := systemEndpointStatus{
		Domain: "五方分", Endpoint: "/api/panel-summary", Source: "panel-summary+us-stocks", Refreshable: true, Status: "ok",
	}
	if !panel.Exists || panel.Error != "" {
		endpoint.Stale = true
		endpoint.Status = "unavailable"
		endpoint.StaleReason = firstNonEmpty(panel.Error, "panel snapshot unavailable")
		return endpoint
	}
	if !universe.Exists || universe.Error != "" {
		endpoint.DataTime = firstNonEmpty(panel.GeneratedAt, "unknown")
		endpoint.Stale = true
		endpoint.Status = "stale"
		endpoint.StaleReason = firstNonEmpty(universe.Error, "critical us-stocks input unavailable")
		return endpoint
	}
	panelTime, panelOK := parseLooseDataTime(panel.GeneratedAt)
	universeTime, universeOK := parseLooseDataTime(universe.GeneratedAt)
	if !panelOK || !universeOK {
		endpoint.DataTime = "unknown"
		endpoint.Stale = true
		endpoint.Status = "stale"
		endpoint.StaleReason = "panel or us-stocks data time is unknown"
		return endpoint
	}
	oldest := panelTime
	if universeTime.Before(oldest) {
		oldest = universeTime
	}
	endpoint.DataTime = oldest.UTC().Format(time.RFC3339)
	panelStale, panelReason := staleIfOlder(panel.GeneratedAt, panelSummaryFreshFor)
	inputStale, inputReason := staleIfOlder(universe.GeneratedAt, 24*time.Hour)
	endpoint.Stale = panelStale || inputStale
	if panelStale {
		endpoint.StaleReason = "panel: " + panelReason
	}
	if inputStale {
		if endpoint.StaleReason != "" {
			endpoint.StaleReason += "; "
		}
		endpoint.StaleReason += "us-stocks: " + inputReason
	}
	if endpoint.Stale {
		endpoint.Status = "stale"
	}
	return endpoint
}

func readOnlySnapshotEndpoint(domain, path, source string, file systemDataFileStatus) systemEndpointStatus {
	endpoint := endpointFromFile(domain, path, source, file, false, 24*time.Hour)
	if !file.Exists || file.Error != "" {
		endpoint.Status = "unavailable"
		return endpoint
	}
	endpoint.Stale = true
	endpoint.Status = "stale"
	endpoint.StaleReason = "read-only snapshot; no live refresh was requested"
	return endpoint
}

func marketEndpointFromFile(file systemDataFileStatus) systemEndpointStatus {
	endpoint := endpointFromFile("US行情", "/api/market", "stale-snapshot:us", file, true, 90*time.Second)
	if file.Exists && file.Error == "" {
		endpoint.Stale = true
		endpoint.Status = "stale"
		endpoint.StaleReason = "live US market unavailable; serving last real snapshot"
	}
	return endpoint
}

func endpointFromMarketStatus(domain, path string, status market.PayloadStatus) systemEndpointStatus {
	reason := marketPayloadStaleReason(status.Source, status.Stale)
	return systemEndpointStatus{
		Domain: domain, Endpoint: path, Source: status.Source, DataTime: dataTimeOrEmpty(status.DataTime),
		Stale: status.Stale, StaleReason: reason, Refreshable: true, Status: map[bool]string{true: "stale", false: "ok"}[status.Stale], Session: status.Session,
	}
}

func endpointFromFreshness(domain, path string, freshness dataFreshnessMeta) systemEndpointStatus {
	status := "ok"
	if freshness.Stale {
		status = "stale"
	}
	return systemEndpointStatus{
		Domain: domain, Endpoint: path, Source: freshness.Source, DataTime: freshness.DataTime,
		Stale: freshness.Stale, StaleReason: freshness.StaleReason, Refreshable: freshness.Refreshable, Status: status,
	}
}

func endpointFromFile(domain, endpoint, source string, file systemDataFileStatus, refreshable bool, maxAge time.Duration) systemEndpointStatus {
	dataTime := firstNonEmpty(file.GeneratedAt, file.ModifiedAt)
	stale := false
	reason := ""
	if !file.Exists {
		stale = true
		reason = "required source file missing"
	} else if file.Error != "" {
		stale = true
		reason = file.Error
	} else {
		stale, reason = staleIfOlder(dataTime, maxAge)
	}
	status := "ok"
	if stale {
		status = "stale"
	}
	return systemEndpointStatus{
		Domain:      domain,
		Endpoint:    endpoint,
		Source:      source,
		DataTime:    dataTime,
		Stale:       stale,
		StaleReason: reason,
		Refreshable: refreshable,
		Status:      status,
	}
}

func optionalEndpointFromFile(domain, endpoint, source string, file systemDataFileStatus, maxAge time.Duration) systemEndpointStatus {
	if !file.Exists {
		return systemEndpointStatus{
			Domain:      domain,
			Endpoint:    endpoint,
			Source:      source,
			Stale:       true,
			StaleReason: "source file unavailable",
			Refreshable: false,
			Status:      "unavailable",
		}
	}
	return endpointFromFile(domain, endpoint, source, file, false, maxAge)
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
		if status.Error == "" && spec.ID == "etf-analyses" {
			var data etfAnalysesFile
			if err := json.Unmarshal(raw, &data); err != nil {
				status.Error = err.Error()
			} else if err := normalizeETFAnalysisAUM(&data); err != nil {
				status.Error = err.Error()
			} else if err := validateCurrentETFAnalysis(data); err != nil {
				status.Error = err.Error()
			}
		}
		if status.Error == "" && spec.ID == "macro" && status.Count > 0 && !hasPositiveSeries(raw) {
			lastErr = errors.New("macro snapshot contains no priced series")
			status = systemDataFileStatus{ID: spec.ID}
			continue
		}
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

func hasPositiveSeries(raw []byte) bool {
	var payload struct {
		Series []struct {
			Price float64 `json:"price"`
		} `json:"series"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	for _, item := range payload.Series {
		if item.Price > 0 {
			return true
		}
	}
	return false
}

func inspectJSONMetadata(raw []byte) (string, int, string) {
	var anyValue any
	if err := json.Unmarshal(raw, &anyValue); err != nil {
		return "", 0, err.Error()
	}
	switch v := anyValue.(type) {
	case []any:
		latest := time.Time{}
		for _, item := range v {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			value := firstString(row, "publishedAt", "generated_at", "generatedAt", "updated")
			if ts, ok := parseLooseDataTime(value); ok && ts.After(latest) {
				latest = ts
			}
		}
		if latest.IsZero() {
			return "", len(v), ""
		}
		return latest.UTC().Format(time.RFC3339), len(v), ""
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
	checks = append(checks, researchEvidenceCheck(cfg))
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

func researchEvidenceCheck(cfg *config.Config) systemCheck {
	check := systemCheck{
		ID:       "research.evidence",
		Status:   "warn",
		Severity: "high",
		Remedy:   "Run an authenticated analysis and retain run_id, input_hash, evidence, and claim links.",
	}
	if cfg == nil || strings.TrimSpace(cfg.ResultsDir) == "" {
		check.Detail = "analysis results directory is not configured"
		return check
	}
	dir := filepath.Join(cfg.ResultsDir, "api_results")
	entries, err := os.ReadDir(dir)
	if err != nil {
		check.Detail = "no persisted research results"
		return check
	}
	total := 0
	complete := 0
	evidenceOnly := 0
	legacyArchive := 0
	unavailableArchive := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			total++
			continue
		}
		var result agents.AnalysisResult
		if json.Unmarshal(raw, &result) != nil {
			total++
			continue
		}
		if result.Status == "" && result.ResearchMode == "" && result.Audit.RunID == "" && result.Audit.MethodVersion == "" {
			legacyArchive++
			continue
		}
		if result.Status == agents.ResearchStatusUnavailable &&
			result.Decision == agents.ResearchDecisionUnavailable &&
			strings.TrimSpace(result.Audit.RunID) != "" &&
			strings.TrimSpace(result.Audit.InputHash) != "" {
			unavailableArchive++
			continue
		}
		total++
		if health := agents.ApplyPublicationGate(&result); health.Publishable {
			switch result.Status {
			case agents.ResearchStatusPublished:
				complete++
			case agents.ResearchStatusEvidenceOnly:
				complete++
				evidenceOnly++
			}
		}
	}
	if total == 0 {
		check.Detail = fmt.Sprintf("no publishable auditable research results; unavailable archive=%d; legacy archive=%d", unavailableArchive, legacyArchive)
		return check
	}
	if complete != total {
		check.Detail = fmt.Sprintf("complete evidence chains=%d/%d; invalid evidence chains=%d; unavailable archive=%d; legacy archive=%d", complete, total, total-complete, unavailableArchive, legacyArchive)
		return check
	}
	check.Status = "ok"
	check.Severity = ""
	check.Remedy = ""
	check.Detail = fmt.Sprintf("complete evidence chains=%d/%d; evidence-only dossiers=%d; unavailable archive=%d; legacy archive=%d", complete, total, evidenceOnly, unavailableArchive, legacyArchive)
	return check
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
