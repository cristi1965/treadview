import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { StockGodShell } from '../components/layout/StockGodShell';
import { Heatmap } from '../components/heatmap';
import { DataStatus, LoadingSpinner } from '../components/common';
import { HeatmapNode, HeatmapNodeWithPosition, HeatmapConfig } from '../types/heatmap';
import { prepareNodes } from '../utils/heatmap';
import { get, getWithMeta, type APIResponseMeta } from '../utils/api';
import { assessDataTrust } from '../utils/dataTrust';
import {
  loadPulseCompanies,
  invalidatePulseCache,
  PULSE_INDUSTRIES,
  PULSE_LAYERS,
  REGION_TABS,
  PulseCompany,
  inIndustry,
  lensValue,
  PulseRegion,
} from '../utils/pulseData';
import { useUiStore } from '../stores/uiStore';
import { useI18n } from '../i18n';
import { CnToolbox } from '../components/cn/CnToolbox';
import { UpcomingEvents } from '../components/calendar/UpcomingEvents';
import { MarketMovers } from '../components/calendar/MarketMovers';
import { SentimentPanel } from '../components/sentiment/SentimentPanel';
import { KeyWatchlistPanel } from '../components/KeyWatchlistPanel';
import { MarketAnomaliesRadar } from '../components/MarketAnomaliesRadar';
import { QDIIPremiumRadar } from '../components/QDIIPremiumRadar';
import { AshareMarketRadar } from '../components/AshareMarketRadar';
import { RefreshCw } from 'lucide-react';

const LENSES = ['过热度', '综合', '巴菲特', '段永平', 'Serenity', '德鲁肯米勒', '情绪资金面', '分歧'] as const;
const MODES = ['脉冲热力', '产业链', '板块'] as const;

const HEAT_FILTERS = [
  { id: 'all', label: '全部', min: 0, max: 100 },
  { id: 'hot', label: '过热 ≥85', min: 85, max: 100 },
  { id: 'warm', label: '偏热 70-85', min: 70, max: 85 },
  { id: 'fair', label: '合理 50-70', min: 50, max: 70 },
  { id: 'cool', label: '偏冷 <50', min: 0, max: 50 },
] as const;

const LENS_SUB: Record<string, string> = {
  过热度: '估值贵 + 离52周高点 + RSI/动量 · 越高越泡沫(不随当天涨跌跳)',
  综合: '已判读各方真实评分均值',
  巴菲特: '价值 · 护城河',
  段永平: '价值 · 商业模式',
  Serenity: 'alpha · 供应链瓶颈',
  德鲁肯米勒: 'alpha · 宏观流动性',
  情绪资金面: '盘口 · 资金流',
  分歧: '5 方评分极差 · 越大越撕裂(分歧即信号)',
};

const RAMP_MARKS = [
  { v: 0, label: '深价值', color: '#3F1A8C' },
  { v: 30, label: '偏冷', color: '#1E5BCC' },
  { v: 50, label: '中性', color: '#1A8E72' },
  { v: 70, label: '合理', color: '#A6A828' },
  { v: 85, label: '偏热', color: '#DC4914' },
  { v: 100, label: '过热警告', color: '#C4126B' },
];

function scoreTone(score: number): string {
  if (score >= 85) return '过热警告';
  if (score >= 70) return '偏热';
  if (score >= 50) return '合理';
  if (score >= 30) return '偏冷';
  return '深价值';
}

function pulseToNode(c: PulseCompany, score: number | null): HeatmapNode {
  return {
    symbol: c.ticker,
    name: c.name,
    price: c.livePrice || 0,
    changePercent: c.pct || 0,
    marketCap: (c.marketCapB || 0) * 1_000_000_000,
    sector: c.segment || c.layer,
    avgScore: score ?? 0,
    scores: c.scores,
    divergence: c.divergence,
    layer: c.layer.startsWith('L') ? `${c.layer} ${PULSE_LAYERS.find((l) => l.id === c.layer)?.name || ''}`.trim() : c.layer,
    country: c.region === 'CN' ? 'China' : c.region === 'US' ? 'United States' : c.region,
    venue: c.region === 'CN' ? 'cn' : 'us',
  };
}

