import { computed, readonly, ref } from "vue";
import { en } from "./messages/en";
import { ja } from "./messages/ja";
import { zhCN } from "./messages/zh-CN";
import { detectSystemLocale } from "./detectLocale";
import { DEFAULT_LOCALE, isLocalePreference, type Locale, type LocalePreference } from "./locale";
import type { MessageParams, MessageValue } from "./locale";

export type MessageKey = keyof typeof en;

const messages: Record<Locale, Record<MessageKey, MessageValue>> = { en, "zh-CN": zhCN, ja };
const currentLocale = ref<Locale>(DEFAULT_LOCALE);
const currentPreference = ref<LocalePreference>("system");

function format(template: string, params: MessageParams): string {
  return template.replace(/\{([A-Za-z0-9_]+)\}/g, (_, key: string) => String(params[key] ?? `{${key}}`));
}

export function t(key: MessageKey, params: MessageParams = {}): string {
  const value = messages[currentLocale.value][key];
  if (value === undefined) throw new Error(`Missing ${currentLocale.value} translation: ${key}`);
  return typeof value === "function" ? value(params) : format(value, params);
}

export function setLocalePreference(preference: LocalePreference, environment: Pick<Navigator, "languages" | "language"> = navigator): void {
  currentPreference.value = preference;
  currentLocale.value = preference === "system" ? detectSystemLocale(environment) : preference;
  document.documentElement.lang = currentLocale.value;
}

export function initializeLocale(preference: unknown): void {
  setLocalePreference(isLocalePreference(preference) ? preference : "system");
}

export function useI18n() {
  return {
    locale: readonly(currentLocale),
    preference: readonly(currentPreference),
    languageName: computed(() => ({ en: "English", "zh-CN": "简体中文", ja: "日本語" }[currentLocale.value])),
    t,
    setLocalePreference
  };
}

export { DEFAULT_LOCALE, SUPPORTED_LOCALES, type Locale, type LocalePreference } from "./locale";
export type { MessageParams, MessageValue } from "./locale";
