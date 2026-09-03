package workbook

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestNormalizeWorksheetCellOrderPreservesNamespacedCellXML(t *testing.T) {
	source := []byte(`<?xml version="1.0" encoding="UTF-8"?>` +
		`<x:worksheet xmlns:x="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<x:sheetData><x:row r="1">` +
		`<x:c r="C1"><x:f>A1+1</x:f><x:v>3</x:v></x:c><!--slot-->` +
		`<x:c r="A1" t="inlineStr"><x:is><x:t>A</x:t></x:is></x:c>` +
		`<x:c r="B1"/>` +
		`</x:row></x:sheetData></x:worksheet>`)

	normalized, rows, err := normalizeWorksheetCellOrder(source)
	if err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("reordered rows = %d, want 1", rows)
	}
	text := string(normalized)
	positions := []int{
		strings.Index(text, `r="A1"`),
		strings.Index(text, `r="B1"`),
		strings.Index(text, `r="C1"`),
	}
	if positions[0] < 0 || positions[1] <= positions[0] || positions[2] <= positions[1] {
		t.Fatalf("cell order was not normalized: %s", text)
	}
	for _, preserved := range []string{"<!--slot-->", "<x:f>A1+1</x:f>", "<x:t>A</x:t>"} {
		if !strings.Contains(text, preserved) {
			t.Fatalf("normalized XML lost %q: %s", preserved, text)
		}
	}
	second, rows, err := normalizeWorksheetCellOrder(normalized)
	if err != nil {
		t.Fatal(err)
	}
	if rows != 0 || !bytes.Equal(second, normalized) {
		t.Fatal("normalizing an ordered worksheet should be a byte-for-byte no-op")
	}
}

func TestReaderRepairsOutOfOrderCellsWithoutChangingSource(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "out-of-order.xlsx")
	file := excelize.NewFile()
	if err := file.SetCellStr("Sheet1", "A1", "alpha"); err != nil {
		t.Fatal(err)
	}
	if err := file.SetCellInt("Sheet1", "Q1", 17); err != nil {
		t.Fatal(err)
	}
	if err := file.SetCellStr("Sheet1", "S1", "right"); err != nil {
		t.Fatal(err)
	}
	if err := file.SetCellFormula("Sheet1", "T1", "Q1+1"); err != nil {
		t.Fatal(err)
	}
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	disorderWorksheetRowForTest(t, path, "xl/worksheets/sheet1.xml", 1, []string{"A1", "S1", "T1", "Q1"})

	before, err := Identity(path)
	if err != nil {
		t.Fatal(err)
	}
	opened, snapshot, err := (Reader{}).Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	after, err := Identity(path)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("tolerant open modified the source workbook")
	}
	sheet := snapshot.ByName["Sheet1"]
	if got := sheet.Cell(1, 1); !got.Present || got.Raw != "alpha" || got.Type != "string" {
		t.Fatalf("A1 = %+v", got)
	}
	if got := sheet.Cell(1, 17); !got.Present || got.Raw != "17" || got.Type != "number" {
		t.Fatalf("Q1 = %+v", got)
	}
	if got := sheet.Cell(1, 19); !got.Present || got.Raw != "right" {
		t.Fatalf("S1 = %+v", got)
	}
	if got := sheet.Cell(1, 20); !got.Present || got.Formula != "Q1+1" {
		t.Fatalf("T1 = %+v", got)
	}
}

func disorderWorksheetRowForTest(
	t *testing.T,
	path, entryName string,
	row int,
	cellOrder []string,
) {
	t.Helper()
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	found := false
	for _, entry := range archive.File {
		if entry.Name != entryName {
			if err := writer.Copy(entry); err != nil {
				t.Fatal(err)
			}
			continue
		}
		found = true
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		data = permuteWorksheetRowForTest(t, data, row, cellOrder)
		header := entry.FileHeader
		header.CRC32 = 0
		header.CompressedSize = 0
		header.CompressedSize64 = 0
		header.UncompressedSize = 0
		header.UncompressedSize64 = 0
		destination, err := writer.CreateHeader(&header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := destination.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if !found {
		t.Fatalf("worksheet entry %q was not found", entryName)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, output.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func permuteWorksheetRowForTest(t *testing.T, source []byte, row int, cellOrder []string) []byte {
	t.Helper()
	rowPattern := regexp.MustCompile(`(?s)<row\b[^>]*\br="` + regexp.QuoteMeta(strconv.Itoa(row)) + `"[^>]*>(.*?)</row>`)
	rowMatch := rowPattern.FindSubmatchIndex(source)
	if rowMatch == nil {
		t.Fatalf("row %d was not found in worksheet XML", row)
	}
	content := source[rowMatch[2]:rowMatch[3]]
	var reordered bytes.Buffer
	for _, cell := range cellOrder {
		cellPattern := regexp.MustCompile(`(?s)<c\b[^>]*\br="` + regexp.QuoteMeta(cell) + `"[^>]*(?:/>|>.*?</c>)`)
		fragment := cellPattern.Find(content)
		if fragment == nil {
			t.Fatalf("cell %s was not found in row %d", cell, row)
		}
		reordered.Write(fragment)
	}
	result := append([]byte(nil), source[:rowMatch[2]]...)
	result = append(result, reordered.Bytes()...)
	result = append(result, source[rowMatch[3]:]...)
	return result
}
