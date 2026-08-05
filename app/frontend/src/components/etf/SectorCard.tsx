import React from 'react';
import { ETFSector } from '../../types/etf';
import { formatAUM, formatPercent } from '../../utils/format';

interface SectorCardProps {
  sector: ETFSector;
  onClick: () => void;
}

export const SectorCard: React.FC<SectorCardProps> = ({ sector, onClick }) => {
  return (
    <div
      className="border border-line rounded-xl p-4 bg-surface transition cursor-pointer hover:bg-surface-2 hover:border-accent/20"
      onClick={onClick}
    >
      {/* 板块名称 */}
      <h3 className="text-lg font-semibold text-ink mb-2">
        {sector.name}
      </h3>
      
      {/* ETF 数量和规模 */}
      <p className="text-[13px] text-muted mb-4">
        {sector.etfCount} 只 ETF · {formatAUM(sector.aum)} AUM
      </p>
      
      {/* 性能指标网格 */}
      <div className="grid grid-cols-2 gap-3">
        {/* 5年最强 */}
        <div className="flex flex-col gap-1">
          <span className="text-[11px] text-faint uppercase tracking-wide">
            5年最强
          </span>
          <span className="font-mono text-[13px] font-semibold text-ink">
            {sector.topPerformer.ticker}
          </span>
          <span className="font-mono text-xl font-bold text-up">
            {formatPercent(sector.topPerformer.return5y, true)}
          </span>
        </div>
        
        {/* 最深回撤 */}
        <div className="flex flex-col gap-1">
          <span className="text-[11px] text-faint uppercase tracking-wide">
            最深回撤
          </span>
          <span className="font-mono text-xl font-bold text-down">
            {formatPercent(sector.maxDrawdown, false)}
          </span>
        </div>
      </div>
    </div>
  );
};
