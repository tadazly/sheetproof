# 发布 SheetProof

本项目把产品事实、桌面产物和网站内容分开维护。发布前先修改事实源，再生成派生文件，
不要直接在 README、CHANGELOG 或 `site/app/content/` 中重复改版本信息。

## 1. 更新产品事实

1. 在 `product/product.json` 更新版本、发布渠道、下载地址和截图说明。
2. 在 `product/changelog.json` 追加本次版本记录。
3. 运行：

   ```bash
   node scripts/sync-product-content.mjs
   python scripts/generate-brand-assets.py
   ```

4. 检查生成的 README、CHANGELOG、CLI 版本文件与网站内容是否一致。

## 2. 验证代码与网站

```bash
go test ./...
go vet ./...

cd frontend
npm run lint
npm run typecheck
npm run test
npm run build

cd ../site
npm install
npm run build
npm test
```

Windows 桌面构建使用离线优先入口：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/invoke-wails.ps1 build
```

构建完成后按 `docs/manual-acceptance.md` 启动本次产物。README 和官网使用
`cmd/genproductdemo` 的游戏配置表示例，并从当次桌面流程直接捕获应用客户区。
整窗截图不得露出桌面或其他窗口；指南可以补充直接捕获的关键区域截图。

## 3. 制作发行包

- Windows：发布 `build/bin/SheetProof.exe`，正式公开前建议完成代码签名。
- macOS：发布构建得到的 `SheetProof.app` 压缩包，正式公开前建议完成签名与公证。
- 计算每个文件的 SHA-256，并把摘要写入 GitHub Release 说明。
- 将安装包上传到 GitHub Releases 后，把最终 URL 写回 `product/product.json`，再次运行
  内容同步和网站构建。下载地址为空时，网站必须继续显示“尚未发布签名安装包”。

## 4. 发布网站

先部署到私有预览环境，逐页检查首页、功能、使用说明、下载、更新日志、图标、两张
真实截图和移动端布局。确认后再按 `docs/deployment.md` 发布到正式域名。

## 5. 发布后检查

- `SheetProof version` 与页面版本一致。
- GitHub Release 的文件名、大小和 SHA-256 可核对。
- 下载页没有指向不存在的平台包。
- README 与官网的能力描述仍来自当前实现，不把 roadmap 写成已支持。
- 新版本截图来自该版本桌面产物，边缘没有下层桌面或其他应用内容。
