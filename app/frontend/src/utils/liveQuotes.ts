import { get } from './api';
import type { Stock } from '../types/stocks';

export type LiveQuote = {
  price: number;
  pct: number;
  session?: string;
  prevClose?: number;
  source?: string;
};

export type QuoteMap = Record<string, LiveQuote>;

const CHUNK = 80;

/** Batch-fetch live quotes from /api/quote (TradingView/Yahoo). */
export async function fetchLiveQuotes(symbols: string[]): Promise<QuoteMap> {
  const uniq = [...new Set(symbols.map((s) => s.trim().toUpperCase()).filter(Boolean))];
  if (!uniq.length) return {};

  const out: QuoteMap = {};
  for (let i = 0; i < uniq.length; i += CHUNK) {
    const chunk = uniq.slice(i, i + CHUNK);
    try {
      const resp = await get<{ quotes?: QuoteMap }>(`/api/quote?syms=${encodeURIComponent(chunk.join(','))}`);
      const quotes = resp?.quotes || {};
      for (const [sym, q] of Object.entries(quotes)) {
        if (!q || !(q.price > 0)) continue;
        out[sym.toUpperCase()] = q;
      }
    } catch {
      /* keep partial */
    }
  }
  return out;
}

export function applyQuotesToStocks(stocks: Stock[], quotes: QuoteMap): Stock[] {
  if (!stocks.length || !Object.keys(quotes).length) return stocks;
  return stocks.map((s) => {
    const q = quotes[s.symbol.toUpperCase()];
    if (!q) return s;
    const price = q.price || s.price;
    const changePercent = q.pct ?? s.changePercent;
    return {
      ...s,
      price,
      changePercent,
      change: Number(((price * changePercent) / 100).toFixed(4)),
    };
  });
}

/** Poll helper — calls fn immediately, then every intervalMs while visible. */
export function startQuotePolling(fn: () => void | Promise<void>, intervalMs = 20_000): () => void {
  let cancelled = false;
  const tick = async () => {
    if (cancelled || document.hidden) return;
    try {
      await fn();
    } catch {
      /* ignore */
    }
  };
  void tick();
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
