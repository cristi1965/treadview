import { useEffect, useRef } from 'react'
import { useAnalysisStore, NodeEvent } from '../stores/analysisStore'
import { wsUrl } from '../utils/api'
import { bootstrapLocalAdminSession } from './useAdminSession'

export const useWebSocket = (enabled = true) => {
  const { setWsStatus, handleWsEvent } = useAnalysisStore()
  const socketRef = useRef<WebSocket | null>(null)
  const reconnectTimeoutRef = useRef<number | null>(null)
  const activeRef = useRef(false)
  const connectingRef = useRef(false)

  const scheduleReconnect = () => {
    if (!activeRef.current || reconnectTimeoutRef.current) return
    reconnectTimeoutRef.current = window.setTimeout(() => {
      reconnectTimeoutRef.current = null
      void connect()
    }, 3000)
  }

  const connect = async () => {
    if (!activeRef.current || socketRef.current || connectingRef.current) return

    connectingRef.current = true
    setWsStatus('connecting')
    const authorized = await bootstrapLocalAdminSession()
    connectingRef.current = false
    if (!activeRef.current) return
    if (!authorized) {
      setWsStatus('disconnected')
      scheduleReconnect()
      return
    }
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
      scheduleReconnect()
    }

    ws.onerror = (err) => {
      if (activeRef.current) console.warn('[WS] Error:', err)
      ws.close()
    }
  }

  useEffect(() => {
    if (!enabled) {
      activeRef.current = false
      setWsStatus('disconnected')
      return
    }
    activeRef.current = true
    void connect()
    return () => {
      activeRef.current = false
      if (socketRef.current) {
        socketRef.current.close()
      }
      if (reconnectTimeoutRef.current) {
        window.clearTimeout(reconnectTimeoutRef.current)
      }
    }
  }, [enabled])
}
