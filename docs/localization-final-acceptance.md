# SheetProof 中英日国际化最终验收

## 1. 结论

**最终结论：Fail（验收证据不完整）**。

本轮发现并修复了两处真实缺陷。第一处是仓库模式带预选 XLSX 和 Git 引用启动时，
`Controller` 会在会话打开前提前结束加载，前端随即停止轮询并永久显示空白工作区。
修复后的回归测试连续运行 20 次通过，重新构建的 Windows Wails 应用也已分别以
English、简体中文和日本語打开真实临时仓库，显示预选引用、16 处首表差异、未提交
工作区状态和只读右侧。

第二处是官网首页轮播在浏览器启用“减少动态效果”时完全停止。现已移除该偏好对定时器
的禁用；三条说明继续每 4.8 秒轮换，并保留原有 0.28 秒文字淡入/4px 位移和 0.18 秒圆点
过渡。修复后的静态构建、10 项网站测试和真实浏览器回归均通过。

自动化、Windows 构建、已执行的桌面流程、三语演示语义、90/16/15 基线、三语截图、
网站静态路由、SEO、README/CHANGELOG 和内容同步均通过。但本轮没有用真实桌面手势
完成三种语言下的“从 Explorer 拖入仓库目录”，也没有实际点击仓库文件树和 revision
下拉框完成切换；`Follow system` 重新探测以及浏览器禁用 JavaScript 仅有自动化或静态
产物证据，没有本轮独立的真实交互证据。根据任务要求，这些项目必须标记为未验证，且
不属于允许 Conditional Pass 的例外，因此不能判定 Pass 或 Conditional Pass。

## 2. 环境、日期与仓库状态

| 项目 | 结果 |
| --- | --- |
| 日期与时区 | 2026-08-05，Asia/Shanghai |
| 操作系统 | Windows amd64 |
| Go | 本机 `C:\Program Files\Go\bin\go.exe`（Wails 构建输出） |
| Node | v22.11.0 |
| Wails | v2.10.2 |
| 当前分支 | `feat/i18n-en-zh-ja` |
| HEAD | `3743ac7629881cb5b6cb8f8606d41892653b12de` |
| 远端 | `origin` → `https://github.com/tadazly/sheetproof.git`（fetch/push） |
| 初始工作区 | `git status --short` 为空；没有修改或未跟踪文件，与任务描述中“预计存在尚未提交修改”不同 |
| 初始无关修改 | 无 |
| 初始私人路径或构建产物 | 受控文件中未发现维护者私人绝对路径；`build/bin/SheetProof.exe` 为忽略产物 |

验收结束时的精确工作区统计见第 15 节。没有创建新分支，没有 reset、clean、强制 checkout、
暂存、提交、push、标签、Release 或网站部署。

## 3. 自动化门禁

| 命令 | 结果 | 数量与说明 |
| --- | --- | --- |
| `node scripts/sync-product-content.mjs` | Pass | 同步版本 0.3.0 的 en、zh-CN、ja 内容 |
| `node scripts/sync-product-content.mjs --check` | Pass | 最终复跑无内容漂移 |
| `git diff --check` | Pass | 无空白错误；仅 Git 提示工作树 LF 将来可能转换为 CRLF |
| `go test ./...` | Pass | 135/135 测试通过；14 个有测试包通过，2 个无测试包跳过 |
| `go vet ./...` | Pass | 无诊断 |
| `go test -race ./...` | 未验证（环境阻塞） | `CGO_ENABLED=0`：`-race requires cgo`；`CGO_ENABLED=1`：C 编译器 `gcc` 不在 PATH。不是代码测试失败 |
| 前端 `npm run lint` | Pass | 0 error、0 warning |
| 前端 `npm run typecheck` | Pass | 0 error |
| 前端 `npm test` | Pass | 7/7 文件，66/66 测试 |
| 前端 `npm run build` | Pass | Vite 6.4.3，28 modules transformed |
| 网站 `npm run lint` | Pass with warnings | 0 error、4 个既有 `@next/next/no-img-element` warning |
| 网站 `npm test` | Pass | 10/10 测试；测试内静态构建成功 |
| 网站 `npm run build` | Pass | 16 routes prerendered，0 skipped；产品路由为 15 个，另含框架生成路由 |

