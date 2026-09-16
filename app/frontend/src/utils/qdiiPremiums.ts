import { assessDataTrust, type DataTrustAssessment } from './dataTrust';
import type { APIResponseMeta } from './api';

export interface QDIIPremiumItem {
  code: string;
  name: string;
  nav: number | null;
  navDate: string;
  price: number | null;
  pricePct: number | null;
  premiumPct: number | null;
  priceObservedAt?: string;
  priceSource?: string;
  level: 'safe' | 'caution' | 'danger' | 'extreme' | 'unknown';
  index: 'nasdaq' | 'sp500' | 'dow' | 'other';
  status: 'complete' | 'unknown';
  statusReason?: string;
}

export interface QDIIPremiumResponse {
  items: QDIIPremiumItem[];
  count: number;
  status?: 'live' | 'stale' | 'unknown';
}

export type CompleteQDIIPremiumItem = QDIIPremiumItem & {
  nav: number;
  price: number;
  premiumPct: number;
  level: 'safe' | 'caution' | 'danger' | 'extreme';
  status: 'complete';
};

export interface TrustedQDIISelection {
  items: CompleteQDIIPremiumItem[];
  trust: DataTrustAssessment;
}

export const isCompleteQDIIItem = (item: QDIIPremiumItem): item is CompleteQDIIPremiumItem =>
  item.status === 'complete' &&
  Number.isFinite(item.nav) && Number(item.nav) > 0 &&
  Number.isFinite(item.price) && Number(item.price) > 0 &&
  Number.isFinite(item.premiumPct) && item.level !== 'unknown' &&
  Boolean(item.navDate && item.priceObservedAt && item.priceSource);

export const selectTrustedQDIIItems = (
  response: QDIIPremiumResponse | null | undefined,
  meta: APIResponseMeta | null | undefined,
): TrustedQDIISelection => {
  const sourceItems = Array.isArray(response?.items) ? response.items : [];
  const completeItems = sourceItems.filter(isCompleteQDIIItem);
  const responseComplete = response?.status === 'live' &&
    response.count === sourceItems.length &&
    sourceItems.length > 0 &&
    completeItems.length === sourceItems.length;
  const trust = assessDataTrust(meta, responseComplete);
  return { items: trust.state === 'live' ? completeItems : [], trust };
};
