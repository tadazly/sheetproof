# 发布 SheetProof

产品事实、桌面产物和官网内容分开维护。发布前先修改事实源，再生成 README、CHANGELOG 和网站内容；不要直接修改生成文件中的版本信息。

## 1. 更新产品事实

1. 在 `product/product.json` 更新版本、发布渠道、下载地址和截图说明。
2. 在 `product/changelog.json` 记录用户实际能看到的功能与修复。不要写提示词、内部任务、测试安排、截图制作或发布计划。
3. 运行：

   ```bash
   node scripts/sync-product-content.mjs
   python scripts/generate-brand-assets.py
   ```

4. 检查 README、CHANGELOG、CLI 版本文件和官网内容是否一致。

## 2. 验证代码与网站

```bash
go test ./...
go vet ./...

cd frontend
npm ci
npm run lint
npm run typecheck
npm run test
npm run build

cd ../site
npm ci
npm run lint
npm test
```

Windows 桌面构建使用离线优先入口：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/invoke-wails.ps1 build
```

构建完成后按 `docs/manual-acceptance.md` 启动本次桌面产物并完成实机验收。README 和官网截图必须来自对应版本的真实应用窗口，不得露出桌面、用户名、路径、账号或其他私人内容。

## 3. GitHub Release 自动构建

`.github/workflows/release.yml` 提供两种运行方式：

- 手动运行 `workflow_dispatch`：执行测试并生成 Windows amd64 与 macOS universal 构建产物，仅作为 Actions artifact 保存，不创建 Release。
- 推送与产品版本一致的 `v*` 标签：执行相同验证，生成 `SheetProof-windows-amd64.exe`、`SheetProof-macos-universal.zip` 和 `SHA256SUMS.txt`，并创建或更新 GitHub Draft Release。

例如 `product/product.json` 中版本为 `0.1.0` 时，发布标签必须是 `v0.1.0`。版本不一致会直接失败。Release 默认保持草稿，完成实机验收、文件校验和文案核对后再由维护者手动发布。重复运行同一标签会更新现有草稿资产。

工作流只给 Release job `contents: write`，其他 job 使用只读权限。当前产物没有代码签名或 macOS 公证，不得把它们描述为已签名版本；后续接入签名时，凭据必须使用 GitHub Actions Secrets，不能写进工作流或仓库文件。

Release 发布后，把稳定的下载 URL 写回 `product/product.json`，再次同步内容、构建和测试官网，然后部署正式站点。

## 4. 发布官网

按照 `docs/deployment.md` 把 `site/dist/client/` 发布到 `https://sheetproof.luyilabs.com/`。每次用户可见的功能变化或修复都必须同步官网文案与更新日志；只完成本地修改或构建不算交付。

## 5. 发布后检查

- `SheetProof version`、产品事实源、README 和官网版本一致。
- GitHub Release 文件名、大小和 SHA-256 可以核对。
- 下载页没有指向不存在的文件，也没有把未签名产物写成已签名。
- README 和官网只描述当前实现，不把计划写成已支持功能。
- 更新日志只保留用户可感知的变化，没有私人信息、提示词或内部任务内容。
- 正式域名、源站和同一 Caddy 实例上的既有站点均正常。
