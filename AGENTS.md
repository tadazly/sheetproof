# ugxlsx 项目协作规则

## 开始工作前

1. 完整阅读 `docs/iteration-2-handoff.md`、`README.md` 和
   `docs/architecture.md`。
2. 查看 `git status --short`。当前第二、三轮成果可能尚未全部提交；这些改动
   属于项目基线，不得用 `git reset`、`git checkout --` 或覆盖式操作丢弃。
3. 涉及 GUI 行为时继续阅读 `docs/manual-acceptance.md`。
4. 优先使用项目技能 `.agents/skills/ugxlsx-development/SKILL.md`。

## 必须保持的兼容不变量

- 保留“直接打开左右两个 `.xlsx`”和“打开本地 Git 仓库”两种入口。
- 两种入口复用 `internal/app.Session`、`internal/diff`、`internal/merge`、
  `internal/history` 和 `internal/storage`，不得另写一套表格逻辑。
- 左侧是唯一可写来源；仓库模式左侧必须读取工作区真实文件，包括未提交修改。
- 右侧永远只读。仓库引用必须通过 Git 对象读取，不得 checkout、switch、
  fetch，也不得把临时文件放进用户仓库。
- 仓库文件始终使用仓库根目录相对路径，不能只用文件名匹配。
- Git 命令必须使用参数数组、明确的 `git -C <root>`、超时和候选引用校验，
  不得拼接 shell 字符串。
- 保存必须继续经过外部修改检测、同目录临时文件、同步、重开校验和原子替换。
- 合并和编辑必须进入撤销历史；保存不得自动 add、commit、push 或清空 Git
  modified 状态。
- 前端不得一次加载整个工作簿。保留视口 Region API、同步滚动、实际滚动值
  校正和请求序号防乱序。
- 快速切换仓库文件、引用或区域时，旧异步结果不得覆盖新选择。
- 差异相等语义由 Go 后端决定：`present + raw + formula + type`；样式和
  `display` 不参与相等判断。

## 当前 UI 约定

- 仓库侧栏提供“仓库文件 / 差异表 / 工作表与差异”页签。
- 仓库文件页签包含小字号目录树、文件/目录搜索和刷新。
- 差异表使用可复用的后台语义索引：Git 只筛选双方共有的变化候选，最终必须按
  Go 后端相等语义过滤；未经确认或实际无差异的表不得显示。精确单元格差异数
  在打开表格后由 Go 后端计算。
- 左侧单元格双击进入原位编辑；Enter 或失焦提交，Esc 取消。
- 数字形文本若与右侧原始值相同，保存类型应跟随右侧，避免可见值相同但类型
  不同造成虚假持续高亮。
- 底部固定为两行差异栏，显示左右值和类型，不恢复旧的底部编辑表单。
- 配色遵循 Git 双栏习惯：增加为绿、删除为红、修改在左侧显示旧值红色且右侧
  显示新值绿色、冲突使用高辨识度橙色；选中时保留语义底色并叠加蓝色边框。
  行分类和冲突判断由 Go 后端决定。
- 冲突行要求首行存在 `id` 列、同一物理行 ID 相同且其余实际数据列全部不同；
  冲突追加必须经过 merge/history，自动 ID 从左侧最大整数 ID 连续递增。
- 冲突覆盖/追加结果必须作为可撤销的会话标记显示在右侧；追加到左侧的目标行
  使用新增色，不能因纯坐标差异被错误显示为删除。

## 实施与验证

- 搜索文件和文本优先使用 `rg`、`rg --files`。
- 修改 Go 后运行 `gofmt`，并至少执行
  `GOCACHE=/tmp/ugxlsx-go-cache go test ./...`。
- 修改前端后执行：

  ```bash
  cd frontend
  npm run lint
  npm run typecheck
  npm run test
  npm run build
  ```

- 影响桌面集成或交付前执行：

  ```bash
  GOCACHE=/tmp/ugxlsx-go-cache \
    go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build
  ```

- 高风险核心修改还应执行 `go vet ./...` 和 `go test -race ./...`。
- 自动化测试、桌面构建和实机手工验收必须分别陈述；没有实际操作 GUI 时，
  不得声称完整手工主流程已通过。
- 任何 UI 布局、样式、显示条件或交互改动，在交付前都必须由 Codex 使用当次
  构建的桌面产物实际启动并操作受影响流程，保存截图核对关键布局和状态；仅有
  前端单测、浏览器 DOM 验证或 Wails 构建成功不得交付。若环境无法完成实机 GUI
  验收，必须继续修复验收条件或明确阻塞，不得把未验收的 UI 改动交给用户试错。
- 行为、命令、限制或验证基线变化时，同步更新 README、架构、手工验收和当前
  迭代交接文档。

## 范围与安全

- 不主动实现 Git fetch/pull/push/add/commit、分支切换、分支管理或冲突恢复。
- 不扩大到 `.xls`、`.xlsm`、`.ods`、CSV 或非表格文件编辑。
- 不破坏用户工作区中的无关修改，不提交或发布，除非用户明确要求。
- `docs/iteration-1-handoff.md` 是历史事实基线；当前状态写入
  `docs/iteration-2-handoff.md`，不要把第一轮文档改写成第二轮结果。
