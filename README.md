# ugxlsx

`ugxlsx` 是面向 Git/UGit 工作流的 `.xlsx` 对比与合并工具。它提供 Wails + Vue 3 桌面界面和无界面的 CLI；差异、合并、撤销和安全保存都由同一套 Go 核心实现。

## 已实现

- 以本地 Git 工作区为入口的三栏界面：仓库 XLSX 目录树、当前工作区、其他本地/远端分支
- 现代化桌面生产力界面：统一设计 Token、线性 SVG 图标、分层工具栏、紧凑来源卡与结果摘要，
  并针对最小窗口、高 DPI、键盘焦点和减少动态效果进行适配
- 独立应用图标使用左右表格、红绿差异和双向合并语义，macOS 与 Windows
  共享同一 SVG 设计源，不再使用 Wails 默认 “W” 占位图标
- 从仓库子目录自动定位根目录，支持目录拖放、最近仓库自动恢复和可调目录树宽度
- “切换仓库”提供最近打开的 10 个仓库列表；当前仓库明确标记，失效路径禁用，
  也可从弹窗继续选择其他仓库；原“打开本地仓库”目录选择入口保持不变
- 仓库侧栏提供“仓库文件 / 差异表 / 工作表与差异”三页签；目录树仅展示 `.xlsx`，
  后台索引刷新不会改变用户已展开或收起的目录
- 差异表使用可持久化的语义索引：Git 先筛出双方共有的变化候选，后台再按 Go 核心相等语义精确过滤
- 手工刷新会先校验索引签名，仓库内容未变化时直接复用；工具内保存只增量更新
  当前表格的索引成员，不重复比较其他未变化表格
- 仓库打开不等待逐表解析；缓存未命中时先显示索引中状态，未验证表不会出现，精确单元格差异数只在打开该表后显示
- 关闭窗口会取消正在执行的差异表索引，不等待剩余工作簿全部比较完成
- 其他分支通过 Git 对象读取到系统临时目录，不 checkout、switch、fetch 或修改工作区
- 本地分支优先、远端分支随后；排除当前分支和 `origin/HEAD` 等符号引用
- 按仓库记住最近选择的完整对比分支引用；无记录或引用失效时优先选择当前分支对应的远端引用
- 分支缺少同路径文件、无其他分支、Detached HEAD、Git 操作中、损坏 XLSX 等独立界面状态
- 仓库模式合并和编辑只保存到当前工作区文件，不自动 add、commit 或 push
- `.xlsx` 工作簿、工作表集合和工作表顺序对比
- 文本、数字、布尔、日期、显式空字符串、真正空单元格、公式及类型差异
- 左右双栏、同步滚动、窗口化渲染、差异分页索引和上一处/下一处导航
- 快速连续导航请求防乱序；原生 sticky 图层固定列标题和行号栏，直接拖动左右任一滚动条都不依赖事件后补偿
- 紧凑单元格、`Ctrl/Command + 滚轮` 缩放、左右同步列宽调整
- 按工作簿和工作表持久化缩放比例及各列宽度
- 单格、Shift 范围、鼠标拖拽范围和整行选择
- 使用 Git 双栏配色：增加为绿色、删除为红色、修改为左红右绿、冲突为橙色；
  “工作表与差异”同时显示四类行数量
- 差异索引提供“增加 / 删除 / 修改 / 冲突”筛选页签，并按“冲突 > 修改 > 删除
  > 增加”自动选择首个有内容的分类和索引项
- 打开表格时若当前摘要工作表没有差异，会自动进入首个有差异的工作表，并让
  左右视口同步定位到上述优先分类的第一处；编辑和保存后的局部刷新保留当前视口
- 右键可复制/覆盖单元格或整行；冲突行还可按左侧最大数字 ID 自动续号，或逐行指定新 ID 后追加到左侧
- 冲突覆盖或追加后，右侧来源行显示处理方式；追加到左侧的新行保持新增色，撤销
  同时移除处理标记