沙箱内首次运行 Go 时默认用户缓存目录被拒绝，改用仓库内临时 `GOCACHE` 后通过；临时缓存
最终已删除。Vitest、vinext/Vite 在沙箱内启动子进程时报 `spawn EPERM`，在批准的沙箱外
以相同命令通过。直接运行 Wails 脚本时 npm 不在 PATH，显式加入已安装 Node 目录后通过。
这些均为执行环境问题。演示验证期间两个忽略的临时 Go helper 同时存在曾造成重复 `main`
编译错误，删除 helper 后完整 Go 套件通过；没有把该 harness 错误记为产品失败。

针对本轮缺陷：

- 修复前：`go test . -run TestControllerRepositoryBootstrapWaitsForSelectedWorkbook -count=20`
  连续 20 次复现 `Loading=false`、`HasSession=false`。
- 修复后：同一命令连续 20 次通过。

## 4. Windows Wails 构建

`powershell -ExecutionPolicy Bypass -File scripts/invoke-wails.ps1 build` 使用已安装的 Wails
v2.10.2 完成 bindings、前端、资源和 Windows amd64 应用构建，用时 9.524 秒。

| 检查 | 结果 |
| --- | --- |
| `Test-Path .\build\bin\SheetProof.exe` | `True` |
| 文件大小 | 17,417,216 bytes |
| SHA-256 | `24F031C9DFA83212720CE7A35F776A2FC0E1EE4C7359E11592DCC34EFDE38F53` |
| `git ls-files build/bin/SheetProof.exe` | 无输出，未追踪 |
| `git check-ignore -v ...` | `.gitignore:12:/build/*` |

该 EXE 仅用于本轮真实桌面验收，不进入建议提交范围。

## 5. 三语桌面核心流程

所有“通过”的 GUI 项均使用本轮修复代码构建的真实 Wails EXE，不是 Vite 页面。截图证据保存
在忽略目录 `build/acceptance/localization-final/gui/`，公开截图另见第 8 节。

| 流程 | English | 简体中文 | 日本語 | 说明 |
| --- | --- | --- | --- | --- |
| 欢迎页与设置窗口 | Pass | Pass | Pass | 文案完整，无主按钮截断或主流程混语 |
| 直接打开两份 XLSX、切换工作表 | Pass | Pass | Pass | 使用对应 Locale 演示工作簿 |
| 上一处/下一处差异、滚动条标记 | Pass | Pass | Pass | 实际点击导航；差异位置标记可见 |
| 自动主键识别 | Pass | Pass | Pass | 首表自动识别 A 列 key，主键对齐为 16 |
| 右键列标题设置主键 | Pass | Pass | Pass | 实际将 B 列设为 key |
| 取消主键、物理行比较 | Pass | Pass | Pass | 实际恢复为 90 处首表差异 |
| 修改筛选 | Pass | Pass | Pass | 实际切换 |
| 新增/删除筛选 | 未验证 | 未验证 | 未验证 | 数量与类别可见，但未逐一实际点击筛选按钮 |
| 冲突筛选 | Pass | Pass | Pass | 使用最小真实冲突工作簿，显示 1 行/2 单元格冲突 |
| 复制单元格到左侧 | Pass | Pass | Pass | C2 复制后首表 16→15 |
| 复制整行到左侧 | Pass | Pass | Pass | 冲突记录整行复制后差异清零，Undo 可用 |
| 全选、批量清空、撤销 | Pass | Pass | Pass | 清空后显示 132 处差异，撤销恢复 16 |
| 按住 Tab 查看打开时内容 | Pass | Pass | Pass | 按下显示原值，松开恢复当前值 |
| 保存 | Pass | Pass | Pass | 实际保存临时副本，状态更新 |
| 另存为/导出副本 | Pass | Pass | Pass | 实际使用 Windows 原生保存对话框，导出文件存在 |
| 右侧外部修改与自动重载 | Pass | Pass | Pass | 显示本地化通知并自动刷新只读侧 |
| 左侧外部修改确认 | Pass | Pass | Pass | 显示本地化“稍后/重载最新”等确认文案 |
| UGit 配置入口 | Pass | Pass | Pass | 当前机器已配置为本应用，三语均显示无需更新；未写 Git 配置 |
| 打开真实本地 Git 仓库 | Pass | Pass | Pass | 修复后均加载文件树、引用和 16 处首表差异 |
| 从 Explorer 拖入仓库目录 | 未验证 | 未验证 | 未验证 | 本轮未执行真实文件夹拖放；仅有自动化的 Wails drag/drop 配置测试 |
| 在文件树中点击选择 XLSX | 未验证 | 未验证 | 未验证 | GUI 使用 `--file` 预选路径启动，未实际点击文件树 |
| 在下拉框中切换 Git revision | 未验证 | 未验证 | 未验证 | GUI 使用 `--ref acceptance-base` 预选引用，未实际点击切换 |
| 保持会话切换语言 | Pass | Pass | Pass | 实际 en→ja；工作簿、工作表、选区与未保存修改保持 |

