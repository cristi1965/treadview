package llm

import (
	"context"
	"fmt"
)

type unavailableClient struct {
	reason string
}

func NewUnavailableClient(reason string) LLMClient {
	return &unavailableClient{reason: reason}
}

func (c *unavailableClient) GenerateWithTools(
	ctx context.Context,
	systemPrompt string,
	messages []ChatMessage,
	tools []ToolDef,
	useDeep bool,
	toolExecutor func(name string, args map[string]any) (string, error),
	onStream func(text string),
) (string, error) {
	return "", c.err()
}

func (c *unavailableClient) Generate(ctx context.Context, systemPrompt, userPrompt string, useDeep bool) (string, error) {
	return "", c.err()
}

func (c *unavailableClient) StructuredGenerate(ctx context.Context, systemPrompt, userPrompt string, useDeep bool, target any) error {
	return c.err()
}

func (c *unavailableClient) err() error {
	if c.reason == "" {
		return fmt.Errorf("LLM client unavailable")
	}
	return fmt.Errorf("LLM client unavailable: %s", c.reason)
}