- 左侧文本、数字、公式和清空编辑
- 左侧双击原位编辑，底部两行显示左右值、类型和差异状态
- 选中差异保留语义底色及蓝色边框；复制后真正相等的单元格立即取消差异色
- 无值但带样式或默认类型元数据的单元格统一视为真正空白，复制单元格或整行后
  不会出现左右都空却仍显示红/绿色的虚假差异；显式空字符串仍完整保留
- 会话级复制/批量复制/编辑撤销；支持 Ctrl/Command+Z，保存后仍保留历史
- 默认保存左侧；`Ctrl/Command + S` 保存当前文件，`Ctrl/Command + Shift + S` 另存为
- 另存为默认沿用左侧文件名，首次打开系统下载目录，并记住上次成功保存目录
- 仓库模式“另存为”导出独立副本，不改变工作区文件保存目标，也不把未保存的
  工作区编辑误标为已保存
- 直接文件模式启动读取时显示居中的加载窗口；仓库差异索引在目录界面中后台更新
- 所有主要操作具备 hover、active、focus、disabled 和 busy 反馈；错误、空状态、
  缺失来源、无差异和后台索引使用一致的状态语言
- 同目录临时文件、写盘同步、临时文件校验、原子替换、保存后重开校验
- 基于大小、纳秒 mtime 和 SHA-256 的外部修改检测
- JSON/文本 CLI 差异报告

## 快速开始

要求 Go 1.24+、Node.js 20+。macOS 桌面构建还需要 Xcode Command Line Tools；
在 macOS 交叉构建 Windows `amd64` 版本还需要 `mingw-w64`。

```bash
cd frontend
npm install
npm run build
cd ..

make build
open build/bin/ugxlsx.app
```

不安装全局 Wails CLI 也可构建桌面发布包：

```bash
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build
```

只生成用于分发测试的最小桌面产物（不生成安装器或 zip）：

```bash
# 当前 Mac 架构
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build

# Windows 11 amd64；macOS 交叉构建前先安装 brew install mingw-w64
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build \
  -platform windows/amd64
```

产物分别为 `build/bin/ugxlsx.app` 和 `build/bin/ugxlsx.exe`。
应用图标的可维护源文件是 `build/appicon.svg`；`build/appicon.png` 用于
Wails/macOS 图标生成，`build/windows/icon.ico` 是 Windows 多尺寸资源。

macOS 上，Wails GUI 必须使用桌面构建产物：

```text
build/bin/ugxlsx.app/Contents/MacOS/ugxlsx
```

不要把普通 `go build -o build/bin/ugxlsx .` 生成的无标签 CLI
二进制配置为 UGit GUI 工具；该二进制仅适合 `diff`、`version`
等无界面命令，执行 `compare` 会被 Wails 拒绝启动。

开发模式：

```bash
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 dev
```

## CLI

### 打开本地 Git 仓库

```bash
ugxlsx repo --path "/path/to/repository"
```

可以直接定位仓库内的表格和对比分支：

```bash
ugxlsx repo \
  --path "/path/to/repository" \
  --file "config/activity/reward.xlsx" \
  --ref "origin/develop"
```

`--file` 必须是仓库根目录下的 `.xlsx` 相对路径，`--ref` 必须是当前本地
仓库已经存在的本地分支或远端跟踪分支。该命令不会执行 `git fetch`，也不会
切换当前分支。

### UGit 配置

将应用放到准备长期使用的固定位置后，可以直接点击顶部工具栏的“配置 UGit”。
应用会显示原生确认对话框，然后把 UGit 的全局 `*.xlsx` 差异工具和合并工具
注册为当前正在运行的 ugxlsx：

- 只替换 `*.xlsx` 对应的 `difftool` / `mergetool` 项，不修改其他后缀；
- 自动写入差异、合并和 `trustExitCode=false` 配置，并在写入后重新读取校验；
- 如果应用被移动，再次点击会检测当前可执行文件路径并覆盖旧路径；
- 如果任一步失败，会尝试恢复配置前的全部 `*.xlsx` 工具项；
- 已经正确配置时不会重复写入；UGit 正在运行时，配置后应重启 UGit；
- Windows 会优先使用当前版本 UGit 自带的 `cmd\\git.exe`，确认框和结果中显示
  实际 Git 路径及配置来源，避免把配置静默写入另一套 Git 上下文。

