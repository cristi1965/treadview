import type { APIResponseMeta } from './api';
import { assessDataTrust } from './dataTrust';
import type {
  AdaptedGPUPriceHistory,
  AdaptedGPUPriceReference,
  AdaptedGPUPrices,
  GPUPriceHistoryPoint,
  GPUPriceHistoryResponse,
  GPUPriceReferenceResponse,
  GPUPriceQuote,
  GPUPricesResponse,
  GPUProviderStatus,
  GPUReferencePrice,
  GPUReferenceProvider,
} from '../types/gpuPrices';

type UnknownRecord = Record<string, unknown>;

export interface GPUUpstreamRefreshAvailability {
  enabled: boolean;
  reason: string;
}

const DYNAMIC_GPU_PROVIDERS = ['runpod', 'modal', 'lambda', 'vast'];

const record = (value: unknown): UnknownRecord | null =>
  value !== null && typeof value === 'object' && !Array.isArray(value) ? value as UnknownRecord : null;

const pick = (value: UnknownRecord, ...keys: string[]) => {
  for (const key of keys) if (value[key] !== undefined && value[key] !== null) return value[key];
  return undefined;
};

const text = (value: unknown) => typeof value === 'string' ? value.trim() : '';
const positive = (value: unknown) => {
  const parsed = typeof value === 'number' ? value : Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
};
const nonNegative = (value: unknown) => {
  const parsed = typeof value === 'number' ? value : Number(value);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : undefined;
};
const validTime = (value: unknown) => {
  const parsed = text(value);
  return parsed && Number.isFinite(Date.parse(parsed)) ? parsed : '';
};
const httpURL = (value: unknown) => {
  const parsed = text(value);
  try {
    const url = new URL(parsed);
    return url.protocol === 'https:' || url.protocol === 'http:' ? parsed : '';
  } catch {
    return '';
  }
};

export const normalizeGPUPriceQuote = (input: unknown): GPUPriceQuote | null => {
  const value = record(input);
  if (!value) return null;

  const provider = text(pick(value, 'provider'));
  const gpuModel = text(pick(value, 'gpuModel', 'gpu_model'));
  const product = text(pick(value, 'product'));
  const billingMode = text(pick(value, 'billingMode', 'billing_mode'));
  const gpuCount = positive(pick(value, 'gpuCount', 'gpu_count'));
  const currency = text(pick(value, 'currency'));
  const rawPrice = positive(pick(value, 'rawPrice', 'raw_price'));
  const rawUnit = text(pick(value, 'rawUnit', 'raw_unit'));
  const priceUsdPerGpuHour = positive(pick(value, 'priceUsdPerGpuHour', 'price_usd_per_gpu_hour'));
  const sourceUrl = httpURL(pick(value, 'sourceUrl', 'source_url'));
  const observedAt = validTime(pick(value, 'observedAt', 'observed_at'));

  if (!provider || !gpuModel || !product || !billingMode || !gpuCount || !currency || !rawPrice || !rawUnit || !priceUsdPerGpuHour || !sourceUrl || !observedAt) {
    return null;
  }

  const optionalPositive = (camel: string, snake: string) => positive(pick(value, camel, snake));
  const optionalNonNegative = (camel: string, snake: string) => nonNegative(pick(value, camel, snake));
  return {
    provider,
    gpuModel,
    product,
    billingMode,
    gpuCount,
    memoryGiB: optionalPositive('memoryGiB', 'memory_gib'),
    region: text(pick(value, 'region')) || undefined,
    offerId: text(pick(value, 'offerId', 'offer_id')) || undefined,
    availability: text(pick(value, 'availability')) || undefined,
    currency: currency.toUpperCase(),
    rawPrice,
    rawUnit,
    priceUsdPerGpuHour,
    instanceTotalUsdPerHour: optionalPositive('instanceTotalUsdPerHour', 'instance_total_usd_per_hour'),
    storageUsdPerHour: optionalNonNegative('storageUsdPerHour', 'storage_usd_per_hour'),
    bandwidthUpUsdPerTb: optionalNonNegative('bandwidthUpUsdPerTb', 'bandwidth_up_usd_per_tb'),
    bandwidthDownUsdPerTb: optionalNonNegative('bandwidthDownUsdPerTb', 'bandwidth_down_usd_per_tb'),
    sourceUrl,
    observedAt,
  };
};

const normalizeProviderStatus = (input: unknown, providerHint = ''): GPUProviderStatus | null => {
  const value = record(input);
  if (!value) return null;
  const provider = text(pick(value, 'provider')) || providerHint;
  const status = text(pick(value, 'status', 'state'));
  if (!provider || !status) return null;
  return {
    provider,
    status,
    configured: typeof value.configured === 'boolean' ? value.configured : undefined,
    message: text(pick(value, 'message', 'error', 'reason', 'errorCode')) || undefined,
    observedAt: validTime(pick(value, 'observedAt', 'observed_at', 'lastSuccessAt', 'last_success_at')) || undefined,
    lastAttemptAt: validTime(pick(value, 'lastAttemptAt', 'last_attempt_at')) || undefined,
    lastSuccessAt: validTime(pick(value, 'lastSuccessAt', 'last_success_at')) || undefined,
    errorCode: text(pick(value, 'errorCode', 'error_code')) || undefined,
    quoteCount: nonNegative(pick(value, 'quoteCount', 'quote_count', 'count')),
  };
};

