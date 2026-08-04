package workbook

import "strings"

// RecordKeyColumn returns the shared column that identifies logical records.
//
// A literal "id" header is unambiguous and remains the first choice. Localized
// table schemas often qualify that name (for example "地图ID"), so a single
// shared header ending in ID is also accepted. Multiple qualified-ID columns
// are deliberately treated as ambiguous: callers must retain physical-row
// semantics instead of guessing between fields such as mapID and parentID.
func RecordKeyColumn(left, right *SheetSnapshot) int {
	maxCol := max(sheetColumnCount(left), sheetColumnCount(right))
	qualified := 0
	qualifiedCount := 0
	for col := 1; col <= maxCol; col++ {
		leftHeader := cellText(left.Cell(1, col))
		rightHeader := cellText(right.Cell(1, col))
		if leftHeader == "" || rightHeader == "" || !strings.EqualFold(leftHeader, rightHeader) {
			continue
		}
		if strings.EqualFold(leftHeader, "id") {
			return col
		}
		if hasIDSuffix(leftHeader) {
			qualified = col
			qualifiedCount++
		}
	}
	if qualifiedCount == 1 {
		return qualified
	}
	return 0
}

func hasIDSuffix(header string) bool {
	header = strings.TrimSpace(header)
	return len(header) > 2 && strings.EqualFold(header[len(header)-2:], "id")
}

func cellText(value CellValue) string {
	text := value.Raw
	if text == "" {
		text = value.Display
	}
	return strings.TrimSpace(text)
}

func sheetColumnCount(sheet *SheetSnapshot) int {
	if sheet == nil {
		return 0
	}
	return sheet.MaxCol
}
