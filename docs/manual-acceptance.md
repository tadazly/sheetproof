# 手工 GUI 验收

以下流程覆盖第一轮已经实现的主路径，控制在 15 步内：

1. 执行 `go run ./cmd/gentestdata --dir ./testdata` 生成左右工作簿。
2. 执行 `go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 dev`。要验证命令行启动加载，可先执行 `go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build`，再用桌面产物运行 `ugxlsx compare --left testdata/left.xlsx --right testdata/right.xlsx`；确认启动期间出现居中的“正在加载并比较工作簿”提示。
3. 未传入文件时点击“打开左右文件”，依次选择 `testdata/left.xlsx` 和 `testdata/right.xlsx`；确认顶部完整路径、左右角色、工作表状态和总差异数可见。
4. 快速连续点击“下一处”和“上一处”，尤其导航到工作表底部差异；确认左右同时定位，A/B/C…列标题保持在顶部，不出现白屏、大片异常空白或旧区域回跳。
5. 勾选“差异索引”，点击任意条目；确认显示当前差异序号，并定位到对应坐标。
6. 选择“数据 表”，按住 `Ctrl/Command` 滚动鼠标滚轮；确认两侧同步缩放。拖动 A/B 列标题分隔线，确认左右列宽同步。
7. 切换到其他工作表再切回“数据 表”，确认该工作簿/工作表的缩放和列宽设置恢复。
8. 点击行号 1，确认整行高亮；Shift 点击其他行号确认扩展选择。在右侧差异单元格上右键，确认出现“复制到左侧”。
9. 从右侧 A1 拖到 D1，或点击 A1 后 Shift 点击 D1；确认显示选中数量，并且复制按钮只统计选区内的差异单元格。
10. 连续执行两次单格复制，按 `Ctrl/Command+Z` 两次；确认按操作倒序逐步撤销。再执行一次批量复制并撤销，确认整批作为一个步骤撤回。
11. 选择左侧单格，在底部编辑区分别验证文本、数字、公式和“清空”；确认不允许直接批量编辑范围。
12. 修改后点击“保存左侧”；确认保存完成后撤销仍可用。执行撤销后应重新显示“有未保存修改”，再次保存后结果落盘。
13. 按 `Ctrl/Command+Shift+S`；确认默认文件名沿用左侧文件名，首次目录是系统“下载”目录。取消不应显示错误；另存成功后再次打开，确认默认目录是上次保存位置。
14. 关闭存在未保存修改的窗口，确认出现“取消 / 丢弃并关闭”提示；选择取消应继续留在应用中。
15. 执行 `go run . diff --left testdata/left.xlsx --right testdata/right.xlsx --format json`，确认 JSON 合法且差异数符合操作结果；再用 Excel、LibreOffice 或 WPS 打开保存文件，确认公式、样式、J1:K1 合并区、行高和列宽仍存在。

## UGit 验收

可写工作区模式：

```bash
ugxlsx compare \
  --left "${LOCAL_FILE}" \
  --right "${REMOTE_FILE}" \
  --left-label "当前分支" \
  --right-label "对比版本"
```

确认 `${LOCAL_FILE}` 是工作区真实文件而不是 Git 临时文件。复制、编辑并
点击“保存左侧”后，使用 `git status` 或 UGit 工作区状态确认该 `.xlsx`
真实发生变化。

只读历史模式在参数末尾增加 `--readonly-left`，此时复制、编辑和保存按钮
应不可用，右侧文件在两种模式下都不得被修改。