macOS 注册 `.app/Contents/MacOS/ugxlsx`，Windows 注册当前 `ugxlsx.exe`。
macOS App Translocation 或其他系统临时目录中的程序不会被注册；应先把应用移动
到固定目录。macOS 优先使用系统 `PATH` 中的 Git；Windows 优先选择当前版本
UGit 自带的 Git，再回退到 `PATH` 和常见 UGit/Git 安装位置。

Windows 的后台 Git/UGit CLI 子进程使用无控制台窗口的进程属性，避免建立仓库
差异索引时由 `git.exe` / `conhost.exe` 产生临时任务栏图标。该设置不作用于
ugxlsx 主窗口，也不改变 macOS/Linux 行为。Windows 的未保存确认使用原生三按钮
对话框，关闭或切换时均提供“保存并继续 / 不保存并继续 / 取消”。原生
`TaskDialogIndirect` 调用会锁定当前 OS 线程并初始化 STA，避免首次真正显示确认框时
返回 `HRESULT 0x80070057`。

如需手工配置，在 UGit 的“设置 → 工具 → 差异工具”中添加：

| 字段 | 内容 |
|---|---|
| 后缀 | `*.xlsx` |
| 工具 | `Custom` |
| 路径（macOS） | `/绝对路径/ugxlsx/build/bin/ugxlsx.app/Contents/MacOS/ugxlsx` |
| 路径（Windows） | `C:\绝对路径\ugxlsx.exe` |
| Args | `compare --left "$LOCAL" --right "$REMOTE"` |

例如当前仓库的实际路径是：

```text
/Users/luyi/splan-git/ugxlsx/build/bin/ugxlsx.app/Contents/MacOS/ugxlsx
```

Git difftool 会设置标准的 `GIT_DIFF_PATH_COUNTER` 和
`GIT_DIFF_PATH_TOTAL` 环境变量。`ugxlsx` 检测到这些变量后自动把整个差异
会话设为只读，不依赖 Args 是否显式带 `--readonly-left`。界面两侧显示
“Git 差异快照 · 只读”，隐藏冗长临时目录，并禁用编辑、复制合并、撤销、
另存和保存。配置后在 `.xlsx` 文件上选择“使用差异工具查看”，UGit 会启动
GUI，并在窗口关闭后继续。

UGit 5.51.0 还会为每次调用设置 `LOCAL_TITLE`、`REMOTE_TITLE` 和
`WORKSPACE_PATH`。未显式传入 `--left-label` 或 `--right-label` 时，
`ugxlsx` 自动使用前两个变量显示真实的两侧分支/引用名；UGit 常规双分支比较
中分别对应命令里的左、右引用。显式 label 参数仍可逐侧覆盖自动值。它们是
UGit 扩展变量，不是普通 `git difftool` 保证提供的标准变量。

Git 对新增或删除的文件会把不存在的一侧传为 `/dev/null`（Windows 环境也可能
使用 `NUL`）。`compare` 会在系统临时目录创建一个工作表结构相同、但不含数据的
占位 `.xlsx`，继续使用正常差异会话显示新增/删除内容；窗口关闭后自动清理，
不会向用户仓库写入临时文件。

需要修改工作区文件时应使用 ugxlsx 自身的仓库模式，或直接以普通 `compare`
命令打开真实文件；不要把 Git difftool 当作可写入口。

在 UGit 的“合并工具”中应单独配置输出目标，不能直接复用上面的差异参数：

| 字段 | 内容 |
|---|---|
| 后缀 | `*.xlsx` |
| 工具 | `Custom` |
| 路径 | 与差异工具相同 |
| Args | `compare --left "$LOCAL" --right "$REMOTE" --output "$MERGED"` |

