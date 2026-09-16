import React, { useEffect, useState } from 'react'
import { useAnalysisStore, AnalysisHistory } from '../stores/analysisStore'
import { AlertTriangle, ArrowLeft, Calendar, CheckCircle2, Clock, ChevronRight, RefreshCw, TestTube2 } from 'lucide-react'
import { useI18n } from '../i18n'
import { BilingualMarkdown, ReportViewToggle } from '../components/BilingualMarkdown'
import { EvidenceOnlyDossierView, formatEvidenceDecision } from '../components/research/EvidenceOnlyDossierView'
import { Link, useSearchParams } from 'react-router-dom'

type IntegrityState = 'valid' | 'degraded' | 'invalid' | 'unknown'
type ResearchRecordKind = 'evidence-only' | 'agent-run' | 'unavailable' | 'legacy'

export const deriveResearchRecordKind = (item: AnalysisHistory): ResearchRecordKind => {
  const mode = item.research_mode?.trim().toLowerCase()
  const status = item.status?.trim().toLowerCase()
  if (mode === 'evidence-only' || mode === 'evidence_only' || status === 'evidence_only') return 'evidence-only'
  if (status === 'research_unavailable') return 'unavailable'
  if (status === 'published' && Object.keys(item.audit?.models || {}).length > 0) return 'agent-run'
  return 'legacy'
}

const recordTypeStyle = (kind: ResearchRecordKind, language: 'zh' | 'en') => kind === 'evidence-only'
  ? { label: language === 'zh' ? '仅证据包 · 未调用 LLM · 非 10-Agent' : 'Evidence package · no LLM · not 10-Agent', className: 'border-sky-500/40 bg-sky-500/10 text-sky-200' }
  : kind === 'agent-run'
    ? { label: language === 'zh' ? '多智能体 Agent Run' : 'Multi-agent run', className: 'border-violet-500/40 bg-violet-500/10 text-violet-200' }
    : kind === 'unavailable'
      ? { label: language === 'zh' ? '研究不可用' : 'Research unavailable', className: 'border-red-500/45 bg-red-500/15 text-red-200' }
      : { label: language === 'zh' ? 'Legacy / 类型未知' : 'Legacy / unknown type', className: 'border-slate-500/45 bg-slate-500/15 text-slate-200' }

export const deriveAnalysisIntegrity = (item: AnalysisHistory): IntegrityState => {
  const audit = item.audit
  const health = item.research_health || audit?.health
  if (health?.structure_status === 'invalid') return 'invalid'
  if (item.status === 'research_unavailable' || (health && (!health.publishable || health.source_status !== 'healthy'))) return 'degraded'
  if (!audit?.run_id) return 'unknown'
  if (!audit.input_hash || !audit.inputs || !Array.isArray(audit.evidence) || !Array.isArray(audit.claims) || audit.evidence.length === 0 || audit.claims.length === 0) {
    return 'invalid'
  }
  const evidenceIDs = new Set(audit.evidence.map((entry) => entry.id).filter(Boolean))
  if (audit.claims.some((claim) => !claim.content_hash || !claim.evidence_ids?.length || claim.evidence_ids.some((id) => !evidenceIDs.has(id)))) {
    return 'invalid'
  }
  if (audit.evidence.some((entry) => !entry.status)) {
    return 'unknown'
  }
  if (audit.evidence.some((entry) => {
    if (entry.status === 'declared' || entry.kind === 'generated-artifact') return false
    const hasPersistedPayload = Boolean(entry.payload_ref && entry.payload_size)
    const hasInlineAuthenticatedInput = entry.kind === 'user-input' && Boolean(entry.payload_excerpt)
    return !entry.content_hash || (!hasPersistedPayload && !hasInlineAuthenticatedInput) || entry.data_time === 'unknown' || entry.status !== 'captured'
  })) {
    return 'degraded'
  }
  return 'valid'
}

