import React, { useState, useEffect } from 'react'
import { useWhalesStore } from '../stores/whalesStore'
import { 
  Search, TrendingUp, ArrowUp, ArrowDown, Users, 
  Landmark, BarChart3, RefreshCw, X,
  DollarSign, Activity, AlertCircle
} from 'lucide-react'
import {
  ResponsiveContainer, PieChart, Pie, Cell, BarChart, Bar,
  XAxis, YAxis, Tooltip
} from 'recharts'

interface RedditStock {
  rank: number
  name: string
  symbol: string
  mentions: number
  change?: number
}

type ViewMode = 'reddit' | 'gurus' | 'congress' | 'consensus'

export const WhalesSimple: React.FC = () => {
  const { 
    gurus, consensusStocks, congressTrades, currentStockHolders,
    syncing,
    fetchGurus, fetchConsensus, fetchCongressTrades, fetchStockHolders,
    triggerSync, clearStockHolders
  } = useWhalesStore()

  // View mode
  const [viewMode, setViewMode] = useState<ViewMode>('reddit')
  const [searchQuery, setSearchQuery] = useState('')
  const [sortBy, setSortBy] = useState<'mentions' | 'rank'>('mentions')
  
  // Filters
  const [partyFilter, setPartyFilter] = useState<'all' | 'Democratic' | 'Republican'>('all')
  const [tradeTypeFilter, setTradeTypeFilter] = useState<'all' | 'BUY' | 'SELL'>('all')
  
  // Modal state
  const [selectedStock, setSelectedStock] = useState<string | null>(null)

  // 模拟 Reddit 数据
  const [redditStocks] = useState<RedditStock[]>([
    { rank: 1, name: 'Micron Technology', symbol: 'MU', mentions: 1084, change: 15.2 },
    { rank: 2, name: 'Meta Platforms', symbol: 'META', mentions: 512, change: 8.5 },
    { rank: 3, name: "Wendy's Company", symbol: 'WEN', mentions: 351, change: -2.3 },
    { rank: 4, name: 'Microsoft', symbol: 'MSFT', mentions: 317, change: 5.1 },
    { rank: 5, name: 'SPDR S&P 500 ETF', symbol: 'SPY', mentions: 309, change: 1.8 },
    { rank: 6, name: 'Nebius Group', symbol: 'NBIS', mentions: 223, change: 22.4 },
    { rank: 7, name: 'NVIDIA', symbol: 'NVDA', mentions: 181, change: 45.6 },
    { rank: 8, name: 'Apple Inc.', symbol: 'AAPL', mentions: 156, change: -1.2 },
    { rank: 9, name: 'Tesla Inc.', symbol: 'TSLA', mentions: 142, change: 12.8 },
    { rank: 10, name: 'Amazon.com', symbol: 'AMZN', mentions: 128, change: 3.4 },
    { rank: 11, name: 'Advanced Micro Devices', symbol: 'AMD', mentions: 115, change: 18.9 },
    { rank: 12, name: 'Palantir Technologies', symbol: 'PLTR', mentions: 98, change: 35.2 }
  ])

  // 初始化加载数据
  useEffect(() => {
    fetchConsensus()
    fetchGurus()
    fetchCongressTrades()
    triggerSync()
  }, [])

  // 定期刷新（每30秒）
  useEffect(() => {
    const interval = setInterval(() => {
      if (viewMode === 'gurus') fetchGurus()
      if (viewMode === 'congress') fetchCongressTrades(
        partyFilter === 'all' ? undefined : partyFilter,
        tradeTypeFilter === 'all' ? undefined : tradeTypeFilter
      )
      if (viewMode === 'consensus') fetchConsensus()
    }, 30000)
    return () => clearInterval(interval)
  }, [viewMode, partyFilter, tradeTypeFilter])

  // 搜索过滤
  const filteredRedditStocks = redditStocks.filter(stock =>
    stock.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    stock.symbol.toLowerCase().includes(searchQuery.toLowerCase())
  )

  const filteredConsensus = consensusStocks.filter(stock =>
    stock.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    stock.symbol.toLowerCase().includes(searchQuery.toLowerCase())
  )

  const filteredGurus = gurus.filter(guru =>
    guru.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    guru.topStock.toLowerCase().includes(searchQuery.toLowerCase())
  )

  const filteredCongress = congressTrades.filter(trade =>
    (trade.politician.toLowerCase().includes(searchQuery.toLowerCase()) ||
     trade.symbol.toLowerCase().includes(searchQuery.toLowerCase()))
  )

  // 排序
  const sortedStocks = [...filteredRedditStocks].sort((a, b) => {
    if (sortBy === 'mentions') return b.mentions - a.mentions
    return a.rank - b.rank
  })

  // 图表数据准备
  const sectorData = [
    { name: '科技', value: 38, color: '#3b82f6' },
    { name: '金融', value: 22, color: '#8b5cf6' },
    { name: '医疗', value: 16, color: '#10b981' },
    { name: '能源', value: 14, color: '#f59e0b' },
    { name: '其他', value: 10, color: '#6b7280' }
  ]

  const trendData = redditStocks.slice(0, 7).map(stock => ({
    symbol: stock.symbol,
    mentions: stock.mentions,
    change: stock.change || 0
  }))

  const handleStockClick = (symbol: string) => {
    setSelectedStock(symbol)
    fetchStockHolders(symbol)
  }

  return (
    <div style={{
      minHeight: '100vh',
      backgroundColor: '#050814',
      padding: '0',
      fontFamily: 'var(--font-sans)'
    }}>
      {/* Header Section */}
      <div style={{
        backgroundColor: '#0a1122',
        borderBottom: '1px solid rgba(249, 115, 22, 0.2)',
        padding: '1rem 1.25rem',
        position: 'sticky',
        top: 0,
        zIndex: 100,
        backdropFilter: 'blur(12px)'
      }}>
        {/* Logo & Actions Row */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          marginBottom: '0.75rem',
          flexWrap: 'wrap',
          gap: '0.75rem'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <span style={{ fontSize: '2rem' }}>🦊</span>
            <div>
              <h1 style={{
                fontSize: '1.4rem',
                fontWeight: 700,
                background: 'linear-gradient(135deg, #f97316 0%, #fb923c 100%)',
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
                margin: 0,
                lineHeight: 1.2
              }}>美股狐狸</h1>
              <p style={{
                fontSize: '0.75rem',
                color: '#6b7280',
                margin: 0,
                marginTop: '0.15rem'
              }}>巨鲸追踪 · 聪明钱共识</p>
            </div>
          </div>

          <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
            {/* Sync Status */}
            {syncing ? (
              <div style={{
                display: 'flex',
                alignItems: 'center',
                gap: '0.4rem',
                padding: '0.5rem 0.85rem',
                backgroundColor: 'rgba(234, 179, 8, 0.1)',
                border: '1px solid rgba(234, 179, 8, 0.3)',
                borderRadius: '0.5rem',
                fontSize: '0.75rem',
                color: '#eab308'
              }}>
                <RefreshCw size={14} className="spin" />
                同步中...
              </div>
            ) : null}

            {/* Join Group Button */}
            <button style={{
              backgroundColor: '#10b981',
              color: 'white',
              border: 'none',
              borderRadius: '0.5rem',
              padding: '0.55rem 1rem',
              fontSize: '0.85rem',
              fontWeight: 600,
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '0.4rem',
              transition: 'all 0.2s ease',
              boxShadow: '0 2px 8px rgba(16, 185, 129, 0.3)'
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = '#059669'
              e.currentTarget.style.transform = 'translateY(-1px)'
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = '#10b981'
              e.currentTarget.style.transform = 'translateY(0)'
            }}
            >
              <Users size={15} />
              进群
            </button>
          </div>
        </div>

        {/* View Mode Tabs */}
        <div style={{
          display: 'flex',
          gap: '0.5rem',
          overflowX: 'auto',
          paddingBottom: '0.5rem',
          borderBottom: '1px solid rgba(255,255,255,0.05)'
        }}>
          {[
            { mode: 'reddit' as ViewMode, icon: TrendingUp, label: 'Reddit热度' },
            { mode: 'consensus' as ViewMode, icon: BarChart3, label: '共识持仓' },
            { mode: 'gurus' as ViewMode, icon: Landmark, label: '机构13F' },
            { mode: 'congress' as ViewMode, icon: Users, label: '国会交易' }
          ].map(({ mode, icon: Icon, label }) => (
            <button
              key={mode}
              onClick={() => setViewMode(mode)}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '0.4rem',
                padding: '0.5rem 1rem',
                backgroundColor: viewMode === mode ? 'rgba(249, 115, 22, 0.15)' : 'transparent',
                border: `1px solid ${viewMode === mode ? '#f97316' : 'transparent'}`,
                borderRadius: '0.5rem',
                color: viewMode === mode ? '#f97316' : '#9ca3af',
                fontSize: '0.85rem',
                fontWeight: 600,
                cursor: 'pointer',
                transition: 'all 0.2s ease',
                whiteSpace: 'nowrap'
              }}
            >
              <Icon size={16} />
              {label}
            </button>
          ))}
        </div>
      </div>

      {/* Search Bar */}
      <div style={{
        padding: '1rem 1.25rem',
        backgroundColor: '#050814'
      }}>
        <div style={{ position: 'relative', maxWidth: '600px' }}>
          <Search size={18} style={{
            position: 'absolute',
            left: '1rem',
            top: '50%',
            transform: 'translateY(-50%)',
            color: '#6b7280',
            pointerEvents: 'none'
          }} />
          <input
            type="text"
            placeholder={
              viewMode === 'reddit' ? '搜索股票代码或名称 (如 MU、NVDA)' :
              viewMode === 'consensus' ? '搜索共识持仓股票' :
              viewMode === 'gurus' ? '搜索机构名称或持仓股' :
              '搜索议员或交易股票'
            }
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            style={{
              width: '100%',
              backgroundColor: '#0d1628',
              border: '1px solid rgba(249, 115, 22, 0.15)',
              borderRadius: '0.75rem',
              padding: '0.85rem 1rem 0.85rem 3rem',
              color: 'white',
              fontSize: '0.9rem',
              outline: 'none',
              transition: 'all 0.2s ease'
            }}
            onFocus={(e) => {
              e.currentTarget.style.borderColor = '#f97316'
              e.currentTarget.style.boxShadow = '0 0 0 3px rgba(249, 115, 22, 0.15)'
            }}
            onBlur={(e) => {
              e.currentTarget.style.borderColor = 'rgba(249, 115, 22, 0.15)'
              e.currentTarget.style.boxShadow = 'none'
            }}
          />
          {searchQuery && (
            <button
              onClick={() => setSearchQuery('')}
              style={{
                position: 'absolute',
                right: '1rem',
                top: '50%',
                transform: 'translateY(-50%)',
                background: 'none',
                border: 'none',
                color: '#6b7280',
                cursor: 'pointer',
                padding: '0.25rem',
                display: 'flex',
                alignItems: 'center'
              }}
            >
              <X size={16} />
            </button>
          )}
        </div>

        {/* Congress Filters */}
        {viewMode === 'congress' && (
          <div style={{
            marginTop: '1rem',
            display: 'flex',
            gap: '0.75rem',
            flexWrap: 'wrap',
            alignItems: 'center'
          }}>
            <span style={{ fontSize: '0.8rem', color: '#9ca3af' }}>筛选:</span>
            {['all', 'Democratic', 'Republican'].map(party => (
              <button
                key={party}
                onClick={() => {
                  setPartyFilter(party as any)
                  fetchCongressTrades(
                    party === 'all' ? undefined : party,
                    tradeTypeFilter === 'all' ? undefined : tradeTypeFilter
                  )
                }}
                style={{
                  padding: '0.35rem 0.75rem',
                  fontSize: '0.75rem',
                  fontWeight: 600,
                  borderRadius: '0.4rem',
                  border: `1px solid ${partyFilter === party ? '#f97316' : 'rgba(255,255,255,0.1)'}`,
                  backgroundColor: partyFilter === party ? 'rgba(249, 115, 22, 0.15)' : 'transparent',
                  color: partyFilter === party ? '#f97316' : '#9ca3af',
                  cursor: 'pointer',
                  transition: 'all 0.15s ease'
                }}
              >
                {party === 'all' ? '全部党派' : party === 'Democratic' ? '民主党' : '共和党'}
              </button>
            ))}
            <div style={{ width: '1px', height: '20px', backgroundColor: 'rgba(255,255,255,0.1)' }} />
            {['all', 'BUY', 'SELL'].map(type => (
              <button
                key={type}
                onClick={() => {
                  setTradeTypeFilter(type as any)
                  fetchCongressTrades(
                    partyFilter === 'all' ? undefined : partyFilter,
                    type === 'all' ? undefined : type
                  )
                }}
                style={{
                  padding: '0.35rem 0.75rem',
                  fontSize: '0.75rem',
                  fontWeight: 600,
                  borderRadius: '0.4rem',
                  border: `1px solid ${tradeTypeFilter === type ? '#f97316' : 'rgba(255,255,255,0.1)'}`,
                  backgroundColor: tradeTypeFilter === type ? 'rgba(249, 115, 22, 0.15)' : 'transparent',
                  color: tradeTypeFilter === type ? '#f97316' : '#9ca3af',
                  cursor: 'pointer',
                  transition: 'all 0.15s ease'
                }}
              >
                {type === 'all' ? '全部' : type === 'BUY' ? '买入' : '卖出'}
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Main Content Area */}
      <div style={{ padding: '0 1.25rem 2rem' }}>
        
        {/* Reddit热度视图 */}
        {viewMode === 'reddit' && (
          <div>
            {/* Charts Grid (Desktop only) */}
            <div style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))',
              gap: '1rem',
              marginBottom: '1.5rem'
            }} className="hide-on-mobile">
              {/* Sector Distribution */}
              <div style={{
                backgroundColor: '#0d1628',
                border: '1px solid rgba(249, 115, 22, 0.15)',
                borderRadius: '1rem',
                padding: '1.25rem'
              }}>
                <h3 style={{ 
                  fontSize: '0.95rem', 
                  fontWeight: 600, 
                  color: 'white',
                  marginBottom: '1rem',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '0.5rem'
                }}>
                  <BarChart3 size={18} style={{ color: '#f97316' }} />
                  行业分布
                </h3>
                <ResponsiveContainer width="100%" height={200}>
                  <PieChart>
                    <Pie
                      data={sectorData}
                      cx="50%"
                      cy="50%"
                      innerRadius={50}
                      outerRadius={80}
                      paddingAngle={2}
                      dataKey="value"
                    >
                      {sectorData.map((entry, index) => (
                        <Cell key={`cell-${index}`} fill={entry.color} />
                      ))}
                    </Pie>
                    <Tooltip 
                      contentStyle={{ 
                        backgroundColor: '#0d1628', 
                        border: '1px solid rgba(249, 115, 22, 0.3)',
                        borderRadius: '0.5rem',
                        fontSize: '0.8rem'
                      }} 
                    />
                  </PieChart>
                </ResponsiveContainer>
              </div>

              {/* Trend Chart */}
              <div style={{
                backgroundColor: '#0d1628',
                border: '1px solid rgba(249, 115, 22, 0.15)',
                borderRadius: '1rem',
                padding: '1.25rem'
              }}>
                <h3 style={{ 
                  fontSize: '0.95rem', 
                  fontWeight: 600, 
                  color: 'white',
                  marginBottom: '1rem',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '0.5rem'
                }}>
                  <Activity size={18} style={{ color: '#10b981' }} />
                  热度趋势
                </h3>
                <ResponsiveContainer width="100%" height={200}>
                  <BarChart data={trendData}>
                    <XAxis 
                      dataKey="symbol" 
                      stroke="#6b7280" 
                      fontSize={11}
                      tick={{ fill: '#9ca3af' }}
                    />
                    <YAxis 
                      stroke="#6b7280" 
                      fontSize={11}
                      tick={{ fill: '#9ca3af' }}
                    />
                    <Tooltip 
                      contentStyle={{ 
                        backgroundColor: '#0d1628', 
                        border: '1px solid rgba(249, 115, 22, 0.3)',
                        borderRadius: '0.5rem',
                        fontSize: '0.8rem'
                      }} 
                    />
                    <Bar dataKey="mentions" fill="#f97316" radius={[4, 4, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            </div>

            {/* Table Header */}
            <div style={{
              display: 'grid',
              gridTemplateColumns: '50px 1fr 90px 110px',
              padding: '0.75rem 1rem',
              backgroundColor: '#0d1628',
              borderRadius: '0.75rem 0.75rem 0 0',
              borderBottom: '1px solid rgba(249, 115, 22, 0.2)',
              fontSize: '0.75rem',
              fontWeight: 600,
              color: '#9ca3af',
              textTransform: 'uppercase',
              letterSpacing: '0.05em'
            }}>
              <div>排名</div>
              <div>股票名称</div>
              <div>代码</div>
              <div 
                style={{ 
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '0.25rem',
                  userSelect: 'none'
                }}
                onClick={() => setSortBy(sortBy === 'mentions' ? 'rank' : 'mentions')}
              >
                提及次数
                <TrendingUp size={12} style={{ 
                  color: sortBy === 'mentions' ? '#f97316' : '#6b7280'
                }} />
              </div>
            </div>

            {/* Stock List */}
            <div style={{
              backgroundColor: '#0a1122',
              borderRadius: '0 0 0.75rem 0.75rem',
              overflow: 'hidden'
            }}>
              {sortedStocks.map((stock, index) => (
                <div
                  key={stock.symbol}
                  onClick={() => handleStockClick(stock.symbol)}
                  style={{
                    display: 'grid',
                    gridTemplateColumns: '50px 1fr 90px 110px',
                    padding: '1rem',
                    borderBottom: index < sortedStocks.length - 1 ? '1px solid rgba(255,255,255,0.03)' : 'none',
                    transition: 'all 0.2s ease',
                    cursor: 'pointer'
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.backgroundColor = 'rgba(249, 115, 22, 0.08)'
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.backgroundColor = 'transparent'
                  }}
                >
                  {/* Rank */}
                  <div style={{
                    fontSize: '1.1rem',
                    fontWeight: 700,
                    color: stock.rank <= 3 ? '#f97316' : '#6b7280',
                    display: 'flex',
                    alignItems: 'center'
                  }}>
                    {stock.rank}
                  </div>

                  {/* Name */}
                  <div style={{
                    fontSize: '0.9rem',
                    fontWeight: 500,
                    color: 'white',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                    display: 'flex',
                    alignItems: 'center'
                  }}>
                    {stock.name}
                  </div>

                  {/* Symbol */}
                  <div style={{
                    fontSize: '0.85rem',
                    fontWeight: 700,
                    color: '#10b981',
                    fontFamily: 'var(--font-mono)',
                    letterSpacing: '0.02em',
                    display: 'flex',
                    alignItems: 'center'
                  }}>
                    {stock.symbol}
                  </div>

                  {/* Mentions & Change */}
                  <div style={{
                    display: 'flex',
                    flexDirection: 'column',
                    justifyContent: 'center',
                    gap: '0.15rem'
                  }}>
                    <span style={{
                      fontSize: '0.95rem',
                      fontWeight: 600,
                      color: 'white'
                    }}>
                      {stock.mentions.toLocaleString()}
                    </span>
                    {stock.change !== undefined && (
                      <span style={{
                        fontSize: '0.7rem',
                        fontWeight: 600,
                        color: stock.change >= 0 ? '#10b981' : '#ef4444',
                        display: 'flex',
                        alignItems: 'center',
                        gap: '0.15rem'
                      }}>
                        {stock.change >= 0 ? <ArrowUp size={10} /> : <ArrowDown size={10} />}
                        {Math.abs(stock.change)}%
                      </span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* 共识持仓视图 */}
        {viewMode === 'consensus' && (
          <div>
            <div style={{
              backgroundColor: '#0d1628',
              border: '1px solid rgba(249, 115, 22, 0.15)',
              borderRadius: '1rem',
              padding: '1.25rem',
              marginBottom: '1rem'
            }}>
              <div style={{
                display: 'flex',
                alignItems: 'center',
                gap: '0.5rem',
                marginBottom: '0.5rem'
              }}>
                <DollarSign size={18} style={{ color: '#f97316' }} />
                <h3 style={{ fontSize: '1rem', fontWeight: 600, color: 'white', margin: 0 }}>
                  聪明钱共识 - 巨头最爱重仓股
                </h3>
              </div>
              <p style={{ fontSize: '0.8rem', color: '#6b7280', margin: 0 }}>
                根据最新 13F 申报数据，统计被最多机构重仓持有的股票
              </p>
            </div>

            {filteredConsensus.map((stock, index) => (
              <div
                key={stock.symbol}
                onClick={() => handleStockClick(stock.symbol)}
                style={{
                  backgroundColor: '#0d1628',
                  border: '1px solid rgba(255,255,255,0.05)',
                  borderRadius: '0.75rem',
                  padding: '1rem',
                  marginBottom: '0.75rem',
                  cursor: 'pointer',
                  transition: 'all 0.2s ease'
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.borderColor = '#f97316'
                  e.currentTarget.style.transform = 'translateX(4px)'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.borderColor = 'rgba(255,255,255,0.05)'
                  e.currentTarget.style.transform = 'translateX(0)'
                }}
              >
                <div style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  marginBottom: '0.75rem'
                }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
                    <span style={{
                      fontSize: '1.2rem',
                      fontWeight: 800,
                      color: index < 3 ? '#f97316' : '#6b7280',
                      width: '30px'
                    }}>
                      {index + 1}
                    </span>
                    <div>
                      <div style={{
                        fontSize: '1rem',
                        fontWeight: 700,
                        color: 'white',
                        marginBottom: '0.15rem'
                      }}>
                        {stock.symbol}
                      </div>
                      <div style={{
                        fontSize: '0.8rem',
                        color: '#9ca3af'
                      }}>
                        {stock.name}
                      </div>
                    </div>
                  </div>
                  <div style={{
                    textAlign: 'right'
                  }}>
                    <div style={{
                      fontSize: '1.1rem',
                      fontWeight: 700,
                      color: '#10b981',
                      marginBottom: '0.15rem'
                    }}>
                      {stock.avgWeight.toFixed(1)}%
                    </div>
                    <div style={{
                      fontSize: '0.75rem',
                      color: stock.change.startsWith('+') ? '#10b981' : '#ef4444',
                      fontWeight: 600
                    }}>
                      {stock.change}
                    </div>
                  </div>
                </div>
                <div style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '0.5rem',
                  paddingTop: '0.75rem',
                  borderTop: '1px solid rgba(255,255,255,0.05)'
                }}>
                  <Users size={14} style={{ color: '#6b7280' }} />
                  <span style={{ fontSize: '0.75rem', color: '#9ca3af' }}>
                    {stock.guruCount} 个机构持仓
                  </span>
                  <div style={{
                    flex: 1,
                    height: '4px',
                    backgroundColor: 'rgba(255,255,255,0.05)',
                    borderRadius: '2px',
                    overflow: 'hidden',
                    marginLeft: '0.5rem'
                  }}>
                    <div style={{
                      width: `${Math.min((stock.guruCount / 10) * 100, 100)}%`,
                      height: '100%',
                      backgroundColor: '#f97316',
                      borderRadius: '2px'
                    }} />
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* 机构13F视图 */}
        {viewMode === 'gurus' && (
          <div>
            <div style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
              gap: '1rem'
            }}>
              {filteredGurus.map((guru) => {
                // 使用 DiceBear Avatars API 生成高质量头像
                // 风格选项: adventurer, avataaars, bottts, identicon, initials, lorelei, personas, pixel-art
                const avatarStyle = 'lorelei' // 专业商务风格
                const seed = encodeURIComponent(guru.name) // 使用机构名称作为种子
                const avatarUrl = `https://api.dicebear.com/7.x/${avatarStyle}/svg?seed=${seed}&backgroundColor=1e293b,334155,475569&radius=50`
                
                // 备用方案：UI Avatars (如果 DiceBear 加载失败)
                const fallbackAvatarUrl = `https://ui-avatars.com/api/?name=${encodeURIComponent(guru.name)}&background=f97316&color=fff&size=200&bold=true&format=svg`
                
                return (
                  <div
                    key={guru.id}
                    style={{
                      backgroundColor: '#0d1628',
                      border: '1px solid rgba(249, 115, 22, 0.15)',
                      borderRadius: '1rem',
                      padding: '1.25rem',
                      cursor: 'pointer',
                      transition: 'all 0.2s ease'
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.borderColor = '#f97316'
                      e.currentTarget.style.boxShadow = '0 8px 24px rgba(249, 115, 22, 0.15)'
                      e.currentTarget.style.transform = 'translateY(-4px)'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.borderColor = 'rgba(249, 115, 22, 0.15)'
                      e.currentTarget.style.boxShadow = 'none'
                      e.currentTarget.style.transform = 'translateY(0)'
                    }}
                  >
                    <div style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '1rem',
                      marginBottom: '1rem'
                    }}>
                      {/* 真实头像 */}
                      <div style={{
                        width: '60px',
                        height: '60px',
                        borderRadius: '50%',
                        overflow: 'hidden',
                        flexShrink: 0,
                        border: '3px solid rgba(249, 115, 22, 0.2)',
                        boxShadow: '0 4px 12px rgba(249, 115, 22, 0.2)',
                        backgroundColor: '#1e293b',
                        position: 'relative'
                      }}>
                        <img
                          src={avatarUrl}
                          alt={guru.name}
                          style={{
                            width: '100%',
                            height: '100%',
                            objectFit: 'cover'
                          }}
                          onError={(e) => {
                            // 如果主 API 失败，切换到备用方案
                            e.currentTarget.src = fallbackAvatarUrl
                          }}
                        />
                        {/* 在线状态指示器 */}
                        <div style={{
                          position: 'absolute',
                          bottom: '2px',
                          right: '2px',
                          width: '12px',
                          height: '12px',
                          backgroundColor: '#10b981',
                          border: '2px solid #0d1628',
                          borderRadius: '50%',
                          boxShadow: '0 0 8px rgba(16, 185, 129, 0.5)'
                        }} />
                      </div>
                      
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <h4 style={{
                          fontSize: '1rem',
                          fontWeight: 700,
                          color: 'white',
                          marginBottom: '0.25rem',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap'
                        }}>
                          {guru.name}
                        </h4>
                        <p style={{
                          fontSize: '0.75rem',
                          color: '#9ca3af',
                          margin: 0,
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap'
                        }}>
                          {guru.title}
                        </p>
                        <p style={{
                          fontSize: '0.7rem',
                          color: '#6b7280',
                          margin: 0,
                          marginTop: '0.15rem',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap'
                        }}>
                          {guru.fundName}
                        </p>
                      </div>
                    </div>
                    <div style={{
                      display: 'grid',
                      gridTemplateColumns: '1fr 1fr',
                      gap: '0.75rem',
                      fontSize: '0.8rem',
                      paddingTop: '1rem',
                      borderTop: '1px solid rgba(255,255,255,0.05)'
                    }}>
                      <div>
                        <div style={{ color: '#6b7280', marginBottom: '0.25rem', fontSize: '0.7rem' }}>管理规模</div>
                        <div style={{ color: 'white', fontWeight: 600, fontFamily: 'var(--font-mono)', fontSize: '0.85rem' }}>
                          {guru.aum}
                        </div>
                      </div>
                      <div>
                        <div style={{ color: '#6b7280', marginBottom: '0.25rem', fontSize: '0.7rem' }}>持仓数</div>
                        <div style={{ color: 'white', fontWeight: 600, fontSize: '0.85rem' }}>{guru.positionCount}</div>
                      </div>
                      <div style={{ gridColumn: '1 / -1', marginTop: '0.25rem' }}>
                        <div style={{ color: '#6b7280', marginBottom: '0.35rem', fontSize: '0.7rem' }}>第一重仓股</div>
                        <div style={{
                          display: 'flex',
                          justifyContent: 'space-between',
                          alignItems: 'center',
                          backgroundColor: 'rgba(16, 185, 129, 0.05)',
                          padding: '0.5rem 0.75rem',
                          borderRadius: '0.5rem',
                          border: '1px solid rgba(16, 185, 129, 0.1)'
                        }}>
                          <span style={{
                            color: '#10b981',
                            fontWeight: 700,
                            fontFamily: 'var(--font-mono)',
                            fontSize: '0.9rem'
                          }}>
                            {guru.topStock}
                          </span>
                          <span style={{ 
                            color: '#f97316', 
                            fontWeight: 700,
                            fontSize: '0.9rem',
                            backgroundColor: 'rgba(249, 115, 22, 0.1)',
                            padding: '0.15rem 0.5rem',
                            borderRadius: '0.25rem'
                          }}>
                            {guru.topStockWeight.toFixed(1)}%
                          </span>
                        </div>
                      </div>
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        )}

        {/* 国会交易视图 */}
        {viewMode === 'congress' && (
          <div>
            {filteredCongress.map((trade) => {
              // 获取议员姓名首字母缩写
              const getInitials = (name: string) => {
                const parts = name.split(' ')
                if (parts.length >= 2) {
                  return parts[0].charAt(0) + parts[parts.length - 1].charAt(0)
                }
                return name.substring(0, 2).toUpperCase()
              }
              
              const initials = getInitials(trade.politician)
              
              // 根据党派设置头像背景色
              const avatarBgColor = trade.party === 'Democratic' ? '#3b82f6' : '#ef4444'
              
              return (
                <div
                  key={trade.id}
                  onClick={() => handleStockClick(trade.symbol)}
                  style={{
                    backgroundColor: '#0d1628',
                    border: '1px solid rgba(255,255,255,0.05)',
                    borderRadius: '0.75rem',
                    padding: '1rem',
                    marginBottom: '0.75rem',
                    cursor: 'pointer',
                    transition: 'all 0.2s ease'
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.borderColor = '#f97316'
                    e.currentTarget.style.transform = 'translateX(4px)'
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.borderColor = 'rgba(255,255,255,0.05)'
                    e.currentTarget.style.transform = 'translateX(0)'
                  }}
                >
                  <div style={{
                    display: 'flex',
                    gap: '1rem',
                    marginBottom: '0.75rem'
                  }}>
                    {/* 议员头像 - 首字母缩写 */}
                    <div style={{
                      width: '60px',
                      height: '60px',
                      borderRadius: '50%',
                      backgroundColor: avatarBgColor,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      flexShrink: 0,
                      boxShadow: `0 4px 12px ${avatarBgColor}40`,
                      fontSize: '1.3rem',
                      fontWeight: 700,
                      color: 'white',
                      fontFamily: 'var(--font-sans)',
                      letterSpacing: '0.05em'
                    }}>
                      {initials}
                    </div>

                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{
                        display: 'flex',
                        justifyContent: 'space-between',
                        alignItems: 'start',
                        marginBottom: '0.35rem',
                        gap: '0.5rem'
                      }}>
                        <div style={{ flex: 1, minWidth: 0 }}>
                          <h4 style={{
                            fontSize: '1.05rem',
                            fontWeight: 600,
                            color: 'white',
                            marginBottom: '0.35rem',
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap'
                          }}>
                            {trade.politician}
                          </h4>
                          <div style={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: '0.5rem',
                            flexWrap: 'wrap'
                          }}>
                            <span style={{
                              display: 'inline-flex',
                              alignItems: 'center',
                              padding: '0.2rem 0.6rem',
                              borderRadius: '0.3rem',
                              backgroundColor: trade.party === 'Democratic' ? 'rgba(59, 130, 246, 0.2)' : 'rgba(239, 68, 68, 0.2)',
                              color: trade.party === 'Democratic' ? '#60a5fa' : '#f87171',
                              fontWeight: 600,
                              fontSize: '0.75rem',
                              border: `1px solid ${trade.party === 'Democratic' ? 'rgba(59, 130, 246, 0.3)' : 'rgba(239, 68, 68, 0.3)'}`
                            }}>
                              {trade.party === 'Democratic' ? '民主党(D)' : '共和党(R)'}
                            </span>
                            <span style={{
                              fontSize: '0.8rem',
                              color: '#9ca3af'
                            }}>
                              {trade.title || 'Representative'}
                            </span>
                          </div>
                        </div>
                        <span style={{
                          padding: '0.3rem 0.7rem',
                          borderRadius: '0.4rem',
                          fontSize: '0.75rem',
                          fontWeight: 700,
                          backgroundColor: trade.type === 'BUY' ? 'rgba(16, 185, 129, 0.2)' : 'rgba(239, 68, 68, 0.2)',
                          color: trade.type === 'BUY' ? '#10b981' : '#ef4444',
                          whiteSpace: 'nowrap',
                          border: `1px solid ${trade.type === 'BUY' ? 'rgba(16, 185, 129, 0.3)' : 'rgba(239, 68, 68, 0.3)'}`
                        }}>
                          {trade.type === 'BUY' ? '买入' : '卖出'}
                        </span>
                      </div>
                    </div>
                  </div>

                  <div style={{
                    display: 'grid',
                    gridTemplateColumns: 'auto 1fr auto',
                    gap: '1rem',
                    alignItems: 'center',
                    paddingTop: '0.75rem',
                    borderTop: '1px solid rgba(255,255,255,0.05)',
                    fontSize: '0.85rem'
                  }}>
                    <div style={{
                      fontFamily: 'var(--font-mono)',
                      fontWeight: 700,
                      color: '#10b981',
                      fontSize: '1rem',
                      letterSpacing: '0.05em'
                    }}>
                      {trade.symbol}
                    </div>
                    <div style={{
                      color: '#9ca3af',
                      fontSize: '0.85rem'
                    }}>
                      {trade.amount}
                    </div>
                    <div style={{
                      color: '#6b7280',
                      fontSize: '0.75rem'
                    }}>
                      {trade.date}
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        )}

        {/* Empty States */}
        {((viewMode === 'reddit' && sortedStocks.length === 0) ||
          (viewMode === 'consensus' && filteredConsensus.length === 0) ||
          (viewMode === 'gurus' && filteredGurus.length === 0) ||
          (viewMode === 'congress' && filteredCongress.length === 0)) && (
          <div style={{
            padding: '4rem 2rem',
            textAlign: 'center',
            color: '#6b7280'
          }}>
            <AlertCircle size={48} style={{ margin: '0 auto 1rem', opacity: 0.3 }} />
            <p style={{ fontSize: '0.9rem', marginBottom: '0.5rem' }}>未找到匹配的数据</p>
            <p style={{ fontSize: '0.8rem', opacity: 0.7 }}>尝试调整搜索条件或筛选器</p>
          </div>
        )}
      </div>

      {/* Stock Holders Modal */}
      {selectedStock && currentStockHolders.length > 0 && (
        <div
          onClick={() => {
            setSelectedStock(null)
            clearStockHolders()
          }}
          style={{
            position: 'fixed',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            backgroundColor: 'rgba(5, 8, 20, 0.9)',
            backdropFilter: 'blur(8px)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
            padding: '1rem'
          }}
        >
          <div
            onClick={(e) => e.stopPropagation()}
            style={{
              backgroundColor: '#0d1628',
              border: '1px solid rgba(249, 115, 22, 0.3)',
              borderRadius: '1rem',
              padding: '1.5rem',
              maxWidth: '600px',
              width: '100%',
              maxHeight: '80vh',
              overflowY: 'auto'
            }}
          >
            <div style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              marginBottom: '1.5rem'
            }}>
              <h3 style={{
                fontSize: '1.2rem',
                fontWeight: 700,
                color: 'white',
                margin: 0
              }}>
                {selectedStock} 机构持仓明细
              </h3>
              <button
                onClick={() => {
                  setSelectedStock(null)
                  clearStockHolders()
                }}
                style={{
                  background: 'none',
                  border: 'none',
                  color: '#9ca3af',
                  cursor: 'pointer',
                  padding: '0.25rem'
                }}
              >
                <X size={24} />
              </button>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
              {currentStockHolders.map((holder, index) => (
                <div
                  key={index}
                  style={{
                    backgroundColor: 'rgba(255,255,255,0.02)',
                    border: '1px solid rgba(255,255,255,0.05)',
                    borderRadius: '0.5rem',
                    padding: '1rem'
                  }}
                >
                  <div style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'start',
                    marginBottom: '0.5rem'
                  }}>
                    <div>
                      <h4 style={{
                        fontSize: '0.95rem',
                        fontWeight: 600,
                        color: 'white',
                        marginBottom: '0.25rem'
                      }}>
                        {holder.guruName}
                      </h4>
                      <p style={{
                        fontSize: '0.75rem',
                        color: '#9ca3af',
                        margin: 0
                      }}>
                        {holder.fundName}
                      </p>
                    </div>
                    <span style={{
                      fontSize: '1rem',
                      fontWeight: 700,
                      color: '#f97316'
                    }}>
                      {holder.weight.toFixed(1)}%
                    </span>
                  </div>
                  <div style={{
                    display: 'grid',
                    gridTemplateColumns: '1fr 1fr',
                    gap: '0.5rem',
                    fontSize: '0.8rem',
                    paddingTop: '0.5rem',
                    borderTop: '1px solid rgba(255,255,255,0.05)'
                  }}>
                    <div>
                      <span style={{ color: '#6b7280' }}>持仓市值: </span>
                      <span style={{ color: 'white', fontWeight: 600 }}>{holder.value}</span>
                    </div>
                    <div>
                      <span style={{ color: '#6b7280' }}>股数: </span>
                      <span style={{ color: 'white', fontWeight: 600 }}>{holder.shares}</span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Footer */}
      <div style={{
        padding: '2rem 1.25rem',
        backgroundColor: '#0a1122',
        borderTop: '1px solid rgba(249, 115, 22, 0.15)',
        marginTop: '3rem'
      }}>
        <div style={{
          maxWidth: '800px',
          margin: '0 auto',
          textAlign: 'center'
        }}>
          <p style={{
            fontSize: '0.8rem',
            color: '#6b7280',
            lineHeight: 1.6,
            margin: 0,
            marginBottom: '0.75rem'
          }}>
            数据来源于 Reddit 社区热度统计、SEC 13F 申报、国会交易披露等公开数据。
            <br />
            每 24 小时更新一次，仅供参考学习，不构成投资建议。
          </p>
          <div style={{
            display: 'flex',
            justifyContent: 'center',
            gap: '1.5rem',
            fontSize: '0.75rem',
            color: '#9ca3af',
            marginTop: '1rem'
          }}>
            <a href="#" style={{ color: '#9ca3af', textDecoration: 'none' }}>关于我们</a>
            <span>·</span>
            <a href="#" style={{ color: '#9ca3af', textDecoration: 'none' }}>数据来源</a>
            <span>·</span>
            <a href="#" style={{ color: '#9ca3af', textDecoration: 'none' }}>免责声明</a>
          </div>
        </div>
      </div>

      {/* Inline Styles for Animations */}
      <style>{`
        .spin {
          animation: spin 1s linear infinite;
        }
        
        @keyframes spin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }

        @media (max-width: 768px) {
          .hide-on-mobile {
            display: none !important;
          }
        }
      `}</style>
    </div>
  )
}
