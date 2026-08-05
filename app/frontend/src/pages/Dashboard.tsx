import React, { useState } from 'react'
import { useAnalysisStore } from '../stores/analysisStore'
import { AgentFlowGraph } from '../components/AgentFlowGraph'
import { LiveLog } from '../components/LiveLog'
import { StockChart } from '../components/StockChart'
import { ReportCard } from '../components/ReportCard'
import { DecisionGauge } from '../components/DecisionGauge'
import { Play, Square, BrainCircuit } from 'lucide-react'

export const Dashboard: React.FC = () => {
  const {
    isRunning,
    currentNode,
    currentPhase,
    currentProgress,
    logs,
    marketReport,
    fundamentalsReport,
    sentimentReport,
    newsReport,
    debateHistory,
    researchPlan,
    traderProposal,
    riskHistory,
    finalDecision,
    decisionType,
    startAnalysis,
    stopAnalysis
  } = useAnalysisStore()

  const getTodayDateString = () => {
    const today = new Date()
    const year = today.getFullYear()
    const month = String(today.getMonth() + 1).padStart(2, '0')
    const day = String(today.getDate()).padStart(2, '0')
    return `${year}-${month}-${day}`
  }

  const [ticker, setTicker] = useState('NVDA')
  const [date, setDate] = useState(getTodayDateString)
  const [assetType, setAssetType] = useState('stock')

  const handleStart = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!ticker || !date) return
    try {
      await startAnalysis(ticker, date, assetType)
    } catch (err) {
      alert('Failed to start: ' + err)
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
      {/* Control Panel */}
      <div className="card" style={{ padding: '2rem' }}>
        <form onSubmit={handleStart} style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
          gap: '1.5rem',
          alignItems: 'end'
        }}>
          <div className="input-group">
            <span className="input-label">Ticker Symbol</span>
            <input
              type="text"
              className="text-input"
              value={ticker}
              onChange={(e) => setTicker(e.target.value.toUpperCase())}
              disabled={isRunning}
              placeholder="e.g. NVDA"
            />
          </div>

          <div className="input-group">
            <span className="input-label">Analysis Trade Date</span>
            <input
              type="date"
              className="text-input"
              value={date}
              onChange={(e) => setDate(e.target.value)}
              disabled={isRunning}
            />
          </div>

          <div className="input-group">
            <span className="input-label">Asset Pipeline</span>
            <select
              className="text-input"
              value={assetType}
              onChange={(e) => setAssetType(e.target.value)}
              disabled={isRunning}
              style={{ appearance: 'none', WebkitAppearance: 'none' }}
            >
              <option value="stock">US Equities / Stock</option>
              <option value="crypto">Cryptocurrency</option>
            </select>
          </div>

          <div style={{ display: 'flex', gap: '1rem' }}>
            {!isRunning ? (
              <button type="submit" className="btn btn-primary" style={{ flex: 1 }}>
                <Play size={18} /> Launch Analysis
              </button>
            ) : (
              <button type="button" className="btn btn-secondary" onClick={stopAnalysis} style={{ flex: 1, backgroundColor: 'rgba(244, 63, 94, 0.15)', borderColor: 'rgba(244, 63, 94, 0.3)', color: 'var(--color-danger)' }}>
                <Square size={18} /> Abort Execution
              </button>
            )}
          </div>
        </form>

        {isRunning && (
          <div style={{ marginTop: '1.5rem', display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem' }}>
              <span style={{ color: 'var(--text-secondary)' }}>{currentPhase}</span>
              <span style={{ fontWeight: 600, color: 'var(--color-primary)' }}>{currentProgress}%</span>
            </div>
            <div style={{ height: '6px', backgroundColor: 'rgba(255,255,255,0.05)', borderRadius: '3px', overflow: 'hidden' }}>
              <div style={{ height: '100%', width: `${currentProgress}%`, backgroundColor: 'var(--color-primary)', transition: 'width 0.4s ease' }} />
            </div>
          </div>
        )}
      </div>

      {/* Workflow Map */}
      <div className="card" style={{ padding: '1rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', padding: '1rem' }}>
          <BrainCircuit size={20} color="var(--color-primary)" />
          <h2 style={{ fontSize: '1.1rem', color: 'white' }}>Multi-Agent System Flow Graph</h2>
        </div>
        <AgentFlowGraph currentNode={currentNode} currentPhase={currentPhase} />
      </div>

      {/* Main Grid: Live Output & Chart */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(400px, 1fr))',
        gap: '2rem'
      }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
          <StockChart ticker={ticker} />
          <LiveLog logs={logs} />
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
          {/* Decision */}
          <DecisionGauge decision={decisionType} />

          {/* Collapsible Report Cards */}
          <ReportCard
            title="Portfolio Manager Final Verdict"
            icon={<BrainCircuit />}
            content={finalDecision}
            isLoading={currentNode === 'Portfolio Manager'}
          />

          <ReportCard
            title="Technical Analysis Report"
            icon={<BrainCircuit />}
            content={marketReport}
            isLoading={currentNode === 'Market Analyst'}
          />

          <ReportCard
            title="Fundamentals Analysis Report"
            icon={<BrainCircuit />}
            content={fundamentalsReport}
            isLoading={currentNode === 'Fundamentals Analyst'}
          />

          <ReportCard
            title="Public Sentiment Assessment"
            icon={<BrainCircuit />}
            content={sentimentReport}
            isLoading={currentNode === 'Sentiment Analyst'}
          />

          <ReportCard
            title="News & Macroeconomic Assessment"
            icon={<BrainCircuit />}
            content={newsReport}
            isLoading={currentNode === 'News Analyst'}
          />

          <ReportCard
            title="Research Team Debate"
            icon={<BrainCircuit />}
            content={debateHistory}
            isLoading={currentNode === 'Bull Researcher' || currentNode === 'Bear Researcher'}
          />

          <ReportCard
            title="Research Manager Recommendation"
            icon={<BrainCircuit />}
            content={researchPlan}
            isLoading={currentNode === 'Research Manager'}
          />

          <ReportCard
            title="Trader Transaction Proposal"
            icon={<BrainCircuit />}
            content={traderProposal}
            isLoading={currentNode === 'Trader'}
          />

          <ReportCard
            title="Risk Management Debate"
            icon={<BrainCircuit />}
            content={riskHistory}
            isLoading={currentPhase.includes('Risk') && !finalDecision}
          />
        </div>
      </div>
    </div>
  )
}
