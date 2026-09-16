package gpupricing

import (
	"context"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestOfficialProvidersRequireCredentialsBeforeRequest(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return response(http.StatusOK, `{}`), nil
	})}
	providers := []Provider{
		&runPodProvider{http: client}, &lambdaProvider{http: client}, &vastProvider{http: client},
		&modalProvider{run: func(context.Context, []string) ([]byte, error) { calls++; return []byte(`{}`), nil }},
	}
	for _, provider := range providers {
		if provider.Configured() {
			t.Fatalf("%s unexpectedly configured", provider.Name())
		}
		if _, err := provider.Fetch(context.Background(), time.Now()); errorCode(err) != StatusUnconfigured {
			t.Fatalf("%s error=%v", provider.Name(), err)
		}
	}
	if calls != 0 {
		t.Fatalf("unconfigured providers performed %d requests", calls)
	}
}

func TestRunPodUsesAuthenticatedGraphQLAndPreservesProducts(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Query().Get("api_key") != "runpod-secret" {
			t.Fatalf("unexpected authenticated request: %s %s", request.Method, request.URL.String())
		}
		return response(http.StatusOK, `{"data":{"gpuTypes":[{"id":"NVIDIA GeForce RTX 4090","displayName":"RTX 4090","memoryInGb":24,"securePrice":0.74,"communityPrice":0.34,"stockStatus":"Medium"}]}}`), nil
	})}
	at := time.Date(2026, 9, 9, 8, 0, 0, 0, time.UTC)
	quotes, err := (&runPodProvider{key: "runpod-secret", endpoint: runPodSourceURL, http: client}).Fetch(context.Background(), at)
	if err != nil || len(quotes) != 2 {
		t.Fatalf("quotes=%+v err=%v", quotes, err)
	}
	if quotes[0].RawUnit != "USD/GPU-hour" || quotes[0].ObservedAt != at.Format(time.RFC3339) || quotes[0].SourceURL != runPodSourceURL {
		t.Fatalf("provenance lost: %+v", quotes[0])
	}
}

func TestLambdaUsesBearerAndConvertsInstanceCentsPerHour(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer lambda-secret" {
			t.Fatalf("authorization=%q", request.Header.Get("Authorization"))
		}
		return response(http.StatusOK, `{"data":{"gpu_8x_b200":{"instance_type":{"name":"gpu_8x_b200","description":"8x NVIDIA B200 GPU, 1440 GB RAM","specs":{"gpus":8,"memory_gib":1440}},"regions_with_capacity_available":[{"name":"us-west-1"}],"price_cents_per_hour":5352}}}`), nil
	})}
	quotes, err := (&lambdaProvider{key: "lambda-secret", endpoint: lambdaSourceURL, http: client}).Fetch(context.Background(), time.Now().UTC())
	if err != nil || len(quotes) != 1 {
		t.Fatalf("quotes=%+v err=%v", quotes, err)
	}
	if quotes[0].GPUCount != 8 || quotes[0].PriceUSDPerGPUHour != 6.69 || quotes[0].InstanceTotalUSDPerHour == nil || *quotes[0].InstanceTotalUSDPerHour != 53.52 {
		t.Fatalf("conversion=%+v", quotes[0])
	}
}

func TestVastUsesBearerAndKeepsInstanceAndPerGPUPrices(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer vast-secret" {
			t.Fatalf("authorization=%q", request.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"verified":{"eq":true}`) {
			t.Fatalf("missing official offer filters: %s", body)
		}
		return response(http.StatusOK, `{"offers":[{"ask_contract_id":42,"gpu_name":"H100 SXM","num_gpus":2,"gpu_ram":81920,"dph_base":4.2,"dph_total":4.6,"storage_cost":0.1,"disk_space":100,"inet_up_cost":2,"inet_down_cost":1,"geolocation":"US","rentable":true}]}`), nil
	})}
	quotes, err := (&vastProvider{key: "vast-secret", endpoint: vastSourceURL, http: client}).Fetch(context.Background(), time.Now().UTC())
	if err != nil || len(quotes) != 1 {
		t.Fatalf("quotes=%+v err=%v", quotes, err)
	}
	quote := quotes[0]
	if quote.OfferID != "42" || quote.PriceUSDPerGPUHour != 2.3 || quote.InstanceTotalUSDPerHour == nil || *quote.InstanceTotalUSDPerHour != 4.6 || quote.ComputeUSDPerHour == nil || *quote.ComputeUSDPerHour != 4.2 || quote.MemoryGiB == nil || *quote.MemoryGiB != 80 {
		t.Fatalf("vast quote=%+v", quote)
	}
}

func TestModalOfficialCLIRatesKeepRawSeconds(t *testing.T) {
	called := false
	provider := &modalProvider{tokenID: "id", tokenSecret: "secret", run: func(_ context.Context, args []string) ([]byte, error) {
		called = true
		if strings.Join(args, " ") != "billing rates --json" {
			t.Fatalf("args=%v", args)
		}
		return []byte(`{"gpuRates":[{"gpu_type":"H100 SXM","usd_per_second":0.001097,"unit":"second"}]}`), nil
	}}
	quotes, err := provider.Fetch(context.Background(), time.Now().UTC())
	if err != nil || !called || len(quotes) != 1 {
		t.Fatalf("quotes=%+v called=%v err=%v", quotes, called, err)
	}
	if quotes[0].RawPrice != 0.001097 || math.Abs(quotes[0].PriceUSDPerGPUHour-3.9492) > 1e-12 || quotes[0].RawUnit != "USD/GPU-second" {
		t.Fatalf("modal conversion=%+v", quotes[0])
	}
}

func TestHTTPProviderErrorsAreClassifiedWithoutResponseBodies(t *testing.T) {
	for _, test := range []struct {
		status int
		code   string
	}{{http.StatusUnauthorized, "unauthorized"}, {http.StatusTooManyRequests, "rate_limited"}, {http.StatusBadGateway, "upstream_error"}} {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return response(test.status, `{"secret":"must-not-leak"}`), nil
		})}
		_, err := (&lambdaProvider{key: "secret", endpoint: lambdaSourceURL, http: client}).Fetch(context.Background(), time.Now())
		if errorCode(err) != test.code || strings.Contains(err.Error(), "must-not-leak") {
			t.Fatalf("status=%d err=%v", test.status, err)
		}
	}
}
