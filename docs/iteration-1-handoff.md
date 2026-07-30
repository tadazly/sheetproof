# 第一轮交付与第二轮迭代交接

更新时间：2026-07-30

> 本文保留第一轮历史事实，不再代表当前工作区。第二轮仓库模式及后续 UI
> 优化的当前基线见 `docs/iteration-2-handoff.md`；新会话应优先读取后者。

本文是新会话接手 `ugxlsx` 的事实基线。README 面向使用者，
`architecture.md` 面向实现设计；本文集中记录第一轮实际完成范围、行为
约定、验证证据、已知边界和第二轮开始时应保留的技术约束。

## 当前状态

第一轮核心流程已经可以运行：

```text
两个 .xlsx 路径
→ Wails GUI 异步加载
→ 查看工作表和单元格差异
→ 单格/范围/整行选择
→ 将选区内右侧差异复制到左侧
→ 编辑左侧单元格
→ 分步撤销
→ 安全保存或另存为
→ 重新打开校验
→ UGit 通过 compare 命令调用
```

当前 macOS arm64 桌面产物：

```text
build/bin/ugxlsx.app
build/bin/ugxlsx.app/Contents/MacOS/ugxlsx
```

普通 `go build` 生成的二进制只应用于无界面的 `diff`、`version` 等命令，
不能代替带 Wails 构建标签和资源的桌面产物启动 `compare`。

## 技术基线

| 层 | 当前选择 |
|---|---|
| 核心语言 | Go，模块声明 Go 1.24 |
| Excel | excelize v2.10.1 |
| 桌面 | Wails v2.10.2 |
| 前端 | Vue 3.5 + TypeScript 5.7 + Vite 6 |
| 前端测试 | Vitest + Vue Test Utils + jsdom |
| Windows 系统 API | `golang.org/x/sys/windows` |
| 支持格式 | 仅 `.xlsx` |

核心比较、合并、撤销和保存都在 Go 中，前端不拥有 Excel 文件状态，也不
实现差异算法。

## 已实现功能清单

### 文件和启动

- GUI 无参数启动后可以分别选择左侧和右侧 `.xlsx`。
- `ugxlsx compare --left ... --right ...` 直接启动并异步加载两个文件。
- 左侧是默认编辑/保存目标，右侧始终只读。
- 顶部显示左右标签和完整路径。
- 支持 `--title`、`--left-label`、`--right-label`、`--readonly-left`、
  `--output`。
- 启动读取和建立差异索引期间显示居中的加载提示。
- 相同路径、文件不存在、不可读、损坏、无工作表和非 `.xlsx` 都返回明确
  错误。
- 存在未保存修改时关闭窗口，会弹出“取消 / 丢弃并关闭”确认。

### 工作表与单元格差异

- 识别两边共有、仅左侧、仅右侧的工作表。
- 识别工作表顺序不同。
- 工作表列表显示状态和差异数量。
- 差异比较依据：
  - 单元格是否真实存在；
  - 原始值；
  - 公式文本；
  - 单元格类型。
- 能区分空单元格、显式空字符串、数字 `0`、布尔值、日期、公式和普通
  字符串。
- `display` 值用于界面显示，但当前不参与相等判断。
- 样式不作为差异类型；`StyleID` 不参与相等判断。
- 工作表状态：`equal`、`modified`、`left-only`、`right-only`。
- 单元格状态：`unchanged`、`left-added`、`right-added`、`modified`、
  `left-missing`、`right-missing`。
- 状态定义在 `internal/diff/diff.go`。

### 差异界面和大表渲染

- 左侧工作表栏、顶部工具栏、左右并排网格、底部编辑区和状态栏。
- 左右滚动同步，选择差异时同步定位。
- 差异、当前选区、当前活动单元格有不同视觉状态。
- 当前前端只请求 48 行 × 20 列视口；后端 Region API 上限是
  300 行 × 100 列。
- 网格画布保持逻辑尺寸，只创建当前区域 DOM，不渲染完整大表。
- `Ctrl/Command + 鼠标滚轮` 缩放，范围 70%～180%。
- 拖动列标题边界调整列宽，范围 48～420 基础像素。
- 缩放和列宽按“左侧工作簿绝对路径 + 工作表名”保存在 WebView
  `localStorage`。
- 上一处/下一处和差异索引可以定位差异，并显示当前序号。
- 快速导航具有区域请求序号，过期响应不会覆盖最新响应。
- 导航到工作表底部/右侧时使用浏览器实际限制后的滚动位置，列标题和虚拟
  区域不会漂移到画面中央。

### 选择、合并和编辑

