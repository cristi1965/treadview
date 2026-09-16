package llm

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
)

// RoutedClient assigns quick research roles to DeepSeek and deep synthesis to Gemini.
type RoutedClient struct {
	quick     LLMClient
	deep      LLMClient
	quickName string
	deepName  string
}

func NewRoutedClient(quick, deep LLMClient) *RoutedClient {
	return NewNamedRoutedClient(quick, deep, "quick", "deep")
}

func NewNamedRoutedClient(quick, deep LLMClient, quickName, deepName string) *RoutedClient {
	return &RoutedClient{quick: quick, deep: deep, quickName: quickName, deepName: deepName}
}

type RouteEvent struct {
	Provider      string
	RequestedDeep bool
	Fallback      bool
}

type RouteTrace struct {
	mu     sync.Mutex
	events []RouteEvent
}

type routeTraceContextKey struct{}

func NewRouteTrace() *RouteTrace { return &RouteTrace{} }

func WithRouteTrace(ctx context.Context, trace *RouteTrace) context.Context {
	return context.WithValue(ctx, routeTraceContextKey{}, trace)
}

func (t *RouteTrace) record(event RouteEvent) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.events = append(t.events, event)
	t.mu.Unlock()
}

func (t *RouteTrace) LastSuccessful() (RouteEvent, bool) {
	if t == nil {
		return RouteEvent{}, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.events) == 0 {
		return RouteEvent{}, false
	}
	return t.events[len(t.events)-1], true
}

func (c *RoutedClient) record(ctx context.Context, useDeep, fallback bool) {
	trace, _ := ctx.Value(routeTraceContextKey{}).(*RouteTrace)
	provider := c.quickName
	if useDeep && !fallback {
		provider = c.deepName
	}
	trace.record(RouteEvent{Provider: provider, RequestedDeep: useDeep, Fallback: fallback})
}

func (c *RoutedClient) selected(useDeep bool) LLMClient {
	if useDeep {
		return c.deep
	}
	return c.quick
}

// ProbeSelectedProvider tests the configured provider directly. Health/config
// probes must not turn a failed deep provider into a false success via fallback.
func ProbeSelectedProvider(ctx context.Context, client LLMClient, systemPrompt, userPrompt string, useDeep bool) (string, error) {
	if routed, ok := client.(*RoutedClient); ok {
		return routed.selected(useDeep).Generate(ctx, systemPrompt, userPrompt, useDeep)
	}
	return client.Generate(ctx, systemPrompt, userPrompt, useDeep)
}

func (c *RoutedClient) GenerateWithTools(ctx context.Context, systemPrompt string, messages []ChatMessage, tools []ToolDef, useDeep bool, toolExecutor func(string, map[string]any) (string, error), onStream func(string)) (string, error) {
	result, err := c.selected(useDeep).GenerateWithTools(ctx, systemPrompt, messages, tools, useDeep, toolExecutor, onStream)
	if useDeep && isTransientProviderError(err) {
		log.Printf("[LLM] Deep provider temporarily unavailable; falling back to quick provider: %v", err)
		result, err = c.quick.GenerateWithTools(ctx, systemPrompt, messages, tools, true, toolExecutor, onStream)
		if err == nil {
			c.record(ctx, useDeep, true)
		}
		return result, err
	}
	if err == nil {
		c.record(ctx, useDeep, false)
	}
	return result, err
}

func (c *RoutedClient) Generate(ctx context.Context, systemPrompt, userPrompt string, useDeep bool) (string, error) {
	result, err := c.selected(useDeep).Generate(ctx, systemPrompt, userPrompt, useDeep)
	if useDeep && isTransientProviderError(err) {
		log.Printf("[LLM] Deep provider temporarily unavailable; falling back to quick provider: %v", err)
		result, err = c.quick.Generate(ctx, systemPrompt, userPrompt, true)
		if err == nil {
			c.record(ctx, useDeep, true)
		}
		return result, err
	}
	if err == nil {
		c.record(ctx, useDeep, false)
	}
	return result, err
}

func (c *RoutedClient) StructuredGenerate(ctx context.Context, systemPrompt, userPrompt string, useDeep bool, target any) error {
	err := c.selected(useDeep).StructuredGenerate(ctx, systemPrompt, userPrompt, useDeep, target)
	if useDeep && isTransientProviderError(err) {
		log.Printf("[LLM] Deep provider temporarily unavailable; falling back to quick provider: %v", err)
		err = c.quick.StructuredGenerate(ctx, systemPrompt, userPrompt, true, target)
		if err == nil {
			c.record(ctx, useDeep, true)
		}
		return err
	}
	if err == nil {
		c.record(ctx, useDeep, false)
	}
	return err
}

func isTransientProviderError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"error 429", "status 429", "api 429", "too many requests", "resource_exhausted",
		"error 500", "status 500", "api 500",
		"error 502", "status 502", "api 502", "bad gateway",
		"error 503", "status 503", "api 503", "unavailable", "high demand",
		"error 504", "status 504", "api 504", "gateway timeout",
		"tls handshake timeout", "i/o timeout", "client.timeout exceeded",
		"connection reset", "connection refused", "unexpected eof",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
