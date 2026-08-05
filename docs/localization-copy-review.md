# SheetProof 三语母语化审校清单

审计日期：2026-08-05
审计基线：`feat/i18n-en-zh-ja` @ `5645dc6` 加当前未提交第一轮 i18n 工作区
审计方式：源码逐项核对、三语演示工作簿生成、当前 Wails 产物三语实机截图、官网 15 个静态路由浏览器检查。

> 本文件记录第二轮文案审校的修改依据。稳定 JSON/CLI 字段、差异/合并/保存语义、版本号与下载资产不在修改范围。

## 审计结论摘要

- 应用前端的主要标签已覆盖三语，但原生文件选择、UGit、未保存、Git 操作和仓库后台通知仍有中文硬编码；非中文 locale 会出现混合语言，且部分安全提示无法按 locale 回译。
- 当前日语工具栏在 1600×1000 客户区仍有多处换成两行；英语品牌副标题被截断。差异索引坐标后直接显示稳定枚举 `modified`，造成三语界面混用英语。
- 官网首页与更新日志按 locale 输出；功能、使用说明、下载页正文仍固定为中文。静态产物的 `<html lang>` 正确，但三语截图资源全部缺失。
- 三语 README 的公共模板夹入一整句英文和英文 `Key alignment` 章节；日文版因此不能独立阅读。
- 演示生成器保持相同 sheet、ID、key、插入位置和数值变化；人类可读数据已分语言，但日文“ドロップ比重”、中文“职业定位”等仍需本土化，且列宽尚未按 locale 调整。
- 现有公开截图只包含中文 UI/中文数据；三语元数据已指向 `screenshots/<locale>/...`，实际文件不存在。

## 逐项清单

