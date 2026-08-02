package diff

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ug-tools/ugxlsx/internal/workbook"
)

type CellStatus string

const (
	Unchanged    CellStatus = "unchanged"
	LeftOnly     CellStatus = "left-added"
	RightOnly    CellStatus = "right-added"
	Modified     CellStatus = "modified"
	LeftMissing  CellStatus = "left-missing"
	RightMissing CellStatus = "right-missing"
)

type RowStatus string

const (
	RowUnchanged RowStatus = "unchanged"
	RowAdded     RowStatus = "added"
	RowDeleted   RowStatus = "deleted"
	RowModified  RowStatus = "modified"
	RowConflict  RowStatus = "conflict"
)

type SheetStatus string

const (
	SheetEqual     SheetStatus = "equal"
	SheetModified  SheetStatus = "modified"
	SheetLeftOnly  SheetStatus = "left-only"
	SheetRightOnly SheetStatus = "right-only"
)

type CellDiff struct {
	Ref       workbook.CellRef   `json:"ref"`
	Status    CellStatus         `json:"status"`
	RowStatus RowStatus          `json:"rowStatus"`
	Left      workbook.CellValue `json:"left"`
	Right     workbook.CellValue `json:"right"`
}

type RowDiff struct {
	Row    int       `json:"row"`
	ID     string    `json:"id,omitempty"`
	Status RowStatus `json:"status"`
}

type SheetDiff struct {
	Name             string      `json:"name"`
	Status           SheetStatus `json:"status"`
	LeftIndex        int         `json:"leftIndex"`
	RightIndex       int         `json:"rightIndex"`
	OrderDifferent   bool        `json:"orderDifferent"`
	DifferenceCount  int         `json:"differenceCount"`
	MaxRow           int         `json:"maxRow"`
	MaxCol           int         `json:"maxCol"`
	IDColumn         int         `json:"idColumn"`
	NextID           int64       `json:"nextId"`
	AddedRowCount    int         `json:"addedRowCount"`
	DeletedRowCount  int         `json:"deletedRowCount"`
	ModifiedRowCount int         `json:"modifiedRowCount"`
	ConflictRowCount int         `json:"conflictRowCount"`
	Rows             []RowDiff   `json:"rows,omitempty"`
	Differences      []CellDiff  `json:"differences,omitempty"`
}

type WorkbookDiff struct {
	Equal               bool         `json:"equal"`
	LeftFile            string       `json:"leftFile"`
	RightFile           string       `json:"rightFile"`
	SheetCount          int          `json:"sheetCount"`
	DifferentSheetCount int          `json:"differentSheetCount"`
	DifferenceCount     int          `json:"differenceCount"`
	Sheets              []*SheetDiff `json:"sheets"`
}

func Compare(left, right *workbook.WorkbookSnapshot) *WorkbookDiff {
	result := &WorkbookDiff{LeftFile: left.Path, RightFile: right.Path}
	names := orderedUnion(left, right)
	for _, name := range names {
		ls := left.ByName[name]
		rs := right.ByName[name]
		sd := compareSheet(name, ls, rs)
		result.Sheets = append(result.Sheets, sd)
		if sd.Status != SheetEqual || sd.OrderDifferent {
			result.DifferentSheetCount++
		}
		result.DifferenceCount += sd.DifferenceCount
	}
	result.SheetCount = len(result.Sheets)
	result.Equal = result.DifferenceCount == 0 && result.DifferentSheetCount == 0
	return result
}

func orderedUnion(left, right *workbook.WorkbookSnapshot) []string {
	names := make([]string, 0, len(left.Sheets)+len(right.Sheets))
	seen := make(map[string]bool, cap(names))
	for _, sheet := range left.Sheets {
		names = append(names, sheet.Name)
		seen[sheet.Name] = true
	}
	for _, sheet := range right.Sheets {
		if !seen[sheet.Name] {
			names = append(names, sheet.Name)
		}
	}
	return names
}

