import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { get as apiGet } from '../utils/api';
import { StockLogo } from './StockLogo';
import { PersonAvatar } from './PersonAvatar';
import { Activity, ShieldAlert, Eye, Moon, Mic, RefreshCw, Sparkles, Zap } from 'lucide-react';

interface AnomaliesData {
  updatedAt: string;
  vix: {
    level: number;
    change: number;
    pct: number;
    status: string;
    termStructure: string;
  };
  ivRankTop: {
    sym: string;
    name: string;
    ivRank: number;
    ivPct: number;
    callPutRatio: number;
    expectedMove: string;
    stance: string;
  }[];
  darkPool: {
    netSentiment: number;
    sentimentLabel: string;
    prints: {
      time: string;
      sym: string;
      price: number;
      size: string;
      value: string;
      side: string;
      sideLabel: string;
      venue: string;
    }[];
  };
  nightSessionSpikes: {
    sym: string;
    name: string;
    price: number;
    nightPct: string;
    volMult: string;
    reason: string;
  }[];
  interviewsAndTranscripts: {
    time: string;
    source: string;
    speaker: string;
    title: string;
    summary: string;
    conclusion?: string;
    actionAdvice?: string;
    confidence?: string;
    targetTickers?: string[];
    impact: string;
    sentiment: 'BULLISH' | 'BEARISH' | 'NEUTRAL';
  }[];
}

