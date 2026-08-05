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
}
