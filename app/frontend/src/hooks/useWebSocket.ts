import { useEffect, useRef } from 'react'
import { useAnalysisStore, NodeEvent } from '../stores/analysisStore'
import { wsUrl } from '../utils/api'

export const useWebSocket = () => {
  const { setWsStatus, handleWsEvent } = useAnalysisStore()
  const socketRef = useRef<WebSocket | null>(null)
  const reconnectTimeoutRef = useRef<number | null>(null)
  const activeRef = useRef(false)

  const connect = () => {
    if (!activeRef.current || socketRef.current) return

    setWsStatus('connecting')
    const ws = new WebSocket(wsUrl())
    socketRef.current = ws

    ws.onopen = () => {
      setWsStatus('connected')
      console.log('[WS] Connected to backend')
      if (reconnectTimeoutRef.current) {
        window.clearTimeout(reconnectTimeoutRef.current)
        reconnectTimeoutRef.current = null
      }
    }

    ws.onmessage = (event) => {
      try {
        const data: NodeEvent = JSON.parse(event.data)
        handleWsEvent(data)
      } catch (err) {
        console.error('[WS] Failed to parse message:', err)
      }
    }

    ws.onclose = () => {
      setWsStatus('disconnected')
      socketRef.current = null
      if (!activeRef.current) return
      console.log('[WS] Disconnected, scheduling reconnect...')
      
      // Auto-reconnect after 3 seconds
      reconnectTimeoutRef.current = window.setTimeout(() => {
        connect()
      }, 3000)
    }

    ws.onerror = (err) => {
      if (activeRef.current) console.warn('[WS] Error:', err)
      ws.close()
    }
  }

  useEffect(() => {
    activeRef.current = true
    connect()
    return () => {
      activeRef.current = false
      if (socketRef.current) {
        socketRef.current.close()
      }
      if (reconnectTimeoutRef.current) {
        window.clearTimeout(reconnectTimeoutRef.current)
      }
    }
  }, [])
}
