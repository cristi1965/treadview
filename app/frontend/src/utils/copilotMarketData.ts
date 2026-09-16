import type { APIResponseMeta } from './api';
import { assessDataTrust } from './dataTrust';

export interface TrustedCopilotMarketData {
  price?: number;
  pct?: number;
  premiumPct?: number;
  nav?: number;
  currency?: string;
  level?: string;
  meta: APIResponseMeta;
}

const finite = (value: unknown): value is number => typeof value === 'number' && Number.isFinite(value);

const isCompleteMeta = (value: unknown): value is APIResponseMeta => {
  if (!value || typeof value !== 'object') return false;
  const meta = value as Record<string, unknown>;
  return typeof meta.source === 'string'
    && typeof meta.dataTime === 'string'
    && typeof meta.refreshedAt === 'string'
    && Number.isFinite(Date.parse(meta.refreshedAt))
    && typeof meta.stale === 'boolean'
    && typeof meta.staleReason === 'string'
    && typeof meta.refreshable === 'boolean'
    && Array.isArray(meta.partialErrors)
    && meta.partialErrors.every((item) => typeof item === 'string');
};

export const normalizeCopilotMarketData = (value: unknown): TrustedCopilotMarketData | undefined => {
  if (!value || typeof value !== 'object') return undefined;
  const candidate = value as Record<string, unknown>;
  const meta = candidate.meta;
  const hasMarketFact = finite(candidate.price) || finite(candidate.premiumPct) || finite(candidate.nav);
  if (!isCompleteMeta(meta) || assessDataTrust(meta, hasMarketFact).state !== 'live') return undefined;
  return {
    ...(finite(candidate.price) ? { price: candidate.price } : {}),
    ...(finite(candidate.pct) ? { pct: candidate.pct } : {}),
    ...(finite(candidate.premiumPct) ? { premiumPct: candidate.premiumPct } : {}),
    ...(finite(candidate.nav) ? { nav: candidate.nav } : {}),
    ...(typeof candidate.currency === 'string' ? { currency: candidate.currency } : {}),
    ...(typeof candidate.level === 'string' ? { level: candidate.level } : {}),
    meta,
  };
};
