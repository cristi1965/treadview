import React, { useState, useEffect } from 'react';
import { ChevronLeft } from 'lucide-react';
import type { WhaleDetail, Holding } from '../types/whales';
import { apiUrl } from '../utils/api';

interface WhaleDetailProps {
  slug: string;
}

export const WhaleDetailPage: React.FC<WhaleDetailProps> = ({ slug }) => {
  const [detail, setDetail] = useState<WhaleDetail | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadDetail();
  }, [slug]);

  const loadDetail = async () => {
    setLoading(true);
    try {
      const response = await fetch(apiUrl(`/api/whales/gurus/${slug}`));
      if (response.ok) {
        const data = await response.json();
        setDetail(data);
      }
    } catch (error) {
      console.error('Failed to load whale detail:', error);
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="flex min-h-[420px] items-center justify-center">
        <div className="text-muted">加载中...</div>
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="flex min-h-[420px] items-center justify-center rounded-xl border border-line bg-surface">
        <div className="text-muted">未找到该投资者</div>
      </div>
    );
  }

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
            <span>{detail.updatedAt}</span>
          </div>

          {/* 统计标签 */}
          <div className="mt-3 flex flex-wrap gap-2 text-xs">
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
          </div>
        </header>

        {/* 持仓列表 */}
        <div className="divide-y divide-line/60 overflow-hidden rounded-xl border border-line bg-surface">
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
                  均 {holding.consensusCount}
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
        </div>

        {/* 免责声明 */}
        <p className="mt-4 text-[11px] leading-relaxed text-faint">
          数据 = Dataroma 13F(季度披露,有 ~45 天滞后);占比为组合权重,非实时。
          「均」= 五方均分。共识 ≠ 正确,大佬也会一起踏空 · 非投资建议。
        </p>
      </div>
    </div>
  );
};
