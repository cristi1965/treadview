// 颜色映射工具

/**
 * 根据分数获取颜色 (0-100)
 */
export const getScoreColor = (score: number): string => {
  if (score >= 85) return '#22c55e'; // 深绿 - 过热
  if (score >= 70) return '#86efac'; // 浅绿 - 偏热
  if (score >= 50) return '#f97316'; // 橙色 - 合理
  if (score >= 30) return '#60a5fa'; // 浅蓝 - 偏冷
  return '#3b82f6'; // 深蓝 - 深价值
};

/**
 * 获取热力图颜色渐变
 */
export const getHeatmapColor = (value: number, min: number = 0, max: number = 100): string => {
  const normalizedValue = (value - min) / (max - min);
  
  if (normalizedValue >= 0.85) return '#ef4444'; // 红色 - 过热
  if (normalizedValue >= 0.70) return '#f97316'; // 橙色 - 偏热
  if (normalizedValue >= 0.50) return '#eab308'; // 黄色 - 合理
  if (normalizedValue >= 0.30) return '#3b82f6'; // 蓝色 - 偏冷
  return '#1e40af'; // 深蓝 - 深价值
};

/**
 * 党派颜色
 */
export const getPartyColor = (party: 'D' | 'R' | string): string => {
  if (party === 'D') return '#3b82f6'; // 民主党 - 蓝色
  if (party === 'R') return '#ef4444'; // 共和党 - 红色
  return '#9ca3af'; // 其他 - 灰色
};

/**
 * 涨跌颜色
 */
export const getChangeColor = (change: number): string => {
  if (change > 0) return '#22c55e'; // 绿色 - 上涨
  if (change < 0) return '#ef4444'; // 红色 - 下跌
  return '#9ca3af'; // 灰色 - 不变
};

/**
 * RGB 转 RGBA
 */
export const rgbToRgba = (rgb: string, alpha: number): string => {
  const match = rgb.match(/^#([0-9a-f]{6})$/i);
  if (!match) return rgb;
  
  const hex = match[1];
  const r = parseInt(hex.substring(0, 2), 16);
  const g = parseInt(hex.substring(2, 4), 16);
  const b = parseInt(hex.substring(4, 6), 16);
  
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
};

/**
 * 生成颜色渐变数组
 */
export const generateColorGradient = (
  startColor: string,
  endColor: string,
  steps: number
): string[] => {
  const start = hexToRgb(startColor);
  const end = hexToRgb(endColor);
  
  if (!start || !end) return [startColor, endColor];
  
  const colors: string[] = [];
  
  for (let i = 0; i < steps; i++) {
    const ratio = i / (steps - 1);
    const r = Math.round(start.r + (end.r - start.r) * ratio);
    const g = Math.round(start.g + (end.g - start.g) * ratio);
    const b = Math.round(start.b + (end.b - start.b) * ratio);
    colors.push(`rgb(${r}, ${g}, ${b})`);
  }
  
  return colors;
};

function hexToRgb(hex: string): { r: number; g: number; b: number } | null {
  const match = hex.match(/^#([0-9a-f]{6})$/i);
  if (!match) return null;
  
  const hexStr = match[1];
  return {
    r: parseInt(hexStr.substring(0, 2), 16),
    g: parseInt(hexStr.substring(2, 4), 16),
    b: parseInt(hexStr.substring(4, 6), 16)
  };
}
