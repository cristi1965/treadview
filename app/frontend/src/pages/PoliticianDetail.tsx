import React, { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { StockGodShell } from '../components/layout/StockGodShell';
import { apiUrl } from '../utils/api';

type TradeRow = {
  action: string;
  symbol: string;
  amount: string;
  date: string;
  title?: string;
};

type PoliticianDetail = {
  id: string;
  slug: string;
  name: string;
  party: 'D' | 'R' | string;
  state: string;
  district: string;
  title: string;
  tradeCount: number;
  latestTrade?: TradeRow;
  trades: TradeRow[];
};

export const PoliticianDetailPage: React.FC = () => {
  const { slug = '' } = useParams<{ slug: string }>();
  const navigate = useNavigate();
  const [detail, setDetail] = useState<PoliticianDetail | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      try {
        const res = await fetch(apiUrl(`/api/whales/congress/${encodeURIComponent(slug)}`));
        if (!res.ok) {
          if (!cancelled) setDetail(null);
          return;
        }
        const data = (await res.json()) as PoliticianDetail;
        if (!cancelled) setDetail(data);
      } catch {
        if (!cancelled) setDetail(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [slug]);

  if (loading) {
    return (
      <StockGodShell title="国会交易">
        <div className="flex min-h-[420px] items-center justify-center text-muted">加载中...</div>
      </StockGodShell>
    );
  }

  if (!detail) {
    return (
      <StockGodShell title="国会交易">
        <div className="rounded-xl border border-line bg-surface p-10 text-center text-muted">
          未找到该议员
          <div className="mt-3">
            <button type="button" className="text-accent underline" onClick={() => navigate('/whales')}>
              返回聪明钱
            </button>
          </div>
        </div>
      </StockGodShell>
    );
  }

  const partyLabel = detail.party === 'D' ? '民主党' : detail.party === 'R' ? '共和党' : detail.party;

  return (
    <StockGodShell title={detail.name}>
      <div className="mx-auto max-w-[960px] text-foreground">
        <div className="mb-3 flex items-center gap-2 text-xs">
          <button type="button" className="text-muted hover:text-ink" onClick={() => navigate('/whales')}>
            聪明钱
          </button>
          <span className="text-faint">/</span>
          <span className="text-muted">国会</span>
          <span className="text-faint">/</span>
          <span className="text-ink">{detail.name}</span>
        </div>

        <header className="mb-5 border-b border-line pb-4">
          <h1 className="text-2xl font-semibold tracking-tight text-ink sm:text-3xl">{detail.name}</h1>
          <div className="mt-2 flex flex-wrap items-center gap-2 text-sm text-muted">
            <span
              className={`inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs ${
                detail.party === 'D' ? 'bg-[#3B82F6]/15 text-[#3B82F6]' : 'bg-[#EF4444]/15 text-[#EF4444]'
              }`}
            >
              {partyLabel}
            </span>
            <span>{detail.title || 'Representative'}</span>
            <span>·</span>
            <span>
              {detail.state} {detail.district}
            </span>
            <span>·</span>
            <span>{detail.tradeCount} 笔披露</span>
          </div>
        </header>

        <section className="overflow-hidden rounded-xl border border-line bg-surface">
          <div className="border-b border-line px-4 py-3 text-sm font-semibold text-ink">交易明细</div>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[560px] text-left text-sm">
              <thead className="bg-surface-2 text-xs text-faint">
                <tr>
                  <th className="px-4 py-2 font-medium">日期</th>
                  <th className="px-4 py-2 font-medium">方向</th>
                  <th className="px-4 py-2 font-medium">标的</th>
                  <th className="px-4 py-2 font-medium">金额</th>
                </tr>
              </thead>
              <tbody>
                {(detail.trades || []).map((t, i) => (
                  <tr key={`${t.date}-${t.symbol}-${i}`} className="border-t border-line/70">
                    <td className="px-4 py-2.5 text-muted">{t.date || '—'}</td>
                    <td
                      className={`px-4 py-2.5 font-medium ${
                        t.action === 'sell' || t.action === 'SELL' ? 'text-[#f6465d]' : 'text-[#2ebd85]'
                      }`}
                    >
                      {(t.action || '').toUpperCase() === 'SELL' || t.action === 'sell' ? 'SELL' : 'BUY'}
                    </td>
                    <td className="px-4 py-2.5">
                      <button
                        type="button"
                        className="font-semibold text-ink hover:text-accent"
                        onClick={() => navigate(`/stock/${t.symbol}?market=us`)}
                      >
                        {t.symbol}
                      </button>
                    </td>
                    <td className="px-4 py-2.5 font-mono text-xs text-muted">{t.amount || '—'}</td>
                  </tr>
                ))}
                {!detail.trades?.length && (
                  <tr>
                    <td colSpan={4} className="px-4 py-8 text-center text-muted">
                      暂无交易记录
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </StockGodShell>
  );
};
