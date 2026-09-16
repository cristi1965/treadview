package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"trading-agents/internal/config"
	"trading-agents/internal/gpupricing"
)

func (h *Handler) GetGPUPrices(c *gin.Context) {
	snapshot := h.gpuPriceSnapshot()
	setDataFreshness(c, gpuPriceFreshness(snapshot))
	c.JSON(http.StatusOK, snapshot)
}

func (h *Handler) GetGPUPriceStatus(c *gin.Context) {
	snapshot := h.gpuPriceSnapshot()
	status := snapshot
	status.Quotes = []gpupricing.Quote{}
	status.Count = 0
	setDataFreshness(c, gpuPriceFreshness(snapshot))
	c.JSON(http.StatusOK, status)
}

func (h *Handler) GetGPUPriceReference(c *gin.Context) {
	reference := gpupricing.PublicReference()
	setDataFreshness(c, dataFreshnessMeta{
		Source: gpupricing.ReferenceSource, DataTime: gpupricing.ReferenceVerifiedAt,
		Stale: true, StaleReason: "historical public pricing reference; not live provider observations", Refreshable: false,
	})
	c.JSON(http.StatusOK, reference)
}

func (h *Handler) GetGPUPriceHistory(c *gin.Context) {
	days, ok := boundedPositiveQuery(c, "days", 30, 3650)
	if !ok {
		return
	}
	limit, ok := boundedPositiveQuery(c, "limit", 1000, 5000)
	if !ok {
		return
	}
	provider := strings.ToLower(strings.TrimSpace(c.Query("provider")))
	if provider != "" && provider != gpupricing.ProviderRunPod && provider != gpupricing.ProviderModal && provider != gpupricing.ProviderLambda && provider != gpupricing.ProviderVast {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider must be runpod, modal, lambda, or vast"})
		return
	}
	if h.gpuPricing == nil {
		setDataFreshness(c, dataFreshnessMeta{Source: "gpu-price-history", DataTime: "unknown", Stale: true, StaleReason: "GPU price history store unavailable", Refreshable: false})
		c.JSON(http.StatusOK, gpupricing.HistoryResult{Items: []gpupricing.Quote{}, Days: days})
		return
	}
	result, err := h.gpuPricing.History(c.Request.Context(), gpupricing.HistoryFilter{
		Provider: provider, GPUModel: strings.TrimSpace(c.Query("gpuModel")), Days: days, Limit: limit,
	})
	if err != nil {
		staleDataError(c, http.StatusInternalServerError, dataFreshnessMeta{Source: "gpu-price-history", Refreshable: false}, "GPU price history unavailable")
		return
	}
	setDataFreshness(c, gpuPriceHistoryFreshness(result, time.Now().UTC()))
	c.JSON(http.StatusOK, result)
}

func (h *Handler) RefreshGPUPrices(c *gin.Context) {
	if h.gpuPricing == nil {
		staleDataError(c, http.StatusServiceUnavailable, dataFreshnessMeta{Source: "official-provider-apis", Refreshable: false}, "GPU price module unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Minute)
	defer cancel()
	result, err := h.gpuPricing.Refresh(ctx)
	if errors.Is(err, gpupricing.ErrRefreshInProgress) {
		setDataFreshness(c, gpuPriceFreshness(result.Snapshot))
		c.JSON(http.StatusConflict, gin.H{"error": "GPU price refresh already in progress", "snapshot": result.Snapshot})
		return
	}
	if err != nil {
		meta := gpuPriceFreshness(result.Snapshot)
		meta.Stale = true
		meta.StaleReason = "GPU price refresh unavailable"
		staleDataError(c, http.StatusServiceUnavailable, meta, "GPU price refresh unavailable")
		return
	}
	setDataFreshness(c, gpuPriceFreshness(result.Snapshot))
	c.JSON(http.StatusOK, result)
}

func (h *Handler) ReloadGPUPriceConfig(c *gin.Context) {
	if h == nil || h.config == nil || h.gpuPricing == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "GPU price module unavailable"})
		return
	}
	next := *h.config
	if err := config.ReloadGPUProviderCredentials(&next); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if err := h.gpuPricing.Reconfigure(&next); errors.Is(err, gpupricing.ErrRefreshInProgress) {
		c.JSON(http.StatusConflict, gin.H{"error": "GPU price refresh already in progress"})
		return
	} else if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "GPU provider configuration reload failed"})
		return
	}
	snapshot := h.gpuPricing.Current()
	setDataFreshness(c, gpuPriceFreshness(snapshot))
	c.JSON(http.StatusOK, gin.H{
		"status": snapshot.Status, "dataMode": snapshot.DataMode, "testOnly": snapshot.TestOnly,
		"providers": snapshot.Providers, "nextRefreshAt": snapshot.NextRefreshAt,
	})
}

