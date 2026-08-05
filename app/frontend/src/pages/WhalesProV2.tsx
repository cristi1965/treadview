import React, { useState, useEffect } from 'react'
import { useWhalesStore } from '../stores/whalesStore'
import { 
  Search, Menu, Users, Building2, ChevronRight, TrendingUp,
  Eye, Flame, Landmark
} from 'lucide-react'

// Lucide Waves Horizontal Icon SVG
const WavesIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" style={{ width: '20px', height: '20px' }}>
    <path d="M2 12q2.5 2 5 0t5 0 5 0 5 0"></path>
    <path d="M2 19q2.5 2 5 0t5 0 5 0 5 0"></path>
    <path d="M2 5q2.5 2 5 0t5 0 5 0 5 0"></path>
  </svg>
)

export const WhalesProV2: React.FC = () => {
  const { 
    gurus, consensusStocks, congressTrades,
    fetchGurus, fetchConsensus, fetchCongressTrades,
    triggerSync
  } = useWhalesStore()

  // 状态
  const [activeTab, setActiveTab] = useState<'institutions' | 'congress'>('institutions')
  const [categoryFilter, setCategoryFilter] = useState<'all' | 'us' | 'a_share' | 'private' | 'hot_money'>('all')
  const [searchQuery, setSearchQuery] = useState('')

  useEffect(() => {
    fetchConsensus()
    fetchGurus()
    fetchCongressTrades()
    triggerSync()
  }, [])

  // 过滤机构
  const filteredGurus = gurus.filter(guru => {
    const matchesSearch = guru.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
                         guru.topStock.toLowerCase().includes(searchQuery.toLowerCase())
    if (categoryFilter === 'all') return matchesSearch
    return matchesSearch && guru.type === categoryFilter
  })

  // 获取首字母
  const getInitials = (name: string) => {
    const parts = name.split(' ')
    if (parts.length >= 2) {
      return parts[0].charAt(0) + parts[parts.length - 1].charAt(0)
    }
    return name.substring(0, 2).toUpperCase()
  }

  return (
    <div style={{
      minHeight: '100vh',
      backgroundColor: '#0a0a0a',
      color: 'white',
      padding: '2rem 1rem'
    }}>
      <div style={{ maxWidth: '1400px', margin: '0 auto' }}>
        {/* Header */}
        <header style={{ marginBottom: '2rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '0.5rem' }}>
            <WavesIcon />
            <h1 style={{ fontSize: '1.5rem', fontWeight: 600, letterSpacing: '-0.02em' }}>
              聪明钱
            </h1>
          </div>
          <p style={{ fontSize: '0.875rem', color: '#888', marginTop: '0.5rem' }}>
            关注仓位占比与季度变动 —— 重仓、建仓与减持代表不同含义，而非仅看是否持有。
          </p>
        </header>

        {/* Tab Switch */}
        <div style={{
          display: 'inline-flex',
          borderRadius: '0.5rem',
          border: '1px solid rgba(255,255,255,0.08)',
          backgroundColor: '#151515',
          padding: '0.25rem',
          marginBottom: '1.5rem'
        }}>
          <button
            onClick={() => setActiveTab('institutions')}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '0.5rem',
              borderRadius: '0.375rem',
              padding: '0.5rem 1rem',
              fontSize: '0.8125rem',
              fontWeight: 500,
              transition: 'all 0.2s',
              backgroundColor: activeTab === 'institutions' ? '#1f1f1f' : 'transparent',
              color: activeTab === 'institutions' ? 'white' : '#999',
              border: 'none',
              cursor: 'pointer'
            }}
          >
            <Building2 size={16} />
            机构 13F
          </button>
          <button
            onClick={() => setActiveTab('congress')}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '0.5rem',
              borderRadius: '0.375rem',
              padding: '0.5rem 1rem',
              fontSize: '0.8125rem',
              fontWeight: 500,
              transition: 'all 0.2s',
              backgroundColor: activeTab === 'congress' ? '#1f1f1f' : 'transparent',
              color: activeTab === 'congress' ? 'white' : '#999',
              border: 'none',
              cursor: 'pointer'
            }}
          >
            <Landmark size={16} />
            国会 · {congressTrades.length}
          </button>
        </div>


        {/* Content placeholder */}
        <div style={{
          border: '1px solid rgba(255,255,255,0.08)',
          borderRadius: '0.75rem',
          padding: '3rem',
          textAlign: 'center',
          backgroundColor: '#151515'
        }}>
          <p style={{ color: '#666', fontSize: '0.875rem' }}>
            {activeTab === 'institutions' ? '机构持仓数据加载中...' : '国会交易数据加载中...'}
          </p>
        </div>
      </div>
    </div>
  )
}
