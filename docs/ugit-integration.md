# UGit 集成

SheetProof 可注册为 UGit 的 `*.xlsx` 差异与合并工具。配置只针对 XLSX，不会覆盖
CSV 或其他外部工具设置。

## 自动配置

把 `SheetProof.app` 或 `SheetProof.exe` 放到固定安装目录，启动应用后点击“配置 UGit”。
对话框会显示检测到的 Git 配置来源和旧的 XLSX 工具路径；确认后注册当前应用路径。
移动应用后需要重新执行配置。

## 差异工具参数

```text
compare --left "$LOCAL" --right "$REMOTE"
```

从 Git difftool 或 UGit 启动时，应用会根据完整的 Git difftool 环境进入双侧只读模式。
它也处理新增或删除文件传入的 `/dev/null`（Windows 上兼容 `NUL`），临时占位簿只在
系统临时目录创建并在退出时清理。

## 合并工具参数

```text
compare --left "$LOCAL" --right "$REMOTE" --base "$BASE" --output "$MERGED"
```

合并时 `$LOCAL` 和 `$REMOTE` 是可选择的内容来源，`$BASE` 用于解释三方语义，保存
目标是 `$MERGED`。未点击保存即关闭时不应改写 `$MERGED`。

## 普通可写比较

如果希望直接修改真实工作区文件，不要从 difftool 入口启动；显式运行：

```bash
SheetProof compare \
  --left "/path/to/worktree/file.xlsx" \
  --right "/path/to/reference.xlsx" \
  --left-label "当前分支" \
  --right-label "对比版本"
```

左侧是唯一可写来源。保存只更新工作区文件，不会自动执行 Git add、commit 或 push。
完整验收步骤见 `docs/manual-acceptance.md`。
