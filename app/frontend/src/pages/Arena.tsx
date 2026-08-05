import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { StockGodShell } from '../components/layout/StockGodShell';
import { LoadingSpinner } from '../components/common';
import { StockSelector, ComparisonCard } from '../components/arena';
import { get as apiGet } from '../utils/api';
import { Stock } from '../types/stocks';
import { findUsStock, loadUsMarketStocks } from '../utils/stockgodData';

type Player = {
  id: string;
  rank: number;
  short: string;
  name: string;
  style: string;
  value: number;
  returnPct: number;
  cash: number;
  holdings: Holding[];
  trades: Trade[];
};

type Holding = {
  symbol: string;
  name: string;
  shares: number;
  cost: number;
  price: number;
  day: number;
  pnl: number;
  thesis: string;
};

type Trade = {
  date: string;
  action: string;
  symbol: string;
  detail: string;
};

interface ArenaResponse {
  market: string;
  settleDate: string;
  players: Player[];
}

const money = (value: number) => `$${Math.round(value).toLocaleString('en-US')}`;
const pctClass = (value: number) => (value >= 0 ? 'text-up' : 'text-down');
const pct = (value: number) => `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`;

const PRESETS: Array<[string, string]> = [
  ['NVDA', 'AMD'],
  ['AAPL', 'MSFT'],
  ['TSLA', 'RIVN'],
  ['META', 'GOOGL'],
];

