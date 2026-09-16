import React, { useState } from 'react';
import {
  Activity,
  Flame,
  TrendingUp,
  Percent,
  Sliders,
  BarChart3,
  ShieldAlert,
  Zap,
  Info,
  ArrowUpRight,
  ArrowDownRight,
} from 'lucide-react';

interface FearGreedGaugeProps {
  score: number; // 0 - 100
  label: string;
  vix: number;
  vixChangePct: number;
  upCount: number;
  downCount: number;
  limitUpCount: number;
  limitDownCount: number;
  totalTurnoverB: number;
  northboundFlowB: number;
  turnoverStatus?: string;
  summary?: string;
}

export const FearGreedGauge: React.FC<FearGreedGaugeProps> = ({
  score = 72,
  label = '贪婪情绪高涨',
  vix = 14.85,
  vixChangePct = -3.2,
  upCount = 3948,
  downCount = 1182,
  limitUpCount = 96,
  limitDownCount = 3,
  totalTurnoverB = 25832,
  northboundFlowB = 88.6,
  turnoverStatus = '放量破 2.5 万亿',
  summary = '两市呈现普涨格局，科技主线与高景气出海板块领跑，赚钱效应极强。',
}) => {
  const [activeTab, setActiveTab] = useState<'GAUGE' | 'COMPONENTS'>('GAUGE');

  // Angle for semicircle needle: 0 -> -90deg, 50 -> 0deg, 100 -> +90deg
  const clampedScore = Math.max(0, Math.min(100, score));
  const needleAngle = (clampedScore / 100) * 180 - 90;

  const getScoreColor = (s: number) => {
    if (s >= 75) return '#10b981'; // 极度贪婪
    if (s >= 55) return '#34d399'; // 贪婪
    if (s >= 45) return '#f59e0b'; // 中性
    if (s >= 25) return '#f97316'; // 恐惧
    return '#ef4444'; // 极度恐惧
  };

  const currentColor = getScoreColor(clampedScore);
  const totalCount = upCount + downCount || 5000;
  const upRatio = Math.round((upCount / totalCount) * 100);

  // 辅助衍生情绪指标 (Sentiment Breakdown Components)
  const sentimentComponents = [
    { name: '期权 Put/Call 比率 (PCR)', val: '0.74', state: '多头看涨', note: '衍生品多头占优', isBullish: true },
    { name: '市场最高连板高度', val: '7 连板', state: '打开高度', note: '短线投机接力极强', isBullish: true },
    { name: '涨停炸板率', val: '16.8%', state: '低风险', note: '打板封板承接强劲', isBullish: true },
    { name: '站上 20 日均线比例', val: '76.4%', state: '多头趋势', note: '超 3/4 标的处于上升通道', isBullish: true },
    { name: '破净股 (PB<1) 比例', val: '7.6%', state: '历史底部', note: '低估值安全垫厚', isBullish: true },
    { name: '避险资产 (Gold/美债)', val: '平稳', state: '风险偏好高', note: '资金集中流入权益', isBullish: true },
  ];

  return (
    <div className="flex flex-col justify-between rounded-2xl border border-amber-500/25 bg-gradient-to-b from-[#111625] to-[#0a0d16] p-4 text-slate-100 shadow-xl">
      {/* Header with View Switcher */}
      <div className="flex items-center justify-between border-b border-slate-800 pb-2.5">
        <div className="flex items-center gap-2">
          <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-amber-500/15 text-amber-400 border border-amber-500/30">
            <Activity size={16} className="animate-pulse" />
          </div>
          <span className="text-sm font-bold text-slate-100">市场恐慌与贪婪情绪仪表盘</span>
        </div>

        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={() => setActiveTab('GAUGE')}
            className={`rounded px-2.5 py-0.5 text-[11px] font-semibold transition ${
              activeTab === 'GAUGE' ? 'bg-amber-500/25 text-amber-300 border border-amber-500/40 shadow-sm font-bold' : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            仪表盘
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('COMPONENTS')}
            className={`rounded px-2.5 py-0.5 text-[11px] font-semibold transition ${
              activeTab === 'COMPONENTS' ? 'bg-amber-500/25 text-amber-300 border border-amber-500/40 shadow-sm font-bold' : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            衍生指标
          </button>
        </div>
      </div>

      {activeTab === 'GAUGE' ? (
        <>
          {/* Semicircle Gauge Visual - Clean Speedometer */}
          <div className="relative my-2 flex flex-col items-center justify-center pt-2">
            <svg width="270" height="145" viewBox="0 0 270 145" className="overflow-visible">
              <defs>
                <linearGradient id="gaugeGradientClean" x1="0%" y1="0%" x2="100%" y2="0%">
                  <stop offset="0%" stopColor="#ef4444" />
                  <stop offset="25%" stopColor="#f97316" />
                  <stop offset="50%" stopColor="#eab308" />
                  <stop offset="75%" stopColor="#34d399" />
                  <stop offset="100%" stopColor="#10b981" />
                </linearGradient>
                <filter id="needleDropShadow" x="-20%" y="-20%" width="140%" height="140%">
                  <feDropShadow dx="0" dy="2" stdDeviation="3" floodColor="#000" floodOpacity="0.8" />
                </filter>
              </defs>

              {/* Background Track (Thick & Smooth) */}
              <path
                d="M 30 130 A 105 105 0 0 1 240 130"
                fill="none"
                stroke="#1e293b"
                strokeWidth="16"
                strokeLinecap="round"
              />

              {/* Colored Gradient Active Arc */}
              <path
                d="M 30 130 A 105 105 0 0 1 240 130"
                fill="none"
                stroke="url(#gaugeGradientClean)"
                strokeWidth="16"
                strokeLinecap="round"
              />

              {/* Clear Outside Scale Ticks & Numbers (Placed completely outside the arc) */}
              {/* 0 (Left bottom) */}
              <g transform="translate(15, 134)">
                <text fontSize="11" fontWeight="700" fill="#f87171" textAnchor="middle">0</text>
              </g>

              {/* 25 (Upper left) */}
              <g transform="translate(56, 44)">
                <text fontSize="11" fontWeight="600" fill="#fb923c" textAnchor="middle">25</text>
              </g>

              {/* 50 (Top center) */}
              <g transform="translate(135, 14)">
                <text fontSize="12" fontWeight="700" fill="#fde047" textAnchor="middle">50</text>
              </g>

              {/* 75 (Upper right) */}
              <g transform="translate(214, 44)">
                <text fontSize="11" fontWeight="600" fill="#4ade80" textAnchor="middle">75</text>
              </g>

              {/* 100 (Right bottom) */}
              <g transform="translate(255, 134)">
                <text fontSize="11" fontWeight="700" fill="#34d399" textAnchor="middle">100</text>
              </g>

              {/* Dynamic Needle with Center Hub */}
              <g transform={`translate(135, 130) rotate(${needleAngle})`} filter="url(#needleDropShadow)">
                <polygon points="-4,0 4,0 0,-96" fill={currentColor} />
                <circle cx="0" cy="0" r="9" fill="#0b0f19" stroke={currentColor} strokeWidth="3.5" />
                <circle cx="0" cy="0" r="3.5" fill="#ffffff" />
              </g>
            </svg>

            {/* Large Digital Score & Label Readout */}
            <div className="mt-0.5 text-center">
              <div
                className="flex items-baseline justify-center gap-1 font-mono text-3xl font-black tracking-tight"
                style={{ color: currentColor, textShadow: `0 0 16px ${currentColor}40` }}
              >
                {clampedScore}
                <span className="text-xs text-slate-400 font-normal">/100</span>
              </div>
              <div
                className="mt-1 inline-flex items-center gap-1.5 rounded-full px-3 py-0.5 text-xs font-bold shadow-sm"
                style={{
                  backgroundColor: `${currentColor}20`,
                  color: currentColor,
                  border: `1px solid ${currentColor}40`,
                }}
              >
                <Flame size={13} className="animate-pulse" /> {label}
              </div>
            </div>
          </div>

          {/* 5-Zone High-Contrast Range Bar */}
          <div className="mt-1.5 grid grid-cols-5 gap-1 text-[10px] font-semibold text-center">
            <span
              className={`rounded py-1 transition ${
                clampedScore < 25
                  ? 'bg-rose-500/30 text-rose-300 ring-1 ring-rose-400 font-bold shadow-sm'
                  : 'bg-slate-900/70 text-slate-400'
              }`}
            >
              极恐 0-25
            </span>
            <span
              className={`rounded py-1 transition ${
                clampedScore >= 25 && clampedScore < 45
                  ? 'bg-orange-500/30 text-orange-300 ring-1 ring-orange-400 font-bold shadow-sm'
                  : 'bg-slate-900/70 text-slate-400'
              }`}
            >
              恐慌 25-45
            </span>
            <span
              className={`rounded py-1 transition ${
                clampedScore >= 45 && clampedScore < 55
                  ? 'bg-amber-500/30 text-amber-300 ring-1 ring-amber-400 font-bold shadow-sm'
                  : 'bg-slate-900/70 text-slate-400'
              }`}
            >
              中性 45-55
            </span>
            <span
              className={`rounded py-1 transition ${
                clampedScore >= 55 && clampedScore < 75
                  ? 'bg-emerald-500/30 text-emerald-300 ring-1 ring-emerald-400 font-bold shadow-sm'
                  : 'bg-slate-900/70 text-slate-400'
              }`}
            >
              贪婪 55-75
            </span>
            <span
              className={`rounded py-1 transition ${
                clampedScore >= 75
                  ? 'bg-emerald-400/30 text-emerald-200 ring-1 ring-emerald-300 font-bold shadow-sm'
                  : 'bg-slate-900/70 text-slate-400'
              }`}
            >
              极贪 75-100
            </span>
          </div>

          {/* 4 Key Market Breadth Cards (Clean 2x2 Grid) */}
          <div className="mt-3 grid grid-cols-2 gap-2 text-xs">
            {/* Card 1: 涨跌家数比 + 比例条 */}
            <div className="rounded-xl bg-slate-900/80 p-2.5 border border-slate-800 flex flex-col justify-between">
              <div className="text-[11px] text-slate-400 font-medium">涨跌家数分布</div>
              <div className="mt-1 flex items-center justify-between font-mono font-bold">
                <span className="text-emerald-400 text-sm">↑ {upCount}</span>
                <span className="text-rose-400 text-sm">↓ {downCount}</span>
              </div>
              {/* Mini progress bar */}
              <div className="mt-1.5 flex h-1.5 w-full overflow-hidden rounded-full bg-slate-800">
                <div className="bg-emerald-400 transition-all" style={{ width: `${upRatio}%` }} />
                <div className="bg-rose-400 transition-all" style={{ width: `${100 - upRatio}%` }} />
              </div>
            </div>

            {/* Card 2: 涨停 / 跌停 / 炸板率 */}
            <div className="rounded-xl bg-slate-900/80 p-2.5 border border-slate-800 flex flex-col justify-between">
              <div className="text-[11px] text-slate-400 font-medium">涨跌停与炸板率</div>
              <div className="mt-1 flex items-center gap-1.5">
                <span className="rounded bg-emerald-500/20 px-1.5 py-0.5 font-mono text-xs font-bold text-emerald-300 border border-emerald-500/30">
                  封板 {limitUpCount}
                </span>
                <span className="rounded bg-rose-500/20 px-1.5 py-0.5 font-mono text-xs font-bold text-rose-300 border border-rose-500/30">
                  跌停 {limitDownCount}
                </span>
              </div>
              <div className="mt-1 text-[10px] text-slate-400">炸板率 16.8% (低风险)</div>
            </div>

            {/* Card 3: 两市总成交 */}
            <div className="rounded-xl bg-slate-900/80 p-2.5 border border-slate-800 flex flex-col justify-between">
              <div className="text-[11px] text-slate-400 font-medium">两市总成交额</div>
              <div className="mt-0.5 font-mono text-base font-black text-amber-300">
                {(totalTurnoverB / 10000).toFixed(2)} 万亿
              </div>
              <div className="text-[10px] text-emerald-400 font-medium">{turnoverStatus}</div>
            </div>

            {/* Card 4: 北向外资净买入 */}
            <div className="rounded-xl bg-slate-900/80 p-2.5 border border-slate-800 flex flex-col justify-between">
              <div className="text-[11px] text-slate-400 font-medium">外资北向净流入</div>
              <div className={`mt-0.5 font-mono text-base font-black ${northboundFlowB >= 0 ? 'text-emerald-400' : 'text-rose-400'}`}>
                {northboundFlowB >= 0 ? '+' : ''}{northboundFlowB.toFixed(1)} 亿
              </div>
              <div className="text-[10px] text-slate-400 font-mono">
                VIX: <span className="text-emerald-400 font-semibold">{vix.toFixed(2)}</span>
              </div>
            </div>
          </div>
        </>
      ) : (
        /* 衍生类似指标视图 (Related Sentiment Breakdown Matrix) */
        <div className="my-2 space-y-2 text-xs">
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            {sentimentComponents.map((item) => (
              <div key={item.name} className="rounded-xl bg-slate-900/80 p-2.5 border border-slate-800 flex flex-col justify-between">
                <div className="flex items-center justify-between">
                  <span className="text-[11px] text-slate-300 font-semibold truncate">{item.name}</span>
                  <span className="rounded bg-emerald-500/20 px-1.5 py-0.5 text-[10px] font-bold text-emerald-300 border border-emerald-500/30">
                    {item.state}
                  </span>
                </div>
                <div className="mt-1 flex items-baseline justify-between">
                  <span className="font-mono text-sm font-black text-amber-300">{item.val}</span>
                  <span className="text-[10px] text-slate-400">{item.note}</span>
                </div>
              </div>
            ))}
          </div>

          <div className="rounded-xl bg-slate-950/60 p-2 border border-slate-800/80 flex items-center justify-between text-[11px]">
            <span className="text-slate-400 flex items-center gap-1">
              <VixIcon /> CBOE VIX 恐慌指数:
            </span>
            <span className="font-mono font-bold text-emerald-400">
              {vix.toFixed(2)} ({vixChangePct >= 0 ? '+' : ''}{vixChangePct.toFixed(1)}%)
            </span>
          </div>
        </div>
      )}

      {/* Summary note */}
      <p className="mt-2.5 rounded-lg bg-slate-950/60 p-2 text-[11px] text-slate-300 border border-slate-800/80 leading-relaxed">
        <span className="font-semibold text-amber-400">💡 宏观情绪总评: </span>
        {summary}
      </p>
    </div>
  );
};

const VixIcon = () => (
  <span className="inline-block h-2 w-2 rounded-full bg-emerald-400 animate-ping mr-1" />
);
