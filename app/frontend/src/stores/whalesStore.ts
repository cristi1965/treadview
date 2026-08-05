import { create } from 'zustand'
import { API_BASE_URL } from '../utils/api'

// 新的类型定义 - 兼容原有和新的 Whales 页面
export interface Guru {
  id: number | string
  name: string
  nameEn?: string
  title?: string
  fundName: string
  company?: string
  slug?: string
  aum?: string
  positionCount: number
  holdings?: number
  topStock: string
  topStockSymbol?: string
  topStockWeight: number
  topStockPercentage?: number
  avatarCode?: string
  avatar?: string
  type?: string
}

export interface Holding {
  id: number
  guruId: number
  stockSymbol: string
  stockName: string
  value: string
  shares: string
  change: string
  weight: number
}

export interface CongressTrade {
  id: number
  politician: string
  title: string
  party: 'Democratic' | 'Republican'
  district: string
  symbol: string
  type: 'BUY' | 'SELL'
  amount: string
  date: string
}

export interface ConsensusStock {
  symbol: string
  name: string
  guruCount: number
  avgWeight: number
  change: string
}

export interface StockHolder {
  guruName: string
  fundName: string
  value: string
  shares: string
  change: string
  weight: number
  avatarCode: string
}

interface WhalesState {
  gurus: Guru[]
  currentGuru: Guru | null
  currentGuruHoldings: Holding[]
  congressTrades: CongressTrade[]
  consensusStocks: ConsensusStock[]
  currentStockHolders: StockHolder[]
  syncing: boolean
  lastSync: string
  syncStatus: string
  
  fetchGurus: (type?: string) => Promise<void>
  fetchGuruDetails: (id: number) => Promise<void>
  fetchCongressTrades: (party?: string, type?: string) => Promise<void>
  fetchConsensus: () => Promise<void>
  fetchStockHolders: (symbol: string) => Promise<void>
  fetchSyncStatus: () => Promise<void>
  triggerSync: () => Promise<void>
  clearGuruDetails: () => void
  clearStockHolders: () => void
}

const API_BASE = `${API_BASE_URL}/api/whales`

export const useWhalesStore = create<WhalesState>((set) => ({
  gurus: [],
  currentGuru: null,
  currentGuruHoldings: [],
  congressTrades: [],
  consensusStocks: [],
  currentStockHolders: [],
  syncing: false,
  lastSync: '',
  syncStatus: 'Idle',

  fetchGurus: async (type) => {
    try {
      const url = type ? `${API_BASE}/gurus?type=${type}` : `${API_BASE}/gurus`
      const resp = await fetch(url)
      if (resp.ok) {
        const gurus = await resp.json()
        set({ gurus: gurus || [] })
      }
    } catch (err) {
      console.error('Failed to fetch gurus:', err)
    }
  },

  fetchGuruDetails: async (id) => {
    try {
      const resp = await fetch(`${API_BASE}/gurus/${id}`)
      if (resp.ok) {
        const data = await resp.json()
        set({
          currentGuru: data.guru,
          currentGuruHoldings: data.holdings || []
        })
      }
    } catch (err) {
      console.error('Failed to fetch guru details:', err)
    }
  },

  fetchCongressTrades: async (party, type) => {
    try {
      let query = []
      if (party) query.push(`party=${party}`)
      if (type) query.push(`type=${type}`)
      const queryString = query.length > 0 ? `?${query.join('&')}` : ''
      const resp = await fetch(`${API_BASE}/congress${queryString}`)
      if (resp.ok) {
        const trades = await resp.json()
        set({ congressTrades: trades || [] })
      }
    } catch (err) {
      console.error('Failed to fetch congress trades:', err)
    }
  },

  fetchConsensus: async () => {
    try {
      const resp = await fetch(`${API_BASE}/consensus`)
      if (resp.ok) {
        const consensus = await resp.json()
        set({ consensusStocks: consensus || [] })
      }
    } catch (err) {
      console.error('Failed to fetch consensus:', err)
    }
  },

  fetchStockHolders: async (symbol) => {
    try {
      const resp = await fetch(`${API_BASE}/stock/${symbol}`)
      if (resp.ok) {
        const holders = await resp.json()
        set({ currentStockHolders: holders || [] })
      }
    } catch (err) {
      console.error('Failed to fetch stock holders:', err)
    }
  },

  fetchSyncStatus: async () => {
    try {
      const resp = await fetch(`${API_BASE}/status`)
      if (resp.ok) {
        const status = await resp.json()
        set({
          syncing: status.syncing,
          lastSync: status.lastSync || '',
          syncStatus: status.syncStatus || 'Idle'
        })
      }
    } catch (err) {
      console.error('Failed to fetch sync status:', err)
    }
  },

  triggerSync: async () => {
    try {
      const resp = await fetch(`${API_BASE}/sync`, { method: 'POST' })
      if (resp.ok) {
        set({ syncing: true, syncStatus: 'Syncing via EDGAR...' })
      }
    } catch (err) {
      console.error('Failed to trigger sync:', err)
    }
  },

  clearGuruDetails: () => set({ currentGuru: null, currentGuruHoldings: [] }),
  clearStockHolders: () => set({ currentStockHolders: [] })
}))
