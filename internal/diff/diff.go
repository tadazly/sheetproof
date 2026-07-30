package diff

import (
	"fmt"
	"sort"

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

type SheetStatus string

const (
	SheetEqual     SheetStatus = "equal"
	SheetModified  SheetStatus = "modified"
	SheetLeftOnly  SheetStatus = "left-only"
	SheetRightOnly SheetStatus = "right-only"
)

type CellDiff struct {
	Ref    workbook.CellRef   `json:"ref"`
	Status CellStatus         `json:"status"`
	Left   workbook.CellValue `json:"left"`
	Right  workbook.CellValue `json:"right"`
}

type SheetDiff struct {
	Name            string      `json:"name"`
	Status          SheetStatus `json:"status"`
	LeftIndex       int         `json:"leftIndex"`
	RightIndex      int         `json:"rightIndex"`
	OrderDifferent  bool        `json:"orderDifferent"`
	DifferenceCount int         `json:"differenceCount"`
	MaxRow          int         `json:"maxRow"`
	MaxCol          int         `json:"maxCol"`
	Differences     []CellDiff  `json:"differences,omitempty"`
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
