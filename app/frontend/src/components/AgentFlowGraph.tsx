import React from 'react'
import { motion } from 'framer-motion'
import { TrendingUp, FileText, Heart, Newspaper, HelpCircle, UserCheck, ShieldAlert, Award } from 'lucide-react'
import { I18nKey, useI18n } from '../i18n'

interface NodeProps {
  id: string
  label: string
  icon: React.ReactNode
  isActive: boolean
  isCompleted: boolean
  description: string
}

const flowNodes: Array<Omit<NodeProps, 'label' | 'description'> & { labelKey: I18nKey; descKey: I18nKey }> = [
  { id: 'Market Analyst', labelKey: 'flow.tech', descKey: 'flow.techDesc', icon: <TrendingUp size={20} />, isActive: false, isCompleted: false },
  { id: 'Fundamentals Analyst', labelKey: 'flow.fund', descKey: 'flow.fundDesc', icon: <FileText size={20} />, isActive: false, isCompleted: false },
  { id: 'Sentiment Analyst', labelKey: 'flow.sent', descKey: 'flow.sentDesc', icon: <Heart size={20} />, isActive: false, isCompleted: false },
  { id: 'News Analyst', labelKey: 'flow.news', descKey: 'flow.newsDesc', icon: <Newspaper size={20} />, isActive: false, isCompleted: false },
  { id: 'Debate', labelKey: 'flow.debate', descKey: 'flow.debateDesc', icon: <HelpCircle size={20} />, isActive: false, isCompleted: false },
  { id: 'Research Manager', labelKey: 'flow.rm', descKey: 'flow.rmDesc', icon: <UserCheck size={20} />, isActive: false, isCompleted: false },
  { id: 'Trader', labelKey: 'flow.trader', descKey: 'flow.traderDesc', icon: <TrendingUp size={20} />, isActive: false, isCompleted: false },
  { id: 'Risk', labelKey: 'flow.risk', descKey: 'flow.riskDesc', icon: <ShieldAlert size={20} />, isActive: false, isCompleted: false },
  { id: 'Portfolio Manager', labelKey: 'flow.pm', descKey: 'flow.pmDesc', icon: <Award size={20} />, isActive: false, isCompleted: false },
]

interface AgentFlowGraphProps {
  currentNode: string
  currentPhase: string
}

export const AgentFlowGraph: React.FC<AgentFlowGraphProps> = ({ currentNode, currentPhase }) => {
  const { t } = useI18n()
  const getNodeState = (nodeId: string) => {
    let isActive = false
    let isCompleted = false

    if (nodeId === 'Debate') {
      isActive = currentPhase.includes('Debate') && !currentPhase.includes('Risk')
      isCompleted = currentPhase.includes('Trading') || currentPhase.includes('Risk') || currentPhase.includes('Decision') || currentPhase.includes('Complete')
    } else if (nodeId === 'Risk') {
      isActive = currentPhase.includes('Risk') && !currentPhase.includes('Decision')
      isCompleted = currentPhase.includes('Decision') || currentPhase.includes('Complete')
    } else {
      isActive = currentNode === nodeId
      
      // Basic completion logic based on typical sequencing
      const analystNodes = ['Market Analyst', 'Fundamentals Analyst', 'Sentiment Analyst', 'News Analyst']
      const currentIdx = analystNodes.indexOf(currentNode)
      const thisIdx = analystNodes.indexOf(nodeId)
      
      if (thisIdx !== -1) {
        if (currentIdx === -1) {
          isCompleted = currentPhase.includes('Debate') || currentPhase.includes('Manager') || currentPhase.includes('Trading') || currentPhase.includes('Risk') || currentPhase.includes('Decision') || currentPhase.includes('Complete')
        } else {
          isCompleted = thisIdx < currentIdx
        }
      } else if (nodeId === 'Research Manager') {
        isCompleted = currentPhase.includes('Trading') || currentPhase.includes('Risk') || currentPhase.includes('Decision') || currentPhase.includes('Complete')
      } else if (nodeId === 'Trader') {
        isCompleted = currentPhase.includes('Risk') || currentPhase.includes('Decision') || currentPhase.includes('Complete')
      } else if (nodeId === 'Portfolio Manager') {
        isCompleted = currentPhase.includes('Complete')
      }
    }

    return { isActive, isCompleted }
  }

  return (
    <div className="flow-graph-container" style={{ padding: '2rem 1rem', overflowX: 'auto' }}>
      <div className="flow-grid" style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
        gap: '1.5rem',
        position: 'relative'
      }}>
        {flowNodes.map((node) => {
          const { isActive, isCompleted } = getNodeState(node.id)
          return (
            <motion.div
              key={node.id}
              className={`flow-node-card ${isActive ? 'active' : ''} ${isCompleted ? 'completed' : ''}`}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.3 }}
              style={{
                background: isCompleted 
                  ? 'rgba(16, 185, 129, 0.08)' 
                  : isActive 
                    ? 'rgba(99, 102, 241, 0.15)' 
                    : 'var(--bg-card)',
                border: isCompleted 
                  ? '1px solid rgba(16, 185, 129, 0.4)' 
                  : isActive 
                    ? '1px solid var(--color-primary)' 
                    : '1px solid var(--border-color)',
                borderRadius: '1rem',
                padding: '1.25rem',
                display: 'flex',
                alignItems: 'flex-start',
                gap: '1rem',
                position: 'relative',
                boxShadow: isActive ? 'var(--shadow-glow)' : 'var(--shadow-md)',
                animation: isActive ? 'pulse-glow 2s infinite' : 'none'
              }}
            >
              <div className="node-icon-wrapper" style={{
                background: isCompleted 
                  ? 'rgba(16, 185, 129, 0.15)' 
                  : isActive 
                    ? 'rgba(99, 102, 241, 0.2)' 
                    : 'rgba(255, 255, 255, 0.05)',
                color: isCompleted 
                  ? 'var(--color-success)' 
                  : isActive 
                    ? 'var(--color-primary)' 
                    : 'var(--text-secondary)',
                borderRadius: '0.75rem',
                padding: '0.6rem',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center'
              }}>
                {node.icon}
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                <div style={{ 
                  fontWeight: 600, 
                  fontSize: '0.95rem',
                  color: isCompleted ? '#a7f3d0' : isActive ? 'white' : 'var(--text-primary)'
                }}>
                  {t(node.labelKey)}
                </div>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                  {t(node.descKey)}
                </div>
                {isActive && (
                  <span style={{ 
                    fontSize: '0.7rem', 
                    color: 'var(--color-primary)', 
                    fontWeight: 700,
                    textTransform: 'uppercase',
                    letterSpacing: '0.05em',
                    marginTop: '0.25rem'
                  }}>
                    {t('flow.analyzing')}
                  </span>
                )}
                {isCompleted && (
                  <span style={{ 
                    fontSize: '0.7rem', 
                    color: 'var(--color-success)', 
                    fontWeight: 700,
                    textTransform: 'uppercase',
                    letterSpacing: '0.05em',
                    marginTop: '0.25rem'
                  }}>
                    {t('flow.completed')}
                  </span>
                )}
              </div>
            </motion.div>
          )
        })}
      </div>
    </div>
  )
}