- 支持点击单格。
- 支持 Shift 扩展矩形选区。
- 支持鼠标拖动矩形选区。
- 支持点击行号选择整行，Shift 或拖动选择多行。
- 单元格和行号右键菜单提供“复制到左侧”。
- 范围/整行复制只提交选区内实际存在差异的坐标。
- 单次批量复制后端上限 10,000 个坐标，并去重。
- 支持普通值、数字、布尔、日期、公式、空值、样式、超链接和批注复制。
- 批量复制中途失败时倒序回滚已经应用的项目。
- 左侧单格编辑支持文本、数字、公式和清空。
- 不支持范围批量输入，也不提供格式工具栏。

### 撤销语义

- `Ctrl/Command + Z` 撤销。
- 多次独立复制或编辑可以逐步撤销。
- 一次批量复制作为一个撤销步骤。
- 保存不会清空撤销历史。
- 保存后撤销会重新标记为“有未保存修改”；再次保存才把撤销结果落盘。
- 撤销使用单调状态编号比较当前状态和最近保存状态，能够正确处理
  “修改 → 撤销 → 新修改”的分支。
- 当前没有重做。

### 保存和另存为

- “保存左侧”默认写入加载时的左侧路径；`--output` 可以改变初始目标。
- `Ctrl/Command + Shift + S` 打开系统原生另存为窗口。
- 另存为默认文件名沿用原左侧文件名。
- 首次另存默认目录：
  - macOS：`~/Downloads`，不存在时退回用户主目录；
  - Windows 11：通过 Known Folder API 获取 Downloads，支持系统重定向，
    获取失败时退回用户主目录。
- 只在另存成功后记录最后目录，下一次优先使用。
- macOS 偏好位于用户配置目录下的 `ugxlsx/preferences.json`；
  Windows 位于 AppData 对应用户配置目录。
- 取消另存为是正常操作，不显示错误。
- 另存成功后会话保存目标切换到新文件，之后“保存左侧”继续写新目标。

安全保存顺序：

1. 校验扩展名、目录和写权限。
2. 对当前左侧目标计算 `size + mtime(ns) + SHA-256`，检测外部修改。
3. 在目标同目录写 `.ugxlsx-*.xlsx` 临时文件。
4. `fsync` 并用 excelize 重新打开临时文件。
5. 替换前再次检测外部修改。
6. 原子替换目标；Windows 使用 `MoveFileEx` 的 replace/write-through 标志。
7. 同步目录并重新打开最终文件验证。

任何替换前失败都会删除临时文件并保留原目标。保存期间重复点击由前端忙碌
状态和后端会话锁阻止重入。

### 错误和警告

- GUI 后端错误通过统一 `guard` 显示在顶部错误栏，操作结束后恢复按钮状态。
- CLI 错误写入 stderr，并按错误代码映射退出码。
- 切换文件时先完整打开新会话，成功后才替换旧会话。
- `--readonly-left` 会在后端再次阻止复制、编辑和保存，不只依赖按钮禁用。
- 批量复制坐标非法、数量超过上限或工作表任一侧不存在时明确失败。
- 数字编辑先经过 `ParseFloat`，非法数字不会写入工作簿。
- 保存会区分目录不存在、无写权限、文件只读、外部修改、临时写入失败、
  替换失败和保存后校验失败。
- 回滚或撤销恢复失败等可继续运行的诊断进入会话 `warnings`，状态栏显示
  最近一条；值、公式、样式、超链接和批注路径都实现了明确错误返回。
- 图片、图表等不在单元格捕获范围内的高级对象不会逐项产生界面警告，其
  保真风险通过格式拒绝和“当前限制”统一说明。

### CLI

已实现：

```bash
ugxlsx compare --left A.xlsx --right B.xlsx
ugxlsx diff --left A.xlsx --right B.xlsx --format json
ugxlsx diff --left A.xlsx --right B.xlsx --format text
ugxlsx version
ugxlsx help
```

`diff` 的 JSON 输出包含 `equal`、左右路径、工作表数量、不同工作表数量、
单元格差异总数和每张表摘要。发现正常差异仍返回退出码 0，脚本通过
`equal` 判断。

退出码：

| 代码 | 含义 |
|---:|---|
| 0 | 正常完成 |
| 1 | 参数或普通运行错误 |
| 2 | 文件读取、损坏、缺失或无工作表 |
| 3 | 保存、写权限或外部修改错误 |
| 4 | 不支持的格式 |
| 5 | 为后续宿主结果协议保留；当前 GUI 正常关闭统一为 0 |

## UGit 集成约定

可写工作区合并：

```bash
ugxlsx compare \
  --left "$LOCAL" \
  --right "$REMOTE" \
  --left-label "当前分支" \
  --right-label "对比版本"
```

此模式的前提是 `$LOCAL` 确实指向工作区文件。点击保存会真实修改它。

只读提交/历史查看：

```bash
ugxlsx compare \
  --left "$LOCAL" \
  --right "$REMOTE" \
  --readonly-left \
  --left-label "本地版本" \
  --right-label "历史版本"
```

