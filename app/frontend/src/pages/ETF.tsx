import React, { useEffect, useRef, useState } from 'react';
import { ChevronDown, ChevronLeft, RefreshCw } from 'lucide-react';
import { Link, useNavigate, useParams, useLocation } from 'react-router-dom';
import { StockGodShell } from '../components/layout/StockGodShell';
import { LoadingSpinner } from '../components/common';
import { useETFStore, type ETFRefreshDiagnostics } from '../stores/etfStore';
import { ETFCategory, ETFSortBy, ETFSector } from '../types/etf';
import { get, post } from '../utils/api';
import { ETF_PRESETS } from '../utils/presets';
import { CnToolbox } from '../components/cn/CnToolbox';
import { I18nKey, useI18n } from '../i18n';
import { useAdminSession } from '../hooks/useAdminSession';

const CATEGORIES: Array<{ id: ETFCategory | 'all'; label: string }> = [
  { id: 'all', label: '全部' },
  { id: 'broad', label: '宽基' },
  { id: 'industry', label: '行业' },
  { id: 'theme', label: '主题' },
  { id: 'factor', label: '因子/策略' },
  { id: 'bond', label: '债券' },
  { id: 'commodity', label: '商品' },
  { id: 'leveraged', label: '杠杆/反向' },
  { id: 'other', label: '其他' },
];

const SORT_OPTIONS: Array<{ id: ETFSortBy; label: string }> = [
  { id: 'aum', label: '规模' },
  { id: 'return1y', label: '近1年' },
  { id: 'return5y', label: '近5年' },
  { id: 'drawdown', label: '抗跌' },
];

interface ETFHoldingDetail {
  sym: string;
  name: string;
  venue: string;
  updated: string;
  holdings: Array<{ symbol: string; name: string; weight: number }>;
  dataMode?: 'current' | 'historical';
  degradedReason?: string;
}

type DetailLookup = {
  id: string;
  status: 'idle' | 'loading' | 'ready' | 'error';
  data: ETFHoldingDetail | null;
  error?: string;
};

const HistoricalDataNotice: React.FC<{ updated: string }> = ({ updated }) => (
  <div className="rounded-lg border border-amber-500/35 bg-amber-500/5 px-4 py-3 text-sm leading-relaxed text-amber-200">
    <strong>历史快照模式</strong>
    <span className="ml-2 text-muted">
      实时 ETF 数据不可用，当前展示 {updated || '时间未标记'} 的本地研究快照，不代表实时价格或当前收益。
    </span>
  </div>
);

const formatAum = (value: number) => {
  if (value >= 1_000_000_000_000) return `$${(value / 1_000_000_000_000).toFixed(1)}T`;
  if (value >= 1_000_000_000) return `$${Math.round(value / 1_000_000_000)}B`;
  if (value >= 1_000_000) return `$${Math.round(value / 1_000_000)}M`;
  return `$${Math.round(value)}`;
};

const formatAge = (seconds?: number) => {
  if (seconds == null || seconds < 0) return '未知';
  if (seconds < 3600) return `${Math.max(1, Math.round(seconds / 60))} 分钟`;
  if (seconds < 86400) return `${Math.round(seconds / 3600)} 小时`;
  return `${Math.round(seconds / 86400)} 天`;
};

const formatDiagnosticTime = (value?: string) => value ? new Date(value).toLocaleString() : '暂无';