英文简洁度、中文自然度和日文桌面软件习惯在上述已执行页面未发现阻塞问题。技术 key、分支名、
相对路径按规则保留英文。三语仓库启动修复截图分别为
`repo-fixed-en.png`、`repo-fixed-zh-CN.png`、`repo-fixed-ja.png`。

## 6. Locale 检测与回退

共享向量 `product/locale-test-vectors.json` 由 Go 和前端测试共同读取，17 项前端 locale
测试以及 Go localization 测试通过。

| 输入 | 预期 | 结果 |
| --- | --- | --- |
| `en` / `en-US` / `en-GB` | `en` | Pass（自动化） |
| `zh` / `zh-CN` / `zh-Hans` / `zh-SG` | `zh-CN` | Pass（自动化） |
| `ja` / `ja-JP` | `ja` | Pass（自动化） |
| `fr-FR` / 未知值 / 空值 | `en` | Pass（自动化） |
| `zh-TW` / `zh-HK` / `zh-Hant` | `en` | Pass（自动化） |

| 行为 | 结果 |
| --- | --- |
| 手动选择优先于环境语言 | Pass；自动化覆盖，且真实桌面语言切换生效 |
| `Follow system` 重新读取环境 | 未验证；自动化覆盖重新探测逻辑，本轮未以真实 GUI 改变环境并点击复核 |
| 偏好在重启后保留 | Pass；真实选择日本語，正常退出后不带 `--lang` 重启仍为日本語 |
| 未保存会话不因切换丢失 | Pass；真实 en→ja 保留会话、选区和未保存标记 |
| 默认与未知回退英语 | Pass（自动化） |

验收后已恢复原偏好文件，没有把本机用户配置加入仓库。

## 7. 三语演示工作簿

读取生成器帮助和源码后实际运行：

```text
go run ./cmd/genproductdemo --locale en
go run ./cmd/genproductdemo --locale zh-CN
go run ./cmd/genproductdemo --locale ja
```

每个 Locale 均生成 5 份工作簿。独立脚本使用工作簿解析库读取三语产物，确认 15 份文件的
sheet 数量、技术 key、ID 顺序、插入/删除位置、数值变化和资源路径一致。三种语言均为 3 个
sheet：英文 `Character Growth / Stage Rewards / Skill Tuning`，中文
`角色成长 / 关卡掉落 / 技能参数`，日文
`キャラクター成長 / ステージ報酬 / スキル設定`。表头、角色、地图和技能展示名均对应
Locale；未发现英文拼音/中文转写、日文中文展示名残留或中文术语漂移。

## 8. 固定差异基线与三语真实截图

当次 EXE 对三种 Locale 的结果完全一致：

| 场景 | en | zh-CN | ja | 预期 |
| --- | ---: | ---: | ---: | ---: |
| 物理行比较 | 90 | 90 | 90 | 90 |
| 主键对齐 | 16 | 16 | 16 | 16 |
| 应用一项修改后 | 15 | 15 | 15 | 15 |

公开目录每种语言均有 4 张由真实 Wails 应用重新生成的图片：

| Locale | 主键对齐 | 物理行 | 聚焦差异 | 应用修改后 |
| --- | --- | --- | --- | --- |
| en | `en/key-alignment.png` 1424×861 | `en/physical-rows.png` 1424×861 | `en/focused-difference.png` 1204×650 | `en/merge-result.png` 1204×650 |
| zh-CN | `zh-CN/key-alignment.png` 1424×861 | `zh-CN/physical-rows.png` 1424×861 | `zh-CN/focused-difference.png` 1204×650 | `zh-CN/merge-result.png` 1204×650 |
| ja | `ja/key-alignment.png` 1424×861 | `ja/physical-rows.png` 1424×861 | `ja/focused-difference.png` 1204×650 | `ja/merge-result.png` 1204×650 |