export const MarketAnomaliesRadar: React.FC = () => {
  const navigate = useNavigate();
  const [data, setData] = useState<AnomaliesData | null>(null);
  const [tab, setTab] = useState<'VIX_IV' | 'DARK_POOL' | 'NIGHT_SPIKES' | 'INTERVIEWS'>('VIX_IV');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retryVersion, setRetryVersion] = useState(0);

  useEffect(() => {
    let active = true;
    (async () => {
      try {
        setLoading(true);
        setError(null);
        const payload = await apiGet<AnomaliesData>('/api/market/anomalies');
        if (active && payload) {
          setData(payload);
          setError(null);
        }
      } catch (err) {
        if (active) {
          setData(null);
          setError(err instanceof Error ? `大盘异动数据暂不可用：${err.message}` : '大盘异动数据暂不可用');
        }
      } finally {
        if (active) setLoading(false);
      }
    })();

    return () => {
      active = false;
    };
  }, [retryVersion]);

  if (loading) {
    return (
      <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-6 text-center text-xs text-slate-400">
        <Activity className="mx-auto mb-2 h-5 w-5 animate-spin text-amber-400" />
        正在载入大盘异动、VIX 波动率与暗盘资金流...
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-6 text-center text-xs text-slate-400">
        <ShieldAlert className="mx-auto mb-2 h-5 w-5 text-amber-400" />
        <p>{error}</p>
		<p className="mt-1 text-[11px] text-slate-500">来源：大盘异动服务 /api/market/anomalies</p>
        <button type="button" onClick={() => setRetryVersion((value) => value + 1)} className="mx-auto mt-3 inline-flex items-center gap-1.5 rounded-md border border-slate-700 px-3 py-2 font-semibold text-amber-300 hover:bg-slate-800">
          <RefreshCw className="h-3.5 w-3.5" />重试异动数据
        </button>
      </div>
    );
  }

  if (!data) {
    return (
      <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-6 text-center text-xs text-slate-400">
        <ShieldAlert className="mx-auto mb-2 h-5 w-5 text-amber-400" />
        <p>大盘异动数据暂不可用</p>
		<p className="mt-1 text-[11px] text-slate-500">来源：大盘异动服务 /api/market/anomalies</p>
        <button type="button" onClick={() => setRetryVersion((value) => value + 1)} className="mx-auto mt-3 inline-flex items-center gap-1.5 rounded-md border border-slate-700 px-3 py-2 font-semibold text-amber-300 hover:bg-slate-800">
          <RefreshCw className="h-3.5 w-3.5" />重试异动数据
        </button>
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-indigo-500/20 bg-gradient-to-b from-[#0b0e17] to-[#070a12] p-3.5 sm:p-5 shadow-xl">
      {/* Header Bar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between border-b border-slate-800/80 pb-3 sm:pb-4">
        <div className="flex items-center gap-2.5 sm:gap-3">
          <div className="flex h-8 w-8 sm:h-9 sm:w-9 items-center justify-center rounded-xl bg-gradient-to-br from-indigo-500/20 to-purple-500/20 text-indigo-400 border border-indigo-500/30 shrink-0">
            <Activity size={18} className="animate-pulse" />
          </div>
          <div>
            <h3 className="flex flex-wrap items-center gap-2 text-sm font-bold text-slate-100 sm:text-base">
              大盘异动与衍生品/资金流看板
              <span className="rounded-full bg-indigo-500/10 px-2 py-0.5 text-[10px] sm:text-xs text-indigo-300 border border-indigo-500/20">
                VIX · 暗盘 · 夜盘 · 采访
              </span>
            </h3>
            <p className="text-[11px] sm:text-xs text-slate-400">
              参照 Unusual Whales, MarketChameleon & OpenBB 构建的大盘风险与资金雷达
            </p>
          </div>
        </div>

        {/* Tab Filters (Horizontal Scroll on Mobile) */}
        <div className="flex max-w-full flex-wrap items-center gap-1 rounded-xl border border-slate-800 bg-slate-900/90 p-1 text-xs sm:flex-nowrap sm:overflow-x-auto sm:[-webkit-overflow-scrolling:touch] sm:[scrollbar-width:none] sm:[&::-webkit-scrollbar]:hidden">
          <button
            onClick={() => setTab('VIX_IV')}
            className={`shrink-0 flex items-center gap-1.5 rounded-lg px-2.5 sm:px-3 py-1.5 text-[11px] sm:text-xs font-semibold transition ${
              tab === 'VIX_IV'
                ? 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/40 shadow-sm font-bold'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <ShieldAlert size={12} /> VIX & 期权 IV
          </button>
          <button
            onClick={() => setTab('DARK_POOL')}
            className={`shrink-0 flex items-center gap-1.5 rounded-lg px-2.5 sm:px-3 py-1.5 text-[11px] sm:text-xs font-semibold transition ${
              tab === 'DARK_POOL'
                ? 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/40 shadow-sm font-bold'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Eye size={12} /> 暗盘大单 Flow
          </button>
          <button
            onClick={() => setTab('NIGHT_SPIKES')}
            className={`shrink-0 flex items-center gap-1.5 rounded-lg px-2.5 sm:px-3 py-1.5 text-[11px] sm:text-xs font-semibold transition ${
              tab === 'NIGHT_SPIKES'
                ? 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/40 shadow-sm font-bold'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Moon size={12} /> 夜盘/盘前异动
          </button>
          <button
            onClick={() => setTab('INTERVIEWS')}
            className={`shrink-0 flex items-center gap-1.5 rounded-lg px-2.5 sm:px-3 py-1.5 text-[11px] sm:text-xs font-semibold transition ${
              tab === 'INTERVIEWS'
                ? 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/40 shadow-sm font-bold'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Mic size={12} /> CEO/联储速递
          </button>
        </div>
      </div>

      {/* Tab 1: VIX & Options IV Radar */}
      {tab === 'VIX_IV' && (
        <div className="mt-4 space-y-4">
          {/* VIX Banner */}
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between rounded-xl border border-indigo-500/30 bg-indigo-950/20 p-4">
            <div className="flex items-center gap-3">
              <div className="text-2xl font-black text-slate-100">
                VIX <span className="text-xl text-indigo-400">{data.vix.level.toFixed(2)}</span>
              </div>
              <span
                className={`rounded px-2 py-0.5 text-xs font-bold ${
                  data.vix.change <= 0 ? 'bg-emerald-500/20 text-emerald-300' : 'bg-rose-500/20 text-rose-300'
                }`}
              >
                {data.vix.change >= 0 ? '+' : ''}
                {data.vix.change.toFixed(2)} ({data.vix.pct.toFixed(2)}%)
              </span>
            </div>
            <div className="text-xs text-slate-300 space-y-0.5 text-left sm:text-right">
              <div className="font-semibold text-indigo-300">{data.vix.status}</div>
              <div className="text-slate-400 text-[11px]">{data.vix.termStructure}</div>
            </div>
          </div>

          {/* Top IV Rank Stocks Grid */}
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {data.ivRankTop.map((item) => (
              <div
                key={item.sym}
                className="rounded-xl border border-slate-800 bg-slate-900/60 p-3.5 hover:border-indigo-500/30 transition text-xs space-y-2"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <StockLogo symbol={item.sym} size={28} />
                    <div>
                      <div className="font-bold text-slate-100 text-sm leading-tight">{item.sym}</div>
                      <div className="text-[10px] text-slate-400">{item.name}</div>
                    </div>
                  </div>
                  <span className="rounded bg-indigo-500/20 px-2 py-0.5 text-[11px] font-bold text-indigo-300 border border-indigo-500/30">
                    IV Rank: {item.ivRank}%
                  </span>
                </div>

                {/* Progress bar */}
                <div className="space-y-1">
                  <div className="flex justify-between text-[11px] text-slate-400">
                    <span>IV Percentile ({item.ivPct}%)</span>
                    <span>预期波动: {item.expectedMove}</span>
                  </div>
                  <div className="h-1.5 w-full rounded-full bg-slate-800 overflow-hidden">
                    <div
                      className="h-full bg-gradient-to-r from-indigo-500 to-purple-500"
                      style={{ width: `${item.ivPct}%` }}
                    />
                  </div>
                </div>

                <div className="flex items-center justify-between text-[11px] pt-1 border-t border-slate-800/60">
                  <span className="text-slate-400">Call/Put 比率: <strong className="text-emerald-400">{item.callPutRatio}</strong></span>
                  <span className="text-indigo-300 font-medium">{item.stance}</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Tab 2: Dark Pool Block Prints */}
      {tab === 'DARK_POOL' && (
        <div className="mt-4 space-y-4">
          {/* Sentiment Bar */}
          <div className="rounded-xl border border-slate-800 bg-slate-900/70 p-4 space-y-2">
            <div className="flex items-center justify-between text-xs font-semibold text-slate-200">
              <span className="flex items-center gap-1.5">
                <Eye size={14} className="text-indigo-400" /> 暗盘机构资金流倾向
              </span>
              <span className="text-emerald-400">{data.darkPool.sentimentLabel}</span>
            </div>
            <div className="h-2 w-full rounded-full bg-slate-800 overflow-hidden flex">
              <div
                className="h-full bg-emerald-500 transition-all duration-500"
                style={{ width: `${data.darkPool.netSentiment}%` }}
              />
              <div
                className="h-full bg-rose-500 transition-all duration-500"
                style={{ width: `${100 - data.darkPool.netSentiment}%` }}
              />
            </div>
          </div>

          {/* Prints Table */}
          <div className="overflow-x-auto rounded-xl border border-slate-800 bg-slate-900/40">
            <table className="w-full text-left text-xs text-slate-300">
              <thead className="bg-slate-900/90 text-slate-400 font-semibold border-b border-slate-800">
                <tr>
                  <th className="p-3">成交时间</th>
                  <th className="p-3">股票标的</th>
                  <th className="p-3">暗盘成交价</th>
                  <th className="p-3">成交股数</th>
                  <th className="p-3">总涉及金额</th>
                  <th className="p-3">机构资金方向</th>
                  <th className="p-3">交易场所</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {data.darkPool.prints.map((p, idx) => (
                  <tr key={idx} className="hover:bg-slate-800/40 transition">
                    <td className="p-3 font-mono text-slate-400">{p.time}</td>
                    <td className="p-3 font-bold text-slate-100 flex items-center gap-2">
                      <StockLogo symbol={p.sym} size={22} />
                      {p.sym}
                    </td>
                    <td className="p-3 font-mono">${p.price.toFixed(2)}</td>
                    <td className="p-3 font-mono text-slate-300">{p.size}</td>
                    <td className="p-3 font-mono font-bold text-amber-300">{p.value}</td>
                    <td className="p-3">
                      <span
                        className={`rounded px-2 py-0.5 text-[11px] font-semibold ${
                          p.side.includes('BULLISH')
                            ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                            : 'bg-rose-500/20 text-rose-300 border border-rose-500/30'
                        }`}
                      >
                        {p.sideLabel}
                      </span>
                    </td>
                    <td className="p-3 text-slate-400 text-[11px]">{p.venue}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Tab 3: Night Session & Premarket Spikes */}
      {tab === 'NIGHT_SPIKES' && (
        <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
          {data.nightSessionSpikes.map((item) => (
            <div
              key={item.sym}
              className="rounded-xl border border-slate-800 bg-slate-900/60 p-4 space-y-2 hover:border-indigo-500/30 transition text-xs"
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <StockLogo symbol={item.sym} size={28} />
                  <div>
                    <span className="text-base font-bold text-slate-100">{item.sym}</span>
                    <span className="text-slate-400 ml-1.5">{item.name}</span>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <span className="font-mono text-sm font-bold text-slate-100">${item.price.toFixed(2)}</span>
                  <span
                    className={`rounded px-2 py-0.5 font-bold ${
                      item.nightPct.startsWith('+')
                        ? 'bg-emerald-500/20 text-emerald-300'
                        : 'bg-rose-500/20 text-rose-300'
                    }`}
                  >
                    {item.nightPct}
                  </span>
                </div>
              </div>

              <div className="flex items-center gap-2">
                <span className="rounded bg-purple-500/20 px-2 py-0.5 text-[11px] font-bold text-purple-300 border border-purple-500/30">
                  ⚡ {item.volMult}
                </span>
                <span className="text-slate-300">{item.reason}</span>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Tab 4: CEO / Fed Interviews & Transcripts */}
      {tab === 'INTERVIEWS' && (
        <div className="mt-4 space-y-4">
          {data.interviewsAndTranscripts.map((item, idx) => (
            <div
              key={idx}
              className="rounded-xl border border-slate-800 bg-slate-900/70 p-4 space-y-3 hover:border-indigo-500/40 transition text-xs shadow-md"
            >
              {/* Header with Avatar & Tags */}
              <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between border-b border-slate-800/80 pb-2.5">
                <div className="flex items-center gap-2.5">
                  <PersonAvatar
                    name={item.speaker}
                    size={38}
                    role={item.source.includes('Fed') || item.speaker.includes('古尔斯比') ? 'FED' : 'CEO'}
                  />
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-bold text-slate-100 text-sm">{item.speaker}</span>
                      <span className="rounded bg-indigo-500/20 px-2 py-0.5 text-[10px] font-bold text-indigo-300 border border-indigo-500/30">
                        {item.source}
                      </span>
                      {item.confidence && (
                        <span className="rounded bg-slate-800 px-1.5 py-0.5 text-[10px] text-amber-300 font-mono">
                          ⚡ {item.confidence}
                        </span>
                      )}
                    </div>
                    <span className="text-slate-400 font-mono text-[10px]">{item.time}</span>
                  </div>
                </div>

                <span
                  className={`rounded-lg px-2.5 py-1 font-bold text-xs self-start sm:self-auto ${
                    item.sentiment === 'BULLISH'
                      ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                      : 'bg-rose-500/20 text-rose-300 border border-rose-500/30'
                  }`}
                >
                  {item.sentiment === 'BULLISH' ? '🟢 AI 判读 · 强共振利好' : '🔴 AI 判读 · 谨慎防守'}
                </span>
              </div>

              {/* Title & Speech Summary */}
              <div>
                <h4 className="text-sm font-bold text-slate-100 leading-snug">{item.title}</h4>
                <p className="mt-1 text-slate-300 leading-relaxed text-xs">{item.summary}</p>
              </div>

              {/* Actionable Conclusion & Advice Box */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-2.5 pt-1">
                {/* 核心结论 */}
                <div className="rounded-lg border border-indigo-500/30 bg-indigo-950/30 p-2.5">
                  <div className="text-[11px] font-bold text-indigo-300 flex items-center gap-1 mb-1">
                    <Sparkles size={12} className="text-indigo-400" /> AI 核心结论与定调
                  </div>
                  <p className="text-xs text-slate-200 font-medium leading-relaxed">
                    {item.conclusion || '当前来源未提供结论，已隐藏。'}
                  </p>
                </div>

                {/* 操盘与行动建议 */}
                <div className="rounded-lg border border-amber-500/30 bg-amber-950/20 p-2.5">
                  <div className="text-[11px] font-bold text-amber-300 flex items-center gap-1 mb-1">
                    <Zap size={12} className="text-amber-400" /> 操盘策略与行动建议
                  </div>
                  <p className="text-xs text-amber-100 font-medium leading-relaxed">
                    {item.actionAdvice || '当前来源未提供操作建议，已隐藏。'}
                  </p>
                </div>
              </div>

              {/* Target Tickers / Impact Row */}
              <div className="flex flex-wrap items-center justify-between gap-2 pt-2 border-t border-slate-800/60 text-xs">
                <div className="flex items-center gap-1.5 text-slate-400">
                  <span className="font-semibold text-slate-300">🎯 关联/影响标的:</span>
                  <span className="text-amber-300">{item.impact}</span>
                </div>

                {item.targetTickers && item.targetTickers.length > 0 && (
                  <div className="flex items-center gap-1.5">
                    {item.targetTickers.map((sym) => (
                      <button
                        key={sym}
                        onClick={() => navigate(sym.startsWith('300') || sym.startsWith('600') ? `/scan?market=cn&q=${sym}` : `/stock/${sym}`)}
                        className="flex items-center gap-1 rounded bg-slate-800/80 px-2 py-0.5 text-[11px] font-bold text-slate-200 hover:bg-slate-700 hover:text-white border border-slate-700/60 transition"
                      >
                        <StockLogo symbol={sym} size={16} />
                        {sym}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
