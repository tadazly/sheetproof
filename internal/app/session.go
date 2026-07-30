package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/ug-tools/ugxlsx/internal/diff"
	"github.com/ug-tools/ugxlsx/internal/history"
	"github.com/ug-tools/ugxlsx/internal/merge"
	"github.com/ug-tools/ugxlsx/internal/storage"
	"github.com/ug-tools/ugxlsx/internal/workbook"
	"github.com/xuri/excelize/v2"
)

type Options struct {
	Title          string `json:"title"`
	LeftLabel      string `json:"leftLabel"`
	RightLabel     string `json:"rightLabel"`
	ReadonlyLeft   bool   `json:"readonlyLeft"`
	Output         string `json:"output"`
	RepositoryPath string `json:"repositoryPath,omitempty"`
	RepositoryFile string `json:"repositoryFile,omitempty"`
	RepositoryRef  string `json:"repositoryRef,omitempty"`
}

type Summary struct {
	Options       Options            `json:"options"`
	Diff          *diff.WorkbookDiff `json:"diff"`
	Dirty         bool               `json:"dirty"`
	UndoCount     int                `json:"undoCount"`
	Warnings      []string           `json:"warnings"`
	SelectedSheet string             `json:"selectedSheet"`
}

type RegionCell struct {
	Row    int                `json:"row"`
	Col    int                `json:"col"`
	Axis   string             `json:"axis"`
	Left   workbook.CellValue `json:"left"`
	Right  workbook.CellValue `json:"right"`
	Status diff.CellStatus    `json:"status"`
}

type Region struct {
	Sheet   string       `json:"sheet"`
	FromRow int          `json:"fromRow"`
	ToRow   int          `json:"toRow"`
	FromCol int          `json:"fromCol"`
	ToCol   int          `json:"toCol"`
	Cells   []RegionCell `json:"cells"`
}

type CellCoordinate struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type Session struct {
	mu          sync.RWMutex
	leftFile    *excelize.File
	rightFile   *excelize.File
	left        *workbook.WorkbookSnapshot
	right       *workbook.WorkbookSnapshot
	currentDiff *diff.WorkbookDiff
	history     history.Stack
	options     Options
	dirty       bool
	warnings    []string
	stateID     uint64
	savedState  uint64
	nextState   uint64
}

func Open(leftPath, rightPath string, options Options) (*Session, error) {
	same, err := workbook.SamePath(leftPath, rightPath)
	if err != nil {
		return nil, err
	}
	if same {
		return nil, &workbook.Error{Code: workbook.ErrSameFile, Path: leftPath, Err: fmt.Errorf("left and right paths refer to the same file")}
	}
	reader := workbook.Reader{}
	leftFile, left, err := reader.Open(leftPath)
	if err != nil {
		return nil, err
	}
	rightFile, right, err := reader.Open(rightPath)
	if err != nil {
		_ = leftFile.Close()
		return nil, err
	}
	options = defaultOptions(options)
	return &Session{
		leftFile: leftFile, rightFile: rightFile,
		left: left, right: right, currentDiff: diff.Compare(left, right),
		options: options, stateID: 1, savedState: 1, nextState: 2,
	}, nil
}

// OpenLeft opens the editable workbook without manufacturing an empty right
// workbook. Repository mode uses it while no comparison ref is selected or the
// selected ref does not contain the same path.
func OpenLeft(leftPath string, options Options) (*Session, error) {
	reader := workbook.Reader{}
	leftFile, left, err := reader.Open(leftPath)
	if err != nil {
		return nil, err
	}
	options = defaultOptions(options)
	return &Session{
		leftFile: leftFile, left: left, options: options,
		stateID: 1, savedState: 1, nextState: 2,
	}, nil
}

func defaultOptions(options Options) Options {
	if options.LeftLabel == "" {
		options.LeftLabel = "本地（可编辑）"
	}
	if options.RightLabel == "" {
		options.RightLabel = "对比来源（只读）"
	}
	return options
}

// ReplaceRight changes only the read-only comparison source. In-memory edits,
// undo history and the left save target remain intact.
func (s *Session) ReplaceRight(rightPath, rightLabel string) error {
	reader := workbook.Reader{}
	rightFile, right, err := reader.Open(rightPath)
	if err != nil {
		return err
	}
	s.mu.Lock()
	old := s.rightFile
	s.rightFile = rightFile
	s.right = right
	s.currentDiff = diff.Compare(s.left, right)
	if rightLabel != "" {
		s.options.RightLabel = rightLabel
	}
	s.mu.Unlock()
	if old != nil {
		return old.Close()
	}
	return nil
}