12 张原图已逐张打开检查：UI 与工作簿语言一致，表头和展示数据已本地化，技术 key 保留，
按钮未截断，列宽可读，数量和合并目标正确。旧图曾暴露真实仓库绝对路径并展示旧工具栏，
本轮已用通用 `D:\AetherFront\configs` 演示路径和当前应用重拍替换。公开图不含用户名、账号、
私人目录或桌面内容。README 引用为 English→`en`、中文→`zh-CN`、日本語→`ja`；网站三语
内容也使用对应目录，caption、alt 与画面语义一致。

根截图目录另有 7 张历史兼容图片；本轮没有删除它们。当前三语 README 与网站本地化页面
均引用 Locale 子目录中的新图。

## 9. 网站路由与交互

独立静态产物检查脚本对要求的 15/15 路由全部通过：

| Locale | 路由 |
| --- | --- |
| en | `/`、`/features/`、`/guide/`、`/download/`、`/changelog/` |
| zh-CN | `/zh-CN/` 及 `features/`、`guide/`、`download/`、`changelog/` |
| ja | `/ja/` 及 `features/`、`guide/`、`download/`、`changelog/` |

每页均检查可访问的静态 HTML、正确 `<html lang>`、本地化 title/description、canonical、
`hreflang` 的 en/zh-CN/ja/x-default（x-default 指向英语）、Open Graph locale、本地化内部导航、
对应截图和下载链接。JSON-LD 首页三语通过；内页未输出无关 schema。

真实应用内浏览器检查：

- 浏览器原偏好首次把 `/` 导向中文；在 `/zh-CN/guide/#ugit` 手动选择 English 后进入
  `/guide/#ugit`，锚点保留，随后访问 `/` 保持英语，没有再次自动跳转或循环。
- 390×844 日本語功能页的原生移动菜单与语言选择器可用，语言链接保持当前页面；
  `scrollWidth` 不超过视口，没有横向溢出。
- 日文 ScreenshotViewer 实际打开，标题、caption、缩放按钮与关闭入口已本地化。
- HeroMessageCarousel、GitReviewDemo、ScreenshotViewer 的三语内容由静态测试和浏览器画面复核。
- 发现 HeroMessageCarousel 会在 `prefers-reduced-motion: reduce` 时停止定时器并完成修复；
  在该偏好实际为 `true` 的应用内浏览器中，说明会自动切换，计算样式仍为
  `hero-message-enter`/`0.28s` 和圆点 `0.18s` 过渡，当前页面无脚本错误。
- 静态英语 HTML 包含可阅读正文和原生菜单结构；**没有在本轮真实浏览器中关闭 JavaScript**，
  因此“禁用 JavaScript 的实际浏览器会话”标为未验证。

下载页链接均指向 v0.3.0 的 Windows、macOS universal、source 和 `SHA256SUMS.txt`；Preview、
Windows 未签名、macOS 未公证文案在三语测试中保持。没有部署网站，符合本轮禁令。

## 10. SEO

15 个产品路由的以下项目全部通过静态产物解析：

- 本地化 `<title>` 和 description；
- self canonical；
- `en`、`zh-CN`、`ja`、`x-default` hreflang；
- `x-default` 指向英语对应页；
- `og:locale` 分别为 `en_US`、`zh_CN`、`ja_JP`；
- 三语首页 JSON-LD 可解析且产品事实一致；
- alt、caption、aria-label 属于当前 Locale。

## 11. README、CHANGELOG 与 GitHub 内容

检查 `README.md`、`README.zh-CN.md`、`README.ja.md` 和三份 CHANGELOG：

- 根 README 为完整英文，三份语言导航互链正确；
- 产品事实、0.3.0 版本、下载 URL、SHA-256 说明、License、未签名/未公证说明和已知限制一致；
- 三份 README 分别引用 en、zh-CN、ja 截图；
- 三份 CHANGELOG 均含 0.1.0、0.2.0、0.3.0；
- 没有旧仓库地址或英文/日文 README 中的中文公共段落；
- GitHub 仓库为公开的 `tadazly/sheetproof`，Description 为英文，Topics 为英文技术词；
- 最终 `sync-product-content.mjs --check` 通过。

