# ugxlsx

`ugxlsx` 是面向 Git/UGit 工作流的 `.xlsx` 对比与合并工具。它提供 Wails + Vue 3 桌面界面和无界面的 CLI；差异、合并、撤销和安全保存都由同一套 Go 核心实现。

## 已实现

- `.xlsx` 工作簿、工作表集合和工作表顺序对比
- 文本、数字、布尔、日期、显式空字符串、真正空单元格、公式及类型差异
- 左右双栏、同步滚动、窗口化渲染、差异分页索引和上一处/下一处导航
- 快速连续导航请求防乱序，底部滚动时列标题保持固定
- 紧凑单元格、`Ctrl/Command + 滚轮` 缩放、左右同步列宽调整
- 按工作簿和工作表持久化缩放比例及各列宽度
- 单格、Shift 范围、鼠标拖拽范围和整行选择
- 右键菜单及批量复制差异到左侧（值、公式、样式、超链接、批注）
- 左侧文本、数字、公式和清空编辑
- 会话级复制/批量复制/编辑撤销；支持 Ctrl/Command+Z，保存后仍保留历史
- 默认保存左侧；`Ctrl/Command + Shift + S` 另存为
- 另存为默认沿用左侧文件名，首次打开系统下载目录，并记住上次成功保存目录
- 启动读取和建立差异索引时显示居中的加载窗口
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
| 缩放 | `Ctrl/Command + 鼠标滚轮`；点击百分比恢复 100% |
| 调整列宽 | 拖动列标题右边界；左右同步并按工作簿/工作表缓存 |
| 单格选择 | 点击单元格 |
| 范围选择 | Shift 扩展，或按住鼠标拖选 |
| 整行选择 | 点击或拖动行号，Shift 可扩展多行 |
| 复制到左侧 | 右键选区或点击顶部“复制到左侧” |
| 编辑左侧 | 选择单格后在底部编辑区设置文本、数字、公式或清空 |
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
internal/preferences 另存为目录偏好
internal/app        共享会话和视口 API
internal/cli        命令、JSON/text 输出、退出码
frontend            Vue 3 + TypeScript 窗口化双栏 UI
cmd/gentestdata     验收工作簿生成器
docs                架构和手工验收
```

详细设计见 [docs/architecture.md](docs/architecture.md)。
第一轮完整交付状态和第二轮接手入口见
[docs/iteration-1-handoff.md](docs/iteration-1-handoff.md)。

## 已知限制

- 不支持 `.xls`、`.xlsm`、`.ods`、CSV 和加密工作簿。
- 不编辑图片、图表、数据透视表、宏、外部连接或条件格式。
- 右侧独有工作表会显示，但首版没有“一键复制整个工作表”；可逐单元格合并的前提是工作表两侧都存在。
- 范围和整行选择只会复制选区内存在差异的单元格；单次最多提交 10,000 个坐标。
- 差异索引前端当前一次读取当前工作表最多 10,000 条，尚未提供跨页 UI。
- 不提供重做、完整公式计算、格式工具栏或 Excel 级公式编辑器。
- excelize 会重写 OOXML 包。未修改的常见内容已有回归测试，但图片、图表、透视表、外部连接、复杂条件格式和厂商私有扩展没有保真承诺。请勿用本工具保存包含宏的文件（`.xlsm` 会在打开前被拒绝）。
- CLI `compare` 的正常窗口关闭状态统一为 0，不能仅凭进程退出码判断用户是否点击保存。
