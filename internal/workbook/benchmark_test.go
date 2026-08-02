package workbook

import (
	"path/filepath"
	"testing"

	"github.com/tadazly/sheetproof/internal/testutil"
)

func BenchmarkRead100KCells(b *testing.B) {
	path := filepath.Join(b.TempDir(), "read-100k.xlsx")
	if err := testutil.CreateLarge(path, 1, 100_000); err != nil {
		b.Fatal(err)
	}
	reader := Reader{}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		file, _, err := reader.Open(path)
		if err != nil {
			b.Fatal(err)
		}
		_ = file.Close()
	}
}