const ETFDiagnostics: React.FC<{ diagnostics: ETFRefreshDiagnostics | null }> = ({ diagnostics }) => {
  if (!diagnostics) return null;
  const overAge = diagnostics.ageSeconds < 0 || diagnostics.ageSeconds > diagnostics.maxAgeSeconds;
  return (
    <div className={`grid gap-2 border px-3 py-2 text-xs sm:grid-cols-2 lg:grid-cols-4 ${overAge ? 'border-amber-500/35 bg-amber-500/5 text-amber-200' : 'border-line bg-surface text-muted'}`}>
      <span>快照年龄 <strong>{formatAge(diagnostics.ageSeconds)}</strong> / 阈值 {formatAge(diagnostics.maxAgeSeconds)}</span>
      <span>最近尝试 <strong>{formatDiagnosticTime(diagnostics.lastAttemptAt)}</strong></span>
      <span>下次计划 <strong>{formatDiagnosticTime(diagnostics.nextScheduledAt)}</strong></span>
      <span>覆盖 <strong>{diagnostics.scopeCompleted ?? 0}/{diagnostics.scopeRequested ?? 0}</strong>{diagnostics.lastAttemptStatus ? ` · ${diagnostics.lastAttemptStatus}` : ''}</span>
      {diagnostics.lastAttemptError ? <span className="sm:col-span-2 lg:col-span-4 text-down">{diagnostics.lastAttemptError}</span> : null}
    </div>
  );
};

const SectorRow: React.FC<{ sector: ETFSector }> = ({ sector }) => {
  const navigate = useNavigate();
  return (
  <section className="overflow-hidden rounded-xl border border-line bg-surface">
    <button onClick={() => navigate(`/etf/${sector.id}`)} className="flex w-full items-center gap-2 px-4 py-3 text-left transition hover:bg-surface-2">
      <span className="text-[15px] font-semibold text-ink">{sector.name}</span>
      <span className="rounded-full bg-surface-3 px-2 py-0.5 text-[10px] font-medium text-muted">
        {sector.etfCount}
      </span>
      <span className="font-mono text-[11px] text-faint">{formatAum(sector.aum)}</span>
      <span className="flex-1" />
      <span className="hidden items-center gap-1 text-[11px] sm:flex">
        <span className="text-faint">5年最强</span>
        <span className="font-mono font-semibold text-ink">{sector.topPerformer.ticker}</span>
        <span className="font-mono font-semibold text-up">+{Math.round(sector.topPerformer.return5y)}%</span>
      </span>
      <span className="ml-2 hidden items-center gap-1 text-[11px] sm:flex">
        <span className="text-faint">最深回撤</span>
        <span className="font-mono font-semibold text-down">{Math.round(sector.maxDrawdown)}%</span>
      </span>
      <ChevronDown className="h-4 w-4 shrink-0 text-faint transition" />
    </button>
  </section>
  );
};

