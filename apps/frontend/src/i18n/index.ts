import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import { translationResources } from "./resources";

if (!i18n.isInitialized) {
  void i18n
    .use(initReactI18next)
    .init({
      resources: translationResources,
      lng: "ko",
      fallbackLng: "ko",
      interpolation: {
        escapeValue: false,
      },
      returnNull: false,
    });
}

export default i18n;

