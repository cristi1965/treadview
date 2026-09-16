package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/agents"
	"trading-agents/internal/config"
	"trading-agents/internal/gpupricing"
	"trading-agents/internal/llm"
	"trading-agents/internal/market"
	"trading-agents/internal/orchestrator"
)

func TestSystemStatusReportsUnavailableRuntimeLLM(t *testing.T) {
	cfg := &config.Config{Port: "8765", LLMProvider: "dual", QuickThinkLLM: "quick", DeepThinkLLM: "deep", ResultsDir: t.TempDir()}
	handler := &Handler{config: cfg, orchestrator: orchestrator.New(cfg, llm.NewUnavailableClient("missing provider key")), hub: NewHub()}
	status := handler.currentSystemStatus()
	check := profileCheckByID(status.Checks, "llm.provider")
	if status.Status != "degraded" || check.Status != "warn" || !strings.Contains(check.Detail, "unavailable") || !strings.Contains(check.Detail, "missing provider key") {
		t.Fatalf("unavailable runtime LLM was reported as healthy: status=%s check=%+v", status.Status, check)
	}
}

func TestCollectSystemStatus(t *testing.T) {
	root := t.TempDir()
	backendRoot := filepath.Join(root, "backend")
	now := time.Now().UnixMilli()
	mustWrite(t, filepath.Join(backendRoot, "data", "market.json"), fmt.Sprintf(`{"quotes":{"RKLB":{"price":64.68}},"ts":%d,"count":1}`, now))
	mustWrite(t, filepath.Join(backendRoot, "data", "a-market.json"), fmt.Sprintf(`{"quotes":{"601398":{"price":7.27,"pct":0.5,"vol":1000,"mcapYi":20000}},"ts":%d,"count":1}`, now))
	mustWrite(t, filepath.Join(backendRoot, "data", "macro.json"), fmt.Sprintf(`{"series":[],"ts":%d}`, now))
	mustWrite(t, filepath.Join(backendRoot, "data", "premarket-movers.json"), fmt.Sprintf(`{"items":[],"ts":%d}`, now))
	mustWrite(t, filepath.Join(backendRoot, "data", "reports-live.json"), `[{"id":"r1","content":"我不是神"}]`)
	mustWrite(t, filepath.Join(root, "frontend", "public", "data", "us-stocks.json"), fmt.Sprintf(`{"generated_at":"ts:%d","count":1,"stocks":[{"sym":"RKLB","price":64.68,"pct":10.38,"mcapB":38.69,"vol":18162886}]}`, now))
	mustWrite(t, filepath.Join(root, "frontend", "public", "data", "us-panel-summary.json"), `{"count":1,"stocks":{"RKLB":{"sc":[55,60,60,75,45],"div":30}}}`)
	mustWrite(t, filepath.Join(root, "frontend", "public", "data", "reports.json"), `[{"id":"r1","content":"我不是神"}]`)

	t.Setenv("STOCKGOD_LIVE_MIRROR", "")
	resp := collectSystemStatus(backendRoot, &config.Config{
		Port:          "8765",
		LLMProvider:   "deepseek",
		DeepThinkLLM:  "deepseek-chat",
		QuickThinkLLM: "deepseek-chat",
	}, 2)
	if resp.Status != "degraded" {
		t.Fatalf("status=%s checks=%+v", resp.Status, resp.Checks)
	}
	if endpoint := endpointByPath(resp.Endpoints, "/api/market"); !endpoint.Stale || endpoint.Status != "stale" {
		t.Fatalf("snapshot-backed direct market must remain stale: %+v", endpoint)
	}
	if endpoint := endpointByPath(resp.Endpoints, "/api/etf/sectors"); !endpoint.Stale || endpoint.Status != "unavailable" {
		t.Fatalf("optional unavailable endpoint must degrade status: %+v", endpoint)
	}
	if resp.Runtime.WSClients != 2 {
		t.Fatalf("ws clients=%d", resp.Runtime.WSClients)
	}
	if got := dataFileByID(resp.DataFiles, "us-stocks").Count; got != 1 {
		t.Fatalf("us-stocks count=%d", got)
	}
	if got := dataFileByID(resp.DataFiles, "reports-live").BrandViolations; got != 0 {
		t.Fatalf("brand violations=%d", got)
	}
}

