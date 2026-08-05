package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"google.golang.org/genai"

	"trading-agents/internal/config"
)

// GeminiClient wraps the Google Generative AI SDK. Implements LLMClient.
type GeminiClient struct {
	client        *genai.Client
	deepModel     string
	quickModel    string
	thinkingLevel string
	temperature   *float32
}

// NewGeminiClient creates a new Gemini client.
func NewGeminiClient(cfg *config.Config) (*GeminiClient, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: cfg.GoogleAPIKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	gc := &GeminiClient{
		client:        client,
		deepModel:     cfg.DeepThinkLLM,
		quickModel:    cfg.QuickThinkLLM,
		thinkingLevel: cfg.ThinkingLevel,
	}

	if cfg.HasTemperature {
		t := float32(cfg.Temperature)
		gc.temperature = &t
	}

	return gc, nil
}

// GenerateWithTools implements LLMClient.
func (gc *GeminiClient) GenerateWithTools(
	ctx context.Context,
	systemPrompt string,
	messages []ChatMessage,
	tools []ToolDef,
	useDeep bool,
	toolExecutor func(name string, args map[string]any) (string, error),
	onStream func(text string),
) (string, error) {
	modelName := gc.quickModel
	if useDeep {
		modelName = gc.deepModel
	}

	var genaiTools []*genai.Tool
	if len(tools) > 0 {
		var funcDecls []*genai.FunctionDeclaration
		for _, t := range tools {
			props := make(map[string]*genai.Schema)
			for k, v := range t.Parameters {
				props[k] = schemaParamToGenAI(v)
			}
			fd := &genai.FunctionDeclaration{
				Name:        t.Name,
				Description: t.Description,
				Parameters: &genai.Schema{
					Type:       genai.TypeObject,
					Properties: props,
					Required:   t.Required,
				},
			}
			funcDecls = append(funcDecls, fd)
		}
		genaiTools = []*genai.Tool{{FunctionDeclarations: funcDecls}}
	}

	genConfig := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemPrompt, genai.RoleUser),
		Tools:             genaiTools,
	}
	if gc.temperature != nil {
		genConfig.Temperature = genai.Ptr(float32(*gc.temperature))
	}

	var contents []*genai.Content
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			contents = append(contents, genai.NewContentFromText(msg.Content, genai.RoleUser))
		case "model":
			contents = append(contents, genai.NewContentFromText(msg.Content, genai.RoleModel))
		case "tool":
			contents = append(contents, &genai.Content{
				Role: genai.RoleUser,
				Parts: []*genai.Part{
					genai.NewPartFromFunctionResponse(msg.ToolName, map[string]any{
						"result": msg.Content,
					}),
				},
			})
		}
	}

	for i := 0; i < 10; i++ {
		resp, err := gc.client.Models.GenerateContent(ctx, modelName, contents, genConfig)
		if err != nil {
			return "", fmt.Errorf("Gemini API error: %w", err)
		}

		if len(resp.Candidates) == 0 {
			return "", fmt.Errorf("no candidates in Gemini response")
		}

		candidate := resp.Candidates[0]

		var functionCalls []*genai.FunctionCall
		var textParts []string

		for _, part := range candidate.Content.Parts {
			if part.FunctionCall != nil {
				functionCalls = append(functionCalls, part.FunctionCall)
			}
			if part.Text != "" {
				textParts = append(textParts, part.Text)
			}
		}

		if len(functionCalls) == 0 {
			result := strings.Join(textParts, "\n")
			if onStream != nil {
				onStream(result)
			}
			return result, nil
		}

		contents = append(contents, candidate.Content)

		for _, fc := range functionCalls {
			if toolExecutor == nil {
				return "", fmt.Errorf("model requested tool %q but no executor provided", fc.Name)
			}

			log.Printf("[LLM/Gemini] Tool call: %s(%v)", fc.Name, fc.Args)

			result, execErr := toolExecutor(fc.Name, fc.Args)
			if execErr != nil {
				result = fmt.Sprintf("Error: %v", execErr)
			}
			if len(result) > 30000 {
				result = result[:30000] + "\n... [truncated]"
			}

			contents = append(contents, &genai.Content{
				Role: genai.RoleUser,
				Parts: []*genai.Part{
					genai.NewPartFromFunctionResponse(fc.Name, map[string]any{
						"result": result,
					}),
				},
			})
		}
	}

	return "", fmt.Errorf("tool-calling loop exceeded max iterations")
}

// Generate implements LLMClient.
func (gc *GeminiClient) Generate(ctx context.Context, systemPrompt, userPrompt string, useDeep bool) (string, error) {
	msgs := []ChatMessage{{Role: "user", Content: userPrompt}}
	return gc.GenerateWithTools(ctx, systemPrompt, msgs, nil, useDeep, nil, nil)
}

// StructuredGenerate implements LLMClient.
func (gc *GeminiClient) StructuredGenerate(ctx context.Context, systemPrompt, userPrompt string, useDeep bool, target any) error {
	result, err := gc.Generate(ctx, systemPrompt, userPrompt, useDeep)
	if err != nil {
		return err
	}
	jsonStr := extractJSON(result)
	return json.Unmarshal([]byte(jsonStr), target)
}

// schemaParamToGenAI converts our generic SchemaParam to genai.Schema.
func schemaParamToGenAI(p *SchemaParam) *genai.Schema {
	if p == nil {
		return nil
	}
	s := &genai.Schema{
		Description: p.Description,
	}
	switch strings.ToLower(p.Type) {
	case "string":
		s.Type = genai.TypeString
	case "number":
		s.Type = genai.TypeNumber
	case "integer":
		s.Type = genai.TypeInteger
	case "boolean":
		s.Type = genai.TypeBoolean
	case "array":
		s.Type = genai.TypeArray
		s.Items = schemaParamToGenAI(p.Items)
	case "object":
		s.Type = genai.TypeObject
		s.Properties = make(map[string]*genai.Schema)
		for k, v := range p.Properties {
			s.Properties[k] = schemaParamToGenAI(v)
		}
		s.Required = p.Required
	default:
		s.Type = genai.TypeString
	}
	return s
}

// extractJSON tries to pull a JSON block from model output.
func extractJSON(s string) string {
	if idx := strings.Index(s, "```json"); idx >= 0 {
		start := idx + 7
		if end := strings.Index(s[start:], "```"); end >= 0 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	if idx := strings.Index(s, "```"); idx >= 0 {
		start := idx + 3
		if nl := strings.Index(s[start:], "\n"); nl >= 0 {
			start += nl + 1
		}
		if end := strings.Index(s[start:], "```"); end >= 0 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	s = strings.TrimSpace(s)
	if (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) {
		return s
	}
	return s
}