| locale | key / 文件位置 | 当前文本（节选） | surface | 问题类型 | 推荐修改 | 布局 | 安全/边界 |
|---|---|---|---|---|---|---|---|
| en | `app.title` | Workbook comparison and merge | app-navigation | terminology | 改为更直接的 XLSX review/merge 表述 | 品牌副标题需更短 | 否 |
| en | `toolbar.copyCellsToLeft` | Copy 1 cell to the left | app-toolbar | too-verbose | `Copy 1 cell left` / 复数模板 | 可减宽 | 否 |
| en | `toolbar.saveLeft` / `saveWorktree` | Save left / Save to current worktree | app-save | terminology | 直接文件用 `Save left`; 仓库用 `Save to worktree` | 是 | 保持左侧唯一可写 |
| en | `repository.differingWorkbooks` | Differing workbooks | app-repository | unnatural-word-order | `Changed workbooks` 或 `Workbook changes` | 否 | 后端语义筛选事实不变 |
| en | `repository.selectReference` | Select a comparison reference | app-repository | terminology | 用户操作用 `Choose a Git revision to compare`，技术帮助再说明 reference | 可能 | 不暗示 fetch |
| en | `diff.currentSheetEqual` | The current worksheet is identical | app-diff | wrong-register | `No differences in this sheet` | 否 | 否 |
| en | 差异索引状态 | `2:C · modified` | app-diff | cultural-mismatch | 将稳定枚举只用于数据，显示层改为 `Modified` | 否 | 不改稳定字段 |
| en | `alignment.key` | Key alignment{count} | app-diff | too-vague | 数量模板显示 `Key: {column}` 或 `Key-aligned · {count}`（依现有数据） | 可能 | 不改对齐语义 |
| en | `source.originalEditable` | Original workbook · Editable | app-diff | terminology | 直接模式保留；仓库模式继续明确 `Worktree · Editable` | 否 | 左侧唯一可写 |
| en | `grid.previewBeforeAfter` | Before/After · Hold | app-diff | wrong-register | `Hold for before` 或 `Before · Hold` | 减宽 | 否 |
| en | `context.appendSpecified` | specified id | app-merge | capitalization | `specified ID` | 否 | 否 |
| en | `externalChange.dirty` | Another program saved... | app-save | too-verbose | 拆为发生事项 + `Reload before saving to avoid overwriting it.` | 否 | 必须保留放弃未保存编辑风险 |
| en | `errors.corruptWorkbook` | damaged, encrypted, or cannot be read | app-errors | too-vague | 区分当前可判定原因；至少给出下一步“确认是未加密 .xlsx” | 可能 | 不承诺可恢复 |
| zh-CN | `toolbar.copyCellsToLeft` | 复制 1 格到左侧 | app-toolbar | terminology | `复制 1 个单元格到左侧`；空间不足处 `复制到左侧` | 可能 | 否 |
| zh-CN | `repository.detachedHead` | 分离头指针 | app-repository | terminology | `Detached HEAD`（开发者工具常用写法） | 否 | 否 |
| zh-CN | `repository.openDescription` | 选择导入…的方式 | app-dialogs | unnatural-word-order | `选择或拖入含 XLSX 的本地 Git 仓库。` | 减高 | 否 |
| zh-CN | `repository.noDifferentWorkbooks` | 没有差异工作簿 | app-repository | unnatural-word-order | `工作区与所选版本中的 XLSX 内容一致` | 可能 | 只指已语义确认的共有表 |
| zh-CN | `repository.selectReference` | 请选择对比引用 | app-repository | too-vague | `请选择要对比的 Git 版本`，帮助文本再解释分支/引用 | 否 | 不暗示联网 |
| zh-CN | `diff.filteredRows` | {categories}行 | app-diff | punctuation | 分类与“行”之间按自然组合输出，避免 `新增、修改行` 歧义 | 否 | 否 |
| zh-CN | `alignment.help` | 保守地按物理行比较 | app-diff | too-verbose | `空白或重复主键只按所在物理行比较。` | 否 | 保持局部回退事实 |
| zh-CN | `resolution.appendedEnd` | 左侧工作簿末尾 | app-merge | terminology | `左侧末尾`；状态已给上下文 | 减宽 | 否 |
| zh-CN | `externalChange.*` | 其他程序保存了此工作簿… | app-save | too-verbose | 按“文件已被其他程序修改。重新载入磁盘版本后才能继续保存…”统一 | 可能 | 强调避免覆盖外部改动 |
| zh-CN | `errors.saveFailed` | 无法安全保存工作簿 | app-errors | too-vague | 保留结构化原因，并提示原文件未替换/可重试（仅在事实可证时） | 可能 | 不新增保证 |
| ja | `app.title` | ブックの比較とマージ | app-navigation | literal-translation | `XLSX の差分確認と反映` | 减宽 | 避免误解为自动合并 |
| ja | `toolbar.openRepository` | ローカル Git リポジトリを開く | app-toolbar | layout | `Git リポジトリを開く` | 是 | 否 |
| ja | `toolbar.openFiles` | 2 つのファイルを開く | app-toolbar | layout | `2 ファイルを開く` | 是 | 否 |
| ja | `toolbar.copyCellsToLeft` | セルを左側へコピー | app-toolbar | terminology | 真实动作更适合 `左側へ反映` | 是 | 明确不是三方自动マージ |
| ja | `toolbar.configureUGit` | UGit を設定 | app-toolbar | layout | `UGit 設定` | 是 | 否 |
| ja | `toolbar.saveLeft` | 左側を保存 | app-save | layout | 保留文本，调整控件 nowrap/min-width | 是 | 左侧唯一可写 |
| ja | `repository.differingWorkbooks` | 差分のあるブック | app-repository | too-verbose | `差分ブック` | 减宽 | 否 |
| ja | `repository.selectReference` | 比較する参照を選択 | app-repository | literal-translation | `比較する Git リビジョンを選択` | 可能 | 不暗示 fetch |
| ja | `repository.pathUnavailable` | パスを取得できません | app-repository | fact-drift | `パスが見つかりません` | 否 | 实际是路径失效 |
| ja | `diff.currentSheetEqual` | 現在のシートは同一です | app-diff | unnatural-word-order | `このシートに差分はありません` | 否 | 否 |
| ja | `alignment.key` | キーで整列 | app-diff | terminology | `キー列で照合` 或 `キーで比較` | 可能 | 不改变行映射 |
| ja | `grid.previewBeforeAfter` | 変更前後 · 長押し | app-diff | wrong-register | 桌面按钮用 `変更前を表示（押している間）` 的短形式 | 是 | 否 |
| ja | `context.copy*` | 左側へコピー | app-merge | terminology | 单格/整行批准动作统一为 `左側へ反映`，追加仍用 `追加` | 可能 | 区分 copy 与 append |
| ja | `externalChange.defer` | 後で | app-save | too-vague | `今は再読み込みしない` | 可能 | 用户知道仍不可安全保存 |
| ja | `externalChange.dirty` | 別のプログラムが…破棄されます | app-save | literal-translation | 两句自然说明“ほかのアプリで変更”“上書き防止のため再読み込み” | 可能 | 保留未保存编辑丢失风险 |
| ja | `errors.unsupportedFormat` | 暗号化されていない .xlsx ブックのみ… | app-errors | wrong-register | `開けるのは暗号化されていない .xlsx ファイルだけです。` | 可能 | 不支持格式不扩展 |
| all | `controller.go:ConfigureUGit` | 全部中文原生对话框 | app-dialogs | safety-meaning | 复用现有 runtime locale，提供三语标题、正文、按钮、成功/取消结果 | 是 | 只替换 `*.xlsx`；显示旧路径/Git 来源 |
| all | `controller.go:beforeClose/confirmSessionSwitch` | 中文“保存并继续…” | app-dialogs | safety-meaning | 三语完整模板；按钮值与业务分支解耦，避免按翻译文本判断 | 可能 | 保存/丢弃/取消语义必须准确 |
| all | `controller.go:Save` | 中文 Git 操作中警告 | app-save | safety-meaning | 三语说明只修改当前 XLSX，不完成 Git 操作 | 可能 | 不自动恢复/提交 |
| all | `controller.go:SelectAndOpen/SaveAs` | 中文原生选择器标题 | app-dialogs | cultural-mismatch | 按 locale 提供自然标题和筛选名 | 否 | 右侧只读、保存目标不变 |
| all | `controller.go` 仓库通知 | 中文加载/索引/缺失引用通知 | app-repository | screenshot-mismatch | 三语化可见通知，动态句使用完整模板 | 可能 | 不改 Git 行为 |
| all | `internal/app/row_alignment.go` | 中文 Git 合并说明 | app-merge | safety-meaning | 三语化四种基线结论 | 可能 | 必须区分文件冲突与双方语义冲突 |
| all | `internal/app/session.go` 可见警告 | 中文 ID/追加/保存错误 | app-errors | safety-meaning | 通过 locale 输出三语，稳定 error code/JSON 不变 | 可能 | 不改变合并/ID 规则 |
| all | `internal/cli/cli.go` UGit 标签 | 中文“当前工作区/选中版本” | cli | screenshot-mismatch | 按 `--lang` 输出展示标签；JSON 键/枚举不翻译 | 否 | CLI 稳定字段不变 |
| en | `product/locales/en.json` description | compares ... validated references ... editable left workbook | website-home | too-verbose | 先说用户结果，再说明 Git 读取与左侧写入边界 | 否 | 不弱化只读/不 fetch |
| en | `site/homeCopy.ts` hero | in Git like code | website-home | marketing-cliche | 改为独立自然标题，如 `Review XLSX changes in Git, record by record.` | 是 | 不承诺代码级完整 diff |
| en | `homeCopy.process` | Review in four steps | website-home | marketing-cliche | `From comparison to a saved result` | 否 | 否 |
| zh-CN | `homeCopy.hero` | 像代码一样审阅和选择性合并 | website-home | marketing-cliche | 保留产品已形成的克制表达，减少“像代码一样”泛化 | 是 | 不夸大格式支持 |
| ja | `homeCopy.hero` | コードと同じように確認し | website-home | literal-translation | 直接说 `Git で管理する XLSX の差分を、レコード単位で確認。` | 是 | 不夸大 |
| ja | `homeCopy.core` | 設定表レビューに合わせて設計しています | website-home | unnatural-word-order | 日语开发工具常用简洁说明，减少“設計しています” | 否 | 否 |
| all | `site/app/features/page.tsx` | 功能页固定中文 | website-features | screenshot-mismatch | 建立组件内三语 copy map，不改路由/i18n 架构 | 是 | 90/16、16/15 事实一致 |
| all | `site/app/guide/page.tsx` | 指南固定中文 | website-guide | screenshot-mismatch | 三语化教程、快捷键、UGit 四步与 FAQ | 是 | UGit 两条差异工具属正常事实 |
| all | `site/app/download/page.tsx` | 下载页固定中文 | website-download | safety-meaning | 三语化平台、未签名/未公证、SHA-256、源码说明 | 是 | 不弱化风险，不改资产 |
| all | `site/app/layout.tsx` | 源码 `<html lang="zh-CN">` | website-seo | cultural-mismatch | 源码默认改为 `en`；静态后处理继续写各 locale | 否 | 默认语言仍英语 |
| en | `seo.home.title` | Review and selectively merge... | website-seo | too-verbose | 更像搜索标题，保留 Git/XLSX 核心词 | 无 | 不夸大 |
| ja | `seo.*` | Git 参照/反映等直译 | website-seo | literal-translation | 使用日本用户搜索表达 `XLSX 差分`, `Git リビジョン`, `変更を取り込む` | 无 | 保留签名限制 |
| all | `ScreenshotViewer` aria | aria 拼接完整 alt | app-accessibility | too-verbose | dialog/trigger 使用短操作名 + 简短图名，caption 保持正文 | 否 | 否 |
| ja | `GitReviewDemo` | マップID / 報酬グループ / 行ずれ変更 | website-home | terminology | `マップ ID`, `ドロップテーブル`, `位置ずれによる変更` | 可能 | ID/删除数量不变 |
| en | `GitReviewDemo` | Legacy Season Gate | website-home | cultural-mismatch | 让四个地图名属于同一世界观；删除记录名称不应像内部占位 | 可能 | 同 ID/key 不变 |
| zh-CN | `GitReviewDemo` | 旧赛季入口 | website-home | cultural-mismatch | 调整为同一幻想世界观地图名 | 可能 | 同 ID/key 不变 |
| ja | `GitReviewDemo` | 旧シーズンゲート | website-home | cultural-mismatch | 改为自然日语幻想地名 | 可能 | 同 ID/key 不变 |
| en | `README.md` 首段 | 先说明共享 Go core | readme | fact-drift | 第一屏先回答用途/对象；内部实现移至构建或架构链接 | 否 | 产品事实不变 |
| zh-CN | `README.zh-CN.md` | 英文共享 core 句、`Key alignment` 英文段 | readme | screenshot-mismatch | 模板改为三语独立段落 | 否 | 三语事实一致 |
| ja | `README.ja.md` | 英文共享 core 句、`Key alignment` 英文段 | readme | screenshot-mismatch | 完整日文化，补 Gatekeeper/下载说明 | 否 | 未签名/未公证不弱化 |
| all | `CHANGELOG*` | 部分条目偏实现/任务语气 | changelog | too-verbose | 每版按用户结果分句，技术细节仅保留必要限制 | 否 | 每个版本事实一致 |
| ja | changelog `マッピング/ポーリング/フォールバック` | 片假名/实现词集中 | changelog | wrong-register | 换成用户能感知的自然表述 | 否 | 不删除事实 |
| en | `cmd/genproductdemo` headers | First-clear Qty / Effect Asset | demo-workbooks | terminology | `First-Clear Quantity`, `Effect Asset Path` 等统一标题风格 | 是 | 数值/差异不变 |
| zh-CN | demo headers | 职业定位 / 生命值 | demo-workbooks | terminology | 统一为 `定位`、`生命值`（或全表一致的项目约定） | 是 | 否 |
| ja | demo headers | ドロップ比重 / エフェクト | demo-workbooks | literal-translation | `ドロップ重み`、`エフェクトパス`；角色/关卡/技能统一世界观 | 是 | 否 |
| all | demo column widths | 三语同一列宽 | demo-workbooks | layout | `writeWorkbook` 按 locale/表设置列宽 | 是 | 场景与差异数不变 |
| all | `site/public/screenshots/<locale>/...` | 文件不存在 | screenshots | screenshot-mismatch | 用当次三语 Wails 构建生成 4×3 张真实截图 | 是 | 90/16 与 16/15 场景必须一致 |
| zh-CN | 现有 `sheetproof-*-v030.png` | 中文 UI/数据 | screenshots | screenshot-mismatch | 仅作第一轮对照；正式引用改为 locale 目录新图 | 是 | 不删除旧资产，避免无关破坏 |
| all | 安全边界集合 | 文件本地、无 Excel、未签名/未公证、左写右只读等 | website/readme/app | safety-meaning | 完成三语回译表并逐项验证 | 否 | 禁止弱化、夸大或新增保证 |