func TestSystemStatusUsesDirectMarketAndWhalesFreshness(t *testing.T) {
	root := t.TempDir()
	backendRoot := filepath.Join(root, "backend")
	now := time.Now().UTC().Truncate(time.Second)
	mustWrite(t, filepath.Join(backendRoot, "data", "a-market.json"), fmt.Sprintf(`{"quotes":{"601398":{"price":7.27}},"ts":%d,"count":1}`, now.UnixMilli()))
	mustWrite(t, filepath.Join(backendRoot, "data", "reports-live.json"), `[{"id":"r1","content":"我不是神"}]`)
	mustWrite(t, filepath.Join(root, "frontend", "public", "data", "us-stocks.json"), fmt.Sprintf(`{"generated_at":"ts:%d","count":1,"stocks":[{"sym":"RKLB","price":64.68}]}`, now.UnixMilli()))
	mustWrite(t, filepath.Join(root, "frontend", "public", "data", "us-panel-summary.json"), fmt.Sprintf(`{"generated_at":"ts:%d","count":1,"stocks":{"RKLB":{"sc":[1,2,3,4,5]}}}`, now.UnixMilli()))

	marketMeta := market.PayloadStatus{Source: "live-tv@" + now.Format(time.RFC3339), DataTime: now, Stale: false}
	cnMarketMeta := market.PayloadStatus{Source: "stale-snapshot:cn@2020-01-02T20:00:00Z", DataTime: time.Date(2020, 1, 2, 20, 0, 0, 0, time.UTC), Stale: true}
	whalesMeta := dataFreshnessMeta{Source: "sec-edgar-13f", DataTime: "2026-06-30", Stale: true, StaleReason: "incomplete disclosure metadata", Refreshable: true}
	qdiiMeta := dataFreshnessMeta{Source: "eastmoney:FundMNFInfo", DataTime: "unknown", Stale: true, StaleReason: "incomplete QDII critical inputs", Refreshable: true}
	resp := collectSystemStatusWithFreshness(backendRoot, &config.Config{Port: "8765", LLMProvider: "deepseek"}, 0, &marketMeta, &whalesMeta, &qdiiMeta, &cnMarketMeta)

	marketEndpoint := endpointByPath(resp.Endpoints, "/api/market")
	if marketEndpoint.Source != marketMeta.Source || marketEndpoint.DataTime != now.Format(time.RFC3339) || marketEndpoint.Stale != marketMeta.Stale || marketEndpoint.Status != "ok" {
		t.Fatalf("market freshness drifted from direct payload: endpoint=%+v meta=%+v", marketEndpoint, marketMeta)
	}
	cnEndpoint := endpointByPath(resp.Endpoints, "/api/a-market")
	if cnEndpoint.Source != cnMarketMeta.Source || cnEndpoint.DataTime != cnMarketMeta.DataTime.Format(time.RFC3339) || cnEndpoint.Stale != cnMarketMeta.Stale {
		t.Fatalf("CN market freshness drifted from direct payload: endpoint=%+v meta=%+v", cnEndpoint, cnMarketMeta)
	}
	whalesEndpoint := endpointByPath(resp.Endpoints, "/api/whales/*")
	if whalesEndpoint.Source != whalesMeta.Source || whalesEndpoint.DataTime != whalesMeta.DataTime || whalesEndpoint.Stale != whalesMeta.Stale || whalesEndpoint.StaleReason != whalesMeta.StaleReason {
		t.Fatalf("whales freshness drifted from direct endpoint: endpoint=%+v meta=%+v", whalesEndpoint, whalesMeta)
	}
	qdiiEndpoint := endpointByPath(resp.Endpoints, "/api/etf/premiums")
	if qdiiEndpoint.Source != qdiiMeta.Source || qdiiEndpoint.DataTime != "unknown" || !qdiiEndpoint.Stale || qdiiEndpoint.Status != "unknown" {
		t.Fatalf("QDII freshness drifted from direct endpoint: endpoint=%+v meta=%+v", qdiiEndpoint, qdiiMeta)
	}
	if resp.Status != "degraded" {
		t.Fatalf("stale whales must degrade system status: %+v", resp)
	}
}

