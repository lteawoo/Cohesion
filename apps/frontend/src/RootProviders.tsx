import { App as AntdApp, ConfigProvider } from 'antd';
import App from '@/App';
import { AuthProvider } from '@/features/auth/AuthContext';
import { useSettingsStore } from '@/stores/settingsStore';
import { buildCohesionThemeConfig } from '@/theme/themeConfig';

export default function RootProviders() {
  const currentTheme = useSettingsStore((state) => state.theme);
  const isDarkMode = currentTheme === 'dark';
  const themeClassName = isDarkMode ? 'app-theme-dark' : 'app-theme-light';

  return (
    <ConfigProvider theme={buildCohesionThemeConfig(isDarkMode)}>
      <AntdApp className={themeClassName}>
        <AuthProvider>
          <App />
        </AuthProvider>
      </AntdApp>
    </ConfigProvider>
  );
}
