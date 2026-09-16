import { getWithMeta, type APIResponseMeta } from './api';

export type PulseRegion = 'US' | 'CN' | 'HK' | 'TW' | 'KR' | 'EU' | 'JP';

export interface PulseCompany {
  ticker: string;
  name: string;
  layer: string; // L0..L7 or industry-specific
  segment?: string;
  region: PulseRegion | string;
  marketCapB: number;
  heat: number;
  industries: string[];
  moat?: number;
  pos52?: number;
  pct?: number;
  livePrice?: number;
  liveMcapYi?: number;
  /** merged from panel when available */
  scores?: {
    buffett: number;
    duanyongping: number;
    serenity: number;
    druckenmiller: number;
    sentiment: number;
  };
  avgScore?: number;
  divergence?: number;
}

export interface PulseIndustry {
  id: string;
  name: string;
  desc?: string;
}

export interface PulseLoadResult {
  companies: PulseCompany[];
  meta: APIResponseMeta;
  dataMode: 'live' | 'historical';
  degradedReason?: string;
}

/** Live site industry chip order + labels */
export const PULSE_INDUSTRIES: PulseIndustry[] = [
  { id: 'AI', name: 'AI 产业链', desc: 'L0 能源 → L7 端侧 · 8 层全景' },
  { id: 'rare-metals', name: '稀有 / 战略金属', desc: '稀土 / 锗 / 锡 / 钨 / 钽 / 铟 / 镓 / 铂族' },
  { id: 'humanoid', name: '人形机器人', desc: '减速器 / 丝杠 / 伺服 / 稀土永磁 / 传感器 / 整机' },
  { id: 'defense', name: '国防 / 军工', desc: '航发 / 雷达 / 隐身材料 / IMU / 特种 IC / UAV' },
  { id: 'biotech', name: '生物医药', desc: '创新药 / CRO / API / IVD / 医械 / 培养基' },
  { id: 'ev', name: '新能源车', desc: '整车 / 电池 / 锂电材料' },
  { id: 'solar-storage', name: '光伏 / 储能', desc: '组件 / 逆变器 / 储能系统' },
  { id: 'consumer', name: '消费电子', desc: '手机 / 硬件 / 消费电子' },
  { id: 'finance', name: '金融', desc: '银行 / 保险 / 资管 / 支付' },
  { id: 'real-estate', name: '地产 / REIT', desc: '开发 / REIT' },
  { id: 'software-internet', name: '软件 / 互联网', desc: '软件 / 平台 / 互联网' },
  { id: 'industrials', name: '工业 / 制造', desc: '机械 / 制造 / 工业服务' },
  { id: 'energy', name: '能源 / 油气', desc: '油气 / 能源' },
  { id: 'utilities', name: '公用事业', desc: '电力 / 燃气水务' },
  { id: 'consumer-retail', name: '大消费', desc: '可选 + 必需消费' },
];

export const PULSE_LAYERS = [
  { id: 'L0', name: '能源底座' },
  { id: 'L1', name: 'EDA · 设备 · 材料' },
  { id: 'L2', name: '晶圆 · 封装 · HBM' },
  { id: 'L3', name: 'AI 芯片' },
  { id: 'L4', name: '数据中心基建' },
  { id: 'L5', name: '云 · 模型 · 数据' },
  { id: 'L6', name: 'AI 应用' },
  { id: 'L7', name: '端侧 · 入口' },
] as const;

export const REGION_TABS: { id: 'ALL' | PulseRegion; label: string }[] = [
  { id: 'US', label: '美股' },
  { id: 'CN', label: 'A 股' },
  { id: 'HK', label: '港股' },
  { id: 'TW', label: '台股' },
  { id: 'KR', label: '韩股' },
  { id: 'EU', label: '欧' },
  { id: 'JP', label: '日' },
];

let cache: Promise<PulseLoadResult> | null = null;
let cacheRegion = 'ALL';
let cacheAt = 0;
const PULSE_CACHE_TTL_MS = 45_000;

async function fetchPulseFile(region: string = 'ALL'): Promise<PulseLoadResult> {
  const params = new URLSearchParams({ mode: 'best' });
  if (region && region !== 'ALL') params.set('region', region);
  const result = await getWithMeta<{
    companies?: PulseCompany[];
    dataMode?: 'live' | 'historical';
    degradedReason?: string;
  }>(`/api/pulse?${params.toString()}`);
  const data = result.data;
  let list = (data.companies || []) as PulseCompany[];
  if (region && region !== 'ALL') {
    list = list.filter((c) => String(c.region).toUpperCase() === region.toUpperCase());
  }
  return {
    companies: list,
    meta: result.meta,
    dataMode: data.dataMode === 'historical' ? 'historical' : 'live',
    degradedReason: data.degradedReason,
  };
}

export function invalidatePulseCache(): void {
  cache = null;
  cacheAt = 0;
}

export async function loadPulseCompanies(region: 'ALL' | PulseRegion | string = 'ALL'): Promise<PulseLoadResult> {
  const key = String(region || 'ALL').toUpperCase();
  const fresh = cache && cacheRegion === key && Date.now() - cacheAt < PULSE_CACHE_TTL_MS;
  if (!fresh) {
    cacheRegion = key;
    cacheAt = Date.now();
    cache = fetchPulseFile(key);
  }
  const request = cache!;
  try {
    return await request;
  } catch (err) {
    if (cache === request) {
      cache = null;
      cacheAt = 0;
    }
    throw err;
  }
}

export function inIndustry(c: PulseCompany, industryId: string): boolean {
  return (c.industries || []).includes(industryId);
}

export function lensValue(c: PulseCompany, lens: string): number | null {
  if (lens === '过热度' || lens === 'heat') return c.heat;
  if (lens === '分歧' || lens === 'divergence') return c.divergence ?? null;
  if (lens === '综合' || lens === 'triple') {
    if (c.avgScore && c.avgScore > 0) return c.avgScore;
    const vals = c.scores ? Object.values(c.scores).filter((v) => v > 0) : [];
    return vals.length ? Math.round(vals.reduce((a, b) => a + b, 0) / vals.length) : null;
  }
  const map: Record<string, keyof NonNullable<PulseCompany['scores']>> = {
    巴菲特: 'buffett',
    段永平: 'duanyongping',
    Serenity: 'serenity',
    德鲁肯米勒: 'druckenmiller',
    情绪资金面: 'sentiment',
  };
  const key = map[lens];
  if (!key || !c.scores) return null;
  const v = c.scores[key];
  return v > 0 ? v : null;
}
