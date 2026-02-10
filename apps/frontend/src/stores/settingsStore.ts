import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type ViewMode = 'grid' | 'table';
export type SortBy = 'name' | 'modTime' | 'size';
export type SortOrder = 'ascend' | 'descend';
export type Language = 'ko' | 'en';

interface ServerSettings {
  // HTTP Server
  httpEnabled: boolean;
  httpPort: number;

  // WebDAV
  webdavEnabled: boolean;
  webdavPort: number;

  // FTP
  ftpEnabled: boolean;
  ftpPort: number;

  // SFTP
  sftpEnabled: boolean;
  sftpPort: number;
}

interface UISettings {
  // Theme
  theme: 'light' | 'dark';

  // File browser defaults
  defaultViewMode: ViewMode;
  defaultSortBy: SortBy;
  defaultSortOrder: SortOrder;
  showHiddenFiles: boolean;

  // General
  language: Language;
}

interface SettingsStore extends UISettings, ServerSettings {
  // UI Actions
  setTheme: (theme: 'light' | 'dark') => void;
  toggleTheme: () => void;
  setDefaultViewMode: (mode: ViewMode) => void;
  setDefaultSort: (sortBy: SortBy, sortOrder: SortOrder) => void;
  setShowHiddenFiles: (show: boolean) => void;
  setLanguage: (lang: Language) => void;

  // Server Actions
  setHttpEnabled: (enabled: boolean) => void;
  setHttpPort: (port: number) => void;
  setWebdavEnabled: (enabled: boolean) => void;
  setWebdavPort: (port: number) => void;
  setFtpEnabled: (enabled: boolean) => void;
  setFtpPort: (port: number) => void;
  setSftpEnabled: (enabled: boolean) => void;
  setSftpPort: (port: number) => void;

  resetToDefaults: () => void;
}

const defaultServerSettings: ServerSettings = {
  httpEnabled: true,
  httpPort: 3000,
  webdavEnabled: true,
  webdavPort: 3000, // WebDAV uses same port as HTTP (path: /dav/)
  ftpEnabled: false,
  ftpPort: 21,
  sftpEnabled: false,
  sftpPort: 22,
};

const defaultUISettings: UISettings = {
  theme: 'dark',
  defaultViewMode: 'grid',
  defaultSortBy: 'name',
  defaultSortOrder: 'ascend',
  showHiddenFiles: false,
  language: 'ko',
};

const defaultSettings = {
  ...defaultUISettings,
  ...defaultServerSettings,
};

export const useSettingsStore = create<SettingsStore>()(
  persist(
    (set) => ({
      ...defaultSettings,

      setTheme: (theme) => set({ theme }),

      toggleTheme: () =>
        set((state) => ({ theme: state.theme === 'dark' ? 'light' : 'dark' })),

      setDefaultViewMode: (mode) => set({ defaultViewMode: mode }),

      setDefaultSort: (sortBy, sortOrder) =>
        set({ defaultSortBy: sortBy, defaultSortOrder: sortOrder }),

      setShowHiddenFiles: (show) => set({ showHiddenFiles: show }),

      setLanguage: (lang) => set({ language: lang }),

      // Server actions
      setHttpEnabled: (enabled) => set({ httpEnabled: enabled }),
      setHttpPort: (port) => set({ httpPort: port }),
      setWebdavEnabled: (enabled) => set({ webdavEnabled: enabled }),
      setWebdavPort: (port) => set({ webdavPort: port }),
      setFtpEnabled: (enabled) => set({ ftpEnabled: enabled }),
      setFtpPort: (port) => set({ ftpPort: port }),
      setSftpEnabled: (enabled) => set({ sftpEnabled: enabled }),
      setSftpPort: (port) => set({ sftpPort: port }),

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
