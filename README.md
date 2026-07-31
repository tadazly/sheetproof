# ugxlsx

`ugxlsx` 是面向 Git/UGit 工作流的 `.xlsx` 对比与合并工具。它提供 Wails + Vue 3 桌面界面和无界面的 CLI；差异、合并、撤销和安全保存都由同一套 Go 核心实现。

## 已实现

- 以本地 Git 工作区为入口的三栏界面：仓库 XLSX 目录树、当前工作区、其他本地/远端分支
- 从仓库子目录自动定位根目录，支持目录拖放、最近仓库自动恢复和可调目录树宽度
- 仓库侧栏提供“仓库文件 / 差异表 / 工作表与差异”三页签；目录树仅展示 `.xlsx`，
  后台索引刷新不会改变用户已展开或收起的目录
- 差异表使用可持久化的语义索引：Git 先筛出双方共有的变化候选，后台再按 Go 核心相等语义精确过滤
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
- 右键可复制/覆盖单元格或整行；冲突行还可按左侧最大数字 ID 自动续号，或逐行指定新 ID 后追加到左侧
- 冲突覆盖或追加后，右侧来源行显示处理方式；追加到左侧的新行保持新增色，撤销
  同时移除处理标记
- 左侧文本、数字、公式和清空编辑
- 左侧双击原位编辑，底部两行显示左右值、类型和差异状态
- 选中差异保留语义底色及蓝色边框；复制后真正相等的单元格立即取消差异色
- 会话级复制/批量复制/编辑撤销；支持 Ctrl/Command+Z，保存后仍保留历史
- 默认保存左侧；`Ctrl/Command + Shift + S` 另存为
- 另存为默认沿用左侧文件名，首次打开系统下载目录，并记住上次成功保存目录
- 直接文件模式启动读取时显示居中的加载窗口；仓库差异索引在目录界面中后台更新
- 同目录临时文件、写盘同步、临时文件校验、原子替换、保存后重开校验
- 基于大小、纳秒 mtime 和 SHA-256 的外部修改检测
- JSON/文本 CLI 差异报告

## 快速开始

要求 Go 1.24+、Node.js 20+。macOS 桌面构建还需要 Xcode Command Line Tools。

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

UGit 有两种常见使用方式。要把结果保存回工作区文件时，`$LOCAL` 必须是
真实工作区路径，并且不要添加 `--readonly-left`：

在 UGit 的“设置 → 工具 → 差异工具”中添加：

| 字段 | 内容 |
|---|---|
| 后缀 | `*.xlsx` |
| 工具 | `Custom` |
| 路径（macOS） | `/绝对路径/ugxlsx/build/bin/ugxlsx.app/Contents/MacOS/ugxlsx` |
| Args | `compare --left "$LOCAL" --right "$REMOTE" --left-label "当前分支" --right-label "对比版本"` |

例如当前仓库的实际路径是：

```text
/Users/luyi/splan-git/ugxlsx/build/bin/ugxlsx.app/Contents/MacOS/ugxlsx
```

部分 UGit 的提交/历史对比会把左右两个版本都导出为临时文件。这种场景
只适合查看，建议在 Args 中增加 `--readonly-left`，避免用户误以为保存
临时文件会修改工作区。配置后在 `.xlsx` 文件上选择“使用差异工具查看”，
UGit 会启动 GUI，并在窗口关闭后继续。

使用前应先确认 UGit 对当前命令提供的 `$LOCAL` 含义：

- `$LOCAL` 是工作区真实文件：可以使用可写配置，保存会修改该文件。
- `$LOCAL` 是 UGit/Git 临时文件：使用 `--readonly-left`，或另存为到新文件。

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

左侧文件是编辑和默认保存目标，右侧始终只读。只有用户点击“保存左侧”后才写回；未保存关闭时桌面端会要求确认。`--output` 将默认保存目标改为指定 `.xlsx` 路径。

“另存为”也可使用 `Ctrl+Shift+S`（macOS 为 `Command+Shift+S`）。默认文件名是左侧工作簿的原文件名；首次另存默认打开当前用户的“下载”目录，成功保存后会在系统用户配置目录中记录该目录，下一次另存从上次位置开始。macOS 使用用户配置目录和 `~/Downloads`，Windows 11 使用 AppData，并通过系统 Known Folder 获取实际下载目录（包括被系统重定向的下载目录）。

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
internal/preferences 另存目录、最近仓库、侧栏宽度、对比分支偏好和语义差异索引
internal/repository Git 工作区发现、XLSX 扫描、分支与对象读取
internal/app        共享会话和视口 API
internal/cli        命令、JSON/text 输出、退出码
frontend            Vue 3 + TypeScript 窗口化双栏 UI
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
