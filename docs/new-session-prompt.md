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

重要：当前 main 的 `6933f8a` 已包含第二、三轮交付，main 相对 origin/main
领先 1 个提交且尚未推送。交接文档更新和现代化 UI 的 `frontend/src`、
`frontend/dist`、README、架构及手工验收文档修改仍未提交，它们都是有效成果，
必须保留。除非用户明确要求，不要 commit、push、改写提交或发布。若还有任何
其他未提交改动，也视为用户有效成果；不要 reset、checkout、覆盖或清理。

以 docs/iteration-2-handoff.md 记录的已实现且已验证行为作为兼容基线。必须
保留仓库模式和直接双文件模式，共用现有 Go 差异、合并、撤销和安全保存逻辑。
仓库模式左侧读取工作区真实文件，右侧只通过 Git 对象读取，不允许 checkout、
switch、fetch、add、commit 或 push。快速切换必须继续防止旧异步结果覆盖新
选择。后台差异表索引必须保持可取消：关闭窗口不能等待全部候选表比较完成；
索引轮询也不能覆盖用户对目录树的展开/收起状态。

当前 Go 测试、19 项前端测试、生产构建和 macOS Wails 打包已通过；真实 Wails
应用只检查了未选文件和已加载无差异双网格的现代化界面，没有完成真实 GUI 全流程
手工验收。不要把自动化或构建结果表述成手工验收通过。下一轮若涉及仓库 GUI，
优先按 docs/manual-acceptance.md 复核索引期间关闭、左右滚动固定层、四类配色、
冲突覆盖/追加标记和撤销。

实施完成后按 AGENTS.md 运行与风险相称的测试。自动化测试、Wails 桌面构建和
真实 GUI 手工验收要分别汇报；没有实际完成的验收不要声称通过。若行为、架构
或限制改变，同步更新 README、architecture、manual-acceptance 和当前交接文档。

本轮需求：
[在这里粘贴下一轮需求]
```
