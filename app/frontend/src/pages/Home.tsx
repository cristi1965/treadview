import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { StockGodShell } from '../components/layout/StockGodShell';
import { Heatmap } from '../components/heatmap';
import { LoadingSpinner } from '../components/common';
import { HeatmapNode, HeatmapNodeWithPosition, HeatmapConfig } from '../types/heatmap';
import { prepareNodes } from '../utils/heatmap';
import { get } from '../utils/api';
import {
  loadUsMarketStocks,
  loadAMarketStocks,
  filterStocksByHeatMarket,
  PremarketMovers,
  HeatMarket,
} from '../utils/stockgodData';
import { applyQuotesToStocks, fetchLiveQuotes, startQuotePolling } from '../utils/liveQuotes';
import { Stock } from '../types/stocks';
import { useUiStore } from '../stores/uiStore';

const MARKETS: HeatMarket[] = ['美股', 'A 股', '港股', '台股', '韩股', '欧', '日'];
const HEAT_FILTERS = ['全部', '过热 ≥85', '偏热 70-85', '合理 50-70', '偏冷 <50'] as const;
const LAYERS = [
  '全部 8 层',
  'L0能源底座',
  'L1EDA · 设备 · 材料',
  'L2晶圆 · 封装 · HBM',
  'L3AI 芯片',
  'L4数据中心基建',
  'L5云 · 模型 · 数据',
  'L6AI 应用',
  'L7端侧 · 入口',
] as const;
const LENSES = ['过热度', '综合', '巴菲特', '段永平', 'Serenity', '德鲁肯米勒', '情绪资金面', '分歧'] as const;
const MODES = ['脉冲热力', '产业链', '板块'] as const;

const INDUSTRY_KEYWORDS: Record<string, string[]> = {
  'AI 产业链': ['ai', '半导体', 'semi', 'software', 'technology', '科技', '算力', 'cloud', 'data'],
  '稀有 / 战略金属': ['metal', 'mining', 'materials', '稀土', '锂', '铜'],
  '人形机器人': ['robot', '自动化', 'industrial'],
  '国防 / 军工': ['defense', 'aerospace', '军工', '国防'],
  '生物医药': ['health', 'bio', 'pharma', '医药', '医疗'],
  '新能源车': ['auto', 'ev', '电动', '汽车'],
  '光伏 / 储能': ['solar', 'energy', '储能', '光伏', '电池'],
  '消费电子': ['consumer electronics', 'hardware', '消费电子'],
  '金融': ['finance', 'bank', '金融', '保险'],
  '地产 / REIT': ['real estate', 'reit', '地产'],
  '软件 / 互联网': ['software', 'internet', '互联网', '软件'],
  '工业 / 制造': ['industrial', 'manufactur', '工业', '制造'],
  '能源 / 油气': ['energy', 'oil', 'gas', '油气', '能源'],
  '公用事业': ['utilities', '公用'],
  '大消费': ['consumer', 'retail', '消费', '零售'],
};

const LAYER_KEYWORDS: Record<string, string[]> = {
  'L0能源底座': ['energy', 'power', 'utility', '能源'],
  'L1EDA · 设备 · 材料': ['eda', 'equipment', 'material', '设备', '材料'],
  'L2晶圆 · 封装 · HBM': ['wafer', 'hbm', 'foundry', '封装', '晶圆', 'memory'],
  'L3AI 芯片': ['gpu', 'chip', 'nvda', '半导体', 'ai 芯片', 'semi'],
  'L4数据中心基建': ['data center', 'server', 'infra', '数据中心'],
  'L5云 · 模型 · 数据': ['cloud', 'model', 'saas', '云'],
  'L6AI 应用': ['application', 'app', '应用'],
  'L7端侧 · 入口': ['edge', 'device', 'phone', '端侧', '入口'],
};

const INDEX_SYMS = ['SPY', 'QQQ', 'DIA', 'IWM', 'VIXY'];

