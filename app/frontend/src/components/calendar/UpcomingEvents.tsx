import React, { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { APIResponseMeta, getWithMeta } from '../../utils/api';
import { assessDataTrust, DataTrustAssessment } from '../../utils/dataTrust';
import { I18nKey, useI18n } from '../../i18n';
import { DataStatus } from '../common';

interface EarningsDetail {
  ticker: string;
  companyName: string;
  expectedEPS: number;
  marketCap: string;
  time: string;
}

interface MarketEvent {
  id: string;
  date: string;
  dayOfWeek: string;
  isToday?: boolean;
  type: 'macro' | 'earnings' | 'policy' | 'holiday';
  isImportant?: boolean;
  title: string;
  description?: string;
  relatedTickers?: string[];
  earnings?: EarningsDetail[];
}

type TimeRange = 'this-week' | 'next-week' | 'this-month';

const TYPE_BADGE: Record<string, { labelKey: I18nKey; cls: string }> = {
  macro:    { labelKey: 'cal.macro', cls: 'border-line bg-surface-3 text-muted' },
  earnings: { labelKey: 'cal.earn', cls: 'border-accent/25 bg-accent/15 text-accent' },
  policy:   { labelKey: 'cal.policy', cls: 'border-down/25 bg-down/15 text-down' },
  holiday:  { labelKey: 'cal.holiday', cls: 'border-line bg-surface-2 text-faint' },
};

function getWeekRange(offset: number): [string, string] {
  const now = new Date();
  const day = now.getDay();
  const mondayOffset = day === 0 ? -6 : 1 - day;
  const monday = new Date(now);
  monday.setDate(now.getDate() + mondayOffset + offset * 7);
  const friday = new Date(monday);
  friday.setDate(monday.getDate() + 6);
  return [fmt(monday), fmt(friday)];
}

function getMonthRange(): [string, string] {
  const now = new Date();
  const start = fmt(now);
  const end = new Date(now.getFullYear(), now.getMonth() + 1, 0);
  return [start, fmt(end)];
}

function fmt(d: Date): string {
  return d.toISOString().slice(0, 10);
}

function fmtShort(dateStr: string): string {
  const d = new Date(dateStr + 'T00:00:00');
  return `${d.getMonth() + 1}/${d.getDate()}`;
}

export const UpcomingEvents: React.FC = () => {
  const { t } = useI18n();
  const [events, setEvents] = useState<MarketEvent[]>([]);
  const [range, setRange] = useState<TimeRange>('this-week');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [trust, setTrust] = useState<DataTrustAssessment | null>(null);
  const [meta, setMeta] = useState<APIResponseMeta | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      setError('');
      try {
        const response = await getWithMeta<{ events: MarketEvent[] }>('/api/market/calendar');
        if (!cancelled) {
          const nextEvents = response.data.events || [];
          setEvents(nextEvents);
          setMeta(response.meta);
          setTrust(assessDataTrust(response.meta, nextEvents.length > 0));
        }
      } catch (err) {
        if (!cancelled) {
          setEvents([]);
          setMeta(null);
          setTrust(null);
          setError(err instanceof Error ? err.message : '市场日历暂不可用');
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const filtered = useMemo(() => {
    let start: string, end: string;
    if (range === 'this-week') [start, end] = getWeekRange(0);
    else if (range === 'next-week') [start, end] = getWeekRange(1);
    else [start, end] = getMonthRange();
    const visibleEvents = trust?.state === 'unavailable' ? [] : events;
    return visibleEvents.filter((e) => e.date >= start && e.date <= end);
  }, [events, range, trust]);

  const importantCount = filtered.filter((e) => e.isImportant).length;

  return (
    <div className="rounded-xl border border-line bg-surface">
      <div className="flex items-center justify-between border-b border-line px-4 py-3">
        <span className="font-mono text-[10px] uppercase tracking-wider text-faint">{t('cal.title')}</span>
        {importantCount > 0 && (
          <span className="rounded border border-down/25 bg-down/15 px-1.5 py-0.5 text-[10px] font-semibold text-down">
            {t('cal.hot', { n: importantCount })}
          </span>
        )}
      </div>

      <div className="flex gap-0.5 border-b border-line px-3 py-2">
        {([
          ['this-week', 'cal.thisWeek'],
          ['next-week', 'cal.nextWeek'],
          ['this-month', 'cal.thisMonth'],
        ] as [TimeRange, I18nKey][]).map(([id, key]) => (
          <button
            key={id}
            onClick={() => setRange(id)}
            className={`rounded-md px-3 py-1.5 text-xs font-medium transition ${
              range === id ? 'bg-surface-3 text-ink' : 'text-muted hover:bg-surface-2 hover:text-ink'
            }`}
          >
            {t(key)}
          </button>
        ))}
      </div>

      <div className="max-h-[380px] overflow-y-auto p-2">
        {loading && (
          <div className="py-6 text-center text-xs text-muted">{t('cal.loading')}</div>
        )}

        {!loading && error && (
          <div className="py-4">
            <DataStatus state="unavailable" message={error} compact />
          </div>
        )}

        {!loading && !error && trust && trust.state !== 'live' && (
          <div className="py-2">
            <DataStatus state={trust.state} dataTime={meta?.dataTime} source={meta?.source} message={trust.reason} compact />
          </div>
        )}

        {!loading && !error && trust?.state !== 'unavailable' && filtered.length === 0 && (
          <div className="py-6 text-center text-xs text-muted">
            {t('cal.empty', { range: t(range === 'this-week' ? 'cal.thisWeek' : range === 'next-week' ? 'cal.nextWeek' : 'cal.thisMonth') })}
          </div>
        )}

        {!loading && !error && trust?.state !== 'unavailable' && filtered.map((ev) => {
          const badge = TYPE_BADGE[ev.type] || TYPE_BADGE.macro;
          return (
            <div
              key={ev.id}
              className={`mb-1 rounded-md px-2.5 py-2 transition ${
                ev.isToday
                  ? 'border border-accent/25 bg-accent/5'
                  : 'hover:bg-surface-2'
              }`}
            >
              <div className="flex items-start gap-2">
                <div className="mt-0.5 w-8 shrink-0 text-center">
                  <div className="font-mono text-[11px] font-semibold tabular-nums text-ink">
                    {fmtShort(ev.date)}
                  </div>
                  <div className="text-[9px] text-faint">{ev.dayOfWeek}</div>
                </div>

                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-1.5">
                    {ev.isImportant && (
                      <span className="inline-block h-1.5 w-1.5 shrink-0 rounded-full bg-down" />
                    )}
                    <span className={`shrink-0 rounded border px-1 py-px text-[9px] font-medium ${badge.cls}`}>
                      {t(badge.labelKey)}
                    </span>
                    <span className="min-w-0 truncate text-xs font-medium text-ink">
                      {ev.title}
                    </span>
                  </div>

                  {ev.description && (
                    <p className="mt-0.5 text-[11px] leading-relaxed text-faint">{ev.description}</p>
                  )}

                  {ev.relatedTickers && ev.relatedTickers.length > 0 && (
                    <div className="mt-1 flex flex-wrap gap-1">
                      {ev.relatedTickers.slice(0, 4).map((t) => (
                        <Link
                          key={t}
                          to={`/stock/${t}`}
                          className="rounded bg-surface-2 px-1.5 py-0.5 font-mono text-[9px] font-medium text-accent transition hover:bg-surface-3"
                        >
                          {t}
                        </Link>
                      ))}
                    </div>
                  )}

                  {ev.earnings && ev.earnings.length > 0 && (
                    <div className="mt-1 space-y-0.5">
                      {ev.earnings.slice(0, 3).map((e) => (
                        <Link
                          key={e.ticker}
                          to={`/stock/${e.ticker}`}
                          className="flex items-center gap-1.5 rounded px-1 py-0.5 text-[10px] transition hover:bg-surface-2"
                        >
                          <span className="w-10 font-mono font-semibold text-ink">{e.ticker}</span>
                          <span className="min-w-0 flex-1 truncate text-faint">{e.companyName}</span>
                          <span className="shrink-0 font-mono text-muted">
                            EPS {e.expectedEPS > 0 ? `$${e.expectedEPS}` : `−$${Math.abs(e.expectedEPS)}`}
                          </span>
                        </Link>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};
