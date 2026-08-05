import { PageIntro } from "../components/PageIntro";
import { ScreenshotViewer } from "../components/ScreenshotViewer";
import { SiteShell } from "../components/SiteShell";
import { localizedScreenshots } from "../content";
import type { Locale } from "../i18n";
import { pageCopy } from "../pageCopy";
import { pageMetadata } from "../seo";

export const metadata = pageMetadata("en", "guide", "/guide");
const stepShots = ["keyAlignment", "focusedDifference", "mergeResult"];
const commands = ["sheetproof compare --left current.xlsx --right target.xlsx", "sheetproof repo --path /repo --file config.xlsx --ref origin/main"];

export default function GuidePage({ locale = "en" }: { locale?: Locale }) {
  const copy = pageCopy.guide[locale];
  const screenshots = localizedScreenshots(locale);
  return <SiteShell locale={locale} semanticPath="/guide"><main>
    <PageIntro eyebrow={copy.intro[0]} title={copy.intro[1]} description={copy.intro[2]} />
    <section className="section page-width guide-layout">
      <div className="guide-scenes">{copy.steps.map((step, index) => {
        const shot = screenshots.find((item) => item.id === stepShots[index]);
        if (!shot) throw new Error(`Missing screenshot metadata: ${stepShots[index]}`);
        return <article className="guide-scene" key={step[0]}>
          <div className="guide-scene-copy"><span>{String(index + 1).padStart(2, "0")}</span><div><h2>{step[0]}</h2><p>{step[1]}</p></div></div>
          <ScreenshotViewer locale={locale} src={shot.src} alt={shot.alt} caption={shot.caption} width={shot.width} height={shot.height} />
        </article>;
      })}</div>
      <aside className="shortcut-card"><p className="eyebrow">{copy.shortcuts[0]}</p><h2>{copy.shortcuts[1]}</h2><dl>{copy.shortcuts.slice(2).map((item) => <div key={item[0]}><dt>{item[0]}</dt><dd>{item[1]}</dd></div>)}</dl></aside>
    </section>
    <section className="section page-width mode-grid">{copy.modes.map((mode, index) => <article key={mode[0]}><p className="eyebrow">{mode[0]}</p><h2>{mode[1]}</h2><p>{mode[2]}</p><pre><code>{commands[index]}</code></pre></article>)}</section>
    <section className="section page-width ugit-guide" id="ugit">
      <div className="section-heading compact"><p className="eyebrow">{copy.ugit[0]}</p><h2>{copy.ugit[1]}</h2><p>{copy.ugit[2]}</p></div>
      <ol className="ugit-setup-steps">{copy.ugit.slice(3).map((step, index) => <li key={step[0]}><span>{String(index + 1).padStart(2, "0")}</span><div><h3>{step[0]}</h3><p>{step[1]}</p></div></li>)}</ol>
      <div className="ugit-config-note"><strong>{copy.ugitNote[0]}</strong><p>{copy.ugitNote[1]}</p></div>
    </section>
    <section className="section page-width faq-section"><div className="section-heading compact"><p className="eyebrow">FAQ</p><h2>{locale === "en" ? "Before you start" : locale === "ja" ? "よくある質問" : "使用前常见问题"}</h2></div>{copy.faq.map((item, index) => <details open={index === 0} key={item[0]}><summary>{item[0]}</summary><p>{item[1]}</p></details>)}</section>
  </main></SiteShell>;
}
