import { create } from 'zustand';
import { ETFSector, ETFCategory, ETFSortBy } from '../types/etf';
import { APIError, get as apiGet } from '../utils/api';

export type ETFDataMode = 'current' | 'historical';

export interface ETFRefreshDiagnostics {
  ageSeconds: number;
  maxAgeSeconds: number;
  lastAttemptAt?: string;
  nextScheduledAt?: string;
  lastAttemptStatus?: string;
  lastAttemptError?: string;
  scopeRequested?: number;
  scopeCompleted?: number;
}

export interface EtfMember {
  sym: string;
  name: string;
  aum?: number;
  ret1y?: number | null;
  ret5y?: number | null;
  mdd?: number | null;
  sector: string;
  kind: string;
  expense?: number;
  verdict?: string;
}

interface ETFStore {
  sectors: ETFSector[];
  allSectors: ETFSector[];
  etfMembers: EtfMember[];
  categories: Map<ETFCategory, number>;
  loading: boolean;
  error: string | null;
  degradedReason: string | null;
  dataMode: ETFDataMode;
  dataUpdated: string;
  metadataAsOf: string;
  diagnostics: ETFRefreshDiagnostics | null;
  lastFetchedAt: number;
  activeCategory: ETFCategory | 'all';
  sortBy: ETFSortBy;
  searchQuery: string;
  fetchSectors: (force?: boolean) => Promise<void>;
  setDataMode: (mode: ETFDataMode) => void;
  setActiveCategory: (category: ETFCategory | 'all') => void;
  setSortBy: (sortBy: ETFSortBy) => void;
  setSearchQuery: (query: string) => void;
  getMembersForSector: (sectorName: string) => EtfMember[];
}

interface ETFSectorsResponse {
  sectors: ETFSector[];
  total: number;
  categories: Record<ETFCategory, number>;
  etfs?: EtfMember[];
  updated?: string;
  metadataAsOf?: string;
  dataMode?: ETFDataMode;
  degradedReason?: string;
  aumUnit?: 'usd';
  diagnostics?: ETFRefreshDiagnostics;
}

const ETF_CACHE_TTL_MS = 60_000;

function applyFilters(
  all: ETFSector[],
  activeCategory: ETFCategory | 'all',
  sortBy: ETFSortBy,
  searchQuery: string
): ETFSector[] {
  let list = all;
  if (activeCategory !== 'all') {
    list = list.filter((s) => s.category === activeCategory);
  }
  const q = searchQuery.trim().toLowerCase();
  if (q) {
    list = list.filter(
      (s) =>
        s.name.toLowerCase().includes(q) ||
        s.topPerformer.ticker.toLowerCase().includes(q) ||
        s.id.includes(q)
    );
  }

  return [...list].sort((a, b) => {
    if (sortBy === 'return1y') return (b.return1y || 0) - (a.return1y || 0);
    if (sortBy === 'return5y') return (b.return5y || 0) - (a.return5y || 0);
    if (sortBy === 'drawdown') return (b.maxDrawdown || 0) - (a.maxDrawdown || 0);
    return b.aum - a.aum;
  });
}

export const useETFStore = create<ETFStore>((set, getState) => ({
  sectors: [],
  allSectors: [],
  etfMembers: [],
  categories: new Map(),
  loading: false,
  error: null,
  degradedReason: null,
  dataMode: 'current',
  dataUpdated: '',
  metadataAsOf: '',
  diagnostics: null,
  lastFetchedAt: 0,
  activeCategory: 'all',
  sortBy: 'aum',
  searchQuery: '',

  fetchSectors: async (force = false) => {
    const { activeCategory, sortBy, searchQuery, allSectors, lastFetchedAt, dataMode } = getState();
    set({ loading: true, error: null });

    try {
      let source = allSectors;
      let categories = getState().categories;
      let etfMembers = getState().etfMembers;
      const shouldFetch = force || source.length === 0 || Date.now() - lastFetchedAt > ETF_CACHE_TTL_MS;

      if (shouldFetch) {
        const response = await apiGet<ETFSectorsResponse>(`/api/etf/sectors?sort=${sortBy}&mode=${dataMode}`);
        source = response.sectors;
        categories = new Map(Object.entries(response.categories) as [ETFCategory, number][]);
        etfMembers = response.etfs || [];
        set({
          dataMode: response.dataMode || dataMode,
          dataUpdated: response.updated || '',
          metadataAsOf: response.metadataAsOf || '',
          degradedReason: response.degradedReason || null,
          diagnostics: response.diagnostics || null,
        });
      }

      set({
        allSectors: source,
        etfMembers,
        categories,
        sectors: applyFilters(source, activeCategory, sortBy, searchQuery),
        lastFetchedAt: Date.now(),
        loading: false,
        error: null,
      });
    } catch (error) {
      const unavailable = error instanceof APIError ? error.data as Partial<ETFSectorsResponse> | undefined : undefined;
      set({
        error: error instanceof Error ? error.message : 'Failed to load ETF dataset',
        allSectors: [],
        etfMembers: [],
        categories: new Map(),
        sectors: [],
        loading: false,
        degradedReason: error instanceof Error ? error.message : 'ETF dataset unavailable',
        dataUpdated: unavailable?.updated || '',
        metadataAsOf: unavailable?.metadataAsOf || '',
        diagnostics: unavailable?.diagnostics || null,
        lastFetchedAt: 0,
      });
    }
  },

  setDataMode: (dataMode) => {
    set({ dataMode, lastFetchedAt: 0, error: null, degradedReason: null, diagnostics: null, metadataAsOf: '' });
    void getState().fetchSectors(true);
  },

  setActiveCategory: (category) => {
    set({ activeCategory: category });
    getState().fetchSectors();
  },

  setSortBy: (sortBy) => {
    set({ sortBy });
    getState().fetchSectors();
  },

  setSearchQuery: (query) => {
    set({ searchQuery: query });
  },

  getMembersForSector: (sectorName: string) => {
    return getState()
      .etfMembers.filter((e) => e.sector === sectorName)
      .sort((a, b) => (b.aum || 0) - (a.aum || 0));
  },
}));
