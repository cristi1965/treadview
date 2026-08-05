package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// DeepSeekClient implements LLMClient via the OpenAI-compatible API.
// Works with DeepSeek, OpenAI, and any OpenAI-compatible endpoint.
type DeepSeekClient struct {
	apiKey      string
	baseURL     string
	httpClient  *http.Client
	deepModel   string
	quickModel  string
	temperature *float32
}

// NewDeepSeekClient creates a DeepSeek (OpenAI-compatible) client.
// apiKey: DeepSeek API key
// baseURL: e.g. "https://api.deepseek.com/v1"
// deepModel: model for complex reasoning (e.g. "deepseek-chat")
// quickModel: model for quick tasks (e.g. "deepseek-chat")
func NewDeepSeekClient(apiKey, baseURL, deepModel, quickModel string, temperature *float32) *DeepSeekClient {
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}
	if deepModel == "" {
		deepModel = "deepseek-chat"
	}
	if quickModel == "" {
		quickModel = "deepseek-chat"
	}

	return &DeepSeekClient{
		apiKey:      apiKey,
		baseURL:     strings.TrimRight(baseURL, "/"),
		deepModel:   deepModel,
		quickModel:  quickModel,
		temperature: temperature,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// ---- OpenAI-compatible DTOs (only what we need) ----

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAIToolCall struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"`
	Function openAIFunctionCall  `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIToolDef struct {
	Type     string          `json:"type"`
	Function json.RawMessage `json:"function"`
}

type openAIFunctionDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  openAISchema `json:"parameters"`
}

type openAISchema struct {
	Type       string                 `json:"type"`
	Properties map[string]any         `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
	Items      any                    `json:"items,omitempty"`
}

type openAIChatRequest struct {
	Model       string            `json:"model"`
	Messages    []openAIMessage   `json:"messages"`
	Tools       []openAIToolDef   `json:"tools,omitempty"`
	Temperature *float32          `json:"temperature,omitempty"`
	Stream      bool              `json:"stream,omitempty"`
}

type openAIChatResponse struct {
	Choices []openAIChoice `json:"choices"`
	Usage   *openAIUsage   `json:"usage,omitempty"`
}

type openAIChoice struct {
	Index   int             `json:"index"`
	Message openAIMessage   `json:"message"`
}


type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ---- LLMClient implementation ----

// GenerateWithTools implements LLMClient.
func (c *DeepSeekClient) GenerateWithTools(
	ctx context.Context,
	systemPrompt string,
	messages []ChatMessage,
	tools []ToolDef,
	useDeep bool,
	toolExecutor func(name string, args map[string]any) (string, error),
	onStream func(text string),
) (string, error) {
	model := c.quickModel
	if useDeep {
		model = c.deepModel
	}

	// Convert messages
	openAIMsgs := []openAIMessage{
		{Role: "system", Content: systemPrompt},
	}
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			openAIMsgs = append(openAIMsgs, openAIMessage{Role: "user", Content: msg.Content})
		case "model":
			m := openAIMessage{Role: "assistant", Content: msg.Content}
			if len(msg.ToolCalls) > 0 {
				m.ToolCalls = make([]openAIToolCall, len(msg.ToolCalls))
				for j, tc := range msg.ToolCalls {
					argsJSON, _ := json.Marshal(tc.Args)
					m.ToolCalls[j] = openAIToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: openAIFunctionCall{
							Name:      tc.Name,
							Arguments: string(argsJSON),
						},
					}
				}
			}
			openAIMsgs = append(openAIMsgs, m)
		case "tool":
			openAIMsgs = append(openAIMsgs, openAIMessage{
				Role:       "tool",
				Content:    msg.Content,
				ToolCallID: msg.ToolName,
			})
		}
	}

	// Convert tools
	var openAITools []openAIToolDef
	if len(tools) > 0 {
		for _, t := range tools {
			fd := openAIFunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  buildOpenAISchema(t),
			}
			fdJSON, _ := json.Marshal(fd)
			openAITools = append(openAITools, openAIToolDef{
				Type:     "function",
				Function: fdJSON,
			})
		}
	}

	// Tool-calling loop (max 10 iterations)
	for i := 0; i < 10; i++ {
		req := openAIChatRequest{
			Model:       model,
			Messages:    openAIMsgs,
			Temperature: c.temperature,
		}
		if len(openAITools) > 0 {
			req.Tools = openAITools
		}

		resp, err := c.doChatRequest(ctx, &req)
		if err != nil {
			return "", fmt.Errorf("DeepSeek API error: %w", err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no choices in response")
		}

		choice := resp.Choices[0]
		msg := choice.Message
		content := msg.Content

		// Check for tool calls
		if len(msg.ToolCalls) > 0 {
			// Add assistant message
			openAIMsgs = append(openAIMsgs, msg)

			for _, tc := range msg.ToolCalls {
				if tc.Type != "function" {
					continue
				}
				if toolExecutor == nil {
					return "", fmt.Errorf("model requested tool %q but no executor provided", tc.Function.Name)
				}

				var args map[string]any
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					args = map[string]any{}
				}

				log.Printf("[LLM/DeepSeek] Tool call: %s(%v)", tc.Function.Name, args)

				result, execErr := toolExecutor(tc.Function.Name, args)
				if execErr != nil {
					result = fmt.Sprintf("Error: %v", execErr)
				}
				if len(result) > 30000 {
					result = result[:30000] + "\n... [truncated]"
				}

				openAIMsgs = append(openAIMsgs, openAIMessage{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
				})
			}
			continue
		}

		// No tool calls — return content
		result := content
		if onStream != nil {
			onStream(result)
		}
		return result, nil
	}

	return "", fmt.Errorf("tool-calling loop exceeded max iterations")
}