当前是基于 `$LOCAL` 和 `$REMOTE` 的双向表格语义合并，不读取 `$BASE`。只有用户
点击保存后，结果才通过安全保存流程写入 `$MERGED`；不要省略 `--output
"$MERGED"`，否则 Git 提供的临时 `$LOCAL` 可能成为默认保存目标。

如果点击后无窗口，先在终端检查桌面二进制：

```bash
/绝对路径/ugxlsx.app/Contents/MacOS/ugxlsx version
```

直接从脚本启动 GUI 的示例：

```bash
/绝对路径/ugxlsx.app/Contents/MacOS/ugxlsx compare \
  --left "${LOCAL_FILE}" \
  --right "${REMOTE_FILE}" \
  --left-label "当前分支" \
  --right-label "目标分支"
```

其他 GUI 参数：

```text
--title TEXT
--readonly-left
--output FILE
```

普通 `compare` 的左侧文件是编辑和默认保存目标，右侧始终只读。只有用户点击
“保存左侧”后才写回；未保存关闭时桌面端会要求确认。`--output` 将默认保存
目标改为指定 `.xlsx` 路径。Git difftool 启动的 `compare` 会自动覆盖为全局
只读；Git mergetool 不带 difftool 环境标记，因此仍可编辑。

使用 `Ctrl+S`（macOS 为 `Command+S`）保存当前文件；“另存为”使用
`Ctrl+Shift+S`（macOS 为 `Command+Shift+S`）。默认文件名是左侧工作簿的
原文件名；首次另存默认打开当前用户的“下载”目录，成功保存后会在系统用户
配置目录中记录该目录，下一次另存从上次位置开始。仓库模式下该操作是“导出
副本”：不会改变当前工作区保存目标，导出后工作区中尚未保存的编辑仍保持
未保存状态。macOS 使用用户配置目录和 `~/Downloads`，Windows 11 使用
AppData，并通过系统 Known Folder 获取实际下载目录（包括被系统重定向的下载
目录）。

### GUI 操作速查

| 操作 | 使用方式 |
|---|---|
| 上一处/下一处差异 | 顶部按钮；选中后左右表格同步定位 |
| 表格缩放 | `Ctrl/Command + 鼠标滚轮`；工具栏显示“缩放 100%”，点击可恢复 100% |
| 调整列宽 | 拖动列标题右边界；左右同步并按工作簿/工作表缓存 |
| 单格选择 | 点击单元格 |
| 范围选择 | Shift 扩展，或按住鼠标拖选 |
| 整行选择 | 点击或拖动行号，Shift 可扩展多行 |
| 复制单元格/整行到左侧 | 在增加、删除或修改的单元格/行上右键 |
| 处理冲突行 | 右键后覆盖单元格/整行，或用自动/指定 ID 将右侧整行追加到左侧 |
| 编辑左侧 | 双击左侧单元格直接编辑；Enter 或失焦提交，Esc 取消 |
| 撤销 | `Ctrl/Command + Z`；保存后撤销历史仍保留 |
| 保存当前文件 | `Ctrl/Command + S` |
| 另存为 | `Ctrl/Command + Shift + S` |

无界面对比：

```bash
ugxlsx diff --left A.xlsx --right B.xlsx --format json
ugxlsx diff --left A.xlsx --right B.xlsx --format text
```

`diff` 在“文件相同”和“文件不同”两种成功对比中均返回 `0`，脚本通过 JSON 的 `equal` 字段判断。这避免把正常发现差异误报为运行错误。

### 退出码

| 代码 | 含义 |
|---:|---|
| 0 | 命令成功（`diff` 是否相同见 `equal`） |
| 1 | 参数错误或普通运行错误 |
| 2 | 文件不存在、不可读、损坏或无工作表 |
| 3 | 保存失败或检测到外部修改 |
| 4 | 不支持的文件格式 |
| 5 | 为宿主集成预留的用户取消/未保存退出码；当前 GUI 正常关闭返回 0 |

