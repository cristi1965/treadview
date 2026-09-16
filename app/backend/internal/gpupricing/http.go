package gpupricing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

const maxProviderBody = 16 << 20

func doJSON(ctx context.Context, client *http.Client, method, endpoint string, body io.Reader, headers map[string]string, target any) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return providerError{code: "request_invalid"}
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return providerError{code: "request_failed"}
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return providerError{code: "unauthorized"}
	case http.StatusTooManyRequests:
		return providerError{code: "rate_limited"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return providerError{code: "upstream_error"}
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxProviderBody))
	if err := decoder.Decode(target); err != nil {
		return providerError{code: "invalid_response"}
	}
	return nil
}

func floatPointer(value float64) *float64 {
	if value <= 0 {
		return nil
	}
	copy := value
	return &copy
}
