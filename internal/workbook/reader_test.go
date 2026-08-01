package workbook

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ug-tools/ugxlsx/internal/testutil"
	"github.com/xuri/excelize/v2"
)

func TestReaderOpenContextStopsBeforeCancelledScan(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := (Reader{}).OpenContext(ctx, filepath.Join(t.TempDir(), "ignored.xlsx")); !errors.Is(err, context.Canceled) {
		t.Fatalf("OpenContext error = %v, want context canceled", err)
	}
}

func TestReaderPreservesEmptyZeroFormulaAndUnicode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "类型 中文.xlsx")
	f := excelize.NewFile()
	if err := f.SetCellStr("Sheet1", "A1", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellInt("Sheet1", "B1", 0); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellBool("Sheet1", "C1", false); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellFormula("Sheet1", "D1", "B1+1"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellStr("Sheet1", "E1", "中文 <>&"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("Sheet1", "F1", time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	opened, snapshot, err := (Reader{}).Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	sheet := snapshot.ByName["Sheet1"]
	if !sheet.Cell(1, 1).Present || sheet.Cell(1, 1).Raw != "" {
		t.Fatalf("explicit empty string lost: %+v", sheet.Cell(1, 1))
	}
	if got := sheet.Cell(1, 2); !got.Present || got.Raw != "0" || got.Type != "number" {
		t.Fatalf("zero mismatch: %+v", got)
	}
	if got := sheet.Cell(1, 3); !got.Present || got.Type != "bool" {
		t.Fatalf("bool mismatch: %+v", got)
	}
	if got := sheet.Cell(1, 4).Formula; got != "B1+1" {
		t.Fatalf("formula = %q", got)
	}
	if got := sheet.Cell(1, 6); got.Type != "date" || got.Raw == "" {
		t.Fatalf("date mismatch: %+v", got)
	}
	if sheet.Cell(2, 1).Present {
		t.Fatal("truly empty cell should not be present")
	}
}

func TestReaderErrors(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := (Reader{}).Open(filepath.Join(dir, "missing.xlsx")); !HasCode(err, ErrNotFound) {
		t.Fatalf("missing file error = %v", err)
	}
	txt := filepath.Join(dir, "book.xls")
	if err := os.WriteFile(txt, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := (Reader{}).Open(txt); !HasCode(err, ErrUnsupported) {
		t.Fatalf("unsupported error = %v", err)
	}
	corrupt, err := testutil.CorruptFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := (Reader{}).Open(corrupt); !HasCode(err, ErrCorrupt) {
		t.Fatalf("corrupt error = %v", err)
	}
	compound := filepath.Join(dir, "legacy-renamed.xlsx")
	if err := os.WriteFile(compound, []byte{
		0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1, 0x00,
	}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := (Reader{}).Open(compound); !HasCode(err, ErrUnsupported) {
		t.Fatalf("renamed OLE compound error = %v", err)
	}
}
