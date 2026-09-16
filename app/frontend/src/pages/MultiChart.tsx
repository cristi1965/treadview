import React, { useState } from 'react';
import { StockGodShell } from '../components/layout/StockGodShell';
import { TradingViewAdvancedChart } from '../components/TradingViewAdvancedChart';
import { PineScriptModal } from '../components/PineScriptModal';
import { Grid, LayoutGrid, Maximize2, Code2, Sparkles, Plus, ArrowRight } from 'lucide-react';
import { StockLogo } from '../components/StockLogo';
import { useDialogFocus } from '../hooks/useDialogFocus';

type LayoutMode = '1x1' | '2x1' | '2x2';

interface ChartSlot {
  id: string;
  symbol: string;
  interval: '1' | '5' | '15' | '60' | 'D' | 'W';
}

const PRESET_GROUPS = [
  {
    name: '👑 科技七巨头 M7',
    symbols: ['NVDA', 'TSLA', 'AAPL', 'MSFT'],
  },
  {
    name: '⚡ 美股前日剧烈波动',
    symbols: ['MU', 'AMD', 'ARM', 'IREN'],
  },
  {
    name: '🇨🇳 A股算力四剑客',
    symbols: ['300750', '300308', '688256', '601127'],
  },
  {
    name: '🎯 大盘对冲与核心',
    symbols: ['SPY', 'QQQ', 'NVDA', 'PLTR'],
  },
  {
    name: '高波动主题人工清单',
    symbols: ['PLTR', 'MSTR', 'CRWV', 'NBIS'],
  },
];