const PillButton: React.FC<{ active?: boolean; children: React.ReactNode; onClick?: () => void; className?: string }> = ({
  active,
  children,
  onClick,
  className = '',
}) => (
  <button
    onClick={onClick}
    className={`shrink-0 rounded-lg px-3 py-1.5 text-[12px] font-medium transition ${
      active ? 'bg-surface-3 text-ink' : 'bg-surface text-muted hover:bg-surface-2 hover:text-ink'
    } ${className}`}
  >
    {children}
  </button>
);

const VerticalChip: React.FC<{ active?: boolean; children: React.ReactNode; onClick?: () => void }> = ({
  active,
  children,
  onClick,
}) => (
  <button
    onClick={onClick}
    className={`w-full rounded-lg px-3 py-2 text-left text-[12px] transition ${
      active ? 'bg-accent/10 font-semibold text-accent' : 'text-muted hover:bg-surface-2 hover:text-ink'
    }`}
  >
    {children}
  </button>
);

function inferLayer(stock: Stock): string {
  const hay = `${stock.sector} ${stock.industry || ''} ${stock.segment || ''} ${stock.subSector || ''} ${stock.name}`.toLowerCase();
  for (const [layer, keys] of Object.entries(LAYER_KEYWORDS)) {
    if (keys.some((k) => hay.includes(k.toLowerCase()))) return layer;
  }
  return 'L6AI 应用';
}

function matchesIndustry(stock: Stock, industry: string): boolean {
  const keys = INDUSTRY_KEYWORDS[industry] || [industry.toLowerCase()];
  const hay = `${stock.sector} ${stock.industry || ''} ${stock.segment || ''} ${stock.subSector || ''} ${stock.name}`.toLowerCase();
  return keys.some((k) => hay.includes(k.toLowerCase()));
}

function lensScore(node: HeatmapNode, lens: string): number {
  if (lens === '分歧') return node.divergence ?? 0;
  if (lens === '巴菲特') return node.scores?.buffett ?? node.avgScore;
  if (lens === '段永平') return node.scores?.duanyongping ?? node.avgScore;
  if (lens === 'Serenity') return node.scores?.serenity ?? node.avgScore;
  if (lens === '德鲁肯米勒') return node.scores?.druckenmiller ?? node.avgScore;
  if (lens === '情绪资金面') return node.scores?.sentiment ?? node.avgScore;
  if (lens === '过热度') return node.avgScore;
  return node.avgScore;
}

function heatMatch(score: number, heatFilter: string): boolean {
  if (heatFilter === '全部') return true;
  if (heatFilter === '过热 ≥85') return score >= 85;
  if (heatFilter === '偏热 70-85') return score >= 70 && score < 85;
  if (heatFilter === '合理 50-70') return score >= 50 && score < 70;
  if (heatFilter === '偏冷 <50') return score < 50;
  return true;
}

function scoreTone(score: number): string {
  if (score >= 85) return '过热';
  if (score >= 70) return '偏热';
  if (score >= 50) return '合理';
  if (score >= 30) return '偏冷';
  return '极冷';
}

function stockToNode(s: Stock): HeatmapNode {
  return {
    symbol: s.symbol,
    name: s.name,
    price: s.price,
    changePercent: s.changePercent,
    marketCap: s.marketCap,
    sector: s.sector,
    avgScore: s.avgScore,
    scores: s.scores,
    divergence: s.divergence,
    segment: s.segment,
    subSector: s.subSector,
    industry: s.industry,
    layer: inferLayer(s),
    country: s.country,
    venue: s.venue,
  };
}

function asStock(n: HeatmapNode): Stock {
  return {
    symbol: n.symbol,
    name: n.name,
    price: n.price,
    change: 0,
    changePercent: n.changePercent,
    marketCap: n.marketCap,
    volume: 0,
    sector: n.sector,
    scores: n.scores || { buffett: 0, duanyongping: 0, serenity: 0, druckenmiller: 0, sentiment: 0 },
    avgScore: n.avgScore,
    isWatched: false,
    industry: n.industry,
    segment: n.segment,
    subSector: n.subSector,
    country: n.country,
    venue: n.venue,
    divergence: n.divergence,
  };
}