// DetachRight keeps the editable workbook open while explicitly representing
// that there is no right-side workbook to compare.
func (s *Session) DetachRight(rightLabel string) error {
	s.mu.Lock()
	old := s.rightFile
	s.rightFile = nil
	s.right = nil
	s.currentDiff = nil
	if rightLabel != "" {
		s.options.RightLabel = rightLabel
	}
	s.mu.Unlock()
	if old != nil {
		return old.Close()
	}
	return nil
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var errs []error
	if s.leftFile != nil {
		errs = append(errs, s.leftFile.Close())
	}
	if s.rightFile != nil {
		errs = append(errs, s.rightFile.Close())
	}
	return errors.Join(errs...)
}

func (s *Session) Summary() Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	selected := ""
	presentation := s.presentationDiffLocked()
	if len(presentation.Sheets) > 0 {
		selected = presentation.Sheets[0].Name
	}
	return Summary{
		Options: s.options, Diff: compactDiff(presentation),
		Dirty: s.dirty, UndoCount: s.history.Len(),
		Warnings: append([]string{}, s.warnings...), SelectedSheet: selected,
	}
}

func (s *Session) presentationDiffLocked() *diff.WorkbookDiff {
	if s.currentDiff != nil {
		return s.currentDiff
	}
	result := &diff.WorkbookDiff{
		Equal: false, LeftFile: s.left.Path, Sheets: make([]*diff.SheetDiff, 0, len(s.left.Sheets)),
	}
	for _, sheet := range s.left.Sheets {
		result.Sheets = append(result.Sheets, &diff.SheetDiff{
			Name: sheet.Name, Status: diff.SheetEqual, LeftIndex: sheet.Index, RightIndex: -1,
			MaxRow: sheet.MaxRow, MaxCol: sheet.MaxCol,
		})
	}
	result.SheetCount = len(result.Sheets)
	return result
}

func compactDiff(source *diff.WorkbookDiff) *diff.WorkbookDiff {
	copy := *source
	copy.Sheets = make([]*diff.SheetDiff, len(source.Sheets))
	for i, sheet := range source.Sheets {
		sheetCopy := *sheet
		sheetCopy.Differences = nil
		copy.Sheets[i] = &sheetCopy
	}
	return &copy
}

func (s *Session) Differences(sheet string, offset, limit int) ([]diff.CellDiff, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.currentDiff == nil {
		if s.left.ByName[sheet] != nil {
			return []diff.CellDiff{}, nil
		}
		return nil, fmt.Errorf("worksheet %q not found", sheet)
	}
	for _, item := range s.currentDiff.Sheets {
		if item.Name != sheet {
			continue
		}
		if offset < 0 {
			offset = 0
		}
		if limit <= 0 || limit > 10000 {
			limit = 1000
		}
		if offset >= len(item.Differences) {
			return []diff.CellDiff{}, nil
		}
		end := min(offset+limit, len(item.Differences))
		return append([]diff.CellDiff(nil), item.Differences[offset:end]...), nil
	}
	return nil, fmt.Errorf("worksheet %q not found", sheet)
}

func (s *Session) Region(sheet string, fromRow, rowCount, fromCol, colCount int) (Region, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if fromRow < 1 || fromCol < 1 || rowCount < 1 || colCount < 1 || rowCount > 300 || colCount > 100 {
		return Region{}, fmt.Errorf("invalid region (maximum 300 rows by 100 columns)")
	}
	leftSheet := s.left.ByName[sheet]
	var rightSheet *workbook.SheetSnapshot
	if s.right != nil {
		rightSheet = s.right.ByName[sheet]
	}
	if leftSheet == nil && rightSheet == nil {
		return Region{}, fmt.Errorf("worksheet %q not found", sheet)
	}
	statuses := make(map[workbook.CellKey]diff.CellStatus)
	if s.currentDiff != nil {
		for _, sheetDiff := range s.currentDiff.Sheets {
			if sheetDiff.Name == sheet {
				for _, cellDiff := range sheetDiff.Differences {
					statuses[workbook.CellKey{Row: cellDiff.Ref.Row, Col: cellDiff.Ref.Col}] = cellDiff.Status
				}
				break
			}
		}
	}
	region := Region{
		Sheet: sheet, FromRow: fromRow, ToRow: fromRow + rowCount - 1,
		FromCol: fromCol, ToCol: fromCol + colCount - 1,
		Cells: make([]RegionCell, 0, rowCount*colCount),
	}
	for row := region.FromRow; row <= region.ToRow; row++ {
		for col := region.FromCol; col <= region.ToCol; col++ {
			ref := workbook.CellRef{Sheet: sheet, Row: row, Col: col}
			key := workbook.CellKey{Row: row, Col: col}
			status := statuses[key]
			if status == "" {
				status = diff.Unchanged
			}
			region.Cells = append(region.Cells, RegionCell{
				Row: row, Col: col, Axis: ref.Axis(),
				Left: leftSheet.Cell(row, col), Right: rightSheet.Cell(row, col),
				Status: status,
			})
		}
	}
	return region, nil
}