部分 UGit 操作会把 `$LOCAL` 也导出成临时文件。第二轮如果继续增强 UGit
集成，必须先确认 UGit 不同入口的变量语义，不能假设 `$LOCAL` 永远是
工作区真实路径。

macOS 的 UGit “路径”必须指向：

```text
/绝对路径/build/bin/ugxlsx.app/Contents/MacOS/ugxlsx
```

## 代码导航

| 路径 | 职责 |
|---|---|
| `main.go` | CLI 入口、Wails 启动和窗口配置 |
| `controller.go` | Wails API、文件对话框、异步启动 |
| `internal/workbook` | `.xlsx` 校验、文件身份、稀疏快照、类型分类 |
| `internal/diff` | O(n) 工作表/单元格比较和增量差异更新 |
| `internal/merge` | 单元格完整状态捕获和应用 |
| `internal/history` | 批量撤销条目和状态编号 |
| `internal/storage` | 临时写入、外部修改检查、原子替换、重开校验 |
| `internal/preferences` | 另存目录偏好和平台下载目录 |
| `internal/app` | 会话、区域 API、复制、编辑、撤销和保存 |
| `internal/cli` | 命令参数、报告和退出码 |
| `frontend/src/App.vue` | 双栏网格、虚拟视口、选择、导航和快捷键 |
| `frontend/src/gridSelection.ts` | 选区范围工具 |
| `frontend/src/diffNav.ts` | 差异索引循环导航 |
| `cmd/gentestdata` | 生成手工验收工作簿 |

## 当前限制

- 不支持 `.xls`、`.xlsm`、`.ods`、CSV 和加密工作簿。
- 不编辑图片、图表、数据透视表、宏、外部连接和条件格式。
- 右侧独有工作表可以显示和统计，但不能整表复制到左侧。
- 单元格复制要求该工作表两边都存在。
- 样式差异不参与差异判断。
- 当前工作表差异索引前端一次最多读取 10,000 条，尚无跨页 UI。
- 不支持重做、范围批量输入、公式计算引擎和完整格式编辑。
- excelize 会重写 OOXML 包；常见样式和结构已有回归测试，但高级对象和
  厂商私有扩展不承诺完全保真。
- GUI 正常关闭统一返回 0，宿主不能只靠进程退出码判断用户是否保存。
- macOS arm64 已执行真实 GUI 验收；Windows 11 的下载目录代码已完成
  amd64 目标编译检查，但第一轮没有在 Windows 11 实机完成完整 Wails
  GUI 验收。

## 第一轮验证快照

最后一次完整验证：

```text
go test ./...                         PASS
go vet ./...                          PASS
go test -race ./...                   PASS
npm run lint                          PASS
npm run typecheck                     PASS
npm run test                          PASS（10 tests）
npm run build                         PASS
Windows preferences amd64 编译检查    PASS
wails build                           PASS（darwin/arm64）
CLI 与保存/重开集成测试                PASS
真实大表快速导航 GUI 验收              PASS
```

真实 GUI 回归使用 81 处差异的大表，连续快速导航到约第 28776 行，确认：

- 左右同步定位；
- 表头固定在顶部；
- 没有白屏；
- 没有过期区域回跳。

此前还实际验证过：

- 远端第 28756 行复制到左侧后不白屏；
- 保存后 `Command+Z` 可以恢复该差异；
- 撤销后重新进入未保存状态；
- `Command+Shift+S` 打开 Downloads，并沿用 `left.xlsx` 文件名。

性能结果及测试环境见 `docs/benchmark.md`，手工验收步骤见
`docs/manual-acceptance.md`。

## 第二轮开始建议

新会话应先执行：

```bash
pwd
sed -n '1,360p' docs/iteration-1-handoff.md
go test ./...
cd frontend && npm run test && npm run typecheck
```

实施第二轮时应保持以下不变量：

1. 左侧是唯一可写工作簿，右侧永远只读。
2. 核心差异与文件修改留在 Go 后端。
3. 不把整份工作簿一次性发送到前端。
4. 任何保存增强都不能绕过临时文件、外部修改检查和重开校验。
5. 新的合并操作必须进入可撤销命令栈。
6. 快速导航和滚动修改必须保留请求防乱序与实际滚动值逻辑。
7. 新增文件格式前必须先定义保真边界，尤其是 `.xlsm`。

可复制到新会话的开场说明：

```text
请先阅读 docs/iteration-1-handoff.md、README.md 和
docs/architecture.md，以第一轮已实现并验证的行为作为兼容基线。
不要回退安全保存、保存后可撤销、虚拟区域请求防乱序、UGit 左右文件角色
和文件保真限制。完成第二轮需求后同步更新交接文档和手工验收步骤。
```
