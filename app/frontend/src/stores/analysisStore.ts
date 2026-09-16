import { create } from 'zustand'
import { API_BASE_URL, authorizedFetch } from '../utils/api'
import { appendBilingual } from '../utils/bilingual'
import type { ConsoleLog } from '../utils/analysisLog'

export interface NodeEvent {
  type: string
  node: string
  content?: string
  status?: string
  progress?: number
  timestamp: number
}

export type ResearchDecision = 'BUY' | 'HOLD' | 'SELL' | 'OBSERVE' | 'research_unavailable'

export interface ResearchHealth {
  structure_status: 'complete' | 'invalid' | string
  source_status: 'healthy' | 'degraded' | 'unavailable' | string
  publishable: boolean
  data_time: string
  reasons?: string[]
}

export interface AnalysisHistory {
  ticker: string
  trade_date: string
  research_mode?: string
  status: 'published' | 'research_unavailable' | string
  decision: ResearchDecision
  message?: string
  research_health: ResearchHealth
  completed_at: string
  duration_secs: number
  generation_duration_secs?: number
  elapsed_secs?: number
  duration_ms?: number
  generation_duration_ms?: number
  started_at?: string
  audit?: AnalysisAudit
  dossier?: EvidenceOnlyDossier
  state: {
    market_report?: string
    fundamentals_report?: string
    sentiment_report?: string
    news_report?: string
    x_player_takes?: string
    investment_plan?: string
    trader_investment_plan?: string
    final_trade_decision?: string
    investment_debate_state?: {
      history: string
    }
    risk_debate_state?: {
      history: string
    }
  }
}

export interface EvidenceOnlyFact {
  category: string
  summary: string
  provider: string
  source_url: string
  data_time: string
  filing_date?: string
  evidence_ids: string[]
}

export interface EvidenceCalculation {
  name: string
  formula: string
  inputs: Record<string, number>
  value?: number
  unit: string
  status: string
  reason?: string
  evidence_ids: string[]
}

export interface EvidenceFinancialPeriod {
  fiscal_period: string
  period_end: string
  filing_date: string
  accession: string
  source_url: string
  total_revenue?: number
  gross_profit?: number
  operating_income?: number
  net_income?: number
  total_assets?: number
  total_liabilities?: number
  stockholders_equity?: number
  cash_and_equivalents?: number
  available_fields: string[]
}

export interface EvidenceHistoricalObservation {
  date: string
  open: number
  high: number
  low: number
  close: number
  volume: number
}

export interface EvidenceHistorical {
  sample_count: number
  minimum_sessions: number
  start_date: string
  end_date: string
  time_granularity: string
  status: string
  observations: EvidenceHistoricalObservation[]
}

export interface EvidenceNewsItem {
  title: string
  url: string
  published_at: string
  source: string
  source_tier: string
  content_type: string
  theme: string
  relevance_rule: string
}

export interface EvidenceNewsReview {
  input_count: number
  included_count: number
  excluded_low_relevance: number
  duplicates_removed: number
  rules: string[]
  items: EvidenceNewsItem[]
}

export interface EvidenceResearchHolding {
  symbol: string
  quantity?: number
  weight_pct?: number
}

export interface EvidenceResearchScenario {
  revenue_growth_pct?: number
  ps_multiple?: number
}

export interface EvidenceResearchContext {
  mandate?: string
  holdings?: EvidenceResearchHolding[]
  liquidity?: string
  tax?: string
  risk_budget?: string
  issuer_cap_pct?: number
  scenario?: EvidenceResearchScenario
}

