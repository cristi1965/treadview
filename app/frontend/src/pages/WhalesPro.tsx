import React, { useState, useEffect } from 'react'
import { useWhalesStore } from '../stores/whalesStore'
import { Avatar } from '../components/Avatar'
import { 
  Search, Menu, TrendingUp, BarChart3, Users, 
  Flame, Trophy, Eye, Star, ChevronRight
} from 'lucide-react'

export const WhalesPro: React.FC = () => {
  const { 
    gurus, consensusStocks, congressTrades,
    fetchGurus, fetchConsensus, fetchCongressTrades
  } = useWhalesStore()

  // 状态
  const [activeNav, setActiveNav] = useState<'gurus' | 'congress'>('gurus')
  const [guruFilter, setGuruFilter] = useState<'all' | 'us' | 'a_share' | 'private' | 'hot_money'>('all')
  const [searchQuery, setSearchQuery] = useState('')
  const [expandedGuru, setExpandedGuru] = useState<number | string | null>(null)
  const [showSidebar, setShowSidebar] = useState(true)

  useEffect(() => {
    fetchConsensus()
    fetchGurus()
    fetchCongressTrades()
  }, [])

  // 过滤机构
  const filteredGurus = gurus.filter(guru => {
    const matchesSearch = guru.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
                         guru.topStock.toLowerCase().includes(searchQuery.toLowerCase())
    if (guruFilter === 'all') return matchesSearch
    // 根据类型筛选
    return matchesSearch
  })

  // 议员首字母
  const getInitials = (name: string) => {
    const parts = name.split(' ')
    if (parts.length >= 2) {
      return parts[0].charAt(0) + parts[parts.length - 1].charAt(0)
    }
    return name.substring(0, 2).toUpperCase()
  }

  return (
    <div style={{
      display: 'flex',
      minHeight: '100vh',
      backgroundColor: '#0a0a0a',
      color: 'white',
      fontFamily: 'var(--font-sans)'
    }}>
      {/* 左侧导航栏 */}
      {showSidebar && (
        <div style={{
          width: '180px',
          backgroundColor: '#121212',
          borderRight: '1px solid rgba(255,255,255,0.05)',
          padding: '1.5rem 0',
          display: 'flex',
          flexDirection: 'column',
          gap: '0.5rem'
        }}>
          {/* Logo */}
          <div style={{
            padding: '0 1.5rem',
            marginBottom: '1rem',
            display: 'flex',
            alignItems: 'center',
            gap: '0.75rem'
          }}>
            <div style={{
              width: '36px',
              height: '36px',
              backgroundColor: '#f97316',
              borderRadius: '8px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: '1.2rem'
            }}>
              🦊
            </div>
            <div>
              <div style={{ fontSize: '0.95rem', fontWeight: 600 }}>我不是神</div>
              <div style={{ fontSize: '0.7rem', color: '#666' }}>聪明钱追踪</div>
            </div>
          </div>

          {/* 导航项 */}
          {[
            { icon: '🔥', label: '热力图', key: 'heatmap' },
            { icon: '📋', label: '列表', key: 'list' },
            { icon: '📊', label: 'ETF', key: 'etf' },
            { icon: '💰', label: '聪明钱', key: 'whales', active: true },
            { icon: '⚔️', label: '对决', key: 'battle' },
            { icon: '📰', label: '盘报', key: 'report' },
            { icon: '⭐', label: '我的', key: 'my' }
          ].map((item) => (
            <div
              key={item.key}
              style={{
                padding: '0.75rem 1.5rem',
                cursor: 'pointer',
                backgroundColor: item.active ? 'rgba(249, 115, 22, 0.1)' : 'transparent',
                borderLeft: item.active ? '3px solid #f97316' : '3px solid transparent',
                color: item.active ? '#f97316' : '#999',
                fontSize: '0.9rem',
                display: 'flex',
                alignItems: 'center',
                gap: '0.75rem',
                transition: 'all 0.2s'
              }}
              onMouseEnter={(e) => {
                if (!item.active) {
                  e.currentTarget.style.backgroundColor = 'rgba(255,255,255,0.03)'
                  e.currentTarget.style.color = 'white'
                }
              }}
              onMouseLeave={(e) => {
                if (!item.active) {
                  e.currentTarget.style.backgroundColor = 'transparent'
                  e.currentTarget.style.color = '#999'
                }
              }}
            >
              <span style={{ fontSize: '1.1rem' }}>{item.icon}</span>
              {item.label}
            </div>
          ))}
        </div>
      )}

      {/* 主内容区 */}
      <div style={{ flex: 1, overflow: 'auto' }}>
        {/* 顶部栏 */}
        <div style={{
          padding: '1.25rem 2rem',
          borderBottom: '1px solid rgba(255,255,255,0.05)',
          backgroundColor: '#0f0f0f',
          position: 'sticky',
          top: 0,
          zIndex: 100
        }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
              <button
                onClick={() => setShowSidebar(!showSidebar)}
                style={{
                  background: 'none',
                  border: 'none',
                  color: '#999',
                  cursor: 'pointer',
                  padding: '0.5rem'
                }}
              >
                <Menu size={20} />
              </button>
              <div>
                <h1 style={{ fontSize: '1.5rem', fontWeight: 600, margin: 0, display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <span style={{ fontSize: '1.3rem' }}>💰</span>
                  聪明钱
                </h1>
                <p style={{ fontSize: '0.85rem', color: '#666', margin: '0.25rem 0 0 0' }}>
                  美国顶级资本买什么就买什么 - 45 天滞后数据仅供对冲基金趋势参考，
                </p>
              </div>
            </div>

            <div style={{ display: 'flex', gap: '1rem', alignItems: 'center' }}>
              <div style={{
                position: 'relative',
                width: '300px'
              }}>
                <Search size={16} style={{
                  position: 'absolute',
                  left: '12px',
                  top: '50%',
                  transform: 'translateY(-50%)',
                  color: '#666'
                }} />
                <input
                  type="text"
                  placeholder="搜代码 / 名称 / 标签..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  style={{
                    width: '100%',
                    padding: '0.6rem 1rem 0.6rem 2.5rem',
                    backgroundColor: '#1a1a1a',
                    border: '1px solid rgba(255,255,255,0.08)',
                    borderRadius: '8px',
                    color: 'white',
                    fontSize: '0.85rem',
                    outline: 'none'
                  }}
                  onFocus={(e) => e.target.style.borderColor = '#f97316'}
                  onBlur={(e) => e.target.style.borderColor = 'rgba(255,255,255,0.08)'}
                />
              </div>
              <button style={{
                padding: '0.6rem 1.25rem',
                backgroundColor: '#f97316',
                border: 'none',
                borderRadius: '8px',
                color: 'white',
                fontSize: '0.9rem',
                fontWeight: 600,
                cursor: 'pointer'
              }}>
                Buy
              </button>
            </div>
          </div>

          {/* 标签切换 */}
          <div style={{ display: 'flex', gap: '1rem', alignItems: 'center' }}>
            <button
              onClick={() => setActiveNav('gurus')}
              style={{
                padding: '0.5rem 1rem',
                backgroundColor: activeNav === 'gurus' ? 'rgba(255,255,255,0.08)' : 'transparent',
                border: activeNav === 'gurus' ? '1px solid rgba(255,255,255,0.1)' : '1px solid transparent',
                borderRadius: '6px',
                color: activeNav === 'gurus' ? 'white' : '#999',
                fontSize: '0.85rem',
                fontWeight: 500,
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '0.5rem'
              }}
            >
              <span style={{ fontSize: '1rem' }}>📋</span>
              机构 13F
            </button>
            <button
              onClick={() => setActiveNav('congress')}
              style={{
                padding: '0.5rem 1rem',
                backgroundColor: activeNav === 'congress' ? 'rgba(255,255,255,0.08)' : 'transparent',
                border: activeNav === 'congress' ? '1px solid rgba(255,255,255,0.1)' : '1px solid transparent',
                borderRadius: '6px',
                color: activeNav === 'congress' ? 'white' : '#999',
                fontSize: '0.85rem',
                fontWeight: 500,
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '0.5rem'
              }}
            >
              <span style={{ fontSize: '1rem' }}>🏛️</span>
              国会 · {congressTrades.length}
            </button>
          </div>
        </div>

        {/* 内容区 */}
        <div style={{ padding: '2rem' }}>
          {activeNav === 'gurus' && (
            <div>
              {/* 分类筛选 */}
              <div style={{
                display: 'flex',
                gap: '0.75rem',
                marginBottom: '1.5rem',
                flexWrap: 'wrap'
              }}>
                {[
                  { key: 'all', label: '全部', icon: '📁' },
                  { key: 'us', label: '美股大佬', icon: '🇺🇸', color: '#3b82f6' },
                  { key: 'a_share', label: 'A股顶流', icon: '🇨🇳', color: '#ef4444' },
                  { key: 'private', label: '私募大佬', icon: '🏦', color: '#8b5cf6' },
                  { key: 'hot_money', label: '游资席位', icon: '🔥', color: '#f59e0b' }
                ].map((filter) => (
                  <button
                    key={filter.key}
                    onClick={() => setGuruFilter(filter.key as any)}
                    style={{
                      padding: '0.5rem 1.25rem',
                      backgroundColor: guruFilter === filter.key ? 'rgba(249, 115, 22, 0.15)' : 'rgba(255,255,255,0.03)',
                      border: `1px solid ${guruFilter === filter.key ? '#f97316' : 'rgba(255,255,255,0.08)'}`,
                      borderRadius: '8px',
                      color: guruFilter === filter.key ? '#f97316' : '#999',
                      fontSize: '0.85rem',
                      fontWeight: 500,
                      cursor: 'pointer',
                      display: 'flex',
                      alignItems: 'center',
                      gap: '0.5rem',
                      transition: 'all 0.2s'
                    }}
                  >
                    <span>{filter.icon}</span>
                    {filter.label}
                  </button>
                ))}
              </div>

              {/* 机构卡片网格 */}
              <div style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))',
                gap: '1rem'
              }}>
                {filteredGurus.map((guru) => (
                  <div
                    key={guru.id}
                    onClick={() => setExpandedGuru(expandedGuru === guru.id ? null : guru.id)}
                    style={{
                      backgroundColor: '#151515',
                      border: '1px solid rgba(255,255,255,0.06)',
                      borderRadius: '12px',
                      padding: '1.25rem',
                      cursor: 'pointer',
                      transition: 'all 0.2s',
                      position: 'relative',
                      overflow: 'hidden'
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.borderColor = '#f97316'
                      e.currentTarget.style.transform = 'translateY(-4px)'
                      e.currentTarget.style.boxShadow = '0 12px 24px rgba(249, 115, 22, 0.15)'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.borderColor = 'rgba(255,255,255,0.06)'
                      e.currentTarget.style.transform = 'translateY(0)'
                      e.currentTarget.style.boxShadow = 'none'
                    }}
                  >
                    {/* HOT 标签 */}
                    {guru.positionCount > 50 && (
                      <div style={{
                        position: 'absolute',
                        top: '0.75rem',
                        right: '0.75rem',
                        backgroundColor: '#ef4444',
                        color: 'white',
                        fontSize: '0.65rem',
                        fontWeight: 700,
                        padding: '0.25rem 0.5rem',
                        borderRadius: '4px',
                        textTransform: 'uppercase'
                      }}>
                        HOT
                      </div>
                    )}

                    {/* 头像 + 名称 */}
                    <div style={{ display: 'flex', alignItems: 'start', gap: '1rem', marginBottom: '1rem' }}>
                      <div style={{
                        width: '52px',
                        height: '52px',
                        borderRadius: '50%',
                        background: 'linear-gradient(135deg, #f97316 0%, #fb923c 100%)',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        fontSize: '1.3rem',
                        fontWeight: 700,
                        color: 'white',
                        flexShrink: 0
                      }}>
                        {guru.name.charAt(0)}
                      </div>
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <h3 style={{
                          fontSize: '0.95rem',
                          fontWeight: 600,
                          marginBottom: '0.35rem',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap'
                        }}>
                          {guru.name}
                        </h3>
                        <p style={{
                          fontSize: '0.75rem',
                          color: '#666',
                          margin: 0,
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap'
                        }}>
                          {guru.fundName}
                        </p>
                        <div style={{
                          fontSize: '0.7rem',
                          color: '#999',
                          marginTop: '0.25rem'
                        }}>
                          ${guru.aum} · {guru.positionCount} 持仓
                        </div>
                      </div>
                    </div>

                    {/* 第一重仓 */}
                    <div style={{
                      backgroundColor: 'rgba(16, 185, 129, 0.08)',
                      border: '1px solid rgba(16, 185, 129, 0.15)',
                      borderRadius: '8px',
                      padding: '0.75rem',
                      marginTop: '1rem'
                    }}>
                      <div style={{
                        display: 'flex',
                        justifyContent: 'space-between',
                        alignItems: 'center',
                        marginBottom: '0.5rem'
                      }}>
                        <span style={{
                          fontSize: '0.8rem',
                          fontWeight: 700,
                          color: '#10b981',
                          fontFamily: 'var(--font-mono)'
                        }}>
                          {guru.topStock}
                        </span>
                        <span style={{
                          fontSize: '0.85rem',
                          fontWeight: 700,
                          color: 'white'
                        }}>
                          {guru.topStockWeight.toFixed(2)}%
                        </span>
                      </div>
                      {/* 进度条 */}
                      <div style={{
                        width: '100%',
                        height: '4px',
                        backgroundColor: 'rgba(16, 185, 129, 0.1)',
                        borderRadius: '2px',
                        overflow: 'hidden'
                      }}>
                        <div style={{
                          width: `${guru.topStockWeight}%`,
                          height: '100%',
                          backgroundColor: '#10b981',
                          borderRadius: '2px'
                        }} />
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {activeNav === 'congress' && (
            <div>
              {/* 国会议员列表 */}
              {congressTrades.map((trade) => {
                const initials = getInitials(trade.politician)
                const avatarBg = trade.party === 'Democratic' ? '#3b82f6' : '#ef4444'
                
                return (
                  <div
                    key={trade.id}
                    style={{
                      backgroundColor: '#151515',
                      border: '1px solid rgba(255,255,255,0.06)',
                      borderRadius: '12px',
                      padding: '1.25rem',
                      marginBottom: '1rem',
                      cursor: 'pointer',
                      transition: 'all 0.2s'
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.borderColor = '#f97316'
                      e.currentTarget.style.transform = 'translateX(8px)'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.borderColor = 'rgba(255,255,255,0.06)'
                      e.currentTarget.style.transform = 'translateX(0)'
                    }}
                  >
                    <div style={{ display: 'flex', gap: '1.25rem', alignItems: 'center' }}>
                      {/* 头像 */}
                      <Avatar
                        name={trade.politician}
                        party={trade.party === 'Democratic' ? 'D' : 'R'}
                        size={56}
                      />

                      <div style={{ flex: 1 }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'start', marginBottom: '0.5rem' }}>
                          <div>
                            <h3 style={{ fontSize: '1.05rem', fontWeight: 600, marginBottom: '0.35rem' }}>
                              {trade.politician}
                            </h3>
                            <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
                              <span style={{
                                fontSize: '0.75rem',
                                color: avatarBg,
                                fontWeight: 600,
                                padding: '0.2rem 0.6rem',
                                backgroundColor: `${avatarBg}20`,
                                borderRadius: '4px'
                              }}>
                                {trade.party === 'Democratic' ? '民主党(D)' : '共和党(R)'}
                              </span>
                              <span style={{ fontSize: '0.8rem', color: '#999' }}>
                                {trade.title || 'Representative'}
                              </span>
                            </div>
                          </div>
                          <span style={{
                            padding: '0.35rem 0.8rem',
                            backgroundColor: trade.type === 'BUY' ? 'rgba(16, 185, 129, 0.2)' : 'rgba(239, 68, 68, 0.2)',
                            color: trade.type === 'BUY' ? '#10b981' : '#ef4444',
                            borderRadius: '6px',
                            fontSize: '0.8rem',
                            fontWeight: 700
                          }}>
                            {trade.type === 'BUY' ? '买入' : '卖出'}
                          </span>
                        </div>

                        <div style={{
                          display: 'grid',
                          gridTemplateColumns: '120px 1fr 100px',
                          gap: '1rem',
                          paddingTop: '0.75rem',
                          borderTop: '1px solid rgba(255,255,255,0.05)',
                          fontSize: '0.85rem'
                        }}>
                          <div style={{
                            fontFamily: 'var(--font-mono)',
                            fontWeight: 700,
                            color: '#10b981',
                            fontSize: '1rem'
                          }}>
                            {trade.symbol}
                          </div>
                          <div style={{ color: '#999' }}>
                            {trade.amount}
                          </div>
                          <div style={{ color: '#666', fontSize: '0.8rem', textAlign: 'right' }}>
                            {trade.date}
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>

      {/* 内联样式 */}
      <style>{`
        * {
          box-sizing: border-box;
        }
        
        ::-webkit-scrollbar {
          width: 8px;
          height: 8px;
        }
        
        ::-webkit-scrollbar-track {
          background: #0a0a0a;
        }
        
        ::-webkit-scrollbar-thumb {
          background: #333;
          border-radius: 4px;
        }
        
        ::-webkit-scrollbar-thumb:hover {
          background: #555;
        }
      `}</style>
    </div>
  )
}