const VerticalChip: React.FC<{ active?: boolean; children: React.ReactNode; onClick?: () => void }> = ({
  active,
  children,
  onClick,
}) => (
  <button
    onClick={onClick}
    className={`w-full rounded-md px-2.5 py-1.5 text-left text-xs transition ${
      active ? 'bg-surface-3 font-medium text-ink' : 'text-muted hover:bg-surface-2 hover:text-ink'
    }`}
  >
    {children}
  </button>
);

export const Home: React.FC = () => {
  const navigate = useNavigate();
  const language = useUiStore((s) => s.language);
  const { t } = useI18n();
  const [companies, setCompanies] = useState<PulseCompany[]>([]);
  const [macroTickers, setMacroTickers] = useState<[string, string, string][]>([]);
  const [macroError, setMacroError] = useState('');
  const [panelError, setPanelError] = useState('');
  const [pulseMeta, setPulseMeta] = useState<APIResponseMeta | null>(null);
  const [pulseHistorical, setPulseHistorical] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [mode, setMode] = useState<(typeof MODES)[number]>('脉冲热力');
  const [region, setRegion] = useState<'ALL' | PulseRegion>('US');
  const [heatFilter, setHeatFilter] = useState<(typeof HEAT_FILTERS)[number]['id']>('all');
  const [layerFocus, setLayerFocus] = useState<string | null>(null);
  const [lens, setLens] = useState<(typeof LENSES)[number]>('过热度');
  const [industry, setIndustry] = useState('AI');
  const [locateQuery, setLocateQuery] = useState('');
  const [refreshVersion, setRefreshVersion] = useState(0);
  const panelStocksRef = useRef<Record<string, { sc?: number[]; div?: number }>>({});

  const retryHomeData = useCallback(() => {
    invalidatePulseCache();
    setRefreshVersion((value) => value + 1);
  }, []);

  const config: HeatmapConfig = useMemo(
    () => ({
      width: Math.min(Math.max(window.innerWidth - 120, 320), 900),
      height: window.innerWidth < 640 ? 520 : 640,
      minRadius: 2.8,
      maxRadius: 28,
      forceStrength: -22,
      collisionPadding: 1.5,
      alphaDecay: 0.025,
    }),
    []
  );

  useEffect(() => {
    const refresh = () => retryHomeData();
    window.addEventListener('stockgod:home-data-refresh', refresh);
    return () => window.removeEventListener('stockgod:home-data-refresh', refresh);
  }, [retryHomeData]);

  useEffect(() => {
    let cancelled = false;
    panelStocksRef.current = {};
    setPanelError(language === 'en' ? 'Validating score evidence...' : '正在验证评分证据...');
    (async () => {
      try {
        const [macroResult, panelResult] = await Promise.allSettled([
          getWithMeta<{ series: any[] }>('/api/macro'),
          getWithMeta<{
            stocks?: Record<string, { sc?: number[]; div?: number }>;
            validation?: { status?: string; reasons?: string[] };
          }>('/api/panel-summary'),
        ]);
        if (cancelled) return;
        const macroResponse = macroResult.status === 'fulfilled' ? macroResult.value : null;
        const macroResp = macroResponse?.data;
        const macroTrust = assessDataTrust(macroResponse?.meta, Boolean(macroResp?.series?.length));
        const panelResponse = panelResult.status === 'fulfilled' ? panelResult.value : null;
        const panelValidated = panelResponse?.data?.validation?.status === 'validated';
        const panelUsable = Boolean(
          panelResponse &&
          panelValidated &&
          !panelResponse.meta.stale &&
          panelResponse.meta.source &&
          panelResponse.meta.dataTime &&
          panelResponse.meta.dataTime !== 'unknown'
        );
        const panel = panelUsable ? panelResponse?.data : null;
        panelStocksRef.current = (panel?.stocks || {}) as Record<string, { sc?: number[]; div?: number }>;
        const validationReason = panelResponse?.data?.validation?.reasons?.join('；');
        setPanelError(panelUsable
          ? ''
          : !panelValidated
            ? (validationReason || (language === 'en' ? 'Scores are unvalidated and hidden' : '评分尚未通过历史验证，已隐藏'))
            : (language === 'en' ? 'Score overview source is unavailable' : '评分概览来源暂不可用'));
        if (macroTrust.state === 'live' && macroResp?.series?.length) {
          setMacroTickers(
            macroResp.series.map((item: any) => [
              item.name,
              item.kind === 'rate' ? `${item.price.toFixed(2)}%` : Math.round(item.price).toLocaleString('en-US'),
              `${item.pct >= 0 ? '+' : ''}${item.pct.toFixed(2)}%`,
            ]) as [string, string, string][]
          );
          setMacroError('');
        } else {
          setMacroTickers([]);
          setMacroError(language === 'en'
            ? `Macro values hidden: ${macroTrust.reason}`
            : `宏观数值已隐藏：${macroTrust.reason}`);
        }
      } catch {
        if (!cancelled) {
          setMacroTickers([]);
          setMacroError(language === 'en' ? 'Macro snapshot is temporarily unavailable' : '宏观快照暂不可用');
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [language, refreshVersion]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        setLoading(true);
        setError(null);
        const pulseResult = await loadPulseCompanies(region);
        if (cancelled) return;
        const pulse = pulseResult.companies;
        setPulseMeta(pulseResult.meta);
        setPulseHistorical(pulseResult.dataMode === 'historical' || pulseResult.meta.stale);
        const panelStocks = panelStocksRef.current;
        const merged = pulse.map((c) => {
          const withoutOverview = { ...c, scores: undefined, avgScore: undefined, divergence: undefined };
          const row = panelStocks[c.ticker];
          if (!row?.sc || row.sc.length < 5) return withoutOverview;
          const scores = {
            buffett: Math.round(row.sc[0] ?? 0),
            duanyongping: Math.round(row.sc[1] ?? 0),
            serenity: Math.round(row.sc[2] ?? 0),
            druckenmiller: Math.round(row.sc[3] ?? 0),
            sentiment: Math.round(row.sc[4] ?? 0),
          };
          const avgScore = Math.round(row.sc.reduce((a, b) => a + b, 0) / row.sc.length);
          return { ...withoutOverview, scores, avgScore, divergence: row.div ?? 0 };
        });
        setCompanies(merged);
        setError(null);
      } catch {
        if (!cancelled) {
          setCompanies([]);
          setPulseMeta(null);
          setPulseHistorical(false);
          setError(language === 'en' ? 'Pulse snapshot is temporarily unavailable' : '脉冲快照暂不可用，当前区域无可靠数据');
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [language, region, refreshVersion]);

  const industryUniverse = useMemo(
    () => companies.filter((c) => inIndustry(c, industry)),
    [companies, industry]
  );

  const industryCounts = useMemo(() => {
    const map = new Map<string, number>();
    for (const ind of PULSE_INDUSTRIES) map.set(ind.id, 0);
    for (const c of companies) {
      if (region !== 'ALL' && c.region !== region) continue;
      for (const id of c.industries || []) {
        if (map.has(id)) map.set(id, (map.get(id) || 0) + 1);
      }
    }
    return map;
  }, [companies, region]);

  const filtered = useMemo(() => {
    const band = HEAT_FILTERS.find((f) => f.id === heatFilter) || HEAT_FILTERS[0];
    let list = industryUniverse.filter((c) => region === 'ALL' || c.region === region);
    if (layerFocus) list = list.filter((c) => c.layer === layerFocus);
    list = list.filter((c) => {
      const v = lensValue(c, lens);
      if (v == null) return heatFilter === 'all';
      return v >= band.min && v <= band.max;
    });
    const q = locateQuery.trim().toLowerCase();
    if (q) {
      list = list.filter(
        (c) => c.ticker.toLowerCase().includes(q) || (c.name || '').toLowerCase().includes(q)
      );
    }
    return [...list].sort((a, b) => (b.marketCapB || 0) - (a.marketCapB || 0)).slice(0, 1200);
  }, [industryUniverse, region, layerFocus, heatFilter, lens, locateQuery]);

  const nodes: HeatmapNodeWithPosition[] = useMemo(() => {
    if (mode !== '脉冲热力' || filtered.length === 0) return [];
    const raw = filtered.map((c) => {
      const v = lensValue(c, lens);
      // 过热度 always has heat; other lenses stay 0 when unscored → gray particle
      return pulseToNode(c, lens === '过热度' ? c.heat : v);
    });
    return prepareNodes(
      raw,
      config.width,
      config.height,
      config.minRadius,
      config.maxRadius,
      'score',
      (n) => n.avgScore,
      industry === 'AI' ? 'layer-scatter' : 'force'
    );
  }, [filtered, config, lens, mode, industry]);

  const metrics = useMemo(() => {
    const scores = filtered.map((c) => lensValue(c, lens)).filter((v): v is number => v != null);
    if (!scores.length) return { avg: 0, over: 0, total: 0, cheap: 0, scored: 0 };
    const avg = Math.round(scores.reduce((a, b) => a + b, 0) / scores.length);
    return {
      avg,
      over: scores.filter((s) => s >= (lens === '过热度' ? 85 : 70)).length,
      total: scores.length,
      cheap: scores.filter((s) => s < (lens === '过热度' ? 50 : 40)).length,
      scored: scores.length,
    };
  }, [filtered, lens]);

  const highScores = useMemo(
    () =>
      [...filtered]
        .map((c) => ({ c, v: lensValue(c, lens) }))
        .filter((x) => x.v != null)
        .sort((a, b) => (b.v as number) - (a.v as number))
        .slice(0, 8),
    [filtered, lens]
  );

  const lowScores = useMemo(
    () =>
      [...filtered]
        .map((c) => ({ c, v: lensValue(c, lens) }))
        .filter((x) => x.v != null && (x.v as number) > 0)
        .sort((a, b) => (a.v as number) - (b.v as number))
        .slice(0, 6),
    [filtered, lens]
  );

  const sectorBuckets = useMemo(() => {
    const map = new Map<string, number>();
    for (const c of companies.filter((x) => region === 'ALL' || x.region === region)) {
      const seg = c.segment || '其他';
      map.set(seg, (map.get(seg) || 0) + 1);
    }
    return [...map.entries()].sort((a, b) => b[1] - a[1]).slice(0, 24);
  }, [companies, region]);

  const activeIndustry = PULSE_INDUSTRIES.find((i) => i.id === industry) || PULSE_INDUSTRIES[0];
  const judged = companies.filter((c) => (c.avgScore || 0) > 0).length;
  const tone = metrics.scored > 0 ? scoreTone(metrics.avg) : '暂无可用信号';

  const rampCss =
    'linear-gradient(90deg,#3F1A8C 0%,#1E5BCC 30%,#1A8E72 50%,#A6A828 70%,#DC4914 85%,#C4126B 100%)';

  return (
    <StockGodShell title={language === 'en' ? 'Heatmap' : '热力图'} macroTickers={macroTickers}>
      <div className="space-y-3">
        {macroError && (
          <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 px-3 py-2">
            <DataStatus state="unavailable" label={language === 'en' ? 'Macro degraded' : '宏观数据降级'} message={macroError} compact />
          </div>
        )}
        {panelError && (
          <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 px-3 py-2">
            <DataStatus state="unavailable" label={language === 'en' ? 'Score overview unavailable' : '评分概览不可用'} message={panelError} compact />
          </div>
        )}
        <div className="grid grid-cols-3 gap-1 rounded-lg bg-surface-2 p-1 sm:flex sm:flex-wrap sm:items-center">
          {MODES.map((item) => (
            <button
              key={item}
              onClick={() => setMode(item)}
              className={`min-w-0 rounded-md px-2 py-1.5 text-xs font-semibold transition sm:px-4 sm:text-sm ${
                mode === item ? 'bg-accent text-black shadow' : 'text-muted hover:text-ink'
              }`}
            >
              {item === '脉冲热力' ? t('home.modePulse') : item === '产业链' ? t('home.modeChain') : t('home.modeSector')}
            </button>
          ))}
        </div>

        <header className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b border-line pb-3">
          <div className="mr-auto flex min-w-0 items-baseline gap-3">
            <h1 className="shrink-0 text-[22px] font-semibold tracking-tight text-ink">
              {activeIndustry.name} · {mode === '脉冲热力' ? t('home.pulseMap') : mode === '产业链' ? t('home.modeChain') : t('home.modeSector')}
            </h1>
            <p className="hidden truncate text-xs text-faint lg:block">
              {filtered.length} 个标的 · 尺寸=市值 · 颜色={lens}
            </p>
          </div>
          <input
            value={locateQuery}
            onChange={(e) => setLocateQuery(e.target.value)}
            placeholder={t('home.locate')}
            className="w-full rounded-lg border border-line bg-surface px-3 py-1.5 text-sm outline-none focus:border-faint sm:w-[220px]"
          />
          <span className="inline-flex items-center gap-1.5 rounded border border-up/30 bg-up/10 px-2.5 py-1 font-mono text-xs text-up">
            <span className="h-1.5 w-1.5 rounded-full bg-up" />
            {t('home.judged', { a: judged, b: companies.length || '—' })}
          </span>
        </header>

        {mode !== '板块' && (
          <div className="flex flex-wrap items-center gap-2 sm:flex-nowrap sm:overflow-x-auto sm:[scrollbar-width:none] sm:[&::-webkit-scrollbar]:hidden">
            {PULSE_INDUSTRIES.map((ind) => {
              const active = industry === ind.id;
              const count = industryCounts.get(ind.id) || 0;
              return (
                <button
                  key={ind.id}
                  title={ind.desc}
                  onClick={() => setIndustry(ind.id)}
                  className={`flex shrink-0 items-baseline gap-1.5 rounded-md px-2.5 py-1 text-[13px] font-medium transition ${
                    active
                      ? 'bg-surface-3 text-ink'
                      : ind.id === 'AI'
                        ? 'bg-surface-2 text-accent ring-1 ring-accent/30'
                        : 'bg-surface text-muted ring-1 ring-line hover:bg-surface-2'
                  }`}
                >
                  <span>{ind.name}</span>
                  <span className={`font-mono text-[10px] tabular-nums ${active ? 'opacity-70' : 'text-faint'}`}>
                    {count}
                  </span>
                </button>
              );
            })}
          </div>
        )}

        <p className="rounded-lg border border-line bg-surface px-3 py-2 text-[11px] leading-relaxed text-faint">
          {t('home.aiDisclaimer')}
        </p>

        {pulseHistorical && (
          <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 px-3 py-2">
            <DataStatus
              state="stale"
              label={language === 'en' ? 'Historical pulse snapshot' : '历史脉冲快照'}
              dataTime={pulseMeta?.dataTime}
              source={pulseMeta?.source}
              message={language === 'en' ? 'Prices, changes and heat values are for review only, not current signals.' : '价格、涨跌与过热度仅供回看，不是当前交易信号。'}
              compact
            />
          </div>
        )}

        {mode !== '板块' && (
          <div className="flex flex-wrap items-center gap-1.5 rounded-xl border border-line bg-surface px-4 py-3">
            <span className="mr-1 font-mono text-xs uppercase tracking-wider text-faint">{t('home.market')}</span>
            {REGION_TABS.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setRegion(tab.id)}
                className={`rounded-md px-3 py-1.5 text-xs font-medium transition ${
                  region === tab.id ? 'bg-surface-3 text-ink' : 'bg-surface-2 text-muted hover:bg-line'
                }`}
              >
                {tab.id === 'US' ? t('home.regionUS') : tab.id === 'CN' ? t('home.regionCN') : tab.id === 'HK' ? t('home.regionHK') : tab.id === 'TW' ? t('home.regionTW') : tab.id === 'KR' ? t('home.regionKR') : tab.id === 'EU' ? t('home.regionEU') : t('home.regionJP')}
              </button>
            ))}
          </div>
        )}

        {mode === '脉冲热力' && region === 'CN' && <CnToolbox />}

        {mode === '脉冲热力' && (
          <div className="grid grid-cols-12 gap-4 lg:gap-5">
            <aside className="col-span-12 space-y-4 lg:col-span-3">
              <div className="rounded-xl border border-line bg-surface p-4">
                <div className="mb-2 text-[10px] font-mono uppercase tracking-wider text-faint">
                  Market · {lens}
                </div>
                <div className="flex items-baseline gap-2">
                  <span className="text-[28px] font-semibold tabular-nums leading-none text-ink">{metrics.scored > 0 ? metrics.avg : '—'}</span>
                  {metrics.scored > 0 && <span className="text-sm text-muted">/ 100</span>}
                </div>
                <div className="mt-1 text-sm font-medium text-muted">{tone}</div>
                <div className="mt-3 grid grid-cols-3 gap-2 text-center">
                  <div>
                    <div className="text-lg font-semibold tabular-nums text-down">{metrics.over}</div>
                    <div className="mt-0.5 text-[10px] uppercase tracking-wider text-muted">
                      {lens === '过热度' ? '过热/泡沫' : '高'}
                    </div>
                  </div>
                  <div>
                    <div className="text-lg font-semibold tabular-nums text-ink">{metrics.total}</div>
                    <div className="mt-0.5 text-[10px] uppercase tracking-wider text-muted">总数</div>
                  </div>
                  <div>
                    <div className="text-lg font-semibold tabular-nums text-accent">{metrics.cheap}</div>
                    <div className="mt-0.5 text-[10px] uppercase tracking-wider text-muted">
                      {lens === '过热度' ? '便宜/破位' : '低'}
                    </div>
                  </div>
                </div>
              </div>

              <div className="rounded-xl border border-line bg-surface p-4">
                <div className="mb-3 text-[10px] font-mono uppercase tracking-wider text-faint">Heat Filter</div>
                <div className="flex flex-col gap-1">
                  {HEAT_FILTERS.map((f) => (
                    <VerticalChip key={f.id} active={heatFilter === f.id} onClick={() => setHeatFilter(f.id)}>
                      {f.label}
                    </VerticalChip>
                  ))}
                </div>
              </div>

              <div className="rounded-xl border border-line bg-surface p-4">
                <div className="mb-3 text-[10px] font-mono uppercase tracking-wider text-faint">Layer Focus</div>
                <div className="flex max-h-[280px] flex-col gap-0.5 overflow-y-auto">
                  <VerticalChip active={!layerFocus} onClick={() => setLayerFocus(null)}>
                    全部 {PULSE_LAYERS.length} 层
                  </VerticalChip>
                  {PULSE_LAYERS.map((l) => (
                    <VerticalChip
                      key={l.id}
                      active={layerFocus === l.id}
                      onClick={() => setLayerFocus(layerFocus === l.id ? null : l.id)}
                    >
                      <span className="mr-1.5 font-mono text-[10px] text-faint">{l.id}</span>
                      {l.name}
                    </VerticalChip>
                  ))}
                </div>
              </div>
            </aside>

            <section className="order-first col-span-12 space-y-4 lg:order-none lg:col-span-6">
              <div className="space-y-2 rounded-xl border border-line bg-surface px-3 py-3">
                <div className="flex flex-wrap items-center gap-1 rounded-lg bg-surface-2 p-1">
                  {LENSES.map((item) => (
                    <button
                      key={item}
                      title={LENS_SUB[item]}
                      onClick={() => setLens(item)}
                      className={`rounded-md px-2.5 py-1.5 text-xs font-semibold transition ${
                        lens === item ? 'bg-surface text-ink' : 'text-muted hover:text-ink'
                      }`}
                    >
                      {item}
                    </button>
                  ))}
                </div>
                <div className="truncate font-mono text-[10px] text-faint">{LENS_SUB[lens]}</div>
                <div className="relative pt-1">
                  <div className="h-2 w-full rounded-full ring-1 ring-white/5" style={{ background: rampCss }} />
                  <div className="relative mt-1.5 h-7">
                    {RAMP_MARKS.map((m) => (
                      <div
                        key={m.v}
                        className="absolute flex -translate-x-1/2 flex-col items-center"
                        style={{ left: `${m.v}%` }}
                      >
                        <div className="-mt-2 h-1.5 w-px bg-line" />
                        <span className="mt-0.5 font-mono text-[9px] text-muted">{m.v}</span>
                        <span className="mt-0.5 whitespace-nowrap text-[9px] font-medium" style={{ color: m.color }}>
                          {m.label}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              </div>

              {loading && (
                <div className="flex items-center justify-center rounded-2xl border border-line bg-[#06080F]" style={{ height: config.height }}>
                  <LoadingSpinner size="lg" />
                </div>
              )}
              {error && (
                <div role="alert" className="rounded-xl border border-line bg-base px-3 py-8 text-center text-sm text-muted">
                  <p>{error}</p>
                  <button type="button" onClick={retryHomeData} className="mt-3 inline-flex items-center gap-1.5 rounded-md border border-line px-3 py-2 text-xs font-semibold text-accent hover:bg-surface-2">
                    <RefreshCw className="h-3.5 w-3.5" />重试脉冲数据
                  </button>
                </div>
              )}
              {!loading && !error && nodes.length > 0 && (
                <div className="overflow-hidden rounded-2xl border border-line bg-[#06080F] ring-1 ring-white/10">
                  <Heatmap
                    nodes={nodes}
                    config={config}
                    layout={industry === 'AI' ? 'layer-scatter' : 'force'}
                    onNodeClick={(node) => navigate(`/stock/${node.symbol}`)}
                  />
                </div>
              )}
              {!loading && !error && nodes.length === 0 && (
                <div className="rounded-xl border border-line bg-base px-3 py-10 text-center text-sm text-muted">
                  当前筛选无结果，试试放宽热度 / 层级 / 行业。
                </div>
              )}

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className="rounded-xl border border-line bg-surface p-4">
                  <div className="mb-3 flex items-baseline justify-between">
                    <h3 className="text-sm font-semibold text-ink">{t('home.top8')}</h3>
                    <span className="font-mono text-[10px] uppercase tracking-wider text-faint">{lens}</span>
                  </div>
                  <div className="space-y-1">
                    {highScores.map(({ c, v }, index) => (
                      <button
                        key={c.ticker}
                        onClick={() => navigate(`/stock/${c.ticker}`)}
                        className="flex min-h-[40px] w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs transition hover:bg-surface-2"
                      >
                        <span className="w-4 font-mono text-[10px] text-faint">{index + 1}</span>
                        <span className="w-14 truncate font-mono text-xs font-semibold text-ink">{c.ticker}</span>
                        <span className="min-w-0 flex-1 truncate text-muted">{c.name}</span>
                        <span className="font-mono text-xs font-semibold tabular-nums text-accent">{v}</span>
                      </button>
                    ))}
                  </div>
                </div>
                <div className="rounded-xl border border-line bg-surface p-4">
                  <div className="mb-3 flex items-baseline justify-between">
                    <h3 className="text-sm font-semibold text-ink">{t('home.low6')}</h3>
                    <span className="font-mono text-[10px] uppercase tracking-wider text-faint">{lens}</span>
                  </div>
                  <div className="space-y-1">
                    {lowScores.map(({ c, v }, index) => (
                      <button
                        key={c.ticker}
                        onClick={() => navigate(`/stock/${c.ticker}`)}
                        className="flex min-h-[40px] w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs transition hover:bg-surface-2"
                      >
                        <span className="w-4 font-mono text-[10px] text-faint">{index + 1}</span>
                        <span className="w-14 truncate font-mono text-xs font-semibold text-ink">{c.ticker}</span>
                        <span className="min-w-0 flex-1 truncate text-muted">{c.name}</span>
                        <span className="font-mono text-xs font-semibold tabular-nums text-down">{v}</span>
                      </button>
                    ))}
                  </div>
                </div>
              </div>
            </section>

            <aside className="col-span-12 space-y-4 lg:col-span-3">
              <div className="sticky top-6 space-y-4">
                <SentimentPanel defaultTab={region === 'CN' ? 'cn' : 'us'} />
                <UpcomingEvents />
                <MarketMovers />
              </div>
            </aside>
          </div>
        )}

        {mode === '产业链' && (
          <div className="rounded-xl border border-line bg-surface p-4">
            <div className="mb-3 text-sm font-semibold text-ink">{t('home.chainDist')}</div>
            <div className="grid gap-2 sm:grid-cols-2">
              {PULSE_INDUSTRIES.map((ind) => (
                <button
                  key={ind.id}
                  onClick={() => {
                    setIndustry(ind.id);
                    setMode('脉冲热力');
                  }}
                  className="flex items-center justify-between rounded-lg border border-line bg-base px-3 py-2 text-left text-sm hover:bg-surface-2"
                >
                  <span className="text-ink">{ind.name}</span>
                  <span className="font-mono text-muted">{industryCounts.get(ind.id) || 0}</span>
                </button>
              ))}
            </div>
          </div>
        )}

        {mode === '板块' && (
          <div className="rounded-xl border border-line bg-surface p-4">
            <div className="mb-3 text-sm font-semibold text-ink">{t('home.sectorDist')}</div>
            <div className="grid gap-2 sm:grid-cols-2">
              {sectorBuckets.map(([name, count]) => (
                <div key={name} className="flex items-center justify-between rounded-lg border border-line bg-base px-3 py-2 text-sm">
                  <span className="truncate text-ink">{name}</span>
                  <span className="font-mono text-muted">{count}</span>
                </div>
              ))}
            </div>
          </div>
        )}

        <section className="space-y-3 border-t border-line pt-5" aria-label={language === 'en' ? 'Supporting market views' : '辅助市场观察'}>
          <AshareMarketRadar />
          <KeyWatchlistPanel />
          <MarketAnomaliesRadar />
          <QDIIPremiumRadar />
        </section>

        <footer className="space-y-2 border-t border-line pt-6 text-center text-xs text-faint">
          <div className="text-sm font-semibold text-ink">Not a Stock God</div>
          <div>{t('home.footerTag')}</div>
          <div>{t('home.footerCn')}</div>
          <div className="mx-auto max-w-3xl leading-relaxed">
            {t('home.footerLegal')}
          </div>
          <div className="flex flex-wrap items-center justify-center gap-3 pt-2">
            <a href="/about" className="hover:text-ink">{t('home.aboutLink')}</a>
            <a href="/terms" className="hover:text-ink">{t('home.terms')}</a>
            <a href="/privacy" className="hover:text-ink">{t('home.privacy')}</a>
            <a href="/tactical" className="hover:text-ink">Paper 模拟</a>
          </div>
          <div>{t('home.footerMeta', { n: companies.length })}</div>
          <div>© Not a Stock God · Not Financial Advice</div>
        </footer>
      </div>
    </StockGodShell>
  );
};
