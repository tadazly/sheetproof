# 第二、三轮交付与下一轮迭代交接

更新时间：2026-08-05

本文是新会话接手 SheetProof（原 `ugxlsx`）的当前事实基线。第一轮历史交付保留在
`docs/iteration-1-handoff.md`；项目约束见根目录 `AGENTS.md`，实现设计见
`docs/architecture.md`。

## 当前工作区状态

本次 Windows 接手开始时实际分支是 `main`，当时提交及远端均为：

```text
5315d1a Implement repository comparison and merge workflow
```

用户提示中的 `6933f8a` 和“领先 origin/main 1 个提交”与本机实际状态不一致；
修改前工作树为 clean。第一批 Windows 修复已提交为
`ee5663a Fix Windows desktop integrations` 并推送；后续 TaskDialog STA 修复的
准确提交状态应以接手时的 `git status` 和 `git log` 为准。除非用户明确要求，
不得自行 commit、push、改写提交或发布。新会话看到的工作树改动均应视为用户
有效成果，不得执行 `git reset --hard`、
`git checkout --` 或覆盖式清理。

主要改动范围：

```text
controller.go / controller_test.go
internal/repository/
internal/app/
internal/cli/
internal/preferences/
frontend/src/
frontend/dist/
README.md
docs/
main.go
```

每次 Wails/前端生产构建都会更新带内容哈希的 `frontend/dist/assets` 文件名，
删除旧哈希并新增新哈希属于正常构建结果。

Windows 的 Wails 入口现统一为 `scripts/invoke-wails.ps1`：先检查匹配版本的已安装
CLI 和 `build/tools/wails-v2.10.2.exe`，没有时再从 `GOMODCACHE` 中固定版本源码
离线编译；后续子命令统一使用项目内 `build/cache/go-build`，避免默认缓存的历史
ACL 造成拒绝访问。`go run ...@v2.10.2` 即使已有源码也可能为弃用元数据访问 Go Proxy，
因此其触网或失败不得再表述为“本机没有 Wails”；只有离线入口确认源码目录或
依赖缓存确实不完整时才能请求联网。

### 2026-07-31 Windows 实机修复

- 实际宿主为 Windows 10 Pro 21H2（build 19044），不是 Windows 11；因此下面的
  原生构建和取证不能表述为 Win11 完整验收。
- 缓存未命中的 250 表真实仓库取证捕获到
  `ugxlsx.exe -> git.exe -> conhost.exe`。对应命令包括候选扫描，以及逐表
  `cat-file -e` / `show <ref>:<path>`；不是 Go 工作簿解析或前端轮询。
- `internal/repository` 与 `internal/ugit` 的后台命令统一经
  `internal/backgroundcmd` 创建。Windows build tag 设置 `CREATE_NO_WINDOW`，
  macOS/Linux 为空操作；超时、context 取消、参数数组和错误输出均保留。
- Wails 2.10.2 Windows 问题对话框固定使用 `MB_YESNO`，忽略自定义按钮并返回
  `Yes` / `No`。这同时导致“配置 UGit”确认无效和未保存切换/关闭无法匹配中文
  选择。Windows 已改用 TaskDialog；关闭和切换均提供“保存并继续 / 不保存并继续 /
  取消”，非 Windows 继续使用 Wails 实现。后续实机复测发现首次真正进入
  TaskDialog 时返回 `E_INVALIDARG (0x80070057)`；根因是 Wails 绑定回调线程未满足
  TaskDialog 的 STA 前置条件。Windows 辅助函数现锁定同一 OS 线程、初始化 STA，
  调用完成后成对反初始化。删除 UGit XLSX 配置后的同路径复测已不再立即报错，
  原生模态对话框成功进入等待选择状态。
- 2026-08-01 用户再次报告“配置 UGit”仍收到 `HRESULT 0x80070057`，导致确认链路
  在真正写配置前中断。Windows 选择对话框现只接受当前进程中可见且启用的 owner；
  TaskDialog 失败时先去掉 owner 重试，再回退到标准 `MessageBoxW`，并明确映射
  两按钮与三按钮的业务含义。新增回归覆盖 TaskDialog 返回 `E_INVALIDARG` 后选择
  “确定”仍返回“配置 UGit”，从而继续调用原有 `internal/ugit` 事务式写入流程。
- 本机 UGit 是 5.51.0。UGit 自带 Git 2.45.2，
  系统 PATH Git 是 2.37.1.windows.1。Windows 配置逻辑现优先按自然版本选择
  UGit 自带 `cmd\\git.exe`，并回读 `--show-origin`；界面显示 Git 路径和配置来源。
- 已在真实全局配置中覆盖错误 XLSX 旧路径，先写入带中文和空格的固定测试路径，
  再覆盖为最终 `build\\bin\\ugxlsx.exe`；回读来源为用户全局 Git 配置文件，命令中的
  `$LOCAL`、`$REMOTE`、`$MERGED`
  保持原样。临时 EXE 已删除。UGit 已强制退出并重启，且从 UGit 打开 250 变更验证仓库。
  由于 Windows 10 图形捕获接口不支持当前自动化截图，尚未从 UGit 文件行真实
  触发差异和合并各一次。
- Windows EXE 原本已嵌入正确 ICO，但 Wails 只设置 `ICON_SMALL`；现在
  `OnDomReady` 后补设 `ICON_SMALL`、`ICON_SMALL2` 和 `ICON_BIG`。资源管理器、
  任务栏和 Alt+Tab 的目视复核仍需真正的 Windows 11 会话完成。

本轮自动化结果：`go test ./...`、`go vet ./...`、前端 lint/typecheck、29 项测试、
前端生产构建和 Windows amd64 Wails 原生构建通过，`git diff --check` 通过。
`go test -race ./...` 先因 `CGO_ENABLED=0` 未启动，启用 CGO 后又因本机没有
`gcc` 失败，不能记为通过。最终仅保留 `build/bin/ugxlsx.exe`：17,225,728 字节，
SHA-256 `0D515D01B7F4C983D2B8D016AD9C2C4AA66347E16D801EF0821114F6FD4993D8`；
PE machine `0x8664`、PE32+、GUI subsystem。关联图标提取结果与已确认的项目图标
PNG 哈希一致。最终 EXE 已在本机原生启动并显示 250 表索引结果；任务栏无闪烁、
索引中关闭、UGit 差异/合并、保存快捷键和三处系统图标仍需 Win11 目视/交互验收。

## 当前可运行主流程

### 2026-08-06 应用内帮助

- 顶部工具栏在设置左侧提供独立“帮助 / Help / ヘルプ”按钮，欢迎页和已打开工作簿时均可用；
  宽屏显示文字，既有窄屏规则下只保留图标。
- 独立模态从构建后的 `frontend/package.json` 读取版本，按界面语言用 Wails
  `BrowserOpenURL` 打开官网 guide 页面，并集中展示与前端实际实现对应的查找、浏览、编辑和
  保存快捷键。它拦截背景快捷键、锁定 Tab 焦点，并在关闭后恢复帮助按钮焦点。
- 产品事实源、三语功能文案与三语官网 guide 同步记录该能力；`0.6.0` 预览候选保留
  已发布 `0.5.0` 的记录不变。

### 2026-08-06 v0.6.0 正式发布候选

- 最新公开、非草稿 Release 是 Preview `v0.5.0`。本轮新增应用内帮助，属于向后兼容的用户工作流扩展，
  因此按 SemVer 推导 Preview `v0.6.0`。
- 版本、三语更新日志、README、CLI、桌面/官网 package 与 lockfile、官网生成内容和预期下载链接已同步。
  候选尚未暂存、提交、推送、打标签、创建 Release 或部署官网；须在维护者确认后执行这些外部操作。

### 2026-08-06 v0.6.0 正式发布完成

- 维护者确认后，发布提交 `7c6a12d7eabe52c45856e5f1ee653a9ac41408a3` 已普通推送至 `main`，
  随后创建并推送 annotated tag `v0.6.0`；没有强推、移动既有标签或混入忽略的验收与审计产物。
- 标签前手动 Release workflow run `31069433223` 及标签 workflow run `31069748362` 均通过源码验证、
  Windows amd64 和 macOS universal 构建；标签 workflow 已创建 Draft Release。两项平台资产与
  `SHA256SUMS.txt` 逐项核对一致后，整理后的用户更新说明已替换自动生成内容，并公开为 Preview：
  `https://github.com/tadazly/sheetproof/releases/tag/v0.6.0`。Windows 产物仍未签名，macOS 产物
  仍未签名且未公证，Release 与官网均保留该限制。
- 常规 Go、前端与官网验证均通过；官网 lint 为 0 错误、静态构建 16 条路由、11 项渲染测试通过。
  Windows 桌面产物报告版本 `0.6.0`，并已以 English、简体中文和日本語实际打开帮助窗口、检查版本、
  本地化说明与快捷键后保存截图验收。
- 公开 Release 后重新完成官网内容验证，并通过 `scripts/deploy-site-lightsail.ps1` 原子部署到正式域名。
  源站静态文件与 Caddy 配置有效，同一 Caddy 实例的 5 个既有虚拟主机均完成源站健康检查；Cloudflare
  公网首页、功能、使用说明、下载、更新日志、favicon 和 Open Graph 图片均返回 200 且具有代理响应头，
  下载页已指向两个 `v0.6.0` 平台资产。
- 最终隐私扫描覆盖发布范围、Release Notes、官网静态产物以及下载并展开的 Windows/macOS 实际资产；
  私钥、访问令牌、构建机/工作区路径和私网 IPv4 均未命中。未发现需删除、轮换或重写历史的敏感数据。

### 仓库模式

```text
打开或拖入本地 Git 仓库
→ 定位仓库根目录并恢复最近仓库
→ 在仅含 XLSX 的目录树中选择相对路径
→ 左侧读取当前工作区真实文件
→ 右侧选择其他本地或远端跟踪分支
→ 通过 Git 对象读取同路径文件
→ 复用现有工作簿差异、合并、编辑和撤销
→ 安全保存到当前工作区文件
```

已实现行为：

- 初始空状态以“打开本地仓库”为主入口，直接双文件入口保留为次入口。
- 原生目录选择、目录拖放、从子目录向上定位根目录、普通目录/文件拒绝。
- 记住最后一次成功打开的仓库；最近仓库无效时安全回到空状态。
- 偏好中按最近使用顺序保存最多 10 个仓库。“切换仓库”显示最近仓库弹窗，当前
  仓库与失效路径不可点击，并保留“选择其他仓库”入口；顶部和欢迎页的“打开本地
  仓库”仍直接使用原生目录选择器。
- 顶部显示仓库名称、完整路径、当前分支、Detached HEAD、工作区修改和进行中
  Git 操作。
- 目录树仅显示包含 `.xlsx` 的目录和 `.xlsx` 文件，隐藏 `.git`，支持中文、
  空格、多层路径、展开折叠、刷新、拖动调宽和持久化宽度。
- 目录树使用紧凑小字号；顶部搜索框按完整相对路径筛选文件和目录，搜索时自动
  展开匹配路径。后台差异索引轮询保留用户的目录展开集合，不会令刚收起的目录
  自动展开。
- 搜索框使用易点击的独立清除按钮；聚焦时在下方显示当前仓库最近 5 条搜索记录，
  记录以仓库根路径为键保存在本地，失焦或按 Enter/Esc 后自动收起且不同仓库互不串用。
- 仓库侧栏有“仓库文件 / 差异表 / 工作表与差异”三页签；最后一项保留原有
  Sheet 切换和差异索引。
- 差异表使用可持久化的精确语义索引。Git 只负责筛选双方共有且内容变化的
  候选，缓存未命中时仓库界面先返回，后台复用原有 Go 引擎过滤无语义差异的
  XLSX；索引完成前不显示未经确认的候选，也不在文件名旁预计算单元格差异数。
  打开具体表格后仍由原有 Go 引擎显示精确差异数。窗口关闭会取消当前索引的
  Git 读取和工作簿扫描，再完成临时文件清理，不等待所有候选比较完毕。
- 单个候选若损坏、无工作表或内容其实是旧二进制/加密 OLE 复合格式，会被明确
  跳过并继续索引；有效结果仍按完整签名缓存，因此未变化时下次启动不重复失败，
  文件修复后刷新会因签名变化重新检查。Git、权限、取消等真实故障仍终止索引。
- 当前分支从本地分支候选中排除；本地分支排在远端之前；组内自然排序；
  `origin/HEAD` 等符号引用排除。
- 对比分支按仓库记录完整引用并在下次打开时恢复；没有记录或记录项已不存在
  时优先选择当前分支的 upstream，其次选择同名远端引用，最后才回退到候选
  第一项；不自动 fetch。
- 右侧使用完整 `refs/heads/...` 或 `refs/remotes/...`，不因同名分支选错。
- 右侧通过 Git 对象导出到系统临时目录，不切换分支、不修改工作区；临时文件
  使用唯一目录并在替换来源或退出时清理。
- 未选择文件、正在加载、正在比较、无其他分支、未选引用、引用中缺少同路径
  文件、无效 XLSX 和读取失败都有独立界面状态，不使用空白网格冒充。
- 切换文件/仓库时如有工具内未保存编辑，支持“保存并继续 / 不保存并继续 /
  取消”。仅切换右侧引用会保留左侧内存编辑并重新计算差异。
- 仓库模式保存目标固定为 `<root>/<relative.xlsx>`；不执行 add、commit、
  push 或任何 Git 恢复操作。保存后重新读取 Git 状态；若能证明 HEAD、引用、
  文件列表和其他 XLSX 都未变化，只增量更新当前表格的差异表成员与缓存。手工
  刷新在签名未变化时直接复用缓存，不再无条件重建。
- 仓库模式“另存为”导出独立副本，不改变工作区保存目标，也不清除当前 dirty
  状态；`Ctrl/Command+S` 保存当前工作区文件。
- merge、rebase、cherry-pick 等操作进行中时可以查看，保存前显示警告。

### 直接文件模式

第一轮直接选择两个 `.xlsx` 的能力完整保留，仍使用同一套 Session、差异、
合并、撤销和安全保存逻辑。CLI `compare` 兼容 UGit/Git difftool 在新增或删除
文件时传入的 `/dev/null`（Windows 兼容 `NUL`）：CLI 在系统临时目录建立同
工作表结构的空白占位簿，窗口关闭后清理，不写入用户仓库。检测到 Git
difftool 的成对路径计数环境变量时会强制整个会话只读，界面隐藏临时目录并
明确显示两侧都是 Git 只读快照；普通双文件和 Git mergetool 不受影响。UGit
5.51.0 注入的 `LOCAL_TITLE` / `REMOTE_TITLE` 会在未显式传 label 参数时
自动成为两侧展示名，显式参数逐侧优先。UGit 的 XLSX 差异工具现注册为
`SpreadsheetCompare` 直接路径列表协议；当列表经实际 Git 目录确认包含真实工作区
文件时，该文件会交换到可编辑左侧，两个历史快照或归属不明时仍双侧只读。合并工具必须同时传
`--base "$BASE" --output "$MERGED"`。`$BASE` 用于三方语义说明，实际覆盖、
追加和结果保存仍由 `$LOCAL` / `$REMOTE` 会话完成。

顶部低频操作“配置 UGit”可把当前 macOS `.app` 内部可执行文件或 Windows
`.exe` 注册为全局 `*.xlsx` 差异/合并工具。它先检测现有 XLSX 项，经原生对话框
确认后只替换该后缀，并保持 `trustExitCode=false`；写后精确校验，失败时恢复
快照。应用移动后再次点击会覆盖旧路径；临时/App Translocation 路径不允许
注册。UGit 运行中完成配置后需要重启 UGit。

### CLI 仓库入口

```bash
sheetproof repo --path "/path/to/repository"

sheetproof repo \
  --path "/path/to/repository" \
  --file "config/activity/reward.xlsx" \
  --ref "origin/develop"
```

`--file` 和 `--ref` 无效时明确失败，不静默选择其他候选；命令不会 fetch 或
切换分支。

## 当前表格 UI 行为

- 界面已使用统一的语义设计 Token 和同一套线性 SVG 图标；顶部按来源、差异导航、
  编辑与合并、保存结果分组，文件来源卡明确区分可写原始表格与只读目标表格。
- 网格上方新增紧凑结果摘要，显示总差异、增加、删除、修改、冲突、当前筛选和
  选择数量；相等时明确显示“两侧内容一致”。
- 结果摘要的增加、删除、修改、冲突已改为多选行筛选。摘要的单元格差异与四类
  行数只对应当前选中的工作表，不再累加整本工作簿。每次启动默认全不选并显示
  完整表格；选择后通过后端稀疏 `FilteredRegion` 只加载对应语义行，左右缺失侧
  仍成对显示。`1`–`4` 切换四类，`5` 切换全选/全不选；左侧差异索引切换分类时
  同步为对应单类筛选。冲突追加 ID 后，冲突筛选把左侧追加目标行与右侧原来源行
  配对，不把目标行作为另一分类重复显示；筛选选择不跨启动保存。切换筛选前会
  记录当前可见/选中的来源物理行与视口偏移，取消后回到同一原始行；新分类不含
  该行时选择最近的匹配行，不再一律跳回第一行。
- 按钮、输入框、页签、空状态、错误条、加载层、ID 弹窗和状态栏共享统一状态语言，
  支持可见键盘焦点、disabled/busy 反馈、减少动态效果和最小窗口附近的响应式收缩。
- 左右表格同步滚动、窗口化区域加载、同步列宽、缩放、差异导航、范围/整行
  选择和批量复制继续有效。
- 区域请求仍保持 48×20，不加载整张工作表；窗口会在可见范围前后留出缓冲，
  并在连续滚动期间约每帧合并预取相邻区域，不再等待停滚后才请求，因此快速
  纵向或横向滚动时不易露出空白画布。
- Region 不再复用全局 `guard/busy`，滚动预取不会令顶部工具栏、导航、保存等按钮
  反复 disabled，从状态源头消除整窗闪烁。
- 普通滚轮和触控板横纵输入不再逐事件重排；连续增量按动画帧合并，再把浏览器
  钳制后的实际位置同时写入左右两侧，并忽略已同步位置产生的回声 `scroll`。
  直接拖动滚动条及程序化滚动继续使用原生 `scroll` 回退同步。
