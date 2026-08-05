import React from 'react'
import { motion } from 'framer-motion'
import { ShieldCheck, ShieldAlert, AlertTriangle } from 'lucide-react'

interface DecisionGaugeProps {
  decision: 'BUY' | 'HOLD' | 'SELL' | null
}

export const DecisionGauge: React.FC<DecisionGaugeProps> = ({ decision }) => {
  if (!decision) return null

  const getStyle = () => {
    switch (decision) {
      case 'BUY':
        return {
          bg: 'rgba(16, 185, 129, 0.1)',
          border: '1px solid rgba(16, 185, 129, 0.4)',
          color: 'var(--color-success)',
          shadow: '0 0 30px rgba(16, 185, 129, 0.2)',
          icon: <ShieldCheck size={48} />,
          text: 'STRATEGIC BUY RECOMMENDATION'
        }
      case 'SELL':
        return {
          bg: 'rgba(244, 63, 94, 0.1)',
          border: '1px solid rgba(244, 63, 94, 0.4)',
          color: 'var(--color-danger)',
          shadow: '0 0 30px rgba(244, 63, 94, 0.2)',
          icon: <ShieldAlert size={48} />,
          text: 'STRATEGIC SELL RECOMMENDATION'
        }
      default:
        return {
          bg: 'rgba(245, 158, 11, 0.1)',
          border: '1px solid rgba(245, 158, 11, 0.4)',
          color: 'var(--color-warning)',
          shadow: '0 0 30px rgba(245, 158, 11, 0.2)',
          icon: <AlertTriangle size={48} />,
          text: 'STRATEGIC HOLD RECOMMENDATION'
        }
    }
  }

  const style = getStyle()

  return (
    <motion.div
      initial={{ scale: 0.9, opacity: 0 }}
      animate={{ scale: 1, opacity: 1 }}
      transition={{ type: 'spring', damping: 15 }}
      style={{
        background: style.bg,
        border: style.border,
        borderRadius: '1.25rem',
        padding: '2rem',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: '1rem',
        boxShadow: style.shadow,
        textAlign: 'center',
        margin: '1.5rem 0'
      }}
    >
      <div style={{ color: style.color }}>
        {style.icon}
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
        <h2 style={{
          color: 'white',
          fontSize: '2.5rem',
          fontWeight: 800,
          letterSpacing: '-0.025em',
        }}>
          {decision}
        </h2>
        <div style={{
          color: style.color,
          fontSize: '0.875rem',
          fontWeight: 700,
          letterSpacing: '0.1em',
          textTransform: 'uppercase'
        }}>
          {style.text}
        </div>
      </div>
    </motion.div>
  )
}
