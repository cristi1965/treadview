import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { DataStatus } from '../common';
import { APIError, getWithMeta, type APIResponseMeta } from '../../utils/api';
import { assessDataTrust } from '../../utils/dataTrust';

export interface CnEtfRec {
  symbol: string;
  name: string;
  theme: string;
  why: string;
  risk: string;
  priority: number;
  price?: number;
  pct?: number;
  hasQuote?: boolean;
}

export interface CnStockPick {
  symbol: string;
  name: string;
  price?: number;
  pct?: number;
  mcapYi?: number;
  heat?: number;
  moat?: number;
  tag?: string;
  why?: string;
  layer?: string;
  industries?: string[];
  hasQuote?: boolean;
  rule?: string;
}

interface CnPicksResponse {
  disclaimer?: string;
  updated?: string;
  configUpdated?: string;
  etf_recs?: CnEtfRec[];
  hot?: CnStockPick[];
  conviction?: CnStockPick[];
  seed?: CnStockPick[];
  rules?: Record<string, string>;
}

const pctClass = (pct?: number) => {
  if (pct == null) return 'text-muted';
  if (pct > 0) return 'text-up';
  if (pct < 0) return 'text-down';
  return 'text-muted';
};

const StockLine: React.FC<{ s: CnStockPick; showHeat?: boolean }> = ({ s, showHeat }) => (
  <Link
    to={`/stock/${s.symbol}`}
    className="flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs transition hover:bg-surface-2"
  >
    <span className="w-14 shrink-0 font-mono font-semibold text-ink">{s.symbol}</span>
    <span className="min-w-0 flex-1 truncate text-muted">{s.name}</span>
    {s.tag ? <span className="shrink-0 rounded bg-surface-3 px-1.5 py-0.5 text-[10px] text-faint">{s.tag}</span> : null}
    {showHeat && s.heat != null ? <span className="w-8 text-right font-mono text-accent">{Math.round(s.heat)}</span> : null}
    {s.hasQuote ? (
      <span className={`w-14 shrink-0 text-right font-mono tabular-nums ${pctClass(s.pct)}`}>
        {s.pct != null ? `${s.pct >= 0 ? '+' : ''}${s.pct.toFixed(1)}%` : '—'}
      </span>
    ) : (
      <span className="w-14 shrink-0 text-right font-mono text-faint">—</span>
    )}
  </Link>
);