export const MultiChart: React.FC = () => {
  const [layout, setLayout] = useState<LayoutMode>('2x2');
  const [slots, setSlots] = useState<ChartSlot[]>([
    { id: '1', symbol: 'NVDA', interval: '5' },
    { id: '2', symbol: 'TSLA', interval: '5' },
    { id: '3', symbol: 'PLTR', interval: '15' },
    { id: '4', symbol: '300750', interval: 'D' },
  ]);

  const [activeSlotModal, setActiveSlotModal] = useState<number | null>(null);
  const [tempSymbol, setTempSymbol] = useState('');
  const [isPineModalOpen, setIsPineModalOpen] = useState(false);
  const closeSymbolModal = () => setActiveSlotModal(null);
  const symbolDialogRef = useDialogFocus<HTMLDivElement>(activeSlotModal !== null, closeSymbolModal);

  const updateSlotSymbol = (index: number, newSymbol: string) => {
    if (!newSymbol.trim()) return;
    const clean = newSymbol.trim().toUpperCase();
    setSlots((prev) => {
      const next = [...prev];
      if (next[index]) {
        next[index] = { ...next[index], symbol: clean };
      }
      return next;
    });
    setActiveSlotModal(null);
    setTempSymbol('');
  };

  const applyPreset = (symbols: string[]) => {
    setSlots([
      { id: '1', symbol: symbols[0] || 'NVDA', interval: '5' },
      { id: '2', symbol: symbols[1] || 'TSLA', interval: '5' },
      { id: '3', symbol: symbols[2] || 'AAPL', interval: '15' },
      { id: '4', symbol: symbols[3] || 'MSFT', interval: 'D' },
    ]);
  };

  const visibleSlots = layout === '1x1' ? [slots[0]] : layout === '2x1' ? slots.slice(0, 2) : slots;

  return (
    <StockGodShell title="多图行情观察工作台">
      <div className="space-y-4">
        {/* Top Control Bar */}
        <div className="flex flex-col gap-3 rounded-2xl border border-line bg-surface p-4 shadow-sm lg:flex-row lg:items-center lg:justify-between">
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-xl font-bold tracking-tight text-ink flex items-center gap-2">
                <LayoutGrid size={20} className="text-accent" />
                多图行情观察工作台
              </h1>
              <span className="rounded bg-accent/15 px-2 py-0.5 text-xs font-semibold text-accent border border-accent/25">
                1~4 图布局
              </span>
            </div>
            <p className="mt-1 text-xs text-muted">
              同屏观察 1~4 只美股/A股与多周期图表；第三方图表的来源、延迟和交易时段以图内标记为准。
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2.5">
            {/* Layout Toggles */}
            <div className="flex items-center rounded-lg border border-line bg-base p-1">
              <button
                type="button"
                onClick={() => setLayout('1x1')}
                className={`rounded px-2.5 py-1 text-xs font-semibold transition ${
                  layout === '1x1' ? 'bg-accent text-black shadow' : 'text-muted hover:text-ink'
                }`}
                title="单图大屏沉浸"
              >
                单图 1x1
              </button>
              <button
                type="button"
                onClick={() => setLayout('2x1')}
                className={`rounded px-2.5 py-1 text-xs font-semibold transition ${
                  layout === '2x1' ? 'bg-accent text-black shadow' : 'text-muted hover:text-ink'
                }`}
                title="双图对比"
              >
                双图 2x1
              </button>
              <button
                type="button"
                onClick={() => setLayout('2x2')}
                className={`rounded px-2.5 py-1 text-xs font-semibold transition ${
                  layout === '2x2' ? 'bg-accent text-black shadow' : 'text-muted hover:text-ink'
                }`}
                title="四图矩阵盯盘"
              >
                四图 2x2
              </button>
            </div>

            {/* Pine Script 6-in-1 Button */}
            <button
              type="button"
              onClick={() => setIsPineModalOpen(true)}
              className="flex items-center gap-1.5 rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-3 py-1.5 text-xs font-bold text-emerald-400 hover:bg-emerald-500/20 active:scale-95 transition"
            >
              <Code2 size={14} /> 多指标观察模板
            </button>
          </div>
        </div>

        {/* Preset Group Chips */}
        <div className="flex items-center gap-2 overflow-x-auto pb-1 text-xs">
          <span className="text-muted font-medium shrink-0">盯盘预设:</span>
          {PRESET_GROUPS.map((group) => (
            <button
              key={group.name}
              type="button"
              onClick={() => applyPreset(group.symbols)}
              className="shrink-0 rounded-lg border border-line bg-surface px-2.5 py-1 text-muted hover:border-accent/40 hover:text-ink transition font-medium"
            >
              {group.name}
            </button>
          ))}
        </div>

        {/* Dynamic Grid Layout */}
        <div
          className={`grid gap-4 ${
            layout === '1x1'
              ? 'grid-cols-1'
              : layout === '2x1'
              ? 'grid-cols-1 lg:grid-cols-2'
              : 'grid-cols-1 lg:grid-cols-2'
          }`}
        >
          {visibleSlots.map((slot, index) => (
            <div key={slot.id} className="relative rounded-xl border border-line bg-surface overflow-hidden shadow">
              {/* Slot Quick Switcher Header */}
              <div className="flex items-center justify-between border-b border-line bg-surface-2 px-3 py-1.5 text-xs">
                <div className="flex items-center gap-2">
                  <StockLogo symbol={slot.symbol} size={20} />
                  <span className="font-mono font-bold text-ink">{slot.symbol}</span>
                  <button
                    type="button"
                    onClick={() => {
                      setActiveSlotModal(index);
                      setTempSymbol(slot.symbol);
                    }}
                    className="rounded border border-line bg-base px-1.5 py-0.5 text-[10px] text-muted hover:text-ink transition"
                  >
                    换票 ⇄
                  </button>
                </div>

                <div className="flex items-center gap-1.5">
                  <a
                    href={`/stock/${slot.symbol}`}
                    className="rounded border border-line bg-base px-2 py-0.5 text-[11px] text-muted hover:text-ink flex items-center gap-0.5"
                  >
                    详情 <ArrowRight size={11} />
                  </a>
                </div>
              </div>

              {/* Chart Component */}
              <TradingViewAdvancedChart
                symbol={slot.symbol}
                defaultInterval={slot.interval}
                height={layout === '1x1' ? 620 : 420}
              />
            </div>
          ))}
        </div>

        {/* Change Symbol Modal */}
        {activeSlotModal !== null && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-xs p-4">
            <div ref={symbolDialogRef} role="dialog" aria-modal="true" aria-labelledby="symbol-dialog-title" tabIndex={-1} className="w-full max-w-sm rounded-xl border border-line bg-surface p-5 shadow-2xl space-y-4">
              <h3 id="symbol-dialog-title" className="text-sm font-bold text-ink">更换盯盘标的</h3>
              <div>
                <label htmlFor="multichart-symbol" className="text-xs text-muted block mb-1">输入美股代码或 A 股 6 位代码:</label>
                <input
                  id="multichart-symbol"
                  type="text"
                  value={tempSymbol}
                  onChange={(e) => setTempSymbol(e.target.value.toUpperCase())}
                  placeholder="例如: NVDA, TSLA, 300750"
                  className="w-full rounded-lg border border-line bg-base px-3 py-2 font-mono text-sm uppercase text-ink outline-none focus:border-accent"
                  autoFocus
                />
              </div>
              <div className="flex flex-wrap gap-1.5 text-xs">
                {['NVDA', 'AAPL', 'TSLA', 'PLTR', 'CRWV', '300750', '300308', '601127'].map((quick) => (
                  <button
                    key={quick}
                    type="button"
                    onClick={() => setTempSymbol(quick)}
                    className="rounded bg-surface-2 px-2 py-1 font-mono text-[11px] text-muted hover:text-ink"
                  >
                    {quick}
                  </button>
                ))}
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={closeSymbolModal}
                  className="rounded-lg border border-line px-3 py-1.5 text-xs text-muted hover:text-ink"
                >
                  取消
                </button>
                <button
                  type="button"
                  onClick={() => updateSlotSymbol(activeSlotModal, tempSymbol)}
                  className="rounded-lg bg-accent px-4 py-1.5 text-xs font-bold text-black hover:brightness-110"
                >
                  确认更换
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Pine Script 6-in-1 Modal */}
        <PineScriptModal
          isOpen={isPineModalOpen}
          onClose={() => setIsPineModalOpen(false)}
        />

      </div>
    </StockGodShell>
  );
};
