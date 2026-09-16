import { Stock } from '../types/stocks';

export type ScanPresetId =
  | 'us-momentum'
  | 'us-highscore'
  | 'cn-strong'
  | 'cn-large'
  | 'etf-ai';

export interface ScanPreset {
  id: ScanPresetId;
  label: string;
  market: 'us' | 'cn';
  describe: string;
  filter: (s: Stock) => boolean;
  sortBy?: 'change' | 'marketcap' | 'avgscore';
}

export const SCAN_PRESETS: ScanPreset[] = [
  {
    id: 'us-momentum',
    label: '美股动量',
    market: 'us',
    describe: '中盘以上 + 当日涨幅靠前',
    sortBy: 'change',
    filter: (s) => s.marketCap >= 2_000_000_000 && s.changePercent >= 3,
  },
  {
    id: 'us-highscore',
    label: '美股高分',
    market: 'us',
    describe: '五方均分 ≥ 65',
    sortBy: 'avgscore',
    filter: (s) => s.avgScore >= 65,
  },
  {
    id: 'cn-strong',
    label: 'A股强势',
    market: 'cn',
    describe: '涨幅 ≥ 5%',
    sortBy: 'change',
    filter: (s) => s.changePercent >= 5,
  },
  {
    id: 'cn-large',
    label: 'A股大盘',
    market: 'cn',
    describe: '市值靠前（亿元口径）',
    sortBy: 'marketcap',
    filter: (s) => s.marketCap >= 50_000_000_000,
  },
];

export const ETF_PRESETS = [
  { id: 'all', label: '全部' },
  { id: 'ai-semi', label: 'AI/半导体', match: /semi|chip|ai|nvda|sox|半导体|科技|信息/i },
  { id: 'broad', label: '宽基', match: /spy|qqq|voo|iwm|沪深|创业|中证|500|300/i },
  { id: 'cn', label: 'A股 ETF', match: /^\d{6}$|华泰|易方达|华夏|南方|国联/i },
] as const;
