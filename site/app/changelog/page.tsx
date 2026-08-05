import { PageIntro } from "../components/PageIntro";
import { SiteShell } from "../components/SiteShell";
import { generatedReleases, localeContent } from "../content";
import type { Locale } from "../i18n";
import { pageMetadata } from "../seo";

export const metadata = pageMetadata("en", "changelog", "/changelog");

export default function ChangelogPage({ locale = "en" }: { locale?: Locale }) {
  const intro = { en: ["Changelog", "Release notes", "User-visible features and fixes in every SheetProof release."], "zh-CN": ["版本记录", "更新日志", "这里只记录用户能看到的功能变化和问题修复。"], ja: ["変更履歴", "リリースノート", "各リリースのユーザー向け機能追加と修正を記録します。"] }[locale];
  const localized = generatedReleases.locales[locale];
  return <SiteShell locale={locale} semanticPath="/changelog"><main>
    <PageIntro eyebrow={intro[0]} title={intro[1]} description={intro[2]} />
    <section className="section page-width changelog-list">
      {generatedReleases.releases.map((release) => { const text = Object.entries(localized).find(([version]) => version === release.version)?.[1]; if (!text) throw new Error(`Missing ${locale} changelog for ${release.version}`); const versionLabel = release.version === "unreleased" ? ({ en: "Unreleased", "zh-CN": "未发布", ja: "未リリース" }[locale]) : `v${release.version}`; return <article key={release.version}><div className="release-meta"><strong>{versionLabel}</strong>{release.date ? <span>{release.date}</span> : null}<em>{localeContent(locale).product.channel}</em></div><div><p className="overline">{text.title}</p><h2>{text.summary}</h2><ul>{Object.entries(text.changes).map(([id, change]) => <li key={id}>{change}</li>)}</ul></div></article>; })}
    </section>
  </main></SiteShell>;
}
