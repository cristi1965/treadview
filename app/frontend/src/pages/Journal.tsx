import React, { useState, useEffect } from 'react'
import { useCommandStore } from '../stores/commandStore'
import { Plus } from 'lucide-react'
import { useI18n } from '../i18n'
import { useSearchParams } from 'react-router-dom'

export const Journal: React.FC = () => {
  const { t } = useI18n()
  const [searchParams] = useSearchParams()
  const sourcePaperOrderID = searchParams.get('paper_order_id')?.trim() || ''
  const sourceResearchRunID = searchParams.get('research_run_id')?.trim() || ''
  const { trades, addTrade, fetchTrades, fetchStats, stats, tradesError, statsError } = useCommandStore()
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')

  // Form State
  const [symbol, setSymbol] = useState(searchParams.get('symbol')?.trim().toUpperCase() || '')
  const [direction, setDirection] = useState<'BUY' | 'SELL'>('BUY')
  const [entryPrice, setEntryPrice] = useState('')
  const [exitPrice, setExitPrice] = useState('')
  const [shares, setShares] = useState('')
  const [pnl, setPnl] = useState('')
  const [emotionScore, setEmotionScore] = useState(5)
  const [notes, setNotes] = useState('')

  useEffect(() => {
    void fetchTrades()
    void fetchStats()
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

    setSaving(true)
    setSaveError('')
    try {
      await addTrade({
        symbol: symbol.toUpperCase(),
        direction,
        entryPrice: Number(entryPrice),
        exitPrice: Number(exitPrice),
        shares: Number(shares),
        pnl: Number(pnl),
        emotionScore: Number(emotionScore),
        notes,
        paperOrderId: sourcePaperOrderID || undefined,
        researchRunId: sourceResearchRunID || undefined,
      })
      setSymbol('')
      setEntryPrice('')
      setExitPrice('')
      setShares('')
      setPnl('')
      setNotes('')
    } catch (error) {
      setSaveError(error instanceof Error ? error.message : t('journal.saveFail'))
    } finally {
      setSaving(false)
    }
  }

  const exportTrades = () => {
    const blob = new Blob([JSON.stringify(trades, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `trading_journal_${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
  }

  const QUICK_TICKERS = ['NVDA', 'TSLA', 'AAPL', 'PLTR', 'CRWV', 'NBIS', '300750', '300308']

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
      {/* Page Header */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flexWrap: 'wrap' }}>
          <h2 style={{ fontSize: '1.5rem', fontWeight: 700, color: 'white' }}>{t('journal.title')}</h2>
          <span style={{ border: '1px solid rgba(56, 189, 248, 0.45)', color: '#bae6fd', padding: '0.2rem 0.5rem', fontSize: '0.7rem', fontWeight: 800 }}>PAPER ONLY</span>
        </div>
        <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
          {t('journal.sub')}
        </p>
        {sourcePaperOrderID && <p className="text-xs text-sky-200">来源 Paper 订单 <span className="font-mono">{sourcePaperOrderID}</span>{sourceResearchRunID ? <> · 研究 Run <span className="font-mono">{sourceResearchRunID}</span></> : null}</p>}
      </div>

      {(tradesError || statsError) && (
        <div role="alert" className="card" style={{ borderColor: 'var(--color-danger)', color: 'var(--color-danger)' }}>
          <strong>{t('journal.loadFail')}</strong>
          <div style={{ marginTop: '0.35rem', fontSize: '0.8rem' }}>{[tradesError, statsError].filter(Boolean).join('；')}</div>
          <button type="button" className="btn btn-secondary" style={{ marginTop: '0.75rem' }} onClick={() => void Promise.all([fetchTrades(), fetchStats()])}>
            {t('journal.retry')}
          </button>
        </div>
      )}

      {/* Metrics Row */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
        gap: '1.5rem'
      }}>
        <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
          <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>{t('journal.pnl')}</span>
          <span style={{
            fontSize: '1.8rem',
            fontWeight: 700,
            fontFamily: 'var(--font-mono)',
            color: (stats?.totalPnL || 0) >= 0 ? 'var(--color-success)' : 'var(--color-danger)'
          }}>
            {statsError ? '—' : `$${(stats?.totalPnL || 0).toLocaleString(undefined, { minimumFractionDigits: 2 })}`}
          </span>
          <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{t('journal.pnlHint')}</span>
        </div>

        <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
          <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>{t('journal.winRate')}</span>
          <span style={{
            fontSize: '1.8rem',
            fontWeight: 700,
            fontFamily: 'var(--font-mono)',
            color: 'var(--color-info)'
          }}>
            {statsError ? '—' : `${((stats?.winRate || 0) * 100).toFixed(1)}%`}
          </span>
          <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
            {tradesError ? '—' : t('journal.winLoss', { win: trades.filter(tr => tr.pnl > 0).length, loss: trades.filter(tr => tr.pnl <= 0).length })}
          </span>
        </div>

        <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
          <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>{t('journal.count')}</span>
          <span style={{
            fontSize: '1.8rem',
            fontWeight: 700,
            fontFamily: 'var(--font-mono)',
            color: 'white'
          }}>
            {tradesError ? '—' : trades.length}
          </span>
          <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{tradesError ? t('journal.unavailable') : t('journal.synced')}</span>
        </div>
      </div>

      {/* Main Grid */}
      <div className="grid grid-cols-1 items-start gap-5 lg:grid-cols-[minmax(280px,1fr)_minmax(0,2fr)] lg:gap-8">
        {/* Input Form */}
        <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid var(--border-color)', paddingBottom: '0.5rem' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <Plus size={18} style={{ color: 'var(--color-primary)' }} />
              <h3 style={{ fontSize: '1rem', fontWeight: 600, color: 'white' }}>{t('journal.add')}</h3>
            </div>
            {!tradesError && trades.length > 0 && (
              <button
                type="button"
                onClick={exportTrades}
                className="btn btn-secondary"
                style={{ fontSize: '0.75rem', padding: '0.25rem 0.5rem' }}
                title="导出交易复盘记录为 JSON"
              >
                📥 导出
              </button>
            )}
          </div>

          {/* Quick Tickers */}
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.35rem' }}>
            {QUICK_TICKERS.map((sym) => (
              <button
                key={sym}
                type="button"
                onClick={() => setSymbol(sym)}
                className="min-h-11 sm:min-h-0"
                style={{
                  fontSize: '0.7rem',
                  padding: '0.2rem 0.45rem',
                  borderRadius: '0.25rem',
                  background: symbol === sym ? 'var(--color-primary)' : 'rgba(255,255,255,0.06)',
                  color: symbol === sym ? '#000' : 'var(--text-secondary)',
                  border: '1px solid rgba(255,255,255,0.1)',
                  cursor: 'pointer',
                  fontWeight: 600,
                }}
              >
                {sym}
              </button>
            ))}
          </div>

          <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="input-group">
                <span className="input-label">{t('journal.symbol')}</span>
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
                <span className="input-label">{t('journal.dir')}</span>
                <select
                  value={direction}
                  onChange={(e) => setDirection(e.target.value as 'BUY' | 'SELL')}
                  className="text-input"
                  style={{ appearance: 'none', WebkitAppearance: 'none' }}
                >
                  <option value="BUY">{t('journal.long')}</option>
                  <option value="SELL">{t('journal.short')}</option>
                </select>
              </div>
            </div>

            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <div className="input-group">
                <span className="input-label">{t('journal.entry')}</span>
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
                <span className="input-label">{t('journal.exit')}</span>
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
                <span className="input-label">{t('journal.shares')}</span>
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

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="input-group">
                <span className="input-label">{t('journal.calcPnl')}</span>
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
                  <span>{t('journal.emotion')}</span>
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
              <span className="input-label">{t('journal.notes')}</span>
              <textarea
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                rows={3}
                placeholder={t('journal.notesPh')}
                className="text-input"
                style={{ resize: 'none', height: '80px', padding: '0.5rem 0.75rem' }}
              />
            </div>

            {saveError && <div role="alert" style={{ color: 'var(--color-danger)', fontSize: '0.8rem' }}>{t('journal.saveFail')}：{saveError} {t('journal.retryHint')}</div>}
            <button type="submit" disabled={saving} className="btn btn-primary" style={{ padding: '0.75rem' }}>
              {saving ? t('journal.saving') : t('journal.save')}
            </button>
          </form>
        </div>

        {/* Trade History Table */}
        <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem', overflowX: 'auto' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderBottom: '1px solid var(--border-color)', paddingBottom: '0.5rem' }}>
            <h3 style={{ fontSize: '1rem', fontWeight: 600, color: 'white' }}>{t('journal.history')}</h3>
            <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>{tradesError ? t('journal.unavailable') : t('journal.loaded', { n: trades.length })}</span>
          </div>

          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border-color)', color: 'var(--text-secondary)', textAlign: 'left' }}>
                <th style={{ padding: '0.75rem 0.5rem' }}>{t('journal.colSymbol')}</th>
                <th style={{ padding: '0.75rem 0.5rem' }}>{t('journal.colDir')}</th>
                <th style={{ padding: '0.75rem 0.5rem' }}>{t('journal.colPx')}</th>
                <th style={{ padding: '0.75rem 0.5rem' }}>{t('journal.colShares')}</th>
                <th style={{ padding: '0.75rem 0.5rem', textAlign: 'right' }}>{t('journal.colPnl')}</th>
                <th style={{ padding: '0.75rem 0.5rem', textAlign: 'center' }}>{t('journal.colEmo')}</th>
                <th style={{ padding: '0.75rem 0.5rem' }}>{t('journal.colNotes')}</th>
              </tr>
            </thead>
            <tbody>
              {trades.map((trade) => (
                <tr key={trade.id} style={{ borderBottom: '1px solid rgba(255, 255, 255, 0.02)' }}>
                  <td style={{ padding: '0.85rem 0.5rem', fontFamily: 'var(--font-mono)', fontWeight: 700, color: 'white' }}>
                    {trade.symbol}
                    {trade.paperOrderId && <span className="mt-1 block text-[10px] font-normal text-sky-300">Paper {trade.paperOrderId}</span>}
                    {trade.researchRunId && <span className="block text-[10px] font-normal text-slate-400">Run {trade.researchRunId}</span>}
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
                      {trade.direction === 'BUY' ? t('journal.longTag') : t('journal.shortTag')}
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
              {(tradesError || trades.length === 0) && (
                <tr>
                  <td colSpan={7} style={{ padding: '3rem', textAlign: 'center', color: 'var(--text-muted)' }}>
                    {tradesError ? t('journal.unavailable') : t('journal.empty')}
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