const providerRows = (input: GPUPricesResponse) => {
  const source = input.providers ?? input.providerStatuses;
  if (Array.isArray(source)) return source.map((item) => normalizeProviderStatus(item)).filter((item): item is GPUProviderStatus => Boolean(item));
  const map = record(source);
  return map ? Object.entries(map).map(([provider, item]) => normalizeProviderStatus(item, provider)).filter((item): item is GPUProviderStatus => Boolean(item)) : [];
};

export const getGPUUpstreamRefreshAvailability = (
  providers: GPUProviderStatus[],
  meta: APIResponseMeta | null | undefined,
  adminAuthorized: boolean,
  checking = false,
): GPUUpstreamRefreshAvailability => {
  const byProvider = new Map(providers.map((provider) => [provider.provider.toLowerCase(), provider]));
  const allProvidersUnconfigured = DYNAMIC_GPU_PROVIDERS.every((provider) => {
    const status = byProvider.get(provider);
    return status && (status.configured === false || status.status.toLowerCase() === 'unconfigured');
  });
  if (allProvidersUnconfigured) {
    return {
      enabled: false,
      reason: '先在后端配置 provider 凭证；官方核验参考价仍可访问，动态最低价、价差与历史图保持隐藏。',
    };
  }
  if (meta?.refreshable === false) {
    return {
      enabled: false,
      reason: '后端标记当前数据不可刷新；请先在后端配置 provider 凭证或检查刷新能力。官方核验参考价仍可访问。',
    };
  }
  if (checking) return { enabled: false, reason: '正在检查动态数据与管理会话；官方核验参考价仍可访问。' };
  if (!adminAuthorized) {
    return { enabled: false, reason: '上游刷新需要管理会话；当前仍可读取快照与官方核验参考价。' };
  }
  if (!meta || meta.refreshable !== true) {
    return { enabled: false, reason: '后端未确认上游可刷新；当前仍可读取快照与官方核验参考价。' };
  }
  return { enabled: true, reason: '已授权刷新已配置的 provider。' };
};

export const adaptGPUPricesResponse = (
  input: GPUPricesResponse | null | undefined,
  meta: APIResponseMeta | null | undefined,
): AdaptedGPUPrices => {
  const sourceQuotes = Array.isArray(input?.quotes) ? input.quotes : Array.isArray(input?.items) ? input.items : [];
  const validQuotes = sourceQuotes.map(normalizeGPUPriceQuote).filter((item): item is GPUPriceQuote => Boolean(item));
  const declaredCount = typeof input?.count === 'number' ? input.count : sourceQuotes.length;
  const responseState = text(input?.status).toLowerCase();
  const explicitUnavailable = ['stale', 'unavailable', 'error', 'unknown'].includes(responseState);
  const complete = !explicitUnavailable && sourceQuotes.length > 0 && validQuotes.length === sourceQuotes.length && declaredCount === sourceQuotes.length;
  const trust = assessDataTrust(meta, complete);
  return {
    quotes: trust.state === 'live' ? validQuotes : [],
    providers: input ? providerRows(input) : [],
    meta: meta || null,
    trust,
    declaredCount,
    nextRefreshAt: validTime(input?.nextRefreshAt) || '',
	dataMode: text(input?.dataMode),
	testOnly: input?.testOnly === true,
  };
};

export const normalizeGPUHistoryPoint = (input: unknown): GPUPriceHistoryPoint | null => {
  const value = record(input);
  if (!value) return null;
  const provider = text(pick(value, 'provider'));
  const gpuModel = text(pick(value, 'gpuModel', 'gpu_model'));
  const billingMode = text(pick(value, 'billingMode', 'billing_mode'));
  const priceUsdPerGpuHour = positive(pick(value, 'priceUsdPerGpuHour', 'price_usd_per_gpu_hour', 'price'));
  const observedAt = validTime(pick(value, 'observedAt', 'observed_at', 'timestamp'));
  if (!provider || !gpuModel || !billingMode || !priceUsdPerGpuHour || !observedAt) return null;
  return {
    provider,
    gpuModel,
    billingMode,
    priceUsdPerGpuHour,
    observedAt,
    sourceUrl: httpURL(pick(value, 'sourceUrl', 'source_url')) || undefined,
  };
};

