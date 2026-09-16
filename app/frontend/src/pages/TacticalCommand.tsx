import React, { useState, useEffect, useRef } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { APIError } from '../utils/api';
import { useAdminSession } from '../hooks/useAdminSession';
import { ShieldAlert, Moon, AlertCircle, Copy, Check, Calculator, Zap, ArrowRight, Activity, Bell, Flame, Globe, Compass, ChevronRight, MessageSquare, ClipboardList, Ban, TestTube2, RefreshCw } from 'lucide-react';
import { StockLogo } from '../components/StockLogo';
import { DataStatus } from '../components/common';
import { useQDIIPremiums } from '../hooks/useQDIIPremiums';
import { fetchQuoteResult, startQuotePolling, type QuoteResult } from '../utils/liveQuotes';
import { createOrderPlan, inferOrderCurrency, type TradeStrategy } from '../utils/orderPlan';
import { buildPaperOrderRequest, cancelPaperOrder, establishPaperDailyEquityBaseline, getPaperAccount, getPaperPortfolioRisk, getPaperStopScannerStatus, listPaperAuditEvents, listPaperOrders, paperSchedulerPollDelay, simulatePaperFill, submitPaperOrder, validatePaperOrder, verifyPaperAuditChain, type PaperAccount, type PaperAuditEvent, type PaperAuditVerification, type PaperOrder, type PaperPortfolioRisk, type PaperPosition, type PaperStopScannerStatus } from '../utils/paperOrders';

