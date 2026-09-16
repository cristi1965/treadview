import React, { useState, useRef, useEffect } from 'react';
import { post } from '../utils/api';
import {
  MessageSquare,
  Send,
  ShieldAlert,
  Zap,
  Sparkles,
  RefreshCw,
  Copy,
  Check,
  Bot,
  User,
  Trash2,
  TrendingUp,
  Globe,
  Sliders,
  DollarSign,
  AlertTriangle,
  Lightbulb,
  Compass,
  ArrowRight
} from 'lucide-react';
import { StockLogo } from '../components/StockLogo';
import { fetchQuoteResult, type QuoteMap } from '../utils/liveQuotes';
import { assessDataTrust, type DataTrustState } from '../utils/dataTrust';
import { normalizeCopilotMarketData, type TrustedCopilotMarketData } from '../utils/copilotMarketData';

interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: number;
  persona?: string;
  symbol?: string;
  marketData?: TrustedCopilotMarketData;
}

interface PersonaOption {
  id: string;
  name: string;
  title: string;
  icon: string;
  desc: string;
  color: string;
}

const PERSONAS: PersonaOption[] = [
  {
    id: 'general',
    name: '全能导师',
    title: 'NEMO 综合交易导师',
    icon: '🤖',
    desc: '平衡技术、基本面、宏观与实战策略',
    color: 'from-amber-500/20 to-orange-500/20 text-amber-400 border-amber-500/30',
  },
  {
    id: 'cro',
    name: '首席风控官',
    title: 'CRO 极度严苛风控',
    icon: '🛡️',
    desc: '先核验数据来源与时间，再讨论风险预算',
    color: 'from-rose-500/20 to-red-500/20 text-rose-400 border-rose-500/30',
  },
  {
    id: 'quant',
    name: '量化策略师',
    title: '华尔街量化对冲',
    icon: '🧮',
    desc: '数学期望、波动率与情景分析',
    color: 'from-blue-500/20 to-indigo-500/20 text-blue-400 border-blue-500/30',
  },
  {
    id: 'hot_money',
    name: '游资操盘手',
    title: 'A 股顶级短线游资',
    icon: '🗡️',
    desc: '美股隔夜映射、CPO/算力主线、分时量能突破',
    color: 'from-purple-500/20 to-pink-500/20 text-purple-400 border-purple-500/30',
  },
  {
    id: 'macro',
    name: '宏观对冲总监',
    title: '全球宏观周期',
    icon: '🌐',
    desc: '美联储利率路径、美债收益率、大宗与汇率跨资产',
    color: 'from-emerald-500/20 to-teal-500/20 text-emerald-400 border-emerald-500/30',
  },
  {
    id: 'night_owl',
    name: '不盯盘导师',
    title: '隔夜条件单实战',
    icon: '🌙',
    desc: '仅在来源和数据时间可验证时讨论条件单参数',
    color: 'from-amber-400/20 to-yellow-500/20 text-amber-300 border-amber-400/30',
  },
];

const PRESET_PROMPTS = [
  {
    label: '亏损持仓解套诊断',
    icon: '🚑',
    prompt: '我手头持仓发生亏损，处于焦虑迷茫状态。请从风控和数学期望角度，帮我诊断当前该减仓、止损还是卧倒？如何避免更大的本金损伤？',
  },
  {
    label: 'QDII 杀溢价避坑求助',
    icon: '🚨',
    prompt: '我关注的 513100 纳指 ETF 场内溢价较高，请结合当前真实净值和美股期货，评估现在是否有杀溢价踩踏风险？我该如何操作？',
  },
  {
    label: '隔夜条件单参数计算',
    icon: '🌙',
    prompt: '请先核验 NVDA 行情来源、数据时间和 stale 状态；若任一项不可验证，请拒绝计算交易参数并说明缺失证据。',
  },
  {
    label: '美联储利率决议推演',
    icon: '📊',
    prompt: '分析当前美联储最新降息/加息预期对美股科技 M7、纳指 100 以及 A 股核心科技链的流动性冲击与机会。',
  },
  {
    label: '失业期防爆雷资产分配',
    icon: '🛡️',
    prompt: '在收入中断/失业期间，我手里有一笔有限的本金，该如何通过 3 层资产防火墙严格划分救命生存金与低风险防守仓，绝对杜绝爆仓？',
  },
];

