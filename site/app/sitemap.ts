import type { MetadataRoute } from "next";
import { localizedPath, type Locale } from "./i18n";

const origin = "https://sheetproof.luyilabs.com";
const pages = ["/", "/features", "/guide", "/download", "/changelog"];
const locales: Locale[] = ["en", "zh-CN", "ja"];

export default function sitemap(): MetadataRoute.Sitemap {
  return pages.flatMap((path) => locales.map((locale) => ({
    url: new URL(localizedPath(locale, path), origin).toString(),
    changeFrequency: path === "/changelog" || path === "/download" ? "weekly" as const : "monthly" as const,
    priority: path === "/" ? 1 : .8,
    alternates: { languages: { en: new URL(localizedPath("en", path), origin).toString(), "zh-CN": new URL(localizedPath("zh-CN", path), origin).toString(), ja: new URL(localizedPath("ja", path), origin).toString(), "x-default": new URL(localizedPath("en", path), origin).toString() } },
  })));
}