export const adaptGPUPriceHistoryResponse = (
  input: GPUPriceHistoryResponse | null | undefined,
  meta: APIResponseMeta | null | undefined,
): AdaptedGPUPriceHistory => {
  const sourcePoints = Array.isArray(input?.points) ? input.points : Array.isArray(input?.items) ? input.items : Array.isArray(input?.history) ? input.history : [];
  const validPoints = sourcePoints.map(normalizeGPUHistoryPoint).filter((item): item is GPUPriceHistoryPoint => Boolean(item));
  const declaredCount = typeof input?.count === 'number' ? input.count : sourcePoints.length;
  const responseState = text(input?.status).toLowerCase();
  const unavailableState = ['stale', 'unavailable', 'error', 'unknown'].includes(responseState);
  const authenticatedHistoryIsEmpty = meta?.staleReason?.toLowerCase().includes('no authenticated observations') === true;
  const empty = Boolean(input) && sourcePoints.length === 0 && declaredCount === 0 && (!meta?.stale || authenticatedHistoryIsEmpty);
  const complete = !unavailableState && sourcePoints.length > 0 && validPoints.length === sourcePoints.length && declaredCount === sourcePoints.length;
  const trust = assessDataTrust(meta, complete);
  return {
    points: trust.state === 'live' ? validPoints.sort((a, b) => Date.parse(a.observedAt) - Date.parse(b.observedAt)) : [],
    meta: meta || null,
    trust,
    declaredCount,
    empty,
	dataMode: text(input?.dataMode),
	testOnly: input?.testOnly === true,
  };
};

const normalizeReferenceProvider = (input: unknown): GPUReferenceProvider | null => {
  const value = record(input);
  if (!value) return null;
  const provider = text(value.provider).toLowerCase();
  const status = text(value.status);
  const sourceUrl = httpURL(pick(value, 'sourceUrl', 'source_url'));
  if (!provider || !status || !sourceUrl || typeof value.hasFixedReference !== 'boolean') return null;
  return {
    provider,
    status,
    sourceUrl,
    note: text(value.note) || undefined,
    hasFixedReference: value.hasFixedReference,
  };
};

export const adaptGPUPriceReferenceResponse = (
  input: GPUPriceReferenceResponse | null | undefined,
  meta: APIResponseMeta | null | undefined,
): AdaptedGPUPriceReference => {
  const verifiedAt = validTime(input?.verifiedAt);
  const providers = Array.isArray(input?.providers)
    ? input.providers.map(normalizeReferenceProvider).filter((item): item is GPUReferenceProvider => Boolean(item))
    : [];
  const providerSources = new Map(providers.map((provider) => [provider.provider, provider.sourceUrl]));
  const sourceItems = Array.isArray(input?.items) ? input.items : [];
  const items = sourceItems.map((inputItem): GPUReferencePrice | null => {
    const value = record(inputItem);
    if (!value) return null;
    const provider = text(value.provider).toLowerCase();
    const gpuModel = text(pick(value, 'gpuModel', 'gpu_model'));
    const product = text(value.product);
    const billingMode = text(pick(value, 'billingMode', 'billing_mode'));
    const gpuCount = positive(pick(value, 'gpuCount', 'gpu_count'));
    const currency = text(value.currency).toUpperCase();
    const rawPrice = positive(pick(value, 'rawPrice', 'raw_price'));
    const rawUnit = text(pick(value, 'rawUnit', 'raw_unit'));
    const priceUsdPerGpuHour = positive(pick(value, 'priceUsdPerGpuHour', 'price_usd_per_gpu_hour'));
    const sourceUrl = providerSources.get(provider) || '';
    if (!provider || provider === 'vast' || !gpuModel || !product || !billingMode || !gpuCount || currency !== 'USD' || !rawPrice || !rawUnit || !priceUsdPerGpuHour || !sourceUrl || !verifiedAt) return null;
    return {
      provider, gpuModel, product, billingMode, gpuCount, currency, rawPrice, rawUnit,
      priceUsdPerGpuHour,
      instanceTotalUsdPerHour: positive(pick(value, 'instanceTotalUsdPerHour', 'instance_total_usd_per_hour')),
      sourceUrl,
      verifiedAt,
    };
  }).filter((item): item is GPUReferencePrice => Boolean(item));
  const declaredCount = typeof input?.count === 'number' ? input.count : sourceItems.length;
  const referenceMode = [input?.status, input?.dataMode].map((value) => text(value).toLowerCase());
  const modeIsReference = referenceMode.some((value) => value === 'historical' || value === 'stale');
  const providerContractValid = providers.length > 0 && providers.length === (input?.providers?.length || 0) &&
    providers.some((provider) => provider.provider === 'vast' && !provider.hasFixedReference);
  const available = modeIsReference && Boolean(verifiedAt) && sourceItems.length > 0 && items.length === sourceItems.length && declaredCount === sourceItems.length && providerContractValid;
  return {
    items: available ? items : [],
    providers,
    verifiedAt,
    disclaimer: text(input?.disclaimer),
    source: text(input?.source) || meta?.source || '',
    available,
    reason: available ? '' : '官方核验参考价响应不完整',
    meta: meta || null,
  };
};