## 基线实机与浏览器证据

- 桌面：`build/acceptance/localization-copy-review/before/{en,zh-CN,ja}.png`。三语工作簿均可打开；基线偏好为按物理行，显示 90 处差异。日语工具栏出现明显两行换行。
- 官网：本地静态产物 15 个路由均可访问；`en/ja` 的 features、guide、download 仍输出中文；三语 screenshot URL 均为 404。
- 截图：现有四张 v0.3.0 公开图均为中文 UI 和中文表格数据；不存在三语独立图片。

## 高风险语义回译门禁

修改完成后必须逐项回译并在三种语言确认：文件不上传；不依赖 Excel；Windows 未签名；macOS 未签名且未公证；外部修改后先重载；左侧唯一可写；不自动 add/commit/push/fetch/切换分支；仅支持 `.xlsx` 且不支持 `.xlsm`；高级对象不保证完整保真；保存使用同目录临时文件、重开校验和原子替换。

## 第二轮实施与回译结果

### 已实施的主要修正

- 应用：三语来源标签、工具栏、差异索引状态、主键说明、外部修改、未保存切换、Git 操作中保存、UGit 配置、另存为和冲突追加错误均按 runtime locale 输出。稳定 JSON/CLI 字段未翻译。
- 官网：home、features、guide、download、changelog 的 15 个静态路由均有独立三语正文；英文与日文 Hero 使用各自语义断句；SEO title、description、Open Graph、Twitter、JSON-LD 与 `html lang` 按 locale 生成。
- 文档：README 公共模板不再向中日文插入英语段落；限制明确写出 `.xlsm` 不支持与高级对象不保证完整保真。CHANGELOG 保留版本事实并按 locale 生成。
- 演示数据：三语保持相同 sheet、ID、key、插入位置、数值变化和结果数；按 locale 调整人类可读名称、标题与列宽。生成器测试固定首表 `90 → 16 → 15` 的物理行、主键对齐和应用结果。
- 截图：生成 `en`、`zh-CN`、`ja` 各四张真实 Wails 截图。全幅图中的来源标签分别为 `Local (editable)` / `Comparison source (read-only)`、`本地（可编辑）` / `对比来源（只读）`、`ローカル（編集可能）` / `比較元（読み取り専用）`。

