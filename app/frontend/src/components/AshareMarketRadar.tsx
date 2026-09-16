import React, { useState, useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  TrendingUp,
  Activity,
  Flame,
  Globe,
  Sparkles,
  Zap,
  ChevronRight,
  ShieldAlert,
  Clock,
  Layers,
  ArrowRight,
  Compass,
  Briefcase,
  Target,
  Calendar,
  AlertTriangle,
  LayoutGrid,
  Maximize2,
  RefreshCw,
  Search,
  DollarSign,
  Cpu,
  Monitor,
  ExternalLink,
  Code2,
  Sliders,
  Check,
  Grid,
} from 'lucide-react';
import { get as apiGet } from '../utils/api';
import { FearGreedGauge } from './FearGreedGauge';
import { TradingViewAdvancedChart } from './TradingViewAdvancedChart';
import { PineScriptModal } from './PineScriptModal';
import { DataStatus } from './common';

interface MarketIndexItem {
  symbol: string;
  name: string;
  price: number;
  change: number;
  changePct: number;
  turnoverB: number;
  market: 'cn' | 'us' | 'hk' | 'global';
  role?: string;
  available?: boolean;
  source?: string;
  error?: string;
}

interface USOvernightMappingItem {
  id: string;
  usTheme: string;
  usMove: string;
  usCatalyst: string;
  ashareSector: string;
  impactLevel: 'HIGH' | 'MEDIUM' | 'EMERGING';
  actionAdvice: string;
  beneficiaryStocks: {
    symbol: string;
    name: string;
    role: string;
    price: number;
    pct: number;
  }[];
}

interface ForwardLayoutEventItem {
  id: string;
  tier: 'IMMEDIATE' | 'EARLY' | 'PREPARE';
  tierTitle: string;
  timeHorizon: string;
  eventName: string;
  eventDate: string;
  sector: string;
  logic: string;
  keyTickers: string[];
  riskNote: string;
}

interface ComprehensiveMarketData {
  updatedAt: string;
  cnIndices: MarketIndexItem[];
  usIndices: MarketIndexItem[];
  globalIndices?: MarketIndexItem[];
  sentiment: {
    fearGreedScore: number;
    fearGreedLabel: string;
    fearGreedLevel: string;
    vix: number;
    vixChangePct: number;
    upCount: number;
    downCount: number;
    flatCount: number;
    limitUpCount: number;
    limitDownCount: number;
    totalTurnoverB: number;
    northboundFlowB: number;
    turnoverStatus: string;
    sentimentSummary: string;
  };
  usOvernightMapping: USOvernightMappingItem[];
  forwardLayoutPlan: ForwardLayoutEventItem[];
}

export type CockpitTabType = 'ALL' | 'OVERVIEW' | 'US_MAPPING' | 'FORWARD_PLAN' | 'TRADINGVIEW_4SPLIT';

const TV_PRESETS = [
  {
    name: '👑 科技七巨头 M7',
    symbols: ['NVDA', 'TSLA', 'AAPL', 'MSFT'],
  },
  {
    name: '🇨🇳 A 股算力核心龙头',
    symbols: ['300750', '300308', '688256', '601127'],
  },
  {
    name: '🌏 亚太与全球宏观联动',
    symbols: ['NVDA', '300750', 'JPY=X', '^N225'],
  },
  {
    name: '🔥 大盘指数与避险资产',
    symbols: ['SPY', 'QQQ', 'GLD', '^TNX'],
  },
];

