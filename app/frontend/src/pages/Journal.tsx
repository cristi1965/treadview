import React, { useState, useEffect } from 'react'
import { useCommandStore } from '../stores/commandStore'
import { Plus } from 'lucide-react'

export const Journal: React.FC = () => {
  const { trades, addTrade, fetchTrades, fetchStats, stats } = useCommandStore()

  // Form State
  const [symbol, setSymbol] = useState('')
  const [direction, setDirection] = useState<'BUY' | 'SELL'>('BUY')
  const [entryPrice, setEntryPrice] = useState('')
  const [exitPrice, setExitPrice] = useState('')
  const [shares, setShares] = useState('')
  const [pnl, setPnl] = useState('')
  const [emotionScore, setEmotionScore] = useState(5)
  const [notes, setNotes] = useState('')

  useEffect(() => {
    fetchTrades()
    fetchStats()
  }, [])

  // Auto calculate PnL when entry/exit/shares change
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

  const handleSubmit = async (e: React.FormEvent) => {
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
      notes,
    })

    // Reset Form
    setSymbol('')
    setEntryPrice('')
    setExitPrice('')
    setShares('')
    setPnl('')
    setNotes('')
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
      {/* Page Header */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
        <h2 style={{ fontSize: '1.5rem', fontWeight: 700, color: 'white' }}>交易复盘日志 (Trading Journal)</h2>
        <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
          系统化记录每笔真实或模拟交易，自动统计胜率与累积损益，践行知行合一的风控原则。
        </p>
      </div>

      {/* Metrics Row */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
        gap: '1.5rem'
      }}>
        <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
          <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>账户累计净损益</span>
          <span style={{
            fontSize: '1.8rem',
            fontWeight: 700,
            fontFamily: 'var(--font-mono)',
            color: (stats?.totalPnL || 0) >= 0 ? 'var(--color-success)' : 'var(--color-danger)'
          }}>
            ${(stats?.totalPnL || 0).toLocaleString(undefined, { minimumFractionDigits: 2 })}
          </span>
          <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>已实现交易损益汇总</span>
        </div>

        <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
          <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>交易胜率 (Win Rate)</span>
          <span style={{
            fontSize: '1.8rem',
            fontWeight: 700,
            fontFamily: 'var(--font-mono)',
            color: 'var(--color-info)'
          }}>
            {((stats?.winRate || 0) * 100).toFixed(1)}%
          </span>
          <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
            {trades.filter(t => t.pnl > 0).length} 胜 / {trades.filter(t => t.pnl <= 0).length} 负
          </span>
        </div>

        <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
          <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>总交易笔数</span>
          <span style={{
            fontSize: '1.8rem',
            fontWeight: 700,
            fontFamily: 'var(--font-mono)',
            color: 'white'
          }}>
            {trades.length}
          </span>
          <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>本地SQLite数据库已同步</span>
        </div>
      </div>

      {/* Main Grid */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: '1fr 2fr',
        gap: '2rem',
        alignItems: 'start'
      }}>
        {/* Input Form */}
        <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', borderBottom: '1px solid var(--border-color)', paddingBottom: '0.5rem' }}>
            <Plus size={18} style={{ color: 'var(--color-primary)' }} />
            <h3 style={{ fontSize: '1rem', fontWeight: 600, color: 'white' }}>录入最新交易</h3>
          </div>

          <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
              <div className="input-group">
                <span className="input-label">交易标的</span>
                <input
                  type="text"
                  required
                  placeholder="e.g. NVDA"
                  value={symbol}
                  onChange={(e) => setSymbol(e.target.value)}
                  className="text-input"
                  style={{ textTransform: 'uppercase', fontFamily: 'var(--font-mono)' }}
                />
              </div>

              <div className="input-group">
                <span className="input-label">交易方向</span>
                <select
                  value={direction}
                  onChange={(e) => setDirection(e.target.value as 'BUY' | 'SELL')}
                  className="text-input"
                  style={{ appearance: 'none', WebkitAppearance: 'none' }}
                >
                  <option value="BUY">买入 (Long)</option>
                  <option value="SELL">卖出 (Short)</option>
                </select>
              </div>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '0.75rem' }}>
              <div className="input-group">
                <span className="input-label">开仓价</span>
                <input
                  type="number"
                  step="0.01"
                  required
                  value={entryPrice}
                  onChange={(e) => setEntryPrice(e.target.value)}
                  className="text-input"
                  style={{ fontFamily: 'var(--font-mono)' }}
                />
              </div>

              <div className="input-group">
                <span className="input-label">平仓价</span>
                <input
                  type="number"
                  step="0.01"
                  required
                  value={exitPrice}
                  onChange={(e) => setExitPrice(e.target.value)}
                  className="text-input"
                  style={{ fontFamily: 'var(--font-mono)' }}
                />
              </div>

              <div className="input-group">
                <span className="input-label">股数/头寸</span>
                <input
                  type="number"
                  required
                  value={shares}
                  onChange={(e) => setShares(e.target.value)}
                  className="text-input"
                  style={{ fontFamily: 'var(--font-mono)' }}
                />
              </div>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
              <div className="input-group">
                <span className="input-label">计算损益 (USD)</span>
                <input
                  type="number"
                  step="0.01"
                  required
                  value={pnl}
                  onChange={(e) => setPnl(e.target.value)}
                  className="text-input"
                  style={{
                    fontFamily: 'var(--font-mono)',
                    fontWeight: 700,
                    color: Number(pnl) >= 0 ? 'var(--color-success)' : 'var(--color-danger)'
                  }}
                />
              </div>

              <div className="input-group" style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                <span className="input-label" style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <span>心理/情绪评分</span>
                  <span style={{ fontWeight: 600, color: 'var(--color-info)' }}>{emotionScore}/10</span>
                </span>
                <input
                  type="range"
                  min="1"
                  max="10"
                  value={emotionScore}
                  onChange={(e) => setEmotionScore(Number(e.target.value))}
                  style={{ accentColor: 'var(--color-primary)', marginTop: '0.4rem' }}
                />
              </div>
            </div>

            <div className="input-group">
              <span className="input-label">复盘笔记 / 逻辑说明</span>
              <textarea
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                rows={3}
                placeholder="记录当时的市场大盘情绪，开仓的理由以及需要改善的心理偏误..."
                className="text-input"
                style={{ resize: 'none', height: '80px', padding: '0.5rem 0.75rem' }}
              />
            </div>

            <button type="submit" className="btn btn-primary" style={{ padding: '0.75rem' }}>
              保存并写入 SQLite DB
            </button>
          </form>
        </div>

        {/* Trade History Table */}
        <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem', overflowX: 'auto' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderBottom: '1px solid var(--border-color)', paddingBottom: '0.5rem' }}>
            <h3 style={{ fontSize: '1rem', fontWeight: 600, color: 'white' }}>历史明细记录</h3>
            <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>已加载 {trades.length} 条数据</span>
          </div>

          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border-color)', color: 'var(--text-secondary)', textAlign: 'left' }}>
                <th style={{ padding: '0.75rem 0.5rem' }}>标的</th>
                <th style={{ padding: '0.75rem 0.5rem' }}>方向</th>
                <th style={{ padding: '0.75rem 0.5rem' }}>开仓 / 平仓</th>
                <th style={{ padding: '0.75rem 0.5rem' }}>股数</th>
                <th style={{ padding: '0.75rem 0.5rem', textAlign: 'right' }}>实现损益</th>
                <th style={{ padding: '0.75rem 0.5rem', textAlign: 'center' }}>情绪</th>
                <th style={{ padding: '0.75rem 0.5rem' }}>复盘备注</th>
              </tr>
            </thead>
            <tbody>
              {trades.map((trade) => (
                <tr key={trade.id} style={{ borderBottom: '1px solid rgba(255, 255, 255, 0.02)' }}>
                  <td style={{ padding: '0.85rem 0.5rem', fontFamily: 'var(--font-mono)', fontWeight: 700, color: 'white' }}>
                    {trade.symbol}
                  </td>
                  <td style={{ padding: '0.85rem 0.5rem' }}>
                    <span style={{
                      padding: '0.15rem 0.4rem',
                      borderRadius: '4px',
                      fontSize: '0.75rem',
                      fontWeight: 'bold',
                      backgroundColor: trade.direction === 'BUY' ? 'rgba(16, 185, 129, 0.15)' : 'rgba(244, 63, 94, 0.15)',
                      color: trade.direction === 'BUY' ? 'var(--color-success)' : 'var(--color-danger)'
                    }}>
                      {trade.direction === 'BUY' ? '做多' : '做空'}
                    </span>
                  </td>
                  <td style={{ padding: '0.85rem 0.5rem', fontFamily: 'var(--font-mono)', color: 'var(--text-secondary)' }}>
                    ${trade.entryPrice.toFixed(2)} / ${trade.exitPrice.toFixed(2)}
                  </td>
                  <td style={{ padding: '0.85rem 0.5rem', fontFamily: 'var(--font-mono)', color: 'white' }}>
                    {trade.shares}
                  </td>
                  <td style={{
                    padding: '0.85rem 0.5rem',
                    textAlign: 'right',
                    fontFamily: 'var(--font-mono)',
                    fontWeight: 700,
                    color: trade.pnl >= 0 ? 'var(--color-success)' : 'var(--color-danger)'
                  }}>
                    {trade.pnl >= 0 ? '+' : ''}{trade.pnl.toLocaleString(undefined, { minimumFractionDigits: 2 })}
                  </td>
                  <td style={{ padding: '0.85rem 0.5rem', textAlign: 'center' }}>
                    <span style={{
                      padding: '0.15rem 0.35rem',
                      borderRadius: '4px',
                      fontSize: '0.75rem',
                      backgroundColor: 'rgba(255,255,255,0.04)',
                      color: 'var(--text-secondary)',
                      fontFamily: 'var(--font-mono)'
                    }}>
                      {trade.emotionScore}/10
                    </span>
                  </td>
                  <td style={{ padding: '0.85rem 0.5rem', color: 'var(--text-secondary)', maxWidth: '200px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {trade.notes || '—'}
                  </td>
                </tr>
              ))}
              {trades.length === 0 && (
                <tr>
                  <td colSpan={7} style={{ padding: '3rem', textAlign: 'center', color: 'var(--text-muted)' }}>
                    暂无复盘交易明细记录，请使用左侧表单提交录入
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
