# 新会话接手提示词

在同一工作区启动新会话后，可复制以下内容。最后一段替换为下一轮实际需求。

```text
你正在接手 /Users/luyi/splan-git/ugxlsx 的后续迭代。

请使用项目技能 $ugxlsx-development。若当前会话没有自动发现该技能，则直接
完整阅读：

1. AGENTS.md
2. .agents/skills/ugxlsx-development/SKILL.md
3. docs/iteration-2-handoff.md
4. README.md
5. docs/architecture.md
6. 与本次需求相关时再读 docs/manual-acceptance.md

开始修改前先执行：

pwd
git status --short
git log -5 --oneline --decorate
GOCACHE=/tmp/ugxlsx-go-cache go test ./...
cd frontend && npm run test && npm run typecheck

重要：当前 main 的 `058f437` 已包含第二轮基线；第三轮目录树/分支偏好优化
及后续 UI 改动仍可能是工作树中的未提交有效成果。不要 reset、checkout、覆盖
或清理这些改动；保留用户的全部现有修改。

以 docs/iteration-2-handoff.md 记录的已实现且已验证行为作为兼容基线。必须
保留仓库模式和直接双文件模式，共用现有 Go 差异、合并、撤销和安全保存逻辑。
仓库模式左侧读取工作区真实文件，右侧只通过 Git 对象读取，不允许 checkout、
switch、fetch、add、commit 或 push。快速切换必须继续防止旧异步结果覆盖新
选择。

实施完成后按 AGENTS.md 运行与风险相称的测试。自动化测试、Wails 桌面构建和
真实 GUI 手工验收要分别汇报；没有实际完成的验收不要声称通过。若行为、架构
或限制改变，同步更新 README、architecture、manual-acceptance 和当前交接文档。

本轮需求：
[在这里粘贴下一轮需求]
```
