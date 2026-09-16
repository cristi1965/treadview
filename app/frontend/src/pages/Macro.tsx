import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { useCommandStore, MacroEvent } from '../stores/commandStore'
import { APIResponseMeta, getWithMeta } from '../utils/api'
import { assessDataTrust, DataTrustAssessment } from '../utils/dataTrust'
import { AlertTriangle, Plus, X, ChevronLeft, ChevronRight } from 'lucide-react'
import { DataStatus, SegmentedControl } from '../components/common'
import { useI18n } from '../i18n'

interface MarketCalEvent {
  id: string
  date: string
  type?: string
  isImportant?: boolean
  title: string
  description?: string
}

type CalItem = Omit<MacroEvent, 'id'> & { id?: number; source: 'user' | 'feed'; type?: string }

function pad(n: number) {
  return String(n).padStart(2, '0')
}

function ymd(year: number, month0: number, day: number) {
  return `${year}-${pad(month0 + 1)}-${pad(day)}`
}

function todayYmd() {
  const n = new Date()
  return ymd(n.getFullYear(), n.getMonth(), n.getDate())
}

function monthGrid(year: number, month0: number): (number | null)[] {
  const first = new Date(year, month0, 1)
  const startDow = (first.getDay() + 6) % 7
  const daysInMonth = new Date(year, month0 + 1, 0).getDate()
  const cells: (number | null)[] = []
  for (let i = 0; i < startDow; i++) cells.push(null)
  for (let d = 1; d <= daysInMonth; d++) cells.push(d)
  while (cells.length % 7 !== 0) cells.push(null)
  return cells
}

function starsFromFeed(ev: MarketCalEvent) {
  if (ev.isImportant) return 3
  if (ev.type === 'macro' || ev.type === 'policy') return 2
  return 1
}

const ImpactDots: React.FC<{ level: number }> = ({ level }) => (
  <span className="flex items-center gap-[3px]">
    {[1, 2, 3].map((slot) => (
      <span
        key={slot}
        className={`h-[5px] w-[5px] rounded-full ${
          slot > level ? 'bg-white/10' : level >= 3 ? 'bg-ios-red' : level === 2 ? 'bg-ios-orange' : 'bg-ios-label-3'
        }`}
      />
    ))}
  </span>
)

const fieldClass =
  'w-full rounded-ios-sm bg-ios-card-2 px-3 py-2 text-[13px] text-ios-label placeholder:text-ios-label-3 outline-none focus:ring-1 focus:ring-ios-blue'

