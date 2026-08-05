import { create } from 'zustand'
import { API_BASE_URL } from '../utils/api'

export interface NodeEvent {
  type: string
  node: string
  content?: string
  status?: string
  progress?: number
  timestamp: number
}

export interface AnalysisHistory {
  ticker: string
  trade_date: string
  decision: 'BUY' | 'HOLD' | 'SELL'
  completed_at: string
  duration_secs: number
  state: {
    market_report: string
    fundamentals_report: string
    sentiment_report: string
    news_report: string
    investment_plan: string
    trader_investment_plan: string
    final_trade_decision: string
    investment_debate_state: {
      history: string
    }
    risk_debate_state: {
      history: string
    }
  }
}

export interface Config {
  llm_provider: string
  deep_think_llm: string
  quick_think_llm: string
  output_language: string
  max_debate_rounds: number
  max_risk_rounds: number
}

interface AnalysisState {
  // Connection & status
  wsStatus: 'connecting' | 'connected' | 'disconnected'
  isRunning: boolean
  currentProgress: number
  currentPhase: string
  currentNode: string
  logs: string[]

  // Live reports
  marketReport: string
  fundamentalsReport: string
  sentimentReport: string
  newsReport: string
  debateHistory: string
  researchPlan: string
  traderProposal: string
  riskHistory: string
  finalDecision: string
  decisionType: 'BUY' | 'HOLD' | 'SELL' | null

  // Config & History
  config: Config | null
  history: AnalysisHistory[]

  // Setters & Actions
  setWsStatus: (status: 'connecting' | 'connected' | 'disconnected') => void
  startAnalysis: (ticker: string, date: string, assetType?: string) => Promise<void>
  stopAnalysis: () => Promise<void>
  fetchConfig: () => Promise<void>
  updateConfig: (updates: Partial<Config>) => Promise<void>
  fetchHistory: () => Promise<void>
  addLog: (log: string) => void
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
  debateHistory: '',
  researchPlan: '',
  traderProposal: '',
  riskHistory: '',
  finalDecision: '',
  decisionType: null,

  config: null,
  history: [],

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
      debateHistory: '',
      researchPlan: '',
      traderProposal: '',
      riskHistory: '',
      finalDecision: '',
      decisionType: null,
    })

    try {
      const resp = await fetch(`${API_BASE}/analysis/start`, {
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
      get().addLog(`[Error] ${err.message}`)
      throw err;
    }
  },

  stopAnalysis: async () => {
    try {
      await fetch(`${API_BASE}/analysis/stop`, { method: 'POST' })
      set({ isRunning: false, currentPhase: 'Stopped by User' })
    } catch (err: any) {
      get().addLog(`[Error] Failed to stop: ${err.message}`)
    }
  },

  fetchConfig: async () => {
    try {
      const resp = await fetch(`${API_BASE}/config`)
      if (resp.ok) {
        const config = await resp.json()
        set({ config })
      }
    } catch (err: any) {
      console.error('Failed to fetch config:', err)
    }
  },

  updateConfig: async (updates) => {
    try {
      const resp = await fetch(`${API_BASE}/config`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updates),
      })
      if (resp.ok) {
        await get().fetchConfig()
      }
    } catch (err: any) {
      console.error('Failed to update config:', err)
    }
  },

  fetchHistory: async () => {
    try {
      const resp = await fetch(`${API_BASE}/analysis/history`)
      if (resp.ok) {
        const history = await resp.json()
        set({ history })
      }
    } catch (err: any) {
      console.error('Failed to fetch history:', err)
    }
  },

  addLog: (log) => set((state) => ({ logs: [...state.logs.slice(-199), log] })),

  handleWsEvent: (event) => {
    const { type, node, content, progress } = event
    const store = get()

    const formatTime = () => new Date().toLocaleTimeString()

    switch (type) {
      case 'analysis_start':
        store.addLog(`[${formatTime()}] 🚀 Analysis pipeline initiated.`)
        break

      case 'progress':
        set({ currentPhase: node, currentProgress: progress || 0 })
        store.addLog(`[${formatTime()}] ⚡ ${node} in progress (${progress}%).`)
        break

      case 'node_start':
        set({ currentNode: node })
        store.addLog(`[${formatTime()}] 🤖 Agent ${node} is executing...`)
        break

      case 'node_complete':
        set({ currentNode: '' })
        store.addLog(`[${formatTime()}] ✅ Agent ${node} completed successfully.`)

        if (node === 'Market Analyst') set({ marketReport: content })
        else if (node === 'Fundamentals Analyst') set({ fundamentalsReport: content })
        else if (node === 'Sentiment Analyst') set({ sentimentReport: content })
        else if (node === 'News Analyst') set({ newsReport: content })
        else if (node === 'Bull Researcher') {
          set((state) => ({ debateHistory: state.debateHistory + `\n\n### Bull Case\n${content}` }))
        }
        else if (node === 'Bear Researcher') {
          set((state) => ({ debateHistory: state.debateHistory + `\n\n### Bear Case\n${content}` }))
        }
        else if (node === 'Research Manager') set({ researchPlan: content })
        else if (node === 'Trader') set({ traderProposal: content })
        else if (node === 'Aggressive Analyst') {
          set((state) => ({ riskHistory: state.riskHistory + `\n\n### Aggressive View\n${content}` }))
        }
        else if (node === 'Conservative Analyst') {
          set((state) => ({ riskHistory: state.riskHistory + `\n\n### Conservative View\n${content}` }))
        }
        else if (node === 'Neutral Analyst') {
          set((state) => ({ riskHistory: state.riskHistory + `\n\n### Neutral View\n${content}` }))
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
        store.addLog(`[${formatTime()}] 🎉 Analysis pipeline finished successfully!`)
        set({ isRunning: false, currentProgress: 100, currentPhase: 'Complete', currentNode: '' })
        
        // Extract the decision from logs or parse from PM
        if (content) {
          const parts = content.split(' → ')
          if (parts.length > 1) {
            const dec = parts[1].trim().toUpperCase() as 'BUY' | 'HOLD' | 'SELL'
            set({ decisionType: dec })
          }
        }
        store.fetchHistory()
        break

      case 'analysis_error':
        store.addLog(`[${formatTime()}] ❌ Pipeline failed: ${content}`)
        set({ isRunning: false, currentNode: '', currentPhase: 'Failed' })
        break
    }
  }
}))