Wails 的窗口生命周期不会可靠区分“查看后关闭”和“保存后关闭”，所以首版 `compare` 正常关闭统一返回 0；UGit 应通过工作区文件是否变化判断是否保存。退出码 5 保留给后续结果文件协议。

## 测试和验证

```bash
go fmt ./...
go vet ./...
go test ./...
go test -race ./...
go test -bench=. -benchmem ./...

cd frontend
npm run lint
npm run typecheck
npm run test
npm run build

cd ..
./scripts/verify_cli.sh
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build
```

集成测试完全由代码生成工作簿，覆盖多数据类型、中文/特殊字符、多工作表、公式、样式、合并单元格、行高列宽、超链接、批注、安全保存、保存后重开、失败不损坏和外部修改检测。完整 GUI 手工流程见 [docs/manual-acceptance.md](docs/manual-acceptance.md)。

## Benchmark

基准包括：

- 10 个 sheet / 10 万有效单元格 / 完全相同
- 10 个 sheet / 10 万有效单元格 / 1 万差异
- 10 个 sheet / 100 万有效单元格 / 少量差异

运行 `go test -bench=. -benchmem ./...`。实际机器结果记录在 [docs/benchmark.md](docs/benchmark.md)；基准构造在计时区外，测量的是差异归并和索引结果构建。

## 项目结构

```text
internal/workbook   稀疏快照、类型与文件身份
internal/diff       O(n) 差异算法
internal/merge      单元格捕获和跨工作簿应用
internal/history    撤销栈
internal/storage    安全写入
internal/preferences 另存目录、最近 10 个仓库、侧栏宽度、对比分支偏好和语义差异索引
internal/repository Git 工作区发现、XLSX 扫描、分支与对象读取
internal/ugit       UGit *.xlsx 外部工具检测、事务式注册和路径更新
internal/app        共享会话和视口 API
internal/cli        命令、JSON/text 输出、退出码
frontend            Vue 3 + TypeScript 窗口化双栏 UI
build               跨平台应用图标源、平台资源和最小桌面产物
cmd/gentestdata     验收工作簿生成器
docs                架构和手工验收
.agents/skills      项目级 Codex 接手与验证技能
AGENTS.md            项目协作规则和兼容不变量
```

详细设计见 [docs/architecture.md](docs/architecture.md)。
当前工作区状态和后续迭代入口见
[docs/iteration-2-handoff.md](docs/iteration-2-handoff.md)；可复制的新会话提示词见
[docs/new-session-prompt.md](docs/new-session-prompt.md)。第一轮历史事实保留在
[docs/iteration-1-handoff.md](docs/iteration-1-handoff.md)。

## 已知限制

- 不支持 `.xls`、`.xlsm`、`.ods`、CSV 和加密工作簿。
- 仓库模式不负责 fetch、pull、push、add、commit、分支创建/删除/切换或冲突恢复。
- 不编辑图片、图表、数据透视表、宏、外部连接或条件格式。
- 右侧独有工作表会显示，但首版没有“一键复制整个工作表”；可逐单元格合并的前提是工作表两侧都存在。
- 顶部批量复制只提交选区中的差异坐标；右键“复制/覆盖整行”会提交所选行的完整列范围。单次最多处理 10,000 个单元格。
- 冲突行识别依赖首行名为 `id`（不区分大小写）的列，且当前按左右相同物理行判断；没有 `id` 列时仍分类增加/删除/修改，但不提供冲突追加菜单。
- 差异索引前端当前一次读取当前工作表最多 10,000 条，尚未提供跨页 UI。
- 不提供重做、完整公式计算、格式工具栏或 Excel 级公式编辑器。
- excelize 会重写 OOXML 包。未修改的常见内容已有回归测试，但图片、图表、透视表、外部连接、复杂条件格式和厂商私有扩展没有保真承诺。请勿用本工具保存包含宏的文件（`.xlsm` 会在打开前被拒绝）。
- CLI `compare` 的正常窗口关闭状态统一为 0，不能仅凭进程退出码判断用户是否点击保存。
