package gpupricing

const (
	ReferenceVerifiedAt = "2026-09-09T00:00:00Z"
	ReferenceSource     = "official-public-pricing-pages"
)

type ReferenceQuote struct {
	Provider                string   `json:"provider"`
	GPUModel                string   `json:"gpuModel"`
	Product                 string   `json:"product"`
	BillingMode             string   `json:"billingMode"`
	GPUCount                int      `json:"gpuCount"`
	Currency                string   `json:"currency"`
	RawPrice                float64  `json:"rawPrice"`
	RawUnit                 string   `json:"rawUnit"`
	PriceUSDPerGPUHour      float64  `json:"priceUsdPerGpuHour"`
	InstanceTotalUSDPerHour *float64 `json:"instanceTotalUsdPerHour,omitempty"`
}

type ReferenceProvider struct {
	Provider          string `json:"provider"`
	Status            string `json:"status"`
	SourceURL         string `json:"sourceUrl"`
	Note              string `json:"note"`
	HasFixedReference bool   `json:"hasFixedReference"`
}

type ReferenceSnapshot struct {
	Status     string              `json:"status"`
	DataMode   string              `json:"dataMode"`
	Stale      bool                `json:"stale"`
	Source     string              `json:"source"`
	VerifiedAt string              `json:"verifiedAt"`
	Count      int                 `json:"count"`
	Disclaimer string              `json:"disclaimer"`
	Providers  []ReferenceProvider `json:"providers"`
	Items      []ReferenceQuote    `json:"items"`
}

// PublicReference returns a new, read-only historical baseline assembled from
// official public pricing pages verified on ReferenceVerifiedAt.
func PublicReference() ReferenceSnapshot {
	items := []ReferenceQuote{
		refHourly(ProviderRunPod, "B300", "secure-cloud", 1, 7.89),
		refHourly(ProviderRunPod, "B200", "secure-cloud", 1, 6.79),
		refHourly(ProviderRunPod, "H200", "secure-cloud", 1, 4.59),
		refHourly(ProviderRunPod, "H100 SXM", "secure-cloud", 1, 3.49),
		refHourly(ProviderRunPod, "A100 PCIe", "secure-cloud", 1, 1.59),
		refHourly(ProviderRunPod, "A100 SXM", "secure-cloud", 1, 1.59),
		refHourly(ProviderRunPod, "RTX 4090", "secure-cloud", 1, 0.74),

		refPerSecond(ProviderModal, "B300", "serverless-function", 0.001972),
		refPerSecond(ProviderModal, "B200", "serverless-function", 0.001736),
		refPerSecond(ProviderModal, "H200 SXM", "serverless-function", 0.001261),
		refPerSecond(ProviderModal, "H100 SXM", "serverless-function", 0.001097),
		refPerSecond(ProviderModal, "A100 80 GB", "serverless-function", 0.000694),
		refPerSecond(ProviderModal, "A100 40 GB", "serverless-function", 0.000583),
		refPerSecond(ProviderModal, "L40S", "serverless-function", 0.000542),

		refLambda("B200", "8-gpu-instance", 8, 6.69),
		refLambda("H100 SXM", "8-gpu-instance", 8, 3.99),
		refLambda("A100 80 GB", "8-gpu-instance", 8, 2.79),
		refLambda("B200", "1-gpu-instance", 1, 6.99),
		refLambda("GH200", "1-gpu-instance", 1, 2.29),
		refLambda("H100 SXM", "1-gpu-instance", 1, 4.29),
		refLambda("H100 PCIe", "1-gpu-instance", 1, 3.29),
		refLambda("A100 40 GB", "1-gpu-instance", 1, 1.99),
	}
	return ReferenceSnapshot{
		Status: "historical", DataMode: "historical", Stale: true, Source: ReferenceSource,
		VerifiedAt: ReferenceVerifiedAt, Count: len(items),
		Disclaimer: "Historical public reference prices verified on 2026-09-09; not live quotes and not persisted as observations.",
		Providers: []ReferenceProvider{
			{Provider: ProviderRunPod, Status: "historical", SourceURL: "https://www.runpod.io/pricing", Note: "Secure Cloud public prices in USD per GPU-hour.", HasFixedReference: true},
			{Provider: ProviderModal, Status: "historical", SourceURL: "https://modal.com/pricing", Note: "Public per-second GPU prices; hourly values are comparisons multiplied by 3600.", HasFixedReference: true},
			{Provider: ProviderLambda, Status: "historical", SourceURL: "https://lambda.ai/instances", Note: "On-Demand Cloud public prices vary by instance GPU count.", HasFixedReference: true},
			{Provider: ProviderVast, Status: "market-variable", SourceURL: "https://docs.vast.ai/guides/instances/pricing", Note: "Marketplace offers vary continuously; no fixed public reference quote is presented.", HasFixedReference: false},
		},
		Items: items,
	}
}

func refHourly(provider, model, product string, gpuCount int, price float64) ReferenceQuote {
	return ReferenceQuote{Provider: provider, GPUModel: model, Product: product, BillingMode: "on-demand", GPUCount: gpuCount,
		Currency: "USD", RawPrice: price, RawUnit: "USD/GPU-hour", PriceUSDPerGPUHour: price}
}

func refPerSecond(provider, model, product string, price float64) ReferenceQuote {
	return ReferenceQuote{Provider: provider, GPUModel: model, Product: product, BillingMode: "usage-based", GPUCount: 1,
		Currency: "USD", RawPrice: price, RawUnit: "USD/GPU-second", PriceUSDPerGPUHour: price * 3600}
}

func refLambda(model, product string, gpuCount int, perGPUPrice float64) ReferenceQuote {
	total := perGPUPrice * float64(gpuCount)
	return ReferenceQuote{Provider: ProviderLambda, GPUModel: model, Product: product, BillingMode: "on-demand", GPUCount: gpuCount,
		Currency: "USD", RawPrice: perGPUPrice, RawUnit: "USD/GPU-hour", PriceUSDPerGPUHour: perGPUPrice, InstanceTotalUSDPerHour: &total}
}
