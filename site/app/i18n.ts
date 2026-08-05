export type Locale = "en" | "zh-CN" | "ja";

export const DEFAULT_LOCALE: Locale = "en";
export const SUPPORTED_LOCALES = ["en", "zh-CN", "ja"] as const;
export const languageNames: Record<Locale, string> = { en: "English", "zh-CN": "简体中文", ja: "日本語" };

export function normalizeLocale(input: unknown): Locale {
  if (typeof input !== "string") return DEFAULT_LOCALE;
  const value = input.trim().replaceAll("_", "-").toLowerCase();
  if (value === "en" || value.startsWith("en-")) return "en";
  if (["zh", "zh-cn", "zh-hans", "zh-sg"].includes(value)) return "zh-CN";
  if (value === "ja" || value.startsWith("ja-")) return "ja";
  return DEFAULT_LOCALE;
}

export function localePrefix(locale: Locale): string {
  return locale === "en" ? "" : `/${locale}`;
}

export function localizedPath(locale: Locale, semanticPath: string): string {
  const path = semanticPath === "/" ? "" : semanticPath.replace(/\/$/, "");
  return `${localePrefix(locale)}${path}/` || "/";
}

export const shellCopy = {
  en: { home: "SheetProof home", nav: "Main navigation", menu: "Navigation menu", closeMenu: "Close navigation menu", mobileMenu: "Mobile navigation menu", mobileNav: "Mobile main navigation", navigation: "Navigation", product: "Product", project: "Project", version: "Current version", features: "Features", guide: "Guide", download: "Download", changelog: "Changelog", issues: "Report an issue", localOnly: "Workbooks are processed locally", tagline: "Review and selectively merge XLSX changes in Git", language: "Language" },
  "zh-CN": { home: "SheetProof 首页", nav: "主导航", menu: "导航菜单", closeMenu: "关闭导航菜单", mobileMenu: "移动端导航菜单", mobileNav: "移动端主导航", navigation: "导航", product: "产品", project: "项目", version: "当前版本", features: "功能", guide: "使用说明", download: "下载", changelog: "更新日志", issues: "问题反馈", localOnly: "工作簿仅在本机处理", tagline: "审阅 Git 中的 XLSX 变更，只应用确认内容", language: "语言" },
  ja: { home: "SheetProof ホーム", nav: "メインナビゲーション", menu: "ナビゲーションメニュー", closeMenu: "ナビゲーションメニューを閉じる", mobileMenu: "モバイルナビゲーションメニュー", mobileNav: "モバイルメインナビゲーション", navigation: "ナビゲーション", product: "製品", project: "プロジェクト", version: "現在のバージョン", features: "機能", guide: "使い方", download: "ダウンロード", changelog: "変更履歴", issues: "問題を報告", localOnly: "ブックはローカルで処理されます", tagline: "Git 上の XLSX 変更を確認し、必要な項目だけを反映", language: "言語" },
} as const;
