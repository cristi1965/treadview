import { create } from 'zustand';
import { Stock, StockSortBy, SortOrder } from '../types/stocks';
import { get as apiGet } from '../utils/api';
import { filterSortPageStocks, loadUsMarketStocks, loadAMarketStocks, invalidateUsStocksCache } from '../utils/stockgodData';
import { applyQuotesToStocks, fetchQuoteResult } from '../utils/liveQuotes';
import { usePortfolioStore } from './portfolioStore';

interface StocksStore {
  stocks: Stock[];
  baseUsStocks: Stock[];
  baseCnStocks: Stock[];
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
  quoteState: 'loading' | 'live' | 'stale' | 'unavailable' | 'error';
  quoteSource: string;
  quoteDataTime: string;
  quoteMessage: string;

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

let stocksRequestSequence = 0;
let quoteRequestSequence = 0;

function markWatched(stocks: Stock[]): Stock[] {
  const portfolio = usePortfolioStore.getState();
  return stocks.map((stock) => ({
    ...stock,
    isWatched: portfolio.isInWatchlist(stock.symbol) || stock.isWatched,
  }));
}

function computeStats(all: Stock[]) {
  const currentQuotes = all.filter((s) => s.quoteStale === false && s.quoteSource && s.quoteDataTime);
  const upCount = currentQuotes.filter((s) => s.changePercent > 0).length;
  const downCount = currentQuotes.filter((s) => s.changePercent < 0).length;
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
  baseUsStocks: [],
  baseCnStocks: [],
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
  quoteState: 'loading',
  quoteSource: '',
  quoteDataTime: '',
  quoteMessage: '',
  market: 'us',
  sortBy: 'marketcap',
  order: 'desc',
  searchQuery: '',

  fetchStocks: async () => {
    const requestSequence = ++stocksRequestSequence;
    quoteRequestSequence += 1;
    const { market, sortBy, order, page, limit, searchQuery, allUsStocks, allCnStocks } = getState();
    set({ loading: true, error: null });

    try {
      if (market === 'us') {
        // Always re-merge live /api/market on fetch so browser/app refresh is not stuck on seed JSON.
        invalidateUsStocksCache();
        const all = await loadUsMarketStocks();
        if (requestSequence !== stocksRequestSequence) return;
        const pageResult = filterSortPageStocks(all, { searchQuery, sortBy, order, page, limit });
        const stats = computeStats(all);
        set({
          baseUsStocks: all,
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
        if (requestSequence !== stocksRequestSequence) return;
        const pageResult = filterSortPageStocks(all, { searchQuery, sortBy, order, page, limit });
        const stats = computeStats(all);
        set({
          baseCnStocks: all,
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
      if (requestSequence !== stocksRequestSequence) return;
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
      if (requestSequence !== stocksRequestSequence) return;
      console.error('Failed to fetch stocks:', error);
      set({
        error: error instanceof Error ? error.message : 'Failed to fetch stocks',
        loading: false,
      });
    }
  },

  refreshQuotes: async () => {
    const requestSequence = ++quoteRequestSequence;
    const { stocks, market, searchQuery, sortBy, order, page, limit } = getState();
    if (market !== 'us' && market !== 'cn') {
      return;
    }
    const pageSyms = stocks.map((s) => s.symbol);
    let result;
    try {
      result = await fetchQuoteResult(pageSyms);
    } catch (error) {
      if (requestSequence !== quoteRequestSequence) return;
      set({
        quoteState: 'error',
        quoteSource: '',
        quoteDataTime: '',
        quoteMessage: error instanceof Error ? error.message : '报价请求失败',
      });
      return;
    }
    if (requestSequence !== quoteRequestSequence) return;
    const hasProvenance = Boolean(result.meta.source && result.meta.dataTime && result.meta.dataTime !== 'unknown');
    const quotesUsable = !result.meta.stale && hasProvenance && Object.keys(result.quotes).length > 0;
    const quoteMessage = [
      result.meta.staleReason,
      ...result.errors,
      result.missing.length ? `缺少 ${result.missing.length} 个报价` : '',
      !hasProvenance ? '报价来源或数据时间不可验证' : '',
    ].filter(Boolean).join('；');
    if (!quotesUsable) {
      set({
        quoteState: result.meta.stale ? 'stale' : 'unavailable',
        quoteSource: result.meta.source,
        quoteDataTime: result.meta.dataTime,
        quoteMessage,
      });
      return;
    }
    const quotes = result.quotes;
    const receivedQuoteCount = Object.keys(quotes).length;

    set((state) => {
      const withEvidence = (rows: Stock[]) => applyQuotesToStocks(rows, quotes).map((stock) => {
        const hasQuote = Boolean(quotes[stock.symbol.toUpperCase()]);
        return {
          ...stock,
          quoteSource: result.meta.source,
          quoteDataTime: result.meta.dataTime,
          quoteStale: !hasQuote,
          quoteStaleReason: hasQuote ? undefined : '当前响应未包含该标的报价',
        };
      });
      const nextUs = withEvidence(state.baseUsStocks.length > 0 ? state.baseUsStocks : state.allUsStocks);
      const nextCn = withEvidence(state.baseCnStocks.length > 0 ? state.baseCnStocks : state.allCnStocks);
      const activeUniverse = state.market === 'cn' ? nextCn : nextUs;
      const pageResult = filterSortPageStocks(activeUniverse, {
        searchQuery,
        sortBy,
        order,
        page,
        limit,
      });
      return {
        allUsStocks: nextUs,
        allCnStocks: nextCn,
        stocks: markWatched(pageResult.stocks),
        total: pageResult.total,
        hasMore: pageResult.hasMore,
        lastQuoteAt: receivedQuoteCount > 0 ? Date.now() : state.lastQuoteAt,
        quoteState: 'live',
        quoteSource: result.meta.source,
        quoteDataTime: result.meta.dataTime,
        quoteMessage,
        ...computeStats(activeUniverse),
      };
    });
  },

  setMarket: (market) => {
    set({ market, page: 1, quoteState: 'loading', quoteSource: '', quoteDataTime: '', quoteMessage: '' });
  },

  setSortBy: (sortBy) => {
    set({ sortBy, page: 1 });
  },

  setOrder: (order) => {
    set({ order, page: 1 });
  },

  setSearchQuery: (query) => {
    set({ searchQuery: query, page: 1 });
  },

  setPage: (page) => {
    set({ page });
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
