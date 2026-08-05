import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { InstitutionCard } from '../components/InstitutionCard';
import { StockGodShell } from '../components/layout/StockGodShell';
import { CongressCard } from '../components/CongressCard';
import type { CongressMember, Investor, PartyFilter, TabType, TradeTypeFilter } from '../types/whales';
import { apiUrl } from '../utils/api';

interface ConsensusStock {
  symbol: string;
  name: string;
  guruCount: number;
  avgWeight: number;
  change: string;
}

type CategoryFilter = 'all' | 'us_gurus' | 'a_share_top' | 'private' | 'hot_money';
type MiniListMode = 'add' | 'trim';

const WHALES_API = '/api/whales';

const categoryOptions: Array<{ label: string; value: CategoryFilter }> = [
  { label: '全部', value: 'all' },
  { label: '美股大佬', value: 'us_gurus' },
  { label: 'A股顶流', value: 'a_share_top' },
  { label: '私募大佬', value: 'private' },
  { label: '游资席位', value: 'hot_money' },
];

const partyOptions: Array<{ label: string; value: PartyFilter }> = [
  { label: '全部', value: 'all' },
  { label: '民主党', value: 'democrat' },
  { label: '共和党', value: 'republican' },
];

const tradeOptions: Array<{ label: string; value: TradeTypeFilter }> = [
  { label: '买卖', value: 'all' },
  { label: '买入', value: 'buy' },
  { label: '卖出', value: 'sell' },
];

const changeNumber = (value: string) => {
  const parsed = parseFloat(value);
  return Number.isFinite(parsed) ? parsed : 0;
};

const StockBadge: React.FC<{ symbol: string }> = ({ symbol }) => (
  <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full border border-line bg-surface-3 font-mono text-[7px] font-semibold text-accent">
    {symbol.slice(0, 1)}
  </span>
);

const FilterButton: React.FC<{ active: boolean; children: React.ReactNode; onClick: () => void; inTab?: boolean }> = ({
  active,
  children,
  onClick,
  inTab = false,
}) => (
  <button
    onClick={onClick}
    className={`${inTab ? 'inline-flex min-w-[99px] items-center justify-center gap-1.5 px-3.5' : 'px-3'} rounded-md py-2 text-[13px] font-medium transition sm:py-1.5 ${
      active ? 'bg-surface-3 text-ink' : 'text-muted hover:text-ink'
    }`}
  >
    {children}
  </button>
);

const StatPill: React.FC<{ label: string; value: string | number }> = ({ label, value }) => (
  <div className="rounded-lg border border-line bg-base/40 px-3 py-2">
    <div className="text-[11px] text-faint">{label}</div>
    <div className="mt-1 font-mono text-[18px] font-semibold tabular-nums text-ink">{value}</div>
  </div>
);

const MainConsensusRow: React.FC<{ stock: ConsensusStock; index: number }> = ({ stock, index }) => {
  const change = changeNumber(stock.change);
  const changeText = change === 0 ? '·' : change > 0 ? `+${Math.round(change)}` : `${Math.round(change)}`;
  const changeClass = change > 0 ? 'text-up' : change < 0 ? 'text-down' : 'text-faint';

  return (
    <a
      href={`/stock/${stock.symbol}?market=us`}
      className="group flex items-center gap-2 rounded-md px-1.5 py-1.5 transition hover:bg-surface-2"
    >
      <span className="w-5 shrink-0 text-right font-mono text-[10px] tabular-nums text-faint">{index + 1}</span>
      <StockBadge symbol={stock.symbol} />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="truncate text-[13px] font-semibold text-ink group-hover:text-accent">{stock.symbol}</span>
          <span className="truncate text-[12px] text-muted">{stock.name}</span>
        </div>
      </div>
      <span className="w-12 shrink-0 text-right font-mono text-[12px] tabular-nums text-ink">{stock.guruCount} 位</span>
      <span className="w-16 shrink-0 text-right font-mono text-[12px] tabular-nums text-muted">均 {stock.avgWeight.toFixed(1)}%</span>
      <span className={`w-8 shrink-0 text-right font-mono text-[12px] tabular-nums ${changeClass}`}>{changeText}</span>
    </a>
  );
};

