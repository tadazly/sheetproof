package app

import (
	"sort"
	"strconv"
	"strings"

	"github.com/tadazly/sheetproof/internal/diff"
	"github.com/tadazly/sheetproof/internal/workbook"
)

type rowAlignmentResult struct {
	left          *workbook.WorkbookSnapshot
	right         *workbook.WorkbookSnapshot
	leftSources   map[string]map[int]int
	rowSources    map[string]map[int]int
	keyColumns    map[string]int
	alignedSheets map[string]bool
	sheetMoved    map[string]int
	sheetEligible map[string]bool
	moved         int
	available     bool
}

type sheetRowAlignment struct {
	leftPhysicalToLogical  map[int]int
	rightPhysicalToLogical map[int]int
	moved                  int
	applied                bool
	eligible               bool
}

// alignRightRowsByID keeps the original permissive Git mergetool behavior:
// unique IDs are aligned even when other records have blank or duplicate IDs.
func alignRightRowsByID(
	left, right *workbook.WorkbookSnapshot,
) (*workbook.WorkbookSnapshot, map[string]map[int]int, int) {
	result := alignRightRows(left, right, false, nil)
	return result.right, result.rowSources, result.moved
}

// alignRightRowsForComparison is the safe policy used by ordinary file,
// repository and difftool comparisons. Records whose IDs are unique on both
// sides are aligned even when the same sheet also contains blank IDs, duplicate
// IDs or helper rows. Ambiguous records retain physical-row semantics whenever
// possible. Users can switch a session back to exact physical-row comparison
// when row order itself carries business meaning.
func alignRightRowsForComparison(
	left, right *workbook.WorkbookSnapshot,
	keyOverrides ...map[string]int,
) rowAlignmentResult {
	var overrides map[string]int
	if len(keyOverrides) > 0 {
		overrides = keyOverrides[0]
	}
	return alignRightRows(left, right, true, overrides)
}

func alignRightRows(
	left, right *workbook.WorkbookSnapshot,
	requireReliableIDs bool,
	keyOverrides map[string]int,
) rowAlignmentResult {
	if right == nil {
		return rowAlignmentResult{}
	}
	alignedLeft := emptyAlignedWorkbook(left)
	alignedRight := emptyAlignedWorkbook(right)
	leftSources := make(map[string]map[int]int)
	rowSources := make(map[string]map[int]int, len(right.Sheets))
	alignedSheets := make(map[string]bool, len(right.Sheets))
	keyColumns := make(map[string]int)
	sheetMoved := make(map[string]int)
	sheetEligible := make(map[string]bool)
	moved := 0
	available := false
	for _, name := range workbookSheetNames(left, right) {
		leftSheet, rightSheet := left.ByName[name], right.ByName[name]
		keyColumn := resolvedKeyColumn(name, leftSheet, rightSheet, keyOverrides)
		keyColumns[name] = keyColumn
		alignment := alignSheetRows(leftSheet, rightSheet, keyColumn, requireReliableIDs)
		leftSources[name] = invertRowMapping(alignment.leftPhysicalToLogical)
		rowSources[name] = invertRowMapping(alignment.rightPhysicalToLogical)
		alignedSheets[name] = alignment.applied
		sheetMoved[name] = alignment.moved
		sheetEligible[name] = alignment.eligible
		moved += alignment.moved
		available = available || alignment.eligible
		if leftSheet != nil {
			appendAlignedSheet(alignedLeft, alignSheetSnapshot(leftSheet, alignment.leftPhysicalToLogical))
		}
		if rightSheet != nil {
			appendAlignedSheet(alignedRight, alignSheetSnapshot(rightSheet, alignment.rightPhysicalToLogical))
		}
	}
	return rowAlignmentResult{
		left: alignedLeft, right: alignedRight,
		leftSources: leftSources, rowSources: rowSources, keyColumns: keyColumns,
		alignedSheets: alignedSheets, sheetMoved: sheetMoved, sheetEligible: sheetEligible,
		moved: moved, available: available,
	}
}