const integrityStyle = (state: IntegrityState) => state === 'valid'
  ? { label: '整链有效', className: 'border-emerald-500/35 bg-emerald-500/10 text-emerald-300' }
  : state === 'invalid'
    ? { label: '整链无效', className: 'border-red-500/45 bg-red-500/15 text-red-300' }
    : state === 'degraded'
      ? { label: '整链降级', className: 'border-amber-500/45 bg-amber-500/15 text-amber-200' }
      : { label: '完整性未知', className: 'border-slate-500/45 bg-slate-500/15 text-slate-200' }

export const deriveResearchDuration = (item: AnalysisHistory) => {
  const direct = [
    item.generation_duration_secs,
    item.elapsed_secs,
    item.duration_secs,
    item.generation_duration_ms ? item.generation_duration_ms / 1000 : undefined,
    item.duration_ms ? item.duration_ms / 1000 : undefined,
  ].find((value) => typeof value === 'number' && Number.isFinite(value) && value > 0)
  if (direct) return { seconds: direct, derived: false }

  const completed = Date.parse(item.completed_at)
  const candidates = [item.started_at, ...(item.audit?.evidence || []).map((entry) => entry.fetched_at)]
    .map((value) => Date.parse(value || ''))
    .filter(Number.isFinite)
  if (Number.isFinite(completed) && candidates.length > 0) {
    const seconds = (completed - Math.min(...candidates)) / 1000
    if (seconds > 0) return { seconds, derived: true }
  }
  return null
}

const formatResearchDuration = (item: AnalysisHistory, language: 'zh' | 'en') => {
  const duration = deriveResearchDuration(item)
  if (!duration) return language === 'zh' ? '耗时待后端补充' : 'Duration pending backend'
  const seconds = duration.seconds < 10 ? duration.seconds.toFixed(1) : duration.seconds.toFixed(0)
  if (!duration.derived) return language === 'zh' ? `${seconds} 秒` : `${seconds}s`
  return language === 'zh' ? `约 ${seconds} 秒（按证据时间推算）` : `About ${seconds}s (derived from evidence times)`
}

const paperPrefillPath = (item: AnalysisHistory) => {
  const params = new URLSearchParams({ research_ticker: item.ticker })
  if (item.audit?.run_id) params.set('research_run_id', item.audit.run_id)
  return `/tactical?${params.toString()}`
}

