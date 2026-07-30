package diff

import (
	"testing"

	"github.com/ug-tools/ugxlsx/internal/workbook"
)

func TestCompareCellKindsAndSheetStates(t *testing.T) {
	leftSheet := sheet("数据", 0, map[workbook.CellKey]workbook.CellValue{
		{Row: 1, Col: 1}: value("文本", "string"),
		{Row: 1, Col: 2}: value("0", "number"),
		{Row: 1, Col: 3}: value("", "string"),
		{Row: 1, Col: 4}: value("1", "bool"),
		{Row: 1, Col: 5}: {Present: true, Formula: "SUM(A1:B1)", Type: "formula"},
	})
	rightSheet := sheet("数据", 1, map[workbook.CellKey]workbook.CellValue{
		{Row: 1, Col: 1}: value("另一文本", "string"),
		{Row: 1, Col: 3}: value("", "string"),
		{Row: 1, Col: 4}: value("0", "bool"),
		{Row: 1, Col: 5}: {Present: true, Formula: "SUM(A1:B2)", Type: "formula"},
		{Row: 2, Col: 1}: value("新增", "string"),
	})
	leftOnly := sheet("左侧", 1, map[workbook.CellKey]workbook.CellValue{{Row: 1, Col: 1}: value("中", "string")})
	rightOnly := sheet("右侧", 2, map[workbook.CellKey]workbook.CellValue{{Row: 1, Col: 1}: value("文", "string")})
	left := book("left.xlsx", leftSheet, leftOnly)
	right := book("right.xlsx", rightSheet, rightOnly)

	result := Compare(left, right)
	if result.Equal {
		t.Fatal("expected different workbooks")
	}
	if result.DifferenceCount != 7 {
		t.Fatalf("difference count = %d, want 7", result.DifferenceCount)
	}
	if result.DifferentSheetCount != 3 {
		t.Fatalf("different sheet count = %d, want 3", result.DifferentSheetCount)
	}
	data := result.Sheets[0]
	if !data.OrderDifferent || data.Status != SheetModified {
		t.Fatalf("unexpected data sheet state: %+v", data)
	}
	wantStatuses := []CellStatus{Modified, LeftOnly, Modified, Modified, RightOnly}
	for index, want := range wantStatuses {
		if data.Differences[index].Status != want {
			t.Fatalf("diff %d status = %s, want %s", index, data.Differences[index].Status, want)
		}
	}
}

func TestCompareIdentical(t *testing.T) {
	cells := map[workbook.CellKey]workbook.CellValue{{Row: 100, Col: 50}: value("边界", "string")}
	left := book("a.xlsx", sheet("中文 表", 0, cells))
	right := book("b.xlsx", sheet("中文 表", 0, cells))
	result := Compare(left, right)
	if !result.Equal || result.DifferenceCount != 0 {
		t.Fatalf("expected equal: %+v", result)
	}
}

func value(raw, kind string) workbook.CellValue {
	return workbook.CellValue{Present: true, Raw: raw, Display: raw, Type: kind}
}

func sheet(name string, index int, cells map[workbook.CellKey]workbook.CellValue) *workbook.SheetSnapshot {
	result := &workbook.SheetSnapshot{Name: name, Index: index, Cells: cells}
	for key := range cells {
		result.CellList = append(result.CellList, key)
		result.MaxRow = max(result.MaxRow, key.Row)
		result.MaxCol = max(result.MaxCol, key.Col)
	}
	for i := 1; i < len(result.CellList); i++ {
		for j := i; j > 0 && compareKey(result.CellList[j], result.CellList[j-1]) < 0; j-- {
			result.CellList[j], result.CellList[j-1] = result.CellList[j-1], result.CellList[j]
		}
	}
	return result
}

func book(path string, sheets ...*workbook.SheetSnapshot) *workbook.WorkbookSnapshot {
	result := &workbook.WorkbookSnapshot{Path: path, Sheets: sheets, ByName: make(map[string]*workbook.SheetSnapshot)}
	for _, item := range sheets {
		result.ByName[item.Name] = item
	}
	return result
}
