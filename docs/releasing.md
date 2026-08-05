# 发布 SheetProof

产品事实、桌面产物和官网内容分开维护。发布前先修改事实源，再生成 README、CHANGELOG 和网站内容；不要直接修改生成文件中的版本信息。

## 1. 更新产品事实

1. 在 `product/product.json` 更新版本、发布渠道、下载地址和截图说明。
2. 在 `product/changelog/releases.json` 登记版本、日期、渠道和变更 ID，并在
   `product/changelog/{en,zh-CN,ja}.json` 写入三语用户可见功能与修复。不要写提示词、
   内部任务、测试安排、截图制作或发布计划。
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

Windows 离线入口和 GitHub Release workflow 都会设置 Go `-trimpath`。不要绕过该约束：
最终桌面产物的隐私扫描必须确认没有嵌入构建者用户目录或仓库工作区路径。

构建完成后按 `docs/manual-acceptance.md` 启动本次桌面产物并完成实机验收。README 和官网截图必须来自对应版本的真实应用窗口，不得露出桌面、用户名、路径、账号或其他私人内容。

## 3. GitHub Release 自动构建

`.github/workflows/release.yml` 提供两种运行方式：

- 手动运行 `workflow_dispatch`：执行测试并生成 Windows amd64 与 macOS universal 构建产物，仅作为 Actions artifact 保存，不创建 Release。
- 推送与产品版本一致的 `v*` 标签：执行相同验证，生成 `SheetProof-windows-amd64.exe`、`SheetProof-macos-universal.zip` 和 `SHA256SUMS.txt`，并创建或更新 GitHub Draft Release。

例如 `product/product.json` 中版本为 `0.1.0` 时，发布标签必须是 `v0.1.0`。版本不一致会直接失败。Release 默认保持草稿，完成实机验收、文件校验和文案核对后再由维护者手动发布。重复运行同一标签会更新现有草稿资产。

工作流只给 Release job `contents: write`，其他 job 使用只读权限。当前产物没有代码签名或 macOS 公证，不得把它们描述为已签名版本；后续接入签名时，凭据必须使用 GitHub Actions Secrets，不能写进工作流或仓库文件。

Release 发布后，把稳定的下载 URL 写回 `product/product.json`，再次同步内容、构建和测试官网，然后部署正式站点。

### Codex 两阶段正式发布

当维护者提出“发布”“发布正式版本”“推送正式版本”或“推送 vX.Y.Z 版本”时，Codex
先执行发布准备，不立即产生远端变更：

1. 以上一个已发布、非草稿 Release 为基线，整理到当前候选之间的用户可见功能与修复。
2. 有明确版本时校验该版本；没有明确版本时按 SemVer 推导：兼容修复升 patch，新功能
   或明显工作流扩展升 minor，不兼容的稳定公共契约或产品代际变化升 major。首次公开
   Release 默认从 `0.1.0` 开始；`1.0.0` 之前的不兼容预览变化至少升 minor。
3. 更新产品事实、更新日志、README、CHANGELOG、版本文件、发布文档与官网，并完成发布
   所需测试、打包、隐私扫描和受影响 GUI 验收。
4. 汇总版本号与理由、更新内容、变更范围、验证结果、签名状态和即将执行的远端操作，
   明确询问是否正式发布。收到确认前不得 commit、push、打标签、发布 Release 或把候选
   官网部署到生产环境。

候选汇总和发布完成报告都必须包含隐私扫描结论。若发现凭据、私人路径、账号/服务器
身份或个人文件，只报告所在位置、数据类型、暴露风险以及移除或轮换状态，不得在输出
中复述敏感值。实际凭据或私人文件处理完成前必须停止 push；如果数据已经进入公开 Git
历史，应先撤销或轮换凭据，历史重写必须另行获得明确授权。

维护者确认后，Codex 才执行完整发布：提交并推送确认范围，先手动运行一次 Release
workflow 验证两个平台；成功后推送版本标签，等待标签 workflow 创建 Draft Release；
核对产物与校验文件，把自动生成的说明替换为整理后的用户更新内容，再发布 Release。
最后同步最终下载地址，重新构建测试官网，自动部署到 Lightsail，并同时检查源站、
Cloudflare 公网入口、下载、静态资源和同一 Caddy 实例上的既有站点。任何构建或校验
失败都必须停止后续发布，不得发布失败产物。

## 4. 发布官网

按照 `docs/deployment.md` 把 `site/dist/client/` 发布到 `https://sheetproof.luyilabs.com/`。每次用户可见的功能变化或修复都必须同步官网文案与更新日志；只完成本地修改或构建不算交付。

## 5. 发布后检查

- `SheetProof version`、产品事实源、README 和官网版本一致。
- GitHub Release 文件名、大小和 SHA-256 可以核对。
- Windows/macOS 产物中没有构建者用户目录、仓库工作区路径、密钥或令牌。
- 下载页没有指向不存在的文件，也没有把未签名产物写成已签名。
- README 和官网只描述当前实现，不把计划写成已支持功能。
- 更新日志只保留用户可感知的变化，没有私人信息、提示词或内部任务内容。
- 正式域名、源站和同一 Caddy 实例上的既有站点均正常。
