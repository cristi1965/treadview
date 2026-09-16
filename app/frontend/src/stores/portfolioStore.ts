import { create } from 'zustand';
import {
  HoldingItem,
  WatchlistItem,
  AssetVenue,
  inferVenue,
  currencyForVenue,
  CashBalances,
} from '../types/portfolio';

interface PortfolioStore {
  watchlist: WatchlistItem[];
  holdings: HoldingItem[];
  cashBalances: CashBalances;

  addToWatchlist: (symbol: string, name: string, venue?: AssetVenue) => void;
  removeFromWatchlist: (symbol: string) => void;
  isInWatchlist: (symbol: string) => boolean;
  loadWatchlist: () => void;
  saveWatchlist: (items: WatchlistItem[]) => void;

  addHolding: (item: Omit<HoldingItem, 'currentValue' | 'profitLoss' | 'profitLossPercent'>) => void;
  updateHolding: (symbol: string, patch: Partial<HoldingItem>) => void;
  removeHolding: (symbol: string) => void;
  loadHoldings: () => void;
  setCashBalance: (currency: keyof CashBalances, amount: number) => void;
}

const WATCHLIST_KEY = 'stockgod_watchlist';
const HOLDINGS_KEY = 'stockgod_holdings';
const CASH_KEY = 'stockgod_cash_balances';
const EMPTY_CASH: CashBalances = { USD: 0, CNY: 0 };

const normalizeWatch = (item: any): WatchlistItem => {
  const venue = (item.venue as AssetVenue) || inferVenue(String(item.symbol || ''));
  return {
    ...item,
    venue,
    currency: item.currency || currencyForVenue(venue),
    addedAt: new Date(item.addedAt),
  };
};

const normalizeHolding = (item: any): HoldingItem => {
  const venue = (item.venue as AssetVenue) || inferVenue(String(item.symbol || ''));
  return {
    ...item,
    venue,
    currency: item.currency || currencyForVenue(venue),
    purchaseDate: new Date(item.purchaseDate),
  };
};

const loadWatchlistFromStorage = (): WatchlistItem[] => {
  try {
    const data = localStorage.getItem(WATCHLIST_KEY);
    if (!data) return [];
    return JSON.parse(data).map(normalizeWatch);
  } catch {
    return [];
  }
};

const saveWatchlistToStorage = (items: WatchlistItem[]) => {
  try {
    localStorage.setItem(WATCHLIST_KEY, JSON.stringify(items));
  } catch {
    /* ignore */
  }
};

const loadHoldingsFromStorage = (): HoldingItem[] => {
  try {
    const data = localStorage.getItem(HOLDINGS_KEY);
    if (!data) return [];
    return JSON.parse(data).map(normalizeHolding);
  } catch {
    return [];
  }
};

const saveHoldingsToStorage = (items: HoldingItem[]) => {
  try {
    localStorage.setItem(HOLDINGS_KEY, JSON.stringify(items));
  } catch {
    /* ignore */
  }
};

const loadCashBalances = (): CashBalances => {
  try {
    const parsed = JSON.parse(localStorage.getItem(CASH_KEY) || '{}');
    return {
      USD: Number.isFinite(Number(parsed.USD)) && Number(parsed.USD) >= 0 ? Number(parsed.USD) : 0,
      CNY: Number.isFinite(Number(parsed.CNY)) && Number(parsed.CNY) >= 0 ? Number(parsed.CNY) : 0,
    };
  } catch {
    return { ...EMPTY_CASH };
  }
};

const saveCashBalances = (balances: CashBalances) => {
  try {
    localStorage.setItem(CASH_KEY, JSON.stringify(balances));
  } catch {
    /* ignore */
  }
};

export const usePortfolioStore = create<PortfolioStore>((set, get) => ({
  watchlist: loadWatchlistFromStorage(),
  holdings: loadHoldingsFromStorage(),
  cashBalances: loadCashBalances(),

  addToWatchlist: (symbol, name, venue) => {
    const current = get().watchlist;
    const sym = symbol.toUpperCase();
    if (current.some((item) => item.symbol.toUpperCase() === sym)) return;
    const v = venue || inferVenue(sym);
    const newItem: WatchlistItem = {
      symbol: sym,
      name,
      addedAt: new Date(),
      venue: v,
      currency: currencyForVenue(v),
    };
    const next = [...current, newItem];
    saveWatchlistToStorage(next);
    set({ watchlist: next });
  },

  removeFromWatchlist: (symbol) => {
    const next = get().watchlist.filter((item) => item.symbol.toUpperCase() !== symbol.toUpperCase());
    saveWatchlistToStorage(next);
    set({ watchlist: next });
  },

  isInWatchlist: (symbol) =>
    get().watchlist.some((item) => item.symbol.toUpperCase() === symbol.toUpperCase()),

  loadWatchlist: () => set({ watchlist: loadWatchlistFromStorage() }),
  saveWatchlist: (items) => {
    saveWatchlistToStorage(items);
    set({ watchlist: items });
  },

  addHolding: (item) => {
    const venue = item.venue || inferVenue(item.symbol);
    const row: HoldingItem = {
      ...item,
      symbol: item.symbol.toUpperCase(),
      venue,
      currency: item.currency || currencyForVenue(venue),
    };
    const current = get().holdings;
    const existing = current.findIndex((holding) => holding.symbol.toUpperCase() === row.symbol);
    const next = existing < 0
      ? [...current, row]
      : current.map((holding, index) => {
          if (index !== existing) return holding;
          const totalQuantity = holding.quantity + row.quantity;
          return {
            ...holding,
            quantity: totalQuantity,
            avgCost: totalQuantity > 0
              ? (holding.avgCost * holding.quantity + row.avgCost * row.quantity) / totalQuantity
              : row.avgCost,
            notes: row.notes || holding.notes,
            sector: row.sector || holding.sector,
            stopLoss: row.stopLoss ?? holding.stopLoss,
            venue: row.venue,
            currency: row.currency,
            purchaseDate: row.purchaseDate || holding.purchaseDate,
          };
        });
    saveHoldingsToStorage(next);
    set({ holdings: next });
  },

  updateHolding: (symbol, patch) => {
    const next = get().holdings.map((h) =>
      h.symbol.toUpperCase() === symbol.toUpperCase() ? { ...h, ...patch } : h
    );
    saveHoldingsToStorage(next);
    set({ holdings: next });
  },

  removeHolding: (symbol) => {
    const next = get().holdings.filter((h) => h.symbol.toUpperCase() !== symbol.toUpperCase());
    saveHoldingsToStorage(next);
    set({ holdings: next });
  },

  loadHoldings: () => set({ holdings: loadHoldingsFromStorage() }),

  setCashBalance: (currency, amount) => {
    const next = { ...get().cashBalances, [currency]: Number.isFinite(amount) && amount >= 0 ? amount : 0 };
    saveCashBalances(next);
    set({ cashBalances: next });
  },
}));