export const ETF: React.FC = () => {
  const { t } = useI18n();
  const { id } = useParams<{ id?: string }>();
  const location = useLocation();
  const isCnTools = location.hash === '#cn-tools';
  const admin = useAdminSession();
  const [refreshFeedback, setRefreshFeedback] = useState('');
  const [refreshQueuing, setRefreshQueuing] = useState(false);
  const {
    sectors,
    categories,
    loading,
    error,
    dataMode,
    dataUpdated,
    metadataAsOf,
    diagnostics,
    activeCategory,
    sortBy,
    searchQuery,
    fetchSectors,
    setDataMode,
    setActiveCategory,
    setSortBy,
    setSearchQuery,
    getMembersForSector,
  } = useETFStore();

  const triggerRefresh = async () => {
    setRefreshQueuing(true);
    setRefreshFeedback('');
    try {
      const result = await post<{ status?: string }>('/api/etf/refresh');
      setRefreshFeedback(result.status === 'queued' ? '已安排 ETF 上游刷新，完整验证后才会替换当前快照。' : 'ETF 刷新状态已更新。');
    } catch (refreshError) {
      setRefreshFeedback(refreshError instanceof Error ? refreshError.message : 'ETF 刷新请求失败');
    } finally {
      setRefreshQueuing(false);
    }
  };

  const [etfPreset, setEtfPreset] = useState<string>('all');
  const [compareSyms, setCompareSyms] = useState('SPY,QQQ');
  const [compareResult, setCompareResult] = useState<any>(null);
  const [compareLoading, setCompareLoading] = useState(false);
  const [compareError, setCompareError] = useState('');
  const [holdingsSym, setHoldingsSym] = useState('SPY');
  const [holdings, setHoldings] = useState<ETFHoldingDetail | null>(null);
  const [holdingsLoading, setHoldingsLoading] = useState(false);
  const [holdingsError, setHoldingsError] = useState('');
  const holdingsRequestSequence = useRef(0);
  const [detailRetryVersion, setDetailRetryVersion] = useState(0);

  const handleCompare = async () => {
    setCompareLoading(true);
    setCompareError('');
    try {
      setCompareResult(await get(`/api/etf/compare?syms=${encodeURIComponent(compareSyms)}`));
    } catch (error) {
      setCompareResult(null);
      setCompareError(error instanceof Error ? error.message : 'ETF 对比暂不可用');
    } finally {
      setCompareLoading(false);
    }
  };

  const loadHoldings = async (symbol: string) => {
    const normalized = symbol.trim().toUpperCase();
    const requestSequence = ++holdingsRequestSequence.current;
    setHoldingsSym(normalized);
    setHoldings(null);
    setHoldingsError('');
    if (!normalized) {
      setHoldingsLoading(false);
      setHoldingsError('请输入 ETF 代码');
      return;
    }
    setHoldingsLoading(true);
    try {
      const result = await get<ETFHoldingDetail>(`/api/etf/${encodeURIComponent(normalized)}/holdings`);
      if (requestSequence !== holdingsRequestSequence.current) return;
      setHoldings(result);
    } catch (error) {
      if (requestSequence !== holdingsRequestSequence.current) return;
      setHoldingsError(error instanceof Error ? error.message : 'ETF 成分加载失败');
    } finally {
      if (requestSequence !== holdingsRequestSequence.current) return;
      setHoldingsLoading(false);
    }
  };
  const [detailLookup, setDetailLookup] = useState<DetailLookup>({
    id: '',
    status: 'idle',
    data: null,
  });

  const visibleSectors = sectors.filter((sector) => {
    const preset = ETF_PRESETS.find((p) => p.id === etfPreset);
    if (!preset || preset.id === 'all' || !('match' in preset) || !preset.match) return true;
    const blob = `${sector.id} ${sector.name} ${sector.topPerformer?.ticker || ''}`;
    return preset.match.test(blob);
  });

  const activeSector = id
    ? sectors.find((sector) => sector.id === id) || useETFStore.getState().allSectors.find((sector) => sector.id === id)
    : null;

  useEffect(() => {
    if (!isCnTools) void fetchSectors();
  }, [isCnTools]);

  useEffect(() => {
    if (location.hash) {
      const el = document.getElementById(location.hash.slice(1));
      if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  }, [location.hash]);

  useEffect(() => {
    if (!id || loading || activeSector) return;
    let cancelled = false;
    setDetailLookup({ id, status: 'loading', data: null });
    void get<ETFHoldingDetail>(`/api/etf/${encodeURIComponent(id)}/holdings`)
      .then((data) => {
        if (!cancelled) setDetailLookup({ id, status: 'ready', data });
      })
      .catch((lookupError) => {
        if (!cancelled) setDetailLookup({ id, status: 'error', data: null, error: lookupError instanceof Error ? lookupError.message : 'ETF 详情暂不可用' });
      });
    return () => {
      cancelled = true;
    };
  }, [id, loading, activeSector?.id, detailRetryVersion]);

  const currentDetail = detailLookup.id === id ? detailLookup : { id: id || '', status: 'idle' as const, data: null };

  if (id && !activeSector && (loading || currentDetail.status === 'idle' || currentDetail.status === 'loading')) {
    return (
      <StockGodShell title="ETF">
        <div className="mx-auto max-w-[1040px] space-y-4">
          <h1 className="text-[24px] font-semibold text-ink">ETF {id}</h1>
          <div className="flex items-center justify-center rounded-xl border border-line bg-surface py-16">
            <LoadingSpinner size="lg" />
          </div>
        </div>
      </StockGodShell>
    );
  }

  if (id && !activeSector && currentDetail.status === 'ready' && currentDetail.data) {
    const detail = currentDetail.data;
    return (
      <StockGodShell title="ETF">
        <div className="mx-auto max-w-[1040px] space-y-5">
          <Link to="/etf" className="inline-flex items-center gap-1 text-sm text-muted hover:text-ink">
            <ChevronLeft className="h-4 w-4" />
            返回 ETF
          </Link>

          {error ? (
            <div role="alert" className="border border-amber-500/35 bg-amber-500/5 px-4 py-3 text-sm text-amber-200">
              <p>ETF 总表不可用：{error}</p>
              <p className="mt-1 text-xs text-muted">来源 /api/etf/sectors · 数据时间 {dataUpdated || '未知'}；当前仅显示静态基础资料。</p>
              <button type="button" onClick={() => void fetchSectors(true)} className="mt-2 inline-flex items-center gap-1 text-xs font-semibold text-accent">
                <RefreshCw className="h-3.5 w-3.5" />重试 ETF 总表
              </button>
            </div>
          ) : null}

          <header className="border-b border-line pb-4">
            <div className="mb-2 inline-flex rounded-full border border-amber-500/35 bg-amber-500/10 px-2.5 py-1 text-[11px] font-semibold text-amber-200">
              静态基础资料
            </div>
            <h1 className="text-[24px] font-semibold text-ink">{detail.name} ({detail.sym})</h1>
            <p className="mt-2 text-sm leading-relaxed text-muted">
              市场：{detail.venue.toUpperCase()} · 持仓快照日期：{detail.updated || '未标记'}。本页不提供实时价格、涨跌幅或当前净值。
            </p>
            {detail.dataMode === 'historical' ? <p className="mt-2 text-xs text-amber-200">历史持仓参考 · {detail.degradedReason || '数据已超过当前模式年龄阈值'}</p> : null}
          </header>

          <section className="rounded-xl border border-line bg-surface">
            <header className="border-b border-line px-4 py-3">
              <h2 className="text-sm font-semibold text-ink">前十大参考持仓（{detail.holdings.length}）</h2>
              <p className="mt-1 text-xs text-faint">权重为本地种子快照中的近似值，不代表基金公司当日披露。</p>
            </header>
            <div className="divide-y divide-line/60">
              {detail.holdings.map((holding) => (
                <Link key={holding.symbol} to={`/stock/${holding.symbol}`} className="flex items-center justify-between px-4 py-3 hover:bg-surface-2">
                  <span className="min-w-0">
                    <span className="font-mono text-sm font-semibold text-ink">{holding.symbol}</span>
                    <span className="ml-2 text-sm text-muted">{holding.name}</span>
                  </span>
                  <span className="font-mono text-sm text-muted">{holding.weight}%</span>
                </Link>
              ))}
            </div>
          </section>

          <div className="flex flex-wrap gap-2">
            <Link to="/etf#cn-tools" className="rounded-md border border-line px-3 py-2 text-xs text-accent hover:bg-surface-2">进入 A 股工具箱</Link>
            <Link to="/settings" className="rounded-md border border-line px-3 py-2 text-xs text-muted hover:bg-surface-2">查看数据健康与修复入口</Link>
          </div>
        </div>
      </StockGodShell>
    );
  }

  if (id && !activeSector) {
    return (
      <StockGodShell title="ETF">
        <div className="mx-auto max-w-[1040px] space-y-4 rounded-xl border border-line bg-surface p-8">
          <h1 className="text-[24px] font-semibold text-ink">ETF {id}</h1>
          <p className="text-sm text-muted">
            {currentDetail.error || error || '未找到该 ETF 的板块或静态基础资料。'}
          </p>
          <div className="flex flex-wrap gap-2">
            <Link to="/etf" className="inline-flex items-center gap-1 rounded-md border border-line px-3 py-2 text-xs text-accent hover:bg-surface-2">
              <ChevronLeft className="h-3.5 w-3.5" />
              返回 ETF
            </Link>
            <Link to="/settings" className="rounded-md border border-line px-3 py-2 text-xs text-muted hover:bg-surface-2">数据健康与修复入口</Link>
            {currentDetail.status === 'error' && (
              <button type="button" onClick={() => setDetailRetryVersion((value) => value + 1)} className="inline-flex items-center gap-1 rounded-md border border-line px-3 py-2 text-xs font-semibold text-accent hover:bg-surface-2">
                <RefreshCw className="h-3.5 w-3.5" />重试 ETF 详情
              </button>
            )}
          </div>
        </div>
      </StockGodShell>
    );
  }

  if (activeSector) {
    const members = getMembersForSector(activeSector.name).slice(0, 40);

    return (
      <StockGodShell title="ETF">
        <div className="mx-auto max-w-[1040px] space-y-5">
          <Link to="/etf" className="inline-flex items-center gap-1 text-sm text-muted transition hover:text-ink">
            <ChevronLeft className="h-4 w-4" />
            ETF 板块
          </Link>

          {dataMode === 'historical' ? <HistoricalDataNotice updated={dataUpdated} /> : null}

          <header className="border-b border-line pb-4">
            <h1 className="text-[24px] font-semibold tracking-tight text-ink">{activeSector.name}</h1>
            <p className="mt-2 text-sm leading-relaxed text-muted">
              {activeSector.etfCount} 只 ETF · 总规模 {formatAum(activeSector.aum)} · 近 1 年 {activeSector.return1y ?? 0}% · 近 5 年 {activeSector.return5y ?? 0}% · 最大回撤 {activeSector.maxDrawdown}%。
            </p>
          </header>

          <div className="grid gap-3 md:grid-cols-4">
            <div className="rounded-xl border border-line bg-surface p-4">
              <div className="text-xs text-faint">ETF 数量</div>
              <div className="mt-2 font-mono text-xl font-semibold text-ink">{activeSector.etfCount}</div>
            </div>
            <div className="rounded-xl border border-line bg-surface p-4">
              <div className="text-xs text-faint">AUM</div>
              <div className="mt-2 font-mono text-xl font-semibold text-ink">{formatAum(activeSector.aum)}</div>
            </div>
            <div className="rounded-xl border border-line bg-surface p-4">
              <div className="text-xs text-faint">5年最强</div>
              <div className="mt-2 font-mono text-xl font-semibold text-up">{activeSector.topPerformer.ticker} +{Math.round(activeSector.topPerformer.return5y)}%</div>
            </div>
            <div className="rounded-xl border border-line bg-surface p-4">
              <div className="text-xs text-faint">最大回撤</div>
              <div className="mt-2 font-mono text-xl font-semibold text-down">{Math.round(activeSector.maxDrawdown)}%</div>
            </div>
          </div>

          <section className="rounded-xl border border-line bg-surface">
            <header className="border-b border-line px-4 py-3">
              <h2 className="text-sm font-semibold text-ink">板块内 ETF（{members.length}）</h2>
            </header>
            <div className="divide-y divide-line/60">
              {members.length === 0 ? (
                <div className="px-4 py-8 text-center text-sm text-muted">暂无成员数据</div>
              ) : (
                members.map((etf) => (
                  <div key={etf.sym} className="flex items-center justify-between px-4 py-3 transition hover:bg-surface-2">
                    <Link to={`/stock/${etf.sym}`} className="min-w-0 flex-1">
                      <div className="font-mono text-sm font-semibold text-ink">{etf.sym}</div>
                      <div className="truncate text-xs text-muted">{etf.name}</div>
                    </Link>
                    <button
                      type="button"
                      className="mr-3 text-[11px] text-accent"
                      onClick={() => void loadHoldings(etf.sym)}
                      disabled={holdingsLoading && holdingsSym === etf.sym}
                    >
                      {holdingsLoading && holdingsSym === etf.sym ? '加载中…' : '持仓'}
                    </button>
                    <div className="ml-3 text-right">
                      <div className={`font-mono text-sm ${(etf.ret5y || 0) >= 0 ? 'text-up' : 'text-down'}`}>
                        {etf.ret5y != null ? `${etf.ret5y >= 0 ? '+' : ''}${Math.round(etf.ret5y)}%` : '—'}
                      </div>
                      <div className="text-xs text-faint">5Y · AUM {formatAum(etf.aum || 0)}</div>
                    </div>
                  </div>
                ))
              )}
            </div>
          </section>

          {holdingsLoading && <p role="status" className="text-xs text-muted">正在加载 {holdingsSym} 成分…</p>}
          {holdingsError && (
            <p role="alert" className="text-xs text-down">
              {holdingsError} · <button type="button" className="underline" onClick={() => void loadHoldings(holdingsSym)}>重试</button>
            </p>
          )}
          {holdings?.holdings ? (
            <section className="rounded-xl border border-line bg-surface p-4">
              <h3 className="mb-2 text-sm font-semibold text-ink">{holdings.sym} 成分（{holdings.name}）</h3>
              {holdings.dataMode === 'historical' ? <p className="mb-2 text-xs text-amber-200">历史持仓参考 · {holdings.updated || '时间未知'} · {holdings.degradedReason || '数据已超过当前模式年龄阈值'}</p> : null}
              <div className="divide-y divide-line/60">
                {holdings.holdings.map((h: any) => (
                  <Link key={h.symbol} to={`/stock/${h.symbol}`} className="flex justify-between py-2 text-sm hover:text-accent">
                    <span className="font-mono">{h.symbol} <span className="text-muted">{h.name}</span></span>
                    <span className="font-mono text-muted">{h.weight}%</span>
                  </Link>
                ))}
              </div>
            </section>
          ) : null}

          <p className="text-[11px] leading-relaxed text-faint">
            数据 = /api/etf/sectors。回报为区间累计,非年化;最大回撤=区间内峰值到谷底最大跌幅 · 非投资建议。
          </p>
        </div>
      </StockGodShell>
    );
  }

  if (isCnTools) {
    return (
      <StockGodShell title={t('nav.ashare')}>
        <div id="cn-tools" className="mx-auto max-w-[1180px] space-y-5">
          <header className="flex flex-wrap items-end justify-between gap-3 border-b border-line pb-4">
            <div>
              <h1 className="text-[22px] font-semibold text-ink">A 股工具箱</h1>
              <p className="mt-1.5 max-w-3xl text-sm leading-relaxed text-muted">
                仓位约束、机械规则和行情提醒。只有实时报价完整且未过期时，才展示价格、动作与买卖点。
              </p>
            </div>
            <Link to="/etf" className="inline-flex items-center gap-1 rounded-md border border-line px-3 py-2 text-xs text-muted hover:bg-surface-2 hover:text-ink">
              <ChevronLeft className="h-3.5 w-3.5" />
              ETF 数据库
            </Link>
          </header>
          <CnToolbox />
        </div>
      </StockGodShell>
    );
  }

  return (
    <StockGodShell title={t('nav.etf')} searchQuery={searchQuery} onSearchChange={setSearchQuery}>
      <div className="min-w-0 space-y-5 overflow-hidden">
        <header className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 className="text-[22px] font-semibold tracking-tight text-ink">{t('etf.title')}</h1>
            <p className="mt-1.5 text-sm leading-relaxed text-muted">
              {t('etf.sub')}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="inline-flex border border-line bg-surface p-0.5" aria-label="ETF 数据模式">
              <button type="button" onClick={() => setDataMode('current')} className={`px-3 py-1.5 text-xs font-medium ${dataMode === 'current' ? 'bg-surface-3 text-ink' : 'text-muted'}`}>当前</button>
              <button type="button" onClick={() => setDataMode('historical')} className={`px-3 py-1.5 text-xs font-medium ${dataMode === 'historical' ? 'bg-surface-3 text-ink' : 'text-muted'}`}>历史</button>
            </div>
            <button type="button" onClick={() => void triggerRefresh()} disabled={!admin.authorized || refreshQueuing} title={admin.authorized ? '安排后端有限 ETF 范围刷新' : '需要管理会话'} className="inline-flex h-9 items-center gap-1.5 rounded-md border border-line px-3 text-xs font-semibold text-muted hover:bg-surface-2 disabled:opacity-50">
              <RefreshCw className={`h-3.5 w-3.5 ${refreshQueuing ? 'animate-spin' : ''}`} /> 刷新上游
            </button>
            <Link to="/etf#cn-tools" className="rounded-md border border-line px-3 py-2 text-xs font-medium text-accent hover:bg-surface-2">
              进入 A 股工具箱
            </Link>
          </div>
        </header>

        {dataMode === 'historical' ? <HistoricalDataNotice updated={dataUpdated} /> : null}
        {dataMode === 'current' && metadataAsOf && metadataAsOf !== dataUpdated ? (
          <p className="border border-line bg-surface px-3 py-2 text-xs text-muted">
            价格收益与回撤指标截至 {dataUpdated}；AUM、费率和分类元数据截至 {metadataAsOf}，两类日期不合并为同一实时声明。
          </p>
        ) : null}
        <ETFDiagnostics diagnostics={diagnostics} />
        {refreshFeedback ? <p role="status" className="text-xs text-muted">{refreshFeedback}</p> : null}

        <div className="flex flex-wrap gap-1.5">
          {ETF_PRESETS.map((p) => (
            <button
              key={p.id}
              type="button"
              onClick={() => setEtfPreset(p.id)}
              className={`rounded-full px-2.5 py-1 text-[11px] ${etfPreset === p.id ? 'bg-accent/15 text-accent' : 'bg-surface-2 text-muted'}`}
            >
              {p.label}
            </button>
          ))}
        </div>

        <section className="grid gap-3 rounded-xl border border-line bg-surface p-4 md:grid-cols-2">
          <div>
            <div className="mb-2 text-sm font-semibold text-ink">对比 ETF</div>
            <div className="flex gap-2">
              <input
                value={compareSyms}
                onChange={(e) => setCompareSyms(e.target.value.toUpperCase())}
                className="flex-1 rounded-lg border border-line bg-base px-2 py-1.5 font-mono text-sm"
                placeholder="SPY,QQQ"
              />
              <button
                type="button"
                className="rounded-lg bg-accent/15 px-3 text-xs font-semibold text-accent disabled:cursor-wait disabled:opacity-60"
                onClick={() => void handleCompare()}
                disabled={compareLoading}
              >
                {compareLoading ? '对比中...' : '对比'}
              </button>
            </div>
            {compareError ? (
              <p className="mt-2 text-xs text-down" role="alert">
                {compareError} · <button type="button" className="underline" onClick={() => void handleCompare()}>重试</button>
              </p>
            ) : null}
            {compareResult?.shared ? (
              <p className="mt-2 text-xs text-muted">
                {compareResult.dataMode === 'historical' ? `历史持仓参考（${compareResult.updated || '时间未知'}）· ` : ''}
                共同持仓：{compareResult.shared.join(', ') || '无'}
              </p>
            ) : null}
          </div>
          <div>
            <div className="mb-2 text-sm font-semibold text-ink">查看成分</div>
            <div className="flex gap-2">
              <input
                value={holdingsSym}
                onChange={(e) => setHoldingsSym(e.target.value.toUpperCase())}
                className="w-28 rounded-lg border border-line bg-base px-2 py-1.5 font-mono text-sm"
              />
              <button
                type="button"
                className="rounded-lg border border-line px-3 text-xs"
                onClick={() => void loadHoldings(holdingsSym)}
                disabled={holdingsLoading}
              >
                {holdingsLoading ? '加载中…' : '加载'}
              </button>
            </div>
            {holdingsLoading && <p role="status" className="mt-2 text-xs text-muted">正在加载 {holdingsSym} 成分…</p>}
            {holdingsError && (
              <p role="alert" className="mt-2 text-xs text-down">
                {holdingsError} · <button type="button" className="underline" onClick={() => void loadHoldings(holdingsSym)}>重试</button>
              </p>
            )}
            {holdings?.holdings ? (
              <>
                {holdings.dataMode === 'historical' ? <p className="mt-2 text-xs text-amber-200">历史持仓参考 · {holdings.updated || '时间未知'}</p> : null}
                <ul className="mt-2 max-h-40 space-y-1 overflow-auto text-xs">
                  {holdings.holdings.slice(0, 12).map((h: any) => (
                    <li key={h.symbol} className="flex justify-between font-mono">
                      <span>{h.symbol}</span>
                      <span>{h.weight}%</span>
                    </li>
                  ))}
                </ul>
              </>
            ) : null}
          </div>
        </section>

        <div className="min-w-0 space-y-2.5 overflow-hidden">
          <input
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
            placeholder="搜代码 / 名称..."
            className="w-full max-w-xs rounded-lg border border-line bg-surface px-3 py-2 text-sm text-ink outline-none placeholder:text-faint focus:border-accent/50"
          />

          <div className="-mx-1 flex gap-1.5 overflow-x-auto px-1 pb-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
            {CATEGORIES.map((category) => {
              const count = category.id === 'all'
                ? Array.from(categories.values()).reduce((sum, value) => sum + value, 0)
                : categories.get(category.id as ETFCategory);
              const isActive = activeCategory === category.id;
              return (
                <button
                  key={category.id}
                  onClick={() => setActiveCategory(category.id)}
                  className={`shrink-0 whitespace-nowrap rounded-lg border px-3 py-2 text-[13px] font-medium transition ${
                    isActive ? 'border-accent/40 bg-accent/10 text-accent' : 'border-line bg-surface text-muted hover:text-ink'
                  }`}
                >
                  {category.label}{count == null ? '' : ` ${count}`}
                </button>
              );
            })}
          </div>

          <div className="inline-flex gap-0.5 rounded-lg border border-line bg-surface p-0.5">
            {SORT_OPTIONS.map((option) => (
              <button
                key={option.id}
                onClick={() => setSortBy(option.id)}
                className={`rounded-md px-3 py-1.5 text-[13px] font-medium transition ${
                  sortBy === option.id ? 'bg-surface-3 text-ink' : 'text-muted hover:text-ink'
                }`}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>

        {loading && (
          <div className="flex items-center justify-center rounded-xl border border-line bg-surface py-16">
            <LoadingSpinner size="lg" />
          </div>
        )}

        {error && (
          <div role="alert" className="rounded-xl border border-line bg-surface p-8 text-center text-sm text-down">
            <p>{error}</p>
            <p className="mt-1 text-xs text-muted">来源 /api/etf/sectors · 数据时间 {dataUpdated || '未知'} · 当前数据不可用</p>
            <div className="mt-3 flex justify-center gap-2">
              <button type="button" onClick={() => void fetchSectors(true)} className="inline-flex items-center gap-1 rounded-md border border-line px-3 py-2 text-xs font-semibold text-accent hover:bg-surface-2">
                <RefreshCw className="h-3.5 w-3.5" />重试 ETF 总表
              </button>
              <button type="button" onClick={() => setDataMode(dataMode === 'current' ? 'historical' : 'current')} className="rounded-md border border-line px-3 py-2 text-xs font-semibold text-muted hover:bg-surface-2">
                切换到{dataMode === 'current' ? '历史' : '当前'}模式
              </button>
            </div>
          </div>
        )}

        {!loading && !error && (
          <div className="space-y-4">
            {visibleSectors.map((sector) => (
              <SectorRow key={sector.id} sector={sector} />
            ))}
          </div>
        )}

        <footer className="pt-2 text-[11px] leading-relaxed text-faint">
          数据 = Nasdaq(AUM/费率 + 5年日线算回报与最大回撤)。4533 只 ETF、47 个板块。回报为区间累计,非年化;最大回撤=区间内峰值到谷底最大跌幅 · 判决是费率+类型的机械映射 · 非投资建议。
        </footer>
      </div>
    </StockGodShell>
  );
};
