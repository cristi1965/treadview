import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { StockGodShell } from '../components/layout/StockGodShell';
import { EmptyState } from '../components/common';
import { WatchlistCard, HoldingsCard } from '../components/portfolio';
import { usePortfolioStore } from '../stores/portfolioStore';
import { PortfolioTab } from '../types/portfolio';
import { findUsStock, loadUsMarketStocks } from '../utils/stockgodData';
import { useDebounce } from '../hooks/useDebounce';
import { Stock } from '../types/stocks';
import { get } from '../utils/api';
import { useUiStore } from '../stores/uiStore';

export const Portfolio: React.FC = () => {
  const navigate = useNavigate();
  const language = useUiStore((s) => s.language);
  const [activeTab, setActiveTab] = useState<PortfolioTab>('watchlist');
  const {
    watchlist,
    holdings,
    addToWatchlist,
    removeFromWatchlist,
    isInWatchlist,
    addHolding,
    removeHolding,
  } = usePortfolioStore();
  const [query, setQuery] = useState('');
  const [hits, setHits] = useState<Stock[]>([]);
  const debounced = useDebounce(query, 250);

  const [hSymbol, setHSymbol] = useState('');
  const [hName, setHName] = useState('');
  const [hQty, setHQty] = useState('100');
  const [hCost, setHCost] = useState('');
  const [hNotes, setHNotes] = useState('');
  const [quotes, setQuotes] = useState<Record<string, { price: number; pct: number }>>({});

  const currentDate = new Date().toLocaleDateString(language === 'en' ? 'en-US' : 'zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  });

  useEffect(() => {
    let cancelled = false;
    (async () => {
      if (!debounced.trim()) {
        setHits([]);
        return;
      }
      const all = await loadUsMarketStocks().catch(() => []);
      if (cancelled) return;
      const q = debounced.toLowerCase();
      setHits(
        all
          .filter((s) => s.symbol.toLowerCase().includes(q) || s.name.toLowerCase().includes(q))
          .slice(0, 8)
      );
    })();
    return () => {
      cancelled = true;
    };
  }, [debounced]);

  useEffect(() => {
    const syms = [
      ...new Set([...watchlist.map((w) => w.symbol), ...holdings.map((h) => h.symbol)]),
    ];
    if (!syms.length) {
      setQuotes({});
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        const resp = await get<{ quotes: Record<string, { price: number; pct: number }> }>(
          `/api/quote?syms=${syms.slice(0, 40).join(',')}`
        );
        if (!cancelled && resp?.quotes) setQuotes(resp.quotes);
      } catch {
        /* ignore */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [watchlist, holdings]);

  const watchedSymbols = useMemo(() => new Set(watchlist.map((w) => w.symbol)), [watchlist]);

  const enrichedWatchlist = useMemo(
    () =>
      watchlist.map((item) => ({
        ...item,
        price: quotes[item.symbol]?.price ?? item.price,
        changePercent: quotes[item.symbol]?.pct ?? item.changePercent,
      })),
    [watchlist, quotes]
  );

  const enrichedHoldings = useMemo(
    () =>
      holdings.map((item) => {
        const price = quotes[item.symbol]?.price;
        const currentValue = price !== undefined ? price * item.quantity : undefined;
        const costBasis = item.avgCost * item.quantity;
        const profitLoss = currentValue !== undefined ? currentValue - costBasis : undefined;
        const profitLossPercent =
          profitLoss !== undefined && costBasis > 0 ? (profitLoss / costBasis) * 100 : undefined;
        return { ...item, currentValue, profitLoss, profitLossPercent };
      }),
    [holdings, quotes]
  );

  const holdingsSummary = useMemo(() => {
    let cost = 0;
    let value = 0;
    for (const h of enrichedHoldings) {
      cost += h.avgCost * h.quantity;
      if (h.currentValue !== undefined) value += h.currentValue;
    }
    const pl = value - cost;
    const plPct = cost > 0 ? (pl / cost) * 100 : 0;
    return { cost, value, pl, plPct };
  }, [enrichedHoldings]);

  const submitHolding = async (e: React.FormEvent) => {
    e.preventDefault();
    const symbol = hSymbol.trim().toUpperCase();
    const quantity = Number(hQty);
    const avgCost = Number(hCost);
    if (!symbol || !quantity || !avgCost) return;
    let name = hName.trim();
    if (!name) {
      const found = await findUsStock(symbol);
      name = found?.name || symbol;
    }
    addHolding({
      symbol,
      name,
      quantity,
      avgCost,
      purchaseDate: new Date(),
      notes: hNotes.trim() || undefined,
    });
    setHSymbol('');
    setHName('');
    setHQty('100');
    setHCost('');
    setHNotes('');
  };

  const t = language === 'en'
    ? {
        title: 'My portfolio',
        sub: 'Watchlist + holdings, all here.',
        local: 'Stored only in this browser (localStorage)',
        today: 'Today',
        watch: 'Watchlist',
        hold: 'Holdings',
        addWatch: 'Add to watchlist',
        searchPh: 'Search symbol / name…',
        watched: 'Watching',
        add: 'Add',
        emptyWatch: 'Watchlist is empty',
        emptyWatchDesc: 'Search above, or star symbols on the scanner.',
        goScan: 'Open scanner',
        addHold: 'Add holding',
        qty: 'Shares',
        cost: 'Avg cost',
        notes: 'Notes (optional)',
        save: 'Save holding',
        emptyHold: 'No holdings yet',
        emptyHoldDesc: 'Record paper or real positions locally. Quotes refresh via /api/quote.',
        costLabel: 'Cost basis',
        mktLabel: 'Mark-to-market',
        plLabel: 'P&L',
      }
    : {
        title: '我的组合',
        sub: '观察列表 + 持仓,都在这里。观察仅存于你这个浏览器（localStorage）',
        local: '',
        today: '今日',
        watch: '观察列表',
        hold: '持仓',
        addWatch: '添加观察',
        searchPh: '搜代码 / 名称加入观察…',
        watched: '已观察',
        add: '加入',
        emptyWatch: '观察列表为空',
        emptyWatchDesc: '去 全市场扫描 找感兴趣的标的,点 ☆ 加入。',
        goScan: '去全市场扫描',
        addHold: '添加持仓',
        qty: '数量',
        cost: '成本价',
        notes: '备注（可选）',
        save: '保存持仓',
        emptyHold: '还没有持仓',
        emptyHoldDesc: '本地记录纸面或真实仓位。价格通过 /api/quote 刷新。',
        costLabel: '成本',
        mktLabel: '市值',
        plLabel: '盈亏',
      };

  return (
    <StockGodShell title={language === 'en' ? 'Portfolio' : '我的'}>
      <div className="mx-auto max-w-[1104px]">
        <header className="mb-8 flex items-baseline justify-between border-b border-line pb-6">
          <div>
            <h1 className="mb-2 text-[28px] font-semibold text-ink">{t.title}</h1>
            <p className="text-sm text-muted">{t.sub}</p>
            {t.local ? <p className="mt-1 text-xs text-faint">{t.local}</p> : null}
          </div>
          <div className="text-sm text-faint">
            {t.today} {currentDate}
          </div>
        </header>

        <div className="mb-6 inline-flex rounded-lg border border-line bg-surface p-1">
          <button
            className={`rounded-md px-5 py-2 text-sm font-medium transition ${
              activeTab === 'watchlist' ? 'bg-surface-3 text-ink' : 'text-muted hover:text-ink'
            }`}
            onClick={() => setActiveTab('watchlist')}
          >
            {t.watch}
          </button>
          <button
            className={`rounded-md px-5 py-2 text-sm font-medium transition ${
              activeTab === 'holdings' ? 'bg-surface-3 text-ink' : 'text-muted hover:text-ink'
            }`}
            onClick={() => setActiveTab('holdings')}
          >
            {t.hold}
          </button>
        </div>

        {activeTab === 'watchlist' && (
          <div className="space-y-5">
            <section className="rounded-xl border border-line bg-surface p-4">
              <div className="mb-2 text-sm font-semibold text-ink">{t.addWatch}</div>
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t.searchPh}
                className="w-full max-w-md rounded-lg border border-line bg-base px-3 py-2 text-sm text-ink outline-none placeholder:text-faint focus:border-accent/50"
              />
              {hits.length > 0 && (
                <div className="mt-2 divide-y divide-line/60 overflow-hidden rounded-lg border border-line">
                  {hits.map((stock) => {
                    const watched = watchedSymbols.has(stock.symbol) || isInWatchlist(stock.symbol);
                    return (
                      <div key={stock.symbol} className="flex items-center gap-3 px-3 py-2">
                        <button
                          className="min-w-0 flex-1 text-left"
                          onClick={() => navigate(`/stock/${stock.symbol}`)}
                        >
                          <div className="font-mono text-sm font-semibold text-ink">{stock.symbol}</div>
                          <div className="truncate text-xs text-muted">{stock.name}</div>
                        </button>
                        <button
                          disabled={watched}
                          onClick={() => addToWatchlist(stock.symbol, stock.name)}
                          className={`rounded-md border px-3 py-1.5 text-xs font-semibold transition ${
                            watched
                              ? 'cursor-default border-line text-faint'
                              : 'border-accent/40 bg-accent/10 text-accent hover:bg-accent/15'
                          }`}
                        >
                          {watched ? t.watched : t.add}
                        </button>
                      </div>
                    );
                  })}
                </div>
              )}
            </section>

            {enrichedWatchlist.length === 0 ? (
              <EmptyState
                title={t.emptyWatch}
                description={
                  <>
                    {t.emptyWatchDesc}{' '}
                    <a href="/scan" className="text-accent underline">
                      /scan
                    </a>
                  </>
                }
                action={{
                  label: t.goScan,
                  onClick: () => navigate('/scan'),
                }}
              />
            ) : (
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
                {enrichedWatchlist.map((item) => (
                  <WatchlistCard
                    key={item.symbol}
                    item={item}
                    onRemove={() => removeFromWatchlist(item.symbol)}
                    onClick={() => navigate(`/stock/${item.symbol}`)}
                  />
                ))}
              </div>
            )}
          </div>
        )}

        {activeTab === 'holdings' && (
          <div className="space-y-5">
            {enrichedHoldings.length > 0 && (
              <div className="grid grid-cols-3 gap-3">
                <div className="rounded-xl border border-line bg-surface p-4">
                  <div className="text-xs text-muted">{t.costLabel}</div>
                  <div className="mt-1 font-mono text-lg text-ink">${holdingsSummary.cost.toFixed(2)}</div>
                </div>
                <div className="rounded-xl border border-line bg-surface p-4">
                  <div className="text-xs text-muted">{t.mktLabel}</div>
                  <div className="mt-1 font-mono text-lg text-ink">${holdingsSummary.value.toFixed(2)}</div>
                </div>
                <div className="rounded-xl border border-line bg-surface p-4">
                  <div className="text-xs text-muted">{t.plLabel}</div>
                  <div
                    className={`mt-1 font-mono text-lg ${
                      holdingsSummary.pl >= 0 ? 'text-up' : 'text-down'
                    }`}
                  >
                    ${holdingsSummary.pl.toFixed(2)} ({holdingsSummary.plPct.toFixed(2)}%)
                  </div>
                </div>
              </div>
            )}

            <section className="rounded-xl border border-line bg-surface p-4">
              <div className="mb-3 text-sm font-semibold text-ink">{t.addHold}</div>
              <form onSubmit={submitHolding} className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                <input
                  value={hSymbol}
                  onChange={(e) => setHSymbol(e.target.value)}
                  placeholder="NVDA"
                  required
                  className="rounded-lg border border-line bg-base px-3 py-2 font-mono text-sm text-ink outline-none focus:border-accent/50"
                />
                <input
                  value={hName}
                  onChange={(e) => setHName(e.target.value)}
                  placeholder={language === 'en' ? 'Name (optional)' : '名称（可选）'}
                  className="rounded-lg border border-line bg-base px-3 py-2 text-sm text-ink outline-none focus:border-accent/50"
                />
                <input
                  value={hQty}
                  onChange={(e) => setHQty(e.target.value)}
                  placeholder={t.qty}
                  type="number"
                  min="0"
                  step="any"
                  required
                  className="rounded-lg border border-line bg-base px-3 py-2 font-mono text-sm text-ink outline-none focus:border-accent/50"
                />
                <input
                  value={hCost}
                  onChange={(e) => setHCost(e.target.value)}
                  placeholder={t.cost}
                  type="number"
                  min="0"
                  step="any"
                  required
                  className="rounded-lg border border-line bg-base px-3 py-2 font-mono text-sm text-ink outline-none focus:border-accent/50"
                />
                <input
                  value={hNotes}
                  onChange={(e) => setHNotes(e.target.value)}
                  placeholder={t.notes}
                  className="rounded-lg border border-line bg-base px-3 py-2 text-sm text-ink outline-none focus:border-accent/50 sm:col-span-2"
                />
                <button
                  type="submit"
                  className="rounded-lg bg-accent px-4 py-2 text-sm font-semibold text-black transition hover:opacity-90 sm:col-span-2 lg:col-span-1"
                >
                  {t.save}
                </button>
              </form>
            </section>

            {enrichedHoldings.length === 0 ? (
              <EmptyState title={t.emptyHold} description={t.emptyHoldDesc} />
            ) : (
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
                {enrichedHoldings.map((item) => (
                  <HoldingsCard
                    key={item.symbol}
                    item={item}
                    onRemove={() => removeHolding(item.symbol)}
                    onClick={() => navigate(`/stock/${item.symbol}`)}
                  />
                ))}
              </div>
            )}
          </div>
        )}

        <footer className="mt-16 space-y-2 border-t border-line pt-6 text-center text-xs text-faint">
          <div>我不是神 · 五方独立判读 · v0.6</div>
          <div className="text-sm font-semibold text-ink">Not a Stock God</div>
          <div>你不是神,但神陪你一起看股票</div>
          <div>本站不面向中国大陆用户 · This site is not intended for users in mainland China.</div>
          <div className="mx-auto max-w-3xl leading-relaxed">
            所有内容为信息整理与个人研究记录,非投资建议,不构成对任何证券、平台或交易所的要约或背书。股票、加密货币、代币化资产均涉及重大风险,可能损失全部本金。风险请自行分辨与承担。页面含邀请链接(含返佣)。交易前请自行研究并确认所在司法管辖区的合规性。
          </div>
          <div className="flex flex-wrap items-center justify-center gap-3 pt-2">
            <a href="/about" className="hover:text-ink">关于 / 方法论</a>
            <a href="/terms" className="hover:text-ink">服务条款</a>
            <a href="/privacy" className="hover:text-ink">隐私政策</a>
            <a href="/how-to-buy" className="hover:text-ink">如何买</a>
          </div>
        </footer>
      </div>
    </StockGodShell>
  );
};
