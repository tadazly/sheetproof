import { PageIntro } from "../components/PageIntro";
import { SiteShell } from "../components/SiteShell";
import { releases } from "../content";

export default function ChangelogPage() {
  return <SiteShell><main>
    <PageIntro eyebrow="版本记录" title="更新日志" description="这里只记录用户能看到的功能变化和问题修复。" />
    <section className="section page-width changelog-list">
      {releases.map((release) => <article key={release.version}><div className="release-meta"><strong>{release.version === "未发布" ? release.version : `v${release.version}`}</strong><span>{release.date}</span><em>{release.channel}</em></div><div><p className="overline">{release.title}</p><h2>{release.summaryZh}</h2><ul>{release.changesZh.map((change) => <li key={change}>{change}</li>)}</ul></div></article>)}
    </section>
  </main></SiteShell>;
}
