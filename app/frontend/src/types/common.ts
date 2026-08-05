// 通用类型定义

export interface Stock {
  symbol: string;
  name: string;
  price: number;
  changePercent: number;
  marketCap: number;
  industry: string;
  volume: number;
  
  // 五方评分
  scores: {
    buffett: number;
    duanyongping: number;
    serenity: number;
    druckenmiller: number;
    sentiment: number;
  };
  avgScore: number;
  divergence: number;
  
  // 判读后
  afterScore?: number;
}

export interface MarketData {
  name: string;
  value: number;
  change: number;
  changePercent: number;
}

export type SortDirection = 'asc' | 'desc';

export type TimeRange = '1D' | '1W' | '1M' | '3M' | '6M' | '1Y' | '3Y' | '5Y' | 'All';

export interface SelectOption {
  value: string;
  label: string;
}

export interface PaginationProps {
  currentPage: number;
  totalPages: number;
  pageSize: number;
  totalItems: number;
}