func compareSheet(name string, left, right *workbook.SheetSnapshot) *SheetDiff {
	sd := &SheetDiff{Name: name, LeftIndex: -1, RightIndex: -1}
	defer classifyRows(sd, left, right)
	if left != nil {
		sd.LeftIndex = left.Index
		sd.MaxRow = left.MaxRow
		sd.MaxCol = left.MaxCol
	}
	if right != nil {
		sd.RightIndex = right.Index
		sd.MaxRow = max(sd.MaxRow, right.MaxRow)
		sd.MaxCol = max(sd.MaxCol, right.MaxCol)
	}
	if left == nil {
		sd.Status = SheetRightOnly
		for _, key := range right.CellList {
			sd.Differences = append(sd.Differences, CellDiff{
				Ref:    workbook.CellRef{Sheet: name, Row: key.Row, Col: key.Col},
				Status: LeftMissing, Right: right.Cells[key],
			})
		}
		sd.DifferenceCount = len(sd.Differences)
		return sd
	}
	if right == nil {
		sd.Status = SheetLeftOnly
		for _, key := range left.CellList {
			sd.Differences = append(sd.Differences, CellDiff{
				Ref:    workbook.CellRef{Sheet: name, Row: key.Row, Col: key.Col},
				Status: RightMissing, Left: left.Cells[key],
			})
		}
		sd.DifferenceCount = len(sd.Differences)
		return sd
	}
	sd.OrderDifferent = left.Index != right.Index
	leftIndex, rightIndex := 0, 0
	for leftIndex < len(left.CellList) || rightIndex < len(right.CellList) {
		var key workbook.CellKey
		lp, rp := false, false
		switch {
		case rightIndex >= len(right.CellList):
			key, lp = left.CellList[leftIndex], true
			leftIndex++
		case leftIndex >= len(left.CellList):
			key, rp = right.CellList[rightIndex], true
			rightIndex++
		default:
			leftKey, rightKey := left.CellList[leftIndex], right.CellList[rightIndex]
			comparison := compareKey(leftKey, rightKey)
			switch {
			case comparison < 0:
				key, lp = leftKey, true
				leftIndex++
			case comparison > 0:
				key, rp = rightKey, true
				rightIndex++
			default:
				key, lp, rp = leftKey, true, true
				leftIndex++
				rightIndex++
			}
		}
		lv := left.Cells[key]
		rv := right.Cells[key]
		if lp && rp && lv.Equal(rv) {
			continue
		}
		status := Modified
		switch {
		case !lp && rp:
			status = RightOnly
		case lp && !rp:
			status = LeftOnly
		}
		sd.Differences = append(sd.Differences, CellDiff{
			Ref:    workbook.CellRef{Sheet: name, Row: key.Row, Col: key.Col},
			Status: status, Left: lv, Right: rv,
		})
	}
	sd.DifferenceCount = len(sd.Differences)
	if sd.DifferenceCount > 0 {
		sd.Status = SheetModified
	} else {
		sd.Status = SheetEqual
	}
	return sd
}

func classifyRows(sd *SheetDiff, left, right *workbook.SheetSnapshot) {
	sd.Rows = nil
	sd.AddedRowCount = 0
	sd.DeletedRowCount = 0
	sd.ModifiedRowCount = 0
	sd.ConflictRowCount = 0
	sd.IDColumn = findIDColumn(left, right)
	sd.NextID = nextLeftID(left, right, sd.IDColumn)
	sd.MaxRow = max(sheetMaxRow(left), sheetMaxRow(right))
	sd.MaxCol = max(sheetMaxCol(left), sheetMaxCol(right))
	startRow := 1
	if sd.IDColumn > 0 {
		startRow = 2
	}
	rows, leftRows, rightRows, columnsByRow := sparseRows(left, right, startRow)
	statusByRow := make(map[int]RowStatus)
	for _, row := range rows {
		leftHas, rightHas := leftRows[row], rightRows[row]
		status := RowUnchanged
		switch {
		case !leftHas && rightHas:
			status = RowAdded
		case leftHas && !rightHas:
			status = RowDeleted
		default:
			different, allDataDifferent := rowDifference(
				left, right, row, columnsByRow[row], sd.IDColumn,
			)
			if !different {
				continue
			}
			status = RowModified
			leftID := rowID(left, row, sd.IDColumn)
			rightID := rowID(right, row, sd.IDColumn)
			if sd.IDColumn > 0 && leftID != "" && leftID == rightID && allDataDifferent {
				status = RowConflict
			}
		}
		id := rowID(right, row, sd.IDColumn)
		if id == "" {
			id = rowID(left, row, sd.IDColumn)
		}
		sd.Rows = append(sd.Rows, RowDiff{Row: row, ID: id, Status: status})
		statusByRow[row] = status
		switch status {
		case RowAdded:
			sd.AddedRowCount++
		case RowDeleted:
			sd.DeletedRowCount++
		case RowModified:
			sd.ModifiedRowCount++
		case RowConflict:
			sd.ConflictRowCount++
		}
	}
	for index := range sd.Differences {
		status := statusByRow[sd.Differences[index].Ref.Row]
		if status == "" {
			status = RowModified
		}
		sd.Differences[index].RowStatus = status
	}
}

