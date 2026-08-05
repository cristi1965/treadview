// Type definitions for Whales feature

export interface RecentHolding {
  symbol: string;
  name: string;
  weight: number;
  action: string; // 买入 | 卖出 | 持有
}

export interface Investor {
  id: string;
  name: string;           // 中文名
  nameEn: string;         // 英文名
  slug: string;           // URL slug
  company: string;        // 机构名称
  avatar?: string;        // 头像 URL（可选）
  type?: string;
  
  // 统计数据
  holdings: number;       // 持仓数量
  topStock: {
    symbol: string;
    name: string;
    percentage: number;   // 占比 %
  };
  recentHoldings?: RecentHolding[];
}

export interface CongressMember {
  id: string;             // 如 "M001245"
  name: string;           // 姓名
  party: 'D' | 'R';      // 党派：D=民主党, R=共和党
  state: string;          // 州
  district: string;       // 选区
  photoUrl?: string;      // 照片 URL
  
  // 最新交易
  latestTrade: {
    action: 'buy' | 'sell';
    symbol: string;
    symbolName?: string;
    amount: string;       // 如 "$15K-$50K"
    date: string;         // 如 "6/11"
  };
  
  isHot?: boolean;        // 是否热门
}

export interface WhaleDetail {
  name: string;
  nameEn: string;
  company: string;
  reportType: string;     // "13F"
  updatedAt: string;      // 更新日期
  
  // 统计
  totalHoldings: number;
  topTenConcentration: number;  // %
  newPositions: number;
  reducedPositions: number;
  
  // 持仓列表
  holdings: Holding[];
}

export interface Holding {
  rank: number;
  symbol: string;
  name: string;
  marketShare: number;  // 占比 %
  consensusCount: number;  // 机构数
  action: 'hold' | 'buy' | 'sell' | 'new';
  actionLabel: string;  // "持有", "加仓", "减持", "新建仓"
}

export type TabType = 'institutions' | 'congress';
export type PartyFilter = 'all' | 'democrat' | 'republican';
export type TradeTypeFilter = 'all' | 'buy' | 'sell';
