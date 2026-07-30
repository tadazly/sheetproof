package diff

import (
	"fmt"
	"testing"

	"github.com/ug-tools/ugxlsx/internal/workbook"
)

func BenchmarkCompare100KIdentical(b *testing.B) {
	left, right := benchmarkBooks(100_000, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = Compare(left, right)
	}
}

func BenchmarkCompare100KManyDifferences(b *testing.B) {
	left, right := benchmarkBooks(100_000, 10_000)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = Compare(left, right)
	}
}

func BenchmarkCompare1M10SheetsFewDifferences(b *testing.B) {
	left, right := benchmarkBooks(1_000_000, 100)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = Compare(left, right)
	}
}

func benchmarkBooks(total, differenceCount int) (*workbook.WorkbookSnapshot, *workbook.WorkbookSnapshot) {
	left := &workbook.WorkbookSnapshot{Path: "left.xlsx", ByName: map[string]*workbook.SheetSnapshot{}}
	right := &workbook.WorkbookSnapshot{Path: "right.xlsx", ByName: map[string]*workbook.SheetSnapshot{}}
	sheets := 10
	perSheet := total / sheets
	for s := 0; s < sheets; s++ {
		name := fmt.Sprintf("Sheet%d", s+1)
		ls := &workbook.SheetSnapshot{Name: name, Index: s, Cells: make(map[workbook.CellKey]workbook.CellValue, perSheet), CellList: make([]workbook.CellKey, 0, perSheet)}
		rs := &workbook.SheetSnapshot{Name: name, Index: s, Cells: make(map[workbook.CellKey]workbook.CellValue, perSheet), CellList: make([]workbook.CellKey, 0, perSheet)}
		for i := 0; i < perSheet; i++ {
			key := workbook.CellKey{Row: i/20 + 1, Col: i%20 + 1}
			raw := fmt.Sprintf("%d", i)
			lv := workbook.CellValue{Present: true, Raw: raw, Display: raw, Type: "number"}
			rv := lv
			global := s*perSheet + i
			if global < differenceCount {
				rv.Raw, rv.Display = "changed", "changed"
			}
			ls.Cells[key], rs.Cells[key] = lv, rv
			ls.CellList = append(ls.CellList, key)
			rs.CellList = append(rs.CellList, key)
		}
		ls.MaxRow, rs.MaxRow = (perSheet+19)/20, (perSheet+19)/20
		ls.MaxCol, rs.MaxCol = 20, 20
		left.Sheets, right.Sheets = append(left.Sheets, ls), append(right.Sheets, rs)
		left.ByName[name], right.ByName[name] = ls, rs
	}
	return left, right
}
