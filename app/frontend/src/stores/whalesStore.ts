import { create } from 'zustand'
import { get as apiGet, getWithMeta as apiGetWithMeta, post as apiPost, type APIResponseMeta } from '../utils/api'
import type { CongressMember, Investor, PartyFilter, TradeTypeFilter } from '../types/whales'

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
  source: string
  filingDate: string
  sourceURL: string
  filingId: string
  verified: boolean
}

export interface ConsensusStock {
  symbol: string
  name: string
  guruCount: number
  avgWeight: number
  addCount: number
  trimCount: number
  reportPeriod: string
  source: string
  change?: string
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

  pageInvestors: Investor[]
  pageCongressMembers: CongressMember[]
  pageConsensusStocks: ConsensusStock[]
  investorsLoading: boolean
  investorsError: string
  investorsMeta: APIResponseMeta | null
  consensusLoading: boolean
  consensusError: string
  consensusMeta: APIResponseMeta | null
  congressLoading: boolean
  congressError: string
  congressMeta: APIResponseMeta | null
  pageCategory: string
  pageParty: PartyFilter
  pageTradeType: TradeTypeFilter
  
  fetchGurus: (type?: string) => Promise<void>
  fetchGuruDetails: (id: number | string) => Promise<void>
  fetchCongressTrades: (party?: string, type?: string) => Promise<void>
  fetchConsensus: () => Promise<void>
  fetchStockHolders: (symbol: string) => Promise<void>
  fetchSyncStatus: () => Promise<void>
  triggerSync: () => Promise<void>
  fetchPageInvestors: (category?: string) => Promise<void>
  fetchPageConsensus: (category?: string) => Promise<void>
  fetchPageCongress: (party?: PartyFilter, type?: TradeTypeFilter) => Promise<void>
  refreshWhalesPage: () => Promise<void>
  clearGuruDetails: () => void
  clearStockHolders: () => void
}

