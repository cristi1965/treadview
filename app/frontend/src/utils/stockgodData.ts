import { FiveFactorScores, ScoreDetail, ScoreValidation, Stock } from '../types/stocks';
import { ETFCategory, ETFSector } from '../types/etf';
import { apiUrl, getWithMeta } from './api';

/** Same-origin `/data/*` (Vite public / Go dist). Falls back to API host if needed. */
export async function fetchDataJson<T>(path: string): Promise<T> {
  const normalized = path.startsWith('/') ? path : `/${path}`;
  const primary = await fetch(normalized, { cache: 'no-store' });
  if (primary.ok) return primary.json() as Promise<T>;

  const secondary = await fetch(apiUrl(normalized), { cache: 'no-store' });
  if (!secondary.ok) {
    throw new Error(`Failed to load ${normalized}: ${primary.status}/${secondary.status}`);
  }
  return secondary.json() as Promise<T>;
}

interface RawUsStock {
  sym: string;
  name: string;
  price: number;
  pct: number;
  mcapB: number;
  sector: string;
  industry?: string;
  vol: number;
  country?: string;
  seg?: string;
  sub?: string;
}

interface UsStocksFile {
  generated_at?: string;
  count: number;
  stocks: RawUsStock[];
}

interface MarketLiveFile {
  quotes?: Record<string, { price: number; pct: number; vol?: number; mcapB?: number }>;
  ts?: number;
  count?: number;
}

interface PanelStock {
  sc: number[];
  div: number;
  detail?: ScoreDetail;
}

interface PanelFile {
  order: string[];
  generated_at?: string;
  source?: string;
  stocks: Record<string, PanelStock>;
	validation?: ScoreValidation;
}

interface JudgmentFile {
  anchor: string;
  p0: Record<string, number>;
}

interface DilutionFlag {
  tier?: string;
  shelf?: boolean;
  ratio?: number;
}

interface DilutionFile {
  flags: Record<string, DilutionFlag>;
}

interface EtfRaw {
  sym: string;
  name: string;
  aum?: number;
  ret1y?: number | null;
  ret5y?: number | null;
  mdd?: number | null;
  sector: string;
  kind: string;
}

interface EtfSectorRaw {
  sector: string;
  super: string;
  n: number;
  aum: number;
}

interface EtfAnalysesFile {
  updated?: string;
  n: number;
  supers: Record<string, number>;
  sectors: EtfSectorRaw[];
  etfs: EtfRaw[];
}

export interface PremarketMovers {
  session: string;
  label: string;
  gainers: Array<{ sym: string; price: number; pct: number; prevPct?: number }>;
  losers: Array<{ sym: string; price: number; pct: number; prevPct?: number }>;
  ts?: number;
}

export interface MacroSeriesItem {
  sym: string;
  name: string;
  kind: string;
  price: number;
  pct: number;
}

export interface QuoteItem {
  price: number;
  pct: number;
  session?: string;
  prevClose?: number;
}

const SCORE_KEYS: Array<keyof FiveFactorScores> = [
  'buffett',
  'duanyongping',
  'serenity',
  'druckenmiller',
  'sentiment',
];

const SUPER_TO_CATEGORY: Record<string, ETFCategory> = {
  宽基: 'broad',
  行业: 'industry',
  主题: 'theme',
  因子策略: 'factor',
  债券: 'bond',
  商品: 'commodity',
  工具: 'leveraged',
  其他: 'other',
};

const ETF_ANALYSES_MAX_AGE_MS = 7 * 24 * 60 * 60 * 1000;

function assertFreshEtfAnalyses(data: EtfAnalysesFile) {
  const updatedAt = Date.parse(data.updated || '');
  if (!Number.isFinite(updatedAt)) {
    throw new Error(`ETF analyses updated field invalid: ${data.updated || 'missing'}`);
  }
  const age = Date.now() - updatedAt;
  if (age > ETF_ANALYSES_MAX_AGE_MS) {
    const ageHours = Math.round(age / 36e5);
    throw new Error(`ETF analyses stale: updated=${data.updated} age=${ageHours}h`);
  }
}

let usStocksCache: Promise<Stock[]> | null = null;
let usStocksCacheAt = 0;
let aStocksCache: Promise<Stock[]> | null = null;
let aStocksCacheAt = 0;
let etfCache: Promise<{
  sectors: ETFSector[];
  categories: Record<ETFCategory, number>;
  etfs: EtfRaw[];
  total: number;
  updated: string;
}> | null = null;
let etfCacheAt = 0;

const DATA_CACHE_TTL_MS = 60_000;

function isFreshCache(startedAt: number): boolean {
  return Date.now() - startedAt < DATA_CACHE_TTL_MS;
}

