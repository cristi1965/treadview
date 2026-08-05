import React, { useState, useEffect } from 'react'
import { useCommandStore } from '../stores/commandStore'
import { useWhalesStore } from '../stores/whalesStore'
import { WhalesBoard } from '../components/WhalesBoard'
import {
  TrendingUp, Star, Calendar, AlertTriangle,
  Info, Moon, User, Users, Layers, Activity
} from 'lucide-react'
import {
  ResponsiveContainer, AreaChart, Area, XAxis, YAxis, Tooltip,
  BarChart, Bar, Cell
} from 'recharts'

export const MarketBoard: React.FC = () => {
  const {
    trades, stats, events,
    fetchTrades, fetchStats, fetchEvents, addTrade, addEvent
  } = useCommandStore()

  const {
    gurus, congressTrades, fetchGurus, fetchCongressTrades
  } = useWhalesStore()

  // Tab State
  const [activeSubTab, setActiveSubTab] = useState<'market' | 'control' | 'whales'>('market')

  // Trade Form State
  const [symbol, setSymbol] = useState('')
  const [direction, setDirection] = useState<'BUY' | 'SELL'>('BUY')
  const [entryPrice, setEntryPrice] = useState('')
  const [exitPrice, setExitPrice] = useState('')
  const [shares, setShares] = useState('')
  const [pnl, setPnl] = useState('')
  const [emotionScore, setEmotionScore] = useState(5)
  const [notes, setNotes] = useState('')

  // Event Form State
  const [eventName, setEventName] = useState('')
  const [eventDate, setEventDate] = useState('')
  const [eventTime, setEventTime] = useState('')
  const [eventStars, setEventStars] = useState(1)
  const [eventPrevious, setEventPrevious] = useState('')
  const [eventConsensus, setEventConsensus] = useState('')

  // Position Sizer State
  const [totalCapital, setTotalCapital] = useState(100000)
  const [riskPercent, setRiskPercent] = useState(1)
  const [sizerEntry, setSizerEntry] = useState(150)
  const [sizerStop, setSizerStop] = useState(145)

  // Live Night Market State
  const [nightMarket, setNightMarket] = useState([
    { name: '富时中国A50期指', value: 12345.5, change: 0.45, status: 'stable' },
    { name: '纳斯达克100期指', value: 18920.2, change: 0.62, status: 'stable' },
    { name: '标普500期指', value: 5510.8, change: 0.35, status: 'stable' },
    { name: '道指期指', value: 39850.5, change: 0.12, status: 'stable' }
  ])

  // Mock static data for Capital Flow
  const capitalFlowData = [
    { time: '09:30', Inflow: 120, Outflow: 100, Net: 20 },
    { time: '10:30', Inflow: 250, Outflow: 280, Net: -30 },
    { time: '11:30', Inflow: 410, Outflow: 380, Net: 30 },
    { time: '13:30', Inflow: 580, Outflow: 520, Net: 60 },
    { time: '14:30', Inflow: 790, Outflow: 720, Net: 70 },
    { time: '15:30', Inflow: 980, Outflow: 890, Net: 90 }
  ]

  // Mock static data for Sector Concentration
  const sectorData = [
    { name: '人工智能/科技', value: 38, color: '#6366f1' },
    { name: '金融与银行', value: 22, color: '#3b82f6' },
    { name: '生物医药', value: 16, color: '#10b981' },
    { name: '新能源/半导体', value: 14, color: '#f59e0b' },
    { name: '军工/原材料', value: 10, color: '#f43f5e' }
  ]



  useEffect(() => {
    fetchTrades()
    fetchStats()
    fetchEvents()
    fetchGurus()
    fetchCongressTrades()
  }, [])

  // Night Market tick simulator
  useEffect(() => {
    const interval = setInterval(() => {
      setNightMarket(prev => prev.map(item => {
        const rand = (Math.random() - 0.5) * 0.1 // -0.05% to +0.05%
        const newValue = item.value * (1 + rand / 100)
        const newChange = item.change + rand
        const status = rand > 0 ? 'up' : 'down'
        return {
          ...item,
          value: parseFloat(newValue.toFixed(1)),
          change: parseFloat(newChange.toFixed(2)),
          status
        }
      }))
    }, 3000)

    return () => clearInterval(interval)
  }, [])

  // Auto calculate PnL in form
  useEffect(() => {
    if (entryPrice && exitPrice && shares) {
      const ep = Number(entryPrice)
      const xp = Number(exitPrice)
      const sh = Number(shares)
      const calculatedPnl = direction === 'BUY'
        ? (xp - ep) * sh
        : (ep - xp) * sh
      setPnl(calculatedPnl.toFixed(2))
    }
  }, [entryPrice, exitPrice, shares, direction])

  const handleAddTrade = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!symbol || !entryPrice || !exitPrice || !shares) return

    await addTrade({
      symbol: symbol.toUpperCase(),
      direction,
      entryPrice: Number(entryPrice),
      exitPrice: Number(exitPrice),
      shares: Number(shares),
      pnl: Number(pnl),
      emotionScore: Number(emotionScore),
      notes
    })

    // Reset Form
    setSymbol('')
    setEntryPrice('')
    setExitPrice('')
    setShares('')
    setPnl('')
    setNotes('')
  }

  const handleAddEvent = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!eventName || !eventDate) return

    await addEvent({
      name: eventName,
      date: eventDate,
      time: eventTime || '00:00',
      stars: Number(eventStars),
      previous: eventPrevious,
      consensus: eventConsensus
    })

    // Reset Form
    setEventName('')
    setEventDate('')
    setEventTime('')
    setEventStars(1)
    setEventPrevious('')
    setEventConsensus('')
  }

  // Position Sizer calculation
  const sizerResult = (() => {
    if (sizerEntry === sizerStop || sizerEntry <= 0 || sizerStop <= 0) return null
    const riskAmount = totalCapital * (riskPercent / 100)
    const priceRisk = Math.abs(sizerEntry - sizerStop)
    const suggestedShares = riskAmount / priceRisk
    const totalCost = suggestedShares * sizerEntry
    const leverage = totalCost / totalCapital
    return {
      shares: suggestedShares,
      totalCost,
      leverage,
      priceRisk,
      riskAmount
    }
  })()

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
      {/* Sub tabs */}
      <div style={{
        display: 'flex',
        borderBottom: '1px solid var(--border-color)',
        paddingBottom: '0.5rem',
        gap: '2rem'
      }}>
        <button
          onClick={() => setActiveSubTab('market')}
          style={{
            background: 'none',
            border: 'none',
            borderBottom: activeSubTab === 'market' ? '3px solid var(--color-primary)' : '3px solid transparent',
            color: activeSubTab === 'market' ? 'white' : 'var(--text-secondary)',
            padding: '0.5rem 1rem',
            fontSize: '1rem',
            fontWeight: 600,
            cursor: 'pointer',
            transition: 'all 0.2s ease',
            display: 'flex',
            alignItems: 'center',
            gap: '0.5rem'
          }}
        >
          <Activity size={18} /> 大盘核心数据 (Market Board)
        </button>
        <button
          onClick={() => setActiveSubTab('control')}
          style={{
            background: 'none',
            border: 'none',
            borderBottom: activeSubTab === 'control' ? '3px solid var(--color-primary)' : '3px solid transparent',
            color: activeSubTab === 'control' ? 'white' : 'var(--text-secondary)',
            padding: '0.5rem 1rem',
            fontSize: '1rem',
            fontWeight: 600,
            cursor: 'pointer',
            transition: 'all 0.2s ease',
            display: 'flex',
            alignItems: 'center',
            gap: '0.5rem'
          }}
        >
          <Layers size={18} /> 交易风控中心 (Command Center)
        </button>
        <button
          onClick={() => setActiveSubTab('whales')}
          style={{
            background: 'none',
            border: 'none',
            borderBottom: activeSubTab === 'whales' ? '3px solid #f97316' : '3px solid transparent',
            color: activeSubTab === 'whales' ? '#f97316' : 'var(--text-secondary)',
            padding: '0.5rem 1rem',
            fontSize: '1rem',
            fontWeight: 600,
            cursor: 'pointer',
            transition: 'all 0.2s ease',
            display: 'flex',
            alignItems: 'center',
            gap: '0.5rem'
          }}
        >
          <Users size={18} /> 资金巨头 (Whales)
        </button>
      </div>

      {/* Market Board View */}
      {activeSubTab === 'market' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
          {/* Top Panel: Night Market Indices & Capital Flow Chart */}
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))',
            gap: '1.5rem'
          }}>
            {/* Night Market Card */}
            <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <Moon size={18} style={{ color: 'var(--color-info)' }} />
                <h3 style={{ fontSize: '1.1rem', fontWeight: 600, color: 'white' }}>夜盘指数期货 (Live)</h3>
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
                {nightMarket.map((item, index) => (
                  <div key={index} style={{
                    backgroundColor: 'rgba(255,255,255,0.02)',
                    border: '1px solid var(--border-color)',
                    padding: '1rem',
                    borderRadius: '0.75rem',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '0.5rem',
                    transition: 'background-color 0.3s ease',
                    position: 'relative',
                    overflow: 'hidden'
                  }}>
                    {/* Flashing glow indicator based on up/down status */}
                    <div style={{
                      position: 'absolute',
                      left: 0,
                      top: 0,
                      bottom: 0,
                      width: '4px',
                      backgroundColor: item.change >= 0 ? 'var(--color-success)' : 'var(--color-danger)'
                    }} />
                    <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>{item.name}</span>
                    <span style={{ fontSize: '1.4rem', fontWeight: 700, fontFamily: 'var(--font-mono)' }}>
                      {item.value.toLocaleString()}
                    </span>
                    <span style={{
                      fontSize: '0.85rem',
                      fontWeight: 600,
                      color: item.change >= 0 ? 'var(--color-success)' : 'var(--color-danger)'
                    }}>
                      {item.change >= 0 ? '▲' : '▼'} {Math.abs(item.change)}%
                    </span>
                  </div>
                ))}
              </div>
            </div>

            {/* Capital Flow Card */}
            <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <TrendingUp size={18} style={{ color: 'var(--color-success)' }} />
                  <h3 style={{ fontSize: '1.1rem', fontWeight: 600, color: 'white' }}>主力资金流向趋势 (M)</h3>
                </div>
                <span className="badge badge-buy">主力净流入: +90.00M</span>
              </div>
              <div style={{ width: '100%', height: 180 }}>
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={capitalFlowData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                    <XAxis dataKey="time" stroke="var(--text-muted)" fontSize={10} />
                    <YAxis stroke="var(--text-muted)" fontSize={10} />
                    <Tooltip contentStyle={{ backgroundColor: 'var(--bg-dark)', borderColor: 'var(--border-color)' }} />
                    <Area type="monotone" dataKey="Inflow" name="流入" stroke="var(--color-success)" fill="rgba(16, 185, 129, 0.1)" strokeWidth={2} />
                    <Area type="monotone" dataKey="Outflow" name="流出" stroke="var(--color-danger)" fill="rgba(244, 63, 94, 0.1)" strokeWidth={2} />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </div>
          </div>

          {/* Middle Panel: Sector Concentration & Holdings */}
          <div style={{
            display: 'grid',
            gridTemplateColumns: '1fr 1fr',
            gap: '1.5rem',
            alignItems: 'start'
          }}>
            {/* Sector Concentration */}
            <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <Layers size={18} style={{ color: 'var(--color-primary)' }} />
                <h3 style={{ fontSize: '1.1rem', fontWeight: 600, color: 'white' }}>行业板块资金集中度 (%)</h3>
              </div>
              <div style={{ width: '100%', height: 200 }}>
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={sectorData} layout="vertical" margin={{ top: 10, right: 10, left: 30, bottom: 0 }}>
                    <XAxis type="number" stroke="var(--text-muted)" fontSize={10} />
                    <YAxis type="category" dataKey="name" stroke="var(--text-muted)" fontSize={10} width={90} />
                    <Tooltip contentStyle={{ backgroundColor: 'var(--bg-dark)', borderColor: 'var(--border-color)' }} />
                    <Bar dataKey="value" name="权重 %" radius={[0, 4, 4, 0]}>
                      {sectorData.map((entry, index) => (
                        <Cell key={`cell-${index}`} fill={entry.color} />
                      ))}
                    </Bar>
                  </BarChart>
                </ResponsiveContainer>
              </div>
            </div>

            {/* Politician Holdings */}
            <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <User size={18} style={{ color: 'var(--color-warning)' }} />
                <h3 style={{ fontSize: '1.1rem', fontWeight: 600, color: 'white' }}>政府官员最新交易持股 (已接管)</h3>
              </div>
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--border-color)', color: 'var(--text-secondary)', textAlign: 'left' }}>
                      <th style={{ padding: '0.5rem 0' }}>官员</th>
                      <th style={{ padding: '0.5rem 0' }}>标的</th>
                      <th style={{ padding: '0.5rem 0' }}>类型</th>
                      <th style={{ padding: '0.5rem 0' }}>交易金额</th>
                      <th style={{ padding: '0.5rem 0' }}>交易日期</th>
                    </tr>
                  </thead>
                  <tbody>
                    {congressTrades.slice(0, 5).map((item, index) => (
                      <tr key={index} style={{ borderBottom: '1px solid rgba(255,255,255,0.02)', color: 'var(--text-primary)' }}>
                        <td style={{ padding: '0.65rem 0' }}>
                          <div style={{ fontWeight: 600 }}>{item.politician}</div>
                          <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>{item.title}</div>
                        </td>
                        <td style={{ padding: '0.65rem 0', fontFamily: 'var(--font-mono)', fontWeight: 700 }}>{item.symbol}</td>
                        <td style={{ padding: '0.65rem 0' }}>
                          <span style={{
                            padding: '0.15rem 0.4rem',
                            borderRadius: '4px',
                            fontSize: '0.75rem',
                            fontWeight: 'bold',
                            backgroundColor: item.type === 'BUY' ? 'rgba(16, 185, 129, 0.15)' : 'rgba(244, 63, 94, 0.15)',
                            color: item.type === 'BUY' ? 'var(--color-success)' : 'var(--color-danger)'
                          }}>{item.type === 'BUY' ? '买入' : '卖出'}</span>
                        </td>
                        <td style={{ padding: '0.65rem 0', fontFamily: 'var(--font-mono)' }}>{item.amount}</td>
                        <td style={{ padding: '0.65rem 0', color: 'var(--text-secondary)' }}>{item.date}</td>
                      </tr>
                    ))}
                    {congressTrades.length === 0 && (
                      <tr>
                        <td colSpan={5} style={{ padding: '2rem 0', textAlign: 'center', color: 'var(--text-muted)' }}>
                          数据加载中，正在后台同步真实国会买卖...
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          {/* Super Investor Holdings */}
          <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <Users size={18} style={{ color: 'var(--color-info)' }} />
              <h3 style={{ fontSize: '1.1rem', fontWeight: 600, color: 'white' }}>著名投资机构最新季度持仓 (13F已接管)</h3>
            </div>
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.875rem' }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid var(--border-color)', color: 'var(--text-secondary)', textAlign: 'left' }}>
                    <th style={{ padding: '0.75rem' }}>机构名称</th>
                    <th style={{ padding: '0.75rem' }}>第一重仓股</th>
                    <th style={{ padding: '0.75rem' }}>持仓股数</th>
                    <th style={{ padding: '0.75rem' }}>管理总规模 (AUM)</th>
                    <th style={{ padding: '0.75rem', textAlign: 'right' }}>重仓股权重</th>
                  </tr>
                </thead>
                <tbody>
                  {gurus.slice(0, 5).map((item, index) => (
                    <tr key={index} style={{ borderBottom: '1px solid rgba(255,255,255,0.02)', color: 'var(--text-primary)' }}>
                      <td style={{ padding: '0.75rem', fontWeight: 600 }}>{item.name}</td>
                      <td style={{ padding: '0.75rem', fontFamily: 'var(--font-mono)', fontWeight: 700 }}>{item.topStock}</td>
                      <td style={{ padding: '0.75rem', fontFamily: 'var(--font-mono)' }}>{item.positionCount} 只股票</td>
                      <td style={{ padding: '0.75rem', fontFamily: 'var(--font-mono)' }}>{item.aum}</td>
                      <td style={{
                        padding: '0.75rem',
                        textAlign: 'right',
                        fontWeight: 600,
                        color: 'var(--color-success)'
                      }}>{item.topStockWeight.toFixed(1)}%</td>
                    </tr>
                  ))}
                  {gurus.length === 0 && (
                    <tr>
                      <td colSpan={5} style={{ padding: '2.5rem', textAlign: 'center', color: 'var(--text-muted)' }}>
                        数据加载中，正在从 StockGod.xyz 同步最新季度申报...
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* Command Center View */}
      {activeSubTab === 'control' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
          {/* Top Panel: Metrics & Position Sizer */}
          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))',
            gap: '1.5rem',
            alignItems: 'start'
          }}>
            {/* Account Metrics */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
              <div style={{
                display: 'grid',
                gridTemplateColumns: '1fr 1fr',
                gap: '1rem'
              }}>
                <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                  <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>累计净损益</span>
                  <span style={{
                    fontSize: '1.6rem',
                    fontWeight: 700,
                    fontFamily: 'var(--font-mono)',
                    color: (stats?.totalPnL || 0) >= 0 ? 'var(--color-success)' : 'var(--color-danger)'
                  }}>
                    ${(stats?.totalPnL || 0).toLocaleString(undefined, { minimumFractionDigits: 2 })}
                  </span>
                  <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>本季度实现回报</span>
                </div>
                <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                  <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>风控胜率</span>
                  <span style={{ fontSize: '1.6rem', fontWeight: 700, fontFamily: 'var(--font-mono)', color: 'var(--color-info)' }}>
                    {((stats?.winRate || 0) * 100).toFixed(1)}%
                  </span>
                  <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>赢利笔数 / 交易笔数</span>
                </div>
              </div>

              {/* Position Sizer Card */}
              <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', borderBottom: '1px solid var(--border-color)', paddingBottom: '0.5rem' }}>
                  <TrendingUp size={18} style={{ color: 'var(--color-primary)' }} />
                  <h3 style={{ fontSize: '1rem', fontWeight: 600, color: 'white' }}>输入计划交易参数</h3>
                </div>

                <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <label style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>账户总资金 (USD)</label>
                    <span style={{ fontSize: '0.8rem', fontFamily: 'var(--font-mono)', color: 'white' }}>
                      ${totalCapital.toLocaleString()}
                    </span>
                  </div>
                  <input
                    type="number"
                    value={totalCapital}
                    onChange={(e) => setTotalCapital(Number(e.target.value))}
                    className="text-input"
                    style={{ padding: '0.5rem', fontSize: '0.85rem' }}
                  />

                  <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                    <label style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>单笔风险率 ({riskPercent}%)</label>
                    <span style={{ fontSize: '0.8rem', fontFamily: 'var(--font-mono)', color: 'var(--color-danger)' }}>
                      风险额: ${(totalCapital * riskPercent / 100).toFixed(2)}
                    </span>
                  </div>
                  <input
                    type="range"
                    min="0.1"
                    max="10"
                    step="0.1"
                    value={riskPercent}
                    onChange={(e) => setRiskPercent(Number(e.target.value))}
                    style={{ accentColor: 'var(--color-primary)' }}
                  />

                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '0.35rem' }}>
                      <label style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>开仓买入价</label>
                      <input
                        type="number"
                        step="0.01"
                        value={sizerEntry}
                        onChange={(e) => setSizerEntry(Number(e.target.value))}
                        className="text-input"
                        style={{ padding: '0.5rem', fontSize: '0.85rem' }}
                      />
                    </div>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '0.35rem' }}>
                      <label style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>止损价</label>
                      <input
                        type="number"
                        step="0.01"
                        value={sizerStop}
                        onChange={(e) => setSizerStop(Number(e.target.value))}
                        className="text-input"
                        style={{ padding: '0.5rem', fontSize: '0.85rem' }}
                      />
                    </div>
                  </div>
                </div>
              </div>
            </div>

            {/* Position Sizer Results Card */}
            <div className="card" style={{ display: 'flex', flexDirection: 'column', justifyContent: 'space-between', height: '100%' }}>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', borderBottom: '1px solid var(--border-color)', paddingBottom: '0.5rem' }}>
                  <Info size={18} style={{ color: 'var(--color-success)' }} />
                  <h3 style={{ fontSize: '1rem', fontWeight: 600, color: 'white' }}>风控仓位测算结果</h3>
                </div>

                {sizerResult ? (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
                    <div>
                      <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', display: 'block' }}>建议最大股数/头寸</span>
                      <span style={{ fontSize: '2.5rem', fontWeight: 800, fontFamily: 'var(--font-mono)', color: 'var(--color-success)' }}>
                        {Math.floor(sizerResult.shares).toLocaleString()}
                      </span>
                    </div>

                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', borderTop: '1px solid var(--border-color)', paddingTop: '1rem' }}>
                      <div>
                        <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', display: 'block' }}>所需资金规模</span>
                        <span style={{ fontSize: '1rem', fontWeight: 700, fontFamily: 'var(--font-mono)', color: 'white' }}>
                          ${sizerResult.totalCost.toLocaleString(undefined, { maximumFractionDigits: 2 })}
                        </span>
                      </div>
                      <div>
                        <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', display: 'block' }}>名义杠杆倍数</span>
                        <span style={{
                          fontSize: '1rem',
                          fontWeight: 700,
                          fontFamily: 'var(--font-mono)',
                          color: sizerResult.leverage > 1 ? 'var(--color-danger)' : 'white'
                        }}>
                          {sizerResult.leverage.toFixed(2)}x
                        </span>
                      </div>
                    </div>

                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
                      <div>
                        <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', display: 'block' }}>波动点值风险</span>
                        <span style={{ fontSize: '0.9rem', fontWeight: 600, fontFamily: 'var(--font-mono)', color: 'var(--color-danger)' }}>
                          ${sizerResult.priceRisk.toFixed(2)} ({(sizerResult.priceRisk / sizerEntry * 100).toFixed(2)}%)
                        </span>
                      </div>
                      <div>
                        <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', display: 'block' }}>预计最坏亏损额</span>
                        <span style={{ fontSize: '0.9rem', fontWeight: 600, fontFamily: 'var(--font-mono)', color: 'var(--color-danger)' }}>
                          ${sizerResult.riskAmount.toFixed(2)}
                        </span>
                      </div>
                    </div>
                  </div>
                ) : (
                  <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', padding: '3rem 0', color: 'var(--text-muted)', gap: '0.5rem' }}>
                    <Info size={32} style={{ strokeWidth: 1.5 }} />
                    <span style={{ fontSize: '0.8rem' }}>请输入开仓价与不同的止损价以测算</span>
                  </div>
                )}
              </div>

              {sizerResult && sizerResult.leverage > 1 && (
                <div style={{
                  marginTop: '1rem',
                  backgroundColor: 'rgba(244, 63, 94, 0.1)',
                  border: '1px solid rgba(244, 63, 94, 0.2)',
                  borderRadius: '0.5rem',
                  padding: '0.75rem',
                  display: 'flex',
                  gap: '0.5rem',
                  alignItems: 'start'
                }}>
                  <AlertTriangle size={16} color="var(--color-danger)" style={{ flexShrink: 0, marginTop: '2px' }} />
                  <div>
                    <h4 style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--color-danger)' }}>资金越限警报</h4>
                    <p style={{ fontSize: '0.75rem', color: 'rgba(244, 63, 94, 0.8)', marginTop: '0.25rem' }}>
                      所需开仓本金超出总资金，该头寸需要放大为 {sizerResult.leverage.toFixed(2)} 倍融资杠杆。
                    </p>
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Macro Calendar Panel */}
          <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', borderBottom: '1px solid var(--border-color)', paddingBottom: '0.5rem' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <Calendar size={18} style={{ color: 'var(--color-info)' }} />
                <h3 style={{ fontSize: '1rem', fontWeight: 600, color: 'white' }}>本周重要宏观事件预警</h3>
              </div>
            </div>

            {/* Macro form & list */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '1.5rem', alignItems: 'start' }}>
              {/* Form to add macroeconomic event */}
              <form onSubmit={handleAddEvent} style={{
                backgroundColor: 'rgba(255,255,255,0.01)',
                border: '1px solid var(--border-color)',
                borderRadius: '0.75rem',
                padding: '1rem',
                display: 'flex',
                flexDirection: 'column',
                gap: '0.75rem'
              }}>
                <span style={{ fontSize: '0.85rem', fontWeight: 600, color: 'white' }}>录入宏观事件</span>

                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                  <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>事件名称</label>
                  <input
                    type="text"
                    required
                    placeholder="如: 美联储利率决议"
                    value={eventName}
                    onChange={(e) => setEventName(e.target.value)}
                    className="text-input"
                    style={{ padding: '0.4rem', fontSize: '0.8rem' }}
                  />
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.5rem' }}>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                    <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>日期</label>
                    <input
                      type="date"
                      required
                      value={eventDate}
                      onChange={(e) => setEventDate(e.target.value)}
                      className="text-input"
                      style={{ padding: '0.4rem', fontSize: '0.8rem', fontFamily: 'var(--font-mono)' }}
                    />
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                    <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>时间 (可选)</label>
                    <input
                      type="time"
                      value={eventTime}
                      onChange={(e) => setEventTime(e.target.value)}
                      className="text-input"
                      style={{ padding: '0.4rem', fontSize: '0.8rem', fontFamily: 'var(--font-mono)' }}
                    />
                  </div>
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.5rem' }}>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                    <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>前值</label>
                    <input
                      type="text"
                      placeholder="5.25%"
                      value={eventPrevious}
                      onChange={(e) => setEventPrevious(e.target.value)}
                      className="text-input"
                      style={{ padding: '0.4rem', fontSize: '0.8rem' }}
                    />
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                    <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>预期</label>
                    <input
                      type="text"
                      placeholder="5.00%"
                      value={eventConsensus}
                      onChange={(e) => setEventConsensus(e.target.value)}
                      className="text-input"
                      style={{ padding: '0.4rem', fontSize: '0.8rem' }}
                    />
                  </div>
                </div>

                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                  <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>重要星级 (1-3)</label>
                  <select
                    value={eventStars}
                    onChange={(e) => setEventStars(Number(e.target.value))}
                    className="text-input"
                    style={{ padding: '0.4rem', fontSize: '0.8rem' }}
                  >
                    <option value={1}>1 星 (低影响)</option>
                    <option value={2}>2 星 (中等影响)</option>
                    <option value={3}>3 星 (高影响决议)</option>
                  </select>
                </div>

                <button type="submit" className="btn btn-primary" style={{ padding: '0.4rem 1rem', fontSize: '0.8rem', marginTop: '0.25rem' }}>
                  保存宏观警报
                </button>
              </form>

              {/* Event Timeline List */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem', maxHeight: '350px', overflowY: 'auto' }}>
                {events.map((event) => (
                  <div key={event.id} style={{
                    backgroundColor: 'rgba(255,255,255,0.02)',
                    border: '1px solid var(--border-color)',
                    borderRadius: '0.5rem',
                    padding: '0.75rem',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '0.4rem'
                  }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                        <span style={{
                          backgroundColor: 'var(--bg-darker)',
                          padding: '0.2rem 0.5rem',
                          borderRadius: '4px',
                          fontSize: '0.7rem',
                          fontFamily: 'var(--font-mono)'
                        }}>{event.date} {event.time}</span>
                        <span style={{ fontSize: '0.85rem', fontWeight: 600, color: 'white' }}>{event.name}</span>
                      </div>
                      <div style={{ display: 'flex' }}>
                        {Array.from({ length: 3 }).map((_, i) => (
                          <Star key={i} size={12} fill={i < event.stars ? 'var(--color-warning)' : 'transparent'} color={i < event.stars ? 'var(--color-warning)' : 'var(--text-muted)'} />
                        ))}
                      </div>
                    </div>
                    <div style={{ display: 'flex', gap: '1.5rem', fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                      <span>前值: <strong style={{ color: 'white', fontFamily: 'var(--font-mono)' }}>{event.previous || '—'}</strong></span>
                      <span>预测: <strong style={{ color: 'white', fontFamily: 'var(--font-mono)' }}>{event.consensus || '—'}</strong></span>
                    </div>
                  </div>
                ))}
                {events.length === 0 && (
                  <div style={{ textAlign: 'center', padding: '2rem 0', color: 'var(--text-muted)', fontSize: '0.8rem' }}>
                    暂无宏观预警事件，请在左侧添加。
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* Trade Journal Table */}
          <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', borderBottom: '1px solid var(--border-color)', paddingBottom: '0.5rem' }}>
              <Activity size={18} style={{ color: 'var(--color-success)' }} />
              <h3 style={{ fontSize: '1rem', fontWeight: 600, color: 'white' }}>交易复盘日志表</h3>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '1.5rem', alignItems: 'start' }}>
              {/* Form to record trade */}
              <form onSubmit={handleAddTrade} style={{
                backgroundColor: 'rgba(255,255,255,0.01)',
                border: '1px solid var(--border-color)',
                borderRadius: '0.75rem',
                padding: '1rem',
                display: 'flex',
                flexDirection: 'column',
                gap: '0.75rem'
              }}>
                <span style={{ fontSize: '0.85rem', fontWeight: 600, color: 'white' }}>录入最新风控交易</span>

                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.5rem' }}>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                    <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>标的代码</label>
                    <input
                      type="text"
                      required
                      placeholder="AAPL"
                      value={symbol}
                      onChange={(e) => setSymbol(e.target.value)}
                      className="text-input"
                      style={{ padding: '0.4rem', fontSize: '0.8rem', textTransform: 'uppercase', fontFamily: 'var(--font-mono)' }}
                    />
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                    <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>交易方向</label>
                    <select
                      value={direction}
                      onChange={(e) => setDirection(e.target.value as 'BUY' | 'SELL')}
                      className="text-input"
                      style={{ padding: '0.4rem', fontSize: '0.8rem' }}
                    >
                      <option value="BUY">买入 (Long)</option>
                      <option value="SELL">卖出 (Short)</option>
                    </select>
                  </div>
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '0.5rem' }}>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                    <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>开仓价</label>
                    <input
                      type="number"
                      step="0.01"
                      required
                      value={entryPrice}
                      onChange={(e) => setEntryPrice(e.target.value)}
                      className="text-input"
                      style={{ padding: '0.4rem', fontSize: '0.8rem', fontFamily: 'var(--font-mono)' }}
                    />
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                    <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>平仓价</label>
                    <input
                      type="number"
                      step="0.01"
                      required
                      value={exitPrice}
                      onChange={(e) => setExitPrice(e.target.value)}
                      className="text-input"
                      style={{ padding: '0.4rem', fontSize: '0.8rem', fontFamily: 'var(--font-mono)' }}
                    />
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                    <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>成交头寸</label>
                    <input
                      type="number"
                      required
                      value={shares}
                      onChange={(e) => setShares(e.target.value)}
                      className="text-input"
                      style={{ padding: '0.4rem', fontSize: '0.8rem', fontFamily: 'var(--font-mono)' }}
                    />
                  </div>
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.5rem' }}>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                    <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>实现损益 (PnL)</label>
                    <input
                      type="number"
                      step="0.01"
                      required
                      value={pnl}
                      onChange={(e) => setPnl(e.target.value)}
                      className="text-input"
                      style={{ padding: '0.4rem', fontSize: '0.8rem', fontFamily: 'var(--font-mono)' }}
                    />
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                    <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>情绪得分 ({emotionScore})</label>
                    <input
                      type="range"
                      min="1"
                      max="10"
                      value={emotionScore}
                      onChange={(e) => setEmotionScore(Number(e.target.value))}
                      style={{ accentColor: 'var(--color-primary)', marginTop: '0.5rem' }}
                    />
                  </div>
                </div>

                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                  <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>复盘备注</label>
                  <textarea
                    rows={2}
                    value={notes}
                    onChange={(e) => setNotes(e.target.value)}
                    placeholder="开仓逻辑..."
                    className="text-input"
                    style={{ padding: '0.4rem', fontSize: '0.8rem' }}
                  />
                </div>

                <button type="submit" className="btn btn-primary" style={{ padding: '0.4rem 1rem', fontSize: '0.8rem', marginTop: '0.25rem' }}>
                  录入交易记录
                </button>
              </form>

              {/* Trade Log List Table */}
              <div style={{ overflowX: 'auto', maxHeight: '420px', overflowY: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--border-color)', color: 'var(--text-secondary)', textAlign: 'left' }}>
                      <th style={{ padding: '0.5rem' }}>代码</th>
                      <th style={{ padding: '0.5rem' }}>方向</th>
                      <th style={{ padding: '0.5rem' }}>开平仓价</th>
                      <th style={{ padding: '0.5rem' }}>头寸</th>
                      <th style={{ padding: '0.5rem', textAlign: 'right' }}>损益 (USD)</th>
                    </tr>
                  </thead>
                  <tbody>
                    {trades.map((trade) => (
                      <tr key={trade.id} style={{ borderBottom: '1px solid rgba(255,255,255,0.02)', color: 'var(--text-primary)' }}>
                        <td style={{ padding: '0.65rem 0.5rem', fontFamily: 'var(--font-mono)', fontWeight: 700 }}>{trade.symbol}</td>
                        <td style={{ padding: '0.65rem 0.5rem' }}>
                          <span className={trade.direction === 'BUY' ? 'badge badge-buy' : 'badge badge-sell'} style={{ padding: '0.15rem 0.4rem', fontSize: '0.7rem' }}>
                            {trade.direction === 'BUY' ? '买入' : '卖出'}
                          </span>
                        </td>
                        <td style={{ padding: '0.65rem 0.5rem', fontFamily: 'var(--font-mono)' }}>
                          ${trade.entryPrice.toFixed(2)} / ${trade.exitPrice.toFixed(2)}
                        </td>
                        <td style={{ padding: '0.65rem 0.5rem', fontFamily: 'var(--font-mono)' }}>{trade.shares}</td>
                        <td style={{
                          padding: '0.65rem 0.5rem',
                          textAlign: 'right',
                          fontWeight: 700,
                          fontFamily: 'var(--font-mono)',
                          color: trade.pnl >= 0 ? 'var(--color-success)' : 'var(--color-danger)'
                        }}>
                          {trade.pnl >= 0 ? '+' : ''}{trade.pnl.toLocaleString(undefined, { minimumFractionDigits: 2 })}
                        </td>
                      </tr>
                    ))}
                    {trades.length === 0 && (
                      <tr>
                        <td colSpan={5} style={{ textAlign: 'center', padding: '2rem 0', color: 'var(--text-muted)' }}>
                          暂无交易日志记录。
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

      {activeSubTab === 'whales' && <WhalesBoard />}
    </div>
  )
}
