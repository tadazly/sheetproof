# 第二轮交付与后续迭代交接

更新时间：2026-07-30

本文是新会话接手 `ugxlsx` 的当前事实基线。第一轮历史交付保留在
`docs/iteration-1-handoff.md`；项目约束见根目录 `AGENTS.md`，实现设计见
`docs/architecture.md`。

## 当前工作区状态

当前分支是 `main`，当前提交仍为：

```text
ab582d1 first commit
```

第二轮仓库模式、后续目录树和编辑体验优化，以及本文档都尚未形成新提交。
`git status --short` 中的大量修改和新增文件是需要保留的有效成果，不是可以
清理的临时内容。新会话不得执行 `git reset --hard`、`git checkout --` 或
其他会丢失这些改动的操作。

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
- 顶部显示仓库名称、完整路径、当前分支、Detached HEAD、工作区修改和进行中
  Git 操作。
- 目录树仅显示包含 `.xlsx` 的目录和 `.xlsx` 文件，隐藏 `.git`，支持中文、
  空格、多层路径、展开折叠、刷新、拖动调宽和持久化宽度。
- 目录树使用紧凑小字号；顶部搜索框按完整相对路径筛选文件和目录，搜索时自动
  展开匹配路径。
- 仓库侧栏有“仓库文件 / 工作表与差异”页签；后者保留原有 Sheet 切换和
  差异索引。
- 当前分支从本地分支候选中排除；本地分支排在远端之前；组内自然排序；
  `origin/HEAD` 等符号引用排除。
- 默认选择第一个其他本地分支，没有时选择第一个远端分支；不自动 fetch。
- 右侧使用完整 `refs/heads/...` 或 `refs/remotes/...`，不因同名分支选错。
- 右侧通过 Git 对象导出到系统临时目录，不切换分支、不修改工作区；临时文件
  使用唯一目录并在替换来源或退出时清理。
- 未选择文件、正在加载、正在比较、无其他分支、未选引用、引用中缺少同路径
  文件、无效 XLSX 和读取失败都有独立界面状态，不使用空白网格冒充。
- 切换文件/仓库时如有工具内未保存编辑，支持“保存并继续 / 不保存并继续 /
  取消”。仅切换右侧引用会保留左侧内存编辑并重新计算差异。
- 仓库模式保存目标固定为 `<root>/<relative.xlsx>`；不执行 add、commit、
  push 或任何 Git 恢复操作。保存后重新读取 Git 状态。
- merge、rebase、cherry-pick 等操作进行中时可以查看，保存前显示警告。

### 直接文件模式

第一轮直接选择两个 `.xlsx` 的能力完整保留，仍使用同一套 Session、差异、
合并、撤销和安全保存逻辑。CLI `compare`、`diff`、UGit 集成约定不变。

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

- 左右表格同步滚动、窗口化区域加载、同步列宽、缩放、差异导航、范围/整行
  选择和批量复制继续有效。
- 左侧双击单元格原位编辑；Enter 或失焦提交，Esc 取消。右侧始终只读。
- 空字符串清空、`=` 开头作为公式、普通数字作为数字、其他内容作为文本。
- 当输入值与右侧原始值完全相同，会优先沿用右侧数字/文本类型。这解决了右侧
  是文本 `"123"`、左侧输入被误判为数字后肉眼相同但仍持续标黄的问题。
- 底部旧编辑表单已移除，替换为两行差异栏，显示当前工作区值/类型、对比来源
  值/类型和差异状态。
- 未选中的差异单元格为黄色；选中的差异单元格保留黄色底色和橙色标记，同时
  使用蓝色选择边框；无差异选中项使用普通蓝色。编辑后真正相等时黄色消失。
- Go 后端的相等判断仍是 `present + raw + formula + type`；相同显示文本并不
  自动代表相等，底部类型信息用于解释真实类型差异。

## 关键实现位置

| 路径 | 当前职责 |
|---|---|
| `controller.go` | Wails 生命周期、目录/文件对话框、仓库状态机、未保存确认、异步代次 |
| `internal/repository/repository.go` | Git 根发现、XLSX 扫描、分支读取、对象导出、状态检测 |
| `internal/app/session.go` | 左侧会话、替换/卸载右侧、差异、编辑、撤销和保存 |
| `internal/cli/cli.go` | `repo`、`compare`、`diff` 等参数和退出码 |
| `internal/preferences/save_location.go` | 最近保存目录、最近仓库和侧栏宽度 |
| `frontend/src/App.vue` | 两种入口、仓库状态、目录树页签、网格、内联编辑和差异栏 |
| `frontend/src/style.css` | 三栏布局、目录树、内联编辑、选择/差异视觉层级 |
| `frontend/src/backend.ts` | Wails Controller 的前端适配 |
| `frontend/src/types.ts` | Summary、Region 和 Repository 前端类型 |
| `frontend/src/App.test.ts` | 仓库状态、竞态、页签/搜索、编辑和选择交互测试 |

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
- 引用中存在/不存在同路径文件、对象读取不切分支且不修改工作区。
- 分支名和文件路径参数安全。
- 最近仓库恢复、仓库移动后的回退、侧栏宽度偏好。
- 仓库完整对比、合并、编辑、撤销、保存及 Git 状态/分支不变。
- 未保存状态下切换的保存、丢弃和取消路径。
- 目录拖放首项处理和文件拒绝。
- 原有读取、差异、批量复制、撤销、外部修改检测和安全保存。

前端共有 13 个测试，覆盖：

- 仓库优先空状态和直接双文件次入口。
- 目录树、搜索、明确空状态和缺失引用状态。
- 仓库侧栏工作表/差异页签。
- 快速文件切换时忽略过期结果。
- 加载提示、另存快捷键、差异导航和底部区域滚动。
- 拖选、右键批量复制和撤销。
- 双击原位编辑、Enter 提交、Esc 取消。
- 数字形文本跟随右侧文本类型。
- 编辑到相等后差异 class 消失；选中差异保留 difference/selected/active 状态。

## 最近验证快照

2026-07-30 在当前源码状态执行：

```text
GOCACHE=/tmp/ugxlsx-go-cache go test ./...        PASS
cd frontend && npm run lint                       PASS
cd frontend && npm run typecheck                  PASS
cd frontend && npm run test                       PASS（13 tests）
cd frontend && npm run build                      PASS
git diff --check                                  PASS
Wails v2.10.2 build（darwin/arm64）               PASS
```

桌面产物：

```text
build/bin/ugxlsx.app
build/bin/ugxlsx.app/Contents/MacOS/ugxlsx
```

本次最后阶段没有重新执行 `go vet ./...`、`go test -race ./...`，也没有用真实
GUI 逐项重跑 `docs/manual-acceptance.md` 的全部仓库模式流程。发布前应补做，
并分别记录自动化、桌面构建和实机验收结果。

## 后续风险与建议

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
