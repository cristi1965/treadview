export type ETFCategory = 
  | 'broad' 
  | 'industry' 
  | 'theme' 
  | 'factor' 
  | 'bond' 
  | 'commodity' 
  | 'leveraged' 
  | 'other';

export interface ETFSector {
  id: string;
  name: string;
  category: ETFCategory;
  etfCount: number;
  aum: number; // 总规模 (美元)
  
  // 5年最强 ETF
  topPerformer: {
    ticker: string;
    return5y: number; // 百分比
  };
  
  // 最大回撤
  maxDrawdown: number; // 百分比 (负数)
  
  // 排序字段
  return1y?: number; // 近1年平均回报率
  return5y?: number; // 近5年平均回报率
  avgExpenseRatio?: number; // 平均费率
}

export type ETFSortBy = 'aum' | 'return1y' | 'return5y' | 'drawdown';

export interface ETFCategoryInfo {
  id: ETFCategory;
  label: string;
  count: number;
}
