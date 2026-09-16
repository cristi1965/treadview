import React, { useMemo, useState } from 'react';
import { SegmentedControl } from '../common';
import { MarketEvent } from '../../types/reports';
import { useI18n } from '../../i18n';

type Scope = 'all' | 'important';

const importanceOf = (event: MarketEvent): number => {
  const raw = Math.round(Number(event.importance));
  if (Number.isFinite(raw) && raw >= 1) return Math.min(3, raw);
  return event.isImportant ? 3 : 1;
};

const numeric = (value?: string): number | null => {
  if (!value) return null;
  const match = value.replace(/,/g, '').match(/-?\d+(\.\d+)?/);
  if (!match) return null;
  const n = Number(match[0]);
  return Number.isFinite(n) ? n : null;
};

/** Actual above forecast reads green, below reads red — same convention as the tape. */
const actualTone = (event: MarketEvent): string => {
  const actual = numeric(event.actual);
  const forecast = numeric(event.forecast ?? event.previous);
  if (actual === null || forecast === null) return 'text-ios-label';
  if (actual > forecast) return 'text-ios-green';
  if (actual < forecast) return 'text-ios-red';
  return 'text-ios-label';
};

const ImpactDots: React.FC<{ level: number }> = ({ level }) => (
  <span className="flex items-center gap-[3px]" aria-label={`impact ${level}`}>
    {[1, 2, 3].map((slot) => (
      <span
        key={slot}
        className={`h-[5px] w-[5px] rounded-full ${
          slot > level
            ? 'bg-white/10'
            : level >= 3
              ? 'bg-ios-red'
              : level === 2
                ? 'bg-ios-orange'
                : 'bg-ios-label-3'
        }`}
      />
    ))}
  </span>
);

/** Prev / forecast / actual keep fixed widths so they still read as right-aligned columns. */
const Value: React.FC<{ label: string; text?: string; tone?: string }> = ({ label, text, tone }) => (
  <span className="flex w-[86px] shrink-0 items-baseline justify-end gap-1 font-mono text-[11px] tabular-nums">
    <span className="text-ios-label-3">{label}</span>
    <span className={text ? tone ?? 'text-ios-label-2' : 'text-ios-label-3'}>{text || '—'}</span>
  </span>
);

export const EconCalendar: React.FC<{ events: MarketEvent[]; loading?: boolean }> = ({ events, loading }) => {
  const { t, language } = useI18n();
  const [scope, setScope] = useState<Scope>('all');
  const locale = language === 'en' ? 'en-US' : 'zh-CN';

  const groups = useMemo(() => {
    const rows = (Array.isArray(events) ? events : [])
      .filter((event) => scope === 'all' || importanceOf(event) >= 3)
      .slice()
      .sort((a, b) => `${a.date}${a.time ?? ''}`.localeCompare(`${b.date}${b.time ?? ''}`));

    const today = new Date();
    const todayKey = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`;

    const buckets: Array<{ date: string; day: string; meta: string; isToday: boolean; events: MarketEvent[] }> = [];
    rows.forEach((event) => {
      let bucket = buckets.find((b) => b.date === event.date);
      if (!bucket) {
        const parsed = event.date ? new Date(`${event.date}T12:00:00`) : null;
        const valid = parsed && !Number.isNaN(parsed.getTime());
        bucket = {
          date: event.date,
          day: valid ? String(parsed!.getDate()) : '—',
          meta: valid
            ? `${parsed!.toLocaleDateString(locale, { month: 'short' })} · ${parsed!.toLocaleDateString(locale, { weekday: 'short' })}`
            : '',
          isToday: event.isToday === true || event.date === todayKey,
          events: [],
        };
        buckets.push(bucket);
      }
      bucket.events.push(event);
    });
    return buckets;
  }, [events, scope, locale]);

  return (
    <section className="rounded-ios-lg bg-ios-card px-4 py-4">
      <header className="mb-3 flex flex-wrap items-center gap-x-3 gap-y-2">
        <h2 className="text-[17px] font-semibold tracking-tight text-ios-label">{t('flash.calTitle')}</h2>
        <span className="text-[12px] text-ios-label-3">{t('flash.calSub')}</span>
        <SegmentedControl
          className="ml-auto"
          size="sm"
          aria-label={t('flash.calTitle')}
          value={scope}
          onChange={setScope}
          segments={[
            { id: 'all', label: t('flash.all') },
            { id: 'important', label: t('flash.calImportant') },
          ]}
        />
      </header>

      <div className="max-h-[68vh] overflow-y-auto">
        {loading && groups.length === 0 && (
          <p className="py-10 text-center text-[13px] text-ios-label-2">{t('cal.loading')}</p>
        )}

        {!loading && groups.length === 0 && (
          <p className="py-10 text-center text-[13px] text-ios-label-2">{t('flash.calEmpty')}</p>
        )}

        <div className="space-y-5">
          {groups.map((group) => (
            <section key={group.date}>
              <header className="mb-2 flex items-center gap-2 px-1">
                <span className="font-mono text-[22px] font-semibold leading-none tabular-nums text-ios-label">
                  {group.day}
                </span>
                <span className="text-[12px] text-ios-label-2">{group.meta}</span>
                {group.isToday && (
                  <span className="rounded-full bg-accent/20 px-2 py-0.5 text-[11px] font-medium text-accent">
                    {t('flash.today')}
                  </span>
                )}
              </header>

              <ul className="overflow-hidden rounded-ios bg-ios-card-2">
                {group.events.map((event, index) => {
                  const level = importanceOf(event);
                  const last = index === group.events.length - 1;
                  return (
                    <li key={event.id} className="pl-4 transition-colors hover:bg-white/[0.03]">
                      <div className={`flex min-h-[44px] flex-col gap-1 py-2.5 pr-4 ${last ? '' : 'border-b border-ios-hairline'}`}>
                        <div className="flex items-center gap-3">
                          <span className="w-[46px] shrink-0 font-mono text-[12px] tabular-nums text-ios-label-2">
                            {event.time || '—'}
                          </span>
                          {event.country && (
                            <span className="shrink-0 rounded-[5px] bg-white/[0.07] px-1.5 py-px text-[10px] font-medium uppercase tracking-wide text-ios-label-2">
                              {event.country}
                            </span>
                          )}
                          <span className={`min-w-0 flex-1 text-[13.5px] leading-snug ${level >= 3 ? 'font-semibold text-ios-label' : 'text-ios-label'}`}>
                            {event.title}
                          </span>
                          <ImpactDots level={level} />
                        </div>

                        {(event.previous || event.forecast || event.actual) && (
                          <div className="flex flex-wrap items-baseline justify-end gap-x-3 gap-y-1 pl-[58px]">
                            <Value label={t('flash.colPrev')} text={event.previous} />
                            <Value label={t('flash.colFcst')} text={event.forecast} />
                            <Value
                              label={t('flash.colActual')}
                              text={event.actual}
                              tone={`font-semibold ${actualTone(event)}`}
                            />
                          </div>
                        )}

                        {event.relatedTickers && event.relatedTickers.length > 0 && (
                          <p className="pl-[58px] font-mono text-[11px] text-ios-label-3">
                            {event.relatedTickers.slice(0, 4).join(' · ')}
                          </p>
                        )}
                      </div>
                    </li>
                  );
                })}
              </ul>
            </section>
          ))}
        </div>
      </div>
    </section>
  );
};
