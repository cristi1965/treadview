// Portfolio 页面类型定义

export interface WatchlistItem {
  symbol: string;
  name: string;
  addedAt: Date;
  
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
  
  // 计算字段
  currentValue?: number;
  profitLoss?: number;
  profitLossPercent?: number;
}

export type PortfolioTab = 'watchlist' | 'holdings';