func alignSheetRows(
	left, right *workbook.SheetSnapshot,
	keyColumn int,
	requireReliableIDs bool,
) sheetRowAlignment {
	identity := sheetRowAlignment{
		leftPhysicalToLogical:  identitySheetRows(left),
		rightPhysicalToLogical: identitySheetRows(right),
	}
	if left == nil || right == nil || keyColumn == 0 {
		return identity
	}
	leftRows, leftCounts, _ := collectIDRows(left, keyColumn)
	rightRows, rightCounts, _ := collectIDRows(right, keyColumn)
	eligible := hasReliableID(leftRows, leftCounts) || hasReliableID(rightRows, rightCounts)
	identity.eligible = eligible
	if requireReliableIDs && !eligible {
		return identity
	}

	rightToLeft := map[int]int{1: 1}
	leftToRight := map[int]int{1: 1}
	commonIDRows := make(map[int]bool)
	leftUnique := uniqueIDRows(left, keyColumn)
	for id, rightRow := range uniqueIDRows(right, keyColumn) {
		leftRow := leftUnique[id]
		if leftRow == 0 {
			continue
		}
		rightToLeft[rightRow] = leftRow
		leftToRight[leftRow] = rightRow
		commonIDRows[rightRow] = true
	}
	maxCol := max(left.MaxCol, right.MaxCol)
	leftAmbiguous := uniqueAmbiguousRows(left, keyColumn, leftCounts, maxCol)
	for signature, rightRow := range uniqueAmbiguousRows(right, keyColumn, rightCounts, maxCol) {
		leftRow := leftAmbiguous[signature]
		if leftRow == 0 || leftToRight[leftRow] != 0 || rightToLeft[rightRow] != 0 {
			continue
		}
		rightToLeft[rightRow] = leftRow
		leftToRight[leftRow] = rightRow
	}

	// Ambiguous rows retain physical-row semantics only when neither side of
	// that coordinate has already been paired by a reliable record key.
	for row := 2; row <= min(left.MaxRow, right.MaxRow); row++ {
		if leftToRight[row] != 0 || rightToLeft[row] != 0 {
			continue
		}
		leftID, rightID := leftRows[row], rightRows[row]
		leftIsAmbiguous := leftID == "" || leftCounts[leftID] != 1
		rightIsAmbiguous := rightID == "" || rightCounts[rightID] != 1
		if leftIsAmbiguous && rightIsAmbiguous {
			leftToRight[row] = row
			rightToLeft[row] = row
		}
	}

	// Right-only records are inserted immediately before their next common
	// anchor. This preserves their original neighborhood instead of appending
	// every deletion/addition to the end of the comparison grid.
	beforeLeft := make(map[int][]int)
	tailRight := make([]int, 0)
	for rightRow := 2; rightRow <= right.MaxRow; rightRow++ {
		if rightToLeft[rightRow] != 0 {
			continue
		}
		nextLeft := 0
		for candidate := rightRow + 1; candidate <= right.MaxRow; candidate++ {
			if rightToLeft[candidate] != 0 {
				nextLeft = rightToLeft[candidate]
				break
			}
		}
		if nextLeft > 0 {
			beforeLeft[nextLeft] = append(beforeLeft[nextLeft], rightRow)
		} else {
			tailRight = append(tailRight, rightRow)
		}
	}

	leftMapping := make(map[int]int, left.MaxRow)
	rightMapping := make(map[int]int, right.MaxRow)
	logical := 1
	if left.MaxRow > 0 {
		leftMapping[1] = 1
	}
	if right.MaxRow > 0 {
		rightMapping[1] = 1
	}
	for leftRow := 2; leftRow <= left.MaxRow; leftRow++ {
		for _, rightRow := range beforeLeft[leftRow] {
			logical++
			rightMapping[rightRow] = logical
		}
		logical++
		leftMapping[leftRow] = logical
		if rightRow := leftToRight[leftRow]; rightRow > 0 {
			rightMapping[rightRow] = logical
		}
	}
	for _, rightRow := range tailRight {
		logical++
		rightMapping[rightRow] = logical
	}

	moved := 0
	for rightRow := range commonIDRows {
		if rightToLeft[rightRow] != rightRow {
			moved++
		}
	}
	applied := moved > 0 || !sameRowMapping(leftMapping) || !sameRowMapping(rightMapping)
	return sheetRowAlignment{
		leftPhysicalToLogical: leftMapping, rightPhysicalToLogical: rightMapping,
		moved: moved, applied: applied, eligible: eligible,
	}
}

func resolvedKeyColumn(
	name string,
	left, right *workbook.SheetSnapshot,
	overrides map[string]int,
) int {
	if column, configured := overrides[name]; configured {
		return column
	}
	return workbook.RecordKeyColumn(left, right)
}

func identitySheetRows(sheet *workbook.SheetSnapshot) map[int]int {
	rows := make(map[int]int)
	if sheet != nil {
		for row := 1; row <= sheet.MaxRow; row++ {
			rows[row] = row
		}
	}
	return rows
}

func sameRowMapping(mapping map[int]int) bool {
	for physical, logical := range mapping {
		if physical != logical {
			return false
		}
	}
	return true
}

