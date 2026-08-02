package merge

import (
	"strings"
	"testing"

	"github.com/ug-tools/ugxlsx/internal/workbook"
	"github.com/xuri/excelize/v2"
)

func TestApplyIgnoresUnsupportedFillPatternWithoutPoisoningCell(t *testing.T) {
	file := excelize.NewFile()
	defer file.Close()
	ref := workbook.CellRef{Sheet: "Sheet1", Row: 1, Col: 1}
	state := CellState{
		Value: workbook.CellValue{Present: true, Raw: "copied", Display: "copied", Type: "string"},
		Style: &excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 99, Color: []string{"FF6600"}}},
	}
	warnings, err := Apply(file, ref, state)
	if err != nil {
		t.Fatalf("unsupported fill pattern blocked the copy: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "超出支持范围") {
		t.Fatalf("warnings = %#v", warnings)
	}
	if got, err := file.GetCellValue("Sheet1", "A1"); err != nil || got != "copied" {
		t.Fatalf("copied value = %q, err=%v", got, err)
	}

	// A malformed source style must not leave the workbook in a state that
	// makes every later context-menu operation fail.
	next := CellState{Value: workbook.CellValue{Present: true, Raw: "next", Display: "next", Type: "string"}}
	if _, err := Apply(file, ref, next); err != nil {
		t.Fatalf("later copy failed: %v", err)
	}
	if got, err := file.GetCellValue("Sheet1", "A1"); err != nil || got != "next" {
		t.Fatalf("later copied value = %q, err=%v", got, err)
	}
}
