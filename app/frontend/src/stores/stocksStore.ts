import { create } from 'zustand';
import { Stock, StockSortBy, SortOrder } from '../types/stocks';
import { get as apiGet } from '../utils/api';
import { filterSortPageStocks, loadUsMarketStocks, loadAMarketStocks } from '../utils/stockgodData';
import { applyQuotesToStocks, fetchLiveQuotes } from '../utils/liveQuotes';
import { usePortfolioStore } from './portfolioStore';

interface StocksStore {
  stocks: Stock[];
  allUsStocks: Stock[];
  allCnStocks: Stock[];
  loading: boolean;
  error: string | null;
  total: number;
  page: number;
  limit: number;
  hasMore: boolean;
  upCount: number;
  downCount: number;
  judgedCount: number;
  highScoreCount: number;
  consensusCount: number;
  divergenceCount: number;
  dilutionCount: number;
  lastQuoteAt: number;

  market: string;
  sortBy: StockSortBy;
  order: SortOrder;
  searchQuery: string;

  fetchStocks: () => Promise<void>;
  refreshQuotes: () => Promise<void>;
  setMarket: (market: string) => void;
  setSortBy: (sortBy: StockSortBy) => void;
  setOrder: (order: SortOrder) => void;
  setSearchQuery: (query: string) => void;
  setPage: (page: number) => void;
  toggleWatch: (symbol: string) => void;
}

interface StocksResponse {
  stocks: Stock[];
  total: number;
  page: number;
  limit: number;
  hasMore: boolean;
}

function markWatched(stocks: Stock[]): Stock[] {
  const portfolio = usePortfolioStore.getState();
  return stocks.map((stock) => ({
    ...stock,
    isWatched: portfolio.isInWatchlist(stock.symbol) || stock.isWatched,
  }));
}

function computeStats(all: Stock[]) {
  const upCount = all.filter((s) => s.changePercent > 0).length;
  const downCount = all.filter((s) => s.changePercent < 0).length;
  const judged = all.filter((s) => s.judged || s.avgScore > 0);
  const highScoreCount = judged.filter((s) => s.avgScore >= 65).length;
  const consensusCount = judged.filter((s) => {
    const values = Object.values(s.scores);
    return s.avgScore >= 60 && values.filter((v) => v >= 60).length >= 4;
  }).length;
  const divergenceCount = judged.filter((s) => (s.divergence ?? 0) >= 30).length;
  const dilutionCount = all.filter((s) => s.hasDilution).length;
  return {
    upCount,
    downCount,
    judgedCount: judged.length,
    highScoreCount,
    consensusCount,
    divergenceCount,
    dilutionCount,
  };
}