export const TacticalCommand: React.FC = () => {
  const { authorized: hasAdminAccess } = useAdminSession();
  const [searchParams] = useSearchParams();
  const sourceResearchTicker = searchParams.get('research_ticker')?.trim().toUpperCase() || '';
  const sourceResearchRunID = searchParams.get('research_run_id')?.trim() || '';
  // 1. 条件单生成器状态
  const [selectedTicker, setSelectedTicker] = useState(sourceResearchTicker || '513100');
  const [tickerName, setTickerName] = useState(sourceResearchTicker || '纳指ETF国泰');
  const [tradeStrategy, setTradeStrategy] = useState<TradeStrategy | ''>(sourceResearchTicker ? '' : 'BUY_LOW');
  const [householdFundsCNY, setHouseholdFundsCNY] = useState(50000);
  const [monthlyExpenseCNY, setMonthlyExpenseCNY] = useState(4000);
  const [existingQuantity, setExistingQuantity] = useState(0);
  const [paperOrders, setPaperOrders] = useState<PaperOrder[]>([]);
  const [paperListSummary, setPaperListSummary] = useState({ total: 0, hiddenTerminalCount: 0, truncated: false });
  const [paperAccounts, setPaperAccounts] = useState<PaperAccount[]>([]);
  const [paperPositions, setPaperPositions] = useState<PaperPosition[]>([]);
  const [paperPortfolio, setPaperPortfolio] = useState<PaperPortfolioRisk | null>(null);
  const [paperScheduler, setPaperScheduler] = useState<PaperStopScannerStatus | null>(null);
  const [activePaperOrder, setActivePaperOrder] = useState<PaperOrder | null>(null);
  const [paperBusy, setPaperBusy] = useState(false);
  const [paperRefreshBusy, setPaperRefreshBusy] = useState(false);
  const [paperError, setPaperError] = useState('');
  const [paperLoadError, setPaperLoadError] = useState('');
  const [paperAuditLoadError, setPaperAuditLoadError] = useState('');
  const [paperSchedulerError, setPaperSchedulerError] = useState('');
  const [paperWarning, setPaperWarning] = useState('');
  const [pendingClientOrderId, setPendingClientOrderId] = useState('');
  const [fillQuantity, setFillQuantity] = useState(1);
  const [fillConfirmOpen, setFillConfirmOpen] = useState(false);
  const [pendingFillId, setPendingFillId] = useState('');
  const [paperAuditEvents, setPaperAuditEvents] = useState<PaperAuditEvent[]>([]);
  const [paperAuditVerification, setPaperAuditVerification] = useState<PaperAuditVerification | null>(null);
  const paperRefreshGeneration = useRef(0);
  const [qdiiOpen, setQDIIopen] = useState(false);

  // 2. 实时行情与 QDII 杀溢价数据
  const {
    items: qdiiItems,
    loading: qdiiLoading,
    error: qdiiError,
    meta: qdiiMeta,
    trust: qdiiTrust,
    refresh: refreshQDII,
  } = useQDIIPremiums({ enabled: qdiiOpen, pollMs: qdiiOpen ? 30_000 : 0 });
  const [copied, setCopied] = useState(false);
  const [loading, setLoading] = useState(true);
  const [quoteResult, setQuoteResult] = useState<QuoteResult>({
    quotes: {},
    meta: { source: '', dataTime: '', refreshedAt: '', stale: false, staleReason: '', refreshable: false, partialErrors: [] },
    missing: [],
    errors: [],
  });
  const [quoteRefreshVersion, setQuoteRefreshVersion] = useState(0);
  const quoteFetchGeneration = useRef(0);

  useEffect(() => {
    const generation = ++quoteFetchGeneration.current;
    const poll = async () => {
      setLoading(true);
      try {
        const result = await fetchQuoteResult([selectedTicker.trim().toUpperCase()]);
        if (generation === quoteFetchGeneration.current) setQuoteResult(result);
      } catch (e) {
        console.warn('Failed to fetch tactical quotes:', e);
      } finally {
        if (generation === quoteFetchGeneration.current) setLoading(false);
      }
    };
    const stop = startQuotePolling(poll, 30_000);
    return () => {
      quoteFetchGeneration.current++;
      stop();
    };
  }, [selectedTicker, quoteRefreshVersion]);

  const refreshPaperState = async () => {
    const generation = ++paperRefreshGeneration.current;
    if (!hasAdminAccess) {
      setPaperLoadError('本机管理会话尚未就绪，请稍后重试或到设置页检查');
      setPaperRefreshBusy(false);
      return;
    }
    setPaperRefreshBusy(true);
    try {
      const [orders, account, portfolio] = await Promise.all([
        listPaperOrders(),
        getPaperAccount(),
        getPaperPortfolioRisk(),
      ]);
      if (generation !== paperRefreshGeneration.current) return;
      setPaperOrders(orders.orders);
      setPaperListSummary({ total: orders.total, hiddenTerminalCount: orders.hiddenTerminalCount, truncated: orders.truncated });
      setPaperAccounts(account.accounts);
      setPaperPositions(account.positions);
      setPaperPortfolio(portfolio);
      const [auditEvents, auditVerification] = await Promise.allSettled([
        listPaperAuditEvents(),
        verifyPaperAuditChain(),
      ]);
      if (generation !== paperRefreshGeneration.current) return;
      setPaperAuditEvents(auditEvents.status === 'fulfilled' ? auditEvents.value.events : []);
      setPaperAuditLoadError('');
      if (auditVerification.status === 'fulfilled') {
        setPaperAuditVerification(auditVerification.value);
      } else if (auditVerification.reason instanceof APIError && typeof auditVerification.reason.data?.valid === 'boolean') {
        setPaperAuditVerification(auditVerification.reason.data as unknown as PaperAuditVerification);
      } else {
        setPaperAuditVerification(null);
        setPaperAuditLoadError('订单与组合已加载，但审计链状态读取失败');
      }
      setPaperLoadError('');
    } catch (error) {
      if (generation === paperRefreshGeneration.current) {
        setPaperLoadError(error instanceof Error ? error.message : '无法读取 paper 订单记录');
      }
    } finally {
      if (generation === paperRefreshGeneration.current) setPaperRefreshBusy(false);
    }
  };

  useEffect(() => {
    void refreshPaperState();
  }, [hasAdminAccess]);

  useEffect(() => {
    if (!hasAdminAccess) return;
    let cancelled = false;
    let timeout: number | undefined;
    const pollScheduler = async () => {
      let delay = 5_000;
      try {
        const scheduler = await getPaperStopScannerStatus();
        if (cancelled) return;
        setPaperScheduler(scheduler);
        setPaperSchedulerError('');
        delay = paperSchedulerPollDelay(scheduler.interval);
      } catch (error) {
        if (!cancelled) setPaperSchedulerError(error instanceof Error ? error.message : '无法读取 Paper 扫描器状态');
      }
      if (!cancelled) timeout = window.setTimeout(() => void pollScheduler(), delay);
    };
    void pollScheduler();
    return () => {
      cancelled = true;
      if (timeout !== undefined) window.clearTimeout(timeout);
    };
  }, [hasAdminAccess]);

  // 当前选中股票的实时价格
  const normalizedTicker = selectedTicker.trim().toUpperCase();
  const selectedCurrency = inferOrderCurrency(normalizedTicker);
  const selectedPaperAccount = paperAccounts.find((account) => account.currency === selectedCurrency);
  const selectedPortfolioGroup = paperPortfolio?.groups.find((group) => group.currency === selectedCurrency);
  const dailyBaselineMissing = selectedPortfolioGroup != null && !selectedPortfolioGroup.dailyRiskComplete;
  const investableCapital = selectedPaperAccount?.cash ?? 0;
  const emergencyReserveCNY = monthlyExpenseCNY * 8;
  const householdInvestableCNY = Math.max(0, householdFundsCNY - emergencyReserveCNY);
  const householdSingleRiskCNY = householdInvestableCNY * 0.015;
  const currentQuote = quoteResult.quotes[normalizedTicker];
  const basePrice = currentQuote?.price || 0;
  const quoteSource = (currentQuote?.source || '').trim();
  const quoteDataTime = (currentQuote?.dataTime || quoteResult.meta.dataTime || '').trim();
  const quoteObservedAt = Date.parse(quoteDataTime);
  const quoteAgeMs = Number.isFinite(quoteObservedAt) ? Date.now() - quoteObservedAt : Number.POSITIVE_INFINITY;
  const quoteNeedsMarketReview = currentQuote?.session === 'closed' || quoteAgeMs < 0 || quoteAgeMs > 90_000;
  const planningBlockReason = !hasAdminAccess
    ? '需要管理会话才能读取服务器 Paper 账户'
    : !selectedPaperAccount
      ? `${selectedCurrency} Paper 账户不可用`
      : loading
        ? '正在获取报价'
        : quoteResult.missing.includes(normalizedTicker)
          ? '当前标的报价缺失'
          : !quoteDataTime || !quoteSource || quoteSource === 'unknown'
            ? '报价来源或数据时间未知'
            : quoteSource.startsWith('stale-snapshot:')
              ? '当前标的仅有过期快照'
              : '';
  const marketQuoteBlockReason = !hasAdminAccess
    ? '需要管理会话才能读取服务器 Paper 账户'
    : !selectedPaperAccount
      ? `${selectedCurrency} Paper 账户不可用`
    : loading
    ? '正在获取报价'
    : quoteResult.missing.includes(normalizedTicker)
      ? '当前标的报价缺失'
      : !quoteDataTime || !quoteSource || quoteSource === 'unknown'
        ? '报价来源或数据时间未知'
        : quoteSource.startsWith('stale-snapshot:')
          ? '当前标的仅有过期快照'
          : currentQuote?.session === 'closed'
            ? '当前标的市场已闭市；仅保留最后一次供应商观察值'
            : quoteAgeMs < 0 || quoteAgeMs > 90_000
              ? '当前标的报价已超过 90 秒新鲜度窗口'
          : '';
  const orderResult = !tradeStrategy
    ? { ok: false as const, error: '来源研究记录未提供方向或数量，请先由用户选择 Paper 策略' }
    : planningBlockReason
    ? { ok: false as const, error: planningBlockReason }
    : createOrderPlan({
        strategy: tradeStrategy,
        symbol: normalizedTicker,
        price: basePrice,
        investableCapital,
        existingQuantity,
      });
  const orderPlan = orderResult.ok ? orderResult.plan : null;
  const quoteBlockReason = dailyBaselineMissing && orderPlan?.side === 'BUY'
    ? `${selectedCurrency} Paper 子账户当日权益基线尚未建立；等待本地扫描器在常规交易时段建立后再提交 BUY`
    : marketQuoteBlockReason;
  const orderError = 'error' in orderResult ? orderResult.error : '';
  const calculatedOrderPrice = orderPlan?.entry?.toFixed(2) ?? orderPlan?.triggerPrice?.toFixed(2) ?? '—';
  const calculatedProtectiveStop = orderPlan?.protectiveStop?.toFixed(2) ?? '—';
  const calculatedTakeProfit = orderPlan?.takeProfit?.toFixed(2) ?? '—';
  const activeLifecycleRootID = activePaperOrder?.parentOrderId || activePaperOrder?.clientOrderId || '';
  const activeLifecycleOrders = activeLifecycleRootID
    ? paperOrders.filter((order) => order.clientOrderId === activeLifecycleRootID || order.parentOrderId === activeLifecycleRootID)
    : [];
  const orderedPaperOrders = [...paperOrders].sort((a, b) => {
    if (a.clientOrderId === activePaperOrder?.clientOrderId) return -1;
    if (b.clientOrderId === activePaperOrder?.clientOrderId) return 1;
    if (a.status === 'ACCEPTED' && b.status !== 'ACCEPTED') return -1;
    if (b.status === 'ACCEPTED' && a.status !== 'ACCEPTED') return 1;
    return 0;
  });
  const paperLifecycleSteps = [
    { label: '日初基线', done: Boolean(selectedPortfolioGroup?.dailyRiskComplete) },
    { label: '已提交', done: Boolean(activePaperOrder && activePaperOrder.status !== 'REJECTED') },
    { label: '分笔成交', done: activeLifecycleOrders.some((order) => (order.fills?.length ?? 0) > 0) },
    { label: 'OCO 保护', done: activeLifecycleOrders.some((order) => Boolean(order.parentOrderId && order.ocoGroupId)) },
    { label: '组合盈亏', done: Boolean(activePaperOrder && selectedPortfolioGroup?.dailyPnL != null) },
    { label: '审计有效', done: paperAuditVerification?.valid === true && paperAuditVerification.pendingCount === 0 },
  ];
  const relevantAuditEvents = paperAuditEvents.filter((event) =>
    event.action.startsWith('paper-') || event.action.startsWith('paper.') || event.action.startsWith('paper-risk.') || event.target.startsWith('paper-')
  ).slice(0, 8);

  const establishDailyBaseline = async () => {
    setPaperBusy(true);
    setPaperError('');
    setPaperWarning('');
    try {
      const response = await establishPaperDailyEquityBaseline(selectedCurrency);
      setPaperWarning(`${response.currency} ${response.marketDate} 日初权益 ${response.baseline.equity.toFixed(2)}（${response.created ? '已建立' : '已存在，未改写'}）`);
      await refreshPaperState();
    } catch (error) {
      setPaperError(error instanceof Error ? error.message : '日初基线建立失败');
    } finally {
      setPaperBusy(false);
    }
  };

  const copyPaperAuditText = () => {
    if (!orderPlan || !orderPlan.riskLimitPassed) {
      alert(orderError || '风险校验未通过，禁止复制');
      return;
    }
    const script = `【我不是神 PAPER 订单计划】
环境：PAPER 模拟（不会发送到任何券商）
标的：${normalizedTicker} (${tickerName})
Side：${orderPlan.side}
Market：${orderPlan.market}
Currency：${orderPlan.currency}
Order Type：${orderPlan.orderType}
TIF：${orderPlan.tif}
Entry：${orderPlan.entry == null ? 'N/A' : orderPlan.entry.toFixed(2)}
Trigger Price：${orderPlan.triggerPrice == null ? 'N/A' : orderPlan.triggerPrice.toFixed(2)}
Protective Stop：${orderPlan.protectiveStop == null ? 'N/A' : orderPlan.protectiveStop.toFixed(2)}
Take Profit：${orderPlan.takeProfit == null ? 'N/A' : orderPlan.takeProfit.toFixed(2)}
Quantity：${orderPlan.quantity}
Max Loss：${orderPlan.currency} ${orderPlan.maxLoss.toFixed(2)}
Risk Limit：PASS
报价来源：${quoteSource}
数据时间：${quoteDataTime}
风控上限：最大亏损 ${orderPlan.maxRiskAmount.toFixed(2)}；最大名义仓位 ${orderPlan.maxNotionalAmount.toFixed(2)}
说明：仅用于 paper 模拟和审计，不代表真实委托或成交。`;

    void navigator.clipboard.writeText(script).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };

  const submitCurrentPaperOrder = async () => {
    if (!orderPlan || !orderPlan.riskLimitPassed) return;
    setPaperBusy(true);
    setPaperError('');
    setPaperWarning('');
    const clientOrderId = pendingClientOrderId || `paper-${Date.now()}-${normalizedTicker}`;
    setPendingClientOrderId(clientOrderId);
    try {
      const request = buildPaperOrderRequest({
        clientOrderId,
        symbol: normalizedTicker,
        plan: orderPlan,
        referencePrice: basePrice,
        investableCapital,
        quoteSource,
        quoteTime: quoteDataTime,
        researchRunId: sourceResearchRunID || undefined,
        researchTicker: sourceResearchRunID ? sourceResearchTicker : undefined,
      });
      const validation = await validatePaperOrder(request);
      if (validation.portfolio) setPaperPortfolio(validation.portfolio);
      if (!validation.valid) {
        setPaperWarning('服务端预校验未通过，正在用同一 clientOrderId 写入可审计的 REJECTED Paper 记录');
      }
      const response = await submitPaperOrder(request);
      setActivePaperOrder(response.order);
      setFillConfirmOpen(false);
      setPendingClientOrderId('');
      await refreshPaperState();
      if (validation.degraded) {
        setPaperWarning(`减险单在组合估值降级状态下接受：${validation.warnings?.join('；') || '部分持仓无法可靠估值'}`);
      }
      if (response.order.status === 'REJECTED') {
        setPaperError(response.order.rejectionReason || validation.rejectionReasons.join('；'));
      }
    } catch (error) {
      if (error instanceof APIError && error.data?.order) {
        const rejected = error.data.order as PaperOrder;
        setActivePaperOrder(rejected);
        setPendingClientOrderId('');
        await refreshPaperState();
        setPaperError(rejected.rejectionReason || error.message || '服务器组合风控拒绝该订单');
      } else {
        setPaperError(error instanceof Error ? error.message : 'paper 订单提交失败');
      }
    } finally {
      setPaperBusy(false);
    }
  };

  const transitionPaperOrder = async (action: 'cancel' | 'fill') => {
    if (!activePaperOrder) return;
    setPaperBusy(true);
    setPaperError('');
    setPaperWarning('');
    try {
      const remaining = activePaperOrder.remainingQty;
      const requestedQuantity = Math.max(1, Math.min(Math.trunc(fillQuantity), remaining));
      const fillId = pendingFillId || `browser-${activePaperOrder.clientOrderId}-${globalThis.crypto.randomUUID()}`;
      if (action === 'fill' && !pendingFillId) setPendingFillId(fillId);
      const response = action === 'cancel'
        ? await cancelPaperOrder(activePaperOrder.clientOrderId)
        : await simulatePaperFill(activePaperOrder.clientOrderId, { fillId, quantity: requestedQuantity });
      setActivePaperOrder(response.order);
      setFillConfirmOpen(false);
      setFillQuantity(Math.max(1, response.order.remainingQty));
      setPendingFillId('');
      await refreshPaperState();
      if (response.warning) setPaperWarning(response.warning);
    } catch (error) {
      if (error instanceof APIError && error.data?.order) {
        setActivePaperOrder(error.data.order as PaperOrder);
        setPaperError((error.data.rejectionReasons as string[] | undefined)?.join('；') || error.message);
      } else {
        setPaperError(error instanceof Error ? error.message : 'paper 状态更新失败');
      }
    } finally {
      setPaperBusy(false);
    }
  };

  // 筛选高危险溢价 ETF
  const dangerQdii = qdiiItems.filter((i) => i.premiumPct >= 5.0);

  return (
    <div className="space-y-5 pb-10">
      {/* 🌙 模块 Header */}
      <div className="rounded-2xl border border-amber-500/30 bg-gradient-to-r from-[#141824] via-[#0d111a] to-[#17120a] p-4 sm:p-6 shadow-xl">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex items-center gap-3.5">
            <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-gradient-to-br from-amber-500 to-orange-600 font-black text-slate-950 shadow-lg shrink-0">
              <Moon size={28} />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h1 className="text-lg sm:text-2xl font-black text-ink tracking-tight">
                  夜间战术指挥所 · Paper 订单模拟台
                </h1>
                <span className="rounded-full bg-amber-500/15 px-2.5 py-0.5 text-xs font-bold text-amber-300 border border-amber-500/30">
                  Night-Owl Command
                </span>
              </div>
              <p className="text-xs sm:text-sm text-slate-400 mt-1">
                晚上集中做功课 · 验证 paper 订单参数 · 不连接券商、不产生真实成交
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2 shrink-0">
            <Link
              to="/copilot"
              className="flex items-center gap-1.5 rounded-xl border border-amber-500/40 bg-amber-500/10 px-3.5 py-2.5 text-xs sm:text-sm font-bold text-amber-300 hover:bg-amber-500/20 transition shadow"
            >
              <MessageSquare size={16} /> 💬 自由提问导师
            </Link>

          </div>
        </div>
      </div>

      <section className="border-y border-sky-500/30 bg-sky-950/20 px-3 py-3 text-xs leading-relaxed text-sky-100" aria-label="Paper 账户边界">
        <strong>Paper 工作台独立于 QDII 实时数据。</strong>
        <span className="ml-1 text-slate-300">CNY 与 USD 是彼此隔离的 Paper 子账户，现金、预占、持仓和风险分别记账；不做隐式汇兑或跨币种抵扣。QDII 失败不改写 Paper 账本状态。</span>
      </section>

      {sourceResearchTicker && (
        <section className="border-l-2 border-violet-400 bg-violet-950/20 px-3 py-3 text-xs leading-relaxed text-violet-100" aria-label="来源研究记录">
          <strong>来源研究记录：{sourceResearchTicker}</strong>{sourceResearchRunID && <span className="ml-1 break-all font-mono text-violet-200">· Run ID {sourceResearchRunID}</span>}
          <p className="mt-1 text-slate-300">研究记录只预填股票代码和 Run ID，没有传入方向、数量、价格或止损。任何 Paper 计划仍须重新取得新鲜权威报价，并通过服务端提交与成交风控。</p>
        </section>
      )}

      {/* 1. Paper 条件单计划生成器 */}
      <div className="rounded-2xl border border-line bg-surface p-4 sm:p-6 shadow-xl space-y-4">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between border-b border-line pb-3 gap-2">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-accent/15 text-accent border border-accent/30">
              <Calculator size={20} />
            </div>
            <div>
              <h2 className="text-base font-bold text-ink">人工条件单计划生成器</h2>
              <p className="text-xs text-muted">只生成和保存 paper 模拟订单，不会连接任何券商或执行交易</p>
            </div>
          </div>

          <div className="flex items-center gap-1.5 text-xs font-semibold overflow-x-auto">
            <button
              onClick={() => {
                setSelectedTicker('513100');
                setTickerName('纳指ETF国泰');
              }}
              className={`rounded-lg px-3 py-1.5 transition ${
                selectedTicker === '513100' ? 'bg-accent text-black font-bold' : 'bg-surface-2 text-muted'
              }`}
            >
              513100 (纳指ETF)
            </button>
            <button
              onClick={() => {
                setSelectedTicker('NVDA');
                setTickerName('英伟达');
              }}
              className={`rounded-lg px-3 py-1.5 transition ${
                selectedTicker === 'NVDA' ? 'bg-accent text-black font-bold' : 'bg-surface-2 text-muted'
              }`}
            >
              NVDA (英伟达)
            </button>
            <button
              onClick={() => {
                setSelectedTicker('300750');
                setTickerName('宁德时代');
              }}
              className={`rounded-lg px-3 py-1.5 transition ${
                selectedTicker === '300750' ? 'bg-accent text-black font-bold' : 'bg-surface-2 text-muted'
              }`}
            >
              300750 (宁德时代)
            </button>
            <button
              onClick={() => {
                setSelectedTicker('PLTR');
                setTickerName('Palantir');
              }}
              className={`rounded-lg px-3 py-1.5 transition ${
                selectedTicker === 'PLTR' ? 'bg-accent text-black font-bold' : 'bg-surface-2 text-muted'
              }`}
            >
              PLTR (Palantir)
            </button>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {/* Controls */}
          <div className="space-y-3 bg-surface-2/60 p-4 rounded-xl border border-line">
            <div>
              <label className="text-xs font-bold text-muted block mb-1">自定义股票代码 / 名称</label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={selectedTicker}
                  onChange={(e) => setSelectedTicker(e.target.value.toUpperCase())}
                  className="w-28 rounded-lg border border-line bg-surface px-3 py-1.5 text-xs text-ink font-mono uppercase"
                  placeholder="如 NVDA"
                />
                <input
                  type="text"
                  value={tickerName}
                  onChange={(e) => setTickerName(e.target.value)}
                  className="flex-1 rounded-lg border border-line bg-surface px-3 py-1.5 text-xs text-ink"
                  placeholder="标的名称"
                />
              </div>
            </div>

            <div>
              <label className="text-xs font-bold text-muted block mb-1">战术交易模式</label>
              <select
                value={tradeStrategy}
                onChange={(e) => setTradeStrategy(e.target.value as TradeStrategy)}
                className="w-full rounded-lg border border-line bg-surface px-3 py-1.5 text-xs text-ink font-medium"
              >
                {sourceResearchTicker && <option value="" disabled>请选择独立 Paper 策略（研究未提供方向）</option>}
                <option value="BUY_LOW">🎯 预设回撤买入 (-1.8%)</option>
                <option value="BREAKOUT">🚀 向上突破追踪买单 (顺势突破)</option>
                <option value="PROTECT_STOP">🛡️ 固定保护止损 (-3%)</option>
              </select>
            </div>

            {tradeStrategy === 'PROTECT_STOP' && (
              <div>
                <label className="text-xs font-bold text-muted block mb-1">现有持仓数量</label>
                <input
                  type="number"
                  min="0"
                  step="1"
                  value={existingQuantity}
                  onChange={(e) => setExistingQuantity(Number(e.target.value))}
                  className="w-full rounded-lg border border-line bg-surface px-3 py-1.5 text-xs text-ink font-mono"
                />
              </div>
            )}

            <div className="pt-2 border-t border-line/60">
              <div className="flex justify-between text-xs mb-1">
                <span className="text-muted">服务器可用资金:</span>
                <span className="font-mono font-bold text-sky-200">
                  {selectedCurrency} {selectedPaperAccount ? selectedPaperAccount.cash.toFixed(2) : '—'}
                </span>
              </div>
              <div className="mb-2 text-[10px] text-muted">按市场币种隔离，不做 USD/CNY 隐式换算</div>
              <div className="flex justify-between text-xs mb-1">
                <span className="text-muted">参考现价:</span>
                <span className="font-mono font-bold text-ink">
                  {orderPlan?.currency || '—'} {basePrice > 0 ? basePrice.toFixed(2) : '—'}
                </span>
              </div>
              <div className="text-[10px] text-muted">
                来源 {quoteSource || '未知'}{currentQuote?.providerMode ? ` · ${currentQuote.providerMode}` : ''} · 数据时间 {quoteDataTime || '未知'}
              </div>
			  {orderPlan && quoteNeedsMarketReview && <div className="mt-2 text-[11px] font-semibold text-amber-300">待开市复核：当前使用供应商最后观察价，仅可制定和复制计划，不能提交或成交。</div>}
			  {!orderPlan && <div className="mt-2 space-y-2">
				<div className="text-[11px] font-semibold text-rose-400">禁止复制：{orderError}</div>
				<button type="button" onClick={() => void (dailyBaselineMissing ? refreshPaperState() : setQuoteRefreshVersion((value) => value + 1))} disabled={loading || paperBusy} className="inline-flex h-8 items-center gap-1.5 rounded-md border border-line bg-surface px-2.5 text-[11px] font-semibold text-sky-200 hover:bg-surface-2 disabled:cursor-wait disabled:opacity-50">
				  <RefreshCw size={13} className={loading || paperBusy ? 'animate-spin' : ''} /> {dailyBaselineMissing ? '重新检查日初基线' : loading ? '正在获取报价…' : '重新获取报价'}
				</button>
			  </div>}
            </div>
          </div>

          {/* Calculated Output Card */}
          <div className="md:col-span-2 bg-gradient-to-br from-slate-900 to-[#0e1320] p-4 rounded-xl border border-amber-500/30 flex flex-col justify-between space-y-3">
            <div>
              <div className="flex items-center justify-between border-b border-slate-800 pb-2">
                <span className="font-bold text-amber-300 text-sm flex items-center gap-1.5">
                  <StockLogo symbol={selectedTicker} size={20} />
                  {selectedTicker} ({tickerName}) · {orderPlan ? (quoteNeedsMarketReview ? '待开市复核计划' : '条件单参数已计算') : '条件单暂不可用'}
                </span>
                <span className="rounded bg-amber-500/20 px-2 py-0.5 text-[10px] font-mono text-amber-300">
                  {orderPlan ? (quoteNeedsMarketReview ? '仅草稿' : '风控双上限已校验') : '禁止复制'}
                </span>
              </div>

              <div className="grid grid-cols-3 gap-2 mt-3 text-center">
                <div className="rounded-lg bg-slate-950 p-2.5 border border-slate-800">
                  <div className="text-[10px] text-slate-400">{orderPlan?.orderType === 'STOP_MARKET' ? '条件触发价' : '限价委托价'}</div>
                  <div className="text-sm sm:text-base font-black text-amber-400 font-mono mt-0.5">
                    {orderPlan?.currency || '—'} {calculatedOrderPrice}
                  </div>
                </div>
                <div className="rounded-lg bg-rose-950/40 p-2.5 border border-rose-500/30">
                  <div className="text-[10px] text-rose-300">买入后的保护止损</div>
                  <div className="text-sm sm:text-base font-black text-rose-400 font-mono mt-0.5">
                    {orderPlan?.currency || '—'} {calculatedProtectiveStop}
                  </div>
                </div>
                <div className="rounded-lg bg-emerald-950/40 p-2.5 border border-emerald-500/30">
                  <div className="text-[10px] text-emerald-300">止盈目标位</div>
                  <div className="text-sm sm:text-base font-black text-emerald-400 font-mono mt-0.5">
                    {orderPlan?.currency || '—'} {calculatedTakeProfit}
                  </div>
                </div>
              </div>
            </div>

            <div className="flex items-center justify-between pt-2 border-t border-slate-800">
              <span className="text-[11px] text-slate-400">
                STOP_MARKET 仅由本地进程运行时扫描，并非券商 GTC；不代表真实委托或成交
              </span>
              <button
                onClick={copyPaperAuditText}
                disabled={!orderPlan || !orderPlan.riskLimitPassed}
                className="flex items-center gap-1.5 rounded-lg bg-amber-500 px-3 py-1.5 text-xs font-bold text-slate-950 hover:brightness-110 transition shrink-0 disabled:cursor-not-allowed disabled:opacity-40"
              >
                {copied ? <Check size={14} /> : <Copy size={14} />}
                <span>{copied ? '已复制审计文本' : '复制 Paper 审计文本'}</span>
              </button>
            </div>
            {orderPlan && (
              <div className="grid grid-cols-2 gap-2 text-[11px] text-slate-300 sm:grid-cols-4">
                <span>Side <b>{orderPlan.side}</b></span>
                <span>数量 <b>{orderPlan.quantity}</b></span>
                <span className={orderPlan.riskLimitPassed ? '' : 'text-rose-300'}>
                  最大亏损 <b>{orderPlan.currency} {orderPlan.maxLoss.toFixed(2)}</b>
                </span>
                <span>名义上限 <b>{orderPlan.currency} {orderPlan.maxNotionalAmount.toFixed(2)}</b></span>
                {!orderPlan.riskLimitPassed && (
                  <span className="col-span-2 text-rose-300 sm:col-span-4">
                    当前持仓止损风险超过 1.5% 预算；保护单覆盖全部持仓，请另行评估减仓。
                  </span>
                )}
              </div>
            )}
            <button
              type="button"
              onClick={() => void submitCurrentPaperOrder()}
              disabled={!hasAdminAccess || !orderPlan || !orderPlan.riskLimitPassed || Boolean(quoteBlockReason) || paperBusy}
              className="flex h-10 w-full items-center justify-center gap-2 rounded-lg border border-sky-400/40 bg-sky-500/15 px-3 text-xs font-bold text-sky-200 transition hover:bg-sky-500/25 disabled:cursor-not-allowed disabled:opacity-40"
            >
              <ClipboardList size={15} />
              {paperBusy ? '处理中…' : quoteBlockReason && orderPlan ? `暂不可提交：${quoteBlockReason}` : '验证并提交 Paper 订单'}
            </button>
            {!hasAdminAccess && (
              <p className="text-xs text-amber-300">管理操作已锁定。<Link className="font-bold underline" to="/settings">去设置授权</Link></p>
            )}
            {paperError && <p className="text-xs font-semibold text-rose-300">Paper 操作未完成：{paperError}</p>}
            {paperWarning && <p className="text-xs font-semibold text-amber-300">{paperWarning}</p>}
          </div>
        </div>
      </div>

      <section className="border-y border-sky-500/20 bg-sky-950/20 py-4" aria-label="Paper 订单生命周期">
        <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 className="text-sm font-bold text-sky-100">Paper 订单生命周期</h2>
            <p className="mt-1 text-xs text-slate-400">保护 STOP 与止盈 SELL LIMIT 是本地模拟 OCO；仅用本进程观测到的新鲜权威报价触发，不是券商托管 GTC。</p>
          </div>
          <span className="border border-sky-400/40 bg-sky-500/10 px-2 py-1 text-[10px] font-black text-sky-200">PAPER ONLY</span>
        </div>

        <ol className="mb-3 grid grid-cols-2 gap-px border border-sky-500/20 bg-sky-500/20 sm:grid-cols-3 xl:grid-cols-6" aria-label="Paper 生命周期进度">
          {paperLifecycleSteps.map((step, index) => (
            <li key={step.label} className={`min-h-14 bg-slate-950 px-3 py-2 text-[11px] ${step.done ? 'text-emerald-200' : 'text-slate-500'}`}>
              <div className="font-mono">{index + 1}/6</div>
              <div className="mt-1 font-bold">{step.done ? '已完成' : '待完成'} · {step.label}</div>
            </li>
          ))}
        </ol>

        <div className={`mb-3 border p-3 ${paperScheduler?.running ? 'border-emerald-500/40 bg-emerald-950/20' : hasAdminAccess ? 'border-rose-500/60 bg-rose-950/30' : 'border-amber-500/40 bg-amber-950/20'}`}>
          <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
            <span className={`font-black ${paperScheduler?.running ? 'text-emerald-200' : hasAdminAccess ? 'text-rose-200' : 'text-amber-200'}`}>
              本地 OCO 扫描器：{paperScheduler?.running ? 'RUNNING' : hasAdminAccess ? 'STOPPED / 读取失败' : '未授权，状态不可见'}
            </span>
            <span className="text-slate-400">间隔 {paperScheduler?.interval || '—'}</span>
          </div>
          <div className="mt-2 grid gap-1 text-[11px] text-slate-300 md:grid-cols-2">
            <span>最近扫描：{paperScheduler?.lastResult?.completedAt || '尚无可验证扫描记录'}</span>
            <span>关闭市场 {paperScheduler?.lastResult?.closedMarket ?? '—'} · 扫描错误 {paperScheduler?.lastResult?.errors ?? '—'} · 恢复错误 {paperScheduler?.recoveryErrors?.length ?? '—'} · 触发 {paperScheduler?.lastResult?.triggered ?? '—'} · 权益点 {paperScheduler?.lastResult?.equityCheckpointed ? '已写入' : '未写入'}</span>
            <span>交易日历：{paperScheduler?.calendarCoverage ? '服务端已返回覆盖声明；覆盖范围外不视为可交易时段' : '覆盖声明未知，相关触发不可视为可靠'}</span>
            <span>会话规则：{paperScheduler?.sessionPolicy ? '服务端已返回；只在规则允许的时段评估触发' : '规则未知，相关触发不可视为可靠'}</span>
            <span className="md:col-span-2">审计记录：{paperScheduler?.auditRetention ? '服务端已返回保留策略；状态变化与空闲轮询须按原文区分' : '保留策略未知，无法确认扫描记录边界'}</span>
          </div>
          <p className={`mt-2 text-[11px] ${paperScheduler?.running || !hasAdminAccess ? 'text-amber-200' : 'font-bold text-rose-200'}`}>
            本地扫描器不是券商托管 GTC；状态不可见或规则缺失时，不应把保护 STOP 视为已托管，也不代表真实委托或成交。
          </p>
          {paperSchedulerError && <p className="mt-2 text-[11px] font-semibold text-rose-300">扫描器状态读取失败：{paperSchedulerError}</p>}
          {dailyBaselineMissing && hasAdminAccess && (
            <div className="mt-3 flex flex-wrap items-center justify-between gap-2 border-t border-sky-500/20 pt-3">
              <span className="text-[11px] text-amber-200">{selectedCurrency} 当日基线缺失；只能在常规交易时段用服务端权威估值建立。</span>
              <button type="button" onClick={() => void establishDailyBaseline()} disabled={paperBusy || !paperScheduler?.running} className="h-8 border border-sky-400/40 bg-sky-500/10 px-3 text-[11px] font-bold text-sky-200 disabled:opacity-40">
                {paperBusy ? '建立中…' : `建立 ${selectedCurrency} 日初基线`}
              </button>
            </div>
          )}
          {paperScheduler && <details className="mt-2 border-t border-sky-500/20 pt-2 text-[11px] text-slate-400"><summary className="cursor-pointer font-semibold text-sky-200">查看服务端日历、会话、审计与免责声明原文</summary><div className="mt-2 space-y-1 break-words font-mono"><div>calendar_coverage: {paperScheduler.calendarCoverage || 'unknown'}</div><div>session_policy: {paperScheduler.sessionPolicy || 'unknown'}</div><div>audit_retention: {paperScheduler.auditRetention || 'unknown'}</div><div>disclaimer: {paperScheduler.disclaimer || 'unknown'}</div></div></details>}
        </div>

        <div className="mb-3 grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
          {paperAccounts.map((account) => (
            <div key={account.currency} className="border border-sky-500/20 bg-slate-950/50 p-3 text-xs text-slate-300">
              <div className="font-bold text-sky-200">{account.currency} Paper 账户</div>
              <div className="mt-1">可用现金 {account.cash.toFixed(2)}</div>
              <div>预占现金 {account.reservedCash.toFixed(2)} · 版本 {account.version}</div>
            </div>
          ))}
          {paperPositions.map((position) => (
            <div key={`${position.currency}-${position.symbol}`} className="border border-emerald-500/20 bg-slate-950/50 p-3 text-xs text-slate-300">
              <div className="font-bold text-emerald-200">{position.symbol} Paper 持仓</div>
              <div className="mt-1">数量 {position.quantity} · 预占 {position.reservedQuantity}</div>
              <div>均价 {position.currency} {position.averageCost.toFixed(2)}</div>
            </div>
          ))}
        </div>

        {paperPortfolio && (
          <div className="mb-3 border border-sky-500/20 bg-slate-950/50 p-3">
            <div className="mb-2 flex flex-wrap items-center justify-between gap-2 text-xs">
              <span className="font-bold text-sky-200">服务器 Paper 组合风控 · 策略 {paperPortfolio.limits.policyVersion}</span>
              <span className={paperPortfolio.riskComplete ? 'text-emerald-300' : 'text-rose-300'}>
                {paperPortfolio.riskComplete ? '估值、行业与 ADV 完整' : `相关币种买单已阻断：${paperPortfolio.unknownSymbols.join(', ') || '数据不完整'}`}
              </span>
            </div>
            <div className="mb-2 text-[11px] text-slate-400">
              CNY 基准币总览：{paperPortfolio.baseCurrency.status === 'known' && paperPortfolio.baseCurrency.totalEquity != null
                ? `${paperPortfolio.baseCurrency.totalEquity.toFixed(2)}；仅在带来源、时间且有效的汇率证据下合并展示`
                : '未知；缺少可复核汇率时不合并权益，各币种仍按独立子账户与独立限额管理'}
              <details className="mt-1"><summary className="cursor-pointer font-semibold text-sky-200">查看汇率隔离原文与证据</summary><div className="mt-1 break-words font-mono">status: {paperPortfolio.baseCurrency.status} · source: {paperPortfolio.baseCurrency.fxSource || 'unknown'} · observed_at: {paperPortfolio.baseCurrency.fxObservedAt || 'unknown'} · reason: {paperPortfolio.baseCurrency.reason || 'none'}</div></details>
            </div>
            <div className="grid gap-2 text-[11px] text-slate-300 sm:grid-cols-2 xl:grid-cols-4">
              {paperPortfolio.groups.map((group) => {
                const equity = group.totalEquity ?? 0;
                const dailyLossPct = group.dailyLossPct ?? 0;
                const stressLossPct = equity > 0 ? Math.max(0, ...group.stressScenarios.map((item) => item.pnl != null && item.pnl < 0 ? (-item.pnl / equity) * 100 : 0)) : 0;
                return (
                <div key={group.currency}>
                  <b>{group.currency}</b> · 总权益 {group.totalEquity?.toFixed(2) ?? '—'} · 总敞口 {group.grossExposurePct?.toFixed(1) ?? '—'}% / {paperPortfolio.limits.maxPortfolioExposurePct}%
                  <br />单票 {group.largestSymbol || '—'} {group.largestSymbolPct?.toFixed(1) ?? '—'}% · 行业 {group.largestSector || '—'} {group.largestSectorPct?.toFixed(1) ?? '—'}%
                  <br />止损覆盖 {group.stopCoveragePct.toFixed(1)}% / 最低 {paperPortfolio.limits.minStopCoveragePct}% · 未结计划亏损 {group.openOrderMaxLossPct?.toFixed(2) ?? '—'}% / {paperPortfolio.limits.maxOpenOrderLossPct}%
                  <br />日亏 {group.dailyRiskComplete ? `${dailyLossPct.toFixed(2)}%` : '未知'} / 最高 {paperPortfolio.limits.maxDailyLossPct}% · 日初权益 {group.dayStartEquity?.toFixed(2) ?? '—'} · 日PnL {group.dailyPnL?.toFixed(2) ?? '—'}
                  <br />回撤 {group.peakDrawdownPct?.toFixed(2) ?? '—'}% / 最高 {paperPortfolio.limits.maxDrawdownPct}% · 压力损失 {stressLossPct.toFixed(2)}% / 最高 {paperPortfolio.limits.maxStressLossPct}%
                  <br />ADV {group.liquidityComplete ? '完整' : '未知，BUY 阻断'}
                  <br />压力 -5% {group.stressScenarios.find((item) => item.priceShockPct === -5)?.pnl?.toFixed(2) ?? '—'} · -10% {group.stressScenarios.find((item) => item.priceShockPct === -10)?.pnl?.toFixed(2) ?? '—'}
                  <br />行业压力 {group.stressScenarios.find((item) => item.kind === 'sector-concentration')?.pnl?.toFixed(2) ?? '未知'} · 流动性占量 {group.stressScenarios.find((item) => item.kind === 'liquidity')?.liquidityUsagePct != null ? `${group.stressScenarios.find((item) => item.kind === 'liquidity')!.liquidityUsagePct!.toFixed(2)}%` : '未知'}
                </div>
              )})}
            </div>
          </div>
        )}

        {activePaperOrder && (
          <div className="mb-3 grid gap-3 border border-sky-500/30 bg-slate-950/60 p-3 text-xs md:grid-cols-[1fr_auto]">
            <div className="space-y-1 text-slate-300">
              <div className="font-mono text-sky-200">{activePaperOrder.clientOrderId}</div>
              <div>{activePaperOrder.symbol} · {activePaperOrder.side} {activePaperOrder.quantity} · {activePaperOrder.orderType}</div>
              <div>状态 <b>{activePaperOrder.status}</b> · 历史 {activePaperOrder.statusHistory.length} 条</div>
              {activePaperOrder.riskSnapshot.submissionPolicyCheck && (
                <div className={activePaperOrder.riskSnapshot.submissionPolicyCheck.passed ? 'text-emerald-200' : 'text-rose-300'}>
                  提交检查 {activePaperOrder.riskSnapshot.submissionPolicyCheck.policyVersion} · {activePaperOrder.riskSnapshot.submissionPolicyCheck.checkedAt} · {activePaperOrder.riskSnapshot.submissionPolicyCheck.passed ? '通过' : '阻断'}
                  {activePaperOrder.riskSnapshot.submissionPolicyCheck.reasons?.length ? `：${activePaperOrder.riskSnapshot.submissionPolicyCheck.reasons.join('；')}` : ''}
                </div>
              )}
              {activePaperOrder.riskSnapshot.fillPolicyCheck && (
                <div className={activePaperOrder.riskSnapshot.fillPolicyCheck.passed ? 'text-emerald-200' : 'text-rose-300'}>
                  成交复验 {activePaperOrder.riskSnapshot.fillPolicyCheck.policyVersion} · {activePaperOrder.riskSnapshot.fillPolicyCheck.checkedAt} · {activePaperOrder.riskSnapshot.fillPolicyCheck.passed ? '通过' : '阻断'}
                  {activePaperOrder.riskSnapshot.fillPolicyCheck.reasons?.length ? `：${activePaperOrder.riskSnapshot.fillPolicyCheck.reasons.join('；')}` : ''}
                </div>
              )}
              {(activePaperOrder.fillQty ?? 0) > 0 && (
                <div className="text-emerald-200">
                  累计模拟成交 {activePaperOrder.fillQty} / {activePaperOrder.quantity} @ {activePaperOrder.fillPrice?.toFixed(4)} · 剩余 {activePaperOrder.remainingQty} · 滑点 {activePaperOrder.slippage?.toFixed(4)} · 费用 {activePaperOrder.currency} {activePaperOrder.fee?.toFixed(2)}
                  {activePaperOrder.fillModel ? ` · ${activePaperOrder.fillModel}` : ''}
                </div>
              )}
              {!!activePaperOrder.fills?.length && (
                <div className="mt-2 border-t border-sky-500/20 pt-2 text-[11px] text-slate-300">
                  {activePaperOrder.fills.map((fill) => (
                    <div key={fill.fillId} className="font-mono">
                      #{fill.sequence} {fill.quantity} @ {fill.price.toFixed(4)} · quote {fill.quote.price.toFixed(4)} · {fill.quote.source} · {fill.quote.observedAt}
                      {fill.quote.providerURL ? ` · ${fill.quote.providerURL}` : ''} · policy {fill.policyVersion || '—'}
                    </div>
                  ))}
                </div>
              )}
              {activePaperOrder.rejectionReason && <div className="text-rose-300">拒绝原因：{activePaperOrder.rejectionReason}</div>}
              {activePaperOrder.researchRunId && <div className="text-sky-200">来源研究：{activePaperOrder.researchTicker} · <span className="font-mono">{activePaperOrder.researchRunId}</span></div>}
            </div>
            {activePaperOrder.status === 'ACCEPTED' && (
              <div className="flex flex-wrap items-center justify-end gap-2">
                <button
                  type="button"
                  onClick={() => void transitionPaperOrder('cancel')}
                  disabled={paperBusy}
                  className="flex h-11 items-center gap-1.5 border border-rose-500/40 px-3 font-bold text-rose-200 disabled:opacity-40 sm:h-9"
                ><Ban size={14} />取消 Paper 订单</button>
                {!fillConfirmOpen ? (
                  <button
                    type="button"
                    onClick={() => setFillConfirmOpen(true)}
                    disabled={paperBusy}
                    className="flex h-11 items-center gap-1.5 border border-emerald-500/40 px-3 font-bold text-emerald-200 disabled:opacity-40 sm:h-9"
                  ><TestTube2 size={14} />复核模拟成交</button>
                ) : (
                  <div className="flex flex-wrap items-center gap-2 border border-amber-400/40 bg-amber-400/5 p-2 text-[11px] text-amber-100">
                    <span>将写入 {Math.min(fillQuantity, activePaperOrder.remainingQty)} 股模拟成交，写入后进入审计链。</span>
                    <button type="button" onClick={() => setFillConfirmOpen(false)} className="h-11 border border-line px-3 sm:h-9">返回</button>
                    <button type="button" onClick={() => void transitionPaperOrder('fill')} disabled={paperBusy} className="h-11 border border-emerald-500/50 px-3 font-bold text-emerald-200 disabled:opacity-40 sm:h-9">确认标记成交</button>
                  </div>
                )}
                <label className="flex items-center gap-1 text-[11px] text-slate-400">数量
                  <input aria-label="模拟成交数量" type="number" min={1} max={activePaperOrder.remainingQty} step={1} value={Math.min(fillQuantity, activePaperOrder.remainingQty)} onChange={(event) => { setFillQuantity(Number(event.target.value)); setPendingFillId(''); setFillConfirmOpen(false); }} className="h-11 w-20 border border-line bg-slate-950 px-2 font-mono text-slate-200 sm:h-9" />
                </label>
              </div>
            )}
            {activePaperOrder.status !== 'ACCEPTED' && (
              <Link
                to={`/journal?symbol=${encodeURIComponent(activePaperOrder.symbol)}&paper_order_id=${encodeURIComponent(activePaperOrder.clientOrderId)}${activePaperOrder.researchRunId ? `&research_run_id=${encodeURIComponent(activePaperOrder.researchRunId)}` : ''}`}
                className="inline-flex min-h-11 items-center gap-1.5 border border-sky-500/40 px-3 text-xs font-bold text-sky-200 sm:min-h-9"
              ><ClipboardList size={14} />记录到交易复盘</Link>
            )}
          </div>
        )}

        <div className="max-h-[32rem] overflow-auto border border-line">
          <table className="w-full min-w-[720px] text-left text-xs">
            <thead className="bg-slate-900 text-slate-400">
              <tr><th className="px-3 py-2">Client Order ID</th><th>标的</th><th>状态</th><th>报价来源 / 时间</th><th>风险快照</th></tr>
            </thead>
            <tbody>
              {orderedPaperOrders.map((order) => (
                <tr key={order.clientOrderId} className="border-t border-line text-slate-300">
                  <td className="px-3 py-2 font-mono text-sky-200">{order.clientOrderId}</td>
                  <td>{order.symbol} · {order.side} {order.quantity}</td>
                  <td><button type="button" className="font-bold text-sky-200" onClick={() => { setActivePaperOrder(order); setFillQuantity(Math.max(1, order.remainingQty)); setPendingFillId(''); setFillConfirmOpen(false); }}>{order.status}</button></td>
                  <td>{order.quoteSource}<br /><span className="text-[10px] text-slate-500">{order.quoteTime}</span></td>
                  <td>{order.currency} {order.riskSnapshot.maxLoss.toFixed(2)} / {order.riskSnapshot.maxRiskAmount.toFixed(2)}<br /><span className="text-[10px] text-slate-500">{order.riskSnapshot.policyVersion}</span></td>
                </tr>
              ))}
              {!paperOrders.length && <tr><td colSpan={5} className="px-3 py-5 text-center text-slate-500">暂无 paper 订单记录</td></tr>}
            </tbody>
          </table>
        </div>
        {paperListSummary.truncated && (
          <p className="mt-2 text-xs text-amber-200">
            已显示全部活动/保护订单与最近终态记录；另有 {paperListSummary.hiddenTerminalCount} 条较早终态记录未展开（总计 {paperListSummary.total}）。
          </p>
        )}
        {paperLoadError && <p className="mt-2 text-xs font-semibold text-rose-300">Paper 状态读取失败：{paperLoadError}</p>}
        <div className="mt-3 border border-sky-500/20 bg-slate-950/50 p-3 text-[11px] text-slate-300" aria-label="Paper 审计验证">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <b className={paperAuditVerification?.valid && !paperAuditVerification.pendingCount ? 'text-emerald-200' : 'text-rose-300'}>
              审计链 {paperAuditVerification?.valid ? '有效' : '未验证或异常'} · pending {paperAuditVerification?.pendingCount ?? '—'}
            </b>
            <button type="button" onClick={() => void refreshPaperState()} disabled={paperBusy || paperRefreshBusy} className="h-8 border border-line px-3 font-bold text-sky-200 disabled:opacity-40">{paperRefreshBusy ? '刷新中…' : '刷新订单与审计'}</button>
          </div>
          {paperAuditVerification?.reason && <div className="mt-1 text-rose-300">{paperAuditVerification.reason}</div>}
          {paperAuditLoadError && <div className="mt-1 text-rose-300">{paperAuditLoadError}</div>}
          <div className="mt-2 space-y-1 border-t border-sky-500/20 pt-2 font-mono">
            {relevantAuditEvents.map((event) => <div key={event.id}>#{event.id} {event.action} · {event.status} · {event.target}</div>)}
            {!relevantAuditEvents.length && <div className="text-slate-500">暂无 Paper 审计记录</div>}
          </div>
        </div>
      </section>

      <details className="border-y border-line bg-surface/40 px-3 py-3" open={qdiiOpen} onToggle={(event) => setQDIIopen(event.currentTarget.open)}>
        <summary className="flex cursor-pointer list-none items-center justify-between gap-3 text-sm font-bold text-muted">
          <span className="flex items-center gap-2"><ShieldAlert size={18} className="text-amber-300" />QDII 溢价数据（可选实时观察）</span>
          <span className="shrink-0 text-[10px] font-semibold text-faint">不影响 Paper 子账户</span>
        </summary>
        <div className="mt-3 space-y-3 border-t border-line pt-3">
          <div className="flex items-center justify-between gap-3">
            <p className="text-xs leading-relaxed text-muted">QDII 只提供中性溢价等级观察，不产生买卖或安全结论。</p>
            {qdiiTrust.state === 'live' ? <span className="shrink-0 text-xs font-semibold text-amber-200">{dangerQdii.length} 只高溢价观察</span> : <span className="shrink-0 text-xs font-semibold text-amber-200">数据待核验</span>}
          </div>
          {qdiiLoading && qdiiMeta === null ? (
            <DataStatus state="loading" label="正在加载 QDII 数据" />
          ) : qdiiError || qdiiTrust.state !== 'live' ? (
            <div className="border border-amber-500/30 bg-amber-950/20 p-3">
              <DataStatus state={qdiiError ? 'error' : qdiiTrust.state} label={qdiiError ? 'QDII 数据加载失败' : qdiiTrust.state === 'stale' ? 'QDII 数据已过期' : 'QDII 数据不可用'} dataTime={qdiiMeta?.dataTime || 'unknown'} source={qdiiMeta?.source} message={qdiiError || qdiiTrust.reason} onRetry={() => void refreshQDII()} />
              <p className="mt-2 text-xs leading-relaxed text-slate-300">价格、净值、溢价率、危险数量与风险结论已隐藏；任一关键字段或整体状态不可验证时不展示等级。</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 gap-2 text-xs sm:grid-cols-2 lg:grid-cols-3">
              <div className="sm:col-span-2 lg:col-span-3"><DataStatus state="live" label={qdiiLoading ? 'QDII 已验证数据 · 刷新中' : 'QDII 数据已验证'} dataTime={qdiiMeta?.dataTime} source={qdiiMeta?.source} /></div>
              {dangerQdii.length ? dangerQdii.map((item) => (
                <div key={item.code} className="flex items-center justify-between border border-amber-500/30 bg-amber-950/20 p-3"><div><div className="font-bold text-amber-100">{item.name}</div><div className="font-mono text-[10px] text-muted">{item.code}</div></div><div className="text-right"><div className="font-mono font-bold text-amber-200">+{item.premiumPct.toFixed(1)}%</div><div className="text-[10px] text-muted">高溢价观察</div></div></div>
              )) : <p className="sm:col-span-2 lg:col-span-3">当前可验证数据中没有达到 5% 高溢价观察阈值的条目。</p>}
            </div>
          )}
        </div>
      </details>

      {/* 3. CNY 生活账本计算器 */}
      <div className="rounded-2xl border border-slate-800 bg-[#0c101c] p-4 sm:p-6 shadow-xl space-y-4">
        <div className="flex items-center gap-2.5 border-b border-slate-800 pb-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-blue-500/15 text-blue-400 border border-blue-500/30">
            <Calculator size={20} />
          </div>
          <div>
            <h2 className="text-base font-bold text-slate-100">失业期三区防爆雷资金分层计算器</h2>
            <p className="text-xs text-slate-400">仅按 CNY 计算生活备用金，不参与服务器 Paper 订单定仓</p>
          </div>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="space-y-3 bg-slate-900/60 p-4 rounded-xl border border-slate-800">
            <div>
              <label className="text-xs font-bold text-slate-300 block mb-1">当前总备用资金 (CNY)</label>
              <input
                type="number"
                value={householdFundsCNY}
                onChange={(e) => setHouseholdFundsCNY(Number(e.target.value))}
                className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 font-mono"
              />
            </div>
            <div>
              <label className="text-xs font-bold text-slate-300 block mb-1">每月基础生活开支 (房租/吃饭/水电)</label>
              <input
                type="number"
                value={monthlyExpenseCNY}
                onChange={(e) => setMonthlyExpenseCNY(Number(e.target.value))}
                className="w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100 font-mono"
              />
            </div>
          </div>

          <div className="space-y-2 text-xs">
            <div className="rounded-xl border border-rose-500/30 bg-rose-950/30 p-3">
              <div className="flex justify-between font-bold text-rose-300">
                <span>🛡️ 救命资金防爆线 (8 个月生活保障)</span>
                <span className="font-mono text-sm">CNY {emergencyReserveCNY.toLocaleString()}</span>
              </div>
              <p className="text-[11px] text-rose-200/70 mt-1">这部分钱必须放在余额宝/货币基金/活期中，绝对禁止进入股市或买任何高风险资产！</p>
            </div>

            <div className="rounded-xl border border-emerald-500/30 bg-emerald-950/30 p-3">
              <div className="flex justify-between font-bold text-emerald-300">
                <span>🟢 CNY 生活账本可用参考</span>
                <span className="font-mono text-sm">CNY {householdInvestableCNY.toLocaleString()}</span>
              </div>
              <div className="flex justify-between text-[11px] text-emerald-200/80 mt-1">
                <span>单笔交易最大允许损失上限 (1.5% 铁律):</span>
                <span className="font-mono font-bold text-emerald-400">CNY {householdSingleRiskCNY.toFixed(0)}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

    </div>
  );
};
