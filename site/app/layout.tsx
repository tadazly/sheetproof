import type { Metadata } from "next";
import "./globals.css";
import { product } from "./content";

const origin = "https://sheetproof.luyilabs.com";

export const metadata: Metadata = {
  metadataBase: new URL(origin),
  title: { default: `${product.name} ${product.nameZh} — ${product.taglineZh}`, template: `%s · ${product.name}` },
  description: product.descriptionZh,
  icons: { icon: "/brand/favicon.ico", shortcut: "/brand/favicon.ico", apple: "/brand/icon-192.png" },
  openGraph: { title: `${product.name} ${product.nameZh}`, description: product.descriptionZh, url: origin, siteName: product.name, locale: "zh_CN", type: "website", images: [{ url: "/og.png", width: 1731, height: 909, alt: `${product.name} — ${product.slogan}` }] },
  twitter: { card: "summary_large_image", title: `${product.name} ${product.nameZh}`, description: product.descriptionZh, images: ["/og.png"] },
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="zh-CN"><body>{children}</body></html>;
}
