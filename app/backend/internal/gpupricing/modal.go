package gpupricing

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const modalSourceURL = "https://modal.com/docs/cli/latest/billing"

var modalGPUNamePattern = regexp.MustCompile(`(?i)(B300|B200|H200(?:\s*SXM)?|H100(?:\s*SXM)?|A100(?:\s*(?:40|80)\s*GB)?|L40S|L4|A10G?|T4|RTX\s*4090)`)

type commandRunner func(context.Context, []string) ([]byte, error)

type modalProvider struct {
	tokenID     string
	tokenSecret string
	run         commandRunner
}

func (p *modalProvider) Name() string { return ProviderModal }
func (p *modalProvider) Configured() bool {
	return strings.TrimSpace(p.tokenID) != "" && strings.TrimSpace(p.tokenSecret) != ""
}

func (p *modalProvider) Fetch(ctx context.Context, observedAt time.Time) ([]Quote, error) {
	if !p.Configured() {
		return nil, providerError{code: StatusUnconfigured}
	}
	runner := p.run
	if runner == nil {
		runner = p.runOfficialCLI
	}
	raw, err := runner(ctx, []string{"billing", "rates", "--json"})
	if err != nil {
		return nil, providerError{code: "cli_unavailable"}
	}
	quotes, err := parseModalRates(raw, observedAt)
	if err != nil {
		return nil, err
	}
	return quotes, nil
}

func (p *modalProvider) runOfficialCLI(ctx context.Context, args []string) ([]byte, error) {
	path, err := exec.LookPath("modal")
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = append(os.Environ(),
		"MODAL_TOKEN_ID="+strings.TrimSpace(p.tokenID),
		"MODAL_TOKEN_SECRET="+strings.TrimSpace(p.tokenSecret),
	)
	return cmd.Output()
}

func parseModalRates(raw []byte, observedAt time.Time) ([]Quote, error) {
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, providerError{code: "invalid_response"}
	}
	quotes := make([]Quote, 0)
	seen := map[string]bool{}
	var walk func(any, string)
	walk = func(value any, path string) {
		switch typed := value.(type) {
		case map[string]any:
			model := modalGPUModel(firstString(typed, "gpuType", "gpu_type", "resource", "name", "type", "sku") + " " + path)
			price, ok := firstNumber(typed, "usdPerSecond", "usd_per_second", "pricePerSecond", "price_per_second", "rate", "price")
			unit := strings.ToLower(firstString(typed, "unit", "billingUnit", "billing_unit"))
			if model != "" && ok && price > 0 && (unit == "" || strings.Contains(unit, "second")) {
				appendModalQuote(&quotes, seen, model, path, price, observedAt)
			}
			for key, child := range typed {
				childPath := key
				if path != "" {
					childPath = path + "." + key
				}
				if numeric, ok := numberValue(child); ok {
					if childModel := modalGPUModel(childPath); childModel != "" && numeric > 0 {
						appendModalQuote(&quotes, seen, childModel, childPath, numeric, observedAt)
					}
					continue
				}
				walk(child, childPath)
			}
		case []any:
			for index, child := range typed {
				walk(child, path+"["+strconv.Itoa(index)+"]")
			}
		}
	}
	walk(root, "")
	if len(quotes) == 0 {
		return nil, providerError{code: "no_valid_quotes"}
	}
	return quotes, nil
}

func appendModalQuote(quotes *[]Quote, seen map[string]bool, model, identity string, raw float64, observedAt time.Time) {
	identity = model + ":" + firstNonEmpty(identity, model)
	key := model + ":" + identity
	if seen[key] {
		return
	}
	seen[key] = true
	*quotes = append(*quotes, Quote{
		Provider: ProviderModal, GPUModel: model, Product: "serverless-function", BillingMode: "usage-based",
		GPUCount: 1, Currency: "USD", RawPrice: raw, RawUnit: "USD/GPU-second", PriceUSDPerGPUHour: raw * 3600,
		SourceURL: modalSourceURL, ObservedAt: observedAt.Format(time.RFC3339), SourceIdentity: identity,
	})
}

func modalGPUModel(value string) string {
	match := modalGPUNamePattern.FindString(strings.ReplaceAll(value, "_", " "))
	return strings.ToUpper(strings.Join(strings.Fields(match), " "))
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNumber(values map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := numberValue(values[key]); ok {
			return value, true
		}
	}
	return 0, false
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}