// Newer dossiers provide the server assessment. Aliases keep older persisted
// records readable while the response contract is rolling out.
export interface EvidenceContextAssessment {
  inputs?: Record<string, { status: string; value?: string; reason?: string }>
  holding?: {
    symbol: string
    quantity?: number
    weight_pct?: number
    status: string
    reason?: string
    evidence_ids?: string[]
  }
  issuer_cap?: {
    cap_pct?: number
    headroom_pct?: number
    within_cap?: boolean
    status: string
    reason?: string
    evidence_ids?: string[]
  }
  scenario?: EvidenceResearchScenario & {
    status: string
    base_ttm_revenue?: number
    implied_revenue?: number
    implied_market_cap?: number
    shares_outstanding?: number
    implied_price?: number
    current_price?: number
    change_pct?: number
    evidence_ids?: string[]
    reason?: string
    disclosure?: string
  }
  limitations?: string[]
  symbol?: string
  holding_quantity?: number
  holding_weight_pct?: number
  issuer_cap_pct?: number
  issuer_headroom_pct?: number
  headroom_pct?: number
  issuer_limit_exceeded?: boolean
  over_limit?: boolean
  status?: string
  reason?: string
}

export interface EvidenceOnlyDossier {
  method_version: string
  label: string
  disclaimer: string
  facts: EvidenceOnlyFact[]
  calculations: EvidenceCalculation[]
  financial_periods: EvidenceFinancialPeriod[]
  historical: EvidenceHistorical
  news_review: EvidenceNewsReview
  risks: string[]
  gaps: string[]
  conclusion_basis?: string[]
  conclusion: string
  research_context?: EvidenceResearchContext
  context_assessment?: EvidenceContextAssessment
}

export interface AnalysisEvidence {
  id: string
  kind: string
  source: string
  url?: string
  data_time: string
  fetched_at: string
  method_version: string
  content_hash?: string
  payload_excerpt?: string
  payload_truncated?: boolean
  payload_ref?: string
  payload_size?: number
  status: string
  inputs?: Record<string, string>
}

export interface AnalysisClaim {
  id: string
  artifact_id: string
  stage: string
  summary: string
  content_hash: string
  evidence_ids: string[]
  data_time: string
  status: string
  model: string
  method_version: string
  created_at: string
}

export interface AnalysisAudit {
  run_id: string
  created_at: string
  method_version: string
  model_provider: string
  models: Record<string, string>
  inputs: Record<string, string>
  input_hash: string
  evidence: AnalysisEvidence[]
  claims: AnalysisClaim[]
  health: ResearchHealth
}

export interface Config {
  llm_provider: string
  deep_think_llm: string
  quick_think_llm: string
  output_language: string
  max_debate_rounds: number
  max_risk_rounds: number
  llm_backend_url?: string
}

export interface ConfigUpdateResult {
  status: 'updated' | 'updated_with_unavailable_llm' | string
  llm_provider: string
  deep_think_llm: string
  quick_think_llm: string
  llm_backend_url?: string
  runtime_warning?: string
}

interface AnalysisState {
  // Connection & status
  wsStatus: 'connecting' | 'connected' | 'disconnected'
  isRunning: boolean
  currentProgress: number
  currentPhase: string
  currentNode: string
  logs: ConsoleLog[]

  // Live reports
  marketReport: string
  fundamentalsReport: string
  sentimentReport: string
  newsReport: string
  xPlayerTakes: string
  debateHistory: string
  researchPlan: string
  traderProposal: string
  riskHistory: string
  finalDecision: string
  decisionType: 'BUY' | 'HOLD' | 'SELL' | null
  researchDecision: ResearchDecision | null

  // Config & History
  config: Config | null
  history: AnalysisHistory[]
  historyLoading: boolean
  historyError: string

  // Setters & Actions
  setWsStatus: (status: 'connecting' | 'connected' | 'disconnected') => void
  startAnalysis: (ticker: string, date: string, assetType?: string) => Promise<void>
  stopAnalysis: () => Promise<void>
  syncAnalysisStatus: () => Promise<void>
  fetchConfig: () => Promise<void>
  updateConfig: (updates: Partial<Config>) => Promise<ConfigUpdateResult>
  fetchHistory: () => Promise<void>
  addLog: (log: ConsoleLog) => void
  handleWsEvent: (event: NodeEvent) => void
}

const API_BASE = `${API_BASE_URL}/api`

