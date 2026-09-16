import { create } from 'zustand';
import type { ReportView } from '../utils/bilingual';

export type ShellLanguage = 'zh' | 'en';
export type ShellTheme = 'dark' | 'light';
export type { ReportView };

interface UiStore {
  language: ShellLanguage;
  theme: ShellTheme;
  reportView: ReportView;
  setLanguage: (language: ShellLanguage) => void;
  setTheme: (theme: ShellTheme) => void;
  setReportView: (view: ReportView) => void;
  toggleLanguage: () => void;
  toggleTheme: () => void;
}

const readLang = (): ShellLanguage => {
  try {
    return localStorage.getItem('stockgod_language') === 'en' ? 'en' : 'zh';
  } catch {
    return 'zh';
  }
};

const readTheme = (): ShellTheme => {
  try {
    return localStorage.getItem('stockgod_theme') === 'light' ? 'light' : 'dark';
  } catch {
    return 'dark';
  }
};

const readReportView = (): ReportView => {
  try {
    const v = localStorage.getItem('stockgod_report_view');
    if (v === 'zh' || v === 'en' || v === 'zh-en') return v;
  } catch {
    /* ignore */
  }
  return 'zh-en';
};

export const useUiStore = create<UiStore>((set, get) => ({
  language: readLang(),
  theme: readTheme(),
  reportView: readReportView(),
  setLanguage: (language) => {
    localStorage.setItem('stockgod_language', language);
    document.documentElement.lang = language === 'zh' ? 'zh-CN' : 'en';
    set({ language });
  },
  setTheme: (theme) => {
    localStorage.setItem('stockgod_theme', theme);
    document.documentElement.dataset.theme = theme;
    document.documentElement.classList.toggle('stockgod-light', theme === 'light');
    set({ theme });
  },
  setReportView: (view) => {
    localStorage.setItem('stockgod_report_view', view);
    set({ reportView: view });
  },
  toggleLanguage: () => get().setLanguage(get().language === 'zh' ? 'en' : 'zh'),
  toggleTheme: () => get().setTheme(get().theme === 'dark' ? 'light' : 'dark'),
}));
