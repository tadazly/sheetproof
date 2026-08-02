import Link from "next/link";
import { SiteShell } from "./components/SiteShell";
import { ScreenshotViewer } from "./components/ScreenshotViewer";
import { product, features, useCases } from "./content";

export default function HomePage() {
  return (
    <SiteShell>
      <main>
        <section className="hero page-width">
          <div className="hero-copy">
            <p className="eyebrow">LOCAL-FIRST XLSX REVIEW</p>
            <h1>{product.name}<span>{product.nameZh}</span></h1>
            <p className="hero-slogan">{product.slogan}</p>
            <p className="hero-description">{product.descriptionZh}</p>
            <div className="hero-actions">
              <Link className="button button-primary" href="/download">获取 SheetProof</Link>
              <Link className="button button-secondary" href="/guide">查看使用方式</Link>
            </div>
            <div className="truth-row" aria-label="产品边界">
              <span>本地处理</span><span>仅支持 .xlsx</span><span>Git / UGit 工作流</span>
            </div>
          </div>
          <ScreenshotViewer className="hero-shot" src="/screenshots/sheetproof-review-difference.png" alt="SheetProof 聚焦显示游戏角色数值差异" caption="赛季角色数值差异 · 点击查看原图" />
        </section>

        <section className="proof-strip">
          <div className="page-width proof-grid">
            <div><strong>2</strong><span>种入口：双文件与本地仓库</span></div>
            <div><strong>4</strong><span>类变化：增加、删除、修改、冲突</span></div>
            <div><strong>本地</strong><span>工作簿无需上传</span></div>
            <div><strong>可控</strong><span>只有左侧可写</span></div>
          </div>
        </section>

        <section className="section page-width" id="features">
          <div className="section-heading">
            <p className="eyebrow">WHAT IT DOES</p>
            <h2>把二进制工作簿，变成可以审阅的变化</h2>
            <p>值、公式、类型与行级变化集中在同一界面中，减少在多个工作簿之间反复查找。</p>
          </div>
          <div className="feature-grid">
            {features.map((feature, index) => (
              <article className="feature-card" key={feature.title}>
                <span className="feature-number">0{index + 1}</span>
                <h3>{feature.titleZh}</h3>
                <p>{feature.summaryZh}</p>
              </article>
            ))}
          </div>
          <div className="section-link"><Link href="/features">了解能力与边界 <span>→</span></Link></div>
        </section>

        <section className="section process-section">
          <div className="page-width">
            <div className="section-heading light">
              <p className="eyebrow">WORKFLOW</p>
              <h2>差异先可见，结果再落盘</h2>
            </div>
            <div className="process-grid">
              <article><span>01</span><h3>打开来源</h3><p>选择两个 .xlsx，或打开本地 Git 仓库并选择已有引用。</p></article>
              <article><span>02</span><h3>检查差异</h3><p>在同步双栏中按分类导航，查看值、公式、类型和行状态。</p></article>
              <article><span>03</span><h3>应用修改</h3><p>把确认过的单元格或整行复制到左侧；所有操作进入撤销历史。</p></article>
              <article><span>04</span><h3>安全保存</h3><p>检测外部修改，校验临时文件，再原子替换工作区文件。</p></article>
            </div>
          </div>
        </section>

        <section className="section page-width">
          <div className="section-heading compact"><p className="eyebrow">WHEN IT FITS</p><h2>适合需要对变化负责的表格流程</h2></div>
          <div className="use-case-grid">
            {useCases.map((item) => <article key={item.title}><h3>{item.titleZh}</h3><p>{item.summaryZh}</p></article>)}
          </div>
        </section>

        <section className="section page-width boundary-card">
          <div><p className="eyebrow">FOCUSED BY DESIGN</p><h2>专注 .xlsx 变更审阅</h2></div>
          <p>SheetProof 负责比较、选择性合并与安全保存；表格建模、格式设计和 Git 提交仍交给你熟悉的工具。</p>
          <Link className="text-link" href="/features#limits">查看支持范围 →</Link>
        </section>

        <section className="cta-section page-width">
          <div><p className="eyebrow">VERSION {product.version}</p><h2>{product.taglineZh}</h2><p>桌面安装包正在准备中，开发者也可以从源码构建。</p></div>
          <Link className="button button-primary" href="/download">获取 SheetProof</Link>
        </section>
      </main>
    </SiteShell>
  );
}
