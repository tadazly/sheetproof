package workbook

import "testing"

func TestRecordKeyColumn(t *testing.T) {
	tests := []struct {
		name         string
		leftHeaders  []string
		rightHeaders []string
		want         int
	}{
		{name: "literal ID", leftHeaders: []string{"ID", "value"}, rightHeaders: []string{"id", "value"}, want: 1},
		{name: "localized qualified ID", leftHeaders: []string{"地图ID", "地图名称"}, rightHeaders: []string{"地图ID", "地图名称"}, want: 1},
		{name: "qualified ID in another column", leftHeaders: []string{"name", "productId"}, rightHeaders: []string{"name", "PRODUCTID"}, want: 2},
		{name: "different columns", leftHeaders: []string{"地图ID", "name"}, rightHeaders: []string{"name", "地图ID"}, want: 0},
		{name: "different names", leftHeaders: []string{"地图ID", "name"}, rightHeaders: []string{"场景ID", "name"}, want: 0},
		{name: "multiple qualified IDs are ambiguous", leftHeaders: []string{"地图ID", "父级ID"}, rightHeaders: []string{"地图ID", "父级ID"}, want: 0},
		{name: "ID inside a word is not a suffix", leftHeaders: []string{"identity", "value"}, rightHeaders: []string{"identity", "value"}, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			left := headerSheet(test.leftHeaders)
			right := headerSheet(test.rightHeaders)
			if got := RecordKeyColumn(left, right); got != test.want {
				t.Fatalf("RecordKeyColumn() = %d, want %d", got, test.want)
			}
		})
	}
}

func headerSheet(headers []string) *SheetSnapshot {
	sheet := &SheetSnapshot{Cells: make(map[CellKey]CellValue), MaxRow: 1, MaxCol: len(headers)}
	for index, header := range headers {
		key := CellKey{Row: 1, Col: index + 1}
		sheet.Cells[key] = CellValue{Present: true, Raw: header, Display: header, Type: "string"}
		sheet.CellList = append(sheet.CellList, key)
	}
	return sheet
}
