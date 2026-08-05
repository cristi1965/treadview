import React, { useState } from 'react';
import { Input } from '../common/Input';

interface HeaderProps {
  title?: string;
}

export const Header: React.FC<HeaderProps> = ({ title }) => {
  const [searchQuery, setSearchQuery] = useState('');

  return (
    <header className="sticky top-0 z-30 hidden h-[56px] items-center gap-4 border-b border-line bg-base/85 px-6 backdrop-blur-md lg:flex">
      {title && (
        <span className="shrink-0 text-[14px] font-semibold tracking-tight text-ink">
          {title}
        </span>
      )}
      
      <div className="ml-auto flex min-w-0 items-center gap-2.5">
        {/* Search Bar */}
        <div className="w-[260px] shrink">
          <Input
            value={searchQuery}
            onChange={setSearchQuery}
            placeholder="搜代码 / 名称 / 板块…"
            icon={
              <svg className="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="11" cy="11" r="8" />
                <line x1="21" y1="21" x2="16.65" y2="16.65" />
              </svg>
            }
          />
        </div>

        {/* Market Status */}
        <div className="shrink-0">
          <span className="inline-flex shrink-0 items-center gap-1.5 rounded-md border px-2 py-1 text-[11px] font-medium text-faint border-line bg-surface-2">
            <span className="h-1.5 w-1.5 rounded-full bg-faint" />
            美股 假日休市
          </span>
        </div>

        {/* Theme Toggle */}
        <button
          className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-line bg-surface text-muted transition hover:text-ink"
          aria-label="Toggle theme"
        >
          <svg viewBox="0 0 24 24" className="h-[15px] w-[15px]" fill="none" stroke="currentColor" strokeWidth="1.8">
            <circle cx="12" cy="12" r="4" />
            <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41" />
          </svg>
        </button>

        {/* Language Switch */}
        <div className="inline-flex shrink-0 rounded-lg border border-line bg-surface p-0.5 text-[11px] font-semibold">
          <button className="rounded-md px-2.5 py-1.5 transition bg-surface-3 text-ink">
            中
          </button>
          <button className="rounded-md px-2.5 py-1.5 transition text-muted hover:text-ink">
            EN
          </button>
        </div>

        {/* Buy Button */}
        <a
          href="/how-to-buy"
          className="inline-flex min-h-[36px] shrink-0 items-center rounded-lg px-3.5 py-2 text-[13px] font-semibold text-[#1a0f08] transition hover:brightness-110"
          style={{
            background: 'linear-gradient(135deg, #eda57f 0%, #d98a6a 55%, #c2754f 100%)',
            boxShadow: '0 1px 0 rgba(255,255,255,0.25) inset, 0 4px 14px rgba(217,138,106,0.22)'
          }}
        >
          Buy
        </a>
      </div>
    </header>
  );
};
