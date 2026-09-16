// Portfolio 页面类型定义

export type AssetVenue = 'us' | 'cn' | 'etf';
export type PortfolioCurrency = 'USD' | 'CNY';
export type CashBalances = Record<PortfolioCurrency, number>;

export interface WatchlistItem {
  symbol: string;
  name: string;
  addedAt: Date;
  venue?: AssetVenue;
  currency?: 'USD' | 'CNY';

  // 实时数据 (从 API 获取)
  price?: number;
  changePercent?: number;
  avgScore?: number;
}

export interface HoldingItem {
  symbol: string;
  name: string;
  quantity: number;
  avgCost: number;
  purchaseDate: Date;
  notes?: string;
  sector?: string;
  stopLoss?: number;
  venue?: AssetVenue;
  currency?: PortfolioCurrency;

  // 计算字段
  currentValue?: number;
  profitLoss?: number;
  profitLossPercent?: number;
}

export type PortfolioTab = 'watchlist' | 'holdings';

export function inferVenue(symbol: string): AssetVenue {
  const s = symbol.trim().toUpperCase();
  if (/^\d{6}$/.test(s)) return 'cn';
  // common US ETF tickers + CN ETF numeric already caught
  const etfHints = new Set([
    'SPY', 'QQQ', 'IWM', 'DIA', 'VOO', 'VTI', 'SOXX', 'SMH', 'XLK', 'XLF', 'XLE', 'GLD', 'SLV', 'TQQQ', 'SQQQ',
    '510300', '510500', '512480', '159915', '159919', '588000',
  ]);
  if (etfHints.has(s) || s.endsWith('ETF')) return 'etf';
  return 'us';
}

export function currencyForVenue(venue: AssetVenue): PortfolioCurrency {
  return venue === 'cn' ? 'CNY' : 'USD';
}