export const useWhalesStore = create<WhalesState>((set, get) => ({
  gurus: [],
  currentGuru: null,
  currentGuruHoldings: [],
  congressTrades: [],
  consensusStocks: [],
  currentStockHolders: [],
  syncing: false,
  lastSync: '',
  syncStatus: 'Idle',
  pageInvestors: [],
  pageCongressMembers: [],
  pageConsensusStocks: [],
  investorsLoading: false,
  investorsError: '',
  investorsMeta: null,
  consensusLoading: false,
  consensusError: '',
  consensusMeta: null,
  congressLoading: false,
  congressError: '',
  congressMeta: null,
  pageCategory: 'all',
  pageParty: 'all',
  pageTradeType: 'all',

  fetchGurus: async (type) => {
    try {
      const endpoint = type ? `/api/whales/gurus?type=${type}` : `/api/whales/gurus`
      const investors = await apiGet<Investor[]>(endpoint)
      set({ gurus: (investors || []).map((investor) => ({
        id: investor.id,
        name: investor.name,
        nameEn: investor.nameEn,
        fundName: investor.company,
        company: investor.company,
        slug: investor.slug,
        positionCount: investor.holdings,
        holdings: investor.holdings,
        topStock: investor.topStock?.symbol || '—',
        topStockSymbol: investor.topStock?.symbol,
        topStockWeight: investor.topStock?.percentage || 0,
        topStockPercentage: investor.topStock?.percentage,
        type: investor.type,
      })) })
    } catch (err) {
      console.error('Failed to fetch gurus:', err)
      set({ gurus: [] })
    }
  },

  fetchGuruDetails: async (id) => {
    try {
      const data = await apiGet<{ guru: Guru; holdings: Holding[] }>(`/api/whales/gurus/${id}`)
      set({
        currentGuru: data.guru,
        currentGuruHoldings: data.holdings || []
      })
    } catch (err) {
      console.error('Failed to fetch guru details:', err)
      set({ currentGuru: null, currentGuruHoldings: [] })
    }
  },

  fetchCongressTrades: async (party, type) => {
    try {
      let query = []
      if (party) query.push(`party=${party}`)
      if (type) query.push(`type=${type}`)
      const queryString = query.length > 0 ? `?${query.join('&')}` : ''
      const members = await apiGet<CongressMember[]>(`/api/whales/congress${queryString}`)
      set({ congressTrades: (members || []).filter((member) => member.latestTrade?.verified).map((member) => ({
        id: Number(member.id) || 0,
        politician: member.name,
        title: 'Representative',
        party: member.party === 'D' ? 'Democratic' : 'Republican',
        district: `${member.state}${member.district ? `-${member.district}` : ''}`,
        symbol: member.latestTrade.symbol,
        type: member.latestTrade.action === 'sell' ? 'SELL' : 'BUY',
        amount: member.latestTrade.amount,
        date: member.latestTrade.date,
        source: member.latestTrade.source,
        filingDate: member.latestTrade.filingDate,
        sourceURL: member.latestTrade.sourceURL,
        filingId: member.latestTrade.filingId,
        verified: true,
      })) })
    } catch (err) {
      console.error('Failed to fetch congress trades:', err)
      set({ congressTrades: [] })
    }
  },

  fetchConsensus: async () => {
    try {
      const consensus = await apiGet<ConsensusStock[]>(`/api/whales/consensus`)
      set({ consensusStocks: consensus || [] })
    } catch (err) {
      console.error('Failed to fetch consensus:', err)
      set({ consensusStocks: [] })
    }
  },

  fetchStockHolders: async (symbol) => {
    try {
      const holders = await apiGet<StockHolder[]>(`/api/whales/stock/${symbol}`)
      set({ currentStockHolders: holders || [] })
    } catch (err) {
      console.error('Failed to fetch stock holders:', err)
      set({ currentStockHolders: [] })
    }
  },

  fetchSyncStatus: async () => {
    try {
      const status = await apiGet<{ syncing: boolean; lastSync?: string; syncStatus?: string }>(`/api/whales/status`)
      set({
        syncing: status.syncing,
        lastSync: status.lastSync || '',
        syncStatus: status.syncStatus || 'Idle'
      })
    } catch (err) {
      console.error('Failed to fetch sync status:', err)
      set({ syncing: false, syncStatus: 'Unavailable' })
    }
  },

  triggerSync: async () => {
    try {
      await apiPost(`/api/whales/sync`)
      set({ syncing: true, syncStatus: 'Syncing via EDGAR...' })
    } catch (err) {
      console.error('Failed to trigger sync:', err)
      set({ syncing: false, syncStatus: 'Sync failed' })
    }
  },

  fetchPageInvestors: async (category = 'all') => {
    set({ pageCategory: category, investorsLoading: true, investorsError: '' })
    const endpoint = category === 'all' ? '/api/whales/gurus' : `/api/whales/gurus?type=${encodeURIComponent(category)}`
    try {
      const result = await apiGetWithMeta<Investor[]>(endpoint)
      if (get().pageCategory !== category) return
      set({ pageInvestors: result.data || [], investorsMeta: result.meta, investorsLoading: false, investorsError: '' })
    } catch (error) {
      if (get().pageCategory !== category) return
      set({
        pageInvestors: [], investorsMeta: null, investorsLoading: false,
        investorsError: error instanceof Error ? error.message : '机构披露加载失败',
      })
    }
  },

  fetchPageConsensus: async (category = 'all') => {
    set({ pageCategory: category, consensusLoading: true, consensusError: '' })
    const type = category === 'all' ? 'us_gurus' : category
    try {
      const result = await apiGetWithMeta<ConsensusStock[]>(`/api/whales/consensus?type=${encodeURIComponent(type)}`)
      if (get().pageCategory !== category) return
      set({ pageConsensusStocks: result.data || [], consensusMeta: result.meta, consensusLoading: false, consensusError: '' })
    } catch (error) {
      if (get().pageCategory !== category) return
      set({
        pageConsensusStocks: [], consensusMeta: null, consensusLoading: false,
        consensusError: error instanceof Error ? error.message : '共识数据加载失败',
      })
    }
  },

  fetchPageCongress: async (party = 'all', type = 'all') => {
    set({ pageParty: party, pageTradeType: type, congressLoading: true, congressError: '' })
    const params = new URLSearchParams()
    if (party !== 'all') params.set('party', party)
    if (type !== 'all') params.set('type', type)
    const query = params.toString()
    try {
      const result = await apiGetWithMeta<CongressMember[]>(`/api/whales/congress${query ? `?${query}` : ''}`)
      const state = get()
      if (state.pageParty !== party || state.pageTradeType !== type) return
      set({ pageCongressMembers: result.data || [], congressMeta: result.meta, congressLoading: false, congressError: '' })
    } catch (error) {
      const state = get()
      if (state.pageParty !== party || state.pageTradeType !== type) return
      set({
        pageCongressMembers: [], congressMeta: null, congressLoading: false,
        congressError: error instanceof Error ? error.message : '国会披露加载失败',
      })
    }
  },

  refreshWhalesPage: async () => {
    const state = get()
    await Promise.all([
      state.fetchPageInvestors(state.pageCategory),
      state.fetchPageConsensus(state.pageCategory),
      state.fetchPageCongress(state.pageParty, state.pageTradeType),
    ])
  },

  clearGuruDetails: () => set({ currentGuru: null, currentGuruHoldings: [] }),
  clearStockHolders: () => set({ currentStockHolders: [] })
}))
