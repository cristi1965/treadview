export type ReportType = 'premarket' | 'postmarket';

export interface Report {
  id: string;
  type: ReportType;
  title: string;
  date: string; // ISO date
  time: string; // e.g., "16:00 ET"
  summary: string;
  content: string; // Markdown
  publishedAt: string; // ISO datetime
}

export type MarketEventType = 'macro' | 'earnings' | 'policy' | 'holiday';

export interface EarningsDetail {
  ticker: string;
  companyName: string;
  expectedEPS: number;
  marketCap: string;
  time: string;
}

export interface MarketEvent {
  id: string;
  date: string; // ISO date
  dayOfWeek: string;
  isToday?: boolean;
  type: MarketEventType;
  isImportant?: boolean;
  title: string;
  description?: string;
  relatedTickers?: string[];
  earnings?: EarningsDetail[];
  /** Jin10-style columns */
  time?: string; // "08:30" ET
  country?: string; // US / CN / EU / JP / UK / DE
  importance?: number; // 1..3 stars
  previous?: string;
  forecast?: string;
  actual?: string; // empty = 未公布
}

export type FlashKind = 'macro' | 'earnings' | 'company' | 'market' | 'calendar';

export interface FlashItem {
  id: string;
  time: string; // RFC3339
  title: string;
  titleEn?: string;
  body?: string;
  bodyEn?: string;
  importance: number; // 1 | 2 | 3
  kind: FlashKind;
  tickers?: string[];
  source?: string;
  link?: string;
}

export interface FlashResponse {
  items: FlashItem[];
  updatedAt: string;
}
