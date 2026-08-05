"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useRef, type KeyboardEvent, type ReactNode } from "react";
import { localeContent, product } from "../content";
import { languageNames, localizedPath, shellCopy, type Locale } from "../i18n";
import { LanguageRedirect, rememberSiteLocale } from "./LanguageRedirect";

export function SiteShell({ children, locale = "en", semanticPath = "/" }: { children: ReactNode; locale?: Locale; semanticPath?: string }) {
  const pathname = usePathname();
  const copy = shellCopy[locale];
  const nav = [[copy.features, "/features"], [copy.guide, "/guide"], [copy.download, "/download"], [copy.changelog, "/changelog"]];
  const menuDetailsRef = useRef<HTMLDetailsElement>(null);
  const menuButtonRef = useRef<HTMLElement>(null);
  const firstMenuLinkRef = useRef<HTMLAnchorElement>(null);
  const currentPath = pathname.replace(/\/$/, "") || "/";

  const isCurrent = (href: string) => currentPath === localizedPath(locale, href).replace(/\/$/, "") || (href === "/" && currentPath === "/");
  const closeMenu = (restoreFocus = true) => {
    menuDetailsRef.current?.removeAttribute("open");
    if (restoreFocus) requestAnimationFrame(() => menuButtonRef.current?.focus());
  };

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

  return <div className="site-shell" lang={locale}>
    {locale === "en" ? <LanguageRedirect semanticPath={semanticPath} /> : null}
    <header className="site-header">
      <div className="page-width nav-wrap">
        <Link className="site-brand" href={localizedPath(locale, "/")} aria-label={copy.home}>
          <img src="/brand/icon.svg" alt="" />
          <span><strong>{product.name}</strong><small>{locale === "zh-CN" ? "表鉴" : locale === "ja" ? "XLSX レビュー" : "XLSX review"}</small></span>
        </Link>
        <nav className="desktop-nav" aria-label={copy.nav}>
          {nav.map(([label, href]) => <Link className={isCurrent(href) ? "is-current" : undefined} aria-current={isCurrent(href) ? "page" : undefined} key={href} href={localizedPath(locale, href)}>{label}</Link>)}
        </nav>
        <label className="site-language"><span>{copy.language}</span><select value={locale} aria-label={copy.language} onChange={(event) => { const next = event.currentTarget.value as Locale; rememberSiteLocale(next); window.location.assign(`${localizedPath(next, semanticPath)}${window.location.hash}`); }}>{Object.entries(languageNames).map(([value, label]) => <option value={value} key={value}>{label}</option>)}</select></label>
        <a className="github-link" href={product.repository} target="_blank" rel="noreferrer">GitHub <span>↗</span></a>
        <details
          ref={menuDetailsRef}
          className="mobile-menu-details"
          onToggle={(event) => {
            if (event.currentTarget.open) requestAnimationFrame(() => firstMenuLinkRef.current?.focus());
          }}
        >
          <summary ref={menuButtonRef} className="mobile-menu-button" aria-label={copy.menu} aria-controls="mobile-site-menu">
            <span aria-hidden="true" /><span aria-hidden="true" /><span aria-hidden="true" />
          </summary>
          <div className="mobile-menu-layer" onKeyDown={trapMenuFocus}>
            <button className="mobile-menu-backdrop" type="button" tabIndex={-1} aria-label={copy.closeMenu} onClick={() => closeMenu()} />
            <aside id="mobile-site-menu" className="mobile-menu-panel" role="dialog" aria-modal="true" aria-label={copy.mobileMenu}>
              <div className="mobile-menu-header">
                <span><strong>{product.name}</strong><small>{copy.navigation}</small></span>
              </div>
              <nav className="mobile-nav" aria-label={copy.mobileNav}>
                {nav.map(([label, href], index) => <a ref={index === 0 ? firstMenuLinkRef : undefined} className={isCurrent(href) ? "is-current" : undefined} aria-current={isCurrent(href) ? "page" : undefined} key={href} href={localizedPath(locale, href)}><span>{label}</span><span aria-hidden="true">→</span></a>)}
                {Object.entries(languageNames).map(([value, label]) => <a href={localizedPath(value as Locale, semanticPath)} onClick={() => rememberSiteLocale(value as Locale)} key={value}><span>{label}</span><span aria-hidden="true">→</span></a>)}
                <a href={product.repository} target="_blank" rel="noreferrer" onClick={() => closeMenu(false)}><span>GitHub</span><span aria-hidden="true">↗</span></a>
              </nav>
            </aside>
          </div>
        </details>
      </div>
    </header>
    {children}
    <footer className="site-footer">
      <div className="page-width footer-grid">
        <div className="footer-brand"><img src="/brand/icon.svg" alt=""/><strong>{product.name}</strong><span>{copy.tagline}</span></div>
        <div><strong>{copy.product}</strong><Link href={localizedPath(locale, "/features")}>{copy.features}</Link><Link href={localizedPath(locale, "/guide")}>{copy.guide}</Link><Link href={localizedPath(locale, "/download")}>{copy.download}</Link></div>
        <div><strong>{copy.project}</strong><Link href={localizedPath(locale, "/changelog")}>{copy.changelog}</Link><a href={product.repository} target="_blank" rel="noreferrer">GitHub</a><a href={product.issues} target="_blank" rel="noreferrer">{copy.issues}</a></div>
        <div><strong>{copy.version}</strong><span>{product.version} · {localeContent(locale).product.channel}</span><span>Windows / macOS</span><span>.xlsx only</span></div>
      </div>
      <div className="page-width footer-bottom"><span>Copyright (c) 2026 tadazly</span><span>{copy.localOnly}</span></div>
    </footer>
  </div>;
}
