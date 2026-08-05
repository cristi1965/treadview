import React, { useEffect, useState } from 'react'
import { useAnalysisStore, AnalysisHistory } from '../stores/analysisStore'
import { Calendar, Clock, ChevronRight, RefreshCw } from 'lucide-react'
import ReactMarkdown from 'react-markdown'

export const History: React.FC = () => {
  const { history, fetchHistory } = useAnalysisStore()
  const [selectedItem, setSelectedItem] = useState<AnalysisHistory | null>(null)

  useEffect(() => {
    fetchHistory()
  }, [])

  const getDecisionBadgeClass = (decision: string) => {
    switch (decision) {
      case 'BUY': return 'badge badge-buy'
      case 'SELL': return 'badge badge-sell'
      default: return 'badge badge-hold'
    }
  }

  return (
    <div style={{
      display: 'grid',
      gridTemplateColumns: selectedItem ? '1fr 1.5fr' : '1fr',
      gap: '2rem',
      transition: 'grid-template-columns 0.3s ease'
    }}>
      {/* Left List */}
      <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem', height: 'fit-content' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div>
            <h2 style={{ fontSize: '1.25rem', color: 'white' }}>Historical Evaluations</h2>
            <p style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>Audit trials of past multi-agent stock evaluations.</p>
          </div>
          <button className="btn btn-secondary" onClick={() => fetchHistory()} style={{ padding: '0.5rem' }}>
            <RefreshCw size={16} />
          </button>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          {history.length === 0 ? (
            <div style={{ color: 'var(--text-secondary)', textAlign: 'center', padding: '2rem', fontStyle: 'italic' }}>
              No historical data found. Launch an analysis first.
            </div>
          ) : (
            history.map((item, idx) => (
              <div
                key={idx}
                onClick={() => setSelectedItem(item)}
                style={{
                  padding: '1rem',
                  backgroundColor: selectedItem === item ? 'rgba(99, 102, 241, 0.08)' : 'rgba(255,255,255,0.02)',
                  border: selectedItem === item ? '1px solid var(--color-primary)' : '1px solid var(--border-color)',
                  borderRadius: '0.75rem',
                  cursor: 'pointer',
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  transition: 'all 0.2s ease'
                }}
              >
                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                    <span style={{ fontWeight: 700, fontSize: '1.1rem', color: 'white' }}>{item.ticker}</span>
                    <span className={getDecisionBadgeClass(item.decision)}>{item.decision}</span>
                  </div>
                  <div style={{ display: 'flex', gap: '1rem', fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                    <span style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
                      <Calendar size={12} /> {item.trade_date}
                    </span>
                    <span style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
                      <Clock size={12} /> {item.duration_secs.toFixed(0)}s
                    </span>
                  </div>
                </div>
                <ChevronRight size={16} color="var(--text-muted)" />
              </div>
            ))
          )}
        </div>
      </div>

      {/* Right Detail Panel */}
      {selectedItem && (
        <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem', maxHeight: '80vh', overflowY: 'auto' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', borderBottom: '1px solid var(--border-color)', paddingBottom: '1rem' }}>
            <div>
              <h2 style={{ fontSize: '1.5rem', color: 'white' }}>{selectedItem.ticker} Report Details</h2>
              <span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>Evaluated on {selectedItem.trade_date}</span>
            </div>
            <span className={getDecisionBadgeClass(selectedItem.decision)} style={{ fontSize: '1rem', padding: '0.5rem 1rem' }}>
              {selectedItem.decision}
            </span>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
            <div className="markdown-body">
              <h2>Portfolio Manager Recommendation</h2>
              <ReactMarkdown>{selectedItem.state?.final_trade_decision || ''}</ReactMarkdown>
            </div>

            <div className="markdown-body">
              <h2>Technical Analysis</h2>
              <ReactMarkdown>{selectedItem.state?.market_report || ''}</ReactMarkdown>
            </div>

            <div className="markdown-body">
              <h2>Fundamentals Analysis</h2>
              <ReactMarkdown>{selectedItem.state?.fundamentals_report || ''}</ReactMarkdown>
            </div>

            <div className="markdown-body">
              <h2>Public Sentiment</h2>
              <ReactMarkdown>{selectedItem.state?.sentiment_report || ''}</ReactMarkdown>
            </div>

            <div className="markdown-body">
              <h2>News & Macro</h2>
              <ReactMarkdown>{selectedItem.state?.news_report || ''}</ReactMarkdown>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
