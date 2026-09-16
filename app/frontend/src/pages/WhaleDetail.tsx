import React, { useState, useEffect } from 'react';
import { AlertTriangle, ChevronLeft, RefreshCw } from 'lucide-react';
import type { WhaleDetail, Holding } from '../types/whales';
import { get as apiGet } from '../utils/api';

interface WhaleDetailProps {
  slug: string;
}

export const WhaleDetailPage: React.FC<WhaleDetailProps> = ({ slug }) => {
  const [detail, setDetail] = useState<WhaleDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [retryNonce, setRetryNonce] = useState(0);
  const [selectedPeriod, setSelectedPeriod] = useState('');

  useEffect(() => {
    setSelectedPeriod('');
  }, [slug]);

  useEffect(() => {
    let cancelled = false;
    const loadDetail = async () => {
      setLoading(true);
      setError('');
      try {
        const query = selectedPeriod ? `?reportPeriod=${encodeURIComponent(selectedPeriod)}` : '';
        const data = await apiGet<WhaleDetail>(`/api/whales/gurus/${slug}${query}`);
        if (!cancelled) setDetail(data);
      } catch (error) {
        console.error('Failed to load whale detail:', error);
        if (!cancelled) {
          setDetail(null);
          setError(error instanceof Error ? error.message : '投资者资料加载失败');
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    void loadDetail();
    return () => {
      cancelled = true;
    };
  }, [slug, selectedPeriod, retryNonce]);

  if (loading) {
    return (
      <div className="flex min-h-[420px] items-center justify-center">
        <div className="text-muted">加载中...</div>
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="flex min-h-[420px] items-center justify-center rounded-lg border border-line bg-surface px-5">
        <div className="max-w-md text-center">
          <AlertTriangle className="mx-auto h-6 w-6 text-down" />
          <h1 className="mt-3 text-lg font-semibold text-ink">投资者资料暂不可用</h1>
          <p className="mt-2 text-sm text-muted">{error || '未找到该投资者，或当前数据源无法响应。'}</p>
          <div className="mt-4 flex flex-wrap justify-center gap-2">
            <button type="button" onClick={() => setRetryNonce((value) => value + 1)} className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm text-ink hover:bg-surface-2">
              <RefreshCw className="h-4 w-4" />重试
            </button>
            <a href="/whales" className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm text-ink hover:bg-surface-2">
              <ChevronLeft className="h-4 w-4" />返回聪明钱
            </a>
          </div>
        </div>
      </div>
    );
  }

  const hasHoldings = (detail.holdings || []).length > 0;

  const getActionColor = (action: string) => {
    switch (action) {
      case 'buy':
      case 'new':
        return 'text-[#2ebd85]';
      case 'sell':
        return 'text-[#f6465d]';
      default:
        return 'text-muted';
    }
  };

  return (
    <div className="text-foreground">
      <div className="mx-auto max-w-[1152px]">
        {/* 面包屑导航 */}
        <div className="mb-3 flex items-center gap-2 text-xs">
          <a href="/whales" className="text-muted hover:text-ink transition">
            聪明钱
          </a>
          <span className="text-faint">/</span>
          <span className="text-muted">{detail.name}</span>
        </div>

        {/* 头部信息 */}
        <header className="mb-5">
          {/* 名称 */}
          <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
            <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-ink">
              {detail.name}
            </h1>
            <div className="text-lg text-muted">{detail.nameEn}</div>
          </div>

          {/* 机构和日期 */}
          <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted">
            <span>{detail.company}</span>
            <span>{detail.reportType}</span>
            <span>·</span>
            <span>{detail.reportPeriod ? `报告期 ${detail.reportPeriod}` : `来源日期 ${detail.sourceAsOf || '未提供'}`}</span>
            {detail.reportPeriod && <span>申报日 {detail.filingDate || '未知'}</span>}
            {(detail.availablePeriods?.length || 0) > 1 && (
              <label className="inline-flex items-center gap-1 text-xs">
                历史期次
                <select
                  value={selectedPeriod || detail.reportPeriod || ''}
                  onChange={(event) => setSelectedPeriod(event.target.value)}
                  className="rounded-md border border-line bg-surface px-2 py-1 text-ink"
                >
                  {detail.availablePeriods?.map((period) => (
                    <option key={period} value={period}>{period}</option>
                  ))}
                </select>
              </label>
            )}
          </div>
          <div className={`mt-2 text-xs ${detail.stale ? 'text-down' : 'text-faint'}`}>
            来源 {detail.source || '未知'} · 同步时间 {detail.syncedAt || '未知'}
            {detail.reportPeriod ? ` · Accession ${detail.accession || '未知'}` : ' · 非 SEC 原始申报'}
            {detail.sourceURL && <>{' · '}<a className="underline hover:text-accent" href={detail.sourceURL} target="_blank" rel="noreferrer">查看来源</a></>}
            {detail.stale ? ' · 请勿视为最新持仓' : ''}
          </div>

          {/* 统计标签 */}
          {hasHoldings && <div className="mt-3 flex flex-wrap gap-2 text-xs">
            <span className="inline-flex items-center gap-1 rounded-md bg-surface px-2.5 py-1 text-ink">
              持仓 {detail.totalHoldings}
            </span>
            <span className="inline-flex items-center gap-1 rounded-md bg-surface px-2.5 py-1 text-ink">
              前十集中度 {detail.topTenConcentration}%
            </span>
            <span className="inline-flex items-center gap-1 rounded-md bg-surface px-2.5 py-1 text-[#2ebd85]">
              本季新建/加仓 {detail.newPositions}
            </span>
            <span className="inline-flex items-center gap-1 rounded-md bg-surface px-2.5 py-1 text-[#f6465d]">
              减持/清仓 {detail.reducedPositions}
            </span>
          </div>}
        </header>

        {/* 持仓列表 */}
        {hasHoldings ? <div className="divide-y divide-line/60 overflow-hidden rounded-lg border border-line bg-surface">
          {(detail.holdings || []).map((holding) => (
            <a
              key={holding.rank}
              href={`/stock/${holding.symbol}`}
              className="flex items-center gap-3 px-3 py-2.5 transition hover:bg-surface-2 sm:px-4"
            >
              {/* 排名 */}
              <div className="w-6 shrink-0 text-right font-mono text-[10px] tabular-nums text-faint">
                {holding.rank}
              </div>

              {/* 股票代码 */}
              <div className="min-w-[60px] text-[13px] font-semibold text-ink">
                {holding.symbol}
              </div>

              {/* 公司名称 */}
              <div className="flex-1 truncate text-[13px] text-muted">
                {holding.name}
              </div>

              {/* 机构数 */}
              {holding.consensusCount > 0 && (
                <div className="shrink-0 text-[11px] text-faint">
                  同期同类 {holding.consensusCount} 家
                </div>
              )}

              {/* 占比 */}
              <div className="w-14 shrink-0 text-right font-mono text-[13px] font-semibold tabular-nums text-ink">
                {holding.marketShare.toFixed(1)}%
              </div>

              {/* 操作标签 */}
              <div className={`shrink-0 text-[11px] font-medium ${getActionColor(holding.action)}`}>
                {holding.actionLabel}
              </div>
            </a>
          ))}
        </div> : (
          <div className="rounded-lg border border-warning/40 bg-warning/5 px-5 py-6">
            <h2 className="text-base font-semibold text-ink">暂无可核验的持仓披露</h2>
            <p className="mt-2 text-sm leading-6 text-muted">
              当前仅保留投资者资料，数据库没有与报告期和来源匹配的持仓记录，因此不展示旧的持仓数量、集中度或重仓股。
            </p>
            <div className="mt-4 flex flex-wrap gap-2">
              <button type="button" onClick={() => setRetryNonce((value) => value + 1)} className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm text-ink hover:bg-surface-2">
                <RefreshCw className="h-4 w-4" />重新检查
              </button>
              <a href="/settings" className="rounded-md border border-line px-3 py-2 text-sm text-ink hover:bg-surface-2">打开数据修复入口</a>
            </div>
          </div>
        )}

        {/* 免责声明 */}
        <p className="mt-4 text-[11px] leading-relaxed text-faint">
          数据来源 = {detail.source || '未知'}；{detail.reportType === 'SEC 13F'
            ? '13F 为季度披露，通常存在申报滞后，占比为该报告期组合权重，并非实时仓位。'
            : '当前为持仓披露整理，报告期和更新频率以所示来源为准，并非实时仓位。'}
          “同期同类”表示同一报告期、同一机构类别中持有该证券的申报主体数。共识不代表正确 · 非投资建议。
        </p>
      </div>
    </div>
  );
};