export const CnPicksPanel: React.FC<{ compact?: boolean }> = ({ compact }) => {
  const [data, setData] = useState<CnPicksResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [meta, setMeta] = useState<APIResponseMeta | null>(null);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await getWithMeta<CnPicksResponse>('/api/cn/picks');
      setData(result.data);
      setMeta(result.meta);
    } catch (err) {
      setError('A股精选加载失败，请检查本地后端');
      setMeta(err instanceof APIError ? err.meta || null : null);
      setData(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  if (error && !data) {
    return (
      <div className="rounded-xl border border-line bg-surface px-3 py-4 text-center text-xs text-muted">
        <h2 className="mb-2 text-sm font-semibold text-ink">A股工具箱 · ETF 推荐 / 热门 / 高信念</h2>
        <div>{error}</div>
        <button type="button" className="mt-2 rounded-md border border-line px-2 py-1 text-[11px] hover:bg-surface-2" onClick={() => void load()}>
          重试
        </button>
      </div>
    );
  }
  if (loading && !data) {
    return (
      <div className="rounded-xl border border-line bg-surface px-3 py-6 text-center text-xs text-faint">
        <h2 className="mb-2 text-sm font-semibold text-ink">A股工具箱 · ETF 推荐 / 热门 / 高信念</h2>
        加载 A股精选…
      </div>
    );
  }
  if (!data) return null;

  const trust = assessDataTrust(meta, Boolean(data.etf_recs?.length));
  if (trust.state !== 'live') {
    return (
      <div className="rounded-xl border border-line bg-surface px-4 py-4">
        <h2 className="mb-1 text-sm font-semibold text-ink">A股工具箱 · ETF 推荐 / 热门 / 高信念</h2>
        <p className="mb-2 text-xs text-muted">数据待核验</p>
        <DataStatus
          state={trust.state}
          label={trust.state === 'stale' ? 'A 股精选数据已过期' : 'A 股精选数据不可用'}
          dataTime={meta?.dataTime || 'unknown'}
          source={meta?.source}
          message={trust.reason}
          onRetry={() => void load()}
        />
        <p className="mt-3 text-xs leading-relaxed text-muted">行情未通过时间与完整性门禁，已隐藏热门排名、价格和涨跌幅。</p>
      </div>
    );
  }

  const etfs = data.etf_recs || [];
  const hot = data.hot || [];
  const conviction = data.conviction || [];
  const seed = data.seed || [];

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-accent/25 bg-accent/5 px-4 py-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="text-sm font-semibold text-ink">A股工具箱 · ETF 推荐 / 热门 / 高信念</div>
          <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto sm:justify-end">
            <DataStatus state="live" dataTime={meta?.dataTime} compact />
            <button
              type="button"
              className="rounded-md border border-line bg-surface px-2 py-0.5 text-[10px] text-muted hover:text-ink"
              onClick={() => void load()}
              disabled={loading}
            >
              {loading ? '刷新中…' : '刷新'}
            </button>
          </div>
        </div>
        <p className="mt-1 text-[11px] leading-relaxed text-muted">
          {data.disclaimer || '规则筛选，非投资建议。'}
          {!compact && data.rules?.conviction ? ` · ${data.rules.conviction}` : ''}
        </p>
        <p className="mt-1 break-all font-mono text-[9px] leading-relaxed text-faint">
          来源 {meta?.source || '未知'} · 规则版本 {data.configUpdated || data.updated || '未知'}
        </p>
      </div>

      <div className={`grid gap-4 ${compact ? 'lg:grid-cols-1' : 'lg:grid-cols-3'}`}>
        <section className="rounded-xl border border-line bg-surface p-4">
          <div className="mb-2 flex items-baseline justify-between">
            <h3 className="text-sm font-semibold text-ink">推荐 ETF</h3>
            <span className="font-mono text-[10px] text-faint">{etfs.length}</span>
          </div>
          <div className="space-y-2">
            {etfs.slice(0, compact ? 6 : 10).map((e) => (
              <div key={e.symbol} className="rounded-lg border border-line/70 bg-base px-2.5 py-2">
                <div className="flex items-center gap-2">
                  <Link to={`/stock/${e.symbol}`} className="font-mono text-xs font-semibold text-accent hover:underline">
                    {e.symbol}
                  </Link>
                  <span className="min-w-0 flex-1 truncate text-xs text-ink">{e.name}</span>
                  {e.hasQuote ? (
                    <span className={`font-mono text-[11px] tabular-nums ${pctClass(e.pct)}`}>
                      {e.pct != null ? `${e.pct >= 0 ? '+' : ''}${e.pct.toFixed(1)}%` : ''}
                    </span>
                  ) : null}
                </div>
                <div className="mt-1 flex flex-wrap gap-1.5 text-[10px]">
                  <span className="rounded bg-surface-3 px-1.5 py-0.5 text-faint">{e.theme}</span>
                  <span className="rounded bg-surface-3 px-1.5 py-0.5 text-faint">风险 {e.risk}</span>
                </div>
                <p className="mt-1 text-[11px] leading-relaxed text-muted">{e.why}</p>
              </div>
            ))}
          </div>
        </section>

        <section className="rounded-xl border border-line bg-surface p-4">
          <div className="mb-2 flex items-baseline justify-between">
            <h3 className="text-sm font-semibold text-ink">热门 A股</h3>
            <span className="font-mono text-[10px] text-faint">live</span>
          </div>
          <p className="mb-2 text-[10px] text-faint">{data.rules?.hot}</p>
          <div className="space-y-0.5">
            {hot.length === 0 ? (
              <div className="py-6 text-center text-xs text-faint">暂无满足条件的热门（或 CN live 未就绪）</div>
            ) : (
              hot.map((s) => <StockLine key={s.symbol} s={s} showHeat />)
            )}
          </div>
        </section>

        <section className="rounded-xl border border-line bg-surface p-4">
          <div className="mb-2 flex items-baseline justify-between">
            <h3 className="text-sm font-semibold text-ink">高信念筛选</h3>
            <span className="font-mono text-[10px] text-faint">moat≥4</span>
          </div>
          <p className="mb-2 text-[10px] text-faint">{data.rules?.conviction}</p>
          <div className="space-y-0.5">
            {conviction.length === 0 ? (
              <div className="py-4 text-center text-xs text-faint">脉冲宇宙暂无命中，见下方精选种子</div>
            ) : (
              conviction.map((s) => <StockLine key={s.symbol} s={s} showHeat />)
            )}
          </div>
          {seed.length > 0 ? (
            <>
              <div className="mb-1 mt-4 text-[10px] font-mono uppercase tracking-wider text-faint">精选种子</div>
              <div className="space-y-0.5">
                {seed.slice(0, compact ? 6 : 10).map((s) => (
                  <div key={s.symbol}>
                    <StockLine s={s} />
                    {s.why ? <p className="px-2 pb-1 text-[10px] leading-relaxed text-faint">{s.why}</p> : null}
                  </div>
                ))}
              </div>
            </>
          ) : null}
        </section>
      </div>
    </div>
  );
};
