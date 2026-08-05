import React, { useEffect, useMemo, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { BookOpen, Search } from 'lucide-react';
import { get as apiGet } from '../utils/api';
import { useUiStore } from '../stores/uiStore';

interface NotesItem {
  id: string;
  t: string;
  free?: boolean;
}

interface NotesSection {
  key: string;
  icon: string;
  t: string;
  blurb: string;
  items: NotesItem[];
}

interface NotesTOC {
  name: string;
  tagline: string;
  sections: NotesSection[];
  updatedAt?: string;
}

const READ_KEY = 'stockgod-notes-read-ids';

const loadReadIds = (): Set<string> => {
  try {
    const raw = localStorage.getItem(READ_KEY);
    const arr = raw ? (JSON.parse(raw) as string[]) : [];
    return new Set(Array.isArray(arr) ? arr : []);
  } catch {
    return new Set();
  }
};

const saveReadIds = (ids: Set<string>) => {
  localStorage.setItem(READ_KEY, JSON.stringify([...ids]));
};

interface ChartLine {
  y: number;
  label: string;
}

interface ChartMark {
  i: number;
  txt: string;
  below?: boolean;
}

interface ChartData {
  h: number;
  candles: [number, number, number, number][]; // [open, high, low, close]
  lines?: ChartLine[];
  marks?: ChartMark[];
  cap?: string;
}

const NoteChart: React.FC<{ data: ChartData; isDark: boolean }> = ({ data, isDark }) => {
  const { h = 220, candles, lines = [], marks = [], cap } = data;

  // Find min/max values to scale the Y axis
  const allY: number[] = [];
  candles.forEach(([o, h, l, c]) => {
    allY.push(o, h, l, c);
  });
  lines.forEach(line => allY.push(line.y));

  const rawMin = Math.min(...allY);
  const rawMax = Math.max(...allY);
  const padding = (rawMax - rawMin) * 0.15 || 1.0;
  const minY = rawMin - padding;
  const maxY = rawMax + padding;

  const svgWidth = 800;
  const svgHeight = h;
  const paddingLeft = 45;
  const paddingRight = 160;
  const paddingTop = 30;
  const paddingBottom = 30;

  const chartWidth = svgWidth - paddingLeft - paddingRight;
  const chartHeight = svgHeight - paddingTop - paddingBottom;

  const getY = (val: number) => {
    if (maxY === minY) return paddingTop + chartHeight / 2;
    return svgHeight - paddingBottom - ((val - minY) / (maxY - minY)) * chartHeight;
  };

  const n = candles.length;
  const candleSpacing = 16;
  const candleWidth = (chartWidth - (n - 1) * candleSpacing) / n;

  return (
    <div className="my-6">
      <div className={`rounded-xl border p-4 shadow-sm ${
        isDark ? 'border-[#ebeef512] bg-[#111317]' : 'border-[#e8e0d4] bg-white'
      }`}>
        <svg viewBox={`0 0 ${svgWidth} ${svgHeight}`} className="w-full overflow-visible">
          {/* Horizontal lines */}
          {lines.map((line, idx) => {
            const y = getY(line.y);
            return (
              <g key={idx}>
                <line
                  x1={paddingLeft}
                  y1={y}
                  x2={svgWidth - paddingRight}
                  y2={y}
                  stroke={isDark ? 'rgba(217, 138, 106, 0.4)' : 'rgba(180, 83, 9, 0.4)'}
                  strokeDasharray="4 4"
                  strokeWidth={1.5}
                />
                <text
                  x={svgWidth - paddingRight + 8}
                  y={y + 4}
                  className={`text-[11px] font-medium ${isDark ? 'fill-[#d98a6a]' : 'fill-[#b45309]'}`}
                >
                  {line.label} ({line.y})
                </text>
              </g>
            );
          })}

          {/* Candlesticks */}
          {candles.map(([open, high, low, close], idx) => {
            const x = paddingLeft + idx * (candleWidth + candleSpacing);
            const yOpen = getY(open);
            const yClose = getY(close);
            const yHigh = getY(high);
            const yLow = getY(low);

            const isUp = close >= open;
            const candleColor = isUp 
              ? (isDark ? '#2ebd85' : '#10b981') // Green for up
              : (isDark ? '#f6465d' : '#ef4444'); // Red for down

            const rectY = Math.min(yOpen, yClose);
            const rectHeight = Math.max(1, Math.abs(yOpen - yClose));

            return (
              <g key={idx}>
                {/* Wick */}
                <line
                  x1={x + candleWidth / 2}
                  y1={yHigh}
                  x2={x + candleWidth / 2}
                  y2={yLow}
                  stroke={candleColor}
                  strokeWidth={1.5}
                />
                {/* Body */}
                <rect
                  x={x}
                  y={rectY}
                  width={candleWidth}
                  height={rectHeight}
                  fill={candleColor}
                  stroke={candleColor}
                  strokeWidth={1}
                  rx={2}
                />
              </g>
            );
          })}

          {/* Marks */}
          {marks.map((mark, idx) => {
            const candle = candles[mark.i];
            if (!candle) return null;
            const [open, high, low, close] = candle;
            const x = paddingLeft + mark.i * (candleWidth + candleSpacing) + candleWidth / 2;
            
            const isBelow = mark.below;
            const targetY = isBelow ? getY(low) : getY(high);
            const textY = isBelow ? targetY + 16 : targetY - 10;
            const pinY = isBelow ? targetY + 4 : targetY - 4;

            const isBuy = mark.txt.includes('买');
            const markColor = isBuy 
              ? (isDark ? '#d98a6a' : '#b45309') 
              : (isDark ? '#ecedf0' : '#44403c');

            return (
              <g key={idx}>
                {/* Visual pin line connector */}
                <line
                  x1={x}
                  y1={targetY}
                  x2={x}
                  y2={pinY}
                  stroke={markColor}
                  strokeWidth={1}
                />
                {/* Mark circle backdrop */}
                <circle
                  cx={x}
                  cy={textY - 4}
                  r={isBuy ? 11 : 9}
                  fill={isBuy ? (isDark ? 'rgba(217,138,106,0.15)' : 'rgba(180,83,9,0.1)') : 'rgba(128,128,128,0.1)'}
                  stroke={markColor}
                  strokeWidth={1}
                />
                {/* Text tag */}
                <text
                  x={x}
                  y={textY}
                  textAnchor="middle"
                  className="text-[9px] font-bold"
                  fill={markColor}
                >
                  {mark.txt}
                </text>
              </g>
            );
          })}
        </svg>
      </div>
      {cap && (
        <p className={`mt-2 text-center text-xs italic ${isDark ? 'text-[#8e919b]' : 'text-[#78716c]'}`}>
          {cap}
        </p>
      )}
    </div>
  );
};

/** StockGod 「雷司令投资笔记」— 独立壳，不挂 StockGodShell；不影响 /journal。 */
export const Notes: React.FC = () => {
  const { id: routeId } = useParams<{ id?: string }>();
  const navigate = useNavigate();
  const [toc, setToc] = useState<NotesTOC | null>(null);
  const [openSection, setOpenSection] = useState<string | null>(null);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [markdown, setMarkdown] = useState('');
  const [loadingToc, setLoadingToc] = useState(true);
  const [loadingArt, setLoadingArt] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [readIds, setReadIds] = useState<Set<string>>(() => loadReadIds());
  const [query, setQuery] = useState('');
  const [fontSize, setFontSize] = useState<number>(14);

  const theme = useUiStore((s) => s.theme);
  const isDark = theme === 'dark';

  const renderContent = (content: string) => {
    const parts = content.split(/(:::chart[\s\S]*?:::)/g);
    return parts.map((part, idx) => {
      if (part.startsWith(':::chart')) {
        try {
          const jsonStr = part.replace(/^:::chart\s*/, '').replace(/\s*:::$/, '');
          const chartData = JSON.parse(jsonStr);
          return <NoteChart key={idx} data={chartData} isDark={isDark} />;
        } catch (e) {
          console.error('Failed to parse chart JSON:', e);
          return <pre key={idx} className="text-xs text-red-500 overflow-x-auto p-3 rounded-lg bg-surface-2 border border-line">{part}</pre>;
        }
      }
      return (
        <ReactMarkdown
          key={idx}
          components={{
            h2: ({ children }) => (
              <h3 className={`mb-2 mt-6 flex items-center gap-2 text-[15px] font-semibold first:mt-0 ${isDark ? 'text-[#ecedf0]' : 'text-[#1c1917]'}`}>
                <span className={isDark ? 'text-[#d98a6a]' : 'text-[#b45309]'}>◆</span>
                {children}
              </h3>
            ),
            h3: ({ children }) => (
              <h4 className={`mb-1.5 mt-4 text-[13px] font-semibold ${isDark ? 'text-[#ecedf0]' : 'text-[#1c1917]'}`}>{children}</h4>
            ),
            p: ({ children }) => <p className="my-2">{children}</p>,
            ul: ({ children }) => <ul className="my-2 list-disc space-y-1 pl-5">{children}</ul>,
            ol: ({ children }) => <ol className="my-2 list-decimal space-y-1 pl-5">{children}</ol>,
            li: ({ children }) => <li className={`marker:${isDark ? 'text-[#585b66]' : 'text-[#a8a29e]'}`}>{children}</li>,
            strong: ({ children }) => <strong className={`font-semibold ${isDark ? 'text-[#ecedf0]' : 'text-[#1c1917]'}`}>{children}</strong>,
            a: ({ href, children }) => (
              <a href={href} className={`${isDark ? 'text-[#d98a6a]' : 'text-[#b45309]'} hover:underline`} target="_blank" rel="noreferrer">
                {children}
              </a>
            ),
          }}
        >
          {part}
        </ReactMarkdown>
      );
    });
  };

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoadingToc(true);
      setError(null);
      try {
        const data = await apiGet<NotesTOC>('/api/notes/toc');
        if (cancelled) return;
        setToc(data);
        if (routeId) {
          const sec = data.sections?.find((s) => s.items.some((it) => it.id === routeId));
          setOpenSection(sec?.key || data.sections?.[0]?.key || null);
        } else {
          setOpenSection(null);
        }
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : '加载目录失败');
      } finally {
        if (!cancelled) setLoadingToc(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const total = useMemo(
    () => (toc?.sections || []).reduce((sum, sec) => sum + (sec.items?.length || 0), 0),
    [toc]
  );
  const readCount = readIds.size;

  const activeSection = useMemo(
    () => toc?.sections.find((sec) => sec.key === openSection) || null,
    [toc, openSection]
  );

  const filteredItems = useMemo(() => {
    if (!activeSection) return [];
    const q = query.trim().toLowerCase();
    if (!q) return activeSection.items;
    return activeSection.items.filter((item) => item.t.toLowerCase().includes(q));
  }, [activeSection, query]);

  const globalHits = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q || !toc) return [];
    const hits: { section: NotesSection; item: NotesItem }[] = [];
    for (const sec of toc.sections) {
      for (const item of sec.items) {
        if (item.t.toLowerCase().includes(q)) hits.push({ section: sec, item });
      }
    }
    return hits.slice(0, 24);
  }, [toc, query]);

  const activeMeta = useMemo(() => {
    if (!toc || !activeId) return null;
    for (const sec of toc.sections) {
      const idx = sec.items.findIndex((item) => item.id === activeId);
      if (idx >= 0) return { section: sec, item: sec.items[idx], index: idx + 1 };
    }
    return null;
  }, [toc, activeId]);

  useEffect(() => {
    if (activeMeta) {
      document.title = `${activeMeta.item.t} | ${toc?.name || '雷司令投资笔记'} · 我不是神`;
    } else {
      document.title = `${toc?.name || '雷司令投资笔记'} · 我不是神`;
    }
  }, [activeMeta, toc]);

  const openArticle = async (id: string, pushRoute = true) => {
    setActiveId(id);
    if (pushRoute && routeId !== id) {
      navigate(`/notes/${id}`, { replace: false });
    }
    setLoadingArt(true);
    setError(null);
    try {
      const data = await apiGet<{ md: string }>(`/api/notes/art?id=${encodeURIComponent(id)}`);
      setMarkdown(data.md || '');
      setReadIds((prev) => {
        if (prev.has(id)) return prev;
        const next = new Set(prev);
        next.add(id);
        saveReadIds(next);
        return next;
      });
    } catch (err) {
      setMarkdown('');
      setError(err instanceof Error ? err.message : '加载正文失败');
    } finally {
      setLoadingArt(false);
    }
  };

  useEffect(() => {
    if (!routeId || !toc) return;
    if (activeId === routeId && markdown) return;
    void openArticle(routeId, false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [routeId, toc]);

  const sectionReadCount = (sec: NotesSection) => sec.items.filter((item) => readIds.has(item.id)).length;

  const bgClass = isDark ? 'bg-[#08090b] text-[#ecedf0]' : 'bg-[#fdfaf3] text-[#1c1917]';
  const borderClass = isDark ? 'border-[#ebeef512]' : 'border-[#e8e0d4]';
  const textMutedClass = isDark ? 'text-[#8e919b]' : 'text-[#8a8175]';
  const textFaintClass = isDark ? 'text-[#585b66]' : 'text-[#a8a29e]';
  const headerBgClass = isDark ? 'bg-[#08090b]/95' : 'bg-[#fdfaf3]/95';

  if (loadingToc) {
    return (
      <div className={`flex min-h-screen items-center justify-center text-sm ${isDark ? 'bg-[#08090b] text-[#8e919b]' : 'bg-[#fdfaf3] text-[#6b7280]'}`}>
        加载笔记目录…
      </div>
    );
  }

  if (!toc) {
    return (
      <div className={`flex min-h-screen items-center justify-center text-sm ${isDark ? 'bg-[#08090b] text-[#f6465d]' : 'bg-[#fdfaf3] text-[#b42318]'}`}>
        {error || '目录不可用'}
      </div>
    );
  }

  return (
    <div className={`notes-shell flex min-h-screen flex-col transition-colors duration-150 ${bgClass}`}>
      <header className={`sticky top-0 z-20 border-b ${borderClass} ${headerBgClass} backdrop-blur-sm`}>
        <div className="mx-auto flex max-w-[1400px] flex-wrap items-center gap-3 px-4 py-3 sm:px-6">
          <div className="min-w-0 flex-1">
            <h1 className={`font-serif text-[18px] font-semibold tracking-tight sm:text-[20px] ${isDark ? 'text-[#ecedf0]' : 'text-[#1c1917]'}`}>
              {toc.name}
            </h1>
            <p className={`mt-0.5 text-xs ${textMutedClass}`}>{toc.tagline}</p>
          </div>
          <div className="relative w-full max-w-xs sm:w-56">
            <Search className={`pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 ${isDark ? 'text-[#585b66]' : 'text-[#a8a29e]'}`} />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="搜 K线 / MACD / 止损..."
              className={`w-full rounded-full border py-1.5 pl-8 pr-3 text-xs outline-none focus:ring-1 ${
                isDark
                  ? 'bg-[#111317] text-[#ecedf0] placeholder:text-[#585b66] border-[#ebeef512] focus:border-[#d98a6a] focus:ring-[#d98a6a]/30'
                  : 'bg-white text-[#1c1917] placeholder:text-[#a8a29e] border-[#e8e0d4] focus:border-[#c4b5a0] focus:ring-[#c4b5a0]/30'
              }`}
            />
          </div>
          <span className={`shrink-0 text-xs ${textMutedClass}`}>
            已读 {readCount}/{total}
          </span>
          <Link
            to="/"
            className={`shrink-0 rounded-full border px-3 py-1.5 text-xs font-medium transition ${
              isDark
                ? 'border-[#ebeef512] bg-[#111317] text-[#ecedf0] hover:border-[#d98a6a] hover:text-white'
                : 'border-[#e8e0d4] bg-white text-[#44403c] hover:border-[#c4b5a0] hover:text-[#1c1917]'
            }`}
          >
            我不是神 ↗
          </Link>
        </div>
      </header>

      <div className="mx-auto grid w-full max-w-[1400px] flex-1 grid-cols-1 lg:grid-cols-[200px_240px_minmax(0,1fr)]">
        <aside className={`border-b lg:border-b-0 lg:border-r ${borderClass}`}>
          <nav className="max-h-[40vh] space-y-0.5 overflow-y-auto p-2 lg:max-h-[calc(100vh-88px)]">
            <button
              type="button"
              onClick={() => {
                setOpenSection(null);
                setActiveId(null);
                setMarkdown('');
                navigate('/notes', { replace: false });
              }}
              className={`mb-2 flex w-full items-center gap-2 rounded-xl border px-2.5 py-3 text-left shadow-sm transition ${
                isDark
                  ? `border-[#ebeef512] bg-[#111317] ${!openSection && !activeId ? 'ring-1 ring-[#d98a6a]' : 'hover:bg-[#181b21]'}`
                  : `border-[#e8e0d4] bg-white ${!openSection && !activeId ? 'ring-1 ring-[#c4b5a0]' : 'hover:bg-[#fbf7ef]'}`
              }`}
            >
              <BookOpen className={`h-4 w-4 shrink-0 ${isDark ? 'text-[#d98a6a]' : 'text-[#b45309]'}`} strokeWidth={1.6} />
              <span className="min-w-0 flex-1">
                <span className={`block text-[13px] font-semibold ${isDark ? 'text-[#ecedf0]' : 'text-[#1c1917]'}`}>{toc.name}</span>
                <span className={`mt-0.5 block text-[11px] ${textMutedClass}`}>{toc.tagline}</span>
              </span>
            </button>
            {toc.sections.map((sec) => {
              const opened = openSection === sec.key;
              const done = sectionReadCount(sec);
              return (
                <button
                  key={sec.key}
                  type="button"
                  onClick={() => setOpenSection(opened ? null : sec.key)}
                  className={`flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left transition ${
                    opened
                      ? (isDark ? 'bg-[#181b21]' : 'bg-[#f0e9dc]')
                      : (isDark ? 'hover:bg-[#111317]' : 'hover:bg-[#f5efe4]')
                  }`}
                >
                  <span className="text-base leading-none">{sec.icon}</span>
                  <span className="min-w-0 flex-1">
                    <span className={`block text-[13px] font-medium ${isDark ? 'text-[#ecedf0]' : 'text-[#1c1917]'}`}>{sec.t}</span>
                    <span className={`mt-0.5 block text-[11px] ${textFaintClass}`}>
                      {done}/{sec.items.length}
                    </span>
                  </span>
                </button>
              );
            })}
          </nav>
        </aside>

        <aside className={`border-b lg:border-b-0 lg:border-r ${borderClass}`}>
          <div className="max-h-[40vh] overflow-y-auto p-3 lg:max-h-[calc(100vh-88px)]">
            {query.trim() && !activeSection ? (
              <div className="space-y-1">
                <p className={`mb-2 text-[11px] ${textFaintClass}`}>搜索结果 · {globalHits.length}</p>
                {globalHits.length === 0 ? (
                  <p className={`py-8 text-center text-xs ${textFaintClass}`}>没有匹配篇目</p>
                ) : (
                  globalHits.map(({ section, item }) => (
                    <button
                      key={item.id}
                      type="button"
                      onClick={() => {
                        setOpenSection(section.key);
                        void openArticle(item.id);
                      }}
                      className={`flex w-full flex-col rounded-md px-2 py-1.5 text-left text-[12px] transition ${
                        activeId === item.id
                          ? (isDark ? 'bg-[#181b21] text-[#ecedf0]' : 'bg-[#efe6d6] text-[#1c1917]')
                          : (isDark ? 'text-[#8e919b] hover:bg-[#111317]' : 'text-[#57534e] hover:bg-[#f5efe4]')
                      }`}
                    >
                      <span className="truncate">{item.t}</span>
                      <span className={`text-[10px] ${textFaintClass}`}>{section.t}</span>
                    </button>
                  ))
                )}
              </div>
            ) : !activeSection ? (
              <div className={`flex min-h-[220px] flex-col items-center justify-center px-3 text-center text-xs leading-relaxed ${textFaintClass}`}>
                <p>← 点左侧栏目</p>
                <p className="mt-1">展开这一章的目录</p>
              </div>
            ) : (
              <div className="space-y-1">
                <p className={`mb-2 text-[11px] leading-relaxed ${textFaintClass}`}>{activeSection.blurb}</p>
                {filteredItems.map((item) => {
                  const active = activeId === item.id;
                  const read = readIds.has(item.id);
                  return (
                    <button
                      key={item.id}
                      type="button"
                      onClick={() => openArticle(item.id)}
                      className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[12px] transition ${
                        active
                          ? (isDark ? 'bg-[#181b21] text-[#ecedf0]' : 'bg-[#efe6d6] text-[#1c1917]')
                          : (isDark ? 'text-[#8e919b] hover:bg-[#111317]' : 'text-[#57534e] hover:bg-[#f5efe4]')
                      }`}
                    >
                      <span className="min-w-0 flex-1 truncate">{item.t}</span>
                      {read && <span className={`shrink-0 text-[10px] ${textFaintClass}`}>已读</span>}
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        </aside>

        <main className="min-h-[50vh]">
          {!activeId && (
            <div className="px-5 py-5 sm:px-8">
              <div className={`mb-5 rounded-2xl border p-5 shadow-sm ${isDark ? 'border-[#ebeef512] bg-[#111317]' : 'border-[#e8e0d4] bg-white'}`}>
                <div className="flex flex-wrap items-end justify-between gap-3">
                  <div>
                    <p className={`text-xs font-medium ${isDark ? 'text-[#d98a6a]' : 'text-[#b45309]'}`}>{toc.tagline}</p>
                    <h2 className={`mt-1 font-serif text-2xl font-semibold tracking-tight ${isDark ? 'text-[#ecedf0]' : 'text-[#1c1917]'}`}>{toc.name}</h2>
                  </div>
                  <span className={`rounded-full px-3 py-1 text-xs ${isDark ? 'bg-[#181b21] text-[#8e919b]' : 'bg-[#f5efe4] text-[#78716c]'}`}>
                    已读 {readCount}/{total}
                  </span>
                </div>
              </div>
              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                {toc.sections.map((sec) => {
                  const done = sectionReadCount(sec);
                  return (
                    <button
                      key={sec.key}
                      type="button"
                      onClick={() => setOpenSection(sec.key)}
                      className={`min-h-[118px] rounded-xl border p-4 text-left shadow-sm transition hover:-translate-y-0.5 hover:shadow-md ${
                        isDark
                          ? 'border-[#ebeef512] bg-[#111317] hover:border-[#d98a6a]'
                          : 'border-[#e8e0d4] bg-white hover:border-[#c4b5a0]'
                      }`}
                    >
                      <div className="flex items-start gap-3">
                        <span className="text-2xl leading-none">{sec.icon}</span>
                        <span className="min-w-0 flex-1">
                          <span className={`block text-[15px] font-semibold ${isDark ? 'text-[#ecedf0]' : 'text-[#1c1917]'}`}>{sec.t}</span>
                          <span className={`mt-1 block text-xs leading-relaxed ${isDark ? 'text-[#8e919b]' : 'text-[#78716c]'}`}>{sec.blurb}</span>
                          <span className={`mt-3 inline-flex rounded-full px-2.5 py-1 text-[11px] ${isDark ? 'bg-[#181b21] text-[#8e919b]' : 'bg-[#f5efe4] text-[#8a8175]'}`}>
                            {done}/{sec.items.length} 已读
                          </span>
                        </span>
                      </div>
                    </button>
                  );
                })}
              </div>
              <p className={`mt-5 text-center text-xs ${textFaintClass}`}>选一篇开始读 · 全库 {total} 篇 · 已读 {readCount} 篇</p>
            </div>
          )}

          {activeId && (
            <article className="px-5 py-5 sm:px-8">
              {activeMeta && (
                <header className={`mb-5 border-b pb-4 flex flex-wrap items-center justify-between gap-3 ${isDark ? 'border-[#ebeef512]' : 'border-[#e8e0d4]'}`}>
                  <div>
                    <div className={`text-[11px] ${textFaintClass}`}>
                      {activeMeta.section.t} / 第 {activeMeta.index} 篇
                    </div>
                    <h2 className={`mt-1 font-serif text-xl font-semibold tracking-tight ${isDark ? 'text-[#ecedf0]' : 'text-[#1c1917]'}`}>
                      {activeMeta.item.t}
                    </h2>
                  </div>
                  <div className="flex items-center gap-1 sm:gap-1.5 shrink-0">
                    <button
                      type="button"
                      onClick={() => setFontSize(prev => Math.max(12, prev - 1))}
                      className={`rounded px-2.5 py-1 text-xs font-semibold border transition ${
                        isDark 
                          ? 'border-[#ebeef512] bg-[#111317] text-[#ecedf0] hover:border-[#d98a6a] hover:text-white' 
                          : 'border-[#e8e0d4] bg-white text-[#44403c] hover:border-[#c4b5a0] hover:text-[#1c1917]'
                      }`}
                      title="缩小字体"
                    >
                      A-
                    </button>
                    <span className={`text-[11px] font-medium px-1 ${textMutedClass}`}>
                      {fontSize}px
                    </span>
                    <button
                      type="button"
                      onClick={() => setFontSize(prev => Math.min(24, prev + 1))}
                      className={`rounded px-2.5 py-1 text-xs font-semibold border transition ${
                        isDark 
                          ? 'border-[#ebeef512] bg-[#111317] text-[#ecedf0] hover:border-[#d98a6a] hover:text-white' 
                          : 'border-[#e8e0d4] bg-white text-[#44403c] hover:border-[#c4b5a0] hover:text-[#1c1917]'
                      }`}
                      title="放大字体"
                    >
                      A+
                    </button>
                  </div>
                </header>
              )}

              {loadingArt && <p className={`text-sm ${textMutedClass}`}>加载正文…</p>}
              {error && <p className="text-sm text-[#f6465d]">{error}</p>}

              {!loadingArt && markdown && (
                <div className={`space-y-3 leading-relaxed ${isDark ? 'text-[#8e919b]' : 'text-[#57534e]'}`} style={{ fontSize: `${fontSize}px` }}>
                  {renderContent(markdown)}
                </div>
              )}
            </article>
          )}
        </main>
      </div>

      <footer className={`mt-auto border-t px-4 py-5 text-center ${isDark ? 'border-[#ebeef512]' : 'border-[#e8e0d4]'}`}>
        <div className={`font-serif text-sm ${isDark ? 'text-[#ecedf0]' : 'text-[#44403c]'}`}>雷司令投资笔记</div>
        <p className={`mx-auto mt-1 max-w-xl text-[11px] leading-relaxed ${textFaintClass}`}>
          本笔记为投资知识科普资料,不构成任何投资建议。投资有风险,决策请独立
        </p>
      </footer>
    </div>
  );
};
