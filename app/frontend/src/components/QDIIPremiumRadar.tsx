import React from 'react';
import { DataStatus } from './common';
import { Activity, Info, Bell, BellOff, AlertTriangle, ShieldAlert, Zap, AlertCircle, Volume2 } from 'lucide-react';
import { useAlertsStore } from '../stores/alertsStore';
import { useQDIIPremiums } from '../hooks/useQDIIPremiums';
import type { CompleteQDIIPremiumItem } from '../utils/qdiiPremiums';

export const QDIIPremiumRadar: React.FC = () => {
  const { qdiiAlertsEnabled, setQdiiAlertsEnabled, checkQDIIPremiums } = useAlertsStore();
  const { items, loading, error, meta, trust, refresh } = useQDIIPremiums({ onLive: checkQDIIPremiums });

  const handleToggleAlerts = async () => {
    if (!qdiiAlertsEnabled) {
      if (typeof Notification !== 'undefined' && Notification.permission !== 'granted') {
        const p = await Notification.requestPermission();
        if (p !== 'granted') {
          alert('请在浏览器中允许通知权限，以便在 QDII 出现高溢价风险时第一时间弹出桌面警报！');
        }
      }
      setQdiiAlertsEnabled(true);
      checkQDIIPremiums(items);
    } else {
      setQdiiAlertsEnabled(false);
    }
  };

  if (loading && meta === null) {
    return (
      <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-6 text-center text-xs text-slate-400 mt-4">
        <Activity className="mx-auto mb-2 h-5 w-5 animate-spin text-amber-400" />
        正在载入 QDII 场内 ETF 溢价数据...
      </div>
    );
  }

  if (error || trust.state !== 'live') {
    return (
      <div className="mt-4 rounded-2xl border border-slate-800 bg-[#0B0E17] p-4 sm:p-6">
        <div className="mb-3 flex items-center gap-2 text-sm font-bold text-slate-100">
          <ShieldAlert size={18} className="text-amber-400" /> QDII 溢价雷达
        </div>
        <DataStatus
          state={error ? 'error' : trust.state}
          label={error ? 'QDII 数据加载失败' : trust.state === 'stale' ? 'QDII 数据已过期' : 'QDII 数据不可用'}
          dataTime={meta?.dataTime || 'unknown'}
          source={meta?.source}
          message={error || trust.reason}
          onRetry={() => void refresh()}
        />
        <p className="mt-3 text-xs leading-relaxed text-slate-400">
          价格、净值、溢价率、风险结论和告警已暂停；任一关键字段缺失时不会按 0、低风险或反推价格处理。
        </p>
      </div>
    );
  }

  const getIndexName = (index: string) => {
    if (index === 'nasdaq') return '纳指 (Nasdaq)';
    if (index === 'sp500') return '标普 (S&P 500)';
    if (index === 'dow') return '道指 (Dow)';
    return '其他 (Other)';
  };

  const getColorByLevel = (level: string) => {
    switch (level) {
      case 'extreme': return 'bg-rose-500';
      case 'danger': return 'bg-orange-500';
      case 'caution': return 'bg-amber-500';
      case 'safe':
      default: return 'bg-emerald-500';
    }
  };
  
  const getTextColorByLevel = (level: string) => {
    switch (level) {
      case 'extreme': return 'text-rose-400';
      case 'danger': return 'text-orange-400';
      case 'caution': return 'text-amber-400';
      case 'safe':
      default: return 'text-emerald-400';
    }
  };

  const getActionText = (item: CompleteQDIIPremiumItem) => {
    if (item.premiumPct < 0) return '折价观察';
    if (item.level === 'extreme' || item.level === 'danger') return '高溢价偏离';
    if (item.level === 'caution') return '溢价偏高';
    return '溢价偏离较低';
  };

  const grouped = items.reduce((acc, item) => {
    if (!acc[item.index]) acc[item.index] = [];
    acc[item.index].push(item);
    return acc;
  }, {} as Record<string, CompleteQDIIPremiumItem[]>);
  
  const groups = ['nasdaq', 'sp500', 'dow', 'other'].filter(k => grouped[k] && grouped[k].length > 0);

  // Calculate summary stats
  const nasdaqAvg = grouped['nasdaq'] ? (grouped['nasdaq'].reduce((s, i) => s + i.premiumPct, 0) / grouped['nasdaq'].length).toFixed(1) : '0';
  const sp500Avg = grouped['sp500'] ? (grouped['sp500'].reduce((s, i) => s + i.premiumPct, 0) / grouped['sp500'].length).toFixed(1) : '0';
  const dowAvg = grouped['dow'] ? (grouped['dow'].reduce((s, i) => s + i.premiumPct, 0) / grouped['dow'].length).toFixed(1) : '0';

  // 筛选高危杀溢价标的
  return (
    <div className="rounded-2xl border border-slate-800 bg-[#0B0E17] p-3.5 sm:p-5 shadow-xl space-y-3.5">
      {/* Header Bar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between border-b border-slate-800/80 pb-3 sm:pb-4">
        <div className="flex items-center gap-2.5 sm:gap-3">
          <div className="flex h-8 w-8 sm:h-9 sm:w-9 items-center justify-center rounded-xl bg-gradient-to-br from-amber-500/20 to-orange-500/20 text-amber-400 border border-amber-500/30 shrink-0">
            <ShieldAlert size={18} />
          </div>
          <div>
            <h3 className="text-sm sm:text-base font-bold text-slate-100 flex items-center gap-2">
              QDII 溢价偏离观察
              <div className="group relative flex items-center">
                <Info size={13} className="text-slate-400 cursor-help" />
                <div className="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 hidden w-64 rounded-lg bg-slate-800 p-2.5 text-xs text-slate-300 group-hover:block z-10 shadow-lg border border-slate-700">
                  溢价率 = (场内市价 - 基金实盘净值) / 净值。<br/>
                  溢价率只表示场内价格相对净值的偏离，不预测后续价格。
                </div>
              </div>
            </h3>
            <p className="text-[11px] sm:text-xs text-slate-400 flex items-center gap-2">
              30s 自动刷新 · 关键数据时间: {new Date(meta!.dataTime).toLocaleString()}
              {loading && <Activity size={10} className="animate-spin text-amber-400" />}
            </p>
          </div>
        </div>

        {/* Alert Toggle Action */}
        <div className="flex items-center gap-2 shrink-0">
          <button
            type="button"
            onClick={handleToggleAlerts}
            disabled={trust.state !== 'live'}
            className={`flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-bold transition shadow-sm ${
              trust.state !== 'live'
                ? 'cursor-not-allowed border border-slate-800 bg-slate-900 text-slate-600'
                : qdiiAlertsEnabled
                ? 'border border-emerald-500/40 bg-emerald-500/15 text-emerald-300 hover:bg-emerald-500/25'
                : 'border border-slate-700 bg-slate-800 text-slate-400 hover:text-white'
            }`}
          >
            {trust.state !== 'live' ? (
              <><BellOff size={13} /><span>数据不可用，告警暂停</span></>
            ) : qdiiAlertsEnabled ? (
              <>
                <Bell size={13} className="text-emerald-400 animate-bounce" />
                <span>溢价风控实时监控中</span>
              </>
            ) : (
              <>
                <BellOff size={13} />
                <span>开启高溢价偏离告警</span>
              </>
            )}
          </button>
        </div>
      </div>

      {/* Neutral premium-deviation legend. */}
      <div className="relative overflow-hidden rounded-2xl border-2 border-rose-500/80 bg-gradient-to-r from-rose-950/70 via-slate-950 to-amber-950/60 p-4 sm:p-5 shadow-2xl animate-pulse-border">
        {/* Glow Background */}
        <div className="absolute -right-10 -top-10 h-40 w-40 rounded-full bg-rose-500/10 blur-3xl pointer-events-none" />

        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between border-b border-rose-500/30 pb-3.5 mb-3.5">
          <div className="flex items-center gap-3">
            <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-rose-500/25 text-rose-400 border border-rose-500/50 shadow-lg shrink-0">
              <ShieldAlert size={26} className="animate-bounce" />
            </div>
            <div>
              <h3 className="text-base sm:text-lg font-black text-rose-200 flex items-center gap-2">
                QDII 溢价风险观察
                <span className="rounded-full bg-rose-500/30 px-2.5 py-0.5 text-xs font-bold text-rose-100 border border-rose-400/40">
                  偏离等级参考
                </span>
              </h3>
              <p className="text-xs text-rose-300/90 mt-0.5 font-medium">
                溢价率是场内价格相对净值的偏离，不等于未来收益或操作指令；请核验所示净值来源和时间。
              </p>
            </div>
          </div>
        </div>

        {/* Premium bands describe observed deviation only. */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 text-xs">
          <div className="rounded-xl border border-rose-500/40 bg-rose-950/50 p-3 flex flex-col justify-between space-y-2">
            <div>
              <div className="flex items-center justify-between font-black text-rose-300 text-sm">
                <span>高溢价风险区</span>
                <span className="rounded bg-rose-500/30 px-1.5 py-0.5 text-[10px] font-mono text-rose-200">溢价率 &gt; 5%</span>
              </div>
              <p className="mt-1 text-[11px] text-rose-200/80 leading-relaxed">
                <strong className="text-rose-300">标的类型：</strong>高溢价 QDII ETF (如溢价 8%~12% 的纳指/标普挂钩基金)、高杠杆衍生品。<br/>
                <strong className="text-rose-300">展示边界：</strong>只提示价格偏离，不生成仓位、止损或止盈结论。
              </p>
            </div>
          </div>

          {/* 2. 防守持有区 */}
          <div className="rounded-xl border border-amber-500/40 bg-amber-950/40 p-3 flex flex-col justify-between space-y-2">
            <div>
              <div className="flex items-center justify-between font-black text-amber-300 text-sm">
                <span>中等偏离观察区</span>
                <span className="rounded bg-amber-500/30 px-1.5 py-0.5 text-[10px] font-mono text-amber-200">1.5% - 5%</span>
              </div>
              <p className="mt-1 text-[11px] text-amber-200/80 leading-relaxed">
                <strong className="text-amber-300">观察口径：</strong>数值落在中等偏离区间；仅用于比较同类基金当前偏离，不是收益预测或仓位建议。
              </p>
            </div>
          </div>

          <div className="rounded-xl border border-emerald-500/40 bg-emerald-950/40 p-3 flex flex-col justify-between space-y-2">
            <div>
              <div className="flex items-center justify-between font-black text-emerald-300 text-sm">
                <span>低偏离观察区</span>
                <span className="rounded bg-emerald-500/30 px-1.5 py-0.5 text-[10px] font-mono text-emerald-200">溢价率 &le; 1.5%</span>
              </div>
              <p className="mt-1 text-[11px] text-emerald-200/80 leading-relaxed">
                <strong className="text-emerald-300">观察口径：</strong>数值处于低溢价或折价区间；未包含流动性、申赎限制、追踪误差与个人风险承受能力评估。
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Summary Bar */}
      <div className="rounded-xl border border-slate-800 bg-slate-900/60 p-2.5 sm:p-3 flex flex-wrap items-center gap-2.5 sm:gap-4 text-[11px] sm:text-xs font-medium">
        <div className="flex items-center gap-1.5">
          <span className="text-slate-300">纳指 ETF 均溢</span>
          <span className={`font-bold ${parseFloat(nasdaqAvg) > 5 ? 'text-rose-400' : 'text-amber-400'}`}>{nasdaqAvg}%</span>
          {parseFloat(nasdaqAvg) > 8 && <span className="text-rose-500">🔴 极高</span>}
        </div>
        <div className="w-px h-3.5 bg-slate-700 hidden sm:block"></div>
        <div className="flex items-center gap-1.5">
          <span className="text-slate-300">标普 ETF 均溢</span>
          <span className={`font-bold ${parseFloat(sp500Avg) > 5 ? 'text-rose-400' : 'text-amber-400'}`}>{sp500Avg}%</span>
          {parseFloat(sp500Avg) > 8 && <span className="text-rose-500">🔴</span>}
        </div>
        <div className="w-px h-3.5 bg-slate-700 hidden sm:block"></div>
        <div className="flex items-center gap-1.5">
          <span className="text-slate-300">道指 ETF 均溢</span>
          <span className={`font-bold ${parseFloat(dowAvg) > 5 ? 'text-rose-400' : 'text-amber-400'}`}>{dowAvg}%</span>
          {parseFloat(dowAvg) > 2 && parseFloat(dowAvg) <= 5 && <span className="text-amber-500">🟡</span>}
        </div>
      </div>

      {/* ETF Lists */}
      <div className="space-y-5">
        {groups.map(idx => (
          <div key={idx} className="space-y-2.5">
            <h4 className="text-xs sm:text-sm font-bold text-slate-200 border-l-2 border-indigo-500 pl-2">
              {getIndexName(idx)}
            </h4>
            <div className="grid grid-cols-1 gap-2.5 lg:grid-cols-2">
              {grouped[idx].map(item => (
                <div key={item.code} className="rounded-xl border border-slate-800 bg-slate-900/40 p-3 hover:border-slate-700 transition text-xs flex flex-col gap-2">
                  <div className="flex justify-between items-start">
                    <div>
                      <div className="font-bold text-slate-100 text-sm">{item.name}</div>
                      <div className="text-[10px] font-mono text-slate-400">{item.code}</div>
                    </div>
                    <div className="text-right">
                      <div className="font-mono font-bold text-slate-200">
                        ¥{item.price.toFixed(3)}
                        {item.pricePct == null ? (
                          <span className="ml-1 text-[10px] text-slate-500">涨跌 --</span>
                        ) : (
                          <span className={`ml-1 text-[10px] ${item.pricePct >= 0 ? 'text-emerald-400' : 'text-rose-400'}`}>
                            {item.pricePct >= 0 ? '+' : ''}{item.pricePct.toFixed(2)}%
                          </span>
                        )}
                      </div>
                      <div className="text-[10px] text-slate-400">
                        净值: ¥{item.nav.toFixed(4)} ({item.navDate})
                      </div>
                    </div>
                  </div>
                  
                  <div className="pt-1 mt-1 border-t border-slate-800/60">
                    <div className="flex justify-between items-center mb-1">
                      <span className="text-slate-300">溢价率</span>
                      <span className={`font-mono font-bold ${getTextColorByLevel(item.level)}`}>
                        {item.premiumPct > 0 ? '+' : ''}{item.premiumPct.toFixed(2)}%
                      </span>
                    </div>
                    <div className="h-1.5 w-full rounded-full bg-slate-800 overflow-hidden">
                      <div
                        className={`h-full ${getColorByLevel(item.level)}`}
                        style={{ width: `${Math.min(Math.max((item.premiumPct + 5) * 5, 0), 100)}%` }}
                      />
                    </div>
                    <div className={`mt-1.5 text-right font-medium text-[11px] ${getTextColorByLevel(item.level)}`}>
                      {getActionText(item)}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