func (s *Session) CopyRightToLeft(ref workbook.CellRef) error {
	return s.CopyRightToLeftMany(ref.Sheet, []CellCoordinate{{Row: ref.Row, Col: ref.Col}})
}

func (s *Session) CopyRightToLeftMany(sheet string, cells []CellCoordinate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.options.ReadonlyLeft {
		return fmt.Errorf("left workbook is read-only")
	}
	if s.right == nil || s.rightFile == nil {
		return fmt.Errorf("当前没有可用于合并的右侧工作簿")
	}
	if s.right.ByName[sheet] == nil || s.left.ByName[sheet] == nil {
		return fmt.Errorf("worksheet %q must exist on both sides", sheet)
	}
	if len(cells) == 0 {
		return fmt.Errorf("no cells selected")
	}
	if len(cells) > 10000 {
		return fmt.Errorf("too many cells selected (maximum 10000)")
	}
	operations := make([]merge.Operation, 0, len(cells))
	seen := make(map[workbook.CellKey]struct{}, len(cells))
	for _, cell := range cells {
		if cell.Row < 1 || cell.Col < 1 {
			return fmt.Errorf("invalid cell coordinate %d:%d", cell.Row, cell.Col)
		}
		key := workbook.CellKey{Row: cell.Row, Col: cell.Col}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		ref := workbook.CellRef{Sheet: sheet, Row: cell.Row, Col: cell.Col}
		before, err := merge.Capture(s.leftFile, ref)
		if err != nil {
			return err
		}
		after, err := merge.Capture(s.rightFile, ref)
		if err != nil {
			return err
		}
		operations = append(operations, merge.Operation{Ref: ref, Before: before, After: after, Kind: "copy"})
	}
	appliedCount := 0
	for index, operation := range operations {
		warnings, err := merge.Apply(s.leftFile, operation.Ref, operation.After)
		if err != nil {
			s.rollbackCopiesLocked(operations[:appliedCount])
			return fmt.Errorf("copy %s failed: %w", operation.Ref.Axis(), err)
		}
		applied, err := merge.Capture(s.leftFile, operation.Ref)
		if err != nil {
			s.rollbackCopiesLocked(operations[:index+1])
			return fmt.Errorf("capture copied %s failed: %w", operation.Ref.Axis(), err)
		}
		if err := s.updateCellLocked(operation.Ref, applied); err != nil {
			s.rollbackCopiesLocked(operations[:index+1])
			return err
		}
		s.warnings = append(s.warnings, warnings...)
		appliedCount++
	}
	s.recordHistoryLocked(operations)
	return nil
}

func (s *Session) rollbackCopiesLocked(operations []merge.Operation) {
	for index := len(operations) - 1; index >= 0; index-- {
		operation := operations[index]
		if _, err := merge.Apply(s.leftFile, operation.Ref, operation.Before); err != nil {
			s.warnings = append(s.warnings, fmt.Sprintf("回滚 %s 失败: %v", operation.Ref.Axis(), err))
			continue
		}
		if restored, err := merge.Capture(s.leftFile, operation.Ref); err == nil {
			_ = s.updateCellLocked(operation.Ref, restored)
		}
	}
}

func (s *Session) EditLeft(ref workbook.CellRef, value, valueType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.options.ReadonlyLeft {
		return fmt.Errorf("left workbook is read-only")
	}
	if s.left.ByName[ref.Sheet] == nil {
		return fmt.Errorf("left worksheet %q not found", ref.Sheet)
	}
	before, err := merge.Capture(s.leftFile, ref)
	if err != nil {
		return err
	}
	after := before
	after.Value.Present = true
	after.Value.Formula = ""
	switch valueType {
	case "clear":
		after.Value = workbook.CellValue{}
	case "formula":
		after.Value.Raw = ""
		after.Value.Display = ""
		after.Value.Formula = strings.TrimPrefix(value, "=")
		after.Value.Type = "formula"
	case "number":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("invalid number %q", value)
		}
		after.Value.Raw, after.Value.Display, after.Value.Type = value, value, "number"
	case "text", "":
		after.Value.Raw, after.Value.Display, after.Value.Type = value, value, "string"
	default:
		return fmt.Errorf("unsupported edit type %q", valueType)
	}
	warnings, err := merge.Apply(s.leftFile, ref, after)
	if err != nil {
		return err
	}
	applied, err := merge.Capture(s.leftFile, ref)
	if err != nil {
		return err
	}
	s.warnings = append(s.warnings, warnings...)
	if err := s.updateCellLocked(ref, applied); err != nil {
		return err
	}
	s.recordHistoryLocked([]merge.Operation{{Ref: ref, Before: before, After: after, Kind: "edit"}})
	return nil
}

