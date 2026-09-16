import { useCallback, useEffect, useState } from 'react';
import {
  APIError,
  apiUrl,
  authorizedFetch,
  getWithMeta,
} from '../utils/api';
import {
  adaptGPUPriceHistoryResponse,
  adaptGPUPriceReferenceResponse,
  adaptGPUPricesResponse,
} from '../utils/gpuPrices';
import type {
  AdaptedGPUPriceHistory,
  AdaptedGPUPriceReference,
  AdaptedGPUPrices,
  GPUPriceHistoryResponse,
  GPUPriceReferenceResponse,
  GPUPricesResponse,
} from '../types/gpuPrices';

const emptyPrices = (): AdaptedGPUPrices => adaptGPUPricesResponse(null, null);
const emptyHistory = (): AdaptedGPUPriceHistory => adaptGPUPriceHistoryResponse(null, null);
const emptyReference = (): AdaptedGPUPriceReference => adaptGPUPriceReferenceResponse(null, null);

export const useGPUPrices = (pollMs = 60_000) => {
  const [result, setResult] = useState<AdaptedGPUPrices>(emptyPrices);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [upstreamRefreshing, setUpstreamRefreshing] = useState(false);
  const [refreshError, setRefreshError] = useState<string | null>(null);
  const [configReloading, setConfigReloading] = useState(false);
  const [configFeedback, setConfigFeedback] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await getWithMeta<GPUPricesResponse>('/api/gpu-prices');
      setResult(adaptGPUPricesResponse(response.data, response.meta));
    } catch (err) {
      const meta = err instanceof APIError ? err.meta || null : null;
      const payload = err instanceof APIError ? err.data as GPUPricesResponse | undefined : undefined;
      setResult(adaptGPUPricesResponse(payload, meta));
      setError(err instanceof Error ? err.message : 'GPU 价格请求失败');
    } finally {
      setLoading(false);
    }
  }, []);

  const refreshUpstream = useCallback(async () => {
    setUpstreamRefreshing(true);
    setRefreshError(null);
    try {
      const response = await authorizedFetch(apiUrl('/api/gpu-prices/refresh'), {
        method: 'POST',
        cache: 'no-store',
        headers: { 'Content-Type': 'application/json' },
      });
      const body = await response.json().catch(() => ({})) as { error?: string };
      if (!response.ok) throw new APIError(body.error || `GPU 价格刷新失败 (${response.status})`, response.status, body);
      await refresh();
    } catch (err) {
      setRefreshError(err instanceof Error ? err.message : 'GPU 价格刷新失败');
    } finally {
      setUpstreamRefreshing(false);
    }
  }, [refresh]);

  const reloadConfiguration = useCallback(async () => {
    setConfigReloading(true);
    setConfigFeedback(null);
    try {
      const response = await authorizedFetch(apiUrl('/api/gpu-prices/reload-config'), {
        method: 'POST', cache: 'no-store', headers: { 'Content-Type': 'application/json' },
      });
      const body = await response.json().catch(() => ({})) as { error?: string; status?: string };
      if (!response.ok) throw new APIError(body.error || `GPU 凭证重载失败 (${response.status})`, response.status, body);
      setConfigFeedback(`已重新读取后端凭证：${body.status || '已安排刷新'}`);
      await refresh();
    } catch (err) {
      setConfigFeedback(err instanceof Error ? err.message : 'GPU 凭证重载失败');
    } finally {
      setConfigReloading(false);
    }
  }, [refresh]);

  useEffect(() => {
    void refresh();
    if (pollMs <= 0) return;
    const interval = window.setInterval(() => {
      if (!document.hidden) void refresh();
    }, pollMs);
    return () => window.clearInterval(interval);
  }, [pollMs, refresh]);

  return { ...result, loading, error, refresh, refreshUpstream, upstreamRefreshing, refreshError, reloadConfiguration, configReloading, configFeedback };
};

export const useGPUPriceReference = () => {
  const [result, setResult] = useState<AdaptedGPUPriceReference>(emptyReference);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await getWithMeta<GPUPriceReferenceResponse>('/api/gpu-prices/reference');
      setResult(adaptGPUPriceReferenceResponse(response.data, response.meta));
    } catch (err) {
      const meta = err instanceof APIError ? err.meta || null : null;
      setResult(adaptGPUPriceReferenceResponse(null, meta));
      setError(err instanceof Error ? err.message : 'GPU 官方核验参考价请求失败');
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => { void refresh(); }, [refresh]);
  return { ...result, loading, error, refresh };
};

interface GPUHistoryFilters {
  provider: string;
  gpuModel: string;
  billingMode?: string;
  days: number;
}

export const useGPUPriceHistory = (filters: GPUHistoryFilters | null) => {
  const [result, setResult] = useState<AdaptedGPUPriceHistory>(emptyHistory);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    if (!filters?.provider || !filters.gpuModel) {
      setResult(emptyHistory());
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    const params = new URLSearchParams({
      provider: filters.provider,
      gpuModel: filters.gpuModel,
      days: String(filters.days),
    });
    if (filters.billingMode) params.set('billingMode', filters.billingMode);
    try {
      const response = await getWithMeta<GPUPriceHistoryResponse>(`/api/gpu-prices/history?${params.toString()}`);
      const adapted = adaptGPUPriceHistoryResponse(response.data, response.meta);
      setResult({
        ...adapted,
        points: filters.billingMode
          ? adapted.points.filter((point) => point.billingMode === filters.billingMode)
          : adapted.points,
      });
    } catch (err) {
      const meta = err instanceof APIError ? err.meta || null : null;
      setResult(adaptGPUPriceHistoryResponse(null, meta));
      setError(err instanceof Error ? err.message : 'GPU 历史价格请求失败');
    } finally {
      setLoading(false);
    }
  }, [filters?.billingMode, filters?.days, filters?.gpuModel, filters?.provider]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return { ...result, loading, error, refresh };
};
