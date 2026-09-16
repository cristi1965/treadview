package agents

import (
	"context"
	"testing"

	"trading-agents/internal/llm"
)

type researchManagerProbe struct {
	responses []string
	useDeep   []bool
}

func (p *researchManagerProbe) Generate(_ context.Context, _, _ string, useDeep bool) (string, error) {
	p.useDeep = append(p.useDeep, useDeep)
	response := p.responses[0]
	p.responses = p.responses[1:]
	return response, nil
}

func (p *researchManagerProbe) GenerateWithTools(context.Context, string, []llm.ChatMessage, []llm.ToolDef, bool, func(string, map[string]any) (string, error), func(string)) (string, error) {
	panic("unexpected GenerateWithTools call")
}

func (p *researchManagerProbe) StructuredGenerate(context.Context, string, string, bool, any) error {
	panic("unexpected StructuredGenerate call")
}

func TestResearchManagerRepairsUnsafeSynthesisWithQuickProvider(t *testing.T) {
	client := &researchManagerProbe{responses: []string{
		"The bull case supports a buy while the bear case recommends a stop loss.",
		"1. Research Status: Supported\n2. Evidence Rationale: Favorable evidence is balanced by valuation uncertainty.\n3. Monitoring Conditions: Reassess when new filings arrive. Conditional observation only.",
	}}

	got, err := ResearchManager(context.Background(), client, &AgentState{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !ConditionalObservationSafe(got) {
		t.Fatalf("repaired synthesis remains unsafe: %q", got)
	}
	if len(client.useDeep) != 2 || !client.useDeep[0] || client.useDeep[1] {
		t.Fatalf("provider routing=%v want=[true false]", client.useDeep)
	}
}

func TestResearchManagerKeepsSafeSynthesisWithoutRewrite(t *testing.T) {
	client := &researchManagerProbe{responses: []string{"Supported evidence with monitoring conditions. Conditional observation only."}}

	got, err := ResearchManager(context.Background(), client, &AgentState{}, nil)
	if err != nil || got == "" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if len(client.useDeep) != 1 || !client.useDeep[0] {
		t.Fatalf("provider routing=%v want=[true]", client.useDeep)
	}
}

func TestResearchManagerUsesDeterministicObservationWhenRewriteRemainsUnsafe(t *testing.T) {
	client := &researchManagerProbe{responses: []string{
		"Buy after a pullback.",
		"Do not buy; hold the existing position.",
	}}

	got, err := ResearchManager(context.Background(), client, &AgentState{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !ConditionalObservationSafe(got) {
		t.Fatalf("deterministic fallback is unsafe: %q", got)
	}
	if len(client.useDeep) != 2 || !client.useDeep[0] || client.useDeep[1] {
		t.Fatalf("provider routing=%v want=[true false]", client.useDeep)
	}
}
