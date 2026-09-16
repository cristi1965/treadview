export interface DataTrustMeta {
  source?: string;
  dataTime?: string;
  stale?: boolean;
  staleReason?: string;
  partialErrors?: string[];
}

export type DataTrustState = 'live' | 'stale' | 'unavailable';

export interface DataTrustAssessment {
  state: DataTrustState;
  reason: string;
}

export const assessDataTrust = (
  meta: DataTrustMeta | null | undefined,
  hasUsablePayload: boolean,
): DataTrustAssessment => {
  if (!meta) return { state: 'unavailable', reason: '响应未提供数据元信息' };
  if (!meta.source?.trim()) return { state: 'unavailable', reason: '响应未提供数据来源' };

  const dataTime = meta.dataTime?.trim() || '';
  const parsedDataTime = Date.parse(dataTime);
  if (!dataTime || dataTime === 'unknown' || !Number.isFinite(parsedDataTime)) {
    return { state: 'unavailable', reason: '关键输入的数据时间未知' };
  }
  if (parsedDataTime > Date.now() + 60_000) {
    return { state: 'unavailable', reason: '数据时间晚于当前时间' };
  }
  if (meta.partialErrors?.length) {
    return { state: 'unavailable', reason: `部分数据错误：${meta.partialErrors.join('；')}` };
  }
  if (meta.stale) {
    return { state: 'stale', reason: meta.staleReason?.trim() || '数据已过期' };
  }
  if (!hasUsablePayload) return { state: 'unavailable', reason: '响应没有完整可用数据' };
  return { state: 'live', reason: '' };
};
