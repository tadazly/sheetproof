import Link from "next/link";
import type { ReactNode } from "react";
import { product } from "../content";

const nav = [
  ["功能", "/features"],
  ["使用说明", "/guide"],
  ["下载", "/download"],
  ["更新日志", "/changelog"],
];

export function SiteShell({ children }: { children: ReactNode }) {
  return <div className="site-shell">
    <header className="site-header">
      <div className="page-width nav-wrap">
        <Link className="site-brand" href="/" aria-label="SheetProof 首页">
          <img src="/brand/icon.svg" alt="" />
          <span><strong>{product.name}</strong><small>{product.nameZh}</small></span>
        </Link>
        <nav aria-label="主导航">
          {nav.map(([label, href]) => <Link key={href} href={href}>{label}</Link>)}
        </nav>
        <a className="github-link" href={product.repository} target="_blank" rel="noreferrer">GitHub <span>↗</span></a>
      </div>
    </header>
    {children}
    <footer className="site-footer">
      <div className="page-width footer-grid">
        <div className="footer-brand"><img src="/brand/icon.svg" alt=""/><strong>{product.name}</strong><span>{product.taglineZh}</span></div>
        <div><strong>产品</strong><Link href="/features">功能</Link><Link href="/guide">使用说明</Link><Link href="/download">下载</Link></div>
        <div><strong>项目</strong><Link href="/changelog">更新日志</Link><a href={product.repository} target="_blank" rel="noreferrer">GitHub</a><a href={product.issues} target="_blank" rel="noreferrer">问题反馈</a></div>
        <div><strong>当前版本</strong><span>{product.version} · {product.channel}</span><span>Windows / macOS</span><span>.xlsx only</span></div>
      </div>
      <div className="page-width footer-bottom"><span>本地优先的 XLSX 对比与合并工具</span><span>工作簿仅在本机处理</span></div>
    </footer>
  </div>;
}
