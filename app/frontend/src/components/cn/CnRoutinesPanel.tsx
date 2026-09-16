import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { DataStatus } from '../common';
import { APIError, getWithMeta, type APIResponseMeta } from '../../utils/api';
import { assessDataTrust } from '../../utils/dataTrust';
import { useAlertsStore } from '../../stores/alertsStore';

interface Profile {
  id?: string;
  name?: string;
  audience?: string;
  cash_floor_pct?: number;
  core_etf_target_pct?: number;
  satellite_max_pct?: number;
  single_name_max_pct?: number;
  max_deploy_per_week_pct?: number;
  rules?: string[];
}

interface Level {
  pct: number;
  price: number;
  note: string;
}

interface DipSignal {
  level: 'yellow' | 'orange' | 'red';
  label: string;
  action: string;
}

interface DipLine {
  level: 'yellow' | 'orange' | 'red';
  label: string;
  action: string;
  day_pct: number;
  price: number;
  gap: number;
  triggered: boolean;
}

interface SymEval {
  symbol: string;
  name: string;
  role: string;
  weight_hint?: number;
  price?: number;
  pct?: number;
  heat?: number;
  hasQuote?: boolean;
  action: string;
  reason: string;
  buy_levels?: Level[];
  sell_levels?: Level[];
  stop_price?: number;
  dip_signal?: DipSignal;
  dip_lines?: DipLine[];
}

interface RoutineEval {
  id: string;
  name: string;
  style: string;
  horizon: string;
  risk: string;
  why: string;
  playbook?: string[];
  today_bias: string;
  today_summary: string;
  dip_level?: string;
  dip_summary?: string;
  symbols: SymEval[];
}

interface RoutinesResponse {
  disclaimer?: string;
  updated?: string;
  configUpdated?: string;
  profile?: Profile;
  routines?: RoutineEval[];
  today?: { note?: string; actions?: Record<string, number> };
}

const biasLabel: Record<string, string> = {
  buy: '买入窗口',
  trim: '减仓窗口',
  mixed: '分化',
  hold: '持有/定投',
  wait: '观望',
};

const biasClass: Record<string, string> = {
  buy: 'bg-accent/15 text-accent',
  trim: 'bg-down/15 text-down',
  mixed: 'bg-surface-3 text-ink',
  hold: 'bg-surface-2 text-muted',
  wait: 'bg-surface-2 text-faint',
};

const actionClass: Record<string, string> = {
  buy: 'text-accent',
  trim: 'text-down',
  hold: 'text-muted',
  wait: 'text-faint',
};

const dipBadge: Record<string, { bg: string; text: string; border: string; icon: string }> = {
  yellow: { bg: 'bg-yellow-500/10', text: 'text-yellow-600 dark:text-yellow-400', border: 'border-yellow-500/25', icon: '⚠' },
  orange: { bg: 'bg-orange-500/10', text: 'text-orange-600 dark:text-orange-400', border: 'border-orange-500/25', icon: '⚠' },
  red:    { bg: 'bg-down/15', text: 'text-down', border: 'border-down/25', icon: '🔴' },
};

const pctText = (pct?: number) => {
  if (pct == null) return '—';
  return `${pct >= 0 ? '+' : ''}${pct.toFixed(2)}%`;
};

