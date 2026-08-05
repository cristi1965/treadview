import React, { useEffect, useRef } from 'react'

interface LiveLogProps {
  logs: string[]
}

export const LiveLog: React.FC<LiveLogProps> = ({ logs }) => {
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight
    }
  }, [logs])

  return (
    <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem', height: '240px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h3 style={{ fontSize: '1rem', color: 'white', fontWeight: 600 }}>System Console Stream</h3>
        <span style={{ 
          fontSize: '0.7rem', 
          color: 'var(--color-info)', 
          fontFamily: varMono() 
        }}>
          LIVE FEED
        </span>
      </div>

      <div 
        ref={containerRef}
        style={{
          flex: 1,
          backgroundColor: 'rgba(7, 10, 19, 0.7)',
          border: '1px solid var(--border-color)',
          borderRadius: '0.5rem',
          padding: '1rem',
          fontFamily: varMono(),
          fontSize: '0.825rem',
          lineHeight: '1.5',
          overflowY: 'auto',
          display: 'flex',
          flexDirection: 'column',
          gap: '0.5rem',
          color: '#34d399'
        }}
      >
        {logs.length === 0 ? (
          <div style={{ color: 'var(--text-muted)', fontStyle: 'italic' }}>
            System idle. Awaiting instruction...
          </div>
        ) : (
          logs.map((log, idx) => (
            <div key={idx} style={{ 
              color: log.includes('Error') 
                ? 'var(--color-danger)' 
                : log.includes('✅') 
                  ? '#a7f3d0' 
                  : '#34d399' 
            }}>
              {log}
            </div>
          ))
        )}
      </div>
    </div>
  )
}

function varMono() {
  return "var(--font-mono)"
}
