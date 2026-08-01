package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ug-tools/ugxlsx/internal/diff"
	"github.com/ug-tools/ugxlsx/internal/testutil"
	"github.com/ug-tools/ugxlsx/internal/workbook"
	"github.com/xuri/excelize/v2"
)

func TestSessionEndToEndMergeEditUndoSaveAndReopen(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session, err := Open(pair.Left, pair.Right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	before := session.Summary().Diff.DifferenceCount
	if session.Summary().Warnings == nil {
		t.Fatal("warnings must serialize as an empty array, not null")
	}
	if session.Summary().Resolutions == nil {
		t.Fatal("resolutions must serialize as an empty array, not null")
	}
	if before < 7 {
		t.Fatalf("expected several fixture differences, got %d", before)
	}
	for _, ref := range []workbook.CellRef{
		{Sheet: "数据 表", Row: 1, Col: 1},
		{Sheet: "数据 表", Row: 1, Col: 2},
		{Sheet: "数据 表", Row: 1, Col: 4},
		{Sheet: "数据 表", Row: 1, Col: 7},
		{Sheet: "数据 表", Row: 2, Col: 1},
	} {
		if err := session.CopyRightToLeft(ref); err != nil {
			t.Fatalf("copy %s: %v", ref.Axis(), err)
		}
	}
	clearRef := workbook.CellRef{Sheet: "数据 表", Row: 1, Col: 8}
	if err := session.EditLeft(clearRef, "", "clear"); err != nil {
		t.Fatal(err)
	}
	if err := session.Undo(); err != nil {
		t.Fatal(err)
	}
	if err := session.EditLeft(workbook.CellRef{Sheet: "数据 表", Row: 3, Col: 1}, "123.5", "number"); err != nil {
		t.Fatal(err)
	}
	if err := session.Save(""); err != nil {
		t.Fatal(err)
	}
	if session.Dirty() {
		t.Fatal("session remains dirty after save")
	}

	saved, err := excelize.OpenFile(pair.Left)
	if err != nil {
		t.Fatalf("saved workbook cannot be reopened: %v", err)
	}
	defer saved.Close()
	assertCell(t, saved, "数据 表", "A1", "新文本")
	assertCell(t, saved, "数据 表", "B1", "42")
	formula, err := saved.GetCellFormula("数据 表", "D1")
	if err != nil || formula != "SUM(B1,2)" {
		t.Fatalf("formula = %q, err=%v", formula, err)
	}
	assertCell(t, saved, "数据 表", "H1", "保留")
	assertCell(t, saved, "数据 表", "A3", "123.5")
	merges, err := saved.GetMergeCells("数据 表")
	if err != nil || len(merges) != 1 || merges[0].GetStartAxis() != "J1" || merges[0].GetEndAxis() != "K1" {
		t.Fatalf("merge cells not preserved: %+v, %v", merges, err)
	}
	width, err := saved.GetColWidth("数据 表", "A")
	if err != nil || width != 24 {
		t.Fatalf("column width = %v, err=%v", width, err)
	}
	height, err := saved.GetRowHeight("数据 表", 1)
	if err != nil || height != 28 {
		t.Fatalf("row height = %v, err=%v", height, err)
	}
	hasLink, target, err := saved.GetCellHyperLink("数据 表", "A2")
	if err != nil || !hasLink || target != "https://example.org/right" {
		t.Fatalf("hyperlink = %t %q, err=%v", hasLink, target, err)
	}
	comments, err := saved.GetComments("数据 表")
	if err != nil || len(comments) != 1 || comments[0].Text != "右侧批注" {
		t.Fatalf("comment not preserved: %+v, %v", comments, err)
	}
	styleID, err := saved.GetCellStyle("数据 表", "A1")
	if err != nil || styleID == 0 {
		t.Fatalf("style not preserved: id=%d err=%v", styleID, err)
	}
	after := session.Summary().Diff.DifferenceCount
	if after >= before {
		t.Fatalf("differences did not decrease: before=%d after=%d", before, after)
	}
}

func TestSessionBatchCopyIsOneUndoCommand(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session, err := Open(pair.Left, pair.Right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	before, err := session.Region("数据 表", 1, 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.CopyRightToLeftMany("数据 表", []CellCoordinate{
		{Row: 1, Col: 1},
		{Row: 1, Col: 2},
	}); err != nil {
		t.Fatal(err)
	}
	if session.Summary().UndoCount != 1 {
		t.Fatalf("batch undo count = %d, want 1", session.Summary().UndoCount)
	}
	copied, err := session.Region("数据 表", 1, 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if copied.Cells[0].Left.Raw != copied.Cells[0].Right.Raw || copied.Cells[1].Left.Raw != copied.Cells[1].Right.Raw {
		t.Fatalf("batch copy did not apply: %+v", copied.Cells)
	}
	if err := session.Save(""); err != nil {
		t.Fatal(err)
	}
	if session.Dirty() {
		t.Fatal("session is dirty immediately after save")
	}
	if session.Summary().UndoCount != 1 {
		t.Fatalf("save discarded undo history: count = %d", session.Summary().UndoCount)
	}
	if err := session.Undo(); err != nil {
		t.Fatal(err)
	}
	if !session.Dirty() {
		t.Fatal("undo after save must mark the reverted in-memory workbook dirty")
	}
	restored, err := session.Region("数据 表", 1, 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	for index := range before.Cells {
		if restored.Cells[index].Left.Raw != before.Cells[index].Left.Raw {
			t.Fatalf("cell %d after undo = %q, want %q", index, restored.Cells[index].Left.Raw, before.Cells[index].Left.Raw)
		}
	}
	if err := session.Save(""); err != nil {
		t.Fatal(err)
	}
	reopened, err := excelize.OpenFile(pair.Left)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for index, axis := range []string{"A1", "B1"} {
		got, err := reopened.GetCellValue("数据 表", axis, excelize.Options{RawCellValue: true})
		if err != nil || got != before.Cells[index].Left.Raw {
			t.Fatalf("%s after undo and resave = %q, want %q (err=%v)", axis, got, before.Cells[index].Left.Raw, err)
		}
	}
}

func TestSessionRowCopyKeepsStyleOnlyBlankCellsUnchanged(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "blank-left.xlsx")
	right := filepath.Join(dir, "blank-right.xlsx")
	for path, rightSide := range map[string]bool{left: false, right: true} {
		file := excelize.NewFile()
		if err := file.SetCellValue("Sheet1", "A1", "name"); err != nil {
			t.Fatal(err)
		}
		if err := file.SetCellValue("Sheet1", "C1", "limit"); err != nil {
			t.Fatal(err)
		}
		value := "old"
		if rightSide {
			value = "new"
		}
		if err := file.SetCellValue("Sheet1", "A2", value); err != nil {
			t.Fatal(err)
		}
		if rightSide {
			styleID, err := file.NewStyle(&excelize.Style{
				Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F3F4F6"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := file.SetCellStyle("Sheet1", "B2", "B2", styleID); err != nil {
				t.Fatal(err)
			}
		}
		if err := file.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}

	session, err := Open(left, right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if got := session.Summary().Diff.DifferenceCount; got != 1 {
		t.Fatalf("style-only blank produced an initial difference count of %d, want 1", got)
	}
	if err := session.CopyRowsRightToLeft("Sheet1", []int{2}); err != nil {
		t.Fatal(err)
	}
	if got := session.Summary().Diff.DifferenceCount; got != 0 {
		t.Fatalf("blank cells retained differences after row copy: %d", got)
	}
	region, err := session.Region("Sheet1", 2, 1, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, cell := range region.Cells {
		if cell.Status != diff.Unchanged {
			t.Fatalf("%s retained status %s after row copy: %+v", cell.Axis, cell.Status, cell)
		}
		if cell.Col > 1 && (cell.Left.Present || cell.Right.Present) {
			t.Fatalf("%s should remain a true blank on both sides: %+v", cell.Axis, cell)
		}
	}
}

func TestSessionExportKeepsOriginalTargetAndDirtyState(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session, err := Open(pair.Left, pair.Right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.EditLeft(
		workbook.CellRef{Sheet: "数据 表", Row: 1, Col: 1},
		"导出副本",
		"text",
	); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "exported.xlsx")
	if err := session.Export(target); err != nil {
		t.Fatal(err)
	}
	if !session.Dirty() {
		t.Fatal("export incorrectly marked the worktree session as saved")
	}
	if got := session.Summary().Diff.LeftFile; filepath.Clean(got) != filepath.Clean(pair.Left) {
		t.Fatalf("export changed the session target to %q", got)
	}
	exported, err := excelize.OpenFile(target)
	if err != nil {
		t.Fatal(err)
	}
	defer exported.Close()
	if value, err := exported.GetCellValue("数据 表", "A1"); err != nil || value != "导出副本" {
		t.Fatalf("exported A1 = %q, err=%v", value, err)
	}
	original, err := excelize.OpenFile(pair.Left)
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	if value, err := original.GetCellValue("数据 表", "A1"); err != nil || value == "导出副本" {
		t.Fatalf("export modified the original workbook: value=%q err=%v", value, err)
	}
}

func TestSessionCopiesAndAppendsConflictRowsWithUndo(t *testing.T) {
	left, right := createIDWorkbookPair(t)
	session, err := Open(left, right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	sheet := session.Summary().Diff.Sheets[0]
	if sheet.ConflictRowCount != 1 || sheet.DeletedRowCount != 1 || sheet.NextID != 3 {
		t.Fatalf("initial row summary = %+v", sheet)
	}

	if err := session.CopyRowsRightToLeft("配置", []int{2}); err != nil {
		t.Fatal(err)
	}
	region, err := session.Region("配置", 2, 1, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if region.Cells[1].Left.Raw != "right-a" || region.Cells[2].Left.Raw != "right-b" {
		t.Fatalf("copied row = %+v", region.Cells)
	}
	overwritten := session.Summary()
	if overwritten.Diff.Sheets[0].ConflictRowCount != 0 {
		t.Fatalf("conflict remained after overwrite: %+v", overwritten.Diff.Sheets[0])
	}
	for _, cell := range region.Cells {
		if cell.Status != diff.Unchanged {
			t.Fatalf("equal copied cell retained status %s: %+v", cell.Status, region.Cells)
		}
	}
	if len(overwritten.Resolutions) != 1 ||
		overwritten.Resolutions[0].Kind != ResolutionOverwriteRow ||
		overwritten.Resolutions[0].SourceRow != 2 {
		t.Fatalf("overwrite resolution = %+v", overwritten.Resolutions)
	}
	if err := session.Undo(); err != nil {
		t.Fatal(err)
	}
	undoneOverwrite := session.Summary()
	if undoneOverwrite.Diff.Sheets[0].ConflictRowCount != 1 {
		t.Fatal("undo did not restore conflict row")
	}
	if len(undoneOverwrite.Resolutions) != 0 {
		t.Fatalf("undo retained overwrite resolution: %+v", undoneOverwrite.Resolutions)
	}

	assigned, err := session.AppendRowsRightToLeft("配置", []int{2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(assigned) != 1 || assigned[0] != "3" {
		t.Fatalf("automatic IDs = %#v", assigned)
	}
	appended, err := session.Region("配置", 4, 1, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if appended.Cells[0].Left.Raw != "3" ||
		appended.Cells[1].Left.Raw != "right-a" ||
		appended.Cells[2].Left.Raw != "right-b" {
		t.Fatalf("appended row = %+v", appended.Cells)
	}
	appendSummary := session.Summary()
	if len(appendSummary.Resolutions) != 1 ||
		appendSummary.Resolutions[0].Kind != ResolutionAppendAuto ||
		appendSummary.Resolutions[0].SourceRow != 2 ||
		appendSummary.Resolutions[0].TargetRow != 4 ||
		appendSummary.Resolutions[0].TargetID != "3" {
		t.Fatalf("automatic append resolution = %+v", appendSummary.Resolutions)
	}
	if err := session.Undo(); err != nil {
		t.Fatal(err)
	}
	undoneAppend := session.Summary()
	if undoneAppend.Diff.Sheets[0].NextID != 3 {
		t.Fatalf("next ID after undo = %d", undoneAppend.Diff.Sheets[0].NextID)
	}
	if len(undoneAppend.Resolutions) != 0 {
		t.Fatalf("undo retained append resolution: %+v", undoneAppend.Resolutions)
	}

	assigned, err = session.AppendRowsRightToLeft("配置", []int{2}, []string{"10"})
	if err != nil {
		t.Fatal(err)
	}
	if len(assigned) != 1 || assigned[0] != "10" {
		t.Fatalf("specified IDs = %#v", assigned)
	}
	specified := session.Summary().Resolutions
	if len(specified) != 1 || specified[0].Kind != ResolutionAppendSpecified ||
		specified[0].TargetID != "10" {
		t.Fatalf("specified append resolution = %+v", specified)
	}
	appended, err = session.Region("配置", 4, 1, 1, 1)
	if err != nil || appended.Cells[0].Left.Raw != "10" {
		t.Fatalf("specified ID row = %+v, err=%v", appended.Cells, err)
	}
	if _, err := session.AppendRowsRightToLeft("配置", []int{2}, []string{"10"}); err == nil {
		t.Fatal("duplicate specified ID was accepted")
	}
	assigned, err = session.AppendRowsRightToLeft("配置", []int{2}, []string{"custom-id"})
	if err != nil || len(assigned) != 1 || assigned[0] != "custom-id" {
		t.Fatalf("text specified ID = %#v, err=%v", assigned, err)
	}
	appended, err = session.Region("配置", 5, 1, 1, 1)
	if err != nil || appended.Cells[0].Left.Raw != "custom-id" ||
		appended.Cells[0].Left.Type != "string" {
		t.Fatalf("text specified ID cell = %+v, err=%v", appended.Cells, err)
	}
}

func TestGitMergeAlignsShiftedUniqueIDsAndExplainsFileConflict(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.xlsx")
	right := filepath.Join(dir, "right.xlsx")
	left := filepath.Join(dir, "left.xlsx")
	write := func(path string, rows [][]any) {
		t.Helper()
		file := excelize.NewFile()
		for rowIndex, row := range rows {
			for colIndex, value := range row {
				axis, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
				if err := file.SetCellValue("Sheet1", axis, value); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := file.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	baseRows := [][]any{{"id", "name", "type"}, {1, "one", "common"}, {2, "two", "common"}}
	write(base, baseRows)
	write(right, baseRows)
	write(left, [][]any{{"id", "name", "type"}, {"", "left-only", ""}, {1, "one", "common"}, {2, "left-two", "common"}})

	session, err := Open(left, right, Options{GitMerge: true, MergeBase: base})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	summary := session.Summary()
	sheet := summary.Diff.Sheets[0]
	if sheet.DifferenceCount != 3 || sheet.DeletedRowCount != 1 ||
		sheet.ModifiedRowCount != 1 || sheet.ConflictRowCount != 0 {
		t.Fatalf("aligned summary = %+v", sheet)
	}
	if len(summary.Warnings) < 1 || !strings.Contains(summary.MergeNotice, "右侧与共同基线语义一致") {
		t.Fatalf("merge warnings = %#v, notice = %q", summary.Warnings, summary.MergeNotice)
	}
	region, err := session.Region("Sheet1", 2, 3, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if region.Cells[3].Right.Raw != "1" || region.Cells[6].Right.Raw != "2" {
		t.Fatalf("right rows were not aligned to left IDs: %+v", region.Cells)
	}
	if err := session.CopyRightToLeft(workbook.CellRef{Sheet: "Sheet1", Row: 2, Col: 2}); err != nil {
		t.Fatal(err)
	}
	cleared, err := session.Region("Sheet1", 2, 1, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Cells[0].Left.Present || cleared.Cells[0].Left.Raw != "" {
		t.Fatalf("copying an aligned empty cell read an unrelated physical row: %+v", cleared.Cells[0])
	}
	if err := session.CopyRightToLeft(workbook.CellRef{Sheet: "Sheet1", Row: 4, Col: 2}); err != nil {
		t.Fatal(err)
	}
	after, err := session.Region("Sheet1", 4, 1, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if after.Cells[1].Left.Raw != "two" || after.Cells[1].Status != diff.Unchanged {
		t.Fatalf("aligned copy used the wrong physical source row: %+v", after.Cells)
	}
}

func TestSessionConflictCellOverwriteClearsOnlyTheCopiedCell(t *testing.T) {
	left, right := createIDWorkbookPair(t)
	session, err := Open(left, right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	if err := session.CopyRightToLeftMany("配置", []CellCoordinate{{Row: 2, Col: 2}}); err != nil {
		t.Fatal(err)
	}
	region, err := session.Region("配置", 2, 1, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if region.Cells[0].Status != diff.Unchanged ||
		region.Cells[0].Left.Raw != region.Cells[0].Right.Raw {
		t.Fatalf("copied cell still differs: %+v", region.Cells[0])
	}
	if region.Cells[1].Status != diff.Modified || region.Cells[1].RowStatus != diff.RowModified {
		t.Fatalf("remaining cell/row status = %+v", region.Cells[1])
	}
	resolutions := session.Summary().Resolutions
	if len(resolutions) != 1 ||
		resolutions[0].Kind != ResolutionOverwriteCells ||
		resolutions[0].CellCount != 1 {
		t.Fatalf("cell overwrite resolution = %+v", resolutions)
	}
}

func createIDWorkbookPair(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	left := filepath.Join(dir, "id-left.xlsx")
	right := filepath.Join(dir, "id-right.xlsx")
	for path, rightSide := range map[string]bool{left: false, right: true} {
		file := excelize.NewFile()
		if err := file.SetSheetName("Sheet1", "配置"); err != nil {
			t.Fatal(err)
		}
		for axis, value := range map[string]any{
			"A1": "id", "B1": "name", "C1": "value",
			"A2": 1,
		} {
			if err := file.SetCellValue("配置", axis, value); err != nil {
				t.Fatal(err)
			}
		}
		if rightSide {
			if err := file.SetCellValue("配置", "B2", "right-a"); err != nil {
				t.Fatal(err)
			}
			if err := file.SetCellValue("配置", "C2", "right-b"); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := file.SetCellValue("配置", "B2", "left-a"); err != nil {
				t.Fatal(err)
			}
			if err := file.SetCellValue("配置", "C2", "left-b"); err != nil {
				t.Fatal(err)
			}
			if err := file.SetCellValue("配置", "A3", 2); err != nil {
				t.Fatal(err)
			}
			if err := file.SetCellValue("配置", "B3", "deleted"); err != nil {
				t.Fatal(err)
			}
		}
		if err := file.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return left, right
}

func TestSessionUndoStepsThroughSeparateCopies(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session, err := Open(pair.Left, pair.Right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	before, err := session.Region("数据 表", 1, 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.CopyRightToLeft(workbook.CellRef{Sheet: "数据 表", Row: 1, Col: 1}); err != nil {
		t.Fatal(err)
	}
	if err := session.CopyRightToLeft(workbook.CellRef{Sheet: "数据 表", Row: 1, Col: 2}); err != nil {
		t.Fatal(err)
	}
	if session.Summary().UndoCount != 2 {
		t.Fatalf("undo count = %d, want 2", session.Summary().UndoCount)
	}
	if err := session.Undo(); err != nil {
		t.Fatal(err)
	}
	afterFirstUndo, err := session.Region("数据 表", 1, 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if afterFirstUndo.Cells[0].Left.Raw != afterFirstUndo.Cells[0].Right.Raw {
		t.Fatal("first copied cell was unexpectedly undone")
	}
	if afterFirstUndo.Cells[1].Left.Raw != before.Cells[1].Left.Raw {
		t.Fatal("most recent copied cell was not undone")
	}
	if err := session.Undo(); err != nil {
		t.Fatal(err)
	}
	if session.Dirty() {
		t.Fatal("undoing all changes before save should return to a clean state")
	}
}

func TestSessionDetectsExternalModificationAndPreservesIt(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session, err := Open(pair.Left, pair.Right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.EditLeft(workbook.CellRef{Sheet: "数据 表", Row: 4, Col: 1}, "session", "text"); err != nil {
		t.Fatal(err)
	}
	external, err := excelize.OpenFile(pair.Left)
	if err != nil {
		t.Fatal(err)
	}
	if err := external.SetCellStr("数据 表", "Z9", "external"); err != nil {
		t.Fatal(err)
	}
	if err := external.Save(); err != nil {
		t.Fatal(err)
	}
	_ = external.Close()
	if err := session.Save(""); !workbook.HasCode(err, workbook.ErrExternalEdit) {
		t.Fatalf("save error = %v, want external modification", err)
	}
	reopened, err := excelize.OpenFile(pair.Left)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	assertCell(t, reopened, "数据 表", "Z9", "external")
	if got, _ := reopened.GetCellValue("数据 表", "A4"); got == "session" {
		t.Fatal("session changes overwrote external file")
	}
}

func TestSessionValidationAndSaveFailure(t *testing.T) {
	dir := t.TempDir()
	pair, err := testutil.CreatePair(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(pair.Left, pair.Left, Options{}); !workbook.HasCode(err, workbook.ErrSameFile) {
		t.Fatalf("same path error = %v", err)
	}
	session, err := Open(pair.Left, pair.Right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	original, err := os.ReadFile(pair.Left)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "missing-directory", "out.xlsx")
	if err := session.Save(target); !workbook.HasCode(err, workbook.ErrSave) {
		t.Fatalf("save error = %v", err)
	}
	after, err := os.ReadFile(pair.Left)
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != string(after) {
		t.Fatal("failed Save As changed the original file")
	}
}

func TestSessionRejectsReadOnlyLeftFile(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session, err := Open(pair.Left, pair.Right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := os.Chmod(pair.Left, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(pair.Left, 0o644) })
	if err := session.Save(""); !workbook.HasCode(err, workbook.ErrUnwritable) {
		t.Fatalf("save error = %v, want unwritable", err)
	}
}

func TestSessionCanAttachAndDetachRightWithoutLosingLeftEdits(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session, err := OpenLeft(pair.Left, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if session.Summary().Diff.RightFile != "" || session.Summary().Diff.DifferenceCount != 0 {
		t.Fatalf("left-only summary = %+v", session.Summary().Diff)
	}
	if err := session.EditLeft(workbook.CellRef{Sheet: "数据 表", Row: 1, Col: 1}, "未保存编辑", "text"); err != nil {
		t.Fatal(err)
	}
	if err := session.ReplaceRight(pair.Right, "develop"); err != nil {
		t.Fatal(err)
	}
	region, err := session.Region("数据 表", 1, 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if region.Cells[0].Left.Raw != "未保存编辑" || region.Cells[0].Right.Raw != "新文本" {
		t.Fatalf("region after attach = %+v", region.Cells[0])
	}
	if !session.Dirty() || session.Summary().UndoCount != 1 {
		t.Fatalf("left edit state was lost: %+v", session.Summary())
	}
	if err := session.DetachRight("missing"); err != nil {
		t.Fatal(err)
	}
	region, err = session.Region("数据 表", 1, 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if region.Cells[0].Left.Raw != "未保存编辑" || region.Cells[0].Right.Present {
		t.Fatalf("region after detach = %+v", region.Cells[0])
	}
	if err := session.CopyRightToLeft(workbook.CellRef{Sheet: "数据 表", Row: 1, Col: 1}); err == nil {
		t.Fatal("copy unexpectedly succeeded without a right workbook")
	}
}

func assertCell(t *testing.T, f *excelize.File, sheet, axis, want string) {
	t.Helper()
	got, err := f.GetCellValue(sheet, axis, excelize.Options{RawCellValue: true})
	if err != nil || got != want {
		t.Fatalf("%s!%s = %q, want %q (err=%v)", sheet, axis, got, want, err)
	}
}
