import * as d3 from 'd3';
import { HeatmapNode, HeatmapNodeWithPosition } from '../types/heatmap';

/** Score / heat color: cold purple/blue → green → yellow → hot red (live-like) */
export function getScoreHeatColor(score: number): string {
  if (score >= 85) return '#ef4444';
  if (score >= 75) return '#f97316';
  if (score >= 60) return '#eab308';
  if (score >= 45) return '#22c55e';
  if (score >= 30) return '#22d3ee';
  if (score >= 15) return '#3b82f6';
  return '#7c3aed';
}

/**
 * Get color based on change percent
 * Green for positive, red for negative, gray for neutral
 */
export function getNodeColor(changePercent: number): string {
  if (changePercent > 3) return '#22c55e';
  if (changePercent > 1) return '#4ade80';
  if (changePercent > 0) return '#86efac';
  if (changePercent === 0) return '#6b7280';
  if (changePercent > -1) return '#fca5a5';
  if (changePercent > -3) return '#f87171';
  return '#ef4444';
}

export function calculateRadius(
  marketCap: number,
  allMarketCaps: number[],
  minRadius: number,
  maxRadius: number
): number {
  const positive = allMarketCaps.filter((v) => v > 0);
  const min = Math.min(...(positive.length ? positive : [1]));
  const max = Math.max(...(positive.length ? positive : [1]));
  const safeCap = Math.max(marketCap, min);
  const logMarketCap = Math.log(safeCap);
  const logMin = Math.log(min);
  const logMax = Math.log(max);
  const normalized = logMax === logMin ? 0.5 : (logMarketCap - logMin) / (logMax - logMin);
  return minRadius + Math.max(0, Math.min(1, normalized)) * (maxRadius - minRadius);
}

export type HeatmapColorMode = 'change' | 'score';

export type HeatmapLayoutMode = 'force' | 'layer-scatter';

const LAYER_BANDS = [
  'L0能源底座',
  'L1EDA · 设备 · 材料',
  'L2晶圆 · 封装 · HBM',
  'L3AI 芯片',
  'L4数据中心基建',
  'L5云 · 模型 · 数据',
  'L6AI 应用',
  'L7端侧 · 入口',
] as const;

export function layerBandIndex(layer?: string): number {
  if (!layer) return 6;
  const i = LAYER_BANDS.findIndex((l) => layer.startsWith(l.slice(0, 2)) || layer === l);
  return i >= 0 ? i : 6;
}

/**
 * Prepare nodes with position and visual properties
 */
export function prepareNodes(
  nodes: HeatmapNode[],
  width: number,
  height: number,
  minRadius: number,
  maxRadius: number,
  colorMode: HeatmapColorMode = 'score',
  scoreForColor?: (node: HeatmapNode) => number,
  layout: HeatmapLayoutMode = 'force'
): HeatmapNodeWithPosition[] {
  const allMarketCaps = nodes.map((n) => n.marketCap);
  const padX = 56;
  const padY = 28;
  const usableW = Math.max(40, width - padX * 2);
  const usableH = Math.max(40, height - padY * 2);
  const bandH = usableH / LAYER_BANDS.length;

  return nodes.map((node, idx) => {
    const score = scoreForColor ? scoreForColor(node) : node.avgScore;
    const clamped = Math.max(0, Math.min(100, score || 0));
    const band = layerBandIndex(node.layer);
    const jitterX = ((idx % 7) - 3) * 1.2;
    const jitterY = ((idx % 5) - 2) * 1.1;
    const x =
      layout === 'layer-scatter'
        ? padX + (clamped / 100) * usableW + jitterX
        : width / 2 + (Math.random() - 0.5) * width * 0.8;
    const y =
      layout === 'layer-scatter'
        ? padY + band * bandH + bandH * 0.5 + jitterY
        : height / 2 + (Math.random() - 0.5) * height * 0.8;

    return {
      ...node,
      x,
      y,
      radius: calculateRadius(node.marketCap, allMarketCaps, minRadius, maxRadius),
      color: colorMode === 'change' ? getNodeColor(node.changePercent) : getScoreHeatColor(score),
      _score: clamped,
      _band: band,
    } as HeatmapNodeWithPosition & { _score?: number; _band?: number };
  });
}

/**
 * Create D3 force simulation
 */
export function createForceSimulation(
  nodes: HeatmapNodeWithPosition[],
  width: number,
  height: number,
  forceStrength: number,
  collisionPadding: number,
  alphaDecay: number,
  layout: HeatmapLayoutMode = 'force'
) {
  const padX = 56;
  const padY = 28;
  const usableW = Math.max(40, width - padX * 2);
  const usableH = Math.max(40, height - padY * 2);
  const bandH = usableH / LAYER_BANDS.length;

  const sim = d3
    .forceSimulation(nodes)
    .force(
      'collision',
      d3.forceCollide<HeatmapNodeWithPosition>((d) => d.radius + collisionPadding).iterations(2)
    )
    .alphaDecay(alphaDecay);

  if (layout === 'layer-scatter') {
    sim
      .force(
        'x',
        d3
          .forceX<HeatmapNodeWithPosition>((d) => {
            const score = Math.max(0, Math.min(100, (d as any)._score ?? d.avgScore ?? 50));
            return padX + (score / 100) * usableW;
          })
          .strength(0.85)
      )
      .force(
        'y',
        d3
          .forceY<HeatmapNodeWithPosition>((d) => {
            const band = (d as any)._band ?? layerBandIndex(d.layer);
            return padY + band * bandH + bandH * 0.5;
          })
          .strength(0.9)
      )
      .force('charge', d3.forceManyBody().strength(-6));
  } else {
    sim
      .force('charge', d3.forceManyBody().strength(forceStrength))
      .force('center', d3.forceCenter(width / 2, height / 2))
      .force('x', d3.forceX(width / 2).strength(0.05))
      .force('y', d3.forceY(height / 2).strength(0.05));
  }

  return sim;
}