- 打开表格时若摘要选中的工作表无差异，会自动切到首个有差异的工作表；初始
  视口按“冲突、修改、删除、增加”优先级定位到首个差异。编辑、复制、撤销和
  保存触发的摘要刷新保留用户当前视口，工作表请求也有独立代次避免旧结果回写。
- 工具栏缩放按钮显示明确的“缩放 N%”；`Ctrl/Command + 滚轮` 调整比例，点击
  按钮恢复 100%。
- 左侧双击单元格原位编辑；Enter 或失焦提交，Esc 取消。右侧始终只读。
- 左侧标题栏提供按住态“前后对比”。按住按钮，或在左右任一网格聚焦时按住 Tab，
  左侧会暂时显示本次打开表格时的原样；松开立即回到最新结果。工具栏、搜索框和
  内联编辑框中的 Tab 仍沿用原有行为。
- 空字符串清空、`=` 开头作为公式、普通数字作为数字、其他内容作为文本。无值
  但仅带样式或默认类型元数据的空白单元格统一归一为不存在，复制单格或整行后
  不再产生左右都空的红/绿色；显式空字符串仍保留为真实单元格。
- 当输入值与右侧原始值完全相同，会优先沿用右侧数字/文本类型。这解决了右侧
  是文本 `"123"`、左侧输入被误判为数字后肉眼相同但仍持续标黄的问题。
- 底部旧编辑表单已移除，替换为两行差异栏，显示当前工作区值/类型、对比来源
  值/类型和差异状态。
- 配色遵循 Git 双栏习惯：增加值在右侧为绿、删除值在左侧为红、修改为左红
  右绿、缺失侧为中性斜纹、冲突为橙色；选中差异保留语义底色并叠加蓝色边框。
  工作表摘要显示四类行计数；差异索引提供四类页签，默认优先级是冲突、修改、
  删除、增加，并默认选中该分类首项。
- 增加/删除/修改的右键菜单可复制单元格或整行；冲突菜单改为覆盖，并可把右侧
  整行按左侧最大整数 ID 自动续号追加，或在弹窗中为每个选中来源行指定 ID。
  多选中夹有未改动单元格/行时仍显示可用操作，并只提交其中的差异坐标/行；若
  同时选中冲突与非冲突差异，菜单只提示“选中的内容中包含冲突，请单独处理冲突部分”。
  多行覆盖/追加作为一个撤销步骤，完成后增量更新差异和行分类。
- 只有 `id` 列现有非空数据全部为整数时才提供自动/指定 ID 追加。文本属性名等
  非整数 ID 列的纯冲突选区显示“将整行新增到左侧”，并把来源行原样追加到左侧
  数据末尾；增加、删除和修改行不显示该新增操作。来源填充样式编号超出 excelize 0–18
  范围时只降级该填充并保留值/公式；失败回滚也包含可能部分写入的当前单元格，
  不会令后续右键操作持续报同一错误。
- 冲突覆盖/追加会在右侧来源行显示可撤销的处理标记。已复制相等的单元格立即
  清除差异色；追加目标行在左侧显示新增绿，不沿用纯坐标对比产生的删除红。
- Go 后端的相等判断仍是 `present + raw + formula + type`；其中 `type` 是归一后的
  语义类型，共享字符串与内联字符串都记为 `string`，不会因 OOXML 文本存储编码
  不同产生差异。数字与文本、公式等真实类型差异仍由底部类型信息解释。
- 左右网格的行号和列标题使用滚动容器内的原生 sticky 图层。被直接拖动的一侧
  不再等待 `scroll` 回调补偿位置；左侧或右侧作为滚动源时都应从第一帧保持固定。

## 关键实现位置

| 路径 | 当前职责 |
|---|---|
| `controller.go` | Wails 生命周期、仓库状态机、后台语义索引、未保存确认、异步代次 |
| `internal/repository/repository.go` | Git 根发现、XLSX 扫描、分支读取、变化候选、对象导出、状态检测 |
| `internal/app/session.go` | 左侧会话、替换/卸载右侧、差异、编辑、撤销和保存 |
| `internal/cli/cli.go` | `repo`、`compare`、`diff` 等参数和退出码 |
| `internal/ugit/config.go` | UGit `*.xlsx` 工具检测、跨平台 Git 定位、路径更新和配置回滚 |
| `internal/preferences/save_location.go` | 最近保存目录、最近 10 个仓库、侧栏宽度、对比引用和语义差异索引 |
| `frontend/src/App.vue` | 两种入口、仓库状态、三类目录树页签、网格、内联编辑和差异栏 |
| `frontend/src/style.css` | 设计 Token、响应式三栏布局、控件状态、目录树和选择/差异视觉层级 |
| `frontend/src/components/` | 统一 SVG 图标和可复用空状态展示组件 |
| `frontend/src/backend.ts` | Wails Controller 的前端适配 |
| `frontend/src/types.ts` | Summary、Region 和 Repository 前端类型 |
| `frontend/src/App.test.ts` | 仓库状态、竞态、页签/搜索、编辑和选择交互测试 |
| `build/appicon.svg` | 跨平台应用图标设计源；派生 1024 PNG、macOS ICNS 和 Windows 多尺寸 ICO |

`internal/app.Session.FilteredRegion` 负责多分类差异行的压缩视口和冲突追加目标映射；
普通完整表格仍使用原 `Region`，两条路径均保持最大 300×100 的后端边界和前端
48×20 请求窗口。

## 不得破坏的实现约束

1. 左侧仓库文件必须来自工作区，不得替换为 `git show HEAD:path`。
2. 右侧引用不得通过 checkout/switch 获取。
3. 仓库路径和文件匹配必须使用规范化相对路径，不得只看文件名。
4. Git 子进程必须是参数数组调用、明确工作目录、超时、无 shell。
5. 引用必须来自已读取候选列表，不能把任意用户字符串直接交给 Git。
6. `Session.ReplaceRight` 必须保留左侧内存编辑、撤销栈和保存目标。
7. 缺少右侧文件必须使用 `DetachRight`/明确状态，不能构造空工作簿。
8. 区域和仓库加载均有代次编号；旧请求不得覆盖新选择。
9. 保存继续走 `internal/storage` 的安全写入协议。
10. 仓库模式和直接文件模式必须继续复用同一套核心逻辑。

## 自动化覆盖

Go 测试目前覆盖：

- 有效/无效仓库、从子目录定位根目录、中文和空格路径。
- XLSX 扫描和忽略 `.git`。
- 当前分支、Detached HEAD、本地/远端分支顺序、排除当前分支和符号引用。
- 双边共有 Git 候选筛选、后台语义过滤、样式-only 排除、索引缓存/失效、默认
  同名远端、按仓库恢复对比引用，以及关闭时先取消索引再等待清理。
- 损坏、无工作表和伪 `.xlsx` OLE 候选的逐文件跳过、有效结果缓存、重启复用，
  以及文件变化后签名失效；Git/权限等非工作簿格式错误仍保持失败。
- 引用中存在/不存在同路径文件、对象读取不切分支且不修改工作区。
- 分支名和文件路径参数安全。
- 最近仓库恢复、仓库移动后的回退、侧栏宽度偏好。
- 仓库完整对比、合并、编辑、撤销、保存及 Git 状态/分支不变。
- 未保存状态下切换的保存、丢弃和取消路径。
- 目录拖放首项处理和文件拒绝。
- 原有读取、差异、批量复制、撤销、外部修改检测和安全保存。
- 共享字符串与内联字符串的真实 OOXML 生成夹具归一为相同文本语义且比较结果相等。
- 样式-only 空白单元格归一、整行复制后不产生空白差异，以及仓库导出副本不
  改变工作区保存目标和 dirty 状态。
- 未变化手工刷新复用语义索引；保存当前表格时在其他索引输入稳定的前提下只
  更新该表的成员和新签名缓存。
- UGit/Git 外部差异工具 null device 适配、临时占位簿生命周期和双 null 拒绝。
- Git difftool 成对环境标记识别、自动只读和单个环境变量防误判。
- UGit `LOCAL_TITLE` / `REMOTE_TITLE` 自动标签及显式 label 覆盖。
- UGit 自动注册只影响 `*.xlsx`、移动后更新路径、清理同后缀冲突项并保留其他
  后缀配置。
- 增加/删除/修改/冲突行分类、ID 元数据、整行覆盖、自动/指定 ID 多行追加及撤销。

前端共有 27 个测试，覆盖：

- 仓库优先空状态和直接双文件次入口。
- “打开本地仓库”保持原目录选择行为；“切换仓库”显示最近仓库列表、当前/失效
  项禁用，并可从列表直接打开或继续选择其他仓库。
- 目录树、搜索、按仓库缓存的最近 5 条搜索记录、易点击清除按钮、明确空状态和
  缺失引用状态，以及索引轮询期间保持目录收起状态。
- 仓库侧栏三页签、后台索引状态、未验证候选隐藏、差异文件筛选，以及左右两侧
  分别作为滚动源时的同步与原生 sticky 行号/列标题图层。
- 长表滚动在当前区域离开视口前预取带缓冲的新区域，并把连续滚动事件合并为
  每帧至多一次调度。
- Region 请求期间全局工具按钮不进入 disabled；左右任一侧作为普通滚轮输入源时，
  横纵增量按动画帧合并后同步应用到两侧，缩放后的滚动位置也复用同一同步入口。
- 快速文件切换时忽略过期结果。
- 加载提示、`Ctrl/Command+S` 保存、另存快捷键、仓库导出副本、差异导航和
  底部区域滚动。
- 打开时跳过无差异工作表，按分类优先级定位首个差异，并拒绝旧工作表区域回写。
- 拖选、右键批量复制和撤销。
- 多选夹有未改动项时保留右键操作，以及冲突/非冲突混选时的独立处理提示。
- 双击原位编辑、Enter 提交、Esc 取消。
- 数字形文本跟随右侧文本类型。
- 编辑到相等后差异 class 消失；选中差异保留 difference/selected/active 状态。
- 冲突整行视觉、工作表冲突计数、整数 ID 的自动范围/逐行指定 ID，以及非整数
  ID 冲突的原样追加；普通修改行不显示追加操作。
- Git 左红右绿修改配色、四类索引默认优先级/筛选、冲突处理标记和追加目标色。
- 统一 SVG 图标尺寸/可访问属性和可复用空状态文案/操作插槽。
- Git difftool 两侧只读标识、临时目录隐藏及编辑/合并/保存操作禁用。
- “配置 UGit”低频按钮、后端调用和成功状态反馈。

## 最近验证快照

2026-08-01 优化长表滚动稳定性、双侧同步和损坏候选容错后执行：

```text
cd frontend && npm run lint                       PASS
cd frontend && npm run typecheck                  PASS
cd frontend && npm run test                       PASS（26 tests）
cd frontend && npm run build                      PASS
go test ./...                                     PASS
go vet ./...                                      PASS
go test -race ./...                               未运行（CGO_ENABLED=0）
git diff --check                                  PASS
Wails v2.10.2 build（windows/amd64）               PASS
```

新增前端回归覆盖带缓冲的 48×20 区域在旧内容离开视口前开始预取、Region 请求
不改变全局按钮状态、连续滚轮增量按动画帧合并，以及左右任一侧作为输入源时两边
得到相同横纵位置。Go 回归覆盖损坏/伪 `.xlsx` 候选被跳过、有效索引缓存及重启
复用。本次没有实际操作 Wails/UGit 桌面 GUI；长表滚轮、触控板和拖动滚动条的
闪烁及同步手感仍需按 `docs/manual-acceptance.md` 目视验收，不能用自动化代替。

2026-08-01 修复 Windows“配置 UGit”对话框再次返回 `0x80070057` 后执行：

```text
go test ./...                                      PASS
go vet ./...                                       PASS
Windows 对话框定向回归（含 E_INVALIDARG 降级）    PASS
git diff --check                                   PASS
Wails v2.10.2 build（windows/amd64）               PASS
```

桌面产物已重新生成到 `build/bin/ugxlsx.exe`。本次没有实际点击桌面 GUI，也没有
修改用户真实全局 Git 配置；真实“点击配置 → 确认 → 重启 UGit”仍应按
`docs/manual-acceptance.md` 单独手工验收，不能用自动化与桌面构建代替。

2026-07-31 增加最近 10 个仓库切换弹窗后执行：

```text
GOCACHE=<独立可写缓存> go test ./...          PASS
GOCACHE=<独立可写缓存> go vet ./...           PASS
cd frontend && npm run lint                    PASS
cd frontend && npm run typecheck               PASS
cd frontend && npm run test                    PASS（25 tests）
cd frontend && npm run build                   PASS
git diff --check                               PASS
Wails v2.10.2 build（windows/amd64）            PASS
```

本次没有实际操作桌面 GUI；最近仓库顺序、去重、10 项上限、旧配置迁移、路径可用性、
弹窗打开/禁用项/直接切换及原“打开本地仓库”入口不变均由 Go/前端自动化覆盖。

2026-07-31 在第三轮交付提交 `6933f8a` 的源码状态执行：

```text
GOCACHE=/tmp/ugxlsx-go-cache go test ./...        PASS
GOCACHE=/tmp/ugxlsx-go-cache go vet ./...         PASS
GOCACHE=/tmp/ugxlsx-go-cache go test -race ./...  PASS
cd frontend && npm run lint                       PASS
cd frontend && npm run typecheck                  PASS
cd frontend && npm run test                       PASS（17 tests）
cd frontend && npm run build                      PASS
git diff --check                                  PASS
Wails v2.10.2 build（darwin/arm64）               PASS
```

同日现代化 UI 工作树执行：

```text
GOCACHE=/tmp/ugxlsx-go-cache go test ./...        PASS
cd frontend && npm run lint                       PASS
cd frontend && npm run typecheck                  PASS
cd frontend && npm run test                       PASS（19 tests）
cd frontend && npm run build                      PASS
git diff --check                                  PASS
Wails v2.10.2 build（darwin/arm64）               PASS
```

同日修复 UGit/Git 新增/删除文件的 null device 启动兼容及 difftool 自动只读后执行：

```text
GOCACHE=/tmp/ugxlsx-go-cache go test ./...        PASS
GOCACHE=/tmp/ugxlsx-go-cache go vet ./...         PASS
cd frontend && npm run lint                       PASS
cd frontend && npm run typecheck                  PASS
cd frontend && npm run test                       PASS（20 tests）
cd frontend && npm run build                      PASS
git diff --check                                  PASS
Wails v2.10.2 build（darwin/arm64）               PASS
Wails v2.10.2 cross-build（windows/amd64）        PASS
```

同日增加 UGit `LOCAL_TITLE` / `REMOTE_TITLE` 自动分支标签后再次执行：

```text
GOCACHE=/tmp/ugxlsx-go-cache go test ./...        PASS
GOCACHE=/tmp/ugxlsx-go-cache go vet ./...         PASS
cd frontend && npm run lint                       PASS
cd frontend && npm run typecheck                  PASS
cd frontend && npm run test                       PASS（20 tests）
cd frontend && npm run build                      PASS
git diff --check                                  PASS
Wails v2.10.2 build（darwin/arm64）               PASS
macOS ad-hoc deep sign + strict verify            PASS
Wails v2.10.2 cross-build（windows/amd64）        PASS
```

同日增加应用内“配置 UGit”及移动路径自动修复后执行：

```text
GOCACHE=/tmp/ugxlsx-go-cache go test ./...        PASS
GOCACHE=/tmp/ugxlsx-go-cache go vet ./...         PASS
GOCACHE=/tmp/ugxlsx-go-cache go test -race ./...  PASS
cd frontend && npm run lint                       PASS
cd frontend && npm run typecheck                  PASS
cd frontend && npm run test                       PASS（21 tests）
cd frontend && npm run build                      PASS
git diff --check                                  PASS
Wails v2.10.2 build（darwin/arm64）               PASS
macOS ad-hoc deep sign + strict verify            PASS
Wails v2.10.2 cross-build（windows/amd64）        PASS
```

同日替换 Wails 默认应用图标并重新交付两平台最小产物后执行：

```text
GOCACHE=/tmp/ugxlsx-go-cache go test ./...        PASS
GOCACHE=/tmp/ugxlsx-go-cache go vet ./...         PASS
GOCACHE=/tmp/ugxlsx-go-cache go test -race ./...  PASS
cd frontend && npm run lint                       PASS
cd frontend && npm run typecheck                  PASS
cd frontend && npm run test                       PASS（21 tests）
cd frontend && npm run build                      PASS
git diff --check                                  PASS
Wails v2.10.2 build（darwin/arm64）               PASS
macOS ad-hoc deep sign + strict verify            PASS
Wails v2.10.2 cross-build（windows/amd64）        PASS
```

macOS 实际启动新 `.app` 后确认业务窗口正常显示，并在 Finder 刷新后确认应用包
显示新的表格对比/合并图标而非默认 “W”；内嵌 ICNS 为 1024 px。Windows ICO
包含 16、32、48、64、128 和 256 px 六档，`.exe` 包含资源段并确认为 PE32+
GUI x86-64。Windows 11 仍未实机启动或检查任务栏图标。

同日修复空白单元格、保存/另存与索引重复建立问题后执行：

```text
GOCACHE=/tmp/ugxlsx-go-cache go test ./...        PASS
GOCACHE=/tmp/ugxlsx-go-cache go vet ./...         PASS
GOCACHE=/tmp/ugxlsx-go-cache go test -race ./...  PASS
cd frontend && npm run lint                       PASS
cd frontend && npm run typecheck                  PASS
cd frontend && npm run test                       PASS（23 tests）
cd frontend && npm run build                      PASS
git diff --check                                  PASS
Wails v2.10.2 build（darwin/arm64）               PASS
macOS ad-hoc deep sign + strict verify            PASS
Wails v2.10.2 cross-build（windows/amd64）        PASS
```

macOS 真实 GUI 使用系统临时目录中的左右工作簿验证：双击编辑 A1 后
`Command+S` 将状态从“有未保存修改”变为“已保存”，磁盘工作簿包含新值；
`Command+Shift+S` 正常打开原生另存为对话框，保存的副本可打开且包含同一新值。
样式-only 空白单元格复制、仓库模式导出副本保持 dirty、未变化刷新命中缓存及
保存后的当前文件增量索引已由 Go/前端自动化覆盖，但本轮没有在真实仓库 GUI
逐项操作；Windows 11 的 `Ctrl+S`、原生保存对话框和索引表现也仍待实机确认。

