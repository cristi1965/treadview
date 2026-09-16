import React, { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { AlertTriangle, Check, ChevronRight, Code2, Copy, Flame, LayoutGrid } from 'lucide-react';
import { PineScriptModal } from './PineScriptModal';
import { StockLogo } from './StockLogo';
import { DataStatus, type DataState } from './common/DataStatus';
import { fetchQuoteResult, startQuotePolling, type QuoteMap } from '../utils/liveQuotes';

export interface KeyStockItem {
  sym: string;
  name: string;
  category: 'M7' | 'CN' | 'SOCIAL' | 'VOLATILE';
  watchReason: string;
  currency?: 'USD' | 'CNY';
}

const FEATURED_STOCKS: KeyStockItem[] = [
  { sym: 'NVDA', name: '英伟达', category: 'M7', watchReason: 'M7 与 AI 算力产业链观察', currency: 'USD' },
  { sym: 'AMZN', name: '亚马逊', category: 'M7', watchReason: 'M7 与云计算资本开支观察', currency: 'USD' },
  { sym: 'MSFT', name: '微软', category: 'M7', watchReason: 'M7 与企业 AI 应用观察', currency: 'USD' },
  { sym: 'TSLA', name: '特斯拉', category: 'M7', watchReason: 'M7 与电动车产业链观察', currency: 'USD' },
  { sym: 'AAPL', name: '苹果', category: 'M7', watchReason: 'M7 与消费电子生态观察', currency: 'USD' },
  { sym: 'META', name: 'Meta', category: 'M7', watchReason: 'M7 与广告平台资本开支观察', currency: 'USD' },
  { sym: 'GOOGL', name: '谷歌', category: 'M7', watchReason: 'M7 与搜索广告生态观察', currency: 'USD' },
  { sym: '300750', name: '宁德时代', category: 'CN', watchReason: 'A 股电池与储能产业链观察', currency: 'CNY' },
  { sym: '300308', name: '中际旭创', category: 'CN', watchReason: 'A 股光通信产业链观察', currency: 'CNY' },
  { sym: '688256', name: '寒武纪', category: 'CN', watchReason: 'A 股国产算力产业链观察', currency: 'CNY' },
  { sym: '601127', name: '赛力斯', category: 'CN', watchReason: 'A 股智能汽车产业链观察', currency: 'CNY' },
  { sym: '300502', name: '新易盛', category: 'CN', watchReason: 'A 股光模块产业链观察', currency: 'CNY' },
  { sym: '600519', name: '贵州茅台', category: 'CN', watchReason: 'A 股消费与机构持仓观察', currency: 'CNY' },
  { sym: 'PLTR', name: 'Palantir', category: 'SOCIAL', watchReason: '社交平台讨论度人工跟踪', currency: 'USD' },
  { sym: 'MSTR', name: 'MicroStrategy', category: 'SOCIAL', watchReason: '数字资产相关性人工跟踪', currency: 'USD' },
  { sym: 'CRWV', name: 'CoreWeave', category: 'SOCIAL', watchReason: 'AI 算力云供应链人工跟踪', currency: 'USD' },
  { sym: 'NBIS', name: 'Nebius Group', category: 'SOCIAL', watchReason: 'AI 基础设施人工跟踪', currency: 'USD' },
  { sym: 'MU', name: '美光科技', category: 'VOLATILE', watchReason: '存储芯片周期人工跟踪', currency: 'USD' },
  { sym: 'AMD', name: '超威半导体', category: 'VOLATILE', watchReason: '服务器芯片竞争人工跟踪', currency: 'USD' },
  { sym: 'ARM', name: '安谋', category: 'VOLATILE', watchReason: '芯片 IP 与估值波动人工跟踪', currency: 'USD' },
  { sym: 'IREN', name: 'Iris Energy', category: 'VOLATILE', watchReason: '数据中心电力需求人工跟踪', currency: 'USD' },
];

export const KeyWatchlistPanel: React.FC = () => {
  const navigate = useNavigate();
  const [tab, setTab] = useState<'ALL' | 'M7' | 'CN' | 'SOCIAL' | 'VOLATILE'>('ALL');
  const [isPineModalOpen, setIsPineModalOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const [mobileExpanded, setMobileExpanded] = useState(false);
  const [liveQuotes, setLiveQuotes] = useState<QuoteMap>({});
  const [quoteStatus, setQuoteStatus] = useState<{
    state: DataState;
    source?: string;
    dataTime?: string;
    message?: string;
  }>({ state: 'loading' });

  const refreshQuotes = useCallback(async () => {
    setQuoteStatus((previous) => ({ ...previous, state: 'loading', message: undefined }));
    try {
      const result = await fetchQuoteResult(FEATURED_STOCKS.map((stock) => stock.sym));
      const hasProvenance = Boolean(result.meta.source && result.meta.dataTime && result.meta.dataTime !== 'unknown');
      const usable = hasProvenance && !result.meta.stale && Object.keys(result.quotes).length > 0;
      const message = [
        result.meta.staleReason,
        ...result.errors,
        result.missing.length ? `缺少 ${result.missing.length} 个报价` : '',
        !hasProvenance ? '报价来源或数据时间不可验证' : '',
      ].filter(Boolean).join('；');
      setLiveQuotes(usable ? result.quotes : {});
      setQuoteStatus({
        state: result.meta.stale ? 'stale' : usable ? 'live' : 'unavailable',
        source: result.meta.source,
        dataTime: result.meta.dataTime,
        message: message || undefined,
      });
    } catch (error) {
      setLiveQuotes({});
      setQuoteStatus({ state: 'error', message: error instanceof Error ? error.message : '报价请求失败' });
    }
  }, []);

  useEffect(() => {
    const stop = startQuotePolling(refreshQuotes, 15_000);
    return () => stop();
  }, [refreshQuotes]);

  const filtered = tab === 'ALL' ? FEATURED_STOCKS : FEATURED_STOCKS.filter((s) => s.category === tab);

  // 生成 TradingView 兼容格式列表并一键复制
  const copyTradingViewList = () => {
    const tvSymbols = filtered
      .map((s) => {
        if (s.currency === 'CNY') {
          if (s.sym.startsWith('6')) return `SSE:${s.sym}`;
          return `SZSE:${s.sym}`;
        }
        return `NASDAQ:${s.sym}`;
      })
      .join(', ');

    void navigator.clipboard.writeText(tvSymbols).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };

  return (
    <div className="rounded-2xl border border-amber-500/20 bg-gradient-to-b from-[#0e1320] to-[#090c14] p-3.5 sm:p-5 shadow-xl">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between border-b border-slate-800/80 pb-3 sm:pb-4">
        <div className="flex items-center gap-2.5 sm:gap-3">
          <div className="flex h-8 w-8 sm:h-9 sm:w-9 items-center justify-center rounded-xl bg-gradient-to-br from-amber-500/20 to-orange-500/20 text-amber-400 border border-amber-500/30 shrink-0">
            <Flame size={18} className="animate-pulse" />
          </div>
          <div>
            <h3 className="flex flex-wrap items-center gap-2 text-sm font-bold text-slate-100 sm:text-base">
              重点关注标的
              <span className="rounded-full bg-amber-500/10 px-2 py-0.5 text-[10px] sm:text-xs text-amber-300 border border-amber-500/20">
                人工观察清单
              </span>
            </h3>
            <p className="text-[11px] sm:text-xs text-slate-400">
              代码、名称和观察理由是人工配置；价格只在来源、数据时间完整且非过期时显示。
            </p>
          </div>
        </div>

        <div className="flex max-w-full flex-wrap items-center gap-1.5 sm:flex-nowrap sm:overflow-x-auto sm:[-webkit-overflow-scrolling:touch] sm:[scrollbar-width:none] sm:[&::-webkit-scrollbar]:hidden">
          <button
            onClick={copyTradingViewList}
            className="shrink-0 flex items-center gap-1 rounded-lg border border-slate-700 bg-slate-800/80 px-2.5 py-1.5 text-[11px] sm:text-xs font-semibold text-slate-300 hover:text-white hover:border-slate-600 transition"
          >
            {copied ? <Check size={12} className="text-emerald-400" /> : <Copy size={12} />}
            <span>{copied ? '已复制代码' : '复制 TV 列表'}</span>
          </button>
          <button
            onClick={() => setIsPineModalOpen(true)}
            className="shrink-0 flex items-center gap-1 rounded-lg border border-indigo-500/40 bg-indigo-500/15 px-2.5 py-1.5 text-[11px] sm:text-xs font-semibold text-indigo-300 hover:bg-indigo-500/25 transition"
          >
            <Code2 size={12} />
            <span>PineScript 指标</span>
          </button>
          <button
            onClick={() => navigate('/arena')}
            className="shrink-0 flex items-center gap-1 rounded-lg border border-emerald-500/40 bg-emerald-500/15 px-2.5 py-1.5 text-[11px] sm:text-xs font-semibold text-emerald-300 hover:bg-emerald-500/25 transition"
          >
            <LayoutGrid size={12} /> 多图盯盘
          </button>
        </div>
      </div>

      <div className="mt-3 flex flex-wrap items-center justify-between gap-2 rounded-lg border border-slate-800 bg-slate-950/50 px-3 py-2">
        <DataStatus
          state={quoteStatus.state}
          label={quoteStatus.state === 'live' ? '关注清单报价可用' : '关注清单报价不可用'}
          source={quoteStatus.source}
          dataTime={quoteStatus.dataTime}
          message={quoteStatus.message}
          onRetry={() => void refreshQuotes()}
          compact
        />
        {quoteStatus.state !== 'live' && (
          <Link to="/settings" className="inline-flex items-center gap-1 text-[11px] font-semibold text-amber-300 hover:text-amber-200">
            <AlertTriangle size={12} /> 查看数据健康与修复入口
          </Link>
        )}
      </div>

      {/* Filter Tabs Row */}
      <div className="mt-3 flex max-w-full flex-wrap items-center gap-1.5 border-b border-slate-800/60 pb-3 text-xs sm:flex-nowrap sm:overflow-x-auto sm:[-webkit-overflow-scrolling:touch] sm:[scrollbar-width:none] sm:[&::-webkit-scrollbar]:hidden">
        <button
          onClick={() => setTab('ALL')}
          className={`shrink-0 rounded-lg px-2.5 sm:px-3 py-1.5 text-[11px] sm:text-xs font-semibold transition ${
            tab === 'ALL'
              ? 'bg-amber-500/20 text-amber-300 border border-amber-500/40 shadow-sm font-bold'
              : 'bg-slate-900/60 text-slate-400 hover:bg-slate-800 hover:text-slate-200'
          }`}
        >
          全部标的 ({FEATURED_STOCKS.length})
        </button>
        <button
          onClick={() => setTab('M7')}
          className={`shrink-0 rounded-lg px-2.5 sm:px-3 py-1.5 text-[11px] sm:text-xs font-semibold transition ${
            tab === 'M7'
              ? 'bg-amber-500/20 text-amber-300 border border-amber-500/40 shadow-sm font-bold'
              : 'bg-slate-900/60 text-slate-400 hover:bg-slate-800 hover:text-slate-200'
          }`}
        >
          👑 美股 M7 巨头
        </button>
        <button
          onClick={() => setTab('CN')}
          className={`shrink-0 rounded-lg px-2.5 sm:px-3 py-1.5 text-[11px] sm:text-xs font-semibold transition ${
            tab === 'CN'
              ? 'bg-red-500/20 text-red-300 border border-red-500/40 shadow-sm font-bold'
              : 'bg-slate-900/60 text-slate-400 hover:bg-slate-800 hover:text-slate-200'
          }`}
        >
          A股观察
        </button>
        <button
          onClick={() => setTab('SOCIAL')}
          className={`shrink-0 rounded-lg px-2.5 sm:px-3 py-1.5 text-[11px] sm:text-xs font-semibold transition ${
            tab === 'SOCIAL'
              ? 'bg-rose-500/20 text-rose-300 border border-rose-500/40 shadow-sm font-bold'
              : 'bg-slate-900/60 text-slate-400 hover:bg-slate-800 hover:text-slate-200'
          }`}
        >
          社交主题观察
        </button>
        <button
          onClick={() => setTab('VOLATILE')}
          className={`shrink-0 rounded-lg px-2.5 sm:px-3 py-1.5 text-[11px] sm:text-xs font-semibold transition ${
            tab === 'VOLATILE'
              ? 'bg-purple-500/20 text-purple-300 border border-purple-500/40 shadow-sm font-bold'
              : 'bg-slate-900/60 text-slate-400 hover:bg-slate-800 hover:text-slate-200'
          }`}
        >
          波动主题观察
        </button>
      </div>

      {/* Stocks Grid */}
      <div className="mt-4 grid grid-cols-1 gap-2.5 sm:gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {filtered.map((item, index) => {
          const live = liveQuotes[item.sym.toUpperCase()];

          return (
            <div
              key={item.sym}
              className={`group relative flex-col justify-between rounded-xl border border-slate-800/90 bg-slate-900/60 p-3.5 hover:border-amber-500/40 hover:bg-slate-900/90 transition shadow-sm ${index >= 6 && !mobileExpanded ? 'hidden sm:flex' : 'flex'}`}
            >
              <div>
                {/* Card Header */}
                <div className="flex items-start justify-between">
                  <div className="flex items-start gap-2">
                    <StockLogo symbol={item.sym} size={32} />
                    <div>
                      <div className="flex items-center gap-1.5">
                        <span className="text-base font-black text-slate-100 group-hover:text-amber-400 transition">
                          {item.sym}
                        </span>
                        <span className="rounded bg-slate-800 px-1.5 py-0.5 text-[10px] text-slate-400 font-medium">
                          {item.name}
                        </span>
                      </div>
                      <p className="mt-1 max-w-[180px] text-[10px] leading-relaxed text-slate-400">{item.watchReason}</p>
                    </div>
                  </div>
                  <div className="flex flex-col items-end">
                    {live ? (
                      <>
                        <span className="text-sm font-bold text-slate-100">
                          {item.currency === 'CNY' ? '¥' : '$'}{live.price.toFixed(2)}
                        </span>
                        <span className={`text-xs font-semibold ${live.pct >= 0 ? 'text-emerald-400' : 'text-rose-400'}`}>
                          {live.pct >= 0 ? '+' : ''}{live.pct.toFixed(2)}%
                        </span>
                      </>
                    ) : (
                      <span className="text-right text-[10px] font-medium text-slate-500">报价不可用</span>
                    )}
                  </div>
                </div>

                <div className="mt-3 rounded-lg border border-slate-800/60 bg-slate-950/60 px-2.5 py-1.5 text-[10px] text-slate-500">
                  人工选入，不包含预设评分或买卖结论
                </div>
              </div>

              {/* Action Bar */}
              <div className="mt-3 flex items-center gap-2 pt-2 border-t border-slate-800/60">
                <button
                  type="button"
                  onClick={() => navigate(`/stock/${item.sym}`)}
                  className="flex flex-1 items-center justify-center gap-1 rounded-lg bg-slate-800 px-2 py-1.5 text-xs font-semibold text-slate-300 hover:text-white hover:bg-slate-700 transition"
                  title="查看详情与 K 线"
                >
                  查看详情 <ChevronRight size={14} />
                </button>
              </div>
            </div>
          );
        })}
      </div>

      {filtered.length > 6 && (
        <button type="button" onClick={() => setMobileExpanded((value) => !value)} className="mt-3 w-full rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-xs font-semibold text-slate-200 sm:hidden">
          {mobileExpanded ? '收起观察列表' : `展开其余 ${filtered.length - 6} 只标的`}
        </button>
      )}

      {/* PineScript 战术指标源码弹窗 */}
      <PineScriptModal
        isOpen={isPineModalOpen}
        onClose={() => setIsPineModalOpen(false)}
      />
    </div>
  );
};
