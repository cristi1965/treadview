import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import { I18nKey, useI18n } from '../../i18n';

interface NavItem {
  path: string;
  labelKey: I18nKey;
  icon: string;
  group: 'cockpit' | 'main';
}

const navItems: NavItem[] = [
  { path: '/dashboard', labelKey: 'nav.research', icon: '🔎', group: 'cockpit' },
  { path: '/history', labelKey: 'nav.history', icon: '📜', group: 'cockpit' },
  { path: '/lab', labelKey: 'nav.lab', icon: '🧪', group: 'cockpit' },
  { path: '/copilot', labelKey: 'nav.copilot', icon: '💬', group: 'cockpit' },
  { path: '/tactical', labelKey: 'nav.tactical', icon: '🌙', group: 'cockpit' },
  { path: '/journal', labelKey: 'nav.journal', icon: '📓', group: 'cockpit' },
  { path: '/dashboard/macro', labelKey: 'nav.macro', icon: '📅', group: 'cockpit' },
  { path: '/settings', labelKey: 'nav.settings', icon: '⚙️', group: 'cockpit' },
  { path: '/market', labelKey: 'nav.heatmap', icon: '🔥', group: 'main' },
  { path: '/scan', labelKey: 'nav.scan', icon: '📋', group: 'main' },
  { path: '/multichart', labelKey: 'nav.multichart', icon: '🖥', group: 'main' },
  { path: '/gpu-prices', labelKey: 'nav.gpu', icon: '⚡', group: 'main' },
  { path: '/etf', labelKey: 'nav.etf', icon: '📊', group: 'main' },
  { path: '/etf#cn-tools', labelKey: 'nav.ashare', icon: '🛡', group: 'main' },
  { path: '/whales', labelKey: 'nav.whales', icon: '🐋', group: 'main' },
  { path: '/arena', labelKey: 'nav.arena', icon: '⚔️', group: 'main' },
  { path: '/reports', labelKey: 'nav.reports', icon: '📰', group: 'main' },
  { path: '/notes', labelKey: 'nav.notes', icon: '📝', group: 'main' },
  { path: '/portfolio', labelKey: 'nav.mine', icon: '⭐', group: 'main' },
];

const NavLink: React.FC<{ item: NavItem; isActive: boolean; label: string }> = ({ item, isActive, label }) => (
  <Link
    to={item.path}
    className={`flex items-center gap-3 rounded-lg px-3 py-[9px] text-[13.5px] font-medium transition ${
      isActive ? 'bg-accent/10 text-accent' : 'text-muted hover:bg-surface-2 hover:text-ink'
    }`}
  >
    <span className="shrink-0 text-[18px]">{item.icon}</span>
    <span>{label}</span>
  </Link>
);

export const Sidebar: React.FC = () => {
  const location = useLocation();
  const { t } = useI18n();

  const isItemActive = (item: NavItem) => {
    const basePath = item.path.split('#')[0];
    const hash = item.path.includes('#') ? item.path.split('#')[1] : '';
    if (hash) return location.pathname === basePath && location.hash === `#${hash}`;
    if (location.pathname === basePath && !location.hash) return true;
    const moreSpecificMatch = navItems.some((candidate) => {
      const candidatePath = candidate.path.split('#')[0];
      return candidatePath !== basePath && candidatePath.startsWith(`${basePath}/`) && location.pathname.startsWith(candidatePath);
    });
    return !moreSpecificMatch && !location.hash && location.pathname.startsWith(`${basePath}/`);
  };

  return (
    <aside className="wails-sidebar hidden h-screen w-[204px] shrink-0 border-r border-line bg-base/70 px-3 py-4 backdrop-blur-md lg:sticky lg:top-0 lg:flex lg:flex-col">
      <Link to="/" className="group mb-5 flex items-center gap-2.5 px-2">
        <img
          src="/logo.png"
          alt={t('brand')}
          className="h-[30px] w-[30px] shrink-0 rounded-[9px] object-cover ring-1 ring-white/10 transition group-hover:ring-accent/40"
        />
        <span className="flex min-w-0 flex-col leading-none">
          <span className="truncate text-[15px] font-semibold tracking-tight text-ink">
            {t('brand')}
          </span>
          <span className="mt-[3px] font-mono text-[8px] uppercase tracking-[0.28em] text-faint">
            {t('nav.multiAgent')}
          </span>
        </span>
      </Link>

      <nav className="flex flex-1 flex-col gap-0.5 overflow-y-auto">
        <div className="px-3 pb-1 font-mono text-[9px] uppercase tracking-[0.2em] text-faint">{t('cockpit')}</div>
        {navItems.filter((i) => i.group === 'cockpit').map((item) => (
          <NavLink key={item.path} item={item} isActive={isItemActive(item)} label={t(item.labelKey)} />
        ))}

        <div className="mx-2 my-2 border-t border-line" />
        <div className="px-3 pb-1 font-mono text-[9px] uppercase tracking-[0.2em] text-faint">{t('brand')}</div>
        {navItems.filter((i) => i.group === 'main').map((item) => (
          <NavLink key={item.path} item={item} isActive={isItemActive(item)} label={t(item.labelKey)} />
        ))}
      </nav>
    </aside>
  );
};