export type HeatMarket = '美股' | 'A 股' | '港股' | '台股' | '韩股' | '欧' | '日';

const EU_COUNTRIES = new Set([
  'United Kingdom',
  'Germany',
  'France',
  'Netherlands',
  'Switzerland',
  'Ireland',
  'Sweden',
  'Spain',
  'Italy',
  'Denmark',
  'Belgium',
  'Norway',
  'Finland',
  'Austria',
  'Luxembourg',
  'Greece',
]);

interface AMarketFile {
  quotes: Record<string, { price: number; pct: number; vol?: number; mcapYi?: number }>;
  ts?: number;
  count?: number;
}

function emptyScores(): FiveFactorScores {
  return { buffett: 0, duanyongping: 0, serenity: 0, druckenmiller: 0, sentiment: 0 };
}

function mapPanelScores(sc: number[] | undefined): { scores: FiveFactorScores; avgScore: number; divergence: number } {
  if (!sc || sc.length < 5) {
    return { scores: emptyScores(), avgScore: 0, divergence: 0 };
  }
  const scores = emptyScores();
  SCORE_KEYS.forEach((key, i) => {
    scores[key] = Math.round(sc[i] ?? 0);
  });
  const avgScore = Math.round(sc.reduce((a, b) => a + b, 0) / sc.length);
  const mean = sc.reduce((a, b) => a + b, 0) / sc.length;
  const variance = sc.reduce((a, b) => a + (b - mean) ** 2, 0) / sc.length;
  const divergence = Math.round(Math.sqrt(variance));
  return { scores, avgScore, divergence };
}

function slugifySector(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^\w\u4e00-\u9fff]+/g, '-')
    .replace(/^-|-$/g, '') || 'sector';
}

async function loadPanelFile(): Promise<PanelFile> {
  try {
    const result = await getWithMeta<PanelFile>('/api/panel-summary');
    const hasProvenance = Boolean(result.meta.source && result.meta.dataTime && result.meta.dataTime !== 'unknown');
    if (!result.meta.stale && hasProvenance && result.data.validation?.status === 'validated') return result.data;
  } catch {
    /* ignore */
  }
  return { order: [], stocks: {} };
}

/** Drop in-memory US stocks cache (after panel refresh). */
export function invalidateUsStocksCache(): void {
  usStocksCache = null;
  usStocksCacheAt = 0;
}

export async function loadUsMarketStocks(): Promise<Stock[]> {
  if (!usStocksCache || !isFreshCache(usStocksCacheAt)) {
    usStocksCacheAt = Date.now();
    usStocksCache = (async () => {
      // Kick backend live refresh (top caps); don't block first paint on full universe.
      const [us, panel, judgment, dilution, marketLive] = await Promise.all([
        fetchDataJson<UsStocksFile>('/data/us-stocks.json'),
        loadPanelFile(),
        fetchDataJson<JudgmentFile>('/data/judgment-p0-us.json'),
        fetchDataJson<DilutionFile>('/data/dilution-flags.json'),
        getWithMeta<MarketLiveFile>('/api/market').catch(() => null),
      ]);

      const marketHasProvenance = Boolean(
        marketLive?.meta.source && marketLive.meta.dataTime && marketLive.meta.dataTime !== 'unknown'
      );
      const marketUsable = Boolean(marketLive && !marketLive.meta.stale && marketHasProvenance);
      const liveQuotes = marketUsable ? marketLive?.data.quotes || {} : {};

      return us.stocks.map((raw) => {
        const panelRow = panel.stocks[raw.sym];
		const validationUsable = panel.validation?.status === 'validated';
		const { scores, avgScore, divergence: computedDiv } = mapPanelScores(validationUsable ? panelRow?.sc : undefined);
        const divergence = panelRow?.div ?? computedDiv;
        const p0 = judgment.p0[raw.sym];
        const live = liveQuotes[raw.sym.toUpperCase()] || liveQuotes[raw.sym];
        const price = live && live.price > 0 ? live.price : raw.price;
        const pct = live && live.price > 0 ? live.pct : raw.pct;
        let mcapB = raw.mcapB || 0;
        if (live && live.price > 0) {
          if (live.mcapB && live.mcapB > 0) mcapB = live.mcapB;
          else if (raw.price > 0 && mcapB > 0) mcapB = mcapB * (price / raw.price);
        }
        const postAnalysisChange =
          typeof p0 === 'number' && p0 > 0 ? Number((((price - p0) / p0) * 100).toFixed(2)) : undefined;
        const flag = dilution.flags[raw.sym];

        return {
          symbol: raw.sym,
          name: raw.name,
          price,
          change: Number(((price * pct) / 100).toFixed(4)),
          changePercent: pct,
          marketCap: mcapB * 1_000_000_000,
          volume: raw.vol || 0,
          sector: raw.sector || raw.seg || 'Unknown',
          scores,
          avgScore,
          divergence,
          postAnalysisChange,
          isWatched: false,
		  judged: Boolean(panelRow && validationUsable),
		  scoreSource: validationUsable ? panel.source : undefined,
		  scoreGeneratedAt: validationUsable ? panel.generated_at : undefined,
		  quoteSource: marketUsable ? marketLive?.meta.source : 'local-us-stocks-snapshot',
		  quoteDataTime: marketUsable ? marketLive?.meta.dataTime : us.generated_at,
		  quoteStale: !marketUsable,
		  quoteStaleReason: marketUsable
		    ? undefined
		    : marketLive?.meta.staleReason || '静态市场快照，仅供历史参考',
		  scoreDetail: validationUsable ? panelRow?.detail : undefined,
		  scoreValidation: panel.validation,
          hasDilution: Boolean(flag),
          industry: raw.industry,
          segment: raw.seg,
          subSector: raw.sub,
          country: raw.country,
          venue: 'us',
        } satisfies Stock;
      });
    })().catch((err) => {
      usStocksCache = null;
      usStocksCacheAt = 0;
      throw err;
    });
  }
  return usStocksCache;
}

