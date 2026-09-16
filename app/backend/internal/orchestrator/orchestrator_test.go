package orchestrator

import (
	"context"
	"errors"
	"testing"

	"trading-agents/internal/llm"
)

type routeSuccessProbe struct{}

func (routeSuccessProbe) Generate(context.Context, string, string, bool) (string, error) {
	return "fallback output", nil
}
func (routeSuccessProbe) GenerateWithTools(context.Context, string, []llm.ChatMessage, []llm.ToolDef, bool, func(string, map[string]any) (string, error), func(string)) (string, error) {
	return "fallback output", nil
}
func (routeSuccessProbe) StructuredGenerate(context.Context, string, string, bool, any) error {
	return nil
}

type routeFailureProbe struct{}

func (routeFailureProbe) Generate(context.Context, string, string, bool) (string, error) {
	return "", errors.New("Gemini API error: Error 503, Status: UNAVAILABLE")
}
func (routeFailureProbe) GenerateWithTools(context.Context, string, []llm.ChatMessage, []llm.ToolDef, bool, func(string, map[string]any) (string, error), func(string)) (string, error) {
	return "", errors.New("Gemini API error: Error 503, Status: UNAVAILABLE")
}
func (routeFailureProbe) StructuredGenerate(context.Context, string, string, bool, any) error {
	return errors.New("Gemini API error: Error 503, Status: UNAVAILABLE")
}

func TestStageModelUsesActualFallbackProvider(t *testing.T) {
	trace := llm.NewRouteTrace()
	ctx := llm.WithRouteTrace(context.Background(), trace)
	client := llm.NewNamedRoutedClient(routeSuccessProbe{}, routeFailureProbe{}, "deepseek/quick", "gemini/deep")
	if _, err := client.Generate(ctx, "", "", true); err != nil {
		t.Fatal(err)
	}
	if got := stageModel("gemini/deep", trace); got != "deepseek/quick" {
		t.Fatalf("stage model=%q want actual fallback provider", got)
	}
}
