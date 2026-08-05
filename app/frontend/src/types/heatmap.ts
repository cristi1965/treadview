export interface HeatmapNode {
  symbol: string;
  name: string;
  price: number;
  changePercent: number;
  marketCap: number;
  sector: string;
  avgScore: number;
  scores?: {
    buffett: number;
    duanyongping: number;
    serenity: number;
    druckenmiller: number;
    sentiment: number;
  };
  divergence?: number;
  segment?: string;
  subSector?: string;
  industry?: string;
  layer?: string;
  country?: string;
  venue?: 'us' | 'cn' | 'adr';
}

export interface HeatmapNodeWithPosition extends HeatmapNode {
  x: number;
  y: number;
  vx?: number;
  vy?: number;
  radius: number;
  color: string;
}

export interface HeatmapTooltip {
  node: HeatmapNodeWithPosition;
  x: number;
  y: number;
  visible: boolean;
}

export interface HeatmapConfig {
  width: number;
  height: number;
  minRadius: number;
  maxRadius: number;
  forceStrength: number;
  collisionPadding: number;
  alphaDecay: number;
}
