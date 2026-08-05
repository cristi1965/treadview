import { Stock } from './stocks';

export interface StockComparison {
  stockA: Stock | null;
  stockB: Stock | null;
}

export interface ComparisonMetric {
  label: string;
  valueA: string | number;
  valueB: string | number;
  winner?: 'A' | 'B' | 'tie';
}