本轮仓库启动修复已同步到三语产品内容源、README、CHANGELOG、网站生成内容、架构、手工验收
和迭代交接文档。

## 12. 用户可见中文与诊断边界

搜索 `internal/repository`、`internal/ugit`、`internal/cli`、`frontend/src` 后分类如下：

- 正常主流程中文位于 locale 字典和 CLI 中文消息表，属于预期内容；
- frontend 其他命中为语言名称、i18n 字典或测试 fixture；
- `internal/repository/repository.go` 与 `internal/ugit/config.go` 仍有少量缺少 Locale 上下文的
  底层 Git/路径/超时/回滚诊断。它们只在极端底层失败时通过 `err.Error()` 暴露；普通打开、
  比较、合并和保存流程未触发。

这些底层诊断按本轮边界记录为已知非阻塞限制，没有重构整个错误系统。

## 13. 隐私与提交范围

执行：

```text
rg -n "D:/github|D:\\\\github|C:/Users|C:\\\\Users|AppData|\\.ssh|wrangler-auth" .
```

命中仅包括历史文档中的泛称 `AppData`、UGit 搜索逻辑的 `localAppData` 变量，以及
`C:\Users\tester\AppData\Local` 测试 fixture。没有维护者私人绝对路径、服务器身份、密钥、
令牌、`.ssh` 或 wrangler 认证数据。公开截图已移除真实仓库路径。

`build/bin/SheetProof.exe`、GUI 证据、演示/冲突工作簿、导出副本、静态检查 helper 和 Go 缓存
均不在建议提交范围；其中 build 目录内容被忽略，两个未跟踪 Go 缓存已删除。未发现
`node_modules`、覆盖率产物、IDE 文件、凭据或 `.wrangler` 日志进入工作区状态。

## 14. 已知限制与阻塞问题

允许的已知限制：

- `go test -race ./...` 受本机 CGO/GCC 环境阻塞，未验证；
- macOS 未完成实机验收；
- 少量极端底层诊断尚未本地化；
- 网站保留 4 个既有静态 `<img>` 性能 warning。

阻止本轮通过的未验证项目：

1. 三种语言下从 Explorer 真实拖入仓库目录；
2. 三种语言下实际点击仓库文件树选择 XLSX；
3. 三种语言下实际操作 revision 下拉框切换引用；
4. 三种语言下新增、删除筛选按钮的真实点击；
5. 真实 GUI 中 `Follow system` 在环境变化后重新探测；
6. 真实浏览器禁用 JavaScript 后的英语页面阅读与导航。

前 5 项属于用户指定的桌面核心/Locale 流程，不在 Conditional Pass 允许列表中，因此即使
当前没有观察到新的代码缺陷，也必须判定 Fail。第 6 项同样按“无法执行不得推测”标为未验证。

## 15. 最终工作区与提交建议

提交前最终复核为 **41 个未提交条目，其中 39 个受控文件变更、2 个未跟踪文件**；两个未跟踪
文件分别为本报告和前端构建生成的新版哈希 JS，旧版哈希 JS 同时在受控变更中删除。所有修改均与
本轮国际化验收、发现的仓库启动/官网轮播缺陷、三语文档、前端受控构建产物或公开截图直接相关，
未发现无关修改。

当前**不建议以“最终验收通过”名义提交**，应先补齐第 14 节的真实交互证据并更新本报告结论。
用户随后明确要求提交并推送当前验收修复范围；该操作不改变本报告的 Fail 结论，也不代表最终验收通过。
提交范围包括：

- `controller.go`、`controller_test.go`；
- 三语前端短标签、对应测试和受控 `frontend/dist/` 构建产物；
- 三语 `product/` 内容源及同步生成的 README、CHANGELOG、网站内容；
- 官网首页轮播修复及其三语更新记录；
- `docs/architecture.md`、`docs/manual-acceptance.md`、`docs/iteration-2-handoff.md`、本报告；
- `site/public/screenshots/{en,zh-CN,ja}/` 的 12 张公开真实 Wails 截图。

不得提交 EXE、`build/acceptance/` 证据、临时工作簿、导出副本、缓存或本地配置。
