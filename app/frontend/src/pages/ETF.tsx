import React, { useEffect } from 'react';
import { ChevronDown, ChevronLeft } from 'lucide-react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { StockGodShell } from '../components/layout/StockGodShell';
import { LoadingSpinner } from '../components/common';
import { useETFStore } from '../stores/etfStore';
import { useDebounce } from '../hooks/useDebounce';
import { ETFCategory, ETFSortBy, ETFSector } from '../types/etf';

const CATEGORIES: Array<{ id: ETFCategory | 'all'; label: string; fallback: number }> = [
  { id: 'all', label: '全部', fallback: 4533 },
  { id: 'broad', label: '宽基', fallback: 656 },
  { id: 'industry', label: '行业', fallback: 370 },
  { id: 'theme', label: '主题', fallback: 241 },
  { id: 'factor', label: '因子/策略', fallback: 878 },
  { id: 'bond', label: '债券', fallback: 584 },
  { id: 'commodity', label: '商品', fallback: 121 },
  { id: 'leveraged', label: '杠杆/反向', fallback: 601 },
  { id: 'other', label: '其他', fallback: 1082 },
];

const SORT_OPTIONS: Array<{ id: ETFSortBy; label: string }> = [
  { id: 'aum', label: '规模' },
  { id: 'return1y', label: '近1年' },
  { id: 'return5y', label: '近5年' },
  { id: 'drawdown', label: '抗跌' },
];

const formatAum = (value: number) => {
  if (value >= 1_000_000_000_000) return `$${(value / 1_000_000_000_000).toFixed(1)}T`;
  if (value >= 1_000_000_000) return `$${Math.round(value / 1_000_000_000)}B`;
  if (value >= 1_000_000) return `$${Math.round(value / 1_000_000)}M`;
  return `$${Math.round(value)}`;
};

const SectorRow: React.FC<{ sector: ETFSector }> = ({ sector }) => {
  const navigate = useNavigate();
  return (
  <section className="overflow-hidden rounded-xl border border-line bg-surface">
    <button onClick={() => navigate(`/etf/${sector.id}`)} className="flex w-full items-center gap-2 px-4 py-3 text-left transition hover:bg-surface-2">
      <span className="text-[15px] font-semibold text-ink">{sector.name}</span>
      <span className="rounded-full bg-surface-3 px-2 py-0.5 text-[10px] font-medium text-muted">
        {sector.etfCount}
      </span>
      <span className="font-mono text-[11px] text-faint">{formatAum(sector.aum)}</span>
      <span className="flex-1" />
      <span className="hidden items-center gap-1 text-[11px] sm:flex">
        <span className="text-faint">5年最强</span>
        <span className="font-mono font-semibold text-ink">{sector.topPerformer.ticker}</span>
        <span className="font-mono font-semibold text-up">+{Math.round(sector.topPerformer.return5y)}%</span>
      </span>
      <span className="ml-2 hidden items-center gap-1 text-[11px] sm:flex">
        <span className="text-faint">最深回撤</span>
        <span className="font-mono font-semibold text-down">{Math.round(sector.maxDrawdown)}%</span>
      </span>
      <ChevronDown className="h-4 w-4 shrink-0 text-faint transition" />
    </button>
  </section>
  );
};

