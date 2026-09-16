package llm

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type routingCall struct {
	method  string
	useDeep bool
}

type routingProbe struct{ calls []routingCall }

func (p *routingProbe) Generate(_ context.Context, _, _ string, useDeep bool) (string, error) {
	p.calls = append(p.calls, routingCall{"generate", useDeep})
	return "ok", nil
}

type failingRoutingProbe struct{ err error }

func (p *failingRoutingProbe) Generate(context.Context, string, string, bool) (string, error) {
	return "", p.err
}
func (p *failingRoutingProbe) GenerateWithTools(context.Context, string, []ChatMessage, []ToolDef, bool, func(string, map[string]any) (string, error), func(string)) (string, error) {
	return "", p.err
}
func (p *failingRoutingProbe) StructuredGenerate(context.Context, string, string, bool, any) error {
	return p.err
}
func (p *routingProbe) GenerateWithTools(_ context.Context, _ string, _ []ChatMessage, _ []ToolDef, useDeep bool, _ func(string, map[string]any) (string, error), _ func(string)) (string, error) {
	p.calls = append(p.calls, routingCall{"tools", useDeep})
	return "ok", nil
}
func (p *routingProbe) StructuredGenerate(_ context.Context, _, _ string, useDeep bool, _ any) error {
	p.calls = append(p.calls, routingCall{"structured", useDeep})
	return nil
}

func TestRoutedClientSeparatesQuickAndDeepProviders(t *testing.T) {
	quick, deep := &routingProbe{}, &routingProbe{}
	client := NewRoutedClient(quick, deep)
	_, _ = client.Generate(context.Background(), "", "", false)
	_, _ = client.Generate(context.Background(), "", "", true)
	_, _ = client.GenerateWithTools(context.Background(), "", nil, nil, false, nil, nil)
	_, _ = client.GenerateWithTools(context.Background(), "", nil, nil, true, nil, nil)
	_ = client.StructuredGenerate(context.Background(), "", "", false, nil)
	_ = client.StructuredGenerate(context.Background(), "", "", true, nil)
	wantQuick := []routingCall{{"generate", false}, {"tools", false}, {"structured", false}}
	wantDeep := []routingCall{{"generate", true}, {"tools", true}, {"structured", true}}
	if !reflect.DeepEqual(quick.calls, wantQuick) || !reflect.DeepEqual(deep.calls, wantDeep) {
		t.Fatalf("quick calls=%v deep calls=%v", quick.calls, deep.calls)
	}
}

func TestRoutedClientFallsBackToQuickProviderOnTransientDeepFailure(t *testing.T) {
	for _, message := range []string{
		"Gemini API error: Error 503, Status: UNAVAILABLE",
		`Gemini API error: doRequest: Post "https://example.test": net/http: TLS handshake timeout`,
	} {
		t.Run(message, func(t *testing.T) {
			quick := &routingProbe{}
			deep := &failingRoutingProbe{err: errors.New(message)}
			client := NewRoutedClient(quick, deep)

			got, err := client.Generate(context.Background(), "system", "user", true)
			if err != nil || got != "ok" {
				t.Fatalf("transient deep failure did not fall back: got=%q err=%v", got, err)
			}
			if want := []routingCall{{"generate", true}}; !reflect.DeepEqual(quick.calls, want) {
				t.Fatalf("quick fallback calls=%v want=%v", quick.calls, want)
			}
		})
	}
}

func TestRoutedClientDoesNotFallbackOnPermanentOrCancelledFailure(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "permanent", err: errors.New("invalid API key")},
		{name: "cancelled", err: context.Canceled},
	} {
		t.Run(test.name, func(t *testing.T) {
			quick := &routingProbe{}
			client := NewRoutedClient(quick, &failingRoutingProbe{err: test.err})
			if _, err := client.Generate(context.Background(), "system", "user", true); !errors.Is(err, test.err) {
				t.Fatalf("error=%v want=%v", err, test.err)
			}
			if len(quick.calls) != 0 {
				t.Fatalf("unexpected fallback calls=%v", quick.calls)
			}
		})
	}
}

func TestProbeSelectedProviderDoesNotHideDeepFailureWithFallback(t *testing.T) {
	quick := &routingProbe{}
	deepErr := errors.New("Gemini API error: Error 503, Status: UNAVAILABLE")
	client := NewRoutedClient(quick, &failingRoutingProbe{err: deepErr})

	if _, err := ProbeSelectedProvider(context.Background(), client, "system", "user", true); !errors.Is(err, deepErr) {
		t.Fatalf("strict deep probe error=%v want=%v", err, deepErr)
	}
	if len(quick.calls) != 0 {
		t.Fatalf("strict deep probe unexpectedly used fallback: %+v", quick.calls)
	}
}

func TestRoutedClientRecordsActualFallbackProvider(t *testing.T) {
	quick := &routingProbe{}
	deep := &failingRoutingProbe{err: errors.New("Gemini API error: Error 503, Status: UNAVAILABLE")}
	client := NewNamedRoutedClient(quick, deep, "deepseek/quick", "gemini/deep")
	trace := NewRouteTrace()
	ctx := WithRouteTrace(context.Background(), trace)

	if _, err := client.Generate(ctx, "system", "user", true); err != nil {
		t.Fatal(err)
	}
	event, ok := trace.LastSuccessful()
	if !ok || event.Provider != "deepseek/quick" || !event.Fallback || !event.RequestedDeep {
		t.Fatalf("actual fallback provider was not recorded: %+v ok=%t", event, ok)
	}
}
