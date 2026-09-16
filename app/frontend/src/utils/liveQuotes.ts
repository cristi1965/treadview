import { getWithMeta, type APIResponseMeta } from './api';
import type { Stock } from '../types/stocks';

export type LiveQuote = {
  price: number;
  pct: number;
  session?: string;
  prevClose?: number;
  source?: string;
  dataTime?: string;
  timeGranularity?: string;
  sourceSession?: string;
  providerURL?: string;
  providerMode?: string;
};

export type QuoteMap = Record<string, LiveQuote>;

export interface QuoteResult {
  quotes: QuoteMap;
  meta: APIResponseMeta;
  missing: string[];
  errors: string[];
}

const emptyMeta = (): APIResponseMeta => ({
  source: '',
  dataTime: '',
  refreshedAt: '',
  stale: false,
  staleReason: '',
  refreshable: false,
  partialErrors: [],
});

const CHUNK = 80;

const mergeMeta = (current: APIResponseMeta, incoming: APIResponseMeta): APIResponseMeta => {
  const sources = [...new Set([current.source, incoming.source].filter(Boolean))];
  const times = [current.dataTime, incoming.dataTime]
    .filter((value): value is string => Boolean(value && Number.isFinite(Date.parse(value))))
    .sort((a, b) => Date.parse(a) - Date.parse(b));
  const reasons = [...new Set([current.staleReason, incoming.staleReason].filter(Boolean))];
  return {
    source: sources.join('+'),
    dataTime: times[0] || current.dataTime || incoming.dataTime,
    refreshedAt: incoming.refreshedAt || current.refreshedAt,
    stale: current.stale || incoming.stale,
    staleReason: reasons.join('; '),
    refreshable: current.refreshable || incoming.refreshable,
    partialErrors: [...new Set([...current.partialErrors, ...incoming.partialErrors])],
  };
};

/** Batch-fetch quotes from the backend provider chain. */
export async function fetchLiveQuotes(symbols: string[]): Promise<QuoteMap> {
  return (await fetchQuoteResult(symbols)).quotes;
}

export async function fetchQuoteResult(symbols: string[]): Promise<QuoteResult> {
  const uniq = [...new Set(symbols.map((s) => s.trim().toUpperCase()).filter(Boolean))];
  if (!uniq.length) return { quotes: {}, meta: emptyMeta(), missing: [], errors: [] };

  const out: QuoteMap = {};
  const missing = new Set<string>();
  const errors: string[] = [];
  let meta = emptyMeta();
  for (let i = 0; i < uniq.length; i += CHUNK) {
    const chunk = uniq.slice(i, i + CHUNK);
    try {
      const result = await getWithMeta<{ quotes?: QuoteMap; missing?: string[] }>(`/api/quote?syms=${encodeURIComponent(chunk.join(','))}`);
      const quotes = result.data?.quotes || {};
      meta = mergeMeta(meta, result.meta);
      for (const sym of result.data?.missing || []) missing.add(sym.toUpperCase());
      errors.push(...result.meta.partialErrors);
      for (const [sym, q] of Object.entries(quotes)) {
        if (!q || !(q.price > 0)) {
          missing.add(sym.toUpperCase());
          continue;
        }
        out[sym.toUpperCase()] = q;
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : 'quote request failed';
      errors.push(message);
      chunk.forEach((sym) => missing.add(sym));
    }
  }
  for (const sym of uniq) {
    if (!out[sym]) missing.add(sym);
  }
  return { quotes: out, meta, missing: [...missing], errors: [...new Set(errors)] };
}

export function applyQuotesToStocks(stocks: Stock[], quotes: QuoteMap): Stock[] {
  if (!stocks.length || !Object.keys(quotes).length) return stocks;
  return stocks.map((s) => {
    const q = quotes[s.symbol.toUpperCase()];
    if (!q) return s;
    const price = q.price || s.price;
    const changePercent = q.pct ?? s.changePercent;
    const prevClose = q.prevClose || (changePercent !== -100 ? price / (1 + changePercent / 100) : price);
    return {
      ...s,
      price,
      changePercent,
      change: Number((price - prevClose).toFixed(4)),
    };
  });
}

/** Shared quote poller: one in-flight request, paused while hidden, refreshed on resume. */
export function startQuotePolling(
  fn: () => void | Promise<void>,
  intervalMs = 20_000,
  options: { immediate?: boolean } = {}
): () => void {
  let cancelled = false;
  let inFlight = false;
  const tick = async () => {
    if (cancelled || document.hidden || inFlight) return;
    inFlight = true;
    try {
      await fn();
    } catch {
      /* ignore */
    } finally {
      inFlight = false;
    }
  };
  if (options.immediate !== false) void tick();
  const id = window.setInterval(() => void tick(), intervalMs);
  const onVis = () => {
    if (!document.hidden) void tick();
  };
  document.addEventListener('visibilitychange', onVis);
  return () => {
    cancelled = true;
    window.clearInterval(id);
    document.removeEventListener('visibilitychange', onVis);
  };
}

export function slugifyPolitician(name: string): string {
  return name
    .normalize('NFKD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 80);
}
