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
  /** File-level provenance for the five-factor score. */
  scoreSource?: string;
  scoreGeneratedAt?: string;
  /** Provenance for the currently displayed quote or static snapshot. */
  quoteSource?: string;
  quoteDataTime?: string;
  quoteStale?: boolean;
  quoteStaleReason?: string;
  scoreDetail?: ScoreDetail;
	/** Historical validation is required before scores may be treated as signals. */
	scoreValidation?: ScoreValidation;
  /** Present in dilution-flags */
  hasDilution?: boolean;
  industry?: string;
  segment?: string;
  subSector?: string;
  country?: string;
  /** Listing venue hint: us | cn | adr */
  venue?: 'us' | 'cn' | 'adr';
}

export interface ScoreValidationMetric {
	factor: string;
	sample_count: number;
	ic: number;
	ic_95: [number, number];
	direction_accuracy: number;
	accuracy_95: [number, number];
}

export interface ScoreValidation {
	status: 'validated' | 'unvalidated' | string;
	method: string;
	cutoff?: string;
	train_samples: number;
	test_samples: number;
	benchmark: string;
	benchmark_accuracy: number;
	metrics?: ScoreValidationMetric[];
	reasons?: string[];
}

export interface ScoreDetail {
  method_version: string;
  inputs: {
    symbol: string;
    name: string;
    sector: string;
    industry: string;
    segment: string;
    sub: string;
    price: number;
    day_pct: number;
    market_cap_b: number;
    volume: number;
  };
  components: Record<string, Array<{ rule: string; value: number }>>;
  raw_scores: number[];
  final_scores: number[];
  panel_scores: number[];
  reproduces_panel: boolean;
  calibration: string;
}

export type StockSortBy = 'marketcap' | 'price' | 'change' | 'avgscore' | 'volume';
export type SortOrder = 'asc' | 'desc';