export const ETF: React.FC = () => {
  const { id } = useParams<{ id?: string }>();
  const {
    sectors,
    categories,
    loading,
    error,
    activeCategory,
    sortBy,
    searchQuery,
    fetchSectors,
    setActiveCategory,
    setSortBy,
    setSearchQuery,
    getMembersForSector,
  } = useETFStore();

  const debouncedSearch = useDebounce(searchQuery, 300);

  useEffect(() => {
    fetchSectors();
  }, []);

  useEffect(() => {
    fetchSectors();
  }, [debouncedSearch]);

  const activeSector = id ? sectors.find((sector) => sector.id === id) || useETFStore.getState().allSectors.find((s) => s.id === id) : null;

  if (id && !loading && !activeSector) {
    return (
      <StockGodShell title="ETF">
        <div className="rounded-xl border border-line bg-surface p-10 text-center">
          <p className="text-sm text-muted">没找到这个 ETF 板块。</p>
          <Link to="/etf" className="mt-4 inline-flex items-center gap-1 rounded-lg border border-line px-3 py-2 text-sm text-accent hover:bg-surface-2">
            <ChevronLeft className="h-4 w-4" />
            返回 ETF
          </Link>
        </div>
      </StockGodShell>
    );
  }

  if (activeSector) {
    const members = getMembersForSector(activeSector.name).slice(0, 40);

    return (
      <StockGodShell title="ETF">
        <div className="mx-auto max-w-[1040px] space-y-5">
          <Link to="/etf" className="inline-flex items-center gap-1 text-sm text-muted transition hover:text-ink">
            <ChevronLeft className="h-4 w-4" />
            ETF 板块
          </Link>

          <header className="border-b border-line pb-4">
            <h1 className="text-[24px] font-semibold tracking-tight text-ink">{activeSector.name}</h1>
            <p className="mt-2 text-sm leading-relaxed text-muted">
              {activeSector.etfCount} 只 ETF · 总规模 {formatAum(activeSector.aum)} · 近 1 年 {activeSector.return1y ?? 0}% · 近 5 年 {activeSector.return5y ?? 0}% · 最大回撤 {activeSector.maxDrawdown}%。
            </p>
          </header>

          <div className="grid gap-3 md:grid-cols-4">
            <div className="rounded-xl border border-line bg-surface p-4">
              <div className="text-xs text-faint">ETF 数量</div>
              <div className="mt-2 font-mono text-xl font-semibold text-ink">{activeSector.etfCount}</div>
            </div>
            <div className="rounded-xl border border-line bg-surface p-4">
              <div className="text-xs text-faint">AUM</div>
              <div className="mt-2 font-mono text-xl font-semibold text-ink">{formatAum(activeSector.aum)}</div>
            </div>
            <div className="rounded-xl border border-line bg-surface p-4">
              <div className="text-xs text-faint">5年最强</div>
              <div className="mt-2 font-mono text-xl font-semibold text-up">{activeSector.topPerformer.ticker} +{Math.round(activeSector.topPerformer.return5y)}%</div>
            </div>
            <div className="rounded-xl border border-line bg-surface p-4">
              <div className="text-xs text-faint">最大回撤</div>
              <div className="mt-2 font-mono text-xl font-semibold text-down">{Math.round(activeSector.maxDrawdown)}%</div>
            </div>
          </div>

          <section className="rounded-xl border border-line bg-surface">
            <header className="border-b border-line px-4 py-3">
              <h2 className="text-sm font-semibold text-ink">板块内 ETF（{members.length}）</h2>
            </header>
            <div className="divide-y divide-line/60">
              {members.length === 0 ? (
                <div className="px-4 py-8 text-center text-sm text-muted">暂无成员数据</div>
              ) : (
                members.map((etf) => (
                  <Link
                    key={etf.sym}
                    to={`/stock/${etf.sym}`}
                    className="flex items-center justify-between px-4 py-3 transition hover:bg-surface-2"
                  >
                    <div className="min-w-0">
                      <div className="font-mono text-sm font-semibold text-ink">{etf.sym}</div>
                      <div className="truncate text-xs text-muted">{etf.name}</div>
                    </div>
                    <div className="ml-3 text-right">
                      <div className={`font-mono text-sm ${(etf.ret5y || 0) >= 0 ? 'text-up' : 'text-down'}`}>
                        {etf.ret5y != null ? `${etf.ret5y >= 0 ? '+' : ''}${Math.round(etf.ret5y)}%` : '—'}
                      </div>
                      <div className="text-xs text-faint">5Y · AUM {formatAum(etf.aum || 0)}</div>
                    </div>
                  </Link>
                ))
              )}
            </div>
          </section>

          <p className="text-[11px] leading-relaxed text-faint">
            数据 = /data/etf-analyses.json。回报为区间累计,非年化;最大回撤=区间内峰值到谷底最大跌幅 · 非投资建议。
          </p>
        </div>
      </StockGodShell>
    );
  }

  return (
    <StockGodShell title="ETF" searchQuery={searchQuery} onSearchChange={setSearchQuery}>
      <div className="min-w-0 space-y-5 overflow-hidden">
        <header>
          <h1 className="text-[22px] font-semibold tracking-tight text-ink">ETF · 板块业绩</h1>
          <p className="mt-1.5 text-sm leading-relaxed text-muted">
            ETF 按跟踪的板块归类,展示各自的 1 年 / 5 年回报与最大回撤。低费率宽基适合长期持有;杠杆、反向产品波动与损耗较大,仅适合短线。
          </p>
        </header>

        <div className="min-w-0 space-y-2.5 overflow-hidden">
          <input
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
            placeholder="搜代码 / 名称..."
            className="w-full max-w-xs rounded-lg border border-line bg-surface px-3 py-2 text-sm text-ink outline-none placeholder:text-faint focus:border-accent/50"
          />

          <div className="-mx-1 flex gap-1.5 overflow-x-auto px-1 pb-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
            {CATEGORIES.map((category) => {
              const count = category.id === 'all' ? category.fallback : categories.get(category.id as ETFCategory) || category.fallback;
              const isActive = activeCategory === category.id;
              return (
                <button
                  key={category.id}
                  onClick={() => setActiveCategory(category.id)}
                  className={`shrink-0 whitespace-nowrap rounded-lg border px-3 py-2 text-[13px] font-medium transition ${
                    isActive ? 'border-accent/40 bg-accent/10 text-accent' : 'border-line bg-surface text-muted hover:text-ink'
                  }`}
                >
                  {category.label} {count}
                </button>
              );
            })}
          </div>

          <div className="inline-flex gap-0.5 rounded-lg border border-line bg-surface p-0.5">
            {SORT_OPTIONS.map((option) => (
              <button
                key={option.id}
                onClick={() => setSortBy(option.id)}
                className={`rounded-md px-3 py-1.5 text-[13px] font-medium transition ${
                  sortBy === option.id ? 'bg-surface-3 text-ink' : 'text-muted hover:text-ink'
                }`}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>

        {loading && (
          <div className="flex items-center justify-center rounded-xl border border-line bg-surface py-16">
            <LoadingSpinner size="lg" />
          </div>
        )}

        {error && <div className="rounded-xl border border-line bg-surface p-8 text-center text-sm text-down">{error}</div>}

        {!loading && !error && (
          <div className="space-y-4">
            {sectors.map((sector) => (
              <SectorRow key={sector.id} sector={sector} />
            ))}
          </div>
        )}

        <footer className="pt-2 text-[11px] leading-relaxed text-faint">
          数据 = Nasdaq(AUM/费率 + 5年日线算回报与最大回撤)。4533 只 ETF、47 个板块。回报为区间累计,非年化;最大回撤=区间内峰值到谷底最大跌幅 · 判决是费率+类型的机械映射 · 非投资建议。
        </footer>
      </div>
    </StockGodShell>
  );
};
