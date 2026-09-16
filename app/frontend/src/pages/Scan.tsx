import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { ShieldAlert, SlidersHorizontal, Sparkles } from 'lucide-react';
import { StockGodShell } from '../components/layout/StockGodShell';
import { EmptyState, LoadingSpinner } from '../components/common';
import { DataStatus } from '../components/common/DataStatus';
import { RadarChart } from '../components/scan';
import { useStocksStore } from '../stores/stocksStore';
import { usePortfolioStore } from '../stores/portfolioStore';
import { useDebounce } from '../hooks/useDebounce';
import { Stock } from '../types/stocks';
import { formatPrice, formatPercent, formatAUM, formatNumber, getChangeColorClass } from '../utils/format';
import { SCAN_PRESETS } from '../utils/presets';
import { useI18n } from '../i18n';
import { startQuotePolling } from '../utils/liveQuotes';

type RiskFilter = 'all' | 'only-risk' | 'hide-risk';
type JudgementFilter = 'all' | 'judged' | 'high-score' | 'consensus' | 'divergence';
type BullishLens = 'all' | keyof Stock['scores'];
type CapFilter = 'all' | 'large' | 'mid' | 'small';

const MARKET_TABS = [
  { id: 'us', label: '美股 · 全市场' },
  { id: 'cn', label: 'A 股 · 全市场' },
];

const BULLISH_LENSES: Array<{ id: BullishLens; label: string }> = [
  { id: 'all', label: '不限' },
  { id: 'buffett', label: '巴菲特' },
  { id: 'duanyongping', label: '段永平' },
  { id: 'serenity', label: 'Serenity' },
  { id: 'druckenmiller', label: '德鲁肯米勒' },
  { id: 'sentiment', label: '情绪资金面' },
];

const CAP_FILTERS: Array<{ id: CapFilter; label: string }> = [
  { id: 'all', label: '全部市值' },
  { id: 'large', label: '大盘 ≥$10B' },
  { id: 'mid', label: '中盘 $2–10B' },
  { id: 'small', label: '小盘 <$2B' },
];

const LIVE_US_SECTOR_ORDER = [
  'Finance',
  'Consumer Discretionary',
  'Health Care',
  'Technology',
  'Industrials',
  'Real Estate',
  'Energy',
  'Utilities',
  'Consumer Staples',
  'Basic Materials',
  'Telecommunications',
  'Miscellaneous',
  'Communication Services',
];

const scoreTooltip = (stock: Stock) => stock.judged
  ? `巴菲特 ${stock.scores.buffett} · 段永平 ${stock.scores.duanyongping} · Serenity ${stock.scores.serenity} · 德鲁肯米勒 ${stock.scores.druckenmiller} · 情绪资金面 ${stock.scores.sentiment} · 分歧 ${Math.round(stock.divergence ?? 0)}`
  : '评分来源不可用';

const getScoreColor = (score: number): string => {
  if (score >= 70) return 'text-up';
  if (score >= 50) return 'text-accent';
  return 'text-faint';
};

const capMatches = (marketCap: number, filter: CapFilter) => {
  if (filter === 'large') return marketCap >= 10_000_000_000;
  if (filter === 'mid') return marketCap >= 2_000_000_000 && marketCap < 10_000_000_000;
  if (filter === 'small') return marketCap < 2_000_000_000;
  return true;
};

const hasCurrentQuote = (stock: Stock) => Boolean(
  stock.quoteStale === false && stock.quoteSource && stock.quoteDataTime && stock.quoteDataTime !== 'unknown'
);

const Chip: React.FC<{ active: boolean; danger?: boolean; children: React.ReactNode; onClick: () => void }> = ({
  active,
  danger,
  children,
  onClick,
}) => (
  <button
    onClick={onClick}
    className={`min-h-9 rounded-full px-2.5 py-1.5 text-[11px] transition ${
      active
        ? danger
          ? 'border border-down/40 bg-down/10 text-down'
          : 'bg-surface-3 text-ink'
        : 'bg-surface-2 text-muted hover:bg-line hover:text-ink'
    }`}
  >
    {children}
  </button>
);

