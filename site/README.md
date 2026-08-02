# SheetProof website

`site/` 是 SheetProof 的多页面产品官网，包含首页、功能、使用说明、下载和更新日志。
站点使用 vinext 构建为 Cloudflare Worker 兼容产物；不包含用户账户、数据库或文件上传。

产品事实不在此目录手工维护。先修改根目录的：

- `product/product.json`：名称、版本、下载、特性、截图；
- `product/changelog.json`：面向用户的版本记录。

然后从仓库根目录运行：

```bash
node scripts/sync-product-content.mjs
```

该命令同步 README、CHANGELOG 和 `site/app/content/`。本地开发与构建：

```bash
cd site
npm install
npm run dev
npm run build
```

部署到普通静态服务器或自有主机的说明见根目录 `docs/deployment.md`；通过 Sites
发布时使用 `.openai/hosting.json` 和现有 Worker 入口。
