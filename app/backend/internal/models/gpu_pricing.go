package models

import "time"

// GPUPriceObservation is one authenticated, provider-observed price point.
// SourceIdentity and ObservedAt form the provider-side observation identity.
type GPUPriceObservation struct {
	ID                      uint      `gorm:"primaryKey" json:"id"`
	CreatedAt               time.Time `json:"createdAt"`
	Provider                string    `gorm:"size:24;not null;index;uniqueIndex:idx_gpu_price_identity,priority:1" json:"provider"`
	SourceIdentity          string    `gorm:"size:240;not null;uniqueIndex:idx_gpu_price_identity,priority:2" json:"sourceIdentity"`
	ObservedAt              time.Time `gorm:"not null;index;uniqueIndex:idx_gpu_price_identity,priority:3" json:"observedAt"`
	GPUModel                string    `gorm:"size:100;not null;index" json:"gpuModel"`
	Product                 string    `gorm:"size:100;not null" json:"product"`
	BillingMode             string    `gorm:"size:40;not null;index" json:"billingMode"`
	GPUCount                int       `gorm:"not null" json:"gpuCount"`
	MemoryGiB               *float64  `json:"memoryGiB,omitempty"`
	Region                  string    `gorm:"size:100;index" json:"region,omitempty"`
	OfferID                 string    `gorm:"size:120;index" json:"offerId,omitempty"`
	Availability            string    `gorm:"size:80" json:"availability,omitempty"`
	Currency                string    `gorm:"size:8;not null" json:"currency"`
	RawPrice                float64   `gorm:"not null" json:"rawPrice"`
	RawUnit                 string    `gorm:"size:40;not null" json:"rawUnit"`
	PriceUSDPerGPUHour      float64   `gorm:"not null" json:"priceUsdPerGpuHour"`
	InstanceTotalUSDPerHour *float64  `json:"instanceTotalUsdPerHour,omitempty"`
	ComputeUSDPerHour       *float64  `json:"computeUsdPerHour,omitempty"`
	StorageUSDPerHour       *float64  `json:"storageUsdPerHour,omitempty"`
	BandwidthUpUSDPerTB     *float64  `json:"bandwidthUpUsdPerTb,omitempty"`
	BandwidthDownUSDPerTB   *float64  `json:"bandwidthDownUsdPerTb,omitempty"`
	SourceURL               string    `gorm:"size:500;not null" json:"sourceUrl"`
}
