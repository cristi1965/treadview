import { create } from 'zustand'
import { get as apiGet, post as apiPost } from '../utils/api'

export interface Trade {
  id: number
  createdAt: string
  environment: 'PAPER'
  symbol: string
  direction: 'BUY' | 'SELL'
  entryPrice: number
  exitPrice: number
  shares: number
  pnl: number
  emotionScore: number
  notes: string
  paperOrderId?: string
  researchRunId?: string
}

export interface Stats {
  environment: 'PAPER'
  totalPnL: number
  winRate: number
  totalTrades: number
}

export interface MacroEvent {
  id: number
  date: string
  time: string
  name: string
  stars: number
  previous: string
  consensus: string
}

interface CommandState {
  trades: Trade[]
  stats: Stats | null
  events: MacroEvent[]
  tradesError: string | null
  statsError: string | null
  eventsError: string | null
  fetchTrades: () => Promise<void>
  fetchStats: () => Promise<void>
  fetchEvents: () => Promise<void>
  addTrade: (trade: Omit<Trade, 'id' | 'createdAt' | 'environment'>) => Promise<void>
  addEvent: (event: Omit<MacroEvent, 'id'>) => Promise<void>
}

const errorMessage = (error: unknown, fallback: string) =>
  error instanceof Error && error.message ? error.message : fallback

export const useCommandStore = create<CommandState>((set, get) => ({
  trades: [],
  stats: null,
  events: [],
  tradesError: null,
  statsError: null,
  eventsError: null,

  fetchTrades: async () => {
    set({ tradesError: null })
    try {
      const trades = await apiGet<Trade[]>('/api/trades')
      if (!Array.isArray(trades)) throw new Error('交易日志返回格式无效')
      set({ trades, tradesError: null })
    } catch (err) {
      set({ trades: [], tradesError: errorMessage(err, '无法读取交易日志') })
    }
  },

  fetchStats: async () => {
    set({ statsError: null })
    try {
      const stats = await apiGet<Stats>('/api/stats')
      if (!stats || typeof stats !== 'object'
        || typeof stats.totalPnL !== 'number'
        || typeof stats.winRate !== 'number'
        || typeof stats.totalTrades !== 'number') {
        throw new Error('交易统计返回格式无效')
      }
      set({ stats, statsError: null })
    } catch (err) {
      set({ stats: null, statsError: errorMessage(err, '无法读取交易统计') })
    }
  },

  fetchEvents: async () => {
    set({ eventsError: null })
    try {
      const events = await apiGet<MacroEvent[]>('/api/events')
      if (!Array.isArray(events)) throw new Error('自建宏观事件返回格式无效')
      set({ events, eventsError: null })
    } catch (err) {
      set({ events: [], eventsError: errorMessage(err, '无法读取自建宏观事件') })
    }
  },

  addTrade: async (tradeData) => {
    await apiPost('/api/trades', { ...tradeData, environment: 'PAPER' })
    await Promise.all([get().fetchTrades(), get().fetchStats()])
  },

  addEvent: async (eventData) => {
    await apiPost('/api/events', eventData)
    await get().fetchEvents()
  },
}))
