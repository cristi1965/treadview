import React from 'react';
import { AlertTriangle, CheckCircle2, Clock3, RefreshCw, WifiOff } from 'lucide-react';

export type DataState = 'loading' | 'live' | 'stale' | 'unavailable' | 'error';

interface DataStatusProps {
  state: DataState;
  label?: string;
  dataTime?: string;
  source?: string;
  message?: string;
  onRetry?: () => void;
  compact?: boolean;
}

const LABELS: Record<DataState, string> = {
  loading: '加载中', live: '实时', stale: '过期快照', unavailable: '暂无数据', error: '加载失败',
};

export const DataStatus: React.FC<DataStatusProps> = ({ state, label, dataTime, source, message, onRetry, compact = false }) => {
  const Icon = state === 'live' ? CheckCircle2 : state === 'stale' ? Clock3 : state === 'loading' ? RefreshCw : state === 'error' ? AlertTriangle : WifiOff;
  const tone = state === 'live' ? 'text-up' : state === 'stale' ? 'text-accent' : state === 'loading' ? 'text-muted' : 'text-down';
  const parsedTime = dataTime && dataTime !== 'unknown' ? Date.parse(dataTime) : Number.NaN;
  const time = dataTime === 'unknown' ? '未知' : Number.isFinite(parsedTime) ? new Date(parsedTime).toLocaleString() : '';
  return (
    <div className={`flex flex-wrap items-center gap-x-2 gap-y-1 ${compact ? 'text-[10px]' : 'text-xs'}`}>
      <span className={`inline-flex items-center gap-1 font-medium ${tone}`}>
        <Icon className={state === 'loading' ? 'h-3 w-3 animate-spin' : 'h-3 w-3'} />
        {label || LABELS[state]}
      </span>
      {time && <span className="font-mono text-faint">数据时间 {time}</span>}
      {source && <span className="text-faint">来源 {source}</span>}
      {message && <span className="text-muted">{message}</span>}
      {onRetry && state !== 'loading' && (
        <button type="button" onClick={onRetry} className="inline-flex items-center gap-1 text-accent hover:text-ink">
          <RefreshCw className="h-3 w-3" /> 重试
        </button>
      )}
    </div>
  );
};