const POPULAR_SYMBOLS = ['NVDA', '513100', 'QQQ', '300750', '300308', 'PLTR', 'TSLA', 'NDX'];

export const TradingCopilot: React.FC = () => {
  const [messages, setMessages] = useState<ChatMessage[]>(() => {
    try {
      const saved = localStorage.getItem('nemo_trading_copilot_chat');
      if (saved) {
        const parsed = JSON.parse(saved);
        if (Array.isArray(parsed)) {
          return parsed.map((message: ChatMessage) => {
            const marketData = normalizeCopilotMarketData(message.marketData);
            return marketData ? { ...message, marketData } : { ...message, marketData: undefined };
          });
        }
      }
    } catch {}
    return [
      {
        id: 'welcome',
        role: 'assistant',
        content: `👋 **你好！我是 NEMO「我不是神」实战金融交易 AI 导师终端。**

这里是你的**自主提问与战术参谋部**。任何关于：
- 📉 **持仓亏损诊断与保命防守**
- 🛡️ **QDII 场内杀溢价与 ETF 避坑**
- 🌙 **有来源、数据时间与非过期状态的条件单参数复核**
- 🌐 **美股隔夜映射 ➔ A 股联动主线研判**
- 🧮 **非对称 1:3 盈亏比与失业期 3 层防爆雷资金配置**

你可以在此提问。系统只有在接口返回来源、数据时间且未标记 stale 时才使用行情；证据不足时应拒绝给出交易参数。所有条件单与交易计划仅用于 Paper 模拟，不会连接券商或执行真实交易。

👇 请选择下方的快捷指令，或在输入框中直接告诉我你的交易疑问：`,
        timestamp: Date.now(),
        persona: 'general',
      },
    ];
  });

  const [input, setInput] = useState('');
  const [selectedPersona, setSelectedPersona] = useState<string>('general');
  const [selectedSymbol, setSelectedSymbol] = useState<string>('');
  const [useDeep, setUseDeep] = useState(false);
  const [loading, setLoading] = useState(false);
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [liveQuotes, setLiveQuotes] = useState<QuoteMap>({});
  const [quoteTrust, setQuoteTrust] = useState<{ state: DataTrustState | 'loading'; reason: string }>({ state: 'loading', reason: '' });

  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    try {
      localStorage.setItem('nemo_trading_copilot_chat', JSON.stringify(messages));
    } catch {}
  }, [messages]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, loading]);

  useEffect(() => {
    let active = true;
    void fetchQuoteResult(POPULAR_SYMBOLS).then((result) => {
      if (!active) return;
      const trust = assessDataTrust(result.meta, Object.keys(result.quotes).length > 0 && result.missing.length === 0);
      setQuoteTrust(trust);
      setLiveQuotes(trust.state === 'live' ? result.quotes : {});
    }).catch((error) => {
      if (!active) return;
      setLiveQuotes({});
      setQuoteTrust({ state: 'unavailable', reason: error instanceof Error ? error.message : '行情不可用' });
    });
    return () => { active = false; };
  }, []);

  const handleSend = async (questionText?: string) => {
    const q = (questionText || input).trim();
    if (!q || loading) return;

    const userMsg: ChatMessage = {
      id: `user-${Date.now()}`,
      role: 'user',
      content: q,
      timestamp: Date.now(),
      symbol: selectedSymbol || undefined,
      persona: selectedPersona,
    };

    setMessages((prev) => [...prev, userMsg]);
    if (!questionText) setInput('');
    setLoading(true);

    try {
      // Build history
      const historyPayload = messages.slice(-6).map((m) => ({
        role: m.role,
        content: m.content,
      }));

      const res = await post<{
        answer: string;
        persona: string;
        symbol?: string;
        marketData?: unknown;
        timestamp: number;
      }>('/api/chat/ask', {
        question: q,
        persona: selectedPersona,
        selectedSymbol: selectedSymbol || undefined,
        history: historyPayload,
        useDeep,
      }, { timeoutMs: 95_000 });

      const assistantMsg: ChatMessage = {
        id: `ai-${Date.now()}`,
        role: 'assistant',
        content: res.answer,
        timestamp: res.timestamp || Date.now(),
        persona: res.persona || selectedPersona,
        symbol: res.symbol,
        marketData: normalizeCopilotMarketData(res.marketData),
      };

      setMessages((prev) => [...prev, assistantMsg]);
    } catch (err: any) {
      const errorMsg: ChatMessage = {
        id: `err-${Date.now()}`,
        role: 'assistant',
        content: `⚠️ **研判生成失败**：${err.message || '网络连接或服务暂时不可用，请稍后重试。'}`,
        timestamp: Date.now(),
        persona: selectedPersona,
      };
      setMessages((prev) => [...prev, errorMsg]);
    } finally {
      setLoading(false);
    }
  };

  const handleClearChat = () => {
    if (confirm('确定要清空当前对话记录吗？')) {
      const initialWelcome: ChatMessage = {
        id: 'welcome',
        role: 'assistant',
        content: '💬 对话已重置。请在下方选择交易导师人设或输入你的金融交易问题：',
        timestamp: Date.now(),
        persona: 'general',
      };
      setMessages([initialWelcome]);
      try {
        localStorage.removeItem('nemo_trading_copilot_chat');
      } catch {}
    }
  };

  const copyContent = (id: string, text: string) => {
    void navigator.clipboard.writeText(text).then(() => {
      setCopiedId(id);
      setTimeout(() => setCopiedId(null), 2000);
    });
  };

  const currentPersonaObj = PERSONAS.find((p) => p.id === selectedPersona) || PERSONAS[0];

  return (
    <div className="mx-auto flex h-[calc(100dvh-118px)] min-h-[32rem] max-w-6xl flex-col space-y-3 pb-4 lg:h-[calc(100dvh-80px)]">
      {/* 顶部 Header & 人设选择栏 */}
      <div className="rounded-2xl border border-line bg-surface p-3.5 sm:p-4 shadow-lg shrink-0">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-line pb-3">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-amber-500/20 to-orange-500/20 text-amber-400 border border-amber-500/30 text-xl shrink-0">
              💬
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h1 className="text-base sm:text-lg font-black text-ink">
                  AI 交易问答与战术参谋终端
                </h1>
                <span className="rounded-full bg-amber-500/15 px-2 py-0.5 text-[11px] font-bold text-amber-300 border border-amber-500/30">
                  Trading Copilot
                </span>
              </div>
              <p className="text-xs text-muted">
                自由提问 · 仅在来源与数据时间通过门禁时注入行情 · 6 大模拟导师
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <label className="flex items-center gap-1.5 text-xs text-muted cursor-pointer hover:text-ink transition select-none bg-surface-2 px-2.5 py-1.5 rounded-lg border border-line">
              <input
                type="checkbox"
                checked={useDeep}
                onChange={(e) => setUseDeep(e.target.checked)}
                className="rounded border-line bg-surface text-amber-500 focus:ring-0"
              />
              <Sparkles size={13} className={useDeep ? 'text-amber-400 animate-spin' : 'text-muted'} />
              <span>深度多步推理 (DeepThink)</span>
            </label>

            <button
              onClick={handleClearChat}
              title="清空对话"
              className="flex items-center gap-1 rounded-lg border border-line bg-surface-2 px-2.5 py-1.5 text-xs text-muted hover:text-rose-400 hover:border-rose-500/40 transition"
            >
              <Trash2 size={13} />
              <span className="hidden sm:inline">清空</span>
            </button>
          </div>
        </div>

        {/* 🎭 导师人设卡片滑动栏 */}
        <div className="mt-3 flex items-center gap-2 overflow-x-auto pb-1 scrollbar-none">
          <span className="text-xs text-muted font-bold shrink-0">选择导师：</span>
          {PERSONAS.map((p) => (
            <button
              key={p.id}
              onClick={() => setSelectedPersona(p.id)}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-bold transition shrink-0 border ${
                selectedPersona === p.id
                  ? `bg-surface-2 border-amber-500/60 text-ink shadow-md`
                  : `bg-surface-2/40 border-line text-muted hover:text-ink hover:border-line/80`
              }`}
            >
              <span className="text-sm">{p.icon}</span>
              <span>{p.name}</span>
            </button>
          ))}
        </div>
      </div>

      {/* 💬 对话主滚动区 */}
      <div className="flex-1 overflow-y-auto rounded-2xl border border-line bg-[#0B0E17] p-3.5 sm:p-5 shadow-inner space-y-4">
        {messages.map((m) => {
          const isUser = m.role === 'user';
          const pObj = PERSONAS.find((p) => p.id === m.persona) || PERSONAS[0];

          return (
            <div
              key={m.id}
              className={`flex gap-3 ${isUser ? 'flex-row-reverse' : 'flex-row'}`}
            >
              {/* Avatar */}
              <div
                className={`flex h-8 w-8 sm:h-9 sm:w-9 items-center justify-center rounded-xl shrink-0 text-sm sm:text-base border ${
                  isUser
                    ? 'bg-amber-500/20 text-amber-300 border-amber-500/40'
                    : 'bg-slate-800 text-slate-200 border-slate-700'
                }`}
              >
                {isUser ? <User size={16} /> : <span>{pObj.icon}</span>}
              </div>

              {/* Message Bubble */}
              <div className={`max-w-[88%] sm:max-w-[80%] space-y-1.5`}>
                <div className={`flex items-center gap-2 text-[10px] text-muted ${isUser ? 'justify-end' : 'justify-start'}`}>
                  <span>{isUser ? '我的提问' : `${pObj.title}`}</span>
                  <span>·</span>
                  <span>{new Date(m.timestamp).toLocaleTimeString()}</span>
                  {m.symbol && (
                    <span className="rounded bg-slate-800 px-1.5 py-0.2 font-mono text-amber-300 border border-slate-700">
                      {m.symbol}
                    </span>
                  )}
                </div>

                <div
                  className={`rounded-2xl p-3.5 sm:p-4 text-xs sm:text-sm leading-relaxed border relative group ${
                    isUser
                      ? 'bg-amber-500/10 border-amber-500/30 text-ink'
                      : 'bg-slate-900/90 border-slate-800 text-slate-100 shadow-lg'
                  }`}
                >
                  {/* Live market data badge if attached */}
                  {m.marketData && Object.keys(m.marketData).length > 0 && (
                    <div className="mb-3 rounded-lg border border-slate-800 bg-slate-950/80 p-2 text-xs flex flex-wrap items-center gap-3">
                      <span className="text-[10px] font-bold text-amber-400">⚡ 事实行情注入:</span>
                      {m.marketData.price !== undefined && (
                        <span className="font-mono">
                          现价: ¥/$ {m.marketData.price}{' '}
                          {m.marketData.pct !== undefined && (
                            <span className={m.marketData.pct >= 0 ? 'text-emerald-400' : 'text-rose-400'}>
                              ({m.marketData.pct >= 0 ? '+' : ''}{m.marketData.pct}%)
                            </span>
                          )}
                        </span>
                      )}
                      {m.marketData.premiumPct !== undefined && (
                        <span className="font-mono text-rose-400 font-bold">
                          溢价率: {m.marketData.premiumPct >= 0 ? '+' : ''}{m.marketData.premiumPct.toFixed(2)}%
                        </span>
                      )}
                      <span className="text-[10px] text-slate-400">来源 {m.marketData.meta.source} · 数据时间 {new Date(m.marketData.meta.dataTime).toLocaleString()}</span>
                    </div>
                  )}

                  {/* Render content as formatted text */}
                  <div className="prose prose-invert prose-xs max-w-none whitespace-pre-wrap">
                    {m.content}
                  </div>

                  {/* Copy button */}
                  {!isUser && (
                    <button
                      onClick={() => copyContent(m.id, m.content)}
                      className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition rounded-lg bg-slate-800/80 border border-slate-700 p-1.5 text-slate-300 hover:text-white"
                      title="复制回答内容"
                    >
                      {copiedId === m.id ? <Check size={13} className="text-emerald-400" /> : <Copy size={13} />}
                    </button>
                  )}
                </div>
              </div>
            </div>
          );
        })}

        {loading && (
          <div className="flex gap-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-slate-800 text-slate-200 border border-slate-700 shrink-0">
              <Bot size={16} className="animate-pulse text-amber-400" />
            </div>
            <div className="rounded-2xl bg-slate-900 border border-slate-800 p-3.5 text-xs text-slate-300 flex items-center gap-2">
              <RefreshCw size={13} className="animate-spin text-amber-400" />
              <span>{currentPersonaObj.name}正在查阅实盘行情、精算风控与推演决策中...</span>
            </div>
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* 🚀 快捷提问指令 Chips */}
      <div className="flex items-center gap-2 overflow-x-auto py-1 scrollbar-none shrink-0">
        <span className="text-[11px] font-bold text-muted shrink-0">快捷提问:</span>
        {PRESET_PROMPTS.map((p, idx) => (
          <button
            key={idx}
            onClick={() => handleSend(p.prompt)}
            disabled={loading}
            className="flex items-center gap-1.5 rounded-full border border-line bg-surface-2/60 px-3 py-1 text-[11px] font-medium text-muted hover:text-ink hover:border-amber-500/40 hover:bg-surface-2 transition shrink-0"
          >
            <span>{p.icon}</span>
            <span>{p.label}</span>
          </button>
        ))}
      </div>

      {/* ⌨️ 底部输入与标的注入控制栏 */}
      <div className="rounded-2xl border border-line bg-surface p-3 shadow-xl shrink-0 space-y-2">
          <div className="flex items-center justify-between gap-2 overflow-x-auto pb-1 text-xs">
          <div className="flex items-center gap-1.5 shrink-0">
            <span className="text-muted text-[11px]">绑定标的 (可选):</span>
            {POPULAR_SYMBOLS.map((sym) => {
              const q = liveQuotes[sym];
              return (
                <button
                  key={sym}
                  onClick={() => setSelectedSymbol(selectedSymbol === sym ? '' : sym)}
                  className={`rounded-lg px-2 py-0.5 font-mono text-[11px] transition flex items-center gap-1 border ${
                    selectedSymbol === sym
                      ? 'bg-amber-500 text-black font-bold border-amber-500'
                      : 'bg-surface-2 text-muted border-line hover:text-ink'
                  }`}
                >
                  <StockLogo symbol={sym} size={13} />
                  <span>{sym}</span>
                  {q && (
                    <span className={`text-[9px] ${q.pct >= 0 ? 'text-emerald-400' : 'text-rose-400'}`}>
                      {q.pct >= 0 ? '+' : ''}{q.pct.toFixed(1)}%
                    </span>
                  )}
                </button>
              );
            })}
          </div>
          {quoteTrust.state !== 'live' && quoteTrust.state !== 'loading' && (
            <p role="status" className="text-[10px] text-amber-200">热门标的行情未通过可信度门禁，已隐藏涨跌数据：{quoteTrust.reason}</p>
          )}

          {selectedSymbol && (
            <button
              onClick={() => setSelectedSymbol('')}
              className="text-[10px] text-muted hover:text-rose-400 transition underline shrink-0"
            >
              清除绑定 ({selectedSymbol})
            </button>
          )}
        </div>

        <form
          onSubmit={(e) => {
            e.preventDefault();
            void handleSend();
          }}
          className="flex items-center gap-2"
        >
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder={`向 ${currentPersonaObj.name} 自由提问（如：持仓盈亏诊断、今晚怎么挂条件单、宏观降息影响...）`}
            disabled={loading}
            className="flex-1 rounded-xl border border-line bg-surface-2 px-3.5 py-2.5 text-xs sm:text-sm text-ink placeholder-muted focus:outline-none focus:border-amber-500/50"
          />

          <button
            type="submit"
            disabled={!input.trim() || loading}
            className="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-amber-500 to-orange-500 px-4 py-2.5 text-xs sm:text-sm font-black text-slate-950 shadow-md hover:brightness-110 active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed transition shrink-0"
          >
            {loading ? <RefreshCw size={15} className="animate-spin" /> : <Send size={15} />}
            <span>提问</span>
          </button>
        </form>
      </div>
    </div>
  );
};
