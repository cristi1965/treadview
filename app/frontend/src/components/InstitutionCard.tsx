import React from 'react';
import { Avatar } from './Avatar';
import type { Investor } from '../types/whales';

interface InstitutionCardProps {
  investor: Investor;
  onClick?: () => void;
}

export const InstitutionCard: React.FC<InstitutionCardProps> = ({ investor, onClick }) => {
  const topSymbol = investor.topStock?.symbol || '—';

  return (
    <button
      type="button"
      onClick={onClick}
      className="group relative flex h-[170px] w-[150px] shrink-0 cursor-pointer flex-col rounded-xl border border-line bg-surface p-3 text-center transition hover:border-line-2 focus:outline-none focus:ring-2 focus:ring-accent/35"
    >
      {/* 头像 */}
      <Avatar
        src={investor.avatar}
        slug={investor.slug}
        letter={investor.name ? investor.name.charAt(0) : 'I'}
        size={56}
        className="mx-auto"
      />

      {/* 投资者名称 */}
      <div className="mt-2 h-[18px] truncate text-[13px] font-semibold text-ink transition group-hover:text-accent">
        {investor.name}
      </div>

      {/* 机构名称 */}
      <div className="mt-0.5 h-[28px] overflow-hidden text-[10px] leading-[14px] text-faint">
        {investor.company}
      </div>

      {/* 顶部持仓股票 */}
      <div className="mt-auto flex min-h-[18px] items-center justify-center gap-1 text-[11px]">
        {topSymbol !== '—' && (
          <span className="flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full border border-line bg-surface-3 font-mono text-[7px] font-semibold text-accent">
            {topSymbol.slice(0, 1)}
          </span>
        )}
        <span className="max-w-[54px] truncate font-medium text-accent">{topSymbol}</span>
        <span className="font-mono text-muted tabular-nums">
          {investor.topStock?.percentage !== undefined ? `${investor.topStock.percentage.toFixed(2)}%` : '—'}
        </span>
      </div>

      {/* 持仓数量 */}
      <div className="mt-1 h-[14px] text-[10px] text-faint">
        {investor.holdings} 持仓
      </div>
    </button>
  );
};
