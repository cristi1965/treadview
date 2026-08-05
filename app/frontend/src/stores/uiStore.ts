import { create } from 'zustand';

export type ShellLanguage = 'zh' | 'en';
export type ShellTheme = 'dark' | 'light';

interface UiStore {
  language: ShellLanguage;
  theme: ShellTheme;
  setLanguage: (language: ShellLanguage) => void;
  setTheme: (theme: ShellTheme) => void;
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

export const useUiStore = create<UiStore>((set, get) => ({
  language: readLang(),
  theme: readTheme(),
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
  toggleLanguage: () => get().setLanguage(get().language === 'zh' ? 'en' : 'zh'),
  toggleTheme: () => get().setTheme(get().theme === 'dark' ? 'light' : 'dark'),
}));