export const AshareMarketRadar: React.FC = () => {
  const navigate = useNavigate();
  const [cockpitTab, setCockpitTab] = useState<CockpitTabType>('ALL');
  const [marketTab, setMarketTab] = useState<'CN' | 'US' | 'GLOBAL'>('CN');
  const [layoutTierTab, setLayoutTierTab] = useState<'ALL' | 'IMMEDIATE' | 'EARLY' | 'PREPARE'>('ALL');
  const [data, setData] = useState<ComprehensiveMarketData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const overviewInFlight = useRef(false);
  const [isRefreshing, setIsRefreshing] = useState(false);

  // TradingView 4-Split & Indicator Injection State
  const [isPineModalOpen, setIsPineModalOpen] = useState(false);
  const [tvLayout, setTvLayout] = useState<'2x2' | '2x1' | '1x1'>('2x2');
  const [tvSlots, setTvSlots] = useState<{ symbol: string; interval: '1' | '5' | '15' | '60' | 'D' | 'W' }[]>([
    { symbol: 'NVDA', interval: '5' },
    { symbol: '300750', interval: 'D' },
    { symbol: 'JPY=X', interval: '60' },
    { symbol: '^N225', interval: '15' },
  ]);
  const [editingSlotIdx, setEditingSlotIdx] = useState<number | null>(null);
  const [slotSymbolInput, setSlotSymbolInput] = useState('');

  const fetchOverview = async (showRefreshEffect = false) => {
    if (overviewInFlight.current) return;
    overviewInFlight.current = true;
    if (showRefreshEffect) setIsRefreshing(true);
    try {
      const res = await apiGet<ComprehensiveMarketData>('/api/market/overview');
      if (res) {
        setData(res);
        setError(null);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '行情总览暂不可用');
      setData(null);
    } finally {
      setLoading(false);
      if (showRefreshEffect) setTimeout(() => setIsRefreshing(false), 500);
      overviewInFlight.current = false;
    }
  };

  useEffect(() => {
    void fetchOverview();
    const interval = setInterval(() => {
      if (!document.hidden) {
        void fetchOverview();
      }
    }, 8000);
    return () => clearInterval(interval);
  }, []);

  const applyTvPreset = (symbols: string[]) => {
    setTvSlots([
      { symbol: symbols[0] || 'NVDA', interval: '5' },
      { symbol: symbols[1] || '300750', interval: 'D' },
      { symbol: symbols[2] || 'JPY=X', interval: '60' },
      { symbol: symbols[3] || '^N225', interval: '15' },
    ]);
  };

  const updateSlotSymbol = (idx: number, newSym: string) => {
    if (!newSym.trim()) return;
    setTvSlots((prev) => {
      const next = [...prev];
      if (next[idx]) {
        next[idx] = { ...next[idx], symbol: newSym.trim().toUpperCase() };
      }
      return next;
    });
    setEditingSlotIdx(null);
    setSlotSymbolInput('');
  };

  const updateSlotInterval = (idx: number, newInterval: '1' | '5' | '15' | '60' | 'D' | 'W') => {
    setTvSlots((prev) => {
      const next = [...prev];
      if (next[idx]) {
        next[idx] = { ...next[idx], interval: newInterval };
      }
      return next;
    });
  };

  if (loading && !data) {
    return (
      <div className="flex min-h-[220px] items-center justify-center rounded-2xl border border-line bg-surface p-6 text-muted">
        <Activity className="h-6 w-6 animate-spin text-accent" />
        <span className="ml-2 text-xs">正在载入 A 股大盘看板、情绪温度计与美股隔夜映射...</span>
      </div>
    );
  }

  const indices =
    marketTab === 'CN'
      ? data?.cnIndices || []
      : marketTab === 'US'
      ? data?.usIndices || []
      : data?.globalIndices || [];

  const sentiment = data?.sentiment || {
    fearGreedScore: 0, fearGreedLabel: '暂无可靠数据', fearGreedLevel: 'neutral', vix: 0, vixChangePct: 0,
    upCount: 0, downCount: 0, flatCount: 0, limitUpCount: 0, limitDownCount: 0, totalTurnoverB: 0, northboundFlowB: 0,
    turnoverStatus: '暂无可靠数据', sentimentSummary: '实时统计尚未返回，暂不生成情绪结论。',
  };

  const filteredLayoutPlans = (data?.forwardLayoutPlan || []).filter((item) => {
    if (layoutTierTab === 'ALL') return true;
    return item.tier === layoutTierTab;
  });

  if (error && !data) {
    return (
      <div className="rounded-2xl border border-down/40 bg-surface p-6">
        <div className="flex items-center gap-2 text-sm font-semibold text-down">
          <AlertTriangle className="h-4 w-4" />
          实时大盘暂不可用
        </div>
        <p className="mt-2 text-xs leading-relaxed text-muted">{error}。未显示旧价格或推断性数字。</p>
        <DataStatus state="unavailable" message="外部行情源未返回可靠数据" onRetry={() => void fetchOverview(true)} />
      </div>
    );
  }

  const visibleTvSlots = tvLayout === '1x1' ? [tvSlots[0]] : tvLayout === '2x1' ? tvSlots.slice(0, 2) : tvSlots;

  return (
    <div className="space-y-4">
      {/* 🧭 中枢视图控制器 (Cockpit View Controller Bar) */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between rounded-2xl border border-line bg-surface-2/80 p-2 text-xs backdrop-blur-md shadow-sm">
        <div className="flex items-center gap-1.5 font-semibold overflow-x-auto w-full sm:w-auto pb-1 sm:pb-0 [-webkit-overflow-scrolling:touch] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
          <button
            type="button"
            onClick={() => setCockpitTab('ALL')}
            className={`shrink-0 flex items-center gap-1.5 rounded-xl px-3 py-1.5 transition text-[11px] sm:text-xs ${
              cockpitTab === 'ALL'
                ? 'bg-accent text-black font-bold shadow'
                : 'text-muted hover:bg-surface hover:text-ink'
            }`}
          >
            <LayoutGrid size={13} /> 全部综合战术看板
          </button>
          <button
            type="button"
            onClick={() => setCockpitTab('OVERVIEW')}
            className={`shrink-0 flex items-center gap-1.5 rounded-xl px-3 py-1.5 transition text-[11px] sm:text-xs ${
              cockpitTab === 'OVERVIEW'
                ? 'bg-accent text-black font-bold shadow'
                : 'text-muted hover:bg-surface hover:text-ink'
            }`}
          >
            <Compass size={13} /> 核心大盘与情绪仪表盘
          </button>
          <button
            type="button"
            onClick={() => setCockpitTab('US_MAPPING')}
            className={`shrink-0 flex items-center gap-1.5 rounded-xl px-3 py-1.5 transition text-[11px] sm:text-xs ${
              cockpitTab === 'US_MAPPING'
                ? 'bg-accent text-black font-bold shadow'
                : 'text-muted hover:bg-surface hover:text-ink'
            }`}
          >
            <Globe size={13} /> 美股隔夜映射 ➔ A股掘金
          </button>
          <button
            type="button"
            onClick={() => setCockpitTab('FORWARD_PLAN')}
            className={`shrink-0 flex items-center gap-1.5 rounded-xl px-3 py-1.5 transition text-[11px] sm:text-xs ${
              cockpitTab === 'FORWARD_PLAN'
                ? 'bg-accent text-black font-bold shadow'
                : 'text-muted hover:bg-surface hover:text-ink'
            }`}
          >
            <Calendar size={13} /> 大事催化与三级前瞻布局
          </button>
          <button
            type="button"
            onClick={() => setCockpitTab('TRADINGVIEW_4SPLIT')}
            className={`shrink-0 flex items-center gap-1.5 rounded-xl px-3 py-1.5 transition text-[11px] sm:text-xs ${
              cockpitTab === 'TRADINGVIEW_4SPLIT'
                ? 'bg-emerald-500 text-black font-bold shadow'
                : 'text-emerald-400 hover:bg-emerald-500/10'
            }`}
          >
            <Monitor size={13} /> 4分屏 TradingView 盯盘 & 指标
          </button>
        </div>

        <div className="flex items-center justify-between sm:justify-end gap-2 shrink-0">
          <button
            type="button"
            onClick={() => void fetchOverview(true)}
            className={`flex items-center gap-1 rounded-lg border border-line bg-surface px-2.5 py-1 text-[11px] text-muted hover:text-ink transition ${
              isRefreshing ? 'opacity-50' : ''
            }`}
            title="手动强制刷新"
          >
            <RefreshCw size={12} className={isRefreshing ? 'animate-spin text-accent' : ''} />
            <span>刷新</span>
          </button>
          <DataStatus
            state={error ? 'unavailable' : data ? 'live' : 'unavailable'}
            dataTime={data?.updatedAt}
            source={data ? 'market-overview-live' : undefined}
            message={error || undefined}
            compact
            onRetry={() => void fetchOverview(true)}
          />
        </div>
      </div>

      {/* 1. 显著大盘核心指数与情绪仪表盘 (Prominent Multi-Market Indices + Fear/Greed Gauge) */}
      {(cockpitTab === 'ALL' || cockpitTab === 'OVERVIEW') && (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
          {/* Left 2 Cols: 大盘指数切换与实时看板 */}
          <div className="lg:col-span-2 flex flex-col justify-between rounded-2xl border border-line bg-gradient-to-b from-[#0e1322] to-[#0a0d16] p-3.5 sm:p-5 shadow-xl">
            <div>
              {/* Market Switcher Header */}
              <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between border-b border-slate-800 pb-3.5">
                <div className="flex items-center gap-2.5">
                  <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-accent/15 text-accent border border-accent/30 shrink-0">
                    <Compass size={18} />
                  </div>
                  <div>
                      <h3 className="text-sm sm:text-base font-bold text-ink flex items-center gap-2">
                        全球核心大盘全景
                      <span className={`rounded-full px-2 py-0.5 text-[10px] sm:text-[11px] font-semibold border ${
                        data && !error ? 'bg-up/15 text-up border-up/25' : 'bg-down/15 text-down border-down/25'
                      }`}>
                        {data && !error ? '数据源已返回' : error ? '数据源不可用' : '等待数据'}
                      </span>
                    </h3>
                    <p className="text-[11px] sm:text-xs text-muted">
                      {marketTab === 'CN'
                        ? 'A 股三大指数、科创/北证与恒生指数全景量价异动'
                        : marketTab === 'US'
                        ? '美股纳指100、标普500、纳指综合、道琼斯与 VIX 恐慌指数'
                        : '影响美股的亚太与全球宏观枢纽：日经225、美元/日元套息、韩股存储、恒生科技与美债'}
                    </p>
                  </div>
                </div>

                {/* Triple Tab Buttons (Horizontal Scroll on Mobile) */}
                <div className="flex items-center rounded-xl bg-surface-2 p-1 border border-line text-xs font-semibold gap-1 overflow-x-auto max-w-full [-webkit-overflow-scrolling:touch] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                  <button
                    type="button"
                    onClick={() => setMarketTab('CN')}
                    className={`shrink-0 rounded-lg px-2.5 sm:px-3 py-1.5 transition text-[11px] sm:text-xs ${
                      marketTab === 'CN'
                        ? 'bg-red-500/25 text-red-300 border border-red-500/40 shadow-sm font-bold'
                        : 'text-muted hover:text-ink'
                    }`}
                  >
                    🇨🇳 A 股核心大盘
                  </button>
                  <button
                    type="button"
                    onClick={() => setMarketTab('US')}
                    className={`shrink-0 rounded-lg px-2.5 sm:px-3 py-1.5 transition text-[11px] sm:text-xs ${
                      marketTab === 'US'
                        ? 'bg-blue-500/25 text-blue-300 border border-blue-500/40 shadow-sm font-bold'
                        : 'text-muted hover:text-ink'
                    }`}
                  >
                    🇺🇸 美股核心大盘
                  </button>
                  <button
                    type="button"
                    onClick={() => setMarketTab('GLOBAL')}
                    className={`shrink-0 rounded-lg px-2.5 sm:px-3 py-1.5 transition text-[11px] sm:text-xs ${
                      marketTab === 'GLOBAL'
                        ? 'bg-amber-500/25 text-amber-300 border border-amber-500/40 shadow-sm font-bold'
                        : 'text-muted hover:text-ink'
                    }`}
                  >
                    🌏 亚太与全球宏观 (日/韩/港/日元)
                  </button>
                </div>
              </div>

              {/* Indices Large Prominent Cards Grid */}
              <div className="mt-3.5 grid grid-cols-2 gap-2 sm:gap-3 sm:grid-cols-3">
                {indices.map((idx) => {
                  const available = idx.available !== false && idx.price > 0;
                  const isUp = idx.changePct >= 0;
                  return (
                    <div
                      key={idx.symbol}
                      className={`group relative flex flex-col justify-between rounded-xl border p-3.5 transition hover:brightness-110 shadow-sm ${
                        !available ? 'border-line bg-surface-2' : isUp ? 'border-emerald-500/30 bg-emerald-950/15' : 'border-rose-500/30 bg-rose-950/15'
                      }`}
                    >
                      <div className="flex items-center justify-between">
                        <span className="font-bold text-[13px] text-ink truncate mr-1">{idx.name}</span>
                        <span className="font-mono text-[10px] text-faint shrink-0">{idx.symbol}</span>
                      </div>

                      <div className="mt-2 flex items-baseline justify-between">
                        <div className="font-mono text-xl font-black tracking-tight text-ink">
                          {available
                            ? idx.price.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })
                            : '暂无数据'}
                        </div>
                        <div
                          className={`font-mono text-xs font-bold ${
                            !available ? 'text-faint' : isUp ? 'text-emerald-400' : 'text-rose-400'
                          }`}
                        >
                          {available ? `${isUp ? '+' : ''}${idx.changePct.toFixed(2)}%` : '等待行情'}
                        </div>
                      </div>

                      <div className="mt-2 flex items-center justify-between border-t border-slate-800/60 pt-1.5 text-[10px] text-muted">
                        <span>
                          {available ? `${isUp ? '▲' : '▼'} ${Math.abs(idx.change).toFixed(2)}${idx.symbol === 'TNX' ? ' 个百分点' : ' 点'}` : '数据源未返回'}
                        </span>
                        {idx.source ? (
                          <span className="truncate text-slate-400 text-[10px] font-medium">来源: {idx.source}</span>
                        ) : idx.role ? (
                          <span className="truncate text-slate-400 text-[10px] font-medium">{idx.role}</span>
                        ) : idx.turnoverB > 0 ? (
                          <span className="font-mono text-faint">
                            成交 {(idx.turnoverB).toFixed(0)} 亿
                          </span>
                        ) : null}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>

            {/* Quick Market Status Footer */}
            <div className="mt-4 flex flex-wrap items-center justify-between rounded-xl bg-surface-2/60 p-2.5 border border-line text-xs">
              <div className="flex items-center gap-2 text-muted">
                <span className="h-2 w-2 rounded-full bg-emerald-400 animate-pulse" />
                <span>
                  {marketTab === 'CN' ? 'A 股状态: ' : marketTab === 'US' ? '美股状态: ' : '亚太宏观: '}
                  <strong className="text-ink">
                    {sentiment.turnoverStatus || '等待可靠数据源状态'}
                  </strong>
                </span>
              </div>
              <div className="flex items-center gap-3 font-mono text-[11px]">
                <span className="text-muted">
                  总成交: <strong className="text-amber-300">{(sentiment.totalTurnoverB / 10000).toFixed(2)} 万亿</strong>
                </span>
                <span className="text-muted">
                  外资流入: <strong className={sentiment.northboundFlowB >= 0 ? 'text-emerald-400' : 'text-rose-400'}>{sentiment.northboundFlowB > 0 ? '+' : ''}{sentiment.northboundFlowB} 亿</strong>
                </span>
              </div>
            </div>
          </div>

          {/* Right 1 Col: 恐慌与贪婪情绪仪表盘 (Fear & Greed Semicircle Gauge) */}
          <FearGreedGauge
            score={sentiment.fearGreedScore}
            label={sentiment.fearGreedLabel}
            vix={sentiment.vix}
            vixChangePct={sentiment.vixChangePct}
            upCount={sentiment.upCount}
            downCount={sentiment.downCount}
            limitUpCount={sentiment.limitUpCount}
            limitDownCount={sentiment.limitDownCount}
            totalTurnoverB={sentiment.totalTurnoverB}
            northboundFlowB={sentiment.northboundFlowB}
            turnoverStatus={sentiment.turnoverStatus}
            summary={sentiment.sentimentSummary}
          />
        </div>
      )}

      {/* 2. TradingView embedded multi-chart and indicator reference. */}
      {(cockpitTab === 'ALL' || cockpitTab === 'TRADINGVIEW_4SPLIT') && (
        <div className="rounded-2xl border border-emerald-500/25 bg-gradient-to-b from-[#0e1622] to-[#0a0d16] p-5 shadow-xl">
          {/* Header & Controls */}
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between border-b border-slate-800 pb-3.5">
            <div className="flex items-center gap-3">
              <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
                <Monitor size={20} className="animate-pulse" />
              </div>
              <div>
                <h3 className="text-base font-bold text-slate-100 flex items-center gap-2">
                  TradingView 4 分屏图表 & 指标参考
                  <span className="rounded-full bg-emerald-500/20 px-2 py-0.5 text-xs text-emerald-300 border border-emerald-500/30">
                    1~4 图布局
                  </span>
                </h3>
                <p className="text-xs text-slate-400">
                  第三方图表独立加载；来源、延迟、交易时段和指标口径以图内标记为准，不纳入本系统实时数据门禁。
                </p>
              </div>
            </div>

            {/* Layout Toggles & Indicator Injection */}
            <div className="flex flex-wrap items-center gap-2">
              <div className="flex rounded-lg bg-surface-2 p-1 border border-line text-xs font-semibold">
                <button
                  type="button"
                  onClick={() => setTvLayout('1x1')}
                  className={`rounded px-2.5 py-1 transition ${tvLayout === '1x1' ? 'bg-accent text-black font-bold shadow' : 'text-muted hover:text-ink'}`}
                >
                  单图 1x1
                </button>
                <button
                  type="button"
                  onClick={() => setTvLayout('2x1')}
                  className={`rounded px-2.5 py-1 transition ${tvLayout === '2x1' ? 'bg-accent text-black font-bold shadow' : 'text-muted hover:text-ink'}`}
                >
                  双图 2x1
                </button>
                <button
                  type="button"
                  onClick={() => setTvLayout('2x2')}
                  className={`rounded px-2.5 py-1 transition ${tvLayout === '2x2' ? 'bg-accent text-black font-bold shadow' : 'text-muted hover:text-ink'}`}
                >
                  四分屏 2x2
                </button>
              </div>

              <button
                type="button"
                onClick={() => setIsPineModalOpen(true)}
                className="flex items-center gap-1.5 rounded-lg border border-emerald-500/40 bg-emerald-500/15 px-3 py-1.5 text-xs font-bold text-emerald-300 hover:bg-emerald-500/25 transition shadow-sm"
              >
                <Code2 size={13} /> 注入 6-in-1 指标脚本
              </button>

              <a
                href="https://www.tradingview.com/chart/"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-1 rounded-lg border border-slate-700 bg-slate-800/80 px-2.5 py-1.5 text-xs font-medium text-slate-300 hover:bg-slate-700 hover:text-white transition"
                title="直连 TradingView 官方独立图表"
              >
                <ExternalLink size={13} /> TradingView 官方 ↗
              </a>
            </div>
          </div>

          {/* Quick Presets Bar */}
          <div className="mt-3 flex flex-wrap items-center gap-2 border-b border-slate-800/80 pb-3 text-xs">
            <span className="text-slate-400 font-medium">一键切换预设组:</span>
            {TV_PRESETS.map((preset) => (
              <button
                key={preset.name}
                type="button"
                onClick={() => applyTvPreset(preset.symbols)}
                className="rounded-lg border border-slate-800 bg-slate-900/90 px-2.5 py-1 text-slate-300 hover:border-emerald-500/40 hover:text-emerald-300 transition"
              >
                {preset.name}
              </button>
            ))}
          </div>

          {/* 4-Split Grid Charts */}
          <div
            className={`mt-4 grid gap-3.5 ${
              tvLayout === '1x1' ? 'grid-cols-1' : tvLayout === '2x1' ? 'grid-cols-1 lg:grid-cols-2' : 'grid-cols-1 lg:grid-cols-2'
            }`}
          >
            {visibleTvSlots.map((slot, idx) => (
              <div
                key={`${slot.symbol}-${slot.interval}-${idx}`}
                className="relative flex flex-col rounded-xl border border-slate-800 bg-slate-950 p-2 shadow-inner"
              >
                {/* Slot Control Header */}
                <div className="flex items-center justify-between border-b border-slate-800/80 pb-2 mb-2 px-1">
                  <div className="flex items-center gap-2">
                    {editingSlotIdx === idx ? (
                      <div className="flex items-center gap-1.5">
                        <input
                          type="text"
                          value={slotSymbolInput}
                          onChange={(e) => setSlotSymbolInput(e.target.value)}
                          placeholder="代码如 NVDA / 300750"
                          className="w-28 rounded bg-slate-900 border border-emerald-500 px-2 py-0.5 text-xs text-white font-mono uppercase"
                          autoFocus
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') updateSlotSymbol(idx, slotSymbolInput);
                            if (e.key === 'Escape') setEditingSlotIdx(null);
                          }}
                        />
                        <button
                          onClick={() => updateSlotSymbol(idx, slotSymbolInput)}
                          className="rounded bg-emerald-500 px-2 py-0.5 text-xs font-bold text-black"
                        >
                          确定
                        </button>
                      </div>
                    ) : (
                      <button
                        onClick={() => {
                          setEditingSlotIdx(idx);
                          setSlotSymbolInput(slot.symbol);
                        }}
                        className="group flex items-center gap-1.5 font-mono text-sm font-black text-slate-100 hover:text-emerald-400 transition"
                        title="点击换股票"
                      >
                        <span>{slot.symbol}</span>
                        <span className="text-[10px] text-slate-500 group-hover:text-emerald-400">(换标的 ✎)</span>
                      </button>
                    )}
                  </div>

                  {/* Interval Selectors */}
                  <div className="flex items-center gap-1 text-[11px] font-mono">
                    {(['1', '5', '15', '60', 'D', 'W'] as const).map((itv) => (
                      <button
                        key={itv}
                        onClick={() => updateSlotInterval(idx, itv)}
                        className={`rounded px-1.5 py-0.5 transition ${
                          slot.interval === itv
                            ? 'bg-emerald-500/25 text-emerald-300 font-bold border border-emerald-500/40'
                            : 'text-slate-400 hover:text-slate-200'
                        }`}
                      >
                        {itv === 'D' ? '日' : itv === 'W' ? '周' : `${itv}m`}
                      </button>
                    ))}

                    <a
                      href={`https://www.tradingview.com/chart/?symbol=${encodeURIComponent(slot.symbol)}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="ml-1 text-slate-500 hover:text-emerald-400 transition"
                      title="在 TradingView 官方独立打开此股票"
                    >
                      <ExternalLink size={12} />
                    </a>
                  </div>
                </div>

                {/* Embedded Real-Time Chart */}
                <div className="h-[360px] w-full overflow-hidden rounded-lg">
                  <TradingViewAdvancedChart
                    symbol={slot.symbol}
                    defaultInterval={slot.interval}
                    height={360}
                  />
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 3. 美股隔夜映射 ➔ A股联动掘金矩阵 (US Overnight Spillover -> A-Share Beneficiary Mapping) */}
      {(cockpitTab === 'ALL' || cockpitTab === 'US_MAPPING') && (
        <div className="rounded-2xl border border-indigo-500/25 bg-gradient-to-b from-[#0f1426] to-[#0a0d16] p-5 shadow-xl">
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between border-b border-slate-800 pb-3.5">
            <div className="flex items-center gap-3">
              <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-indigo-500/15 text-indigo-400 border border-indigo-500/30">
                <Globe size={20} className="animate-pulse" />
              </div>
              <div>
                <h3 className="text-base font-bold text-slate-100 flex items-center gap-2">
                  美股隔夜映射 ➔ A 股联动掘金矩阵
                  <span className="rounded-full bg-indigo-500/20 px-2 py-0.5 text-xs text-indigo-300 border border-indigo-500/30">
                    涵盖日韩港与美股宏观驱动
                  </span>
                </h3>
                <p className="text-xs text-slate-400">
                  追踪美股科技狂飙、日元套息平仓警戒、韩国 SK 海力士/三星存储爆单与港股恒生科技，实时穿透至 A 股受惠产业链
                </p>
              </div>
            </div>
          </div>

          {/* Mapping Matrix Cards */}
          <div className="mt-4 grid grid-cols-1 gap-3.5 lg:grid-cols-2">
            {(data?.usOvernightMapping || []).map((item) => (
              <div
                key={item.id}
                className="flex flex-col justify-between rounded-xl border border-slate-800/90 bg-slate-900/60 p-4 hover:border-indigo-500/40 hover:bg-slate-900/90 transition shadow-sm"
              >
                <div>
                  {/* Header: US Theme vs A-Share Sector */}
                  <div className="flex items-start justify-between gap-2 border-b border-slate-800/80 pb-2.5">
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="rounded bg-blue-500/20 px-2 py-0.5 text-[11px] font-bold text-blue-300 border border-blue-500/30">
                          🌍 全球宏观与美股动向
                        </span>
                        <h4 className="text-sm font-bold text-slate-100">{item.usTheme}</h4>
                      </div>
                      <div className="mt-1 font-mono text-xs font-semibold text-emerald-400">
                        {item.usMove}
                      </div>
                    </div>
                    <span className="rounded-md bg-indigo-500/20 px-2.5 py-1 text-xs font-bold text-indigo-300 border border-indigo-500/30">
                      🇨🇳 {item.ashareSector}
                    </span>
                  </div>

                  {/* Catalyst Description */}
                  <p className="mt-2.5 text-xs text-slate-300 leading-relaxed bg-slate-950/40 p-2.5 rounded-lg border border-slate-800/60">
                    <span className="font-semibold text-amber-400">⚡ 催化驱动: </span>
                    {item.usCatalyst}
                  </p>

                  {/* Action advice */}
                  <div className="mt-2 text-xs text-indigo-200">
                    <span className="font-semibold text-indigo-400">🎯 操作策略: </span>
                    {item.actionAdvice}
                  </div>
                </div>

                {/* Beneficiary A-Share Stocks Chips */}
                <div className="mt-3.5 border-t border-slate-800/80 pt-3">
                  <div className="text-[11px] font-semibold text-slate-400 mb-2 flex items-center gap-1">
                    <Sparkles size={13} className="text-amber-400" /> A 股相关标的 (点击查看详情):
                  </div>
                  <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                    {item.beneficiaryStocks.map((stock) => (
                      <button
                        key={stock.symbol}
                        onClick={() => navigate(`/stock/${stock.symbol}`)}
                        type="button"
                        className="group flex flex-col justify-between rounded-lg border border-slate-800 bg-slate-950/70 p-2 text-left hover:border-amber-500/40 hover:bg-slate-900 transition"
                      >
                        <div className="flex items-center justify-between">
                          <span className="font-mono text-xs font-bold text-slate-200 group-hover:text-amber-400 transition">
                            {stock.symbol}
                          </span>
                          <span className={`font-mono text-xs font-bold ${stock.pct >= 0 ? 'text-emerald-400' : 'text-rose-400'}`}>
                            {stock.pct >= 0 ? '+' : ''}{stock.pct.toFixed(2)}%
                          </span>
                        </div>
                        <div className="mt-1 flex items-center justify-between">
                          <span className="text-xs font-semibold text-slate-300 truncate">{stock.name}</span>
                          <span className="font-mono text-[11px] text-slate-400">¥{stock.price.toFixed(2)}</span>
                        </div>
                        <div className="mt-1 truncate text-[10px] text-slate-400">{stock.role}</div>
                      </button>
                    ))}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 4. 大事催化与三级前瞻布局作战图 (3-Tier Forward Layout: 现在布局 / 提前布局 / 准备布局) */}
      {(cockpitTab === 'ALL' || cockpitTab === 'FORWARD_PLAN') && (
        <div className="rounded-2xl border border-amber-500/25 bg-gradient-to-b from-[#14121a] to-[#0a0d16] p-3.5 sm:p-5 shadow-xl">
          {/* Header & 3-Tier Tabs */}
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between border-b border-slate-800 pb-3.5">
            <div className="flex items-center gap-2.5 sm:gap-3">
              <div className="flex h-8 w-8 sm:h-9 sm:w-9 items-center justify-center rounded-xl bg-amber-500/15 text-amber-400 border border-amber-500/30 shrink-0">
                <Calendar size={18} className="animate-pulse" />
              </div>
              <div>
                <h3 className="text-sm sm:text-base font-bold text-slate-100 flex items-center gap-2">
                  大事催化与三级前瞻布局作战图
                  <span className="rounded-full bg-amber-500/20 px-2 py-0.5 text-[10px] sm:text-xs text-amber-300 border border-amber-500/30">
                    时间窗口梯次
                  </span>
                </h3>
                <p className="text-[11px] sm:text-xs text-slate-400">
                  按「现在布局 (0-3天)」、「提前布局 (1-3周)」、「准备布局 (1-3月)」三个作战周期梯次配置
                </p>
              </div>
            </div>

            {/* Tier Switcher (Horizontal Scroll on Mobile) */}
            <div className="flex items-center rounded-xl bg-slate-900/90 p-1 border border-slate-800 text-xs font-semibold overflow-x-auto max-w-full [-webkit-overflow-scrolling:touch] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
              <button
                type="button"
                onClick={() => setLayoutTierTab('ALL')}
                className={`shrink-0 rounded-lg px-2.5 sm:px-3 py-1.5 transition text-[11px] sm:text-xs ${
                  layoutTierTab === 'ALL'
                    ? 'bg-amber-500/25 text-amber-300 border border-amber-500/40 shadow-sm font-bold'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                全部计划
              </button>
              <button
                type="button"
                onClick={() => setLayoutTierTab('IMMEDIATE')}
                className={`shrink-0 rounded-lg px-2.5 sm:px-3 py-1.5 transition text-[11px] sm:text-xs ${
                  layoutTierTab === 'IMMEDIATE'
                    ? 'bg-rose-500/25 text-rose-300 border border-rose-500/40 shadow-sm font-bold'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                🔴 现在 (0~3天)
              </button>
              <button
                type="button"
                onClick={() => setLayoutTierTab('EARLY')}
                className={`shrink-0 rounded-lg px-2.5 sm:px-3 py-1.5 transition text-[11px] sm:text-xs ${
                  layoutTierTab === 'EARLY'
                    ? 'bg-amber-500/25 text-amber-300 border border-amber-500/40 shadow-sm font-bold'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                🟡 提前 (1~3周)
              </button>
              <button
                type="button"
                onClick={() => setLayoutTierTab('PREPARE')}
                className={`shrink-0 rounded-lg px-2.5 sm:px-3 py-1.5 transition text-[11px] sm:text-xs ${
                  layoutTierTab === 'PREPARE'
                    ? 'bg-blue-500/25 text-blue-300 border border-blue-500/40 shadow-sm font-bold'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                🔵 准备 (1~3月)
              </button>
            </div>
          </div>

          {/* 3-Tier Forward Layout Cards */}
          <div className="mt-4 grid grid-cols-1 gap-3.5 md:grid-cols-2 lg:grid-cols-3">
            {filteredLayoutPlans.map((event) => {
              const isNow = event.tier === 'IMMEDIATE';
              const isEarly = event.tier === 'EARLY';

              const badgeBg = isNow
                ? 'bg-rose-500/20 text-rose-300 border-rose-500/30'
                : isEarly
                ? 'bg-amber-500/20 text-amber-300 border-amber-500/30'
                : 'bg-blue-500/20 text-blue-300 border-blue-500/30';

              return (
                <div
                  key={event.id}
                  className="flex flex-col justify-between rounded-xl border border-slate-800/90 bg-slate-900/60 p-4 hover:border-amber-500/40 hover:bg-slate-900/90 transition shadow-sm"
                >
                  <div>
                    {/* Top Badge & Time Horizon */}
                    <div className="flex items-center justify-between border-b border-slate-800/80 pb-2">
                      <span className={`rounded px-2 py-0.5 text-[11px] font-bold border ${badgeBg}`}>
                        {event.tierTitle}
                      </span>
                      <span className="font-mono text-xs text-amber-400 flex items-center gap-1">
                        <Clock size={12} /> {event.eventDate}
                      </span>
                    </div>

                    {/* Event Title & Sector */}
                    <h4 className="mt-2.5 text-sm font-bold text-slate-100 leading-snug">
                      {event.eventName}
                    </h4>
                    <div className="mt-1 inline-flex items-center gap-1 text-xs font-semibold text-emerald-400">
                      <Layers size={13} /> {event.sector}
                    </div>

                    {/* Logic */}
                    <p className="mt-2 text-xs text-slate-300 leading-relaxed bg-slate-950/50 p-2.5 rounded-lg border border-slate-800/60">
                      <span className="font-semibold text-amber-400">💡 催化逻辑: </span>
                      {event.logic}
                    </p>
                  </div>

                  {/* Tickers & Risk Note */}
                  <div className="mt-3 space-y-2 border-t border-slate-800/80 pt-2.5">
                    <div className="flex flex-wrap items-center gap-1.5">
                      <span className="text-[11px] text-slate-400">核心标的:</span>
                      {event.keyTickers.map((ticker) => {
                        const sym = ticker.split(' ')[0];
                        return (
                          <button
                            key={ticker}
                            type="button"
                            onClick={() => navigate(`/stock/${sym}`)}
                            className="rounded bg-slate-800 px-2 py-0.5 text-xs font-semibold text-slate-200 hover:bg-amber-500/20 hover:text-amber-300 border border-slate-700/60 hover:border-amber-500/30 transition"
                          >
                            {ticker}
                          </button>
                        );
                      })}
                    </div>

                    <div className="flex items-center gap-1 text-[11px] text-rose-300/80">
                      <AlertTriangle size={12} className="text-rose-400 shrink-0" />
                      <span className="truncate">{event.riskNote}</span>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Pine Script 6-in-1 Indicator Injection Modal */}
      <PineScriptModal
        isOpen={isPineModalOpen}
        onClose={() => setIsPineModalOpen(false)}
      />
    </div>
  );
};
