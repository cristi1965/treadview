import React from 'react'
import ReactDOM from 'react-dom/client'
import { AppRoutes } from './AppRoutes.tsx'
import { ErrorBoundary } from './components/ErrorBoundary.tsx'
import './index.css'

// Wails 桌面壳：启用标题栏拖动与红绿灯避让样式
if (import.meta.env.VITE_WAILS === '1') {
  document.documentElement.classList.add('wails-desktop')
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <AppRoutes />
    </ErrorBoundary>
  </React.StrictMode>,
)
