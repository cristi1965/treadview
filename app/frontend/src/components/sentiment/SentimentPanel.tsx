import React, { useEffect, useState } from 'react';
import { APIResponseMeta, getWithMeta } from '../../utils/api';
import { assessDataTrust, DataTrustAssessment } from '../../utils/dataTrust';
import { I18nKey, useI18n } from '../../i18n';
import { DataStatus } from '../common';

interface Gauge {
  id: string;
  name: string;
  value: number;
  pct: number;
  unit: string;
  zone: string;
  tip: string;
  thresholds?: number[];
  extra?: string;
}

interface Panel {
  label: string;
  gauges: Gauge[];
}

interface SentimentData {
  ts: string;
  us: Panel;
  cn: Panel;
}

const ZONE_STYLE: Record<string, { bg: string; text: string; labelKey: I18nKey }> = {
  panic:       { bg: 'bg-down/15 border-down/25', text: 'text-down', labelKey: 'sent.panic' },
  restrictive: { bg: 'bg-down/15 border-down/25', text: 'text-down', labelKey: 'sent.restrictive' },
  elevated:    { bg: 'bg-accent/15 border-accent/25', text: 'text-accent', labelKey: 'sent.elevated' },
  hot:         { bg: 'bg-accent/15 border-accent/25', text: 'text-accent', labelKey: 'sent.hot' },
  greed:       { bg: 'bg-accent/15 border-accent/25', text: 'text-accent', labelKey: 'sent.greed' },
  neutral:     { bg: 'bg-surface-2 border-line', text: 'text-muted', labelKey: 'sent.neutral' },
  normal:      { bg: 'bg-surface-2 border-line', text: 'text-muted', labelKey: 'sent.normal' },
  positive:    { bg: 'bg-up/15 border-up/25', text: 'text-up', labelKey: 'sent.positive' },
  loose:       { bg: 'bg-up/15 border-up/25', text: 'text-up', labelKey: 'sent.loose' },
  complacent:  { bg: 'bg-surface-3 border-line', text: 'text-faint', labelKey: 'sent.complacent' },
  weak:        { bg: 'bg-surface-3 border-line', text: 'text-faint', labelKey: 'sent.weak' },
  strong:      { bg: 'bg-down/15 border-down/25', text: 'text-down', labelKey: 'sent.strong' },
};

function fmtValue(g: Gauge): string {
  if (g.id === 'limit_up' && g.extra) return `${g.value} / ${g.extra}`;
  if (g.value >= 10000) return `${(g.value / 10000).toFixed(2)}0k`;
  if (g.value >= 1000 && !g.unit.includes('%')) return g.value.toLocaleString('en-US', { maximumFractionDigits: 0 });
  if (Number.isInteger(g.value)) return String(g.value);
  return g.value.toFixed(g.value < 10 ? 2 : g.value < 100 ? 1 : 0);
}

const GaugeRow: React.FC<{ g: Gauge }> = ({ g }) => {
  const { t } = useI18n();
  const style = ZONE_STYLE[g.zone] || ZONE_STYLE.neutral;
  return (
    <div className="group flex items-center gap-2 rounded-md px-2 py-1.5 transition hover:bg-surface-2" title={g.tip}>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <span className="truncate text-xs text-muted">{g.name}</span>
          <span className={`shrink-0 rounded border px-1 py-px text-[9px] font-medium ${style.bg} ${style.text}`}>
            {t(style.labelKey)}
          </span>
        </div>
      </div>
      <div className="flex shrink-0 items-baseline gap-1.5 text-right">
        <span className="font-mono text-xs font-semibold tabular-nums text-ink">
          {fmtValue(g)}
        </span>
        {g.unit && <span className="text-[10px] text-faint">{g.unit}</span>}
        {g.pct !== 0 && (
          <span className={`font-mono text-[10px] tabular-nums ${g.pct > 0 ? 'text-up' : 'text-down'}`}>
            {g.pct > 0 ? '+' : ''}{g.pct.toFixed(2)}%
          </span>
        )}
      </div>
    </div>
  );
};

type Tab = 'us' | 'cn';

