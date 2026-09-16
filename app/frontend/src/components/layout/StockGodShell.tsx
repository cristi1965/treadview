import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import {
  BarChart3,
  BookOpen,
  Bot,
  Calendar,
  ChevronLeft,
  ChevronRight,
  ClipboardList,
  Cpu,
  FlaskConical,
  History,
  MessageSquare,
  Moon,
  Flame,
  Globe2,
  LayoutGrid,
  Menu,
  RefreshCw,
  Search,
  Settings,
  ShieldAlert,
  Star,
  Sun,
  Swords,
  Waves,
  X,
} from 'lucide-react';
import { Stock } from '../../types/stocks';
import { get as apiGet } from '../../utils/api';
import { usePortfolioStore } from '../../stores/portfolioStore';
import { useUiStore } from '../../stores/uiStore';
import { getUSMarketStatus } from '../../i18n';
import {
  GlobalDataFreshnessControl,
  useGlobalDataFreshness,
  type GlobalDataFreshnessController,
} from './GlobalDataFreshnessControl';
import { createLatestRequestGate } from '../../utils/latestRequest';
import { useDialogFocus } from '../../hooks/useDialogFocus';

interface StockGodShellProps {
  title: string;
  children: React.ReactNode;
  searchQuery?: string;
  onSearchChange?: (value: string) => void;
  macroTickers?: [string, string, string][];
}

interface NavItem {
  path: string;
  label: string;
  labelEn: string;
  icon: React.FC<any>;
  group?: 'main' | 'cockpit';
}

const navItems: NavItem[] = [
  { path: '/market', label: '实验行情', labelEn: 'Market lab', icon: Flame, group: 'main' },
  { path: '/scan', label: '股票列表', labelEn: 'Stocks', icon: LayoutGrid, group: 'main' },
  { path: '/multichart', label: '多图盯盘', labelEn: 'MultiChart', icon: LayoutGrid, group: 'main' },
  { path: '/gpu-prices', label: 'GPU 租金', labelEn: 'GPU Prices', icon: Cpu, group: 'main' },
  { path: '/etf', label: 'ETF', labelEn: 'ETF', icon: BarChart3, group: 'main' },
  { path: '/etf#cn-tools', label: 'A股·套路', labelEn: 'A-Share', icon: ShieldAlert, group: 'main' },
  { path: '/whales', label: '聪明钱', labelEn: 'Whales', icon: Waves, group: 'main' },
  { path: '/arena', label: '对决', labelEn: 'Arena', icon: Swords, group: 'main' },
  { path: '/reports', label: '盘报', labelEn: 'Reports', icon: Globe2, group: 'main' },
  { path: '/notes', label: '笔记', labelEn: 'Notes', icon: BookOpen, group: 'main' },
  { path: '/portfolio', label: '观察与持仓', labelEn: 'Watch & holdings', icon: Star, group: 'main' },
  { path: '/dashboard', label: '研究工作台', labelEn: 'Research', icon: Bot, group: 'cockpit' },
  { path: '/history', label: '研究记录', labelEn: 'Research history', icon: History, group: 'cockpit' },
  { path: '/lab', label: '研究实验室', labelEn: 'Research lab', icon: FlaskConical, group: 'cockpit' },
  { path: '/copilot', label: '研究助手', labelEn: 'Research assistant', icon: MessageSquare, group: 'cockpit' },
  { path: '/tactical', label: 'Paper 工作台', labelEn: 'Paper workspace', icon: Moon, group: 'cockpit' },
  { path: '/journal', label: '交易复盘', labelEn: 'Journal', icon: ClipboardList, group: 'cockpit' },
  { path: '/dashboard/macro', label: '宏观日历', labelEn: 'Macro', icon: Calendar, group: 'cockpit' },
  { path: '/settings', label: '设置', labelEn: 'Settings', icon: Settings, group: 'cockpit' },
];

interface SearchResponse {
  results: Stock[];
  count: number;
}

const labels = {
  zh: {
    searchPlaceholder: '搜代码 / 名称 / 板块…',
    buy: 'Buy',
    observe: '观察',
    commandTitle: '搜索股票 / 页面',
    empty: '没有找到结果',
    quick: '快速入口',
  },
  en: {
    searchPlaceholder: 'Search symbol / name / sector...',
    buy: 'Buy',
    observe: 'Watch',
    commandTitle: 'Search stocks / pages',
    empty: 'No results',
    quick: 'Quick links',
  },
} as const;

const quickLinks = [
  { label: '实验行情', labelEn: 'Market lab', path: '/market' },
  { label: '全市场扫描', labelEn: 'Scanner', path: '/scan' },
  { label: 'GPU 算力租金', labelEn: 'GPU Rental Prices', path: '/gpu-prices' },
  { label: 'ETF 板块', labelEn: 'ETF sectors', path: '/etf' },
  { label: 'A股·套路·跳水预警', labelEn: 'A-Share Tools', path: '/etf#cn-tools' },
  { label: '聪明钱', labelEn: 'Smart money', path: '/whales' },
  { label: '五神对决', labelEn: 'Arena', path: '/arena' },
  { label: '盘报', labelEn: 'Reports', path: '/reports' },
  { label: '投资笔记', labelEn: 'Notes', path: '/notes' },
  { label: '我的组合', labelEn: 'Portfolio', path: '/portfolio' },
  { label: 'AI 交易问答', labelEn: 'Trading Copilot', path: '/copilot' },
  { label: '夜间战术指挥所', labelEn: 'Tactical Command', path: '/tactical' },
  { label: '策略评估（驾驶舱）', labelEn: 'Strategy (Cockpit)', path: '/dashboard' },
  { label: '交易复盘（驾驶舱）', labelEn: 'Journal (Cockpit)', path: '/journal' },
  { label: '宏观日历（驾驶舱）', labelEn: 'Macro Calendar', path: '/dashboard/macro' },
  { label: '设置', labelEn: 'Settings', path: '/settings' },
];