export const History: React.FC = () => {
  const { history, historyLoading, historyError, fetchHistory } = useAnalysisStore()
  const [selectedItem, setSelectedItem] = useState<AnalysisHistory | null>(null)
  const [mobileDetailOpen, setMobileDetailOpen] = useState(false)
  const [searchParams] = useSearchParams()
  const requestedRunID = searchParams.get('run_id')
  const { t, language } = useI18n()

  useEffect(() => {
    fetchHistory()
  }, [])

  const getDecisionBadgeClass = (decision: string) => {
    switch (decision) {
      case 'BUY': return 'badge badge-buy'
      case 'SELL': return 'badge badge-sell'
      case 'HOLD': return 'badge badge-hold'
      case 'OBSERVE': return 'badge border border-sky-500/40 bg-sky-500/10 text-sky-200'
      default: return 'badge border border-red-500/45 bg-red-500/15 text-red-200'
    }
  }

  const historyGroups = [
    { kind: 'evidence-only' as const, archived: false, title: language === 'zh' ? '仅证据包' : 'Evidence packages', description: language === 'zh' ? '确定性证据整理，不调用 LLM，也不是 10-Agent 分析。' : 'Deterministic evidence review; no LLM call and not a 10-Agent analysis.' },
    { kind: 'agent-run' as const, archived: false, title: language === 'zh' ? '多智能体 Agent Run' : 'Multi-agent runs', description: language === 'zh' ? '包含模型与代理运行记录的研究结果。' : 'Research results with model and agent runtime records.' },
    { kind: 'unavailable' as const, archived: true, title: language === 'zh' ? '研究不可用' : 'Research unavailable', description: language === 'zh' ? '因来源或结构不完整而未发布的记录。' : 'Records withheld because sources or structure were incomplete.' },
    { kind: 'legacy' as const, archived: true, title: language === 'zh' ? 'Legacy / 类型未知' : 'Legacy / unknown type', description: language === 'zh' ? '缺少足够运行类型元数据，仅供追溯。' : 'Insufficient run-type metadata; retained for traceability only.' },
  ].map((group) => ({ ...group, items: history.filter((item) => deriveResearchRecordKind(item) === group.kind) }))
    .filter((group) => group.items.length > 0)

  useEffect(() => {
    if (historyLoading || historyError) return
    if (history.length === 0) {
      setSelectedItem(null)
      setMobileDetailOpen(false)
      return
    }
    if (requestedRunID) {
      const requested = history.find((item) => item.audit?.run_id === requestedRunID)
      if (requested && selectedItem?.audit?.run_id !== requestedRunID) {
        setSelectedItem(requested)
        setMobileDetailOpen(true)
      }
      if (requested) return
    }
    if (selectedItem) return
    const primary = historyGroups.find((group) => !group.archived)?.items[0]
    if (primary) setSelectedItem(primary)
  }, [history, historyLoading, historyError, requestedRunID, selectedItem])

  const selectedKind = selectedItem ? deriveResearchRecordKind(selectedItem) : null

  return (
    <div className={`grid grid-cols-1 gap-4 lg:gap-8 ${selectedItem ? 'lg:grid-cols-[minmax(280px,1fr)_minmax(0,1.5fr)]' : ''}`}>
      {/* Left List */}
      <div className={`card ${mobileDetailOpen && selectedItem ? 'hidden lg:flex' : 'flex'}`} style={{ flexDirection: 'column', gap: '1.5rem', height: 'fit-content' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div>
            <h1 style={{ fontSize: '1.25rem', color: 'white' }}>{t('hist.title')}</h1>
            <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>{t('hist.sub')}</p>
          </div>
          <button className="btn btn-secondary" onClick={() => fetchHistory()} disabled={historyLoading} style={{ padding: '0.5rem' }}>
            <RefreshCw size={16} className={historyLoading ? 'animate-spin' : ''} />
          </button>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          {historyLoading && history.length === 0 && (
            <div className="border-y border-line py-6 text-center text-sm text-muted">正在加载历史记录…</div>
          )}
          {historyError && (
            <div role="alert" className="space-y-3 border-l-2 border-rose-500 bg-rose-500/5 px-3 py-3 text-sm text-rose-200">
              <p>历史记录加载失败：{historyError}</p>
              {history.length > 0 && <p className="text-xs text-muted">已保留上次成功加载的记录。</p>}
              <button type="button" className="btn btn-secondary" onClick={() => fetchHistory()}>重试</button>
            </div>
          )}
          {!historyLoading && !historyError && history.length === 0 ? (
            <div className="space-y-3 border-y border-line py-6 text-center text-sm text-muted">
              <p>{t('hist.empty')}</p>
              <p className="text-xs">{language === 'zh' ? '先在驾驶舱检查管理权限和 historical-research 门禁，再生成不调用 LLM 的仅证据研究包。' : 'Check admin access and the historical-research gate in the cockpit, then create an evidence-only package without an LLM call.'}</p>
              <Link to="/dashboard#research-entry" className="inline-flex rounded-md border border-accent/40 bg-accent/10 px-3 py-2 text-xs font-semibold text-accent">
                {language === 'zh' ? '前往可审计研究入口' : 'Open the auditable research path'}
              </Link>
            </div>
          ) : historyGroups.map((group) => {
            const content = <>
              <div>
                <h3 className="text-sm font-semibold text-white">{group.title}</h3>
                <p className="text-[11px] text-muted">{group.description}</p>
              </div>
              {group.items.map((item) => {
                const integrity = integrityStyle(deriveAnalysisIntegrity(item))
                const type = recordTypeStyle(deriveResearchRecordKind(item), language)
                return (
                  <button
                    type="button"
                    key={`${item.audit?.run_id || item.ticker}-${item.completed_at}`}
                    onClick={() => { setSelectedItem(item); setMobileDetailOpen(true) }}
                    className="flex w-full items-center justify-between gap-3 rounded-lg border p-3 text-left transition"
                    style={{
                      backgroundColor: selectedItem === item ? 'rgba(99, 102, 241, 0.08)' : 'rgba(255,255,255,0.02)',
                      borderColor: selectedItem === item ? 'var(--color-primary)' : 'var(--border-color)',
                    }}
                  >
                    <span className="min-w-0 space-y-2">
                      <span className="flex flex-wrap items-center gap-2">
                        <span style={{ fontWeight: 700, fontSize: '1.1rem', color: 'white' }}>{item.ticker}</span>
                        <span className={getDecisionBadgeClass(item.decision)}>{deriveResearchRecordKind(item) === 'evidence-only' ? formatEvidenceDecision(item.dossier?.conclusion || item.decision, language) : item.decision}</span>
                        <span className={`inline-flex items-center rounded border px-1.5 py-0.5 text-[10px] font-semibold ${integrity.className}`}>{integrity.label}</span>
                      </span>
                      <span className={`inline-flex rounded border px-2 py-1 text-[10px] font-semibold ${type.className}`}>{type.label}</span>
                      {deriveResearchRecordKind(item) === 'evidence-only' && (
                        <span className="block text-[11px] font-semibold text-amber-200">{language === 'zh' ? '非投资建议' : 'Not investment advice'} · {formatEvidenceDecision(item.dossier?.conclusion || item.decision, language)}</span>
                      )}
                      <span className="flex flex-wrap gap-3 text-xs text-muted">
                        <span className="flex items-center gap-1"><Calendar size={12} /> {item.trade_date}</span>
                        <span className="flex items-center gap-1"><Clock size={12} /> {formatResearchDuration(item, language)}</span>
                      </span>
                    </span>
                    <ChevronRight size={16} className="shrink-0 text-faint" />
                  </button>
                )
              })}
            </>
            if (group.archived) {
              return <details key={group.kind} className="border-y border-line py-3">
                <summary className="cursor-pointer text-sm font-semibold text-muted">
                  {language === 'zh' ? `失败归档（${group.items.length}）` : `Failed archive (${group.items.length})`}
                </summary>
                <div className="mt-3 space-y-2">{content}</div>
              </details>
            }
            return <section key={group.kind} className="space-y-2">{content}</section>
          })}
        </div>
      </div>

      {/* Right Detail Panel */}
      {selectedItem && (
        <div className={`card flex-col gap-6 overflow-y-auto lg:flex lg:max-h-[80vh] ${mobileDetailOpen ? 'flex' : 'hidden'}`}>
          <button type="button" onClick={() => setMobileDetailOpen(false)} className="inline-flex items-center gap-2 self-start border border-line px-3 py-2 text-xs font-semibold text-accent lg:hidden"><ArrowLeft size={15} />{language === 'zh' ? '返回研究记录列表' : 'Back to research records'}</button>
          <div className="flex flex-wrap items-start justify-between gap-3 border-b border-line pb-4">
            <div>
              <h2 style={{ fontSize: '1.5rem', color: 'white' }}>{t('hist.details', { ticker: selectedItem.ticker })}</h2>
              <span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>{t('hist.evalOn', { date: selectedItem.trade_date })}</span>
            </div>
            <div className="flex flex-wrap items-center justify-end gap-2">
              <span className={getDecisionBadgeClass(selectedItem.decision)} style={{ fontSize: '1rem', padding: '0.5rem 1rem' }}>
                {selectedKind === 'evidence-only' ? formatEvidenceDecision(selectedItem.dossier?.conclusion || selectedItem.decision, language) : selectedItem.decision}
              </span>
              {(() => {
                const state = deriveAnalysisIntegrity(selectedItem)
                const status = integrityStyle(state)
                const Icon = state === 'valid' ? CheckCircle2 : AlertTriangle
                return <span className={`inline-flex items-center gap-1 rounded border px-2 py-1 text-xs font-semibold ${status.className}`}><Icon size={13} />{status.label}</span>
              })()}
            </div>
          </div>
          {selectedKind === 'evidence-only' && (
            <section className="border-y border-line py-3">
              <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <div><h2 className="text-sm font-semibold text-white">{language === 'zh' ? '转入独立 Paper 模拟' : 'Open independent Paper simulation'}</h2><p className="mt-1 text-xs text-muted">{language === 'zh' ? '仅传入股票代码与研究 Run ID；不预填方向、数量、价格或止损。Paper 仍须取得新鲜报价并通过服务端风控。' : 'Only the ticker and research Run ID are passed. Side, quantity, price, and stops are not prefilled; Paper still requires a fresh quote and server risk checks.'}</p></div>
                <Link to={paperPrefillPath(selectedItem)} className="inline-flex shrink-0 items-center justify-center gap-2 border border-sky-500/40 bg-sky-500/10 px-3 py-2 text-xs font-semibold text-sky-200"><TestTube2 size={15} />{language === 'zh' ? '去 Paper 模拟' : 'Open Paper simulation'}</Link>
              </div>
            </section>
          )}
          <ReportViewToggle />

          {selectedKind === 'agent-run' && (
            <section className="border border-violet-500/35 bg-violet-500/10 p-4 text-sm font-bold text-violet-100">
              多智能体 Agent Run · 模型与代理运行记录
            </section>
          )}

          {selectedItem.research_health ? (
            <section className="border border-line p-4 text-sm text-muted">
              <h2 className="text-base text-white">{language === 'zh' ? '研究发布状态' : 'Research publication status'}</h2>
              <div className="mt-2">{language === 'zh' ? '结构' : 'Structure'} {selectedItem.research_health.structure_status} · {language === 'zh' ? '来源' : 'Sources'} {selectedItem.research_health.source_status} · {language === 'zh' ? '数据期' : 'Data time'} {selectedItem.research_health.data_time || 'unknown'}</div>
              {selectedItem.research_health.reasons?.length ? <div className="mt-2 text-amber-200">{selectedItem.research_health.reasons.join('；')}</div> : null}
            </section>
          ) : null}

          {selectedKind === 'evidence-only' && selectedItem.dossier ? <EvidenceOnlyDossierView dossier={selectedItem.dossier} decision={selectedItem.decision} language={language} ticker={selectedItem.ticker} /> : null}

          <section style={{ border: '1px solid var(--border-color)', padding: '1rem' }}>
            <h2 style={{ fontSize: '1rem', color: 'white' }}>{language === 'zh' ? '结构化证据链' : 'Structured evidence chain'}</h2>
            {selectedItem.audit?.run_id ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem', marginTop: '0.75rem', fontSize: '0.78rem' }}>
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '0.5rem', color: 'var(--text-secondary)' }}>
                  <span>Run ID<br /><code style={{ color: 'white' }}>{selectedItem.audit.run_id}</code></span>
                  <span>{language === 'zh' ? '方法版本' : 'Method version'}<br /><code style={{ color: 'white' }}>{selectedItem.audit.method_version}</code></span>
                  <span>{language === 'zh' ? '模型' : 'Model'}<br /><code style={{ color: 'white' }}>{selectedItem.audit.model_provider} / {Object.values(selectedItem.audit.models || {}).join(' / ')}</code></span>
                  <span>{language === 'zh' ? '输入哈希' : 'Input hash'}<br /><code title={selectedItem.audit.input_hash} style={{ color: 'white' }}>{selectedItem.audit.input_hash?.slice(0, 24) || 'missing'}…</code></span>
                </div>
                <div style={{ color: 'var(--text-secondary)', overflowWrap: 'anywhere' }}>
                  {language === 'zh' ? '输入' : 'Inputs'}: {Object.entries(selectedItem.audit.inputs || {}).map(([key, value]) => `${key}=${value || 'unknown'}`).join(' · ')}
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.45rem' }}>
                  {(selectedItem.audit.claims || []).map((claim) => (
                    <div key={claim.id} style={{ borderTop: '1px solid var(--border-color)', paddingTop: '0.45rem' }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', gap: '0.75rem', color: 'white' }}>
                        <strong>{claim.stage}</strong><code>{claim.model}</code>
                      </div>
                      <div style={{ color: 'var(--text-secondary)', overflowWrap: 'anywhere' }}>
                        {claim.content_hash} · Evidence: {claim.evidence_ids.join(', ')}
                      </div>
                    </div>
                  ))}
                </div>
                <details>
                  <summary style={{ cursor: 'pointer', color: 'var(--color-primary)' }}>{language === 'zh' ? `查看 ${selectedItem.audit.evidence?.length || 0} 条证据对象` : `View ${selectedItem.audit.evidence?.length || 0} evidence objects`}</summary>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem', marginTop: '0.5rem' }}>
                    {(selectedItem.audit.evidence || []).map((evidence) => (
                      <div key={evidence.id} style={{ color: 'var(--text-secondary)', overflowWrap: 'anywhere' }}>
                        <code style={{ color: 'white' }}>{evidence.id}</code> · {evidence.source} · {evidence.status} · data {evidence.data_time} · fetched {evidence.fetched_at}
                        {evidence.payload_ref ? <> · payload {evidence.payload_ref} ({evidence.payload_size || 0} bytes)</> : null}
                        {evidence.inputs ? <> · inputs {Object.entries(evidence.inputs).map(([key, value]) => `${key}=${value}`).join(', ')}</> : null}
                        {evidence.url ? <> · <a href={evidence.url} target="_blank" rel="noreferrer" style={{ color: 'var(--color-primary)' }}>核对来源</a></> : null}
                      </div>
                    ))}
                  </div>
                </details>
              </div>
            ) : (
              <p style={{ marginTop: '0.5rem', color: 'var(--text-secondary)', fontSize: '0.8rem' }}>Legacy 结果：未保存 run ID、模型版本、输入来源和内容哈希，不能作为可复核研究底稿。</p>
            )}
          </section>

          {(selectedKind === 'agent-run' || selectedKind === 'legacy') && <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
            <div className="border-l-2 border-accent bg-accent/5 p-4">
              <h2>{selectedItem.audit?.model_provider === 'deepseek+gemini'
                ? (language === 'zh' ? '双模型研究结论' : 'Dual-model research conclusion')
                : (language === 'zh' ? 'Agent 研究结论' : 'Agent research conclusion')}</h2>
              <BilingualMarkdown content={selectedItem.state?.investment_plan || selectedItem.state?.final_trade_decision || ''} />
            </div>
            <div>
              <h2>{language === 'zh' ? 'Paper 模拟边界' : t('hist.pm')}</h2>
              <BilingualMarkdown content={selectedItem.state?.final_trade_decision || ''} />
            </div>

            <div>
              <h2>{t('hist.xPlayers')}</h2>
              <BilingualMarkdown content={selectedItem.state?.x_player_takes || ''} />
            </div>

            <div>
              <h2>{t('hist.tech')}</h2>
              <BilingualMarkdown content={selectedItem.state?.market_report || ''} />
            </div>

            <div>
              <h2>{t('hist.fund')}</h2>
              <BilingualMarkdown content={selectedItem.state?.fundamentals_report || ''} />
            </div>

            <div>
              <h2>{t('hist.sent')}</h2>
              <BilingualMarkdown content={selectedItem.state?.sentiment_report || ''} />
            </div>

            <div>
              <h2>{t('hist.news')}</h2>
              <BilingualMarkdown content={selectedItem.state?.news_report || ''} />
            </div>
          </div>}
        </div>
      )}
    </div>
  )
}