func invertRowMapping(mapping map[int]int) map[int]int {
	result := make(map[int]int, len(mapping))
	for physical, logical := range mapping {
		result[logical] = physical
	}
	return result
}

func emptyAlignedWorkbook(source *workbook.WorkbookSnapshot) *workbook.WorkbookSnapshot {
	if source == nil {
		return nil
	}
	return &workbook.WorkbookSnapshot{
		Path: source.Path, Identity: source.Identity,
		Sheets: make([]*workbook.SheetSnapshot, 0, len(source.Sheets)),
		ByName: make(map[string]*workbook.SheetSnapshot, len(source.Sheets)),
	}
}

func appendAlignedSheet(book *workbook.WorkbookSnapshot, sheet *workbook.SheetSnapshot) {
	if book == nil || sheet == nil {
		return
	}
	book.Sheets = append(book.Sheets, sheet)
	book.ByName[sheet.Name] = sheet
}

func alignSheetSnapshot(
	source *workbook.SheetSnapshot,
	physicalToLogical map[int]int,
) *workbook.SheetSnapshot {
	if source == nil {
		return nil
	}
	copySheet := &workbook.SheetSnapshot{
		Name: source.Name, Index: source.Index, MaxCol: source.MaxCol,
		Cells:    make(map[workbook.CellKey]workbook.CellValue, len(source.Cells)),
		CellList: make([]workbook.CellKey, 0, len(source.CellList)),
	}
	for _, key := range source.CellList {
		logicalRow := physicalToLogical[key.Row]
		if logicalRow == 0 {
			continue
		}
		logicalKey := workbook.CellKey{Row: logicalRow, Col: key.Col}
		copySheet.Cells[logicalKey] = source.Cells[key]
		copySheet.CellList = append(copySheet.CellList, logicalKey)
	}
	for _, logicalRow := range physicalToLogical {
		copySheet.MaxRow = max(copySheet.MaxRow, logicalRow)
	}
	sort.Slice(copySheet.CellList, func(i, j int) bool {
		leftKey, rightKey := copySheet.CellList[i], copySheet.CellList[j]
		return leftKey.Row < rightKey.Row ||
			(leftKey.Row == rightKey.Row && leftKey.Col < rightKey.Col)
	})
	return copySheet
}

func workbookSheetNames(left, right *workbook.WorkbookSnapshot) []string {
	names := make([]string, 0)
	seen := make(map[string]bool)
	for _, book := range []*workbook.WorkbookSnapshot{left, right} {
		if book == nil {
			continue
		}
		for _, sheet := range book.Sheets {
			if !seen[sheet.Name] {
				seen[sheet.Name] = true
				names = append(names, sheet.Name)
			}
		}
	}
	return names
}

func collectIDRows(
	sheet *workbook.SheetSnapshot,
	idCol int,
) (map[int]string, map[string]int, map[int]bool) {
	rows := make(map[int]string)
	counts := make(map[string]int)
	populated := make(map[int]bool)
	for _, row := range populatedRows(sheet) {
		populated[row] = true
		if row < 2 {
			continue
		}
		id := cellText(sheet.Cell(row, idCol))
		rows[row] = id
		if id != "" {
			counts[id]++
		}
	}
	return rows, counts, populated
}

func hasReliableID(rows map[int]string, counts map[string]int) bool {
	for _, id := range rows {
		if id != "" && counts[id] == 1 {
			return true
		}
	}
	return false
}

func uniqueAmbiguousRows(
	sheet *workbook.SheetSnapshot,
	idCol int,
	idCounts map[string]int,
	maxCol int,
) map[string]int {
	rows := make(map[string]int)
	duplicates := make(map[string]bool)
	for _, row := range populatedRows(sheet) {
		if row < 2 {
			continue
		}
		id := cellText(sheet.Cell(row, idCol))
		if id != "" && idCounts[id] == 1 {
			continue
		}
		signature := semanticRowSignature(sheet, row, maxCol)
		if _, exists := rows[signature]; exists {
			duplicates[signature] = true
			continue
		}
		rows[signature] = row
	}
	for signature := range duplicates {
		delete(rows, signature)
	}
	return rows
}

func semanticRowSignature(sheet *workbook.SheetSnapshot, row, maxCol int) string {
	var signature strings.Builder
	for col := 1; col <= maxCol; col++ {
		value := sheet.Cell(row, col)
		if value.Present {
			signature.WriteByte('1')
		} else {
			signature.WriteByte('0')
		}
		writeSignaturePart(&signature, value.Raw)
		writeSignaturePart(&signature, value.Formula)
		writeSignaturePart(&signature, value.Type)
	}
	return signature.String()
}

