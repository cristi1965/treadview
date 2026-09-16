// API 基础配置：同域用相对路径；仅跨域开发时设置 VITE_API_BASE_URL
const rawBase = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? '';

function resolveApiBase(): string {
  if (!rawBase) return '';
  try {
    if (typeof window !== 'undefined') {
      const target = new URL(rawBase, window.location.origin);
      // Avoid localhost vs 127.0.0.1 CORS traps when page is served by Go on same port
      if (target.port === window.location.port || (!target.port && !window.location.port)) {
        const sameHost =
          target.hostname === window.location.hostname ||
          (['localhost', '127.0.0.1'].includes(target.hostname) &&
            ['localhost', '127.0.0.1'].includes(window.location.hostname));
        if (sameHost) return '';
      }
    }
  } catch {
    /* ignore */
  }
  return rawBase.replace(/\/$/, '');
}

export const API_BASE_URL = resolveApiBase();

export const ADMIN_TOKEN_STORAGE_KEY = 'stockgod_admin_token_session';
export const ADMIN_SESSION_STATE_EVENT = 'stockgod-admin-session-state';
let localSessionEstablished = false;

const publishAdminSessionState = (authorized: boolean) => {
  localSessionEstablished = authorized;
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent(ADMIN_SESSION_STATE_EVENT, { detail: { authorized } }));
  }
};

export const getAdminToken = () => {
  if (typeof window === 'undefined') return '';
  return window.sessionStorage.getItem(ADMIN_TOKEN_STORAGE_KEY)?.trim() || '';
};

export const setAdminToken = (token: string) => {
  if (typeof window === 'undefined') return;
  const value = token.trim();
  if (value) window.sessionStorage.setItem(ADMIN_TOKEN_STORAGE_KEY, value);
  else window.sessionStorage.removeItem(ADMIN_TOKEN_STORAGE_KEY);
};

export const clearAdminToken = () => {
  if (typeof window !== 'undefined') window.sessionStorage.removeItem(ADMIN_TOKEN_STORAGE_KEY);
};

export const withAdminAuth = (options: RequestInit = {}): RequestInit => {
  const token = getAdminToken();
  const headers = new Headers(options.headers);
  if (token) headers.set('Authorization', `Bearer ${token}`);
  return { ...options, headers };
};

let localSessionRecovery: Promise<boolean> | null = null;

export const establishLocalAdminSession = () => {
  if (localSessionRecovery) return localSessionRecovery;
  localSessionRecovery = fetch(apiUrl('/api/admin/session/local'), {
    method: 'POST',
    credentials: 'same-origin',
  })
    .then((response) => {
      publishAdminSessionState(response.ok);
      return response.ok;
    })
    .catch(() => false)
    .finally(() => { localSessionRecovery = null; });
  return localSessionRecovery;
};

export const verifyOrEstablishLocalAdminSession = async () => {
  // A same-origin POST both verifies the backend's local-session capability
  // and rotates the HttpOnly session without an expected 401 probe.
  return (await establishLocalAdminSession()) || Boolean(getAdminToken());
};

export const authorizedFetch = async (input: RequestInfo | URL, options: RequestInit = {}) => {
  const firstInput = input instanceof Request ? input.clone() : input;
  const retryInput = input instanceof Request ? input.clone() : input;
  const token = getAdminToken();
  if (!token && !localSessionEstablished) await establishLocalAdminSession();
  const response = await fetch(firstInput, { ...withAdminAuth(options), credentials: 'same-origin' });
  if (response.status === 403 && token) {
    const errorData = await response.clone().json().catch(() => ({}));
    if (errorData?.error === 'invalid admin token') {
      clearAdminToken();
      publishAdminSessionState(false);
      if (await establishLocalAdminSession()) {
        return fetch(retryInput, { ...withAdminAuth(options), credentials: 'same-origin' });
      }
      return response;
    }
  }
  if (response.status !== 401 || token) return response;
  publishAdminSessionState(false);
  if (!(await establishLocalAdminSession())) return response;
  return fetch(retryInput, { ...withAdminAuth(options), credentials: 'same-origin' });
};

const REQUEST_TIMEOUT_MS = 5_000;

async function authorizedFetchWithTimeout(input: RequestInfo | URL, options: RequestInit = {}, timeoutMs = REQUEST_TIMEOUT_MS) {
  const controller = new AbortController();
  const upstreamSignal = options.signal;
  const abortFromUpstream = () => controller.abort(upstreamSignal?.reason);
  if (upstreamSignal?.aborted) abortFromUpstream();
  else upstreamSignal?.addEventListener('abort', abortFromUpstream, { once: true });
  const timer = window.setTimeout(() => controller.abort(new Error('请求超时，请重试')), timeoutMs);

  try {
    return await authorizedFetch(input, { ...options, signal: controller.signal });
  } finally {
    window.clearTimeout(timer);
    upstreamSignal?.removeEventListener('abort', abortFromUpstream);
  }
}