### 高风险语义回译

| 事实 | en 回译 | zh-CN 回译 | ja 回译 | 结论 |
|---|---|---|---|---|
| 文件不上传 | Files stay local; app/site do not upload workbooks | 文件只在本机处理，官网无上传入口 | ローカルで処理し、Web サイトにもアップロード機能なし | 未弱化，未承诺绝对网络隔离 |
| 不依赖 Excel | Excel is not required | 不依赖 Excel | Excel は不要 | 一致 |
| 签名状态 | Windows unsigned; macOS unsigned and not notarized | Windows 未签名；macOS 未签名且未公证 | Windows は未署名、macOS は未署名・未公証 | 一致，并要求 GitHub Releases / SHA-256 |
| 左侧唯一可写 | Only the left side is writable; right is read-only | 左侧唯一可写，右侧始终只读 | 書き込み可能なのは左側だけ、右側は常に読み取り専用 | 一致 |
| Git 自动操作 | No fetch/checkout/switch/add/commit/push | 不 fetch、checkout、切换分支、暂存、提交或推送 | fetch、checkout、ブランチ切り替え、stage、commit、push を行わない | 一致；未暗示联网更新 |
| 主键与回退 | Align reliable keys; ambiguous records alone fall back to physical rows | 可靠记录按主键对齐，歧义记录仅自身回退物理行 | 信頼できるキーで揃え、曖昧なレコードだけ物理行比較 | 一致 |
| 外部修改 | Reload before saving to avoid overwriting external changes | 重新载入磁盘版本后再保存，避免覆盖外部修改 | 外部変更を上書きしないよう再読み込みしてから保存 | 风险与下一步均保留 |
| 格式与高级对象 | `.xlsx` only; no `.xlsm`; advanced objects not guaranteed to round-trip intact | 仅 `.xlsx`；不支持 `.xlsm`；高级对象不保证完整保真 | `.xlsx` のみ；`.xlsm` 非対応；高度なオブジェクトの完全保持は保証しない | 未扩大支持范围 |
| 保存机制 | Same-directory temporary file, reopen validation, atomic replacement | 同目录临时文件、重开校验、原子替换 | 同じディレクトリの一時ファイル、再オープン検証、アトミック置換 | 一致，未新增“绝不丢失”等保证 |
| License | MIT License | MIT License | MIT License | 一致 |