macOS 真实 GUI 使用独立的临时 `GIT_CONFIG_GLOBAL` 验证“配置 UGit”：确认框
显示预置旧路径和当前 `.app` 内部可执行文件路径；确认后只替换 XLSX 差异/合并
项，合并参数包含 `$MERGED`，`trustExitCode=false`，预置 CSV 工具保持不变。
再次点击识别为已正确配置且不重复写入。验收后关闭应用并删除临时配置，用户
真实 `~/.gitconfig` 未被修改。Windows 仍仅完成 macOS 上的交叉构建和 PE
x86-64 格式检查，尚未在 Win11 实机点击该按钮。

使用 UGit 日志中的真实命令和实际仓库重跑
`git difftool --tool=*.xlsx_Custom ... autoOpenPanel.xlsx`：Git 将缺失侧传为
`/dev/null`，修复前进程以 `unsupported_format` 在约 80ms 内退出；修复后
macOS 窗口正常加载并显示 687 处差异，两侧均标为 Git 只读快照，左侧网格权限
显示“只读”，复制合并、撤销、另存和保存按钮均禁用，界面未显示宿主临时目录；
关闭窗口后 difftool 正常结束。该验证没有保存或修改测试仓库。Windows 产物仅
完成 macOS 上的交叉构建和 PE x86-64 格式检查，尚未在 Windows 11 实机启动。

同日增加打开表格自动定位首个差异后执行：

```text
GOCACHE=/tmp/ugxlsx-go-cache go test ./...        PASS
cd frontend && npm run lint                       PASS
cd frontend && npm run typecheck                  PASS
cd frontend && npm run test                       PASS（24 tests）
cd frontend && npm run build                      PASS
git diff --check                                  PASS
Wails v2.10.2 build（darwin/arm64）               PASS
macOS ad-hoc deep sign + strict verify            PASS
Wails v2.10.2 cross-build（windows/amd64）        PASS
```

前端回归覆盖“摘要默认工作表无差异、后续工作表同时含修改和冲突”的场景，
确认自动切换到有差异的工作表并按默认优先级定位到冲突坐标 D120。macOS 真实
GUI 另用 `cmd/gentestdata` 生成的临时左右文件启动桌面产物，确认加载完成后差异
导航显示 `1 / 5`，状态栏和选中单元格定位到 A1。该次仅验收启动定位，没有重跑
编辑、保存、仓库索引和冲突处理全流程；Windows 仍只有交叉构建，未完成实机验收。

另用生产 CSS 和与应用一致的双网格 DOM 在真实浏览器滚动模型中验证：左右两侧
分别横向滚动 600px 后，各自行号层的屏幕横坐标保持不变；两侧分别纵向滚动
500px 后，列标题屏幕纵坐标保持不变。

桌面产物：

```text
build/bin/ugxlsx.app
build/bin/ugxlsx.app/Contents/MacOS/ugxlsx
build/bin/ugxlsx.exe
```

应用图标已从 Wails 默认 “W” 替换为独立的表格对比/合并图形：深蓝底、左右
表格、红绿差异和中心双向箭头。`build/appicon.svg` 是设计源，
`build/appicon.png` 和 `build/windows/icon.ico` 是构建输入；图标已检查
256/64/32 px 缩放。仍需在 Windows 11 实机确认资源管理器和任务栏的图标缓存
刷新与显示效果，不能用 macOS 上的交叉构建代替该项手工验收。

现代化改造后实际启动了 Wails 桌面产物，检查了仓库未选表格状态和已加载、无差异
双网格状态，并保存了前后截图。仍没有用真实 GUI 逐项重跑
`docs/manual-acceptance.md` 的全部仓库模式流程；
尤其“索引执行中关闭窗口”、冲突追加/覆盖后的双侧颜色及处理标记仍只有自动化
覆盖和桌面构建，没有真实应用手工确认。发布前必须补做，并与上述自动化、桌面
构建结果分开记录。

## 2026-08-01 UGit 工作区对比与合并语义修正

在 Windows 11 / UGit 5.51.0 的实际日志中确认，“使用差异工具与工作区对比”原先
执行的是单提交 `git difftool ... <sha> -- <file>`；当所选版本与工作区文件没有
Git 字节差异时，命令约 44ms 正常退出但不会调用外部工具。普通“使用差异工具
查看”传入两个提交，存在差异时可以正常启动，因此故障不在 Wails 启动本身。

自动配置现把 XLSX difftool 注册为 UGit 特殊识别的 `SpreadsheetCompare`。UGit
会自行准备两侧文件并写入 `SpreadsheetCompare-*.txt`；CLI 新增严格的两路径列表
入口并直接建立只读差异会话，不再依赖 `git difftool` 是否产生命令调用。合并工具
继续使用 `Custom`，但参数增加 `--base "$BASE"`，以便把 Git 文件级冲突与实际
三方表格语义分开说明。

用户截图对应的真实 mergetool 临时文件复核结果为：LOCAL↔REMOTE 共 4856 个
单元格差异、1 个删除行、1262 个修改行、0 个冲突行；BASE↔REMOTE 语义完全
相等，只有 LOCAL 相对 BASE 改变。Git 的 `UU` 是二进制 XLSX 文件级冲突，不等于
双方修改了同一表格记录。截图中 6415 行插入导致后续 ID 整体错位，旧物理坐标
比较把该位移放大为连续修改。

Git mergetool 会话现以左侧为逻辑坐标，按双方唯一、非空 `id` 对齐右侧记录；
空白或重复 ID 继续使用安全的坐标回退。Region、差异、复制和追加共享同一映射，
且右侧逻辑空行不会回退读取下一条原物理记录。共同基线分别判断 LOCAL / REMOTE
是否存在核心语义变化，界面显示固定的文件级冲突说明；橙色仍只表示满足既有
“同 ID 且其余实际数据列全部不同”规则的双方语义冲突，不会为迎合 Git 的 `UU`
状态而把所有差异染成橙色。

用同一组真实临时文件运行新会话探针后，1261 条错位同 ID 记录被对齐，结果从
4856 个单元格差异收敛为 6 个单元格差异、1 个删除行、1 个修改行、0 个冲突行；
合并说明正确判断 REMOTE 与 BASE 语义一致。该探针只读打开文件，没有保存。

自动化新增覆盖 UGit 路径列表协议、`--base` 元数据、错位唯一 ID 对齐、基线一侧
未变说明、对齐后的右侧复制，以及逻辑空行复制不会串行。前端新增 Git 合并来源
标识和文件级/语义冲突说明。Windows 真实 UGit 的日志和临时三方文件已用于只读
诊断，但尚未用新构建产物重新点击 UGit 菜单完成 GUI 手工验收；发布前按
`docs/manual-acceptance.md` 新增步骤补做。

同日修复 Git 合并说明引入的桌面布局回归：`.content` 原本只有“来源卡、摘要、
双表格、底部详情”四个子节点，新增的独立说明节点占用了第三条
`minmax(0, 1fr)` 弹性轨道，导致双表格被挤进 72px 固定轨道。现将结果摘要与说明
包装为 `comparison-summary-stack`，二者共同占用一条自动高度轨道，双表格恢复为
唯一弹性内容区；前端测试同时约束该包装层包含摘要和说明两个节点。

本次 UI 修复已使用当次 Windows Wails 产物和用户截图对应的 LOCAL / REMOTE /
BASE 临时文件完成真实桌面验收。固定 2400×1050 窗口的整窗截图确认：说明保持
单行紧凑，双表格显示正常数据高度，底部两行差异详情和状态栏均可见，差异仍为
6 个单元格、1 个删除行、1 个修改行、0 个冲突行；验收只读运行，没有生成输出
文件，窗口随后正常退出。为缩短后续验收，新增
`scripts/capture-wails-window.ps1`，把启动指定构建、按进程定位窗口、DPI 感知固定
尺寸、整窗截图、取消置顶和干净退出合并为一个命令。UI 实机验收已同步写入
`AGENTS.md` 和项目技能，作为所有后续 UI 交付的强制门禁。

本次自动化与交付验证：

```text
go test ./...                                      PASS
go vet ./...                                       PASS
frontend lint / typecheck / test（28 tests）/ build PASS
git diff --check                                   PASS
Wails v2.10.2 build（windows/amd64）                PASS
go test -race ./...                                未执行：本机 CGO_DISABLED 且无 C 编译器
```

## 2026-08-02 产品化包装

对当前代码、README、架构和桌面功能完成审计后，没有进行高风险目录重构：现有 Go
包边界、共享 Session、仓库安全约束、视口 Region API 和安全保存链路已经正式且可
维护。产品名确定为 SheetProof（表鉴），仓库和 Go 模块名继续保留 `ugxlsx` 以避免
破坏兼容性。桌面标题、CLI 版本信息、Wails 产物名和应用图标已更新；当前 Windows
产物为 `build/bin/SheetProof.exe`。

新增 `product/product.json` 与 `product/changelog.json` 作为产品事实源，由
`scripts/sync-product-content.mjs` 同步 README、CHANGELOG、CLI 版本和网站内容。
`site/` 新增首页、功能、指南、下载和更新日志五个路由；品牌 SVG 源文件和多尺寸
PNG/ICO/favicon 由 `scripts/generate-brand-assets.py` 生成。发布、部署和 UGit 接入
分别记录在 `docs/releasing.md`、`docs/deployment.md` 和
`docs/ugit-integration.md`。

Windows 当次 Wails 产物已真实打开生成的左右 XLSX：初始显示 7 个语义差异；在右侧
选择 A1，执行“复制单元格到左侧”并保存后，重新用 CLI 验证差异降为 6。两张整窗
截图均为 1600×1000（16:10），完整包含标题栏、工具栏、来源、差异导航、双网格、
底部详情和状态栏，存放于 `site/public/screenshots/`。这次没有重跑完整仓库模式或
UGit GUI 主流程，不能把直接文件验收扩大表述为全量手工验收。

官网通过 Sites 保存并维持仅限所有者访问的私有部署。加入前后对比说明与“只标出
改过格子”的文案后，已发布为版本 3，正式地址为
`https://sheetproof-app.kuyami.chatgpt.site/`。在用户明确切换到自有服务器之前，
后续每次网站改动都必须构建、测试并更新到该地址；只修改本地源码不算交付完成。
自有服务器尚未部署，因为没有 SSH、域名、目标目录和进程管理信息；所需信息及
验证方式见部署文档。

收到产品截图反馈后，新增 `cmd/genproductdemo` 生成角色成长、关卡掉落和技能参数
三张游戏配置表。产品截图不再使用“旧文本 / 新文本”等测试值；Windows 桌面端以
该示例实际打开 31 处差异，将右侧 C2 角色名称复制到左侧并保存后，CLI 复核差异
降为 30。`capture-wails-window.ps1` 新增客户区和客户区内指定区域捕获，三张官网
截图均不包含桌面、任务栏、其他窗口或圆角阴影中的下层内容。

官网截图改为整块可点击的大图查看器，支持键盘焦点、Esc/关闭按钮和移动端布局；
使用说明页以“打开、核对、合并保存”三个关键画面展示主流程。首页、功能、下载、
更新日志和页脚删除了测试、验证、事实来源及防伪式内部说明，未发布安装包统一显示
为“正在建设中”。内部路由自动检查与本地浏览器点击验收覆盖主导航和大图开关。

## 2026-08-02 左侧前后对比

Session 现在会在打开左侧工作簿时保留一份稀疏只读快照。Region 在现有视口范围内
同时返回当前左值和打开时左值，因此前端仍只拿 48×20 的窗口，不会为了回看功能
加载整本工作簿。编辑、合并、撤销和保存只更新当前左侧；打开快照在整个会话中
保持不变，保存后也不会悄悄变成新的基线。

左侧网格标题栏加入“前后对比 · 按住”。按住鼠标、Space 或 Enter 会显示打开时
状态，松开回到最新；左右任一网格被点击后，按住 Tab 也能回看。网格使用
`tabindex=-1` 接收点击后的焦点，不加入原有 Tab 遍历顺序；工具栏、搜索框与原位
编辑输入框都不会截获 Tab。回看期间只给相对打开快照发生语义变化的单元格加上
蓝色提示，没改过的格子维持普通底色，不把当前左右差异颜色硬套到打开时旧值上；
右侧对比来源保持不变。

产品事实源、README、CHANGELOG、官网功能页和使用说明已同步加入这项能力；文案
以“改完不放心时，按住看一眼”为使用场景，不写内部实现或测试口号。

当次 Windows 桌面验收使用新构建的 `build/bin/SheetProof.exe` 和重新生成的产品
演示表格完成：从差异索引选中 E2，把右侧 `1360` 复制到左侧后，按住按钮会把
左侧临时恢复为打开时的 `1280`，且只有 E2 显示蓝色提示，其余未改格子保持普通
底色；松开立即恢复 `1360`。分别在左右网格聚焦后按住、松开 Tab，预览均正确
进入并退出；工具栏按钮聚焦时按 Tab 未进入预览，原有焦点遍历不受影响。截图位于
`build/acceptance/before-after-v3/`。自动化验证包括 Go 全量测试与 vet、前端 lint、
typecheck、30 项测试与生产构建、官网构建和 3 项路由渲染测试、Windows Wails
v2.10.2 构建；竞态测试因本机 CGO 关闭且没有 GCC，仍不能执行。

## 2026-08-02 UGit 工作区差异可写

本机 UGit 5.51 源码确认 `SpreadsheetCompare` 的工作区比较协议：路径列表和版本
快照位于当前仓库实际 Git 目录的 `ugit/diff`，第一项是版本快照，第二项是工作树
中的真实文件；两个历史版本比较时两项都在 `ugit/diff`。此前 CLI 虽能用目录位置
把第二项标成“工作区”，仍固定按 UGit 顺序打开并设置 `ReadonlyLeft=true`，因此
工作区位于永远只读的右侧且整个会话被锁定。

CLI 现通过 `internal/repository.IdentifyWorktreeFile` 规范化候选工作区路径，并把其
`git rev-parse --absolute-git-dir` 与列表所属 `<git-dir>/ugit/diff` 做目录身份比较。
只有版本快照位于该目录、工作区候选属于同一仓库且是现有 `.xlsx` 时，才把真实
工作区交换到左侧并设置 `UGitWorktree`；普通仓库与独立 Git 目录的 linked worktree
都适用。两个快照、错误 Git 目录、缺失文件或任一归属不明情况继续进入原有
`GitDiff + ReadonlyLeft` 安全回退，不会仅凭列表顺序开放写入。

前端把该模式显示为“当前工作区 · 可编辑 / Git 版本快照 · 只读”，工作区显示真实
路径，右侧只显示快照文件名，保存按钮明确写“保存到当前工作区”。编辑、复制、
撤销和保存继续复用同一 Session 与安全写入器，不自动 add、commit、push 或改变
分支。Go 回归覆盖普通仓库、linked worktree、两快照和错误 Git 目录；前端回归覆盖
来源卡、路径隐藏、左侧编辑权限和保存文案。

当次 Windows Wails 产物以真实临时 Git 仓库及其
`.git/ugit/diff/SpreadsheetCompare-*.txt` 启动。整窗截图确认工作区位于左侧并显示
“当前工作区 · 可编辑”，右侧显示“Git 版本快照 · 只读”，快照目录未暴露，保存按钮
写明“保存到当前工作区”。实机双击左侧 C2 写入 `UGIT_EDIT_OK` 并按 `Ctrl+S` 后，
工作区 XLSX 的 SHA-256 从
`6BA7A8721028D140B95690D1E2FAB3DCBE1BA57B7F1A17EA9BFCDC572C83E3BC` 变为
`7609DE5D99BE1FF95ACB65F3D8E5B3F7F84517458846792AD03BA6F7E197072F`，保存值存在于
`xl/sharedStrings.xml`，Git 状态仍仅为未暂存的 modified。再用同一 Git 目录中的
两个版本快照启动，整窗截图确认两侧均显示“Git 差异快照 · 只读”，左侧提示不能
编辑或保存。截图位于 `build/acceptance/ugit-worktree/`，验收后窗口均正常退出。

本次验证：`go test ./...`、`go vet ./...`、前端 lint/typecheck/31 项测试/build、
`git diff --check` 和 Windows Wails v2.10.2 build 均通过；未执行竞态测试。

## 2026-08-02 差异行多选筛选

对比结果中的四类行指标现为多选筛选按钮。未选择任何项时继续使用普通 Region
显示完整表格；选择任意组合后使用压缩的 FilteredRegion，只在左右网格显示匹配
语义行。筛选只在当前运行会话内保留，数字键 1–4 对应新增、删除、修改、冲突，5 在四类
全选和全部不选间切换。左侧差异索引仍保持单分类导航；点击其分类页签会同步把
行筛选切成对应单类，避免导航到已隐藏行。

当次 Windows 桌面验收使用一张同时包含 1 个新增、1 个删除、1 个修改和 1 个冲突
行的生成工作簿。未筛选时完整数据可见；点击“新增+修改”后，左右只保留真实物理
行 2 和 4，新增行左侧仍显示配对空行。实际聚焦网格后依次按 4、1、3、2、4，
分别确认从冲突单类回到完整表格、只开新增、增加修改、增加删除、再增加冲突；
另用 5 验证全选/全不选；重启应用后应恢复为全部不选并显示完整数据。

随后点击左侧冲突索引页签，确认顶部同步为冲突单类并只显示来源物理行 5；从右侧
冲突行菜单实际执行“将整行新增为 id:7 到左侧”，确认同一筛选行左侧改为真实目标
物理行 7、显示 ID 7 和新增绿色，右侧仍显示来源物理行 5 及处理标记，追加目标未
以删除行重复出现。本次追加未保存，强制结束验收进程后 CLI 复核测试工作簿仍为
原 9 处差异。截图位于 `build/acceptance/row-filter/`。

本次验证：`go test ./...`、`go vet ./...`、前端 lint/typecheck/32 项测试/build、
`git diff --check` 和 Windows Wails v2.10.2 build 均通过；未执行竞态测试。

### 当前工作表计数、非整数 ID 冲突追加与筛选定位修正

后续反馈修正了筛选选择持久化和摘要歧义：每次启动固定为全部不选，对比结果的
单元格差异与四类行数只读取当前工作表。Windows 实机在“属性 / ID冲突”两张表间
切换后，摘要分别显示“2 处差异、修改行 1、冲突行 0”和“4 处差异、修改行 0、
冲突行 2”，没有继续显示工作簿总数。

