import { PageIntro } from "../components/PageIntro";
import { SiteShell } from "../components/SiteShell";
import { releases } from "../content";

export default function ChangelogPage() {
  return <SiteShell><main>
    <PageIntro eyebrow="CHANGELOG" title="更新日志" description="查看每个版本新增的功能、体验改进与问题修复。" />
    <section className="section page-width changelog-list">
      {releases.map((release) => <article key={release.version}><div className="release-meta"><strong>v{release.version}</strong><span>{release.date}</span><em>{release.channel}</em></div><div><p className="overline">{release.title}</p><h2>{release.summaryZh}</h2><ul>{release.changesZh.map((change) => <li key={change}>{change}</li>)}</ul></div></article>)}
    </section>
  </main></SiteShell>;
}
