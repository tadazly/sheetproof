import { PageIntro } from "../components/PageIntro";
import { ScreenshotViewer } from "../components/ScreenshotViewer";
import { SiteShell } from "../components/SiteShell";
import { localeContent, localizedScreenshots } from "../content";
import type { Locale } from "../i18n";
import { pageCopy } from "../pageCopy";
import { pageMetadata } from "../seo";

export const metadata = pageMetadata("en", "features", "/features");
const featureKeys = ["sideBySideReview", "rowFilters", "beforeAfter", "safeSave"] as const;

export default function FeaturesPage({ locale = "en" }: { locale?: Locale }) {
  const copy = pageCopy.features[locale];
  const features = localeContent(locale).features;
  const screenshots = localizedScreenshots(locale);
  const shot = (id: string) => {
    const item = screenshots.find((candidate) => candidate.id === id);
    if (!item) throw new Error(`Missing screenshot metadata: ${id}`);
    return item;
  };
  const renderShot = (id: string) => {
    const item = shot(id);
    return <ScreenshotViewer locale={locale} src={item.src} alt={item.alt} caption={item.caption} width={item.width} height={item.height} />;
  };
  return <SiteShell locale={locale} semanticPath="/features"><main>
    <PageIntro eyebrow={copy.intro[0]} title={copy.intro[1]} description={copy.intro[2]} aside={<><strong>{copy.scope[0]}</strong><code>{copy.scope[1]}</code><span>{copy.scope[2]}</span></>} />
    <section className="section page-width capability-stories">
      {copy.stories.map((story, index) => <article key={story[0]}>
        <div className="capability-story-number"><span>{String(index + 1).padStart(2, "0")}</span><em>{story[0]}</em></div>
        <div className="capability-story-copy"><h2>{story[1]}</h2><p>{story[2]}</p></div>
        <ul>{story.slice(3).map((point) => <li key={point}>{point}</li>)}</ul>
      </article>)}
    </section>
    <section className="section supporting-section"><div className="page-width">
      <div className="section-heading compact"><p className="eyebrow">{copy.supporting[0]}</p><h2>{copy.supporting[1]}</h2><p>{copy.supporting[2]}</p></div>
      <div className="supporting-grid">{featureKeys.map((key) => <article key={key}><p className="overline">{features[key].title}</p><h3>{features[key].title}</h3><p>{features[key].summary}</p></article>)}</div>
    </div></section>
    <section className="section page-width screenshot-section">
      <div className="section-heading compact"><p className="eyebrow">{copy.compare[0]}</p><h2>{copy.compare[1]}</h2><p>{copy.compare[2]}</p></div>
      <div className="screenshot-comparisons">
        <section className="screenshot-comparison-group" aria-labelledby="alignment-comparison-title">
          <div className="screenshot-comparison-heading"><span>01</span><div><h3 id="alignment-comparison-title">{copy.alignment[0]}</h3><p>{copy.alignment[1]}</p></div></div>
          <div className="screenshot-comparison-grid"><article><strong>{copy.alignment[2]}</strong>{renderShot("physicalRows")}</article><article className="is-recommended"><strong>{copy.alignment[3]}</strong>{renderShot("keyAlignment")}</article></div>
        </section>
        <section className="screenshot-comparison-group" aria-labelledby="merge-comparison-title">
          <div className="screenshot-comparison-heading"><span>02</span><div><h3 id="merge-comparison-title">{copy.merge[0]}</h3><p>{copy.merge[1]}</p></div></div>
          <div className="screenshot-comparison-grid"><article><strong>{copy.merge[2]}</strong>{renderShot("focusedDifference")}</article><article className="is-recommended"><strong>{copy.merge[3]}</strong>{renderShot("mergeResult")}</article></div>
        </section>
      </div>
    </section>
    <section id="limits" className="section page-width limits-section">
      <div className="section-heading compact"><p className="eyebrow">{copy.limitsIntro[0]}</p><h2>{copy.limitsIntro[1]}</h2><p>{copy.limitsIntro[2]}</p></div>
      <div className="limits-grid">{copy.limits.map((item) => <article key={item[0]}><h3>{item[0]}</h3><p>{item[1]}</p></article>)}</div>
    </section>
  </main></SiteShell>;
}
