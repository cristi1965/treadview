import React from 'react';
import { FileSearch, Moon, Search, ShieldCheck, Sun } from 'lucide-react';
import { Link } from 'react-router-dom';
import { Input } from '../common/Input';
import { getUSMarketStatus, useI18n } from '../../i18n';
import { useUiStore } from '../../stores/uiStore';
import { GlobalDataFreshnessControl, type GlobalDataFreshnessController } from './GlobalDataFreshnessControl';

interface HeaderProps {
  title?: string;
  dataFreshness: GlobalDataFreshnessController;
  surface?: 'default' | 'research';
}

export const Header: React.FC<HeaderProps> = ({ title, dataFreshness, surface = 'default' }) => {
  const { language, t } = useI18n();
  const theme = useUiStore((s) => s.theme);
  const toggleLanguage = useUiStore((s) => s.toggleLanguage);
  const toggleTheme = useUiStore((s) => s.toggleTheme);
  const [searchQuery, setSearchQuery] = React.useState('');
  const marketStatus = getUSMarketStatus(language);

  return (
    <header className="wails-titlebar sticky top-0 z-30 hidden h-[56px] items-center gap-4 border-b border-line bg-base/85 px-6 backdrop-blur-md lg:flex">
      {title && (
        <span className="shrink-0 text-[14px] font-semibold tracking-tight text-ink">
          {title}
        </span>
      )}

      <div className="ml-auto flex min-w-0 items-center gap-2.5">
        {surface === 'research' ? (
          <nav className="flex min-w-0 items-center gap-1" aria-label={language === 'zh' ? '研究产品边界' : 'Research product boundary'}>
            <span className="mr-1 inline-flex items-center gap-1.5 rounded-md border border-sky-500/30 bg-sky-500/10 px-2 py-1 text-[11px] font-semibold text-sky-200"><ShieldCheck size={13} />{language === 'zh' ? '历史研究 + Paper 模拟 · 非实时' : 'Historical research + Paper simulation · not live'}</span>
            <Link to="/dashboard" className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[11px] font-semibold text-muted hover:bg-surface-2 hover:text-ink"><FileSearch size={13} />{language === 'zh' ? '研究入口' : 'Research'}</Link>
            <Link to="/history" className="rounded-md px-2 py-1 text-[11px] font-semibold text-muted hover:bg-surface-2 hover:text-ink">{language === 'zh' ? '证据记录' : 'Evidence'}</Link>
            <Link to="/settings#product-readiness" className="rounded-md px-2 py-1 text-[11px] font-semibold text-muted hover:bg-surface-2 hover:text-ink">{language === 'zh' ? '产品门禁' : 'Gates'}</Link>
          </nav>
        ) : <div className="w-[260px] shrink">
          <Input
            value={searchQuery}
            onChange={setSearchQuery}
            placeholder={t('searchPlaceholder')}
            icon={
              <Search className="h-3.5 w-3.5" />
            }
          />
        </div>}

        {surface === 'default' && <div className="shrink-0">
          <span className="inline-flex shrink-0 items-center gap-1.5 rounded-md border border-line bg-surface-2 px-2 py-1 text-[11px] font-medium text-faint">
            <span className={`h-1.5 w-1.5 rounded-full ${marketStatus.color}`} />
            {marketStatus.text}
          </span>
        </div>}

        {surface === 'default' && <GlobalDataFreshnessControl language={language} controller={dataFreshness} />}

        <button
          className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-line bg-surface text-muted transition hover:text-ink"
          aria-label="Toggle theme"
          title={theme === 'dark' ? t('themeToLight') : t('themeToDark')}
          onClick={toggleTheme}
        >
          {theme === 'dark' ? (
            <Moon className="h-[15px] w-[15px]" strokeWidth={1.8} />
          ) : (
            <Sun className="h-[15px] w-[15px]" strokeWidth={1.8} />
          )}
        </button>

        <div className="inline-flex shrink-0 rounded-lg border border-line bg-surface p-0.5 text-[11px] font-semibold">
          <button
            onClick={() => language === 'en' && toggleLanguage()}
            className={`rounded-md px-2.5 py-1.5 transition ${language === 'zh' ? 'bg-surface-3 text-ink' : 'text-muted hover:text-ink'}`}
          >
            中
          </button>
          <button
            onClick={() => language === 'zh' && toggleLanguage()}
            className={`rounded-md px-2.5 py-1.5 transition ${language === 'en' ? 'bg-surface-3 text-ink' : 'text-muted hover:text-ink'}`}
          >
            EN
          </button>
        </div>

        {surface === 'default' && <a
          href="/tactical"
          className="inline-flex min-h-[36px] shrink-0 items-center rounded-lg px-3.5 py-2 text-[13px] font-semibold text-[#1a0f08] transition hover:brightness-110"
          style={{
            background: 'linear-gradient(135deg, #eda57f 0%, #d98a6a 55%, #c2754f 100%)',
            boxShadow: '0 1px 0 rgba(255,255,255,0.25) inset, 0 4px 14px rgba(217,138,106,0.22)',
          }}
        >
          Paper 模拟
        </a>}
      </div>
    </header>
  );
};
