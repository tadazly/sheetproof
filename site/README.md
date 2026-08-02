# SheetProof website

`site/` 是 SheetProof 的多页面官网，包含首页、功能、使用说明、下载和更新日志。站点不包含用户账户、数据库或文件上传；生产构建输出静态文件，由 Caddy 直接提供。

产品事实不要在本目录重复维护：

- `product/product.json`：名称、版本、下载、特性和截图；
- `product/changelog.json`：面向用户的版本记录。

从仓库根目录运行：

```bash
node scripts/sync-product-content.mjs
```

本地开发和验证：

```bash
cd site
npm ci
npm run dev
npm run lint
npm test
```

`npm test` 会构建并检查 `dist/client/` 中的静态页面。正式站点为 `https://sheetproof.luyilabs.com/`，部署方法见根目录的 `docs/deployment.md`。

`.openai/hosting.json` 和 Worker 入口保留用于历史构建兼容，不是当前正式发布目标；除非维护者明确切换，官网不得再发布到旧 Sites 地址。
