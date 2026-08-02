import { PageIntro } from "../components/PageIntro";
import { ScreenshotViewer } from "../components/ScreenshotViewer";
import { SiteShell } from "../components/SiteShell";
import { screenshots } from "../content";

const walkthrough = [
  { id: "direct-compare", number: "01", title: "打开工作簿", body: "选择左右两个 .xlsx，或从本地 Git 仓库选择工作区文件和对比引用。左侧可编辑，右侧保持只读。" },
  { id: "review-difference", number: "02", title: "定位并核对差异", body: "按修改、增加、删除或冲突筛选。选中单元格后，两侧值、类型和行状态会保持同步。" },
  { id: "merge-applied", number: "03", title: "合并并保存", body: "把确认过的单元格或整行复制到左侧。操作可撤销；保存只写入左侧工作簿。" },
];

export default function GuidePage() {
  return <SiteShell><main>
    <PageIntro eyebrow="GUIDE" title="完成一次对比与合并" description="以角色平衡配置表为例，三个关键画面覆盖从打开到保存的主流程。" />
    <section className="section page-width guide-layout">
      <div className="guide-scenes">{walkthrough.map((step) => {
        const shot = screenshots.find((item) => item.id === step.id)!;
        return <article className="guide-scene" key={step.id}>
          <div className="guide-scene-copy"><span>{step.number}</span><div><h2>{step.title}</h2><p>{step.body}</p></div></div>
          <ScreenshotViewer src={shot.src} alt={shot.alt} caption={shot.caption} />
        </article>;
      })}</div>
      <aside className="shortcut-card"><p className="eyebrow">SHORTCUTS</p><h2>常用操作</h2><dl><div><dt>保存</dt><dd>Ctrl / Command + S</dd></div><div><dt>另存为</dt><dd>Ctrl / Command + Shift + S</dd></div><div><dt>撤销</dt><dd>Ctrl / Command + Z</dd></div><div><dt>缩放</dt><dd>Ctrl / Command + 滚轮</dd></div><div><dt>编辑左侧</dt><dd>双击单元格</dd></div></dl></aside>
    </section>
    <section className="section page-width mode-grid">
      <article><p className="eyebrow">DIRECT FILES</p><h2>双文件模式</h2><p>适合临时核对两个工作簿。左侧可编辑并可切换保存目标，右侧始终只读。</p><pre><code>sheetproof compare --left current.xlsx --right target.xlsx</code></pre></article>
      <article><p className="eyebrow">LOCAL REPOSITORY</p><h2>Git 仓库模式</h2><p>适合配置表与版本库。左侧读取真实工作区，右侧从 Git 对象导出到系统临时目录。</p><pre><code>sheetproof repo --path /repo --file config.xlsx --ref origin/main</code></pre></article>
    </section>
    <section className="section page-width faq-section"><div className="section-heading compact"><p className="eyebrow">FAQ</p><h2>常见问题</h2></div><details open><summary>工作簿会上传吗？</summary><p>不会。当前版本只在本机读取和处理文件，官网也没有工作簿上传入口。</p></details><details><summary>保存会自动提交到 Git 吗？</summary><p>不会。保存只修改当前工作区文件；暂存、提交和推送仍由你使用原有 Git 工具完成。</p></details><details><summary>能处理宏工作簿吗？</summary><p>不能。当前只支持 .xlsx，.xlsm 会在打开前被拒绝。</p></details></section>
  </main></SiteShell>;
}
