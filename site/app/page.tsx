import Link from "next/link";
import { SiteShell } from "./components/SiteShell";
import { GitReviewDemo } from "./components/GitReviewDemo";
import { HeroMessageCarousel } from "./components/HeroMessageCarousel";
import { product, useCases } from "./content";

const differentiators = [
  {
    number: "01",
    overline: "Git 版本",
    title: "直接比较工作区与 Git 引用",
    body: "左侧读取尚未提交的工作区文件，右侧读取已验证的 Git 引用；不 checkout、不 fetch，也不切换分支。",
  },
  {
    number: "02",
    overline: "记录对齐",
    title: "按主键识别新增与删除",
    body: "自动识别或右键指定主键列。中间新增、删除不再把后续共同记录放大成连续修改。",
  },
  {
    number: "03",
    overline: "合并保存",
    title: "只将确认内容写入左侧",
    body: "逐格或整行写入唯一可编辑的左侧，所有操作可撤销；保存前检测外部修改并执行原子替换。",
  },
];

export default function HomePage() {
  return (
    <SiteShell>
      <main>
        <section className="hero page-width">
          <div className="hero-copy">
            <p className="eyebrow">{product.name} · {product.nameZh}</p>
            <h1><span className="hero-title-line">让 Git 中的 XLSX，</span><span className="hero-title-line">也能像代码一样</span><span className="hero-title-line">审阅和选择性合并。</span></h1>
            <p className="hero-description">直接比较工作区中的 .xlsx 与本地 Git 引用。支持按主键对齐记录、逐项合并、撤销和安全保存；不切换分支，也不自动提交。</p>
            <HeroMessageCarousel />
            <div className="hero-actions">
              <Link className="button button-primary" href="/download">下载 SheetProof</Link>
              <Link className="button button-secondary" href="/guide">查看使用说明</Link>
            </div>
            <div className="truth-row" aria-label="产品边界">
              <span>文件不上传</span><span>无需 Excel</span><span>MIT 开源</span>
            </div>
          </div>
          <GitReviewDemo />
        </section>

        <section className="proof-strip">
          <div className="page-width proof-grid">
            <div><strong>工作区文件</strong><span>包含尚未提交的修改</span></div>
            <div><strong>Git 引用</strong><span>右侧只读，不切换分支</span></div>
            <div><strong>主键对齐</strong><span>识别实际记录增删</span></div>
            <div><strong>安全保存</strong><span>可撤销，不自动提交</span></div>
          </div>
        </section>

        <section className="section page-width" id="features">
          <div className="section-heading">
            <p className="eyebrow">核心能力</p>
            <h2>针对 Git 管理的 XLSX 配置表</h2>
            <p>从版本读取、记录对齐到合并保存，均围绕配置表审阅设计。</p>
          </div>
          <div className="feature-grid differentiator-grid">
            {differentiators.map((item) => (
              <article className="feature-card differentiator-card" key={item.number}>
                <div><span className="feature-number">{item.number}</span><span className="feature-overline">{item.overline}</span></div>
                <h3>{item.title}</h3>
                <p>{item.body}</p>
              </article>
            ))}
          </div>
          <div className="section-link"><Link href="/features">查看全部功能与限制 <span>→</span></Link></div>
        </section>

        <section className="section comparison-section">
          <div className="page-width">
            <div className="section-heading compact">
              <p className="eyebrow">对比方式</p>
              <h2>按记录比较，而不是只按行号</h2>
              <p>SheetProof 直接读取工作区和 Git 引用，并将确认结果写回左侧文件。</p>
            </div>
            <div className="comparison-table" role="table" aria-label="常见表格对比流程与 SheetProof 的区别">
              <div className="comparison-row comparison-head" role="row"><span>审阅环节</span><strong>坐标式或手工流程</strong><strong>SheetProof</strong></div>
              <div className="comparison-row" role="row"><span>版本来源</span><p>手动准备两个文件副本</p><p>工作区直接比较已验证的 Git 引用</p></div>
              <div className="comparison-row" role="row"><span>记录增删</span><p>按 A1、A2…比较，后续记录可能错位</p><p>可靠记录按主键对齐，歧义记录回退行号</p></div>
              <div className="comparison-row" role="row"><span>合并结果</span><p>查看后回到原文件手动整理</p><p>逐格或整行写入左侧，并进入撤销历史</p></div>
              <div className="comparison-row" role="row"><span>Git 操作</span><p>另外维护当前分支和工作区状态</p><p>不切分支、不 fetch、不自动 add 或 commit</p></div>
            </div>
          </div>
        </section>

        <section className="section process-section">
          <div className="page-width">
            <div className="section-heading light">
              <p className="eyebrow">对比与合并</p>
              <h2>四个步骤完成审阅</h2>
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
          <div className="section-heading compact"><p className="eyebrow">适用场景</p><h2>常见使用场景</h2></div>
          <div className="use-case-grid">
            {useCases.map((item) => <article key={item.title}><h3>{item.titleZh}</h3><p>{item.summaryZh}</p></article>)}
          </div>
        </section>

        <section className="section page-width trust-section">
          <div className="trust-card"><strong>本地处理</strong><p>工作簿不上传，官网也没有文件上传入口。</p></div>
          <div className="trust-card"><strong>开源可核验</strong><p>MIT License，发布资产提供 SHA-256 校验文件。</p></div>
          <div className="trust-card"><strong>保存有边界</strong><p>左侧唯一可写；检测外部修改后再原子替换。</p></div>
        </section>

        <section className="section page-width ugit-section">
          <div className="ugit-card">
            <div><p className="eyebrow">UGit 集成</p><h2>在 UGit 中调用 SheetProof</h2></div>
            <div><p>SheetProof 可注册为 UGit 的 .xlsx 差异与合并工具。工作区文件保持在可编辑左侧，版本快照只读；保存仍由你明确触发。</p><a className="text-link" href="/guide/#ugit">查看接入与使用方式 →</a></div>
          </div>
        </section>

        <section className="section page-width boundary-card">
          <div><p className="eyebrow">当前范围</p><h2>当前支持 .xlsx</h2></div>
          <p>SheetProof 负责比较、选择性合并与安全保存；表格建模、格式设计和 Git 提交仍交给你熟悉的工具。</p>
          <Link className="text-link" href="/features#limits">查看支持范围 →</Link>
        </section>

        <section className="cta-section page-width">
          <div><p className="eyebrow">版本 {product.version}</p><h2>下载 SheetProof</h2><p>Windows 与 macOS 预览版由 GitHub Releases 提供，并附带 SHA-256 校验文件。</p></div>
          <Link className="button button-primary" href="/download">前往下载</Link>
        </section>
      </main>
    </SiteShell>
  );
}
