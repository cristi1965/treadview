import React, { useEffect, useMemo, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { Avatar } from '../components/Avatar';
import { RadarChart } from '../components/scan';
import { LoadingSpinner } from '../components/common';
import { DataStatus, type DataState } from '../components/common/DataStatus';
import { ScoreDetail, ScoreValidation, Stock } from '../types/stocks';
import { formatAUM, formatNumber, formatPercent, formatPrice, getChangeColorClass } from '../utils/format';
import { get as apiGet, getWithMeta, type APIResponseMeta, type APIResult } from '../utils/api';
import { findUsStock } from '../utils/stockgodData';
import { usePortfolioStore } from '../stores/portfolioStore';
import { StockLogo } from '../components/StockLogo';
import { TradingViewAdvancedChart } from '../components/TradingViewAdvancedChart';
import { BrainCircuit, ExternalLink } from 'lucide-react';
import { fetchQuoteResult, startQuotePolling } from '../utils/liveQuotes';

interface Holder {
  guruName: string;
  fundName: string;
  value: string;
  shares: string;
  change: string;
  weight: number;
  avatarCode: string;
  reportPeriod?: string;
  source?: string;
  sourceAsOf?: string;
  sourceURL?: string;
  stale?: boolean;
}

interface SearchResponse {
  results: Stock[];
  count: number;
}

interface FundamentalsResponse {
  symbol: string;
  name?: string;
  sector?: string;
  industry?: string;
  price?: number;
  trailingPE?: number;
  forwardPE?: number;
  priceToSales?: number;
  enterpriseToEbitda?: number;
  peg?: number;
  priceToBook?: number;
  grossMargin?: number;
  profitMargin?: number;
  roe?: number;
  revenueGrowth?: number;
  dividendYield?: number;
  beta?: number;
  targetMeanPrice?: number;
  recommendation?: string;
  fiftyTwoWeekHigh?: number;
  fiftyTwoWeekLow?: number;
  fiftyTwoWeekPos?: number;
	totalRevenue?: number;
	netIncome?: number;
	totalAssets?: number;
	totalLiabilities?: number;
	stockholdersEquity?: number;
  source?: string;
  fiscalPeriod: string;
  asOf: string;
	filingDate?: string;
	accession?: string;
	sourceURL?: string;
  fetchedAt: string;
  sourceLinks: Array<{ source: string; label: string; url: string }>;
  fieldSources: Record<string, { source: string; url: string; asOf: string; fiscalPeriod?: string; filingDate?: string; accession?: string; periodStart?: string }>;
}

interface ScoreEvidenceResponse {
  generated_at?: string;
  source?: string;
  method_version?: string;
  stocks: Record<string, { sc: number[]; div: number; detail?: ScoreDetail }>;
	validation?: ScoreValidation;
}

interface NewsItem {
  title: string;
  source: string;
  link?: string;
  published?: number | string;
}

interface EvidenceState {
  state: DataState;
  dataTime?: string;
  source?: string;
  message?: string;
  count?: number;
}

const requestWithEvidence = async <T,>(endpoint: string): Promise<{ result?: APIResult<T>; error?: string }> => {
  try {
    return { result: await getWithMeta<T>(endpoint) };
  } catch (error) {
    return { error: error instanceof Error ? error.message : '请求失败' };
  }
};

const evidenceState = (meta: APIResponseMeta | undefined, count: number, error?: string): EvidenceState => {
  if (error) return { state: 'error', message: error, count };
  if (!meta) return { state: 'unavailable', message: '缺少来源元数据', count };
  const details = [meta.staleReason, ...meta.partialErrors].filter(Boolean).join('；') || undefined;
  if (meta.stale) {
    return { state: 'stale', dataTime: meta.dataTime, source: meta.source, message: details, count };
  }
  if (!meta.source || !meta.dataTime || meta.dataTime === 'unknown') {
    return { state: 'unavailable', dataTime: meta.dataTime, source: meta.source, message: '来源或数据时间不可验证', count };
  }
  return { state: 'live', dataTime: meta.dataTime, source: meta.source, message: details, count };
};

type ScoreKey = 'buffett' | 'duanyongping' | 'serenity' | 'druckenmiller' | 'sentiment';

const fmtNum = (v?: number, digits = 1) =>
  typeof v === 'number' && Number.isFinite(v) && v !== 0 ? v.toFixed(digits) : '—';

const fmtPctRatio = (v?: number) =>
  typeof v === 'number' && Number.isFinite(v) && v !== 0 ? `${(v * 100).toFixed(1)}%` : '—';

const fmtMoney = (v?: number) =>
  typeof v === 'number' && Number.isFinite(v) && v !== 0 ? `$${v.toFixed(2)}` : '—';

const recommendLabel = (key?: string) => {
  const map: Record<string, string> = {
    strong_buy: '强烈买入',
    buy: '买入',
    hold: '持有',
    underperform: '跑输',
    sell: '卖出',
  };
  return key ? map[key] || key : '—';
};

const buildMetricRows = (m: FundamentalsResponse | null, livePrice?: number): Array<[string, string, string]> => {
  if (!m) {
    return [
      ['PE', '—', 'trailingPE'],
      ['前瞻PE', '—', 'forwardPE'],
      ['PS', '—', 'priceToSales'],
      ['EV/EBITDA', '—', 'enterpriseToEbitda'],
      ['PEG', '—', 'peg'],
      ['PB', '—', 'priceToBook'],
      ['毛利率', '—', 'grossMargin'],
      ['净利率', '—', 'profitMargin'],
      ['ROE', '—', 'roe'],
      ['营收增速', '—', 'revenueGrowth'],
      ['股息率', '—', 'dividendYield'],
      ['Beta', '—', 'beta'],
      ['分析师目标', '—', 'targetMeanPrice'],
      ['分析师评级', '—', 'recommendation'],
      ['52周位置', '—', 'fiftyTwoWeekPos'],
	  ['营收', '—', 'totalRevenue'],
	  ['净利润', '—', 'netIncome'],
	  ['总资产', '—', 'totalAssets'],
	  ['总负债', '—', 'totalLiabilities'],
	  ['股东权益', '—', 'stockholdersEquity'],
    ];
  }
  const price = livePrice || m.price || 0;
  let target = fmtMoney(m.targetMeanPrice);
  if (m.targetMeanPrice && price > 0) {
    const upside = ((m.targetMeanPrice - price) / price) * 100;
    target = `${fmtMoney(m.targetMeanPrice)} (${upside >= 0 ? '+' : ''}${upside.toFixed(0)}%)`;
  }
  // Yahoo dividendYield is sometimes already a fraction (0.005) and sometimes percent-like
  let div = '—';
  if (typeof m.dividendYield === 'number' && m.dividendYield !== 0) {
    div = m.dividendYield > 1 ? `${m.dividendYield.toFixed(2)}%` : fmtPctRatio(m.dividendYield);
  }
  return [
    ['PE', fmtNum(m.trailingPE, 1), 'trailingPE'],
    ['前瞻PE', fmtNum(m.forwardPE, 1), 'forwardPE'],
    ['PS', fmtNum(m.priceToSales, 1), 'priceToSales'],
    ['EV/EBITDA', fmtNum(m.enterpriseToEbitda, 1), 'enterpriseToEbitda'],
    ['PEG', fmtNum(m.peg, 1), 'peg'],
    ['PB', fmtNum(m.priceToBook, 1), 'priceToBook'],
    ['毛利率', fmtPctRatio(m.grossMargin), 'grossMargin'],
    ['净利率', fmtPctRatio(m.profitMargin), 'profitMargin'],
    ['ROE', fmtPctRatio(m.roe), 'roe'],
    ['营收增速', fmtPctRatio(m.revenueGrowth), 'revenueGrowth'],
    ['股息率', div, 'dividendYield'],
    ['Beta', fmtNum(m.beta, 1), 'beta'],
    ['分析师目标', target, 'targetMeanPrice'],
    ['分析师评级', recommendLabel(m.recommendation), 'recommendation'],
    ['52周位置', typeof m.fiftyTwoWeekPos === 'number' ? `${Math.round(m.fiftyTwoWeekPos)}%` : '—', 'fiftyTwoWeekPos'],
	['营收', typeof m.totalRevenue === 'number' ? formatAUM(m.totalRevenue) : '—', 'totalRevenue'],
	['净利润', typeof m.netIncome === 'number' ? formatAUM(m.netIncome) : '—', 'netIncome'],
	['总资产', typeof m.totalAssets === 'number' ? formatAUM(m.totalAssets) : '—', 'totalAssets'],
	['总负债', typeof m.totalLiabilities === 'number' ? formatAUM(m.totalLiabilities) : '—', 'totalLiabilities'],
	['股东权益', typeof m.stockholdersEquity === 'number' ? formatAUM(m.stockholdersEquity) : '—', 'stockholdersEquity'],
  ];
};

const scoreMeta: Array<{
  key: ScoreKey;
  label: string;
  framework: string;
  tagline: (s: number) => string;
}> = [
  {
    key: 'buffett',
    label: '巴菲特',
    framework: '价值 · 护城河',
    tagline: (s) => `面板分 ${s}`,
  },
  {
    key: 'duanyongping',
    label: '段永平',
    framework: '价值 · 商业模式',
    tagline: (s) => `面板分 ${s}`,
  },
  {
    key: 'serenity',
    label: 'Serenity',
    framework: 'alpha · 供应链瓶颈',
    tagline: (s) => `面板分 ${s}`,
  },
  {
    key: 'druckenmiller',
    label: '德鲁肯米勒',
    framework: 'alpha · 宏观流动性',
    tagline: (s) => `面板分 ${s}`,
  },
  {
    key: 'sentiment',
    label: '情绪资金面',
    framework: '盘口 · 资金流',
    tagline: (s) => `面板分 ${s}`,
  },
];

const scoreMethodDescription = (source?: string) => {
  const normalized = (source || '').toLowerCase();
  if (normalized.includes('heuristic')) {
    return '本地启发式评分：输入主要为行业、市值、单日涨跌和成交量，并做分布校准；不是逐股 AI 研究或真人观点。';
  }
  if (normalized.includes('llm')) {
    return '批量 LLM 方法论模拟：输入范围有限，不代表对应投资人本人观点，也没有经过收益回测。';
  }
  if (normalized.includes('seed')) {
    return '种子评分覆盖：来源于已有评分快照，需结合生成时间和最新基本面复核。';
  }
  return '评分方法来源未标明；仅可作为候选排序线索，不应直接用于仓位决策。';
};

const chainMap: Record<string, { label: string; upstream: string[]; downstream: string[] }> = {
  NVDA: {
    label: 'AI 算力 / 数据中心加速计算（GPU + 网络 + CUDA 生态）中游',
    upstream: ['TSM', 'ASML', 'MU', 'AMAT', 'ANET'],
    downstream: ['MSFT', 'AMZN', 'GOOGL', 'META', 'ORCL'],
  },
  AAPL: {
    label: '消费电子 / 生态平台 中游',
    upstream: ['TSM', 'QCOM', 'AVGO', 'SWKS'],
    downstream: ['AAPL'],
  },
  MSFT: {
    label: '云与企业软件 / AI 应用层',
    upstream: ['NVDA', 'AVGO', 'TSM'],
    downstream: ['MSFT', 'CRM', 'NOW'],
  },
};

const scoreColor = (score: number) => {
  if (score >= 70) return 'text-up';
  if (score >= 50) return 'text-accent';
  return 'text-faint';
};

const holderLabel = (name: string, fund: string) => {
  if (/pelosi|议员|congress|politician/i.test(`${name} ${fund}`)) return '政客 / 议员';
  if (/私募|private/i.test(fund)) return '私募大佬';
  if (/游资|龙虎/i.test(fund) || /^\d{6}/.test(name)) return '游资席位';
  return '美股大佬';
};

const actionLabel = (change: string) => {
  const c = (change || '').toLowerCase();
  if (/新|new/.test(c)) return '新建仓';
  if (/减|trim|sell|减持/.test(c)) return '减仓';
  if (/加|buy|加仓/.test(c)) return '加仓';
  return change || '持有';
};

export const StockDetail: React.FC = () => {
  const navigate = useNavigate();
  const { symbol } = useParams<{ symbol: string }>();
  const normalizedSymbol = (symbol || '').toUpperCase();
  const [holders, setHolders] = useState<Holder[]>([]);
  const [stock, setStock] = useState<Stock | null>(null);
  const [fundamentals, setFundamentals] = useState<FundamentalsResponse | null>(null);
  const [news, setNews] = useState<NewsItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [retryVersion, setRetryVersion] = useState(0);
  const [etfOwnersResult, setEtfOwnersResult] = useState<{
    owners: { etf: string; name: string; weight: number }[];
    updated?: string;
    dataMode?: 'current' | 'historical';
    degradedReason?: string;
  }>({ owners: [] });
  const [chartTimeframe, setChartTimeframe] = useState<'1D' | '5D' | '1M' | '6M' | '1Y' | 'MAX'>('1M');
  const [quoteStatus, setQuoteStatus] = useState<{
    state: DataState;
    dataTime?: string;
    source?: string;
    message?: string;
  }>({ state: 'loading' });
  const [fundamentalsStatus, setFundamentalsStatus] = useState<EvidenceState>({ state: 'loading' });
  const [newsStatus, setNewsStatus] = useState<EvidenceState>({ state: 'loading' });
  const [holdersStatus, setHoldersStatus] = useState<EvidenceState>({ state: 'loading' });
  const [scoreStatus, setScoreStatus] = useState<EvidenceState>({ state: 'loading' });
  const { addToWatchlist, removeFromWatchlist, isInWatchlist } = usePortfolioStore();
  const watched = isInWatchlist(normalizedSymbol);

  useEffect(() => {
    let cancelled = false;
    void apiGet<typeof etfOwnersResult>(
      `/api/etf/owners?sym=${encodeURIComponent(normalizedSymbol)}`
    )
      .then((res) => {
        if (!cancelled) setEtfOwnersResult({ ...res, owners: res?.owners || [] });
      })
      .catch(() => {
        if (!cancelled) setEtfOwnersResult({ owners: [] });
      });
    return () => {
      cancelled = true;
    };
  }, [normalizedSymbol]);

  useEffect(() => {
    let cancelled = false;

    const fetchData = async () => {
      let renderedFromSnapshot = false;
      try {
        setLoading(true);
        setLoadError('');
        setStock(null);
        setHolders([]);
        setFundamentals(null);
        setNews([]);
        setQuoteStatus({ state: 'loading' });
        setFundamentalsStatus({ state: 'loading' });
        setNewsStatus({ state: 'loading' });
        setHoldersStatus({ state: 'loading' });
        setScoreStatus({ state: 'loading' });

        const fromData = await findUsStock(normalizedSymbol).catch(() => null);
        if (cancelled) return;
        if (fromData) {
          renderedFromSnapshot = true;
          setStock(fromData);
          setQuoteStatus({
            state: fromData.quoteStale === false ? 'live' : 'stale',
            source: fromData.quoteSource,
            dataTime: fromData.quoteDataTime,
            message: fromData.quoteStale === false
              ? undefined
              : fromData.quoteStaleReason || '静态市场快照，仅供历史参考',
          });
          setLoading(false);
        }

        const [holdersCall, stockResponse, quoteCall, fundCall, newsCall, scoreCall] = await Promise.all([
          requestWithEvidence<Holder[]>(`/api/whales/stock/${normalizedSymbol}`),
          apiGet<SearchResponse>(`/api/stocks/search?q=${encodeURIComponent(normalizedSymbol)}`).catch(() => ({
            results: [],
            count: 0,
          })),
          fetchQuoteResult([normalizedSymbol])
            .then((result) => ({ result, error: '' }))
            .catch((error) => ({ result: null, error: error instanceof Error ? error.message : '报价请求失败' })),
          requestWithEvidence<FundamentalsResponse>(`/api/fundamentals?sym=${encodeURIComponent(normalizedSymbol)}`),
          requestWithEvidence<{ news?: NewsItem[] }>(`/api/news?sym=${encodeURIComponent(normalizedSymbol)}`),
          requestWithEvidence<ScoreEvidenceResponse>(`/api/panel-summary?sym=${encodeURIComponent(normalizedSymbol)}`),
        ]);

        if (cancelled) return;

        const holderRows = holdersCall.result?.data || [];
        const fundRes = fundCall.result?.data;
        const newsRows = newsCall.result?.data?.news || [];
        setHolders(holderRows);
        setHoldersStatus(evidenceState(holdersCall.result?.meta, holderRows.length, holdersCall.error));
        setFundamentalsStatus(evidenceState(fundCall.result?.meta, fundRes?.symbol ? 1 : 0, fundCall.error));
        setNewsStatus(evidenceState(newsCall.result?.meta, newsRows.length, newsCall.error));
        const scoreRow = scoreCall.result?.data?.stocks?.[normalizedSymbol];
        const nextScoreStatus = scoreRow
          ? evidenceState(scoreCall.result?.meta, 1, scoreCall.error)
          : { state: scoreCall.error ? 'error' as const : 'unavailable' as const, message: scoreCall.error || '当前标的无评分记录', count: 0 };
		const scoreValidated = scoreCall.result?.data?.validation?.status === 'validated';
		const scoreUsable = nextScoreStatus.state === 'live' && scoreValidated && Boolean(scoreRow?.sc?.length && scoreRow.sc.length >= 5);
		setScoreStatus(scoreValidated ? nextScoreStatus : {
		  state: 'unavailable', count: scoreRow ? 1 : 0,
		  message: scoreCall.result?.data?.validation?.reasons?.join('；') || '评分尚未完成真实历史验证，不能作为信号',
		});

        if (fundRes && fundRes.symbol) {
          setFundamentals(fundRes);
        }
        setNews(newsRows);

        const exactSnapshot =
          fromData ||
          stockResponse.results.find((item) => item.symbol.toLowerCase() === normalizedSymbol.toLowerCase()) ||
          stockResponse.results[0] ||
          null;
        const exact = exactSnapshot ? {
          ...exactSnapshot,
          scores: scoreUsable ? {
            buffett: Math.round(scoreRow!.sc[0] || 0),
            duanyongping: Math.round(scoreRow!.sc[1] || 0),
            serenity: Math.round(scoreRow!.sc[2] || 0),
            druckenmiller: Math.round(scoreRow!.sc[3] || 0),
            sentiment: Math.round(scoreRow!.sc[4] || 0),
          } : { buffett: 0, duanyongping: 0, serenity: 0, druckenmiller: 0, sentiment: 0 },
          avgScore: scoreUsable ? Math.round(scoreRow!.sc.reduce((sum, value) => sum + value, 0) / scoreRow!.sc.length) : 0,
          divergence: scoreUsable ? scoreRow!.div : 0,
          judged: scoreUsable,
          scoreDetail: scoreUsable ? scoreRow!.detail : undefined,
          scoreSource: scoreUsable ? scoreCall.result?.data?.source : undefined,
          scoreGeneratedAt: scoreUsable ? scoreCall.result?.data?.generated_at : undefined,
		  scoreValidation: scoreCall.result?.data?.validation,
        } : null;

        const quoteResponse = quoteCall.result;
        const quoteHasProvenance = Boolean(
          quoteResponse?.meta.source && quoteResponse.meta.dataTime && quoteResponse.meta.dataTime !== 'unknown'
        );
        const quote = quoteResponse?.quotes?.[normalizedSymbol] || quoteResponse?.quotes?.[normalizedSymbol.toUpperCase()];
        const quoteUsable = Boolean(quoteResponse && !quoteResponse.meta.stale && quoteHasProvenance && quote?.price && quote.price > 0);
        if (!quoteCall.result) {
          setQuoteStatus({ state: 'error', message: quoteCall.error || '报价请求失败' });
        } else {
          setQuoteStatus({
            state: quoteResponse.meta.stale ? 'stale' : quoteUsable ? 'live' : 'unavailable',
            dataTime: quoteResponse.meta.dataTime,
            source: quoteResponse.meta.source,
            message: [quoteResponse.meta.staleReason, ...quoteResponse.errors, quoteResponse.missing.length ? '当前标的报价缺失' : '']
              .filter(Boolean)
              .join('；') || undefined,
          });
        }
        if (exact && quoteUsable && quote) {
          const livePrice = quote.price || exact.price;
          const livePct = quote.pct ?? exact.changePercent;
          let marketCap = exact.marketCap;
          if (exact.price > 0 && livePrice > 0 && marketCap > 0) {
            marketCap = marketCap * (livePrice / exact.price);
          }
          setStock({
            ...exact,
            price: livePrice,
            changePercent: livePct,
            change: Number((((livePrice) * (livePct)) / 100).toFixed(4)),
            marketCap,
            sector: exact.sector || fundRes?.sector || exact.sector,
            industry: exact.industry || fundRes?.industry || exact.industry,
          });
        } else if (exact) {
          setStock(exact);
        } else if (fundRes?.symbol) {
          const livePrice = quoteUsable ? quote?.price || 0 : 0;
          const livePct = quote?.pct ?? 0;
          setStock({
            symbol: fundRes.symbol,
            name: fundRes.name || fundRes.symbol,
            price: livePrice,
            change: Number(((livePrice * livePct) / 100).toFixed(4)),
            changePercent: livePct,
            marketCap: 0,
            volume: 0,
            sector: fundRes.sector || 'Unknown',
            industry: fundRes.industry,
            scores: { buffett: 0, duanyongping: 0, serenity: 0, druckenmiller: 0, sentiment: 0 },
            avgScore: 0,
            divergence: 0,
            isWatched: false,
            judged: false,
            hasDilution: false,
            venue: 'us',
          });
        } else {
          setStock(null);
        }
      } catch (err) {
        console.error('Failed to fetch stock detail:', err);
        if (!cancelled) setLoadError(err instanceof Error ? err.message : '股票详情加载失败');
      } finally {
        if (!cancelled && !renderedFromSnapshot) {
          setLoading(false);
        }
      }
    };

    if (normalizedSymbol) {
      fetchData();
    }

    return () => {
      cancelled = true;
    };
  }, [normalizedSymbol, retryVersion]);

  // Keep price/pct live while on the detail page
  useEffect(() => {
    if (!normalizedSymbol) return;
    let cancelled = false;
    const tick = async () => {
      if (cancelled || document.hidden) return;
      try {
        const quoteResponse = await fetchQuoteResult([normalizedSymbol]);
        const quoteHasProvenance = Boolean(
          quoteResponse.meta.source && quoteResponse.meta.dataTime && quoteResponse.meta.dataTime !== 'unknown'
        );
        if (quoteResponse.meta.stale || !quoteHasProvenance) {
          setQuoteStatus({
            state: quoteResponse.meta.stale ? 'stale' : 'unavailable',
            dataTime: quoteResponse.meta.dataTime,
            source: quoteResponse.meta.source,
            message: [quoteResponse.meta.staleReason, ...quoteResponse.errors].filter(Boolean).join('；') || undefined,
          });
          return;
        }
        const quote =
          quoteResponse.quotes?.[normalizedSymbol] ||
          quoteResponse.quotes?.[normalizedSymbol.toUpperCase()];
        if (!quote || !(quote.price > 0)) {
          setQuoteStatus({ state: 'unavailable', message: '当前标的报价缺失' });
          return;
        }
        setQuoteStatus({
          state: 'live',
          dataTime: quoteResponse.meta.dataTime,
          source: quoteResponse.meta.source,
          message: quoteResponse.errors.join('；') || undefined,
        });
        setStock((prev) =>
          prev
            ? {
                ...prev,
                price: quote.price,
                changePercent: quote.pct ?? prev.changePercent,
                change: Number((((quote.price || prev.price) * (quote.pct ?? prev.changePercent)) / 100).toFixed(4)),
                marketCap:
                  prev.price > 0 && quote.price > 0 && prev.marketCap > 0
                    ? prev.marketCap * (quote.price / prev.price)
                    : prev.marketCap,
              }
            : prev
        );
      } catch (error) {
        setQuoteStatus({ state: 'error', message: error instanceof Error ? error.message : '报价刷新失败' });
      }
    };
    const stopPolling = startQuotePolling(tick, 15_000, { immediate: false });
    return () => {
      cancelled = true;
      stopPolling();
    };
  }, [normalizedSymbol]);

  const topHoldersWeight = useMemo(() => holders.slice(0, 10).reduce((sum, holder) => sum + (holder.weight || 0), 0), [holders]);
  const metricRows = useMemo(() => buildMetricRows(fundamentals, stock?.price), [fundamentals, stock?.price]);
  const hasAnyData = !!stock || holders.length > 0;
  const chain = chainMap[normalizedSymbol] || {
    label: stock?.sector ? `${stock.sector} · 人工维护产业链索引` : '人工维护产业链索引',
    upstream: ['TSM', 'ASML', 'AVGO'],
    downstream: ['MSFT', 'AMZN', 'GOOGL'],
  };
  const divergence = stock?.divergence ?? Math.max(...Object.values(stock?.scores || { a: 0 }), 0) - Math.min(...Object.values(stock?.scores || { a: 0 }), 0);

  if (loading) {
    return (
      <div className="flex min-h-[460px] items-center justify-center">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  return (
    <div className="text-foreground">
      {loadError && (
        <div role="alert" className="mb-4 flex flex-wrap items-center justify-between gap-3 border-l-2 border-rose-500 bg-rose-500/5 px-3 py-2 text-sm text-rose-200">
          <span>部分详情请求失败：{loadError}</span>
          <button type="button" onClick={() => setRetryVersion((value) => value + 1)} className="min-h-11 border border-rose-500/40 px-3 font-semibold sm:min-h-9">重试</button>
        </div>
      )}
      {!stock && loadError && (
        <div className="mb-5 border-y border-line py-8 text-center">
          <h1 className="text-lg font-semibold text-ink">暂时无法确认 {normalizedSymbol}</h1>
          <p className="mt-2 text-sm text-muted">这是加载失败，不代表该标的不存在。</p>
        </div>
      )}
      <header className="mb-5 border-b border-line pb-4">
        <div className="mb-3 flex flex-wrap items-center gap-2 text-xs text-muted">
          <Link to="/market" className="transition hover:text-ink">实验行情</Link>
          <span className="text-faint">/</span>
          <Link to="/scan" className="transition hover:text-ink">扫描</Link>
          <span className="text-faint">/</span>
          <Link to="/portfolio" className="transition hover:text-ink">观察</Link>
          <span className="text-faint">/</span>
          <span className="text-ink">{normalizedSymbol}</span>
        </div>

        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="flex items-start gap-3">
            <StockLogo symbol={normalizedSymbol} size={48} className="mt-1" />
            <div>
              <div className="flex flex-wrap items-center gap-2">
                <h1 className="text-2xl font-semibold tracking-tight text-ink sm:text-3xl">
                  {stock?.name || normalizedSymbol}
                </h1>
                <span className="rounded border border-line px-1.5 py-0.5 font-mono text-[11px] text-muted">{normalizedSymbol}</span>
                <button
                  type="button"
                  onClick={() => {
                    if (watched) removeFromWatchlist(normalizedSymbol);
                    else addToWatchlist(normalizedSymbol, stock?.name || normalizedSymbol);
                  }}
                  className={`rounded-md border px-2.5 py-1 text-xs font-medium transition ${
                    watched ? 'border-accent/40 bg-accent/10 text-accent' : 'border-line text-muted hover:text-ink'
                  }`}
                >
                  {watched ? '★ 已收藏' : '☆ 收藏'}
                </button>

                {/* 🚀 驾驶舱 10-Agent 研判 */}
                <button
                  type="button"
                  onClick={() => navigate(`/dashboard?symbol=${normalizedSymbol}`)}
                  className="inline-flex items-center gap-1 rounded-md border border-indigo-500/40 bg-indigo-500/10 px-2.5 py-1 text-xs font-semibold text-indigo-300 transition hover:bg-indigo-500/20 active:scale-95"
                >
                  <BrainCircuit size={13} className="text-indigo-400" /> 驾驶舱深度研判
                </button>
              </div>
              <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted">
                {stock?.marketCap ? <span>市值 {formatAUM(stock.marketCap)}</span> : null}
                {stock?.sector ? <span className="rounded bg-surface-3 px-1.5 py-0.5">{stock.sector}</span> : null}
              </div>
            </div>
          </div>

          {stock && (
            <div className={`rounded-lg border px-3 py-2 text-right ${quoteStatus.state === 'live' ? 'border-transparent' : 'border-amber-500/40 bg-amber-500/10'}`}>
              {quoteStatus.state !== 'live' && (
                <div className="mb-1 text-[11px] font-bold text-amber-300">静态快照价格 · 不可用于决策</div>
              )}
              <div className={`font-mono text-2xl font-semibold ${quoteStatus.state === 'live' ? 'text-ink' : 'text-amber-200'}`}>
                {stock.price > 0 ? formatPrice(stock.price) : '—'}
              </div>
              <div className={`font-mono text-sm font-semibold ${quoteStatus.state === 'live' ? getChangeColorClass(stock.changePercent) : 'text-faint'}`}>
                {quoteStatus.state === 'live' ? formatPercent(stock.changePercent) : '涨跌已隐藏'}
              </div>
              <div className="mt-1 flex justify-end">
                <DataStatus
                  state={quoteStatus.state}
                  label={quoteStatus.state === 'live' ? '当前报价' : '报价已过期或来源不可用'}
                  dataTime={quoteStatus.dataTime}
                  source={quoteStatus.source}
                  message={quoteStatus.message}
                  compact
                />
              </div>
            </div>
          )}
        </div>
      </header>

      {!hasAnyData ? (
        <div className="flex min-h-[420px] items-center justify-center rounded-xl border border-line bg-surface text-center">
          <div>
            <p className="font-mono text-5xl font-semibold tracking-tight text-accent">404</p>
            <h2 className="mt-4 text-lg font-semibold text-ink">没找到这个页面</h2>
            <p className="mt-1.5 text-sm text-muted">链接可能失效,或股票代码不存在。</p>
            <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
              <Link to="/market" className="rounded-lg border border-accent/30 bg-accent/10 px-4 py-2 text-sm font-semibold text-accent transition hover:bg-accent/15">回实验行情</Link>
              <Link to="/scan" className="rounded-lg border border-line bg-surface px-4 py-2 text-sm text-muted transition hover:text-ink">去列表找票</Link>
            </div>
          </div>
        </div>
      ) : (
        <div className="space-y-5">
          {stock && (
            <>
              <section className="border-y border-line py-4">
                <div className="mb-2 flex flex-wrap items-baseline justify-between gap-2">
                  <h2 className="text-sm font-semibold text-ink">研究证据账本</h2>
                  <span className="text-[11px] text-faint">状态按各接口独立判断，不以页面加载时间冒充数据时间</span>
                </div>
                {[
                  { label: '行情', status: quoteStatus },
                  { label: '基本面', status: fundamentalsStatus },
                  { label: '新闻', status: newsStatus },
                  { label: '机构披露', status: holdersStatus },
                  { label: '五方评分', status: scoreStatus },
                ].map(({ label, status }) => (
                  <div key={label} className="grid gap-1 border-t border-line/60 py-2.5 sm:grid-cols-[92px_1fr] sm:items-start">
                    <div className="text-xs font-medium text-ink">
                      {label}
                      {'count' in status && typeof status.count === 'number' ? <span className="ml-1 text-faint">({status.count})</span> : null}
                    </div>
                    <DataStatus
                      state={status.state}
                      dataTime={status.dataTime}
                      source={status.source}
                      message={status.message}
                      compact
                    />
                  </div>
                ))}
              </section>

              <div className="grid gap-3 md:grid-cols-4">
                <section className="rounded-xl border border-line bg-surface p-4">
                  <div className="text-xs text-faint">市值</div>
                  <div className="mt-2 font-mono text-xl font-semibold text-ink">{formatAUM(stock.marketCap)}</div>
                </section>
                <section className="rounded-xl border border-line bg-surface p-4">
                  <div className="text-xs text-faint">成交量</div>
                  <div className="mt-2 font-mono text-xl font-semibold text-ink">{formatNumber(stock.volume)}</div>
                </section>
                <section className="rounded-xl border border-line bg-surface p-4">
                  <div className="text-xs text-faint">五方均分</div>
				  <div className={`mt-2 font-mono text-xl font-semibold ${scoreStatus.state === 'live' ? scoreColor(stock.avgScore) : 'text-faint'}`}>
					{scoreStatus.state === 'live' ? Math.round(stock.avgScore) : '—'}
				  </div>
                </section>
                <section className="rounded-xl border border-line bg-surface p-4">
                  <div className="text-xs text-faint">分歧</div>
				  <div className="mt-2 font-mono text-xl font-semibold text-accent">{scoreStatus.state === 'live' ? Math.round(divergence) : '—'}</div>
                </section>
              </div>

              {/* TradingView Advanced Real-Time Chart & AI Pivot Levels */}
              <TradingViewAdvancedChart
                symbol={normalizedSymbol}
                price={stock?.price || fundamentals?.price || 0}
                height={480}
              />

              <section className="rounded-xl border border-line bg-surface p-4">
                <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <h2 className="text-sm font-semibold text-ink">真实基本面</h2>
                    <p className="mt-1 text-xs text-muted">财报期 {fundamentals?.fiscalPeriod || 'unknown'} · 底层数据截至 {fundamentals?.asOf || 'unknown'} · 抓取于 {fundamentals?.fetchedAt || 'unknown'}</p>
					<p className="mt-1 text-[11px] text-faint">申报日 {fundamentals?.filingDate || 'unknown'} · accession {fundamentals?.accession || 'unknown'}</p>
                    <div className="mt-1 flex flex-wrap gap-2 text-[11px] text-faint">
                      {(fundamentals?.sourceLinks || []).map((link) => (
                        <a key={link.url} href={link.url} target="_blank" rel="noreferrer" className="text-accent hover:underline">
                          {link.label} <ExternalLink size={10} className="inline" />
                        </a>
                      ))}
                      {!fundamentals?.sourceLinks?.length && <span>来源链接 unknown</span>}
                    </div>
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-5">
                  {metricRows.map(([label, value, field]) => {
                    const highlight = ['毛利率', '净利率', 'ROE', '营收增速', '前瞻PE'].includes(label);
					const provenance = value === '—' ? undefined : fundamentals?.fieldSources?.[field];
                    return (
					  <div key={label} title={provenance ? `${provenance.source} · ${provenance.periodStart || '?'} 至 ${provenance.asOf} · ${provenance.accession || ''}` : '字段来源 unknown'} className={`rounded-lg border px-3 py-2 ${highlight ? 'border-accent/30 bg-accent/5' : 'border-line bg-base'}`}>
                        <div className="text-[11px] text-faint">{label}</div>
                        <div className="mt-1 font-mono text-sm font-semibold text-ink">{value}</div>
                        <div className="mt-1 truncate text-[10px] text-faint">{provenance?.source || 'source unknown'}</div>
                      </div>
                    );
                  })}
                </div>
              </section>

              <section className="rounded-xl border border-line bg-surface p-4">
                {scoreStatus.state !== 'live' ? (
                  <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 p-4">
                    <DataStatus
                      state={scoreStatus.state}
                      label="五方评分来源不可用"
                      dataTime={scoreStatus.dataTime}
                      source={scoreStatus.source}
                      message={scoreStatus.message}
                    />
                    <p className="mt-2 text-xs text-muted">旧快照分数和派生结论已隐藏，来源恢复且数据非过期后才显示。</p>
                  </div>
                ) : (
                  <>
                <div className="flex flex-wrap items-center justify-between gap-4">
                  <div>
                    <h2 className="text-sm font-semibold text-ink">5 方独立评分</h2>
                    <p className="mt-1 text-xs text-muted">五种公开方法论模拟评分,分歧越大越值得复核</p>
                  </div>
                  <div className="flex items-center gap-3">
                    <RadarChart scores={stock.scores} size={72} />
                    <div className="text-right">
                      <div className="text-lg font-semibold text-ink">均分 {Math.round(stock.avgScore)}</div>
                      <div className="text-xs text-muted">分歧 {Math.round(divergence)}</div>
                    </div>
                  </div>
                </div>

                <p className="mt-3 text-[11px] leading-relaxed text-faint">
                  {scoreMethodDescription(stock.scoreSource)} 来源 {stock.scoreSource || '未知'}
                  {stock.scoreGeneratedAt ? ` · 生成于 ${new Date(stock.scoreGeneratedAt).toLocaleString()}` : ' · 生成时间未知'}。
                  并非本人真实观点、发言或持仓,亦不代表其本人。仅供研究参考,非投资建议。
                </p>

				{stock.scoreValidation && (
				  <div className={`mt-3 border-y border-line py-3 text-xs ${stock.scoreValidation.status === 'validated' ? 'text-muted' : 'text-accent'}`}>
					<div className="flex flex-wrap items-center justify-between gap-2">
					  <strong>历史验证: {stock.scoreValidation.status}</strong>
					  <span className="font-mono">{stock.scoreValidation.method || 'unknown'}</span>
					</div>
					<p className="mt-2">
					  训练样本 {stock.scoreValidation.train_samples ?? 0} · 测试样本 {stock.scoreValidation.test_samples ?? 0}
					  {stock.scoreValidation.cutoff ? ` · 切分 ${stock.scoreValidation.cutoff}` : ''}
					</p>
					<p className="mt-1">基准: {stock.scoreValidation.benchmark || 'unknown'}</p>
					{stock.scoreValidation.reasons?.length ? <p className="mt-1">原因: {stock.scoreValidation.reasons.join('；')}</p> : null}
					{stock.scoreValidation.status !== 'validated' && <p className="mt-1 font-semibold">未验证评分不作为投资信号。</p>}
				  </div>
				)}

                {stock.scoreDetail ? (
                  <div className="mt-3 border-y border-line py-3 text-xs">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <span className="font-semibold text-ink">可复现评分明细</span>
                      <span className="font-mono text-faint">{stock.scoreDetail.method_version}</span>
                    </div>
                    <div className="mt-2 grid grid-cols-2 gap-2 text-muted sm:grid-cols-4">
                      <span>快照价 <b className="font-mono text-ink">{stock.scoreDetail.inputs.price}</b></span>
                      <span>单日涨跌 <b className="font-mono text-ink">{stock.scoreDetail.inputs.day_pct}%</b></span>
                      <span>市值 <b className="font-mono text-ink">{stock.scoreDetail.inputs.market_cap_b}B</b></span>
                      <span>成交量 <b className="font-mono text-ink">{stock.scoreDetail.inputs.volume}</b></span>
                    </div>
                    <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-muted">
                      {scoreMeta.map((item, index) => (
                        <span key={item.key}>{item.label} <b className="font-mono text-ink">{stock.scoreDetail?.raw_scores[index]} → {stock.scoreDetail?.final_scores[index]}</b></span>
                      ))}
                    </div>
                    <div className="mt-3 grid gap-2 md:grid-cols-2">
                      {scoreMeta.map((item) => {
                        const componentKey = item.key === 'duanyongping' ? 'duan' : item.key;
                        const components = stock.scoreDetail?.components?.[componentKey] || [];
                        return (
                          <details key={item.key} className="border-t border-line/70 pt-2">
                            <summary className="cursor-pointer font-medium text-ink">{item.label} 组件贡献 ({components.length})</summary>
                            <div className="mt-1 space-y-1 font-mono text-[10px] text-muted">
                              {components.map((component, index) => (
                                <div key={`${component.rule}-${index}`} className="flex items-center justify-between gap-3">
                                  <span className="break-all">{component.rule}</span>
                                  <span className={component.value >= 0 ? 'text-up' : 'text-down'}>{component.value >= 0 ? '+' : ''}{component.value}</span>
                                </div>
                              ))}
                              {!components.length && <span className="text-faint">组件明细 unknown</span>}
                            </div>
                          </details>
                        );
                      })}
                    </div>
                    <p className={`mt-2 text-[11px] ${stock.scoreDetail.reproduces_panel ? 'text-up' : 'text-accent'}`}>
                      {stock.scoreDetail.reproduces_panel
                        ? '复算结果与当前面板分一致。'
                        : '当前面板分含种子或 LLM 覆盖，以下启发式复算不能解释覆盖后的最终分。'}
                    </p>
                    <p className="mt-2 break-words font-mono text-[10px] text-faint">{stock.scoreDetail.calibration}</p>
                  </div>
                ) : (
                  <p className="mt-3 text-xs text-accent">当前评分文件缺少可复现输入明细，不能仅凭最终分核对计算。</p>
                )}

                <div className="mt-4 space-y-3">
                  {scoreMeta.map((item) => {
                    const score = stock.scores[item.key] ?? 0;
                    return (
                      <article key={item.key} className="rounded-lg border border-line bg-base px-4 py-3">
                        <div className="flex flex-wrap items-baseline justify-between gap-2">
                          <div>
                            <span className="text-sm font-semibold text-ink">{item.label}</span>
                            <span className="ml-2 text-[11px] text-faint">{item.framework}</span>
                          </div>
                          <div className="text-right">
                            <span className={`font-mono text-lg font-semibold ${scoreColor(score)}`}>{score}</span>
                            <div className="text-[11px] text-muted">{item.tagline(score)}</div>
                          </div>
                        </div>
                        <p className="mt-2 text-xs leading-relaxed text-muted">
                          仅展示当前可验证面板分，不在前端生成买卖结论或模仿投资人发言。
                        </p>
                      </article>
                    );
                  })}
                </div>

                <div className="mt-4 rounded-lg border border-line bg-base px-4 py-3">
                  <h3 className="text-xs font-semibold text-ink">分歧焦点</h3>
                  <p className="mt-2 text-xs leading-relaxed text-muted">仅展示五方数值差异，不在前端按阈值生成偏多、买卖或仓位结论。</p>
                </div>
                  </>
                )}
              </section>

              <section className="rounded-xl border border-line bg-surface p-4">
                <h2 className="text-sm font-semibold text-ink">产业链定位</h2>
                <p className="mt-1 text-xs text-muted">{chain.label}</p>
                <div className="mt-3 grid gap-3 md:grid-cols-2">
                  <div>
                    <div className="mb-1.5 text-[11px] font-medium uppercase tracking-wider text-faint">上游</div>
                    <div className="flex flex-wrap gap-2">
                      {chain.upstream.map((sym) => (
                        <Link key={sym} to={`/stock/${sym}`} className="rounded-md border border-line bg-base px-2.5 py-1 font-mono text-xs font-semibold text-ink hover:border-accent/40 hover:text-accent">
                          {sym}
                        </Link>
                      ))}
                    </div>
                  </div>
                  <div>
                    <div className="mb-1.5 text-[11px] font-medium uppercase tracking-wider text-faint">下游</div>
                    <div className="flex flex-wrap gap-2">
                      {chain.downstream.map((sym) => (
                        <Link key={sym} to={`/stock/${sym}`} className="rounded-md border border-line bg-base px-2.5 py-1 font-mono text-xs font-semibold text-ink hover:border-accent/40 hover:text-accent">
                          {sym}
                        </Link>
                      ))}
                    </div>
                  </div>
                </div>
              </section>

              {etfOwnersResult.owners.length > 0 ? (
                <section className="rounded-xl border border-line bg-surface p-4">
                  <div className="mb-3 flex items-center justify-between">
                    <h2 className="text-sm font-semibold text-ink">相关 ETF</h2>
                    <span className="text-[11px] text-faint">{etfOwnersResult.dataMode === 'historical' ? `历史持仓参考 · ${etfOwnersResult.updated || '时间未知'}` : 'ETF 持仓来源'}</span>
                  </div>
                  {etfOwnersResult.dataMode === 'historical' ? <p className="mb-3 text-xs text-amber-200">{etfOwnersResult.degradedReason || '数据已超过当前模式年龄阈值'}</p> : null}
                  <div className="flex flex-wrap gap-2">
                    {etfOwnersResult.owners.map((o) => (
                      <Link
                        key={o.etf}
                        to={`/stock/${o.etf}`}
                        className="rounded-lg border border-line bg-base px-2.5 py-1.5 text-xs transition hover:border-accent/40 hover:text-accent"
                        title={o.name}
                      >
                        <span className="font-mono font-semibold">{o.etf}</span>
                        <span className="ml-1.5 text-faint">{o.weight}%</span>
                      </Link>
                    ))}
                  </div>
                </section>
              ) : null}

              <section className="rounded-xl border border-line bg-surface p-4">
                <div className="mb-3 flex items-center justify-between">
                  <h2 className="text-sm font-semibold text-ink">近期新闻</h2>
                  <span className="text-[11px] text-faint">Yahoo Finance</span>
                </div>
                <div className="space-y-2">
                  {news.length === 0 ? (
                    <div className="rounded-lg border border-line bg-base px-3 py-2 text-xs text-faint">暂无相关新闻</div>
                  ) : (
                    news.map((item) => (
                      <div key={`${item.title}-${item.published || ''}`} className="rounded-lg border border-line bg-base px-3 py-2 text-xs leading-relaxed text-muted">
                        {item.link ? (
                          <a href={item.link} target="_blank" rel="noreferrer" className="text-ink hover:text-accent hover:underline">
                            {item.title}
                          </a>
                        ) : (
                          <span className="text-ink">{item.title}</span>
                        )}
                        <span className="ml-2 text-faint">· {item.source}</span>
                      </div>
                    ))
                  )}
                </div>
              </section>

              <section className="rounded-xl border border-line bg-surface p-4">
                <h2 className="text-sm font-semibold text-ink">外部资料 & 持仓</h2>
                <p className="mt-1 text-xs text-muted">五方独立判读见上。也可以去这些地方核对行情与基本面:</p>
                <div className="mt-3 flex flex-wrap gap-3 text-sm">
                  <a className="text-accent hover:underline" href={`https://xueqiu.com/S/${normalizedSymbol}`} target="_blank" rel="noreferrer">雪球 ↗</a>
                  <a className="text-accent hover:underline" href={`https://quote.eastmoney.com/us/${normalizedSymbol}.html`} target="_blank" rel="noreferrer">东方财富 ↗</a>
                  <Link to="/market" className="text-muted hover:text-ink">← 回实验行情</Link>
                </div>
              </section>
            </>
          )}

          <section className="rounded-xl border border-line bg-surface">
            <header className="flex flex-wrap items-center justify-between gap-3 border-b border-line px-4 py-3">
              <div>
                <h2 className="text-sm font-semibold text-ink">谁在持仓</h2>
                <p className="mt-0.5 text-xs text-muted">当前有 {holders.length} 家顶级投资机构/大户公开披露持有 {normalizedSymbol}。</p>
              </div>
              <div className="flex items-center gap-3 text-xs">
                <div className="text-right text-faint">
                  <div>前十集中度</div>
                  <div className="font-mono text-sm text-ink">{topHoldersWeight.toFixed(2)}%</div>
                </div>
                <Link to="/whales" className="rounded-md border border-line px-2.5 py-1 text-accent hover:border-accent/40">
                  全部聪明钱 →
                </Link>
              </div>
            </header>

            {holders.length === 0 ? (
              <div className="px-4 py-12 text-center text-sm text-muted">暂无该股票的持仓披露数据</div>
            ) : (
              <div className="divide-y divide-line/60 overflow-hidden">
                {holders.slice(0, 16).map((holder, idx) => (
                  <div key={idx} className="flex items-center gap-3 px-4 py-3 transition hover:bg-surface-2">
                    <Avatar
                      name={holder.guruName}
                      slug={holder.fundName}
                      letter={holder.avatarCode || holder.guruName.charAt(0)}
                      size={40}
                    />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-[13px] font-semibold text-ink">
                        <span className="mr-1.5 text-[11px] font-normal text-faint">{holderLabel(holder.guruName, holder.fundName)}</span>
                        {holder.guruName}
                      </div>
                      <div className="truncate text-[11px] text-faint">{holder.fundName}</div>
                    </div>
                    <div className="hidden text-right sm:block">
                      <div className="font-mono text-[12px] text-muted">{holder.value}</div>
                      <div className="text-[11px] text-faint">{holder.shares}</div>
                    </div>
                    <div className="w-20 text-right">
                      <div className="font-mono text-[13px] font-semibold text-ink">{holder.weight.toFixed(2)}%</div>
                      <div className={`text-[11px] ${/减|trim|sell/.test(holder.change.toLowerCase()) ? 'text-down' : 'text-up'}`}>
                        {actionLabel(holder.change)}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
            <p className="border-t border-line px-4 py-3 text-[11px] leading-relaxed text-faint">
              占比 &lt;1% 多为试探仓 / 研究标记,非重仓 conviction —— 看占比,别看名单。
            </p>
          </section>

          <p className="text-[11px] leading-relaxed text-faint">
            行情来源以页面状态标识为准；机构持仓来源 = {holdersStatus.source || holders.map((holder) => holder.source).filter(Boolean).join(' / ') || '未知'}，数据时间 = {holdersStatus.dataTime || holders.find((holder) => holder.reportPeriod || holder.sourceAsOf)?.reportPeriod || holders.find((holder) => holder.sourceAsOf)?.sourceAsOf || '未知'}。披露持仓并非实时仓位；AI 判读为方法论模拟 · 非投资建议。
          </p>
        </div>
      )}

    </div>
  );
};
