import { PageIntro } from "../components/PageIntro";
import { SiteShell } from "../components/SiteShell";
import { downloads, product } from "../content";

export default function DownloadPage() {
  return <SiteShell><main>
    <PageIntro eyebrow="下载" title="获取 SheetProof" description="从 GitHub Releases 下载 Windows 或 macOS 预览版，并使用随版本提供的 SHA-256 文件核对下载。" aside={<><strong>{product.version}</strong><span>{product.channel}</span><small>Windows / macOS</small></>} />
    <section className="section page-width download-grid">
      <article className="download-card primary-card"><span className="platform">WINDOWS AMD64</span><h2>Windows 预览版</h2><p>下载独立的 SheetProof 可执行文件。当前版本未进行代码签名，首次运行时 Windows 可能显示安全提示。</p><a className="button button-primary" href={downloads.windows}>下载 Windows 版</a><small>SheetProof-windows-amd64.exe</small></article>
      <article className="download-card primary-card"><span className="platform">MACOS UNIVERSAL</span><h2>macOS 预览版</h2><p>下载同时支持 Apple Silicon 与 Intel 的应用压缩包。当前版本未签名，也未经过 Apple 公证。</p><a className="button button-primary" href={downloads.macos}>下载 macOS 版</a><small>SheetProof-macos-universal.zip</small></article>
      <article className="download-card"><span className="platform">源码</span><h2>从源码构建</h2><p>需要 Go 1.24+、Node.js 20+ 和对应平台的桌面构建环境。</p><a className="button button-secondary" href={downloads.source}>下载源码</a><code>scripts/invoke-wails.ps1 build</code></article>
      <article className="download-card"><span className="platform">校验与版本说明</span><h2>GitHub Releases</h2><p>查看面向用户的更新内容，并下载 SHA256SUMS.txt 核对 Windows 与 macOS 产物。</p><a className="button button-secondary" href={downloads.releases} target="_blank" rel="noreferrer">查看 Releases ↗</a><small>发布渠道：{product.channel}</small></article>
    </section>
    <section className="section page-width release-process"><div><p className="eyebrow">从源码构建</p><h2>本地构建</h2></div><ol><li><strong>安装依赖</strong><span>准备 Go、Node.js 和对应平台的桌面构建环境。</span></li><li><strong>构建前端</strong><span>在 frontend 目录安装依赖并生成生产资源。</span></li><li><strong>构建桌面端</strong><span>Windows 使用仓库内脚本，macOS 使用 Wails 构建命令。</span></li><li><strong>启动应用</strong><span>构建结果位于 build/bin 目录。</span></li></ol></section>
    <section className="section page-width system-note"><h2>下载前说明</h2><p>v{product.version} 是预览版本。桌面产物当前未进行代码签名，macOS 版本也未公证；请只从项目 GitHub Releases 下载并核对 SHA-256。</p></section>
  </main></SiteShell>;
}
