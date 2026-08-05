import React from 'react';
import { Link, useLocation } from 'react-router-dom';

interface NavItem {
  path: string;
  label: string;
  icon: string;
}

const navItems: NavItem[] = [
  { path: '/', label: 'Market Heatmap', icon: '🔥' },
  { path: '/dashboard', label: 'Strategy Evaluation', icon: '🤖' },
  { path: '/scan', label: 'Asset Screener', icon: '📋' },
  { path: '/etf', label: 'ETF Analytics', icon: '📊' },
  { path: '/whales', label: 'Smart Money', icon: '🐋' },
  { path: '/journal', label: 'Trade Journal', icon: '📓' },
  { path: '/arena', label: 'Macro Arena', icon: '⚔️' },
  { path: '/dashboard/macro', label: 'Macro Calendar', icon: '📅' },
  { path: '/reports', label: 'Evaluation Reports', icon: '📰' },
  { path: '/history', label: 'History Logs', icon: '📜' },
  { path: '/portfolio', label: 'Watchlist', icon: '⭐' },
  { path: '/settings', label: 'Settings', icon: '⚙️' }
];

export const Sidebar: React.FC = () => {
  const location = useLocation();
  
  return (
    <aside className="hidden lg:flex lg:flex-col w-[204px] shrink-0 sticky top-0 h-screen border-r border-line bg-base/70 px-3 py-4 backdrop-blur-md">
      {/* Logo */}
      <Link to="/" className="group mb-5 flex items-center gap-2.5 px-2">
        <img
          src="/logo.png"
          alt="TradingAgents"
          className="h-[30px] w-[30px] shrink-0 rounded-[9px] object-cover ring-1 ring-white/10 transition group-hover:ring-accent/40"
        />
        <span className="flex min-w-0 flex-col leading-none">
          <span className="truncate text-[15px] font-semibold tracking-tight text-ink">
            Trading<span className="text-accent">Agents</span>
          </span>
          <span className="mt-[3px] font-mono text-[8px] uppercase tracking-[0.28em] text-faint">
            Multi-Agent System
          </span>
        </span>
      </Link>

      {/* Navigation */}
      <nav className="flex flex-col gap-0.5">
        {navItems.map((item) => {
          const isActive = location.pathname === item.path || 
                          (item.path !== '/' && location.pathname.startsWith(item.path));
          
          return (
            <Link
              key={item.path}
              to={item.path}
              className={`
                flex items-center gap-3 rounded-lg px-3 py-[9px] 
                text-[13.5px] font-medium transition
                ${isActive 
                  ? 'bg-accent/10 text-accent' 
                  : 'text-muted hover:bg-surface-2 hover:text-ink'
                }
              `}
            >
              <span className="text-[18px] shrink-0">{item.icon}</span>
              <span>{item.label}</span>
            </Link>
          );
        })}
      </nav>
    </aside>
  );
};
