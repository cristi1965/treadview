import React, { useEffect, useRef } from 'react'
import { useI18n } from '../i18n'
import { useUiStore } from '../stores/uiStore'
import type { ConsoleLog } from '../utils/analysisLog'
import { formatConsoleLog } from '../utils/analysisLog'

interface LiveLogProps {
  logs: ConsoleLog[]
}

export const LiveLog: React.FC<LiveLogProps> = ({ logs }) => {
  const { t } = useI18n()
  const view = useUiStore((s) => s.reportView)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight
    }
  }, [logs])

  return (
    <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem', height: '240px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h3 style={{ fontSize: '1rem', color: 'white', fontWeight: 600 }}>{t('log.title')}</h3>
        <span style={{
          fontSize: '0.7rem',
          color: 'var(--color-info)',
          fontFamily: varMono()
        }}>
          {t('log.live')}
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
            {t('log.idle')}
          </div>
        ) : (
          logs.map((log, idx) => {
            const text = formatConsoleLog(log, view)
            const isErr = log.kind === 'error' || log.kind === 'start_fail' || log.kind === 'stop_fail'
            const isOk = log.kind === 'node_complete' || log.kind === 'done'
            return (
              <div key={idx} style={{
                color: isErr
                  ? 'var(--color-danger)'
                  : isOk
                    ? '#a7f3d0'
                    : '#34d399'
              }}>
                {text}
              </div>
            )
          })
        )}
      </div>
    </div>
  )
}

function varMono() {
  return "var(--font-mono)"
}
