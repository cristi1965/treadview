import React, { useState } from 'react';
import { X, Copy, Check, Sparkles, Code2, Play, ExternalLink } from 'lucide-react';
import { useDialogFocus } from '../hooks/useDialogFocus';

interface PineScriptModalProps {
  isOpen: boolean;
  onClose: () => void;
}

const PINE_SCRIPT_CODE = `//@version=5
indicator("我不是神 · 多指标观察模板", overlay=true)

// ==========================================
// 1. 四重指数移动平均线 (EMA 20 / 50 / 100 / 200)
// ==========================================
ema20  = ta.ema(close, 20)
ema50  = ta.ema(close, 50)
ema100 = ta.ema(close, 100)
ema200 = ta.ema(close, 200)

plot(ema20,  "EMA 20 (短线生命线)",   color=color.rgb(250, 204, 21), linewidth=1)
plot(ema50,  "EMA 50 (中线分水岭)",   color=color.rgb(56, 189, 248), linewidth=2)
plot(ema100, "EMA 100 (机构建仓线)",  color=color.rgb(168, 85, 247), linewidth=2)
plot(ema200, "EMA 200 (牛熊分界线)",  color=color.rgb(239, 68, 68),  linewidth=3)

// ==========================================
// 2. 布林带 (Bollinger Bands 20, 2.0)
// ==========================================
[bbMid, bbUpper, bbLower] = ta.bb(close, 20, 2.0)
pUpper = plot(bbUpper, "布林上轨 (阻力/超买)", color=color.new(color.teal, 50))
pLower = plot(bbLower, "布林下轨 (支撑/超卖)", color=color.new(color.teal, 50))
fill(pUpper, pLower, color=color.new(color.teal, 93), title="布林带通道")

// ==========================================
// 3. 成交量加权平均价 (Session VWAP)
// ==========================================
vwapValue = ta.vwap(hlc3)
plot(vwapValue, "VWAP (大单筹码中枢)", color=color.rgb(249, 115, 22), linewidth=2, style=plot.style_circles)

// ==========================================
// 4. 动态支撑与阻力枢轴点 (Pivot Support & Resistance)
// ==========================================
hHigh = ta.highest(high, 20)
lLow  = ta.lowest(low, 20)
plot(hHigh, "20周期 突破阻力位", color=color.new(color.red, 30), style=plot.style_linebr, linewidth=1)
plot(lLow,  "20周期 支撑观察位", color=color.new(color.green, 30), style=plot.style_linebr, linewidth=1)

// ==========================================
// 5. RSI 动量极值预警标记 (RSI 14)
// ==========================================
rsiVal = ta.rsi(close, 14)
isOverbought = rsiVal >= 75
isOversold   = rsiVal <= 25

plotshape(isOversold,   title="RSI 极端超卖", style=shape.triangleup,   location=location.belowbar, color=color.green, size=size.small, text="RSI低位")
plotshape(isOverbought, title="RSI 极端超买", style=shape.triangledown, location=location.abovebar, color=color.red,   size=size.small, text="RSI高位")

// ==========================================
// 6. MACD 金叉/死叉多空信号
// ==========================================
[macdLine, signalLine, _] = ta.macd(close, 12, 26, 9)
macdBullCross = ta.crossover(macdLine, signalLine) and rsiVal < 65
plotshape(macdBullCross, title="MACD 多头共振", style=shape.labelup, location=location.belowbar, color=color.rgb(16, 185, 129), text="MACD共振", textcolor=color.white, size=size.tiny)
`;

