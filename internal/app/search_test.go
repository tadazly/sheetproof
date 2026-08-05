package app

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tadazly/sheetproof/internal/diff"
	"github.com/tadazly/sheetproof/internal/workbook"
)

func TestSessionSearchSidesOptionsFormulaAndCurrentSheet(t *testing.T) {
	left := searchWorkbookSnapshot(map[string][]workbook.CellValue{
		"First": {
			{Present: true, Raw: "Alpha alpha", Type: "string"},
			{Present: true, Raw: "猫 dog", Type: "string"},
			{Present: true, Raw: "猫咪", Type: "string"},
			{Present: true, Formula: "SUM(1,2)", Type: "formula"},
			{Present: true, Raw: "", Type: "string"},
		},
		"Second": {{Present: true, Raw: "alpha", Type: "string"}},
	})
	right := searchWorkbookSnapshot(map[string][]workbook.CellValue{
		"First":  {{Present: true, Raw: "right target", Type: "string"}},
		"Second": {{Present: true, Raw: "right target", Type: "string"}},
	})
	session := searchSnapshotSession(left, right)

	leftResult := mustSearch(t, session, "First", SearchLeft, "alpha", false, false, false, nil)
	if leftResult.Count != 1 || leftResult.CurrentIndex != 1 {
		t.Fatalf("left result = %+v", leftResult)
	}
	rightResult := mustSearch(t, session, "First", SearchRight, "target", false, false, false, nil)
	if rightResult.Count != 1 {
		t.Fatalf("right result = %+v", rightResult)
	}
	leftAfterRight, err := session.NavigateSearch(SearchLeft, 1)
	if err != nil || leftAfterRight.Count != 1 || leftAfterRight.CurrentIndex != 1 {
		t.Fatalf("right search changed left cache: %+v / %v", leftAfterRight, err)
	}

	if result := mustSearch(t, session, "First", SearchLeft, "ALPHA", true, false, false, nil); result.Count != 0 {
		t.Fatalf("case-sensitive result = %+v", result)
	}
	if result := mustSearch(t, session, "First", SearchLeft, "猫", true, true, false, nil); result.Count != 1 {
		t.Fatalf("unicode whole-word result = %+v", result)
	}
	anchored, err := session.Search(
		"First", SearchLeft, "猫", true, false, false, nil,
		CellCoordinate{Row: 3, Col: 1},
	)
	if err != nil || anchored.Count != 2 || anchored.CurrentRef == nil || anchored.CurrentRef.Row != 3 {
		t.Fatalf("anchored initial result = %+v / %v", anchored, err)
	}
	if result := mustSearch(t, session, "First", SearchLeft, `^猫.*dog$`, true, false, true, nil); result.Count != 1 {
		t.Fatalf("regexp result = %+v", result)
	}
	if result := mustSearch(t, session, "First", SearchLeft, `=SUM\(1,2\)`, true, false, true, nil); result.Count != 1 {
		t.Fatalf("formula result = %+v", result)
	}
	if result := mustSearch(t, session, "First", SearchLeft, `^$`, true, false, true, nil); result.Count != 1 {
		t.Fatalf("explicit empty result = %+v", result)
	}
	if result := mustSearch(t, session, "First", SearchLeft, "", false, false, false, nil); result.Count != 0 {
		t.Fatalf("empty query result = %+v", result)
	}
	if result := mustSearch(t, session, "First", SearchLeft, "alpha", false, false, false, nil); result.Count != 1 {
		t.Fatalf("first-sheet result = %+v", result)
	}
	if result := mustSearch(t, session, "Second", SearchLeft, "alpha", false, false, false, nil); result.Count != 1 {
		t.Fatalf("second-sheet result = %+v", result)
	}
}

