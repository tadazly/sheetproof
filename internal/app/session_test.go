package app

import (
	"os"
	"path/filepath"
	"testing"

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
