import React from 'react';
import { WatchlistItem } from '../../types/portfolio';
import { formatPrice, formatPercent, getChangeColorClass } from '../../utils/format';

interface WatchlistCardProps {
  item: WatchlistItem;
  onRemove: () => void;
  onClick: () => void;
}

export const WatchlistCard: React.FC<WatchlistCardProps> = ({
  item,
  onRemove,
  onClick
}) => {
  return (
    <div
      className="border border-line rounded-xl p-4 bg-surface transition cursor-pointer hover:bg-surface-2 hover:border-accent/20"
      onClick={onClick}
    >
      <div className="flex items-baseline justify-between mb-3">
        <div>
          <span className="font-mono text-lg font-bold text-ink">
            {item.symbol}
          </span>
          <span className="ml-2 text-sm text-muted">
            {item.name}
          </span>
        </div>
        
        <button
          className="text-sm text-faint hover:text-accent transition"
          onClick={(e) => {
            e.stopPropagation();
            onRemove();
          }}
        >
          ☆ 移除
        </button>
      </div>
      
      {item.price !== undefined && (
        <div className="flex items-baseline gap-3">
          <span className="font-mono text-base text-ink">
            {formatPrice(item.price)}
          </span>
          
          {item.changePercent !== undefined && (
            <span className={`font-mono text-sm font-semibold ${getChangeColorClass(item.changePercent)}`}>
              {formatPercent(item.changePercent)}
            </span>
          )}
        </div>
      )}
      
      {item.avgScore !== undefined && (
        <div className="mt-2 text-xs text-muted">
          五方均分: <span className="font-semibold text-ink">{item.avgScore}</span>
        </div>
      )}
    </div>
  );
};
