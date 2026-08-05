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
    public data?: any
  ) {
    super(message);
    this.name = 'APIError';
  }
}

// 通用 fetch 包装器
async function fetchAPI<T>(
  endpoint: string,
  options: RequestInit = {}
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
    const response = await fetch(url, defaultOptions);
    
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

// GET 请求
export async function get<T>(endpoint: string): Promise<T> {
  return fetchAPI<T>(endpoint, { method: 'GET' });
}

// POST 请求
export async function post<T>(endpoint: string, data?: any): Promise<T> {
  return fetchAPI<T>(endpoint, {
    method: 'POST',
    body: data ? JSON.stringify(data) : undefined,
  });
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
