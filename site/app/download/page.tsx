import { PageIntro } from "../components/PageIntro";
import { SiteShell } from "../components/SiteShell";
import { downloads, product } from "../content";

export default function DownloadPage() {
  return <SiteShell><main>
    <PageIntro eyebrow="DOWNLOAD" title="获取 SheetProof" description="Windows 与 macOS 桌面安装包正在准备中。开发者可以从源码构建当前版本。" aside={<><strong>{product.version}</strong><span>{product.channel}</span><small>Windows / macOS</small></>} />
    <section className="section page-width download-grid">
      <article className="download-card primary-card"><span className="platform">DESKTOP</span><h2>GitHub Releases</h2><p>Windows 与 macOS 安装包发布后可在 Releases 下载，并附带版本说明和文件校验值。</p><a className="button button-primary" href={downloads.releases} target="_blank" rel="noreferrer">前往 Releases ↗</a><small>桌面安装包正在建设中</small></article>
      <article className="download-card"><span className="platform">SOURCE</span><h2>从源码构建</h2><p>适合开发者和希望提前体验的用户。需要 Go 1.24+、Node.js 20+ 与平台桌面工具链。</p><a className="button button-secondary" href={downloads.source}>下载源码</a><code>scripts/invoke-wails.ps1 build</code></article>
    </section>
    <section className="section page-width release-process"><div><p className="eyebrow">BUILD FROM SOURCE</p><h2>本地构建</h2></div><ol><li><strong>安装依赖</strong><span>准备 Go、Node.js 与对应平台的桌面构建环境。</span></li><li><strong>构建前端</strong><span>在 frontend 目录安装依赖并生成生产资源。</span></li><li><strong>构建桌面端</strong><span>Windows 使用仓库内脚本，macOS 使用 Wails 构建命令。</span></li><li><strong>启动应用</strong><span>构建结果位于 build/bin 目录。</span></li></ol></section>
    <section className="section page-width system-note"><h2>安装包正在建设中</h2><p>首批桌面安装包将覆盖 Windows 与 macOS。发布后，本页会直接提供对应平台的下载入口。</p></section>
  </main></SiteShell>;
}
