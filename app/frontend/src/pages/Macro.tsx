import React, { useEffect, useState } from 'react'
import { useCommandStore } from '../stores/commandStore'
import { Calendar as CalendarIcon, Star, AlertTriangle, Plus, X } from 'lucide-react'

export const Macro: React.FC = () => {
  const { events, fetchEvents, addEvent } = useCommandStore()
  const [showAddForm, setShowAddForm] = useState(false)

  // New Event Form State
  const [name, setName] = useState('')
  const [date, setDate] = useState('')
  const [time, setTime] = useState('')
  const [stars, setStars] = useState(1)
  const [previous, setPrevious] = useState('')
  const [consensus, setConsensus] = useState('')

  useEffect(() => {
    fetchEvents()
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name || !date) return

    await addEvent({
      name,
      date,
      time: time || '00:00',
      stars: Number(stars),
      previous,
      consensus,
    })

    // Reset Form
    setName('')
    setDate('')
    setTime('')
    setStars(1)
    setPrevious('')
    setConsensus('')
    setShowAddForm(false)
  }

  // Count high importance events
  const highImpactCount = events.filter((e) => e.stars === 3).length

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem', maxWidth: '1000px', margin: '0 auto' }}>
      {/* Page Header */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
        <h2 style={{ fontSize: '1.5rem', fontWeight: 700, color: 'white' }}>宏观事件日历 (Macroeconomic Calendar)</h2>
        <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
          追踪全球核心经济指标发布（CPI、非农、利率决议），避开重大事件带来的高波动流动性冲击。
        </p>
      </div>

      {/* Info & Control Bar */}
      <div className="card" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '1rem', flexWrap: 'wrap' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
          <CalendarIcon className="w-5 h-5" style={{ color: 'var(--color-primary)' }} />
          <span style={{ fontSize: '0.95rem', fontWeight: 600 }}>宏观警报状态</span>
          {highImpactCount > 0 && (
            <span style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '0.35rem',
              fontSize: '0.75rem',
              padding: '0.2rem 0.6rem',
              borderRadius: '6px',
              backgroundColor: 'rgba(244, 63, 94, 0.15)',
              border: '1px solid rgba(244, 63, 94, 0.3)',
              color: 'var(--color-danger)',
              fontWeight: 700,
              animation: 'pulse 2s infinite'
            }}>
              <AlertTriangle size={14} />
              {highImpactCount} 个三星高危事件
            </span>
          )}
        </div>

        <button
          onClick={() => setShowAddForm(!showAddForm)}
          className={`btn ${showAddForm ? 'btn-secondary' : 'btn-primary'}`}
          style={{ padding: '0.5rem 1rem', fontSize: '0.85rem', display: 'flex', alignItems: 'center', gap: '0.35rem' }}
        >
          {showAddForm ? <X size={16} /> : <Plus size={16} />}
          {showAddForm ? '取消录入' : '新增宏观事件'}
        </button>
      </div>

      {/* Add Form Panel */}
      {showAddForm && (
        <div className="card animate-fade-in" style={{ border: '1px solid rgba(99, 102, 241, 0.2)' }}>
          <h3 style={{ fontSize: '1rem', fontWeight: 600, color: 'white', marginBottom: '1.25rem' }}>新建经济指标发布</h3>
          <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '1rem' }}>
              <div className="input-group">
                <span className="input-label">事件/指标名称</span>
                <input
                  type="text"
                  required
                  placeholder="e.g. 美国6月核心CPI年率"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="text-input"
                />
              </div>

              <div className="input-group">
                <span className="input-label">发布日期</span>
                <input
                  type="date"
                  required
                  value={date}
                  onChange={(e) => setDate(e.target.value)}
                  className="text-input"
                  style={{ fontFamily: 'var(--font-mono)' }}
                />
              </div>

              <div className="input-group">
                <span className="input-label">发布时间 (EST/北京时间)</span>
                <input
                  type="time"
                  value={time}
                  onChange={(e) => setTime(e.target.value)}
                  className="text-input"
                  style={{ fontFamily: 'var(--font-mono)' }}
                />
              </div>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '1rem' }}>
              <div className="input-group">
                <span className="input-label">重要程度 / 波动影响</span>
                <select
                  value={stars}
                  onChange={(e) => setStars(Number(e.target.value))}
                  className="text-input"
                  style={{ appearance: 'none', WebkitAppearance: 'none' }}
                >
                  <option value={1}>★☆☆ (低波动影响)</option>
                  <option value={2}>★★☆ (中度波动影响)</option>
                  <option value={3}>★★★ (高风险/利率会议/非农)</option>
                </select>
              </div>

              <div className="input-group">
                <span className="input-label">上期前值</span>
                <input
                  type="text"
                  placeholder="e.g. 3.4%"
                  value={previous}
                  onChange={(e) => setPrevious(e.target.value)}
                  className="text-input"
                  style={{ fontFamily: 'var(--font-mono)' }}
                />
              </div>

              <div className="input-group">
                <span className="input-label">市场共识预期值</span>
                <input
                  type="text"
                  placeholder="e.g. 3.2%"
                  value={consensus}
                  onChange={(e) => setConsensus(e.target.value)}
                  className="text-input"
                  style={{ fontFamily: 'var(--font-mono)' }}
                />
              </div>
            </div>

            <button type="submit" className="btn btn-primary" style={{ alignSelf: 'flex-start', padding: '0.6rem 1.5rem' }}>
              保存发布计划
            </button>
          </form>
        </div>
      )}

      {/* Events List */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
        {events.map((event) => (
          <div key={event.id} className="card" style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            padding: '1.25rem 1.5rem',
            gap: '1.5rem',
            flexWrap: 'wrap',
            borderColor: event.stars === 3 ? 'rgba(244, 63, 94, 0.2)' : 'var(--border-color)'
          }}>
            <div style={{ display: 'flex', gap: '1.25rem', alignItems: 'center' }}>
              <div style={{
                backgroundColor: 'rgba(255, 255, 255, 0.03)',
                border: '1px solid var(--border-color)',
                padding: '0.4rem 0.8rem',
                borderRadius: '6px',
                textAlign: 'center',
                minWidth: '80px'
              }}>
                <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)', display: 'block', fontFamily: 'var(--font-mono)' }}>{event.time}</span>
                <span style={{ fontSize: '0.85rem', fontWeight: 700, color: 'var(--text-primary)', fontFamily: 'var(--font-mono)' }}>{event.date}</span>
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <span style={{ fontSize: '0.95rem', fontWeight: 700, color: 'white' }}>{event.name}</span>
                  <div style={{ display: 'flex', gap: '0.1rem' }}>
                    {Array.from({ length: 3 }).map((_, i) => (
                      <Star
                        key={i}
                        size={12}
                        style={{
                          fill: i < event.stars ? 'var(--color-warning)' : 'none',
                          color: i < event.stars ? 'var(--color-warning)' : 'var(--text-muted)'
                        }}
                      />
                    ))}
                  </div>
                </div>
                <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>敏感资产: 美元指数 (DXY), 标普500期指, 黄金, 纳斯达克成分股</span>
              </div>
            </div>

            <div style={{ display: 'flex', gap: '2rem', fontSize: '0.85rem', fontFamily: 'var(--font-mono)' }}>
              <div>
                <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)', display: 'block' }}>前值</span>
                <span style={{ color: 'var(--text-primary)' }}>{event.previous || '—'}</span>
              </div>
              <div>
                <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)', display: 'block' }}>共识预期</span>
                <span style={{ color: 'var(--text-primary)', fontWeight: 600 }}>{event.consensus || '—'}</span>
              </div>
              <div>
                <span style={{ fontSize: '0.7rem', color: 'var(--text-muted)', display: 'block' }}>实际公布</span>
                <span style={{ color: 'var(--text-muted)' }}>待公布</span>
              </div>
            </div>
          </div>
        ))}

        {events.length === 0 && (
          <div className="card" style={{ padding: '4rem', textAlign: 'center', color: 'var(--text-muted)', fontSize: '0.85rem' }}>
            当前无已规划宏观事件。点击“新增宏观事件”开始录入预警指标。
          </div>
        )}
      </div>
    </div>
  )
}
