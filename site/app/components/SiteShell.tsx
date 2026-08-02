"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { product } from "../content";

const nav = [
  ["功能", "/features"],
  ["使用说明", "/guide"],
  ["下载", "/download"],
  ["更新日志", "/changelog"],
];

export function SiteShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const [menuOpen, setMenuOpen] = useState(false);
  const menuButtonRef = useRef<HTMLButtonElement>(null);
  const firstMenuLinkRef = useRef<HTMLAnchorElement>(null);
  const currentPath = pathname.replace(/\/$/, "") || "/";

  const isCurrent = (href: string) => currentPath === href;
  const closeMenu = (restoreFocus = true) => {
    setMenuOpen(false);
    if (restoreFocus) requestAnimationFrame(() => menuButtonRef.current?.focus());
  };

  useEffect(() => {
    if (!menuOpen) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    requestAnimationFrame(() => firstMenuLinkRef.current?.focus());
    return () => { document.body.style.overflow = previousOverflow; };
  }, [menuOpen]);

  useEffect(() => {
    const media = window.matchMedia("(min-width: 961px)");
    const closeAtDesktop = (event: MediaQueryListEvent) => {
      if (event.matches) setMenuOpen(false);
    };
    media.addEventListener("change", closeAtDesktop);
    return () => media.removeEventListener("change", closeAtDesktop);
  }, []);

  const trapMenuFocus = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Escape") {
      event.preventDefault();
      closeMenu();
      return;
    }
    if (event.key !== "Tab") return;
    const focusable = Array.from(event.currentTarget.querySelectorAll<HTMLElement>("a[href], button:not([disabled])")).filter((element) => element.tabIndex >= 0);
    if (focusable.length === 0) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  };

  return <div className="site-shell">
    <header className="site-header">
      <div className="page-width nav-wrap">
        <Link className="site-brand" href="/" aria-label="SheetProof 首页">
          <img src="/brand/icon.svg" alt="" />
          <span><strong>{product.name}</strong><small>{product.nameZh}</small></span>
        </Link>
        <nav className="desktop-nav" aria-label="主导航">
          {nav.map(([label, href]) => <Link className={isCurrent(href) ? "is-current" : undefined} aria-current={isCurrent(href) ? "page" : undefined} key={href} href={href}>{label}</Link>)}
        </nav>
        <a className="github-link" href={product.repository} target="_blank" rel="noreferrer">GitHub <span>↗</span></a>
        <button
          ref={menuButtonRef}
          className="mobile-menu-button"
          type="button"
          aria-label="打开导航菜单"
          aria-controls="mobile-site-menu"
          aria-expanded={menuOpen}
          onClick={() => setMenuOpen(true)}
        >
          <span aria-hidden="true" /><span aria-hidden="true" /><span aria-hidden="true" />
        </button>
      </div>
    </header>
    {menuOpen && <div className="mobile-menu-layer" onKeyDown={trapMenuFocus}>
      <button className="mobile-menu-backdrop" type="button" tabIndex={-1} aria-label="关闭导航菜单" onClick={() => closeMenu()} />
      <aside id="mobile-site-menu" className="mobile-menu-panel" role="dialog" aria-modal="true" aria-label="移动端导航菜单">
        <div className="mobile-menu-header">
          <span><strong>{product.name}</strong><small>导航</small></span>
          <button className="mobile-menu-close" type="button" aria-label="关闭导航菜单" onClick={() => closeMenu()}><span aria-hidden="true">×</span></button>
        </div>
        <nav className="mobile-nav" aria-label="移动端主导航">
          {nav.map(([label, href], index) => <Link ref={index === 0 ? firstMenuLinkRef : undefined} className={isCurrent(href) ? "is-current" : undefined} aria-current={isCurrent(href) ? "page" : undefined} key={href} href={href} onClick={() => closeMenu(false)}><span>{label}</span><span aria-hidden="true">→</span></Link>)}
          <a href={product.repository} target="_blank" rel="noreferrer" onClick={() => closeMenu(false)}><span>GitHub</span><span aria-hidden="true">↗</span></a>
        </nav>
      </aside>
    </div>}
    {children}
    <footer className="site-footer">
      <div className="page-width footer-grid">
        <div className="footer-brand"><img src="/brand/icon.svg" alt=""/><strong>{product.name}</strong><span>{product.taglineZh}</span></div>
        <div><strong>产品</strong><Link href="/features">功能</Link><Link href="/guide">使用说明</Link><Link href="/download">下载</Link></div>
        <div><strong>项目</strong><Link href="/changelog">更新日志</Link><a href={product.repository} target="_blank" rel="noreferrer">GitHub</a><a href={product.issues} target="_blank" rel="noreferrer">问题反馈</a></div>
        <div><strong>当前版本</strong><span>{product.version} · {product.channel}</span><span>Windows / macOS</span><span>.xlsx only</span></div>
      </div>
      <div className="page-width footer-bottom"><span>Copyright (c) 2026 tadazly</span><span>工作簿仅在本机处理</span></div>
    </footer>
  </div>;
}