func (s *Session) Undo() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := s.history.Pop()
	if err != nil {
		return err
	}
	commands := entry.Operations
	undone := make([]merge.Operation, 0, len(commands))
	for index := len(commands) - 1; index >= 0; index-- {
		command := commands[index]
		warnings, err := merge.Apply(s.leftFile, command.Ref, command.Before)
		if err != nil {
			s.restoreUndoneLocked(undone)
			s.history.PushEntry(entry)
			return err
		}
		applied, err := merge.Capture(s.leftFile, command.Ref)
		if err != nil {
			s.restoreUndoneLocked(append(undone, command))
			s.history.PushEntry(entry)
			return err
		}
		s.warnings = append(s.warnings, warnings...)
		if err := s.updateCellLocked(command.Ref, applied); err != nil {
			s.restoreUndoneLocked(append(undone, command))
			s.history.PushEntry(entry)
			return err
		}
		undone = append(undone, command)
	}
	s.stateID = entry.BeforeState
	s.dirty = s.stateID != s.savedState
	return nil
}

func (s *Session) restoreUndoneLocked(operations []merge.Operation) {
	for index := len(operations) - 1; index >= 0; index-- {
		operation := operations[index]
		if _, err := merge.Apply(s.leftFile, operation.Ref, operation.After); err != nil {
			s.warnings = append(s.warnings, fmt.Sprintf("恢复 %s 失败: %v", operation.Ref.Axis(), err))
			continue
		}
		if restored, err := merge.Capture(s.leftFile, operation.Ref); err == nil {
			_ = s.updateCellLocked(operation.Ref, restored)
		}
	}
}

func (s *Session) Save(target string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.options.ReadonlyLeft {
		return fmt.Errorf("left workbook is read-only")
	}
	if target == "" {
		target = s.options.Output
	}
	if target == "" {
		target = s.left.Path
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	leftAbs, _ := filepath.Abs(s.left.Path)
	var expected *workbook.FileIdentity
	if filepath.Clean(abs) == filepath.Clean(leftAbs) {
		expected = &s.left.Identity
	}
	id, err := (storage.SafeWriter{}).Save(s.leftFile, abs, expected)
	if err != nil {
		return err
	}
	s.left.Path = abs
	s.left.Identity = id
	s.options.Output = abs
	s.savedState = s.stateID
	s.dirty = false
	return nil
}

func (s *Session) Dirty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dirty
}

func (s *Session) recordHistoryLocked(operations []merge.Operation) {
	after := s.nextState
	s.nextState++
	s.history.PushEntry(history.Entry{
		Operations:  operations,
		BeforeState: s.stateID,
		AfterState:  after,
	})
	s.stateID = after
	s.dirty = s.stateID != s.savedState
}

func (s *Session) updateCellLocked(ref workbook.CellRef, state merge.CellState) error {
	sheet := s.left.ByName[ref.Sheet]
	if sheet == nil {
		return fmt.Errorf("left worksheet %q not found", ref.Sheet)
	}
	key := workbook.CellKey{Row: ref.Row, Col: ref.Col}
	_, existed := sheet.Cells[key]
	if state.Value.Present {
		sheet.Cells[key] = state.Value
		if !existed {
			index := sort.Search(len(sheet.CellList), func(i int) bool {
				item := sheet.CellList[i]
				return item.Row > key.Row || (item.Row == key.Row && item.Col >= key.Col)
			})
			sheet.CellList = append(sheet.CellList, workbook.CellKey{})
			copy(sheet.CellList[index+1:], sheet.CellList[index:])
			sheet.CellList[index] = key
		}
		sheet.MaxRow = max(sheet.MaxRow, ref.Row)
		sheet.MaxCol = max(sheet.MaxCol, ref.Col)
	} else {
		delete(sheet.Cells, key)
		if existed {
			index := sort.Search(len(sheet.CellList), func(i int) bool {
				item := sheet.CellList[i]
				return item.Row > key.Row || (item.Row == key.Row && item.Col >= key.Col)
			})
			if index < len(sheet.CellList) && sheet.CellList[index] == key {
				sheet.CellList = append(sheet.CellList[:index], sheet.CellList[index+1:]...)
			}
		}
	}
	if s.currentDiff == nil || s.right == nil {
		return nil
	}
	return s.currentDiff.UpdateCell(ref, state.Value, s.right.ByName[ref.Sheet].Cell(ref.Row, ref.Col))
}
