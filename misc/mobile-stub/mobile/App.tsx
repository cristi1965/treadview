import React, { useEffect, useState } from 'react'
import {
  SafeAreaView,
  ScrollView,
  StatusBar,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
  NativeModules,
  ActivityIndicator
} from 'react-native'
import { Play, Square, RefreshCw, Activity, ShieldCheck, Award } from 'lucide-react-native'

const { TradingBackendBridge } = NativeModules

const App = () => {
  const [ticker, setTicker] = useState('NVDA')
  const [date, setDate] = useState('2024-05-10')
  const [isRunning, setIsRunning] = useState(false)
  const [progress, setProgress] = useState(0)
  const [phase, setPhase] = useState('Idle')
  const [currentNode, setCurrentNode] = useState('')
  const [logs, setLogs] = useState<string[]>([])
  const [finalDecision, setFinalDecision] = useState('')
  const [decision, setDecision] = useState<string | null>(null)

  const [wsConnected, setWsConnected] = useState(false)
  const wsRef = React.useRef<WebSocket | null>(null)

  // 1. Start Go Backend inside App process on mount
  useEffect(() => {
    if (TradingBackendBridge) {
      console.log('[Mobile] Launching Go Backend Thread')
      // Pass empty string to load API key from iOS process environment/env files,
      // or configure an override key if needed.
      TradingBackendBridge.startBackend('')
    } else {
      console.log('[Mobile] TradingBackendBridge native module not found (running in Expo / Sim?)')
    }

    // Connect to WebSocket local server on 8765
    connectWS()

    return () => {
      if (wsRef.current) wsRef.current.close()
    }
  }, [])

  const connectWS = () => {
    const ws = new WebSocket('ws://127.0.0.1:8765/ws')
    wsRef.current = ws

    ws.onopen = () => {
      setWsConnected(true)
      addLog('System connected to Go backend.')
    }

    ws.onmessage = (e) => {
      try {
        const event = JSON.parse(e.data)
        handleEvent(event)
      } catch (err) {
        console.log('WS error:', err)
      }
    }

    ws.onclose = () => {
      setWsConnected(false)
      setTimeout(connectWS, 3000)
    }
  }

  const addLog = (log: string) => {
    setLogs((prev) => [...prev.slice(-99), `[${new Date().toLocaleTimeString()}] ${log}`])
  }

  const handleEvent = (event: any) => {
    const { type, node, content, progress: prog } = event
    switch (type) {
      case 'analysis_start':
        addLog(`🚀 Analysis pipeline started for ${ticker}`)
        break
      case 'progress':
        setPhase(node)
        setProgress(prog || 0)
        break
      case 'node_start':
        setCurrentNode(node)
        addLog(`🤖 Agent ${node} started...`)
        break
      case 'node_complete':
        setCurrentNode('')
        addLog(`✅ Agent ${node} finished.`)
        if (node === 'Portfolio Manager') {
          setFinalDecision(content)
        }
        break
      case 'analysis_complete':
        addLog(`🎉 Analysis complete!`)
        setIsRunning(false)
        if (content && content.includes(' → ')) {
          const dec = content.split(' → ')[1].trim()
          setDecision(dec)
        }
        break
      case 'analysis_error':
        addLog(`❌ Error: ${content}`)
        setIsRunning(false)
        break
    }
  }

  const [lang, setLang] = useState('English')

  const triggerAnalysis = async () => {
    if (isRunning) return
    setIsRunning(true)
    setProgress(0)
    setPhase('Starting...')
    setLogs([])
    setFinalDecision('')
    setDecision(null)

    try {
      // Save output language selection to backend config
      await fetch('http://127.0.0.1:8765/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ output_language: lang }),
      })

      const resp = await fetch('http://127.0.0.1:8765/api/analysis/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ticker, trade_date: date }),
      })
      if (!resp.ok) {
        throw new Error('API server returned error')
      }
    } catch (e: any) {
      setIsRunning(false)
      addLog(`Failed to start: ${e.message}`)
    }
  }

  const abortAnalysis = async () => {
    try {
      await fetch('http://127.0.0.1:8765/api/analysis/stop', { method: 'POST' })
      setIsRunning(false)
      setPhase('Stopped')
    } catch (e) {
      console.log('Failed to stop', e)
    }
  }

  return (
    <SafeAreaView style={styles.container}>
      <StatusBar barStyle="light-content" />
      
      {/* Header */}
      <View style={styles.header}>
        <View>
          <Text style={styles.title}>TradingAgents</Text>
          <Text style={styles.subtitle}>iOS Multi-Agent Cockpit</Text>
        </View>
        <View style={[styles.statusIndicator, { backgroundColor: wsConnected ? 'rgba(16,185,129,0.1)' : 'rgba(244,63,94,0.1)' }]}>
          <Text style={[styles.statusText, { color: wsConnected ? '#10b981' : '#f43f5e' }]}>
            {wsConnected ? '● Online' : '○ Offline'}
          </Text>
        </View>
      </View>

      <ScrollView contentContainerStyle={styles.scrollContent}>
        {/* Controls Card */}
        <View style={styles.card}>
          <Text style={styles.cardTitle}>Run Stock Analysis</Text>
          
          <View style={styles.formRow}>
            <View style={styles.inputGroup}>
              <Text style={styles.label}>Ticker</Text>
              <TextInput
                style={styles.input}
                value={ticker}
                onChangeText={(val) => setTicker(val.toUpperCase())}
                placeholder="NVDA"
                placeholderTextColor="#6b7280"
                editable={!isRunning}
              />
            </View>

            <View style={styles.inputGroup}>
              <Text style={styles.label}>Trade Date</Text>
              <TextInput
                style={styles.input}
                value={date}
                onChangeText={setDate}
                placeholder="YYYY-MM-DD"
                placeholderTextColor="#6b7280"
                editable={!isRunning}
              />
            </View>
          </View>

          {/* Language Toggle Buttons */}
          <View style={{ flexDirection: 'row', gap: 10, marginBottom: 16 }}>
            <TouchableOpacity 
              style={{ flex: 1, padding: 8, borderRadius: 6, borderWidth: 1, borderColor: lang === 'English' ? '#6366f1' : 'rgba(255,255,255,0.08)', backgroundColor: lang === 'English' ? 'rgba(99,102,241,0.1)' : 'transparent', alignItems: 'center' }}
              onPress={() => setLang('English')}
              disabled={isRunning}
            >
              <Text style={{ color: lang === 'English' ? 'white' : '#9ca3af', fontWeight: 'bold', fontSize: 12 }}>English</Text>
            </TouchableOpacity>
            <TouchableOpacity 
              style={{ flex: 1, padding: 8, borderRadius: 6, borderWidth: 1, borderColor: lang === 'Chinese' ? '#6366f1' : 'rgba(255,255,255,0.08)', backgroundColor: lang === 'Chinese' ? 'rgba(99,102,241,0.1)' : 'transparent', alignItems: 'center' }}
              onPress={() => setLang('Chinese')}
              disabled={isRunning}
            >
              <Text style={{ color: lang === 'Chinese' ? 'white' : '#9ca3af', fontWeight: 'bold', fontSize: 12 }}>中文 (Chinese)</Text>
            </TouchableOpacity>
          </View>

          {!isRunning ? (
            <TouchableOpacity style={styles.btnPrimary} onPress={triggerAnalysis}>
              <Play size={18} color="white" />
              <Text style={styles.btnText}>Start Evaluation</Text>
            </TouchableOpacity>
          ) : (
            <TouchableOpacity style={styles.btnDanger} onPress={abortAnalysis}>
              <Square size={18} color="white" />
              <Text style={styles.btnText}>Cancel Analysis</Text>
            </TouchableOpacity>
          )}

          {isRunning && (
            <View style={styles.progressContainer}>
              <View style={styles.progressHeader}>
                <Text style={styles.progressPhase}>{phase}</Text>
                <Text style={styles.progressPct}>{progress}%</Text>
              </View>
              <View style={styles.progressBar}>
                <View style={[styles.progressFill, { width: `${progress}%` }]} />
              </View>
            </View>
          )}
        </View>

        {/* Decision gauge */}
        {decision && (
          <View style={[styles.card, styles.decisionCard, { borderColor: decision === 'BUY' ? '#10b981' : decision === 'SELL' ? '#f43f5e' : '#f59e0b' }]}>
            <Award size={36} color={decision === '#10b981' ? '#10b981' : '#f59e0b'} />
            <Text style={styles.decisionTitle}>DECISION: {decision}</Text>
            <Text style={styles.decisionSubtitle}>Derived by Portfolio Manager Agent</Text>
          </View>
        )}

        {/* Live console logs */}
        <View style={styles.card}>
          <Text style={styles.cardTitle}>Mobile Log Stream</Text>
          <View style={styles.console}>
            {logs.length === 0 ? (
              <Text style={styles.emptyConsole}>Awaiting analysis start...</Text>
            ) : (
              logs.map((logStr, idx) => (
                <Text key={idx} style={styles.consoleLine}>{logStr}</Text>
              ))
            )}
          </View>
        </View>

        {/* Final MD Report */}
        {finalDecision !== '' && (
          <View style={styles.card}>
            <Text style={styles.cardTitle}>PM Final Decision Report</Text>
            <Text style={styles.reportText}>{finalDecision}</Text>
          </View>
        )}
      </ScrollView>
    </SafeAreaView>
  )
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#070a13',
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingHorizontal: 20,
    paddingVertical: 15,
    borderBottomWidth: 1,
    borderBottomColor: 'rgba(255,255,255,0.08)',
  },
  title: {
    fontSize: 22,
    fontWeight: 'bold',
    color: 'white',
  },
  subtitle: {
    fontSize: 12,
    color: '#6366f1',
    fontWeight: '600',
    marginTop: 2,
  },
  statusIndicator: {
    paddingHorizontal: 12,
    paddingVertical: 6,
    borderRadius: 20,
  },
  statusText: {
    fontSize: 11,
    fontWeight: '700',
  },
  scrollContent: {
    padding: 20,
    gap: 20,
  },
  card: {
    backgroundColor: '#151c33',
    borderWidth: 1,
    borderColor: 'rgba(255,255,255,0.08)',
    borderRadius: 12,
    padding: 16,
  },
  cardTitle: {
    fontSize: 15,
    fontWeight: '600',
    color: 'white',
    marginBottom: 14,
  },
  formRow: {
    flexDirection: 'row',
    gap: 15,
    marginBottom: 16,
  },
  inputGroup: {
    flex: 1,
    gap: 6,
  },
  label: {
    fontSize: 11,
    color: '#9ca3af',
    fontWeight: '600',
  },
  input: {
    backgroundColor: 'rgba(7, 10, 19, 0.6)',
    borderWidth: 1,
    borderColor: 'rgba(255,255,255,0.08)',
    borderRadius: 6,
    paddingHorizontal: 12,
    paddingVertical: 10,
    color: 'white',
    fontSize: 14,
  },
  btnPrimary: {
    backgroundColor: '#6366f1',
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
    gap: 8,
    borderRadius: 6,
    paddingVertical: 12,
  },
  btnDanger: {
    backgroundColor: '#f43f5e',
    flexDirection: 'row',
    justify('center'),
    justifyContent: 'center',
    alignItems: 'center',
    gap: 8,
    borderRadius: 6,
    paddingVertical: 12,
  },
  btnText: {
    color: 'white',
    fontWeight: 'bold',
    fontSize: 14,
  },
  progressContainer: {
    marginTop: 15,
    gap: 6,
  },
  progressHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
  },
  progressPhase: {
    fontSize: 12,
    color: '#9ca3af',
  },
  progressPct: {
    fontSize: 12,
    color: '#6366f1',
    fontWeight: '700',
  },
  progressBar: {
    height: 5,
    backgroundColor: 'rgba(255,255,255,0.05)',
    borderRadius: 3,
    overflow: 'hidden',
  },
  progressFill: {
    height: '100%',
    backgroundColor: '#6366f1',
  },
  decisionCard: {
    alignItems: 'center',
    borderWidth: 1,
    backgroundColor: 'rgba(16,185,129,0.05)',
    paddingVertical: 20,
    gap: 8,
  },
  decisionTitle: {
    fontSize: 20,
    fontWeight: '800',
    color: 'white',
  },
  decisionSubtitle: {
    fontSize: 12,
    color: '#9ca3af',
  },
  console: {
    backgroundColor: 'rgba(7, 10, 19, 0.7)',
    borderRadius: 6,
    padding: 10,
    minHeight: 120,
    maxHeight: 180,
  },
  emptyConsole: {
    color: '#6b7280',
    fontStyle: 'italic',
    fontSize: 12,
  },
  consoleLine: {
    fontFamily: 'System',
    color: '#34d399',
    fontSize: 11,
    marginBottom: 4,
  },
  reportText: {
    color: '#d1d5db',
    fontSize: 13,
    lineHeight: 20,
  }
})

export default App
