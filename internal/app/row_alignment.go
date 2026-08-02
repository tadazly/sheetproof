package app

import (
	"sort"
	"strings"

	"github.com/tadazly/sheetproof/internal/diff"
	"github.com/tadazly/sheetproof/internal/workbook"
)

// alignRightRowsByID keeps the editable left workbook's physical rows as the
// presentation coordinate system and remaps right-side rows with the same
// unique ID onto them. This is intentionally enabled only for Git mergetool
// sessions: an insertion in one branch must not turn every following record
// into a coordinate-based modification, while ordinary two-file comparison
// retains its exact physical-coordinate semantics.
func alignRightRowsByID(
	left, right *workbook.WorkbookSnapshot,
) (*workbook.WorkbookSnapshot, map[string]map[int]int, int) {
	if right == nil {
		return nil, nil, 0
	}
	aligned := &workbook.WorkbookSnapshot{
		Path: right.Path, Identity: right.Identity,
		Sheets: make([]*workbook.SheetSnapshot, 0, len(right.Sheets)),
		ByName: make(map[string]*workbook.SheetSnapshot, len(right.Sheets)),
	}
	rowSources := make(map[string]map[int]int, len(right.Sheets))
	moved := 0
	for _, rightSheet := range right.Sheets {
		leftSheet := left.ByName[rightSheet.Name]
		mapping, sheetMoved := alignSheetRows(leftSheet, rightSheet)
		moved += sheetMoved
		rowSources[rightSheet.Name] = make(map[int]int, len(mapping))
		for physical, logical := range mapping {
			rowSources[rightSheet.Name][logical] = physical
		}
		copySheet := &workbook.SheetSnapshot{
			Name: rightSheet.Name, Index: rightSheet.Index, MaxCol: rightSheet.MaxCol,
			Cells:    make(map[workbook.CellKey]workbook.CellValue, len(rightSheet.Cells)),
			CellList: make([]workbook.CellKey, 0, len(rightSheet.CellList)),
		}
		for _, key := range rightSheet.CellList {
			logicalRow := mapping[key.Row]
			if logicalRow == 0 {
				logicalRow = key.Row
			}
			logicalKey := workbook.CellKey{Row: logicalRow, Col: key.Col}
			copySheet.Cells[logicalKey] = rightSheet.Cells[key]
			copySheet.CellList = append(copySheet.CellList, logicalKey)
			copySheet.MaxRow = max(copySheet.MaxRow, logicalRow)
		}
		sort.Slice(copySheet.CellList, func(i, j int) bool {
			leftKey, rightKey := copySheet.CellList[i], copySheet.CellList[j]
			return leftKey.Row < rightKey.Row ||
				(leftKey.Row == rightKey.Row && leftKey.Col < rightKey.Col)
		})
		aligned.Sheets = append(aligned.Sheets, copySheet)
		aligned.ByName[copySheet.Name] = copySheet
	}
	return aligned, rowSources, moved
}

func alignSheetRows(left, right *workbook.SheetSnapshot) (map[int]int, int) {
	rows := populatedRows(right)
	mapping := make(map[int]int, len(rows))
	if left == nil {
		for _, row := range rows {
			mapping[row] = row
		}
		return mapping, 0
	}
	idColumn := idColumn(left, right)
	if idColumn == 0 {
		for _, row := range rows {
			mapping[row] = row
		}
		return mapping, 0
	}
	leftIDs := uniqueIDRows(left, idColumn)
	rightIDs := uniqueIDRows(right, idColumn)
	used := make(map[int]struct{})
	for _, row := range populatedRows(left) {
		used[row] = struct{}{}
	}
	nextRow := max(left.MaxRow, right.MaxRow) + 1
	moved := 0
	for _, physicalRow := range rows {
		if physicalRow == 1 {
			mapping[physicalRow] = physicalRow
			continue
		}
		id := cellText(right.Cell(physicalRow, idColumn))
		if leftRow, exists := leftIDs[id]; id != "" && exists && rightIDs[id] == physicalRow {
			mapping[physicalRow] = leftRow
			if leftRow != physicalRow {
				moved++
			}
			continue
		}
		if _, occupied := used[physicalRow]; !occupied {
			mapping[physicalRow] = physicalRow
			used[physicalRow] = struct{}{}
			continue
		}
		for {
			if _, occupied := used[nextRow]; !occupied {
				break
			}
			nextRow++
		}
		mapping[physicalRow] = nextRow
		used[nextRow] = struct{}{}
		nextRow++
	}
	return mapping, moved
}

func identityRightRows(snapshot *workbook.WorkbookSnapshot) map[string]map[int]int {
	result := make(map[string]map[int]int)
	if snapshot == nil {
		return result
	}
	for _, sheet := range snapshot.Sheets {
		rows := make(map[int]int)
		for _, row := range populatedRows(sheet) {
			rows[row] = row
		}
		result[sheet.Name] = rows
	}
	return result
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

func idColumn(left, right *workbook.SheetSnapshot) int {
	for col := 1; col <= max(sheetMaxCol(left), sheetMaxCol(right)); col++ {
		for _, sheet := range []*workbook.SheetSnapshot{left, right} {
			if strings.EqualFold(cellText(sheet.Cell(1, col)), "id") {
				return col
			}
		}
	}
	return 0
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

func sheetMaxCol(sheet *workbook.SheetSnapshot) int {
	if sheet == nil {
		return 0
	}
	return sheet.MaxCol
}

func mergeSemanticNotice(
	base, left, right *workbook.WorkbookSnapshot,
) string {
	leftAligned, _, _ := alignRightRowsByID(base, left)
	rightAligned, _, _ := alignRightRowsByID(base, right)
	leftChanged := !diff.Compare(base, leftAligned).Equal
	rightChanged := !diff.Compare(base, rightAligned).Equal
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
