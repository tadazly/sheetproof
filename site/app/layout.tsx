import type { Metadata } from "next";
import "./globals.css";
import generated from "./content/generated/content.json";

const origin = "https://sheetproof.luyilabs.com";

export const metadata: Metadata = {
  metadataBase: new URL(origin),
  title: { default: generated.locales.en.seo.home.title, template: `%s · SheetProof` },
  description: generated.locales.en.seo.home.description,
  icons: { icon: "/brand/favicon.ico", shortcut: "/brand/favicon.ico", apple: "/brand/icon-192.png" },
  openGraph: { title: generated.locales.en.seo.home.title, description: generated.locales.en.seo.home.description, url: origin, siteName: "SheetProof", locale: "en_US", type: "website", images: [{ url: "/og.png", width: 1731, height: 909, alt: generated.locales.en.seo.home.title }] },
  twitter: { card: "summary_large_image", title: generated.locales.en.seo.home.title, description: generated.locales.en.seo.home.description, images: ["/og.png"] },
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body>{children}</body></html>;
}
