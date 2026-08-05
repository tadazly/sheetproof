import Link from "next/link";
import { SiteShell } from "./components/SiteShell";
import { GitReviewDemo } from "./components/GitReviewDemo";
import { HeroMessageCarousel } from "./components/HeroMessageCarousel";
import { product } from "./content";
import type { Locale } from "./i18n";
import { pageMetadata, SoftwareApplicationJsonLd } from "./seo";
import { homeCopy } from "./homeCopy";

export const metadata = pageMetadata("en", "home", "/");

export default function HomePage({ locale = "en" }: { locale?: Locale }) {
  const copy = homeCopy[locale];
  return (
    <SiteShell locale={locale} semanticPath="/">
      <SoftwareApplicationJsonLd locale={locale} />
      <main>
        <section className="hero page-width">
          <div className="hero-copy">
            <p className="eyebrow">{copy.brand}</p>
            <h1 aria-label={copy.hero.join(locale === "en" ? " " : "")}>{copy.hero.map((line) => <span className="hero-title-line" aria-hidden="true" key={line}>{line}</span>)}</h1>
            <p className="hero-description">{copy.description}</p>
            <HeroMessageCarousel locale={locale} />
            <div className="hero-actions">
              <Link className="button button-primary" href={locale === "en" ? "/download/" : `/${locale}/download/`}>{copy.download}</Link>
              <Link className="button button-secondary" href={locale === "en" ? "/guide/" : `/${locale}/guide/`}>{copy.guide}</Link>
            </div>
            <div className="truth-row" aria-label={copy.truthLabel}>
              {copy.truth.map((item) => <span key={item}>{item}</span>)}
            </div>
          </div>
          <GitReviewDemo locale={locale} />
        </section>

        <section className="proof-strip">
          <div className="page-width proof-grid">
            {copy.proof.map(([title, body]) => <div key={title}><strong>{title}</strong><span>{body}</span></div>)}
          </div>
        </section>

        <section className="section page-width" id="features">
          <div className="section-heading">
            <p className="eyebrow">{copy.core[0]}</p><h2>{copy.core[1]}</h2><p>{copy.core[2]}</p>
          </div>
          <div className="feature-grid differentiator-grid">
            {copy.differentiators.map((item, index) => (
              <article className="feature-card differentiator-card" key={item[1]}>
                <div><span className="feature-number">{String(index + 1).padStart(2, "0")}</span><span className="feature-overline">{item[0]}</span></div>
                <h3>{item[1]}</h3><p>{item[2]}</p>
              </article>
            ))}
          </div>
          <div className="section-link"><Link href={locale === "en" ? "/features/" : `/${locale}/features/`}>{copy.more} <span>→</span></Link></div>
        </section>

        <section className="section comparison-section">
          <div className="page-width">
            <div className="section-heading compact">
              <p className="eyebrow">{copy.compareHeading[0]}</p><h2>{copy.compareHeading[1]}</h2><p>{copy.compareHeading[2]}</p>
            </div>
            <div className="comparison-table" role="table" aria-label={copy.compareLabel}>
              {copy.compare.map((row, index) => <div className={`comparison-row${index === 0 ? " comparison-head" : ""}`} role="row" key={row[0]}><span>{row[0]}</span>{index === 0 ? <><strong>{row[1]}</strong><strong>{row[2]}</strong></> : <><p>{row[1]}</p><p>{row[2]}</p></>}</div>)}
            </div>
          </div>
        </section>

        <section className="section process-section">
          <div className="page-width">
            <div className="section-heading light">
              <p className="eyebrow">{copy.processHeading[0]}</p><h2>{copy.processHeading[1]}</h2>
            </div>
            <div className="process-grid">
              {copy.process.map(([title, body], index) => <article key={title}><span>{String(index + 1).padStart(2, "0")}</span><h3>{title}</h3><p>{body}</p></article>)}
            </div>
          </div>
        </section>

        <section className="section page-width">
          <div className="section-heading compact"><p className="eyebrow">{copy.casesHeading[0]}</p><h2>{copy.casesHeading[1]}</h2></div>
          <div className="use-case-grid">
            {copy.cases.map(([title, body]) => <article key={title}><h3>{title}</h3><p>{body}</p></article>)}
          </div>
        </section>

        <section className="section page-width trust-section">
          {copy.trust.map(([title, body]) => <div className="trust-card" key={title}><strong>{title}</strong><p>{body}</p></div>)}
        </section>

        <section className="section page-width ugit-section">
          <div className="ugit-card">
            <div><p className="eyebrow">{copy.ugit[0]}</p><h2>{copy.ugit[1]}</h2></div>
            <div><p>{copy.ugit[2]}</p><a className="text-link" href={`${locale === "en" ? "/guide/" : `/${locale}/guide/`}#ugit`}>{copy.ugit[3]} →</a></div>
          </div>
        </section>

        <section className="section page-width boundary-card">
          <div><p className="eyebrow">{copy.boundary[0]}</p><h2>{copy.boundary[1]}</h2></div><p>{copy.boundary[2]}</p>
          <Link className="text-link" href={`${locale === "en" ? "/features/" : `/${locale}/features/`}#limits`}>{copy.boundary[3]} →</Link>
        </section>

        <section className="cta-section page-width">
          <div><p className="eyebrow">{copy.cta[0]} {product.version}</p><h2>{copy.cta[1]}</h2><p>{copy.cta[2]}</p></div>
          <Link className="button button-primary" href={locale === "en" ? "/download/" : `/${locale}/download/`}>{copy.cta[3]}</Link>
        </section>
      </main>
    </SiteShell>
  );
}