export const apiUrl = (endpoint: string) => {
  if (!API_BASE_URL) return endpoint.startsWith('/') ? endpoint : `/${endpoint}`;
  return `${API_BASE_URL}${endpoint.startsWith('/') ? endpoint : `/${endpoint}`}`;
};

export const wsUrl = (endpoint = '/ws') => {
  if (!API_BASE_URL) {
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${proto}//${window.location.host}${endpoint}`;
  }
  const base = new URL(API_BASE_URL);
  base.protocol = base.protocol === 'https:' ? 'wss:' : 'ws:';
  base.pathname = endpoint;
  base.search = '';
  base.hash = '';
  return base.toString();
};

// API 错误类
export class APIError extends Error {
  constructor(
    message: string,
    public status?: number,
    public data?: any,
    public meta?: APIResponseMeta,
  ) {
    super(message);
    this.name = 'APIError';
  }
}

export interface APIResponseMeta {
  source: string;
  dataTime: string;
  refreshedAt: string;
  stale: boolean;
  staleReason: string;
  refreshable: boolean;
  partialErrors: string[];
}

export interface APIResult<T> {
  data: T;
  meta: APIResponseMeta;
}

function responseMeta(response: Response): APIResponseMeta {
  const partialErrors = response.headers.get('X-Data-Partial-Errors') || '';
  return {
    source: response.headers.get('X-Data-Source') || '',
    dataTime: response.headers.get('X-Data-Time') || '',
    refreshedAt: response.headers.get('X-Data-Refreshed-At') || '',
    stale: response.headers.get('X-Data-Stale') === 'true',
    staleReason: response.headers.get('X-Data-Stale-Reason') || '',
    refreshable: response.headers.get('X-Data-Refreshable') === 'true',
    partialErrors: partialErrors ? partialErrors.split(',').map((item) => item.trim()).filter(Boolean) : [],
  };
}

// 通用 fetch 包装器
async function fetchAPI<T>(
  endpoint: string,
  options: RequestInit = {},
  timeoutMs = REQUEST_TIMEOUT_MS,
): Promise<T> {
  const url = apiUrl(endpoint);
  
  const defaultOptions: RequestInit = {
    cache: 'no-store',
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
    ...options,
  };

  try {
    const response = await authorizedFetchWithTimeout(url, defaultOptions, timeoutMs);
    
    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}));
      throw new APIError(
        errorData.error || `API Error: ${response.statusText}`,
        response.status,
        errorData
      );
    }

    return await response.json();
  } catch (error) {
    if (error instanceof APIError) {
      throw error;
    }
    throw new APIError(
      error instanceof Error ? error.message : 'Network error',
      undefined,
      error
    );
  }
}

async function fetchAPIWithMeta<T>(endpoint: string, options: RequestInit = {}): Promise<APIResult<T>> {
  const url = apiUrl(endpoint);
  const response = await authorizedFetchWithTimeout(url, {
    cache: 'no-store',
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });

  if (!response.ok) {
    const errorData = await response.json().catch(() => ({}));
    throw new APIError(errorData.error || `API Error: ${response.statusText}`, response.status, errorData, responseMeta(response));
  }

  return { data: await response.json() as T, meta: responseMeta(response) };
}

// GET 请求（附带时间戳与防缓存 Request Headers，确保拿到的永远是最新数据）
export async function get<T>(endpoint: string): Promise<T> {
  const sep = endpoint.includes('?') ? '&' : '?';
  const timestamped = `${endpoint}${sep}_t=${Date.now()}`;
  return fetchAPI<T>(timestamped, {
    method: 'GET',
    headers: {
      'Cache-Control': 'no-cache, no-store, must-revalidate, max-age=0',
      'Pragma': 'no-cache',
      'Expires': '0',
    },
  });
}

export async function getWithMeta<T>(endpoint: string): Promise<APIResult<T>> {
  const sep = endpoint.includes('?') ? '&' : '?';
  const timestamped = `${endpoint}${sep}_t=${Date.now()}`;
  return fetchAPIWithMeta<T>(timestamped, {
    method: 'GET',
    headers: {
      'Cache-Control': 'no-cache, no-store, must-revalidate, max-age=0',
      'Pragma': 'no-cache',
      'Expires': '0',
    },
  });
}

// POST 请求
export async function post<T>(endpoint: string, data?: any, options: { timeoutMs?: number } = {}): Promise<T> {
  return fetchAPI<T>(endpoint, {
    method: 'POST',
    body: data ? JSON.stringify(data) : undefined,
  }, options.timeoutMs);
}

// PUT 请求
export async function put<T>(endpoint: string, data?: any): Promise<T> {
  return fetchAPI<T>(endpoint, {
    method: 'PUT',
    body: data ? JSON.stringify(data) : undefined,
  });
}

// DELETE 请求
export async function del<T>(endpoint: string): Promise<T> {
  return fetchAPI<T>(endpoint, { method: 'DELETE' });
}
