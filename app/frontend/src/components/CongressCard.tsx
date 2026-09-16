import React from 'react';
import { ExternalLink } from 'lucide-react';
import { Avatar } from './Avatar';
import type { CongressMember } from '../types/whales';

interface CongressCardProps {
  member: CongressMember;
  onClick?: () => void;
}

export const CongressCard: React.FC<CongressCardProps> = ({ member, onClick }) => {
  const partyLabel = member.party === 'D' ? 'D' : 'R';
  const tradeLabel = member.latestTrade?.action === 'buy' ? 'BUY' : 'SELL';

  return (
    <article className="group relative flex min-h-[220px] w-full max-w-[190px] flex-col rounded-xl border border-line bg-surface p-3 text-center transition hover:border-line-2">
      <button type="button" onClick={onClick} className="absolute inset-0 z-10 rounded-xl focus:outline-none focus:ring-2 focus:ring-accent/35" aria-label={`查看 ${member.name} 披露详情`} />
      {/* Hot 标签 */}
      {member.isHot && member.latestTrade?.verified && (
        <div className="absolute left-2 top-2 rounded bg-accent/10 px-1.5 py-0.5 text-[10px] font-semibold text-accent">
          Hot
        </div>
      )}
      {!member.latestTrade?.verified && (
        <div className="absolute left-2 top-2 rounded bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-semibold text-amber-300">
          未核验
        </div>
      )}

      {/* 头像 */}
      <Avatar
        src={member.photoUrl}
        name={member.name}
        slug={member.id}
        letter={member.name ? member.name.charAt(0) : 'C'}
        size={56}
        party={member.party}
        className="mx-auto"
      />

      {/* 议员姓名 */}
      <div className="mt-2 h-[18px] truncate text-[13px] font-semibold text-ink transition group-hover:text-accent">
        {member.name}
      </div>

      {/* 党派 + 选区 */}
      <div className="mt-0.5 flex h-[14px] items-center justify-center gap-1 text-[10px] text-faint">
        <span
          className={`h-1.5 w-1.5 rounded-full ${
            member.party === 'D' ? 'bg-[#3B82F6]' : 'bg-[#EF4444]'
          }`}
        />
        <span>{partyLabel} · {member.state}{member.district}</span>
      </div>

      {/* 交易操作 + 股票 */}
      <div className="mt-auto flex min-h-[18px] items-center justify-center gap-1">
        <span className="text-[13px] text-muted">{tradeLabel}</span>
        <span className="text-[13px] font-semibold text-ink">
          {member.latestTrade?.symbol || '—'}
        </span>
      </div>

      {/* 交易金额 + 日期 */}
      <div className="tnum mt-1 h-[14px] truncate font-mono text-[10px] text-faint">
        {member.latestTrade?.amount || '—'} · {member.latestTrade?.date || '—'}
      </div>
      <div className="relative z-10 mt-2 border-t border-line/70 pt-2 text-[10px] leading-4 text-faint">
        <div className="truncate" title={member.latestTrade?.source || 'unknown'}>
          来源 {member.latestTrade?.source || 'unknown'} · 申报 {member.latestTrade?.filingDate || 'unknown'}
        </div>
        {member.latestTrade?.sourceURL && member.latestTrade.sourceURL !== 'unknown' ? (
          <a href={member.latestTrade.sourceURL} target="_blank" rel="noreferrer" className="relative z-20 mt-1 inline-flex items-center gap-1 text-accent hover:underline">
            <ExternalLink className="h-3 w-3" /> 原文 {member.latestTrade.filingId && member.latestTrade.filingId !== 'unknown' ? member.latestTrade.filingId : ''}
          </a>
        ) : (
          <span className="mt-1 block text-down">原文标识 unknown</span>
        )}
      </div>
    </article>
  );
};
