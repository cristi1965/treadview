import { useCallback, useEffect, useRef, useState } from 'react';
import { APIError, getWithMeta, type APIResponseMeta } from '../utils/api';
import { assessDataTrust, type DataTrustAssessment } from '../utils/dataTrust';
import {
  selectTrustedQDIIItems,
  type CompleteQDIIPremiumItem,
  type QDIIPremiumResponse,
} from '../utils/qdiiPremiums';

interface UseQDIIPremiumsOptions {
  pollMs?: number;
  onLive?: (items: CompleteQDIIPremiumItem[]) => void;
  enabled?: boolean;
}

export interface QDIIPremiumsState {
  items: CompleteQDIIPremiumItem[];
  loading: boolean;
  error: string | null;
  meta: APIResponseMeta | null;
  trust: DataTrustAssessment;
  refresh: () => Promise<void>;
}

export const useQDIIPremiums = ({ pollMs = 30_000, onLive, enabled = true }: UseQDIIPremiumsOptions = {}): QDIIPremiumsState => {
  const [items, setItems] = useState<CompleteQDIIPremiumItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [meta, setMeta] = useState<APIResponseMeta | null>(null);
  const [trust, setTrust] = useState<DataTrustAssessment>(() => assessDataTrust(null, false));
  const onLiveRef = useRef(onLive);

  useEffect(() => {
    onLiveRef.current = onLive;
  }, [onLive]);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await getWithMeta<QDIIPremiumResponse>('/api/etf/premiums');
      const selection = selectTrustedQDIIItems(result.data, result.meta);
      setMeta(result.meta);
      setTrust(selection.trust);
      setItems(selection.items);
      if (selection.trust.state === 'live') onLiveRef.current?.(selection.items);
    } catch (err) {
      const errorMeta = err instanceof APIError ? err.meta || null : null;
      setMeta(errorMeta);
      setTrust(assessDataTrust(errorMeta, false));
      setItems([]);
      setError(err instanceof Error ? err.message : '无法获取 QDII 溢价数据');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!enabled) {
      setLoading(false);
      return;
    }
    void refresh();
    if (pollMs <= 0) return;
    const interval = window.setInterval(() => void refresh(), pollMs);
    return () => window.clearInterval(interval);
  }, [enabled, pollMs, refresh]);

  return { items, loading, error, meta, trust, refresh };
};
