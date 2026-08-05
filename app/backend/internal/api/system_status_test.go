package api

import (
	"os"
	"path/filepath"
	"testing"

	"trading-agents/internal/config"
)

func TestCollectSystemStatus(t *testing.T) {
	root := t.TempDir()
	backendRoot := filepath.Join(root, "backend")
	mustWrite(t, filepath.Join(backendRoot, "data", "a-market.json"), `{"quotes":{"601398":{"price":7.27,"pct":0.5,"vol":1000,"mcapYi":20000}},"ts":1783590554081,"count":1}`)
	mustWrite(t, filepath.Join(backendRoot, "data", "reports-live.json"), `[{"id":"r1","content":"我不是神"}]`)
	mustWrite(t, filepath.Join(root, "frontend", "public", "data", "us-stocks.json"), `{"generated_at":"2026-07-31 03:31 ET","count":1,"stocks":[{"sym":"RKLB","price":64.68,"pct":10.38,"mcapB":38.69,"vol":18162886}]}`)
	mustWrite(t, filepath.Join(root, "frontend", "public", "data", "us-panel-summary.json"), `{"count":1,"stocks":{"RKLB":{"sc":[55,60,60,75,45],"div":30}}}`)
	mustWrite(t, filepath.Join(root, "frontend", "public", "data", "reports.json"), `[{"id":"r1","content":"我不是神"}]`)

	t.Setenv("STOCKGOD_LIVE_MIRROR", "")
	resp := collectSystemStatus(backendRoot, &config.Config{
		Port:          "8765",
		LLMProvider:   "deepseek",
		DeepThinkLLM:  "deepseek-chat",
		QuickThinkLLM: "deepseek-chat",
	}, 2)
	if resp.Status != "ok" {
		t.Fatalf("status=%s checks=%+v", resp.Status, resp.Checks)
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