func (h *Handler) gpuPriceSnapshot() gpupricing.Snapshot {
	if h == nil || h.gpuPricing == nil {
		return gpupricing.Snapshot{Status: gpupricing.StatusUnavailable, Providers: []gpupricing.ProviderStatus{}, Quotes: []gpupricing.Quote{}}
	}
	return h.gpuPricing.Current()
}

func gpuPriceFreshness(snapshot gpupricing.Snapshot) dataFreshnessMeta {
	source := "official-provider-apis"
	if snapshot.TestOnly || snapshot.DataMode == gpupricing.RuntimeFixtureDataMode {
		source = gpupricing.RuntimeFixtureDataMode
	}
	meta := dataFreshnessMeta{Source: source, DataTime: snapshot.ObservedAt, Refreshable: false}
	configured := 0
	for _, provider := range snapshot.Providers {
		if provider.Configured {
			configured++
		}
		if provider.ErrorCode != "" {
			meta.PartialErrors = append(meta.PartialErrors, provider.Provider+":"+provider.ErrorCode)
		}
	}
	meta.Refreshable = configured > 0
	switch snapshot.Status {
	case gpupricing.StatusLive:
		return meta
	case gpupricing.StatusUnconfigured:
		meta.Stale = true
		meta.StaleReason = "GPU price provider credentials are not configured"
	case gpupricing.StatusRefreshing:
		meta.Stale = true
		meta.StaleReason = "GPU price refresh is in progress"
	case gpupricing.StatusPartial:
		meta.Stale = true
		meta.StaleReason = "one or more GPU price providers are stale or unavailable"
	case gpupricing.StatusStale:
		meta.Stale = true
		meta.StaleReason = "last authenticated GPU price observations are stale"
	default:
		meta.Stale = true
		meta.StaleReason = "authenticated GPU price observations are unavailable"
	}
	return meta
}

func gpuPriceHistoryFreshness(result gpupricing.HistoryResult, now time.Time) dataFreshnessMeta {
	source := "gpu-price-history"
	if result.TestOnly || result.DataMode == gpupricing.RuntimeFixtureDataMode {
		source = gpupricing.RuntimeFixtureDataMode
	}
	meta := dataFreshnessMeta{Source: source, DataTime: "unknown", Refreshable: false}
	if len(result.Items) == 0 {
		meta.Stale = true
		meta.StaleReason = "no authenticated observations"
		return meta
	}
	latest := time.Time{}
	for _, item := range result.Items {
		observedAt, err := time.Parse(time.RFC3339, item.ObservedAt)
		if err == nil && (latest.IsZero() || observedAt.After(latest)) {
			latest = observedAt
		}
	}
	if latest.IsZero() {
		meta.Stale = true
		meta.StaleReason = "authenticated observations have invalid timestamps"
		return meta
	}
	meta.DataTime = latest.UTC().Format(time.RFC3339)
	if latest.After(now.Add(time.Minute)) {
		meta.Stale = true
		meta.StaleReason = "authenticated observation time is in the future"
		return meta
	}
	age := now.Sub(latest)
	if age > gpupricing.ObservationFreshFor {
		meta.Stale = true
		meta.StaleReason = "authenticated observations exceed the freshness window"
	}
	return meta
}

func boundedPositiveQuery(c *gin.Context, name string, fallback, maximum int) (int, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 || value > maximum {
		c.JSON(http.StatusBadRequest, gin.H{"error": name + " must be between 1 and " + strconv.Itoa(maximum)})
		return 0, false
	}
	return value, true
}
