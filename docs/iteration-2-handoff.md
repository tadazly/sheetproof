# 第二、三轮交付与下一轮迭代交接

更新时间：2026-08-02

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