func writeSignaturePart(signature *strings.Builder, value string) {
	signature.WriteString(strconv.Itoa(len(value)))
	signature.WriteByte(':')
	signature.WriteString(value)
}

func identityWorkbookRows(snapshot *workbook.WorkbookSnapshot) map[string]map[int]int {
	result := make(map[string]map[int]int)
	if snapshot == nil {
		return result
	}
	for _, sheet := range snapshot.Sheets {
		rows := make(map[int]int)
		for row := 1; row <= sheet.MaxRow; row++ {
			rows[row] = row
		}
		result[sheet.Name] = rows
	}
	return result
}

func prepareRightForComparison(
	left, right *workbook.WorkbookSnapshot,
	gitMerge bool,
	mode RowAlignmentMode,
	keyOverrides map[string]int,
) rowAlignmentResult {
	if right == nil {
		return rowAlignmentResult{}
	}
	if gitMerge {
		return alignRightRows(left, right, false, keyOverrides)
	}
	alignment := alignRightRowsForComparison(left, right, keyOverrides)
	if mode != RowAlignmentPosition {
		return alignment
	}
	alignedSheets := make(map[string]bool, len(right.Sheets))
	return rowAlignmentResult{
		left: left, right: right,
		leftSources: identityWorkbookRows(left), rowSources: identityWorkbookRows(right),
		keyColumns: alignment.keyColumns, alignedSheets: alignedSheets,
		sheetMoved: alignment.sheetMoved, sheetEligible: alignment.sheetEligible,
		available: alignment.available,
	}
}

// CompareSnapshots applies the same safe unique-ID alignment used by desktop
// comparison sessions, then runs the existing cell and conflict classifier.
// The CLI uses this helper so its counts stay consistent with the GUI.
func CompareSnapshots(
	left, right *workbook.WorkbookSnapshot,
) *diff.WorkbookDiff {
	alignment := alignRightRowsForComparison(left, right)
	return diff.CompareWithKeyColumns(alignment.left, alignment.right, alignment.keyColumns)
}

func populatedRows(sheet *workbook.SheetSnapshot) []int {
	if sheet == nil {
		return nil
	}
	seen := make(map[int]struct{})
	for _, key := range sheet.CellList {
		seen[key.Row] = struct{}{}
	}
	rows := make([]int, 0, len(seen))
	for row := range seen {
		rows = append(rows, row)
	}
	sort.Ints(rows)
	return rows
}

func uniqueIDRows(sheet *workbook.SheetSnapshot, idColumn int) map[string]int {
	rows := make(map[string]int)
	duplicates := make(map[string]struct{})
	for _, row := range populatedRows(sheet) {
		if row < 2 {
			continue
		}
		id := cellText(sheet.Cell(row, idColumn))
		if id == "" {
			continue
		}
		if _, exists := rows[id]; exists {
			duplicates[id] = struct{}{}
			continue
		}
		rows[id] = row
	}
	for id := range duplicates {
		delete(rows, id)
	}
	return rows
}

func cellText(value workbook.CellValue) string {
	text := value.Raw
	if text == "" {
		text = value.Display
	}
	return strings.TrimSpace(text)
}

func mergeSemanticNotice(
	base, left, right *workbook.WorkbookSnapshot,
) string {
	leftAlignment := alignRightRows(base, left, false, nil)
	rightAlignment := alignRightRows(base, right, false, nil)
	leftChanged := !diff.CompareWithKeyColumns(
		leftAlignment.left, leftAlignment.right, leftAlignment.keyColumns,
	).Equal
	rightChanged := !diff.CompareWithKeyColumns(
		rightAlignment.left, rightAlignment.right, rightAlignment.keyColumns,
	).Equal
	switch {
	case leftChanged && rightChanged:
		return "Git 已将此 XLSX 标记为文件级冲突；左右两侧都相对共同基线存在表格语义变化，橙色仅标记同一 ID 记录的双方不兼容修改。"
	case leftChanged:
		return "Git 已将此 XLSX 标记为文件级冲突，但右侧与共同基线语义一致；当前只有左侧存在实际表格变化，没有双方语义冲突。"
	case rightChanged:
		return "Git 已将此 XLSX 标记为文件级冲突，但左侧与共同基线语义一致；当前只有右侧存在实际表格变化，没有双方语义冲突。"
	default:
		return "Git 已将此 XLSX 标记为文件级冲突，但两侧与共同基线的表格语义均一致；差别仅来自 OOXML 二进制封装或未参与相等判断的显示/样式。"
	}
}