func TestSystemStatusFreshnessReflectsCoreChecksOnly(t *testing.T) {
	resp := systemStatusResponse{
		Status: "degraded",
		Endpoints: []systemEndpointStatus{
			{Domain: "行情", Endpoint: "/api/market", Source: "live-tv", DataTime: "2026-09-07T09:30:00Z", Status: "ok"},
			{Domain: "宏观", Endpoint: "/api/macro", Source: "macro-snapshot", DataTime: "2026-09-05T12:00:00Z", Stale: true, StaleReason: "data age exceeded", Status: "stale"},
			{Domain: "Notes", Endpoint: "/api/notes/*", Source: "local-notes", Status: "local-authored"},
		},
		Checks: []systemCheck{{ID: "research.evidence", Status: "warn", Detail: "incomplete evidence"}},
	}
	meta := systemCapabilityFreshness(resp)
	if !meta.Stale || meta.Source != "core-capability-checks" {
		t.Fatalf("capability freshness=%+v", meta)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	setDataFreshness(ctx, meta)
	if recorder.Header().Get("X-Data-Stale") != "true" {
		t.Fatalf("headers=%v", recorder.Header())
	}
}

func TestCollectSystemStatusWarnsOnBrandAndMirror(t *testing.T) {
	root := t.TempDir()
	backendRoot := filepath.Join(root, "backend")
	mustWrite(t, filepath.Join(backendRoot, "data", "a-market.json"), `{"quotes":{"601398":{"price":7.27}},"count":1}`)
	mustWrite(t, filepath.Join(backendRoot, "data", "reports-live.json"), `[{"id":"r1","content":"我不是股神"}]`)
	mustWrite(t, filepath.Join(root, "frontend", "public", "data", "us-stocks.json"), `{"count":1,"stocks":[{"sym":"RKLB","price":64.68}]}`)
	mustWrite(t, filepath.Join(root, "frontend", "public", "data", "us-panel-summary.json"), `{"count":1,"stocks":{"RKLB":{"sc":[1,2,3,4,5]}}}`)

	t.Setenv("STOCKGOD_LIVE_MIRROR", "true")
	resp := collectSystemStatus(backendRoot, &config.Config{Port: "8765", LLMProvider: "deepseek"}, 0)
	if resp.Status != "degraded" {
		t.Fatalf("status=%s checks=%+v", resp.Status, resp.Checks)
	}
	if !hasCheck(resp.Checks, "content.brand", "warn") {
		t.Fatalf("missing brand warning: %+v", resp.Checks)
	}
	if !hasCheck(resp.Checks, "mode.live_mirror", "warn") {
		t.Fatalf("missing live mirror warning: %+v", resp.Checks)
	}
}

func TestCollectSystemStatusDegradesOnStaleEndpoint(t *testing.T) {
	root := t.TempDir()
	backendRoot := filepath.Join(root, "backend")
	mustWrite(t, filepath.Join(backendRoot, "data", "a-market.json"), `{"quotes":{"601398":{"price":7.27}},"ts":946684800000,"count":1}`)
	mustWrite(t, filepath.Join(backendRoot, "data", "reports-live.json"), `[{"id":"r1","content":"我不是神"}]`)
	mustWrite(t, filepath.Join(root, "frontend", "public", "data", "us-stocks.json"), `{"generated_at":"2000-01-01 00:00 ET","count":1,"stocks":[{"sym":"RKLB","price":64.68}]}`)
	mustWrite(t, filepath.Join(root, "frontend", "public", "data", "us-panel-summary.json"), `{"count":1,"stocks":{"RKLB":{"sc":[1,2,3,4,5]}}}`)

	t.Setenv("STOCKGOD_LIVE_MIRROR", "")
	resp := collectSystemStatus(backendRoot, &config.Config{Port: "8765", LLMProvider: "deepseek"}, 0)
	if resp.Status != "degraded" {
		t.Fatalf("status=%s endpoints=%+v checks=%+v", resp.Status, resp.Endpoints, resp.Checks)
	}
	if endpoint := endpointByPath(resp.Endpoints, "/api/stocks?market=us"); endpoint.Status != "stale" {
		t.Fatalf("us endpoint=%+v", endpoint)
	}
}

func TestGPUUnconfiguredDoesNotDegradeCoreCapabilityFreshness(t *testing.T) {
	snapshot := gpupricing.Snapshot{
		Status: gpupricing.StatusUnconfigured,
		Providers: []gpupricing.ProviderStatus{
			{Provider: "runpod", Status: gpupricing.StatusUnconfigured},
			{Provider: "modal", Status: gpupricing.StatusUnconfigured},
		},
	}
	meta := systemCapabilityFreshness(systemStatusResponse{Status: "ok", Endpoints: []systemEndpointStatus{gpuPriceSystemEndpoint(snapshot)}})
	if meta.Stale {
		t.Fatalf("optional GPU endpoint degraded core capability freshness: %+v", meta)
	}
}

func TestSystemStatusStocksUsesRuntimeMarketFreshness(t *testing.T) {
	root := t.TempDir()
	backendRoot := filepath.Join(root, "backend")
	mustWrite(t, filepath.Join(root, "frontend", "public", "data", "us-stocks.json"), `{"generated_at":"2099-01-01T00:00:00Z","count":1,"stocks":[{"sym":"RKLB","price":64.68}]}`)
	files := collectSystemDataFiles(backendRoot)
	runtime := &market.PayloadStatus{Source: "stale-snapshot:us-incomplete", DataTime: time.Date(2026, 9, 4, 20, 0, 0, 0, time.UTC), Stale: true, Market: "us"}

	endpoint := endpointByPath(buildSystemEndpointStatuses(files, runtime, nil, nil), "/api/stocks?market=us")
	if endpoint.Status != "stale" || !endpoint.Stale || endpoint.Source != runtime.Source || endpoint.DataTime != "2026-09-04T20:00:00Z" {
		t.Fatalf("US stocks endpoint ignored runtime quote freshness: %+v", endpoint)
	}
}

func TestSystemStatusETFUsesPromotedCurrentFile(t *testing.T) {
	root := t.TempDir()
	backendRoot := filepath.Join(root, "backend")
	currentPath := filepath.Join(backendRoot, "data", "etf-analyses-current.json")
	mustWrite(t, currentPath, `{"updated":"2026-09-10","metadataAsOf":"2026-09-10","source":"provider+metadata","aumUnit":"usd","n":1,"scope":{"kind":"top-aum","requested":1,"completed":1,"complete":true},"sectors":[{"sector":"test","super":"宽基","n":1,"aum":1}],"etfs":[{"sym":"SPY","name":"SPY","sector":"test","kind":"宽基","aum":1,"expense":0.09,"ret1y":1,"ret5y":2,"mdd":-3}]}`)
	mustWrite(t, filepath.Join(backendRoot, "data", "etf-analyses.json"), `{"updated":"2020-01-02","aumUnit":"usd","n":1,"sectors":[{"sector":"legacy","super":"宽基","n":1,"aum":1}],"etfs":[{"sym":"OLD"}]}`)

	files := collectSystemDataFiles(backendRoot)
	etfFile := dataFileByID(files, "etf-analyses")
	if filepath.Base(etfFile.Path) != "etf-analyses-current.json" || etfFile.GeneratedAt != "2026-09-10" {
		t.Fatalf("system status did not select promoted ETF current file: %+v", etfFile)
	}
	endpoint := endpointByPath(buildSystemEndpointStatuses(files, nil, nil, nil), "/api/etf/sectors")
	if endpoint.DataTime != "2026-09-10" || endpoint.Source != "etf-analyses" {
		t.Fatalf("ETF endpoint did not use promoted current file: %+v", endpoint)
	}

	mustWrite(t, currentPath, `{"updated":"2026-09-10","metadataAsOf":"2026-09-10","source":"provider+metadata","aumUnit":"usd","n":1,"scope":{"kind":"top-aum","requested":1,"completed":1,"complete":true},"sectors":[{"sector":"test","super":"宽基","n":1,"aum":1}],"etfs":[{"sym":"SPYM","name":"SPYM","sector":"test","kind":"宽基","aum":1,"ret1y":1,"ret5y":2,"mdd":-3}]}`)
	invalidFiles := collectSystemDataFiles(backendRoot)
	invalidEndpoint := endpointByPath(buildSystemEndpointStatuses(invalidFiles, nil, nil, nil), "/api/etf/sectors")
	if !invalidEndpoint.Stale || invalidEndpoint.StaleReason == "" {
		t.Fatalf("system status accepted incomplete promoted current file: %+v", invalidEndpoint)
	}
}

func TestSystemStatusPanelAndReportsUseUnderlyingDataTimes(t *testing.T) {
	root := t.TempDir()
	backendRoot := filepath.Join(root, "backend")
	mustWrite(t, filepath.Join(backendRoot, "data", "reports-live.json"), `[{"id":"r1","publishedAt":"2020-01-02T16:00:00-05:00"}]`)
	mustWrite(t, filepath.Join(root, "frontend", "public", "data", "us-stocks.json"), `{"generated_at":"2020-01-02T20:00:00Z","count":1,"stocks":[{"sym":"RKLB","price":10}]}`)
	mustWrite(t, filepath.Join(root, "frontend", "public", "data", "us-panel-summary.json"), `{"generated_at":"2026-09-06T10:00:00Z","count":1,"stocks":{"RKLB":{"sc":[1,2,3,4,5]}}}`)

	files := collectSystemDataFiles(backendRoot)
	endpoints := buildSystemEndpointStatuses(files, nil, nil, nil)
	panel := endpointByPath(endpoints, "/api/panel-summary")
	if panel.DataTime != "2020-01-02T20:00:00Z" || !panel.Stale {
		t.Fatalf("panel endpoint did not inherit oldest input: %+v", panel)
	}
	reports := endpointByPath(endpoints, "/api/reports")
	if reports.DataTime != "2020-01-02T21:00:00Z" || !reports.Stale || reports.Refreshable {
		t.Fatalf("reports endpoint drifted from direct reports freshness: %+v", reports)
	}
}

func TestResearchEvidenceCheckRequiresLinkedEvidence(t *testing.T) {
	resultsDir := t.TempDir()
	cfg := &config.Config{ResultsDir: resultsDir}
	if got := researchEvidenceCheck(cfg); got.Status != "warn" {
		t.Fatalf("missing research results must warn: %+v", got)
	}

	resultPath := filepath.Join(resultsDir, "api_results", "AAPL.json")
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	audit := agents.NewAnalysisAudit(agents.AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock"}, "English", "test", "q", "d", 1, 1, now)
	audit.Evidence = append(audit.Evidence, agents.Evidence{ID: "tool-1", Kind: "tool-invocation", Source: "get_stock_data", DataTime: "2026-09-05", FetchedAt: now.Format(time.RFC3339), MethodVersion: "v3", ContentHash: agents.ContentHash("payload"), PayloadExcerpt: "2026-09-05,100", PayloadRef: "evidence/run/tool-1.payload.gz", PayloadSize: 7, Status: "captured"})
	marketArtifact := audit.RecordStage("market report", "source-bound market report", "q", []string{"tool-1"}, now)
	researchArtifact := audit.RecordStage("research decision", "conditional synthesis", "q", []string{marketArtifact}, now)
	audit.RecordStage("portfolio decision", "conditional observation", "deterministic", []string{researchArtifact}, now)
	published := agents.AnalysisResult{Status: agents.ResearchStatusPublished, Audit: audit, State: agents.AgentState{MarketReport: "source-bound market report", InvestmentPlan: "conditional synthesis", FinalTradeDecision: "conditional observation"}}
	publishedRaw, err := json.Marshal(published)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, resultPath, string(publishedRaw))
	if got := researchEvidenceCheck(cfg); got.Status != "ok" {
		t.Fatalf("complete evidence chain must pass: %+v", got)
	}
	mustWrite(t, filepath.Join(resultsDir, "api_results", "legacy.json"), `{"ticker":"MSFT","trade_date":"2025-01-02","decision":"HOLD"}`)
	if got := researchEvidenceCheck(cfg); got.Status != "ok" || !strings.Contains(got.Detail, "legacy archive=1") || !strings.Contains(got.Detail, "1/1") {
		t.Fatalf("legacy archive must stay visible without invalidating current evidence: %+v", got)
	}

	unavailable := agents.AnalysisResult{
		Status:   agents.ResearchStatusUnavailable,
		Decision: agents.ResearchDecisionUnavailable,
		Audit:    agents.AnalysisAudit{RunID: "run-unavailable", InputHash: "sha256:unavailable"},
	}
	unavailableRaw, err := json.Marshal(unavailable)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(resultsDir, "api_results", "unavailable.json"), string(unavailableRaw))
	if got := researchEvidenceCheck(cfg); got.Status != "ok" || !strings.Contains(got.Detail, "unavailable archive=1") || !strings.Contains(got.Detail, "1/1") {
		t.Fatalf("explicit unavailable result must remain visible without invalidating publishable evidence: %+v", got)
	}

	mustWrite(t, resultPath, `{"audit":{"run_id":"run-1","input_hash":"sha256:abc","inputs":{"ticker":"AAPL"},"evidence":[{"id":"ev-1","source":"get_stock_data","data_time":"unknown","status":"declared"}],"claims":[{"evidence_ids":["missing"]}]}}`)
	if got := researchEvidenceCheck(cfg); got.Status != "warn" || !strings.Contains(got.Detail, "invalid evidence chains") {
		t.Fatalf("unresolved/invalid evidence must warn: %+v", got)
	}

	mustWrite(t, filepath.Join(resultsDir, "api_results", "broken.json"), `{not-json`)
	if got := researchEvidenceCheck(cfg); got.Status != "warn" || !strings.Contains(got.Detail, "0/2") {
		t.Fatalf("malformed persisted result must count as invalid: %+v", got)
	}

	mustWrite(t, resultPath, `{"audit":{"run_id":"run-1","input_hash":"","inputs":{},"evidence":[],"claims":[]}}`)
	if got := researchEvidenceCheck(cfg); got.Status != "warn" {
		t.Fatalf("unlinked research result must warn: %+v", got)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func dataFileByID(files []systemDataFileStatus, id string) systemDataFileStatus {
	for _, file := range files {
		if file.ID == id {
			return file
		}
	}
	return systemDataFileStatus{}
}

func hasCheck(checks []systemCheck, id, status string) bool {
	for _, check := range checks {
		if check.ID == id && check.Status == status {
			return true
		}
	}
	return false
}

func endpointByPath(endpoints []systemEndpointStatus, path string) systemEndpointStatus {
	for _, endpoint := range endpoints {
		if endpoint.Endpoint == path {
			return endpoint
		}
	}
	return systemEndpointStatus{}
}
