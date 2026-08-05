import type { Metadata } from "next";
import { createElement } from "react";
import generated from "./content/generated/content.json";
import { localizedPath, type Locale } from "./i18n";

export type PageId = "home" | "features" | "guide" | "download" | "changelog";
const origin = generated.facts.product.website;
const ogLocales: Record<Locale, string> = { en: "en_US", "zh-CN": "zh_CN", ja: "ja_JP" };

export function pageMetadata(locale: Locale, page: PageId, semanticPath: string): Metadata {
  const seo = generated.locales[locale].seo[page];
  const canonical = new URL(localizedPath(locale, semanticPath), origin).toString();
  const languages = {
    en: new URL(localizedPath("en", semanticPath), origin).toString(),
    "zh-CN": new URL(localizedPath("zh-CN", semanticPath), origin).toString(),
    ja: new URL(localizedPath("ja", semanticPath), origin).toString(),
    "x-default": new URL(localizedPath("en", semanticPath), origin).toString(),
  };
  return {
    title: page === "home" ? { absolute: seo.title } : seo.title,
    description: seo.description,
    alternates: { canonical, languages },
    openGraph: { title: seo.title, description: seo.description, url: canonical, siteName: "SheetProof", locale: ogLocales[locale], type: "website", images: [{ url: "/og.png", width: 1731, height: 909, alt: seo.title }] },
    twitter: { card: "summary_large_image", title: seo.title, description: seo.description, images: ["/og.png"] },
  };
}

export function SoftwareApplicationJsonLd({ locale }: { locale: Locale }) {
  const description = generated.locales[locale].product.description;
  const json = { "@context": "https://schema.org", "@type": "SoftwareApplication", name: "SheetProof", applicationCategory: "DeveloperApplication", operatingSystem: "Windows, macOS", softwareVersion: generated.facts.product.version, description, url: origin, downloadUrl: generated.facts.downloads.releases, license: "https://opensource.org/licenses/MIT" };
  return createElement("script", { type: "application/ld+json", dangerouslySetInnerHTML: { __html: JSON.stringify(json).replaceAll("<", "\\u003c") } });
}
