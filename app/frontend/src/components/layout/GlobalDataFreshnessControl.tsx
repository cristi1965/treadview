import React, { useCallback, useEffect, useId, useMemo, useState } from 'react';
import { ChevronDown, RefreshCw, X } from 'lucide-react';
import { Link, useLocation } from 'react-router-dom';
import { get as apiGet, getWithMeta } from '../../utils/api';
import { assessDataTrust } from '../../utils/dataTrust';
import { useStocksStore } from '../../stores/stocksStore';

interface SystemDataFileStatus {
  id: string;
  exists: boolean;
  modifiedAt?: string;
  generatedAt?: string;
  count?: number;
}

export interface SystemStatusResponse {
  status: string;
  checkedAt: string;
  dataFiles?: SystemDataFileStatus[];
  endpoints?: Array<{
    endpoint: string;
    source?: string;
    dataTime?: string;
    stale: boolean;
    staleReason?: string;
    refreshable?: boolean;
    status: string;
  }>;
}

export interface GlobalDataFreshnessController {
  status: SystemStatusResponse | null;
  refreshing: boolean;
  error: string;
  refresh: () => Promise<void>;
}

interface FreshnessSummary {
  degraded: boolean;
  hasUnknownTime: boolean;
  oldestKnownTimestamp: number | null;
}

const copy = {
  zh: {
    dataUnknown: '时间未知', dataPrefix: '数据', degraded: '实时数据未就绪', healthy: '实时数据已核验',
    diagnostic: '实时数据诊断', diagnosticHint: '历史研究可独立使用；Paper 引擎可运行不代表当前订单可提交，下单仍受标的行情门禁限制。',
    checked: '检查时间', noEndpoints: '没有取得端点状态', research: '进入历史研究',
    refresh: '更新', refreshing: '更新中', failed: '部分失败', close: '收起诊断',
  },
  en: {
    dataUnknown: 'Time unknown', dataPrefix: 'Data', degraded: 'Live data not ready', healthy: 'Live data verified',
    diagnostic: 'Live data diagnostics', diagnosticHint: 'Historical research remains independent. A ready Paper engine does not mean an order is currently submittable; symbol quote gates still apply.',
    checked: 'Checked', noEndpoints: 'No endpoint status available', research: 'Open research',
    refresh: 'Refresh', refreshing: 'Refreshing', failed: 'Partial fail', close: 'Close diagnostics',
  },
} as const;

export const summarizeGlobalDataFreshness = (status: SystemStatusResponse | null): FreshnessSummary => {
  if (!status?.endpoints?.length) return { degraded: true, hasUnknownTime: true, oldestKnownTimestamp: null };
  const relevant = status.endpoints.filter((endpoint) => endpoint.status !== 'local-authored');
  if (relevant.length === 0) return { degraded: true, hasUnknownTime: true, oldestKnownTimestamp: null };

  const knownTimestamps: number[] = [];
  let hasUnknownTime = false;
  for (const endpoint of relevant) {
    const value = endpoint.dataTime?.trim() || '';
    const parsed = value && value !== 'unknown' ? Date.parse(value) : Number.NaN;
    if (Number.isFinite(parsed)) knownTimestamps.push(parsed);
    else hasUnknownTime = true;
  }
  const endpointDegraded = relevant.some((endpoint) =>
    endpoint.stale || ['stale', 'unavailable', 'error'].includes(endpoint.status),
  );
  return {
    degraded: status.status !== 'ok' || endpointDegraded || hasUnknownTime,
    hasUnknownTime,
    oldestKnownTimestamp: knownTimestamps.length > 0 ? Math.min(...knownTimestamps) : null,
  };
};

const formatDataTime = (timestamp: number | null, language: 'zh' | 'en') => {
  if (timestamp === null) return copy[language].dataUnknown;
  return `${copy[language].dataPrefix} ${new Intl.DateTimeFormat(language === 'zh' ? 'zh-CN' : 'en-US', {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  }).format(new Date(timestamp))}`;
};

