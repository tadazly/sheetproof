import type { Metadata } from "next";
import { headers } from "next/headers";
import "./globals.css";
import { product } from "./content";

export async function generateMetadata(): Promise<Metadata> {
  const incoming = await headers();
  const host = incoming.get("host") ?? "localhost:3000";
  const protocol = host.startsWith("localhost") || host.startsWith("127.0.0.1") ? "http" : "https";
  const origin = `${protocol}://${host}`;
  return {
    title: { default: `${product.name} ${product.nameZh} — ${product.taglineZh}`, template: `%s · ${product.name}` },
    description: product.descriptionZh,
    icons: { icon: "/brand/favicon.ico", shortcut: "/brand/favicon.ico", apple: "/brand/icon-192.png" },
    openGraph: { title: `${product.name} ${product.nameZh}`, description: product.descriptionZh, url: origin, siteName: product.name, locale: "zh_CN", type: "website", images: [{ url: `${origin}/og.png`, width: 1731, height: 909, alt: `${product.name} — ${product.slogan}` }] },
    twitter: { card: "summary_large_image", title: `${product.name} ${product.nameZh}`, description: product.descriptionZh, images: [`${origin}/og.png`] },
  };
}

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="zh-CN"><body>{children}</body></html>;
}
