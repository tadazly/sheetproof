package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tadazly/sheetproof/internal/diff"
	"github.com/tadazly/sheetproof/internal/testutil"
	"github.com/tadazly/sheetproof/internal/workbook"
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

func TestSessionClearLeftSelectionIsOneUndoCommand(t *testing.T) {
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
	if err := session.ClearLeftSelection("数据 表", 1, 1, 1, 2, nil); err != nil {
		t.Fatal(err)
	}
	if session.Summary().UndoCount != 1 {
		t.Fatalf("clear selection undo count = %d, want 1", session.Summary().UndoCount)
	}
	cleared, err := session.Region("数据 表", 1, 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	for index, cell := range cleared.Cells {
		if cell.Left.Present {
			t.Fatalf("cell %d was not cleared: %+v", index, cell.Left)
		}
	}
	if err := session.Undo(); err != nil {
		t.Fatal(err)
	}
	restored, err := session.Region("数据 表", 1, 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	for index := range before.Cells {
		if restored.Cells[index].Left != before.Cells[index].Left {
			t.Fatalf("cell %d after undo = %+v, want %+v", index, restored.Cells[index].Left, before.Cells[index].Left)
		}
	}
}

func TestSessionRegionKeepsOpeningLeftValueAfterEditAndSave(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session, err := Open(pair.Left, pair.Right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	before, err := session.Region("数据 表", 1, 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	openingValue := before.Cells[0].Left
	if before.Cells[0].OriginalLeft != openingValue {
		t.Fatalf("opening value = %+v, want %+v", before.Cells[0].OriginalLeft, openingValue)
	}

	if err := session.EditLeft(workbook.CellRef{Sheet: "数据 表", Row: 1, Col: 1}, "会话最新值", "text"); err != nil {
		t.Fatal(err)
	}
	if err := session.Save(""); err != nil {
		t.Fatal(err)
	}

	after, err := session.Region("数据 表", 1, 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if after.Cells[0].Left.Raw != "会话最新值" {
		t.Fatalf("latest left value = %q, want 会话最新值", after.Cells[0].Left.Raw)
	}
	if after.Cells[0].OriginalLeft != openingValue {
		t.Fatalf("opening value changed after edit/save: got %+v, want %+v", after.Cells[0].OriginalLeft, openingValue)
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
	filtered, err := session.FilteredRegion(
		"配置", []diff.RowStatus{diff.RowConflict, diff.RowDeleted}, 1, 10, 1, 3,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !filtered.Filtered || filtered.TotalRows != 2 || len(filtered.Cells) != 6 {
		t.Fatalf("filtered region shape = %+v", filtered)
	}
	if filtered.Cells[0].Row != 1 || filtered.Cells[0].SourceRow != 2 ||
		filtered.Cells[3].Row != 2 || filtered.Cells[3].SourceRow != 3 {
		t.Fatalf("filtered row mapping = %+v", filtered.Cells)
	}
	if _, err := session.FilteredRegion("配置", []diff.RowStatus{diff.RowUnchanged}, 1, 10, 1, 3); err == nil {
		t.Fatal("unchanged row filter was accepted")
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
	filtered, err = session.FilteredRegion(
		"配置", []diff.RowStatus{diff.RowConflict}, 1, 10, 1, 3,
	)
	if err != nil {
		t.Fatal(err)
	}
	if filtered.TotalRows != 1 || filtered.Cells[0].SourceRow != 2 ||
		filtered.Cells[0].LeftRow != 4 || filtered.Cells[0].RightRow != 2 ||
		filtered.Cells[0].Left.Raw != "3" || filtered.Cells[0].Right.Raw != "1" {
		t.Fatalf("appended conflict pairing = %+v", filtered.Cells)
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

func TestSessionRejectsAppendingNonConflictRows(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "no-id-left.xlsx")
	right := filepath.Join(dir, "no-id-right.xlsx")
	for path, changed := range map[string]bool{left: false, right: true} {
		file := excelize.NewFile()
		rows := [][]any{{"属性", "值"}, {"速度", 10}, {"末行", 99}}
		if changed {
			rows[1][1] = 20
		}
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

	session, err := Open(left, right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if got := session.Summary().Diff.Sheets[0].IDColumn; got != 0 {
		t.Fatalf("ID column = %d, want 0", got)
	}
	if _, err := session.AppendRowsRightToLeft("Sheet1", []int{2}, nil); err == nil {
		t.Fatal("modified row append was accepted")
	}
	if session.Dirty() || len(session.Summary().Resolutions) != 0 {
		t.Fatalf("rejected append changed session state: %+v", session.Summary())
	}
}

func TestSessionAppendsTextIDConflictWithoutReplacingID(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "text-id-left.xlsx")
	right := filepath.Join(dir, "text-id-right.xlsx")
	for path, changed := range map[string]bool{left: false, right: true} {
		file := excelize.NewFile()
		rows := [][]any{{"id", "值"}, {"活动id", "旧值"}, {"关卡名字", "相同"}}
		if changed {
			rows[1][1] = "新值"
		}
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

	session, err := Open(left, right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	sheet := session.Summary().Diff.Sheets[0]
	if sheet.IDColumn != 1 || sheet.NextID != 0 || sheet.ConflictRowCount != 1 {
		t.Fatalf("text ID summary = %+v", sheet)
	}
	assigned, err := session.AppendRowsRightToLeft("Sheet1", []int{2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(assigned) != 0 {
		t.Fatalf("text ID append assigned IDs: %#v", assigned)
	}
	region, err := session.Region("Sheet1", 4, 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if region.Cells[0].Left.Raw != "活动id" || region.Cells[1].Left.Raw != "新值" {
		t.Fatalf("text ID appended row = %+v", region.Cells)
	}
	resolution := session.Summary().Resolutions
	if len(resolution) != 1 || resolution[0].Kind != ResolutionAppendRow || resolution[0].TargetID != "" {
		t.Fatalf("text ID resolution = %+v", resolution)
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

func TestOrdinaryComparisonAlignsInsertedUniqueIDAndPreservesConflictRule(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.xlsx")
	right := filepath.Join(dir, "right.xlsx")
	writeRowsWorkbook(t, left, [][]any{
		{"id", "name", "type"},
		{1, "one", "common"},
		{2, "two", "common"},
		{3, "left-three", "left-type"},
	})
	writeRowsWorkbook(t, right, [][]any{
		{"id", "name", "type"},
		{1, "one", "common"},
		{99, "inserted", "new"},
		{2, "two", "common"},
		{3, "right-three", "right-type"},
	})

	session, err := Open(left, right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	summary := session.Summary()
	sheet := summary.Diff.Sheets[0]
	if sheet.AddedRowCount != 1 || sheet.DeletedRowCount != 0 ||
		sheet.ModifiedRowCount != 0 || sheet.ConflictRowCount != 1 {
		t.Fatalf("aligned row counts = %+v", sheet)
	}
	if !summary.RowAlignment.Available || !summary.RowAlignment.Applied ||
		summary.RowAlignment.Mode != RowAlignmentAuto || summary.RowAlignment.Moved != 2 {
		t.Fatalf("alignment summary = %+v", summary.RowAlignment)
	}
	if len(summary.Warnings) == 0 || !strings.Contains(summary.Warnings[0], "唯一 ID 对齐") {
		t.Fatalf("alignment warnings = %#v", summary.Warnings)
	}

	region, err := session.Region("Sheet1", 2, 4, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if region.Cells[6].Right.Raw != "2" || region.Cells[9].Right.Raw != "3" {
		t.Fatalf("common IDs were not aligned: %+v", region.Cells)
	}
	if region.Cells[9].RowStatus != diff.RowConflict {
		t.Fatalf("same-ID all-data change lost conflict status: %+v", region.Cells[9])
	}
	if region.Cells[3].Right.Raw != "99" || region.Cells[3].Left.Present ||
		region.Cells[3].RowStatus != diff.RowAdded {
		t.Fatalf("inserted ID row was not classified as added in place: %+v", region.Cells[3])
	}

	if err := session.CopyRowsRightToLeft("Sheet1", []int{3}); err != nil {
		t.Fatal(err)
	}
	afterCopy := session.Summary().Diff.Sheets[0]
	if afterCopy.AddedRowCount != 0 || afterCopy.ConflictRowCount != 1 {
		t.Fatalf("copying aligned addition changed unrelated classifications: %+v", afterCopy)
	}
	copied, err := session.Region("Sheet1", 3, 1, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if copied.Cells[0].Left.Raw != "99" || copied.Cells[1].Left.Raw != "inserted" {
		t.Fatalf("aligned copy used wrong physical source row: %+v", copied.Cells)
	}
	if err := session.Undo(); err != nil {
		t.Fatal(err)
	}
	afterUndo := session.Summary().Diff.Sheets[0]
	if afterUndo.AddedRowCount != 1 || afterUndo.ConflictRowCount != 1 {
		t.Fatalf("undo did not restore aligned classifications: %+v", afterUndo)
	}
	if err := session.CopyRowsRightToLeft("Sheet1", []int{5}); err != nil {
		t.Fatal(err)
	}
	afterConflictCopy := session.Summary()
	if afterConflictCopy.Diff.Sheets[0].ConflictRowCount != 0 || len(afterConflictCopy.Resolutions) != 1 {
		t.Fatalf("aligned conflict copy result = %+v", afterConflictCopy)
	}
	resolution := afterConflictCopy.Resolutions[0]
	if resolution.SourceRow != 5 || resolution.TargetRow != 4 || resolution.TargetSourceRow != 5 {
		t.Fatalf("aligned conflict resolution coordinates = %+v", resolution)
	}
	if err := session.Undo(); err != nil {
		t.Fatal(err)
	}
	if got := session.Summary().Diff.Sheets[0].ConflictRowCount; got != 1 {
		t.Fatalf("undo did not restore aligned conflict: %d", got)
	}
}

func TestOrdinaryComparisonAlignsLocalizedQualifiedIDHeader(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.xlsx")
	right := filepath.Join(dir, "right.xlsx")
	writeRowsWorkbook(t, left, [][]any{
		{"地图ID", "地图名称", "资源"},
		{20044, "共同地图", "room44"},
		{20045, "仅工作区", "room45"},
		{50001, "克洛斯星", "klos"},
		{50002, "克洛斯星林间", "klosWoodland"},
	})
	writeRowsWorkbook(t, right, [][]any{
		{"地图ID", "地图名称", "资源"},
		{20044, "共同地图", "room44"},
		{50001, "克洛斯星", "klos"},
		{50002, "克洛斯星林间", "klosWoodland"},
	})

	session, err := Open(left, right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	summary := session.Summary()
	sheet := summary.Diff.Sheets[0]
	if sheet.DeletedRowCount != 1 || sheet.AddedRowCount != 0 ||
		sheet.ModifiedRowCount != 0 || sheet.ConflictRowCount != 0 {
		t.Fatalf("localized ID row counts = %+v", sheet)
	}
	if sheet.IDColumn != 1 || !summary.RowAlignment.Available ||
		!summary.RowAlignment.Applied || summary.RowAlignment.Moved != 2 {
		t.Fatalf("localized ID alignment metadata = sheet %+v alignment %+v", sheet, summary.RowAlignment)
	}
	region, err := session.Region("Sheet1", 3, 3, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if region.Cells[0].Left.Raw != "20045" || region.Cells[0].Right.Present ||
		region.Cells[0].RowStatus != diff.RowDeleted {
		t.Fatalf("left-only localized ID was not deleted: %+v", region.Cells[0])
	}
	if region.Cells[3].Left.Raw != "50001" || region.Cells[3].Right.Raw != "50001" ||
		region.Cells[3].Status != diff.Unchanged ||
		region.Cells[6].Left.Raw != "50002" || region.Cells[6].Right.Raw != "50002" ||
		region.Cells[6].Status != diff.Unchanged {
		t.Fatalf("localized common IDs were not aligned: %+v", region.Cells)
	}
}

func TestRightOnlyMiddleRowsKeepTheirOriginalNeighborhood(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.xlsx")
	right := filepath.Join(dir, "right.xlsx")
	leftRows := [][]any{{"id", "value"}}
	rightRows := [][]any{{"id", "value"}}
	for id := 1; id <= 103; id++ {
		rightRows = append(rightRows, []any{id, fmt.Sprintf("value-%d", id)})
		if id != 43 && id != 48 {
			leftRows = append(leftRows, []any{id, fmt.Sprintf("value-%d", id)})
		}
	}
	writeRowsWorkbook(t, left, leftRows)
	writeRowsWorkbook(t, right, rightRows)

	session, err := Open(left, right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	sheet := session.Summary().Diff.Sheets[0]
	if sheet.AddedRowCount != 2 || sheet.DeletedRowCount != 0 ||
		sheet.ModifiedRowCount != 0 || sheet.ConflictRowCount != 0 {
		t.Fatalf("middle deletion counts = %+v", sheet)
	}
	wantRows := map[int]string{44: "43", 49: "48"}
	for _, row := range sheet.Rows {
		if wantID := wantRows[row.Row]; wantID != "" {
			if row.Status != diff.RowAdded || row.ID != wantID || row.LeftRow != 0 || row.RightRow != row.Row {
				t.Fatalf("middle deletion row = %+v, want logical row %d ID %s", row, row.Row, wantID)
			}
			delete(wantRows, row.Row)
		}
	}
	if len(wantRows) != 0 {
		t.Fatalf("middle deletion rows were not retained in place: %#v", wantRows)
	}
	region, err := session.Region("Sheet1", 43, 7, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if region.Cells[2].Right.Raw != "43" || region.Cells[2].Left.Present ||
		region.Cells[4].Left.Raw != "44" || region.Cells[4].Right.Raw != "44" ||
		region.Cells[12].Right.Raw != "48" || region.Cells[12].Left.Present {
		t.Fatalf("middle deletion neighborhood = %+v", region.Cells)
	}
	tail, err := session.Region("Sheet1", 104, 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if tail.Cells[0].Left.Raw != "103" || tail.Cells[0].Right.Raw != "103" ||
		tail.Cells[0].RowStatus != diff.RowUnchanged {
		t.Fatalf("tail contains displaced deletion instead of final common ID: %+v", tail.Cells)
	}
}

func TestSessionCanSetAndClearManualKeyColumnBeforeEditing(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.xlsx")
	right := filepath.Join(dir, "right.xlsx")
	writeRowsWorkbook(t, left, [][]any{{"code", "value"}, {1, "one"}, {3, "three"}})
	writeRowsWorkbook(t, right, [][]any{{"code", "value"}, {1, "one"}, {2, "two"}, {3, "three"}})

	session, err := Open(left, right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if got := session.Summary().Diff.Sheets[0].IDColumn; got != 0 {
		t.Fatalf("non-ID header selected automatic key column %d", got)
	}
	if err := session.SetKeyColumn("Sheet1", 1); err != nil {
		t.Fatal(err)
	}
	keyed := session.Summary()
	if keyed.Diff.Sheets[0].IDColumn != 1 || keyed.Diff.Sheets[0].AddedRowCount != 1 ||
		keyed.Diff.Sheets[0].ModifiedRowCount != 0 || !keyed.RowAlignment.Sheets["Sheet1"].Applied {
		t.Fatalf("manual key result = %+v / %+v", keyed.Diff.Sheets[0], keyed.RowAlignment.Sheets["Sheet1"])
	}
	region, err := session.Region("Sheet1", 3, 2, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if region.Cells[0].Right.Raw != "2" || region.Cells[0].Left.Present ||
		region.Cells[2].Left.Raw != "3" || region.Cells[2].Right.Raw != "3" {
		t.Fatalf("manual-key in-place insertion = %+v", region.Cells)
	}
	if err := session.SetKeyColumn("Sheet1", 0); err != nil {
		t.Fatal(err)
	}
	if cleared := session.Summary(); cleared.Diff.Sheets[0].IDColumn != 0 ||
		cleared.RowAlignment.Sheets["Sheet1"].Applied {
		t.Fatalf("cleared manual key = %+v / %+v", cleared.Diff.Sheets[0], cleared.RowAlignment.Sheets["Sheet1"])
	}
	if err := session.SetKeyColumn("Sheet1", 1); err != nil {
		t.Fatal(err)
	}
	if err := session.EditLeft(workbook.CellRef{Sheet: "Sheet1", Row: 2, Col: 2}, "edited", "text"); err != nil {
		t.Fatal(err)
	}
	if err := session.SetKeyColumn("Sheet1", 0); err == nil {
		t.Fatal("manual key changed despite existing undo history")
	}
}

func TestAlignedSelectionClearUsesEditablePhysicalRowsAndCanUndo(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.xlsx")
	right := filepath.Join(dir, "right.xlsx")
	writeRowsWorkbook(t, left, [][]any{{"code", "value"}, {1, "one"}, {3, "three"}})
	writeRowsWorkbook(t, right, [][]any{{"code", "value"}, {1, "one"}, {2, "two"}, {3, "three"}})

	session, err := Open(left, right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.SetKeyColumn("Sheet1", 1); err != nil {
		t.Fatal(err)
	}
	// Logical row 4 is editable physical row 3 because logical row 3 is the
	// right-only code 2 record.
	if err := session.ClearLeftSelection("Sheet1", 4, 4, 2, 2, nil); err != nil {
		t.Fatal(err)
	}
	region, err := session.Region("Sheet1", 2, 3, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if region.Cells[1].Left.Raw != "one" || region.Cells[5].Left.Present || region.Cells[5].Right.Raw != "three" {
		t.Fatalf("logical clear targeted the wrong editable row: %+v", region.Cells)
	}
	if err := session.Undo(); err != nil {
		t.Fatal(err)
	}
	restored, err := session.Region("Sheet1", 4, 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Cells[1].Left.Raw != "three" || restored.Cells[1].Status != diff.Unchanged {
		t.Fatalf("undo did not restore aligned physical row: %+v", restored.Cells)
	}
}

func TestOrdinaryComparisonDoesNotGuessBetweenMultipleQualifiedIDHeaders(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.xlsx")
	right := filepath.Join(dir, "right.xlsx")
	writeRowsWorkbook(t, left, [][]any{{"地图ID", "父级ID", "名称"}, {1, 10, "one"}, {2, 10, "two"}})
	writeRowsWorkbook(t, right, [][]any{{"地图ID", "父级ID", "名称"}, {2, 10, "two"}, {1, 10, "one"}})

	session, err := Open(left, right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	summary := session.Summary()
	if summary.RowAlignment.Available || summary.RowAlignment.Applied || summary.Diff.Sheets[0].IDColumn != 0 {
		t.Fatalf("ambiguous qualified IDs enabled alignment: %+v / %+v", summary.RowAlignment, summary.Diff.Sheets[0])
	}
}

func TestUniqueIDAlignmentAppliesAcrossComparisonEntryPoints(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.xlsx")
	right := filepath.Join(dir, "right.xlsx")
	writeRowsWorkbook(t, left, [][]any{{"id", "value"}, {"", "helper"}, {1, "one"}, {2, "two"}})
	writeRowsWorkbook(t, right, [][]any{{"id", "value"}, {"", "helper"}, {99, "new"}, {1, "one"}, {2, "two"}})

	for _, test := range []struct {
		name    string
		options Options
	}{
		{name: "direct files"},
		{name: "git difftool", options: Options{GitDiff: true, ReadonlyLeft: true}},
		{name: "UGit worktree", options: Options{UGitWorktree: true}},
		{name: "repository", options: Options{RepositoryPath: dir, RepositoryFile: "table.xlsx"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, err := Open(left, right, test.options)
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			sheet := session.Summary().Diff.Sheets[0]
			if sheet.AddedRowCount != 1 || sheet.ModifiedRowCount != 0 || sheet.DeletedRowCount != 0 {
				t.Fatalf("entry point row counts = %+v", sheet)
			}
		})
	}
}

func TestOrdinaryComparisonPartiallyAlignsReliableIDsAroundAmbiguousRows(t *testing.T) {
	for _, test := range []struct {
		name string
		tail [][]any
	}{
		{
			name: "duplicate IDs elsewhere in the sheet",
			tail: [][]any{{42, "helper-a"}, {42, "helper-b"}},
		},
		{
			name: "blank ID helper row elsewhere in the sheet",
			tail: [][]any{{"", "helper"}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			left := filepath.Join(dir, "left.xlsx")
			right := filepath.Join(dir, "right.xlsx")
			leftRows := [][]any{
				{"id", "value"},
				{4, "common-four"},
				{7, "deleted-seven"},
				{8, "deleted-eight"},
				{9, "deleted-nine"},
				{1001, "common-1001"},
				{1002, "common-1002"},
			}
			rightRows := [][]any{
				{"id", "value"},
				{4, "common-four"},
				{1001, "common-1001"},
				{1002, "common-1002"},
			}
			leftRows = append(leftRows, test.tail...)
			rightRows = append(rightRows, test.tail...)
			writeRowsWorkbook(t, left, leftRows)
			writeRowsWorkbook(t, right, rightRows)
			session, err := Open(left, right, Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			summary := session.Summary()
			if !summary.RowAlignment.Available || !summary.RowAlignment.Applied || summary.RowAlignment.Moved != 2 {
				t.Fatalf("reliable IDs were not partially aligned: %+v", summary.RowAlignment)
			}
			sheet := summary.Diff.Sheets[0]
			if sheet.DeletedRowCount != 3 || sheet.AddedRowCount != 0 ||
				sheet.ModifiedRowCount != 0 || sheet.ConflictRowCount != 0 {
				t.Fatalf("ambiguous helper rows created false differences: %+v", sheet)
			}
			region, err := session.Region("Sheet1", 3, 5, 1, 2)
			if err != nil {
				t.Fatal(err)
			}
			if region.Cells[0].Right.Present || region.Cells[0].RowStatus != diff.RowDeleted ||
				region.Cells[2].Right.Present || region.Cells[2].RowStatus != diff.RowDeleted ||
				region.Cells[4].Right.Present || region.Cells[4].RowStatus != diff.RowDeleted {
				t.Fatalf("left-only IDs were not retained as deleted rows: %+v", region.Cells)
			}
			if region.Cells[6].Left.Raw != "1001" || region.Cells[6].Right.Raw != "1001" ||
				region.Cells[6].Status != diff.Unchanged ||
				region.Cells[8].Left.Raw != "1002" || region.Cells[8].Right.Raw != "1002" ||
				region.Cells[8].Status != diff.Unchanged {
				t.Fatalf("common IDs after deleted rows were not aligned: %+v", region.Cells)
			}
		})
	}
}

func TestOrdinaryComparisonFallsBackWithoutAnyReliableID(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.xlsx")
	right := filepath.Join(dir, "right.xlsx")
	writeRowsWorkbook(t, left, [][]any{{"id", "value"}, {1, "left-a"}, {1, "left-b"}})
	writeRowsWorkbook(t, right, [][]any{{"id", "value"}, {1, "right-a"}, {1, "right-b"}})

	session, err := Open(left, right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	summary := session.Summary()
	if summary.RowAlignment.Available || summary.RowAlignment.Applied {
		t.Fatalf("ambiguous-only IDs enabled semantic alignment: %+v", summary.RowAlignment)
	}
	region, err := session.Region("Sheet1", 2, 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if region.Cells[0].Right.Raw != "1" || region.Cells[1].Right.Raw != "right-a" {
		t.Fatalf("ambiguous-only IDs did not retain physical rows: %+v", region.Cells)
	}
}

func TestOrdinaryComparisonRequiresSameIDHeaderColumn(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.xlsx")
	right := filepath.Join(dir, "right.xlsx")
	writeRowsWorkbook(t, left, [][]any{{"id", "value"}, {1, "same"}})
	writeRowsWorkbook(t, right, [][]any{{"value", "id"}, {"same", 1}})

	session, err := Open(left, right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if alignment := session.Summary().RowAlignment; alignment.Available || alignment.Applied {
		t.Fatalf("different ID columns enabled alignment: %+v", alignment)
	}
	region, err := session.Region("Sheet1", 2, 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if region.Cells[0].Right.Raw != "same" || region.Cells[1].Right.Raw != "1" {
		t.Fatalf("different ID columns did not retain physical rows: %+v", region.Cells)
	}
}

func TestOrdinaryComparisonCanSwitchBackToPhysicalRowsBeforeEditing(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left.xlsx")
	right := filepath.Join(dir, "right.xlsx")
	writeRowsWorkbook(t, left, [][]any{{"id", "value"}, {1, "one"}, {2, "two"}})
	writeRowsWorkbook(t, right, [][]any{{"id", "value"}, {2, "two"}, {1, "one"}})

	session, err := Open(left, right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if sheet := session.Summary().Diff.Sheets[0]; sheet.DifferenceCount != 0 {
		t.Fatalf("ID-aligned reorder differences = %+v", sheet)
	}
	if err := session.SetRowAlignment(RowAlignmentPosition); err != nil {
		t.Fatal(err)
	}
	positionSummary := session.Summary()
	if positionSummary.RowAlignment.Mode != RowAlignmentPosition || positionSummary.RowAlignment.Applied {
		t.Fatalf("position alignment summary = %+v", positionSummary.RowAlignment)
	}
	if sheet := positionSummary.Diff.Sheets[0]; sheet.ModifiedRowCount != 2 {
		t.Fatalf("physical-row reorder counts = %+v", sheet)
	}
	if err := session.SetRowAlignment(RowAlignmentAuto); err != nil {
		t.Fatal(err)
	}
	if sheet := session.Summary().Diff.Sheets[0]; sheet.DifferenceCount != 0 {
		t.Fatalf("restored ID alignment = %+v", sheet)
	}
	if err := session.EditLeft(workbook.CellRef{Sheet: "Sheet1", Row: 2, Col: 2}, "edited", "text"); err != nil {
		t.Fatal(err)
	}
	if err := session.SetRowAlignment(RowAlignmentPosition); err == nil {
		t.Fatal("alignment switched despite existing undo history")
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

func writeRowsWorkbook(t *testing.T, path string, rows [][]any) {
	t.Helper()
	file := excelize.NewFile()
	for rowIndex, row := range rows {
		for colIndex, value := range row {
			axis, err := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
			if err != nil {
				t.Fatal(err)
			}
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

func TestSessionDetectsAndReloadsExternalChangesPerSide(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session, err := Open(pair.Left, pair.Right, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.EditLeft(workbook.CellRef{Sheet: "数据 表", Row: 4, Col: 1}, "session edit", "text"); err != nil {
		t.Fatal(err)
	}

	setWorkbookCell(t, pair.Left, "数据 表", "A1", "externally changed left workbook")
	setWorkbookCell(t, pair.Right, "数据 表", "A1", "externally changed right workbook")
	changes, err := session.ExternalChanges()
	if err != nil {
		t.Fatal(err)
	}
	if !changes.Left.Changed || !changes.Left.Writable || changes.Left.Signature == "" {
		t.Fatalf("left external change = %+v", changes.Left)
	}
	if !changes.Right.Changed || changes.Right.Writable || changes.Right.Signature == "" {
		t.Fatalf("right external change = %+v", changes.Right)
	}

	if err := session.ReloadRight(); err != nil {
		t.Fatal(err)
	}
	afterRight, err := session.Region("数据 表", 1, 4, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if afterRight.Cells[0].Right.Raw != "externally changed right workbook" {
		t.Fatalf("right value after reload = %q", afterRight.Cells[0].Right.Raw)
	}
	if afterRight.Cells[3].Left.Raw != "session edit" || !session.Dirty() {
		t.Fatal("reloading the read-only side discarded editable left-side work")
	}

	if err := session.ReloadLeft(); err != nil {
		t.Fatal(err)
	}
	afterLeft, err := session.Region("数据 表", 1, 4, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if afterLeft.Cells[0].Left.Raw != "externally changed left workbook" {
		t.Fatalf("left value after reload = %q", afterLeft.Cells[0].Left.Raw)
	}
	if session.Dirty() || session.Summary().UndoCount != 0 {
		t.Fatalf("reloaded left session state = dirty %v, undo %d", session.Dirty(), session.Summary().UndoCount)
	}
	if afterLeft.Cells[0].OriginalLeft.Raw != afterLeft.Cells[0].Left.Raw {
		t.Fatal("left reload did not establish the latest disk version as the new preview baseline")
	}
	changes, err = session.ExternalChanges()
	if err != nil {
		t.Fatal(err)
	}
	if changes.Left.Changed || changes.Right.Changed {
		t.Fatalf("changes remained after reload = %+v", changes)
	}
}

func TestSessionReportsReadonlyLeftExternalChange(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session, err := Open(pair.Left, pair.Right, Options{ReadonlyLeft: true})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	setWorkbookCell(t, pair.Left, "数据 表", "A1", "readonly source changed")
	changes, err := session.ExternalChanges()
	if err != nil {
		t.Fatal(err)
	}
	if !changes.Left.Changed || changes.Left.Writable {
		t.Fatalf("readonly left change = %+v", changes.Left)
	}
}

func setWorkbookCell(t *testing.T, path, sheet, axis, value string) {
	t.Helper()
	file, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := file.SetCellStr(sheet, axis, value); err != nil {
		t.Fatal(err)
	}
	if err := file.Save(); err != nil {
		t.Fatal(err)
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

func TestCollectRegionStatusesOnlyBuildsRequestedViewport(t *testing.T) {
	sheetDiff := &diff.SheetDiff{Name: "large"}
	for row := 1; row <= 10_000; row++ {
		status := diff.RowModified
		if row == 9_999 {
			status = diff.RowConflict
		}
		sheetDiff.Rows = append(sheetDiff.Rows, diff.RowDiff{Row: row, Status: status})
		for _, col := range []int{1, 3, 5} {
			sheetDiff.Differences = append(sheetDiff.Differences, diff.CellDiff{
				Ref: workbook.CellRef{Sheet: "large", Row: row, Col: col}, Status: diff.Modified,
			})
		}
	}
	statuses := make(map[workbook.CellKey]diff.CellStatus)
	rowStatuses := make(map[int]diff.RowStatus)
	collectRegionStatuses(
		sheetDiff,
		[]regionRow{{source: 5}, {source: 9_999}},
		2,
		4,
		statuses,
		rowStatuses,
	)

	if len(statuses) != 2 {
		t.Fatalf("viewport statuses = %d, want 2", len(statuses))
	}
	for _, row := range []int{5, 9_999} {
		if statuses[workbook.CellKey{Row: row, Col: 3}] != diff.Modified {
			t.Fatalf("missing requested status for row %d", row)
		}
		if _, exists := statuses[workbook.CellKey{Row: row, Col: 1}]; exists {
			t.Fatalf("included off-screen column for row %d", row)
		}
	}
	if rowStatuses[5] != diff.RowModified || rowStatuses[9_999] != diff.RowConflict {
		t.Fatalf("row statuses = %#v", rowStatuses)
	}
}

func assertCell(t *testing.T, f *excelize.File, sheet, axis, want string) {
	t.Helper()
	got, err := f.GetCellValue(sheet, axis, excelize.Options{RawCellValue: true})
	if err != nil || got != want {
		t.Fatalf("%s!%s = %q, want %q (err=%v)", sheet, axis, got, want, err)
	}
}