// Generate implements LLMClient.
func (c *DeepSeekClient) Generate(ctx context.Context, systemPrompt, userPrompt string, useDeep bool) (string, error) {
	msgs := []ChatMessage{{Role: "user", Content: userPrompt}}
	return c.GenerateWithTools(ctx, systemPrompt, msgs, nil, useDeep, nil, nil)
}

// StructuredGenerate implements LLMClient.
func (c *DeepSeekClient) StructuredGenerate(ctx context.Context, systemPrompt, userPrompt string, useDeep bool, target any) error {
	// Use structured output via response_format: { type: "json_object" }
	model := c.quickModel
	if useDeep {
		model = c.deepModel
	}

	body := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"response_format": map[string]string{"type": "json_object"},
	}
	if c.temperature != nil {
		body["temperature"] = *c.temperature
	}

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/chat/completions",
		bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("DeepSeek API error: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DeepSeek API %d: %s", resp.StatusCode, string(raw))
	}

	var chatResp openAIChatResponse
	if err := json.Unmarshal(raw, &chatResp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return fmt.Errorf("no choices in response")
	}

	return json.Unmarshal([]byte(chatResp.Choices[0].Message.Content), target)
}

// ---- helpers ----

func (c *DeepSeekClient) doChatRequest(ctx context.Context, reqData *openAIChatRequest) (*openAIChatResponse, error) {
	jsonBody, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/chat/completions",
		bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP error: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API %d: %s", resp.StatusCode, string(raw))
	}

	var chatResp openAIChatResponse
	if err := json.Unmarshal(raw, &chatResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if chatResp.Usage != nil {
		log.Printf("[LLM/DeepSeek] Tokens: %d prompt + %d completion = %d total",
			chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens, chatResp.Usage.TotalTokens)
	}

	return &chatResp, nil
}

func (c *DeepSeekClient) setAuthHeader(req *http.Request) {
	if strings.HasPrefix(c.apiKey, "Bearer ") {
		req.Header.Set("Authorization", c.apiKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

func buildOpenAISchema(t ToolDef) openAISchema {
	s := openAISchema{
		Type:       "object",
		Properties: make(map[string]any),
		Required:   t.Required,
	}
	for name, param := range t.Parameters {
		s.Properties[name] = schemaParamToOpenAPI(param)
	}
	return s
}

func schemaParamToOpenAPI(p *SchemaParam) map[string]any {
	m := map[string]any{
		"type":        p.Type,
		"description": p.Description,
	}
	switch p.Type {
	case "object":
		if p.Properties != nil {
			props := make(map[string]any)
			for k, v := range p.Properties {
				props[k] = schemaParamToOpenAPI(v)
			}
			m["properties"] = props
		}
		if len(p.Required) > 0 {
			m["required"] = p.Required
		}
	case "array":
		if p.Items != nil {
			m["items"] = schemaParamToOpenAPI(p.Items)
		}
	}
	if len(p.Enum) > 0 {
		m["enum"] = p.Enum
	}
	return m
}
