package llm

import "context"

// LLMClient is the common interface for all LLM providers (Gemini, DeepSeek, etc.)
type LLMClient interface {
	// GenerateWithTools sends a prompt with optional tool definitions.
	// It handles the tool-calling loop automatically.
	GenerateWithTools(
		ctx context.Context,
		systemPrompt string,
		messages []ChatMessage,
		tools []ToolDef,
		useDeep bool,
		toolExecutor func(name string, args map[string]any) (string, error),
		onStream func(text string),
	) (string, error)

	// Generate sends a simple prompt without tools.
	Generate(ctx context.Context, systemPrompt, userPrompt string, useDeep bool) (string, error)

	// StructuredGenerate sends a prompt and parses the JSON response into target.
	StructuredGenerate(ctx context.Context, systemPrompt, userPrompt string, useDeep bool, target any) error
}
