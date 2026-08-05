import React, { useState } from 'react'
import ReactMarkdown from 'react-markdown'
import { ChevronDown, ChevronUp } from 'lucide-react'

interface ReportCardProps {
  title: string
  icon: React.ReactNode
  content: string
  isLoading: boolean
}

export const ReportCard: React.FC<ReportCardProps> = ({ title, icon, content, isLoading }) => {
  const [isCollapsed, setIsCollapsed] = useState(false)

  if (!content && !isLoading) return null

  return (
    <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '1rem', overflow: 'hidden' }}>
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        borderBottom: '1px solid var(--border-color)',
        paddingBottom: '0.75rem',
        cursor: 'pointer'
      }} onClick={() => setIsCollapsed(!isCollapsed)}>
        <div style={{ display: 'flex', alignContent: 'center', alignItems: 'center', gap: '0.75rem' }}>
          <div style={{ color: 'var(--color-primary)', display: 'flex' }}>
            {icon}
          </div>
          <h3 style={{ fontSize: '1.1rem', fontWeight: 600, color: 'white' }}>{title}</h3>
        </div>
        <div style={{ display: 'flex', gap: '0.5rem', color: 'var(--text-secondary)' }}>
          {isCollapsed ? <ChevronDown size={18} /> : <ChevronUp size={18} />}
        </div>
      </div>

      {!isCollapsed && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          {content ? (
            <div className="markdown-body" style={{ overflowX: 'auto' }}>
              <ReactMarkdown>{content}</ReactMarkdown>
            </div>
          ) : (
            <div style={{ color: 'var(--text-muted)', fontStyle: 'italic', fontSize: '0.9rem' }}>
              Waiting for agent feedback...
            </div>
          )}
          
          {isLoading && (
            <div style={{
              display: 'flex',
              alignItems: 'center',
              gap: '0.5rem',
              color: 'var(--color-primary)',
              fontSize: '0.85rem',
              fontWeight: 500,
              borderTop: '1px solid var(--border-color)',
              paddingTop: '0.5rem'
            }}>
              <span className="streaming-dot" style={{
                width: '8px',
                height: '8px',
                backgroundColor: 'var(--color-primary)',
                borderRadius: '50%',
                display: 'inline-block',
                animation: 'pulse-glow 1s infinite'
              }}></span>
              Agent is streaming details in real-time...
            </div>
          )}
        </div>
      )}
    </div>
  )
}
