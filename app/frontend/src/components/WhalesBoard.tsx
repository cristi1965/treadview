import React, { useState, useEffect, useRef } from 'react'
import { useWhalesStore } from '../stores/whalesStore'
import { Avatar } from './Avatar'
import {
  Search, Users, Landmark, UserCheck,
  X, Filter, ShieldAlert
} from 'lucide-react'

export const WhalesBoard: React.FC = () => {
  const {
    gurus, currentGuru, currentGuruHoldings, congressTrades, consensusStocks, currentStockHolders,
    syncing, lastSync,
    fetchGurus, fetchGuruDetails, fetchCongressTrades, fetchConsensus, fetchStockHolders,
    fetchSyncStatus, clearGuruDetails
  } = useWhalesStore()

  // Tab State
  const [whalesTab, setWhalesTab] = useState<'13f' | 'congress'>('13f')

  // Guru category filter
  const [guruFilter, setGuruFilter] = useState<'all' | 'superinvestor' | 'fund' | 'private_fund' | 'hot_money'>('all')

  // Congress filters
  const [partyFilter, setPartyFilter] = useState<'all' | 'Democratic' | 'Republican'>('all')
  const [typeFilter, setTypeFilter] = useState<'all' | 'BUY' | 'SELL'>('all')

  // Search State
  const [searchQuery, setSearchQuery] = useState('')
  const [showSuggestions, setShowSuggestions] = useState(false)
  const [selectedStockSymbol, setSelectedStockSymbol] = useState<string | null>(null)

  const searchRef = useRef<HTMLDivElement>(null)

  // Major symbols for search autocomplete
  const popularSymbols = [
    { symbol: 'AAPL', name: 'Apple Inc.' },
    { symbol: 'MSFT', name: 'Microsoft Corp.' },
    { symbol: 'NVDA', name: 'NVIDIA Corp.' },
    { symbol: 'TSLA', name: 'Tesla Inc.' },
    { symbol: 'AMD', name: 'Advanced Micro Devices' },
    { symbol: 'BAC', name: 'Bank of America' },
    { symbol: 'AXP', name: 'American Express' },
    { symbol: 'KO', name: 'Coca-Cola Co.' },
    { symbol: 'CVX', name: 'Chevron Corp.' },
    { symbol: 'COIN', name: 'Coinbase Global' },
    { symbol: 'ROKU', name: 'Roku Inc.' },
    { symbol: '000858.SZ', name: '五粮液' },
    { symbol: '600519.SS', name: '贵州茅台' },
    { symbol: '00700.HK', name: '腾讯控股' },
    { symbol: '300760.SZ', name: '迈瑞医疗' }
  ]

  // Filter suggestion based on search query
  const suggestions = popularSymbols.filter(item =>
    item.symbol.toLowerCase().includes(searchQuery.toLowerCase()) ||
    item.name.toLowerCase().includes(searchQuery.toLowerCase())
  )

  useEffect(() => {
    fetchGurus(guruFilter === 'all' ? undefined : guruFilter)
  }, [guruFilter])

  useEffect(() => {
    fetchCongressTrades(
      partyFilter === 'all' ? undefined : partyFilter,
      typeFilter === 'all' ? undefined : typeFilter
    )
  }, [partyFilter, typeFilter])

  // Initial read-only setup. Administrative sync is explicit in Settings.
  useEffect(() => {
    fetchConsensus()
    fetchSyncStatus()
  }, [])

  // 2. Periodic sync status and data polling every 20 seconds
  useEffect(() => {
    const interval = setInterval(() => {
      fetchSyncStatus()
      fetchGurus(guruFilter === 'all' ? undefined : guruFilter)
      fetchCongressTrades(
        partyFilter === 'all' ? undefined : partyFilter,
        typeFilter === 'all' ? undefined : typeFilter
      )
      fetchConsensus()
    }, 20000)

    return () => clearInterval(interval)
  }, [guruFilter, partyFilter, typeFilter])

  // Close suggestions dropdown on click outside
  useEffect(() => {
    const handleOutsideClick = (e: MouseEvent) => {
      if (searchRef.current && !searchRef.current.contains(e.target as Node)) {
        setShowSuggestions(false)
      }
    }
    document.addEventListener('mousedown', handleOutsideClick)
    return () => document.removeEventListener('mousedown', handleOutsideClick)
  }, [])

  const handleStockClick = (symbol: string) => {
    setSelectedStockSymbol(symbol)
    fetchStockHolders(symbol)
    setShowSuggestions(false)
    setSearchQuery('')
  }

  const getPartyLabel = (party: string) => {
    return party === 'Democratic' ? '民主党' : '共和党'
  }

  const getPartyColor = (party: string) => {
    return party === 'Democratic' ? '#3b82f6' : '#ef4444' // Blue / Red
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
      
      {/* Search Header Row */}
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        flexWrap: 'wrap',
        gap: '1.5rem',
        backgroundColor: 'rgba(21, 28, 51, 0.4)',
        border: '1px solid var(--border-color)',
        padding: '1.25rem 1.5rem',
        borderRadius: '1rem',
        backdropFilter: 'blur(8px)'
      }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flexWrap: 'wrap' }}>
            <span style={{ fontSize: '1.5rem' }}>🐋</span>
            <h2 style={{ fontSize: '1.3rem', fontWeight: 700, color: 'white', margin: 0 }}>巨头追踪 & 聪明钱共识</h2>
            
            {syncing ? (
              <span style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '0.35rem',
                fontSize: '0.7rem',
                padding: '0.15rem 0.5rem',
                borderRadius: '9999px',
                backgroundColor: 'rgba(234, 179, 8, 0.1)',
                border: '1px solid rgba(234, 179, 8, 0.25)',
                color: '#eab308',
                fontWeight: 600
              }}>
                <span style={{
                  width: '6px',
                  height: '6px',
                  borderRadius: '50%',
                  backgroundColor: '#eab308',
                  boxShadow: '0 0 6px #eab308'
                }} />
                更新中...
              </span>
            ) : (
              <span style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '0.35rem',
                fontSize: '0.7rem',
                padding: '0.15rem 0.5rem',
                borderRadius: '9999px',
                backgroundColor: 'rgba(34, 197, 94, 0.06)',
                border: '1px solid rgba(34, 197, 94, 0.15)',
                color: '#22c55e',
                fontWeight: 500
              }}>
                ✓ 披露快照已加载 {lastSync && `(同步: ${lastSync.split(' ')[1] || lastSync})`}
              </span>
            )}
          </div>
          <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
            按披露期跟踪机构 13F 与可核验国会交易记录，不是实时交易信号
          </p>
        </div>

        {/* Autocomplete Search Bar */}
        <div ref={searchRef} style={{ position: 'relative', width: '320px' }}>
          <div style={{ position: 'relative' }}>
            <Search size={16} style={{
              position: 'absolute',
              left: '12px',
              top: '50%',
              transform: 'translateY(-50%)',
              color: 'var(--text-muted)'
            }} />
            <input
              type="text"
              placeholder="搜索股票代码以查看谁持有该股..."
              value={searchQuery}
              onChange={(e) => {
                setSearchQuery(e.target.value)
                setShowSuggestions(true)
              }}
              onFocus={() => setShowSuggestions(true)}
              className="text-input"
              style={{
                width: '100%',
                paddingLeft: '2.25rem',
                fontSize: '0.85rem',
                borderRadius: '9999px',
                borderColor: showSuggestions ? '#f97316' : 'var(--border-color)',
                outline: 'none',
                boxShadow: showSuggestions ? '0 0 10px rgba(249, 115, 22, 0.15)' : 'none'
              }}
            />
          </div>

          {/* Suggestions Dropdown */}
          {showSuggestions && suggestions.length > 0 && (
            <div style={{
              position: 'absolute',
              top: '110%',
              left: 0,
              right: 0,
              backgroundColor: '#0d1222',
              border: '1px solid var(--border-color)',
              borderRadius: '0.75rem',
              boxShadow: '0 10px 25px rgba(0,0,0,0.5)',
              zIndex: 1000,
              maxHeight: '220px',
              overflowY: 'auto',
              padding: '0.5rem 0'
            }}>
              {suggestions.map((item, index) => (
                <div
                  key={index}
                  onClick={() => handleStockClick(item.symbol)}
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    padding: '0.6rem 1rem',
                    cursor: 'pointer',
                    fontSize: '0.85rem',
                    color: 'var(--text-primary)',
                    transition: 'background-color 0.15s ease'
                  }}
                  onMouseEnter={(e) => e.currentTarget.style.backgroundColor = 'rgba(249, 115, 22, 0.1)'}
                  onMouseLeave={(e) => e.currentTarget.style.backgroundColor = 'transparent'}
                >
                  <span style={{ fontWeight: 700, fontFamily: 'var(--font-mono)', color: '#f97316' }}>{item.symbol}</span>
                  <span style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>{item.name}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Primary Whales Tabs */}
      <div style={{ display: 'flex', gap: '1rem', borderBottom: '1px solid var(--border-color)', paddingBottom: '0.25rem' }}>
        <button
          onClick={() => setWhalesTab('13f')}
          style={{
            background: 'none',
            border: 'none',
            borderBottom: whalesTab === '13f' ? '3px solid #f97316' : '3px solid transparent',
            color: whalesTab === '13f' ? '#f97316' : 'var(--text-secondary)',
            fontWeight: whalesTab === '13f' ? 700 : 500,
            padding: '0.5rem 1rem',
            fontSize: '0.95rem',
            cursor: 'pointer',
            transition: 'all 0.15s ease'
          }}
        >
          机构持仓 (13F Filings)
        </button>
        <button
          onClick={() => setWhalesTab('congress')}
          style={{
            background: 'none',
            border: 'none',
            borderBottom: whalesTab === 'congress' ? '3px solid #f97316' : '3px solid transparent',
            color: whalesTab === 'congress' ? '#f97316' : 'var(--text-secondary)',
            fontWeight: whalesTab === 'congress' ? 700 : 500,
            padding: '0.5rem 1rem',
            fontSize: '0.95rem',
            cursor: 'pointer',
            transition: 'all 0.15s ease'
          }}
        >
          国会交易 (Congress Trades)
        </button>
      </div>

      {/* Institutional 13F View */}
      {whalesTab === '13f' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
          
          {/* Gurus Filter & Grid */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', flexWrap: 'wrap' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                <Filter size={14} /> 巨头类型:
              </div>
              <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
                {['all', 'superinvestor', 'fund', 'private_fund', 'hot_money'].map((filter) => (
                  <button
                    key={filter}
                    onClick={() => setGuruFilter(filter as any)}
                    style={{
                      padding: '0.3rem 0.8rem',
                      borderRadius: '9999px',
                      fontSize: '0.8rem',
                      fontWeight: 600,
                      border: '1px solid',
                      cursor: 'pointer',
                      backgroundColor: guruFilter === filter ? 'rgba(249, 115, 22, 0.15)' : 'rgba(255,255,255,0.02)',
                      borderColor: guruFilter === filter ? '#f97316' : 'var(--border-color)',
                      color: guruFilter === filter ? '#f97316' : 'var(--text-secondary)',
                      transition: 'all 0.15s ease',
                      marginTop: '0.25rem'
                    }}
                  >
                    {filter === 'all' && '全部'}
                    {filter === 'superinvestor' && '美股大佬'}
                    {filter === 'fund' && 'A股顶流'}
                    {filter === 'private_fund' && '私募大佬'}
                    {filter === 'hot_money' && '游资席位'}
                  </button>
                ))}
              </div>
            </div>

            {/* Gurus Grid Card */}
            <div style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
              gap: '1.25rem'
            }}>
              {gurus.map((guru) => (
                <div
                  key={guru.id}
                  onClick={() => fetchGuruDetails(guru.id)}
                  className="card"
                  style={{
                    cursor: 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '1.25rem',
                    position: 'relative',
                    overflow: 'hidden',
                    borderColor: 'var(--border-color)',
                    transition: 'all 0.2s ease'
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.borderColor = '#f97316'
                    e.currentTarget.style.boxShadow = '0 0 15px rgba(249, 115, 22, 0.08)'
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.borderColor = 'var(--border-color)'
                    e.currentTarget.style.boxShadow = 'var(--shadow-lg)'
                  }}
                >
                  {/* Circular Avatar */}
                  <Avatar
                    name={guru.name}
                    slug={guru.slug || String(guru.id)}
                    letter={guru.avatarCode}
                    size={48}
                  />

                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.2rem', overflow: 'hidden' }}>
                    <h4 style={{ fontSize: '1rem', fontWeight: 700, color: 'white', whiteSpace: 'nowrap', textOverflow: 'ellipsis', overflow: 'hidden' }}>
                      {guru.name}
                    </h4>
                    <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', display: 'block' }}>
                      {guru.fundName} · {guru.title}
                    </span>
                    <div style={{ display: 'flex', gap: '0.75rem', fontSize: '0.75rem', marginTop: '0.2rem', color: 'var(--text-muted)' }}>
                      <span>持仓数: <strong style={{ color: 'white' }}>{guru.positionCount}</strong></span>
                      <span>第一重仓: <strong style={{ color: '#f97316' }}>{guru.topStock}</strong></span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Smart Money Consensus */}
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))',
            gap: '1.5rem'
          }}>
            {/* Left side: Guru consensus rankings */}
            <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
              <div style={{ display: 'flex', alignItems: 'center', justifySelf: 'start', gap: '0.5rem', borderBottom: '1px solid var(--border-color)', paddingBottom: '0.5rem', width: '100%' }}>
                <Users size={16} style={{ color: '#f97316' }} />
                <h3 style={{ fontSize: '1rem', fontWeight: 700, color: 'white' }}>聪明钱共识：巨头最爱重仓股排行</h3>
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
                {consensusStocks.map((item, index) => (
                  <div
                    key={index}
                    onClick={() => handleStockClick(item.symbol)}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      padding: '0.75rem',
                      backgroundColor: 'rgba(255,255,255,0.01)',
                      border: '1px solid var(--border-color)',
                      borderRadius: '0.75rem',
                      cursor: 'pointer',
                      transition: 'border-color 0.15s ease'
                    }}
                    onMouseEnter={(e) => e.currentTarget.style.borderColor = '#f97316'}
                    onMouseLeave={(e) => e.currentTarget.style.borderColor = 'var(--border-color)'}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
                      <span style={{ fontSize: '0.85rem', fontWeight: 800, color: 'var(--text-muted)', width: '16px' }}>
                        {index + 1}
                      </span>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.15rem' }}>
                        <span style={{ fontWeight: 700, fontFamily: 'var(--font-mono)', color: 'white' }}>{item.symbol}</span>
                        <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>{item.name}</span>
                      </div>
                    </div>

                    <div style={{ display: 'flex', alignItems: 'center', gap: '1.5rem', fontSize: '0.8rem' }}>
                      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: '0.2rem' }}>
                        <span style={{ color: 'var(--text-secondary)' }}>共识度 ({item.guruCount} 巨头)</span>
                        {/* Progress Bar representation */}
                        <div style={{ width: '80px', height: '5px', backgroundColor: 'rgba(255,255,255,0.05)', borderRadius: '9999px', overflow: 'hidden' }}>
                          <div style={{
                            width: `${(item.guruCount / 8) * 100}%`,
                            height: '100%',
                            backgroundColor: '#f97316',
                            borderRadius: '9999px'
                          }} />
                        </div>
                      </div>

                      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', minWidth: '60px' }}>
                        <span style={{ fontFamily: 'var(--font-mono)', fontWeight: 700, color: 'white' }}>{item.avgWeight.toFixed(1)}%</span>
                        <span style={{
                          fontSize: '0.7rem',
                          fontWeight: 600,
                          color: item.change.startsWith('+') ? 'var(--color-success)' : 'var(--color-danger)'
                        }}>{item.change}</span>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            {/* Right side: Stats Analysis */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
              <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <Landmark size={18} style={{ color: 'var(--color-info)' }} />
                  <span style={{ fontSize: '0.9rem', color: 'var(--text-secondary)' }}>大仓位聪明钱概况</span>
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem', marginTop: '0.5rem' }}>
                  <div style={{ display: 'flex', justifySelf: 'start', justifyContent: 'space-between', width: '100%', fontSize: '0.85rem' }}>
                    <span style={{ color: 'var(--text-secondary)' }}>已追踪机构 13F 资金规模:</span>
                    <strong style={{ color: 'white' }}>超过 $450B USD</strong>
                  </div>
                  <div style={{ display: 'flex', justifySelf: 'start', justifyContent: 'space-between', width: '100%', fontSize: '0.85rem' }}>
                    <span style={{ color: 'var(--text-secondary)' }}>顶级重仓行业共识:</span>
                    <strong style={{ color: 'var(--color-info)' }}>科技 / 半导体核心</strong>
                  </div>
                  <div style={{ display: 'flex', justifySelf: 'start', justifyContent: 'space-between', width: '100%', fontSize: '0.85rem' }}>
                    <span style={{ color: 'var(--text-secondary)' }}>平均季度换手率:</span>
                    <strong style={{ color: 'white' }}>4.8% (偏向长期主义价值投资)</strong>
                  </div>
                </div>
              </div>

              <div className="card" style={{ display: 'flex', gap: '0.75rem', alignItems: 'start', backgroundColor: 'rgba(249, 115, 22, 0.03)', borderColor: 'rgba(249, 115, 22, 0.15)' }}>
                <ShieldAlert size={18} style={{ color: '#f97316', flexShrink: 0, marginTop: '2px' }} />
                <div>
                  <h4 style={{ fontSize: '0.85rem', fontWeight: 750, color: '#f97316' }}>机构 13F 披露警示</h4>
                  <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', lineHeight: 1.5, marginTop: '0.25rem' }}>
                    13F 持仓报告通常在每个季度结束后的 45 天内提交披露（如每季度末），反映的是历史持仓数据，不含空头头寸及衍生品对冲交易。仅供趋势研究参考，切忌盲目跟风买入。
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Congress Trading View */}
      {whalesTab === 'congress' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
          
          {/* Congress Filters */}
          <div style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            flexWrap: 'wrap',
            gap: '1rem',
            backgroundColor: 'rgba(255,255,255,0.01)',
            border: '1px solid var(--border-color)',
            padding: '0.75rem 1rem',
            borderRadius: '0.75rem'
          }}>
            <div style={{ display: 'flex', gap: '1.5rem', alignItems: 'center' }}>
              <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
                <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>党派选择:</span>
                {['all', 'Democratic', 'Republican'].map((party) => (
                  <button
                    key={party}
                    onClick={() => setPartyFilter(party as any)}
                    style={{
                      padding: '0.25rem 0.6rem',
                      borderRadius: '4px',
                      fontSize: '0.75rem',
                      fontWeight: 600,
                      border: '1px solid',
                      cursor: 'pointer',
                      backgroundColor: partyFilter === party
                        ? (party === 'Democratic' ? 'rgba(59, 130, 246, 0.15)' : party === 'Republican' ? 'rgba(239, 68, 68, 0.15)' : 'rgba(249, 115, 22, 0.15)')
                        : 'rgba(255,255,255,0.02)',
                      borderColor: partyFilter === party
                        ? (party === 'Democratic' ? '#3b82f6' : party === 'Republican' ? '#ef4444' : '#f97316')
                        : 'var(--border-color)',
                      color: partyFilter === party
                        ? (party === 'Democratic' ? '#3b82f6' : party === 'Republican' ? '#ef4444' : '#f97316')
                        : 'var(--text-secondary)',
                      transition: 'all 0.15s ease'
                    }}
                  >
                    {party === 'all' && '全部党派'}
                    {party === 'Democratic' && '民主党 (D)'}
                    {party === 'Republican' && '共和党 (R)'}
                  </button>
                ))}
              </div>

              <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
                <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>交易方向:</span>
                {['all', 'BUY', 'SELL'].map((type) => (
                  <button
                    key={type}
                    onClick={() => setTypeFilter(type as any)}
                    style={{
                      padding: '0.25rem 0.6rem',
                      borderRadius: '4px',
                      fontSize: '0.75rem',
                      fontWeight: 600,
                      border: '1px solid',
                      cursor: 'pointer',
                      backgroundColor: typeFilter === type
                        ? (type === 'BUY' ? 'rgba(16, 185, 129, 0.15)' : 'rgba(244, 63, 94, 0.15)')
                        : 'rgba(255,255,255,0.02)',
                      borderColor: typeFilter === type
                        ? (type === 'BUY' ? 'var(--color-success)' : 'var(--color-danger)')
                        : 'var(--border-color)',
                      color: typeFilter === type
                        ? (type === 'BUY' ? 'var(--color-success)' : 'var(--color-danger)')
                        : 'var(--text-secondary)',
                      transition: 'all 0.15s ease'
                    }}
                  >
                    {type === 'all' && '全部方向'}
                    {type === 'BUY' && '买入'}
                    {type === 'SELL' && '卖出'}
                  </button>
                ))}
              </div>
            </div>

            <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
              <UserCheck size={14} style={{ color: 'var(--color-warning)' }} />
              已追踪 96 位美国国会议员近期公开呈报交易
            </div>
          </div>

          {/* Congress Table Card */}
          <div className="card" style={{ padding: '1rem' }}>
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid var(--border-color)', color: 'var(--text-secondary)', textAlign: 'left' }}>
                    <th style={{ padding: '0.75rem' }}>议员</th>
                    <th style={{ padding: '0.75rem' }}>党派地区</th>
                    <th style={{ padding: '0.75rem' }}>交易股票</th>
                    <th style={{ padding: '0.75rem' }}>动作</th>
                    <th style={{ padding: '0.75rem' }}>交易额区间</th>
                    <th style={{ padding: '0.75rem' }}>呈报交易日期</th>
                  </tr>
                </thead>
                <tbody>
                  {congressTrades.map((trade) => (
                    <tr key={trade.id} style={{ borderBottom: '1px solid rgba(255,255,255,0.02)', color: 'var(--text-primary)' }}>
                      <td style={{ padding: '0.75rem', fontWeight: 600 }}>{trade.politician}</td>
                      <td style={{ padding: '0.75rem' }}>
                        <span style={{
                          color: getPartyColor(trade.party),
                          fontWeight: 'bold',
                          marginRight: '0.4rem'
                        }}>
                          {getPartyLabel(trade.party)}
                        </span>
                        <span style={{ color: 'var(--text-secondary)', fontSize: '0.8rem' }}>{trade.district}</span>
                      </td>
                      <td style={{
                        padding: '0.75rem',
                        fontFamily: 'var(--font-mono)',
                        fontWeight: 700,
                        color: '#f97316',
                        cursor: 'pointer'
                      }} onClick={() => handleStockClick(trade.symbol)}>
                        {trade.symbol}
                      </td>
                      <td style={{ padding: '0.75rem' }}>
                        <span className={trade.type === 'BUY' ? 'badge badge-buy' : 'badge badge-sell'} style={{ padding: '0.15rem 0.4rem', fontSize: '0.7rem' }}>
                          {trade.type === 'BUY' ? '买入' : '卖出'}
                        </span>
                      </td>
                      <td style={{ padding: '0.75rem', fontFamily: 'var(--font-mono)' }}>{trade.amount}</td>
                      <td style={{ padding: '0.75rem', color: 'var(--text-secondary)' }}>{trade.date}</td>
                    </tr>
                  ))}
                  {congressTrades.length === 0 && (
                    <tr>
                      <td colSpan={6} style={{ textAlign: 'center', padding: '2rem 0', color: 'var(--text-muted)' }}>
                        未找到符合条件的国会交易记录
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* Guru Portfolio Modal Overlay */}
      {currentGuru && (
        <div style={{
          position: 'fixed',
          top: 0, left: 0, right: 0, bottom: 0,
          backgroundColor: 'rgba(7, 10, 19, 0.8)',
          backdropFilter: 'blur(8px)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 2000,
          padding: '2rem'
        }}>
          <div className="card" style={{
            width: '100%',
            maxWidth: '850px',
            maxHeight: '85vh',
            overflowY: 'auto',
            display: 'flex',
            flexDirection: 'column',
            gap: '1.5rem',
            position: 'relative'
          }}>
            {/* Close Button */}
            <button
              onClick={clearGuruDetails}
              style={{
                position: 'absolute',
                top: '1.25rem',
                right: '1.25rem',
                background: 'none',
                border: 'none',
                color: 'var(--text-secondary)',
                cursor: 'pointer'
              }}
              onMouseEnter={(e) => e.currentTarget.style.color = 'white'}
              onMouseLeave={(e) => e.currentTarget.style.color = 'var(--text-secondary)'}
            >
              <X size={20} />
            </button>

            {/* Profile Header */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '1.5rem', borderBottom: '1px solid var(--border-color)', paddingBottom: '1.25rem' }}>
              <Avatar
                name={currentGuru.name}
                slug={currentGuru.slug || String(currentGuru.id)}
                letter={currentGuru.avatarCode}
                size={60}
              />
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                <h3 style={{ fontSize: '1.25rem', fontWeight: 800, color: 'white' }}>{currentGuru.name}</h3>
                <span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                  {currentGuru.fundName} · {currentGuru.title}
                </span>
                <div style={{ display: 'flex', gap: '1.5rem', fontSize: '0.8rem', marginTop: '0.25rem', color: 'var(--text-muted)' }}>
                  <span>总申报规模: <strong style={{ color: 'white' }}>{currentGuru.aum}</strong></span>
                  <span>标的持仓数: <strong style={{ color: 'white' }}>{currentGuru.positionCount}</strong></span>
                  <span>第一重仓比重: <strong style={{ color: '#f97316' }}>{currentGuru.topStock} ({currentGuru.topStockWeight}%)</strong></span>
                </div>
              </div>
            </div>

            {/* Holdings Table */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
              <span style={{ fontSize: '0.875rem', fontWeight: 700, color: 'white' }}>完整 13F 重仓持仓细则</span>
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--border-color)', color: 'var(--text-secondary)', textAlign: 'left' }}>
                      <th style={{ padding: '0.6rem 0.5rem' }}>重仓股</th>
                      <th style={{ padding: '0.6rem 0.5rem' }}>持有股数</th>
                      <th style={{ padding: '0.6rem 0.5rem' }}>总持仓市值</th>
                      <th style={{ padding: '0.6rem 0.5rem' }}>仓位变动</th>
                      <th style={{ padding: '0.6rem 0.5rem' }}>占资产比重</th>
                    </tr>
                  </thead>
                  <tbody>
                    {currentGuruHoldings.map((item, index) => (
                      <tr key={index} style={{ borderBottom: '1px solid rgba(255,255,255,0.02)', color: 'var(--text-primary)' }}>
                        <td style={{ padding: '0.65rem 0.5rem' }}>
                          <div style={{ fontWeight: 700, fontFamily: 'var(--font-mono)', color: '#f97316' }}>{item.stockSymbol}</div>
                          <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>{item.stockName}</div>
                        </td>
                        <td style={{ padding: '0.65rem 0.5rem', fontFamily: 'var(--font-mono)' }}>{item.shares}</td>
                        <td style={{ padding: '0.65rem 0.5rem', fontFamily: 'var(--font-mono)' }}>{item.value}</td>
                        <td style={{
                          padding: '0.65rem 0.5rem',
                          fontFamily: 'var(--font-mono)',
                          fontWeight: 600,
                          color: item.change.startsWith('+') ? 'var(--color-success)' : item.change.startsWith('-') ? 'var(--color-danger)' : 'var(--text-secondary)'
                        }}>{item.change}</td>
                        <td style={{ padding: '0.65rem 0.5rem', minWidth: '150px' }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                            <span style={{ minWidth: '35px', fontWeight: 700, fontFamily: 'var(--font-mono)' }}>{item.weight.toFixed(1)}%</span>
                            <div style={{ flex: 1, height: '6px', backgroundColor: 'rgba(255,255,255,0.05)', borderRadius: '9999px', overflow: 'hidden' }}>
                              <div style={{ width: `${(item.weight / currentGuru.topStockWeight) * 100}%`, height: '100%', backgroundColor: '#f97316', borderRadius: '9999px' }} />
                            </div>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Stock Holders Consensus Modal Overlay (Who holds this stock?) */}
      {selectedStockSymbol && (
        <div style={{
          position: 'fixed',
          top: 0, left: 0, right: 0, bottom: 0,
          backgroundColor: 'rgba(7, 10, 19, 0.8)',
          backdropFilter: 'blur(8px)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 2000,
          padding: '2rem'
        }}>
          <div className="card" style={{
            width: '100%',
            maxWidth: '680px',
            maxHeight: '80vh',
            overflowY: 'auto',
            display: 'flex',
            flexDirection: 'column',
            gap: '1.5rem',
            position: 'relative'
          }}>
            {/* Close Button */}
            <button
              onClick={() => setSelectedStockSymbol(null)}
              style={{
                position: 'absolute',
                top: '1.25rem',
                right: '1.25rem',
                background: 'none',
                border: 'none',
                color: 'var(--text-secondary)',
                cursor: 'pointer'
              }}
              onMouseEnter={(e) => e.currentTarget.style.color = 'white'}
              onMouseLeave={(e) => e.currentTarget.style.color = 'var(--text-secondary)'}
            >
              <X size={20} />
            </button>

            {/* Title */}
            <div style={{ borderBottom: '1px solid var(--border-color)', paddingBottom: '1rem' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <span style={{ fontSize: '1.25rem', fontWeight: 800, fontFamily: 'var(--font-mono)', color: '#f97316' }}>{selectedStockSymbol}</span>
                <h3 style={{ fontSize: '1.1rem', fontWeight: 800, color: 'white' }}>巨头共同持有明细报告</h3>
              </div>
              <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginTop: '0.25rem' }}>
                当前已收录持股明细的巨头基金列表如下：
              </p>
            </div>

            {/* Holders List */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--border-color)', color: 'var(--text-secondary)', textAlign: 'left' }}>
                      <th style={{ padding: '0.6rem 0.5rem' }}>基金大佬</th>
                      <th style={{ padding: '0.6rem 0.5rem' }}>持有股数</th>
                      <th style={{ padding: '0.6rem 0.5rem' }}>持股市值</th>
                      <th style={{ padding: '0.6rem 0.5rem' }}>季度仓位变动</th>
                      <th style={{ padding: '0.6rem 0.5rem', textAlign: 'right' }}>占仓位权重</th>
                    </tr>
                  </thead>
                  <tbody>
                    {currentStockHolders.map((item, index) => (
                      <tr key={index} style={{ borderBottom: '1px solid rgba(255,255,255,0.02)', color: 'var(--text-primary)' }}>
                        <td style={{ padding: '0.65rem 0.5rem' }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
                            <Avatar
                              name={item.guruName}
                              slug={item.fundName}
                              letter={item.avatarCode}
                              size={30}
                            />
                            <div>
                              <div style={{ fontWeight: 700 }}>{item.guruName}</div>
                              <div style={{ fontSize: '0.7rem', color: 'var(--text-secondary)' }}>{item.fundName}</div>
                            </div>
                          </div>
                        </td>
                        <td style={{ padding: '0.65rem 0.5rem', fontFamily: 'var(--font-mono)' }}>{item.shares}</td>
                        <td style={{ padding: '0.65rem 0.5rem', fontFamily: 'var(--font-mono)' }}>{item.value}</td>
                        <td style={{
                          padding: '0.65rem 0.5rem',
                          fontFamily: 'var(--font-mono)',
                          fontWeight: 600,
                          color: item.change.startsWith('+') ? 'var(--color-success)' : item.change.startsWith('-') ? 'var(--color-danger)' : 'var(--text-secondary)'
                        }}>{item.change}</td>
                        <td style={{ padding: '0.65rem 0.5rem', textAlign: 'right', fontWeight: 700, fontFamily: 'var(--font-mono)' }}>
                          {item.weight.toFixed(2)}%
                        </td>
                      </tr>
                    ))}
                    {currentStockHolders.length === 0 && (
                      <tr>
                        <td colSpan={5} style={{ textAlign: 'center', padding: '2rem 0', color: 'var(--text-muted)' }}>
                          暂无巨头机构持有该股票的数据记录
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </div>
      )}

    </div>
  )
}
