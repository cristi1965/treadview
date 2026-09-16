import React, { useRef, useState } from 'react';
import { AlertTriangle, ArrowRight, FileSearch, History as HistoryIcon, Plus, ShieldCheck, Trash2 } from 'lucide-react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { formatEvidenceDecision } from '../components/research/EvidenceOnlyDossierView';
import { useI18n } from '../i18n';
import { useAnalysisStore } from '../stores/analysisStore';
import { apiUrl, authorizedFetch } from '../utils/api';
import { useAdminSession } from '../hooks/useAdminSession';
import { HistoricalReadiness, normalizeHistoricalReadiness } from '../utils/readiness';

interface ResearchContextDraft {
  mandate: string;
  liquidity: string;
  tax: string;
  riskBudget: string;
  issuerCapPct: string;
  revenueGrowthPct: string;
  psMultiple: string;
}

interface HoldingDraft {
  id: number;
  symbol: string;
  quantity: string;
  weightPct: string;
}

const translateResearchReason = (reason: string) => {
  if (reason.startsWith('quote/history latest completed trading date conflict:')) {
    return reason.replace('quote/history latest completed trading date conflict:', '行情与历史序列的最新完成交易日冲突：');
  }
  const fixed: Record<string, string> = {
    'no complete evidence-only dossier with a PIT quote': '暂无包含时点行情的完整仅证据包',
    'no complete evidence-only dossier with sufficient independent OHLCV history': '暂无包含足够独立 OHLCV 历史的完整仅证据包',
    'no complete evidence-only dossier with filing-bound fundamentals': '暂无包含披露日财务数据的完整仅证据包',
    'no complete evidence-only dossier with a reviewed news source': '暂无包含已复核新闻来源的完整仅证据包',
    'no complete evidence-only dossier': '暂无通过发布门禁的完整仅证据包',
  };
  return fixed[reason] || reason;
};

const translateResearchError = (message: string) => {
  if (/Bearer token required|local admin session expired/i.test(message)) {
    return '本机管理会话已失效，自动恢复失败，请刷新页面后重试';
  }
  return message;
};

const today = () => {
  const value = new Date();
  return `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, '0')}-${String(value.getDate()).padStart(2, '0')}`;
};

