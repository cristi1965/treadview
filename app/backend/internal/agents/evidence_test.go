package agents

import (
	"strings"
	"testing"
	"time"
)

func TestAnalysisAuditRecordsStableHashesAndEvidenceLinks(t *testing.T) {
	now := time.Date(2026, 9, 6, 9, 30, 0, 0, time.UTC)
	audit := NewAnalysisAudit(AnalysisRequest{Ticker: "aapl", TradeDate: "2026-09-05", AssetType: "stock"}, "Bilingual", "deepseek", "quick-v1", "deep-v1", 2, 1, now)
	audit.Evidence = append(audit.Evidence, Evidence{
		ID: "tool-1", Kind: "tool-invocation", Source: "get_fundamentals", DataTime: "2025-12-31",
		FetchedAt: now.Format(time.RFC3339), MethodVersion: "tool-v3", ContentHash: ContentHash("payload"),
		PayloadExcerpt: "Fiscal Period: 2025-12-31", PayloadRef: "evidence/run/tool-1.payload.gz", PayloadSize: 7, Status: "captured",
	})
	artifactID := audit.RecordStage("fundamentals report", "revenue grew 10%", "quick-v1", []string{"tool-1"}, now.Add(time.Minute))

	if audit.RunID == "" || audit.InputHash == "" || len(audit.Evidence) != 6 || len(audit.Claims) != 1 {
		t.Fatalf("incomplete audit: %+v", audit)
	}
	claim := audit.Claims[0]
	if claim.ContentHash != ContentHash("revenue grew 10%") {
		t.Fatalf("claim hash mismatch: %s", claim.ContentHash)
	}
	if artifactID == "" || len(claim.EvidenceIDs) != 1 || claim.EvidenceIDs[0] != "tool-1" {
		t.Fatalf("claim evidence links missing: %+v", claim.EvidenceIDs)
	}
	if claim.DataTime != "2025-12-31" || claim.Status != "captured" {
		t.Fatalf("claim did not inherit upstream health: %+v", claim)
	}
	if audit.Evidence[1].DataTime != "unknown" {
		t.Fatalf("unverified source time must be explicit unknown: %+v", audit.Evidence[1])
	}
}

func TestAuditPublicationRequiresReviewableCapturedSourceAndRejectsCycles(t *testing.T) {
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	audit := NewAnalysisAudit(AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock"}, "English", "test", "q", "d", 1, 1, now)
	audit.Evidence = append(audit.Evidence,
		Evidence{ID: "failed", Kind: "tool-invocation", Source: "get_news", DataTime: "unknown", FetchedAt: now.Format(time.RFC3339), MethodVersion: "v3", ContentHash: ContentHash("unavailable"), PayloadExcerpt: "unavailable", Status: "unavailable"},
		Evidence{ID: "market", Kind: "tool-invocation", Source: "get_stock_data", DataTime: "2026-09-05", FetchedAt: now.Format(time.RFC3339), MethodVersion: "v3", ContentHash: ContentHash("bars"), PayloadExcerpt: "2026-09-05,100", PayloadRef: "evidence/run/market.payload.gz", PayloadSize: 4, Status: "captured"},
	)
	upstream := audit.RecordStage("market report", "report", "q", []string{"market"}, now.Add(time.Minute))
	audit.RecordStage("portfolio decision", "conditional observation", "d", []string{upstream}, now.Add(2*time.Minute))
	health := AssessAuditForPublication(audit)
	if health.Publishable || health.StructureStatus != "complete" || health.SourceStatus != "unavailable" || health.DataTime != "2026-09-05" {
		t.Fatalf("failed attempt must remain auditable and make source health unavailable: %+v", health)
	}
	result := &AnalysisResult{Audit: audit}
	MarkResearchUnavailable(result, "preflight failed")
	if result.ResearchHealth.SourceStatus != "unavailable" {
		t.Fatalf("unavailable source status was weakened: %+v", result.ResearchHealth)
	}

	audit.Claims[len(audit.Claims)-1].EvidenceIDs = []string{audit.Claims[len(audit.Claims)-1].ArtifactID}
	health = AssessAuditForPublication(audit)
	if health.Publishable || !strings.Contains(strings.Join(health.Reasons, " "), "cycle") {
		t.Fatalf("self-proving generated artifact must fail closed: %+v", health)
	}
}

func TestAuditReportsLineageFailureEvenWhenClaimStructureIsInvalid(t *testing.T) {
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	audit := NewAnalysisAudit(AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock"}, "English", "test", "q", "d", 1, 1, now)
	audit.Evidence = append(audit.Evidence, Evidence{
		ID: "legacy-aapl", Kind: "tool-invocation", Source: "get_stock_data", DataTime: "unknown",
		FetchedAt: now.Format(time.RFC3339), MethodVersion: "legacy", ContentHash: ContentHash("payload"),
		PayloadExcerpt: "payload", Status: "captured",
	})
	audit.Claims = append(audit.Claims, Claim{
		ID: "claim-legacy", Stage: "portfolio decision", ContentHash: ContentHash("observe"),
		EvidenceIDs: []string{"legacy-aapl"}, Status: "captured", DataTime: "unknown",
	})

	health := AssessAuditForPublication(audit)
	if health.StructureStatus != "invalid" || health.SourceStatus != "degraded" || health.Publishable {
		t.Fatalf("structure and source failures must both be reported: %+v", health)
	}
	reasons := strings.Join(health.Reasons, " ")
	if !strings.Contains(reasons, "incomplete claim metadata") || !strings.Contains(reasons, "unhealthy source evidence") {
		t.Fatalf("expected both structural and lineage reasons: %+v", health.Reasons)
	}
}

func TestApplyPublicationGateSanitizesExecutableAdvice(t *testing.T) {
	result := &AnalysisResult{
		Ticker: "CRWV", TradeDate: "2026-08-14", Decision: "HOLD",
		State: AgentState{InvestmentPlan: "sell 50%", TraderInvestmentPlan: "stop loss 99", FinalTradeDecision: "buy calls"},
	}
	health := ApplyPublicationGate(result)
	if health.Publishable || result.Status != ResearchStatusUnavailable || result.Decision != ResearchDecisionUnavailable {
		t.Fatalf("unhealthy result was not blocked: %+v", result)
	}
	raw := result.State.InvestmentPlan + result.State.TraderInvestmentPlan + result.State.FinalTradeDecision
	if raw != "" {
		t.Fatalf("executable text survived gate: %q", raw)
	}
}

func TestRecordStageIgnoresEmptyArtifacts(t *testing.T) {
	audit := NewAnalysisAudit(AnalysisRequest{Ticker: "AAPL", TradeDate: "2026-09-05", AssetType: "stock"}, "English", "test", "q", "d", 1, 1, time.Now())
	if id := audit.RecordStage("empty", "  ", "q", nil, time.Now()); id != "" || len(audit.Claims) != 0 {
		t.Fatal("empty output must not create a claim")
	}
}
