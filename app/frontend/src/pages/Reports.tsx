import React, { useEffect, useMemo, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import { ChevronDown, ChevronUp } from 'lucide-react';
import { useNavigate, useParams } from 'react-router-dom';
import { StockGodShell } from '../components/layout/StockGodShell';
import { EmptyState, LoadingSpinner } from '../components/common';
import { useReportsStore } from '../stores/reportsStore';
import { MarketEvent, Report, ReportType } from '../types/reports';

const FILTERS: Array<{ id: 'all' | ReportType | 'intraday'; label: string }> = [
  { id: 'all', label: '全部' },
  { id: 'premarket', label: '盘前' },
  { id: 'intraday', label: '盘中' },
  { id: 'postmarket', label: '收盘' },
];

const formatDateLabel = (date: string) => {
  const [, month, day] = date.split('-');
  const weekday = new Date(`${date}T12:00:00`).toLocaleDateString('zh-CN', { weekday: 'short' });
  return `${month}-${day} · ${weekday}`;
};

const eventMeta = (event: MarketEvent) => {
  if (event.isToday) return '今天';
  if (event.date === '2026-07-06') return '后天';
  return '';
};

const typeLabel = (type: ReportType) => (type === 'postmarket' ? '收盘' : '盘前');

const typeClass = (type: ReportType) =>
  type === 'postmarket' ? 'border-down/30 bg-down/10 text-down' : 'border-accent/30 bg-accent/15 text-accent';

const EventItem: React.FC<{ event: MarketEvent }> = ({ event }) => (
  <div className="border-b border-line/60 pb-3 last:border-b-0 last:pb-0">
    <div className="mb-1 flex items-center gap-2 text-xs">
      <span className="font-mono text-faint">
        {event.date.slice(5).replace('-', '/')} {event.dayOfWeek}
      </span>
      {eventMeta(event) && <span className="text-faint">· {eventMeta(event)}</span>}
      <span className="text-faint">—</span>
    </div>
    <div className="flex items-start gap-2">
      <span className="mt-0.5 text-xs">{event.isImportant ? '🔴' : ''}</span>
      <div className="min-w-0 flex-1">
        <div className="font-medium text-ink">{event.title}</div>
        {event.description && <div className="mt-0.5 text-xs text-muted">{event.description}</div>}
        {event.relatedTickers && event.relatedTickers.length > 0 && (
          <div className="mt-1 flex flex-wrap gap-1.5 font-mono text-[11px]">
            {event.relatedTickers.map((ticker) => (
              <a key={ticker} href={`/stock/${ticker}?market=us`} className="text-accent hover:underline">
                {ticker}
              </a>
            ))}
          </div>
        )}
      </div>
      <span className="rounded bg-surface-3 px-1.5 py-0.5 text-[10px] text-faint">{event.type === 'earnings' ? '财报' : '宏观'}</span>
    </div>
    {event.earnings && event.earnings.length > 0 && (
      <div className="mt-2 space-y-1.5">
        {event.earnings.map((earning) => (
          <div key={earning.ticker} className="rounded-lg border border-line bg-base px-2.5 py-2 text-[11px]">
            <div className="flex items-center justify-between gap-2">
              <a href={`/stock/${earning.ticker}?market=us`} className="font-mono font-semibold text-ink hover:text-accent">
                {earning.ticker}
              </a>
              <span className="text-faint">{earning.time === 'premarket' ? '盘前' : earning.time}</span>
            </div>
            <div className="mt-1 truncate text-muted">{earning.companyName}</div>
            <div className="mt-1 text-faint">预期 EPS {earning.expectedEPS} · {earning.marketCap}</div>
          </div>
        ))}
      </div>
    )}
  </div>
);

const ReportArticle: React.FC<{ report: Report; expanded: boolean; onToggle: () => void }> = ({
  report,
  expanded,
  onToggle,
}) => (
  <article id={`report-${report.id}`} className="overflow-hidden rounded-xl border border-line bg-surface">
    <button className="flex w-full items-start gap-3 px-5 py-4 text-left transition hover:bg-surface-2" onClick={onToggle}>
      <span className={`mt-0.5 shrink-0 rounded border px-1.5 py-0.5 text-[10px] font-medium ${typeClass(report.type)}`}>
        {typeLabel(report.type)}
      </span>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
          <h2 className="min-w-0 text-sm font-semibold text-ink sm:truncate">{report.title}</h2>
          <span className="shrink-0 text-[11px] text-faint">{report.time}</span>
        </div>
        {!expanded && <p className="mt-1 truncate text-xs text-muted">{report.summary}</p>}
      </div>
      <span className="shrink-0 text-faint">{expanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}</span>
    </button>
    {expanded && (
      <div className="border-t border-line px-4 py-4 text-sm leading-relaxed text-muted sm:px-5">
        <ReactMarkdown
          components={{
            h2: ({ children }) => <h2 className="mb-2 mt-5 text-[15px] font-semibold text-ink first:mt-0">{children}</h2>,
            p: ({ children }) => <p className="my-2">{children}</p>,
            strong: ({ children }) => <strong className="font-semibold text-ink">{children}</strong>,
            ul: ({ children }) => <ul className="my-2 space-y-1">{children}</ul>,
            li: ({ children }) => <li className="ml-4 list-disc marker:text-faint">{children}</li>,
            a: ({ href, children }) => (
              <a href={href} className="font-mono text-accent hover:underline">
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
  const { id: routeId } = useParams<{ id?: string }>();
  const navigate = useNavigate();
  const { reports, marketEvents, expandedReportIds, loading, error, hasMore, total, fetchReports, fetchMarketCalendar, toggleReport } = useReportsStore();
  const [filter, setFilter] = useState<'all' | ReportType | 'intraday'>('all');
  const safeReports = Array.isArray(reports) ? reports : [];
  const safeMarketEvents = Array.isArray(marketEvents) ? marketEvents : [];

  useEffect(() => {
    fetchReports(0);
    fetchMarketCalendar();
  }, []);

  // Deep link /reports/:id — expand that report
  useEffect(() => {
    if (!routeId || !safeReports.length) return;
    if (!expandedReportIds.has(routeId)) {
      toggleReport(routeId);
    }
  }, [routeId, safeReports.length]);

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
    <StockGodShell title="盘报">
      <div className="space-y-3">
        <header className="flex flex-wrap items-baseline gap-x-3">
          <h1 className="text-[22px] font-semibold tracking-tight text-ink">盘报</h1>
          <p className="text-sm text-muted">市场日历 · 今日大事 · 盘前/收盘总结 · 非投资建议</p>
        </header>

        <div className="grid grid-cols-1 items-start gap-x-6 lg:grid-cols-[360px_minmax(0,1fr)]">
          <aside className="mb-7 rounded-xl border border-line bg-surface p-4 lg:sticky lg:top-[72px]">
            <div className="mb-4">
              <h2 className="text-sm font-semibold text-ink">市场日历 · 接下来盯什么</h2>
              <p className="mt-1 text-xs text-faint">🔴 = 重磅</p>
            </div>
            {safeMarketEvents.length === 0 ? (
              <p className="py-8 text-center text-xs text-muted">暂无日历事件</p>
            ) : (
              <div className="space-y-3">
                {safeMarketEvents.map((event) => (
                  <EventItem key={event.id} event={event} />
                ))}
              </div>
            )}
          </aside>

          <main className="min-w-0">
            <div className="mb-4 inline-flex w-full rounded-lg border border-line bg-surface p-0.5 text-sm sm:w-auto">
              {FILTERS.map((item) => (
                <button
                  key={item.id}
                  onClick={() => setFilter(item.id)}
                  className={`flex-1 rounded-md px-3 py-2 text-center font-medium transition sm:flex-none ${
                    filter === item.id ? 'bg-surface-3 text-ink' : 'text-muted hover:text-ink'
                  }`}
                >
                  {item.label}
                </button>
              ))}
            </div>

            {loading && safeReports.length === 0 && (
              <div className="flex items-center justify-center rounded-xl border border-line bg-surface py-16">
                <LoadingSpinner size="lg" />
              </div>
            )}

            {error && <EmptyState title="加载失败" description={error} action={{ label: '重试', onClick: () => fetchReports(0) }} />}

            {!loading && !error && groupedReports.length === 0 && <EmptyState title="暂无盘报" description="敬请期待最新市场分析" />}

            {groupedReports.map((group) => (
              <section key={group.date} className="mb-6">
                <div className="mb-2 flex items-center justify-between text-xs">
                  <h2 className="font-semibold text-muted">{formatDateLabel(group.date)}</h2>
                  <span className="text-faint">{group.reports.length} 篇</span>
                </div>
                <div className="space-y-3">
                  {group.reports.map((report) => (
                    <ReportArticle
                      key={report.id}
                      report={report}
                      expanded={expandedReportIds.has(report.id)}
                      onToggle={() => {
                        toggleReport(report.id);
                        navigate(`/reports/${report.id}`);
                      }}
                    />
                  ))}
                </div>
              </section>
            ))}

            {hasMore && (
              <button
                onClick={() => fetchReports(safeReports.length)}
                className="w-full rounded-xl border border-dashed border-line py-2.5 text-xs text-muted transition hover:border-accent/40 hover:text-ink"
              >
                {loading ? '加载中...' : `查看更早 · 还有 ${Math.max(0, Math.ceil((total - safeReports.length) / 2))} 天`}
              </button>
            )}
          </main>
        </div>

        <footer className="mt-12 text-center text-xs text-faint">我不是神 · 盘报仅供参考 · 非投资建议</footer>
      </div>
    </StockGodShell>
  );
};
