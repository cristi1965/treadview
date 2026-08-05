import React from 'react';
import { HeatmapNodeWithPosition } from '../../types/heatmap';
import { formatPrice, formatPercent } from '../../utils/format';
import { formatMarketCapShort } from '../../utils/heatmap';

interface HeatmapTooltipProps {
  node: HeatmapNodeWithPosition | null;
  x: number;
  y: number;
  visible: boolean;
}

export const HeatmapTooltip: React.FC<HeatmapTooltipProps> = ({
  node,
  x,
  y,
  visible,
}) => {
  if (!visible || !node) return null;

  // Position tooltip to avoid going off-screen
  const tooltipWidth = 280;
  const tooltipHeight = 200;
  const padding = 16;
  
  let posX = x + 20;
  let posY = y + 20;
  
  // Adjust if tooltip would go off right edge
  if (posX + tooltipWidth > window.innerWidth - padding) {
    posX = x - tooltipWidth - 20;
  }
  
  // Adjust if tooltip would go off bottom edge
  if (posY + tooltipHeight > window.innerHeight - padding) {
    posY = y - tooltipHeight - 20;
  }

  const changeColor = node.changePercent >= 0 ? 'text-up' : 'text-down';

  return (
    <div
      className="fixed z-50 pointer-events-none"
      style={{
        left: `${posX}px`,
        top: `${posY}px`,
      }}
    >
      <div className="bg-surface border border-line rounded-lg p-4 shadow-2xl min-w-[280px]">
        {/* Header */}
        <div className="mb-3 pb-3 border-b border-line">
          <div className="flex items-center justify-between mb-1">
            <span className="text-lg font-semibold text-ink font-mono">
              {node.symbol}
            </span>
            <span className={`text-sm font-medium ${changeColor}`}>
              {formatPercent(node.changePercent)}
            </span>
          </div>
          <p className="text-sm text-muted truncate">{node.name}</p>
        </div>

        {/* Metrics */}
        <div className="space-y-2">
          <div className="flex justify-between items-center">
            <span className="text-xs text-faint">Price</span>
            <span className="text-sm font-mono text-ink">
              {formatPrice(node.price)}
            </span>
          </div>

          <div className="flex justify-between items-center">
            <span className="text-xs text-faint">Market Cap</span>
            <span className="text-sm font-mono text-ink">
              {formatMarketCapShort(node.marketCap)}
            </span>
          </div>

          <div className="flex justify-between items-center">
            <span className="text-xs text-faint">Sector</span>
            <span className="text-sm text-muted truncate max-w-[160px]">
              {node.sector}
            </span>
          </div>

          <div className="flex justify-between items-center">
            <span className="text-xs text-faint">Avg Score</span>
            <span className="text-sm font-mono text-ink">
              {node.avgScore.toFixed(1)}
            </span>
          </div>
        </div>

        {/* Footer hint */}
        <div className="mt-3 pt-3 border-t border-line">
          <p className="text-xs text-faint text-center">
            Click to view details
          </p>
        </div>
      </div>
    </div>
  );
};