/** Draw layer/score axes for scatter layout. */
export function drawLayerScatterGuides(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number
) {
  const dpr = window.devicePixelRatio || 1;
  const padX = 56;
  const padY = 28;
  const usableW = Math.max(40, width - padX * 2);
  const usableH = Math.max(40, height - padY * 2);
  const bandH = usableH / LAYER_BANDS.length;

  ctx.save();
  ctx.scale(dpr, dpr);

  ctx.strokeStyle = 'rgba(236,237,240,0.06)';
  ctx.lineWidth = 1;
  for (let i = 0; i <= LAYER_BANDS.length; i += 1) {
    const y = padY + i * bandH;
    ctx.beginPath();
    ctx.moveTo(padX, y);
    ctx.lineTo(padX + usableW, y);
    ctx.stroke();
  }
  for (const tick of [0, 25, 50, 75, 100]) {
    const x = padX + (tick / 100) * usableW;
    ctx.beginPath();
    ctx.moveTo(x, padY);
    ctx.lineTo(x, padY + usableH);
    ctx.stroke();
    ctx.fillStyle = 'rgba(236,237,240,0.35)';
    ctx.font = '10px ui-monospace, SFMono-Regular, Menlo, monospace';
    ctx.textAlign = 'center';
    ctx.fillText(String(tick), x, height - 10);
  }

  ctx.fillStyle = 'rgba(236,237,240,0.45)';
  ctx.font = '10px ui-sans-serif, system-ui, sans-serif';
  ctx.textAlign = 'left';
  LAYER_BANDS.forEach((label, i) => {
    const y = padY + i * bandH + bandH * 0.5;
    ctx.fillText(label.replace(/^L(\d)/, 'L$1 '), 8, y + 3);
  });

  ctx.restore();
}

/**
 * Draw nodes on canvas (does not clear — caller owns background/guides).
 */
export function drawNodes(
  ctx: CanvasRenderingContext2D,
  nodes: HeatmapNodeWithPosition[],
  highlightedNode: HeatmapNodeWithPosition | null
) {
  const dpr = window.devicePixelRatio || 1;
  ctx.save();
  ctx.scale(dpr, dpr);

  // Draw all nodes
  nodes.forEach((node) => {
    // Skip if position is invalid
    if (!node.x || !node.y || !node.radius) {
      return;
    }

    const isHighlighted = highlightedNode?.symbol === node.symbol;

    ctx.beginPath();
    ctx.arc(node.x, node.y, node.radius, 0, 2 * Math.PI);
    ctx.fillStyle = node.color;
    ctx.globalAlpha = isHighlighted ? 1 : 0.82;
    ctx.fill();

    if (isHighlighted) {
      ctx.strokeStyle = '#ffffff';
      ctx.lineWidth = 2;
      ctx.stroke();
    }

    ctx.globalAlpha = 1;

    if (node.radius >= 12) {
      ctx.fillStyle = '#ffffff';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';

      const symbolToShow = node.symbol;

      if (node.radius >= 20) {
        ctx.font = 'bold 9px sans-serif';
        ctx.fillText(symbolToShow, node.x, node.y - 5);
        ctx.font = '500 8px sans-serif';
        const changeStr = (node.changePercent > 0 ? '+' : '') + node.changePercent.toFixed(1) + '%';
        ctx.fillText(changeStr, node.x, node.y + 6);
      } else {
        ctx.font = 'bold 9px sans-serif';
        ctx.fillText(symbolToShow, node.x, node.y);
      }
    }
  });

  ctx.restore();
}

/**
 * Find node at position (for hover detection)
 */
export function findNodeAtPosition(
  nodes: HeatmapNodeWithPosition[],
  x: number,
  y: number
): HeatmapNodeWithPosition | null {
  // Iterate in reverse order to check top nodes first
  for (let i = nodes.length - 1; i >= 0; i--) {
    const node = nodes[i];
    const dx = x - node.x;
    const dy = y - node.y;
    const distance = Math.sqrt(dx * dx + dy * dy);
    
    if (distance <= node.radius) {
      return node;
    }
  }
  
  return null;
}

/**
 * Format market cap for display
 */
export function formatMarketCapShort(marketCap: number): string {
  if (marketCap >= 1e12) {
    return `$${(marketCap / 1e12).toFixed(2)}T`;
  }
  if (marketCap >= 1e9) {
    return `$${(marketCap / 1e9).toFixed(2)}B`;
  }
  if (marketCap >= 1e6) {
    return `$${(marketCap / 1e6).toFixed(2)}M`;
  }
  return `$${marketCap.toFixed(0)}`;
}
