import { PageIntro } from "../components/PageIntro";
import { SiteShell } from "../components/SiteShell";
import { downloads, product } from "../content";
import type { Locale } from "../i18n";
import { pageCopy } from "../pageCopy";
import { pageMetadata } from "../seo";

export const metadata = pageMetadata("en", "download", "/download");

export default function DownloadPage({ locale = "en" }: { locale?: Locale }) {
  const copy = pageCopy.download[locale];
  return <SiteShell locale={locale} semanticPath="/download"><main>
    <PageIntro eyebrow={copy.intro[0]} title={copy.intro[1]} description={copy.intro[2]} aside={<><strong>{product.version}</strong><span>{locale === "en" ? "Preview" : locale === "ja" ? "プレビュー" : "预览版"}</span><small>Windows / macOS</small></>} />
    <section className="section page-width download-platform-grid">
      <article className="download-card primary-card"><div className="download-card-top"><span className="platform">WINDOWS AMD64</span><span>{copy.windows[0]}</span></div><h2>{copy.windows[1]}</h2><p>{copy.windows[2]}</p><a className="button button-primary" href={downloads.windows}>{copy.windows[3]}</a><small>SheetProof-windows-amd64.exe · v{product.version}</small></article>
      <article className="download-card primary-card"><div className="download-card-top"><span className="platform">MACOS UNIVERSAL</span><span>{copy.mac[0]}</span></div><h2>{copy.mac[1]}</h2><p>{copy.mac[2]}</p><a className="button button-primary" href={downloads.macos}>{copy.mac[3]}</a><small>SheetProof-macos-universal.zip · v{product.version}</small></article>
    </section>
    <section className="page-width download-trust-strip" aria-label={copy.trust[0]}>
      <div><strong>{copy.trust[1][0]}</strong><span>{copy.trust[1][1]}</span></div>
      <div><strong>{copy.trust[2][0]}</strong><span>{copy.trust[2][1]}</span></div>
      <div><strong>{copy.trust[3][0]}</strong><a href={downloads.checksums}>{copy.trust[3][1]}</a></div>
    </section>
    <section className="section page-width download-secondary-grid">
      <article className="download-secondary-card"><span className="platform">{copy.source[0]}</span><div><h2>{copy.source[1]}</h2><p>{copy.source[2]}</p></div><a className="text-link" href={downloads.source}>{copy.source[3]} v{product.version} →</a></article>
      <article className="download-secondary-card"><span className="platform">{copy.releases[0]}</span><div><h2>{copy.releases[1]}</h2><p>{copy.releases[2]}</p></div><a className="text-link" href={downloads.releases} target="_blank" rel="noreferrer">{copy.releases[3]}</a></article>
    </section>
    <section className="section page-width release-process"><div><p className="eyebrow">{copy.build[0]}</p><h2>{copy.build[1]}</h2></div><ol>{copy.build.slice(2).map((step) => <li key={step[0]}><strong>{step[0]}</strong><span>{step[1]}</span></li>)}</ol></section>
    <section className="section page-width system-note"><h2>{copy.note[0]}</h2><p>v{product.version} {copy.note[1]}</p></section>
  </main></SiteShell>;
}