export const useAnalysisStore = create<AnalysisState>((set, get) => ({
  wsStatus: 'disconnected',
  isRunning: false,
  currentProgress: 0,
  currentPhase: 'Idle',
  currentNode: '',
  logs: [],

  marketReport: '',
  fundamentalsReport: '',
  sentimentReport: '',
  newsReport: '',
  xPlayerTakes: '',
  debateHistory: '',
  researchPlan: '',
  traderProposal: '',
  riskHistory: '',
  finalDecision: '',
  decisionType: null,
  researchDecision: null,

  config: null,
  history: [],
  historyLoading: false,
  historyError: '',

  setWsStatus: (status) => set({ wsStatus: status }),

  startAnalysis: async (ticker, date, assetType = 'stock') => {
    set({
      isRunning: true,
      currentProgress: 0,
      currentPhase: 'Initiating...',
      currentNode: 'System',
      logs: [],
      marketReport: '',
      fundamentalsReport: '',
      sentimentReport: '',
      newsReport: '',
      xPlayerTakes: '',
      debateHistory: '',
      researchPlan: '',
      traderProposal: '',
      riskHistory: '',
      finalDecision: '',
      decisionType: null,
      researchDecision: null,
    })

    try {
      const resp = await authorizedFetch(`${API_BASE}/analysis/start`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ticker, trade_date: date, asset_type: assetType }),
      })
      if (!resp.ok) {
        const data = await resp.json()
        throw new Error(data.error || 'Failed to start analysis')
      }
    } catch (err: any) {
      set({ isRunning: false })
      get().addLog({ ts: Date.now(), kind: 'start_fail', extra: err.message })
      throw err;
    }
  },

  stopAnalysis: async () => {
    try {
      const resp = await authorizedFetch(`${API_BASE}/analysis/stop`, { method: 'POST' })
      if (!resp.ok) {
        const data = await resp.json().catch(() => ({}))
        throw new Error(data.error || 'Failed to stop analysis')
      }
      set({ isRunning: false, currentPhase: 'Stopped by User' })
    } catch (err: any) {
      get().addLog({ ts: Date.now(), kind: 'stop_fail', extra: err.message })
	  throw err
    }
  },

  syncAnalysisStatus: async () => {
    const resp = await authorizedFetch(`${API_BASE}/analysis/status`, { cache: 'no-store' })
    if (!resp.ok) {
      const data = await resp.json().catch(() => ({}))
      throw new Error(data.error || '无法读取实验运行状态')
    }
    const data = await resp.json() as { status?: string }
    const running = data.status === 'running' || data.status === 'stopping'
    set({
      isRunning: running,
      currentPhase: data.status === 'stopping' ? 'Stopping...' : running ? 'Running...' : 'Idle',
      currentNode: running ? get().currentNode : '',
    })
  },

  fetchConfig: async () => {
    try {
      const resp = await authorizedFetch(`${API_BASE}/config`, { cache: 'no-store' })
      if (resp.ok) {
        const config = await resp.json()
        set({ config })
      } else {
        set({ config: null })
      }
    } catch (err: any) {
      console.error('Failed to fetch config:', err)
      set({ config: null })
    }
  },

  updateConfig: async (updates) => {
    try {
      const resp = await authorizedFetch(`${API_BASE}/config`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updates),
      })
      if (resp.ok) {
		const result = await resp.json() as ConfigUpdateResult
        await get().fetchConfig()
		return result
      } else {
        const data = await resp.json().catch(() => ({}))
        set({ config: null })
        throw new Error(data.error || 'Failed to update config')
      }
    } catch (err: any) {
      console.error('Failed to update config:', err)
      set({ config: null })
      throw err
    }
  },

  fetchHistory: async () => {
    set({ historyLoading: true, historyError: '' })
    try {
      const resp = await authorizedFetch(`${API_BASE}/analysis/history`, { cache: 'no-store' })
      if (!resp.ok) {
        const data = await resp.json().catch(() => ({})) as { error?: string }
        throw new Error(data.error || `历史记录请求失败 (${resp.status})`)
      }
      const history = await resp.json()
      if (!Array.isArray(history)) throw new Error('历史记录格式错误')
      set({ history, historyLoading: false })
    } catch (err: any) {
      console.error('Failed to fetch history:', err)
      set({ historyLoading: false, historyError: err instanceof Error ? err.message : '历史记录加载失败' })
    }
  },

  addLog: (log) => set((state) => ({ logs: [...state.logs.slice(-199), log] })),

  handleWsEvent: (event) => {
    const { type, node, content, progress } = event
    const store = get()
    const now = Date.now()

    switch (type) {
      case 'analysis_start':
        set({ isRunning: true, currentPhase: 'Running...' })
        store.addLog({ ts: now, kind: 'start' })
        break

      case 'progress':
        set({ currentPhase: node, currentProgress: progress || 0 })
        store.addLog({ ts: now, kind: 'progress', node, progress: progress || 0 })
        break

      case 'node_start':
        set({ currentNode: node })
        store.addLog({ ts: now, kind: 'node_start', node })
        break

      case 'node_complete':
        set({ currentNode: '' })
        store.addLog({ ts: now, kind: 'node_complete', node })

        if (node === 'Market Analyst') set({ marketReport: content })
        else if (node === 'Fundamentals Analyst') set({ fundamentalsReport: content })
        else if (node === 'Sentiment Analyst') set({ sentimentReport: content })
        else if (node === 'News Analyst') set({ newsReport: content })
        else if (node === 'X Players') set({ xPlayerTakes: content })
        else if (node === 'Bull Researcher') {
          set((state) => ({ debateHistory: appendBilingual(state.debateHistory, 'Bull Case', '多方观点', content || '') }))
        }
        else if (node === 'Bear Researcher') {
          set((state) => ({ debateHistory: appendBilingual(state.debateHistory, 'Bear Case', '空方观点', content || '') }))
        }
        else if (node === 'Research Manager') set({ researchPlan: content })
        else if (node === 'Trader') set({ traderProposal: content })
        else if (node === 'Aggressive Analyst') {
          set((state) => ({ riskHistory: appendBilingual(state.riskHistory, 'Aggressive View', '激进观点', content || '') }))
        }
        else if (node === 'Conservative Analyst') {
          set((state) => ({ riskHistory: appendBilingual(state.riskHistory, 'Conservative View', '保守观点', content || '') }))
        }
        else if (node === 'Neutral Analyst') {
          set((state) => ({ riskHistory: appendBilingual(state.riskHistory, 'Neutral View', '中性观点', content || '') }))
        }
        else if (node === 'Portfolio Manager') set({ finalDecision: content })
        break

      case 'stream':
        if (node === 'Market Analyst') set((state) => ({ marketReport: state.marketReport + content }))
        else if (node === 'Fundamentals Analyst') set((state) => ({ fundamentalsReport: state.fundamentalsReport + content }))
        else if (node === 'Sentiment Analyst') set((state) => ({ sentimentReport: state.sentimentReport + content }))
        else if (node === 'News Analyst') set((state) => ({ newsReport: state.newsReport + content }))
        break

      case 'analysis_complete':
        store.addLog({ ts: now, kind: 'done' })
        set({ isRunning: false, currentProgress: 100, currentPhase: 'Complete', currentNode: '' })

        if (content) {
          const parts = content.split(' → ')
          if (parts.length > 1) {
            const raw = parts[1].trim().toUpperCase()
            const dec: ResearchDecision = raw === 'BUY' || raw === 'HOLD' || raw === 'SELL' || raw === 'OBSERVE'
              ? raw
              : 'research_unavailable'
            set({
              researchDecision: dec,
              decisionType: dec === 'BUY' || dec === 'HOLD' || dec === 'SELL' ? dec : null,
            })
          }
        }
        store.fetchHistory()
        break

      case 'analysis_unavailable':
        store.addLog({ ts: now, kind: 'error', extra: content || 'research_unavailable' })
        set({ isRunning: false, currentNode: '', currentPhase: 'Research unavailable', researchDecision: 'research_unavailable', decisionType: null })
        store.fetchHistory()
        break

      case 'analysis_error':
        store.addLog({ ts: now, kind: 'error', extra: content })
        set({ isRunning: false, currentNode: '', currentPhase: 'Failed' })
        break
    }
  }
}))
