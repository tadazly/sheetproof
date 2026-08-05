import { DEFAULT_LOCALE, normalizeLocale, type Locale, type LocalePreference } from "./locale";

export interface LocaleEnvironment {
  languages?: readonly string[];
  language?: string;
}

export function detectSystemLocale(environment: LocaleEnvironment = navigator): Locale {
  for (const candidate of environment.languages ?? []) {
    const normalized = normalizeLocale(candidate);
    if (normalized !== DEFAULT_LOCALE || /^en(?:-|$)/i.test(candidate)) return normalized;
  }
  return normalizeLocale(environment.language);
}

export function resolveLocale(preference: LocalePreference, environment?: LocaleEnvironment): Locale {
  return preference === "system" ? detectSystemLocale(environment) : preference;
}
