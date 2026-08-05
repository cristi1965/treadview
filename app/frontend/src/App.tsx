import React, { useState } from 'react'
import { Dashboard } from './pages/Dashboard'
import { History } from './pages/History'
import { Settings } from './pages/Settings'
import { WhalesSimple } from './pages/WhalesSimple'
import { WhalesPro } from './pages/WhalesPro'
import { Journal } from './pages/Journal'
import { Macro } from './pages/Macro'
import { useWebSocket } from './hooks/useWebSocket'
import { useAnalysisStore } from './stores/analysisStore'
import { LayoutDashboard, History as HistoryIcon, Settings as SettingsIcon, Radio, LineChart, BookOpen, Calendar } from 'lucide-react'

type Tab = 'dashboard' | 'market' | 'journal' | 'macro' | 'history' | 'settings'

const App: React.FC = () => {
  // Activate WebSocket connection
  useWebSocket()

  const { wsStatus } = useAnalysisStore()
  const [activeTab, setActiveTab] = useState<Tab>('dashboard')

  const getWsStatusStyle = () => {
    switch (wsStatus) {
      case 'connected':
        return { color: 'var(--color-success)', text: 'Live Connected' }
      case 'connecting':
        return { color: 'var(--color-warning)', text: 'Syncing Backend...' }
      default:
        return { color: 'var(--color-danger)', text: 'Disconnected' }
    }
  }

  const wsStyle = getWsStatusStyle()

  return (
    <div className="app-container">
      {/* Header */}
      <header className="header">
        <div className="logo-section">
          <div className="logo-icon">📈</div>
          <div>
            <h1 className="logo-text">TradingAgents</h1>
            <div className="logo-sub">AI Multi-Agent Cockpit</div>
          </div>
        </div>

        {/* Navigation */}
        <nav className="nav-links">
          <button
            onClick={() => setActiveTab('dashboard')}
            className={`nav-link ${activeTab === 'dashboard' ? 'active' : ''}`}
            style={{ background: 'none', border: 'none', cursor: 'pointer' }}
          >
            <LayoutDashboard size={18} /> Dashboard
          </button>
          <button
            onClick={() => setActiveTab('market')}
            className={`nav-link ${activeTab === 'market' ? 'active' : ''}`}
            style={{ background: 'none', border: 'none', cursor: 'pointer' }}
          >
            <LineChart size={18} /> 美股狐狸 (Whales)
          </button>
          <button
            onClick={() => setActiveTab('journal')}
            className={`nav-link ${activeTab === 'journal' ? 'active' : ''}`}
            style={{ background: 'none', border: 'none', cursor: 'pointer' }}
          >
            <BookOpen size={18} /> 交易日志 (Journal)
          </button>
          <button
            onClick={() => setActiveTab('macro')}
            className={`nav-link ${activeTab === 'macro' ? 'active' : ''}`}
            style={{ background: 'none', border: 'none', cursor: 'pointer' }}
          >
            <Calendar size={18} /> 宏观日历 (Macro)
          </button>
          <button
            onClick={() => setActiveTab('history')}
            className={`nav-link ${activeTab === 'history' ? 'active' : ''}`}
            style={{ background: 'none', border: 'none', cursor: 'pointer' }}
          >
            <HistoryIcon size={18} /> History
          </button>
          <button
            onClick={() => setActiveTab('settings')}
            className={`nav-link ${activeTab === 'settings' ? 'active' : ''}`}
            style={{ background: 'none', border: 'none', cursor: 'pointer' }}
          >
            <SettingsIcon size={18} /> Settings
          </button>
        </nav>

        {/* Live WS Status Indicator */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: '0.5rem',
          backgroundColor: 'rgba(255,255,255,0.03)',
          padding: '0.5rem 1rem',
          borderRadius: '9999px',
          border: '1px solid var(--border-color)',
          fontSize: '0.8rem',
          fontWeight: 600
        }}>
          <Radio size={14} color={wsStyle.color} style={{ animation: wsStatus === 'connecting' ? 'pulse-glow 1s infinite' : 'none' }} />
          <span style={{ color: 'white' }}>{wsStyle.text}</span>
        </div>
      </header>

      {/* Main Panel Content */}
      <main className="main-content" style={{ 
        padding: activeTab === 'market' ? '0' : '2rem',
        maxWidth: activeTab === 'market' ? '100%' : '1400px'
      }}>
        {activeTab === 'dashboard' && <Dashboard />}
        {activeTab === 'market' && <WhalesPro />}
        {activeTab === 'journal' && <Journal />}
        {activeTab === 'macro' && <Macro />}
        {activeTab === 'history' && <History />}
        {activeTab === 'settings' && <Settings />}
      </main>
    </div>
  )
}

export default App
