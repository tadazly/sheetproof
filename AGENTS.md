# SheetProof 项目协作规则

## 开始与上下文

- 修改前运行 `git status --short --branch`；现有改动属于用户工作，不得用
  `git reset`、`git checkout --` 或覆盖式操作丢弃。
- 按任务加载最小上下文：开发、调试、评审、测试和项目文档使用
  `.agents/skills/sheetproof-development/SKILL.md`；官网、公开产品事实和更新日志使用
  `.agents/skills/sheetproof-site/SKILL.md`；正式版本与 GitHub Release 使用
  `.agents/skills/sheetproof-release/SKILL.md`。
- 搜索优先使用 `rg`、`rg --files`，只读取与当前任务相关的代码、测试和文档章节；
  除非任务是全局审计，不要默认完整读取 README、架构和累计交接文档。
- `docs/iteration-2-handoff.md` 是当前累计交接；`docs/iteration-1-handoff.md` 仅是历史事实。

## 永久兼容不变量

- 保留“直接打开左右两个 `.xlsx`”和“打开本地 Git 仓库”两种入口，并复用
  `internal/app.Session`、`internal/diff`、`internal/merge`、`internal/history` 和
  `internal/storage`，不得另写表格逻辑。
- 左侧是唯一可写来源；仓库模式读取真实工作区文件（包括未提交修改）。右侧永远只读，
  必须通过 Git 对象读取；不得 checkout、switch、fetch，临时文件不得进入用户仓库。
- 仓库文件使用根目录相对路径。Git 命令使用参数数组、明确的 `git -C <root>`、超时和
  候选引用校验，不得拼接 shell 字符串。
- 保存继续经过外部修改检测、同目录临时文件、同步、重开校验和原子替换。编辑与合并
  必须进入撤销历史；保存不得自动 add、commit、push 或清空 Git modified 状态。
- 前端只加载视口 Region，保留同步滚动、实际滚动值校正和请求代次保护；快速切换文件、
  引用或区域时，旧异步结果不得覆盖新选择。
- 相等语义由 Go 后端决定：`present + raw + formula + type`；样式和 `display` 不参与判断。
- 主键与逻辑行映射由 Go 后端决定：自动识别双方同列的 `id`，或唯一同名且以 `ID` 结尾的
  候选；多个候选不得猜测。允许在产生撤销历史前手动设置/取消主键或切换按行号比较。
  空白、重复等歧义只让对应记录保守回退；左右独有记录和列保持在相邻共同项附近。
- 冲突覆盖、追加和标记必须经过 Session/merge/history 并可撤销；追加目标按新增显示，
  自动整数 ID 从左侧最大值连续递增。

## 验证与交付边界

- 按开发 skill 运行与改动范围相称的格式化、测试、构建和高风险检查；自动化测试、桌面
  打包和真实 GUI 验收必须分开报告。
- UI 布局、样式、显示条件或交互改动必须使用当次桌面产物实际操作、截图并目视核对；
  前端单测、浏览器 DOM 或 Wails 构建不能替代真实桌面验收。
- 面向用户的功能或修复按网站 skill 同步公开事实、更新日志和官网；正式版本按发布 skill
  执行两阶段流程，确认前不得 commit、push、打标签、发布 Release 或部署候选官网。

## 范围与安全

- 不主动执行 Git fetch/pull/push/add/commit、分支切换或冲突恢复；仅在用户明确授权的
  范围内执行，且不得强推或移动已有标签。
- 不扩大到 `.xls`、`.xlsm`、`.ods`、CSV 或非表格文件编辑，不破坏无关改动。
- 公开仓库不得写入服务器身份、本机路径、认证配置、密钥或其他凭据。发布前隐私扫描若
  发现敏感数据，只报告位置、类型、风险与处理状态，不复述值，并在处理完成前停止推送。
- 行为或约束变化时只更新相关文档；保留历史交接，不为当前状态改写第一轮文档。