const InertDiv = 'div' as unknown as React.ComponentType<React.HTMLAttributes<HTMLDivElement> & {
  inert?: string;
  ref?: React.Ref<HTMLDivElement>;
}>;

const StockGodLogo: React.FC<{ size?: 'sm' | 'md' }> = ({ size = 'md' }) => (
  <div
    className={`flex shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-amber-500 to-orange-600 font-black tracking-tighter text-slate-950 shadow-md ring-1 ring-amber-400/40 transition group-hover:ring-amber-400/80 ${
      size === 'sm' ? 'h-[30px] w-[30px] text-xs' : 'h-8 w-8 text-sm'
    }`}
    style={{
      boxShadow: '0 2px 10px rgba(245, 158, 11, 0.35)',
    }}
  >
    <svg viewBox="0 0 32 32" width="20" height="20" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M7 24V8L17 20V8H21V24L11 12V24H7Z" fill="#0b0f19" />
      <circle cx="25" cy="8" r="2.5" fill="#0b0f19" />
    </svg>
  </div>
);

interface ShellSidebarProps {
  onOpenWatchlist: () => void;
}

const ShellSidebar: React.FC<ShellSidebarProps> = ({ onOpenWatchlist }) => {
  const location = useLocation();
  const language = useUiStore((s) => s.language);
  const watchlist = usePortfolioStore((s) => s.watchlist);
  const [collapsed, setCollapsed] = useState(() => localStorage.getItem('stockgod_sidebar_collapsed') === 'true');

  const toggleCollapse = () => {
    setCollapsed((prev) => {
      const next = !prev;
      localStorage.setItem('stockgod_sidebar_collapsed', String(next));
      return next;
    });
  };

  return (
    <aside
      className={`wails-sidebar hidden h-screen shrink-0 border-r border-line bg-[#08090b]/85 px-3 py-4 backdrop-blur-md transition-all duration-300 lg:sticky lg:top-0 lg:flex lg:flex-col ${
        collapsed ? 'w-[68px]' : 'w-[204px]'
      }`}
    >
      <Link to="/" className={`group mb-5 flex items-center gap-2.5 px-2 ${collapsed ? 'justify-center' : ''}`}>
        <StockGodLogo size="sm" />
        {!collapsed && (
          <span className="flex min-w-0 flex-col leading-none">
            <span className="truncate text-[15px] font-black tracking-tight text-ink flex items-center gap-1">
              我不是神 <span className="text-[12px] font-normal text-slate-400">Not a Stock God</span>
            </span>
            <span className="mt-[4px] font-mono text-[8px] uppercase tracking-[0.22em] text-faint">
              NOT A STOCK GOD
            </span>
          </span>
        )}
      </Link>

      <nav className="flex flex-1 flex-col gap-1 overflow-y-auto [-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
        {!collapsed && (
          <div className="px-3 pb-1 pt-1 font-mono text-[9px] uppercase tracking-[0.2em] text-faint">
            {language === 'en' ? 'Main' : '核心功能'}
          </div>
        )}
        {navItems.filter((i) => i.group !== 'cockpit').map((item) => {
          const Icon = item.icon;
          const basePath = item.path.split('#')[0];
          const hash = item.path.includes('#') ? item.path.split('#')[1] : '';
          const isActive = hash
            ? location.pathname === basePath && location.hash === `#${hash}`
            : location.pathname === item.path || (item.path !== '/' && location.pathname.startsWith(item.path) && !location.hash);
          const isMine = item.path === '/portfolio';

          return (
            <Link
              key={item.path}
              to={item.path}
              title={collapsed ? (language === 'en' ? item.labelEn : item.label) : undefined}
              className={`relative flex items-center gap-3 rounded-lg px-3 py-[9px] text-[13.5px] font-medium transition-all duration-200 ${
                collapsed ? 'justify-center px-0 py-2.5' : ''
              } ${
                isActive
                  ? 'border border-accent/20 bg-accent/12 text-accent font-semibold shadow-sm'
                  : 'text-muted hover:bg-surface-2 hover:text-ink'
              }`}
            >
              {isActive && (
                <span className="absolute left-0 top-1.5 bottom-1.5 w-1 rounded-r-full bg-accent" />
              )}
              <Icon className="h-[18px] w-[18px] shrink-0" strokeWidth={1.85} />
              {!collapsed && (
                <span className="min-w-0 flex-1 truncate">{language === 'en' ? item.labelEn : item.label}</span>
              )}
              {!collapsed && isMine && watchlist.length > 0 && (
                <span className="rounded-full bg-accent/20 px-1.5 py-0.5 font-mono text-[10px] font-semibold text-accent">
                  {watchlist.length}
                </span>
              )}
            </Link>
          );
        })}

        <div className="mx-2 my-2 border-t border-line" />

        {/* Watchlist Quick Drawer Trigger Button inside Sidebar */}
        <button
          type="button"
          onClick={onOpenWatchlist}
          title={collapsed ? (language === 'en' ? 'Watchlist' : '观察抽屉') : undefined}
          className={`group flex items-center gap-3 rounded-lg px-3 py-[9px] text-[13.5px] font-medium text-muted transition-all hover:bg-surface-2 hover:text-ink ${
            collapsed ? 'justify-center px-0' : ''
          }`}
        >
          <Star className="h-[18px] w-[18px] shrink-0 fill-amber-400/20 text-amber-400 group-hover:scale-110 transition-transform" strokeWidth={1.85} />
          {!collapsed && (
            <span className="min-w-0 flex-1 truncate text-left">{language === 'en' ? 'Watch Drawer' : '观察抽屉'}</span>
          )}
          {!collapsed && (
            <span className="rounded-full bg-amber-400/20 px-1.5 py-0.5 font-mono text-[10px] font-semibold text-amber-400">
              {watchlist.length}
            </span>
          )}
        </button>

        <div className="mx-2 my-2 border-t border-line" />
        {!collapsed && (
          <div className="px-3 pb-1 font-mono text-[9px] uppercase tracking-[0.2em] text-faint">
            {language === 'en' ? 'Cockpit' : 'AI 驾驶舱'}
          </div>
        )}

        {navItems.filter((i) => i.group === 'cockpit').map((item) => {
          const Icon = item.icon;
          const isActive = location.pathname === item.path;

          return (
            <Link
              key={item.path}
              to={item.path}
              title={collapsed ? (language === 'en' ? item.labelEn : item.label) : undefined}
              className={`relative flex items-center gap-3 rounded-lg px-3 py-[9px] text-[13.5px] font-medium transition-all duration-200 ${
                collapsed ? 'justify-center px-0 py-2.5' : ''
              } ${
                isActive
                  ? 'border border-accent/20 bg-accent/12 text-accent font-semibold shadow-sm'
                  : 'text-muted hover:bg-surface-2 hover:text-ink'
              }`}
            >
              {isActive && (
                <span className="absolute left-0 top-1.5 bottom-1.5 w-1 rounded-r-full bg-accent" />
              )}
              <Icon className="h-[18px] w-[18px] shrink-0" strokeWidth={1.85} />
              {!collapsed && (
                <span className="min-w-0 flex-1 truncate">{language === 'en' ? item.labelEn : item.label}</span>
              )}
            </Link>
          );
        })}
      </nav>

      <div className="mt-auto pt-2">
        <button
          type="button"
          onClick={toggleCollapse}
          className="flex w-full items-center justify-center rounded-lg border border-line bg-surface py-2 text-muted transition hover:bg-surface-2 hover:text-ink"
          title={collapsed ? '展开导航栏' : '折叠导航栏'}
        >
          {collapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronLeft className="h-4 w-4" />}
        </button>
      </div>
    </aside>
  );
};

