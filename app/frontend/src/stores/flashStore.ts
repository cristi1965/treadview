import { create } from 'zustand';
import { FlashItem, FlashKind, FlashResponse } from '../types/reports';
import { getWithMeta } from '../utils/api';

export type ImportanceFilter = 'all' | 'important';
export type KindFilter = 'all' | Exclude<FlashKind, 'calendar'>;

const POLL_MS = 60_000;
const HIGHLIGHT_MS = 6_000;
const KINDS: FlashKind[] = ['macro', 'earnings', 'company', 'market', 'calendar'];

interface FlashStore {
  items: FlashItem[];
  updatedAt: string;
  source: string;
  staleReason: string;
  lastUpdated: number | null;
  loading: boolean;
  error: string | null;
  /** true when the feed is degraded to non-live data */
  degraded: boolean;
  paused: boolean;
  importance: ImportanceFilter;
  kind: KindFilter;
  newIds: Set<string>;

  fetchFlash: (silent?: boolean) => Promise<void>;
  startPolling: () => void;
  stopPolling: () => void;
  setPaused: (paused: boolean) => void;
  setImportance: (importance: ImportanceFilter) => void;
  setKind: (kind: KindFilter) => void;
}

const asString = (value: unknown): string => (typeof value === 'string' ? value : '');

const asKind = (value: unknown): FlashKind =>
  KINDS.includes(value as FlashKind) ? (value as FlashKind) : 'market';

const asImportance = (value: unknown): number => {
  const n = Math.round(Number(value));
  if (!Number.isFinite(n)) return 1;
  return Math.min(3, Math.max(1, n));
};

/** Tolerant of a missing/partial payload so a half-baked snapshot never blanks the page. */
const normalize = (raw: unknown): FlashItem[] => {
  const list: unknown[] = Array.isArray(raw)
    ? raw
    : Array.isArray((raw as { items?: unknown[] } | null)?.items)
      ? ((raw as { items: unknown[] }).items)
      : [];

  const seen = new Set<string>();
  const items: FlashItem[] = [];

  list.forEach((entry, index) => {
    if (!entry || typeof entry !== 'object') return;
    const row = entry as Record<string, unknown>;
    const title = asString(row.title) || asString(row.titleEn);
    if (!title) return;
    const id = asString(row.id) || `${asString(row.time)}-${index}`;
    if (seen.has(id)) return;
    seen.add(id);
    const tickers = Array.isArray(row.tickers)
      ? row.tickers.filter((t): t is string => typeof t === 'string' && t.length > 0)
      : undefined;
    items.push({
      id,
      time: asString(row.time),
      title,
      titleEn: asString(row.titleEn) || undefined,
      body: asString(row.body) || undefined,
      bodyEn: asString(row.bodyEn) || undefined,
      importance: asImportance(row.importance),
      kind: asKind(row.kind),
      tickers: tickers && tickers.length > 0 ? tickers : undefined,
      source: asString(row.source) || undefined,
      link: asString(row.link) || undefined,
    });
  });

  return items.sort((a, b) => (a.time < b.time ? 1 : a.time > b.time ? -1 : 0));
};

let pollTimer: ReturnType<typeof setInterval> | null = null;
let highlightTimer: ReturnType<typeof setTimeout> | null = null;

export const useFlashStore = create<FlashStore>((set, getState) => ({
  items: [],
  updatedAt: '',
  source: '',
  staleReason: '',
  lastUpdated: null,
  loading: false,
  error: null,
  degraded: false,
  paused: false,
  importance: 'all',
  kind: 'all',
  newIds: new Set(),

  fetchFlash: async (silent = false) => {
    const { items: previous, importance, kind } = getState();
    if (!silent) set({ loading: true });

    const params = new URLSearchParams({ limit: '60', importance });
    if (kind !== 'all') params.set('kind', kind);

    let next: FlashItem[] | null = null;
    let updatedAt = '';
    let failure = '';

    try {
      const response = await getWithMeta<FlashResponse>(`/api/flash?${params.toString()}`);
      next = normalize(response.data);
      updatedAt = asString(response.data?.updatedAt) || response.meta.dataTime;
      set({
        source: response.meta.source,
        staleReason: response.meta.staleReason,
        degraded: response.meta.stale,
      });
    } catch (error) {
      failure = error instanceof Error ? error.message : 'flash request failed';
    }

    if (!next) {
      set({
        items: [],
        updatedAt: '',
        source: '',
        staleReason: failure || 'flash unavailable',
        lastUpdated: null,
        loading: false,
        error: failure || 'flash unavailable',
        degraded: true,
        newIds: new Set(),
      });
      return;
    }

    const previousIds = new Set(previous.map((item) => item.id));
    const newIds =
      previous.length > 0
        ? new Set(next.filter((item) => !previousIds.has(item.id)).map((item) => item.id))
        : new Set<string>();

    set({
      items: next,
      updatedAt,
      lastUpdated: Date.now(),
      loading: false,
      error: null,
      newIds,
    });

    if (highlightTimer) clearTimeout(highlightTimer);
    if (newIds.size > 0) {
      highlightTimer = setTimeout(() => set({ newIds: new Set() }), HIGHLIGHT_MS);
    }
  },

  startPolling: () => {
    if (pollTimer) return;
    pollTimer = setInterval(() => {
      if (getState().paused) return;
      void getState().fetchFlash(true);
    }, POLL_MS);
  },

  stopPolling: () => {
    if (pollTimer) clearInterval(pollTimer);
    pollTimer = null;
    if (highlightTimer) clearTimeout(highlightTimer);
    highlightTimer = null;
  },

  setPaused: (paused) => {
    set({ paused });
    if (!paused) void getState().fetchFlash(true);
  },

  setImportance: (importance) => {
    set({ importance });
    void getState().fetchFlash(true);
  },

  setKind: (kind) => {
    set({ kind });
    void getState().fetchFlash(true);
  },
}));
