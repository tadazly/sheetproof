import { PageIntro } from "../components/PageIntro";
import { ScreenshotViewer } from "../components/ScreenshotViewer";
import { SiteShell } from "../components/SiteShell";
import { screenshots } from "../content";

const walkthrough = [
  { id: "direct-compare", number: "01", title: "打开工作簿", body: "选择左右两个 .xlsx，或从本地 Git 仓库选择工作区文件和对比引用。左侧可编辑，右侧保持只读。" },
  { id: "review-difference", number: "02", title: "定位并核对差异", body: "摘要数字只对应当前工作表。按修改、增加、删除或冲突多选筛选；每次启动默认显示完整数据，切换或取消筛选会留在正在核对的原始行。选中单元格后，两侧值、类型和行状态会保持同步。" },
  { id: "merge-applied", number: "03", title: "合并并保存", body: "把确认过的单元格或整行复制到左侧。想核对改动时，按住左侧“前后对比”或在表格中按住 Tab；改过的格子会单独标出来，松手就回到最新结果。确认后再保存。" },
];

export default function GuidePage() {
  return <SiteShell><main>
    <PageIntro eyebrow="使用说明" title="完成一次对比与合并" description="下面用三个画面说明怎样打开文件、核对差异并保存结果。" />
    <section className="section page-width guide-layout">
      <div className="guide-scenes">{walkthrough.map((step) => {
        const shot = screenshots.find((item) => item.id === step.id)!;
        return <article className="guide-scene" key={step.id}>
          <div className="guide-scene-copy"><span>{step.number}</span><div><h2>{step.title}</h2><p>{step.body}</p></div></div>
          <ScreenshotViewer src={shot.src} alt={shot.alt} caption={shot.caption} width={shot.width} height={shot.height} />
        </article>;
      })}</div>
      <aside className="shortcut-card"><p className="eyebrow">快捷键</p><h2>常用操作</h2><dl><div><dt>差异行筛选</dt><dd>1–4 切换分类，5 全选</dd></div><div><dt>回看修改前</dt><dd>表格聚焦后按住 Tab</dd></div><div><dt>保存</dt><dd>Ctrl / Command + S</dd></div><div><dt>另存为</dt><dd>Ctrl / Command + Shift + S</dd></div><div><dt>撤销</dt><dd>Ctrl / Command + Z</dd></div><div><dt>缩放</dt><dd>Ctrl / Command + 滚轮</dd></div><div><dt>编辑左侧</dt><dd>双击单元格</dd></div></dl></aside>
    </section>
    <section className="section page-width mode-grid">
      <article><p className="eyebrow">直接打开文件</p><h2>双文件模式</h2><p>适合临时核对两个工作簿。左侧可编辑并可切换保存目标，右侧始终只读。</p><pre><code>sheetproof compare --left current.xlsx --right target.xlsx</code></pre></article>
      <article><p className="eyebrow">打开本地仓库</p><h2>Git 仓库模式</h2><p>适合纳入版本管理的配置表。左侧读取真实工作区，右侧读取选中的 Git 版本。</p><pre><code>sheetproof repo --path /repo --file config.xlsx --ref origin/main</code></pre></article>
    </section>
    <section className="section page-width ugit-guide" id="ugit">
      <div className="section-heading compact"><p className="eyebrow">UGit 集成</p><h2>在应用内完成配置</h2><p>不需要手工编辑 UGit 配置文件。把 SheetProof 放在固定位置后，在应用里完成一次注册即可。</p></div>
      <ol className="ugit-setup-steps">
        <li><span>01</span><div><h3>固定应用位置</h3><p>将 Windows 的 .exe 或 macOS 的 SheetProof.app 放到长期使用的目录。以后移动应用，需要重新配置。</p></div></li>
        <li><span>02</span><div><h3>点击“配置 UGit”</h3><p>打开 SheetProof，在顶部操作区点击“配置 UGit”。确认对话框会显示正在使用的 Git 配置来源和原有 XLSX 工具路径。</p></div></li>
        <li><span>03</span><div><h3>确认替换 XLSX 工具</h3><p>确认后只会更新 <code>*.xlsx</code> 的差异与合并工具，不影响 CSV 或其他文件类型。完成后重新打开 UGit。</p></div></li>
        <li><span>04</span><div><h3>在 UGit 中发起对比</h3><p>像平时一样查看 .xlsx 变更。工作区文件会放在可编辑左侧，Git 快照保持只读；保存后仍由你决定何时暂存和提交。</p></div></li>
      </ol>
      <div className="ugit-config-note"><strong>看到两条 XLSX 差异工具是正常的</strong><p>UGit 的差异工具列表会同时保留 <code>SpreadsheetCompare</code> 和 <code>Custom</code> 两个兼容入口；合并工具列表显示一条 <code>Custom</code>。这表示接入已经完成，不需要删除其中任何一条。</p></div>
    </section>
    <section className="section page-width faq-section"><div className="section-heading compact"><p className="eyebrow">常见问题</p><h2>使用前可能想知道</h2></div><details open><summary>工作簿会上传吗？</summary><p>不会。当前版本只在本机读取和处理文件，官网也没有工作簿上传入口。</p></details><details><summary>保存会自动提交到 Git 吗？</summary><p>不会。保存只修改当前工作区文件；暂存、提交和推送仍由你使用原有 Git 工具完成。</p></details><details><summary>能处理宏工作簿吗？</summary><p>不能。当前只支持 .xlsx，.xlsm 会在打开前被拒绝。</p></details></section>
  </main></SiteShell>;
}