export const PineScriptModal: React.FC<PineScriptModalProps> = ({ isOpen, onClose }) => {
  const [copied, setCopied] = useState(false);
  const dialogRef = useDialogFocus<HTMLDivElement>(isOpen, onClose);

  if (!isOpen) return null;

  const handleCopy = () => {
    navigator.clipboard.writeText(PINE_SCRIPT_CODE);
    setCopied(true);
    setTimeout(() => setCopied(false), 2500);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-black/80 p-0 backdrop-blur-sm animate-in fade-in duration-200 sm:items-center sm:p-4">
      <div ref={dialogRef} role="dialog" aria-modal="true" aria-labelledby="pine-script-title" tabIndex={-1} className="relative flex max-h-[100dvh] w-full max-w-2xl flex-col overflow-hidden border border-line bg-surface shadow-2xl sm:max-h-[90dvh] sm:rounded-lg">
        {/* Header */}
        <div className="flex items-start justify-between gap-3 border-b border-line bg-surface-2 px-4 py-3 sm:items-center sm:px-5 sm:py-4">
          <div className="flex items-center gap-2.5">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-emerald-500 to-teal-700 text-white shadow-lg">
              <Code2 size={20} />
            </div>
            <div>
              <h2 id="pine-script-title" className="flex flex-wrap items-center gap-2 text-sm font-bold text-ink sm:text-base">
                TradingView 多指标观察模板
                <span className="rounded bg-emerald-500/20 px-2 py-0.5 text-[11px] font-semibold text-emerald-400 border border-emerald-500/30">
                  自定义脚本
                </span>
              </h2>
              <p className="text-xs text-muted">
                汇总 EMA、BOLL、VWAP、支撑阻力、RSI 与 MACD 的图表观察信息，不构成交易信号
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            aria-label="关闭 Pine Script 弹窗"
            className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg text-muted transition hover:bg-surface-3 hover:text-ink sm:h-8 sm:w-8"
          >
            <X size={18} />
          </button>
        </div>

        {/* 3-Step Guide */}
        <div className="grid grid-cols-1 gap-2 border-b border-line bg-base/50 p-3 text-xs sm:grid-cols-3">
          <div className="flex items-center gap-2 rounded-lg bg-surface p-2 border border-line">
            <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-accent/20 font-mono text-[11px] font-bold text-accent">1</span>
            <span className="text-muted">点击下方按钮 <strong>一键复制</strong> 源码</span>
          </div>
          <div className="flex items-center gap-2 rounded-lg bg-surface p-2 border border-line">
            <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-accent/20 font-mono text-[11px] font-bold text-accent">2</span>
            <span className="text-muted">打开 TradingView 下方 <strong>Pine Editor</strong></span>
          </div>
          <div className="flex items-center gap-2 rounded-lg bg-surface p-2 border border-line">
            <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-accent/20 font-mono text-[11px] font-bold text-accent">3</span>
            <span className="text-muted">粘贴并点击 <strong>添加到图表</strong> 即可</span>
          </div>
        </div>

        {/* Code Viewer */}
        <div className="relative flex-1 overflow-y-auto bg-[#0f141c] p-4 font-mono text-[12px] text-emerald-300 leading-relaxed select-all">
          <pre>{PINE_SCRIPT_CODE}</pre>
        </div>

        {/* Footer Actions */}
        <div className="flex flex-wrap items-center justify-between gap-2 border-t border-line bg-surface-2 px-4 py-3 sm:px-5 sm:py-3.5">
          <a
            href="https://www.tradingview.com/chart/"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1.5 text-xs text-muted hover:text-accent transition"
          >
            <ExternalLink size={13} /> 前往 TradingView 图表页 ↗
          </a>

          <div className="flex items-center gap-3">
            <button
              onClick={onClose}
              className="min-h-11 rounded-lg border border-line bg-surface px-4 py-2 text-xs font-medium text-muted transition hover:text-ink sm:min-h-9"
            >
              关闭
            </button>
            <button
              onClick={handleCopy}
              className="flex min-h-11 items-center gap-1.5 rounded-lg bg-accent px-5 py-2 text-xs font-bold text-black shadow-lg transition hover:brightness-110 active:scale-95 sm:min-h-9"
            >
              {copied ? <Check size={15} /> : <Copy size={15} />}
              {copied ? '已复制到剪贴板！' : '一键复制代码'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
