import { App as AntdApp, ConfigProvider } from "antd";
import { useEffect } from "react";
import enUS from "antd/locale/en_US";
import koKR from "antd/locale/ko_KR";
import App from "@/App";
import { AuthProvider } from "@/features/auth/AuthContext";
import { useSettingsStore } from "@/stores/settingsStore";
import { buildCohesionThemeConfig } from "@/theme/themeConfig";
import i18n from "@/i18n";

const antdLocales = {
  ko: koKR,
  en: enUS,
} as const;

export default function RootProviders() {
  const currentTheme = useSettingsStore((state) => state.theme);
  const currentLanguage = useSettingsStore((state) => state.language);
  const isDarkMode = currentTheme === 'dark';
  const themeClassName = isDarkMode ? 'app-theme-dark' : 'app-theme-light';

  useEffect(() => {
    if (i18n.language !== currentLanguage) {
      void i18n.changeLanguage(currentLanguage);
    }
  }, [currentLanguage]);

  const locale = antdLocales[currentLanguage] ?? koKR;

  return (
    <ConfigProvider theme={buildCohesionThemeConfig(isDarkMode)} locale={locale}>
      <AntdApp className={themeClassName}>
        <AuthProvider>
          <App />
        </AuthProvider>
      </AntdApp>
    </ConfigProvider>
  );
}
