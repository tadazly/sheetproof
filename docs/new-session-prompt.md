# 新会话接手提示词

在同一工作区启动新会话后，可复制以下内容。最后一段替换为下一轮实际需求。

```text
你正在接手 SheetProof 的后续迭代。先确认实际工作目录、操作系统和当前 Git 状态，
不要沿用旧提示中的路径、平台、提交或工作区假设。

使用仓库自动发现的项目 skills，并按任务选择最小上下文：

- 开发、调试、评审、测试或项目文档：sheetproof-development
- 官网、公开产品事实、更新日志或用户可见变更同步：sheetproof-site
- 正式版本、GitHub Release、Release Notes 或正式资产：sheetproof-release

若 skill 未自动发现，先读 AGENTS.md 和对应 SKILL.md。用 rg 定位相关代码、测试、
architecture、iteration-2-handoff 和 manual-acceptance 的具体章节；除全局审计外，
不要默认完整加载累计文档。

开始修改前先执行：

Get-Location
git status --short --branch
git log -5 --oneline --decorate

现有修改属于用户工作，不得 reset、checkout、覆盖或清理。除非用户明确授权，
不要 add、commit、push、切换分支或发布。必须保留仓库模式和直接双文件模式及其
共享的 Go 差异、合并、撤销和安全保存逻辑；仓库右侧只通过 Git 对象读取，旧异步
结果不得覆盖新选择。

实施后按对应 skill 运行与风险相称的验证。自动化测试、桌面打包和真实 GUI 验收
分别汇报；未实际完成的验收不得声称通过。只同步与本次行为、约束或公开事实有关的
文档和官网内容。

本轮需求：
[在这里粘贴下一轮需求]
```