### 仍需母语人士确认

- 日语开发团队对 `キーで比較`、`キー列によるレコード照合` 与 `Git リビジョン` 的偏好可能随团队术语表不同；当前选择符合常见开发工具表达，但尚未经过独立日本母语审校者签字确认。
- 中文角色、关卡和技能名以及日文幻想名称已检查为同一世界观风格，仍属于创意命名，不存在唯一“正确”译法。
- Windows 环境无法验证 macOS Gatekeeper 的实际按钮名称；文案只陈述未签名、未公证和需要用户明确允许，没有提供未经实测的逐按钮教程。

### 残留混合语言边界

- 正常打开双文件、仓库选择、主键/物理行比较、应用修改、保存、外部重载与截图所覆盖的三语流程中，未发现固定中文来源标签或中文 UI 残留。
- `internal/repository` 与 `internal/ugit` 仍有少量仅在路径校验、Git 命令失败或配置回滚失败时透出的中文底层诊断。它们不出现在本轮正常流程截图中，但英文或日文环境触发这些异常时，错误详情仍可能混入中文。本轮已把 Controller 层可见的仓库空状态、引用缺失、刷新、索引和缓存通知补成完整三语模板；底层诊断需要后续引入稳定错误码后再本土化，避免按错误字符串做脆弱映射。