export const useStocksStore = create<StocksStore>((set, getState) => ({
  stocks: [],
  allUsStocks: [],
  allCnStocks: [],
  loading: false,
  error: null,
  total: 0,
  page: 1,
  limit: 50,
  hasMore: false,
  upCount: 0,
  downCount: 0,
  judgedCount: 0,
  highScoreCount: 0,
  consensusCount: 0,
  divergenceCount: 0,
  dilutionCount: 0,
  lastQuoteAt: 0,
  market: 'us',
  sortBy: 'marketcap',
  order: 'desc',
  searchQuery: '',

  fetchStocks: async () => {
    const { market, sortBy, order, page, limit, searchQuery, allUsStocks, allCnStocks } = getState();
    set({ loading: true, error: null });

    try {
      if (market === 'us') {
        const all = allUsStocks.length > 0 ? allUsStocks : await loadUsMarketStocks();
        const pageResult = filterSortPageStocks(all, { searchQuery, sortBy, order, page, limit });
        const stats = computeStats(all);
        set({
          allUsStocks: all,
          stocks: markWatched(pageResult.stocks),
          total: pageResult.total,
          hasMore: pageResult.hasMore,
          loading: false,
          ...stats,
        });
        void getState().refreshQuotes();
        return;
      }

      if (market === 'cn') {
        const all = allCnStocks.length > 0 ? allCnStocks : await loadAMarketStocks();
        const pageResult = filterSortPageStocks(all, { searchQuery, sortBy, order, page, limit });
        const stats = computeStats(all);
        set({
          allCnStocks: all,
          stocks: markWatched(pageResult.stocks),
          total: pageResult.total,
          hasMore: pageResult.hasMore,
          loading: false,
          ...stats,
        });
        void getState().refreshQuotes();
        return;
      }

      const params = new URLSearchParams({
        market,
        sort: sortBy,
        order,
        page: page.toString(),
        limit: limit.toString(),
      });
      if (searchQuery) params.append('q', searchQuery);

      const response = await apiGet<StocksResponse>(`/api/stocks?${params.toString()}`);
      set({
        stocks: markWatched(response.stocks),
        total: response.total,
        hasMore: response.hasMore,
        loading: false,
        upCount: 0,
        downCount: 0,
        judgedCount: 0,
        highScoreCount: 0,
        consensusCount: 0,
        divergenceCount: 0,
        dilutionCount: 0,
      });
    } catch (error) {
      console.error('Failed to fetch stocks:', error);
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch stocks',
        loading: false,
      });
    }
  },

  refreshQuotes: async () => {
    const { stocks, allUsStocks, allCnStocks, market } = getState();
    const pageSyms = stocks.map((s) => s.symbol);
    // Also refresh top universe slice so subsequent sorts stay fresher
    const universe = market === 'cn' ? allCnStocks : allUsStocks;
    const topSyms = universe.slice(0, 120).map((s) => s.symbol);
    const quotes = await fetchLiveQuotes([...pageSyms, ...topSyms]);
    if (!Object.keys(quotes).length) return;

    set((state) => {
      const nextUs = applyQuotesToStocks(state.allUsStocks, quotes);
      const nextCn = applyQuotesToStocks(state.allCnStocks, quotes);
      const nextPage = applyQuotesToStocks(state.stocks, quotes);
      const statsSource = state.market === 'cn' ? nextCn : nextUs;
      return {
        allUsStocks: nextUs,
        allCnStocks: nextCn,
        stocks: markWatched(nextPage),
        lastQuoteAt: Date.now(),
        ...computeStats(statsSource.length ? statsSource : nextPage),
      };
    });
  },

  setMarket: (market) => {
    set({ market, page: 1 });
    getState().fetchStocks();
  },

  setSortBy: (sortBy) => {
    set({ sortBy, page: 1 });
    getState().fetchStocks();
  },

  setOrder: (order) => {
    set({ order, page: 1 });
    getState().fetchStocks();
  },

  setSearchQuery: (query) => {
    set({ searchQuery: query, page: 1 });
  },

  setPage: (page) => {
    set({ page });
    getState().fetchStocks();
  },

  toggleWatch: (symbol) => {
    const stock =
      getState().stocks.find((item) => item.symbol === symbol) ||
      getState().allUsStocks.find((item) => item.symbol === symbol) ||
      getState().allCnStocks.find((item) => item.symbol === symbol);
    const portfolio = usePortfolioStore.getState();
    if (stock) {
      if (portfolio.isInWatchlist(symbol) || stock.isWatched) {
        portfolio.removeFromWatchlist(symbol);
      } else {
        portfolio.addToWatchlist(symbol, stock.name);
      }
    }

    set((state) => ({
      stocks: state.stocks.map((s) => (s.symbol === symbol ? { ...s, isWatched: !s.isWatched } : s)),
      allUsStocks: state.allUsStocks.map((s) =>
        s.symbol === symbol ? { ...s, isWatched: !s.isWatched } : s
      ),
      allCnStocks: state.allCnStocks.map((s) =>
        s.symbol === symbol ? { ...s, isWatched: !s.isWatched } : s
      ),
    }));
  },
}));
