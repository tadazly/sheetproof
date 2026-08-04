import { PageIntro } from "../components/PageIntro";
import { SiteShell } from "../components/SiteShell";
import { downloads, product } from "../content";

export default function DownloadPage() {
  return <SiteShell><main>
    <PageIntro eyebrow="下载" title="下载 SheetProof" description="Windows amd64 与 macOS universal 预览版由 GitHub Releases 提供。下载后请核对 SHA-256。" aside={<><strong>{product.version}</strong><span>{product.channel}</span><small>Windows / macOS</small></>} />
    <section className="section page-width download-platform-grid">
      <article className="download-card primary-card"><div className="download-card-top"><span className="platform">WINDOWS AMD64</span><span>独立 EXE</span></div><h2>Windows 预览版</h2><p>下载后直接运行 SheetProof 可执行文件。当前版本未进行代码签名，首次运行时 Windows 可能显示安全提示。</p><a className="button button-primary" href={downloads.windows}>下载 Windows 版</a><small>SheetProof-windows-amd64.exe · v{product.version}</small></article>
      <article className="download-card primary-card"><div className="download-card-top"><span className="platform">MACOS UNIVERSAL</span><span>Apple Silicon + Intel</span></div><h2>macOS 预览版</h2><p>下载 universal 应用压缩包。当前版本未签名，也未经过 Apple 公证，首次打开需要由用户明确允许。</p><a className="button button-primary" href={downloads.macos}>下载 macOS 版</a><small>SheetProof-macos-universal.zip · v{product.version}</small></article>
    </section>
    <section className="page-width download-trust-strip" aria-label="下载与隐私保证">
      <div><strong>文件留在本机</strong><span>应用与官网都不会上传工作簿</span></div>
      <div><strong>MIT 开源</strong><span>源码和构建流程可公开核验</span></div>
      <div><strong>SHA-256 校验</strong><a href={downloads.checksums}>直接下载校验文件 →</a></div>
    </section>
    <section className="section page-width download-secondary-grid">
      <article className="download-secondary-card"><span className="platform">源码</span><div><h2>从源码构建</h2><p>需要 Go 1.24+、Node.js 20+ 和对应平台的桌面构建环境。</p></div><a className="text-link" href={downloads.source}>下载 v{product.version} 源码 →</a></article>
      <article className="download-secondary-card"><span className="platform">版本与资产</span><div><h2>GitHub Releases</h2><p>查看更新内容、平台产物和完整校验文件。</p></div><a className="text-link" href={downloads.releases} target="_blank" rel="noreferrer">查看 Releases ↗</a></article>
    </section>
    <section className="section page-width release-process"><div><p className="eyebrow">从源码构建</p><h2>本地构建</h2></div><ol><li><strong>安装依赖</strong><span>准备 Go、Node.js 和对应平台的桌面构建环境。</span></li><li><strong>构建前端</strong><span>在 frontend 目录安装依赖并生成生产资源。</span></li><li><strong>构建桌面端</strong><span>Windows 使用仓库内脚本，macOS 使用 Wails 构建命令。</span></li><li><strong>启动应用</strong><span>构建结果位于 build/bin 目录。</span></li></ol></section>
    <section className="section page-width system-note"><h2>下载前说明</h2><p>v{product.version} 是预览版本。桌面产物当前未进行代码签名，macOS 版本也未公证；请只从项目 GitHub Releases 下载并核对 SHA-256。</p></section>
  </main></SiteShell>;
}
