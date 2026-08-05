import { create } from 'zustand';
import { HoldingItem, WatchlistItem } from '../types/portfolio';

interface PortfolioStore {
  watchlist: WatchlistItem[];
  holdings: HoldingItem[];

  addToWatchlist: (symbol: string, name: string) => void;
  removeFromWatchlist: (symbol: string) => void;
  isInWatchlist: (symbol: string) => boolean;
  loadWatchlist: () => void;
  saveWatchlist: (items: WatchlistItem[]) => void;

  addHolding: (item: Omit<HoldingItem, 'currentValue' | 'profitLoss' | 'profitLossPercent'>) => void;
  updateHolding: (symbol: string, patch: Partial<HoldingItem>) => void;
  removeHolding: (symbol: string) => void;
  loadHoldings: () => void;
}

const WATCHLIST_KEY = 'stockgod_watchlist';
const HOLDINGS_KEY = 'stockgod_holdings';

const loadWatchlistFromStorage = (): WatchlistItem[] => {
  try {
    const data = localStorage.getItem(WATCHLIST_KEY);
    if (!data) return [];
    const parsed = JSON.parse(data);
    return parsed.map((item: any) => ({
      ...item,
      addedAt: new Date(item.addedAt),
    }));
  } catch (error) {
    console.error('Failed to load watchlist:', error);
    return [];
  }
};

const saveWatchlistToStorage = (items: WatchlistItem[]) => {
  try {
    localStorage.setItem(WATCHLIST_KEY, JSON.stringify(items));
  } catch (error) {
    console.error('Failed to save watchlist:', error);
  }
};

const loadHoldingsFromStorage = (): HoldingItem[] => {
  try {
    const data = localStorage.getItem(HOLDINGS_KEY);
    if (!data) return [];
    const parsed = JSON.parse(data);
    return parsed.map((item: any) => ({
      ...item,
      purchaseDate: new Date(item.purchaseDate),
    }));
  } catch (error) {
    console.error('Failed to load holdings:', error);
    return [];
  }
};

const saveHoldingsToStorage = (items: HoldingItem[]) => {
  try {
    localStorage.setItem(HOLDINGS_KEY, JSON.stringify(items));
  } catch (error) {
    console.error('Failed to save holdings:', error);
  }
};

export const usePortfolioStore = create<PortfolioStore>((set, get) => ({
  watchlist: loadWatchlistFromStorage(),
  holdings: loadHoldingsFromStorage(),

  addToWatchlist: (symbol: string, name: string) => {
    const current = get().watchlist;
    if (current.some((item) => item.symbol === symbol)) return;

    const newItem: WatchlistItem = {
      symbol,
      name,
      addedAt: new Date(),
    };

    const updated = [...current, newItem];
    set({ watchlist: updated });
    saveWatchlistToStorage(updated);
  },

  removeFromWatchlist: (symbol: string) => {
    const updated = get().watchlist.filter((item) => item.symbol !== symbol);
    set({ watchlist: updated });
    saveWatchlistToStorage(updated);
  },

  isInWatchlist: (symbol: string) => get().watchlist.some((item) => item.symbol === symbol),

  loadWatchlist: () => set({ watchlist: loadWatchlistFromStorage() }),

  saveWatchlist: (items: WatchlistItem[]) => {
    set({ watchlist: items });
    saveWatchlistToStorage(items);
  },

  addHolding: (item) => {
    const current = get().holdings;
    const existing = current.findIndex((h) => h.symbol.toUpperCase() === item.symbol.toUpperCase());
    let updated: HoldingItem[];
    if (existing >= 0) {
      const prev = current[existing];
      const totalQty = prev.quantity + item.quantity;
      const avgCost =
        totalQty > 0 ? (prev.avgCost * prev.quantity + item.avgCost * item.quantity) / totalQty : item.avgCost;
      updated = current.map((h, i) =>
        i === existing
          ? {
              ...h,
              quantity: totalQty,
              avgCost,
              notes: item.notes || h.notes,
              purchaseDate: item.purchaseDate || h.purchaseDate,
            }
          : h
      );
    } else {
      updated = [...current, { ...item, symbol: item.symbol.toUpperCase() }];
    }
    set({ holdings: updated });
    saveHoldingsToStorage(updated);
  },

  updateHolding: (symbol, patch) => {
    const updated = get().holdings.map((h) =>
      h.symbol.toUpperCase() === symbol.toUpperCase() ? { ...h, ...patch } : h
    );
    set({ holdings: updated });
    saveHoldingsToStorage(updated);
  },

  removeHolding: (symbol) => {
    const updated = get().holdings.filter((h) => h.symbol.toUpperCase() !== symbol.toUpperCase());
    set({ holdings: updated });
    saveHoldingsToStorage(updated);
  },

  loadHoldings: () => set({ holdings: loadHoldingsFromStorage() }),
}));
