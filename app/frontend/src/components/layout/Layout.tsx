import React, { useEffect, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { Header } from './Header';
import { Menu, X, Moon, Sun } from 'lucide-react';
import { useUiStore } from '../../stores/uiStore';
import { getUSMarketStatus } from '../../i18n';
import { GlobalDataFreshnessControl, useGlobalDataFreshness } from './GlobalDataFreshnessControl';
import { useDialogFocus } from '../../hooks/useDialogFocus';

interface LayoutProps {
  children: React.ReactNode;
  title?: string;
}

export const Layout: React.FC<LayoutProps> = ({ children, title }) => {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const { language, toggleLanguage, theme, toggleTheme } = {
    language: useUiStore((s) => s.language),
    toggleLanguage: useUiStore((s) => s.toggleLanguage),
    theme: useUiStore((s) => s.theme),
    toggleTheme: useUiStore((s) => s.toggleTheme),
  };
  const location = useLocation();
  const researchSurface = ['/dashboard', '/history', '/lab', '/copilot', '/tactical', '/journal', '/settings']
    .some((path) => location.pathname === path || location.pathname.startsWith(`${path}/`));
  const marketStatus = getUSMarketStatus(language);
  const dataFreshness = useGlobalDataFreshness(!researchSurface);
  const closeMobileMenu = () => setMobileMenuOpen(false);
  const mobileMenuRef = useDialogFocus<HTMLDivElement>(mobileMenuOpen, closeMobileMenu);

  useEffect(() => {
    document.documentElement.lang = language === 'zh' ? 'zh-CN' : 'en';
    document.title = title ? `${title} · 我不是神` : '我不是神 · 多智能体研究';
  }, [language, title]);

  const navCockpit = [
    { path: '/dashboard', label: '研究工作台', icon: '🔎' },
    { path: '/history', label: '研究记录', icon: '📜' },
    { path: '/lab', label: '研究实验室', icon: '🧪' },
    { path: '/copilot', label: '研究助手', icon: '💬' },
    { path: '/tactical', label: 'Paper 工作台', icon: '🌙' },
    { path: '/journal', label: '交易复盘', icon: '📓' },
    { path: '/dashboard/macro', label: '宏观日历', icon: '📅' },
    { path: '/settings', label: '设置', icon: '⚙️' },
  ];

  const navMain = [
    { path: '/market', label: '实验行情', icon: '🔥' },
    { path: '/scan', label: '股票列表', icon: '📋' },
    { path: '/multichart', label: '多图盯盘', icon: '🖥' },
    { path: '/gpu-prices', label: 'GPU 租金', icon: '⚡' },
    { path: '/etf', label: 'ETF', icon: '📊' },
    { path: '/etf#cn-tools', label: 'A股工具', icon: '🛡' },
    { path: '/whales', label: '聪明钱', icon: '🐋' },
    { path: '/arena', label: '对决', icon: '⚔️' },
    { path: '/reports', label: '盘报', icon: '📰' },
    { path: '/notes', label: '笔记', icon: '📝' },
    { path: '/portfolio', label: '观察与持仓', icon: '⭐' },
  ];

  return (
    <div className="flex min-h-screen bg-base text-ink flex-col lg:flex-row">
      {/* Desktop Sidebar */}
      <Sidebar />

      {/* Mobile Top Navigation Header */}
      <div className="sticky top-0 z-40 border-b border-line bg-[#08090b]/90 backdrop-blur-md lg:hidden">
        <div className="flex h-[54px] items-center justify-between px-3.5">
          <Link to="/" className="flex items-center gap-2">
            <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-amber-500 to-orange-600 shadow-md">
              <span className="font-black text-xs text-slate-950">N</span>
            </div>
            <span className="font-black text-[14px] text-ink flex items-baseline gap-1">
              我不是神 <span className="text-[11px] font-medium text-slate-400">多智能体</span>
            </span>
          </Link>

          <div className="flex items-center gap-1.5">
            {!researchSurface && <span className="inline-flex items-center gap-1 rounded-md border border-line bg-surface-2 px-1.5 py-0.5 text-[10px] font-medium text-faint">
              <span className={`h-1.5 w-1.5 rounded-full ${marketStatus.color}`} />
              {marketStatus.text}
            </span>}

            <button
              type="button"
              onClick={toggleTheme}
              className="flex h-11 w-11 items-center justify-center rounded-lg border border-line bg-surface text-muted hover:text-ink sm:h-8 sm:w-8"
              aria-label="Toggle Theme"
            >
              {theme === 'dark' ? <Moon size={14} /> : <Sun size={14} />}
            </button>

            <button
              type="button"
              onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
              className="flex h-11 w-11 items-center justify-center rounded-lg border border-line bg-surface text-muted hover:text-ink sm:h-8 sm:w-8"
              aria-label="Toggle Menu"
            >
              {mobileMenuOpen ? <X size={16} /> : <Menu size={16} />}
            </button>
          </div>
        </div>
        <div className="flex items-center justify-between gap-2 px-3.5 pb-2">
          {researchSurface ? <><span className="min-w-0 truncate text-[10px] font-semibold text-sky-200">{language === 'zh' ? '历史研究 + Paper 模拟 · 非实时' : 'Historical research + Paper · not live'}</span><Link to="/settings#product-readiness" className="shrink-0 text-[10px] font-semibold text-accent">{language === 'zh' ? '产品门禁' : 'Gates'}</Link></> : <div className="ml-auto"><GlobalDataFreshnessControl language={language} controller={dataFreshness} compact /></div>}
        </div>
      </div>

      {/* Mobile Slide-Over Menu Overlay */}
      {mobileMenuOpen && (
        <div ref={mobileMenuRef} role="dialog" aria-modal="true" aria-label={language === 'zh' ? '导航菜单' : 'Navigation menu'} tabIndex={-1} className="fixed inset-0 z-50 flex flex-col bg-base/95 backdrop-blur-lg lg:hidden animate-in fade-in duration-150">
          <div className="flex h-[54px] items-center justify-between border-b border-line px-4">
            <div className="flex items-center gap-2">
              <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-amber-500 to-orange-600">
                <span className="font-black text-xs text-slate-950">N</span>
              </div>
              <span className="font-black text-[14px] text-ink">我不是神 · 导航中心</span>
            </div>
            <button
              onClick={closeMobileMenu}
              aria-label={language === 'zh' ? '关闭导航菜单' : 'Close navigation menu'}
              className="flex h-11 w-11 items-center justify-center rounded-lg border border-line bg-surface text-muted sm:h-8 sm:w-8"
            >
              <X size={16} />
            </button>
          </div>

          <div className="flex-1 overflow-y-auto p-4 space-y-4">
            <div>
              <div className="px-2 pb-1.5 text-[10px] font-mono uppercase tracking-wider text-faint">
                研究与工作台
              </div>
              <div className="space-y-1">
                {navCockpit.map((item) => (
                  <Link
                    key={item.path}
                    to={item.path}
                    onClick={() => setMobileMenuOpen(false)}
                    className={`flex min-h-11 items-center gap-2.5 rounded-xl px-3 py-2 text-sm font-medium transition ${
                      location.pathname === item.path
                        ? 'bg-accent/15 text-accent font-bold border border-accent/25'
                        : 'text-muted hover:bg-surface-2 hover:text-ink'
                    }`}
                  >
                    <span>{item.icon}</span>
                    <span>{item.label}</span>
                  </Link>
                ))}
              </div>
            </div>

            <div className="border-t border-line pt-3">
              <div className="px-2 pb-1.5 text-[10px] font-mono uppercase tracking-wider text-faint">
                全景看盘与选股
              </div>
              <div className="space-y-1">
                {navMain.map((item) => (
                  <Link
                    key={item.path}
                    to={item.path}
                    onClick={() => setMobileMenuOpen(false)}
                    className={`flex min-h-11 items-center gap-2.5 rounded-xl px-3 py-2 text-sm font-medium transition ${
                      location.pathname === item.path
                        ? 'bg-accent/15 text-accent font-bold border border-accent/25'
                        : 'text-muted hover:bg-surface-2 hover:text-ink'
                    }`}
                  >
                    <span>{item.icon}</span>
                    <span>{item.label}</span>
                  </Link>
                ))}
              </div>
            </div>

            <div className="border-t border-line pt-3 flex items-center justify-between px-2">
              <span className="text-xs text-muted">语言切换</span>
              <button
                onClick={toggleLanguage}
                className="rounded-lg border border-line bg-surface px-3 py-1 text-xs font-semibold text-ink"
              >
                {language === 'zh' ? '中文 (切换 EN)' : 'English (Switch 中文)'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col min-w-0">
        <Header title={title} dataFreshness={dataFreshness} surface={researchSurface ? 'research' : 'default'} />

        <main className="flex-1 px-2.5 py-3 sm:px-4 sm:py-5 md:p-6 overflow-x-hidden">
          <div className="mx-auto max-w-[1480px]">
            {children}
          </div>
        </main>
      </div>
    </div>
  );
};