export const Home: React.FC = () => {
  const navigate = useNavigate();
  const language = useUiStore((s) => s.language);
  const [usStocksRaw, setUsStocksRaw] = useState<Stock[]>([]);
  const [cnStocksRaw, setCnStocksRaw] = useState<Stock[]>([]);
  const [usNodes, setUsNodes] = useState<HeatmapNode[]>([]);
  const [cnNodes, setCnNodes] = useState<HeatmapNode[]>([]);
  const [industryCounts, setIndustryCounts] = useState<Array<[string, number]>>(
    Object.keys(INDUSTRY_KEYWORDS).map((name) => [name, 0])
  );
  const [macroTickers, setMacroTickers] = useState<[string, string, string][]>([]);
  const [indexQuotes, setIndexQuotes] = useState<Record<string, { price: number; pct: number }>>({});
  const [premarket, setPremarket] = useState<PremarketMovers | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [quoteAge, setQuoteAge] = useState('');
  const [mode, setMode] = useState<(typeof MODES)[number]>('脉冲热力');
  const [market, setMarket] = useState<HeatMarket>('美股');
  const [heatFilter, setHeatFilter] = useState<(typeof HEAT_FILTERS)[number]>('全部');
  const [layer, setLayer] = useState<(typeof LAYERS)[number]>('全部 8 层');
  const [lens, setLens] = useState<(typeof LENSES)[number]>('过热度');
  const [industry, setIndustry] = useState('AI 产业链');
  const [locateQuery, setLocateQuery] = useState('');

  const config: HeatmapConfig = useMemo(
    () => ({
      width: Math.min(window.innerWidth - 520, 720),
      height: 560,
      minRadius: 3,
      maxRadius: 36,
      forceStrength: -28,
      collisionPadding: 2,
      alphaDecay: 0.02,
    }),
    []
  );

  useEffect(() => {
    const fetchData = async () => {
      try {
        setLoading(true);
        const [usStocks, cnStocks, macroResp, moversResp, indexResp] = await Promise.all([
          loadUsMarketStocks(),
          loadAMarketStocks().catch(() => [] as Stock[]),
          get<{ series: any[] }>('/api/macro').catch(() => ({ series: [] })),
          get<PremarketMovers>('/api/premarket-movers').catch(() => null),
          get<{ quotes: Record<string, { price: number; pct: number }> }>(
            `/api/quote?syms=${INDEX_SYMS.join(',')}`
          ).catch(() => ({ quotes: {} })),
        ]);

        setUsStocksRaw(usStocks);
        setCnStocksRaw(cnStocks);
        const nodes = usStocks.filter((s) => s.marketCap > 0).map(stockToNode);
        const aNodes = cnStocks.filter((s) => s.marketCap > 0).map(stockToNode);
        setUsNodes(nodes);
        setCnNodes(aNodes);
        setIndexQuotes(indexResp?.quotes || {});
        setIndustryCounts(
          Object.keys(INDUSTRY_KEYWORDS).map((name) => [
            name,
            nodes.filter((n) => matchesIndustry(asStock(n), name)).length,
          ])
        );

        if (macroResp?.series?.length > 0) {
          setMacroTickers(
            macroResp.series.map((item: any) => [
              item.name,
              item.kind === 'rate' ? `${item.price.toFixed(2)}%` : Math.round(item.price).toLocaleString('en-US'),
              `${item.pct >= 0 ? '+' : ''}${item.pct.toFixed(2)}%`,
            ]) as [string, string, string][]
          );
        }
        if (moversResp) setPremarket(moversResp);
        setError(null);
      } catch (err) {
        console.error(err);
        setError(language === 'en' ? 'Failed to load data' : '数据加载失败');
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, [language]);

  // Live quote overlay for visible heat universe + index strip (20s)
  useEffect(() => {
    if (!usStocksRaw.length && !cnStocksRaw.length) return;
    const refresh = async () => {
      const pool = market === 'A 股' ? cnStocksRaw : usStocksRaw;
      const top = pool
        .filter((s) => s.marketCap > 0)
        .sort((a, b) => b.marketCap - a.marketCap)
        .slice(0, 200)
        .map((s) => s.symbol);
      const quotes = await fetchLiveQuotes([...INDEX_SYMS, ...top]);
      if (!Object.keys(quotes).length) return;
      setIndexQuotes((prev) => ({ ...prev, ...quotes }));
      if (market === 'A 股') {
        const next = applyQuotesToStocks(cnStocksRaw, quotes);
        setCnStocksRaw(next);
        setCnNodes(next.filter((s) => s.marketCap > 0).map(stockToNode));
      } else {
        const next = applyQuotesToStocks(usStocksRaw, quotes);
        setUsStocksRaw(next);
        setUsNodes(next.filter((s) => s.marketCap > 0).map(stockToNode));
      }
      setQuoteAge(new Date().toLocaleTimeString());
    };
    return startQuotePolling(refresh, 20_000);
  }, [market, usStocksRaw.length, cnStocksRaw.length]);

  const displayMacro = useMemo(() => {
    // Prefer live-like bond/index strip from /api/macro; fall back to ETF proxies.
    if (macroTickers.length > 0) return macroTickers;
    return INDEX_SYMS.map((sym) => {
      const q = indexQuotes[sym];
      if (!q) return null;
      return [sym, q.price.toFixed(2), `${q.pct >= 0 ? '+' : ''}${q.pct.toFixed(2)}%`] as [string, string, string];
    }).filter(Boolean) as [string, string, string][];
  }, [indexQuotes, macroTickers]);

  const marketPool = useMemo(() => (market === 'A 股' ? cnNodes : usNodes), [market, usNodes, cnNodes]);

  const filteredRaw = useMemo(() => {
    let list = filterStocksByHeatMarket(marketPool.map(asStock), market).map(stockToNode);

    if (market === '美股') {
      list = list.filter((n) => matchesIndustry(asStock(n), industry));
      if (layer !== '全部 8 层') list = list.filter((n) => n.layer === layer);
    }

    const scoreOf = (n: HeatmapNode) =>
      market === 'A 股' || (n.avgScore === 0 && market !== '美股')
        ? Math.max(0, Math.min(100, 50 + n.changePercent * 5))
        : lensScore(n, lens);

    list = list.filter((n) => heatMatch(scoreOf(n), heatFilter));

    const q = locateQuery.trim().toLowerCase();
    if (q) {
      list = list.filter((n) => n.symbol.toLowerCase().includes(q) || n.name.toLowerCase().includes(q));
    }
    return list.sort((a, b) => b.marketCap - a.marketCap).slice(0, market === 'A 股' ? 1200 : 986);
  }, [marketPool, market, industry, layer, heatFilter, lens, locateQuery]);

  const nodes: HeatmapNodeWithPosition[] = useMemo(() => {
    if (filteredRaw.length === 0 || mode !== '脉冲热力') return [];
    return prepareNodes(
      filteredRaw,
      config.width,
      config.height,
      config.minRadius,
      config.maxRadius,
      market === 'A 股' ? 'change' : 'score',
      (n) =>
        market === 'A 股' ? Math.max(0, Math.min(100, 50 + n.changePercent * 5)) : lensScore(n, lens),
      market === 'A 股' ? 'force' : 'layer-scatter'
    );
  }, [filteredRaw, config, lens, market, mode]);

  const marketMetrics = useMemo(() => {
    if (!filteredRaw.length) return { avg: 0, over: 0, total: 0, cheap: 0 };
    const scores = filteredRaw.map((n) =>
      market === 'A 股' || (n.avgScore === 0 && market !== '美股')
        ? Math.max(0, Math.min(100, 50 + n.changePercent * 5))
        : lensScore(n, lens)
    );
    const avg = Math.round(scores.reduce((a, b) => a + b, 0) / scores.length);
    return {
      avg,
      over: scores.filter((s) => s >= 75).length,
      total: scores.length,
      cheap: scores.filter((s) => s <= 45).length,
    };
  }, [filteredRaw, lens, market]);

  const highScores = useMemo(
    () =>
      [...filteredRaw]
        .map((n) => ({ n, s: lensScore(n, lens) || Math.max(0, Math.min(100, 50 + n.changePercent * 5)) }))
        .sort((a, b) => b.s - a.s)
        .slice(0, 8)
        .map(({ n, s }) => [n.symbol, n.name, Math.round(s)] as const),
    [filteredRaw, lens]
  );

  const lowScores = useMemo(
    () =>
      [...filteredRaw]
        .map((n) => ({ n, s: lensScore(n, lens) || Math.max(0, Math.min(100, 50 + n.changePercent * 5)) }))
        .filter((x) => x.s > 0)
        .sort((a, b) => a.s - b.s)
        .slice(0, 6)
        .map(({ n, s }) => [n.symbol, n.name, Math.round(s)] as const),
    [filteredRaw, lens]
  );

  const sectorBuckets = useMemo(() => {
    const map = new Map<string, number>();
    for (const n of filteredRaw) map.set(n.sector || '其他', (map.get(n.sector || '其他') || 0) + 1);
    return [...map.entries()].sort((a, b) => b[1] - a[1]).slice(0, 24);
  }, [filteredRaw]);

  const judged = filteredRaw.filter((n) => n.avgScore > 0).length;
  const tone = scoreTone(marketMetrics.avg);

  return (
    <StockGodShell title={language === 'en' ? 'Heatmap' : '热力图'} macroTickers={displayMacro}>
      <div className="space-y-3">
        <section className="rounded-xl border border-line bg-surface px-4 py-3 sm:px-5">
          <div className="text-[15px] font-semibold tracking-tight text-ink">你不是神,但神陪你一起看股票</div>
          <p className="mt-1 max-w-3xl text-[12px] leading-relaxed text-muted">
            五种投资框架,对同一只票各自独立打分。分歧越大,越值得你亲自研究。
          </p>
          <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-faint">
            <span>1 热力图 / 列表里找一只票</span>
            <span>2 点开看五方各自的判读与分歧</span>
            <span>3 按均分 / 分歧排序,挑你想深挖的</span>
          </div>
        </section>

        {/* Mode tabs — live: 脉冲热力 / 产业链 / 板块 */}
        <div className="flex flex-wrap items-center gap-1 border-b border-line pb-2">
          {MODES.map((item) => (
            <button
              key={item}
              onClick={() => setMode(item)}
              className={`relative px-3 py-2 text-[13px] font-medium transition ${
                mode === item ? 'text-ink' : 'text-muted hover:text-ink'
              }`}
            >
              {item}
              <span
                className={`absolute inset-x-2 -bottom-[9px] h-[2px] rounded-full bg-accent transition ${
                  mode === item ? 'opacity-100' : 'opacity-0'
                }`}
              />
            </button>
          ))}
        </div>

        <header className="flex flex-wrap items-baseline gap-3">
          <h1 className="text-[22px] font-semibold tracking-tight text-ink">
            {industry} · {mode === '脉冲热力' ? '脉冲热力图' : mode}
          </h1>
          <p className="text-xs text-faint">
            {filteredRaw.length} 个标的 · 尺寸=市值 · 颜色={lens}
          </p>
          <span className="ml-auto rounded-full bg-up/10 px-2.5 py-1 font-mono text-xs text-up">
            判读 {judged}/{usNodes.length || '—'}
          </span>
        </header>

        <div className="flex flex-wrap items-center gap-2">
          <input
            value={locateQuery}
            onChange={(e) => setLocateQuery(e.target.value)}
            placeholder="定位:代码 / 名称…"
            className="min-h-9 w-full max-w-xs rounded-lg border border-line bg-base px-3 text-sm text-ink outline-none placeholder:text-faint focus:border-accent/50"
          />
        </div>

        <div className="flex gap-1.5 overflow-x-auto pb-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
          {industryCounts.map(([name, count]) => (
            <PillButton key={name} active={industry === name} onClick={() => setIndustry(name)}>
              {name}
              <span className="ml-1 font-mono text-faint">{count}</span>
            </PillButton>
          ))}
        </div>

        <p className="text-[11px] leading-relaxed text-faint">
          AI 方法论模拟 —— 巴菲特 / 段永平 / 德鲁肯米勒 / Serenity 的评分由 AI 依据各自公开方法论生成,并非本人真实观点或持仓,亦不代表其本人。非投资建议。
        </p>

        <div className="flex flex-wrap gap-1.5 border-b border-line pb-2">
          {MARKETS.map((item) => (
            <button
              key={item}
              onClick={() => setMarket(item)}
              className={`relative px-2.5 py-1.5 text-[12px] font-medium transition ${
                market === item ? 'text-ink' : 'text-muted hover:text-ink'
              }`}
            >
              {item}
              <span
                className={`absolute inset-x-1 -bottom-[9px] h-[2px] rounded-full bg-accent transition ${
                  market === item ? 'opacity-100' : 'opacity-0'
                }`}
              />
            </button>
          ))}
        </div>

        {market !== '美股' && (
          <div className="rounded-lg border border-line bg-base px-3 py-2 text-center text-xs text-muted">
            {market === 'A 股'
              ? `A 股热力图 · ${cnNodes.length} 只（行情快照）。暂无五方评分，颜色按涨跌映射。`
              : `${market} · 美股上市相关标的 ${filteredRaw.length} 只`}
          </div>
        )}

        <div className="grid gap-3 lg:grid-cols-[168px_minmax(0,1fr)_240px]">
          {/* Left filters — live order */}
          <aside className="space-y-3">
            <section className="rounded-xl border border-line bg-surface p-3">
              <div className="mb-2 text-[10px] font-semibold uppercase tracking-[0.16em] text-faint">Heat Filter</div>
              <div className="space-y-0.5">
                {HEAT_FILTERS.map((item) => (
                  <VerticalChip key={item} active={heatFilter === item} onClick={() => setHeatFilter(item)}>
                    {item}
                  </VerticalChip>
                ))}
              </div>
            </section>
            <section className="rounded-xl border border-line bg-surface p-3">
              <div className="mb-2 text-[10px] font-semibold uppercase tracking-[0.16em] text-faint">Layer Focus</div>
              <div className="max-h-[280px] space-y-0.5 overflow-y-auto">
                {LAYERS.map((item) => (
                  <VerticalChip key={item} active={layer === item} onClick={() => setLayer(item)}>
                    {item}
                  </VerticalChip>
                ))}
              </div>
            </section>
          </aside>

          {/* Center: score + lenses + heatmap */}
          <section className="min-w-0 space-y-3">
            <div className="grid gap-3 sm:grid-cols-[180px_minmax(0,1fr)]">
              <div className="rounded-xl border border-line bg-surface p-4">
                <div className="text-[11px] text-muted">Market · {lens}</div>
                <div className="mt-1 font-mono text-[40px] font-semibold leading-none text-ink">{marketMetrics.avg}</div>
                <div className="mt-1 text-sm text-muted">/ 100 · {tone}</div>
                <div className="mt-3 grid grid-cols-3 gap-1 text-center text-[10px]">
                  <div>
                    <div className="font-mono text-sm text-down">{marketMetrics.over}</div>
                    <div className="text-faint">过热/泡沫</div>
                  </div>
                  <div>
                    <div className="font-mono text-sm text-ink">{marketMetrics.total}</div>
                    <div className="text-faint">总数</div>
                  </div>
                  <div>
                    <div className="font-mono text-sm text-up">{marketMetrics.cheap}</div>
                    <div className="text-faint">便宜/破位</div>
                  </div>
                </div>
              </div>

              <div className="rounded-xl border border-line bg-surface p-3">
                <div className="mb-2 flex flex-wrap gap-1">
                  {LENSES.map((item) => (
                    <button
                      key={item}
                      onClick={() => setLens(item)}
                      className={`relative px-2 py-1 text-[11px] transition ${
                        lens === item ? 'font-semibold text-ink' : 'text-muted hover:text-ink'
                      }`}
                    >
                      {item}
                      <span
                        className={`absolute inset-x-1 -bottom-0.5 h-[2px] rounded-full bg-accent transition ${
                          lens === item ? 'opacity-100' : 'opacity-0'
                        }`}
                      />
                    </button>
                  ))}
                </div>
                <div className="relative mt-3 h-3 overflow-hidden rounded-full">
                  <div
                    className="absolute inset-0"
                    style={{
                      background:
                        'linear-gradient(90deg,#1d4ed8 0%,#22d3ee 25%,#22c55e 40%,#eab308 60%,#f97316 80%,#ef4444 100%)',
                    }}
                  />
                  <div
                    className="absolute top-1/2 h-4 w-1 -translate-y-1/2 rounded-full bg-white shadow"
                    style={{ left: `calc(${Math.min(100, marketMetrics.avg)}% - 2px)` }}
                  />
                </div>
                <div className="mt-1 flex justify-between text-[9px] text-faint">
                  <span>极冷</span>
                  <span>偏冷</span>
                  <span>中性</span>
                  <span>合理</span>
                  <span>偏热</span>
                  <span>过热警报</span>
                </div>
                <p className="mt-3 text-[11px] leading-relaxed text-faint">
                  估值贵 + 离52周高点 + RSI/动量 · 越高越泡沫(不随当天涨跌跳)
                </p>
              </div>
            </div>

            {loading && (
              <div className="flex items-center justify-center rounded-xl border border-line bg-base" style={{ height: config.height }}>
                <LoadingSpinner size="lg" />
              </div>
            )}
            {error && (
              <div className="rounded-xl border border-line bg-base px-3 py-10 text-center text-sm text-muted">{error}</div>
            )}

            {!loading && !error && mode === '脉冲热力' && nodes.length > 0 && (
              <div className="overflow-hidden rounded-xl border border-line bg-base">
                <Heatmap
                  nodes={nodes}
                  config={config}
                  layout={market === 'A 股' ? 'force' : 'layer-scatter'}
                  onNodeClick={(node) => navigate(`/stock/${node.symbol}`)}
                />
              </div>
            )}

            {!loading && !error && mode === '脉冲热力' && nodes.length === 0 && (
              <div className="rounded-xl border border-line bg-base px-3 py-10 text-center text-sm text-muted">
                当前筛选无结果，试试放宽热度 / 层级 / 行业。
              </div>
            )}

            {!loading && !error && mode === '产业链' && (
              <div className="rounded-xl border border-line bg-surface p-4">
                <div className="mb-3 text-sm font-semibold text-ink">产业链分布</div>
                <div className="grid gap-2 sm:grid-cols-2">
                  {industryCounts.map(([name, count]) => (
                    <button
                      key={name}
                      onClick={() => {
                        setIndustry(name);
                        setMode('脉冲热力');
                      }}
                      className="flex items-center justify-between rounded-lg border border-line bg-base px-3 py-2 text-left text-sm hover:bg-surface-2"
                    >
                      <span className="text-ink">{name}</span>
                      <span className="font-mono text-muted">{count}</span>
                    </button>
                  ))}
                </div>
              </div>
            )}

            {!loading && !error && mode === '板块' && (
              <div className="rounded-xl border border-line bg-surface p-4">
                <div className="mb-3 text-sm font-semibold text-ink">板块分布</div>
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

            <div className="rounded-xl border border-line bg-surface p-3 text-[11px] leading-relaxed text-faint">
              <div>悬停或点击粒子</div>
              <div>每个粒子的尺寸 = 市值 log</div>
              <div>颜色 = 当前镜头的真实评分</div>
              <div>灰 = 该镜头下还没判读</div>
            </div>
          </section>

          {/* Right TOP lists */}
          <aside className="space-y-3">
            <section className="rounded-xl border border-line bg-surface p-3">
              <h3 className="mb-2 text-sm font-semibold text-ink">高分 TOP 8</h3>
              <div className="space-y-1">
                {highScores.map(([symbol, name, score], index) => (
                  <button
                    key={symbol}
                    onClick={() => navigate(`/stock/${symbol}`)}
                    className="flex w-full items-center gap-2 rounded-lg px-1.5 py-1.5 text-left text-xs transition hover:bg-surface-2"
                  >
                    <span className="w-4 font-mono text-faint">{index + 1}</span>
                    <span className="w-12 font-mono font-semibold text-ink">{symbol}</span>
                    <span className="min-w-0 flex-1 truncate text-muted">{name}</span>
                    <span className="font-mono text-accent">{score}</span>
                  </button>
                ))}
              </div>
            </section>
            <section className="rounded-xl border border-line bg-surface p-3">
              <h3 className="mb-2 text-sm font-semibold text-ink">低分 TOP 6</h3>
              <div className="space-y-1">
                {lowScores.map(([symbol, name, score], index) => (
                  <button
                    key={symbol}
                    onClick={() => navigate(`/stock/${symbol}`)}
                    className="flex w-full items-center gap-2 rounded-lg px-1.5 py-1.5 text-left text-xs transition hover:bg-surface-2"
                  >
                    <span className="w-4 font-mono text-faint">{index + 1}</span>
                    <span className="w-12 font-mono font-semibold text-ink">{symbol}</span>
                    <span className="min-w-0 flex-1 truncate text-muted">{name}</span>
                    <span className="font-mono text-down">{score}</span>
                  </button>
                ))}
              </div>
            </section>

            {premarket && (
              <section className="rounded-xl border border-line bg-surface p-3">
                <div className="mb-2 text-xs font-semibold text-muted">{premarket.label || '盘前异动'}</div>
                {premarket.gainers.slice(0, 3).map((item) => (
                  <button
                    key={item.sym}
                    onClick={() => navigate(`/stock/${item.sym}`)}
                    className="flex w-full items-center gap-2 py-1 text-left text-xs"
                  >
                    <span className="font-mono text-ink">{item.sym}</span>
                    <span className="ml-auto font-mono text-up">+{item.pct.toFixed(2)}%</span>
                  </button>
                ))}
              </section>
            )}
          </aside>
        </div>

        <footer className="space-y-2 border-t border-line pt-6 text-center text-xs text-faint">
          <div className="text-sm font-semibold text-ink">Not a Stock God</div>
          <div>你不是神,但神陪你一起看股票</div>
          <div>本站不面向中国大陆用户 · This site is not intended for users in mainland China.</div>
          <div className="mx-auto max-w-3xl leading-relaxed">
            所有内容为信息整理与个人研究记录,非投资建议,不构成对任何证券、平台或交易所的要约或背书。股票、加密货币、代币化资产均涉及重大风险,可能损失全部本金。风险请自行分辨与承担。页面含邀请链接(含返佣)。交易前请自行研究并确认所在司法管辖区的合规性。
          </div>
          <div className="flex flex-wrap items-center justify-center gap-3 pt-2">
            <a href="/about" className="hover:text-ink">
              关于 / 方法论
            </a>
            <a href="/terms" className="hover:text-ink">
              服务条款
            </a>
            <a href="/privacy" className="hover:text-ink">
              隐私政策
            </a>
            <a href="/how-to-buy" className="hover:text-ink">
              如何买
            </a>
          </div>
          <div>
            实时行情 TV/Yahoo{quoteAge ? ` · 刷新 ${quoteAge}` : ''} · 五方判读 · v0.7
          </div>
          <div>© Not a Stock God · Not Financial Advice</div>
        </footer>
      </div>
    </StockGodShell>
  );
};
