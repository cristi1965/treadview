import React, { useEffect, useState } from 'react';
import { AlertTriangle, Beaker, BrainCircuit, Play, Square } from 'lucide-react';
import { AgentFlowGraph } from '../components/AgentFlowGraph';
import { AshareMarketRadar } from '../components/AshareMarketRadar';
import { ReportViewToggle } from '../components/BilingualMarkdown';
import { DecisionGauge } from '../components/DecisionGauge';
import { KeyWatchlistPanel } from '../components/KeyWatchlistPanel';
import { LiveLog } from '../components/LiveLog';
import { MarketAnomaliesRadar } from '../components/MarketAnomaliesRadar';
import { QDIIPremiumRadar } from '../components/QDIIPremiumRadar';
import { ReportCard } from '../components/ReportCard';
import { StockChart } from '../components/StockChart';
import { useI18n } from '../i18n';
import { useAnalysisStore } from '../stores/analysisStore';
import { useUiStore } from '../stores/uiStore';
import { useAdminSession } from '../hooks/useAdminSession';
import { formatPhaseLabel } from '../utils/analysisLog';

const today = () => {
  const value = new Date();
  return `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, '0')}-${String(value.getDate()).padStart(2, '0')}`;
};

export const ResearchLab: React.FC = () => {
  const { authorized: hasAdminAccess } = useAdminSession();
  const {
    isRunning, currentNode, currentPhase, currentProgress, logs,
    marketReport, fundamentalsReport, sentimentReport, newsReport,
    xPlayerTakes, debateHistory, researchPlan, traderProposal, riskHistory,
    finalDecision, decisionType, startAnalysis, stopAnalysis, syncAnalysisStatus,
  } = useAnalysisStore();
  const { t } = useI18n();
  const reportView = useUiStore((state) => state.reportView);
  const [ticker, setTicker] = useState('NVDA');
  const [date, setDate] = useState(today);
  const [assetType, setAssetType] = useState('stock');
  const [error, setError] = useState('');

  useEffect(() => {
    void syncAnalysisStatus().catch((reason) => {
      setError(reason instanceof Error ? reason.message : String(reason));
    });
  }, [syncAnalysisStatus]);

  const handleStart = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!ticker || !date || !hasAdminAccess) return;
    setError('');
    try {
      await startAnalysis(ticker, date, assetType);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    }
  };

  const handleStop = async () => {
    setError('');
    try {
      await stopAnalysis();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    }
  };

  return (
    <div className="flex flex-col gap-5">
      <header className="border border-amber-500/35 bg-amber-500/10 p-4 sm:p-5">
        <div className="flex items-start gap-3"><AlertTriangle className="mt-0.5 shrink-0 text-amber-300" size={20} /><div><p className="text-xs font-semibold text-amber-200">未纳入当前产品承诺</p><h1 className="mt-1 text-xl font-bold text-ink">实验室 · 实时数据未就绪</h1><p className="mt-1 text-sm leading-relaxed text-muted">本页会请求实时行情并可调用外部 LLM。当前全局实时数据门禁未通过，任何输出都不得作为交易指令；默认研究请返回历史研究工作台。</p></div></div>
      </header>

      <section className="border-y border-line py-4">
        <h2 className="flex items-center gap-2 text-base font-bold text-ink"><Beaker size={18} className="text-violet-300" />多智能体实验</h2>
        {!hasAdminAccess && <p className="mt-2 text-xs text-amber-200">未配置管理权限，实验启动已禁用。</p>}
        <form onSubmit={handleStart} className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-4 sm:items-end">
          <label className="space-y-1 text-xs"><span>{t('dash.ticker')}</span><input className="w-full border border-line bg-surface-2 px-3 py-2" value={ticker} onChange={(event) => setTicker(event.target.value.toUpperCase())} disabled={isRunning} /></label>
          <label className="space-y-1 text-xs"><span>{t('dash.date')}</span><input type="date" className="w-full border border-line bg-surface-2 px-3 py-2" value={date} onChange={(event) => setDate(event.target.value)} disabled={isRunning} /></label>
          <label className="space-y-1 text-xs"><span>{t('dash.pipeline')}</span><select className="w-full border border-line bg-surface-2 px-3 py-2" value={assetType} onChange={(event) => setAssetType(event.target.value)} disabled={isRunning}><option value="stock">{t('dash.stock')}</option><option value="crypto">{t('dash.crypto')}</option></select></label>
          {!isRunning ? <button type="submit" disabled={!hasAdminAccess} className="flex items-center justify-center gap-2 border border-violet-500/35 bg-violet-500/10 px-3 py-2 text-sm font-semibold text-violet-200 disabled:cursor-not-allowed disabled:opacity-40"><Play size={15} />启动实验分析</button> : <button type="button" onClick={() => void handleStop()} className="flex items-center justify-center gap-2 border border-rose-500/35 px-3 py-2 text-sm text-rose-300"><Square size={15} />{t('dash.abort')}</button>}
        </form>
        {error && <div role="alert" className="mt-3 flex flex-wrap items-center gap-2 text-xs text-rose-300"><span>实验操作失败：{error}</span><button type="button" className="border border-rose-400/40 px-2 py-1" onClick={() => void syncAnalysisStatus().then(() => setError('')).catch((reason) => setError(reason instanceof Error ? reason.message : String(reason)))}>重新同步状态</button></div>}
        {isRunning && <div className="mt-3"><div className="flex justify-between text-xs"><span>{formatPhaseLabel(currentPhase, reportView)}</span><span>{currentProgress}%</span></div><div className="mt-1 h-2 overflow-hidden bg-surface-2"><div className="h-full bg-violet-400" style={{ width: `${currentProgress}%` }} /></div></div>}
      </section>

      <section className="border border-line bg-surface p-4 sm:p-5">
        <div className="mb-3 flex items-center gap-2"><BrainCircuit size={18} className="text-violet-300" /><h2 className="text-base font-bold text-ink">实验流程</h2></div>
        <div className="overflow-x-auto"><AgentFlowGraph currentNode={currentNode} currentPhase={currentPhase} /></div>
      </section>

      <section className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <div className="min-w-0 space-y-4"><StockChart ticker={ticker} /><LiveLog logs={logs} /></div>
        <div className="min-w-0 space-y-4"><div className="flex justify-end"><ReportViewToggle /></div><DecisionGauge decision={decisionType} /><ReportCard title={t('dash.pmVerdict')} icon={<BrainCircuit />} content={finalDecision} isLoading={currentNode === 'Portfolio Manager'} /><ReportCard title={t('dash.xPlayers')} icon={<BrainCircuit />} content={xPlayerTakes} isLoading={currentNode === 'X Players'} /><ReportCard title={t('dash.techReport')} icon={<BrainCircuit />} content={marketReport} isLoading={currentNode === 'Market Analyst'} /><ReportCard title={t('dash.fundReport')} icon={<BrainCircuit />} content={fundamentalsReport} isLoading={currentNode === 'Fundamentals Analyst'} /><ReportCard title={t('dash.sentReport')} icon={<BrainCircuit />} content={sentimentReport} isLoading={currentNode === 'Sentiment Analyst'} /><ReportCard title={t('dash.newsReport')} icon={<BrainCircuit />} content={newsReport} isLoading={currentNode === 'News Analyst'} /><ReportCard title={t('dash.researchDebate')} icon={<BrainCircuit />} content={debateHistory} isLoading={false} /><ReportCard title={t('dash.researchPlan')} icon={<BrainCircuit />} content={researchPlan} isLoading={false} /><ReportCard title={t('dash.traderProposal')} icon={<BrainCircuit />} content={traderProposal} isLoading={false} /><ReportCard title={t('dash.riskDebate')} icon={<BrainCircuit />} content={riskHistory} isLoading={false} /></div>
      </section>

      <section className="space-y-4 border-t border-line pt-5">
        <h2 className="text-base font-bold text-ink">实时观测实验组件</h2>
        <AshareMarketRadar />
        <KeyWatchlistPanel />
        <MarketAnomaliesRadar />
        <QDIIPremiumRadar />
      </section>
    </div>
  );
};