export const Dashboard: React.FC = () => {
  const { authorized: hasAdminAccess } = useAdminSession();
  const { history, fetchHistory } = useAnalysisStore();
  const { t, language } = useI18n();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [ticker, setTicker] = useState(() => searchParams.get('symbol')?.toUpperCase() || 'NVDA');
  const [date, setDate] = useState(today);
  const [historicalReadiness, setHistoricalReadiness] = useState<HistoricalReadiness | null>(null);
  const [readinessLoading, setReadinessLoading] = useState(true);
  const [evidenceBusy, setEvidenceBusy] = useState(false);
  const [evidenceError, setEvidenceError] = useState('');
  const [researchContext, setResearchContext] = useState<ResearchContextDraft>({
    mandate: '', liquidity: '', tax: '', riskBudget: '', issuerCapPct: '', revenueGrowthPct: '', psMultiple: '',
  });
  const [holdings, setHoldings] = useState<HoldingDraft[]>([{ id: 1, symbol: '', quantity: '', weightPct: '' }]);
  const nextHoldingID = useRef(2);

  const loadHistoricalReadiness = React.useCallback(async () => {
    setReadinessLoading(true);
    try {
      const response = await fetch(apiUrl('/api/readiness?profile=historical-research'), { cache: 'no-store' });
      const data = await response.json().catch(() => null);
      const readiness = normalizeHistoricalReadiness(data, response.status);
      if (!readiness) throw new Error(`历史研究门禁不可用 (HTTP ${response.status})`);
      setHistoricalReadiness(readiness);
    } catch (error) {
      setHistoricalReadiness(null);
      setEvidenceError(error instanceof Error ? error.message : '无法读取历史研究门禁');
    } finally {
      setReadinessLoading(false);
    }
  }, []);

  React.useEffect(() => {
    const symbol = searchParams.get('symbol');
    if (symbol) setTicker(symbol.toUpperCase());
  }, [searchParams]);

  React.useEffect(() => {
    void Promise.all([loadHistoricalReadiness(), fetchHistory()]);
  }, [fetchHistory, loadHistoricalReadiness]);

  const handleEvidenceOnlyStart = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!ticker || !date || !hasAdminAccess) return;
    const parseOptionalNumber = (raw: string, label: string) => {
      if (!raw.trim()) return undefined;
      const value = Number(raw);
      if (!Number.isFinite(value)) throw new Error(`${label}必须是有效数字`);
      return value;
    };
    let normalizedHoldings: Array<{ symbol: string; quantity?: number; weight_pct?: number }> = [];
    let issuerCapPct: number | undefined;
    let revenueGrowthPct: number | undefined;
    let psMultiple: number | undefined;
    try {
      normalizedHoldings = holdings
        .filter((holding) => holding.symbol.trim() || holding.quantity.trim() || holding.weightPct.trim())
        .map((holding, index) => {
          if (!holding.symbol.trim()) throw new Error(`第 ${index + 1} 行持仓缺少股票代码`);
          const quantity = parseOptionalNumber(holding.quantity, `第 ${index + 1} 行数量`);
          const weightPct = parseOptionalNumber(holding.weightPct, `第 ${index + 1} 行权重`);
          if (quantity === undefined && weightPct === undefined) throw new Error(`第 ${index + 1} 行至少填写数量或权重`);
          if (quantity != null && quantity < 0) throw new Error(`第 ${index + 1} 行数量不能为负数`);
          if (weightPct != null && (weightPct < 0 || weightPct > 100)) throw new Error(`第 ${index + 1} 行权重须在 0 到 100 之间`);
          return { symbol: holding.symbol.trim(), ...(quantity === undefined ? {} : { quantity }), ...(weightPct === undefined ? {} : { weight_pct: weightPct }) };
        });
      issuerCapPct = parseOptionalNumber(researchContext.issuerCapPct, '单一发行人上限');
      revenueGrowthPct = parseOptionalNumber(researchContext.revenueGrowthPct, '收入增长情景');
      psMultiple = parseOptionalNumber(researchContext.psMultiple, '市销率情景');
      if (issuerCapPct != null && (issuerCapPct < 0 || issuerCapPct > 100)) throw new Error('单一发行人上限须在 0 到 100 之间');
      if (psMultiple != null && psMultiple < 0) throw new Error('市销率情景不能为负数');
    } catch (error) {
      setEvidenceError(error instanceof Error ? `研究约束错误：${error.message}` : '研究约束格式错误');
      return;
    }
    setEvidenceBusy(true);
    setEvidenceError('');
    try {
      const response = await authorizedFetch(apiUrl('/api/analysis/evidence-only'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          ticker,
          trade_date: date,
          asset_type: 'stock',
          research_context: {
            mandate: researchContext.mandate,
            holdings: normalizedHoldings,
            liquidity: researchContext.liquidity,
            tax: researchContext.tax,
            risk_budget: researchContext.riskBudget,
            ...(issuerCapPct === undefined ? {} : { issuer_cap_pct: issuerCapPct }),
            ...(revenueGrowthPct === undefined && psMultiple === undefined ? {} : {
              scenario: {
                ...(revenueGrowthPct === undefined ? {} : { revenue_growth_pct: revenueGrowthPct }),
                ...(psMultiple === undefined ? {} : { ps_multiple: psMultiple }),
              },
            }),
          },
        }),
      });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) {
        const reasons = data.preflight?.reasons as string[] | undefined;
        throw new Error(reasons?.length ? reasons.map(translateResearchReason).join('；') : (data.error || data.message || `HTTP ${response.status}`));
      }
      const runID = typeof data?.audit?.run_id === 'string' ? data.audit.run_id : '';
      if (!runID) throw new Error('研究包已返回，但缺少可追溯的 run ID');
      await fetchHistory();
      navigate(`/history?run_id=${encodeURIComponent(runID)}`);
    } catch (error) {
      setEvidenceError(error instanceof Error ? translateResearchError(error.message) : '仅证据研究包生成失败');
      await loadHistoricalReadiness();
    } finally {
      setEvidenceBusy(false);
    }
  };

  const evidenceRecords = history
    .filter((item) => item.research_mode === 'evidence-only' || item.research_mode === 'evidence_only' || item.status === 'evidence_only')
    .slice(0, 4);

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col gap-5">
      <header className="border-b border-line pb-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <p className="text-xs font-semibold text-accent">历史时点研究</p>
            <h1 className="mt-1 text-xl font-bold text-ink sm:text-2xl">仅证据研究工作台</h1>
            <p className="mt-1 max-w-2xl text-sm leading-relaxed text-muted">基于指定交易日整理行情、财务披露、历史序列与新闻证据。默认不调用 LLM，不输出买卖、持有、仓位或执行建议。</p>
          </div>
          <Link to="/settings#product-readiness" className="inline-flex items-center gap-2 text-sm font-semibold text-accent">
            <ShieldCheck size={16} /> 查看产品门禁
          </Link>
        </div>
      </header>

      <nav className="grid grid-cols-2 gap-2 sm:hidden" aria-label="研究快捷入口">
        <a href="#research-entry" className="border border-accent/35 bg-accent/10 px-3 py-2 text-center text-xs font-semibold text-accent">开始历史研究</a>
        <Link to="/settings" className="border border-line bg-surface px-3 py-2 text-center text-xs font-semibold text-muted">数据健康与恢复</Link>
      </nav>

      <section id="research-entry" className="scroll-mt-20 border border-line bg-surface p-4 sm:p-6">
        <div className="flex flex-col gap-3 border-b border-line pb-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h2 className="flex items-center gap-2 text-base font-bold text-ink sm:text-lg"><FileSearch className="h-5 w-5 text-accent" />生成仅证据研究包</h2>
            <p className="mt-1 text-xs leading-relaxed text-muted">生成结果会进入研究证据记录；每项结论均需关联来源、数据时间和证据 ID。</p>
          </div>
          <span className={`inline-flex self-start border px-2 py-1 text-xs font-semibold ${historicalReadiness?.ready ? 'border-emerald-500/35 bg-emerald-500/10 text-emerald-300' : 'border-amber-500/40 bg-amber-500/10 text-amber-200'}`}>
            {readinessLoading ? '检查历史研究门禁…' : `历史研究 · ${historicalReadiness?.ready ? '就绪' : '未就绪'} · HTTP ${historicalReadiness?.httpStatus || '—'}`}
          </span>
        </div>

        {!hasAdminAccess && <div className="my-4 flex items-start gap-2 border-l-2 border-amber-500 px-3 text-xs leading-relaxed text-amber-200"><AlertTriangle size={15} className="mt-0.5 shrink-0" /><span>管理权限未启用，生成已禁用。<Link to="/settings" className="ml-1 font-semibold text-accent">前往授权</Link></span></div>}
        {historicalReadiness && !historicalReadiness.ready && (
          <details className="my-4 border-y border-line py-3 text-xs text-muted">
            <summary className="cursor-pointer font-semibold text-amber-200">当前门禁未通过，展开查看原因</summary>
            <ul className="mt-2 list-disc space-y-1 pl-5">
              {historicalReadiness.checks.filter((check) => check.status !== 'ok').map((check) => <li key={check.id}><code>{check.id}</code>：{translateResearchReason(check.detail || check.remedy || '未通过')}</li>)}
            </ul>
            <p className="mt-2 text-faint">此门禁只证明历史时点研究链，不代表实时行情就绪，也不提供交易信号。</p>
            <p className="mt-1 text-amber-100">尚无合格研究包时仍可发起首次生成；生成后会重新检查全部门禁。</p>
          </details>
        )}

        <form onSubmit={handleEvidenceOnlyStart} className="mt-4 space-y-4">
          <div className="grid grid-cols-1 items-end gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] sm:gap-4">
            <label className="space-y-1 text-xs font-semibold text-muted"><span>{t('dash.ticker')}</span><input type="text" className="w-full border border-line bg-surface-2 px-3 py-2 text-sm font-mono uppercase text-ink focus:border-accent focus:outline-none" value={ticker} onChange={(event) => setTicker(event.target.value.toUpperCase())} disabled={evidenceBusy} placeholder={t('dash.tickerPh')} /></label>
            <label className="space-y-1 text-xs font-semibold text-muted"><span>{t('dash.date')}</span><input type="date" className="w-full border border-line bg-surface-2 px-3 py-2 text-sm text-ink focus:border-accent focus:outline-none" value={date} onChange={(event) => setDate(event.target.value)} disabled={evidenceBusy} /></label>
            <button type="submit" disabled={!hasAdminAccess || evidenceBusy} className="flex w-full items-center justify-center gap-2 bg-accent px-4 py-2 text-sm font-bold text-black transition hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-45"><FileSearch size={16} />{evidenceBusy ? '生成中…' : '生成仅证据研究包'}</button>
          </div>
          <details className="border-y border-line py-3">
            <summary className="cursor-pointer text-xs font-semibold text-accent">可选研究约束（原样写入审计输入，空值不推断）</summary>
            <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
              <label className="space-y-1 text-xs font-semibold text-muted"><span>投资授权 / mandate</span><textarea className="min-h-20 w-full resize-y border border-line bg-surface-2 px-3 py-2 text-sm font-normal text-ink focus:border-accent focus:outline-none" value={researchContext.mandate} onChange={(event) => setResearchContext((current) => ({ ...current, mandate: event.target.value }))} placeholder="例：仅研究美股大盘股，不授权下单" /></label>
              <label className="space-y-1 text-xs font-semibold text-muted"><span>流动性要求 / liquidity</span><textarea className="min-h-20 w-full resize-y border border-line bg-surface-2 px-3 py-2 text-sm font-normal text-ink focus:border-accent focus:outline-none" value={researchContext.liquidity} onChange={(event) => setResearchContext((current) => ({ ...current, liquidity: event.target.value }))} placeholder="例：T+2，需每日可变现" /></label>
              <label className="space-y-1 text-xs font-semibold text-muted"><span>税务背景 / tax</span><textarea className="min-h-20 w-full resize-y border border-line bg-surface-2 px-3 py-2 text-sm font-normal text-ink focus:border-accent focus:outline-none" value={researchContext.tax} onChange={(event) => setResearchContext((current) => ({ ...current, tax: event.target.value }))} placeholder="例：未提供，不做税务推断" /></label>
              <label className="space-y-1 text-xs font-semibold text-muted"><span>风险预算 / risk_budget</span><textarea className="min-h-20 w-full resize-y border border-line bg-surface-2 px-3 py-2 text-sm font-normal text-ink focus:border-accent focus:outline-none" value={researchContext.riskBudget} onChange={(event) => setResearchContext((current) => ({ ...current, riskBudget: event.target.value }))} placeholder="例：单一发行人上限 5%" /></label>
              <label className="space-y-1 text-xs font-semibold text-muted"><span>单一发行人上限 / issuer_cap_pct（%）</span><input type="number" min="0" max="100" step="0.01" className="w-full border border-line bg-surface-2 px-3 py-2 text-sm font-normal text-ink focus:border-accent focus:outline-none" value={researchContext.issuerCapPct} onChange={(event) => setResearchContext((current) => ({ ...current, issuerCapPct: event.target.value }))} placeholder="例：5" /></label>
              <div className="grid grid-cols-2 gap-3">
                <label className="space-y-1 text-xs font-semibold text-muted"><span>收入增长情景（%）</span><input type="number" step="0.01" className="w-full border border-line bg-surface-2 px-3 py-2 text-sm font-normal text-ink focus:border-accent focus:outline-none" value={researchContext.revenueGrowthPct} onChange={(event) => setResearchContext((current) => ({ ...current, revenueGrowthPct: event.target.value }))} placeholder="可选" /></label>
                <label className="space-y-1 text-xs font-semibold text-muted"><span>市销率情景</span><input type="number" min="0" step="0.01" className="w-full border border-line bg-surface-2 px-3 py-2 text-sm font-normal text-ink focus:border-accent focus:outline-none" value={researchContext.psMultiple} onChange={(event) => setResearchContext((current) => ({ ...current, psMultiple: event.target.value }))} placeholder="可选" /></label>
              </div>
              <div className="space-y-2 sm:col-span-2">
                <div className="flex items-center justify-between gap-3">
                  <span className="text-xs font-semibold text-muted">持仓 / holdings（至少填写 symbol 与 quantity 或 weight_pct）</span>
                  <button type="button" onClick={() => setHoldings((current) => [...current, { id: nextHoldingID.current++, symbol: '', quantity: '', weightPct: '' }])} className="inline-flex items-center gap-1 border border-accent/40 px-2 py-1 text-xs font-semibold text-accent"><Plus size={13} />新增持仓</button>
                </div>
                <div className="space-y-2">
                  {holdings.map((holding, index) => (
                    <div key={holding.id} className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_36px] gap-2">
                      <label className="min-w-0"><span className="sr-only">第 {index + 1} 行股票代码</span><input type="text" className="w-full min-w-0 border border-line bg-surface-2 px-2 py-2 text-xs font-mono text-ink focus:border-accent focus:outline-none" value={holding.symbol} onChange={(event) => setHoldings((current) => current.map((item) => item.id === holding.id ? { ...item, symbol: event.target.value } : item))} placeholder="symbol" /></label>
                      <label className="min-w-0"><span className="sr-only">第 {index + 1} 行数量</span><input type="number" min="0" step="any" className="w-full min-w-0 border border-line bg-surface-2 px-2 py-2 text-xs text-ink focus:border-accent focus:outline-none" value={holding.quantity} onChange={(event) => setHoldings((current) => current.map((item) => item.id === holding.id ? { ...item, quantity: event.target.value } : item))} placeholder="quantity" /></label>
                      <label className="min-w-0"><span className="sr-only">第 {index + 1} 行权重百分比</span><input type="number" min="0" max="100" step="0.01" className="w-full min-w-0 border border-line bg-surface-2 px-2 py-2 text-xs text-ink focus:border-accent focus:outline-none" value={holding.weightPct} onChange={(event) => setHoldings((current) => current.map((item) => item.id === holding.id ? { ...item, weightPct: event.target.value } : item))} placeholder="weight %" /></label>
                      <button type="button" aria-label={`删除第 ${index + 1} 行持仓`} title="删除持仓" onClick={() => setHoldings((current) => current.length === 1 ? [{ ...current[0], symbol: '', quantity: '', weightPct: '' }] : current.filter((item) => item.id !== holding.id))} className="inline-flex h-9 w-9 items-center justify-center border border-line text-muted hover:border-rose-500/50 hover:text-rose-300"><Trash2 size={15} /></button>
                    </div>
                  ))}
                </div>
                <p className="text-[11px] font-normal text-faint">空行不会提交；界面会生成与后端兼容的 holdings JSON 数组。</p>
              </div>
            </div>
          </details>
        </form>
        {evidenceError && <div role="alert" className="mt-3 border-l-2 border-rose-500 px-3 text-xs leading-relaxed text-rose-300"><strong>仅证据研究包未生成：</strong>{evidenceError}</div>}
      </section>

      <section className="border-y border-line py-4">
        <div className="flex items-center justify-between gap-3">
          <div><h2 className="flex items-center gap-2 text-base font-bold text-ink"><HistoryIcon size={18} className="text-accent" />已有证据摘要</h2><p className="mt-1 text-xs text-muted">仅展示已保存的 evidence-only 研究记录。</p></div>
          <Link to="/history" className="inline-flex shrink-0 items-center gap-1 text-xs font-semibold text-accent">全部记录 <ArrowRight size={14} /></Link>
        </div>
        <div className="mt-3 divide-y divide-line">
          {evidenceRecords.length ? evidenceRecords.map((item) => (
            <Link key={item.audit?.run_id || `${item.ticker}-${item.completed_at}`} to={item.audit?.run_id ? `/history?run_id=${encodeURIComponent(item.audit.run_id)}` : '/history'} className="grid gap-2 py-3 transition hover:bg-white/[0.02] sm:grid-cols-[120px_minmax(0,1fr)_auto] sm:items-center sm:px-2">
              <div><strong className="text-ink">{item.ticker}</strong><div className="text-[11px] text-faint">{item.trade_date}</div></div>
              <div className="min-w-0 text-xs text-muted"><span className="font-semibold text-sky-200">{formatEvidenceDecision(item.dossier?.conclusion || item.decision, language)}</span><span className="mt-1 block">事实 {item.dossier?.facts?.length || 0} · 计算 {item.dossier?.calculations?.length || 0} · 财务期 {item.dossier?.financial_periods?.length || 0} · 历史 {item.dossier?.historical?.sample_count || 0}</span></div>
              <span className="text-[11px] text-emerald-300">{item.research_health?.publishable ? '证据链完整' : '需复核'}</span>
            </Link>
          )) : <p className="py-6 text-center text-sm text-muted">暂无已保存的仅证据研究，使用上方单一入口生成。</p>}
        </div>
      </section>

      <aside className="flex flex-col gap-2 border-l-2 border-amber-500 px-3 py-1 text-xs leading-relaxed text-muted sm:flex-row sm:items-center sm:justify-between">
        <span><strong className="text-amber-200">实验室不属于当前产品承诺。</strong> 实时行情与 10-Agent 流程仅供受控实验，实时数据未就绪时不得用于交易判断。</span>
        <Link to="/lab" className="inline-flex shrink-0 items-center gap-1 font-semibold text-accent">进入实验室 <ArrowRight size={14} /></Link>
      </aside>
    </div>
  );
};
