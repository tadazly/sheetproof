package fileopener

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateWorkbookRequiresExistingXLSXFile(t *testing.T) {
	dir := t.TempDir()
	workbook := filepath.Join(dir, "book.xlsx")
	if err := os.WriteFile(workbook, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := validateWorkbook(workbook)
	if err != nil {
		t.Fatal(err)
	}
	if got != workbook {
		t.Fatalf("validated path = %q, want %q", got, workbook)
	}
	if _, err := validateWorkbook(dir); err == nil {
		t.Fatal("directory unexpectedly accepted as workbook")
	}
	if _, err := validateWorkbook(filepath.Join(dir, "missing.xlsx")); err == nil {
		t.Fatal("missing workbook unexpectedly accepted")
	}
}