“属性”表虽以 `id` 为首列表头，但下面实际存放文本属性名，因此不再把该列误判为
可自增整数 ID。纯冲突行右键显示“将整行新增到左侧”，执行后右侧第 2 行连同文本
ID 原样追加为左侧第 4 行，标记为“已新增到左侧末尾”；普通修改行不显示该操作。
来源 C2 使用故意写入的未知厂商填充模式时，追加仍应成功，状态栏只提示忽略该填充。

长表定位验收另使用唯一差异位于原始第 95 行的 120 行工作簿。开启修改行筛选后
压缩视图只显示物理第 95 行，取消后完整表格仍定位在第 95 行附近，没有跳回表头；
截图位于同目录 `06`–`08`。自动化新增覆盖第 95 行筛选开关的双侧 `scrollTop`
恢复、非整数 ID 冲突行末尾追加/撤销/过滤配对，以及异常填充后的后续复制。

本次最终验证：`go test ./...`、`go vet ./...`、前端 lint/typecheck/32 项测试/build、
官网 build/3 项路由测试和 Windows Wails v2.10.2 build 均通过；竞态测试在
`CGO_ENABLED=1` 后因本机缺少 GCC 无法构建，未记为通过。

官网内容已在上述验证通过后同步并重新发布。Sites 生产版本为 v4，来源提交
`5ab53515db51c2262c6fff824f2cad02b6fdff85`，部署
`appgdep_6a6f01ea95b8819195519b54920f06c8` 于 2026-08-02 成功，生产地址仍为
`https://sheetproof-app.kuyami.chatgpt.site/`。

### 文本 ID 冲突与普通修改行右键菜单修正

后续实表反馈确认，仅凭首行存在 `id` 就把整列当作可自增整数 ID 会误判属性定义表：
该类表在 `id` 列实际存放“活动id / 关卡名字”等文本字段名。当前差异摘要仍保留
`IDColumn` 供冲突分类使用，但只有左右现有非空 ID 都可解析为整数时才返回正数
`NextID` 并开放自动/指定 ID 追加。文本 ID 冲突使用无 ID 覆盖的原样整行追加，
不会把来源 ID 替换成 `1~N`。前端同时把“将整行新增到左侧”严格限制到纯冲突
选区；增加、删除和修改行只显示复制操作，Session 后端也会拒绝非冲突来源行的
追加请求。

自动化新增差异层、Session 与前端回归，覆盖文本 ID 仍分类为冲突但 `NextID=0`、
原样追加保留文本 ID，以及普通修改行不显示追加项。`go test ./...`、`go vet ./...`、
前端 lint/typecheck/33 项测试/build 和 `git diff --check` 通过；`go test -race ./...`
仍因本机没有 `gcc` 无法构建。Windows Wails v2.10.2 使用已经单独验证的生产前端
执行 `build -s` 后成功生成 `build/bin/SheetProof.exe`；完整构建入口在前端阶段因
本机 `C:\nodejs\npm.cmd` ACL 拒绝访问失败，不代表 Wails 或模块缓存缺失。

当次 Windows 桌面产物分别打开文本 ID 冲突与普通修改工作簿并实际右键。文本 ID
冲突菜单只显示覆盖、覆盖整行和“将整行新增到左侧”，实际追加后左侧新增行保留
“活动id”且右侧显示“已新增到左侧末尾”；普通修改行菜单只显示复制单元格和复制
整行。验收截图位于 `build/acceptance/menu-id-fix/`，窗口随后正常关闭。

## 2026-08-02 共享字符串与内联字符串语义归一

实表反馈中的 C3 左右可见值同为“巴伯洛皮肤挑战（一般）”，但底部类型分别显示
`string` 与 `inline-string`。直接检查用户提供的 XLSX 包确认：左侧 C3 是共享字符串
`<c r="C3" s="5" t="s"><v>42</v></c>`，右侧 C3 是内联字符串
`<c r="C3" s="5" t="inlineStr"><is>...</is></c>`；解码文本和样式编号都相同。
两者只是 OOXML 文本序列化方式，不是业务单元格类型差异。此前 Reader 把它们分类
为不同 `type`，而 `CellValue.Equal` 又包含 `type`，因此整表字符串存储方式变化被
放大为大量修改。

`workbook.ClassifyCellType` 现把 `CellTypeSharedString` 和 `CellTypeInlineString` 都归一
为 `string`。`present + raw + formula + type` 相等规则本身不变；数字与文本、公式、
布尔值、日期和显式空字符串等真实语义差异仍保留。回归测试使用 excelize 普通写入
和 StreamWriter 生成真实的共享/内联字符串 OOXML，先断言底层存储类型确实不同，
再断言 Reader 类型均为 `string` 且整本比较相等。

用反馈中的真实 LOCAL/REMOTE 文件只读复核：修复前截图显示整本 12,328 处差异，
其中 `activityCard` 12,181、`属性` 139、`语言版本_tw` 8；修复后 CLI 与桌面均显示
整本 1,029 处差异，其中 `activityCard` 1,018、`属性` 11、`语言版本_tw` 0。剩余结果
对应 111 个删除行、26 个修改行等真实变化。新 Windows Wails 产物实际启动后检查了
总览和零差异工作表；后者明确显示“当前工作表内容一致”，左右文本和类型均为
`string`，没有红绿高亮。截图位于
`build/acceptance/string-storage-normalization/01-overview.png` 和
`04-equal-sheet.png`，验收只读运行、未保存文件，窗口均正常退出。

本次验证：

```text
go test ./...                                      PASS
go vet ./...                                       PASS
frontend lint / typecheck / test（33 tests）/ build PASS
git diff --check                                   PASS
Wails v2.10.2 build -s（windows/amd64）             PASS
go test -race ./...                                未通过：启用 CGO 后本机缺少 gcc
```

规定的完整 Wails 构建入口已先执行并确认本地 Wails 2.10.2 与模块缓存完整，但在重复
编译前端时因本机 `C:\nodejs\npm.cmd` ACL 拒绝访问而失败；前端已使用项目现有依赖
单独完成四项检查和生产构建，随后同一离线优先脚本以 `build -s` 成功生成
`build/bin/SheetProof.exe`。这不是 Wails 或依赖缓存缺失。

## 2026-08-02 官网自托管与发行自动化

官网正式地址切换为 `https://sheetproof.luyilabs.com/`。`site/` 使用静态导出，生产
文件由 Caddy 从 `/var/www/sheetproof.luyilabs.com` 直接提供，不需要 Node.js 进程。
部署脚本要求执行者在本机传入 SSH 目标，不在公开仓库保存服务器 IP、SSH 用户名或
别名、本机用户路径、本机认证配置、密钥、证书私钥或令牌。

官网硬编码文案和产品事实源改为直接说明实际能力；更新日志只保留用户可感知的功能
与修复，补入共享/内联字符串归一、差异表容错、筛选位置、Git/UGit 对齐、文本 ID
冲突追加、前后对比和 Windows 后台命令等近期修复，不再把品牌、截图、测试或发布
安排写成产品更新。

新增 `.github/workflows/release.yml`：手动运行只生成 Actions artifacts；推送与产品
版本一致的 `v*` 标签时，生成 Windows amd64、macOS universal 和 SHA-256 校验文件，
并创建 Draft Release。产物当前没有代码签名或 macOS 公证，后续凭据只能存入 GitHub
Actions Secrets。

项目规则、开发技能、架构、发布、部署和手工验收文档已同步。以后每次用户可见的
功能迭代或修复都必须同步官网内容，完成静态构建和测试，并发布到正式域名；旧 Sites
地址不再是正式发布目标。

本次静态站点已实际发布。Caddy 候选配置和安装后的完整配置均通过校验，服务 reload
成功；源站与 Cloudflare 公网入口的首页、功能、指南、下载、更新日志、favicon 和
Open Graph 图片均返回 200，公网响应确认经过 Cloudflare，既有虚拟主机复核正常。
Caddy 已正常提供源站 HTTPS，因此没有配置额外的 Origin Certificate。

正式版本发布流程进一步固化到项目技能：收到“发布”“发布正式版本”或“推送
vX.Y.Z”后，先以上一个公开 Release 为基线整理用户可见差异；未指定版本时按 SemVer
影响范围推导版本号。产品事实、更新日志、README、CHANGELOG、版本文件、发布文档和
官网在准备阶段同步并完成验证，但在维护者确认前不执行 commit、push、标签、Release
发布或生产官网部署。确认后先以手动 workflow 验证双平台构建，再推送标签、检查并
发布 Draft Release，最后自动部署 Lightsail 官网并核验源站、Cloudflare 和既有站点。
正式发布候选汇总和最终报告还必须给出隐私扫描结论；若发现敏感数据，仅报告位置、
类别、风险与处理/轮换状态，不复述敏感值，处理完成前停止 push。任何公开历史重写都
需要维护者另行明确授权。

## 2026-08-02 v0.1.0 发布候选准备

GitHub 只读核验确认仓库尚无公开 Release 和标签，`v0.1.0` 因此是有效的首次公开
预览版本。候选已把 Windows amd64、macOS universal、源码归档和 Releases 地址同步到
产品事实、README 与官网；下载页明确说明当前桌面产物未签名，macOS 版本未公证，并
要求核对随 Release 提供的 `SHA256SUMS.txt`。收到维护者正式确认前，这些稳定 URL
保持为可预测的候选地址，官网不部署到生产环境。

候选自动化结果：Go 全量测试与 vet、前端 lint/typecheck/33 项测试/build、官网 lint、
静态导出和 4 项渲染测试均通过；官网 lint 保留 4 条既有 `<img>` 优化警告但没有错误。
`go test -race ./...` 在启用 CGO 后仍因本机缺少 GCC 无法构建，未记为通过。Windows
使用规定的离线优先脚本完成 Wails v2.10.2 正式构建，本地验收产物为
`build/bin/SheetProof.exe`。

桌面实机验收使用当次构建和代表性双文件夹具完成。默认状态截图确认来源卡、差异摘要、
双网格、语义颜色、底部两行详情和状态栏均完整；实际点击无结果的“删除行”筛选后，
左右网格正确显示成对空状态且布局未重叠。截图位于
`build/acceptance/release-v0.1.0/`，验收进程均正常退出。两个既有真实 Git mergetool
会话保持不动，本次验收使用同一构建的临时命名副本隔离 WebView2 数据目录。

官网下载页的真实浏览器检查确认 DOM 只有一个主内容区、一个标题和四张下载卡；
Windows/macOS 资产地址、源码地址、SHA-256 提示、签名限制和页脚均正常。隐私扫描未
发现私钥标记、凭据赋值或敏感文件名；当前树中的真实本机用户路径已从本文移除，测试
代码只保留合成路径夹具。既有公开 Git 历史仍包含低风险本机路径记录，未发现对应凭据，
无需轮换；本次不重写历史。当前仍没有 `LICENSE` 文件，维护者选择并加入明确许可证前，
不得确认公开发布。候选尚未 commit、push、打标签、运行远端发布工作流或部署官网。

## 2026-08-02 仓库、模块与 MIT 许可迁移

当前源码把正式仓库地址统一为 `https://github.com/tadazly/sheetproof`，Go module 统一为
`github.com/tadazly/sheetproof`，桌面与 CLI 的新安装标识统一使用 `SheetProof` / `sheetproof`。
仓库根目录加入标准 MIT License，版权主体为 `Copyright (c) 2026 tadazly`；产品事实源、
README、CHANGELOG、官网内容与下载地址由同步脚本保持一致。

这次改名保留现有用户数据兼容：WebView 仓库搜索历史和工作表布局首次读取旧 `ugxlsx`
key 时复制到新的 `sheetproof` key，后续只写新 key；系统配置目录从
`ugxlsx/preferences.json` 迁移到 `SheetProof/preferences.json`，仅在新文件不存在时复制
完整旧偏好且不删除旧文件。UGit 配置继续枚举全部 `*.xlsx` 工具项，因此能识别旧版
`ugxlsx` 可执行文件路径，用户确认后只替换 XLSX 项并保留其他后缀配置。`legacyName`、
兼容常量、迁移测试和此前真实交付记录中的旧名称继续保留。

本次验证通过 `go test ./...`、`go vet ./...`、前端 lint/typecheck/34 项测试/build，
以及官网 lint/5 项渲染测试。Windows Wails v2.10.2 使用隔离输出名成功完成完整构建；
随后实际启动该产物进入仓库模式，核对品牌、仓库树与搜索面板布局，并验证搜索历史在
退出重启后仍可恢复。截图保存在 `build/acceptance/repository-rename/`，验收进程均正常
退出。默认 `build/bin/SheetProof.exe` 当时被既有用户进程占用，因此没有覆盖该文件。

GitHub 连接只读检查确认当前账号对 `tadazly/ugxlsx` 具有管理员权限，但本机没有可用的
GitHub CLI，连接器也不提供仓库改名写操作。因此远端仓库尚未原地改名，`origin` 暂时仍
指向旧地址，仓库描述、主页与在线许可证识别也尚未更新；不得提前把 `origin` 指向尚不
存在的新仓库。维护者安装并登录 `gh` 后，应按本节对应迁移清单完成远端改名和元数据
更新。官网源码与生成内容已经验证，本轮未获得 Lightsail SSH 目标，因此尚未部署生产站。

## 2026-08-02 官网移动导航与目标用户定位

官网在 960px 以下不再隐藏全部导航，而是显示 44×44px 汉堡按钮和右侧抽屉。抽屉
包含功能、使用说明、下载、更新日志与 GitHub，当前路由显示选中状态；打开时锁定
页面滚动、自动把焦点移入菜单并形成完整焦点循环，支持 Escape、遮罩和关闭按钮退出，
关闭后恢复触发按钮焦点。桌面导航同时补充当前页状态。

首页首屏改为“面向 Git 工作流的 XLSX 差异审阅 / 让配置表变更也能逐格审阅”，
明确服务于游戏开发和数据团队管理的配置表、数值表与运营表，不再暗示泛用 Excel
格式、图表或宏审阅能力。首屏信任标签改为“文件留在本机 / 理解 XLSX 语义 /
为 Git 工作流设计”，UGit 下沉为独立集成模块。官网所有 GitHub 入口统一读取产品
事实源中的 `https://github.com/tadazly/sheetproof`，页脚补充
`Copyright (c) 2026 tadazly`。

本次内容同步、官网 lint、静态导出与 7 项渲染测试通过；lint 只有 4 条既有 `<img>`
优化警告，没有错误。真实浏览器分别以 390×844 手机、820×900 平板和 1280×900
桌面视口验收：移动端按钮实测 44×44px，抽屉布局、五个入口、当前页选中、滚动
锁定、首尾焦点循环、Escape/遮罩关闭、焦点恢复和内部页面跳转均正常；桌面导航与
移动按钮按断点正确互斥，三个视口均没有页面横向溢出。静态导出使用尾斜杠路由，
当前页判断已在比较前规范化路径，并有回归约束。

同日根据官网反馈，将使用说明页“常用操作”中的“差异行筛选”右侧文案从较长的
“1–4 切换分类，5 切换全部 / 不筛选”精简为“1–4 切换分类，5 全选”。桌面端实际
快捷键和筛选状态机未改变；官网渲染测试约束新文案存在且旧文案不再输出。

随后修复官网在手机缩放和极窄有效视口下的横向溢出。根因是使用说明页模式卡片
中的命令行形成 Grid 子项的固有最小宽度，使卡片在 390px 手机视口仍保持 454px。
模式 Grid 和卡片现在显式使用可收缩轨道与 `min-width: 0`，命令在卡片内换行；
其他指南文案、快捷键和 UGit 配置卡片也允许长文本安全折行，360px 以下进一步收紧
页边距、字号、卡片内边距和页脚列数。

使用说明新增 `#ugit` 配置章节，说明固定应用位置、在顶部点击“配置 UGit”、确认
只替换 `*.xlsx` 的差异与合并工具、重新打开 UGit，以及差异工具中同时出现
`SpreadsheetCompare` 与 `Custom`、合并工具中出现一条 `Custom` 属于正常状态。
首页 UGit 模块使用 `/guide/#ugit` 文档导航，可直接定位到配置步骤。官网更新日志和
渲染测试同步覆盖这些内容。

本次官网 lint 通过（仍只有 4 条既有 `<img>` 优化警告），静态导出和 9 项渲染测试
通过。Chrome 对本次静态产物分别以 390、320、280 和 240 像素宽检查首页与使用说明，
页面宽度均未超过有效视口，越界元素为零；两条命令的 `scrollWidth` 与卡片内可用宽度
一致。截图元素补充固有宽高，避免加载时布局位移；从首页进入 UGit 文档后最终停在
96px 的预留顶部位置。验收截图保存在
`build/acceptance/site-mobile-responsive/ugit-320.png` 和
`build/acceptance/site-mobile-responsive/guide-code-240.png`。

## 2026-08-02 v0.1.0 发布候选复核与产物隐私修正

GitHub 只读复核确认公开仓库默认分支为 `main`，仍没有标签或 Release，因此
`v0.1.0` 是有效的首次公开预览版本。当前 `main` 与 `origin/main` 均指向
`27ac89f`；发布准备开始时工作树没有内容差异。

隐私复核发现，既有官网与 README 产品截图把真实本机工作区路径显示在来源卡中；
本次已用当前真实 Windows 桌面产物、生成式产品演示工作簿和通用临时演示根目录
重新执行筛选、选择、复制与保存流程，并替换三张公开截图。新截图没有用户名、账号、
凭据或真实工作区路径。对应旧截图已存在于公开 Git 历史，风险限于低敏感度的本机
目录信息，未发现相关凭据且无需轮换；本次不改写公开历史。

本地候选 EXE 的字符串扫描还发现编译时用户目录片段。Windows 离线优先入口和 GitHub
Release workflow 现统一为 Go 构建启用 `-trimpath`，且保留调用者已有的其他
`GOFLAGS`。重新构建后的 Windows amd64 EXE 不再包含用户目录或工作区路径，也未发现
私钥标记或常见令牌格式；当前产物仍未签名。发布文档、架构、手工验收和项目技能均已
同步这一门禁。

本轮候选验证通过 Go 全量测试与 vet、前端 lint/typecheck/34 项测试/build、官网 lint、
静态导出与 9 项渲染测试，以及 Windows Wails v2.10.2 完整构建。官网 lint 仍只有 4 条
既有 `<img>` 优化警告。`go test -race ./...` 因当前 Windows 环境 `CGO_ENABLED=0` 且
没有 GCC 仍无法执行，未记为通过。锁定依赖安装对官网完整开发依赖树报告安全审计项；
生产部署是静态导出且不运行 Node，离线生产依赖检查未报告命中，但在线审计详情因不得
向外部发送完整依赖元数据而未继续查询，发布候选汇总必须保留这一限制。

