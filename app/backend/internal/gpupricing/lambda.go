package gpupricing

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const lambdaSourceURL = "https://cloud.lambda.ai/api/v1/instance-types"

var gpuDescriptionPattern = regexp.MustCompile(`(?i)(\d+)\s*x\s*([^,]+?)(?:\s+GPU)?(?:,|$)`)

type lambdaProvider struct {
	key      string
	endpoint string
	http     *http.Client
}

func (p *lambdaProvider) Name() string     { return ProviderLambda }
func (p *lambdaProvider) Configured() bool { return strings.TrimSpace(p.key) != "" }

func (p *lambdaProvider) Fetch(ctx context.Context, observedAt time.Time) ([]Quote, error) {
	if !p.Configured() {
		return nil, providerError{code: StatusUnconfigured}
	}
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = lambdaSourceURL
	}
	var response struct {
		Data map[string]struct {
			InstanceType struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Specs       struct {
					GPUs      int     `json:"gpus"`
					MemoryGiB float64 `json:"memory_gib"`
				} `json:"specs"`
			} `json:"instance_type"`
			Regions []struct {
				Name string `json:"name"`
			} `json:"regions_with_capacity_available"`
			PriceCents float64 `json:"price_cents_per_hour"`
		} `json:"data"`
	}
	headers := map[string]string{"Authorization": "Bearer " + strings.TrimSpace(p.key), "Accept": "application/json"}
	if err := doJSON(ctx, p.http, http.MethodGet, endpoint, nil, headers, &response); err != nil {
		return nil, err
	}
	quotes := make([]Quote, 0, len(response.Data))
	for identity, item := range response.Data {
		gpuCount := item.InstanceType.Specs.GPUs
		gpuModel := lambdaGPUModel(item.InstanceType.Description, item.InstanceType.Name, identity, &gpuCount)
		instancePrice := item.PriceCents / 100
		if gpuCount <= 0 || gpuModel == "" || instancePrice <= 0 {
			continue
		}
		region := ""
		availability := "unavailable"
		if len(item.Regions) > 0 {
			availability = "available"
			regionNames := make([]string, 0, len(item.Regions))
			for _, candidate := range item.Regions {
				if candidate.Name != "" {
					regionNames = append(regionNames, candidate.Name)
				}
			}
			region = strings.Join(regionNames, ",")
		}
		quotes = append(quotes, Quote{
			Provider: ProviderLambda, GPUModel: gpuModel, Product: firstNonEmpty(item.InstanceType.Name, identity), BillingMode: "on-demand",
			GPUCount: gpuCount, MemoryGiB: floatPointer(item.InstanceType.Specs.MemoryGiB), Region: region, Availability: availability,
			Currency: "USD", RawPrice: item.PriceCents, RawUnit: "US-cents/instance-hour",
			PriceUSDPerGPUHour: instancePrice / float64(gpuCount), InstanceTotalUSDPerHour: floatPointer(instancePrice),
			SourceURL: lambdaSourceURL, ObservedAt: observedAt.Format(time.RFC3339), SourceIdentity: identity,
		})
	}
	if len(quotes) == 0 {
		return nil, providerError{code: "no_valid_quotes"}
	}
	return quotes, nil
}

func lambdaGPUModel(description, name, identity string, gpuCount *int) string {
	if match := gpuDescriptionPattern.FindStringSubmatch(description); len(match) == 3 {
		if parsed, err := strconv.Atoi(match[1]); err == nil && *gpuCount <= 0 {
			*gpuCount = parsed
		}
		return strings.TrimSpace(match[2])
	}
	candidate := strings.TrimSpace(firstNonEmpty(name, identity))
	if candidate == "" {
		return ""
	}
	return strings.TrimSpace(strings.NewReplacer("gpu_", "", "_", " ").Replace(candidate))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