const MiniConsensusRow: React.FC<{ stock: ConsensusStock; mode: MiniListMode }> = ({ stock, mode }) => {
  const delta = mode === 'add' ? Math.max(1, Math.round(stock.guruCount / 4)) : -Math.max(1, Math.round(stock.guruCount / 5));

  return (
    <a href={`/stock/${stock.symbol}?market=us`} className="group flex h-[26px] items-center gap-2 rounded-md px-1.5 py-1 transition hover:bg-surface-2">
      <StockBadge symbol={stock.symbol} />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-1.5">
          <span className="truncate text-[12px] font-semibold text-ink group-hover:text-accent">{stock.symbol}</span>
          <span className="truncate text-[11px] text-muted">{stock.name}</span>
        </div>
      </div>
      <span className="shrink-0 font-mono text-[11px] text-faint">{stock.guruCount}持</span>
      <span className={`shrink-0 font-mono text-[11px] tabular-nums ${mode === 'add' ? 'text-up' : 'text-down'}`}>
        {delta > 0 ? '+' : ''}
        {delta} 位
      </span>
    </a>
  );
};

export const Whales: React.FC = () => {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<TabType>('institutions');
  const [searchQuery, setSearchQuery] = useState('');
  const [categoryFilter, setCategoryFilter] = useState<CategoryFilter>('all');
  const [partyFilter, setPartyFilter] = useState<PartyFilter>('all');
  const [tradeTypeFilter, setTradeTypeFilter] = useState<TradeTypeFilter>('all');
  const [investors, setInvestors] = useState<Investor[]>([]);
  const [congressMembers, setCongressMembers] = useState<CongressMember[]>([]);
  const [consensusStocks, setConsensusStocks] = useState<ConsensusStock[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (activeTab !== 'institutions') return;

    const load = async () => {
      setLoading(true);
      try {
        const params = new URLSearchParams();
        if (categoryFilter !== 'all') params.append('type', categoryFilter);
        const query = params.toString();
        const [gurusResponse, consensusResponse] = await Promise.all([
          fetch(apiUrl(`${WHALES_API}/gurus${query ? `?${query}` : ''}`)),
          fetch(apiUrl(`${WHALES_API}/consensus`)),
        ]);

        if (gurusResponse.ok) setInvestors((await gurusResponse.json()) || []);
        if (consensusResponse.ok) setConsensusStocks((await consensusResponse.json()) || []);
      } catch (error) {
        console.error('Failed to load whales page data:', error);
      } finally {
        setLoading(false);
      }
    };

    load();
  }, [activeTab, categoryFilter]);

  useEffect(() => {
    if (activeTab !== 'congress') return;

    const load = async () => {
      setLoading(true);
      try {
        const params = new URLSearchParams();
        if (partyFilter !== 'all') params.append('party', partyFilter);
        if (tradeTypeFilter !== 'all') params.append('type', tradeTypeFilter);
        const response = await fetch(apiUrl(`${WHALES_API}/congress${params.toString() ? `?${params.toString()}` : ''}`));
        if (response.ok) setCongressMembers((await response.json()) || []);
      } catch (error) {
        console.error('Failed to load congress data:', error);
      } finally {
        setLoading(false);
      }
    };

    load();
  }, [activeTab, partyFilter, tradeTypeFilter]);

  const filteredInvestors = useMemo(() => {
    if (!searchQuery) return investors;
    const query = searchQuery.toLowerCase();
    return investors.filter((investor) =>
      [investor.name, investor.nameEn, investor.company, investor.topStock?.symbol, investor.topStock?.name].some((value) =>
        (value || '').toLowerCase().includes(query)
      )
    );
  }, [investors, searchQuery]);

  const filteredCongressMembers = useMemo(() => {
    if (!searchQuery) return congressMembers;
    const query = searchQuery.toLowerCase();
    return congressMembers.filter((member) =>
      [member.name, member.state, member.district, member.latestTrade?.symbol].some((value) =>
        (value || '').toLowerCase().includes(query)
      )
    );
  }, [congressMembers, searchQuery]);

  const searchedConsensus = useMemo(() => {
    if (!searchQuery) return consensusStocks;
    const query = searchQuery.toLowerCase();
    return consensusStocks.filter((stock) => stock.symbol.toLowerCase().includes(query) || stock.name.toLowerCase().includes(query));
  }, [consensusStocks, searchQuery]);

  const topHeld = useMemo(
    () => [...searchedConsensus].sort((a, b) => b.guruCount - a.guruCount || b.avgWeight - a.avgWeight).slice(0, 30),
    [searchedConsensus]
  );
  const mostAdded = useMemo(
    () => [...searchedConsensus].sort((a, b) => changeNumber(b.change) - changeNumber(a.change) || b.guruCount - a.guruCount).slice(0, 20),
    [searchedConsensus]
  );
  const mostTrimmed = useMemo(
    () => [...searchedConsensus].sort((a, b) => changeNumber(a.change) - changeNumber(b.change) || b.guruCount - a.guruCount).slice(0, 20),
    [searchedConsensus]
  );

  const denseMode = categoryFilter === 'hot_money' || categoryFilter === 'a_share_top' || categoryFilter === 'private';
  const stockMarket = denseMode ? 'cn' : 'us';

  const consensusWithThree = consensusStocks.filter((stock) => stock.guruCount >= 3).length;
  const netAdds = consensusStocks.filter((stock) => changeNumber(stock.change) > 0).length;
  const netTrims = consensusStocks.filter((stock) => changeNumber(stock.change) < 0).length;

  return (
    <StockGodShell title="聪明钱" searchQuery={searchQuery} onSearchChange={setSearchQuery}>
      <div className="space-y-7">
              <div>
                <h1 className="text-[22px] font-semibold tracking-tight text-ink">聪明钱</h1>
                <p className="mt-1.5 text-sm text-muted">
                  关注仓位占比与季度变动 —— 重仓、建仓与减持代表不同含义,而非仅看是否持有。
                </p>
              </div>

              <div className="flex flex-wrap items-center gap-2">
                <div className="inline-flex rounded-lg border border-line bg-surface p-0.5">
                  <FilterButton active={activeTab === 'institutions'} onClick={() => setActiveTab('institutions')} inTab>
                    机构 13F
                  </FilterButton>
                  <FilterButton active={activeTab === 'congress'} onClick={() => setActiveTab('congress')} inTab>
                    国会 · {congressMembers.length || 96}
                  </FilterButton>
                </div>

                {activeTab === 'institutions' && (
                  <div className="flex flex-wrap gap-0">
                    {categoryOptions.map((option) => (
                      <FilterButton
                        key={option.value}
                        active={categoryFilter === option.value}
                        onClick={() => setCategoryFilter(option.value)}
                      >
                        {option.label}
                      </FilterButton>
                    ))}
                  </div>
                )}

                {activeTab === 'congress' && (
                  <div className="flex flex-wrap gap-2">
                    <div className="flex flex-wrap gap-0">
                      {partyOptions.map((option) => (
                        <FilterButton key={option.value} active={partyFilter === option.value} onClick={() => setPartyFilter(option.value)}>
                          {option.label}
                        </FilterButton>
                      ))}
                    </div>
                    <div className="flex flex-wrap gap-0">
                      {tradeOptions.map((option) => (
                        <FilterButton
                          key={option.value}
                          active={tradeTypeFilter === option.value}
                          onClick={() => setTradeTypeFilter(option.value)}
                        >
                          {option.label}
                        </FilterButton>
                      ))}
                    </div>
                  </div>
                )}
              </div>

              {loading && <div className="rounded-xl border border-line bg-surface p-8 text-center text-sm text-muted">加载中...</div>}

              {activeTab === 'institutions' && !loading && (
                <>
                  {!denseMode && (
                    <section>
                      <h2 className="mb-2 text-[12px] font-medium uppercase tracking-wider text-faint">知名价投</h2>
                      <div className="flex gap-3 overflow-x-auto pb-2 [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                        {filteredInvestors.slice(0, 24).map((investor) => (
                          <InstitutionCard
                            key={investor.id}
                            investor={investor}
                            onClick={() => {
                              navigate(`/whales/${investor.slug}`);
                            }}
                          />
                        ))}
                      </div>
                    </section>
                  )}

                  {!denseMode && (
                    <section className="rounded-xl border border-line bg-surface p-5">
                      <div className="flex flex-wrap items-start justify-between gap-4">
                        <div>
                          <h2 className="text-[15px] font-semibold text-ink">聪明钱共识</h2>
                          <p className="mt-1 text-[12px] text-muted">
                            {investors.length || 81} 位价投大佬交叉持仓 · 20 May 2026 · 13F
                          </p>
                        </div>
                      </div>

                      <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
                        <StatPill label="价投大佬" value={investors.length || 81} />
                        <StatPill label="≥3 人共识" value={consensusWithThree || 232} />
                        <StatPill label="本季净加码" value={netAdds || 396} />
                        <StatPill label="本季净减持" value={netTrims || 391} />
                      </div>

                      <h3 className="mb-1.5 mt-5 text-[12px] font-medium uppercase tracking-wider text-faint">最多大佬共同持有</h3>
                      <div>
                        {topHeld.length > 0 ? (
                          topHeld.map((stock, index) => <MainConsensusRow key={`${stock.symbol}-${stock.name}-${index}`} stock={stock} index={index} />)
                        ) : (
                          <div className="py-8 text-center text-[12px] text-faint">暂无共识数据</div>
                        )}
                      </div>

                      <div className="mt-5 grid grid-cols-1 gap-4 lg:grid-cols-2">
                        <div>
                          <h3 className="mb-1.5 text-[12px] font-medium uppercase tracking-wider text-faint">本季加码最集中</h3>
                          <div>
                            {mostAdded.map((stock) => (
                              <MiniConsensusRow key={`add-${stock.symbol}-${stock.name}`} stock={stock} mode="add" />
                            ))}
                          </div>
                        </div>
                        <div>
                          <h3 className="mb-1.5 text-[12px] font-medium uppercase tracking-wider text-faint">本季减持最集中</h3>
                          <div>
                            {mostTrimmed.map((stock) => (
                              <MiniConsensusRow key={`trim-${stock.symbol}-${stock.name}`} stock={stock} mode="trim" />
                            ))}
                          </div>
                        </div>
                      </div>
                    </section>
                  )}

                  <section>
                    <div className="mb-3 flex items-baseline justify-between gap-3">
                      <h2 className="text-[15px] font-semibold text-ink">
                        {categoryFilter === 'hot_money'
                          ? '游资席位 · 近端买入'
                          : categoryFilter === 'a_share_top'
                            ? 'A股顶流 · 重仓明细'
                            : categoryFilter === 'private'
                              ? '私募大佬 · 持仓明细'
                              : '机构持仓明细'}
                      </h2>
                      <span className="text-[11px] text-faint">{filteredInvestors.length} 位 · 点击代码进详情</span>
                    </div>
                    <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                      {filteredInvestors.map((investor) => {
                        const rows = investor.recentHoldings?.length
                          ? investor.recentHoldings
                          : investor.topStock?.symbol
                            ? [{ symbol: investor.topStock.symbol, name: investor.topStock.name, weight: investor.topStock.percentage, action: '持有' }]
                            : [];
                        const maxW = Math.max(...rows.map((r) => r.weight || 0), 1);
                        return (
                          <article key={investor.id} className="rounded-xl border border-line bg-surface p-4">
                            <button
                              type="button"
                              onClick={() => navigate(`/whales/${investor.slug}`)}
                              className="mb-3 flex w-full items-start justify-between gap-2 text-left"
                            >
                              <div className="min-w-0">
                                <div className="truncate text-[14px] font-semibold text-ink hover:text-accent">{investor.name}</div>
                                <div className="truncate text-[11px] text-faint">{investor.company || investor.nameEn}</div>
                              </div>
                              <div className="shrink-0 text-right text-[11px] text-muted">{investor.holdings || rows.length} 仓</div>
                            </button>
                            <div className="space-y-1.5">
                              {rows.map((row) => (
                                <a
                                  key={`${investor.id}-${row.symbol}-${row.name}`}
                                  href={`/stock/${row.symbol}?market=${stockMarket}`}
                                  className="group flex items-center gap-2 rounded-md px-1 py-1 hover:bg-surface-2"
                                  onClick={(e) => e.stopPropagation()}
                                >
                                  <span className={`w-8 shrink-0 text-[10px] font-medium ${row.action === '卖出' ? 'text-down' : row.action === '买入' ? 'text-up' : 'text-faint'}`}>
                                    {row.action}
                                  </span>
                                  <StockBadge symbol={row.symbol} />
                                  <span className="w-14 shrink-0 truncate font-mono text-[12px] font-semibold text-ink group-hover:text-accent">{row.symbol}</span>
                                  <span className="min-w-0 flex-1 truncate text-[11px] text-muted">{row.name}</span>
                                  {row.weight > 0 ? (
                                    <div className="flex w-24 shrink-0 items-center gap-1.5">
                                      <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-surface-3">
                                        <div className="h-full rounded-full bg-accent/70" style={{ width: `${Math.min(100, (row.weight / maxW) * 100)}%` }} />
                                      </div>
                                      <span className="w-10 text-right font-mono text-[10px] tabular-nums text-muted">{row.weight.toFixed(1)}%</span>
                                    </div>
                                  ) : (
                                    <span className="shrink-0 font-mono text-[10px] text-faint">—</span>
                                  )}
                                </a>
                              ))}
                            </div>
                          </article>
                        );
                      })}
                    </div>
                  </section>

                  {filteredInvestors.length === 0 && <div className="py-12 text-center text-muted">未找到匹配的投资者</div>}
                </>
              )}

              {activeTab === 'congress' && !loading && (
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
                  {filteredCongressMembers.map((member) => (
                    <CongressCard
                      key={member.id}
                      member={member}
                      onClick={() => {
                        const slug = member.name
                          .toLowerCase()
                          .replace(/[^a-z0-9]+/g, '-')
                          .replace(/^-+|-+$/g, '');
                        navigate(`/whales/congress/${slug || member.id}`);
                      }}
                    />
                  ))}
                  {filteredCongressMembers.length === 0 && <div className="col-span-full py-12 text-center text-muted">未找到匹配的国会议员</div>}
                </div>
              )}
      </div>
    </StockGodShell>
  );
};
