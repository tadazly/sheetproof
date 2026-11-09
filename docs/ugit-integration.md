# UGit 集成

SheetProof 可注册为 UGit 的 `*.xlsx` 差异与合并工具。配置只针对 XLSX，不会覆盖
CSV 或其他外部工具设置。

## 自动配置

把 `SheetProof.app` 或 `SheetProof.exe` 放到固定安装目录，启动应用后点击“配置 UGit”。
对话框会显示检测到的 Git 配置来源和旧的 XLSX 工具路径；确认后注册当前应用路径。
移动应用后需要重新执行配置。

v0.9.0 补充了仓库级覆盖检查：UGit 会优先采用仓库中配置的工具，即使它的规则
是 `*.*`，也可能覆盖全局 XLSX 工具。先在 SheetProof 中打开受影响仓库，再执行
“配置 UGit”，即可替换该仓库旧的 XLSX 专用规则。通用工具、通用作用范围及 CSV
等其他类型的关联保留，通过新增 XLSX 专用规则让表格使用 SheetProof。
未打开仓库时仅配置全局，不会扫描其他仓库。操作失败会恢复原配置；完成后重启 UGit。

### 扫描仓库目录并修复

不必先打开仓库。在“设置 → UGit 集成”选择“扫描仓库目录并修复”，选择代码所在的
目录。扫描递归查找该目录及子目录中的 Git 工作区，只读取配置，不加载工作簿；不进入
`.git` 元数据目录，也不跟随目录链接。普通仓库、嵌套仓库和使用 `.git` 文件的工作区
均可识别。没有旧 XLSX 覆盖配置、仅配置其他后缀或已正确配置的仓库不会列入待修复列表。

检查仓库路径和现有工具，勾选需要修复的项目后点击“修复所选仓库”，再确认。只更新
全局与所选仓库的 XLSX 关联；保留通用 `*.*` 的命令及作用范围、CSV 等其他类型配置。
扫描可以取消，无法检查的位置会单独列出。修复前再次核对配置：扫描后已变化的仓库
跳过并要求重新扫描；单个仓库失败会尝试恢复该仓库与本次事务的全局配置，再处理其他
仓库。界面逐项报告成功、无需修复、配置变化和失败；完成后重启 UGit。

扫描结果只在本次设置窗口中使用，完成修复后需要重新扫描才能再次修改。没有扫描到
的目录或未挂载磁盘不会被处理。共享 Git 配置的 linked worktree 可能一起受同一次
XLSX 配置更新影响，后续已配置的工作区会显示无需修复。

### 为什么差异工具中有两行 XLSX 配置

配置成功后，UGit 的“差异工具”列表中会同时出现下面两行；这是预期结果，不是
重复配置：

| 后缀 | 工具 | 用途 |
|---|---|---|
| `*.xlsx` | `SpreadsheetCompare` | UGit 5.51 专用入口。UGit 直接准备两侧文件和路径列表，支持可靠地启动工作区对比。 |
| `*.xlsx` | `Custom` | 标准 Git difftool 兼容入口，供普通 `git difftool` 或显式选择该工具时使用。 |

两行通常指向同一个 SheetProof 可执行文件，Args 看起来也相同；真正的区别由工具名
触发的 UGit 调用协议决定。UGit 的“合并工具”列表中则只会有一行 `*.xlsx / Custom`，
用于传递 `$LOCAL`、`$REMOTE`、`$BASE` 和 `$MERGED`。不要仅因路径和 Args 相似而
删除差异工具中的 `Custom` 行，否则会失去标准 Git difftool 兼容入口。

## 差异工具参数

```text
compare --left "$LOCAL" --right "$REMOTE"
```

UGit 的 `SpreadsheetCompare` 会把路径列表写到当前仓库实际 Git 目录下的
`ugit/diff`。当列表第一项是该目录中的版本快照、第二项是同一仓库中的真实工作区
`.xlsx` 时，SheetProof 会交叉核对 `git rev-parse --absolute-git-dir`，然后把工作区
交换到可编辑左侧、把版本快照放到只读右侧。保存仍需用户明确操作，并继续走外部
修改检测和原子替换；不会自动 add、commit 或 push。

两个历史版本、已删除文件或无法通过上述仓库归属校验的调用继续保持双侧只读。
普通 Git difftool 也仍根据完整的 Git difftool 环境进入双侧只读模式。它同时处理
新增或删除文件传入的 `/dev/null`（Windows 上兼容 `NUL`），临时占位簿只在系统
临时目录创建并在退出时清理。

## 合并工具参数

```text
compare --left "$LOCAL" --right "$REMOTE" --base "$BASE" --output "$MERGED"
```

合并时 `$LOCAL` 和 `$REMOTE` 是可选择的内容来源，`$BASE` 用于解释三方语义，保存
目标是 `$MERGED`。未点击保存即关闭时不应改写 `$MERGED`。

## 普通可写比较

如果宿主没有提供可验证的 UGit 工作区路径，也可以显式运行：

```bash
SheetProof compare \
  --left "/path/to/worktree/file.xlsx" \
  --right "/path/to/reference.xlsx" \
  --left-label "当前分支" \
  --right-label "对比版本"
```

左侧是唯一可写来源。保存只更新工作区文件，不会自动执行 Git add、commit 或 push。
完整验收步骤见 `docs/manual-acceptance.md`。