export const SentimentPanel: React.FC<{ defaultTab?: Tab }> = ({ defaultTab = 'us' }) => {
  const { t } = useI18n();
  const [data, setData] = useState<SentimentData | null>(null);
  const [tab, setTab] = useState<Tab>(defaultTab);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [trust, setTrust] = useState<DataTrustAssessment | null>(null);
  const [meta, setMeta] = useState<APIResponseMeta | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        setError(null);
        const response = await getWithMeta<SentimentData>('/api/sentiment');
        if (!cancelled) {
          setData(response.data);
          setMeta(response.meta);
          setTrust(assessDataTrust(response.meta, Boolean(response.data.us?.gauges?.length && response.data.cn?.gauges?.length)));
        }
      } catch (err) {
        if (!cancelled) {
          setData(null);
          setMeta(null);
          setTrust(null);
          setError(err instanceof Error ? err.message : '情绪面板暂不可用');
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const panel = data && trust?.state !== 'unavailable' ? (tab === 'us' ? data.us : data.cn) : null;

  const vix = trust?.state !== 'unavailable' ? data?.us.gauges.find((g) => g.id === 'vix') : undefined;
  const fearLevel = trust?.state !== 'unavailable' ? data?.us.gauges.find((g) => g.id === 'fgi') : undefined;

  return (
    <div className="rounded-xl border border-line bg-surface">
      <div className="flex items-center justify-between border-b border-line px-4 py-3">
        <div className="flex items-center gap-2">
          <span className="mr-0.5 font-mono text-[10px] uppercase tracking-wider text-faint">{t('sent.tab')}</span>
          {vix && (
            <span className={`rounded border px-1.5 py-0.5 font-mono text-[10px] font-semibold ${
              vix.value >= 30 ? 'border-down/25 bg-down/15 text-down' :
              vix.value >= 20 ? 'border-accent/25 bg-accent/15 text-accent' :
              'border-up/25 bg-up/15 text-up'
            }`}>
              VIX {vix.value.toFixed(1)}
            </span>
          )}
          {fearLevel && (
            <span className={`rounded border px-1.5 py-0.5 font-mono text-[10px] font-semibold ${
              fearLevel.value >= 75 ? 'border-down/25 bg-down/15 text-down' :
              fearLevel.value >= 55 ? 'border-accent/25 bg-accent/15 text-accent' :
              fearLevel.value >= 45 ? 'border-line bg-surface-2 text-muted' :
              'border-up/25 bg-up/15 text-up'
            }`}>
              {t('sent.fear', { n: fearLevel.value })}
            </span>
          )}
        </div>
      </div>

      <div className="flex gap-0.5 border-b border-line px-3 py-2">
        {([['us', 'sent.us'], ['cn', 'sent.cn']] as [Tab, I18nKey][]).map(([id, key]) => (
          <button
            key={id}
            onClick={() => setTab(id)}
            className={`rounded-md px-3 py-1.5 text-xs font-medium transition ${
              tab === id ? 'bg-surface-3 text-ink' : 'text-muted hover:bg-surface-2 hover:text-ink'
            }`}
          >
            {t(key)}
          </button>
        ))}
      </div>

      <div className="p-1.5">
        {loading && (
          <div className="py-6 text-center text-xs text-muted">{t('sent.loading')}</div>
        )}
        {!loading && error && (
          <div className="flex justify-center py-6 text-center text-xs text-muted">
            <DataStatus state="unavailable" message={error} compact />
          </div>
        )}
        {!loading && !error && !panel && (
          <div className="py-6 text-center text-xs text-muted">{t('sent.empty')}</div>
        )}
        {!loading && !error && trust && trust.state !== 'live' && (
          <div className="px-2 py-2">
            <DataStatus state={trust.state} dataTime={meta?.dataTime} source={meta?.source} message={trust.reason} compact />
          </div>
        )}
        {!loading && panel && trust?.state !== 'unavailable' && panel.gauges.map((g) => (
          <GaugeRow key={g.id} g={g} />
        ))}
      </div>

      {!loading && panel && (
        <div className="border-t border-line px-3 py-2">
          <p className="text-[9px] leading-relaxed text-faint">
            {tab === 'us' ? t('sent.usHint') : t('sent.cnHint')}
          </p>
        </div>
      )}
    </div>
  );
};
