import React from 'react';
import { HoldingItem } from '../../types/portfolio';
import { formatPercent, getChangeColorClass } from '../../utils/format';

interface HoldingsCardProps {
  item: HoldingItem;
  onRemove: () => void;
  onClick: () => void;
}

export const HoldingsCard: React.FC<HoldingsCardProps> = ({ item, onRemove, onClick }) => {
  const pl = item.profitLoss;
  const plPct = item.profitLossPercent;
  const currency = item.currency || (item.venue === 'cn' ? 'CNY' : 'USD');
  const formatMoney = (value: number) =>
    new Intl.NumberFormat('zh-CN', { style: 'currency', currency, minimumFractionDigits: 2 }).format(value);

  return (
    <div
      className="cursor-pointer rounded-xl border border-line bg-surface p-4 transition hover:border-accent/20 hover:bg-surface-2"
      onClick={onClick}
    >
      <div className="mb-3 flex items-baseline justify-between">
        <div>
          <span className="font-mono text-lg font-bold text-ink">{item.symbol}</span>
          <span className="ml-2 text-sm text-muted">{item.name}</span>
        </div>
        <button
          className="text-sm text-faint transition hover:text-down"
          onClick={(e) => {
            e.stopPropagation();
            onRemove();
          }}
        >
          删除
        </button>
      </div>

      <div className="grid grid-cols-2 gap-2 text-xs text-muted">
        <div>
          数量 <span className="font-mono text-ink">{item.quantity}</span>
        </div>
        <div>
          成本 <span className="font-mono text-ink">{formatMoney(item.avgCost)}</span>
        </div>
        <div>
          行业 <span className="text-ink">{item.sector || '未分类'}</span>
        </div>
        <div>
          止损 <span className={item.stopLoss ? 'font-mono text-ink' : 'text-down'}>{item.stopLoss ? formatMoney(item.stopLoss) : '未设置'}</span>
        </div>
        {item.currentValue !== undefined && (
          <div>
            市值 <span className="font-mono text-ink">{formatMoney(item.currentValue)}</span>
          </div>
        )}
        {pl !== undefined && plPct !== undefined && (
          <div>
            盈亏{' '}
            <span className={`font-mono font-semibold ${getChangeColorClass(pl)}`}>
              {formatMoney(pl)} ({formatPercent(plPct)})
            </span>
          </div>
        )}
      </div>

      {item.notes && <p className="mt-2 truncate text-xs text-faint">{item.notes}</p>}
    </div>
  );
};
