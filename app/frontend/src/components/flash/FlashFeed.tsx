import React, { useMemo } from 'react';
import { Play, RefreshCw } from 'lucide-react';
import { SegmentedControl } from '../common';
import { FlashItem } from '../../types/reports';
import { I18nKey, useI18n } from '../../i18n';
import { ImportanceFilter, KindFilter, useFlashStore } from '../../stores/flashStore';
import { FlashItemRow, parseFlashTime } from './FlashItemRow';

const IMPORTANCE_TABS: Array<{ id: ImportanceFilter; labelKey: I18nKey }> = [
  { id: 'all', labelKey: 'flash.all' },
  { id: 'important', labelKey: 'flash.important' },
];

const KIND_TABS: Array<{ id: KindFilter; labelKey: I18nKey }> = [
  { id: 'all', labelKey: 'flash.all' },
  { id: 'macro', labelKey: 'flash.kindMacro' },
  { id: 'earnings', labelKey: 'flash.kindEarnings' },
  { id: 'company', labelKey: 'flash.kindCompany' },
  { id: 'market', labelKey: 'flash.kindMarket' },
];

const pad = (n: number) => String(n).padStart(2, '0');

const dayKey = (date: Date | null) =>
  date ? `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` : 'unknown';

export const FlashFeed: React.FC = () => {
  const { t, language } = useI18n();
  const {
    items,
    updatedAt,
    source,
    staleReason,
    loading,
    error,
    degraded,
    paused,
    importance,
    kind,
    newIds,
    lastUpdated,
    fetchFlash,
    setPaused,
    setImportance,
    setKind,
  } = useFlashStore();

  const visible = useMemo(
    () =>
      items.filter(
        (item) => (importance === 'all' || item.importance >= 3) && (kind === 'all' || item.kind === kind)
      ),
    [items, importance, kind]
  );

  const groups = useMemo(() => {
    const buckets: Array<{ key: string; label: string; items: FlashItem[] }> = [];
    const todayKey = dayKey(new Date());
    visible.forEach((item) => {
      const date = parseFlashTime(item.time);
      const key = dayKey(date);
      let bucket = buckets.find((b) => b.key === key);
      if (!bucket) {
        const weekday = date
          ? date.toLocaleDateString(language === 'en' ? 'en-US' : 'zh-CN', { weekday: 'short' })
          : '';
        const md = date ? `${pad(date.getMonth() + 1)}-${pad(date.getDate())}` : '';
        const label = !date
          ? '—'
          : key === todayKey
            ? `${t('flash.today')} · ${md} ${weekday}`
            : `${md} ${weekday}`;
        bucket = { key, label, items: [] };
        buckets.push(bucket);
      }
      bucket.items.push(item);
    });
    return buckets;
  }, [visible, language, t]);

  const updatedLabel = degraded && updatedAt
    ? `${language === 'en' ? 'Data time' : '数据时间'} ${new Date(updatedAt).toLocaleString(language === 'en' ? 'en-US' : 'zh-CN', { hour12: false })}`
    : lastUpdated
      ? t('flash.updated', {
        time: new Date(lastUpdated).toLocaleTimeString(language === 'en' ? 'en-US' : 'zh-CN', { hour12: false }),
        })
      : t('flash.never');

  const statusLabel = error
    ? (language === 'en' ? 'Unavailable' : '不可用')
    : loading && items.length === 0
      ? (language === 'en' ? 'Checking' : '检查中')
      : degraded
        ? t('flash.snapshot')
        : paused
          ? t('flash.paused')
          : t('flash.live');

  return (
    <section className="rounded-ios-lg bg-ios-card px-4 py-4">
      <header>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <h2 className="text-[17px] font-semibold tracking-tight text-ios-label">{t('flash.title')}</h2>

          <button
            type="button"
            onClick={() => setPaused(!paused)}
            title={paused ? t('flash.resume') : t('flash.pause')}
            className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium transition ${
              error || degraded
                ? 'bg-amber-500/15 text-amber-300'
                : paused || (loading && items.length === 0)
                  ? 'bg-white/[0.07] text-ios-label-2 hover:text-ios-label'
                  : 'bg-ios-green/15 text-ios-green'
            }`}
          >
            {paused && !error && !degraded ? (
              <Play className="h-3 w-3" />
            ) : !error && !degraded && !(loading && items.length === 0) ? (
              <span className="relative flex h-1.5 w-1.5">
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-ios-green opacity-70" />
                <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-ios-green" />
              </span>
            ) : null}
            {statusLabel}
          </button>

          <div className="ml-auto flex items-center gap-2 text-[11px] text-ios-label-3">
            <span className="tabular-nums">{updatedLabel}</span>
            <button
              type="button"
              onClick={() => fetchFlash()}
              title={t('flash.refresh')}
              aria-label={t('flash.refresh')}
              className="rounded-full bg-white/[0.07] p-1.5 text-ios-label-2 transition hover:text-ios-label"
            >
              <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
            </button>
          </div>
        </div>

        <p className="mt-1.5 text-[12px] text-ios-label-3">{t('flash.sub')}</p>
        {degraded && (source || staleReason) && (
          <p className="mt-1 text-[11px] leading-relaxed text-amber-300/90">
            {[source, staleReason].filter(Boolean).join(' · ')}
          </p>
        )}

        <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-2">
          <SegmentedControl
            size="sm"
            aria-label={t('flash.colImp')}
            value={importance}
            onChange={setImportance}
            segments={IMPORTANCE_TABS.map((tab) => ({ id: tab.id, label: t(tab.labelKey) }))}
          />
          <SegmentedControl
            size="sm"
            aria-label={t('flash.kindLabel')}
            value={kind}
            onChange={setKind}
            segments={KIND_TABS.map((tab) => ({ id: tab.id, label: t(tab.labelKey) }))}
          />
          <span className="text-[11px] tabular-nums text-ios-label-3">{t('flash.count', { n: visible.length })}</span>
        </div>
      </header>

      <div className="mt-3 max-h-[68vh] overflow-y-auto lg:max-h-[calc(100vh-240px)]">
        {loading && items.length === 0 && (
          <p className="py-16 text-center text-[13px] text-ios-label-2">{t('flash.loading')}</p>
        )}

        {!loading && error && items.length === 0 && (
          <div className="rounded-ios bg-ios-card-2 px-6 py-12 text-center">
            <p className="text-[15px] font-semibold text-ios-label">{t('flash.fail')}</p>
            <p className="mx-auto mt-2 max-w-sm text-[12.5px] leading-relaxed text-ios-label-2">{t('flash.failDesc')}</p>
            <button
              type="button"
              onClick={() => fetchFlash()}
              className="mt-4 rounded-ios-sm bg-ios-blue px-4 py-2 text-[13px] font-medium text-white transition hover:brightness-110"
            >
              {t('flash.retry')}
            </button>
          </div>
        )}

        {!loading && !error && visible.length === 0 && (
          <div className="rounded-ios bg-ios-card-2 px-6 py-12 text-center">
            <p className="text-[15px] font-semibold text-ios-label">{t('flash.empty')}</p>
            <p className="mx-auto mt-2 max-w-sm text-[12.5px] leading-relaxed text-ios-label-2">{t('flash.emptyDesc')}</p>
          </div>
        )}

        <div className="space-y-5">
          {groups.map((group) => (
            <section key={group.key}>
              <header className="mb-2 px-1">
                <span className="font-mono text-[12px] font-semibold uppercase tracking-wide text-ios-label-2">
                  {group.label}
                </span>
              </header>
              <ul className="overflow-hidden rounded-ios bg-ios-card-2">
                {group.items.map((item, index) => (
                  <FlashItemRow
                    key={item.id}
                    item={item}
                    isNew={newIds.has(item.id)}
                    last={index === group.items.length - 1}
                  />
                ))}
              </ul>
            </section>
          ))}
        </div>
      </div>
    </section>
  );
};
