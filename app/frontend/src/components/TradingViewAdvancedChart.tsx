import React, { useEffect, useRef, useState } from 'react';
import { useUiStore } from '../stores/uiStore';
import { BarChart3, LineChart, Layers, ExternalLink, LoaderCircle, WifiOff } from 'lucide-react';

interface TradingViewAdvancedChartProps {
  symbol: string;
  price?: number;
  height?: number | string;
  defaultInterval?: '1' | '5' | '15' | '60' | 'D' | 'W';
}

const formatTradingViewSymbol = (rawSymbol: string): string => {
  const sym = rawSymbol.trim().toUpperCase();

  // Already prefixed
  if (sym.includes(':')) return sym;

  // Suffix matching
  if (sym.endsWith('.HK')) {
    const code = sym.replace('.HK', '').replace(/^0+/, '') || '1';
    return `HKEX:${code}`;
  }
  if (sym.endsWith('.SS') || sym.endsWith('.SH')) {
    return `SSE:${sym.replace(/\.(SS|SH)$/, '')}`;
  }
  if (sym.endsWith('.SZ')) return `SZSE:${sym.replace('.SZ', '')}`;
  if (sym.endsWith('.BJ')) return `BSE:${sym.replace('.BJ', '')}`;
  if (sym.endsWith('.TW')) return `TWSE:${sym.replace('.TW', '')}`;
  if (sym.endsWith('.T')) return `TSE:${sym.replace('.T', '')}`;
  if (sym.endsWith('.L')) return `LSE:${sym.replace('.L', '')}`;
  if (sym.endsWith('.DE')) return `XETR:${sym.replace('.DE', '')}`;
  if (sym.endsWith('-USD') || sym.endsWith('USDT') || sym === 'BTC' || sym === 'ETH' || sym === 'SOL') {
    const base = sym.replace('-USD', '').replace('USDT', '');
    return `BINANCE:${base}USDT`;
  }

  // Pure numeric codes
  if (/^\d+$/.test(sym)) {
    if (sym.length === 6) {
      if (sym.startsWith('6') || sym.startsWith('5') || sym.startsWith('688')) return `SSE:${sym}`;
      if (sym.startsWith('8') || sym.startsWith('4') || sym.startsWith('92')) return `BSE:${sym}`;
      return `SZSE:${sym}`;
    }
    // HKEX: 01810 -> HKEX:1810, 00700 -> HKEX:700, 9988 -> HKEX:9988
    if (sym.length <= 5) {
      const code = sym.replace(/^0+/, '') || '1';
      return `HKEX:${code}`;
    }
  }

  // NYSE Known list
  const nyseList = [
    'PLTR', 'BABA', 'NIO', 'XPEV', 'LI', 'TSM', 'RBLX', 'SNOW', 'U', 'NET',
    'DIS', 'JPM', 'V', 'MA', 'UNH', 'HD', 'PG', 'BRK.A', 'BRK.B', 'KO', 'PFE',
    'LLY', 'NKE', 'WMT', 'BAC', 'CRM', 'ORCL', 'IBM', 'SPOT'
  ];
  if (nyseList.includes(sym)) return `NYSE:${sym}`;

  // AMEX ETFs
  const amexList = ['SPY', 'IVV', 'SOXL', 'SOXS', 'TQQQ', 'SQQQ', 'SPXL', 'SPXS', 'TMF', 'TMV', 'GLD', 'SLV', 'UVXY'];
  if (amexList.includes(sym)) return `AMEX:${sym}`;

  // Default to NASDAQ for US tech
  return `NASDAQ:${sym}`;
};