func findIDColumn(left, right *workbook.SheetSnapshot) int {
	maxCol := max(sheetMaxCol(left), sheetMaxCol(right))
	for col := 1; col <= maxCol; col++ {
		for _, sheet := range []*workbook.SheetSnapshot{left, right} {
			value := sheet.Cell(1, col)
			label := value.Raw
			if label == "" {
				label = value.Display
			}
			if strings.EqualFold(strings.TrimSpace(label), "id") {
				return col
			}
		}
	}
	return 0
}

func nextLeftID(left, right *workbook.SheetSnapshot, idColumn int) int64 {
	if idColumn == 0 {
		return 0
	}
	hasID := false
	for _, sheet := range []*workbook.SheetSnapshot{left, right} {
		if sheet == nil {
			continue
		}
		for _, key := range sheet.CellList {
			if key.Row < 2 || key.Col != idColumn {
				continue
			}
			id := strings.TrimSpace(rowID(sheet, key.Row, idColumn))
			if id == "" {
				continue
			}
			hasID = true
			if _, err := strconv.ParseInt(id, 10, 64); err != nil {
				return 0
			}
		}
	}
	if !hasID {
		return 0
	}
	var maximum int64
	if left == nil {
		return 1
	}
	for _, key := range left.CellList {
		if key.Row < 2 || key.Col != idColumn {
			continue
		}
		value, err := strconv.ParseInt(strings.TrimSpace(rowID(left, key.Row, idColumn)), 10, 64)
		if err == nil && value > maximum {
			maximum = value
		}
	}
	return maximum + 1
}

func rowID(sheet *workbook.SheetSnapshot, row, idColumn int) string {
	if sheet == nil || idColumn == 0 {
		return ""
	}
	value := sheet.Cell(row, idColumn)
	id := value.Raw
	if id == "" {
		id = value.Display
	}
	return strings.TrimSpace(id)
}

func sparseRows(
	left, right *workbook.SheetSnapshot,
	startRow int,
) ([]int, map[int]bool, map[int]bool, map[int][]int) {
	rowSet := make(map[int]struct{})
	leftRows := make(map[int]bool)
	rightRows := make(map[int]bool)
	columnSets := make(map[int]map[int]struct{})
	add := func(sheet *workbook.SheetSnapshot, presentRows map[int]bool) {
		if sheet == nil {
			return
		}
		for _, key := range sheet.CellList {
			if key.Row < startRow {
				continue
			}
			rowSet[key.Row] = struct{}{}
			presentRows[key.Row] = true
			if columnSets[key.Row] == nil {
				columnSets[key.Row] = make(map[int]struct{})
			}
			columnSets[key.Row][key.Col] = struct{}{}
		}
	}
	add(left, leftRows)
	add(right, rightRows)
	rows := make([]int, 0, len(rowSet))
	for row := range rowSet {
		rows = append(rows, row)
	}
	sort.Ints(rows)
	columnsByRow := make(map[int][]int, len(columnSets))
	for row, columns := range columnSets {
		values := make([]int, 0, len(columns))
		for col := range columns {
			values = append(values, col)
		}
		sort.Ints(values)
		columnsByRow[row] = values
	}
	return rows, leftRows, rightRows, columnsByRow
}

