import React, { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { StockGodShell } from '../components/layout/StockGodShell';
import { DataStatus } from '../components/common/DataStatus';
import { getWithMeta as apiGetWithMeta, type APIResponseMeta } from '../utils/api';

type TradeRow = {
  action: string;
  symbol: string;
  amount: string;
  date: string;
  title?: string;
  source: string;
  filingDate: string;
  sourceURL: string;
  filingId: string;
  verified: boolean;
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
  const [error, setError] = useState('');
  const [meta, setMeta] = useState<APIResponseMeta | null>(null);
  const [retryNonce, setRetryNonce] = useState(0);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      setError('');
      setMeta(null);
      try {
        const result = await apiGetWithMeta<PoliticianDetail>(`/api/whales/congress/${encodeURIComponent(slug)}`);
        if (!cancelled) {
          setDetail(result.data);
          setMeta(result.meta);
        }
      } catch (loadError) {
        if (!cancelled) {
          setDetail(null);
          setError(loadError instanceof Error ? loadError.message : '国会披露加载失败');
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [slug, retryNonce]);

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
          <h1 className="text-lg font-semibold text-ink">国会披露暂不可用</h1>
          <p className="mt-2 text-sm text-muted">{error || '未找到该议员。'}</p>
          <div className="mt-3">
            <button type="button" className="mr-4 text-accent underline" onClick={() => setRetryNonce((value) => value + 1)}>
              重试
            </button>
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

        <div className="mb-4">
          <DataStatus
            state={meta?.stale ? 'stale' : meta?.source ? 'live' : 'unavailable'}
            dataTime={meta?.dataTime}
            source={meta?.source}
            message={meta?.staleReason}
            onRetry={() => setRetryNonce((value) => value + 1)}
            compact
          />
        </div>

        {!detail.trades?.some((trade) => trade.verified) && (
          <div className="mb-4 rounded-xl border border-amber-500/35 bg-amber-500/5 px-4 py-3 text-sm text-amber-200">
            未核验历史样本：缺少可定位原文或申报标识，不作为投资信号。
          </div>
        )}

        <section className="overflow-hidden rounded-xl border border-line bg-surface">
          <div className="border-b border-line px-4 py-3 text-sm font-semibold text-ink">交易明细</div>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[900px] text-left text-sm">
              <thead className="bg-surface-2 text-xs text-faint">
                <tr>
                  <th className="px-4 py-2 font-medium">日期</th>
                  <th className="px-4 py-2 font-medium">方向</th>
                  <th className="px-4 py-2 font-medium">标的</th>
                  <th className="px-4 py-2 font-medium">金额</th>
                  <th className="px-4 py-2 font-medium">申报/来源</th>
                  <th className="px-4 py-2 font-medium">原文</th>
                </tr>
              </thead>
              <tbody>
                {(detail.trades || []).map((t, i) => (
                  <tr key={`${t.date}-${t.symbol}-${i}`} className={`border-t border-line/70 ${t.verified ? '' : 'bg-amber-500/5 opacity-75'}`}>
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
                    <td className="px-4 py-2.5 text-xs text-muted">
                      <div>{t.filingDate || 'unknown'}</div>
                      <div className="text-faint">{t.source || 'unknown'}</div>
                    </td>
                    <td className="px-4 py-2.5 text-xs">
                      {t.sourceURL && t.sourceURL !== 'unknown' ? (
                        <a href={t.sourceURL} target="_blank" rel="noreferrer" className="text-accent hover:underline">
                          {t.filingId && t.filingId !== 'unknown' ? t.filingId : '查看来源'}
                        </a>
                      ) : (
                        <span className="text-down">unknown</span>
                      )}
                    </td>
                  </tr>
                ))}
                {!detail.trades?.length && (
                  <tr>
                    <td colSpan={6} className="px-4 py-8 text-center text-muted">
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
