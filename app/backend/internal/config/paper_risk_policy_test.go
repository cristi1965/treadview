package config

import "testing"

func TestLoadPaperRiskPolicyFromEnvironment(t *testing.T) {
	t.Setenv("STOCKGOD_PAPER_RISK_POLICY_VERSION", "desk-policy-2026-09")
	t.Setenv("STOCKGOD_PAPER_MIN_STOP_COVERAGE_PCT", "95")
	t.Setenv("STOCKGOD_PAPER_MAX_DAILY_LOSS_PCT", "2.5")
	t.Setenv("STOCKGOD_PAPER_MAX_DRAWDOWN_PCT", "9")
	t.Setenv("STOCKGOD_PAPER_MAX_STRESS_LOSS_PCT", "7.5")

	cfg := Load()
	if cfg.PaperRiskPolicyVersion != "desk-policy-2026-09" ||
		cfg.PaperMinStopCoveragePct != 95 ||
		cfg.PaperMaxDailyLossPct != 2.5 ||
		cfg.PaperMaxDrawdownPct != 9 ||
		cfg.PaperMaxStressLossPct != 7.5 {
		t.Fatalf("paper risk policy was not loaded from environment: %+v", cfg)
	}
}

func TestLoadPaperRiskPolicyRejectsOutOfRangeValues(t *testing.T) {
	t.Setenv("STOCKGOD_PAPER_MIN_STOP_COVERAGE_PCT", "101")
	t.Setenv("STOCKGOD_PAPER_MAX_DAILY_LOSS_PCT", "0")
	t.Setenv("STOCKGOD_PAPER_MAX_DRAWDOWN_PCT", "-1")
	t.Setenv("STOCKGOD_PAPER_MAX_STRESS_LOSS_PCT", "not-a-number")

	cfg := Load()
	if cfg.PaperMinStopCoveragePct != 100 || cfg.PaperMaxDailyLossPct != 3 ||
		cfg.PaperMaxDrawdownPct != 12 || cfg.PaperMaxStressLossPct != 10 {
		t.Fatalf("invalid paper risk policy did not use defaults: %+v", cfg)
	}
}