/** Drop in-memory A-share stocks cache. */
export function invalidateAStocksCache(): void {
  aStocksCache = null;
  aStocksCacheAt = 0;
}

/** A-share quotes: prefer live `/api/a-market`, return empty on failure. */
export async function loadAMarketStocks(): Promise<Stock[]> {
  if (!aStocksCache || !isFreshCache(aStocksCacheAt)) {
    aStocksCacheAt = Date.now();
    aStocksCache = (async () => {
      let data: AMarketFile | null = null;
      try {
        const res = await fetch(apiUrl('/api/a-market'), { cache: 'no-store' });
        if (res.ok) data = (await res.json()) as AMarketFile;
      } catch {
        /* fall through */
      }
      if (!data?.quotes) {
        return [] as Stock[];
      }

      return Object.entries(data.quotes || {}).map(([sym, q]) => {
        const mcapYi = q.mcapYi || 0;
        // mcapYi ≈ 亿元人民币 → approximate USD for relative heatmap sizing
        const marketCap = mcapYi * 100_000_000;
        return {
          symbol: sym,
          name: (q as { name?: string }).name || sym,
          price: q.price,
          change: Number(((q.price * q.pct) / 100).toFixed(4)),
          changePercent: q.pct,
          marketCap,
          volume: q.vol || 0,
          sector: inferAShareBoard(sym),
          scores: emptyScores(),
          avgScore: 0,
          divergence: 0,
          isWatched: false,
          judged: false,
          hasDilution: false,
          country: 'China',
          venue: 'cn',
        } satisfies Stock;
      });
    })().catch((err) => {
      aStocksCache = null;
      aStocksCacheAt = 0;
      throw err;
    });
  }
  return aStocksCache;
}

function inferAShareBoard(sym: string): string {
  if (sym.startsWith('688') || sym.startsWith('689')) return '科创板';
  if (sym.startsWith('300') || sym.startsWith('301')) return '创业板';
  if (sym.startsWith('8') || sym.startsWith('4')) return '北交所';
  if (sym.startsWith('6')) return '沪市主板';
  if (sym.startsWith('0') || sym.startsWith('001') || sym.startsWith('002') || sym.startsWith('003')) return '深市主板';
  return 'A股';
}

export function filterStocksByHeatMarket(stocks: Stock[], market: HeatMarket): Stock[] {
  if (market === '美股') return stocks.filter((s) => s.venue !== 'cn');
  if (market === 'A 股') return stocks.filter((s) => s.venue === 'cn');
  if (market === '港股') return stocks.filter((s) => s.country === 'Hong Kong');
  if (market === '台股') return stocks.filter((s) => s.country === 'Taiwan');
  if (market === '韩股') {
    return stocks.filter((s) => {
      const c = s.country || '';
      return c === 'South Korea' || c === 'Korea' || c.includes('Korea');
    });
  }
  if (market === '欧') return stocks.filter((s) => EU_COUNTRIES.has(s.country || ''));
  if (market === '日') return stocks.filter((s) => s.country === 'Japan');
  return stocks;
}

export async function findUsStock(symbol: string): Promise<Stock | null> {
  const all = await loadUsMarketStocks();
  const upper = symbol.toUpperCase();
  const hit = all.find((s) => s.symbol.toUpperCase() === upper);
  if (hit) return hit;
  if (/^\d{6}$/.test(upper)) {
    const cn = await loadAMarketStocks();
    return cn.find((s) => s.symbol === upper) || null;
  }
  return null;
}