启用 `-trimpath` 的最终本地 Windows 产物已再次真实启动，实际切换修改行筛选并目视
核对来源卡、摘要、双网格、差异颜色、底部详情和状态栏，截图保存在
`build/acceptance/release-v0.1.0/trimpath-build.png`，进程正常退出。候选仍未 commit、
push、打标签、运行远端发布 workflow、发布 GitHub Release 或部署生产官网，必须先
取得维护者对正式发布 `v0.1.0` 的明确确认。

## 后续风险与建议

- 下一轮开始时先确认 `git status --short` 包含本轮现代化 UI、生产前端和文档
  修改；若出现其他改动也必须先查明并保留。确认是否需要由用户另行提交这些
  成果或推送当前领先 `origin/main` 的提交；没有明确要求时不要 commit 或 push。
- 优先补做 `docs/manual-acceptance.md` 的 macOS 真实 GUI 仓库模式主流程，
  特别关注索引期间关闭是否即时、目录收起状态、左右滚动 sticky 图层、四类
  配色及冲突处理后的标记与撤销。
- `frontend/src/App.vue` 已包含较多状态机和 UI 逻辑；继续扩展前可评估拆分
  composable/组件，但拆分本身不能改变请求防乱序和选择语义。
- 前端差异索引当前一次最多读取 10,000 条。
- 内联编辑不是 Excel 级编辑器，不支持范围批量输入、布尔/日期专用输入控件、
  完整公式辅助或格式工具栏。
- 仓库刷新不会 fetch；分支列表只反映本地已有引用。
- Windows 仍需实机完整验证目录拖放、原生对话框、下载目录和 Wails 打包行为。
- excelize 对图片、图表、透视表、外部连接、复杂条件格式和厂商扩展不承诺
  完全保真。

## 新会话启动顺序

```bash
pwd
sed -n '1,260p' AGENTS.md
sed -n '1,360p' docs/iteration-2-handoff.md
git status --short
GOCACHE=/tmp/sheetproof-go-cache go test ./...
cd frontend && npm run test && npm run typecheck
```

可直接复制的接手提示词见 `docs/new-session-prompt.md`。

## 2026-08-03：仓库拖入与外部文件修改处理

- 修复 Wails 开启原生目录拖放但未安装前端 DOM 拖放处理器的问题；处理器在 Vue 挂载前
  阻止 WebView 默认导航并把路径交给 Go，首次把仓库目录拖入窗口不再跳转到空白页面。
  Windows 保持 `DisableWebViewDrop=false`，否则 WebView2 会连外部文件拖入也一并拒绝。
- 主界面顶部“打开本地仓库”改为应用内弹窗，同时提供目录拖入区和原生目录选择器；
  欢迎页入口继续保留直接手动选择，窗口全局仍可接收仓库目录拖入。
- 目录拖入在后台打开前发出开始事件，手动选择也进入相同的弹窗加载状态；较大仓库处理期间
  弹窗显示明确进度并禁用重复提交/关闭，成功接收结果后才关闭。导入失败时弹窗保持打开，
  错误在标题下方换行并可滚动查看，不再被高层级遮罩挡住。
- `Session.ExternalChanges` 以加载身份检查左右源文件，`ReloadLeft` 和 `ReloadRight`
  负责重新打开工作簿并重建差异状态；仓库左侧重载后同步刷新 Git 状态和语义索引。
- 前端在窗口可见时轮询，并在窗口重新获得焦点时检查。右侧和只读左侧提示后自动重载；
  可编辑左侧先询问是否载入磁盘最新版，并明确说明会放弃 SheetProof 中尚未保存的修改。
  用户暂不重载时保留警告和显式重载入口，且不再重复散列同一份已知外部修改。
- Go 覆盖拖放配置、拖入开始通知、左右变化检测、只读判定和重载；前端覆盖仓库打开弹窗、导入
  加载/错误、可编辑
  外部修改确认/暂缓/重载，以及只读右侧自动重载。

## 2026-08-03：表格全选与键盘清空

- 左右网格聚焦后支持 `Ctrl/Command+A` 选择当前工作表的全部实际单元格；开启差异行筛选时，
  全选范围限定为当前压缩行集。搜索框和原位编辑框仍使用浏览器原生文本全选。
- 只有可编辑左侧网格响应 Backspace/Delete。单格或范围清空复用 Session、merge 和 history，
  只删除值/公式并保留样式等元数据，整批清空作为一个撤销步骤；右侧和只读左侧不写入。
- 前端只提交选区边界或筛选后的左侧物理行；Go 后端在既有稀疏快照中筛选实际存在的单元格，
  没有为了全选或清空加载整张工作表。
- 自动化验证已通过 `go test ./...`、`go vet ./...`、前端 lint/typecheck/test/build，
  以及官网 lint/build/渲染测试。`go test -race ./...` 因当前 Windows 环境没有 GCC 无法运行，
  不是测试用例失败。
- 使用当次 `scripts/invoke-wails.ps1 build -s` 生成的 `build/bin/SheetProof.exe` 完成 Windows
  实机验收：`Ctrl+A` 选中整张测试表的 20 个单元格；Delete 清空整批后一次 `Ctrl+Z`
  全部恢复；Backspace 清空单格后可撤销；右侧聚焦后连续按 Delete 和 Backspace 内容不变。
  截图保存在 `build/acceptance/keyboard-shortcuts/`，验收结束后已关闭应用与临时本地服务器。

## 2026-08-03 至 2026-08-04：v0.2.0 正式发布

- 上一个公开版本为预览版 `v0.1.0`。本轮保持文件直开与仓库模式兼容，同时新增仓库目录拖入、
  应用内打开进度与错误反馈、外部文件变化检测/安全重载，以及全选和可撤销批量清空，因此按
  向后兼容的新功能升级推导为次版本 `v0.2.0`，而不是补丁版本。
- `product/product.json`、CLI 版本常量、前端与官网 package 清单及锁文件均已同步为 `0.2.0`；
  README、`CHANGELOG.md`、产品 JSON 和官网更新日志已经新增并同步本轮四类用户可见变化。
  Windows、macOS 与源码下载链接均指向待发布的 `v0.2.0` Release。
- 自动化验证通过 Go 全量测试与 `go vet`、前端 lint/typecheck/40 项测试/build、官网 lint/build
  与 9 项渲染测试；官网 lint 仍只有 4 条既有 `<img>` 优化警告。`go test -race ./...` 在
  `CGO_ENABLED=1` 下因当前 Windows 环境没有 GCC 无法执行，未记为通过。
- 已通过标准离线优先入口 `scripts/invoke-wails.ps1 build` 完成 Wails v2.10.2 Windows 全量构建。
  当前 `build/bin/SheetProof.exe version` 返回 `SheetProof 0.2.0`；本地候选 EXE 未签名，macOS
  发布流程也仍未配置签名/公证。
- 已用当次全量构建产物完成真实 Windows GUI 验收：Explorer 目录拖入后正确打开仓库且未出现
  空白 WebView；应用内仓库弹窗布局正常；可编辑左侧外部变化出现重载确认；只读右侧外部变化
  自动重载；Ctrl+A、Delete、Backspace、一次撤销和右侧只读保护均符合预期。证据保存在
  `build/acceptance/release-v0.2.0/`，所有应用、Explorer 和辅助进程均已关闭。
- 隐私扫描覆盖候选差异、更新日志、官网静态产物和 Windows EXE，未发现凭据、私钥、带认证的
  URL、真实用户目录、工作区路径、远程账号或服务器身份。前端压缩包唯一的 IPv4 形态命中来自
  既有 SVG 图标的数字序列，不是网络地址。`v0.1.0..HEAD` 没有新增提交；旧公开历史中已记录的
  低敏感度本机目录截图问题已在上一版本准备阶段替换，无凭据需要轮换，本次不重写历史。
- 维护者明确确认后，发布提交 `3315e65` 已无强推地推送到 `origin/main`。提交级 Release workflow
  的源码验证、Windows amd64 和 macOS universal 构建全部成功后，才创建并推送 annotated tag
  `v0.2.0`；标签级 workflow 再次完成相同验证、双平台构建、校验和与 Draft Release 创建。
- GitHub Release 已作为预览版公开在 `https://github.com/tadazly/sheetproof/releases/tag/v0.2.0`。
  `SheetProof-windows-amd64.exe`、`SheetProof-macos-universal.zip` 和 `SHA256SUMS.txt` 三项资产均可
  公网下载，校验文件中的两条 SHA-256 与实际资产一致。Windows 产物仍未签名，macOS 产物仍未
  签名或公证，发布说明明确保留这些限制。
- 发布阶段发现首次本地提交由 Git 自动带入一项新的本机账户身份；在任何 push 之前已把 author
  和 committer 修正为仓库既有公开身份，未形成远端暴露，也没有凭据需要轮换。最终候选差异、
  更新日志、官网静态产物以及正式 Windows/macOS 二进制均未发现凭据、私钥、真实用户路径、
  工作区路径、远程账号或服务器身份。
- 2026-08-04 通过 `scripts/deploy-site-lightsail.ps1` 将静态官网部署到正式 Lightsail/Caddy 目标。
  远端 Caddy 配置验证通过，源站下载页确认显示 `v0.2.0`；同一 Caddy 实例识别到的 5 个站点
  全部健康。Cloudflare 公网首页、功能、使用说明、下载和更新日志均返回 200 并显示当前版本，
  favicon、SVG/PNG 图标和 Open Graph 图片也完成实际下载验证。部署脚本保留了上一站点目录用于回退。

## 2026-08-04：滚动条差异位置与大表视口性能

- 左右网格的纵向、横向滚动条轨道按新增、删除、修改、冲突的现有颜色显示差异位置；
  开启行筛选后纵向位置使用压缩显示行，横向位置按实际列宽计算。
- 标记被聚合到最多 180 个视觉区间，只在工作表、差异、筛选、缩放或已提交列宽变化时
  重建，不绑定滚动事件，也不为每个差异创建 DOM，避免 10,000 条差异放大绘制成本。
- Region 状态读取改为对已排序差异做当前视口范围二分查找，不再在每次 48×20 请求时
  遍历整张表的全部差异。FilteredRegion 只分配当前窗口的行映射；前后端冲突追加映射
  都先按来源预索引，移除随差异行和处理记录数量形成的嵌套扫描。
- 应用不可见时暂停外部文件定时检查，重新可见或获得焦点后仍立即补查。
- 性能审查未发现前端定时器、全局事件或 Wails 事件订阅的确定性泄漏；它们都在组件
  卸载时清理。`Session` 为“前后对比”保留完整稀疏打开快照，取消中的差异比较也可能
  短暂继续占用内存，这两项属于需要基准与上下文取消设计后再动的中风险点，不能为降低
  内存而破坏回看或仓库索引关闭语义。
- 自动化测试和标准 Wails v2.10.2 Windows 全量构建已通过。首次实机验收时，当前机器的
  WebView2 页面持续以 `STATUS_ACCESS_VIOLATION` 崩溃；移除滚动条轨道渐变、移除标记样式
  绑定以及切换 `native_webview2loader` 的隔离对照均未改变结果，并且同机其他 WebView2
  应用随后也出现相同错误，确认并非本功能绘制路径导致。确认没有 SheetProof 进程残留后
  停止 GUI，重启 Windows 使 WebView2/GPU 环境恢复；隔离修改均已撤销。
- 重启后用最终完整构建实际打开产品演示工作簿：默认视图左右网格的横纵滚动条均显示
  绿色、红色、黄色等现有语义色标记，点击“修改”筛选后网格压缩为 4 行，纵向标记按显示
  行重新分布，横向标记继续按实际列宽分布，布局和交互正常。截图保存在
  `build/acceptance/scrollbar-markers/01-overview-after-restart.png` 和
  `build/acceptance/scrollbar-markers/04-modified-filter-click.png`；每次进程均正常关闭。

## 2026-08-04：普通对比的唯一 ID 行对齐

- 直接打开左右文件、仓库模式、UGit/Git 差异工具和 CLI `diff` 现在复用同一套安全
  行对齐：双方首行存在同位置 `id` 列时，两侧各出现一次的非空 ID 按记录对齐；同表
  其他空白、重复 ID 或辅助行只保守处理自身，不再使整张表回退。只有缺少同位置表头或
  全表没有任何可靠唯一 ID 时才整表按物理行号比较。
- 新增 ID 插入到中间时不再把后续记录放大为连续修改；右侧独有 ID 作为新增行追加到
  对齐视图尾部。会话摘要显示对齐记录数，并允许在产生编辑/合并撤销历史前切换“ID 对齐”
  和“按行号”，便于检查原始物理布局。Git 合并模式继续使用原有宽松 ID 对齐。
- 冲突谓词没有改变：仍要求匹配到同一 ID，且除 ID 外的全部实际数据列都不同。新增测试
  同时覆盖“中间插入 + 后续相同记录 + 真正冲突”，结果为新增行 1、修改行 0、冲突行 1；
  回退按行号后为修改行 2、冲突行 0。复制对齐后的新增行仍读取正确的右侧物理来源，且可撤销。
- 自动化验证通过 `go test ./...`、`go vet ./...`、前端 lint/typecheck/44 项测试/build，
  以及官网 lint/build/10 项渲染测试；官网 lint 仍只有 4 条既有 `<img>` 优化警告。
  `go test -race ./...` 在 `CGO_ENABLED=1` 下因当前 Windows 环境没有 GCC 无法构建，未记为通过。
- 标准 Wails v2.10.2 命令已成功生成绑定，但其重复前端安装步骤因当前进程环境找不到
  `npm` 入口而停止；在独立前端完整构建通过后，使用同一离线优先脚本
  `scripts/invoke-wails.ps1 build -s` 成功生成 `build/bin/SheetProof.exe`。
- 已用当次桌面产物真实打开专用工作簿：默认视图显示新增行 1、修改行 0、冲突行 1，
  ID=99 独立为新增，ID=3 的三个非 ID 列全部不同并保持冲突；点击“ID 对齐”切到按行号后
  显示修改行 2、冲突行 0。截图保存在 `build/acceptance/id-alignment/auto.png` 和
  `build/acceptance/id-alignment/position.png`，两个验收进程均正常关闭。
- 官网源内容和静态产物已同步并通过构建测试；生产部署因当前请求没有单独明确授权修改
  生产站点而被安全审核拒绝，没有通过其他方式绕过，线上站点保持原状。取得维护者明确
  部署授权后，应使用 `scripts/deploy-site-lightsail.ps1 -SshHost <ssh-host>` 发布并核对源站、
  Cloudflare 公网地址和同一 Caddy 实例的既有站点。
- 随后真实仓库截图暴露首版策略过严：工作表只要另有一个空白或重复 ID 行，就会在按钮
  显示“ID 对齐”时整表静默回退，导致 `7/8/9` 删除后从 `1001` 开始的共同记录仍连续错位。
  已改为可靠唯一 ID 局部对齐，并新增包含删除的 `7/8/9`、后续 `1001/1002`、空白及重复
  ID 辅助行的回归；不得再把理想化的全表唯一 ID 测试当作真实数据覆盖。
- 修正版自动化重新通过 Go 全量测试与 vet、前端 lint/typecheck/44 项测试/build、官网
  lint/build/10 项渲染测试；官网 lint 仍只有 4 条既有 `<img>` 优化警告。修正版 Wails
  v2.10.2 桌面产物也已重新构建。真实桌面工作簿同时包含空 ID、重复 ID、删除的
  `7/8/9` 和后续 `1001/1002`：结果只显示删除行 3、修改行 0、ID 对齐 2，右侧
  `1001/1002` 位于逻辑第 8/9 行，空白与重复 ID 辅助行未制造额外差异。截图保存在
  `build/acceptance/id-alignment-real/partial-alignment.png`，验收进程正常关闭。

## 2026-08-04：本地化 ID 表头的统一主键解析

- 真实 `mapItem.xlsx` 再次暴露首版假设过窄：A 列主键表头是“地图ID”，不是字面量
  `id`。该列左右分别有 229/228 个唯一值，228 个共同值中 173 个物理行不同；旧代码
  因表头不匹配完全没有进入对齐，因而把左侧独有的 `20045` 之后所有共同记录误报为修改。
- 新增 `internal/workbook.RecordKeyColumn` 作为对齐、行分类、冲突和自动续号共同使用的
  主键列解析器。精确 `id` 仍优先；否则双方同列同名且全表只有一个以 `ID` 结尾的候选
  （如“地图ID”）才会启用。多个限定 ID 候选保守回退物理行，不按数据唯一度猜测。
- 合成回归覆盖“地图ID”中间删除后共同记录对齐、限定 ID 冲突分类、自动续号以及多个
  候选拒绝猜测。修正后的 CLI 只读核对真实工作区文件与 Git `HEAD`：`mapItem` 从截图中的
  修改行 173 收敛为删除行 1、修改行 0、冲突行 0；13 个单元格差异都属于该删除记录。
- 当次 Wails 产物已实际启动并只读打开同一真实工作区文件和 HEAD 快照。界面显示新增行 0、
  删除行 1、修改行 0、冲突行 0、`ID 对齐 173`；逻辑第 57 行为左侧独有 `20045`，从第 58
  行起 `50001`、`50002` 等共同记录左右一致。截图保存在
  `build/acceptance/record-key-mapitem/mapitem-localized-id.png`，验收进程正常关闭且未保存。
- 验证通过 Go 全量测试和 vet、前端 lint/typecheck/44 项测试/build、官网 lint/build/10 项
  渲染测试以及 Wails v2.10.2 构建；官网 lint 仍只有 4 条既有 `<img>` 优化警告。race 测试
  已尝试，但当前 Windows 环境没有 `gcc`，`runtime/cgo` 无法构建。生产官网部署因当前请求
  未明确授权修改共享线上服务而被安全审核拒绝，未绕过执行，线上保持不变。

## 2026-08-04：中间独有记录定位与可配置主键列

- 真实 `jump.xlsx` 暴露第二版对齐仍以左侧物理行为显示坐标：右侧中间独有记录与已占用
  左行冲突后只能进入尾部，虽然分类数正确，但位置错误。现改为由共同主键记录作锚点构造
  统一逻辑记录序列，同时维护逻辑行到左右物理行的双向来源；单侧记录放在下一条共同记录
  之前，只有没有后续锚点时才进入尾部。