export const Arena: React.FC = () => {
  const [market, setMarket] = useState<'us' | 'cn'>('us');
  const [players, setPlayers] = useState<Player[]>([]);
  const [settleDate, setSettleDate] = useState('2026-07-02');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [stockA, setStockA] = useState<Stock | null>(null);
  const [stockB, setStockB] = useState<Stock | null>(null);
  const [tab, setTab] = useState<'board' | 'compare'>('board');
  const [expandedTrades, setExpandedTrades] = useState<Set<string>>(() => new Set());

  useEffect(() => {
    const loadArena = async () => {
      setLoading(true);
      setError(null);
      try {
        const response = await apiGet<ArenaResponse>(`/api/arena?market=${market}`);
        setPlayers(response.players || []);
        setSettleDate(response.settleDate || '2026-07-02');
        setExpandedTrades(new Set());
      } catch (err) {
        console.error('Failed to load arena:', err);
        setError(err instanceof Error ? err.message : 'Failed to load arena');
      } finally {
        setLoading(false);
      }
    };
    loadArena();
  }, [market]);

  useEffect(() => {
    (async () => {
      await loadUsMarketStocks().catch(() => null);
      const [a, b] = await Promise.all([findUsStock('NVDA'), findUsStock('AMD')]);
      if (a) setStockA(a);
      if (b) setStockB(b);
    })();
  }, []);

  const applyPreset = async (left: string, right: string) => {
    const [a, b] = await Promise.all([findUsStock(left), findUsStock(right)]);
    setStockA(a);
    setStockB(b);
    setTab('compare');
  };

  return (
    <StockGodShell title="对决">
      <div className="space-y-4">
        <header className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <h1 className="text-[22px] font-semibold tracking-tight text-ink">五神对决</h1>
          <div className="inline-flex rounded-lg border border-line bg-surface p-0.5 text-xs">
            <button onClick={() => setMarket('us')} className={`rounded-md px-4 py-1.5 font-semibold transition ${market === 'us' ? 'bg-accent text-black shadow' : 'text-muted hover:text-ink'}`}>美股</button>
            <button onClick={() => setMarket('cn')} className={`rounded-md px-4 py-1.5 font-semibold transition ${market === 'cn' ? 'bg-accent text-black shadow' : 'text-muted hover:text-ink'}`}>A 股</button>
          </div>
          <p className="text-sm text-muted">每人 $1,000,000 虚拟资金 · 只买已判读股票池 · 收盘结账 {settleDate} · 非投资建议</p>
        </header>

        <div className="inline-flex rounded-lg border border-line bg-surface p-0.5 text-sm">
          <button onClick={() => setTab('board')} className={`rounded-md px-4 py-1.5 font-medium ${tab === 'board' ? 'bg-surface-3 text-ink' : 'text-muted'}`}>五神虚拟盘</button>
          <button onClick={() => setTab('compare')} className={`rounded-md px-4 py-1.5 font-medium ${tab === 'compare' ? 'bg-surface-3 text-ink' : 'text-muted'}`}>股票对决</button>
        </div>

        <p className="rounded-lg border border-line bg-surface-2/60 px-3 py-2 text-[11px] leading-relaxed text-muted">
          AI 方法论模拟 —— 五位选手是依据各投资人公开方法论运行的 AI 智能体。虚拟盘 · 教育用途 · 非投资建议。
        </p>

        {tab === 'compare' && (
          <section className="space-y-4 rounded-xl border border-line bg-surface p-4">
            <div className="flex flex-wrap gap-2">
              {PRESETS.map(([a, b]) => (
                <button
                  key={`${a}-${b}`}
                  onClick={() => applyPreset(a, b)}
                  className="rounded-lg border border-line bg-base px-3 py-1.5 font-mono text-xs text-muted transition hover:text-ink"
                >
                  {a} vs {b}
                </button>
              ))}
            </div>
            <div className="grid gap-4 md:grid-cols-2">
              <StockSelector value={stockA} onChange={setStockA} placeholder="选择股票 A…" />
              <StockSelector value={stockB} onChange={setStockB} placeholder="选择股票 B…" />
            </div>
            {stockA && stockB ? (
              <ComparisonCard stockA={stockA} stockB={stockB} />
            ) : (
              <div className="rounded-lg border border-dashed border-line px-4 py-10 text-center text-sm text-muted">
                选择两只股票，查看五方评分与基本面对比。
              </div>
            )}
          </section>
        )}

        {tab === 'board' && loading && (
          <div className="flex items-center justify-center rounded-xl border border-line bg-surface py-16">
            <LoadingSpinner size="lg" />
          </div>
        )}

        {tab === 'board' && error && <div className="rounded-xl border border-line bg-surface p-6 text-sm text-down">{error}</div>}

        {tab === 'board' && !loading && !error && (
          <>
            <div className="grid gap-3 md:grid-cols-5">
              {players.map((player) => (
                <a key={player.id} href={`#${player.id}`} className={`group rounded-xl border bg-surface p-4 transition hover:-translate-y-0.5 hover:border-accent/40 ${player.rank === 1 ? 'border-accent/40 ring-1 ring-accent/20' : 'border-line'}`}>
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-xs text-faint">{player.rank}</span>
                    <span className="flex h-8 w-8 items-center justify-center rounded-full bg-surface-3 text-sm font-semibold text-accent">{player.short}</span>
                    <div className="min-w-0">
                      <div className="truncate text-sm font-semibold text-ink">{player.name}</div>
                      <div className="truncate text-[11px] text-faint">{player.style}</div>
                    </div>
                  </div>
                  <div className="mt-3 font-mono text-lg font-semibold text-ink">{money(player.value)}</div>
                  <div className={`font-mono text-sm font-semibold ${pctClass(player.returnPct)}`}>{pct(player.returnPct)}</div>
                  <div className="mt-1 text-[11px] text-muted">{player.holdings.length} 仓 · 现金 {money(player.cash)}</div>
                </a>
              ))}
            </div>

            <div className="space-y-5">
              {players.map((player) => (
                <section key={player.id} id={player.id} className="scroll-mt-20 rounded-xl border border-line bg-surface">
                  <header className="flex flex-wrap items-center gap-3 border-b border-line px-4 py-3">
                    <span className="flex h-9 w-9 items-center justify-center rounded-full bg-surface-3 font-semibold text-accent">{player.short}</span>
                    <div className="min-w-0 flex-1">
                      <h2 className="text-sm font-semibold text-ink">{player.rank} · {player.name}</h2>
                      <p className="text-xs text-muted">{player.style}</p>
                    </div>
                    <div className="text-right">
                      <div className="font-mono text-sm font-semibold text-ink">{money(player.value)}</div>
                      <div className={`font-mono text-xs font-semibold ${pctClass(player.returnPct)}`}>{pct(player.returnPct)}</div>
                    </div>
                  </header>

                  <div className="overflow-x-auto">
                    <table className="w-full min-w-[860px] border-collapse text-sm">
                      <thead>
                        <tr className="border-b border-line/70 text-xs text-muted">
                          <th className="px-4 py-2 text-left font-medium">持仓</th>
                          <th className="px-3 py-2 text-right font-medium">股数</th>
                          <th className="px-3 py-2 text-right font-medium">成本</th>
                          <th className="px-3 py-2 text-right font-medium">现价</th>
                          <th className="px-3 py-2 text-right font-medium">今日</th>
                          <th className="px-3 py-2 text-right font-medium">盈亏</th>
                          <th className="px-4 py-2 text-left font-medium">他的判词</th>
                        </tr>
                      </thead>
                      <tbody>
                        {player.holdings.map((h) => (
                          <tr key={h.symbol} className="border-b border-line/50 hover:bg-surface-2">
                            <td className="px-4 py-2">
                              <Link to={`/stock/${h.symbol}`} className="font-mono font-semibold text-ink hover:text-accent">{h.symbol}</Link>
                              <div className="text-[11px] text-faint">{h.name}</div>
                            </td>
                            <td className="px-3 py-2 text-right font-mono text-muted">{h.shares.toLocaleString()}</td>
                            <td className="px-3 py-2 text-right font-mono text-muted">{h.cost.toFixed(2)}</td>
                            <td className="px-3 py-2 text-right font-mono text-ink">{h.price.toFixed(2)}</td>
                            <td className={`px-3 py-2 text-right font-mono ${pctClass(h.day)}`}>{pct(h.day)}</td>
                            <td className={`px-3 py-2 text-right font-mono ${pctClass(h.pnl)}`}>{pct(h.pnl)}</td>
                            <td className="max-w-[420px] px-4 py-2 text-xs leading-relaxed text-muted">{h.thesis}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>

                  {player.trades?.length > 0 && (
                    <div className="border-t border-line px-4 py-3">
                      <h3 className="mb-2 text-[12px] font-medium uppercase tracking-wider text-faint">近期成交</h3>
                      {(() => {
                        const isExpanded = expandedTrades.has(player.id);
                        const visibleTrades = isExpanded ? player.trades : player.trades.slice(0, 6);
                        return (
                      <div className="space-y-2">
                        {visibleTrades.map((t, idx) => (
                          <div key={`${player.id}-${t.date}-${t.symbol}-${idx}`} className="grid grid-cols-[72px_40px_64px_minmax(0,1fr)] items-start gap-2 text-[12px]">
                            <span className="font-mono text-faint">{t.date.slice(5)}</span>
                            <span className={`font-medium ${t.action.includes('卖') ? 'text-down' : 'text-up'}`}>{t.action}</span>
                            <Link to={`/stock/${t.symbol}`} className="font-mono font-semibold text-ink hover:text-accent">{t.symbol}</Link>
                            <span className="leading-relaxed text-muted">{t.detail}</span>
                          </div>
                        ))}
                        {!isExpanded && player.trades.length > visibleTrades.length ? (
                          <button
                            type="button"
                            onClick={() =>
                              setExpandedTrades((prev) => {
                                const next = new Set(prev);
                                next.add(player.id);
                                return next;
                              })
                            }
                            className="mt-1 rounded-lg border border-line bg-base px-3 py-1.5 text-xs font-medium text-muted transition hover:border-accent/40 hover:text-ink"
                          >
                            展开全部 {player.trades.length} 笔
                          </button>
                        ) : null}
                        {isExpanded && player.trades.length > 6 ? (
                          <button
                            type="button"
                            onClick={() =>
                              setExpandedTrades((prev) => {
                                const next = new Set(prev);
                                next.delete(player.id);
                                return next;
                              })
                            }
                            className="mt-1 rounded-lg border border-line bg-base px-3 py-1.5 text-xs font-medium text-muted transition hover:border-accent/40 hover:text-ink"
                          >
                            收起
                          </button>
                        ) : null}
                      </div>
                        );
                      })()}
                    </div>
                  )}
                </section>
              ))}
            </div>
          </>
        )}
      </div>
    </StockGodShell>
  );
};
