import { PageIntro } from "../components/PageIntro";
import { ScreenshotViewer } from "../components/ScreenshotViewer";
import { SiteShell } from "../components/SiteShell";
import { features, screenshots } from "../content";

export default function FeaturesPage() {
  return <SiteShell><main>
    <PageIntro eyebrow="CAPABILITIES" title="为 XLSX 变化审阅而设计" description="从单元格内容到 Git 工作区，变化都以可定位、可选择的方式呈现。" aside={<><strong>比较范围</strong><code>值 · 公式 · 类型 · 工作表</code><span>样式不作为内容差异</span></>} />
    <section className="section page-width detail-list">
      {features.map((feature, index) => <article key={feature.title}>
        <span>0{index + 1}</span><div><p className="overline">{feature.title}</p><h2>{feature.titleZh}</h2><p>{feature.summaryZh}</p></div>
      </article>)}
    </section>
    <section className="section page-width screenshot-section">
      <div className="section-heading compact"><p className="eyebrow">DESKTOP WORKFLOW</p><h2>从差异定位到合并保存</h2><p>以赛季角色数值配置为例，查看修改、新增与删除如何在双栏中呈现。</p></div>
      <div className="screenshot-stack">{screenshots.filter((shot) => shot.id !== "review-difference").map((shot) => <ScreenshotViewer key={shot.id} src={shot.src} alt={shot.alt} caption={shot.caption} />)}</div>
    </section>
    <section id="limits" className="section page-width limits-section">
      <div className="section-heading compact"><p className="eyebrow">SUPPORTED SCOPE</p><h2>支持范围</h2><p>SheetProof 聚焦常见 .xlsx 内容与本地版本工作流。</p></div>
      <div className="limits-grid">
        <article><h3>文件格式</h3><p>只支持 .xlsx；不支持 .xls、.xlsm、.ods、CSV 和加密工作簿。</p></article>
        <article><h3>高级对象</h3><p>不编辑图表、图片、透视表、宏、外部连接或复杂条件格式，也不承诺完整保真。</p></article>
        <article><h3>Git 操作</h3><p>不 fetch、pull、push、add、commit，不创建、删除或切换分支。</p></article>
        <article><h3>编辑范围</h3><p>不是 Excel 级编辑器；没有重做、格式工具栏、完整公式计算或整表复制。</p></article>
      </div>
    </section>
  </main></SiteShell>;
}