- 主键配置提升为会话后端状态。自动规则仍优先精确 `id`，否则只接受唯一的同列同名
  `*ID` 表头；没有匹配时默认无主键。用户可在左右任一网格的 A/B/C…列标题右键设置或
  取消当前表主键，双栏同步显示“主键”。产生撤销历史后禁止重配，避免已有复制、编辑、
  合并和撤销记录改变来源含义。
- `RowDiff`、Region 和冲突解决标记分别携带逻辑行、左侧物理行和右侧物理行。编辑、清空、
  普通复制、冲突覆盖/追加及撤销都通过同一映射回到实际工作簿；冲突规则本身没有变化，
  仍要求同一非空主键且其余实际数据列全部不同。
- 自动化新增覆盖 103 条记录中间缺少 ID 43/48 时的位置、尾部共同记录不被挤走、手动设置/
  取消非 ID 主键、对齐后的选择清空与撤销、冲突整行覆盖的真实物理目标及撤销，以及双栏
  列标题右键交互。Go 全量测试和 vet、前端 lint/typecheck/45 项测试/build、官网
  lint/build/10 项渲染测试和 Wails v2.10.2 构建均通过；官网 lint 仍只有 4 条既有
  `<img>` 优化警告。race 已尝试，但当前 Windows 环境没有 GCC，`runtime/cgo` 无法构建。
- 当次桌面产物只读打开真实 `jump.xlsx` 与经 Git 对象哈希校验的 HEAD 快照：分类为新增行
  2、删除/修改/冲突行 0，ID 43 和 48 分别位于逻辑第 44、49 行，表尾仍为最后共同记录。
  A 列由自动规则在双栏标为主键；真实右键菜单可取消，手动把 B 列设为主键后双栏标记同步
  切换。实机还发现并修复了列菜单误带“第 0 行”的显示问题。截图位于
  `build/acceptance/key-alignment/01-jump-overview.png`、`03-column-menu-fixed.png` 和
  `04-manual-b-key.png`；所有验收进程正常退出，真实工作簿未保存或修改。

## 2026-08-04：v0.3.0 正式发布

- 发布准备开始时，GitHub 最新已发布、非草稿版本为预览版 `v0.2.0`，包含 Windows amd64、
  macOS universal 和 `SHA256SUMS.txt`；当时 `main`、`origin/main` 与 `HEAD` 均为
  `f573a36`，本轮用户可见成果位于未提交工作树。
- 本轮新增滚动条差异位置、大表视口性能优化、普通/仓库/UGit/CLI 共用的安全主键记录
  对齐、邻近位置保持和可配置主键列，属于向后兼容的显著工作流扩展，因此从 `0.2.0`
  推导为次版本 `0.3.0`，继续标记为预览版。
- `product/product.json`、`product/changelog.json`、README、CHANGELOG、CLI 版本、前端与
  官网 package 清单/锁文件、官网内容及可预测的 `v0.3.0` 下载地址已经同步。官网渲染
  回归改为明确约束 `v0.3.0` 预览版，不再要求“未发布”标签。
- Go 全量测试与 `go vet`、前端 lint/typecheck/45 项测试/build、官网 lint/build 与
  10 项渲染测试通过；官网 lint 仍只有 4 条既有 `<img>` 优化警告。`go test -race ./...`
  已在 `CGO_ENABLED=1` 下尝试，但当前 Windows 环境没有 `gcc`，在 `runtime/cgo` 编译阶段
  停止，未记为通过。
- 标准离线优先 Wails 入口确认本地 v2.10.2 CLI 与模块缓存完整并成功生成绑定，但系统没有
  `npm` 可执行入口，重复前端安装步骤无法启动。前端已单独完成全部门禁后，使用同一脚本
  `scripts/invoke-wails.ps1 build -s` 成功生成 `build/bin/SheetProof.exe`；CLI `version`
  返回 `SheetProof 0.3.0`。本地候选 EXE 为 17,289,216 字节，SHA-256 为
  `5A50887668462537F6BB5E257BA60162032AD8B673EA3D9D86A591C600AE5BC3`，未签名。
- 当次候选 EXE 已真实启动生成的双文件演示夹具。整窗截图确认 16 处差异、主键对齐摘要、
  左右红/绿语义色、横纵滚动条差异标记、双网格、底部两行详情和状态栏完整；截图保存在
  `build/acceptance/release-v0.3.0/overview.png`。验收进程正常退出，临时工作簿已删除。
  主键列右键切换、本地化 ID 和中间独有记录位置沿用同一源码的本日既有实机验收证据。
- 隐私扫描覆盖 41 个候选文件（含前端旧哈希产物删除）、`v0.2.0..HEAD` 相关历史、官网静态产物和本地 Windows
  EXE，未发现私钥、令牌、凭据赋值、带认证 URL、私人用户路径、私网地址或可疑敏感文件名；
  EXE 也未嵌入当前用户目录或工作区路径。根目录既有未跟踪 `node_modules/`、`.pnpm-store/`
  和忽略的 `build/` 证据目录均不进入发布提交。
- 维护者明确确认后，41 个候选文件以发布提交
  `50db929b2ff022dda2f4b07ff84945729a60e637` 无强推地推送到 `origin/main`。提交级 Release
  workflow（run `30896565272`）的源码验证、Windows amd64 和 macOS universal 构建全部成功；
  随后创建并无强推地推送 annotated tag `v0.3.0`，标签级 workflow（run `30896935909`）
  再次完成源码验证、双平台构建、校验和及 Draft Release 创建。
- GitHub Release 已作为预览版公开在
  `https://github.com/tadazly/sheetproof/releases/tag/v0.3.0`。Windows EXE、macOS ZIP 和
  `SHA256SUMS.txt` 三项资产均可公网下载，校验和文件中的 SHA-256 与实际资产一致：Windows
  为 `5E6082A43AB38768E70BE3499192859D64049BD8F7134D2AFDFCD88865A58D8F`，macOS 为
  `0369300570631618756E5895946409FF98AB98698FBD6508F083620A92930104`。正式 Windows 产物
  未签名，macOS 产物未签名或公证，发布说明保留这些限制。
- 最终发布资产和校验和再次执行隐私扫描，未发现私钥、令牌、凭据、带认证 URL、构建机路径、
  私网地址或服务器身份；无需凭据轮换，也未重写公开历史。
- 通过 `scripts/deploy-site-lightsail.ps1` 将已验证静态产物部署到
  `https://sheetproof.luyilabs.com/`。部署脚本完成源站根页面健康检查并保留上一站点目录用于回退；
  后续源站只读验收确认下载页显示 `v0.3.0`、Caddy 配置有效，同一 Caddy 实例识别到的 5 个
  站点全部健康。Cloudflare 公网首页、功能、使用说明、下载、更新日志、favicon、品牌 SVG 和
  Open Graph 图片均返回 200；下载页与更新日志显示当前版本，三个 GitHub Release 资产均可
  实际访问且大小匹配。正式发布至此完成。

## 2026-08-04：官网差异化定位与信息层级优化

- 首页从通用功能罗列改为围绕“插入一条记录，不该变成几百行差异”的具体问题展开。
  首屏新增代码原生的双栏记录示例，直接呈现真实工作区、`origin/main`、`地图ID` 主键和
  “1 条真实删除 / 0 条错位修改”，避免用抽象装饰图代替产品机制。
- 核心价值收敛为 Git-native、Record-aware、Merge-safe 三条主线，并新增从版本准备、
  记录增删、接受结果到 Git 边界的流程对比。功能页按 Git 上下文、记录语义、受控写回
  三个故事展开；下载页把 Windows 和 macOS 提升为两个主入口，将源码和 Releases 下沉，
  同时增加可直接访问的当前版本 `SHA256SUMS.txt`。
- 产品事实源与 README/官网生成内容已同步；网站 lint 无错误并保留 4 条既有 `<img>`
  优化警告，静态导出成功，11 项渲染测试全部通过。本地静态产物已用真实浏览器在
  1440×900、390×844 以及 320/280/240 像素有效宽度下验收：首页、功能页和下载页均无
  横向溢出，桌面双栏与三卡层级、移动端记录示例和平台下载单列布局正常，移动导航可打开。
- 已通过规定脚本部署到 `https://sheetproof.luyilabs.com/`。Cloudflare 公网的五个页面路由、
  favicon、品牌 SVG 和 Open Graph 图片均返回 200；正式首页与窄屏功能/下载页的新版文案、
  记录示例和校验文件链接已在真实浏览器确认。源站返回 200，Caddy 配置有效，同一实例上的
  5 个站点全部健康。
- 用户随后反馈部分手机在通过抽屉完成一次站内导航后，右上角菜单无法再次打开。移动端
  站内入口改为静态页面原生导航，每次换页重新建立 `SiteShell` 和菜单状态；桌面导航仍保留
  客户端路由。首页、功能页和下载页同步去除口号式及英文概念包装，改为直接说明 Git 版本
  读取、主键记录对齐、左侧合并和保存边界。
- 首页主标题改为“让 Git 中的 XLSX，也能像代码一样审阅和选择性合并。”，下方以克制的
  单卡轮播补充三条通俗说明，覆盖插入记录、行业配置表和逐项确认合并；轮播尊重系统的
  减少动画设置，并保留手动切换。功能页加入两组来自当次 Windows 构建的真实对比截图：
  按行号的 90 处差异对照主键对齐后的 16 处，以及接受一项修改前后的 16/15 处差异。
  截图使用生成的游戏配置数据和通用 `D:\AetherFront\configs` 演示路径；临时目录在截图
  完成后已按精确文件清单验证并删除，四次验收进程均正常退出。
- 网站最终通过 lint、14 项渲染测试和静态构建；lint 无错误，仅保留 4 条既有 `<img>`
  优化警告。真实浏览器在 1440×900 与 390×844 下确认首页和功能页无横向溢出，轮播可
  手动切换，对比图在桌面双列、手机单列；手机抽屉从首页到功能、使用说明和下载连续三次
  原生换页后，每次均可重新打开。首次部署后发现 Cloudflare 仍缓存三个旧截图 URL，随后
  改用 `v030` 版本化文件名重新构建和部署；公网四张图的实际尺寸恢复为 1424×861、
  1424×861、1204×650 和 1204×650。最终源站返回 200，Caddy 配置有效，同一实例当前
  识别的 6 个站点全部健康，正式首页与功能页均已在真实浏览器复核。
- 用户再次确认实际手机上只有首页菜单能打开，功能、使用说明、下载等页面的按钮无响应。
  全新浏览器直接打开非首页时脚本增强可以工作，说明此前修复仍把基本展开能力放在 React
  启动之后，不能覆盖旧脚本、启动失败或移动浏览器时序异常。移动菜单现改为静态 HTML 中的
  原生 `<details>/<summary>`：右上角触发器由浏览器直接切换 `open`，CSS 根据该状态显示
  抽屉、变换关闭图标并锁定滚动；React 只负责焦点、Escape 和遮罩等增强，不再是打开菜单的
  前提。验收除正常逐页直达外，还复制当次静态产物并移除全部脚本：`script` 数量为 0 时，
  功能页仍能展开菜单、进入使用说明后再次展开；临时服务器和无脚本产物均已清理。最终网站
  lint 无错误（4 条既有图片警告）、静态构建和 14 项渲染测试通过；正式站五个页面均返回
  200 且包含原生菜单结构，真实手机视口从功能到说明、下载、更新日志连续换页后每页都能
  再次展开。源站返回 200，Caddy 配置有效，同一实例的 6 个站点全部健康。
- 官网截图查看器改为静态锚点入口加客户端增强：即使 React 尚未启动，点击产品图也会由
  CSS `:target` 打开独立大图；正常运行时支持 1×–5× 缩放、缩放按钮、100% 重置、拖动和
  Escape/浏览器返回关闭。桌面滚轮以指针位置为中心缩放，手机以 Pointer Events 处理双指
  缩放和单指拖动；查看器打开时只在覆盖层内阻止滚轮和触摸默认行为，并锁定背景页面，
  不修改 viewport 或永久禁止页面辅助缩放。
- 参照 Apple 中国和华为消费者官网的大标题、短说明和留白层级重做中文排版：首页主标题
  和内页标题降低最大字号，负字距从英文式的紧缩值收敛到轻微负值，行高提高到
  1.16–1.24；常规正文、卡片说明、截图说明、更新日志和页脚提高到 15–17px 为主。改动后
  需要在桌面和手机逐页复查标题折行、模块高度与横向溢出，避免出现单字悬挂。
- 本轮网站通过 lint（0 错误、4 条既有 `<img>` 优化警告）、静态构建和 15 项渲染测试。
  本地真实浏览器在 1440px 与 390px 下逐页复查：首页主标题桌面为 54px/三行，内页标题
  桌面均为 54px，功能页不再出现单字断行；手机首页为三行，功能页为两行，其余内页为
  一行，五个页面横向溢出均为 0。桌面查看器实测滚轮从 100% 放大到 218%，手机视口实测
  点击开窗、按钮放大到 135% 和拖动，两个视口的 `visualViewport.scale` 始终为 1。
- 已按维护者明确授权通过 `scripts/deploy-site-lightsail.ps1` 部署到正式域名；脚本的源站根
  页面健康检查通过，否则会自动恢复上一目录。Cloudflare 公网首页、功能、使用说明、下载、
  更新日志均加载新版标题与样式，横向溢出为 0；功能页存在 4 个静态大图入口和对应查看器，
  关键版本化截图实际加载为 1424×861。公网查看器在客户端增强完成后实测滚轮从 100%
  放大到 188%，页面缩放保持 1。后续独立只读 SSH 的 Caddy 配置与同实例站点计数复核因
  本机权限审批服务中断而未取得结果；部署脚本本轮没有修改 Caddy 配置，不得把该项记为
  已独立复核。
- 正式站反馈图片放大后“关闭”只会把比例恢复为 100%，弹窗仍留在页面。真实浏览器复现
  确认 React 的 `open` 状态已关闭，但路由仍保留 `#screenshot-viewer-*`，CSS `:target`
  因而继续显示同一弹窗；当前 vinext 路由环境也会保留该片段。查看器现在在客户端增强后
  给关闭状态添加 `.is-closed`，并让它明确覆盖残留的 `:target`；打开时恢复查看器 `id`，
  关闭时移除 `id`、恢复页面滚动并重置缩放。无脚本时仍保留原生锚点回退。390px 真实浏览器
  验收中，图片从 135% 关闭后 `open` 与可见弹窗数均为 0、页面滚动恢复、全部比例回到
  100%，同一图片可再次打开并再次关闭。使用说明页快捷键中的 `Command` 同步缩写为标准
  `⌘` 符号，减少右侧快捷键列宽度。
- 上述关闭修复与快捷键符号已再次通过网站 lint、静态构建和 15 项渲染测试，并使用
  `scripts/deploy-site-lightsail.ps1` 部署到正式域名。Cloudflare 公网 390×844 真实浏览器
  复核中，查看器打开并放大后背景滚动锁定；关闭后 `open=0`、可见弹窗数为 0、页面滚动
  恢复且无横向溢出，同一图片再次打开和第二次关闭均正常。使用说明页的保存、另存、撤销、
  全选和缩放快捷键均显示 `Ctrl / ⌘`，页面不再包含 `Ctrl / Command`。
- 用户实机继续发现两个回归：关闭查看器后页面跳到顶部，手机双指缩放失效。前者由关闭时
  清空 `location.hash` 和修改查看器 `id` 引起；后者的直接原因是页面滚动锁 effect 依赖
  `transform.scale`，每次手势更新都会拆装 `lightbox-open` 和触摸环境。当前实现不再修改
  URL 或查看器 `id`，客户端打开、关闭只切换状态；无脚本关闭锚点回到原图片而非页面顶部。
  页面锁 effect 与缩放状态解耦，关闭时以非平滑滚动恢复打开前坐标。390×844 本地真实点击
  验收中，从 `scrollY=3400` 打开、放大再关闭后仍为 `3400`，弹窗不可见；放大到 135% 时
  `lightbox-open` 始终存在且查看区域 `touch-action: none`，避免双指过程中手势上下文被重置。
- 修复已重新部署到正式域名。Cloudflare 公网复核中，390×844 手机视口从 `scrollY=3400`
  打开并放大到 170%，缩放前后 `lightbox-open=true`、查看区域 `touch-action: none`；关闭后
  回到 `scrollY=3400`，可见弹窗为 0。1440×900 桌面视口从 `scrollY=1800` 打开、放大、
  关闭后仍为 `1800`。两个视口横向溢出均为 0，使用说明页继续显示 `Ctrl / ⌘`。

## 2026-08-05：三语工具栏响应式与设置窗口

- 在当前 Windows Wails 产物的 1210×900 外框实机复现三语工具栏问题：英语缩放/另存区域
  挤压，中文保存与 UGit 相贴，日语的缩放、另存、保存和 UGit 出现实际文字覆盖。当前
  CSS 的紧凑断点低于真实客户区宽度，且所有按钮固定不收缩，是本次重叠的直接原因。
- 工具栏在 1280px 以下改用各语言独立短标签，并收起差异导航、撤销、另存和设置的次要
  文字；更窄断点继续保留图标。宽屏仍显示完整术语，操作顺序和左右写入语义不变。
- 工具栏的语言下拉框与“配置 UGit”移入统一设置窗口。UGit 的成功、取消和错误信息均在
  设置窗口内显示，不再把结果写到模态遮罩后的主界面通知条。
- 设置窗口新增“清理缓存”和“清除所有数据”。前者只删除持久化差异表语义索引；后者
  删除索引、最近仓库、对比引用、另存目录、侧栏宽度、仓库搜索历史和工作簿视图设置，
  保留当前语言。两项操作均有三语二次确认，明确不删除工作簿、不修改仓库或 Git；当前
  对比保持打开，清除所有数据后下次启动不会自动恢复任何仓库。
- 清理时会取消正在运行的差异索引并递增代次；后台旧任务在取得清理代次后不得重新写回
  缓存。偏好层和 Controller 回归分别覆盖仅清缓存与全部清理的保留/删除边界，前端回归
  覆盖设置内 UGit 结果、两类确认说明以及只删除 SheetProof/ugxlsx 相关 localStorage 项。
