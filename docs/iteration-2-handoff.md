# 第二、三轮交付与下一轮迭代交接

更新时间：2026-08-01

本文是新会话接手 `ugxlsx` 的当前事实基线。第一轮历史交付保留在
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
- 本机 UGit 是 5.51.0，安装于
  `C:\\Users\\luyi\\AppData\\Local\\UGit\\app-5.51.0`。UGit 自带 Git 2.45.2，
  系统 PATH Git 是 2.37.1.windows.1。Windows 配置逻辑现优先按自然版本选择
  UGit 自带 `cmd\\git.exe`，并回读 `--show-origin`；界面显示 Git 路径和配置来源。
- 已在真实全局配置中覆盖错误 XLSX 旧路径，先写入带中文和空格的固定测试路径，
  再覆盖为最终 `build\\bin\\ugxlsx.exe`；回读来源为
  `file:C:/Users/luyi/.gitconfig`，命令中的 `$LOCAL`、`$REMOTE`、`$MERGED`
  保持原样。临时 EXE 已删除。UGit 已强制退出并重启，且从 UGit 打开 250 变更验证仓库。
  由于 Windows 10 图形捕获接口不支持当前自动化截图，尚未从 UGit 文件行真实
  触发差异和合并各一次。
- Windows EXE 原本已嵌入正确 ICO，但 Wails 只设置 `ICON_SMALL`；现在
  `OnDomReady` 后补设 `ICON_SMALL`、`ICON_SMALL2` 和 `ICON_BIG`。资源管理器、
  任务栏和 Alt+Tab 的目视复核仍需真正的 Windows 11 会话完成。

本轮自动化结果：`go test ./...`、`go vet ./...`、前端 lint/typecheck、24 项测试、
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
自动成为两侧展示名，显式参数逐侧优先。UGit
合并工具必须用 `--output "$MERGED"` 指定结果路径；当前仍是
`$LOCAL`/`$REMOTE` 双向语义合并，不读取 `$BASE`。

顶部低频操作“配置 UGit”可把当前 macOS `.app` 内部可执行文件或 Windows
`.exe` 注册为全局 `*.xlsx` 差异/合并工具。它先检测现有 XLSX 项，经原生对话框
确认后只替换该后缀，并保持 `trustExitCode=false`；写后精确校验，失败时恢复
快照。应用移动后再次点击会覆盖旧路径；临时/App Translocation 路径不允许
注册。UGit 运行中完成配置后需要重启 UGit。

### CLI 仓库入口

```bash
ugxlsx repo --path "/path/to/repository"

ugxlsx repo \
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
  多行覆盖/追加作为一个撤销步骤，完成后增量更新差异和行分类。
- 冲突覆盖/追加会在右侧来源行显示可撤销的处理标记。已复制相等的单元格立即
  清除差异色；追加目标行在左侧显示新增绿，不沿用纯坐标对比产生的删除红。
- Go 后端的相等判断仍是 `present + raw + formula + type`；相同显示文本并不
  自动代表相等，底部类型信息用于解释真实类型差异。
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
- 双击原位编辑、Enter 提交、Esc 取消。
- 数字形文本跟随右侧文本类型。
- 编辑到相等后差异 class 消失；选中差异保留 difference/selected/active 状态。
- 冲突整行视觉、工作表冲突计数、四项右键操作、多行自动 ID 范围和逐行指定 ID。
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
GOCACHE=/tmp/ugxlsx-go-cache go test ./...
cd frontend && npm run test && npm run typecheck
```

可直接复制的接手提示词见 `docs/new-session-prompt.md`。