const ShellHeader: React.FC<{
  title: string;
  searchQuery: string;
  onSearchChange: (value: string) => void;
  onSearchFocus: () => void;
  theme: 'dark' | 'light';
  onToggleTheme: () => void;
  language: 'zh' | 'en';
  onToggleLanguage: () => void;
  dataFreshness: GlobalDataFreshnessController;
}> = ({ title, searchQuery, onSearchChange, onSearchFocus, theme, onToggleTheme, language, onToggleLanguage, dataFreshness }) => {
  const location = useLocation();
  const t = labels[language];
  const marketStatus = getUSMarketStatus(language);

  // Find active icon for title breadcrumb
  const currentNav = navItems.find(
    (item) => location.pathname === item.path || (item.path !== '/' && location.pathname.startsWith(item.path))
  );
  const ActiveIcon = currentNav?.icon || Flame;

  return (
    <header className="wails-titlebar sticky top-0 z-30 hidden h-[56px] items-center gap-4 border-b border-line bg-[#08090b]/85 px-6 backdrop-blur-md lg:flex">
      <div className="flex shrink-0 items-center gap-2">
        <ActiveIcon className="h-4 w-4 text-accent" strokeWidth={2} />
        <span className="text-[14px] font-semibold tracking-tight text-ink">{title}</span>
      </div>
      <div className="flex-1" />

      <div className="flex min-w-0 items-center gap-2.5">
        <div className="relative w-[240px] shrink">
          <Search className="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-faint" strokeWidth={2} />
          <input
            value={searchQuery}
            onChange={(event) => onSearchChange(event.target.value)}
            onFocus={onSearchFocus}
            placeholder={t.searchPlaceholder}
            className="w-full rounded-lg border border-line bg-surface py-1.5 pl-9 pr-12 text-sm text-ink placeholder-faint transition focus:border-faint focus:bg-surface focus:outline-none focus:ring-2 focus:ring-surface-2"
          />
          <span className="pointer-events-none absolute right-3 top-1/2 hidden -translate-y-1/2 items-center gap-0.5 rounded border border-line bg-[#08090b] px-1.5 py-0.5 font-mono text-[10px] text-faint xl:inline-flex">
            ⌘K
          </span>
        </div>

        <span className="inline-flex shrink-0 items-center gap-1.5 rounded-md border border-line bg-surface-2 px-2.5 py-1 text-[11px] font-medium text-faint">
          <span
            className={`h-2 w-2 rounded-full ${marketStatus.color} ${
              marketStatus.color.includes('up') || marketStatus.color.includes('accent')
                ? 'animate-pulse shadow-[0_0_8px_currentColor]'
                : ''
            }`}
          />
          {marketStatus.text}
        </span>

        <GlobalDataFreshnessControl
          language={language}
          controller={dataFreshness}
        />

        <button
          onClick={onToggleTheme}
          aria-label="Toggle theme"
          title={theme === 'dark' ? '切换到浅色' : '切换到深色'}
          className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-line bg-surface text-muted transition hover:text-ink hover:border-faint"
        >
          {theme === 'dark' ? <Moon className="h-[15px] w-[15px]" strokeWidth={1.8} /> : <Sun className="h-[15px] w-[15px]" strokeWidth={1.8} />}
        </button>

        <div className="inline-flex shrink-0 rounded-lg border border-line bg-surface p-0.5 text-[11px] font-semibold">
          <button onClick={() => language === 'en' && onToggleLanguage()} className={`rounded-md px-2.5 py-1.5 transition ${language === 'zh' ? 'bg-surface-3 text-ink' : 'text-muted hover:text-ink'}`}>中</button>
          <button onClick={() => language === 'zh' && onToggleLanguage()} className={`rounded-md px-2.5 py-1.5 transition ${language === 'en' ? 'bg-surface-3 text-ink' : 'text-muted hover:text-ink'}`}>EN</button>
        </div>

        <a
          href="/tactical"
          className="inline-flex min-h-[36px] shrink-0 items-center rounded-lg px-3.5 py-2 text-[13px] font-semibold text-[#1a0f08] transition hover:brightness-110"
          style={{
            background: 'linear-gradient(135deg, #eda57f 0%, #d98a6a 55%, #c2754f 100%)',
            boxShadow: '0 1px 0 rgba(255,255,255,0.25) inset, 0 4px 14px rgba(217,138,106,0.22)',
          }}
        >
          Paper 模拟
        </a>
      </div>
    </header>
  );
};

