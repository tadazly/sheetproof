package workbook

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"
)

type worksheetCellRange struct {
	start  int
	end    int
	column int
}

type worksheetRowRange struct {
	depth        int
	contentStart int
	contentEnd   int
	cells        []worksheetCellRange
	sortable     bool
}

type worksheetReplacement struct {
	start int
	end   int
	data  []byte
}

const (
	spreadsheetMLTransitional = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	spreadsheetMLStrict       = "http://purl.oclc.org/ooxml/spreadsheetml/main"
)

// openCellOrderRepairedWorkbook creates an internal temporary XLSX in which
// out-of-order worksheet cell elements are sorted by column. Some spreadsheet
// applications tolerate that non-canonical OOXML, while excelize v2.10.1 can
// panic when an earlier cell is to the right of the row's final cell.
func openCellOrderRepairedWorkbook(ctx context.Context, source string) (*excelize.File, int, error) {
	temporary, reorderedRows, err := writeCellOrderRepairedCopy(ctx, source)
	if err != nil || reorderedRows == 0 {
		return nil, reorderedRows, err
	}
	defer os.Remove(temporary)
	file, err := excelize.OpenFile(temporary, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, reorderedRows, err
	}
	// Session persistence always uses SafeWriter with the original snapshot
	// path. Keep excelize's own path consistent as well.
	file.Path = source
	return file, reorderedRows, nil
}

func writeCellOrderRepairedCopy(ctx context.Context, source string) (path string, reorderedRows int, err error) {
	archive, err := zip.OpenReader(source)
	if err != nil {
		return "", 0, err
	}
	defer archive.Close()

	temporary, err := os.CreateTemp("", "sheetproof-cell-order-*.xlsx")
	if err != nil {
		return "", 0, err
	}
	temporaryPath := temporary.Name()
	keep := false
	defer func() {
		_ = temporary.Close()
		if !keep {
			_ = os.Remove(temporaryPath)
		}
	}()

	writer := zip.NewWriter(temporary)
	writerClosed := false
	defer func() {
		if !writerClosed {
			_ = writer.Close()
		}
	}()

	for _, entry := range archive.File {
		if err := contextError(ctx); err != nil {
			return "", 0, err
		}
		if !isWorksheetXML(entry.Name) {
			if err := writer.Copy(entry); err != nil {
				return "", 0, err
			}
			continue
		}

		reader, err := entry.Open()
		if err != nil {
			return "", 0, err
		}
		data, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			return "", 0, readErr
		}
		if closeErr != nil {
			return "", 0, closeErr
		}
		normalized, count, err := normalizeWorksheetCellOrder(data)
		if err != nil {
			return "", 0, fmt.Errorf("normalize %s: %w", entry.Name, err)
		}
		if count == 0 {
			if err := writer.Copy(entry); err != nil {
				return "", 0, err
			}
			continue
		}
		header := entry.FileHeader
		header.CRC32 = 0
		header.CompressedSize = 0
		header.CompressedSize64 = 0
		header.UncompressedSize = 0
		header.UncompressedSize64 = 0
		destination, err := writer.CreateHeader(&header)
		if err != nil {
			return "", 0, err
		}
		if _, err := destination.Write(normalized); err != nil {
			return "", 0, err
		}
		reorderedRows += count
	}
	if err := writer.Close(); err != nil {
		return "", 0, err
	}
	writerClosed = true
	if err := temporary.Sync(); err != nil {
		return "", 0, err
	}
	if err := temporary.Close(); err != nil {
		return "", 0, err
	}
	keep = true
	return temporaryPath, reorderedRows, nil
}

func isWorksheetXML(name string) bool {
	normalized := strings.ToLower(filepath.ToSlash(name))
	return strings.HasPrefix(normalized, "xl/worksheets/") && strings.HasSuffix(normalized, ".xml")
}

func normalizeWorksheetCellOrder(source []byte) ([]byte, int, error) {
	decoder := xml.NewDecoder(bytes.NewReader(source))
	depth := 0
	var row *worksheetRowRange
	var cell *worksheetCellRange
	replacements := make([]worksheetReplacement, 0)

	for {
		tokenStart := decoder.InputOffset()
		token, err := decoder.Token()
		tokenEnd := decoder.InputOffset()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, err
		}
		switch current := token.(type) {
		case xml.StartElement:
			depth++
			if row == nil && isSpreadsheetMLElement(current.Name, "row") {
				row = &worksheetRowRange{
					depth: depth, contentStart: int(tokenEnd), sortable: true,
				}
				continue
			}
			if row == nil || cell != nil || depth != row.depth+1 || !isSpreadsheetMLElement(current.Name, "c") {
				continue
			}
			column, ok := worksheetCellColumn(current)
			if !ok {
				row.sortable = false
			}
			cell = &worksheetCellRange{start: int(tokenStart), column: column}
		case xml.EndElement:
			if row != nil && cell != nil && depth == row.depth+1 && isSpreadsheetMLElement(current.Name, "c") {
				cell.end = int(tokenEnd)
				row.cells = append(row.cells, *cell)
				cell = nil
			}
			if row != nil && depth == row.depth && isSpreadsheetMLElement(current.Name, "row") {
				row.contentEnd = int(tokenStart)
				if row.sortable && worksheetCellsOutOfOrder(row.cells) {
					replacements = append(replacements, worksheetReplacement{
						start: row.contentStart,
						end:   row.contentEnd,
						data:  reorderWorksheetRowContent(source, *row),
					})
				}
				row = nil
			}
			depth--
		}
	}
	if len(replacements) == 0 {
		return source, 0, nil
	}
	var result bytes.Buffer
	result.Grow(len(source))
	previous := 0
	for _, replacement := range replacements {
		result.Write(source[previous:replacement.start])
		result.Write(replacement.data)
		previous = replacement.end
	}
	result.Write(source[previous:])
	return result.Bytes(), len(replacements), nil
}

func isSpreadsheetMLElement(name xml.Name, local string) bool {
	if name.Local != local {
		return false
	}
	return name.Space == "" || name.Space == spreadsheetMLTransitional || name.Space == spreadsheetMLStrict
}

func worksheetCellColumn(element xml.StartElement) (int, bool) {
	for _, attribute := range element.Attr {
		if attribute.Name.Local != "r" || attribute.Name.Space != "" {
			continue
		}
		column, _, err := excelize.CellNameToCoordinates(attribute.Value)
		return column, err == nil && column > 0
	}
	return 0, false
}

func worksheetCellsOutOfOrder(cells []worksheetCellRange) bool {
	for index := 1; index < len(cells); index++ {
		if cells[index].column < cells[index-1].column {
			return true
		}
	}
	return false
}

func reorderWorksheetRowContent(source []byte, row worksheetRowRange) []byte {
	ordered := append([]worksheetCellRange(nil), row.cells...)
	sort.SliceStable(ordered, func(left, right int) bool {
		return ordered[left].column < ordered[right].column
	})
	var result bytes.Buffer
	result.Grow(row.contentEnd - row.contentStart)
	result.Write(source[row.contentStart:row.cells[0].start])
	for index, slot := range row.cells {
		selected := ordered[index]
		result.Write(source[selected.start:selected.end])
		gapEnd := row.contentEnd
		if index+1 < len(row.cells) {
			gapEnd = row.cells[index+1].start
		}
		result.Write(source[slot.end:gapEnd])
	}
	return result.Bytes()
}
