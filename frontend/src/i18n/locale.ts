export type Locale = "en" | "zh-CN" | "ja";
export type LocalePreference = "system" | Locale;
export type MessageParams = Record<string, string | number | boolean | undefined>;
export type MessageValue = string | ((params: MessageParams) => string);

export const DEFAULT_LOCALE: Locale = "en";
export const SUPPORTED_LOCALES = ["en", "zh-CN", "ja"] as const;
export const LOCALE_PREFERENCES = ["system", ...SUPPORTED_LOCALES] as const;

const TRADITIONAL_CHINESE = /^(zh-(tw|hk|mo|hant))(?:-|$)/i;

export function normalizeLocale(value: unknown): Locale {
  const candidate = String(value ?? "").trim().replace(/_/g, "-");
  if (!candidate || TRADITIONAL_CHINESE.test(candidate)) return DEFAULT_LOCALE;
  if (/^en(?:-|$)/i.test(candidate)) return "en";
  if (/^ja(?:-|$)/i.test(candidate)) return "ja";
  if (/^zh(?:-|$)/i.test(candidate)) {
    const lower = candidate.toLowerCase();
    if (lower === "zh" || /^zh-(cn|hans|sg)(?:-|$)/.test(lower)) return "zh-CN";
  }
  return DEFAULT_LOCALE;
}

export function isLocalePreference(value: unknown): value is LocalePreference {
  return LOCALE_PREFERENCES.includes(value as LocalePreference);
}

export function localeName(locale: Locale): string {
  return { en: "English", "zh-CN": "简体中文", ja: "日本語" }[locale];
}