const localizedEndpointStatus = (status: string, language: 'zh' | 'en') => {
  if (language === 'en') return status || 'unknown';
  const labels: Record<string, string> = {
    ok: '正常', live: '正常', stale: '旧快照', unavailable: '不可用', unknown: '未知', error: '错误', 'local-authored': '本地数据',
  };
  return labels[status] || status || '未知';
};

export const useGlobalDataFreshness = (enabled = true): GlobalDataFreshnessController => {
  const location = useLocation();
  const [status, setStatus] = useState<SystemStatusResponse | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');

  const loadStatus = useCallback(async () => {
    try {
      const next = await apiGet<SystemStatusResponse>('/api/system/status');
      setStatus(next);
      return true;
    } catch (err) {
      setStatus(null);
      setError(err instanceof Error ? err.message : '系统数据状态不可用');
      return false;
    }
  }, []);

  const refresh = useCallback(async () => {
    setRefreshing(true);
    setError('');
    try {
      const inspectRefresh = async (label: string, endpoint: string, hasPayload: (data: any) => boolean) => {
        const result = await getWithMeta<any>(endpoint);
        const trust = assessDataTrust(result.meta, hasPayload(result.data));
        if (trust.state === 'live') return;
        const action = result.meta.refreshable ? '更新后仍未就绪' : '数据源不可刷新，仅重新读取了旧快照';
        throw new Error(`${label}：${action}（${trust.reason}）`);
      };
      const tasks = [
        { label: '宏观', task: inspectRefresh('宏观', '/api/macro', (data) => Boolean(data?.series?.length)) },
        { label: '异动', task: inspectRefresh('异动', '/api/premarket-movers', (data) => Boolean(data?.gainers?.length || data?.losers?.length)) },
      ] as const;
      const results = await Promise.allSettled(tasks.map(({ task }) => task));
      const errors = results.flatMap((result, index) => result.status === 'rejected'
        ? [`${tasks[index].label}: ${result.reason instanceof Error ? result.reason.message : String(result.reason)}`]
        : []);
      await useStocksStore.getState().refreshQuotes();
      window.dispatchEvent(new CustomEvent('stockgod:manual-refresh', { detail: { skipQuotes: true } }));
      const statusLoaded = await loadStatus();
      if (statusLoaded) setError(errors.join('；'));
      else if (errors.length > 0) setError((current) => [current, ...errors].filter(Boolean).join('；'));
    } catch (err) {
      setError(err instanceof Error ? err.message : '刷新失败');
    } finally {
      setRefreshing(false);
    }
  }, [loadStatus]);

  useEffect(() => {
    setError('');
    if (!enabled) {
      setStatus(null);
      return;
    }
    void loadStatus();
  }, [enabled, location.pathname, loadStatus]);

  return useMemo(() => ({ status, refreshing, error, refresh }), [status, refreshing, error, refresh]);
};

