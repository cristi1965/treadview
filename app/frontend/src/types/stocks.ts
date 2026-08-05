export interface FiveFactorScores {
  buffett: number;        // 0-100
  duanyongping: number;   // 0-100
  serenity: number;       // 0-100
  druckenmiller: number;  // 0-100
  sentiment: number;      // 0-100
}

export interface Stock {
  symbol: string;
  name: string;
  price: number;
  change: number;
  changePercent: number;
  marketCap: number;
  volume: number;
  sector: string;
  scores: FiveFactorScores;
  avgScore: number;
  postAnalysisChange?: number;
  divergence?: number;
  isWatched: boolean;
  /** Present in us-panel-summary */
  judged?: boolean;
  /** Present in dilution-flags */
  hasDilution?: boolean;
  industry?: string;
  segment?: string;
  subSector?: string;
  country?: string;
  /** Listing venue hint: us | cn | adr */
  venue?: 'us' | 'cn' | 'adr';
}

export type StockSortBy = 'marketcap' | 'price' | 'change' | 'avgscore' | 'volume';
export type SortOrder = 'asc' | 'desc';
