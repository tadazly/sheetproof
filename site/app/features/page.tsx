import { PageIntro } from "../components/PageIntro";
import { ScreenshotViewer } from "../components/ScreenshotViewer";
import { SiteShell } from "../components/SiteShell";
import { features, screenshots } from "../content";

export default function FeaturesPage() {
  return <SiteShell><main>
    <PageIntro eyebrow="功能" title="看清 XLSX 里真正变化的内容" description="从单元格到整行，从双文件到 Git 工作区，都可以在左右表格中逐项核对。" aside={<><strong>比较范围</strong><code>值 · 公式 · 类型 · 工作表</code><span>样式不计入内容差异</span></>} />
    <section className="section page-width detail-list">
      {features.map((feature, index) => <article key={feature.title}>
        <span>0{index + 1}</span><div><p className="overline">{feature.title}</p><h2>{feature.titleZh}</h2><p>{feature.summaryZh}</p></div>
      </article>)}
    </section>
    <section className="section page-width screenshot-section">
      <div className="section-heading compact"><p className="eyebrow">实际界面</p><h2>从定位差异到合并保存</h2><p>下面以角色数值配置为例，展示修改、新增和删除在左右表格中的样子。</p></div>
      <div className="screenshot-stack">{screenshots.filter((shot) => shot.id !== "review-difference").map((shot) => <ScreenshotViewer key={shot.id} src={shot.src} alt={shot.alt} caption={shot.caption} />)}</div>
    </section>
    <section id="limits" className="section page-width limits-section">
      <div className="section-heading compact"><p className="eyebrow">支持范围</p><h2>当前支持什么</h2><p>SheetProof 处理常见 .xlsx 内容和本地版本工作流，以下范围暂不支持。</p></div>
      <div className="limits-grid">
        <article><h3>文件格式</h3><p>只支持 .xlsx；不支持 .xls、.xlsm、.ods、CSV 和加密工作簿。</p></article>
        <article><h3>高级对象</h3><p>不编辑图表、图片、透视表、宏、外部连接或复杂条件格式，也不承诺完整保真。</p></article>
        <article><h3>Git 操作</h3><p>不 fetch、pull、push、add、commit，不创建、删除或切换分支。</p></article>
        <article><h3>编辑范围</h3><p>不是 Excel 级编辑器；没有重做、格式工具栏、完整公式计算或整表复制。</p></article>
      </div>
    </section>
  </main></SiteShell>;
}
