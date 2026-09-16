import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { getWithMeta, type APIResponseMeta } from '../../utils/api';
import { assessDataTrust, type DataTrustState } from '../../utils/dataTrust';
import { useI18n } from '../../i18n';
import { DataStatus } from '../common';

interface Mover {
  sym: string;
  price: number;
  pct: number;
}

interface MoversData {
  label?: string;
  gainers: Mover[];
  losers: Mover[];
  session?: string;
  ts?: number;
}

function formatMoverPrice(price: number): string {
  if (!Number.isFinite(price) || price <= 0) return '暂无价格';
  if (price < 0.01) return `$${price.toFixed(4)}`;
  if (price < 1) return `$${price.toFixed(3)}`;
  return `$${price.toFixed(2)}`;
}

export const MarketMovers: React.FC = () => {
  const { t } = useI18n();
  const [data, setData] = useState<MoversData | null>(null);
  const [meta, setMeta] = useState<APIResponseMeta | null>(null);
  const [state, setState] = useState<DataTrustState | 'loading'>('loading');
  const [error, setError] = useState('');
  const [reloadVersion, setReloadVersion] = useState(0);
  const [tab, setTab] = useState<'gainers' | 'losers'>('gainers');

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setState('loading');
      setError('');
      try {
        const result = await getWithMeta<MoversData>('/api/premarket-movers');
        const hasRows = Boolean(result.data.gainers?.length || result.data.losers?.length);
        const trust = assessDataTrust(result.meta, hasRows);
        if (cancelled) return;
        setMeta(result.meta);
        if (trust.state === 'unavailable') {
          setData(null);
          setState('unavailable');
          setError(trust.reason);
          return;
        }
        setData(result.data);
        setState(trust.state);
        setError(trust.reason);
      } catch (caught) {
        if (cancelled) return;
        setData(null);
        setMeta(null);
        setState('unavailable');
        setError(caught instanceof Error ? caught.message : '异动数据暂不可用');
      }
    })();
    return () => { cancelled = true; };
  }, [reloadVersion]);

  if (!data) {
    return (
      <div className="rounded-xl border border-line bg-surface px-4 py-3">
        <DataStatus
          state={state}
          label={state === 'loading' ? '异动数据加载中' : '异动数据不可用'}
          message={error}
          onRetry={() => setReloadVersion((value) => value + 1)}
        />
      </div>
    );
  }

  const list = tab === 'gainers' ? data.gainers : data.losers;

  return (
    <div className="rounded-xl border border-line bg-surface">
      <div className="flex items-center justify-between border-b border-line px-4 py-3">
        <span className="font-mono text-[10px] uppercase tracking-wider text-faint">
          {data.label || t('mv.title')}
        </span>
        {(data.session || state === 'stale') && (
          <span className="rounded border border-line bg-surface-2 px-1.5 py-0.5 font-mono text-[9px] text-faint">
            {state === 'stale' ? '历史快照' : data.session}
          </span>
        )}
        {meta?.dataTime || data.ts ? (
          <span className="font-mono text-[9px] text-faint">
            {new Date(meta?.dataTime || data.ts || 0).toLocaleString()}
          </span>
        ) : null}
      </div>

      {state === 'stale' && (
        <div className="border-b border-line px-4 py-2">
          <DataStatus state="stale" dataTime={meta?.dataTime} source={meta?.source} message={error} compact />
        </div>
      )}

      <div className="flex gap-0.5 border-b border-line px-3 py-2">
        <button
          onClick={() => setTab('gainers')}
          className={`rounded-md px-3 py-1.5 text-xs font-medium transition ${
            tab === 'gainers' ? 'bg-surface-3 text-ink' : 'text-muted hover:bg-surface-2 hover:text-ink'
          }`}
        >
          {t('mv.gain')}
        </button>
        <button
          onClick={() => setTab('losers')}
          className={`rounded-md px-3 py-1.5 text-xs font-medium transition ${
            tab === 'losers' ? 'bg-surface-3 text-ink' : 'text-muted hover:bg-surface-2 hover:text-ink'
          }`}
        >
          {t('mv.loss')}
        </button>
      </div>

      <div className="p-1.5">
        {list.slice(0, 6).map((m, i) => (
          <Link
            key={m.sym}
            to={`/stock/${m.sym}`}
            className="flex items-center gap-2 rounded-md px-2 py-1.5 text-xs transition hover:bg-surface-2"
          >
            <span className="w-4 font-mono text-[10px] text-faint">{i + 1}</span>
            <span className="w-14 font-mono font-semibold text-ink">{m.sym}</span>
            <span className="min-w-0 flex-1 text-right font-mono tabular-nums text-muted">
              {formatMoverPrice(m.price)}
            </span>
            <span
              className={`w-16 text-right font-mono font-semibold tabular-nums ${
                m.pct >= 0 ? 'text-up' : 'text-down'
              }`}
            >
              {m.pct >= 0 ? '+' : ''}{m.pct.toFixed(2)}%
            </span>
          </Link>
        ))}
      </div>
    </div>
  );
};
