package app

import (
	"sort"
	"strings"

	"github.com/tadazly/sheetproof/internal/workbook"
)

type columnAlignmentResult struct {
	left          *workbook.WorkbookSnapshot
	right         *workbook.WorkbookSnapshot
	leftSources   map[string]map[int]int
	rightSources  map[string]map[int]int
	alignedSheets map[string]bool
}

type sheetColumnAlignment struct {
	leftPhysicalToLogical  map[int]int
	rightPhysicalToLogical map[int]int
	applied                bool
}

// alignWorkbookColumns uses unique, non-empty first-row headers as conservative
// anchors. At least three shared anchors are required, so a few coincidental
// data values cannot remap a headerless positional worksheet. One-sided columns stay
// next to their following shared anchor instead of shifting every later column.
func alignWorkbookColumns(left, right *workbook.WorkbookSnapshot) columnAlignmentResult {
	if right == nil {
		return columnAlignmentResult{
			left: left, leftSources: identityWorkbookColumns(left),
		}
	}
	alignedLeft := emptyAlignedWorkbook(left)
	alignedRight := emptyAlignedWorkbook(right)
	leftSources := make(map[string]map[int]int)
	rightSources := make(map[string]map[int]int)
	alignedSheets := make(map[string]bool)
	for _, name := range workbookSheetNames(left, right) {
		leftSheet, rightSheet := left.ByName[name], right.ByName[name]
		alignment := alignSheetColumns(leftSheet, rightSheet)
		if alignment.applied {
			leftSources[name] = invertColumnMapping(alignment.leftPhysicalToLogical)
			rightSources[name] = invertColumnMapping(alignment.rightPhysicalToLogical)
		} else {
			leftSources[name] = identityLogicalColumns(sheetColumnCount(leftSheet))
			rightSources[name] = identityLogicalColumns(sheetColumnCount(rightSheet))
		}
		alignedSheets[name] = alignment.applied
		if leftSheet != nil {
			appendAlignedSheet(alignedLeft, alignSheetColumnsSnapshot(leftSheet, alignment.leftPhysicalToLogical))
		}
		if rightSheet != nil {
			appendAlignedSheet(alignedRight, alignSheetColumnsSnapshot(rightSheet, alignment.rightPhysicalToLogical))
		}
	}
	return columnAlignmentResult{
		left: alignedLeft, right: alignedRight,
		leftSources: leftSources, rightSources: rightSources,
		alignedSheets: alignedSheets,
	}
}

func identityLogicalColumns(maxCol int) map[int]int {
	result := make(map[int]int, maxCol)
	for col := 1; col <= maxCol; col++ {
		result[col] = col
	}
	return result
}

func alignSheetColumns(left, right *workbook.SheetSnapshot) sheetColumnAlignment {
	identity := sheetColumnAlignment{
		leftPhysicalToLogical:  identitySheetColumns(left),
		rightPhysicalToLogical: identitySheetColumns(right),
	}
	if left == nil || right == nil {
		return identity
	}
	if !headerRowIsTextual(left) || !headerRowIsTextual(right) {
		return identity
	}
	leftHeaders, leftCounts := columnHeaders(left)
	rightHeaders, rightCounts := columnHeaders(right)
	leftUnique := uniqueHeaderColumns(leftHeaders, leftCounts)
	rightUnique := uniqueHeaderColumns(rightHeaders, rightCounts)
	leftToRight := make(map[int]int)
	rightToLeft := make(map[int]int)
	for header, leftCol := range leftUnique {
		rightCol := rightUnique[header]
		if rightCol == 0 {
			continue
		}
		leftToRight[leftCol] = rightCol
		rightToLeft[rightCol] = leftCol
	}
	if len(leftToRight) < 3 || len(leftToRight)*5 < nonEmptyHeaderCount(leftHeaders)*3 || len(leftToRight)*5 < nonEmptyHeaderCount(rightHeaders)*3 {
		return identity
	}

	// Blank/repeated headers inherit the offset of their surrounding shared
	// anchors. Physical-index pairing would undo an insertion's offset and
	// manufacture another missing column (and shifted data) after the anchor.
	// Only equally sized, order-preserving gaps provide an unambiguous pairing.
	anchors := []int{0}
	for col := range leftToRight {
		anchors = append(anchors, col)
	}
	sort.Ints(anchors)
	anchors = append(anchors, left.MaxCol+1)
	previousRight := 0
	for index := 1; index < len(anchors); index++ {
		previousLeft, nextLeft := anchors[index-1], anchors[index]
		nextRight := right.MaxCol + 1
		if nextLeft <= left.MaxCol {
			nextRight = leftToRight[nextLeft]
		}
		if nextRight-previousRight == nextLeft-previousLeft {
			clearGap := true
			for col := previousRight + 1; col < nextRight; col++ {
				if rightToLeft[col] != 0 {
					clearGap = false
					break
				}
			}
			for leftCol := previousLeft + 1; leftCol < nextLeft; leftCol++ {
				if !clearGap {
					break
				}
				rightCol := previousRight + leftCol - previousLeft
				leftHeader, rightHeader := leftHeaders[leftCol], rightHeaders[rightCol]
				leftAmbiguous := leftHeader == "" || leftCounts[leftHeader] != 1
				rightAmbiguous := rightHeader == "" || rightCounts[rightHeader] != 1
				if rightToLeft[rightCol] == 0 && leftAmbiguous && rightAmbiguous && leftHeader == rightHeader {
					leftToRight[leftCol] = rightCol
					rightToLeft[rightCol] = leftCol
				}
			}
		}
		previousRight = nextRight
	}

	beforeLeft := make(map[int][]int)
	tailRight := make([]int, 0)
	for rightCol := 1; rightCol <= right.MaxCol; rightCol++ {
		if rightToLeft[rightCol] != 0 {
			continue
		}
		nextLeft := 0
		for candidate := rightCol + 1; candidate <= right.MaxCol; candidate++ {
			if rightToLeft[candidate] != 0 {
				nextLeft = rightToLeft[candidate]
				break
			}
		}
		if nextLeft > 0 {
			beforeLeft[nextLeft] = append(beforeLeft[nextLeft], rightCol)
		} else {
			tailRight = append(tailRight, rightCol)
		}
	}

	leftMapping := make(map[int]int, left.MaxCol)
	rightMapping := make(map[int]int, right.MaxCol)
	logical := 0
	for leftCol := 1; leftCol <= left.MaxCol; leftCol++ {
		for _, rightCol := range beforeLeft[leftCol] {
			logical++
			rightMapping[rightCol] = logical
		}
		logical++
		leftMapping[leftCol] = logical
		if rightCol := leftToRight[leftCol]; rightCol > 0 {
			rightMapping[rightCol] = logical
		}
	}
	for _, rightCol := range tailRight {
		logical++
		rightMapping[rightCol] = logical
	}
	applied := !sameColumnMapping(leftMapping) || !sameColumnMapping(rightMapping)
	return sheetColumnAlignment{
		leftPhysicalToLogical:  leftMapping,
		rightPhysicalToLogical: rightMapping,
		applied:                applied,
	}
}

