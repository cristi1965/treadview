package gpupricing

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const runPodSourceURL = "https://api.runpod.io/graphql"

type runPodProvider struct {
	key      string
	endpoint string
	http     *http.Client
}

func (p *runPodProvider) Name() string     { return ProviderRunPod }
func (p *runPodProvider) Configured() bool { return strings.TrimSpace(p.key) != "" }

func (p *runPodProvider) Fetch(ctx context.Context, observedAt time.Time) ([]Quote, error) {
	if !p.Configured() {
		return nil, providerError{code: StatusUnconfigured}
	}
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = runPodSourceURL
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, providerError{code: "request_invalid"}
	}
	query := parsed.Query()
	query.Set("api_key", strings.TrimSpace(p.key))
	parsed.RawQuery = query.Encode()
	payload := strings.NewReader(`{"query":"query GPUPrices { gpuTypes { id displayName memoryInGb securePrice communityPrice stockStatus } }"}`)
	var response struct {
		Data struct {
			GPUTypes []struct {
				ID             string   `json:"id"`
				DisplayName    string   `json:"displayName"`
				MemoryInGB     *float64 `json:"memoryInGb"`
				SecurePrice    *float64 `json:"securePrice"`
				CommunityPrice *float64 `json:"communityPrice"`
				StockStatus    string   `json:"stockStatus"`
			} `json:"gpuTypes"`
		} `json:"data"`
		Errors []jsonError `json:"errors"`
	}
	if err := doJSON(ctx, p.http, http.MethodPost, parsed.String(), payload, map[string]string{"Content-Type": "application/json"}, &response); err != nil {
		return nil, err
	}
	if len(response.Errors) > 0 {
		return nil, providerError{code: "invalid_response"}
	}
	quotes := make([]Quote, 0, len(response.Data.GPUTypes)*2)
	for _, item := range response.Data.GPUTypes {
		name := strings.TrimSpace(item.DisplayName)
		if name == "" {
			name = strings.TrimSpace(item.ID)
		}
		for _, price := range []struct {
			product string
			value   *float64
		}{{"secure-cloud", item.SecurePrice}, {"community-cloud", item.CommunityPrice}} {
			if price.value == nil || *price.value <= 0 || name == "" {
				continue
			}
			quotes = append(quotes, Quote{
				Provider: ProviderRunPod, GPUModel: name, Product: price.product, BillingMode: "on-demand",
				GPUCount: 1, MemoryGiB: item.MemoryInGB, Availability: item.StockStatus, Currency: "USD",
				RawPrice: *price.value, RawUnit: "USD/GPU-hour", PriceUSDPerGPUHour: *price.value,
				SourceURL: runPodSourceURL, ObservedAt: observedAt.Format(time.RFC3339), SourceIdentity: item.ID + ":" + price.product,
			})
		}
	}
	if len(quotes) == 0 {
		return nil, providerError{code: "no_valid_quotes"}
	}
	return quotes, nil
}

type jsonError struct {
	Message string `json:"message"`
}