export const CnRoutinesPanel: React.FC = () => {
  const [data, setData] = useState<RoutinesResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [meta, setMeta] = useState<APIResponseMeta | null>(null);
  const [loading, setLoading] = useState(true);
  const [openId, setOpenId] = useState<string | null>(null);
  const [toast, setToast] = useState<string | null>(null);
  const addAlert = useAlertsStore((s) => s.addAlert);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await getWithMeta<RoutinesResponse>('/api/cn/routines');
      setData(result.data);
      setMeta(result.meta);
      setOpenId((prev) => prev || result.data.routines?.[0]?.id || null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'A 股日课加载失败');
      setMeta(err instanceof APIError ? err.meta || null : null);
      setData(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  useEffect(() => {
    if (!toast) return;
    const t = window.setTimeout(() => setToast(null), 2200);
    return () => window.clearTimeout(t);
  }, [toast]);

  if (loading && !data) {
    return <div className="rounded-xl border border-line bg-surface px-3 py-6 text-center text-xs text-faint">加载 A 股日课…</div>;
  }

  const trust = assessDataTrust(meta, Boolean(data?.routines?.length));
  if (error || !data) {
    return (
      <div className="rounded-xl border border-line bg-surface px-4 py-4">
        <h2 className="mb-2 text-sm font-semibold text-ink">A 股日课 · 数据待核验</h2>
        <DataStatus
          state="error"
          label="A 股日课加载失败"
          dataTime={meta?.dataTime || 'unknown'}
          source={meta?.source}
          message={error || trust.reason}
          onRetry={() => void load()}
        />
      </div>
    );
  }

  const profile = data.profile;
  const routines = data.routines || [];

  if (trust.state !== 'live') {
    return (
      <div className="space-y-4">
        <section className="rounded-xl border border-amber-500/25 bg-surface px-4 py-4">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 className="text-sm font-semibold text-ink">A 股日课 · 规则模式</h2>
              <p className="mt-1 text-xs leading-relaxed text-muted">报价未通过门禁；仅展示静态仓位约束和方法说明，不显示价格、今日动作、买卖点或提醒。</p>
            </div>
            <button type="button" onClick={() => void load()} disabled={loading} className="rounded-md border border-line px-2.5 py-1.5 text-xs text-muted hover:bg-surface-2 hover:text-ink">
              {loading ? '刷新中…' : '重试实时报价'}
            </button>
          </div>
          <div className="mt-3">
            <DataStatus state={trust.state} label={trust.state === 'stale' ? '实时报价已过期' : '实时报价不可用'} dataTime={meta?.dataTime || 'unknown'} source={meta?.source} message={trust.reason} />
          </div>
        </section>

        {profile ? (
          <section className="border-y border-line py-4">
            <div className="flex flex-wrap items-baseline justify-between gap-2">
              <h3 className="text-sm font-semibold text-ink">{profile.name || '稳健配置'} · 仓位约束</h3>
              <span className="text-[11px] text-faint">{profile.audience}</span>
            </div>
            <div className="mt-3 grid grid-cols-2 gap-x-5 gap-y-2 text-xs sm:grid-cols-4">
              <div><span className="text-faint">现金底线</span><strong className="ml-2 font-mono text-ink">≥{profile.cash_floor_pct ?? '—'}%</strong></div>
              <div><span className="text-faint">核心 ETF</span><strong className="ml-2 font-mono text-ink">{profile.core_etf_target_pct ?? '—'}%</strong></div>
              <div><span className="text-faint">卫星仓上限</span><strong className="ml-2 font-mono text-ink">≤{profile.satellite_max_pct ?? '—'}%</strong></div>
              <div><span className="text-faint">单标的上限</span><strong className="ml-2 font-mono text-ink">≤{profile.single_name_max_pct ?? '—'}%</strong></div>
            </div>
            <ul className="mt-3 grid gap-2 text-xs leading-relaxed text-muted md:grid-cols-2">
              {(profile?.rules || []).map((rule) => <li key={rule} className="border-l-2 border-accent/50 pl-2.5">{rule}</li>)}
            </ul>
          </section>
        ) : null}

        <div className="grid gap-3 lg:grid-cols-2">
          {routines.map((routine) => (
            <section key={routine.id} className="rounded-xl border border-line bg-surface p-4">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h3 className="text-sm font-semibold text-ink">{routine.name}</h3>
                <span className="font-mono text-[10px] text-faint">{routine.style} · {routine.horizon} · 风险 {routine.risk}</span>
              </div>
              <p className="mt-2 text-xs leading-relaxed text-muted">{routine.why}</p>
              <div className="mt-3 flex flex-wrap gap-1.5">
                {routine.symbols.map((symbol) => <span key={symbol.symbol} className="rounded-md bg-surface-2 px-2 py-1 font-mono text-[10px] text-faint">{symbol.symbol} {symbol.name}</span>)}
              </div>
              {(routine.playbook || []).length > 0 ? <ul className="mt-3 space-y-1 text-[11px] leading-relaxed text-muted">{routine.playbook!.map((item) => <li key={item}>• {item}</li>)}</ul> : null}
            </section>
          ))}
        </div>
      </div>
    );
  }

  const armLevels = (s: SymEval) => {
    let n = 0;
    for (const lv of s.buy_levels || []) {
      if (addAlert(s.symbol, '<=', lv.price, 'cn')) n += 1;
    }
    for (const lv of s.sell_levels || []) {
      if (addAlert(s.symbol, '>=', lv.price, 'cn')) n += 1;
    }
    if (s.stop_price != null && addAlert(s.symbol, '<=', s.stop_price, 'cn')) n += 1;
    setToast(n > 0 ? `已挂 ${n} 条价格提醒（重复已跳过）` : '提醒已存在，未重复添加');
  };

  const armDipAlerts = (s: SymEval) => {
    let n = 0;
    for (const dl of s.dip_lines || []) {
      if (!dl.triggered && addAlert(s.symbol, '<=', dl.price, 'cn')) n += 1;
    }
    setToast(n > 0 ? `已挂 ${n} 条跳水预警提醒` : '跳水提醒已存在，未重复添加');
  };

  const armAllDipAlerts = (r: RoutineEval) => {
    let n = 0;
    for (const s of r.symbols) {
      for (const dl of s.dip_lines || []) {
        if (!dl.triggered && addAlert(s.symbol, '<=', dl.price, 'cn')) n += 1;
      }
    }
    setToast(n > 0 ? `已挂 ${n} 条跳水预警（全套路）` : '所有跳水提醒已存在');
  };

  return (
    <div className="relative space-y-4">
      {toast ? (
        <div className="pointer-events-none absolute right-2 top-2 z-10 rounded-md border border-line bg-surface px-2.5 py-1.5 text-[11px] text-ink shadow">
          {toast}
        </div>
      ) : null}
      <div className="rounded-xl border border-line bg-surface px-4 py-3">
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <h2 className="text-sm font-semibold text-ink">{profile?.name || '稳健跟上时代'} · 买卖点套路</h2>
          <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto sm:justify-end">
            <DataStatus state="live" dataTime={meta?.dataTime} compact />
            <span className="font-mono text-[10px] text-faint">机械网格 · 非稳赚</span>
            <button
              type="button"
              className="shrink-0 whitespace-nowrap rounded-md border border-line px-2 py-0.5 text-[10px] text-muted hover:bg-surface-2 hover:text-ink"
              onClick={() => void load()}
              disabled={loading}
            >
              {loading ? '刷新中…' : '刷新'}
            </button>
          </div>
        </div>
        <p className="mt-1 text-[11px] leading-relaxed text-muted">{data.disclaimer}</p>
        <p className="mt-1 break-all font-mono text-[9px] leading-relaxed text-faint">
          来源 {meta?.source || '未知'} · 规则版本 {data.configUpdated || data.updated || '未知'}
        </p>
        {profile ? (
          <div className="mt-3 grid gap-2 sm:grid-cols-4">
            <div className="rounded-lg bg-base px-2.5 py-2">
              <div className="text-[10px] text-faint">现金缓冲 ≥</div>
              <div className="font-mono text-sm text-ink">{profile.cash_floor_pct}%</div>
            </div>
            <div className="rounded-lg bg-base px-2.5 py-2">
              <div className="text-[10px] text-faint">宽基目标</div>
              <div className="font-mono text-sm text-ink">{profile.core_etf_target_pct}%</div>
            </div>
            <div className="rounded-lg bg-base px-2.5 py-2">
              <div className="text-[10px] text-faint">主题卫星 ≤</div>
              <div className="font-mono text-sm text-ink">{profile.satellite_max_pct}%</div>
            </div>
            <div className="rounded-lg bg-base px-2.5 py-2">
              <div className="text-[10px] text-faint">每周最多投入</div>
              <div className="font-mono text-sm text-ink">{profile.max_deploy_per_week_pct}%</div>
            </div>
          </div>
        ) : null}
        {profile?.rules?.length ? (
          <ul className="mt-3 space-y-1 text-[11px] leading-relaxed text-muted">
            {profile.rules.map((r) => (
              <li key={r}>· {r}</li>
            ))}
          </ul>
        ) : null}
        {data.today?.note ? <p className="mt-2 text-[10px] text-faint">{data.today.note}</p> : null}
      </div>

      <div className="space-y-3">
        {routines.map((r) => {
          const open = openId === r.id;
          return (
            <section key={r.id} className="overflow-hidden rounded-xl border border-line bg-surface">
              <button
                type="button"
                className="flex w-full items-start gap-3 px-4 py-3 text-left hover:bg-surface-2/60"
                onClick={() => setOpenId(open ? null : r.id)}
              >
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-sm font-semibold text-ink">{r.name}</span>
                    <span className={`rounded px-1.5 py-0.5 text-[10px] font-medium ${biasClass[r.today_bias] || biasClass.wait}`}>
                      {biasLabel[r.today_bias] || r.today_bias}
                    </span>
                    {r.dip_level && dipBadge[r.dip_level] ? (
                      <span className={`rounded border px-1.5 py-0.5 text-[10px] font-medium ${dipBadge[r.dip_level].bg} ${dipBadge[r.dip_level].text} ${dipBadge[r.dip_level].border}`}>
                        {dipBadge[r.dip_level].icon} 跳水{r.dip_level === 'red' ? '警报' : '预警'}
                      </span>
                    ) : null}
                    <span className="rounded bg-surface-3 px-1.5 py-0.5 text-[10px] text-faint">{r.style}</span>
                    <span className="rounded bg-surface-3 px-1.5 py-0.5 text-[10px] text-faint">风险 {r.risk}</span>
                  </div>
                  <p className="mt-1 text-[11px] text-muted">{r.today_summary}</p>
                </div>
                <span className="shrink-0 font-mono text-[10px] text-faint">{open ? '收起' : '展开'}</span>
              </button>

              {open ? (
                <div className="space-y-3 border-t border-line px-4 py-3">
                  <p className="text-[11px] leading-relaxed text-muted">{r.why}</p>
                  <div className="text-[10px] text-faint">周期 {r.horizon}</div>
                  {r.playbook?.length ? (
                    <ul className="space-y-0.5 text-[11px] text-muted">
                      {r.playbook.map((p) => (
                        <li key={p}>· {p}</li>
                      ))}
                    </ul>
                  ) : null}

                  {(() => {
                    const hasDipLines = r.symbols.some((s) => s.dip_lines?.length);
                    const activeDip = r.dip_level && r.dip_summary;
                    if (!hasDipLines && !activeDip) return null;
                    return (
                      <div className={`rounded-lg border p-3 ${activeDip && dipBadge[r.dip_level!] ? `${dipBadge[r.dip_level!].bg} ${dipBadge[r.dip_level!].border}` : 'bg-surface-2 border-line'}`}>
                        <div className="flex items-center justify-between">
                          <div className={`text-xs font-semibold ${activeDip && dipBadge[r.dip_level!] ? dipBadge[r.dip_level!].text : 'text-ink'}`}>
                            {activeDip ? `${dipBadge[r.dip_level!]?.icon} 跳水预警 · ${r.dip_level!.toUpperCase()}` : '⛑ 跳水预警线'}
                          </div>
                          <button
                            type="button"
                            className="rounded-md border border-line px-2 py-0.5 text-[10px] text-muted hover:bg-surface-2 hover:text-ink"
                            onClick={() => armAllDipAlerts(r)}
                          >
                            一键设全部跳水预警
                          </button>
                        </div>
                        {activeDip ? <p className="mt-1 text-[11px] leading-relaxed text-ink/80">{r.dip_summary}</p> : null}
                      </div>
                    );
                  })()}

                  <div className="space-y-3">
                    {r.symbols.map((s) => (
                      <div key={s.symbol} className="rounded-lg border border-line/80 bg-base px-3 py-2.5">
                        <div className="flex flex-wrap items-center gap-2">
                          <Link to={`/stock/${s.symbol}`} className="font-mono text-xs font-semibold text-accent hover:underline">
                            {s.symbol}
                          </Link>
                          <span className="text-xs text-ink">{s.name}</span>
                          <span className="rounded bg-surface-3 px-1.5 py-0.5 text-[10px] text-faint">{s.role}</span>
                          {s.weight_hint != null ? (
                            <span className="font-mono text-[10px] text-faint">建议仓 ≤{s.weight_hint}%</span>
                          ) : null}
                          {s.hasQuote ? (
                            <span className="ml-auto font-mono text-[11px] tabular-nums text-muted">
                              {s.price} · {pctText(s.pct)}
                            </span>
                          ) : (
                            <span className="ml-auto text-[10px] text-faint">无报价</span>
                          )}
                        </div>
                        <div className={`mt-1 text-[11px] font-medium ${actionClass[s.action] || ''}`}>
                          今日动作：{biasLabel[s.action] || s.action} — {s.reason}
                        </div>
                        {s.dip_signal ? (
                          <div className={`mt-1.5 flex items-start gap-2 rounded-md border p-2 ${dipBadge[s.dip_signal.level]?.bg || 'bg-surface-2'} ${dipBadge[s.dip_signal.level]?.border || 'border-line'}`}>
                            <span className="shrink-0 text-xs">{dipBadge[s.dip_signal.level]?.icon}</span>
                            <div>
                              <span className={`text-[11px] font-semibold ${dipBadge[s.dip_signal.level]?.text || 'text-ink'}`}>
                                {s.dip_signal.label}
                              </span>
                              <span className="ml-1.5 text-[11px] text-ink/70">{s.dip_signal.action}</span>
                            </div>
                          </div>
                        ) : null}

                        {s.dip_lines?.length ? (
                          <div className="mt-2">
                            <div className="mb-1.5 flex items-center justify-between">
                              <div className="text-[10px] font-mono uppercase tracking-wider text-muted">跳水预警线</div>
                              <button
                                type="button"
                                className="rounded-md border border-line px-1.5 py-0.5 text-[10px] text-muted hover:bg-surface-2 hover:text-ink"
                                onClick={() => armDipAlerts(s)}
                              >
                                设跳水提醒
                              </button>
                            </div>
                            <div className="space-y-1">
                              {s.dip_lines.map((dl) => {
                                const style = dipBadge[dl.level];
                                return (
                                  <div
                                    key={dl.level}
                                    className={`flex items-center justify-between gap-2 rounded-md border px-2 py-1.5 font-mono text-[11px] ${dl.triggered ? `${style?.bg || ''} ${style?.border || 'border-line'}` : 'border-line/60 bg-base'}`}
                                  >
                                    <div className="flex items-center gap-1.5">
                                      <span className={`inline-block h-2 w-2 rounded-full ${dl.level === 'red' ? 'bg-down' : dl.level === 'orange' ? 'bg-orange-500' : 'bg-yellow-500'}`} />
                                      <span className={dl.triggered ? (style?.text || 'text-ink') : 'text-muted'}>{dl.label}</span>
                                      <span className="text-faint">({dl.day_pct}%)</span>
                                    </div>
                                    <div className="flex items-center gap-2">
                                      {!dl.triggered ? (
                                        <span className="text-faint">距 {dl.gap.toFixed(1)}%</span>
                                      ) : (
                                        <span className={style?.text || 'text-down'}>已触发</span>
                                      )}
                                      <button
                                        type="button"
                                        className={`rounded px-1.5 py-0.5 text-[10px] ${dl.triggered ? 'bg-down/10 text-down hover:bg-down/20' : 'bg-accent/10 text-accent hover:bg-accent/20'}`}
                                        title="到此价时提醒我"
                                        onClick={() => addAlert(s.symbol, '<=', dl.price, 'cn')}
                                      >
                                        ≤{dl.price}
                                      </button>
                                    </div>
                                  </div>
                                );
                              })}
                            </div>
                          </div>
                        ) : null}

                        {s.hasQuote ? (
                          <div className="mt-2 grid gap-2 md:grid-cols-2">
                            <div>
                              <div className="mb-1 text-[10px] font-mono uppercase tracking-wider text-accent">买入点</div>
                              <ul className="space-y-1 text-[11px]">
                                {(s.buy_levels || []).map((lv) => (
                                  <li key={`b-${lv.pct}`} className="flex items-center justify-between gap-2 font-mono">
                                    <span className="text-muted">
                                      {lv.pct}% · {lv.note}
                                    </span>
                                    <button
                                      type="button"
                                      className="rounded bg-accent/10 px-1.5 py-0.5 text-accent hover:bg-accent/20"
                                      title="添加价格提醒"
                                      onClick={() => addAlert(s.symbol, '<=', lv.price, 'cn')}
                                    >
                                      ≤{lv.price}
                                    </button>
                                  </li>
                                ))}
                              </ul>
                            </div>
                            <div>
                              <div className="mb-1 text-[10px] font-mono uppercase tracking-wider text-down">卖出/减仓点</div>
                              <ul className="space-y-1 text-[11px]">
                                {(s.sell_levels || []).map((lv) => (
                                  <li key={`s-${lv.pct}`} className="flex items-center justify-between gap-2 font-mono">
                                    <span className="text-muted">
                                      +{lv.pct}% · {lv.note}
                                    </span>
                                    <button
                                      type="button"
                                      className="rounded bg-down/10 px-1.5 py-0.5 text-down hover:bg-down/20"
                                      title="添加价格提醒"
                                      onClick={() => addAlert(s.symbol, '>=', lv.price, 'cn')}
                                    >
                                      ≥{lv.price}
                                    </button>
                                  </li>
                                ))}
                                {s.stop_price != null ? (
                                  <li className="flex items-center justify-between gap-2 font-mono">
                                    <span className="text-muted">止损观察</span>
                                    <button
                                      type="button"
                                      className="rounded bg-down/10 px-1.5 py-0.5 text-down hover:bg-down/20"
                                      onClick={() => addAlert(s.symbol, '<=', s.stop_price!, 'cn')}
                                    >
                                      ≤{s.stop_price}
                                    </button>
                                  </li>
                                ) : null}
                              </ul>
                            </div>
                          </div>
                        ) : null}

                        {s.hasQuote ? (
                          <button
                            type="button"
                            className="mt-2 rounded-md border border-line px-2 py-1 text-[10px] text-muted hover:bg-surface-2 hover:text-ink"
                            onClick={() => armLevels(s)}
                          >
                            一键挂上全部买/卖/止损提醒
                          </button>
                        ) : null}
                      </div>
                    ))}
                  </div>
                </div>
              ) : null}
            </section>
          );
        })}
      </div>
    </div>
  );
};
