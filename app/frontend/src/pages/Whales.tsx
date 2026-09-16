import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { InstitutionCard } from '../components/InstitutionCard';
import { StockGodShell } from '../components/layout/StockGodShell';
import { CongressCard } from '../components/CongressCard';
import { DataStatus } from '../components/common';
import type { PartyFilter, TabType, TradeTypeFilter } from '../types/whales';
import { useWhalesStore, type ConsensusStock } from '../stores/whalesStore';
import { useI18n } from '../i18n';

type CategoryFilter = 'all' | 'us_gurus' | 'a_share_top' | 'private' | 'hot_money';
type MiniListMode = 'add' | 'trim';

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
  const netChange = stock.addCount - stock.trimCount;
  const changeText = netChange === 0 ? '·' : netChange > 0 ? `+${netChange}` : `${netChange}`;
  const changeClass = netChange > 0 ? 'text-up' : netChange < 0 ? 'text-down' : 'text-faint';

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
  const count = mode === 'add' ? stock.addCount : stock.trimCount;

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
        {mode === 'add' ? '+' : '-'}
        {count} 位
      </span>
    </a>
  );
};

export const Whales: React.FC = () => {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<TabType>('institutions');
  const [searchQuery, setSearchQuery] = useState('');
  const [categoryFilter, setCategoryFilter] = useState<CategoryFilter>('all');
  const [partyFilter, setPartyFilter] = useState<PartyFilter>('all');
  const [tradeTypeFilter, setTradeTypeFilter] = useState<TradeTypeFilter>('all');
  const {
    pageInvestors: investors,
    pageCongressMembers: congressMembers,
    pageConsensusStocks: consensusStocks,
    investorsLoading,
    investorsError,
    investorsMeta,
    consensusLoading,
    consensusError,
    consensusMeta,
    congressLoading,
    congressError,
    congressMeta,
    fetchPageInvestors,
    fetchPageConsensus,
    fetchPageCongress,
  } = useWhalesStore();

  useEffect(() => {
    if (activeTab !== 'institutions') return;

    void fetchPageInvestors(categoryFilter);
    void fetchPageConsensus(categoryFilter);
  }, [activeTab, categoryFilter, fetchPageInvestors, fetchPageConsensus]);

  useEffect(() => {
    if (activeTab !== 'congress') return;

    void fetchPageCongress(partyFilter, tradeTypeFilter);
  }, [activeTab, partyFilter, tradeTypeFilter, fetchPageCongress]);

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

  const verifiedCongressMembers = useMemo(
    () => filteredCongressMembers.filter((member) => member.latestTrade?.verified),
    [filteredCongressMembers]
  );
  const unverifiedCongressMembers = useMemo(
    () => filteredCongressMembers.filter((member) => !member.latestTrade?.verified),
    [filteredCongressMembers]
  );

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
    () => [...searchedConsensus].filter((stock) => stock.addCount > 0).sort((a, b) => b.addCount - a.addCount || b.guruCount - a.guruCount).slice(0, 20),
    [searchedConsensus]
  );
  const mostTrimmed = useMemo(
    () => [...searchedConsensus].filter((stock) => stock.trimCount > 0).sort((a, b) => b.trimCount - a.trimCount || b.guruCount - a.guruCount).slice(0, 20),
    [searchedConsensus]
  );

  const denseMode = categoryFilter === 'hot_money' || categoryFilter === 'a_share_top' || categoryFilter === 'private';
  const stockMarket = denseMode ? 'cn' : 'us';

  const consensusWithThree = consensusStocks.filter((stock) => stock.guruCount >= 3).length;
  const netAdds = consensusStocks.filter((stock) => stock.addCount > stock.trimCount).length;
  const netTrims = consensusStocks.filter((stock) => stock.trimCount > stock.addCount).length;
  const reportPeriod = consensusStocks[0]?.reportPeriod || '';
  const consensusInvestorCount = reportPeriod ? investors.filter(
    (investor) => investor.type === 'superinvestor' && investor.reportPeriod === reportPeriod
  ).length : 0;
  const consensusReason = consensusMeta?.staleReason || '';
  const consensusSource = consensusMeta?.source || '';
  const consensusReasonText = consensusReason.includes('no shared disclosure report period') || consensusReason.includes('missing disclosure report period')
    ? '缺少同一报告期的可核对披露，暂不计算共识'
    : consensusReason.includes('source evidence')
      ? '同报告期的来源证据缺失或不一致，暂不计算共识'
      : consensusReason;

  return (
    <StockGodShell title={t('whales.title')} searchQuery={searchQuery} onSearchChange={setSearchQuery}>
      <div className="space-y-7">
              <div>
                <h1 className="text-[22px] font-semibold tracking-tight text-ink">{t('whales.title')}</h1>
                <p className="mt-1.5 text-sm text-muted">
                  关注仓位占比与披露期变动，重仓、建仓与减持代表不同含义，而非仅看是否持有。
                </p>
              </div>

              <div className="flex flex-wrap items-center gap-2">
                <div className="inline-flex rounded-lg border border-line bg-surface p-0.5">
                  <FilterButton active={activeTab === 'institutions'} onClick={() => setActiveTab('institutions')} inTab>
                    机构披露
                  </FilterButton>
                  <FilterButton active={activeTab === 'congress'} onClick={() => setActiveTab('congress')} inTab>
					国会 · {verifiedCongressMembers.length}
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

              {activeTab === 'institutions' && (
                <DataStatus
                  state={investorsError ? 'error' : investorsLoading ? 'loading' : investorsMeta?.stale ? 'stale' : investorsMeta ? 'live' : 'unavailable'}
                  label={!investorsError && !investorsLoading && investorsMeta && !investorsMeta.stale ? '机构披露快照' : undefined}
                  dataTime={investorsMeta?.dataTime}
                  source={investorsMeta?.source}
                  message={investorsError || investorsMeta?.staleReason}
                  onRetry={() => void fetchPageInvestors(categoryFilter)}
                />
              )}
              {activeTab === 'congress' && (
                <DataStatus
                  state={congressError ? 'error' : congressLoading ? 'loading' : congressMeta?.stale ? 'stale' : congressMeta ? 'live' : 'unavailable'}
                  label={!congressError && !congressLoading && congressMeta && !congressMeta.stale ? '国会披露快照' : undefined}
                  dataTime={congressMeta?.dataTime}
                  source={congressMeta?.source}
                  message={congressError || congressMeta?.staleReason}
                  onRetry={() => void fetchPageCongress(partyFilter, tradeTypeFilter)}
                />
              )}

              {activeTab === 'institutions' && !investorsLoading && !investorsError && (
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
                            {consensusInvestorCount} 位申报主体交叉持仓 · 报告期 {reportPeriod || '未知'} · 来源 {consensusSource || '未知'}
                          </p>
                        </div>
                      </div>

                      <div className="mt-3">
                        <DataStatus
                          state={consensusError ? 'error' : consensusLoading ? 'loading' : consensusMeta?.stale ? 'stale' : consensusMeta ? 'live' : 'unavailable'}
                          label={!consensusError && !consensusLoading && consensusMeta && !consensusMeta.stale ? '共识快照' : undefined}
                          dataTime={consensusMeta?.dataTime}
                          source={consensusMeta?.source}
                          message={consensusError || consensusMeta?.staleReason}
                          onRetry={() => void fetchPageConsensus(categoryFilter)}
                          compact
                        />
                      </div>

                      {!consensusLoading && !consensusError && <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
                        <StatPill label="价投大佬" value={consensusInvestorCount} />
                        <StatPill label="≥3 人共识" value={consensusWithThree} />
                        <StatPill label="本期净加码" value={netAdds} />
                        <StatPill label="本期净减持" value={netTrims} />
                      </div>}

                      {!consensusLoading && !consensusError && <><h3 className="mb-1.5 mt-5 text-[12px] font-medium uppercase tracking-wider text-faint">最多大佬共同持有</h3>
                      <div>
                        {topHeld.length > 0 ? (
                          topHeld.map((stock, index) => <MainConsensusRow key={`${stock.symbol}-${stock.name}-${index}`} stock={stock} index={index} />)
                        ) : (
                          <div className="py-8 text-center text-[12px] text-faint">
                            暂无共识数据{consensusReasonText ? `：${consensusReasonText}` : ''}
                          </div>
                        )}
                      </div></>}

                      <div className="mt-5 grid grid-cols-1 gap-4 lg:grid-cols-2">
                        <div>
                          <h3 className="mb-1.5 text-[12px] font-medium uppercase tracking-wider text-faint">本期加码最集中</h3>
                          <div>
                            {mostAdded.map((stock) => (
                              <MiniConsensusRow key={`add-${stock.symbol}-${stock.name}`} stock={stock} mode="add" />
                            ))}
                          </div>
                        </div>
                        <div>
                          <h3 className="mb-1.5 text-[12px] font-medium uppercase tracking-wider text-faint">本期减持最集中</h3>
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
                                {investor.type === 'superinvestor' && (
                                  <div className={`mt-1 text-[10px] ${investor.stale ? 'text-down' : 'text-faint'}`}>
                                    {investor.reportPeriod ? `报告期 ${investor.reportPeriod} · 申报日 ${investor.filingDate || '未知'}` : `来源日期 ${investor.sourceAsOf || '未提供'}`} · 来源 {investor.source || '未知'}
                                    <br />
                                    {investor.reportPeriod ? `同步 ${investor.syncedAt || '未知'} · Accession ${investor.accession || '未知'}` : '非 SEC 原始申报，不展示 Accession'}
                                  </div>
                                )}
                              </div>
                              <div className="shrink-0 text-right text-[11px] text-muted">{investor.holdings} 仓</div>
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

              {activeTab === 'congress' && !congressLoading && !congressError && (
                <div className="space-y-8">
                  <section>
                    <div className="mb-3 flex flex-wrap items-baseline justify-between gap-2">
                      <h2 className="text-[15px] font-semibold text-ink">可核验国会披露榜单</h2>
                      <span className="text-[11px] text-faint">{verifiedCongressMembers.length} 位 · 仅计入来源、申报日、原文与文件标识完整的记录</span>
                    </div>
                    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
                  {verifiedCongressMembers.map((member) => (
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
                  {verifiedCongressMembers.length === 0 && <div className="col-span-full py-12 text-center text-muted">当前筛选无可核验国会披露</div>}
                    </div>
                  </section>
                  {unverifiedCongressMembers.length > 0 && (
                    <section className="rounded-xl border border-amber-500/35 bg-amber-500/5 p-4">
                      <h2 className="text-[15px] font-semibold text-amber-200">未核验历史样本</h2>
                      <p className="mt-1 text-[12px] leading-relaxed text-amber-100/70">
                        这些旧 bootstrap 记录缺少可定位原文或申报标识，不进入榜单计数，也不作为投资信号。
                      </p>
                      <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4 opacity-75">
                        {unverifiedCongressMembers.map((member) => (
                          <CongressCard
                            key={`unverified-${member.id}`}
                            member={member}
                            onClick={() => {
                              const slug = member.name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
                              navigate(`/whales/congress/${slug || member.id}`);
                            }}
                          />
                        ))}
                      </div>
                    </section>
                  )}
                </div>
              )}
      </div>
    </StockGodShell>
  );
};
