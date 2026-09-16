import React, { useEffect, useMemo, useRef, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import { Check, ChevronDown, ChevronUp, Copy, Link2 } from 'lucide-react';
import { Link, useParams } from 'react-router-dom';
import { StockGodShell } from '../components/layout/StockGodShell';
import { DataStatus, LoadingSpinner, SegmentedControl } from '../components/common';
import { FlashFeed } from '../components/flash/FlashFeed';
import { EconCalendar } from '../components/flash/EconCalendar';
import { useReportsStore } from '../stores/reportsStore';
import { useFlashStore } from '../stores/flashStore';
import { Report, ReportType } from '../types/reports';
import { I18nKey, useI18n } from '../i18n';

type Tab = 'flash' | 'reports';

const TABS: Array<{ id: Tab; labelKey: I18nKey }> = [
  { id: 'flash', labelKey: 'flash.tab' },
  { id: 'reports', labelKey: 'flash.tabReports' },
];

const FILTERS: Array<{ id: 'all' | ReportType | 'intraday'; labelKey: I18nKey }> = [
  { id: 'all', labelKey: 'rep.all' },
  { id: 'premarket', labelKey: 'rep.pre' },
  { id: 'intraday', labelKey: 'rep.intra' },
  { id: 'postmarket', labelKey: 'rep.post' },
];

const formatDateLabel = (date: string, locale: string) => {
  const [, month, day] = date.split('-');
  const weekday = new Date(`${date}T12:00:00`).toLocaleDateString(locale, { weekday: 'short' });
  return `${month}-${day} · ${weekday}`;
};

const typeClass = (type: ReportType) =>
  type === 'postmarket' ? 'bg-ios-red/15 text-ios-red' : 'bg-accent/15 text-accent';

const ReportArticle: React.FC<{
  report: Report;
  expanded: boolean;
  typeLabel: string;
  copied: boolean;
  onToggle: () => void;
  onCopy: () => void;
}> = ({ report, expanded, typeLabel, copied, onToggle, onCopy }) => (
  <article id={`report-${report.id}`} className="overflow-hidden rounded-ios bg-ios-card">
    <div className="flex items-start">
      <button className="flex min-w-0 flex-1 items-start gap-3 px-4 py-3.5 text-left transition hover:bg-white/[0.03]" onClick={onToggle}>
        <span className={`mt-0.5 shrink-0 rounded-[5px] px-1.5 py-0.5 text-[10px] font-medium ${typeClass(report.type)}`}>
          {typeLabel}
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
            <h2 className="min-w-0 text-[14px] font-semibold text-ios-label sm:truncate">{report.title}</h2>
            <span className="shrink-0 font-mono text-[11px] tabular-nums text-ios-label-3">{report.time}</span>
          </div>
          {!expanded && <p className="mt-1 truncate text-[12.5px] text-ios-label-2">{report.summary}</p>}
        </div>
        <span className="shrink-0 text-ios-label-3">{expanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}</span>
      </button>
      <div className="flex shrink-0 items-center gap-1 py-2.5 pr-3">
        <Link
          to={`/reports/${report.id}`}
          className="inline-flex h-8 w-8 items-center justify-center rounded-ios-sm text-ios-label-3 transition hover:bg-white/[0.06] hover:text-ios-label"
          aria-label={`直接访问报告：${report.title}`}
          title="直接访问报告"
        >
          <Link2 className="h-3.5 w-3.5" />
        </Link>
        <button
          type="button"
          onClick={onCopy}
          className="inline-flex h-8 w-8 items-center justify-center rounded-ios-sm text-ios-label-3 transition hover:bg-white/[0.06] hover:text-ios-label"
          aria-label={`复制报告链接：${report.title}`}
          title={copied ? '已复制' : '复制报告链接'}
        >
          {copied ? <Check className="h-3.5 w-3.5 text-ios-green" /> : <Copy className="h-3.5 w-3.5" />}
        </button>
      </div>
    </div>
    {expanded && (
      <div className="border-t border-ios-hairline px-4 py-4 text-[13.5px] leading-relaxed text-ios-label-2">
        <ReactMarkdown
          components={{
            h2: ({ children }) => <h2 className="mb-2 mt-5 text-[15px] font-semibold text-ios-label first:mt-0">{children}</h2>,
            p: ({ children }) => <p className="my-2">{children}</p>,
            strong: ({ children }) => <strong className="font-semibold text-ios-label">{children}</strong>,
            ul: ({ children }) => <ul className="my-2 space-y-1">{children}</ul>,
            li: ({ children }) => <li className="ml-4 list-disc marker:text-ios-label-3">{children}</li>,
            a: ({ href, children }) => (
              <a href={href} className="font-mono text-ios-blue hover:underline">
                {children}
              </a>
            ),
          }}
        >
          {report.content}
        </ReactMarkdown>
      </div>
    )}
  </article>
);

export const Reports: React.FC = () => {
  const { t, language } = useI18n();
  const { id: routeId } = useParams<{ id?: string }>();
  const {
    reports,
    marketEvents,
    expandedReportIds,
    loading,
    error,
    calendarError,
    reportsMeta,
    calendarMeta,
    hasMore,
    total,
    fetchReports,
    fetchReportById,
    fetchMarketCalendar,
    toggleReport,
  } = useReportsStore();
  const { fetchFlash, startPolling, stopPolling } = useFlashStore();
  const [tab, setTab] = useState<Tab>(routeId ? 'reports' : 'flash');
  const [filter, setFilter] = useState<'all' | ReportType | 'intraday'>('all');
  const reportsRequested = useRef('');
  const [copiedReportId, setCopiedReportId] = useState('');
  const safeReports = Array.isArray(reports) ? reports : [];
  const safeMarketEvents = Array.isArray(marketEvents) ? marketEvents : [];
  const locale = language === 'en' ? 'en-US' : 'zh-CN';
  const retryReports = () => routeId ? fetchReportById(routeId) : fetchReports(0);

  useEffect(() => {
    void fetchFlash();
    void fetchMarketCalendar();
    startPolling();
    return () => stopPolling();
  }, []);

  // Reports are only needed for the secondary tab (or a /reports/:id deep link).
  useEffect(() => {
    if (tab !== 'reports') return;
    const requestKey = routeId ? `report:${routeId}` : 'report-list';
    if (reportsRequested.current === requestKey) return;
    reportsRequested.current = requestKey;
    if (routeId) void fetchReportById(routeId);
    else void fetchReports(0);
  }, [tab, routeId, fetchReportById, fetchReports]);

  useEffect(() => {
    if (!routeId || !safeReports.length) return;
    if (!expandedReportIds.has(routeId)) {
      toggleReport(routeId);
    }
  }, [routeId, safeReports.length]);

  const copyReportLink = async (id: string) => {
    const permalink = new URL(`/reports/${id}`, window.location.origin).toString();
    try {
      await navigator.clipboard.writeText(permalink);
      setCopiedReportId(id);
      window.setTimeout(() => setCopiedReportId((current) => current === id ? '' : current), 1800);
    } catch {
      setCopiedReportId('');
    }
  };

  const filteredReports = useMemo(() => {
    if (filter === 'all') return safeReports;
    if (filter === 'intraday') {
      return safeReports.filter((report) =>
        /盘中|intraday|午盘|盘中速递/i.test(`${report.type} ${report.title} ${report.summary}`)
      );
    }
    return safeReports.filter((report) => report.type === filter);
  }, [safeReports, filter]);

  const groupedReports = useMemo(() => {
    return filteredReports.reduce<Array<{ date: string; reports: Report[] }>>((groups, report) => {
      const group = groups.find((item) => item.date === report.date);
      if (group) {
        group.reports.push(report);
      } else {
        groups.push({ date: report.date, reports: [report] });
      }
      return groups;
    }, []);
  }, [filteredReports]);

  return (
    <StockGodShell title={t('rep.title')}>
      <div className="-mx-4 -mt-3 min-h-[calc(100vh-56px)] space-y-4 bg-ios-bg px-4 pb-10 pt-4 sm:-mx-6 sm:px-6">
        <header className="flex flex-wrap items-baseline gap-x-3">
          <h1 className="text-[24px] font-semibold tracking-tight text-ios-label">{t('rep.title')}</h1>
          <p className="text-[13px] text-ios-label-2">{t('rep.sub')}</p>
        </header>

        <SegmentedControl
          aria-label={t('rep.title')}
          value={tab}
          onChange={setTab}
          segments={TABS.map((item) => ({ id: item.id, label: t(item.labelKey) }))}
        />

        {tab === 'flash' ? (
          <div className="grid grid-cols-1 items-start gap-4 lg:grid-cols-[minmax(0,1fr)_400px]">
            <FlashFeed />
            <div className="space-y-2 lg:sticky lg:top-[72px]">
              <DataStatus
                state={calendarError ? 'error' : calendarMeta?.stale ? 'stale' : calendarMeta?.source ? 'live' : 'loading'}
                label={!calendarError && !calendarMeta?.stale && calendarMeta?.source ? '日历快照' : undefined}
                dataTime={calendarMeta?.dataTime}
                source={calendarMeta?.source}
                message={calendarError || calendarMeta?.staleReason}
                onRetry={() => void fetchMarketCalendar()}
                compact
              />
              <EconCalendar events={safeMarketEvents} />
            </div>
          </div>
        ) : (
          <main className="min-w-0">
            <div className="mb-3">
              <DataStatus
                state={error ? 'error' : reportsMeta?.stale ? 'stale' : reportsMeta?.source ? 'live' : loading ? 'loading' : 'unavailable'}
                label={!error && !reportsMeta?.stale && reportsMeta?.source ? '报告快照' : undefined}
                dataTime={reportsMeta?.dataTime}
                source={reportsMeta?.source}
                message={error || reportsMeta?.staleReason}
                onRetry={() => void retryReports()}
                compact
              />
            </div>
            <SegmentedControl
              className="mb-4"
              size="sm"
              aria-label={t('rep.title')}
              value={filter}
              onChange={setFilter}
              segments={FILTERS.map((item) => ({ id: item.id, label: t(item.labelKey) }))}
            />

            {loading && safeReports.length === 0 && (
              <div className="flex items-center justify-center rounded-ios bg-ios-card py-16">
                <LoadingSpinner size="lg" />
              </div>
            )}

            {error && safeReports.length === 0 && (
              <div className="rounded-ios bg-ios-card px-6 py-12 text-center">
                <p className="text-[15px] font-semibold text-ios-label">{t('rep.fail')}</p>
                <p className="mx-auto mt-2 max-w-sm text-[12.5px] leading-relaxed text-ios-label-2">{error}</p>
                <button
                  type="button"
                  onClick={() => void retryReports()}
                  className="mt-4 rounded-ios-sm bg-ios-blue px-4 py-2 text-[13px] font-medium text-white transition hover:brightness-110"
                >
                  {t('rep.retry')}
                </button>
              </div>
            )}

            {!loading && !error && groupedReports.length === 0 && (
              <div className="rounded-ios bg-ios-card px-6 py-12 text-center">
                <p className="text-[15px] font-semibold text-ios-label">{t('rep.empty')}</p>
                <p className="mx-auto mt-2 max-w-sm text-[12.5px] leading-relaxed text-ios-label-2">{t('rep.emptyDesc')}</p>
              </div>
            )}

            {groupedReports.map((group) => (
              <section key={group.date} className="mb-6">
                <div className="mb-2 flex items-center justify-between px-1 text-[12px]">
                  <h2 className="font-mono font-semibold tracking-wide text-ios-label-2">{formatDateLabel(group.date, locale)}</h2>
                  <span className="tabular-nums text-ios-label-3">{t('rep.n', { n: group.reports.length })}</span>
                </div>
                <div className="space-y-3">
                  {group.reports.map((report) => (
                    <ReportArticle
                      key={report.id}
                      report={report}
                      expanded={expandedReportIds.has(report.id)}
                      typeLabel={report.type === 'postmarket' ? t('rep.post') : t('rep.pre')}
                      copied={copiedReportId === report.id}
                      onToggle={() => toggleReport(report.id)}
                      onCopy={() => void copyReportLink(report.id)}
                    />
                  ))}
                </div>
              </section>
            ))}

            {hasMore && safeReports.length > 0 && (
              <button
                onClick={() => fetchReports(safeReports.length)}
                className="w-full rounded-ios bg-ios-card py-3 text-[13px] text-ios-label-2 transition hover:text-ios-label"
              >
                {loading ? t('rep.loading') : t('rep.more', { n: Math.max(0, Math.ceil((total - safeReports.length) / 2)) })}
              </button>
            )}
          </main>
        )}

        <footer className="mt-12 text-center text-[11px] text-ios-label-3">{t('flash.footer')}</footer>
      </div>
    </StockGodShell>
  );
};
