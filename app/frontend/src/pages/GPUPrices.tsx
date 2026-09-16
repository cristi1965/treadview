import React, { useEffect, useMemo, useState } from 'react';
import { CalendarClock, ExternalLink, Gauge, RefreshCw, Server, ShieldAlert } from 'lucide-react';
import {
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { DataStatus } from '../components/common';
import { StockGodShell } from '../components/layout/StockGodShell';
import { useAdminSession } from '../hooks/useAdminSession';
import { useGPUPriceHistory, useGPUPriceReference, useGPUPrices } from '../hooks/useGPUPrices';
import type { AdaptedGPUPriceReference, GPUPriceQuote, GPUProviderStatus } from '../types/gpuPrices';
import { getGPUUpstreamRefreshAvailability } from '../utils/gpuPrices';

const PROVIDERS = ['runpod', 'modal', 'lambda', 'vast'];
const providerLabel = (provider: string) => ({ runpod: 'RunPod', modal: 'Modal', lambda: 'Lambda Cloud', vast: 'Vast.ai' }[provider.toLowerCase()] || provider);
const providerConsoleUrl = (provider: string) => ({
  runpod: 'https://www.runpod.io/console/gpu-cloud', modal: 'https://modal.com/apps', lambda: 'https://cloud.lambda.ai/instances', vast: 'https://cloud.vast.ai/create/',
}[provider.toLowerCase()] || 'https://cloud-gpus.com/');
const billingLabel = (mode: string) => ({
  serverless: 'Serverless', per_second: '按秒 Serverless', instance: '云实例', on_demand: '按需实例', 'on-demand': '按需实例', 'usage-based': '按量 Serverless', community: '社区市场', secure_cloud: '安全云', offer: '市场 Offer',
}[mode.toLowerCase()] || mode);
const statusLabel = (status: string) => ({
  live: '可用', stale: '过期', error: '失败', unavailable: '不可用', unconfigured: '未配置', refreshing: '刷新中',
}[status.toLowerCase()] || status);

const formatPrice = (value: number) => `$${value < 0.01 ? value.toFixed(5) : value.toFixed(2)}`;
const formatObserved = (value?: string) => value ? new Date(value).toLocaleString() : '未返回';

const ProviderDiagnostics: React.FC<{ statuses: GPUProviderStatus[]; testOnly?: boolean }> = ({ statuses, testOnly = false }) => {
  const byProvider = new Map(statuses.map((item) => [item.provider.toLowerCase(), item]));
  return (
    <section aria-labelledby="provider-status-title">
      <div className="mb-2 flex items-center justify-between">
        <h2 id="provider-status-title" className="text-sm font-semibold text-ink">数据源诊断</h2>
        <span className="text-[11px] text-faint">凭证仅在后端环境变量或仓库外安全文件</span>
      </div>
      <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
        {PROVIDERS.map((provider) => {
          const item = byProvider.get(provider);
          const status = item?.status || 'unavailable';
          const live = status === 'live';
          return (
            <div key={provider} className="min-w-0 border-l-2 border-line bg-surface/60 px-3 py-2.5">
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-semibold text-ink">{providerLabel(provider)}</span>
                <span className={`text-[11px] font-semibold ${live ? 'text-up' : status === 'unconfigured' ? 'text-accent' : 'text-down'}`}>
				  {testOnly && live ? '夹具可用' : statusLabel(status)}
                </span>
              </div>
              <div className="mt-1 text-[11px] text-faint">{item?.quoteCount ?? 0} 条报价 · {formatObserved(item?.observedAt)}</div>
              {item?.message ? <p className="mt-1 break-words text-[11px] leading-relaxed text-muted">{item.message}</p> : null}
            </div>
          );
        })}
      </div>
    </section>
  );
};

const PriceCard: React.FC<{ quote: GPUPriceQuote; testOnly?: boolean }> = ({ quote, testOnly = false }) => (
  <article className="border-b border-line bg-surface px-3 py-3 last:border-b-0">
    <div className="flex items-start justify-between gap-3">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="font-semibold text-ink">{quote.gpuModel}</span>
          <span className="rounded bg-surface-3 px-1.5 py-0.5 text-[10px] text-muted">{billingLabel(quote.billingMode)}</span>
        </div>
        <p className="mt-1 truncate text-xs text-muted">{providerLabel(quote.provider)} · {quote.product}</p>
      </div>
      <div className="shrink-0 text-right">
        <div className="font-mono text-base font-semibold text-ink">{formatPrice(quote.priceUsdPerGpuHour)}</div>
        <div className="text-[10px] text-faint">/ GPU · 小时</div>
      </div>
    </div>
    <dl className="mt-3 grid grid-cols-2 gap-x-3 gap-y-1 text-[11px]">
      <dt className="text-faint">原始口径</dt><dd className="text-right font-mono text-muted">{quote.currency} {quote.rawPrice} / {quote.rawUnit}</dd>
      <dt className="text-faint">GPU / 显存</dt><dd className="text-right text-muted">{quote.gpuCount} 卡{quote.memoryGiB ? ` · ${quote.memoryGiB} GiB` : ''}</dd>
      <dt className="text-faint">区域 / 可用性</dt><dd className="truncate text-right text-muted">{quote.region || '未标记'} · {quote.availability || '未标记'}</dd>
      <dt className="text-faint">观测时间</dt><dd className="text-right text-muted">{formatObserved(quote.observedAt)}</dd>
    </dl>
	{testOnly ? <span className="mt-2 inline-flex text-[11px] text-accent">验收夹具来源</span> : (
	  <a href={providerConsoleUrl(quote.provider)} target="_blank" rel="noreferrer" className="mt-2 inline-flex items-center gap-1 text-[11px] text-accent hover:text-ink">
		前往官方租用 <ExternalLink className="h-3 w-3" />
	  </a>
	)}
  </article>
);

const ReferencePrices: React.FC<{
  reference: AdaptedGPUPriceReference & { loading: boolean; error: string | null; refresh: () => Promise<void> };
}> = ({ reference }) => {
  const groups = reference.providers.filter((provider) => provider.hasFixedReference).map((provider) => ({
    provider,
    items: reference.items.filter((item) => item.provider === provider.provider),
  })).filter((group) => group.items.length > 0);
  const vast = reference.providers.find((provider) => provider.provider === 'vast');
  return (
    <section aria-labelledby="reference-prices-title" className="border-y border-amber-500/30 bg-amber-500/[0.04] py-4">
      <div className="flex flex-col gap-3 px-3 sm:px-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="max-w-3xl">
          <div className="flex items-center gap-2">
            <CalendarClock className="h-4 w-4 text-amber-300" />
            <h2 id="reference-prices-title" className="text-base font-semibold text-ink">官方核验参考价</h2>
            <span className="rounded border border-amber-500/35 bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-semibold text-amber-200">非实时</span>
          </div>
          <p className="mt-1.5 text-xs leading-relaxed text-muted">
            来自厂商官方公开定价页的人工核验基线，只用于在未配置 API 凭证时理解计费口径。不参与动态最低价、价差或历史图计算。
          </p>
        </div>
        <div className="shrink-0 text-left lg:text-right">
          <div className="text-[10px] uppercase text-faint">核验日期</div>
          <div className="mt-0.5 font-mono text-sm font-semibold text-amber-200">{reference.verifiedAt ? formatObserved(reference.verifiedAt) : '未返回'}</div>
        </div>
      </div>

      <div className="mt-3 px-3 sm:px-4">
        <DataStatus
          state={reference.loading ? 'loading' : reference.error || !reference.available ? 'unavailable' : 'stale'}
          label={reference.loading ? '读取官方核验参考' : reference.available ? '历史核验参考 · 非实时' : '官方核验参考不可用'}
          dataTime={reference.verifiedAt || reference.meta?.dataTime || 'unknown'}
          source={reference.source || reference.meta?.source}
          message={reference.error || reference.reason || reference.disclaimer}
          onRetry={() => void reference.refresh()}
          compact
        />
      </div>

      {reference.available ? (
        <>
          <div className="mt-4 grid gap-x-5 gap-y-4 px-3 sm:px-4 xl:grid-cols-3">
            {groups.map(({ provider, items }) => (
              <div key={provider.provider} className="min-w-0">
                <div className="mb-2 flex items-center justify-between gap-2 border-b border-line pb-2">
                  <span className="text-sm font-semibold text-ink">{providerLabel(provider.provider)}</span>
                  <a href={provider.sourceUrl} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-[11px] text-accent">官方价格页 <ExternalLink className="h-3 w-3" /></a>
                </div>
                <div className="divide-y divide-line/70">
                  {items.map((item) => (
                    <div key={`${item.provider}-${item.gpuModel}-${item.product}`} className="grid grid-cols-[minmax(0,1fr)_auto] gap-3 py-2 text-xs">
                      <div className="min-w-0"><div className="truncate font-medium text-ink">{item.gpuModel}</div><div className="truncate text-[10px] text-faint">{item.product} · {billingLabel(item.billingMode)} · {item.gpuCount} GPU</div></div>
                      <div className="text-right"><div className="font-mono font-semibold text-ink">{formatPrice(item.priceUsdPerGpuHour)}<span className="ml-1 text-[10px] font-normal text-faint">/ GPU·h</span></div><div className="font-mono text-[10px] text-faint">{item.currency} {item.rawPrice} / {item.rawUnit}</div></div>
                    </div>
                  ))}
                </div>
                {provider.note ? <p className="mt-1 text-[10px] leading-relaxed text-faint">{provider.note}</p> : null}
              </div>
            ))}
          </div>
          <div className="mx-3 mt-4 flex items-start gap-2 border-t border-amber-500/20 px-1 pt-3 text-xs text-amber-100 sm:mx-4">
            <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0 text-amber-300" />
            <p><strong>Vast.ai 不含固定参考价。</strong>{vast?.note ? ` ${vast.note}` : ' Vast.ai 是实时市场 offer，价格、区域和可用性持续变动，只能通过已认证动态数据展示。'}</p>
          </div>
          {reference.disclaimer ? <p className="mx-3 mt-2 text-[10px] leading-relaxed text-faint sm:mx-4">{reference.disclaimer}</p> : null}
        </>
      ) : null}
    </section>
  );
};

export const GPUPrices: React.FC = () => {
  const current = useGPUPrices();
  const reference = useGPUPriceReference();
  const admin = useAdminSession();
  const [provider, setProvider] = useState('all');
  const [gpuModel, setGpuModel] = useState('all');
  const [billingMode, setBillingMode] = useState('all');
  const [historyProvider, setHistoryProvider] = useState('');
  const [historyModel, setHistoryModel] = useState('');
  const [historyBilling, setHistoryBilling] = useState('');
  const [days, setDays] = useState(30);

  const options = useMemo(() => ({
    providers: [...new Set(current.quotes.map((item) => item.provider))].sort(),
    models: [...new Set(current.quotes.map((item) => item.gpuModel))].sort(),
    billingModes: [...new Set(current.quotes.map((item) => item.billingMode))].sort(),
  }), [current.quotes]);

  const filtered = useMemo(() => current.quotes.filter((item) =>
    (provider === 'all' || item.provider === provider) &&
    (gpuModel === 'all' || item.gpuModel === gpuModel) &&
    (billingMode === 'all' || item.billingMode === billingMode)
  ).sort((a, b) => a.priceUsdPerGpuHour - b.priceUsdPerGpuHour), [billingMode, current.quotes, gpuModel, provider]);

  useEffect(() => {
    if (!current.quotes.length) {
      setHistoryProvider('');
      setHistoryModel('');
      setHistoryBilling('');
      return;
    }
    const selected = current.quotes.find((item) =>
      item.provider === historyProvider && item.gpuModel === historyModel && item.billingMode === historyBilling
    ) || current.quotes[0];
    setHistoryProvider(selected.provider);
    setHistoryModel(selected.gpuModel);
    setHistoryBilling(selected.billingMode);
  }, [current.quotes, historyBilling, historyModel, historyProvider]);

  const historyCandidates = useMemo(() => current.quotes.filter((item) => item.provider === historyProvider), [current.quotes, historyProvider]);
  const history = useGPUPriceHistory(historyProvider && historyModel ? {
    provider: historyProvider, gpuModel: historyModel, billingMode: historyBilling, days,
  } : null);
  const historyChartData = useMemo(() => {
    const byObservation = new Map<string, number>();
    for (const point of history.points) {
      const currentMinimum = byObservation.get(point.observedAt);
      if (currentMinimum === undefined || point.priceUsdPerGpuHour < currentMinimum) {
        byObservation.set(point.observedAt, point.priceUsdPerGpuHour);
      }
    }
    return [...byObservation.entries()].map(([observedAt, priceUsdPerGpuHour]) => ({
      observedAt,
      priceUsdPerGpuHour,
      label: new Date(observedAt).toLocaleDateString(),
    }));
  }, [history.points]);
  const cheapest = filtered[0];
  const highest = filtered[filtered.length - 1];
  const spread = cheapest && highest && filtered.length > 1 ? ((highest.priceUsdPerGpuHour / cheapest.priceUsdPerGpuHour) - 1) * 100 : null;
  const statusState = current.loading && !current.meta ? 'loading' : current.error ? 'error' : current.trust.state;
	const fixtureMode = current.testOnly || current.dataMode === 'acceptance-fixture';
  const upstreamRefresh = useMemo(() => getGPUUpstreamRefreshAvailability(
    current.providers,
    current.meta,
    admin.authorized,
    (current.loading && !current.meta) || admin.state === 'checking',
  ), [admin.authorized, admin.state, current.loading, current.meta, current.providers]);
  const historyState = history.loading && !history.meta ? 'loading' : history.error ? 'error' : history.trust.state;
  const historyMessage = history.error || (history.empty
    ? '所选条件暂无已落库历史观测。'
    : history.trust.state === 'stale'
      ? '后端返回的历史数据已过期，曲线已隐藏；刷新上游后再试。'
      : history.trust.reason);

  return (
    <StockGodShell title="GPU 租金">
      <div className="space-y-6">
        <header className="border-b border-line pb-4">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div className="max-w-3xl">
              <div className="mb-2 flex items-center gap-2 text-xs font-semibold text-accent"><Server className="h-4 w-4" /> GPU 算力价格</div>
				  <h1 className="text-[24px] font-semibold text-ink">{fixtureMode ? 'GPU 租金验收夹具' : '官方 GPU 租金跟踪'}</h1>
                  <p className="mt-2 text-sm leading-relaxed text-muted">
					{fixtureMode ? '验收夹具仅验证 Service、router、SQLite 与浏览器交互；这些是测试专用数据，不是真实供应商报价。' : 'RunPod、Modal、Lambda Cloud 与 Vast.ai 的官方结构化报价。统一列仅做每 GPU 小时换算，原始计费单位始终保留。'}
                  </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <button type="button" onClick={() => void current.refresh()} disabled={current.loading} className="inline-flex h-9 items-center gap-1.5 rounded-md border border-line bg-surface px-3 text-xs font-semibold text-muted hover:text-ink disabled:opacity-50">
                <RefreshCw className={`h-3.5 w-3.5 ${current.loading ? 'animate-spin' : ''}`} /> 重新读取
              </button>
              <button type="button" onClick={() => void current.reloadConfiguration()} disabled={!admin.authorized || current.configReloading} title="从仓库外 0600 凭证文件或进程环境重新读取，不在浏览器保存密钥" className="inline-flex h-9 items-center gap-1.5 rounded-md border border-line bg-surface px-3 text-xs font-semibold text-muted hover:text-ink disabled:opacity-50">
                <RefreshCw className={`h-3.5 w-3.5 ${current.configReloading ? 'animate-spin' : ''}`} /> 重读凭证
              </button>
              <button type="button" onClick={() => void current.refreshUpstream()} disabled={!upstreamRefresh.enabled || current.upstreamRefreshing} title={upstreamRefresh.reason} aria-describedby="gpu-upstream-refresh-reason" className="inline-flex h-9 items-center gap-1.5 rounded-md border border-accent/40 bg-accent/10 px-3 text-xs font-semibold text-accent disabled:cursor-not-allowed disabled:border-line disabled:bg-surface disabled:text-faint">
                <RefreshCw className={`h-3.5 w-3.5 ${current.upstreamRefreshing ? 'animate-spin' : ''}`} /> 刷新上游
              </button>
            </div>
          </div>
          <div className="mt-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
			<DataStatus state={statusState} label={current.trust.state === 'live' ? (fixtureMode ? (current.loading ? '验收夹具 · 刷新中' : '验收夹具') : (current.loading ? '已验证快照 · 刷新中' : '已验证快照')) : undefined} dataTime={current.meta?.dataTime || 'unknown'} source={current.meta?.source} message={fixtureMode ? '测试专用数据，不得作为真实 GPU 租金证据。' : current.error || current.trust.reason} onRetry={() => void current.refresh()} />
            <span id="gpu-upstream-refresh-reason" className="text-[11px] text-faint">
              {upstreamRefresh.reason}
            </span>
          </div>
          {current.refreshError ? <p role="alert" className="mt-2 text-xs text-down">{current.refreshError}</p> : null}
          {current.configFeedback ? <p role="status" className="mt-2 text-xs text-muted">{current.configFeedback}</p> : null}
          <p className="mt-2 text-[11px] text-faint">凭证只从后端进程环境或仓库外 0600 配置文件读取，页面不接收、不回显也不存储密钥。{current.nextRefreshAt ? ` 下次计划刷新：${new Date(current.nextRefreshAt).toLocaleString()}` : ''}</p>
        </header>

		<ProviderDiagnostics statuses={current.providers} testOnly={fixtureMode} />

        <ReferencePrices reference={reference} />

        {current.trust.state !== 'live' ? (
          <section className="border border-line bg-surface px-4 py-8 text-center">
            <ShieldAlert className="mx-auto h-7 w-7 text-accent" />
            <h2 className="mt-3 text-sm font-semibold text-ink">当前无可信比较数据</h2>
            <p className="mx-auto mt-2 max-w-xl text-xs leading-relaxed text-muted">
              未配置、过期、局部失败或缺少来源时，页面不显示最低价、价差和历史曲线，也不会用静态常量填补。
            </p>
          </section>
        ) : (
          <>
            <section aria-labelledby="current-prices-title" className="space-y-3">
              <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
                <div>
                  <h2 id="current-prices-title" className="text-base font-semibold text-ink">当前报价</h2>
                  <p className="mt-1 text-xs text-faint">不同计费模式仅按换算列并排，实际成本仍受存储、带宽、区域和可用性影响。</p>
                </div>
                <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
                  <label className="text-[11px] text-faint">厂商<select aria-label="筛选厂商" value={provider} onChange={(event) => setProvider(event.target.value)} className="mt-1 block h-9 w-full min-w-0 rounded-md border border-line bg-surface px-2 text-xs text-ink"><option value="all">全部厂商</option>{options.providers.map((item) => <option key={item} value={item}>{providerLabel(item)}</option>)}</select></label>
                  <label className="text-[11px] text-faint">GPU 型号<select aria-label="筛选 GPU 型号" value={gpuModel} onChange={(event) => setGpuModel(event.target.value)} className="mt-1 block h-9 w-full min-w-0 rounded-md border border-line bg-surface px-2 text-xs text-ink"><option value="all">全部型号</option>{options.models.map((item) => <option key={item} value={item}>{item}</option>)}</select></label>
                  <label className="text-[11px] text-faint">计费模式<select aria-label="筛选计费模式" value={billingMode} onChange={(event) => setBillingMode(event.target.value)} className="mt-1 block h-9 w-full min-w-0 rounded-md border border-line bg-surface px-2 text-xs text-ink"><option value="all">全部模式</option>{options.billingModes.map((item) => <option key={item} value={item}>{billingLabel(item)}</option>)}</select></label>
                </div>
              </div>

              {filtered.length ? (
                <>
                  <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
                    <div className="border-l-2 border-up bg-surface px-3 py-2"><div className="text-[11px] text-faint">当前最低换算价</div><div className="mt-1 font-mono text-lg font-semibold text-up">{formatPrice(cheapest.priceUsdPerGpuHour)}</div><div className="truncate text-[10px] text-muted">{providerLabel(cheapest.provider)} · {cheapest.gpuModel}</div></div>
					<div className="border-l-2 border-accent bg-surface px-3 py-2"><div className="text-[11px] text-faint">筛选报价</div><div className="mt-1 font-mono text-lg font-semibold text-ink">{filtered.length}</div><div className="text-[10px] text-muted">{fixtureMode ? '仅验收夹具记录' : '仅已验证记录'}</div></div>
                    <div className="col-span-2 border-l-2 border-line bg-surface px-3 py-2 sm:col-span-1"><div className="text-[11px] text-faint">最高 / 最低价差</div><div className="mt-1 font-mono text-lg font-semibold text-ink">{spread == null ? '—' : `${spread.toFixed(0)}%`}</div><div className="text-[10px] text-muted">不等于综合成本差</div></div>
                  </div>

                  <div className="hidden overflow-hidden border border-line md:block">
                    <div className="overflow-x-auto">
                      <table className="w-full min-w-[980px] text-left text-xs">
                        <thead className="bg-surface-2 text-faint"><tr><th className="px-3 py-2">厂商 / GPU</th><th className="px-3 py-2">产品 / 计费</th><th className="px-3 py-2 text-right">USD / GPU·h</th><th className="px-3 py-2 text-right">原始价格</th><th className="px-3 py-2">GPU / 区域</th><th className="px-3 py-2">可用性 / 观测</th><th className="px-3 py-2">租用</th></tr></thead>
						<tbody className="divide-y divide-line">{filtered.map((quote) => <tr key={`${quote.provider}-${quote.offerId || quote.product}-${quote.gpuModel}-${quote.billingMode}-${quote.observedAt}`} className="bg-surface hover:bg-surface-2"><td className="px-3 py-2.5"><div className="font-semibold text-ink">{providerLabel(quote.provider)}</div><div className="font-mono text-muted">{quote.gpuModel}{quote.memoryGiB ? ` · ${quote.memoryGiB} GiB` : ''}</div></td><td className="px-3 py-2.5"><div className="text-ink">{quote.product}</div><span className="mt-1 inline-block rounded bg-surface-3 px-1.5 py-0.5 text-[10px] text-muted">{billingLabel(quote.billingMode)}</span></td><td className="px-3 py-2.5 text-right font-mono font-semibold text-ink">{formatPrice(quote.priceUsdPerGpuHour)}</td><td className="px-3 py-2.5 text-right font-mono text-muted">{quote.currency} {quote.rawPrice}<div className="text-[10px] text-faint">/ {quote.rawUnit}</div></td><td className="px-3 py-2.5 text-muted">{quote.gpuCount} 卡<div className="text-[10px] text-faint">{quote.region || '未标记'}</div></td><td className="px-3 py-2.5 text-muted">{quote.availability || '未标记'}<div className="text-[10px] text-faint">{formatObserved(quote.observedAt)}</div></td><td className="px-3 py-2.5">{fixtureMode ? <span className="text-accent">验收夹具</span> : <a href={providerConsoleUrl(quote.provider)} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-accent">前往租用 <ExternalLink className="h-3 w-3" /></a>}</td></tr>)}</tbody>
                      </table>
                    </div>
                  </div>
				  <div className="overflow-hidden border border-line md:hidden">{filtered.map((quote) => <PriceCard key={`${quote.provider}-${quote.offerId || quote.product}-${quote.gpuModel}-${quote.billingMode}-${quote.observedAt}`} quote={quote} testOnly={fixtureMode} />)}</div>
                </>
              ) : <div className="border border-line bg-surface px-4 py-8 text-center text-sm text-muted">当前筛选条件下没有报价。</div>}
            </section>

            <section aria-labelledby="history-title" className="space-y-3 border-t border-line pt-5">
              <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
				<div><h2 id="history-title" className="flex items-center gap-2 text-base font-semibold text-ink"><Gauge className="h-4 w-4 text-accent" /> {fixtureMode ? '验收夹具历史' : '真实历史观测'}</h2><p className="mt-1 text-xs text-faint">{fixtureMode ? '仅验证测试数据经后端 SQLite 落库和读取，不代表真实供应商历史。' : '仅消费后端 SQLite 中已落库的 dated history，不由浏览器轮询拼接；同批多 offer 按最低可比价展示。'}</p></div>
                <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                  <select aria-label="历史厂商" value={historyProvider} onChange={(event) => { setHistoryProvider(event.target.value); const next = current.quotes.find((item) => item.provider === event.target.value); setHistoryModel(next?.gpuModel || ''); setHistoryBilling(next?.billingMode || ''); }} className="h-9 min-w-0 rounded-md border border-line bg-surface px-2 text-xs text-ink">{options.providers.map((item) => <option key={item} value={item}>{providerLabel(item)}</option>)}</select>
                  <select aria-label="历史 GPU 型号" value={historyModel} onChange={(event) => { setHistoryModel(event.target.value); const next = historyCandidates.find((item) => item.gpuModel === event.target.value); setHistoryBilling(next?.billingMode || ''); }} className="h-9 min-w-0 rounded-md border border-line bg-surface px-2 text-xs text-ink">{[...new Set(historyCandidates.map((item) => item.gpuModel))].map((item) => <option key={item} value={item}>{item}</option>)}</select>
                  <select aria-label="历史计费模式" value={historyBilling} onChange={(event) => setHistoryBilling(event.target.value)} className="h-9 min-w-0 rounded-md border border-line bg-surface px-2 text-xs text-ink">{[...new Set(historyCandidates.filter((item) => item.gpuModel === historyModel).map((item) => item.billingMode))].map((item) => <option key={item} value={item}>{billingLabel(item)}</option>)}</select>
                  <select aria-label="历史天数" value={days} onChange={(event) => setDays(Number(event.target.value))} className="h-9 min-w-0 rounded-md border border-line bg-surface px-2 text-xs text-ink"><option value={7}>7 天</option><option value={30}>30 天</option><option value={90}>90 天</option></select>
                </div>
              </div>
			  <DataStatus state={historyState} label={history.trust.state === 'live' ? (fixtureMode || history.testOnly ? `${history.points.length} 条验收夹具观测` : `${history.points.length} 条真实观测`) : history.empty ? '暂无历史观测' : history.trust.state === 'stale' ? '历史数据已过期' : undefined} dataTime={history.meta?.dataTime || 'unknown'} source={history.meta?.source} message={fixtureMode ? '验收夹具历史仅用于测试落库链路。' : historyMessage} onRetry={() => void history.refresh()} compact />
              {history.trust.state === 'live' && historyChartData.length ? (
                <div className="h-[300px] w-full border border-line bg-surface px-1 py-3 sm:h-[360px] sm:px-3">
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={historyChartData} margin={{ top: 8, right: 12, left: 0, bottom: 4 }}>
                      <XAxis dataKey="label" stroke="var(--text-muted)" fontSize={10} tickLine={false} axisLine={false} minTickGap={24} />
                      <YAxis dataKey="priceUsdPerGpuHour" domain={['auto', 'auto']} stroke="var(--text-muted)" fontSize={10} tickLine={false} axisLine={false} tickFormatter={(value) => `$${Number(value).toFixed(2)}`} width={48} />
                      <Tooltip formatter={(value: number) => [`$${Number(value).toFixed(4)} / GPU·h`, '换算价']} labelFormatter={(_, payload) => payload[0]?.payload?.observedAt ? formatObserved(payload[0].payload.observedAt) : ''} contentStyle={{ background: 'var(--bg-card)', borderColor: 'var(--border-color)', borderRadius: 6 }} />
                      <Line type="monotone" dataKey="priceUsdPerGpuHour" stroke="var(--color-primary)" strokeWidth={2} dot={historyChartData.length < 20} activeDot={{ r: 4 }} />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              ) : !history.loading ? <div className="border border-line bg-surface px-4 py-8 text-center text-xs text-muted">{history.empty ? '所选条件暂无已落库历史观测。' : history.trust.state === 'stale' ? '历史数据已过期，曲线未展示。' : '暂无可信历史。刷新上游并累积真实观测后再查看。'}</div> : null}
            </section>
          </>
        )}
      </div>
    </StockGodShell>
  );
};