func TestSessionSearchInvalidRegexDoesNotInstallResults(t *testing.T) {
	book := searchWorkbookSnapshot(map[string][]workbook.CellValue{
		"Sheet1": {{Present: true, Raw: "kept", Type: "string"}},
	})
	session := searchSnapshotSession(book, book)
	if result := mustSearch(t, session, "Sheet1", SearchLeft, "kept", true, false, false, nil); result.Count != 1 {
		t.Fatal(result)
	}
	invalid := mustSearch(t, session, "Sheet1", SearchLeft, "[", true, false, true, nil)
	if invalid.Error == "" || invalid.Count != 0 {
		t.Fatalf("invalid regexp result = %+v", invalid)
	}
	kept, err := session.NavigateSearch(SearchLeft, 1)
	if err != nil || kept.Count != 1 {
		t.Fatalf("previous result was replaced: %+v / %v", kept, err)
	}
}

func TestSessionSearchUsesLogicalAlignmentAndFilteredRows(t *testing.T) {
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.xlsx")
	rightPath := filepath.Join(dir, "right.xlsx")
	writeRowsWorkbook(t, leftPath, [][]any{
		{"id", "value"},
		{1, "common one"},
		{2, "left deleted needle"},
		{"", "ambiguous helper"},
		{3, "common three"},
	})
	writeRowsWorkbook(t, rightPath, [][]any{
		{"id", "value"},
		{1, "common one"},
		{99, "right added needle"},
		{"", "ambiguous helper"},
		{3, "common three changed"},
	})
	session, err := Open(leftPath, rightPath, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	added := mustSearch(t, session, "Sheet1", SearchRight, "right added", false, false, false, nil)
	if added.Count != 1 || added.CurrentRef == nil || added.CurrentRef.Row != 4 || added.CurrentRef.SourceRow != 4 {
		t.Fatalf("aligned right result = %+v ref=%+v", added, added.CurrentRef)
	}
	if missing := mustSearch(t, session, "Sheet1", SearchLeft, "right added", false, false, false, nil); missing.Count != 0 {
		t.Fatalf("missing-side cell matched = %+v", missing)
	}
	filteredAdded := mustSearch(t, session, "Sheet1", SearchRight, "needle", false, false, false, []diff.RowStatus{diff.RowAdded})
	if filteredAdded.Count != 1 || filteredAdded.CurrentRef == nil || filteredAdded.CurrentRef.Row != 1 || filteredAdded.CurrentRef.SourceRow != 4 {
		t.Fatalf("filtered added result = %+v", filteredAdded)
	}
	filteredDeleted := mustSearch(t, session, "Sheet1", SearchLeft, "needle", false, false, false, []diff.RowStatus{diff.RowDeleted})
	if filteredDeleted.Count != 1 || filteredDeleted.CurrentRef == nil || filteredDeleted.CurrentRef.Row != 1 {
		t.Fatalf("filtered deleted result = %+v", filteredDeleted)
	}
	multiple := mustSearch(t, session, "Sheet1", SearchLeft, "needle", false, false, false, []diff.RowStatus{diff.RowDeleted, diff.RowModified})
	if multiple.Count != 1 || multiple.CurrentRef == nil || multiple.CurrentRef.SourceRow != 3 {
		t.Fatalf("multi-filter result = %+v", multiple)
	}
	region, err := session.FilteredRegion("Sheet1", []diff.RowStatus{diff.RowAdded}, 1, 48, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	foundMarker := false
	for _, cell := range region.Cells {
		if cell.SourceRow == 4 && cell.Col == 2 && cell.RightMatch {
			foundMarker = true
		}
	}
	if !foundMarker {
		t.Fatal("filtered Region did not include the right-side match marker")
	}
}

func TestSessionSearchCacheInvalidatesAfterMutationsReloadAndAlignment(t *testing.T) {
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.xlsx")
	rightPath := filepath.Join(dir, "right.xlsx")
	replacementPath := filepath.Join(dir, "replacement.xlsx")
	writeRowsWorkbook(t, leftPath, [][]any{{"id", "value"}, {1, "left old"}, {2, "same"}})
	writeRowsWorkbook(t, rightPath, [][]any{{"id", "value"}, {1, "right new"}, {2, "same"}})
	writeRowsWorkbook(t, replacementPath, [][]any{{"id", "value"}, {1, "replacement"}, {2, "same"}})
	session, err := Open(leftPath, rightPath, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	mustSearch(t, session, "Sheet1", SearchLeft, "same", false, false, false, nil)
	if err := session.ClearLeftSelection("Sheet1", 3, 3, 2, 2, nil); err != nil {
		t.Fatal(err)
	}
	assertSearchCacheStale(t, session, SearchLeft)
	if err := session.Undo(); err != nil {
		t.Fatal(err)
	}
	assertSearchCacheStale(t, session, SearchLeft)

	mustSearch(t, session, "Sheet1", SearchLeft, "left", false, false, false, nil)
	if err := session.EditLeft(workbook.CellRef{Sheet: "Sheet1", Row: 2, Col: 2}, "edited", "text"); err != nil {
		t.Fatal(err)
	}
	assertSearchCacheStale(t, session, SearchLeft)
	mustSearch(t, session, "Sheet1", SearchLeft, "edited", false, false, false, nil)
	if err := session.Undo(); err != nil {
		t.Fatal(err)
	}
	assertSearchCacheStale(t, session, SearchLeft)

	mustSearch(t, session, "Sheet1", SearchLeft, "left", false, false, false, nil)
	if err := session.CopyRightToLeft(workbook.CellRef{Sheet: "Sheet1", Row: 2, Col: 2}); err != nil {
		t.Fatal(err)
	}
	assertSearchCacheStale(t, session, SearchLeft)

	mustSearch(t, session, "Sheet1", SearchRight, "right", false, false, false, nil)
	if err := session.ReplaceRight(replacementPath, "replacement"); err != nil {
		t.Fatal(err)
	}
	assertSearchCacheStale(t, session, SearchRight)

	mustSearch(t, session, "Sheet1", SearchLeft, "right", false, false, false, nil)
	setWorkbookCell(t, leftPath, "Sheet1", "B2", "reloaded")
	if err := session.ReloadLeft(); err != nil {
		t.Fatal(err)
	}
	assertSearchCacheStale(t, session, SearchLeft)

	mustSearch(t, session, "Sheet1", SearchLeft, "reloaded", false, false, false, nil)
	if err := session.SetRowAlignment(RowAlignmentPosition); err != nil {
		t.Fatal(err)
	}
	assertSearchCacheStale(t, session, SearchLeft)

	mustSearch(t, session, "Sheet1", SearchRight, "replacement", false, false, false, nil)
	if err := session.DetachRight("missing"); err != nil {
		t.Fatal(err)
	}
	assertSearchCacheStale(t, session, SearchRight)

	keySession, err := Open(leftPath, replacementPath, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer keySession.Close()
	mustSearch(t, keySession, "Sheet1", SearchLeft, "reloaded", false, false, false, nil)
	if err := keySession.SetKeyColumn("Sheet1", 0); err != nil {
		t.Fatal(err)
	}
	assertSearchCacheStale(t, keySession, SearchLeft)
}

func TestSessionSearchReturnsMoreThanTenThousandMatches(t *testing.T) {
	values := make([]workbook.CellValue, 10001)
	for index := range values {
		values[index] = workbook.CellValue{Present: true, Raw: "needle", Type: "string"}
	}
	book := searchWorkbookSnapshot(map[string][]workbook.CellValue{"Sheet1": values})
	session := searchSnapshotSession(book, book)
	result := mustSearch(t, session, "Sheet1", SearchLeft, "needle", false, false, false, nil)
	if result.Count != 10001 {
		t.Fatalf("count = %d", result.Count)
	}
	previous, err := session.NavigateSearch(SearchLeft, -1)
	if err != nil || previous.CurrentIndex != 10001 {
		t.Fatalf("wrapped previous = %+v / %v", previous, err)
	}
}

func TestSessionConcurrentSearchAndRegion(t *testing.T) {
	values := make([]workbook.CellValue, 2000)
	for index := range values {
		values[index] = workbook.CellValue{Present: true, Raw: fmt.Sprintf("value %d", index), Type: "string"}
	}
	book := searchWorkbookSnapshot(map[string][]workbook.CellValue{"Sheet1": values})
	session := searchSnapshotSession(book, book)
	done := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		for index := 0; index < 100; index++ {
			_, _ = session.Search("Sheet1", SearchLeft, "value", false, false, false, nil, CellCoordinate{Row: 1, Col: 1})
		}
	}()
	go func() {
		defer wait.Done()
		for index := 0; index < 100; index++ {
			_, _ = session.Region("Sheet1", 1, 48, 1, 20)
		}
	}()
	go func() {
		wait.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent search and Region timed out")
	}
}

func BenchmarkSessionSearchSparse100K(b *testing.B) {
	sheet := &workbook.SheetSnapshot{
		Name: "Sheet1", Cells: make(map[workbook.CellKey]workbook.CellValue, 100000),
		CellList: make([]workbook.CellKey, 0, 100000),
	}
	for index := 0; index < 100000; index++ {
		key := workbook.CellKey{Row: index*10 + 1, Col: index%1000 + 1}
		sheet.Cells[key] = workbook.CellValue{Present: true, Raw: fmt.Sprintf("配置 value %06d", index), Type: "string"}
		sheet.CellList = append(sheet.CellList, key)
		sheet.MaxRow = key.Row
		if key.Col > sheet.MaxCol {
			sheet.MaxCol = key.Col
		}
	}
	book := &workbook.WorkbookSnapshot{Sheets: []*workbook.SheetSnapshot{sheet}, ByName: map[string]*workbook.SheetSnapshot{"Sheet1": sheet}}
	for _, benchmark := range []struct {
		name          string
		query         string
		caseSensitive bool
		useRegex      bool
	}{
		{name: "plain", query: "value 099999"},
		{name: "case-sensitive", query: "配置 value 099999", caseSensitive: true},
		{name: "regexp", query: `value\s+09[0-9]{4}`, useRegex: true},
	} {
		b.Run(benchmark.name, func(b *testing.B) {
			session := searchSnapshotSession(book, book)
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				session.ClearSearch(SearchLeft)
				_, err := session.Search("Sheet1", SearchLeft, benchmark.query, benchmark.caseSensitive, false, benchmark.useRegex, nil, CellCoordinate{Row: 1, Col: 1})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func mustSearch(
	t testing.TB,
	session *Session,
	sheet string,
	side SearchSide,
	query string,
	caseSensitive, wholeWord, useRegex bool,
	statuses []diff.RowStatus,
) SearchSummary {
	t.Helper()
	result, err := session.Search(
		sheet, side, query, caseSensitive, wholeWord, useRegex, statuses,
		CellCoordinate{Row: 1, Col: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertSearchCacheStale(t *testing.T, session *Session, side SearchSide) {
	t.Helper()
	result, err := session.NavigateSearch(side, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 0 || result.CurrentRef != nil {
		t.Fatalf("stale cache remained active: %+v", result)
	}
}

func searchSnapshotSession(left, right *workbook.WorkbookSnapshot) *Session {
	return &Session{
		left: left, comparisonLeft: left, originalLeft: left,
		right: right, rightSource: right,
		leftRows: identityWorkbookRows(left), rightRows: identityWorkbookRows(right),
		dataGeneration: 1, searches: make(map[SearchSide]*searchCache),
	}
}

func searchWorkbookSnapshot(sheets map[string][]workbook.CellValue) *workbook.WorkbookSnapshot {
	book := &workbook.WorkbookSnapshot{ByName: make(map[string]*workbook.SheetSnapshot)}
	index := 0
	for name, values := range sheets {
		sheet := &workbook.SheetSnapshot{
			Name: name, Index: index, MaxCol: 1,
			Cells:    make(map[workbook.CellKey]workbook.CellValue, len(values)),
			CellList: make([]workbook.CellKey, 0, len(values)),
		}
		for row, value := range values {
			key := workbook.CellKey{Row: row + 1, Col: 1}
			sheet.Cells[key] = value
			sheet.CellList = append(sheet.CellList, key)
			sheet.MaxRow = row + 1
		}
		book.Sheets = append(book.Sheets, sheet)
		book.ByName[name] = sheet
		index++
	}
	return book
}