export const Macro: React.FC = () => {
  const { t, language } = useI18n()
  const weekdays = language === 'en' ? ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'] : ['一', '二', '三', '四', '五', '六', '日']
  const { events, fetchEvents, addEvent, eventsError } = useCommandStore()
  const [feed, setFeed] = useState<MarketCalEvent[]>([])
  const [feedError, setFeedError] = useState('')
  const [feedTrust, setFeedTrust] = useState<DataTrustAssessment | null>(null)
  const [feedMeta, setFeedMeta] = useState<APIResponseMeta | null>(null)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  const [showAddForm, setShowAddForm] = useState(false)
  const now = new Date()
  const [viewYear, setViewYear] = useState(now.getFullYear())
  const [viewMonth, setViewMonth] = useState(now.getMonth())
  const [selectedDay, setSelectedDay] = useState<number>(now.getDate())
  const [scope, setScope] = useState<'all' | 'important'>('all')

  const [name, setName] = useState('')
  const [date, setDate] = useState('')
  const [time, setTime] = useState('')
  const [stars, setStars] = useState(1)
  const [previous, setPrevious] = useState('')
  const [consensus, setConsensus] = useState('')

  const fetchFeed = useCallback(async () => {
    setFeedError('')
    try {
      const response = await getWithMeta<{ events: MarketCalEvent[] }>('/api/market/calendar')
      if (!Array.isArray(response.data.events)) throw new Error(t('macro.feedFormatFail'))
      setFeed(response.data.events)
      setFeedMeta(response.meta)
      setFeedTrust(assessDataTrust(response.meta, response.data.events.length > 0))
    } catch (error) {
      setFeed([])
      setFeedMeta(null)
      setFeedTrust(null)
      setFeedError(error instanceof Error ? error.message : t('macro.feedFail'))
    }
  }, [t])

  useEffect(() => {
    void fetchEvents()
    void fetchFeed()
  }, [fetchEvents, fetchFeed])

  const allItems: CalItem[] = useMemo(() => {
    const user: CalItem[] = (events || []).map((e) => ({ ...e, source: 'user' as const }))
    const seen = new Set(user.map((e) => `${e.date}|${e.name}`))
    const extra: CalItem[] = []
    const visibleFeed = feedTrust?.state === 'unavailable' ? [] : feed
    for (const ev of visibleFeed) {
      const key = `${ev.date}|${ev.title}`
      if (seen.has(key)) continue
      seen.add(key)
      extra.push({
        date: ev.date,
        time: '',
        name: ev.title,
        stars: starsFromFeed(ev),
        previous: '',
        consensus: ev.description || '',
        source: 'feed',
        type: ev.type,
      })
    }
    return [...user, ...extra].sort((a, b) => (a.date + a.time).localeCompare(b.date + b.time))
  }, [events, feed, feedTrust])

  const monthPrefix = `${viewYear}-${pad(viewMonth + 1)}`
  const monthItems = useMemo(
    () => allItems.filter((e) => e.date.startsWith(monthPrefix) && (scope === 'all' || e.stars === 3)),
    [allItems, monthPrefix, scope]
  )

  const byDate = useMemo(() => {
    const map = new Map<string, CalItem[]>()
    for (const e of monthItems) {
      const list = map.get(e.date) || []
      list.push(e)
      map.set(e.date, list)
    }
    return map
  }, [monthItems])

  const cells = useMemo(() => monthGrid(viewYear, viewMonth), [viewYear, viewMonth])
  const today = todayYmd()
  const selectedDate = selectedDay ? ymd(viewYear, viewMonth, selectedDay) : ''
  const selectedItems = selectedDate ? byDate.get(selectedDate) || [] : []
  const highImpactCount = monthItems.filter((e) => e.stars === 3).length
  const daysInMonth = new Date(viewYear, viewMonth + 1, 0).getDate()

  const shiftMonth = (delta: number) => {
    const d = new Date(viewYear, viewMonth + delta, 1)
    setViewYear(d.getFullYear())
    setViewMonth(d.getMonth())
    setSelectedDay(1)
  }

  const goToday = () => {
    const n = new Date()
    setViewYear(n.getFullYear())
    setViewMonth(n.getMonth())
    setSelectedDay(n.getDate())
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name || !date) return
    setSaving(true)
    setSaveError('')
    try {
      await addEvent({
        name,
        date,
        time: time || '00:00',
        stars: Number(stars),
        previous,
        consensus,
      })
      setName('')
      setDate('')
      setTime('')
      setStars(1)
      setPrevious('')
      setConsensus('')
      setShowAddForm(false)
      const [y, m, d] = date.split('-').map(Number)
      if (y && m) {
        setViewYear(y)
        setViewMonth(m - 1)
        setSelectedDay(d || 1)
      }
    } catch (error) {
      setSaveError(error instanceof Error ? error.message : t('macro.saveFail'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-4 font-sans">
      <header className="space-y-1">
        <h2 className="text-[24px] font-semibold tracking-tight text-ios-label">{t('macro.title')}</h2>
        <p className="text-[13px] text-ios-label-2">{t('macro.subMonth')}</p>
      </header>

      {(eventsError || feedError) && (
        <div role="alert" className="rounded-ios-lg border border-ios-red/40 bg-ios-red/10 px-4 py-3 text-[12px] text-ios-red">
          <p className="font-semibold">{t('macro.loadFail')}</p>
          <p className="mt-1">{[eventsError, feedError].filter(Boolean).join('；')}</p>
          <div className="mt-2 flex flex-wrap gap-2">
            {eventsError && <button type="button" onClick={() => void fetchEvents()} className="rounded-ios-sm bg-white/10 px-3 py-1.5">{t('macro.retryCustom')}</button>}
            {feedError && <button type="button" onClick={() => void fetchFeed()} className="rounded-ios-sm bg-white/10 px-3 py-1.5">{t('macro.retryFeed')}</button>}
          </div>
        </div>
      )}

      {!feedError && feedTrust && feedTrust.state !== 'live' && (
        <div className="rounded-ios-lg bg-ios-card px-4 py-3">
          <DataStatus state={feedTrust.state} dataTime={feedMeta?.dataTime} source={feedMeta?.source} message={feedTrust.reason} compact />
        </div>
      )}

      <div className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-ios-lg bg-ios-card px-4 py-3">
        <span className="text-[15px] font-semibold text-ios-label">
          {t('macro.monthHead', { y: viewYear, m: viewMonth + 1, n: monthItems.length })}
        </span>
        {highImpactCount > 0 && (
          <span className="inline-flex items-center gap-1.5 rounded-full bg-ios-red/15 px-2.5 py-1 text-[11px] font-medium text-ios-red">
            <AlertTriangle size={12} />
            {t('macro.threeStar', { n: highImpactCount })}
          </span>
        )}

        <SegmentedControl
          size="sm"
          aria-label={t('macro.impact')}
          value={scope}
          onChange={setScope}
          segments={[
            { id: 'all', label: t('flash.all') },
            { id: 'important', label: t('flash.calImportant') },
          ]}
        />

        <div className="ml-auto flex items-center gap-2">
          <button
            type="button"
            onClick={() => shiftMonth(-1)}
            aria-label={t('macro.prevMonth')}
            className="rounded-full bg-white/[0.07] p-1.5 text-ios-label-2 transition hover:text-ios-label"
          >
            <ChevronLeft size={15} />
          </button>
          <button
            type="button"
            onClick={goToday}
            className="rounded-ios-sm bg-white/[0.07] px-3 py-1.5 text-[12px] font-medium text-ios-label-2 transition hover:text-ios-label"
          >
            {t('macro.thisMonth')}
          </button>
          <button
            type="button"
            onClick={() => shiftMonth(1)}
            aria-label={t('macro.nextMonth')}
            className="rounded-full bg-white/[0.07] p-1.5 text-ios-label-2 transition hover:text-ios-label"
          >
            <ChevronRight size={15} />
          </button>
          <button
            type="button"
            onClick={() => setShowAddForm(!showAddForm)}
            className={`inline-flex items-center gap-1.5 rounded-ios-sm px-3 py-1.5 text-[12px] font-medium transition ${
              showAddForm ? 'bg-white/[0.07] text-ios-label-2 hover:text-ios-label' : 'bg-ios-blue text-white hover:brightness-110'
            }`}
          >
            {showAddForm ? <X size={14} /> : <Plus size={14} />}
            {showAddForm ? t('macro.cancel') : t('macro.add')}
          </button>
        </div>
      </div>

      {showAddForm && (
        <form onSubmit={handleSubmit} className="space-y-4 rounded-ios-lg bg-ios-card p-4">
          <h3 className="text-[15px] font-semibold text-ios-label">{t('macro.new')}</h3>

          <div className="grid gap-3 sm:grid-cols-3">
            <label className="space-y-1.5">
              <span className="text-[11px] text-ios-label-2">{t('macro.name')}</span>
              <input type="text" required placeholder={t('macro.namePh')} value={name} onChange={(e) => setName(e.target.value)} className={fieldClass} />
            </label>
            <label className="space-y-1.5">
              <span className="text-[11px] text-ios-label-2">{t('macro.date')}</span>
              <input type="date" required value={date} onChange={(e) => setDate(e.target.value)} className={`${fieldClass} font-mono tabular-nums`} />
            </label>
            <label className="space-y-1.5">
              <span className="text-[11px] text-ios-label-2">{t('macro.time')}</span>
              <input type="time" value={time} onChange={(e) => setTime(e.target.value)} className={`${fieldClass} font-mono tabular-nums`} />
            </label>
          </div>

          <div className="grid gap-3 sm:grid-cols-3">
            <label className="space-y-1.5">
              <span className="text-[11px] text-ios-label-2">{t('macro.impact')}</span>
              <select value={stars} onChange={(e) => setStars(Number(e.target.value))} className={`${fieldClass} appearance-none`}>
                <option value={1}>{t('macro.star1')}</option>
                <option value={2}>{t('macro.star2')}</option>
                <option value={3}>{t('macro.star3')}</option>
              </select>
            </label>
            <label className="space-y-1.5">
              <span className="text-[11px] text-ios-label-2">{t('macro.prev')}</span>
              <input type="text" placeholder="e.g. 3.4%" value={previous} onChange={(e) => setPrevious(e.target.value)} className={`${fieldClass} font-mono tabular-nums`} />
            </label>
            <label className="space-y-1.5">
              <span className="text-[11px] text-ios-label-2">{t('macro.cons')}</span>
              <input type="text" placeholder="e.g. 3.2%" value={consensus} onChange={(e) => setConsensus(e.target.value)} className={`${fieldClass} font-mono tabular-nums`} />
            </label>
          </div>

          {saveError && <p role="alert" className="text-[12px] text-ios-red">{t('macro.saveFail')}：{saveError} {t('macro.retryHint')}</p>}
          <button type="submit" disabled={saving} className="rounded-ios-sm bg-ios-blue px-4 py-2 text-[13px] font-medium text-white transition hover:brightness-110 disabled:cursor-wait disabled:opacity-60">
            {saving ? t('macro.saving') : t('macro.save')}
          </button>
        </form>
      )}

      <div className="rounded-ios-lg bg-ios-card p-3">
        <div className="mb-1 grid grid-cols-7 gap-1">
          {weekdays.map((d) => (
            <div key={d} className="py-1.5 text-center text-[11px] font-medium text-ios-label-3">
              {d}
            </div>
          ))}
        </div>

        <div className="grid grid-cols-7 gap-1">
          {cells.map((day, idx) => {
            if (day == null) return <div key={`e-${idx}`} className="min-h-[92px] rounded-ios-sm bg-white/[0.02]" />
            const key = ymd(viewYear, viewMonth, day)
            const list = byDate.get(key) || []
            const isToday = key === today
            const isSelected = day === selectedDay
            const hot = list.some((e) => e.stars === 3)
            return (
              <button
                key={key}
                type="button"
                onClick={() => setSelectedDay(day)}
                className={`flex min-h-[92px] flex-col gap-1 overflow-hidden rounded-ios-sm p-1.5 text-left transition ${
                  isSelected ? 'bg-accent/15 ring-1 ring-accent/50' : 'bg-ios-card-2 hover:bg-white/[0.06]'
                }`}
              >
                <div className="flex items-center justify-between">
                  <span
                    className={`inline-flex h-[20px] min-w-[20px] items-center justify-center rounded-full px-1 font-mono text-[12px] tabular-nums ${
                      isToday ? 'bg-accent font-semibold text-[#1a0f08]' : 'font-medium text-ios-label'
                    }`}
                  >
                    {day}
                  </span>
                  {list.length > 0 && (
                    <span className={`font-mono text-[10px] tabular-nums ${hot ? 'text-ios-red' : 'text-ios-label-3'}`}>
                      {list.length}
                    </span>
                  )}
                </div>
                {list.slice(0, 3).map((ev, i) => (
                  <span
                    key={`${ev.name}-${i}`}
                    title={ev.name}
                    className={`truncate text-[11px] leading-tight ${ev.stars === 3 ? 'text-ios-red' : 'text-ios-label-2'}`}
                  >
                    {ev.name}
                  </span>
                ))}
                {list.length > 3 && <span className="text-[10px] text-ios-label-3">+{list.length - 3}</span>}
              </button>
            )
          })}
        </div>

        <p className="mt-2.5 px-1 text-[11px] text-ios-label-3">
          {t('macro.gridHint', { y: viewYear, m: viewMonth + 1, d: daysInMonth })}
        </p>
      </div>

      <section className="space-y-2">
        <h3 className="flex items-center gap-2 px-1">
          <span className="font-mono text-[13px] font-semibold tabular-nums text-ios-label">{selectedDate}</span>
          <span className="text-[12px] text-ios-label-2">
            {selectedItems.length ? t('macro.dayItems', { n: selectedItems.length }) : t('macro.noDay')}
          </span>
          {selectedDate === today && (
            <span className="rounded-full bg-accent/20 px-2 py-0.5 text-[11px] font-medium text-accent">{t('flash.today')}</span>
          )}
        </h3>

        {selectedItems.length > 0 && (
          <ul className="overflow-hidden rounded-ios-lg bg-ios-card">
            {selectedItems.map((event, i) => (
              <li key={`${event.source}-${event.id || event.name}-${i}`} className="pl-4 transition-colors hover:bg-white/[0.03]">
                <div
                  className={`min-h-[44px] py-3 pr-4 ${i === selectedItems.length - 1 ? '' : 'border-b border-ios-hairline'}`}
                >
                  <div className="flex items-center gap-3">
                    <span className="w-[46px] shrink-0 font-mono text-[12px] tabular-nums text-ios-label-2">{event.time || '—'}</span>
                    <span className="min-w-0 flex-1 text-[13.5px] text-ios-label">{event.name}</span>
                    <ImpactDots level={event.stars} />
                    <span className="shrink-0 text-[10px] text-ios-label-3">
                      {event.source === 'user' ? t('macro.userSrc') : t('macro.feedSrc')}
                    </span>
                    {event.source === 'user' && (
                      <span className="hidden shrink-0 items-center gap-2 font-mono text-[12px] tabular-nums sm:flex">
                        <span className={`w-[54px] text-right ${event.previous ? 'text-ios-label-2' : 'text-ios-label-3'}`}>
                          {event.previous || '—'}
                        </span>
                        <span className={`w-[54px] text-right ${event.consensus ? 'text-ios-label' : 'text-ios-label-3'}`}>
                          {event.consensus || '—'}
                        </span>
                      </span>
                    )}
                  </div>
                  {event.source === 'feed' && event.consensus && (
                    <p className="mt-1 pl-[58px] text-[12px] text-ios-label-2">{event.consensus}</p>
                  )}
                  {event.source === 'user' && (
                    <p className="mt-1 pl-[58px] font-mono text-[11px] tabular-nums text-ios-label-3 sm:hidden">
                      {t('flash.trio', { prev: event.previous || '—', fcst: event.consensus || '—', actual: '—' })}
                    </p>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}
