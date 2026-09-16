import React, { useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { StockGodShell } from '../components/layout/StockGodShell';
import { EmptyState } from '../components/common';
import { WatchlistCard, HoldingsCard } from '../components/portfolio';
import { usePortfolioStore } from '../stores/portfolioStore';
import { PortfolioTab } from '../types/portfolio';
import { findUsStock, loadUsMarketStocks, loadAMarketStocks } from '../utils/stockgodData';
import { useDebounce } from '../hooks/useDebounce';
import { Stock } from '../types/stocks';
import { get as apiGet, post } from '../utils/api';
import { useAdminSession } from '../hooks/useAdminSession';
import { useUiStore } from '../stores/uiStore';
import { inferVenue } from '../types/portfolio';
import { useAlertsStore } from '../stores/alertsStore';
import { PineScriptModal } from '../components/PineScriptModal';
import { Code2, Copy, Check, LayoutGrid, Download } from 'lucide-react';
import { buildPortfolioRisk } from '../utils/portfolioRisk';
import { fetchQuoteResult, startQuotePolling } from '../utils/liveQuotes';
import { DataStatus, type DataState } from '../components/common/DataStatus';
import { getPaperPortfolioRisk, type PaperPortfolioRisk } from '../utils/paperOrders';

export const Portfolio: React.FC = () => {
  const { authorized: hasAdminAccess } = useAdminSession();
  const navigate = useNavigate();
  const language = useUiStore((s) => s.language);
  const [activeTab, setActiveTab] = useState<PortfolioTab>('watchlist');
  const {
    watchlist,
    holdings,
    cashBalances,
    addToWatchlist,
    removeFromWatchlist,
    isInWatchlist,
    addHolding,
    removeHolding,
    setCashBalance,
  } = usePortfolioStore();
  const {
    alerts,
    addAlert,
    removeAlert,
    notificationStatus,
    monitorState,
    lastCheckedAt,
    quoteDataTime,
    quoteSource,
    monitorError,
    requestNotifications,
    sendTestNotification,
  } = useAlertsStore();
  const [query, setQuery] = useState('');
  const [hits, setHits] = useState<Stock[]>([]);
  const debounced = useDebounce(query, 250);
  const [batchStatus, setBatchStatus] = useState<string>('');
  const [batchJobId, setBatchJobId] = useState('');
  const [batchPolling, setBatchPolling] = useState(false);
  const [alertSym, setAlertSym] = useState('');
  const [alertPrice, setAlertPrice] = useState('');
  const [alertOp, setAlertOp] = useState<'>=' | '<='>('>=');
  const [alertFeedback, setAlertFeedback] = useState('');

  const [hSymbol, setHSymbol] = useState('');
  const [hName, setHName] = useState('');
  const [hQty, setHQty] = useState('100');
  const [hCost, setHCost] = useState('');
  const [hSector, setHSector] = useState('');
  const [hStopLoss, setHStopLoss] = useState('');
  const [hNotes, setHNotes] = useState('');
  const [quotes, setQuotes] = useState<Record<string, { price: number; pct: number }>>({});
  const [quoteStatus, setQuoteStatus] = useState<{
    state: DataState;
    dataTime?: string;
    source?: string;
    message?: string;
  }>({ state: 'loading' });
  const [paperPortfolio, setPaperPortfolio] = useState<PaperPortfolioRisk | null>(null);
  const [paperPortfolioError, setPaperPortfolioError] = useState('');

  const currentDate = new Date().toLocaleDateString(language === 'en' ? 'en-US' : 'zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  });

  useEffect(() => {
    let cancelled = false;
    (async () => {
      if (!debounced.trim()) {
        setHits([]);
        return;
      }
      const q = debounced.toLowerCase();
      const [us, cn] = await Promise.all([
        loadUsMarketStocks().catch(() => [] as Stock[]),
        loadAMarketStocks().catch(() => [] as Stock[]),
      ]);
      if (cancelled) return;
      const merged = [...us, ...cn];
      setHits(
        merged
          .filter((s) => s.symbol.toLowerCase().includes(q) || s.name.toLowerCase().includes(q))
          .slice(0, 10)
      );
    })();
    return () => {
      cancelled = true;
    };
  }, [debounced]);

  useEffect(() => {
    const syms = [
      ...new Set([...watchlist.map((w) => w.symbol), ...holdings.map((h) => h.symbol)]),
    ];
    if (!syms.length) {
      setQuotes({});
      setQuoteStatus({ state: 'unavailable', message: '暂无需要估值的标的' });
      return;
    }
    let cancelled = false;
    const poll = async () => {
      if (cancelled || document.hidden) return;
      try {
        setQuoteStatus((previous) => ({ ...previous, state: 'loading', message: undefined }));
        const result = await fetchQuoteResult(syms.slice(0, 60));
        if (!cancelled) {
          const hasProvenance = Boolean(result.meta.source && result.meta.dataTime);
          const usable = !result.meta.stale && hasProvenance;
          setQuotes(usable ? result.quotes : {});
          setQuoteStatus({
            state: result.meta.stale ? 'stale' : hasProvenance ? 'live' : 'unavailable',
            dataTime: result.meta.dataTime,
            source: result.meta.source,
            message: [result.meta.staleReason, ...result.errors, result.missing.length ? `缺少 ${result.missing.length} 个报价` : '']
              .filter(Boolean)
              .join('；') || undefined,
          });
        }
      } catch (error) {
        if (!cancelled) {
          setQuotes({});
          setQuoteStatus({ state: 'error', message: error instanceof Error ? error.message : '报价加载失败' });
        }
      }
    };

    const stopPolling = startQuotePolling(poll, 15_000);

    return () => {
      cancelled = true;
      stopPolling();
    };
  }, [watchlist, holdings]);

  useEffect(() => {
    if (!hasAdminAccess) {
	  setPaperPortfolio(null);
      setPaperPortfolioError('管理会话未授权，服务器 Paper 组合不可见');
      return;
    }
    let cancelled = false;
    void getPaperPortfolioRisk()
      .then((result) => {
        if (!cancelled) {
          setPaperPortfolio(result);
          setPaperPortfolioError('');
        }
      })
      .catch((error) => {
        if (!cancelled) setPaperPortfolioError(error instanceof Error ? error.message : '服务器 Paper 组合加载失败');
      });
    return () => {
      cancelled = true;
    };
  }, [hasAdminAccess]);

  const watchedSymbols = useMemo(() => new Set(watchlist.map((w) => w.symbol)), [watchlist]);

  const enrichedWatchlist = useMemo(
    () =>
      watchlist.map((item) => ({
        ...item,
        price: quotes[item.symbol]?.price ?? item.price,
        changePercent: quotes[item.symbol]?.pct ?? item.changePercent,
      })),
    [watchlist, quotes]
  );

  const enrichedHoldings = useMemo(
    () =>
      holdings.map((item) => {
        const price = quotes[item.symbol]?.price;
        const currentValue = price !== undefined ? price * item.quantity : undefined;
        const costBasis = item.avgCost * item.quantity;
        const profitLoss = currentValue !== undefined ? currentValue - costBasis : undefined;
        const profitLossPercent =
          profitLoss !== undefined && costBasis > 0 ? (profitLoss / costBasis) * 100 : undefined;
        return { ...item, currentValue, profitLoss, profitLossPercent };
      }),
    [holdings, quotes]
  );

  const portfolioRisk = useMemo(
    () => buildPortfolioRisk(enrichedHoldings, 25, cashBalances, 40),
    [enrichedHoldings, cashBalances]
  );

  const formatCurrency = (value: number, currency: 'USD' | 'CNY') =>
    new Intl.NumberFormat(language === 'en' ? 'en-US' : 'zh-CN', {
      style: 'currency',
      currency,
      maximumFractionDigits: 2,
    }).format(value);

  const submitHolding = async (e: React.FormEvent) => {
    e.preventDefault();
    const symbol = hSymbol.trim().toUpperCase();
    const quantity = Number(hQty);
    const avgCost = Number(hCost);
    const stopLoss = hStopLoss.trim() ? Number(hStopLoss) : undefined;
    if (!symbol || !(quantity > 0) || !(avgCost > 0) || (stopLoss !== undefined && !(stopLoss > 0))) return;
    let name = hName.trim();
    if (!name) {
      const found = await findUsStock(symbol);
      name = found?.name || symbol;
    }
    addHolding({
      symbol,
      name,
      quantity,
      avgCost,
      purchaseDate: new Date(),
      notes: hNotes.trim() || undefined,
      sector: hSector.trim() || undefined,
      stopLoss,
      venue: inferVenue(symbol),
    });
    setHSymbol('');
    setHName('');
    setHQty('100');
    setHCost('');
    setHSector('');
    setHStopLoss('');
    setHNotes('');
  };

  const pollBatch = async (id: string) => {
    setBatchPolling(true);
    try {
      for (let attempt = 0; attempt < 30; attempt += 1) {
        const job = await apiGet<{
          status: string;
          tickers?: string[];
          done?: string[];
          unavailable?: string[];
          current?: string;
          error?: string;
        }>(`/api/analysis/batch/${encodeURIComponent(id)}`);
        const total = job.tickers?.length || 0;
        const completed = (job.done?.length || 0) + (job.unavailable?.length || 0);
        const detail = `完成 ${completed}/${total}${job.current ? ` · 当前 ${job.current}` : ''}${job.unavailable?.length ? ` · 不可用 ${job.unavailable.length}` : ''}`;
        if (job.status === 'error' || job.status === 'cancelled') {
          setBatchStatus(`批量研究${job.status === 'error' ? '失败' : '已取消'}：${job.error || detail}`);
          return;
        }
        if (job.status === 'done' || (total > 0 && completed >= total)) {
          setBatchStatus(`批量研究已结束 · ${detail}`);
          return;
        }
        setBatchStatus(`批量研究进行中 · ${detail}`);
        await new Promise((resolve) => window.setTimeout(resolve, 2_000));
      }
      setBatchStatus(`批量研究仍在后台运行 · 任务 ${id}，可继续检查进度`);
    } catch (error) {
      setBatchStatus(`批量研究状态读取失败：${error instanceof Error ? error.message : String(error)} · 可重试检查`);
    } finally {
      setBatchPolling(false);
    }
  };

  const runBatch = async () => {
    const tickers = [...new Set([...watchlist.map((w) => w.symbol), ...holdings.map((h) => h.symbol)])]
      .filter((s) => !/^\d{6}$/.test(s))
      .slice(0, 10);
    if (!tickers.length) {
      setBatchStatus('请先加入美股/ETF 观察或持仓');
      return;
    }
    try {
      setBatchStatus('排队中…');
      const resp = await post<{ id: string; tickers: string[] }>('/api/analysis/batch', { tickers });
      setBatchJobId(resp.id);
      setBatchStatus(`已启动 ${resp.id}（${resp.tickers.join(', ')}）`);
      await pollBatch(resp.id);
    } catch (e: any) {
      setBatchStatus(e?.message || '批量分析失败');
    }
  };

  const [isPineModalOpen, setIsPineModalOpen] = useState(false);
  const [copiedTv, setCopiedTv] = useState(false);

  const exportPortfolio = () => {
    const data = {
      watchlist,
      holdings: enrichedHoldings,
      cashBalances,
      exportedAt: new Date().toISOString(),
    };
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `stockgod_portfolio_${new Date().toISOString().slice(0, 10)}.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const copyTradingViewWatchlist = () => {
    const syms = [...new Set([...watchlist.map((w) => w.symbol), ...holdings.map((h) => h.symbol)])];
    if (!syms.length) return;
    const formatSym = (s: string) => {
      if (/^6\d{5}$/.test(s)) return `SSE:${s}`;
      if (/^(00|30)\d{4}$/.test(s)) return `SZSE:${s}`;
      if (['PLTR', 'BABA', 'NIO', 'TSM'].includes(s)) return `NYSE:${s}`;
      return `NASDAQ:${s}`;
    };
    const list = syms.map(formatSym).join(', ');
    navigator.clipboard.writeText(list);
    setCopiedTv(true);
    setTimeout(() => setCopiedTv(false), 2500);
  };

  const t = language === 'en'
    ? {
        title: 'My portfolio',
        sub: 'Server paper risk and a separate browser-only personal ledger.',
        local: 'Stored only in this browser (localStorage)',
        today: 'Today',
        watch: 'Watchlist',
        hold: 'Holdings',
        addWatch: 'Add to watchlist',
        searchPh: 'Search symbol / name…',
        watched: 'Watching',
        add: 'Add',
        emptyWatch: 'Watchlist is empty',
        emptyWatchDesc: 'Search above, or star symbols on the scanner.',
        goScan: 'Open scanner',
        addHold: 'Add holding',
        qty: 'Shares',
        cost: 'Avg cost',
        notes: 'Notes (optional)',
        save: 'Save holding',
        emptyHold: 'No holdings yet',
        emptyHoldDesc: 'Record Paper scenario positions locally. Quotes refresh via /api/quote.',
        costLabel: 'Cost basis',
        mktLabel: 'Mark-to-market',
        plLabel: 'P&L',
      }
    : {
        title: '我的组合',
        sub: '服务器 Paper 风控账本与浏览器个人观察账本分开显示。',
        local: '',
        today: '今日',
        watch: '观察列表',
        hold: '持仓',
        addWatch: '添加观察',
        searchPh: '搜代码 / 名称加入观察…',
        watched: '已观察',
        add: '加入',
        emptyWatch: '观察列表为空',
        emptyWatchDesc: '去 全市场扫描 找感兴趣的标的,点 ☆ 加入。',
        goScan: '去全市场扫描',
        addHold: '添加持仓',
        qty: '数量',
        cost: '成本价',
        notes: '备注（可选）',
        save: '保存持仓',
        emptyHold: '还没有持仓',
        emptyHoldDesc: '本地仅记录 Paper 情景仓位。价格通过 /api/quote 刷新，不代表真实券商账户。',
        costLabel: '成本',
        mktLabel: '市值',
        plLabel: '盈亏',
      };

  return (
    <StockGodShell title={language === 'en' ? 'Portfolio' : '我的'}>
      <div className="mx-auto max-w-[1104px]">
        <header className="mb-8 flex items-baseline justify-between border-b border-line pb-6">
          <div>
            <h1 className="mb-2 text-[28px] font-semibold text-ink">{t.title}</h1>
            <p className="text-sm text-muted">{t.sub}</p>
            {t.local ? <p className="mt-1 text-xs text-faint">{t.local}</p> : null}
          </div>
          <div className="flex flex-col items-end gap-2">
            <div className="text-sm text-faint">
              {t.today} {currentDate}
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={copyTradingViewWatchlist}
                className="rounded-lg border border-line bg-surface px-3 py-1.5 text-xs font-medium text-muted hover:text-ink transition flex items-center gap-1"
                title="一键复制自选与持仓为 TradingView 格式代码"
              >
                {copiedTv ? <Check size={13} className="text-emerald-400" /> : <Copy size={13} />}
                {copiedTv ? '已复制 TV 格式！' : '📋 复制 TV 自选'}
              </button>
              <button
                type="button"
                onClick={() => setIsPineModalOpen(true)}
                className="rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-3 py-1.5 text-xs font-medium text-emerald-400 hover:bg-emerald-500/20 transition flex items-center gap-1"
                title="获取 6-in-1 全能指标源码"
              >
                <Code2 size={13} /> 6-in-1 指标
              </button>
              <button
                type="button"
                onClick={() => navigate('/multichart')}
                className="rounded-lg border border-indigo-500/40 bg-indigo-500/10 px-3 py-1.5 text-xs font-medium text-indigo-300 hover:bg-indigo-500/20 transition flex items-center gap-1"
                title="进入四分屏实时多图盯盘工作台"
              >
                <LayoutGrid size={13} /> 多图盯盘
              </button>
              <button
                type="button"
                onClick={exportPortfolio}
                className="rounded-lg border border-line bg-surface px-3 py-1.5 text-xs font-medium text-muted hover:text-ink transition flex items-center gap-1"
                title="导出我的自选与持仓记录为 JSON 文件备份"
              >
                <Download size={13} /> 导出 JSON
              </button>
            </div>
          </div>
        </header>

        <section className="mb-6 border-y border-sky-500/25 bg-sky-950/15 py-4" aria-label="服务器 Paper 组合">
          <div className="mb-3 flex flex-wrap items-start justify-between gap-2">
            <div>
              <h2 className="text-sm font-semibold text-sky-200">服务器 Paper 组合</h2>
              <p className="mt-1 text-xs text-muted">这是订单接受与风险限制使用的权威账本；按币种独立计算，不做汇率换算。</p>
            </div>
            <span className="border border-sky-500/30 px-2 py-1 text-[10px] font-bold text-sky-200">SERVER PAPER LEDGER</span>
          </div>
          {paperPortfolioError && <p className="text-xs text-amber-300">{paperPortfolioError}</p>}
          {!hasAdminAccess && (
            <div className="mt-3 flex flex-wrap items-center justify-between gap-2 border-t border-sky-500/20 pt-3">
              <p className="text-xs text-muted">服务器 Paper 账本需要当前标签页会话的管理令牌；本机观察账本仍可独立使用。</p>
              <Link to="/settings" className="rounded-lg border border-sky-400/40 bg-sky-500/10 px-3 py-1.5 text-xs font-semibold text-sky-200 hover:bg-sky-500/20">
                去设置验证会话令牌
              </Link>
            </div>
          )}
          {paperPortfolio && (
            <div className="space-y-3">
              {!paperPortfolio.riskComplete && (
                <p className="text-xs font-semibold text-down">估值或行业不完整，相关币种的新增买单将被服务端阻断：{paperPortfolio.unknownSymbols.join(', ') || 'unknown'}</p>
              )}
              <div className="grid gap-3 md:grid-cols-2">
                {paperPortfolio.groups.map((group) => (
                  <div key={group.currency} className="border border-line bg-surface p-3">
                    <div className="mb-2 flex items-center justify-between text-sm">
                      <b>{group.currency}</b>
                      <span className={group.valuationComplete && group.sectorComplete ? 'text-up' : 'text-down'}>
                        {group.valuationComplete && group.sectorComplete ? '风险数据完整' : '风险数据不完整'}
                      </span>
                    </div>
                    <div className="grid grid-cols-2 gap-2 text-xs text-muted sm:grid-cols-3">
                      <span>总权益<br /><b className="text-ink">{group.totalEquity?.toFixed(2) ?? '—'}</b></span>
                      <span>可用 / 预占<br /><b className="text-ink">{group.availableCash.toFixed(2)} / {group.reservedCash.toFixed(2)}</b></span>
                      <span>总敞口<br /><b className="text-ink">{group.grossExposurePct?.toFixed(1) ?? '—'}% / {paperPortfolio.limits.maxPortfolioExposurePct}%</b></span>
                      <span>最大单票<br /><b className="text-ink">{group.largestSymbol || '—'} {group.largestSymbolPct?.toFixed(1) ?? '—'}%</b></span>
                      <span>最大行业<br /><b className="text-ink">{group.largestSector || '—'} {group.largestSectorPct?.toFixed(1) ?? '—'}%</b></span>
                      <span>止损覆盖<br /><b className="text-ink">{group.stopCoveredQuantity}/{group.positionQuantity} · {group.stopCoveragePct.toFixed(1)}%</b></span>
                      <span>未结计划亏损<br /><b className="text-ink">{group.openOrderMaxPlannedLoss.toFixed(2)} · {group.openOrderMaxLossPct?.toFixed(2) ?? '—'}%</b></span>
                      <span>日内 / 未实现盈亏<br /><b className="text-ink">{group.realizedPnLToday.toFixed(2)} / {group.unrealizedPnL?.toFixed(2) ?? '—'}</b></span>
                      <span>峰值回撤<br /><b className="text-ink">{group.peakDrawdown?.toFixed(2) ?? '—'} · {group.peakDrawdownPct?.toFixed(2) ?? '—'}%</b></span>
                      <span>ADV<br /><b className={group.liquidityComplete ? 'text-up' : 'text-down'}>{group.liquidityComplete ? '完整' : '未知，BUY 阻断'}</b></span>
                      <span>行业压力<br /><b className="text-ink">{group.stressScenarios.find((item) => item.kind === 'sector-concentration')?.pnl?.toFixed(2) ?? '未知'}</b></span>
                      <span>流动性占量<br /><b className="text-ink">{group.stressScenarios.find((item) => item.kind === 'liquidity')?.liquidityUsagePct != null ? `${group.stressScenarios.find((item) => item.kind === 'liquidity')!.liquidityUsagePct!.toFixed(2)}%` : '未知'}</b></span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </section>

        <div className="mb-4 border-b border-line pb-3">
          <h2 className="text-sm font-semibold text-ink">个人观察账本（仅当前浏览器）</h2>
          <p className="mt-1 text-xs text-faint">下方观察、持仓和现金来自 localStorage，不参与服务器 Paper 订单风控，也不是权威账户。</p>
        </div>

        {(watchlist.length > 0 || holdings.length > 0) && (
          <div className="mb-4 border-b border-line pb-3">
            <DataStatus
              state={quoteStatus.state}
              dataTime={quoteStatus.dataTime}
              source={quoteStatus.source}
              message={quoteStatus.message}
              compact
            />
          </div>
        )}

        <div className="mb-6 inline-flex rounded-lg border border-line bg-surface p-1">
          <button
            className={`rounded-md px-5 py-2 text-sm font-medium transition ${
              activeTab === 'watchlist' ? 'bg-surface-3 text-ink' : 'text-muted hover:text-ink'
            }`}
            onClick={() => setActiveTab('watchlist')}
          >
            {t.watch}
          </button>
          <button
            className={`rounded-md px-5 py-2 text-sm font-medium transition ${
              activeTab === 'holdings' ? 'bg-surface-3 text-ink' : 'text-muted hover:text-ink'
            }`}
            onClick={() => setActiveTab('holdings')}
          >
            {t.hold}
          </button>
        </div>

        <section className="mb-5 grid gap-3 rounded-xl border border-line bg-surface p-4 md:grid-cols-2">
          <div>
            <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
              <div className="text-sm font-semibold text-ink">价格提醒</div>
              <div className="flex items-center gap-2">
                {notificationStatus === 'default' && (
                  <button
                    type="button"
                    onClick={() => void requestNotifications().then((status) => setAlertFeedback(`通知权限：${status}`))}
                    className="text-xs font-medium text-accent"
                  >
                    启用通知
                  </button>
                )}
                {(notificationStatus === 'granted' || notificationStatus === 'native') && (
                  <button
                    type="button"
                    onClick={() => void sendTestNotification().then((ok) => setAlertFeedback(ok ? '测试通知已发送' : '测试通知发送失败'))}
                    className="text-xs font-medium text-accent"
                  >
                    发送测试
                  </button>
                )}
              </div>
            </div>
            <p className="mb-2 text-[11px] leading-relaxed text-faint">
              仅在本应用运行、页面可见且联网时检查；关闭应用不会提醒。提醒和持仓仅保存在本机浏览器。
            </p>
            <DataStatus
              state={monitorState === 'checking' ? 'loading' : monitorState === 'live' ? 'live' : monitorState === 'stale' ? 'stale' : monitorState === 'error' ? 'error' : 'unavailable'}
              dataTime={quoteDataTime}
              source={quoteSource}
              message={monitorError || (lastCheckedAt ? `上次检查 ${new Date(lastCheckedAt).toLocaleString()}` : `通知权限 ${notificationStatus}`)}
              compact
            />
            <div className="mt-3 flex flex-wrap gap-2">
              <input
                value={alertSym}
                onChange={(e) => setAlertSym(e.target.value.toUpperCase())}
                placeholder="代码"
                className="w-24 rounded-lg border border-line bg-base px-2 py-1.5 font-mono text-sm"
              />
              <select
                value={alertOp}
                onChange={(e) => setAlertOp(e.target.value as '>=' | '<=')}
                className="rounded-lg border border-line bg-base px-2 py-1.5 text-sm"
              >
                <option value=">=">≥</option>
                <option value="<=">≤</option>
              </select>
              <input
                value={alertPrice}
                onChange={(e) => setAlertPrice(e.target.value)}
                placeholder="价格"
                className="w-24 rounded-lg border border-line bg-base px-2 py-1.5 font-mono text-sm"
              />
              <button
                type="button"
                onClick={() => {
                  const p = Number(alertPrice);
                  if (!alertSym || !(p > 0)) {
                    setAlertFeedback('请输入有效代码和价格');
                    return;
                  }
                  const added = addAlert(alertSym, alertOp, p);
                  if (!added) {
                    setAlertFeedback('相同提醒已存在');
                    return;
                  }
                  setAlertSym('');
                  setAlertPrice('');
                  setAlertFeedback('提醒已保存到本机');
                }}
                className="rounded-lg bg-accent/15 px-3 py-1.5 text-xs font-semibold text-accent"
              >
                添加
              </button>
            </div>
            <ul className="mt-2 space-y-1 text-xs text-muted">
              {alerts.map((a) => (
                <li key={a.id} className="flex items-center justify-between gap-2">
                  <span className="font-mono">
                    {a.symbol} {a.op} {a.price} · {a.venue}
                  </span>
                  <button type="button" className="text-down" onClick={() => removeAlert(a.id)}>
                    删
                  </button>
                </li>
              ))}
              {!alerts.length ? <li className="text-faint">暂无提醒</li> : null}
            </ul>
            {alertFeedback && <p className="mt-2 text-xs text-muted">{alertFeedback}</p>}
          </div>
          <div>
            <div className="mb-2 text-sm font-semibold text-ink">批量分析（美股/ETF）</div>
            <button
              type="button"
              onClick={() => void runBatch()}
              disabled={batchPolling}
              className="rounded-lg border border-accent/40 bg-accent/10 px-3 py-2 text-xs font-semibold text-accent"
            >
              {batchPolling ? '批量研究进行中…' : '分析当前观察+持仓（最多 10）'}
            </button>
            {batchJobId && !batchPolling ? <button type="button" onClick={() => void pollBatch(batchJobId)} className="ml-2 rounded-lg border border-line px-3 py-2 text-xs font-semibold text-muted">检查批量进度</button> : null}
            {batchStatus ? <p className="mt-2 text-xs text-muted">{batchStatus}</p> : null}
          </div>
        </section>

        {activeTab === 'watchlist' && (
          <div className="space-y-5">
            <section className="rounded-xl border border-line bg-surface p-4">
              <div className="mb-2 text-sm font-semibold text-ink">{t.addWatch}</div>
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t.searchPh}
                className="w-full max-w-md rounded-lg border border-line bg-base px-3 py-2 text-sm text-ink outline-none placeholder:text-faint focus:border-accent/50"
              />
              {hits.length > 0 && (
                <div className="mt-2 divide-y divide-line/60 overflow-hidden rounded-lg border border-line">
                  {hits.map((stock) => {
                    const watched = watchedSymbols.has(stock.symbol) || isInWatchlist(stock.symbol);
                    return (
                      <div key={stock.symbol} className="flex items-center gap-3 px-3 py-2">
                        <button
                          className="min-w-0 flex-1 text-left"
                          onClick={() => navigate(`/stock/${stock.symbol}`)}
                        >
                          <div className="flex items-center gap-2">
                            <span className="font-mono text-sm font-semibold text-ink">{stock.symbol}</span>
                            <span className="rounded bg-surface-3 px-1.5 py-0.5 text-[10px] uppercase text-faint">
                              {inferVenue(stock.symbol)}
                            </span>
                          </div>
                          <div className="truncate text-xs text-muted">{stock.name}</div>
                        </button>
                        <button
                          disabled={watched}
                          onClick={() => addToWatchlist(stock.symbol, stock.name, inferVenue(stock.symbol))}
                          className={`rounded-md border px-3 py-1.5 text-xs font-semibold transition ${
                            watched
                              ? 'cursor-default border-line text-faint'
                              : 'border-accent/40 bg-accent/10 text-accent hover:bg-accent/15'
                          }`}
                        >
                          {watched ? t.watched : t.add}
                        </button>
                      </div>
                    );
                  })}
                </div>
              )}
            </section>

            {enrichedWatchlist.length === 0 ? (
              <EmptyState
                title={t.emptyWatch}
                description={
                  <>
                    {t.emptyWatchDesc}{' '}
                    <a href="/scan" className="text-accent underline">
                      /scan
                    </a>
                  </>
                }
                action={{
                  label: t.goScan,
                  onClick: () => navigate('/scan'),
                }}
              />
            ) : (
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
                {enrichedWatchlist.map((item) => (
                  <WatchlistCard
                    key={item.symbol}
                    item={item}
                    onRemove={() => removeFromWatchlist(item.symbol)}
                    onClick={() => navigate(`/stock/${item.symbol}`)}
                  />
                ))}
              </div>
            )}
          </div>
        )}

        {activeTab === 'holdings' && (
          <div className="space-y-5">
            <section className="rounded-lg border border-line bg-surface p-4" aria-label="现金余额">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <div className="text-sm font-semibold text-ink">现金与敞口基准</div>
                  <p className="mt-1 text-[11px] text-faint">按币种填写可投资现金；不做汇率换算，也不连接券商账户。</p>
                </div>
                <div className="grid grid-cols-2 gap-2">
                  {(['USD', 'CNY'] as const).map((currency) => (
                    <label key={currency} className="text-[11px] text-muted">
                      {currency} 现金
                      <input
                        type="number"
                        min="0"
                        step="any"
                        value={cashBalances[currency]}
                        onChange={(event) => setCashBalance(currency, Number(event.target.value))}
                        className="mt-1 w-28 rounded-md border border-line bg-base px-2 py-1.5 font-mono text-xs text-ink"
                      />
                    </label>
                  ))}
                </div>
              </div>
            </section>

            {enrichedHoldings.length > 0 && (
              <section className="space-y-3" aria-label="组合风险概览">
                <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted">
                  <span>按币种独立汇总 · 不做汇率换算</span>
                  <span>报价覆盖 {portfolioRisk.quotedHoldingCount}/{portfolioRisk.totalHoldingCount}</span>
                </div>
                {portfolioRisk.groups.map((group) => {
                  const complete = group.quotedCount === group.holdingCount;
                  return (
                    <div key={group.currency} className="rounded-lg border border-line bg-surface p-4">
                      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                        <div className="text-sm font-semibold text-ink">{group.currency} 组合</div>
                        <div className={complete ? 'text-xs text-up' : 'text-xs text-accent'}>
                          {complete ? '报价完整' : `缺少 ${group.holdingCount - group.quotedCount} 个报价，估值、敞口与集中度已阻断`}
                        </div>
                      </div>
                      <div className="grid grid-cols-2 gap-x-4 gap-y-3 md:grid-cols-5">
                        <div>
                          <div className="text-[11px] text-muted">{t.costLabel}</div>
                          <div className="mt-1 font-mono text-sm text-ink">{formatCurrency(group.costBasis, group.currency)}</div>
                        </div>
                        <div>
                          <div className="text-[11px] text-muted">现金</div>
                          <div className="mt-1 font-mono text-sm text-ink">{formatCurrency(group.cashBalance, group.currency)}</div>
                        </div>
                        <div>
                          <div className="text-[11px] text-muted">{t.mktLabel}</div>
                          <div className="mt-1 font-mono text-sm text-ink">
                            {group.marketValue === undefined ? '—' : formatCurrency(group.marketValue, group.currency)}
                          </div>
                        </div>
                        <div>
                          <div className="text-[11px] text-muted">总权益</div>
                          <div className="mt-1 font-mono text-sm text-ink">
                            {group.totalEquity === undefined ? '—' : formatCurrency(group.totalEquity, group.currency)}
                          </div>
                        </div>
                        <div>
                          <div className="text-[11px] text-muted">总敞口</div>
                          <div className="mt-1 font-mono text-sm text-ink">
                            {group.grossExposure === undefined ? '—' : `${formatCurrency(group.grossExposure, group.currency)} · ${group.exposurePct?.toFixed(1)}%`}
                          </div>
                        </div>
                        <div>
                          <div className="text-[11px] text-muted">{t.plLabel}</div>
                          <div className={`mt-1 font-mono text-sm ${
                            group.profitLoss === undefined ? 'text-muted' : group.profitLoss >= 0 ? 'text-up' : 'text-down'
                          }`}>
                            {group.profitLoss === undefined
                              ? '—'
                              : `${formatCurrency(group.profitLoss, group.currency)} (${group.profitLossPercent?.toFixed(2)}%)`}
                          </div>
                        </div>
                        <div>
                          <div className="text-[11px] text-muted">集中度</div>
                          <div className={`mt-1 font-mono text-sm ${group.concentrationWarning ? 'text-down' : 'text-ink'}`}>
                            {group.top1Weight === undefined
                              ? '—'
                              : `Top1 ${group.top1Weight.toFixed(1)}% · Top3 ${group.top3Weight?.toFixed(1)}%`}
                          </div>
                        </div>
                        <div>
                          <div className="text-[11px] text-muted">行业集中度</div>
                          <div className={`mt-1 font-mono text-sm ${group.sectorConcentrationWarning ? 'text-down' : 'text-ink'}`}>
                            {group.largestSectorWeight === undefined
                              ? `— (${group.sectorCoveredCount}/${group.holdingCount} 已分类)`
                              : `${group.largestSector} ${group.largestSectorWeight.toFixed(1)}%`}
                          </div>
                        </div>
                        <div>
                          <div className="text-[11px] text-muted">止损覆盖</div>
                          <div className={`mt-1 font-mono text-sm ${group.stopCoveredCount < group.holdingCount ? 'text-down' : 'text-ink'}`}>
                            {group.stopCoveredCount}/{group.holdingCount} · {group.stopLossCoveragePct.toFixed(1)}% 成本
                          </div>
                        </div>
                        <div>
                          <div className="text-[11px] text-muted">最大计划亏损（成本至止损）</div>
                          <div className="mt-1 font-mono text-sm text-ink">
                            {group.maxPlannedLoss === undefined
                              ? '—（止损未覆盖）'
                              : `${formatCurrency(group.maxPlannedLoss, group.currency)}${group.maxPlannedLossPct === undefined ? '' : ` · ${group.maxPlannedLossPct.toFixed(2)}%`}`}
                          </div>
                        </div>
                      </div>
                      {group.concentrationWarning && (
                        <p className="mt-3 text-xs text-down">
                          {group.largestSymbol} 占该币种组合 {group.top1Weight?.toFixed(1)}%，超过默认 25% 集中度警示线。
                        </p>
                      )}
                      {group.sectorConcentrationWarning && (
                        <p className="mt-2 text-xs text-down">
                          {group.largestSector} 占该币种持仓 {group.largestSectorWeight?.toFixed(1)}%，超过默认 40% 行业警示线。
                        </p>
                      )}
                      {group.stopCoveredCount < group.holdingCount && (
                        <p className="mt-2 text-xs text-down">仍有 {group.holdingCount - group.stopCoveredCount} 个持仓未设置止损，最大计划亏损不可计算。</p>
                      )}
                    </div>
                  );
                })}
              </section>
            )}

            <section className="rounded-xl border border-line bg-surface p-4">
              <div className="mb-3 text-sm font-semibold text-ink">{t.addHold}</div>
              <form onSubmit={submitHolding} className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                <input
                  value={hSymbol}
                  onChange={(e) => setHSymbol(e.target.value)}
                  placeholder="NVDA"
                  required
                  className="rounded-lg border border-line bg-base px-3 py-2 font-mono text-sm text-ink outline-none focus:border-accent/50"
                />
                <input
                  value={hName}
                  onChange={(e) => setHName(e.target.value)}
                  placeholder={language === 'en' ? 'Name (optional)' : '名称（可选）'}
                  className="rounded-lg border border-line bg-base px-3 py-2 text-sm text-ink outline-none focus:border-accent/50"
                />
                <input
                  value={hQty}
                  onChange={(e) => setHQty(e.target.value)}
                  placeholder={t.qty}
                  type="number"
                  min="0"
                  step="any"
                  required
                  className="rounded-lg border border-line bg-base px-3 py-2 font-mono text-sm text-ink outline-none focus:border-accent/50"
                />
                <input
                  value={hCost}
                  onChange={(e) => setHCost(e.target.value)}
                  placeholder={t.cost}
                  type="number"
                  min="0"
                  step="any"
                  required
                  className="rounded-lg border border-line bg-base px-3 py-2 font-mono text-sm text-ink outline-none focus:border-accent/50"
                />
                <input
                  value={hSector}
                  onChange={(e) => setHSector(e.target.value)}
                  placeholder="行业（用于集中度）"
                  className="rounded-lg border border-line bg-base px-3 py-2 text-sm text-ink outline-none focus:border-accent/50"
                />
                <input
                  value={hStopLoss}
                  onChange={(e) => setHStopLoss(e.target.value)}
                  placeholder="计划止损价（可选）"
                  type="number"
                  min="0"
                  step="any"
                  className="rounded-lg border border-line bg-base px-3 py-2 font-mono text-sm text-ink outline-none focus:border-accent/50"
                />
                <input
                  value={hNotes}
                  onChange={(e) => setHNotes(e.target.value)}
                  placeholder={t.notes}
                  className="rounded-lg border border-line bg-base px-3 py-2 text-sm text-ink outline-none focus:border-accent/50 sm:col-span-2"
                />
                <button
                  type="submit"
                  className="rounded-lg bg-accent px-4 py-2 text-sm font-semibold text-black transition hover:opacity-90 sm:col-span-2 lg:col-span-1"
                >
                  {t.save}
                </button>
              </form>
            </section>

            {enrichedHoldings.length === 0 ? (
              <EmptyState title={t.emptyHold} description={t.emptyHoldDesc} />
            ) : (
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
                {enrichedHoldings.map((item) => (
                  <HoldingsCard
                    key={item.symbol}
                    item={item}
                    onRemove={() => removeHolding(item.symbol)}
                    onClick={() => navigate(`/stock/${item.symbol}`)}
                  />
                ))}
              </div>
            )}
          </div>
        )}

        <footer className="mt-16 space-y-2 border-t border-line pt-6 text-center text-xs text-faint">
          <div>我不是神 · 五方独立判读 · v0.6</div>
          <div className="text-sm font-semibold text-ink">Not a Stock God</div>
          <div>你不是神,但神陪你一起看股票</div>
          <div>本站不面向中国大陆用户 · This site is not intended for users in mainland China.</div>
          <div className="mx-auto max-w-3xl leading-relaxed">
            所有内容为信息整理与个人研究记录,非投资建议,不构成对任何证券、平台或交易所的要约或背书。股票、加密货币、代币化资产均涉及重大风险,可能损失全部本金。风险请自行分辨与承担。页面含邀请链接(含返佣)。交易前请自行研究并确认所在司法管辖区的合规性。
            组合、现金、行业和止损数据仅保存在当前浏览器，不会同步到其他设备；价格提醒仅在应用运行、页面可见且联网时轮询，不是券商托管条件单。
          </div>
          <div className="flex flex-wrap items-center justify-center gap-3 pt-2">
            <a href="/about" className="hover:text-ink">关于 / 方法论</a>
            <a href="/terms" className="hover:text-ink">服务条款</a>
            <a href="/privacy" className="hover:text-ink">隐私政策</a>
            <a href="/tactical" className="hover:text-ink">Paper 模拟</a>
          </div>
        </footer>
      </div>

      <PineScriptModal
        isOpen={isPineModalOpen}
        onClose={() => setIsPineModalOpen(false)}
      />
    </StockGodShell>
  );
};