export const GlobalDataFreshnessControl: React.FC<{
  language: 'zh' | 'en';
  controller: GlobalDataFreshnessController;
  compact?: boolean;
}> = ({ language, controller, compact = false }) => {
  const [detailsOpen, setDetailsOpen] = useState(false);
  const diagnosticsId = useId();
  const summary = summarizeGlobalDataFreshness(controller.status);
  const t = copy[language];
  const degraded = summary.degraded || Boolean(controller.error);
  const primaryTime = summary.hasUnknownTime ? t.dataUnknown : formatDataTime(summary.oldestKnownTimestamp, language);
  const endpointDetails = controller.status?.endpoints || [];

  return (
    <div className="relative inline-flex min-w-0 shrink-0" data-testid="global-data-freshness">
      <div
        className={`inline-flex min-w-0 items-center gap-1 rounded-lg border ${
          degraded ? 'border-amber-500/30 bg-amber-500/10 text-amber-200' : 'border-emerald-500/25 bg-emerald-500/10 text-emerald-200'
        } ${compact ? 'px-1.5 py-1' : 'px-2 py-1'}`}
      >
        <button
          type="button"
          onClick={() => setDetailsOpen((open) => !open)}
          aria-expanded={detailsOpen}
          aria-controls={diagnosticsId}
          className="flex min-h-11 min-w-0 items-center gap-1.5 rounded-md px-1 py-0.5 text-left transition hover:bg-white/10 sm:min-h-0"
        >
          <span className="min-w-0">
            <span className={`block whitespace-nowrap font-semibold ${compact ? 'text-[10px]' : 'text-[11px]'}`}>
              {degraded ? t.degraded : t.healthy}
            </span>
            {!compact && <span className="block truncate font-mono text-[9px] opacity-75">{primaryTime}</span>}
          </span>
          <ChevronDown className={`h-3 w-3 shrink-0 transition-transform ${detailsOpen ? 'rotate-180' : ''}`} />
        </button>
        <button
          type="button"
          onClick={() => void controller.refresh()}
          disabled={controller.refreshing}
          aria-label={controller.refreshing ? t.refreshing : t.refresh}
          className="inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-md transition hover:bg-white/10 disabled:cursor-wait disabled:opacity-70 sm:h-7 sm:w-7"
        >
          <RefreshCw className={`h-3.5 w-3.5 ${controller.refreshing ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {detailsOpen && (
        <section
          id={diagnosticsId}
          data-testid="live-data-diagnostics"
          aria-label={t.diagnostic}
          className="absolute right-0 top-[calc(100%+8px)] z-[60] max-h-[65dvh] w-[min(36rem,calc(100vw-1rem))] overflow-y-auto rounded-lg border border-line bg-surface p-3 text-left text-ink shadow-2xl"
        >
          <div className="flex items-start justify-between gap-3 border-b border-line pb-2.5">
            <div>
              <h2 className="text-sm font-semibold text-ink">{t.diagnostic}</h2>
              <p className="mt-1 text-[11px] leading-relaxed text-muted">{t.diagnosticHint}</p>
            </div>
            <button type="button" onClick={() => setDetailsOpen(false)} aria-label={t.close} className="inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-md text-muted hover:bg-surface-2 hover:text-ink sm:h-7 sm:w-7">
              <X className="h-3.5 w-3.5" />
            </button>
          </div>

          <div className="mt-2.5 flex items-center justify-between gap-3 text-[10px] text-faint">
            <span>{t.checked}: {controller.status?.checkedAt || t.dataUnknown}</span>
            <Link to="/dashboard" onClick={() => setDetailsOpen(false)} className="shrink-0 font-semibold text-accent hover:underline">{t.research}</Link>
          </div>

          {controller.error && <p className="mt-2 rounded-md border border-red-500/30 bg-red-500/10 px-2.5 py-2 text-xs text-red-200">{controller.error}</p>}

          <div className="mt-2.5 space-y-2">
            {endpointDetails.length === 0 ? <p className="py-3 text-center text-xs text-muted">{t.noEndpoints}</p> : endpointDetails.map((endpoint) => (
              <div key={endpoint.endpoint} className="rounded-md border border-line bg-surface-2/70 px-2.5 py-2">
                <div className="flex items-center justify-between gap-3">
                  <code className="min-w-0 truncate text-[11px] text-ink">{endpoint.endpoint}</code>
                  <span className={`shrink-0 text-[10px] font-semibold ${endpoint.stale || !['ok', 'live', 'local-authored'].includes(endpoint.status) ? 'text-amber-300' : 'text-emerald-300'}`}>
                    {localizedEndpointStatus(endpoint.status, language)}
                  </span>
                </div>
                <div className="mt-1 grid gap-0.5 text-[10px] leading-relaxed text-faint sm:grid-cols-2">
                  <span>{language === 'zh' ? '数据时间' : 'Data time'}: {endpoint.dataTime || t.dataUnknown}</span>
                  <span className="break-all">{language === 'zh' ? '来源' : 'Source'}: {endpoint.source || t.dataUnknown}</span>
                  <span>{language === 'zh' ? '恢复能力' : 'Recovery'}: {endpoint.refreshable ? (language === 'zh' ? '可主动刷新' : 'Refreshable') : (language === 'zh' ? '仅可重读' : 'Reload only')}</span>
                </div>
                {endpoint.staleReason && <p className="mt-1 break-words text-[10px] leading-relaxed text-amber-200/80">{endpoint.staleReason}</p>}
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
};
