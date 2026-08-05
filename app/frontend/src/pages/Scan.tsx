import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ShieldAlert, SlidersHorizontal, Sparkles } from 'lucide-react';
import { StockGodShell } from '../components/layout/StockGodShell';
import { EmptyState, LoadingSpinner } from '../components/common';
import { RadarChart } from '../components/scan';
import { useStocksStore } from '../stores/stocksStore';
import { useDebounce } from '../hooks/useDebounce';
import { Stock } from '../types/stocks';
import { formatPrice, formatPercent, formatAUM, formatNumber, getChangeColorClass } from '../utils/format';

type RiskFilter = 'all' | 'only-risk' | 'hide-risk';
type JudgementFilter = 'all' | 'judged' | 'high-score' | 'consensus' | 'divergence';
type BullishLens = 'all' | keyof Stock['scores'];
type CapFilter = 'all' | 'large' | 'mid' | 'small';

const MARKET_TABS = [
  { id: 'us', label: '美股 · 全市场', count: 6151 },
  { id: 'cn', label: 'A 股 · 全市场', count: 5504 },
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

const scoreTooltip = (stock: Stock) =>
  `巴菲特 ${stock.scores.buffett} · 段永平 ${stock.scores.duanyongping} · Serenity ${stock.scores.serenity} · 德鲁肯米勒 ${stock.scores.druckenmiller} · 情绪资金面 ${stock.scores.sentiment} · 分歧 ${Math.round(stock.divergence ?? 0)}`;

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
  const navigate = useNavigate();
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
    fetchStocks,
    refreshQuotes,
    setMarket,
    setSortBy,
    setOrder,
    setSearchQuery,
    setPage,
    toggleWatch,
  } = useStocksStore();

  const [riskFilter, setRiskFilter] = useState<RiskFilter>('all');
  const [judgementFilter, setJudgementFilter] = useState<JudgementFilter>('all');
  const [bullishLens, setBullishLens] = useState<BullishLens>('all');
  const [capFilter, setCapFilter] = useState<CapFilter>('all');
  const [sectorFilter, setSectorFilter] = useState<string>('all');
  const [filtersOpen, setFiltersOpen] = useState(true);
  const [localPage, setLocalPage] = useState(1);
  const debouncedSearch = useDebounce(searchQuery, 300);

  useEffect(() => {
    fetchStocks();
  }, []);

  useEffect(() => {
    fetchStocks();
  }, [debouncedSearch, market, sortBy, order, page]);

  // Live quotes for current scan page (20s)
  useEffect(() => {
    let cancelled = false;
    const tick = async () => {
      if (cancelled || document.hidden) return;
      await refreshQuotes();
    };
    void tick();
    const id = window.setInterval(() => void tick(), 20_000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [refreshQuotes, market, page, debouncedSearch]);

  useEffect(() => {
    setLocalPage(1);
    setPage(1);
  }, [riskFilter, judgementFilter, bullishLens, capFilter, sectorFilter, debouncedSearch, market, sortBy, order]);

  const marketSource = market === 'cn' ? allCnStocks : allUsStocks;

  const scoreFilters = useMemo(
    () =>
      [
        { id: 'all' as JudgementFilter, label: `全部` },
        { id: 'judged' as JudgementFilter, label: `已判读(${judgedCount || '—'})` },
        { id: 'high-score' as JudgementFilter, label: `高分 均≥65(${highScoreCount || '—'})` },
        { id: 'consensus' as JudgementFilter, label: `共识好票(${consensusCount || '—'})` },
        { id: 'divergence' as JudgementFilter, label: `分歧大(${divergenceCount || '—'})` },
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
    const riskFlagged =
      stock.hasDilution || (stock.postAnalysisChange ?? 0) <= -4 || (stock.judged && stock.avgScore < 45);
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
    return list;
  }, [
    marketSource,
    stocks,
    riskFilter,
    judgementFilter,
    bullishLens,
    capFilter,
    sectorFilter,
    debouncedSearch,
    sortBy,
    order,
  ]);

  const pageSize = limit || 50;
  const activePage = localPage;
  const filteredStocks = useMemo(() => {
    const start = (activePage - 1) * pageSize;
    return fullyFiltered.slice(start, start + pageSize);
  }, [fullyFiltered, activePage, pageSize]);

  const apiTotal = fullyFiltered.length;
  const marketSnapshotTotal =
    market === 'cn' ? allCnStocks.length || 5504 : allUsStocks.length || 6151;
  const maxPage = Math.max(1, Math.ceil(apiTotal / pageSize));
  const goPage = (next: number) => {
    const clamped = Math.min(maxPage, Math.max(1, next));
    setLocalPage(clamped);
    setPage(clamped);
  };

  const handleSort = (field: 'marketcap' | 'price' | 'change' | 'avgscore' | 'volume') => {
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
        className={`inline-flex items-center transition hover:text-ink ${sortBy === field ? 'text-accent' : 'text-muted'}`}
        onClick={() => handleSort(field)}
      >
        {label}
        <span className="font-mono">{sortMark(field)}</span>
      </button>
    </th>
  );

  return (
    <StockGodShell title="列表" searchQuery={searchQuery} onSearchChange={setSearchQuery}>
      <div className="space-y-4">
        <header className="flex flex-wrap items-baseline gap-x-3 gap-y-1 border-b border-line pb-3">
          <h1 className="text-[22px] font-semibold tracking-tight text-ink">全市场扫描</h1>
          <p className="text-sm text-muted">美股 + A股 · 五方判读(段永平/巴菲特/Serenity/德鲁肯米勒/情绪)</p>
          <button onClick={() => navigate('/portfolio')} className="ml-auto text-xs font-medium text-accent hover:text-ink">我的观察列表</button>
        </header>

        <p className="rounded-lg border border-line bg-surface-2/60 px-3 py-2 text-[11px] leading-relaxed text-muted">
          AI 方法论模拟 —— 巴菲特 / 段永平 / 德鲁肯米勒 / Serenity 的评分由 AI 依据各自公开方法论生成,并非本人真实观点或持仓,亦不代表其本人。非投资建议。
        </p>

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
                {tab.label} {tab.count && tab.id === market ? tab.count : ''}
              </button>
            ))}
          </div>

          <div className="flex flex-wrap items-center gap-4 text-[13px]">
            <span className="font-medium text-ink">股票 {marketSnapshotTotal} 只</span>
            <span className="font-mono text-up">↑ {upCount || '—'} 涨</span>
            <span className="font-mono text-down">↓ {downCount || '—'} 跌</span>
            <span className="text-faint">数据 = /data/us-stocks + 五方面板 · 点列头排序</span>
          </div>
        </div>

        <section className="rounded-xl border border-line bg-surface p-4">
          <div className="mb-3 flex flex-wrap items-center gap-2">
            <button
              onClick={() => setFiltersOpen((value) => !value)}
              className="inline-flex items-center gap-1.5 rounded-lg border border-accent/30 bg-accent/10 px-3 py-1.5 text-sm font-semibold text-accent transition hover:bg-accent/15"
            >
              <SlidersHorizontal className="h-4 w-4" />
              筛选 {filtersOpen ? '▾' : '▸'}
            </button>
            <input
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
              placeholder="搜代码 / 公司 / 行业…"
              className="min-h-9 w-full rounded-lg border border-line bg-base px-3 text-sm text-ink outline-none placeholder:text-faint focus:border-accent/50 sm:w-64"
            />
          </div>

          {filtersOpen && (
            <div className="space-y-3">
              <div>
                <div className="mb-2 flex items-center gap-1.5 text-xs text-muted">
                  <ShieldAlert className="h-3.5 w-3.5 text-down" />
                  印股票风险:
                </div>
                <div className="flex flex-wrap items-center gap-1.5">
                  <Chip active={riskFilter === 'all'} danger onClick={() => setRiskFilter('all')}>
                    全部
                  </Chip>
                  <Chip active={riskFilter === 'only-risk'} onClick={() => setRiskFilter('only-risk')}>
                    只看({dilutionCount || '—'})
                  </Chip>
                  <Chip active={riskFilter === 'hide-risk'} onClick={() => setRiskFilter('hide-risk')}>
                    隐藏风险
                  </Chip>
                </div>
              </div>

              <div>
                <div className="mb-2 flex items-center gap-1.5 text-xs text-muted">
                  <Sparkles className="h-3.5 w-3.5 text-accent" />
                  五方判读:
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
                <div className="mb-2 text-xs text-muted">谁看多 ≥70:</div>
                <div className="flex flex-wrap items-center gap-1.5">
                  {BULLISH_LENSES.map((lens) => (
                    <Chip key={lens.id} active={bullishLens === lens.id} onClick={() => setBullishLens(lens.id)}>
                      {lens.label}
                    </Chip>
                  ))}
                </div>
              </div>

              <div>
                <div className="mb-2 text-xs text-muted">市值:</div>
                <div className="flex flex-wrap items-center gap-1.5">
                  {CAP_FILTERS.map((filter) => (
                    <Chip key={filter.id} active={capFilter === filter.id} onClick={() => setCapFilter(filter.id)}>
                      {filter.label}
                    </Chip>
                  ))}
                </div>
              </div>

              <div>
                <div className="mb-2 text-xs text-muted">行业:</div>
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
            title="加载失败"
            description={error}
            action={{
              label: '重试',
              onClick: fetchStocks,
            }}
          />
        )}

        {!loading && !error && filteredStocks.length === 0 && <EmptyState title="没有找到股票" description="尝试调整搜索或筛选条件" />}

        {filteredStocks.length > 0 && (
          <>
            <div className="divide-y divide-line/60 rounded-xl border border-line bg-surface sm:hidden">
              {filteredStocks.map((stock, index) => (
                <article key={stock.symbol} className="flex cursor-pointer items-center gap-2 px-3 py-2.5" onClick={() => navigate(`/stock/${stock.symbol}`)}>
                  <span className="w-5 shrink-0 text-right font-mono text-[11px] text-faint">{(activePage - 1) * pageSize + index + 1}</span>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-sm font-semibold text-ink">{stock.symbol}</span>
                      <span className={`font-mono text-xs font-semibold ${getChangeColorClass(stock.changePercent)}`}>{formatPercent(stock.changePercent)}</span>
                    </div>
                    <div className="truncate text-xs text-muted">{stock.name}</div>
                    <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-faint">
                      <span>{formatAUM(stock.marketCap)}</span>
                      <span className={getScoreColor(stock.avgScore)}>均 {Math.round(stock.avgScore)}</span>
                      <span>{stock.sector}</span>
                      <span className={getChangeColorClass(stock.postAnalysisChange ?? 0)}>判后 {formatPercent(stock.postAnalysisChange ?? 0)}</span>
                    </div>
                  </div>
                  <button
                    aria-label={stock.isWatched ? '移出 watchlist' : '加入 watchlist'}
                    className={`shrink-0 px-2 py-2 text-lg transition ${stock.isWatched ? 'text-accent' : 'text-faint hover:text-accent'}`}
                    onClick={(event) => {
                      event.stopPropagation();
                      toggleWatch(stock.symbol);
                    }}
                  >
                    {stock.isWatched ? '★' : '☆'}
                  </button>
                </article>
              ))}
            </div>

            <div className="hidden overflow-x-auto rounded-xl border border-line sm:block">
              <table className="w-full min-w-[980px] border-collapse bg-surface">
                <thead>
                  <tr className="border-b border-line bg-base/40">
                    <th className="px-3 py-2 text-right text-xs font-medium text-faint">#</th>
                    <th className="px-3 py-2 text-left text-xs font-medium text-muted">代码 / 名称</th>
                    {sortableHeader('价格', 'price', 'text-right')}
                    {sortableHeader('涨跌%', 'change', 'text-right')}
                    {sortableHeader('市值', 'marketcap', 'text-right')}
                    <th className="px-3 py-2 text-center text-xs font-medium text-muted">五方</th>
                    {sortableHeader('均分', 'avgscore', 'text-right')}
                    <th className="px-3 py-2 text-right text-xs font-medium text-muted">判读后</th>
                    {sortableHeader('成交量', 'volume', 'hidden text-right md:table-cell')}
                    <th className="px-3 py-2 text-left text-xs font-medium text-muted">行业</th>
                    <th className="px-2 py-2 text-center text-xs font-medium text-muted">☆</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredStocks.map((stock, index) => (
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
                      <td className="px-3 py-2 text-right font-mono text-sm text-ink">{formatPrice(stock.price)}</td>
                      <td className={`px-3 py-2 text-right font-mono text-sm font-semibold ${getChangeColorClass(stock.changePercent)}`}>
                        {formatPercent(stock.changePercent)}
                      </td>
                      <td className="px-3 py-2 text-right font-mono text-sm text-muted">{formatAUM(stock.marketCap)}</td>
                      <td className="px-3 py-2 text-center" title={scoreTooltip(stock)}>
                        <RadarChart scores={stock.scores} size={30} />
                      </td>
                      <td className={`px-3 py-2 text-right font-mono text-sm font-semibold ${getScoreColor(stock.avgScore)}`}>{Math.round(stock.avgScore)}</td>
                      <td className={`px-3 py-2 text-right font-mono text-sm font-semibold ${getChangeColorClass(stock.postAnalysisChange ?? 0)}`}>
                        {formatPercent(stock.postAnalysisChange ?? 0)}
                      </td>
                      <td className="hidden px-3 py-2 text-right font-mono text-xs text-muted md:table-cell">{formatNumber(stock.volume)}</td>
                      <td className="max-w-[150px] overflow-hidden truncate px-3 py-2 text-left text-xs text-muted">{stock.sector}</td>
                      <td className="px-2 py-2 text-center">
                        <button
                          aria-label={stock.isWatched ? '移出 watchlist' : '加入 watchlist'}
                          className={`rounded px-1.5 text-base transition ${stock.isWatched ? 'text-accent' : 'text-faint hover:text-accent'}`}
                          onClick={(event) => {
                            event.stopPropagation();
                            toggleWatch(stock.symbol);
                          }}
                        >
                          {stock.isWatched ? '★' : '☆'}
                        </button>
                      </td>
                    </tr>
                  ))}
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
                  « 首页
                </button>
                <button
                  className="inline-flex min-h-9 items-center rounded-md border border-line px-3 py-2 text-xs text-muted transition hover:bg-surface-2 hover:text-ink disabled:cursor-not-allowed disabled:border-line/50 disabled:text-faint/50"
                  onClick={() => goPage(activePage - 1)}
                  disabled={activePage === 1}
                >
                  ‹ 上一页
                </button>
                <span className="px-2 font-mono text-xs text-faint">
                  {activePage} / {maxPage}
                </span>
                <button
                  className="inline-flex min-h-9 items-center rounded-md border border-line px-3 py-2 text-xs text-muted transition hover:bg-surface-2 hover:text-ink disabled:cursor-not-allowed disabled:border-line/50 disabled:text-faint/50"
                  onClick={() => goPage(activePage + 1)}
                  disabled={activePage >= maxPage}
                >
                  下一页 ›
                </button>
                <button
                  className="hidden min-h-9 rounded-md border border-line px-3 py-2 text-xs text-muted transition hover:bg-surface-2 hover:text-ink disabled:cursor-not-allowed disabled:border-line/50 disabled:text-faint/50 sm:inline-flex"
                  onClick={() => goPage(maxPage)}
                  disabled={activePage >= maxPage}
                >
                  末页 »
                </button>
              </div>
            </div>
          </>
        )}

        <footer className="mt-16 border-t border-line pt-6 text-center text-xs text-faint">
          我不是神 · Not a Stock God · A股 + 美股统一五方判读（非投资建议）
        </footer>
      </div>
    </StockGodShell>
  );
};