const MobileTopBar: React.FC<{
  searchQuery: string;
  onSearchChange: (value: string) => void;
  onSearchFocus: () => void;
  language: 'zh' | 'en';
  menuOpen: boolean;
  onToggleMenu: () => void;
  stockChrome?: boolean;
  dataFreshness: GlobalDataFreshnessController;
}> = ({
  searchQuery,
  onSearchChange,
  onSearchFocus,
  language,
  menuOpen,
  onToggleMenu,
  stockChrome,
  dataFreshness,
}) => {
  const location = useLocation();

  return (
  <div className="wails-mobile-top sticky top-0 z-30 border-b border-line bg-[#08090b]/92 backdrop-blur-md lg:hidden">
    <div className="flex h-[56px] items-center gap-3 px-4">
      <Link to="/" onClick={() => menuOpen && onToggleMenu()} className="group flex min-w-0 flex-1 items-center gap-2.5">
        <StockGodLogo />
        <span className="min-w-0">
          <span className="block truncate text-[14px] font-black tracking-tight text-ink">我不是神 · Not a Stock God</span>
          <span className="block truncate font-mono text-[8px] uppercase tracking-[0.22em] text-faint">NOT A STOCK GOD</span>
        </span>
      </Link>
      {stockChrome ? (
        <button
          type="button"
          onClick={onSearchFocus}
          aria-label="搜索"
          className="min-h-11 shrink-0 rounded-lg border border-line bg-surface px-3 py-2 text-xs font-medium text-muted transition hover:text-ink"
        >
          搜索
        </button>
      ) : (
        <button
          onClick={onToggleMenu}
          aria-expanded={menuOpen}
          aria-label={menuOpen ? '关闭菜单' : '打开菜单'}
          className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg border border-line bg-surface text-muted transition hover:text-ink"
        >
          {menuOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
        </button>
      )}
    </div>
    {menuOpen && !stockChrome && (
      <nav className="grid grid-cols-2 gap-1 border-t border-line px-3 py-2">
        {navItems.map((item) => {
          const Icon = item.icon;
          const basePath = item.path.split('#')[0];
          const hash = item.path.includes('#') ? item.path.split('#')[1] : '';
          const active = hash
            ? location.pathname === basePath && location.hash === `#${hash}`
            : location.pathname === item.path || (item.path !== '/' && location.pathname.startsWith(item.path) && !location.hash);
          return (
            <Link
              key={item.path}
              to={item.path}
              onClick={onToggleMenu}
              className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition ${
                active ? 'bg-accent/10 text-accent' : 'text-muted hover:bg-surface-2 hover:text-ink'
              }`}
            >
              <Icon className="h-4 w-4" />
              <span>{language === 'en' ? item.labelEn : item.label}</span>
            </Link>
          );
        })}
      </nav>
    )}
    <div className="px-4 pb-3">
      <div className="mb-2 flex justify-end">
        <GlobalDataFreshnessControl
          language={language}
          controller={dataFreshness}
          compact
        />
      </div>
      <div className="relative">
        <Search className="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-faint" />
        <input
          value={searchQuery}
          onChange={(event) => onSearchChange(event.target.value)}
          onFocus={onSearchFocus}
          placeholder={labels[language].searchPlaceholder}
          className="w-full rounded-lg border border-line bg-surface py-1.5 pl-9 pr-12 text-sm text-ink placeholder-faint transition focus:border-faint focus:bg-surface focus:outline-none focus:ring-2 focus:ring-surface-2"
        />
      </div>
    </div>
  </div>
  );
};

const BottomNav: React.FC = () => {
  const location = useLocation();
  const language = useUiStore((s) => s.language);
  const primary = navItems.filter((item) =>
    ['/market', '/scan', '/etf', '/etf#cn-tools', '/whales', '/portfolio'].includes(item.path)
  );

  return (
    <nav className="fixed inset-x-0 bottom-0 z-40 border-t border-line bg-[#08090b]/92 pb-[env(safe-area-inset-bottom)] backdrop-blur-md lg:hidden">
      <div className="grid grid-cols-6">
        {primary.map((item) => {
          const Icon = item.icon;
          const basePath = item.path.split('#')[0];
          const hash = item.path.includes('#') ? item.path.split('#')[1] : '';
          const active = hash
            ? location.pathname === basePath && location.hash === `#${hash}`
            : location.pathname === item.path || (item.path !== '/' && location.pathname.startsWith(item.path) && !location.hash);
          return (
            <Link
              key={item.path}
              to={item.path}
              className={`flex min-h-[48px] flex-col items-center justify-center gap-0.5 py-2 text-[10px] transition ${
                active ? 'text-accent' : 'text-muted'
              }`}
            >
              <Icon className="h-5 w-5" strokeWidth={1.8} />
              <span>{language === 'en' ? item.labelEn : item.label}</span>
            </Link>
          );
        })}
      </div>
    </nav>
  );
};

const WatchlistRail: React.FC<{
  language: 'zh' | 'en';
  onOpenWatchlist: () => void;
  onSearch?: () => void;
  showSearch?: boolean;
}> = ({ language, onOpenWatchlist, onSearch, showSearch }) => {
  const watchlist = usePortfolioStore((state) => state.watchlist);

  return (
    <div className="fixed left-0 top-1/2 z-30 hidden -translate-y-1/2 flex-col gap-2 sm:flex lg:hidden">
      {showSearch && onSearch ? (
        <button
          type="button"
          onClick={onSearch}
          aria-label="搜索"
          className="flex flex-col items-center gap-1 rounded-r-xl border border-l-0 border-line bg-surface px-2 py-2.5 text-[11px] text-muted transition hover:bg-surface-2 hover:text-ink"
        >
          <Search className="h-3.5 w-3.5" />
          <span>{language === 'en' ? 'Search' : '搜索'}</span>
        </button>
      ) : null}
      <button
        type="button"
        onClick={onOpenWatchlist}
        aria-label={language === 'en' ? 'Open watchlist' : '打开观察列表'}
        className="flex flex-col items-center gap-1.5 rounded-r-xl border border-l-0 border-line bg-surface py-3 pl-2 pr-2.5 transition hover:bg-surface-2"
      >
        <span className="font-mono text-[13px] font-semibold text-ink">{watchlist.length}</span>
        <span className="text-[11px] text-faint [writing-mode:vertical-rl]">{labels[language].observe}</span>
      </button>
    </div>
  );
};

const WatchlistDrawer: React.FC<{
  open: boolean;
  language: 'zh' | 'en';
  onClose: () => void;
}> = ({ open, language, onClose }) => {
  const navigate = useNavigate();
  const { watchlist, removeFromWatchlist } = usePortfolioStore();
  const drawerRef = useDialogFocus<HTMLDivElement>(open, onClose);

  return (
    <InertDiv
      ref={drawerRef}
      role="dialog"
      aria-modal="true"
      aria-label={language === 'en' ? 'Watchlist' : '观察列表'}
      tabIndex={-1}
      inert={!open ? '' : undefined}
      className={`fixed inset-0 z-50 flex transition ${open ? 'pointer-events-auto bg-black/45 backdrop-blur-sm' : 'pointer-events-none bg-transparent'}`}
      onClick={open ? onClose : undefined}
      aria-hidden={!open}
    >
      <div
        className={`flex h-full w-72 flex-col border-r border-line bg-surface shadow-2xl transition-transform duration-200 ${
          open ? 'translate-x-0' : '-translate-x-full'
        }`}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-line px-4 py-3.5">
          <span className="flex items-center gap-1.5 text-[14px] font-semibold text-ink">
            <Star className="h-4 w-4 fill-accent text-accent" />
            {language === 'en' ? `Watchlist (${watchlist.length})` : `观察列表 (${watchlist.length})`}
          </span>
          <button
            type="button"
            onClick={onClose}
            aria-label={language === 'en' ? 'Close' : '收起'}
            className="rounded px-2 py-1 text-xs text-faint transition hover:bg-surface-2 hover:text-ink"
          >
            {language === 'en' ? 'Close' : '收起'}
          </button>
        </div>

        <div className="flex-1 space-y-1 overflow-y-auto p-2">
          {watchlist.length === 0 ? (
            <div className="flex h-48 flex-col items-center justify-center px-4 py-8 text-center">
              <Star className="mb-2 h-8 w-8 text-faint" />
              <p className="text-sm font-semibold text-ink">{language === 'en' ? 'No symbols watched yet' : '还没观察任何股票'}</p>
              <p className="mt-1 text-[11px] text-faint">{language === 'en' ? 'Star symbols on heatmap or scanner' : '去热力图或扫描页点收藏添加'}</p>
            </div>
          ) : (
            watchlist.map((item) => (
              <div
                key={item.symbol}
                className="group flex cursor-pointer items-center justify-between rounded-lg p-2 transition hover:bg-surface-2"
                onClick={() => {
                  navigate(`/stock/${item.symbol}`);
                  onClose();
                }}
              >
                <div className="min-w-0 flex-1">
                  <div className="font-mono text-sm font-semibold text-ink">{item.symbol}</div>
                  <div className="truncate text-xs text-muted">{item.name}</div>
                </div>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    removeFromWatchlist(item.symbol);
                  }}
                  className="rounded p-1 text-faint opacity-0 transition hover:text-down group-hover:opacity-100"
                  title={language === 'en' ? 'Remove' : '移出观察'}
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
            ))
          )}
        </div>

        <div className="border-t border-line bg-surface-2/40 p-3">
          <button
            onClick={() => {
              navigate('/portfolio');
              onClose();
            }}
            className="block w-full rounded-lg border border-line bg-surface px-3 py-2 text-center text-xs font-semibold text-ink transition hover:bg-surface-3"
          >
            {language === 'en' ? 'Full watchlist' : '完整观察页'}
          </button>
        </div>
      </div>
    </InertDiv>
  );
};

const CommandPalette: React.FC<{
  open: boolean;
  query: string;
  language: 'zh' | 'en';
  onQueryChange: (value: string) => void;
  onClose: () => void;
}> = ({ open, query, language, onQueryChange, onClose }) => {
  const navigate = useNavigate();
  const [results, setResults] = useState<Stock[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchError, setSearchError] = useState('');
  const [searchRetryVersion, setSearchRetryVersion] = useState(0);
  const searchGate = useRef(createLatestRequestGate());
  const t = labels[language];
  const paletteRef = useDialogFocus<HTMLDivElement>(open, onClose);

  useEffect(() => {
    const request = searchGate.current.begin();
    if (!open || query.trim().length < 1) {
      setResults([]);
      setSearchError('');
      return;
    }
    const timeout = window.setTimeout(async () => {
      setLoading(true);
      setSearchError('');
      try {
        const trimmedQuery = query.trim();
        const market = /^\d{6}$/.test(trimmedQuery) || /[\u3400-\u9fff]/.test(trimmedQuery) ? 'cn' : 'us';
        const response = await apiGet<SearchResponse>(`/api/stocks/search?q=${encodeURIComponent(trimmedQuery)}&market=${market}`);
        if (!searchGate.current.isCurrent(request)) return;
        setResults(response.results || []);
      } catch (error) {
        if (!searchGate.current.isCurrent(request)) return;
        console.error('Search failed:', error);
        setResults([]);
        setSearchError(language === 'zh' ? '股票搜索服务暂不可用' : 'Stock search is temporarily unavailable');
      } finally {
        if (searchGate.current.isCurrent(request)) setLoading(false);
      }
    }, 180);
    return () => {
      window.clearTimeout(timeout);
      searchGate.current.invalidate();
    };
  }, [language, open, query, searchRetryVersion]);

  const filteredQuickLinks = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return quickLinks;
    return quickLinks.filter((item) => `${item.label} ${item.labelEn} ${item.path}`.toLowerCase().includes(q));
  }, [query]);

  const go = (path: string) => {
    navigate(path);
    onClose();
  };

  if (!open) return null;

  return (
    <div ref={paletteRef} role="dialog" aria-modal="true" aria-label={t.commandTitle} tabIndex={-1} className="fixed inset-0 z-50 bg-black/55 px-4 py-16 backdrop-blur-sm" onMouseDown={onClose}>
      <div className="mx-auto max-w-xl overflow-hidden rounded-xl border border-line bg-surface shadow-2xl" onMouseDown={(event) => event.stopPropagation()}>
        <div className="flex items-center gap-2 border-b border-line px-4 py-3">
          <Search className="h-4 w-4 text-faint" />
          <input
            autoFocus
            value={query}
            onChange={(event) => onQueryChange(event.target.value)}
            placeholder={t.commandTitle}
            className="min-w-0 flex-1 bg-transparent text-sm text-ink outline-none placeholder:text-faint"
          />
          <button onClick={onClose} aria-label={language === 'zh' ? '关闭搜索' : 'Close search'} className="rounded-md p-1 text-faint transition hover:bg-surface-2 hover:text-ink">
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="max-h-[420px] overflow-y-auto p-2">
          {filteredQuickLinks.length > 0 && (
            <div className="mb-2">
              <div className="px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.18em] text-faint">{t.quick}</div>
              {filteredQuickLinks.map((item) => (
                <button key={item.path} onClick={() => go(item.path)} className="flex w-full items-center justify-between rounded-lg px-3 py-2 text-left text-sm transition hover:bg-surface-2">
                  <span className="text-ink">{language === 'zh' ? item.label : item.labelEn}</span>
                  <span className="font-mono text-xs text-faint">{item.path}</span>
                </button>
              ))}
            </div>
          )}

          <div>
            <div className="px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.18em] text-faint">Stocks</div>
            {loading && <div className="px-3 py-4 text-sm text-muted">Searching...</div>}
            {!loading && searchError && (
              <div role="alert" className="px-3 py-4 text-sm text-down">
                <p>{searchError}</p>
                <button type="button" onClick={() => setSearchRetryVersion((value) => value + 1)} className="mt-2 inline-flex items-center gap-1 rounded-md border border-line px-2.5 py-1.5 text-xs font-semibold text-accent hover:bg-surface-2">
                  <RefreshCw className="h-3.5 w-3.5" />{language === 'zh' ? '重试搜索' : 'Retry search'}
                </button>
              </div>
            )}
            {!loading && !searchError && results.length === 0 && query.trim() && <div className="px-3 py-4 text-sm text-muted">{t.empty}</div>}
            {results.map((stock) => (
              <button key={stock.symbol} onClick={() => go(`/stock/${stock.symbol}`)} className="flex w-full items-center justify-between rounded-lg px-3 py-2 text-left transition hover:bg-surface-2">
                <span>
                  <span className="font-mono text-sm font-semibold text-ink">{stock.symbol}</span>
                  <span className="ml-2 text-sm text-muted">{stock.name}</span>
                </span>
                <span className="font-mono text-xs text-faint">{stock.sector}</span>
              </button>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
};

export const StockGodShell: React.FC<StockGodShellProps> = ({
  title,
  children,
  searchQuery = '',
  onSearchChange = () => {},
  macroTickers = [],
}) => {
  const location = useLocation();
  const [shellSearch, setShellSearch] = useState(searchQuery);
  const [commandOpen, setCommandOpen] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [watchlistDrawerOpen, setWatchlistDrawerOpen] = useState(false);
  const dataFreshness = useGlobalDataFreshness();
  const theme = useUiStore((s) => s.theme);
  const language = useUiStore((s) => s.language);
  const toggleTheme = useUiStore((s) => s.toggleTheme);
  const toggleLanguage = useUiStore((s) => s.toggleLanguage);

  useEffect(() => {
    setShellSearch(searchQuery);
  }, [searchQuery]);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.documentElement.classList.toggle('stockgod-light', theme === 'light');
    document.documentElement.lang = language === 'zh' ? 'zh-CN' : 'en';
  }, [theme, language]);

  useEffect(() => {
    let pageTitle = title;
    const path = location.pathname;
    if (path === '/market') {
      pageTitle = '我不是神 · Not a Stock God';
    } else if (path === '/scan') {
      pageTitle = '全市场扫描 · 美股+A股五方判读 · 我不是神';
    } else if (path === '/gpu-prices') {
      pageTitle = 'GPU 算力租金跟踪 | 我不是神';
    } else if (path === '/etf') {
      pageTitle = location.hash === '#cn-tools' ? 'A 股工具箱 | 我不是神' : 'ETF · 板块业绩 | 我不是神';
    } else if (path === '/whales') {
      pageTitle = '聪明钱 · 名人持仓 + 国会交易 | 我不是神';
    } else if (path.startsWith('/whales/congress/')) {
      pageTitle = `${title}的国会交易披露 | 我不是神`;
    } else if (path.startsWith('/whales/')) {
      const id = path.split('/')[2] || '';
      const gurus: Record<string, string> = {
        'howard-marks': '霍华德·马克斯(Oaktree Capital Management)',
        'warren-buffett': '沃伦·巴菲特(Berkshire Hathaway)',
        'michael-burry': '迈克尔·贝里(Scion Asset Management)',
        'bill-gates': '比尔·盖茨(Bill & Melinda Gates Foundation)',
        'charlie-munger': '查理·芒格(Daily Journal)',
        'li-lu': '李录(Himalaya Capital)',
        'charlie-munger-estate': '查理·芒格遗产',
        'bill-ackman': '比尔·阿克曼(Pershing Square)',
        'mohnish-pabrai': '莫尼什·帕伯莱(Pabrai Investment Funds)',
        'guy-spier': '盖伊·斯皮尔(Aquamarine Capital)',
        'david-tepper': '大卫·泰珀(Appaloosa Management)',
        'ray-dalio': '雷·达里奥(Bridgewater Associates)',
        'ken-griffin': '肯·格里芬(Citadel Advisors)',
        'steve-cohen': '史蒂夫·科恩(Point72 Asset Management)',
        'stanley-druckenmiller': '斯坦利·德鲁肯米勒(Duquesne Family Office)',
        'chase-coleman': '蔡斯·科尔曼(Tiger Global Management)',
        'jim-simons': '吉姆·西模斯(Renaissance Technologies)',
        'daniel-loeb': '丹尼尔·勒布(Third Point)',
        'nelson-peltz': '尼尔森·佩尔茨(Trian Fund Management)',
        'carl-icahn': '卡尔·伊坎(Icahn Enterprises)',
      };
      const name = gurus[id] || title || '聪明钱';
      pageTitle = `${name}的 13F 持仓 · 我不是神`;
    } else if (path === '/arena') {
      pageTitle = '五神对决 · 段永平/巴菲特/Serenity/德鲁肯米勒/情绪 虚拟盘 · 我不是神';
    } else if (path === '/reports') {
      pageTitle = '盘报 · 盘前看点 + 收盘复盘 · 我不是神';
    } else if (path === '/macro') {
      pageTitle = '宏观驾驶舱 · 我不是神';
    } else if (path === '/portfolio') {
      pageTitle = '我不是神 · Not a Stock God';
    } else if (path.startsWith('/stock/')) {
      pageTitle = `${title}股价、估值与机构持仓 analysis · 我不是神`;
    } else if (path === '/about') {
      pageTitle = '关于 / 方法论 · About | 我不是神';
    } else if (path === '/terms') {
      pageTitle = '服务条款 · Terms | 我不是神';
    } else if (path === '/privacy') {
      pageTitle = '隐私政策 · Privacy | 我不是神';
    } else if (path === '/how-to-buy') {
      pageTitle = 'Paper 模拟交易 | 我不是神';
    } else {
      pageTitle = title ? `${title} · 我不是神` : '我不是神 · Not a Stock God';
    }
    // Correct some specific wording matching the live site
    if (path.startsWith('/stock/')) {
      const sym = (path.split('/')[2] || '').toUpperCase();
      const companies: Record<string, string> = {
        'NVDA': 'NVIDIA Corporation',
        'AAPL': 'Apple Inc.',
        'MSFT': 'Microsoft Corporation',
        'AMZN': 'Amazon.com Inc.',
        'GOOGL': 'Alphabet Inc.',
        'GOOG': 'Alphabet Inc.',
        'META': 'Meta Platforms Inc.',
        'TSLA': 'Tesla Inc.',
        'AVGO': 'Broadcom Inc.',
        'LLY': 'Eli Lilly and Company',
      };
      const comp = companies[sym];
      if (comp) {
        pageTitle = `${comp}(${sym})股价、估值与机构持仓分析 | 我不是神`;
      } else {
        pageTitle = `${sym}股价、估值与机构持仓分析 | 我不是神`;
      }
    }
    document.title = pageTitle;
  }, [title, location.pathname, location.hash]);

  useEffect(() => {
    setMobileMenuOpen(false);
  }, [location.pathname]);

  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setCommandOpen(true);
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, []);

  const handleSearchChange = (value: string) => {
    setShellSearch(value);
    onSearchChange(value);
  };

  const light = theme === 'light';

  return (
    <div className={`min-h-screen font-sans ${light ? 'bg-[#f4f1ed] text-[#171717] [--shell-bg:#f4f1ed] [--shell-surface:#ffffff] [--shell-ink:#171717]' : 'bg-[#08090b] text-[#ecedf0] [--shell-bg:#08090b] [--shell-surface:#111317] [--shell-ink:#ecedf0]'}`}>
      <div className="lg:flex">
        <ShellSidebar onOpenWatchlist={() => setWatchlistDrawerOpen(true)} />
        <div className="min-w-0 flex-1">
          <ShellHeader
            title={title}
            searchQuery={shellSearch}
            onSearchChange={handleSearchChange}
            onSearchFocus={() => setCommandOpen(true)}
            theme={theme}
            onToggleTheme={toggleTheme}
            language={language}
            onToggleLanguage={toggleLanguage}
            dataFreshness={dataFreshness}
          />
          {macroTickers.length > 0 && (
            <div className="hidden border-b border-line bg-[#08090b]/60 px-6 py-1.5 lg:block">
              <div className="flex gap-4 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                {macroTickers.map(([label, value, change]) => (
                  <div key={label} className="flex shrink-0 items-baseline gap-1.5 whitespace-nowrap font-mono text-[11px]">
                    <span className="text-faint">{label}</span>
                    <span className="text-ink">{value}</span>
                    <span className={String(change).startsWith('-') ? 'text-down' : 'text-up'}>{change}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
          <MobileTopBar
            searchQuery={shellSearch}
            onSearchChange={handleSearchChange}
            onSearchFocus={() => setCommandOpen(true)}
            language={language}
            menuOpen={mobileMenuOpen}
            onToggleMenu={() => setMobileMenuOpen((value) => !value)}
            stockChrome={location.pathname.startsWith('/stock/')}
            dataFreshness={dataFreshness}
          />
          <WatchlistRail
            language={language}
            onOpenWatchlist={() => setWatchlistDrawerOpen(true)}
            onSearch={() => setCommandOpen(true)}
            showSearch
          />
          <main className="mx-auto max-w-[1280px] px-4 pb-[calc(5rem+env(safe-area-inset-bottom))] pt-3 sm:px-6 lg:pb-8">{children}</main>
        </div>
      </div>
      <BottomNav />
      <CommandPalette open={commandOpen} query={shellSearch} language={language} onQueryChange={handleSearchChange} onClose={() => setCommandOpen(false)} />
      <WatchlistDrawer open={watchlistDrawerOpen} language={language} onClose={() => setWatchlistDrawerOpen(false)} />
    </div>
  );
};
