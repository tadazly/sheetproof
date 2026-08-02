package storage

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/tadazly/sheetproof/internal/testutil"
	"github.com/tadazly/sheetproof/internal/workbook"
)

func BenchmarkSafeSave100KCells(b *testing.B) {
	dir := b.TempDir()
	source := filepath.Join(dir, "source.xlsx")
	if err := testutil.CreateLarge(source, 1, 100_000); err != nil {
		b.Fatal(err)
	}
	file, _, err := (workbook.Reader{}).Open(source)
	if err != nil {
		b.Fatal(err)
	}
	defer file.Close()
	writer := SafeWriter{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		target := filepath.Join(dir, fmt.Sprintf("saved-%d.xlsx", i))
		if _, err := writer.Save(file, target, nil); err != nil {
			b.Fatal(err)
		}
	}
}
