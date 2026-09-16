import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { ResponsiveContainer, AreaChart, Area, XAxis, YAxis, Tooltip } from 'recharts'
import { useI18n } from '../i18n'
import { DataStatus } from './common'
import { fetchQuoteResult, startQuotePolling } from '../utils/liveQuotes'
import { assessDataTrust, type DataTrustAssessment } from '../utils/dataTrust'
import type { APIResponseMeta } from '../utils/api'

interface StockChartProps {
  ticker: string
}

export const StockChart: React.FC<StockChartProps> = ({ ticker }) => {
  const { t } = useI18n()
  const symbol = useMemo(() => ticker.trim().toUpperCase(), [ticker])
  const [chartData, setChartData] = useState<Array<{ name: string; price: number }>>([])
  const [meta, setMeta] = useState<APIResponseMeta | null>(null)
  const [trust, setTrust] = useState<DataTrustAssessment>(() => assessDataTrust(null, false))
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const refresh = useCallback(async () => {
    if (!symbol) return
    setLoading(true)
    try {
      const result = await fetchQuoteResult([symbol])
      const quote = result.quotes[symbol]
      const assessment = assessDataTrust(result.meta, Boolean(quote?.price && quote.price > 0))
      setMeta(result.meta)
      setTrust(assessment)

      if (assessment.state !== 'live' || !quote) {
        setChartData([])
        setError(result.errors.join('；'))
        return
      }

      setError('')
      const label = new Date(result.meta.dataTime).toLocaleTimeString([], {
        hour: '2-digit', minute: '2-digit', second: '2-digit',
      })
      setChartData((prev) => [...prev.slice(-29), { name: label, price: Number(quote.price.toFixed(4)) }])
    } catch (err) {
      setMeta(null)
      setTrust(assessDataTrust(null, false))
      setChartData([])
      setError(err instanceof Error ? err.message : '报价请求失败')
    } finally {
      setLoading(false)
    }
  }, [symbol])

  useEffect(() => {
    setChartData([])
    setMeta(null)
    setTrust(assessDataTrust(null, false))
    setError('')
    if (!symbol) {
      setLoading(false)
      return
    }
    return startQuotePolling(refresh, 20_000)
  }, [symbol, refresh])

  const state = loading && meta === null ? 'loading' : error ? 'error' : trust.state
  const label = state === 'loading'
    ? '正在获取报价'
    : trust.state === 'live'
      ? '已验证报价采样'
      : trust.state === 'stale'
        ? '报价已过期，曲线已隐藏'
        : '报价不可用，曲线已隐藏'

  return (
    <div className="card" style={{ height: '320px', display: 'flex', flexDirection: 'column', gap: '1rem' }}>
      <div className="space-y-2">
        <h3 style={{ fontSize: '1rem', color: 'white', fontWeight: 600 }}>
          {t('chart.title', { ticker: symbol || t('chart.stock') })}
        </h3>
        <DataStatus
          state={state}
          label={label}
          dataTime={meta?.dataTime || 'unknown'}
          source={meta?.source}
          message={error || trust.reason}
          onRetry={() => void refresh()}
          compact
        />
      </div>

      <div style={{ flex: 1, width: '100%' }}>
        {trust.state === 'live' && chartData.length > 0 ? (
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
              <defs>
                <linearGradient id="colorPrice" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="var(--color-primary)" stopOpacity={0.3}/>
                  <stop offset="95%" stopColor="var(--color-primary)" stopOpacity={0}/>
                </linearGradient>
              </defs>
              <XAxis dataKey="name" stroke="var(--text-muted)" fontSize={11} tickLine={false} axisLine={false} />
              <YAxis domain={['auto', 'auto']} stroke="var(--text-muted)" fontSize={11} tickLine={false} axisLine={false} />
              <Tooltip
                contentStyle={{
                  backgroundColor: 'var(--bg-card)', borderColor: 'var(--border-color)', borderRadius: '8px', color: 'white',
                }}
              />
              <Area type="monotone" dataKey="price" stroke="var(--color-primary)" strokeWidth={2} fillOpacity={1} fill="url(#colorPrice)" />
            </AreaChart>
          </ResponsiveContainer>
        ) : (
          <div style={{ height: '100%', display: 'grid', placeItems: 'center', color: 'var(--text-muted)', fontSize: '0.85rem', textAlign: 'center', padding: '1rem' }}>
            {state === 'loading' ? '正在等待可信报价' : '过期、缺失或来源不完整的报价不会进入价格曲线'}
          </div>
        )}
      </div>
    </div>
  )
}
