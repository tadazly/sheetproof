import { PageIntro } from "../components/PageIntro";
import { SiteShell } from "../components/SiteShell";
import { downloads, product } from "../content";

export default function DownloadPage() {
  return <SiteShell><main>
    <PageIntro eyebrow="下载" title="获取 SheetProof" description="当前尚未发布 Windows 和 macOS 可执行文件。开发者可以从源码构建预览版。" aside={<><strong>{product.version}</strong><span>{product.channel}</span><small>Windows / macOS</small></>} />
    <section className="section page-width download-grid">
      <article className="download-card primary-card"><span className="platform">桌面端</span><h2>GitHub Releases</h2><p>发布后的 Windows 可执行文件和 macOS 应用压缩包会放在这里，并附带 SHA-256 校验文件。</p><a className="button button-primary" href={downloads.releases} target="_blank" rel="noreferrer">查看 Releases ↗</a><small>当前尚无公开可执行文件</small></article>
      <article className="download-card"><span className="platform">源码</span><h2>从源码构建</h2><p>需要 Go 1.24+、Node.js 20+ 和对应平台的桌面构建环境。</p><a className="button button-secondary" href={downloads.source}>下载源码</a><code>scripts/invoke-wails.ps1 build</code></article>
    </section>
    <section className="section page-width release-process"><div><p className="eyebrow">从源码构建</p><h2>本地构建</h2></div><ol><li><strong>安装依赖</strong><span>准备 Go、Node.js 和对应平台的桌面构建环境。</span></li><li><strong>构建前端</strong><span>在 frontend 目录安装依赖并生成生产资源。</span></li><li><strong>构建桌面端</strong><span>Windows 使用仓库内脚本，macOS 使用 Wails 构建命令。</span></li><li><strong>启动应用</strong><span>构建结果位于 build/bin 目录。</span></li></ol></section>
    <section className="section page-width system-note"><h2>公开发布前说明</h2><p>当前预览版可从源码构建。Windows 和 macOS 可执行文件完成检查后，会通过 GitHub Releases 提供下载。</p></section>
  </main></SiteShell>;
}
