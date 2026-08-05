"use client";

import { useEffect } from "react";
import { localizedPath, normalizeLocale, type Locale } from "../i18n";

const STORAGE_KEY = "sheetproof:site-locale:v1";

export function rememberSiteLocale(locale: Locale) {
  window.localStorage.setItem(STORAGE_KEY, locale);
}

export function LanguageRedirect({ semanticPath }: { semanticPath: string }) {
  useEffect(() => {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    const candidates = saved ? [saved] : [...(navigator.languages ?? []), navigator.language];
    const locale = normalizeLocale(candidates.find(Boolean));
    if (locale === "en") return;
    const target = `${localizedPath(locale, semanticPath)}${window.location.hash}`;
    if (`${window.location.pathname}${window.location.hash}` !== target) window.location.replace(target);
  }, [semanticPath]);
  return null;
}
