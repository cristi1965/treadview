import { create } from 'zustand'
import { API_BASE_URL } from '../utils/api'

export interface Trade {
  id: number
  createdAt: string
  symbol: string
  direction: 'BUY' | 'SELL'
  entryPrice: number
  exitPrice: number
  shares: number
  pnl: number
  emotionScore: number
  notes: string
}

export interface Stats {
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
  fetchTrades: () => Promise<void>
  fetchStats: () => Promise<void>
  fetchEvents: () => Promise<void>
  addTrade: (trade: Omit<Trade, 'id' | 'createdAt'>) => Promise<void>
  addEvent: (event: Omit<MacroEvent, 'id'>) => Promise<void>
}

const API_BASE = `${API_BASE_URL}/api`

export const useCommandStore = create<CommandState>((set, get) => ({
  trades: [],
  stats: null,
  events: [],

  fetchTrades: async () => {
    try {
      const resp = await fetch(`${API_BASE}/trades`)
      if (resp.ok) {
        const trades = await resp.json()
        set({ trades: trades || [] })
      }
    } catch (err) {
      console.error('Failed to fetch trades:', err)
    }
  },

  fetchStats: async () => {
    try {
      const resp = await fetch(`${API_BASE}/stats`)
      if (resp.ok) {
        const stats = await resp.json()
        set({ stats })
      }
    } catch (err) {
      console.error('Failed to fetch stats:', err)
    }
  },

  fetchEvents: async () => {
    try {
      const resp = await fetch(`${API_BASE}/events`)
      if (resp.ok) {
        const events = await resp.json()
        set({ events: events || [] })
      }
    } catch (err) {
      console.error('Failed to fetch events:', err)
    }
  },

  addTrade: async (tradeData) => {
    try {
      const resp = await fetch(`${API_BASE}/trades`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(tradeData),
      })
      if (resp.ok) {
        await get().fetchTrades()
        await get().fetchStats()
      }
    } catch (err) {
      console.error('Failed to add trade:', err)
    }
  },

  addEvent: async (eventData) => {
    try {
      const resp = await fetch(`${API_BASE}/events`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(eventData),
      })
      if (resp.ok) {
        await get().fetchEvents()
      }
    } catch (err) {
      console.error('Failed to add event:', err)
    }
  },
}))