- 本轮按维护者明确范围未修改或发布 `site/`，也未 commit、push、打标签或部署。
- 自动化最终通过 `git diff --check`、`go test ./...`、`go vet ./...`、前端 lint、
  typecheck、66 项测试和 production build；标准离线优先脚本完成 Wails v2.10.2
  Windows 全量构建，产物为 `build/bin/SheetProof.exe`。
- 当次桌面产物已按 en、zh-CN、ja 分别检查 1210×900、1440×900、1600×900；另在
  1210×900 检查仓库模式、三语设置窗口和两种二次确认，在 1440×900 检查三语列标题
  上下文菜单。截图集中在 `build/acceptance/localization-settings/after/`，所有验收进程
  正常退出。隔离 `APPDATA` 验证清缓存后当前比较仍打开；清除所有数据后当前仓库仍打开，
  但下一次启动进入无仓库欢迎页。隔离 `GIT_CONFIG_GLOBAL` 验证 UGit 结果显示在设置窗口。
- 当次 EXE 对三语演示数据的首表 CLI 核对均保持物理行 90、主键对齐 16、应用后 15；
  没有修改差异、主键、合并、保存、Git 或 CLI/JSON 稳定字段行为。
- 随后修复语言偏好重启回归：CLI 原先把操作系统探测出的默认语言和显式 `--lang`
  都写入同一个 `Options.Locale`，Controller 因而把普通启动误判为命令行覆盖并跳过已保存
  偏好。现在 `Options.LocaleExplicit` 只标记真实 `--lang`；普通启动在创建 Controller 时
  恢复偏好语言，显式参数仍只覆盖当前启动。隔离配置实机完成“设置为日语、正常关闭、
  不带 `--lang` 重启”，重启后的主界面和设置下拉框均保持日语。

## 2026-08-05：三语最终验收中的仓库启动竞态

- 最终验收使用当次 Windows Wails 产物和三份隔离临时 Git 仓库独立复现：带预选文件与
  引用启动仓库模式时，文件树已经出现，但比较区会永久停留为空白工作区。后端纯控制器
  检查同时显示仓库扫描成功且稍后能够建立 31 处工作簿差异，因此不是 Git 数据或演示工作簿问题。
- 根因是 `openRepositoryInternal` 在预选工作簿会话仍异步打开时提前清除了 `loading`；前端
  启动轮询据此停止，后续会话结果不再进入初始界面。现在只由预选文件打开流程的最终路径
  清除加载状态；无预选文件的仓库入口仍在扫描完成后正常结束加载。
- 新增 `TestControllerRepositoryBootstrapWaitsForSelectedWorkbook`，修复前连续 20 次均可
  复现 `Loading=false` 且 `HasSession=false`，修复后连续 20 次通过。重新构建的 Wails 产物
  已在 en、zh-CN、ja 三种界面打开真实仓库会话，显示预选引用、16 处首表差异、工作区未提交
  标记及只读右侧。验收截图保存在忽略目录 `build/acceptance/localization-final/gui/`。

## 2026-08-05：官网首页轮播与减少动态效果

- 首页 `HeroMessageCarousel` 原先在 `prefers-reduced-motion: reduce` 时不创建定时器，导致
  使用该系统偏好的浏览器永远停在第一条说明。现在三条使用说明始终每 4.8 秒轮换，并保留
  原有的 0.28 秒文字淡入/4px 位移和 0.18 秒圆点高度/颜色过渡；该轻量效果不再因浏览器偏好减配。

## 2026-08-05：中英日国际化验收结项

- 维护者明确确认当前国际化验收可以标记完成。`docs/localization-final-acceptance.md` 的最终结论
  更新为 Conditional Pass：自动化、Windows Wails 构建、已执行的三语桌面流程、演示数据、
  90/16/15 基线、三语真实截图、网站路由/SEO、README/CHANGELOG 和内容同步均通过。
- `go test -race ./...` 的 CGO/GCC 环境阻塞、macOS 未实机、少量极端底层诊断和网站静态
  `<img>` warning 继续作为允许的已知限制。报告保留未由 Codex 独立执行的真实交互项目，
  并明确它们由维护者接受为证据边界，未虚构为 Codex 已验证。

## 2026-08-05：v0.4.0 正式发布候选

- GitHub 公共 API 只读核验确认最新已发布、非草稿版本仍为预览版 `v0.3.0`，默认分支为
  `main`。当前 `feat/i18n-en-zh-ja` 相对本地和远端 `main` 领先 4 个提交、没有落后，
  工作树在发布准备开始前为 clean；用户已确认国际化验收结项。
- 桌面应用、CLI、README、更新日志、官网、SEO、导航、三语产品截图、语言偏好和设置窗口
  构成向后兼容的新能力，因此按 SemVer 从 `0.3.0` 推导为 `0.4.0`，继续标记 Preview。
  从公开 `v0.3.0` 到候选还包含已部署但未进入该标签的移动导航、截图查看器和首页轮播修复，
  以及仓库预选工作簿启动竞态修复；这些内容从 `0.3.0` 历史条目移入 `0.4.0`。
- `product/product.json`、三语 changelog 事实源、README/CHANGELOG、CLI 版本、前端与官网
  package 清单及锁文件、官网生成内容和可预测的 `v0.4.0` 下载 URL 已同步；发布、架构和项目
  技能文档改为引用拆分后的三语 changelog 事实源。当前发布准备产生 26 个受控文件修改，
  尚未暂存、提交、推送、切换分支、打标签、运行远端 workflow 或部署官网。
- 自动化通过 `go test ./...`、`go vet ./...`、前端 lint/typecheck/66 项测试/build、官网
  lint/10 项渲染测试/build、16 路由静态导出、内容同步检查和 `git diff --check`。官网 lint
  仍只有 4 条既有 `<img>` 性能 warning。`go test -race ./...` 在 `CGO_ENABLED=1` 下因本机
  缺少 `gcc` 于 `runtime/cgo` 编译阶段停止，未记为通过。
- 标准离线优先脚本完成 Wails v2.10.2 Windows amd64 全量构建。候选
  `build/bin/SheetProof.exe` 为 17,417,216 字节，SHA-256 为
  `2D76E3E30CBC00AF2D82122DFF2EB0E0390B8D792782690101BF34620B0EEFDE`，二进制包含版本
  `0.4.0`，当前未签名。
- 当次候选 EXE 已分别以英语、简体中文和日语打开对应生成式演示工作簿。三张 1424×861
  客户区截图均显示 16 处首表差异、主键对齐、双栏语义色、完整底部详情和状态栏，没有
  重叠或截断；每个进程都由截图脚本请求关闭并确认退出。证据位于忽略目录
  `build/acceptance/release-v0.4.0/`，其中本机 `build/` 路径不会进入提交或公开素材。
- 隐私扫描覆盖从 `v0.3.0` 到候选的 116 个文件、相关补丁历史、三语发布说明和本地 Windows
  EXE。未发现私钥、令牌、凭据赋值、带认证 URL、维护者私人用户路径、工作区路径、私网地址
  或可疑敏感文件名；EXE 未嵌入当前工作区路径。公开三语截图沿用维护者已接受的通用演示路径
  证据。未发现需要移除或轮换的敏感数据，也不需要重写公开历史。
- 按两阶段发布规则，下一步必须先取得维护者对 `v0.4.0` 的明确确认。确认后才允许提交候选、
  快进合并并推送 `main`，先手动运行提交级 Release workflow，成功后创建并推送 annotated
  tag `v0.4.0`，等待标签 workflow 生成双平台产物与 Draft Release，核对校验和并发布，
  最后部署和核验 Lightsail/Caddy 正式官网。

## 2026-08-05：v0.4.0 正式发布完成

- 维护者明确确认正式发布后，26 个候选文件以 `Release SheetProof v0.4.0` 提交为
  `9e5d09294752232068c04926d79c08bea1f0fec1`；`feat/i18n-en-zh-ja` 通过
  `--ff-only` 合入 `main` 并推送到 `origin/main`。发布前后均未强推、未移动标签，也未
  覆盖无关工作区修改。
- 标签前手动触发的提交级 `Build desktop release` workflow #9
  （run `30979987792`）通过源码验证、Windows amd64 和 macOS universal 构建；非标签运行的
  Draft Release 作业按预期跳过。随后创建 annotated tag `v0.4.0`，并确认标签对象最终指向
  上述发布提交。
- 标签触发的 workflow #10（run `30980271706`）再次通过源码验证、两个平台构建和 Draft
  Release 创建。实际草稿资产下载后与 `SHA256SUMS.txt` 逐项一致：
  `SheetProof-windows-amd64.exe` 为 16,912,384 字节，SHA-256 为
  `921C34002F2A33D8CF25BD6C3A14602F2A06D233045307C445727CC065F87E6F`；
  `SheetProof-macos-universal.zip` 为 11,045,108 字节，SHA-256 为
  `7FB722C38DE0372AF99F9F4ADE440674BA3276AE2DB5E60C2BB61D6FCE243916`。
  macOS 压缩包包含完整 `SheetProof.app/Contents` 结构。Windows 产物仍未签名；发布说明和
  官网继续明确 Preview、未签名及未公证限制。
- 自动生成的 Release Notes 已替换为整理后的 `v0.4.0` 用户更新说明，GitHub Release
  `https://github.com/tadazly/sheetproof/releases/tag/v0.4.0` 于 2026-08-05 14:15（UTC+8）
  公开为 prerelease。三个公开下载链接均返回 200，实际长度分别与 EXE、ZIP 和 192 字节的
  `SHA256SUMS.txt` 一致。
- Release 公开后重新执行内容同步检查；官网 lint 为 0 错误、4 条既有 `<img>` 性能 warning，
  16 条静态路由构建成功，10/10 渲染测试通过。沙箱内第一次静态构建因 Windows
  `spawn EPERM` 停止，随后以完全相同命令在受控非沙箱环境重跑通过，未修改代码或依赖。
- 已通过 `scripts/deploy-site-lightsail.ps1` 把该静态产物原子部署到
  `https://sheetproof.luyilabs.com/`。源站 Caddy 配置有效且更新日志包含 `0.4.0`；解析 Caddy
  实际 JSON 路由后，当前同实例 5 个站点全部健康。Cloudflare 公网英语、简体中文和日语的
  首页/更新日志及主要功能、使用说明、下载页面均返回 200，favicon 和产品截图实际下载非空。
- 最终隐私扫描覆盖 `v0.3.0..v0.4.0` 候选文件、提交历史、发布说明、公开三语截图，以及从
  Draft Release 下载并展开的 Windows/macOS 实际资产。私钥标记、GitHub 令牌前缀、凭据
  赋值、带认证 URL、维护者私人用户路径、当前工作区路径、Linux/macOS 用户目录、私网 IPv4
  和可疑敏感文件名命中均为 0；未发现敏感数据，不需要移除、轮换凭据或重写历史。

## 2026-08-05：官网首页三语标题排版候选

- 首页 Hero 继续由三语 `hero` 数组生成一个可访问主标题，但人工断点不再强制相同：英文采用
  三段“Review XLSX changes / in Git—then apply / only what you approve.”，简体中文采用
  两段“审阅 Git 中的 XLSX，/ 只应用确认过的修改。”，日文采用两段
  “Git 上の XLSX 変更を確認。/ 承認した変更だけを反映。”。手机端把同一组行段恢复为行内
  文本并自然重排，没有复制另一套标题 DOM。
- 英文桌面标题为 54px/1.14，中文为 54px/1.19，日文为 47px/1.19；手机分别使用响应式
  32–38px，并把中文和日文行高提高到 1.21。标题与正文间距为桌面和平板 24px、手机 18px；
  标题最大宽度分别为 650px、620px 和 630px。Hero 左栏权重小幅提高到 1.08，且只对当前
  Hero 使用 `:lang()` 差异化，不改变内页标题、颜色、字体体系、按钮或演示区。
- 当次静态构建的英语、简体中文和日语首页均用真实浏览器检查 1440×1000、1024×900、
  768×1024、430×932、390×844 和 360×800，共保存 18 张截图到忽略目录
  `build/acceptance/site-hero-layout/after/`。英文为桌面/平板三行、手机四行；中文全部两行；
  日文为桌面/平板两行、手机三行。桌面和平板没有人工行段内部二次换行，全部视口横向溢出为
  0；三语标点自然，中文和日文粗体字形没有上下粘连，桌面左右栏仍保持平衡。
- 官网 lint 为 0 错误、4 条既有 `<img>` 性能 warning；16 条静态路由生产构建和 10/10
  渲染测试通过。仓库没有 `typecheck` 脚本；对 `site/app` 的严格 TypeScript 检查通过，直接按
  根 `tsconfig.json` 检查仍会因既有 `worker/index.ts` 未声明 Cloudflare `Fetcher` 与
  `D1Database` 全局类型而失败，本轮未扩大范围修改 Worker 配置。
- 维护者查看截图和差异后明确授权推送与部署。上述 16 个文件已以
  `Improve multilingual homepage hero typography` 提交并推送到 `origin/main`，没有强推或
  混入截图、临时产物及无关文件。提交前隐私扫描未发现私钥、令牌、认证 URL、私人用户路径、
  私网地址或未跟踪文件。
- 正式官网部署继续使用 `scripts/deploy-site-lightsail.ps1`，但当前 Codex 执行环境把 SSH
  连接导向隔离网络并在认证前关闭，静态归档未上传，远端站点目录没有被替换。Cloudflare
  公网三语首页均仍返回 200，并保持上一版标题，未出现半部署。下一步需在具有正常 SSH
  连通性的维护者主机上重新执行该脚本，再核对源站、公网三语路由和同一 Caddy 实例的既有站点。

## 2026-08-05：无变化编辑与差异复制状态修复

- 本轮开始时重新核对到工作树为 clean、分支为 `main`，本地 `HEAD` 与 `origin/main` 均为
  `4bf6f4d Document homepage deployment blocker`；这晚于任务说明中的 `b41fdbb` 基线。
  实施过程保留已有内容，未 reset、checkout、清理、提交、推送、打标签或发布应用版本。
- 左侧原位编辑现在先以用户实际编辑表达判断是否变化。文本、数字、公式和显式空字符串在
  Enter 或失焦时没有改动会直接结束编辑，不进入 busy、不调用后端，也不重载 Summary、
  Differences 或 Region。后端 `Session` 同时提供权威防线：普通值按
  `present + raw + formula + type` 比较，相同公式在 present、formula 和 type 一致时忽略缓存
  raw；清空仅在原单元格不存在时为 no-op，显式空字符串与不存在仍保持区别。
- 普通单元格复制在应用前按后端语义过滤；display 和样式不参与过滤。全为 no-op 时在
  `merge.Apply`、行重分类和历史记录之前返回，因此 state ID、saved state、dirty、撤销计数、
  warning 与行处理标记均不变；部分差异批量只应用真实差异并共用一个撤销步骤。整行复制不走
  该过滤，继续完整复制值、公式、样式、超链接和批注。
- 工具栏“复制到左侧”删除了零差异时回退加入活动单元格的逻辑。普通 Region 和行筛选
  FilteredRegion 都只使用后端差异索引中的坐标；相等选择保持按钮原位置、显示 0 且禁用，
  混合选择只计数和提交真实差异。右键批量复制、冲突隔离提示和数字形文本跟随右侧类型的行为
  保持不变。
- Go 新增并通过 no-op 文本、数字、公式、缺失清空、显式空字符串、真实类型修正、单格与批量
  复制过滤、部分差异单步撤销、全部相等不产生状态、既有 dirty 不被清除及保存后保持 clean
  的回归测试。`go test ./...` 与 `go vet ./...` 通过；`go test -race ./...` 已以
  `CGO_ENABLED=1` 尝试，但本机缺少 `gcc`，在 `runtime/cgo` 编译阶段停止，未记为通过。
- 前端新增不变文本/数字/公式/显式空字符串不提交、真实修改仍提交，以及普通 Region 和
  FilteredRegion 的相等/混合选择测试；前端 lint、typecheck、7 个测试文件共 69 项测试和生产
  构建全部通过。官网内容同步检查通过，lint 为 0 错误、4 条既有 `<img>` 性能 warning，
  16 条静态路由构建与 10/10 渲染测试通过。
- 离线优先脚本使用 Wails v2.10.2 完成 Windows amd64 全量构建。当次
  `build/bin/SheetProof.exe` 已真实启动并通过鼠标、键盘操作：clean 会话的不变文本、数字、
  公式及显式空字符串编辑保持“已保存”且保存/撤销不可用；相等右侧单元格显示复制 0 并禁用；
  相等与差异混选只显示 1，复制后只产生一个撤销步骤并变为未保存；保存后再执行 no-op 仍为
  clean；右侧文本 `"123"` 的类型跟随后保存为 string 且不再高亮。过程截图与验收脚本位于忽略
  目录 `build/acceptance/noop-copy/`，最终 CLI 复核左右工作簿只剩预期 1 个差异，精确进程已退出。
- README、架构、手工验收、三语 changelog 事实源与生成文件、三语产品能力文案及官网生成内容
  已同步。经 `scripts/deploy-site-lightsail.ps1` 原子部署后，远端更新日志文件 SHA-256 与本地构建
  一致；Caddy 配置有效，同一实例 5 个站点全部健康。Cloudflare 公网 14 个主要三语路由与资源
  返回 200，代理响应头有效，英语、简体中文和日语更新日志均显示本轮两条用户可见修复。

## 2026-08-05：v0.4.1 正式发布候选

- 维护者指出上一轮提示中的版本归属有误：无变化编辑与差异复制修复发生在公开 `v0.4.0`
  之后，不应回填到 `0.4.0` 更新日志。最新公开版本仍为 Preview `v0.4.0`，因此本轮按向后兼容
  修复推导 Patch 版本 `v0.4.1`。三语事实源已恢复 `0.4.0` 的发布时内容，并为 `0.4.1`
  新建只包含本轮两项用户可见修复的独立条目；发生在标签后的官网三语标题排版继续保留在产品
  内容与代码中，但按本轮文案边界不列为版本日志项目。
- `product/product.json`、三语 changelog、README/CHANGELOG、CLI 版本、桌面与官网 package
  清单及 lockfile、官网生成内容和预期下载 URL 已同步为 `0.4.1`。用户可见更新只表述为：未
  实际改变单元格时不再产生未保存状态；“复制到左侧”只计数并处理真实差异。候选内容同步检查
  与 `git diff --check` 通过。
