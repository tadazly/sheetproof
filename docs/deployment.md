# 官网部署

SheetProof 官网发布在 `https://sheetproof.luyilabs.com/`。生产环境使用 Cloudflare 代理、AWS Lightsail 和 Caddy；网站构建为静态文件，不需要在服务器上运行 Node.js、数据库或后台进程。

公开仓库不得保存服务器 IP、SSH 用户名、主机别名、密钥路径、本机认证配置、证书私钥或其他凭据。部署时由维护者在命令行传入本机已有的 SSH 目标。

## 构建

在仓库根目录执行：

```powershell
node scripts/sync-product-content.mjs
Set-Location site
npm ci
npm run lint
npm test
```

`npm test` 会先构建静态站点，再检查首页、功能、使用说明、下载和更新日志的输出。可部署文件位于 `site/dist/client/`，各路由使用独立的 `index.html`。

## 发布静态文件

网站构建和测试通过后，在仓库根目录执行：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/deploy-site-lightsail.ps1 -SshHost <ssh-host>
```

脚本把静态文件上传到临时目录，检查入口文件和权限后再切换到 `/var/www/sheetproof.luyilabs.com`。上一个版本保留在相邻的回退目录中。脚本不会写入 SSH 配置，也不会读取或复制密钥文件。

## Caddy

站点配置为：

```caddyfile
sheetproof.luyilabs.com {
  root * /var/www/sheetproof.luyilabs.com
  encode zstd gzip
  header {
    X-Content-Type-Options "nosniff"
    Referrer-Policy "strict-origin-when-cross-origin"
  }
  file_server
}
```

首次加入站点时必须先读取并备份现有 Caddyfile，只增加新的站点块，不改动已有虚拟主机。候选配置通过 `caddy fmt` 和 `caddy validate` 后才能替换并 reload；reload 失败时恢复备份。

Cloudflare 应使用 **Full (strict)**。橙云不会代替源站与 Cloudflare 之间的 TLS；Caddy 通常可以自动申请并续期公开证书，因此暂时不需要 Cloudflare Origin Certificate。只有自动签发确实受阻，或运维策略明确要求 Origin Certificate 时，才把证书和私钥放在服务器的受限目录中，绝不能提交到仓库。

## 验证

部署后检查：

```bash
curl -fI https://sheetproof.luyilabs.com/
curl -fI https://sheetproof.luyilabs.com/features/
curl -fI https://sheetproof.luyilabs.com/guide/
curl -fI https://sheetproof.luyilabs.com/download/
curl -fI https://sheetproof.luyilabs.com/changelog/
curl -fI https://sheetproof.luyilabs.com/brand/favicon.ico
```

还要绕过 Cloudflare 从服务器本机检查源站，确认正式域名返回当前版本，并确认同一 Caddy 实例上的既有站点仍可访问。域名、TLS、页面、截图、图标和 Open Graph 图片都通过检查后，才算部署完成。
