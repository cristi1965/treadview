import type { APIResponseMeta } from '../utils/api';
import type { DataTrustAssessment } from '../utils/dataTrust';

export type GPUProvider = 'runpod' | 'modal' | 'lambda' | 'vast' | string;

export interface GPUPriceQuote {
  provider: GPUProvider;
  gpuModel: string;
  product: string;
  billingMode: string;
  gpuCount: number;
  memoryGiB?: number;
  region?: string;
  offerId?: string;
  availability?: string;
  currency: string;
  rawPrice: number;
  rawUnit: string;
  priceUsdPerGpuHour: number;
  instanceTotalUsdPerHour?: number;
  storageUsdPerHour?: number;
  bandwidthUpUsdPerTb?: number;
  bandwidthDownUsdPerTb?: number;
  sourceUrl: string;
  observedAt: string;
}

export type GPUProviderState = 'live' | 'stale' | 'error' | 'unavailable' | 'unconfigured' | 'refreshing' | string;

export interface GPUProviderStatus {
  provider: GPUProvider;
  status: GPUProviderState;
  configured?: boolean;
  message?: string;
  observedAt?: string;
  lastAttemptAt?: string;
  lastSuccessAt?: string;
  errorCode?: string;
  quoteCount?: number;
}

export interface GPUPricesResponse {
  quotes?: unknown[];
  items?: unknown[];
  providers?: unknown[] | Record<string, unknown>;
  providerStatuses?: unknown[] | Record<string, unknown>;
  count?: number;
  status?: string;
  observedAt?: string;
  nextRefreshAt?: string;
	dataMode?: string;
	testOnly?: boolean;
}

export interface GPUPriceHistoryPoint {
  provider: GPUProvider;
  gpuModel: string;
  billingMode: string;
  priceUsdPerGpuHour: number;
  observedAt: string;
  sourceUrl?: string;
}

export interface GPUPriceHistoryResponse {
  points?: unknown[];
  items?: unknown[];
  history?: unknown[];
  count?: number;
  status?: string;
	dataMode?: string;
	testOnly?: boolean;
}

export interface GPUReferenceProvider {
  provider: GPUProvider;
  status: string;
  sourceUrl: string;
  note?: string;
  hasFixedReference: boolean;
}

export interface GPUReferencePrice {
  provider: GPUProvider;
  gpuModel: string;
  product: string;
  billingMode: string;
  gpuCount: number;
  currency: string;
  rawPrice: number;
  rawUnit: string;
  priceUsdPerGpuHour: number;
  instanceTotalUsdPerHour?: number;
  sourceUrl: string;
  verifiedAt: string;
}

export interface GPUPriceReferenceResponse {
  status?: string;
  dataMode?: string;
  stale?: boolean;
  source?: string;
  verifiedAt?: string;
  count?: number;
  disclaimer?: string;
  providers?: unknown[];
  items?: unknown[];
}

export interface AdaptedGPUPriceReference {
  items: GPUReferencePrice[];
  providers: GPUReferenceProvider[];
  verifiedAt: string;
  disclaimer: string;
  source: string;
  available: boolean;
  reason: string;
  meta: APIResponseMeta | null;
}

export interface AdaptedGPUPrices {
  quotes: GPUPriceQuote[];
  providers: GPUProviderStatus[];
  meta: APIResponseMeta | null;
  trust: DataTrustAssessment;
  declaredCount: number;
  nextRefreshAt: string;
	dataMode: string;
	testOnly: boolean;
}

export interface AdaptedGPUPriceHistory {
  points: GPUPriceHistoryPoint[];
  meta: APIResponseMeta | null;
  trust: DataTrustAssessment;
  declaredCount: number;
  empty: boolean;
	dataMode: string;
	testOnly: boolean;
}
