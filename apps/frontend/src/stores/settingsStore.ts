import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type Language = 'ko' | 'en';

interface UISettings {
  // Theme
  theme: 'light' | 'dark';

  // General
  language: Language;
}

interface SettingsStore extends UISettings {
  // UI Actions
  setTheme: (theme: 'light' | 'dark') => void;
  toggleTheme: () => void;
  setLanguage: (lang: Language) => void;

  resetToDefaults: () => void;
}

const defaultUISettings: UISettings = {
  theme: 'dark',
  language: 'ko',
};

const defaultSettings = defaultUISettings;

export const useSettingsStore = create<SettingsStore>()(
  persist(
    (set) => ({
      ...defaultSettings,

      setTheme: (theme) => set({ theme }),

      toggleTheme: () =>
        set((state) => ({ theme: state.theme === 'dark' ? 'light' : 'dark' })),

      setLanguage: (lang) => set({ language: lang }),

      resetToDefaults: () => set(defaultSettings),
    }),
    {
      name: 'cohesion-settings',
    }
  )
);

// Backward compatibility: export hooks that match old themeStore API
export const useTheme = () => {
  const theme = useSettingsStore((state) => state.theme);
  const setTheme = useSettingsStore((state) => state.setTheme);
  const toggleTheme = useSettingsStore((state) => state.toggleTheme);

  return {
    isDarkMode: theme === 'dark',
    setTheme: (isDark: boolean) => setTheme(isDark ? 'dark' : 'light'),
    toggleTheme,
  };
};