export const Scan: React.FC = () => {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const {
    stocks,
    allUsStocks,
    allCnStocks,
    loading,
    error,
    page,
    limit,
    market,
    sortBy,
    order,
    searchQuery,
    upCount,
    downCount,
    judgedCount,
    highScoreCount,
    consensusCount,
    divergenceCount,
    dilutionCount,
    quoteState,
    quoteSource,
    quoteDataTime,
    quoteMessage,
    fetchStocks,
    refreshQuotes,
    setMarket,
    setSortBy,
    setOrder,
    setSearchQuery,
    setPage,
    toggleWatch,
  } = useStocksStore();
  const watchlist = usePortfolioStore((state) => state.watchlist);

  const [riskFilter, setRiskFilter] = useState<RiskFilter>('all');
  const [judgementFilter, setJudgementFilter] = useState<JudgementFilter>('all');
  const [bullishLens, setBullishLens] = useState<BullishLens>('all');
  const [capFilter, setCapFilter] = useState<CapFilter>('all');
  const [sectorFilter, setSectorFilter] = useState<string>('all');
  const [filtersOpen, setFiltersOpen] = useState(true);
  const [localPage, setLocalPage] = useState(1);
  const [presetId, setPresetId] = useState<string>('none');
  const debouncedSearch = useDebounce(searchQuery, 300);
  const quoteUsable = quoteState === 'live';
  const watchedSymbols = useMemo(
    () => new Set(watchlist.map((item) => item.symbol.toUpperCase())),
    [watchlist]
  );

  useEffect(() => {
    const marketParam = searchParams.get('market');
    const requestedMarket = marketParam === 'cn' || marketParam === 'us' ? marketParam : null;
    const requestedQuery = searchParams.get('q')?.trim() || '';
    if (requestedMarket) setMarket(requestedMarket);
    if (searchParams.has('q')) setSearchQuery(requestedQuery);
  }, [searchParams, setMarket, setSearchQuery]);

  useEffect(() => {
    fetchStocks();
  }, [debouncedSearch, market, sortBy, order, page]);

  // The visible scan page is a lower-frequency tier than positions and stock detail.
  useEffect(() => {
    return startQuotePolling(refreshQuotes, 45_000, { immediate: false });
  }, [refreshQuotes, market, page, debouncedSearch]);

  useEffect(() => {
    setLocalPage(1);
    setPage(1);
  }, [riskFilter, judgementFilter, bullishLens, capFilter, sectorFilter, presetId, debouncedSearch, market, sortBy, order]);

  const marketSource = market === 'cn' ? allCnStocks : allUsStocks;

  const scoreFilters = useMemo(
    () =>
      [
        { id: 'all' as JudgementFilter, label: t('scan.jAll') },
        { id: 'judged' as JudgementFilter, label: t('scan.jJudged', { n: judgedCount || '—' }) },
        { id: 'high-score' as JudgementFilter, label: t('scan.jHigh', { n: highScoreCount || '—' }) },
        { id: 'consensus' as JudgementFilter, label: t('scan.jCons', { n: consensusCount || '—' }) },
        { id: 'divergence' as JudgementFilter, label: t('scan.jDiv', { n: divergenceCount || '—' }) },
      ] as const,
    [judgedCount, highScoreCount, consensusCount, divergenceCount]
  );

  const sectorCounts = useMemo(() => {
    const source = marketSource.length > 0 ? marketSource : stocks;
    const map = new Map<string, number>();
    for (const s of source) {
      map.set(s.sector, (map.get(s.sector) || 0) + 1);
    }
    if (market === 'us') {
      return LIVE_US_SECTOR_ORDER
        .map((sector) => [sector, map.get(sector) || 0] as [string, number])
        .filter(([, count]) => count > 0);
    }
    return [...map.entries()].sort((a, b) => b[1] - a[1]).slice(0, 16);
  }, [marketSource, stocks]);

  const applyRowFilters = (stock: Stock) => {
    const preset = SCAN_PRESETS.find((p) => p.id === presetId);
    if (preset && !preset.filter(stock)) return false;
    const riskFlagged = stock.hasDilution || (stock.judged && stock.avgScore < 45);
    if (riskFilter === 'only-risk' && !riskFlagged) return false;
    if (riskFilter === 'hide-risk' && riskFlagged) return false;
    if (judgementFilter === 'judged' && !(stock.judged || stock.avgScore > 0)) return false;
    if (judgementFilter === 'high-score' && stock.avgScore < 65) return false;
    if (judgementFilter === 'consensus') {
      const values = Object.values(stock.scores);
      if (stock.avgScore < 60 || values.filter((value) => value >= 60).length < 4) return false;
    }
    if (judgementFilter === 'divergence' && (stock.divergence ?? 0) < 30) return false;
    if (bullishLens !== 'all' && stock.scores[bullishLens] < 70) return false;
    if (!capMatches(stock.marketCap, capFilter)) return false;
    if (sectorFilter !== 'all' && !stock.sector.toLowerCase().includes(sectorFilter.toLowerCase())) return false;
    return true;
  };

  const fullyFiltered = useMemo(() => {
    const q = debouncedSearch.trim().toLowerCase();
    const source = marketSource.length > 0 ? marketSource : stocks;
    let list = source.filter(applyRowFilters);
    if (q) {
      list = list.filter(
        (s) =>
          s.symbol.toLowerCase().includes(q) ||
          s.name.toLowerCase().includes(q) ||
          s.sector.toLowerCase().includes(q)
      );
    }
    const dir = order === 'asc' ? 1 : -1;
    list = [...list].sort((a, b) => {
      const effectiveSort = !quoteUsable && (sortBy === 'price' || sortBy === 'change') ? 'marketcap' : sortBy;
      const av =
        effectiveSort === 'price'
          ? a.price
          : effectiveSort === 'change'
            ? a.changePercent
            : effectiveSort === 'avgscore'
              ? a.avgScore
              : effectiveSort === 'volume'
                ? a.volume
                : a.marketCap;
      const bv =
        effectiveSort === 'price'
          ? b.price
          : effectiveSort === 'change'
            ? b.changePercent
            : effectiveSort === 'avgscore'
              ? b.avgScore
              : effectiveSort === 'volume'
                ? b.volume
                : b.marketCap;
      return (av - bv) * dir;
    });
    return list;
  }, [
    marketSource,
    stocks,
    riskFilter,
    judgementFilter,
    bullishLens,
    capFilter,
    sectorFilter,
    presetId,
    marketSource,
    stocks,
    debouncedSearch,
    sortBy,
    order,
    quoteUsable,
  ]);

  const pageSize = limit || 50;
  const activePage = localPage;
  const filteredStocks = useMemo(() => {
    const start = (activePage - 1) * pageSize;
    return fullyFiltered.slice(start, start + pageSize);
  }, [fullyFiltered, activePage, pageSize]);

  const apiTotal = fullyFiltered.length;
  const marketSnapshotTotal = market === 'cn' ? allCnStocks.length : allUsStocks.length;
  const maxPage = Math.max(1, Math.ceil(apiTotal / pageSize));
  const goPage = (next: number) => {
    const clamped = Math.min(maxPage, Math.max(1, next));
    setLocalPage(clamped);
    setPage(clamped);
  };

  const handleSort = (field: 'marketcap' | 'price' | 'change' | 'avgscore' | 'volume') => {
    if (!quoteUsable && (field === 'price' || field === 'change')) return;
    if (sortBy === field) {
      setOrder(order === 'asc' ? 'desc' : 'asc');
    } else {
      setSortBy(field);
      setOrder('desc');
    }
  };

  const sortMark = (field: string) => (sortBy === field ? (order === 'desc' ? ' ↓' : ' ↑') : '');

  const sortableHeader = (label: string, field: 'marketcap' | 'price' | 'change' | 'avgscore' | 'volume', className = '') => (
    <th className={`px-3 py-2 text-xs font-medium ${className}`}>
      <button
        disabled={!quoteUsable && (field === 'price' || field === 'change')}
        className={`inline-flex items-center transition disabled:cursor-not-allowed disabled:opacity-40 ${sortBy === field ? 'text-accent' : 'text-muted hover:text-ink'}`}
        onClick={() => handleSort(field)}
      >
        {label}
        <span className="font-mono">{sortMark(field)}</span>
      </button>
    </th>
  );

  return (
    <StockGodShell title={t('nav.scan')} searchQuery={searchQuery} onSearchChange={setSearchQuery}>
      <div className="space-y-4">
        <header className="flex flex-wrap items-baseline gap-x-3 gap-y-1 border-b border-line pb-3">
          <h1 className="text-[22px] font-semibold tracking-tight text-ink">{t('scan.title')}</h1>
          <p className="text-sm text-muted">{t('scan.sub')}</p>
          <button onClick={() => navigate('/portfolio')} className="ml-auto text-xs font-medium text-accent hover:text-ink">{t('scan.watch')}</button>
        </header>

        <p className="rounded-lg border border-line bg-surface-2/60 px-3 py-2 text-[11px] leading-relaxed text-muted">
          {t('home.aiDisclaimer')}
        </p>

        <DataStatus
          state={quoteState}
          label={quoteUsable ? '列表报价可用' : '列表报价不可用于当前决策'}
          source={quoteSource}
          dataTime={quoteDataTime}
          message={quoteMessage || (!quoteUsable ? '静态清单仍可浏览，但价格、涨跌和判后变化已隐藏。' : undefined)}
          onRetry={() => void refreshQuotes()}
        />

        <div className="space-y-2.5">
          <div className="inline-flex rounded-lg border border-line bg-surface p-0.5 text-sm">
            {MARKET_TABS.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setMarket(tab.id)}
                className={`rounded-md px-3 py-1.5 font-medium transition sm:px-4 ${
                  market === tab.id ? 'bg-surface-3 text-ink' : 'text-muted hover:text-ink'
                }`}
              >
              {t(tab.id === 'us' ? 'scan.tabUS' : 'scan.tabCN')} {tab.id === market && marketSnapshotTotal > 0 ? marketSnapshotTotal : ''}
              </button>
            ))}
          </div>

          <div className="flex flex-wrap gap-1.5">
            <button
              type="button"
              onClick={() => setPresetId('none')}
              className={`rounded-full px-2.5 py-1 text-[11px] ${presetId === 'none' ? 'bg-surface-3 text-ink' : 'bg-surface-2 text-muted'}`}
            >
              {t('scan.presetAll')}
            </button>
            {SCAN_PRESETS.filter((p) => p.market === market).map((p) => (
              <button
                key={p.id}
                type="button"
                title={p.describe}
                onClick={() => {
                  setPresetId(p.id);
                  if (p.sortBy) setSortBy(p.sortBy);
                }}
                className={`rounded-full px-2.5 py-1 text-[11px] ${presetId === p.id ? 'bg-accent/15 text-accent' : 'bg-surface-2 text-muted'}`}
              >
                {p.label}
              </button>
            ))}
          </div>

          <div className="flex flex-wrap items-center gap-4 text-[13px]">
            <span className="font-medium text-ink">{t('scan.count', { n: marketSnapshotTotal })}</span>
            <span className="font-mono text-up">↑ {t('scan.up', { n: quoteUsable ? upCount : '—' })}</span>
            <span className="font-mono text-down">↓ {t('scan.down', { n: quoteUsable ? downCount : '—' })}</span>
            <span className="text-faint">{t('scan.dataHint')}</span>
          </div>
        </div>

        <section className="rounded-xl border border-line bg-surface p-4">
          <div className="mb-3 flex flex-wrap items-center gap-2">
            <button
              onClick={() => setFiltersOpen((value) => !value)}
              className="inline-flex items-center gap-1.5 rounded-lg border border-accent/30 bg-accent/10 px-3 py-1.5 text-sm font-semibold text-accent transition hover:bg-accent/15"
            >
              <SlidersHorizontal className="h-4 w-4" />
              {t('scan.filter')} {filtersOpen ? '▾' : '▸'}
            </button>
            <input
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
              placeholder={t('scan.searchPh')}
              className="min-h-9 w-full rounded-lg border border-line bg-base px-3 text-sm text-ink outline-none placeholder:text-faint focus:border-accent/50 sm:w-64"
            />
          </div>

          {filtersOpen && (
            <div className="space-y-3">
              <div>
                <div className="mb-2 flex items-center gap-1.5 text-xs text-muted">
                  <ShieldAlert className="h-3.5 w-3.5 text-down" />
                  {t('scan.risk')}
                </div>
                <div className="flex flex-wrap items-center gap-1.5">
                  <Chip active={riskFilter === 'all'} danger onClick={() => setRiskFilter('all')}>
                    全部
                  </Chip>
                  <Chip active={riskFilter === 'only-risk'} onClick={() => setRiskFilter('only-risk')}>
                    {t('scan.onlyRisk', { n: dilutionCount || '—' })}
                  </Chip>
                  <Chip active={riskFilter === 'hide-risk'} onClick={() => setRiskFilter('hide-risk')}>
                    {t('scan.hideRisk')}
                  </Chip>
                </div>
              </div>

              <div>
                <div className="mb-2 flex items-center gap-1.5 text-xs text-muted">
                  <Sparkles className="h-3.5 w-3.5 text-accent" />
                  {t('scan.five')}
                </div>
                <div className="flex flex-wrap items-center gap-1.5">
                  {scoreFilters.map((filter) => (
                    <Chip key={filter.id} active={judgementFilter === filter.id} onClick={() => setJudgementFilter(filter.id)}>
                      {filter.label}
                    </Chip>
                  ))}
                </div>
              </div>

              <div>
                <div className="mb-2 text-xs text-muted">{t('scan.bullish')}</div>
                <div className="flex flex-wrap items-center gap-1.5">
                  {BULLISH_LENSES.map((lens) => (
                    <Chip key={lens.id} active={bullishLens === lens.id} onClick={() => setBullishLens(lens.id)}>
                      {lens.label}
                    </Chip>
                  ))}
                </div>
              </div>

              <div>
                <div className="mb-2 text-xs text-muted">{t('scan.cap')}</div>
                <div className="flex flex-wrap items-center gap-1.5">
                  {CAP_FILTERS.map((filter) => (
                    <Chip key={filter.id} active={capFilter === filter.id} onClick={() => setCapFilter(filter.id)}>
                      {filter.label}
                    </Chip>
                  ))}
                </div>
              </div>

              <div>
                <div className="mb-2 text-xs text-muted">{t('scan.sector')}</div>
                <div className="flex max-h-none flex-wrap items-center gap-1.5 sm:max-h-[60px] sm:overflow-y-auto">
                  <Chip active={sectorFilter === 'all'} onClick={() => setSectorFilter('all')}>
                    全部
                  </Chip>
                  {sectorCounts.map(([sector, count]) => (
                    <Chip key={sector} active={sectorFilter === sector} onClick={() => setSectorFilter(sector)}>
                      {sector} {count}
                    </Chip>
                  ))}
                </div>
              </div>
            </div>
          )}
        </section>

        {loading && stocks.length === 0 && (
          <div className="flex items-center justify-center rounded-xl border border-line bg-surface py-16">
            <LoadingSpinner size="lg" />
          </div>
        )}

        {error && (
          <EmptyState
            title={t('scan.loadFail')}
            description={error}
            action={{
              label: t('scan.retry'),
              onClick: fetchStocks,
            }}
          />
        )}

        {!loading && !error && filteredStocks.length === 0 && <EmptyState title={t('scan.empty')} description={t('scan.emptyDesc')} />}

        {filteredStocks.length > 0 && (
          <>
            <div className="divide-y divide-line/60 rounded-xl border border-line bg-surface sm:hidden">
              {filteredStocks.map((stock, index) => {
                const isWatched = watchedSymbols.has(stock.symbol.toUpperCase());
                return (
                <article key={stock.symbol} className="flex cursor-pointer items-center gap-2 px-3 py-2.5" onClick={() => navigate(`/stock/${stock.symbol}`)}>
                  <span className="w-5 shrink-0 text-right font-mono text-[11px] text-faint">{(activePage - 1) * pageSize + index + 1}</span>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-sm font-semibold text-ink">{stock.symbol}</span>
                      <span className={`font-mono text-xs font-semibold ${quoteUsable && hasCurrentQuote(stock) ? getChangeColorClass(stock.changePercent) : 'text-faint'}`}>
                        {quoteUsable && hasCurrentQuote(stock) ? formatPercent(stock.changePercent) : '报价过期'}
                      </span>
                    </div>
                    <div className="truncate text-xs text-muted">{stock.name}</div>
                    <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-faint">
                      <span>{formatAUM(stock.marketCap)}</span>
                      <span className={stock.judged ? getScoreColor(stock.avgScore) : 'text-faint'}>均 {stock.judged ? Math.round(stock.avgScore) : '—'}</span>
                      <span>{stock.sector}</span>
                    </div>
                  </div>
                  <button
                    aria-label={isWatched ? '移出 watchlist' : '加入 watchlist'}
                    className={`shrink-0 px-2 py-2 text-lg transition ${isWatched ? 'text-accent' : 'text-faint hover:text-accent'}`}
                    onClick={(event) => {
                      event.stopPropagation();
                      toggleWatch(stock.symbol);
                    }}
                  >
                    {isWatched ? '★' : '☆'}
                  </button>
                </article>
                );
              })}
            </div>

            <div className="hidden overflow-x-auto rounded-xl border border-line sm:block">
              <table className="w-full min-w-[980px] border-collapse bg-surface">
                <thead>
                  <tr className="border-b border-line bg-base/40">
                    <th className="px-3 py-2 text-right text-xs font-medium text-faint">#</th>
                    <th className="px-3 py-2 text-left text-xs font-medium text-muted">{t('scan.colSym')}</th>
                    {sortableHeader(t('scan.colPx'), 'price', 'text-right')}
                    {sortableHeader(t('scan.colChg'), 'change', 'text-right')}
                    {sortableHeader(t('scan.colMcap'), 'marketcap', 'text-right')}
                    <th className="px-3 py-2 text-center text-xs font-medium text-muted">{t('scan.colFive')}</th>
                    {sortableHeader(t('scan.colAvg'), 'avgscore', 'text-right')}
                    {sortableHeader(t('scan.colVol'), 'volume', 'hidden text-right md:table-cell')}
                    <th className="px-3 py-2 text-left text-xs font-medium text-muted">{t('scan.colSec')}</th>
                    <th className="px-2 py-2 text-center text-xs font-medium text-muted">☆</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredStocks.map((stock, index) => {
                    const isWatched = watchedSymbols.has(stock.symbol.toUpperCase());
                    return (
                    <tr
                      key={stock.symbol}
                      className="border-b border-line/60 transition last:border-b-0 hover:bg-surface-2"
                      onClick={() => navigate(`/stock/${stock.symbol}`)}
                    >
                      <td className="px-3 py-2 text-right font-mono text-xs text-faint">{(activePage - 1) * pageSize + index + 1}</td>
                      <td className="px-3 py-2">
                        <div className="font-mono text-sm font-semibold text-ink">{stock.symbol}</div>
                        <div className="max-w-[210px] truncate text-xs text-muted">{stock.name}</div>
                      </td>
                      <td className="px-3 py-2 text-right font-mono text-sm text-ink">{quoteUsable && hasCurrentQuote(stock) ? formatPrice(stock.price) : '—'}</td>
                      <td className={`px-3 py-2 text-right font-mono text-sm font-semibold ${quoteUsable && hasCurrentQuote(stock) ? getChangeColorClass(stock.changePercent) : 'text-faint'}`}>
                        {quoteUsable && hasCurrentQuote(stock) ? formatPercent(stock.changePercent) : '—'}
                      </td>
                      <td className="px-3 py-2 text-right font-mono text-sm text-muted">{formatAUM(stock.marketCap)}</td>
                      <td className="px-3 py-2 text-center" title={scoreTooltip(stock)}>
                        {stock.judged ? <RadarChart scores={stock.scores} size={30} /> : <span className="text-faint">—</span>}
                      </td>
                      <td className={`px-3 py-2 text-right font-mono text-sm font-semibold ${stock.judged ? getScoreColor(stock.avgScore) : 'text-faint'}`}>
                        {stock.judged ? Math.round(stock.avgScore) : '—'}
                      </td>
                      <td className="hidden px-3 py-2 text-right font-mono text-xs text-muted md:table-cell">{formatNumber(stock.volume)}</td>
                      <td className="max-w-[150px] overflow-hidden truncate px-3 py-2 text-left text-xs text-muted">{stock.sector}</td>
                      <td className="px-2 py-2 text-center">
                        <button
                          aria-label={isWatched ? '移出 watchlist' : '加入 watchlist'}
                          className={`rounded px-1.5 text-base transition ${isWatched ? 'text-accent' : 'text-faint hover:text-accent'}`}
                          onClick={(event) => {
                            event.stopPropagation();
                            toggleWatch(stock.symbol);
                          }}
                        >
                          {isWatched ? '★' : '☆'}
                        </button>
                      </td>
                    </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-3 text-sm">
              <div className="text-muted">
                第 {apiTotal === 0 ? 0 : (activePage - 1) * pageSize + 1}-{Math.min(activePage * pageSize, apiTotal)} 只 / 共 {apiTotal} 只
              </div>
              <div className="flex items-center gap-1">
                <button
                  className="hidden min-h-9 rounded-md border border-line px-3 py-2 text-xs text-muted transition hover:bg-surface-2 hover:text-ink disabled:cursor-not-allowed disabled:border-line/50 disabled:text-faint/50 sm:inline-flex"
                  onClick={() => goPage(1)}
                  disabled={activePage === 1}
                >
                  {t('scan.first')}
                </button>
                <button
                  className="inline-flex min-h-9 items-center rounded-md border border-line px-3 py-2 text-xs text-muted transition hover:bg-surface-2 hover:text-ink disabled:cursor-not-allowed disabled:border-line/50 disabled:text-faint/50"
                  onClick={() => goPage(activePage - 1)}
                  disabled={activePage === 1}
                >
                  {t('scan.prev')}
                </button>
                <span className="px-2 font-mono text-xs text-faint">
                  {activePage} / {maxPage}
                </span>
                <button
                  className="inline-flex min-h-9 items-center rounded-md border border-line px-3 py-2 text-xs text-muted transition hover:bg-surface-2 hover:text-ink disabled:cursor-not-allowed disabled:border-line/50 disabled:text-faint/50"
                  onClick={() => goPage(activePage + 1)}
                  disabled={activePage >= maxPage}
                >
                  {t('scan.next')}
                </button>
                <button
                  className="hidden min-h-9 rounded-md border border-line px-3 py-2 text-xs text-muted transition hover:bg-surface-2 hover:text-ink disabled:cursor-not-allowed disabled:border-line/50 disabled:text-faint/50 sm:inline-flex"
                  onClick={() => goPage(maxPage)}
                  disabled={activePage >= maxPage}
                >
                  {t('scan.last')}
                </button>
              </div>
            </div>
          </>
        )}

        <footer className="mt-16 border-t border-line pt-6 text-center text-xs text-faint">
          {t('scan.footer')}
        </footer>
      </div>
    </StockGodShell>
  );
};