export const TradingViewAdvancedChart: React.FC<TradingViewAdvancedChartProps> = ({
  symbol,
  height = 520,
  defaultInterval = 'D',
}) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const theme = useUiStore((s) => s.theme);
  const language = useUiStore((s) => s.language);
  const [chartType, setChartType] = useState<'candles' | 'area'>('candles');
  const [interval, setInterval] = useState<string>(defaultInterval);
  const [embedState, setEmbedState] = useState<'loading' | 'ready' | 'unavailable'>('loading');
  const [activeStudies, setActiveStudies] = useState<string[]>([
    'STD;SMA',
    'STD;Bollinger_Bands',
    'STD;VWAP',
    'STD;RSI',
    'STD;MACD',
  ]);

  const tvSymbol = formatTradingViewSymbol(symbol);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    setEmbedState('loading');

    // TradingView's external script can finish asynchronously after React
    // starts the next render. Keep the old host alive briefly so its callback
    // never targets a node removed during a timeframe/study switch.
    const widgetDiv = document.createElement('div');
    widgetDiv.className = 'tradingview-widget-container__widget';
    widgetDiv.style.height = '100%';
    widgetDiv.style.width = '100%';
    container.appendChild(widgetDiv);

    const script = document.createElement('script');
    script.src = 'https://s3.tradingview.com/external-embedding/embed-widget-advanced-chart.js';
    script.type = 'text/javascript';
    script.async = true;
    script.innerHTML = JSON.stringify({
      autosize: true,
      symbol: tvSymbol,
      interval: interval,
      timezone: 'exchange',
      theme: theme === 'light' ? 'light' : 'dark',
      style: chartType === 'candles' ? '1' : '3',
      locale: language === 'zh' ? 'zh_CN' : 'en',
      enable_publishing: false,
      allow_symbol_change: true,
      calendar: false,
      studies: activeStudies,
      support_host: 'https://www.tradingview.com'
    });

    let active = true;
    let iframeLoaded = false;
    let observedFrame: HTMLIFrameElement | null = null;
    const markReady = () => {
      iframeLoaded = true;
      if (active) setEmbedState('ready');
    };
    const markUnavailable = () => {
      if (active) setEmbedState('unavailable');
    };
    script.addEventListener('error', markUnavailable);

    const observeIframe = () => {
      const frame = widgetDiv.querySelector('iframe');
      if (!frame || frame === observedFrame) return;
      observedFrame?.removeEventListener('load', markReady);
      observedFrame?.removeEventListener('error', markUnavailable);
      observedFrame = frame;
      frame.addEventListener('load', markReady, { once: true });
      frame.addEventListener('error', markUnavailable, { once: true });
    };
    const iframeObserver = new MutationObserver(observeIframe);
    iframeObserver.observe(widgetDiv, { childList: true, subtree: true });

    const timeout = window.setTimeout(() => {
      if (active && !iframeLoaded) setEmbedState('unavailable');
    }, 8000);

    widgetDiv.appendChild(script);

    return () => {
      active = false;
      window.clearTimeout(timeout);
      iframeObserver.disconnect();
      observedFrame?.removeEventListener('load', markReady);
      observedFrame?.removeEventListener('error', markUnavailable);
      script.removeEventListener('error', markUnavailable);
      window.setTimeout(() => widgetDiv.remove(), 1000);
    };
  }, [tvSymbol, theme, language, chartType, interval, activeStudies]);

  const toggleStudy = (studyKey: string) => {
    setActiveStudies((prev) =>
      prev.includes(studyKey) ? prev.filter((s) => s !== studyKey) : [...prev, studyKey]
    );
  };

  return (
    <div className="rounded-xl border border-line bg-surface overflow-hidden shadow-lg space-y-0">
      {/* Header controls bar */}
      <div className="flex flex-col gap-2.5 border-b border-line bg-surface-2 p-3 text-xs sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1.5 font-bold text-ink text-sm">
            <BarChart3 size={16} className="text-accent" />
            <span>TradingView 图表 & 指标</span>
          </div>
          <span className="rounded bg-base px-2 py-0.5 font-mono text-[11px] text-muted border border-line font-semibold">
            {tvSymbol}
          </span>
        </div>

        {/* Interval and Style Switchers */}
        <div className="flex flex-wrap items-center gap-2">
          {/* Timeframe intervals */}
          <div className="flex items-center rounded-lg border border-line bg-base p-0.5">
            {[
              { label: '分时', val: '1' },
              { label: '5分', val: '5' },
              { label: '15分', val: '15' },
              { label: '1小时', val: '60' },
              { label: '日K', val: 'D' },
              { label: '周K', val: 'W' },
            ].map((item) => (
              <button
                key={item.val}
                type="button"
                onClick={() => setInterval(item.val)}
                className={`rounded px-2 py-1 text-[11px] font-medium transition ${
                  interval === item.val
                    ? 'bg-accent/20 text-accent font-semibold'
                    : 'text-muted hover:text-ink'
                }`}
              >
                {item.label}
              </button>
            ))}
          </div>

          {/* Chart Style Switcher */}
          <div className="flex items-center rounded-lg border border-line bg-base p-0.5">
            <button
              type="button"
              onClick={() => setChartType('candles')}
              className={`flex items-center gap-1 rounded px-2 py-1 text-[11px] transition ${
                chartType === 'candles' ? 'bg-accent/20 text-accent font-semibold' : 'text-muted hover:text-ink'
              }`}
              title="专业蜡烛图"
            >
              <BarChart3 size={12} /> 蜡烛K线
            </button>
            <button
              type="button"
              onClick={() => setChartType('area')}
              className={`flex items-center gap-1 rounded px-2 py-1 text-[11px] transition ${
                chartType === 'area' ? 'bg-accent/20 text-accent font-semibold' : 'text-muted hover:text-ink'
              }`}
              title="分时面积走势图"
            >
              <LineChart size={12} /> 面积图
            </button>
          </div>

          {/* Direct Link to TradingView Official */}
          <a
            href={`https://www.tradingview.com/symbols/${tvSymbol.replace(':', '-')}/`}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1 rounded-lg border border-line bg-base px-2 py-1 text-[11px] text-muted hover:border-accent/40 hover:text-accent transition"
            title="在 TradingView 官方打开，使用个人账户画线与自定义 Pine Script 策略"
          >
            <ExternalLink size={12} /> 官网全功能 ↗
          </a>
        </div>
      </div>

      {/* Indicator Presets Bar */}
      <div className="flex flex-wrap items-center gap-1.5 border-b border-line bg-base/60 px-3 py-2 text-[11px]">
        <span className="font-semibold text-muted flex items-center gap-1 mr-1">
          <Layers size={13} className="text-accent" /> 叠加指标:
        </span>
        {[
          { key: 'STD;SMA', label: '均线 (SMA)' },
          { key: 'STD;Bollinger_Bands', label: '布林带 (BOLL)' },
          { key: 'STD;VWAP', label: '均价 (VWAP)' },
          { key: 'STD;RSI', label: '强弱指标 (RSI)' },
          { key: 'STD;MACD', label: '异同均线 (MACD)' },
        ].map((ind) => {
          const isActive = activeStudies.includes(ind.key);
          return (
            <button
              key={ind.key}
              type="button"
              onClick={() => toggleStudy(ind.key)}
              className={`rounded px-2 py-0.5 font-medium border transition ${
                isActive
                  ? 'bg-accent/15 border-accent/40 text-accent font-semibold'
                  : 'bg-surface border-line text-faint hover:text-ink'
              }`}
            >
              {isActive ? '✓ ' : '+ '}
              {ind.label}
            </button>
          );
        })}
      </div>

      {/* Embedded Chart Container */}
      <div
        ref={containerRef}
        style={{ height }}
        className="relative w-full bg-[#131722]"
      >
        {embedState !== 'ready' && (
          <div className="absolute inset-0 z-10 flex items-center justify-center bg-[#131722] p-6 text-center">
            {embedState === 'loading' ? (
              <div className="flex items-center gap-2 text-sm text-slate-300">
                <LoaderCircle size={18} className="animate-spin" />
                正在连接 TradingView
              </div>
            ) : (
              <div className="max-w-sm space-y-3">
                <WifiOff size={24} className="mx-auto text-slate-400" />
                <p className="text-sm font-medium text-slate-200">TradingView 暂时无法加载</p>
                <a
                  href={`https://www.tradingview.com/symbols/${tvSymbol.replace(':', '-')}/`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 text-xs font-medium text-sky-400 hover:text-sky-300"
                >
                  在官网查看 <ExternalLink size={12} />
                </a>
              </div>
            )}
          </div>
        )}
      </div>

      <div className="border-t border-line bg-base px-3 py-2 text-[11px] text-faint">
        TradingView 外部展示数据，仅供人工查看，不参与评分、提醒或 Paper 订单计算。
      </div>

    </div>
  );
};
