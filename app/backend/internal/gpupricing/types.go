package gpupricing

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	ProviderRunPod = "runpod"
	ProviderModal  = "modal"
	ProviderLambda = "lambda"
	ProviderVast   = "vast"

	StatusLive         = "live"
	StatusPartial      = "partial"
	StatusStale        = "stale"
	StatusUnavailable  = "unavailable"
	StatusUnconfigured = "unconfigured"
	StatusRefreshing   = "refreshing"
)

var ErrRefreshInProgress = errors.New("GPU price refresh already in progress")

type Quote struct {
	Provider                string   `json:"provider"`
	GPUModel                string   `json:"gpuModel"`
	Product                 string   `json:"product"`
	BillingMode             string   `json:"billingMode"`
	GPUCount                int      `json:"gpuCount"`
	MemoryGiB               *float64 `json:"memoryGiB,omitempty"`
	Region                  string   `json:"region,omitempty"`
	OfferID                 string   `json:"offerId,omitempty"`
	Availability            string   `json:"availability,omitempty"`
	Currency                string   `json:"currency"`
	RawPrice                float64  `json:"rawPrice"`
	RawUnit                 string   `json:"rawUnit"`
	PriceUSDPerGPUHour      float64  `json:"priceUsdPerGpuHour"`
	InstanceTotalUSDPerHour *float64 `json:"instanceTotalUsdPerHour,omitempty"`
	ComputeUSDPerHour       *float64 `json:"computeUsdPerHour,omitempty"`
	StorageUSDPerHour       *float64 `json:"storageUsdPerHour,omitempty"`
	BandwidthUpUSDPerTB     *float64 `json:"bandwidthUpUsdPerTb,omitempty"`
	BandwidthDownUSDPerTB   *float64 `json:"bandwidthDownUsdPerTb,omitempty"`
	SourceURL               string   `json:"sourceUrl"`
	ObservedAt              string   `json:"observedAt"`
	SourceIdentity          string   `json:"-"`
}

type ProviderStatus struct {
	Provider      string `json:"provider"`
	Configured    bool   `json:"configured"`
	Status        string `json:"status"`
	QuoteCount    int    `json:"quoteCount"`
	LastAttemptAt string `json:"lastAttemptAt,omitempty"`
	LastSuccessAt string `json:"lastSuccessAt,omitempty"`
	ErrorCode     string `json:"errorCode,omitempty"`
}

type Snapshot struct {
	Status        string           `json:"status"`
	DataMode      string           `json:"dataMode,omitempty"`
	TestOnly      bool             `json:"testOnly,omitempty"`
	ObservedAt    string           `json:"observedAt,omitempty"`
	NextRefreshAt string           `json:"nextRefreshAt,omitempty"`
	Refreshing    bool             `json:"refreshing"`
	Providers     []ProviderStatus `json:"providers"`
	Quotes        []Quote          `json:"quotes"`
	Count         int              `json:"count"`
}

type RefreshResult struct {
	Snapshot Snapshot `json:"snapshot"`
	Updated  int      `json:"updated"`
}

type HistoryFilter struct {
	Provider string
	GPUModel string
	Days     int
	Limit    int
}

type HistoryResult struct {
	Items    []Quote `json:"items"`
	Count    int     `json:"count"`
	Days     int     `json:"days"`
	DataMode string  `json:"dataMode,omitempty"`
	TestOnly bool    `json:"testOnly,omitempty"`
}

// Provider is the internal seam for authenticated official pricing adapters.
type Provider interface {
	Name() string
	Configured() bool
	Fetch(context.Context, time.Time) ([]Quote, error)
}

type providerError struct{ code string }

func (e providerError) Error() string { return e.code }

func errorCode(err error) string {
	if err == nil {
		return ""
	}
	var classified providerError
	if errors.As(err, &classified) {
		return classified.code
	}
	return "request_failed"
}

func validQuote(q Quote) bool {
	return strings.TrimSpace(q.Provider) != "" && strings.TrimSpace(q.GPUModel) != "" &&
		strings.TrimSpace(q.Product) != "" && strings.TrimSpace(q.BillingMode) != "" &&
		q.GPUCount > 0 && strings.EqualFold(q.Currency, "USD") && q.RawPrice > 0 &&
		strings.TrimSpace(q.RawUnit) != "" && q.PriceUSDPerGPUHour > 0 &&
		strings.TrimSpace(q.SourceURL) != "" && strings.TrimSpace(q.SourceIdentity) != "" &&
		strings.TrimSpace(q.ObservedAt) != ""
}
