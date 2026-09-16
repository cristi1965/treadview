// 价格格式化（支持美股、港股、A股、日股、台股、英股、欧股及加密货币）
export const detectCurrency = (sym?: string): string => {
  if (!sym) return '$';
  const s = sym.trim().toUpperCase();
  if (s.endsWith('.HK') || (/^\d{1,5}$/.test(s) && !/^\d{6}$/.test(s)) || s.startsWith('HKEX:')) return 'HK$';
  if (s.endsWith('.SS') || s.endsWith('.SZ') || s.endsWith('.BJ') || /^\d{6}$/.test(s) || s.startsWith('SSE:') || s.startsWith('SZSE:')) return '¥';
  if (s.endsWith('.TW') || s.startsWith('TWSE:')) return 'NT$';
  if (s.endsWith('.T') || s.startsWith('TSE:')) return 'JP¥';
  if (s.endsWith('.L') || s.startsWith('LSE:')) return '£';
  if (s.endsWith('.DE') || s.startsWith('XETR:')) return '€';
  return '$';
};

export const formatPrice = (price: number, sym?: string): string => {
  const cur = detectCurrency(sym);
  return `${cur}${price.toFixed(2)}`;
};

// 百分比格式化
export const formatPercent = (percent: number, showSign: boolean = true): string => {
  const sign = showSign && percent >= 0 ? '+' : '';
  return `${sign}${percent.toFixed(2)}%`;
};

// 市值/AUM 格式化
export const formatAUM = (aum: number, sym?: string): string => {
  const cur = detectCurrency(sym);
  if (aum >= 1e12) return `${cur}${(aum / 1e12).toFixed(1)}T`;
  if (aum >= 1e9) return `${cur}${Math.round(aum / 1e9)}B`;
  if (aum >= 1e6) return `${cur}${Math.round(aum / 1e6)}M`;
  if (aum >= 1e3) return `${cur}${Math.round(aum / 1e3)}K`;
  return `${cur}${Math.round(aum)}`;
};

// 数字简写格式化
export const formatNumber = (num: number): string => {
  if (num >= 1e9) return `${(num / 1e9).toFixed(1)}B`;
  if (num >= 1e6) return `${(num / 1e6).toFixed(1)}M`;
  if (num >= 1e3) return `${(num / 1e3).toFixed(1)}K`;
  return num.toString();
};

// 涨跌颜色类名
export const getChangeColorClass = (change: number): string => {
  if (change > 0) return 'text-up';
  if (change < 0) return 'text-down';
  return 'text-muted';
};

// 日期格式化
export const formatDate = (date: Date | string): string => {
  const d = typeof date === 'string' ? new Date(date) : date;
  return d.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit'
  });
};

// 时间格式化
export const formatDateTime = (date: Date | string): string => {
  const d = typeof date === 'string' ? new Date(date) : date;
  return d.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  });
};
