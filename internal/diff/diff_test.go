package diff

import (
	"path/filepath"
	"testing"

	"github.com/tadazly/sheetproof/internal/workbook"
	"github.com/xuri/excelize/v2"
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

func TestCompareTreatsSharedAndInlineStringsAsEqual(t *testing.T) {
	dir := t.TempDir()
	sharedPath := filepath.Join(dir, "shared.xlsx")
	inlinePath := filepath.Join(dir, "inline.xlsx")

	shared := excelize.NewFile()
	if err := shared.SetCellStr("Sheet1", "A1", "相同文本"); err != nil {
		t.Fatal(err)
	}
	if err := shared.SaveAs(sharedPath); err != nil {
		t.Fatal(err)
	}
	if err := shared.Close(); err != nil {
		t.Fatal(err)
	}

	inline := excelize.NewFile()
	writer, err := inline.NewStreamWriter("Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.SetRow("A1", []any{"相同文本"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := inline.SaveAs(inlinePath); err != nil {
		t.Fatal(err)
	}
	if err := inline.Close(); err != nil {
		t.Fatal(err)
	}

	leftFile, left, err := (workbook.Reader{}).Open(sharedPath)
	if err != nil {
		t.Fatal(err)
	}
	defer leftFile.Close()
	rightFile, right, err := (workbook.Reader{}).Open(inlinePath)
	if err != nil {
		t.Fatal(err)
	}
	defer rightFile.Close()
	leftStorageType, err := leftFile.GetCellType("Sheet1", "A1")
	if err != nil {
		t.Fatal(err)
	}
	rightStorageType, err := rightFile.GetCellType("Sheet1", "A1")
	if err != nil {
		t.Fatal(err)
	}
	if leftStorageType != excelize.CellTypeSharedString || rightStorageType != excelize.CellTypeInlineString {
		t.Fatalf("fixture storage types = %v/%v, want shared/inline", leftStorageType, rightStorageType)
	}

	leftCell := left.ByName["Sheet1"].Cell(1, 1)
	rightCell := right.ByName["Sheet1"].Cell(1, 1)
	if leftCell.Type != "string" || rightCell.Type != "string" {
		t.Fatalf("normalized types = %q/%q, want string/string", leftCell.Type, rightCell.Type)
	}
	result := Compare(left, right)
	if !result.Equal || result.DifferenceCount != 0 {
		t.Fatalf("storage-only string encoding produced differences: %+v", result)
	}
}

func TestCompareClassifiesAddedDeletedModifiedAndConflictRows(t *testing.T) {
	left := book("left.xlsx", sheet("配置", 0, map[workbook.CellKey]workbook.CellValue{
		{Row: 1, Col: 1}: value("id", "string"),
		{Row: 1, Col: 2}: value("name", "string"),
		{Row: 1, Col: 3}: value("value", "string"),
		{Row: 2, Col: 1}: value("1", "number"),
		{Row: 2, Col: 2}: value("left-a", "string"),
		{Row: 2, Col: 3}: value("left-b", "string"),
		{Row: 3, Col: 1}: value("2", "number"),
		{Row: 3, Col: 2}: value("same", "string"),
		{Row: 3, Col: 3}: value("left", "string"),
		{Row: 5, Col: 1}: value("4", "number"),
		{Row: 5, Col: 2}: value("deleted", "string"),
	}))
	right := book("right.xlsx", sheet("配置", 0, map[workbook.CellKey]workbook.CellValue{
		{Row: 1, Col: 1}: value("ID", "string"),
		{Row: 1, Col: 2}: value("name", "string"),
		{Row: 1, Col: 3}: value("value", "string"),
		{Row: 2, Col: 1}: value("1", "number"),
		{Row: 2, Col: 2}: value("right-a", "string"),
		{Row: 2, Col: 3}: value("right-b", "string"),
		{Row: 3, Col: 1}: value("2", "number"),
		{Row: 3, Col: 2}: value("same", "string"),
		{Row: 3, Col: 3}: value("right", "string"),
		{Row: 4, Col: 1}: value("3", "number"),
		{Row: 4, Col: 2}: value("added", "string"),
	}))

	result := Compare(left, right)
	data := result.Sheets[0]
	if data.IDColumn != 1 || data.NextID != 5 {
		t.Fatalf("id metadata = column %d next %d", data.IDColumn, data.NextID)
	}
	if data.AddedRowCount != 1 || data.DeletedRowCount != 1 ||
		data.ModifiedRowCount != 1 || data.ConflictRowCount != 1 {
		t.Fatalf("row counts = added %d deleted %d modified %d conflict %d",
			data.AddedRowCount, data.DeletedRowCount,
			data.ModifiedRowCount, data.ConflictRowCount)
	}
	want := []RowDiff{
		{Row: 2, ID: "1", Status: RowConflict},
		{Row: 3, ID: "2", Status: RowModified},
		{Row: 4, ID: "3", Status: RowAdded},
		{Row: 5, ID: "4", Status: RowDeleted},
	}
	if len(data.Rows) != len(want) {
		t.Fatalf("rows = %+v", data.Rows)
	}
	for index := range want {
		if data.Rows[index] != want[index] {
			t.Fatalf("row %d = %+v, want %+v", index, data.Rows[index], want[index])
		}
	}
	for _, item := range data.Differences {
		if item.Ref.Row == 2 && item.RowStatus != RowConflict {
			t.Fatalf("conflict cell row status = %s", item.RowStatus)
		}
	}
}

func TestCompareDoesNotOfferNextIDForTextIDData(t *testing.T) {
	left := book("left.xlsx", sheet("属性", 0, map[workbook.CellKey]workbook.CellValue{
		{Row: 1, Col: 1}: value("id", "string"),
		{Row: 1, Col: 2}: value("name", "string"),
		{Row: 2, Col: 1}: value("活动id", "string"),
		{Row: 2, Col: 2}: value("left", "string"),
	}))
	right := book("right.xlsx", sheet("属性", 0, map[workbook.CellKey]workbook.CellValue{
		{Row: 1, Col: 1}: value("id", "string"),
		{Row: 1, Col: 2}: value("name", "string"),
		{Row: 2, Col: 1}: value("活动id", "string"),
		{Row: 2, Col: 2}: value("right", "string"),
	}))

	data := Compare(left, right).Sheets[0]
	if data.IDColumn != 1 || data.NextID != 0 || data.ConflictRowCount != 1 {
		t.Fatalf("text ID metadata = column %d next %d conflicts %d", data.IDColumn, data.NextID, data.ConflictRowCount)
	}
}

func TestCompareUsesLocalizedQualifiedIDForConflictClassification(t *testing.T) {
	left := book("left.xlsx", sheet("配置", 0, map[workbook.CellKey]workbook.CellValue{
		{Row: 1, Col: 1}: value("地图ID", "string"),
		{Row: 1, Col: 2}: value("name", "string"),
		{Row: 1, Col: 3}: value("resource", "string"),
		{Row: 2, Col: 1}: value("50001", "number"),
		{Row: 2, Col: 2}: value("left", "string"),
		{Row: 2, Col: 3}: value("left-room", "string"),
	}))
	right := book("right.xlsx", sheet("配置", 0, map[workbook.CellKey]workbook.CellValue{
		{Row: 1, Col: 1}: value("地图ID", "string"),
		{Row: 1, Col: 2}: value("name", "string"),
		{Row: 1, Col: 3}: value("resource", "string"),
		{Row: 2, Col: 1}: value("50001", "number"),
		{Row: 2, Col: 2}: value("right", "string"),
		{Row: 2, Col: 3}: value("right-room", "string"),
	}))

	data := Compare(left, right).Sheets[0]
	if data.IDColumn != 1 || data.NextID != 50002 || data.ConflictRowCount != 1 || data.ModifiedRowCount != 0 {
		t.Fatalf("localized ID metadata/classification = %+v", data)
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