func rowDifference(
	left, right *workbook.SheetSnapshot,
	row int,
	columns []int,
	idColumn int,
) (different, allDataDifferent bool) {
	allDataDifferent = true
	comparedData := 0
	for _, col := range columns {
		leftValue, rightValue := left.Cell(row, col), right.Cell(row, col)
		if !leftValue.Equal(rightValue) {
			different = true
		}
		if col == idColumn || (!leftValue.Present && !rightValue.Present) {
			continue
		}
		comparedData++
		if leftValue.Equal(rightValue) {
			allDataDifferent = false
		}
	}
	return different, comparedData > 0 && allDataDifferent
}

func sheetMaxRow(sheet *workbook.SheetSnapshot) int {
	if sheet == nil {
		return 0
	}
	return sheet.MaxRow
}

func sheetMaxCol(sheet *workbook.SheetSnapshot) int {
	if sheet == nil {
		return 0
	}
	return sheet.MaxCol
}

func (result *WorkbookDiff) ReclassifyRows(
	sheetName string,
	left, right *workbook.SheetSnapshot,
) error {
	for _, sheet := range result.Sheets {
		if sheet.Name == sheetName {
			classifyRows(sheet, left, right)
			return nil
		}
	}
	return fmt.Errorf("worksheet %q not found in diff", sheetName)
}

func compareKey(left, right workbook.CellKey) int {
	if left.Row < right.Row || (left.Row == right.Row && left.Col < right.Col) {
		return -1
	}
	if left == right {
		return 0
	}
	return 1
}

// UpdateCell incrementally updates an existing two-sided worksheet diff.
// It keeps edits proportional to the number of differences instead of
// rescanning the workbook after every UI operation.
func (result *WorkbookDiff) UpdateCell(ref workbook.CellRef, left, right workbook.CellValue) error {
	var sheet *SheetDiff
	for _, candidate := range result.Sheets {
		if candidate.Name == ref.Sheet {
			sheet = candidate
			break
		}
	}
	if sheet == nil {
		return fmt.Errorf("worksheet %q not found in diff", ref.Sheet)
	}
	if sheet.LeftIndex < 0 || sheet.RightIndex < 0 {
		return fmt.Errorf("incremental cell update requires worksheet %q on both sides", ref.Sheet)
	}
	wasDifferent := sheet.Status != SheetEqual || sheet.OrderDifferent
	index := sort.Search(len(sheet.Differences), func(i int) bool {
		item := sheet.Differences[i].Ref
		return item.Row > ref.Row || (item.Row == ref.Row && item.Col >= ref.Col)
	})
	found := index < len(sheet.Differences) &&
		sheet.Differences[index].Ref.Row == ref.Row &&
		sheet.Differences[index].Ref.Col == ref.Col
	if left.Equal(right) {
		if found {
			sheet.Differences = append(sheet.Differences[:index], sheet.Differences[index+1:]...)
			sheet.DifferenceCount--
			result.DifferenceCount--
		}
	} else {
		status := Modified
		switch {
		case !left.Present && right.Present:
			status = RightOnly
		case left.Present && !right.Present:
			status = LeftOnly
		}
		item := CellDiff{Ref: ref, Status: status, Left: left, Right: right}
		if found {
			sheet.Differences[index] = item
		} else {
			sheet.Differences = append(sheet.Differences, CellDiff{})
			copy(sheet.Differences[index+1:], sheet.Differences[index:])
			sheet.Differences[index] = item
			sheet.DifferenceCount++
			result.DifferenceCount++
		}
	}
	if sheet.DifferenceCount == 0 {
		sheet.Status = SheetEqual
	} else {
		sheet.Status = SheetModified
	}
	sheet.MaxRow = max(sheet.MaxRow, ref.Row)
	sheet.MaxCol = max(sheet.MaxCol, ref.Col)
	isDifferent := sheet.Status != SheetEqual || sheet.OrderDifferent
	switch {
	case wasDifferent && !isDifferent:
		result.DifferentSheetCount--
	case !wasDifferent && isDifferent:
		result.DifferentSheetCount++
	}
	result.Equal = result.DifferenceCount == 0 && result.DifferentSheetCount == 0
	return nil
}