## 第三轮：窄窗工具栏与设置窗口

- 1210×900 实机基线确认三语工具栏在缩放、另存、保存和 UGit 区域发生覆盖；断点改为
  1280px 以下使用 locale 专属短文案：`Repository / Git 仓库 / リポジトリ`、
  `Apply / 应用 / 反映`、百分比缩放和短“保存”。1440px 及以上继续显示完整术语。
- UGit 与语言入口统一移入 `Settings / 设置 / 設定`。UGit 结果只在设置窗口呈现，避免
  主界面位于模态层下方时用户看不到反馈。
- 新增清理文案的三语回译：缓存操作只删除差异索引；全部清理会删除最近仓库、保存的
  对比版本、搜索历史和视图设置，当前会话仍打开，下次启动无恢复仓库；两项操作都明确
  不删除工作簿、不修改仓库或 Git 数据。稳定 JSON/CLI 字段、差异数量与合并/保存语义未改。
- 当次 Windows/Wails 产物已在三语 1210×900、1440×900、1600×900 主界面以及三语
  设置窗口、清缓存/全部清理确认框和列标题上下文菜单中逐张复核；未发现文字重叠、截断、
  不合理换行或应用文案混用。隔离配置实测 UGit 反馈留在设置窗口，两类清理均按文案执行；
  三语演示数据首表继续保持 `90 → 16 → 15`。
