import React, { useEffect, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { Avatar } from '../components/Avatar';
import { RadarChart } from '../components/scan';
import { LoadingSpinner } from '../components/common';
import { Stock } from '../types/stocks';
import { formatAUM, formatNumber, formatPercent, formatPrice, getChangeColorClass } from '../utils/format';
import { apiUrl, get as apiGet } from '../utils/api';
import { findUsStock } from '../utils/stockgodData';
import { usePortfolioStore } from '../stores/portfolioStore';

interface Holder {
  guruName: string;
  fundName: string;
  value: string;
  shares: string;
  change: string;
  weight: number;
  avatarCode: string;
}

interface SearchResponse {
  results: Stock[];
  count: number;
}

interface QuoteResponse {
  quotes: Record<string, { price: number; pct: number; session?: string }>;
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
  source?: string;
}

interface NewsItem {
  title: string;
  source: string;
  link?: string;
  published?: number;
}

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

const buildMetricRows = (m: FundamentalsResponse | null, livePrice?: number): Array<[string, string]> => {
  if (!m) {
    return [
      ['PE', '—'],
      ['前瞻PE', '—'],
      ['PS', '—'],
      ['EV/EBITDA', '—'],
      ['PEG', '—'],
      ['PB', '—'],
      ['毛利率', '—'],
      ['净利率', '—'],
      ['ROE', '—'],
      ['营收增速', '—'],
      ['股息率', '—'],
      ['Beta', '—'],
      ['分析师目标', '—'],
      ['分析师评级', '—'],
      ['52周位置', '—'],
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
    ['PE', fmtNum(m.trailingPE, 1)],
    ['前瞻PE', fmtNum(m.forwardPE, 1)],
    ['PS', fmtNum(m.priceToSales, 1)],
    ['EV/EBITDA', fmtNum(m.enterpriseToEbitda, 1)],
    ['PEG', fmtNum(m.peg, 1)],
    ['PB', fmtNum(m.priceToBook, 1)],
    ['毛利率', fmtPctRatio(m.grossMargin)],
    ['净利率', fmtPctRatio(m.profitMargin)],
    ['ROE', fmtPctRatio(m.roe)],
    ['营收增速', fmtPctRatio(m.revenueGrowth)],
    ['股息率', div],
    ['Beta', fmtNum(m.beta, 1)],
    ['分析师目标', target],
    ['分析师评级', recommendLabel(m.recommendation)],
    ['52周位置', typeof m.fiftyTwoWeekPos === 'number' ? `${Math.round(m.fiftyTwoWeekPos)}%` : '—'],
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
    tagline: (s) => (s >= 70 ? '伟大生意·有边际' : s >= 55 ? '伟大生意·太贵观察' : '质地一般·观望'),
  },
  {
    key: 'duanyongping',
    label: '段永平',
    framework: '价值 · 商业模式',
    tagline: (s) => (s >= 75 ? '顶级好生意·重仓' : s >= 60 ? '好生意·可拿' : '模式未看懂'),
  },
  {
    key: 'serenity',
    label: 'Serenity',
    framework: 'alpha · 供应链瓶颈',
    tagline: (s) => (s >= 70 ? 'bottleneck alpha' : s >= 50 ? 'crowded but valid' : 'no edge here'),
  },
  {
    key: 'druckenmiller',
    label: '德鲁肯米勒',
    framework: 'alpha · 宏观流动性',
    tagline: (s) => (s >= 75 ? '顺风重仓' : s >= 55 ? '趋势可跟' : '顺风减弱'),
  },
  {
    key: 'sentiment',
    label: '情绪资金面',
    framework: '盘口 · 资金流',
    tagline: (s) => (s >= 65 ? '资金面顺·可跟' : s >= 45 ? '过热拥挤·见顶警惕' : '情绪冰点·逆向区'),
  },
];

const nvdaNarratives: Record<ScoreKey, { summary: string; detail: string }> = {
  buffett: {
    summary:
      '无可争议的伟大生意——CUDA 加宝箱般的客户黏性就是护城河——但市值已把未来十年的好消息预支干净,没有安全边际我只能坐在场边看。',
    detail:
      '护城河极宽:CUDA 软件生态锁死开发者、NVLink/网络整机方案、台积电先进产能优先权,三重壁垒让对手很难撕开,毛利率长期 70%+ 证明定价权。但内在价值的麻烦在于周期性——半导体资本开支有节奏,把当前峰值利润率线性外推到十年是危险的,我要的是用四毛买一块的安全边际,如今高价签给的是负边际。生意我打满分,价格让我只能观察不能动手。',
  },
  duanyongping: {
    summary:
      '商业模式是教科书级的顶级——卖铲子、规模越大成本护城河越深、客户离不开 CUDA——价格对真正看懂的生意从来最不重要,这种我敢重仓拿住。',
    detail:
      '商业模式上这是我见过最干净的好生意之一:一次研发、全球复制、边际成本趋零的软件+芯片绑定,客户切换成本极高,本质上是在收 AI 算力的过路费。本分文化上,黄仁勋长期主义、不靠财技、把利润砸回研发和生态,company DNA 没有走歪。价格不是我决策的第一位——看懂了商业模式和人,贵一点也是好生意,真正的风险是看错生意而不是买贵,这家我看懂了。',
  },
  serenity: {
    summary:
      '逻辑完全成立但这是地球上最明牌的票,我抠供应链是为了在 NVDA 上游那些没人看的瓶颈环节埋伏,而不是去追这头已经被全世界盯着的大象。',
    detail:
      '从瓶颈视角看,真正卡脖子、错杀、还没被定价的微观环节不在 NVDA 本身,而在它上游——CoWoS 先进封装、HBM 产能、光模块/CPO、铜连接、电源,这些才是我的猎场。NVDA 是下游 capex 浪潮的最终受益者没错,需求侧逻辑硬,但拥挤度爆表、信息差为零,我的方法论在它身上没有 alpha 可挖。它是验证我整条供应链论点的锚,不是我会下重注的标的。',
  },
  druckenmiller: {
    summary:
      '这就是这一轮 AI 资本开支大趋势里弹性最高、最干净的载体,宏观顺风+超大盘流动性吸盘,我不看护城河也不看估值,我只要骑在最强的马上。',
    detail:
      '自上而下,全球科技巨头的 AI capex 仍在加码,这是一条由产业范式驱动的多年大趋势,而 NVDA 是吸收这股流动性弹性最大的单一载体,资金面够深、能装下大仓位。我从不为估值买单或卖出,趋势在、动量在、龙头地位在,我就重仓骑住;真正会让我砍仓的是 capex 见顶或趋势转向的信号,目前没看到。赔率上,顺着最强宏观主线下注最强标的,这是不对称里最舒服的一种。',
  },
  sentiment: {
    summary:
      '全市场第一权重、人人满仓、ETF 被动资金和散户情绪同向打满,这种众人皆醉的拥挤度里我闻到的是顶部风险而不是顺风。',
    detail:
      '资金面上 NVDA 是所有大型科技/AI ETF 的头号权重,被动流入和主动超配高度同向,13F 机构持仓拥挤,期权 gamma 长期偏多头,意味着任何利好出尽都可能引发拥挤交易的踩踏式回吐。情绪周期上 AI 叙事处于亢奋区而非冰点,没有逆向埋伏的折价空间。我不评价生意好坏,只读盘口众生——当一只票成为全民共识的最大单一押注,情绪面给我的信号是警惕见顶,而非追高。',
  },
};

const genericNarrative = (label: string, score: number, stockName: string): { summary: string; detail: string } => ({
  summary: `${label}框架下对 ${stockName} 给出 ${score} 分 —— 这是按公开方法论做的结构化打分,不是买卖指令。`,
  detail: `分数越高,代表该框架下的质量/趋势/赔率匹配越好;分数越低,代表估值、拥挤度、周期位置或产业位置至少有一项不合意。请把分数当作研究线索,再核对业务、财报与仓位风险。`,
});

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

const decision = (stock?: Stock | null) => {
  if (!stock) return { label: '未判读', className: 'text-faint', note: '缺少完整五方评分。' };
  if (stock.avgScore >= 70 && (stock.divergence ?? 0) < 35) return { label: '共识偏多', className: 'text-up', note: '均分较高且分歧可控,值得进入深挖清单。' };
  if (stock.avgScore >= 60) return { label: '谨慎观察', className: 'text-accent', note: '质量或趋势有亮点,但仍需确认估值与仓位拥挤。' };
  if ((stock.divergence ?? 0) >= 35) return { label: '分歧很大', className: 'text-accent', note: '五方看法差异明显,适合亲自研究而非直接跟随。' };
  return { label: '暂不突出', className: 'text-faint', note: '当前综合分不高,除非你有额外研究优势。' };
};

const stockArchetype = (stock?: Stock | null) => {
  if (!stock) return null;
  const sector = stock.sector.toLowerCase();
  if (stock.symbol === 'NVDA' || sector.includes('semiconductor')) {
    return {
      badge: '垄断股 × 成长股 · 主场 段永平',
      description: 'AI 算力 / 数据中心加速计算（GPU + 网络 + CUDA 生态）',
      why: '网络/品牌/技术/规模壁垒,时间是它的盟友',
      watch: ['毛利率稳定性', '市场份额', '定价权(卡脖子环节最强)'],
      warning: "估值定性 > 定量,PE / 营收都会失真 —— 要加'垄断溢价',回到产业链定性分析",
      growth: '兼具 成长股:也看 收入增速 / 毛利率 · 德鲁肯米勒 的菜',
      focus:
        '同一笔生意,段永平和德鲁肯米勒都喊重仓(一个因看懂顶级商业模式、一个因骑宏观顺风),而巴菲特因没有安全边际只观察、Serenity 因太明牌只旁证、情绪面更直接把这份全民共识读成见顶警惕——分歧本质是"伟大且顺风"与"拥挤且无折价"是否能并存。',
    };
  }

  return {
    badge: '质量股 × 趋势股 · 需要估值校准',
    description: `${stock.sector} · 公开行情、基本面与聪明钱交叉观察`,
    why: '先判断公司属于现金流复利、周期反转、宏观趋势还是情绪交易',
    watch: ['业务质量', '估值位置', '资金拥挤度'],
    warning: '不要只看一个估值倍数。高分要确认价格是否过热,低分也要看是否有周期修复或资金面拐点。',
    growth: '如果五方分歧扩大,优先拆解分歧来自业务质量、估值、产业位置还是短线拥挤度。',
    focus: '共识不是正确答案,它只是提醒你市场正在用哪一种叙事给这家公司定价。',
  };
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
  const { symbol } = useParams<{ symbol: string }>();
  const normalizedSymbol = (symbol || '').toUpperCase();
  const [holders, setHolders] = useState<Holder[]>([]);
  const [stock, setStock] = useState<Stock | null>(null);
  const [fundamentals, setFundamentals] = useState<FundamentalsResponse | null>(null);
  const [news, setNews] = useState<NewsItem[]>([]);
  const [loading, setLoading] = useState(true);
  const { addToWatchlist, removeFromWatchlist, isInWatchlist } = usePortfolioStore();
  const watched = isInWatchlist(normalizedSymbol);

  useEffect(() => {
    let cancelled = false;

    const fetchData = async () => {
      let renderedFromSnapshot = false;
      try {
        setLoading(true);
        setStock(null);
        setHolders([]);
        setFundamentals(null);
        setNews([]);

        const fromData = await findUsStock(normalizedSymbol).catch(() => null);
        if (cancelled) return;
        if (fromData) {
          renderedFromSnapshot = true;
          setStock(fromData);
          setLoading(false);
        }

        const [holdersResponse, stockResponse, quoteResponse, fundRes, newsRes] = await Promise.all([
          fetch(apiUrl(`/api/whales/stock/${normalizedSymbol}`)),
          apiGet<SearchResponse>(`/api/stocks/search?q=${encodeURIComponent(normalizedSymbol)}`).catch(() => ({
            results: [],
            count: 0,
          })),
          apiGet<QuoteResponse>(`/api/quote?syms=${encodeURIComponent(normalizedSymbol)}`).catch(() => ({
            quotes: {},
          })),
          apiGet<FundamentalsResponse>(`/api/fundamentals?sym=${encodeURIComponent(normalizedSymbol)}`).catch(
            () => null as unknown as FundamentalsResponse
          ),
          apiGet<{ news?: NewsItem[] }>(`/api/news?sym=${encodeURIComponent(normalizedSymbol)}`).catch(() => ({
            news: [],
          })),
        ]);

        if (cancelled) return;

        if (holdersResponse.ok) {
          const data = await holdersResponse.json();
          setHolders(data || []);
        } else {
          setHolders([]);
        }

        if (fundRes && fundRes.symbol) {
          setFundamentals(fundRes);
        }
        setNews(newsRes?.news || []);

        const exact =
          fromData ||
          stockResponse.results.find((item) => item.symbol.toLowerCase() === normalizedSymbol.toLowerCase()) ||
          stockResponse.results[0] ||
          null;

        const quote = quoteResponse.quotes?.[normalizedSymbol] || quoteResponse.quotes?.[normalizedSymbol.toUpperCase()];
        if (exact && quote) {
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
          const livePrice = quote?.price || fundRes.price || 0;
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
  }, [normalizedSymbol]);

  // Keep price/pct live while on the detail page
  useEffect(() => {
    if (!normalizedSymbol) return;
    let cancelled = false;
    const tick = async () => {
      if (cancelled || document.hidden) return;
      try {
        const quoteResponse = await apiGet<QuoteResponse>(
          `/api/quote?syms=${encodeURIComponent(normalizedSymbol)}`
        );
        const quote =
          quoteResponse.quotes?.[normalizedSymbol] ||
          quoteResponse.quotes?.[normalizedSymbol.toUpperCase()];
        if (!quote || !(quote.price > 0)) return;
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
      } catch {
        /* ignore */
      }
    };
    const id = window.setInterval(() => void tick(), 15_000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [normalizedSymbol]);

  const topHoldersWeight = useMemo(() => holders.slice(0, 10).reduce((sum, holder) => sum + (holder.weight || 0), 0), [holders]);
  const metricRows = useMemo(() => buildMetricRows(fundamentals, stock?.price), [fundamentals, stock?.price]);
  const verdict = decision(stock);
  const archetype = stockArchetype(stock);
  const hasAnyData = !!stock || holders.length > 0;
  const chain = chainMap[normalizedSymbol] || {
    label: archetype?.description || '产业链定位',
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
      <header className="mb-5 border-b border-line pb-4">
        <div className="mb-3 flex flex-wrap items-center gap-2 text-xs text-muted">
          <Link to="/" className="transition hover:text-ink">热力图</Link>
          <span className="text-faint">/</span>
          <Link to="/scan" className="transition hover:text-ink">扫描</Link>
          <span className="text-faint">/</span>
          <Link to="/portfolio" className="transition hover:text-ink">观察</Link>
          <span className="text-faint">/</span>
          <span className="text-ink">{normalizedSymbol}</span>
        </div>

        <div className="flex flex-wrap items-start justify-between gap-4">
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
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted">
              {stock?.marketCap ? <span>市值 {formatAUM(stock.marketCap)}</span> : null}
              {stock?.sector ? <span className="rounded bg-surface-3 px-1.5 py-0.5">{stock.sector}</span> : null}
            </div>
            {archetype && <p className="mt-2 max-w-2xl text-xs leading-relaxed text-muted">{archetype.description}</p>}
          </div>

          {stock && (
            <div className="text-right">
              <div className="font-mono text-2xl font-semibold text-ink">{formatPrice(stock.price)}</div>
              <div className={`font-mono text-sm font-semibold ${getChangeColorClass(stock.changePercent)}`}>{formatPercent(stock.changePercent)}</div>
              <div className="mt-1 text-[11px] text-faint">集合竞价 / regular</div>
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
              <Link to="/" className="rounded-lg border border-accent/30 bg-accent/10 px-4 py-2 text-sm font-semibold text-accent transition hover:bg-accent/15">回热力图</Link>
              <Link to="/scan" className="rounded-lg border border-line bg-surface px-4 py-2 text-sm text-muted transition hover:text-ink">去列表找票</Link>
            </div>
          </div>
        </div>
      ) : (
        <div className="space-y-5">
          {stock && (
            <>
              {archetype && (
                <section className="rounded-xl border border-accent/25 bg-accent/10 p-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="min-w-0 flex-1">
                      <div className="text-xs font-semibold text-accent">👑 这是 {archetype.badge}</div>
                      <p className="mt-2 text-sm leading-relaxed text-ink">{archetype.why}</p>
                      <div className="mt-2 flex flex-wrap gap-1.5">
                        {archetype.watch.map((item) => (
                          <span key={item} className="rounded border border-line bg-surface px-2 py-0.5 text-[11px] text-muted">
                            该看 {item}
                          </span>
                        ))}
                      </div>
                    </div>
                    <Link
                      to="/"
                      className="rounded-lg border border-accent/30 bg-base px-3 py-2 text-xs font-semibold text-accent transition hover:bg-surface-2"
                    >
                      在热力图中定位 →
                    </Link>
                  </div>
                  <div className="mt-3 grid gap-2 text-xs leading-relaxed text-muted md:grid-cols-3">
                    <div className="rounded-lg border border-line bg-surface px-3 py-2">⚠️ {archetype.warning}</div>
                    <div className="rounded-lg border border-line bg-surface px-3 py-2">{archetype.growth}</div>
                    <div className="rounded-lg border border-line bg-surface px-3 py-2">类型决定用什么尺子量 —— PE 只是众多指标之一,每类股该看的东西不一样</div>
                  </div>
                </section>
              )}

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
                  <div className={`mt-2 font-mono text-xl font-semibold ${scoreColor(stock.avgScore)}`}>{Math.round(stock.avgScore)}</div>
                </section>
                <section className="rounded-xl border border-line bg-surface p-4">
                  <div className="text-xs text-faint">分歧</div>
                  <div className="mt-2 font-mono text-xl font-semibold text-accent">{Math.round(divergence)}</div>
                </section>
              </div>

              <section className="rounded-xl border border-line bg-surface p-4">
                <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <h2 className="text-sm font-semibold text-ink">真实基本面</h2>
                    <p className="mt-1 text-xs text-muted">
                      高亮 = 这类股该重点看 · 数据 {fundamentals?.source ? 'Yahoo 实时' : '加载中…'} · 估值随报价
                    </p>
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-5">
                  {metricRows.map(([label, value]) => {
                    const highlight = ['毛利率', '净利率', 'ROE', '营收增速', '前瞻PE'].includes(label);
                    return (
                      <div key={label} className={`rounded-lg border px-3 py-2 ${highlight ? 'border-accent/30 bg-accent/5' : 'border-line bg-base'}`}>
                        <div className="text-[11px] text-faint">{label}</div>
                        <div className="mt-1 font-mono text-sm font-semibold text-ink">{value}</div>
                      </div>
                    );
                  })}
                </div>
              </section>

              <section className="rounded-xl border border-line bg-surface p-4">
                <div className="flex flex-wrap items-center justify-between gap-4">
                  <div>
                    <h2 className="text-sm font-semibold text-ink">5 方独立评分</h2>
                    <p className="mt-1 text-xs text-muted">五位投资人各按自身框架独立评分,分歧越大越值得关注</p>
                  </div>
                  <div className="flex items-center gap-3">
                    <RadarChart scores={stock.scores} size={72} />
                    <div className="text-right">
                      <div className={`text-lg font-semibold ${verdict.className}`}>{verdict.label}</div>
                      <div className="text-xs text-muted">分歧 {Math.round(divergence)}</div>
                    </div>
                  </div>
                </div>

                <p className="mt-3 text-[11px] leading-relaxed text-faint">
                  AI 方法论模拟 — 以下五方判读由 AI 依据各投资人公开的投资方法论生成,并非本人真实观点、发言或持仓,亦不代表其本人。仅供研究参考,非投资建议。
                </p>

                <div className="mt-4 space-y-3">
                  {scoreMeta.map((item) => {
                    const score = stock.scores[item.key] ?? 0;
                    const narrative =
                      normalizedSymbol === 'NVDA'
                        ? nvdaNarratives[item.key]
                        : genericNarrative(item.label, score, stock.name || normalizedSymbol);
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
                        <p className="mt-2 text-sm leading-relaxed text-ink">{narrative.summary}</p>
                        <p className="mt-2 text-xs leading-relaxed text-muted">{narrative.detail}</p>
                      </article>
                    );
                  })}
                </div>

                <div className="mt-4 rounded-lg border border-line bg-base px-4 py-3">
                  <h3 className="text-xs font-semibold text-ink">分歧焦点</h3>
                  <p className="mt-2 text-xs leading-relaxed text-muted">{archetype?.focus || verdict.note}</p>
                </div>
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
                  <Link to="/" className="text-muted hover:text-ink">← 回热力图</Link>
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
                    <Avatar letter={holder.avatarCode || holder.guruName.charAt(0)} size={40} />
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
            数据 = Nasdaq/Yahoo 快照 + Dataroma 13F / 龙虎榜公开披露。13F 有约 45 天滞后;龙虎榜为交易席位披露。AI 判读为方法论模拟 · 非投资建议。
          </p>
        </div>
      )}
    </div>
  );
};
