import { create } from 'zustand';
import { ETFSector, ETFCategory, ETFSortBy } from '../types/etf';
import { get as apiGet } from '../utils/api';
import { loadEtfAnalyses } from '../utils/stockgodData';

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
  activeCategory: ETFCategory | 'all';
  sortBy: ETFSortBy;
  searchQuery: string;
  fetchSectors: () => Promise<void>;
  setActiveCategory: (category: ETFCategory | 'all') => void;
  setSortBy: (sortBy: ETFSortBy) => void;
  setSearchQuery: (query: string) => void;
  getMembersForSector: (sectorName: string) => EtfMember[];
}

interface ETFSectorsResponse {
  sectors: ETFSector[];
  total: number;
  categories: Record<ETFCategory, number>;
}

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
  activeCategory: 'all',
  sortBy: 'aum',
  searchQuery: '',

  fetchSectors: async () => {
    const { activeCategory, sortBy, searchQuery, allSectors } = getState();
    set({ loading: true, error: null });

    try {
      let source = allSectors;
      let categories = getState().categories;
      let etfMembers = getState().etfMembers;

      if (source.length === 0) {
        try {
          const data = await loadEtfAnalyses();
          source = data.sectors;
          categories = new Map(Object.entries(data.categories) as [ETFCategory, number][]);
          etfMembers = data.etfs as EtfMember[];
        } catch (dataErr) {
          console.warn('ETF /data load failed, falling back to API', dataErr);
          const params = new URLSearchParams();
          if (activeCategory !== 'all') params.append('category', activeCategory);
          params.append('sort', sortBy);
          if (searchQuery) params.append('q', searchQuery);
          const response = await apiGet<ETFSectorsResponse>(`/api/etf/sectors?${params.toString()}`);
          set({
            sectors: response.sectors,
            allSectors: response.sectors,
            categories: new Map(Object.entries(response.categories) as [ETFCategory, number][]),
            loading: false,
          });
          return;
        }
      }

      set({
        allSectors: source,
        etfMembers,
        categories,
        sectors: applyFilters(source, activeCategory, sortBy, searchQuery),
        loading: false,
      });
    } catch (error) {
      console.error('Failed to fetch ETF sectors:', error);
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch ETF sectors',
        loading: false,
      });
    }
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