func headerRowIsTextual(sheet *workbook.SheetSnapshot) bool {
	for col := 1; col <= sheet.MaxCol; col++ {
		value := sheet.Cell(1, col)
		if value.Present && strings.TrimSpace(cellText(value)) != "" && value.Type != "string" {
			return false
		}
	}
	return true
}

func nonEmptyHeaderCount(headers map[int]string) int {
	count := 0
	for _, header := range headers {
		if header != "" {
			count++
		}
	}
	return count
}

func columnHeaders(sheet *workbook.SheetSnapshot) (map[int]string, map[string]int) {
	headers := make(map[int]string, sheet.MaxCol)
	counts := make(map[string]int)
	for col := 1; col <= sheet.MaxCol; col++ {
		header := strings.TrimSpace(cellText(sheet.Cell(1, col)))
		headers[col] = header
		if header != "" {
			counts[header]++
		}
	}
	return headers, counts
}

func uniqueHeaderColumns(headers map[int]string, counts map[string]int) map[string]int {
	result := make(map[string]int)
	for col, header := range headers {
		if header != "" && counts[header] == 1 {
			result[header] = col
		}
	}
	return result
}

func identitySheetColumns(sheet *workbook.SheetSnapshot) map[int]int {
	result := make(map[int]int)
	if sheet != nil {
		for col := 1; col <= sheet.MaxCol; col++ {
			result[col] = col
		}
	}
	return result
}

func identityWorkbookColumns(snapshot *workbook.WorkbookSnapshot) map[string]map[int]int {
	result := make(map[string]map[int]int)
	if snapshot == nil {
		return result
	}
	for _, sheet := range snapshot.Sheets {
		result[sheet.Name] = invertColumnMapping(identitySheetColumns(sheet))
	}
	return result
}

func invertColumnMapping(mapping map[int]int) map[int]int {
	result := make(map[int]int, len(mapping))
	for physical, logical := range mapping {
		result[logical] = physical
	}
	return result
}

func sameColumnMapping(mapping map[int]int) bool {
	for physical, logical := range mapping {
		if physical != logical {
			return false
		}
	}
	return true
}

func alignSheetColumnsSnapshot(source *workbook.SheetSnapshot, physicalToLogical map[int]int) *workbook.SheetSnapshot {
	if source == nil {
		return nil
	}
	copySheet := &workbook.SheetSnapshot{
		Name: source.Name, Index: source.Index, MaxRow: source.MaxRow,
		Cells:    make(map[workbook.CellKey]workbook.CellValue, len(source.Cells)),
		CellList: make([]workbook.CellKey, 0, len(source.CellList)),
	}
	for _, key := range source.CellList {
		logicalCol := physicalToLogical[key.Col]
		if logicalCol == 0 {
			continue
		}
		logicalKey := workbook.CellKey{Row: key.Row, Col: logicalCol}
		copySheet.Cells[logicalKey] = source.Cells[key]
		copySheet.CellList = append(copySheet.CellList, logicalKey)
		copySheet.MaxCol = max(copySheet.MaxCol, logicalCol)
	}
	for _, logicalCol := range physicalToLogical {
		copySheet.MaxCol = max(copySheet.MaxCol, logicalCol)
	}
	sort.Slice(copySheet.CellList, func(i, j int) bool {
		leftKey, rightKey := copySheet.CellList[i], copySheet.CellList[j]
		return leftKey.Row < rightKey.Row || (leftKey.Row == rightKey.Row && leftKey.Col < rightKey.Col)
	})
	return copySheet
}