- Go 的 `go test ./...` 和 `go vet ./...` 通过；`CGO_ENABLED=1 go test -race ./...` 因本机
  没有 `gcc` 在 `runtime/cgo` 编译阶段停止。前端 lint、typecheck、69/69 测试及 production
  build 通过。官网 lint 为 0 错误、4 条既有 `<img>` 性能 warning，16 条静态路由构建和
  10/10 渲染测试通过；联网依赖审计因外部元数据披露策略被拒绝，npm 本地缓存的离线生产依赖
  审计在桌面前端与官网均为 0 个已知漏洞。
- `scripts/invoke-wails.ps1 build` 使用固定 Wails v2.10.2 生成 Windows amd64 候选；
  `build/bin/SheetProof.exe version` 返回 `0.4.1`，文件为 17,419,776 字节，SHA-256 为
  `9A856D39BC9A60E8A20A67D6E6C9BA9A94525654F339B13D257583F1C7E54464`，当前未签名。
  使用显式重建的左右夹具再次完成真实鼠标/键盘验收，确认 clean no-op、相等复制 0、混合复制
  1、真实复制单步撤销、保存后 no-op 和数字形文本类型跟随均符合预期；8 张当次截图仍位于
  `build/acceptance/noop-copy/`，验收进程已精确退出。
- 候选差异、文件名与本地 EXE 的隐私扫描未发现私钥、GitHub/云令牌、认证 URL、私人用户路径、
  工作区路径、私网 IPv4 或可疑敏感文件名。宽松的 `sk-` 二进制模式曾命中一个位于更长标识符
  内部的 255 字节字符串；加入令牌边界与现代令牌前缀后为 0，判定为压缩资源误报，不需要移除、
  轮换凭据或重写历史。
- 当前仍停留在两阶段正式发布的候选确认点：没有暂存、提交、推送、创建或移动标签、运行远程
  Release workflow、发布 GitHub Release，也没有把 `0.4.1` 候选重新部署到正式官网。取得明确
  确认后才会提交并推送 `main`，先以提交 SHA 验证双平台 workflow，再创建 annotated
  `v0.4.1` 标签，核对 Draft Release 的 Windows/macOS 资产与校验和，发布 Preview Release，
  最后重新同步、构建、测试并通过 Lightsail/Caddy 脚本原子部署官网，核对源站、Cloudflare
  三语页面、下载链接和同实例既有站点。

## 2026-08-05：v0.4.1 正式发布完成

- 维护者明确确认正式发布后，29 个候选文件以 `Release SheetProof v0.4.1` 提交为
  `5074bf2771adc8d9df45ee91b5b99a9ab1c249eb`，并以普通 fast-forward 推送到
  `origin/main`。没有强推、分支切换、覆盖无关修改或混入忽略目录中的截图、测试缓存和发布资产。
- 提交级 `Build desktop release` workflow #30991478353 通过源码验证、Windows amd64 与
  macOS universal 构建；无标签运行按设计跳过 Draft Release。随后创建并推送 annotated tag
  `v0.4.1`，标签对象剥离后精确指向上述发布提交，没有移动既有标签。
- 标签 workflow #30991843719 再次通过源码验证、双平台构建和 Draft Release 创建。下载并核对
  Draft 资产后，`SheetProof-windows-amd64.exe` 为 16,914,432 字节、SHA-256 为
  `FD6E233C913A8B7E9919E6474699B943BB3BB0633FCBDA14252DAC413F9E14AD`；
  `SheetProof-macos-universal.zip` 为 11,046,641 字节、SHA-256 为
  `B7FDCD92E8CE88A5B3BEFE71568CA6B4610D3AF9E4EB2622F2E8EFF23E74C57`。两项均与
  192 字节的 `SHA256SUMS.txt` 一致；macOS 包含完整
  `SheetProof.app/Contents` 结构，Windows EXE 返回版本 `0.4.1`。
- Release Notes 已替换为仅两项用户可见修复，没有重新列入官网标题排版。GitHub Release
  `https://github.com/tadazly/sheetproof/releases/tag/v0.4.1` 于 2026-08-05 17:17（UTC+8）
  公开为 prerelease；EXE、macOS ZIP 和校验文件三个未认证公网下载均返回 200，长度与核验资产
  一致。Windows 产物仍未签名，macOS 产物仍未公证，Release 与官网继续明确 Preview 限制。
- 实际 Release EXE 与展开后的 macOS 应用再次扫描私钥、GitHub/云/OpenAI 令牌、认证 URL、
  私人用户路径、工作区路径和私网 IPv4，所有类别均为 0；最终发布范围未发现敏感数据，不需要
  移除资产、轮换凭据或重写历史。
- Release 公开后重新通过内容同步检查；官网 lint 为 0 错误和 4 条既有 `<img>` warning，
  16 条静态路由构建及 10/10 渲染测试通过。`scripts/deploy-site-lightsail.ps1` 已把构建结果
  原子部署到正式域名；源站英语、简体中文和日语更新日志文件 SHA-256 与本地静态产物逐项一致，
  Caddy 配置有效，同实例 5 个已配置站点全部健康。
- Cloudflare 公网 15 个三语产品路由均返回 200 且具有代理响应头；三语更新日志均显示
  `0.4.1` 和两项正确修复，英文页面不包含误归档的标题排版条目，下载页面已指向
  `v0.4.1`，实际 `/brand/favicon.ico` 返回 200 且非空。

## 2026-08-05：当前工作表查找

- 本轮从 clean 工作树开始，完整复核项目规则、技能、交接、README、架构和手工验收后实施；
  没有 reset、checkout、覆盖已有成果、提交、推送、打标签或发布应用版本。功能严格限定为
  Find，不包含 Replace、Replace All、跨工作表/工作簿搜索、历史或条件持久化。
- `internal/app.Session` 新增左右独立的当前搜索缓存。缓存键包含工作表、侧别、查询、大小写、
  Unicode 全词、Go RE2 正则、规范化行筛选和数据代次；结果只保存紧凑逻辑行列索引，Wails
  `Find` / `NavigateFind` / `ClearFind` 只传回数量、当前序号和单个当前坐标。未筛选时扫描
  对齐后的 `comparisonLeft` / `right` 稀疏实际单元格；筛选时复用 `FilteredRegion` 的同一
  `SheetDiff.Rows` 与冲突追加映射，缺失侧不匹配。公式按 `=公式`、其他值按 raw 搜索，显式
  空字符串可被有效正则命中，display 和差异相等语义均未改变。
- Region 只给当前窗口单元格附加左右匹配布尔标记，不传完整匹配列表。编辑、复制、清空、
  合并、撤销、左右重载、右侧替换/移除和主键/行号对齐变化递增会话数据代次；Save 不改变
  内存内容。前端左右状态、150ms 防抖、请求序号、错误和当前索引完全独立；工作表、文件、
  引用、筛选或对齐变化先使旧结果失效再重搜，旧异步结果不能安装或触发旧坐标跳转。
- 每个网格右上角新增不占布局轨道的 VS Code 风格单行 Find，提供 Aa、ab、`.*`、计数、
  上一处、下一处和关闭。`Ctrl/Command+F` 按网格焦点打开，Enter/Shift+Enter 与
  F3/Shift+F3 循环导航；只推进操作侧索引，左右仍滚到同一逻辑坐标。输入控件保留原生
  `Ctrl/Command+A`、数字键和 Tab。普通/当前匹配使用可叠加紫色内描边，不覆盖新增绿、
  删除红、修改红/绿、冲突橙或蓝色选择框；三语 aria-label、title、焦点和 pressed 状态已补齐。
  后续实机反馈发现全局输入焦点描边和 Find 的 `:focus-within` 会在 `Ctrl/Command+F` 后叠成
  两层蓝色矩形框；现仅对 Find 输入覆盖原生蓝色 outline，外层改为单层中性边框，按钮的
  Tab 焦点也改为中性边框与浅底色，保留键盘可辨识性而不再显示突兀蓝框。前端回归确认
  `Ctrl/Command+F` 后真实 activeElement 仍为 Find 输入框；重新构建 Wails 后，以真实键盘
  分别截取输入聚焦和 Tab 到 Aa 的状态，证据为 `17-en-find-input-neutral-focus.png` 与
  `18-en-find-button-neutral-focus.png`（另有同名 `-crop` 局部图）。两种状态均无蓝色外框。
- Go 回归覆盖左右独立、当前工作表、单类/多类筛选、缺失侧、主键逻辑坐标和中间独有/局部
  歧义行、大小写/Unicode 全词/正则/非法正则、单格只计一次、公式/raw/显式空、超过 10,000
  匹配、各类失效与并发 Region；前端回归覆盖焦点快捷键、双侧独立、Enter/F3 正反导航、
  同步滚动、输入原生快捷键、内联状态、旧请求丢弃、筛选/工作表重搜、叠加高亮、单侧关闭和
  不存在任何替换界面。`go test ./...`、`go vet ./...`、前端 lint/typecheck、73/73 测试、
  production build、内容同步和 `git diff --check` 通过。`CGO_ENABLED=1 go test -race ./...`
  已尝试，但本机缺少 `gcc`，停在 `runtime/cgo` 编译前，未记为通过。
- 100,000 个有效单元格被稀疏分布到约 1,000,000 行 × 1,000 列后，Intel i7-14700、
  Windows amd64、`-benchtime=5x` 的完整重新扫描结果为：默认普通查询 29.79ms/op，大小写
  查询 6.64ms/op，正则查询 28.20ms/op；没有设置易受机器波动影响的测试阈值。缓存后导航
  仍为 O(1)，匹配数量不受 Differences 的 10,000 上限约束。
- 离线优先 `scripts/invoke-wails.ps1 build` 使用本地 Wails v2.10.2 完成绑定、前端和
  Windows amd64 全量构建，产物为 `build/bin/SheetProof.exe`。该产物已通过真实键鼠与截图
  验收：双侧不同查询、Aa/ab/正则、F3/Shift+F3 同步定位、主键对齐、Added 筛选缺失侧、
  工作表重搜、无结果/非法正则、单侧关闭、Git difftool 双侧只读、English/简体中文/日本語、
  1210×900/1440×900/1600×900，以及 104 行长表从约第 9 行连续滚到第 92 行。未观察到
  白屏、明显卡顿、工具栏闪烁、Find 压缩网格或控件重叠/溢出；所有验收 PID 均精确退出。
  证据位于忽略目录 `build/acceptance/current-sheet-find/`，关键截图包括
  `04-en-f3-synced-navigation.png`、`05-en-options-combined.png`、
  `06-en-no-result-invalid-regex.png`、`07-en-added-filter-missing-side.png`、
  `08-en-sheet-switch-rerun.png`、`10-zh-both-1210.png`、`11-ja-both-1440.png`、
  `13-en-git-difftool-readonly-1440.png`、`15-long-scroll-after.png` 和
  `16-en-shift-f3-previous-1440.png`。
- README、架构、手工验收、三语产品能力文案、未发布 changelog 与官网生成内容已同步；官网
  lint 为 0 错误和 4 条既有 `<img>` warning，16 条静态路由构建与 10/10 渲染测试通过。
  正式 Lightsail 部署未执行：当前运行环境拒绝从本机历史推断 SSH 目标，项目又要求执行者显式
  提供 `-SshHost`。在维护者明确给出本机 SSH 目标并授权后，需运行
  `scripts/deploy-site-lightsail.ps1`，再核对源站、Cloudflare 三语页面和同一 Caddy 实例既有站点。

## 2026-08-05：v0.5.0 正式发布候选

- 上一个已发布、非草稿版本为 Preview `v0.4.1`。本轮新增完整的当前工作表查找能力，属于向后兼容的
  用户工作流扩展，因此按 SemVer 推导 Minor 版本 `v0.5.0`，继续标记为 Preview。
- `product/product.json`、三语 changelog、README/CHANGELOG、CLI 版本、桌面与官网 package
  清单及 lockfile、官网生成内容和预期下载 URL 已同步为 `0.5.0`。用户可见更新只表述为：左右
  对比网格可独立搜索当前工作表，可组合大小写、Unicode 全词和 RE2 正则选项，在当前行筛选内
  循环导航并保持双栏同步定位；没有把实现任务、测试步骤或发布过程写入公开更新日志。
- 候选范围共 43 个 Git 状态路径，包括当前工作表查找的 Go/Wails/Vue 实现与回归测试、三语文案、
  README/架构/手工验收/本交接文档、产品事实、官网生成内容、版本清单及前端生产产物；没有加入
  忽略目录中的测试缓存、验收截图或本地发布产物。`git diff --check` 与内容同步检查通过。
- `go test ./...` 与 `go vet ./...` 通过；`CGO_ENABLED=1 go test -race ./...` 已尝试，但本机缺少
  `gcc`，停在 `runtime/cgo` 编译阶段，未记为通过。前端 lint、typecheck、7 个文件 73 项测试和
  production build 通过；官网 lint 为 0 错误和 4 条既有 `<img>` warning，16 条静态路由构建与
  10/10 渲染测试通过；桌面前端与官网的离线 production 依赖审计均为 0 个已知漏洞。
- 离线优先脚本使用本地 Wails v2.10.2 和项目既有 Node 垫片完成未跳过前端的 Windows amd64
  全量构建。`build/bin/SheetProof.exe version` 返回 `0.5.0`，候选为 17,459,712 字节，SHA-256
  为 `AD00312578BFF146FAD5F668E52818CAEE0B759F6E1D02E9E374E0F0AECF152B`；Windows 产物未签名，
  后续 GitHub workflow 的 macOS 产物也仍按未签名、未公证处理。
- 当次 `0.5.0` EXE 已完成真实鼠标、键盘与截图验收：双栏独立查询、F3 同步定位、组合选项、无结果、
  非法正则、Added 筛选缺失侧、工作表切换重搜和单侧关闭均符合预期。8 张当次证据位于忽略目录
  `build/acceptance/current-sheet-find/02-en-left-role.png` 至 `09-en-close-right-only.png`；截图已
  逐张目视核对，验收 PID 已精确退出。完整全量构建与验收构建的 EXE 哈希相同。
- 对 43 个候选路径、`v0.4.1..HEAD` 相关历史、官网静态产物和最终 EXE 扫描了私钥、GitHub/云令牌、
  认证 URL、Windows/macOS 私人用户路径、私网 IPv4 和敏感文件名，所有类别均为 0；未发现敏感
  数据，不需要移除文件、轮换凭据或重写历史。
- 远端只读核对确认 `origin/main` 与本地 `HEAD` 均为 `6bc6db42aeb854de1239b532a7e9d1c245ad3f90`，
  远端最高标签仍为 `v0.4.1`，不存在 `v0.5.0`。当前停在两阶段正式发布的候选确认点：尚未暂存、
  提交、推送、打标签、运行远程 Release workflow、发布 GitHub Release 或部署候选官网。维护者明确
  确认后，计划以 `Release SheetProof v0.5.0` 提交并普通推送 `main`，先对提交运行双平台 workflow，
  成功后创建 annotated `v0.5.0` 标签，核对 Draft Release 的 Windows/macOS 资产和校验和并发布
  Preview Release，最后通过 Lightsail/Caddy 脚本原子部署官网并核对源站、Cloudflare 和同实例站点。

## 2026-08-05：v0.5.0 正式发布完成

- 维护者明确确认正式发布后，43 个候选路径以 `Release SheetProof v0.5.0` 提交为
  `78886546f8e93c4761816ea5488e0f936f374652`，并以普通 fast-forward 推送到
  `origin/main`。没有强推、移动已有标签、切换分支、覆盖无关修改，忽略目录中的验收截图、缓存和
  发布审计产物也未进入提交。
- 提交级 `Build desktop release` workflow run `31009128780` 通过源码验证、Windows amd64 和
  macOS universal 构建；随后创建并推送 annotated tag `v0.5.0`，标签对象剥离后精确指向上述发布
  提交。标签 workflow run `31009514325` 再次通过源码验证、双平台构建和 Draft Release 创建。
- 下载并核对 Draft 资产、API digest 与 `SHA256SUMS.txt` 后，Windows EXE 为 16,952,832 字节，
  SHA-256 为 `91BE9DD4F22B5374F1B41B73CFAD4501247B1F3FF185ACEDEDA3F8809B6E7BDF`；
  macOS universal ZIP 为 11,077,081 字节，SHA-256 为
  `F2B091A825D2DDE631A80C02A9CE65BCDD626C20B4AD2A2FCCA0AAC83DF3A77C`；
  `SHA256SUMS.txt` 为 192 字节。EXE 返回版本 `0.5.0`，ZIP 包含完整
  `SheetProof.app/Contents/MacOS/SheetProof`。Windows 产物未签名，macOS 产物未签名、未公证。
- 整理后的用户更新说明已替换自动生成内容，Release 明确保留 Preview、未签名和未公证限制。
  GitHub prerelease `https://github.com/tadazly/sheetproof/releases/tag/v0.5.0` 于
  2026-08-05 21:28（UTC+8）公开；Release 页面、三个资产和源码下载均返回 200，资产长度与审计
  结果一致。
- Release 公开后再次通过内容同步检查、官网 lint（0 错误、4 条既有 `<img>` warning）、16 路由
  静态构建和 10/10 渲染测试。第一次官网部署尝试在上传前因 SSH 连接超时停止，远端未发生半部署；
  维护者提供可用运行时目标后，重试 `scripts/deploy-site-lightsail.ps1` 成功完成原子部署。
- 部署后 Caddy 配置有效，源站 19 项页面与资源检查、6 个版本内容页检查均通过，同一 Caddy 实例的
  5 个虚拟主机全部健康。Cloudflare 公网 15 个三语产品路由均返回 200 且具有代理响应头，6 个下载/
  更新日志页显示 `0.5.0` 并指向 `v0.5.0`，5 个关键静态资源和 3 个 Release 下载均可用。
- 最终隐私扫描覆盖发布范围、提交历史、官网产物、发布说明，以及下载并展开的 Windows/macOS 实际
  资产；私钥、GitHub/云令牌、认证 URL、Windows/macOS 私人用户路径、GitHub runner/工作区路径、
  私网 IPv4 和敏感文件名命中均为 0。未发现需要移除、轮换或重写历史的敏感数据。
- 常规 Go、前端、官网测试与桌面实机验收均已通过；`CGO_ENABLED=1 go test -race ./...` 因本机缺少
  `gcc` 停在 `runtime/cgo` 编译阶段，未记为通过，也未影响已通过的双平台 GitHub Actions 构建。
