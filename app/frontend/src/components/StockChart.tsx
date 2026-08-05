import React from 'react'
import { ResponsiveContainer, AreaChart, Area, XAxis, YAxis, Tooltip } from 'recharts'

const mockChartData = [
  { name: '04-20', price: 82.3 },
  { name: '04-23', price: 83.1 },
  { name: '04-24', price: 84.5 },
  { name: '04-25', price: 83.8 },
  { name: '04-26', price: 85.0 },
  { name: '04-27', price: 86.4 },
  { name: '04-30', price: 87.2 },
  { name: '05-01', price: 86.9 },
  { name: '05-02', price: 88.3 },
  { name: '05-03', price: 89.1 },
  { name: '05-04', price: 88.5 },
  { name: '05-07', price: 89.9 },
  { name: '05-08', price: 91.2 },
  { name: '05-09', price: 90.5 },
  { name: '05-10', price: 92.4 }
]

interface StockChartProps {
  ticker: string
}

export const StockChart: React.FC<StockChartProps> = ({ ticker }) => {
  return (
    <div className="card" style={{ height: '320px', display: 'flex', flexDirection: 'column', gap: '1rem' }}>
      <div>
        <h3 style={{ fontSize: '1rem', color: 'white', fontWeight: 600 }}>
          {ticker || 'Stock'} Price Movement (Historical Trend)
        </h3>
        <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
          Data up to evaluation date
        </span>
      </div>

      <div style={{ flex: 1, width: '100%' }}>
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={mockChartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
            <defs>
              <linearGradient id="colorPrice" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="var(--color-primary)" stopOpacity={0.3}/>
                <stop offset="95%" stopColor="var(--color-primary)" stopOpacity={0}/>
              </linearGradient>
            </defs>
            <XAxis 
              dataKey="name" 
              stroke="var(--text-muted)" 
              fontSize={11} 
              tickLine={false} 
              axisLine={false} 
            />
            <YAxis 
              domain={['auto', 'auto']} 
              stroke="var(--text-muted)" 
              fontSize={11} 
              tickLine={false} 
              axisLine={false} 
            />
            <Tooltip 
              contentStyle={{ 
                backgroundColor: 'var(--bg-card)', 
                borderColor: 'var(--border-color)',
                borderRadius: '8px',
                color: 'white'
              }} 
            />
            <Area 
              type="monotone" 
              dataKey="price" 
              stroke="var(--color-primary)" 
              strokeWidth={2}
              fillOpacity={1} 
              fill="url(#colorPrice)" 
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}
