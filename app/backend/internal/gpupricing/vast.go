package gpupricing

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const vastSourceURL = "https://console.vast.ai/api/v0/bundles/"

type vastProvider struct {
	key      string
	endpoint string
	http     *http.Client
}

func (p *vastProvider) Name() string     { return ProviderVast }
func (p *vastProvider) Configured() bool { return strings.TrimSpace(p.key) != "" }

func (p *vastProvider) Fetch(ctx context.Context, observedAt time.Time) ([]Quote, error) {
	if !p.Configured() {
		return nil, providerError{code: StatusUnconfigured}
	}
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = vastSourceURL
	}
	body := strings.NewReader(`{"q":{"verified":{"eq":true},"rentable":{"eq":true},"rented":{"eq":false},"type":"on-demand"}}`)
	var response struct {
		Offers []struct {
			ID           any     `json:"id"`
			AskContract  any     `json:"ask_contract_id"`
			GPUName      string  `json:"gpu_name"`
			NumGPUs      int     `json:"num_gpus"`
			GPURAM       float64 `json:"gpu_ram"`
			DPHBase      float64 `json:"dph_base"`
			DPHTotal     float64 `json:"dph_total"`
			StorageCost  float64 `json:"storage_cost"`
			DiskSpace    float64 `json:"disk_space"`
			InetUpCost   float64 `json:"inet_up_cost"`
			InetDownCost float64 `json:"inet_down_cost"`
			GeoLocation  string  `json:"geolocation"`
			Rentable     bool    `json:"rentable"`
		} `json:"offers"`
	}
	headers := map[string]string{"Authorization": "Bearer " + strings.TrimSpace(p.key), "Content-Type": "application/json", "Accept": "application/json"}
	if err := doJSON(ctx, p.http, http.MethodPost, endpoint, body, headers, &response); err != nil {
		return nil, err
	}
	quotes := make([]Quote, 0, len(response.Offers))
	for _, item := range response.Offers {
		if item.NumGPUs <= 0 || item.DPHTotal <= 0 || strings.TrimSpace(item.GPUName) == "" {
			continue
		}
		offerID := anyID(firstAny(item.AskContract, item.ID))
		storagePerHour := 0.0
		if item.StorageCost > 0 && item.DiskSpace > 0 {
			storagePerHour = item.StorageCost * item.DiskSpace / (30 * 24)
		}
		quotes = append(quotes, Quote{
			Provider: ProviderVast, GPUModel: strings.TrimSpace(item.GPUName), Product: "market-offer", BillingMode: "on-demand",
			GPUCount: item.NumGPUs, MemoryGiB: vastMemoryGiB(item.GPURAM), Region: item.GeoLocation, OfferID: offerID,
			Availability: map[bool]string{true: "rentable", false: "unavailable"}[item.Rentable], Currency: "USD",
			RawPrice: item.DPHTotal, RawUnit: "USD/instance-hour", PriceUSDPerGPUHour: item.DPHTotal / float64(item.NumGPUs),
			InstanceTotalUSDPerHour: floatPointer(item.DPHTotal), ComputeUSDPerHour: floatPointer(item.DPHBase), StorageUSDPerHour: floatPointer(storagePerHour),
			BandwidthUpUSDPerTB: floatPointer(item.InetUpCost), BandwidthDownUSDPerTB: floatPointer(item.InetDownCost),
			SourceURL: vastSourceURL, ObservedAt: observedAt.Format(time.RFC3339), SourceIdentity: firstNonEmpty(offerID, item.GPUName+":"+item.GeoLocation),
		})
	}
	if len(quotes) == 0 {
		return nil, providerError{code: "no_valid_quotes"}
	}
	return quotes, nil
}

func vastMemoryGiB(value float64) *float64 {
	if value <= 0 {
		return nil
	}
	if value > 1024 {
		value /= 1024
	}
	return floatPointer(value)
}

func firstAny(values ...any) any {
	for _, value := range values {
		if anyID(value) != "" {
			return value
		}
	}
	return nil
}

func anyID(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case int:
		return strconv.Itoa(typed)
	default:
		return ""
	}
}
