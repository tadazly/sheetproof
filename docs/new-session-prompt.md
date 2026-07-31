# 新会话接手提示词

在同一工作区启动新会话后，可复制以下内容。最后一段替换为下一轮实际需求。

```text
你正在接手 ugxlsx 的后续迭代。先确认实际工作目录和操作系统版本，不要沿用旧提示中的路径或平台假设。

请使用项目技能 $ugxlsx-development。若当前会话没有自动发现该技能，则直接
完整阅读：

1. AGENTS.md
2. .agents/skills/ugxlsx-development/SKILL.md
3. docs/iteration-2-handoff.md
4. README.md
5. docs/architecture.md
6. 与本次需求相关时再读 docs/manual-acceptance.md

开始修改前先执行：

Get-Location
git status --short
git log -5 --oneline --decorate
$env:GOCACHE = Join-Path $env:TEMP "ugxlsx-go-cache"
go test ./...
cd frontend
npm run test
npm run typecheck
cd ..

重要：2026-07-31 Windows 接手开始时 HEAD 与 origin/main 都是 `5315d1a`；
第一批 Windows 修复已提交为 `ee5663a` 并推送，旧提示中的 `6933f8a` 已失效。
后续 TaskDialog STA/`0x80070057` 等修复是否提交，以当前 `git status` 和
`git log` 为准。除非用户明确要求，不要 commit、push、改写提交或发布；不要
reset、checkout、覆盖或清理现有修改。

以 docs/iteration-2-handoff.md 记录的已实现且已验证行为作为兼容基线。必须
保留仓库模式和直接双文件模式，共用现有 Go 差异、合并、撤销和安全保存逻辑。
仓库模式左侧读取工作区真实文件，右侧只通过 Git 对象读取，不允许 checkout、
switch、fetch、add、commit 或 push。快速切换必须继续防止旧异步结果覆盖新
选择。后台差异表索引必须保持可取消：关闭窗口不能等待全部候选表比较完成；
索引轮询也不能覆盖用户对目录树的展开/收起状态。

本轮机器实际为 Windows 10 Pro 21H2 build 19044，不是 Windows 11。不要把
Windows 10 原生构建、自动化或进程取证表述成 Win11 手工验收通过。下一轮优先在
Win11 按 docs/manual-acceptance.md 复核索引期间任务栏/关闭、UGit 差异与合并、
未保存三按钮、Ctrl+S/Ctrl+Shift+S 和资源管理器/任务栏/Alt+Tab 图标。

实施完成后按 AGENTS.md 运行与风险相称的测试。自动化测试、Wails 桌面构建和
真实 GUI 手工验收要分别汇报；没有实际完成的验收不要声称通过。若行为、架构
或限制改变，同步更新 README、architecture、manual-acceptance 和当前交接文档。

本轮需求：
[在这里粘贴下一轮需求]
```