function mapEtfAnalyses(data: EtfAnalysesFile) {
  const bySector = new Map<string, EtfRaw[]>();
  for (const etf of data.etfs) {
    const list = bySector.get(etf.sector) || [];
    list.push(etf);
    bySector.set(etf.sector, list);
  }

  const categories = {
    broad: data.supers['宽基'] || 0,
    industry: data.supers['行业'] || 0,
    theme: data.supers['主题'] || 0,
    factor: data.supers['因子策略'] || 0,
    bond: data.supers['债券'] || 0,
    commodity: data.supers['商品'] || 0,
    leveraged: data.supers['工具'] || 0,
    other: data.supers['其他'] || 0,
  } as Record<ETFCategory, number>;

  const sectors: ETFSector[] = data.sectors.map((sec) => {
    const members = bySector.get(sec.sector) || [];
    const with5y = members.filter((m) => typeof m.ret5y === 'number');
    const top = [...with5y].sort((a, b) => (b.ret5y || 0) - (a.ret5y || 0))[0];
    const worstMdd = members.reduce((min, m) => Math.min(min, m.mdd ?? 0), 0);
    const with1y = members.filter((m) => typeof m.ret1y === 'number');
    const avg1y = with1y.reduce((sum, member) => sum + (member.ret1y || 0), 0) / Math.max(1, with1y.length) || 0;
    const avg5y = with5y.reduce((sum, member) => sum + (member.ret5y || 0), 0) / Math.max(1, with5y.length) || 0;

    return {
      id: slugifySector(sec.sector),
      name: sec.sector,
      category: SUPER_TO_CATEGORY[sec.super] || 'other',
      etfCount: sec.n,
      aum: sec.aum,
      topPerformer: {
        ticker: top?.sym || members[0]?.sym || '—',
        return5y: top?.ret5y || 0,
      },
      maxDrawdown: worstMdd,
      return1y: Number(avg1y.toFixed(1)),
      return5y: Number(avg5y.toFixed(1)),
    };
  });

  return { sectors, categories, etfs: data.etfs, total: data.n, updated: data.updated || '' };
}

export async function loadEtfAnalysesSnapshot() {
  const data = await fetchDataJson<EtfAnalysesFile>('/data/etf-analyses.json');
  return mapEtfAnalyses(data);
}

export async function loadEtfAnalyses() {
  if (!etfCache || !isFreshCache(etfCacheAt)) {
    etfCacheAt = Date.now();
    etfCache = (async () => {
      const data = await fetchDataJson<EtfAnalysesFile>('/data/etf-analyses.json');
      assertFreshEtfAnalyses(data);
      return mapEtfAnalyses(data);
    })().catch((err) => {
      etfCache = null;
      etfCacheAt = 0;
      throw err;
    });
  }
  return etfCache;
}

export function filterSortPageStocks(
  all: Stock[],
  opts: {
    searchQuery?: string;
    sortBy?: string;
    order?: 'asc' | 'desc';
    page?: number;
    limit?: number;
  }
): { stocks: Stock[]; total: number; page: number; limit: number; hasMore: boolean } {
  const q = (opts.searchQuery || '').trim().toLowerCase();
  let list = all;
  if (q) {
    list = list.filter(
      (s) =>
        s.symbol.toLowerCase().includes(q) ||
        s.name.toLowerCase().includes(q) ||
        s.sector.toLowerCase().includes(q)
    );
  }

  const sortBy = opts.sortBy || 'marketcap';
  const order = opts.order || 'desc';
  const dir = order === 'asc' ? 1 : -1;
  list = [...list].sort((a, b) => {
    const av =
      sortBy === 'price'
        ? a.price
        : sortBy === 'change'
          ? a.changePercent
          : sortBy === 'avgscore'
            ? a.avgScore
            : sortBy === 'volume'
              ? a.volume
              : a.marketCap;
    const bv =
      sortBy === 'price'
        ? b.price
        : sortBy === 'change'
          ? b.changePercent
          : sortBy === 'avgscore'
            ? b.avgScore
            : sortBy === 'volume'
              ? b.volume
              : b.marketCap;
    return (av - bv) * dir;
  });

  const page = Math.max(1, opts.page || 1);
  const limit = Math.max(1, opts.limit || 50);
  const start = (page - 1) * limit;
  const slice = list.slice(start, start + limit);
  return {
    stocks: slice,
    total: list.length,
    page,
    limit,
    hasMore: start + limit < list.length,
  };
}
