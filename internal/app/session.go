package app

import (
	"context"
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
	GitDiff        bool   `json:"gitDiff"`
	UGitWorktree   bool   `json:"ugitWorktree"`
	GitMerge       bool   `json:"gitMerge"`
	Output         string `json:"output"`
	MergeBase      string `json:"-"`
	RepositoryPath string `json:"repositoryPath,omitempty"`
	RepositoryFile string `json:"repositoryFile,omitempty"`
	RepositoryRef  string `json:"repositoryRef,omitempty"`
}

type Summary struct {
	Options       Options            `json:"options"`
	Diff          *diff.WorkbookDiff `json:"diff"`
	Resolutions   []RowResolution    `json:"resolutions"`
	Dirty         bool               `json:"dirty"`
	UndoCount     int                `json:"undoCount"`
	Warnings      []string           `json:"warnings"`
	MergeNotice   string             `json:"mergeNotice"`
	SelectedSheet string             `json:"selectedSheet"`
}

type ResolutionKind string

const (
	ResolutionOverwriteCells  ResolutionKind = "overwrite-cells"
	ResolutionOverwriteRow    ResolutionKind = "overwrite-row"
	ResolutionAppendAuto      ResolutionKind = "append-auto"
	ResolutionAppendSpecified ResolutionKind = "append-specified"
)

type RowResolution struct {
	Sheet     string         `json:"sheet"`
	SourceRow int            `json:"sourceRow"`
	TargetRow int            `json:"targetRow,omitempty"`
	TargetID  string         `json:"targetId,omitempty"`
	Kind      ResolutionKind `json:"kind"`
	CellCount int            `json:"cellCount,omitempty"`
	stateID   uint64
}

type RegionCell struct {
	Row          int                `json:"row"`
	Col          int                `json:"col"`
	Axis         string             `json:"axis"`
	Left         workbook.CellValue `json:"left"`
	OriginalLeft workbook.CellValue `json:"originalLeft"`
	Right        workbook.CellValue `json:"right"`
	Status       diff.CellStatus    `json:"status"`
	RowStatus    diff.RowStatus     `json:"rowStatus"`
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
	mu           sync.RWMutex
	leftFile     *excelize.File
	rightFile    *excelize.File
	left         *workbook.WorkbookSnapshot
	originalLeft *workbook.WorkbookSnapshot
	right        *workbook.WorkbookSnapshot
	rightSource  *workbook.WorkbookSnapshot
	rightRows    map[string]map[int]int
	currentDiff  *diff.WorkbookDiff
	history      history.Stack
	options      Options
	dirty        bool
	warnings     []string
	mergeNotice  string
	stateID      uint64
	savedState   uint64
	nextState    uint64
	resolutions  []RowResolution
}

func Open(leftPath, rightPath string, options Options) (*Session, error) {
	return OpenContext(context.Background(), leftPath, rightPath, options)
}

// OpenContext builds a comparison session with a cancellable workbook scan.
// It is used by background repository indexing; normal interactive callers use
// Open and retain the existing behavior.
func OpenContext(ctx context.Context, leftPath, rightPath string, options Options) (*Session, error) {
	same, err := workbook.SamePath(leftPath, rightPath)
	if err != nil {
		return nil, err
	}
	if same {
		return nil, &workbook.Error{Code: workbook.ErrSameFile, Path: leftPath, Err: fmt.Errorf("left and right paths refer to the same file")}
	}
	reader := workbook.Reader{}
	leftFile, left, err := reader.OpenContext(ctx, leftPath)
	if err != nil {
		return nil, err
	}
	rightFile, rightSource, err := reader.OpenContext(ctx, rightPath)
	if err != nil {
		_ = leftFile.Close()
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		_ = rightFile.Close()
		_ = leftFile.Close()
		return nil, err
	}
	right := rightSource
	rightRows := identityRightRows(rightSource)
	alignmentWarnings := []string(nil)
	if options.GitMerge {
		var moved int
		right, rightRows, moved = alignRightRowsByID(left, rightSource)
		if moved > 0 {
			alignmentWarnings = append(alignmentWarnings, fmt.Sprintf(
				"检测到 %d 条同 ID 记录位于不同物理行，已按左侧 ID 对齐显示，避免把插入/删除放大为连续修改。", moved,
			))
		}
	}
	currentDiff, err := compareContext(ctx, left, right)
	if err != nil {
		_ = rightFile.Close()
		_ = leftFile.Close()
		return nil, err
	}
	mergeNotice := ""
	if options.GitMerge && strings.TrimSpace(options.MergeBase) != "" {
		baseFile, base, baseErr := reader.OpenContext(ctx, options.MergeBase)
		if baseErr != nil {
			_ = rightFile.Close()
			_ = leftFile.Close()
			return nil, baseErr
		}
		_ = baseFile.Close()
		mergeNotice = mergeSemanticNotice(base, left, rightSource)
	}
	options = defaultOptions(options)
	return &Session{
		leftFile: leftFile, rightFile: rightFile,
		left: left, originalLeft: cloneWorkbookSnapshot(left),
		right: right, rightSource: rightSource, rightRows: rightRows,
		currentDiff: currentDiff, warnings: alignmentWarnings, mergeNotice: mergeNotice,
		options: options, stateID: 1, savedState: 1, nextState: 2,
	}, nil
}

func compareContext(
	ctx context.Context,
	left, right *workbook.WorkbookSnapshot,
) (*diff.WorkbookDiff, error) {
	if ctx.Done() == nil {
		return diff.Compare(left, right), nil
	}
	result := make(chan *diff.WorkbookDiff, 1)
	go func() {
		result <- diff.Compare(left, right)
	}()
	select {
	case currentDiff := <-result:
		return currentDiff, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
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
		leftFile: leftFile, left: left, originalLeft: cloneWorkbookSnapshot(left), options: options,
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
	rightFile, rightSource, err := reader.Open(rightPath)
	if err != nil {
		return err
	}
	s.mu.Lock()
	old := s.rightFile
	right := rightSource
	rightRows := identityRightRows(rightSource)
	if s.options.GitMerge {
		right, rightRows, _ = alignRightRowsByID(s.left, rightSource)
	}
	s.rightFile = rightFile
	s.right = right
	s.rightSource = rightSource
	s.rightRows = rightRows
	s.currentDiff = diff.Compare(s.left, right)
	s.resolutions = nil
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
	s.rightSource = nil
	s.rightRows = nil
	s.currentDiff = nil
	s.resolutions = nil
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
		Resolutions: append([]RowResolution{}, s.resolutions...),
		Dirty:       s.dirty, UndoCount: s.history.Len(),
		Warnings: append([]string{}, s.warnings...), MergeNotice: s.mergeNotice, SelectedSheet: selected,
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
	originalLeftSheet := s.originalLeft.ByName[sheet]
	var rightSheet *workbook.SheetSnapshot
	if s.right != nil {
		rightSheet = s.right.ByName[sheet]
	}
	if leftSheet == nil && rightSheet == nil {
		return Region{}, fmt.Errorf("worksheet %q not found", sheet)
	}
	statuses := make(map[workbook.CellKey]diff.CellStatus)
	rowStatuses := make(map[int]diff.RowStatus)
	if s.currentDiff != nil {
		for _, sheetDiff := range s.currentDiff.Sheets {
			if sheetDiff.Name == sheet {
				for _, cellDiff := range sheetDiff.Differences {
					statuses[workbook.CellKey{Row: cellDiff.Ref.Row, Col: cellDiff.Ref.Col}] = cellDiff.Status
				}
				for _, rowDiff := range sheetDiff.Rows {
					rowStatuses[rowDiff.Row] = rowDiff.Status
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
			rowStatus := rowStatuses[row]
			if rowStatus == "" {
				rowStatus = diff.RowUnchanged
			}
			region.Cells = append(region.Cells, RegionCell{
				Row: row, Col: col, Axis: ref.Axis(),
				Left: leftSheet.Cell(row, col), OriginalLeft: originalLeftSheet.Cell(row, col),
				Right:  rightSheet.Cell(row, col),
				Status: status, RowStatus: rowStatus,
			})
		}
	}
	return region, nil
}

func cloneWorkbookSnapshot(source *workbook.WorkbookSnapshot) *workbook.WorkbookSnapshot {
	if source == nil {
		return nil
	}
	result := &workbook.WorkbookSnapshot{
		Path: source.Path, Identity: source.Identity,
		Sheets: make([]*workbook.SheetSnapshot, 0, len(source.Sheets)),
		ByName: make(map[string]*workbook.SheetSnapshot, len(source.ByName)),
	}
	for _, sourceSheet := range source.Sheets {
		sheet := &workbook.SheetSnapshot{
			Name: sourceSheet.Name, Index: sourceSheet.Index,
			MaxRow: sourceSheet.MaxRow, MaxCol: sourceSheet.MaxCol,
			Cells:    make(map[workbook.CellKey]workbook.CellValue, len(sourceSheet.Cells)),
			CellList: append([]workbook.CellKey(nil), sourceSheet.CellList...),
		}
		for key, value := range sourceSheet.Cells {
			sheet.Cells[key] = value
		}
		result.Sheets = append(result.Sheets, sheet)
		result.ByName[sheet.Name] = sheet
	}
	return result
}

func (s *Session) CopyRightToLeft(ref workbook.CellRef) error {
	return s.CopyRightToLeftMany(ref.Sheet, []CellCoordinate{{Row: ref.Row, Col: ref.Col}})
}

func (s *Session) CopyRightToLeftMany(sheet string, cells []CellCoordinate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.copyRightToLeftManyLocked(sheet, cells, "copy")
}

func (s *Session) copyRightToLeftManyLocked(
	sheet string,
	cells []CellCoordinate,
	kind string,
) error {
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
	conflictCells := make(map[int]int)
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
		if s.rowStatusLocked(sheet, cell.Row) == diff.RowConflict {
			conflictCells[cell.Row]++
		}
		ref := workbook.CellRef{Sheet: sheet, Row: cell.Row, Col: cell.Col}
		before, err := merge.Capture(s.leftFile, ref)
		if err != nil {
			return err
		}
		after, err := s.captureRightLocked(ref)
		if err != nil {
			return err
		}
		operations = append(operations, merge.Operation{Ref: ref, Before: before, After: after, Kind: kind})
	}
	if err := s.applyOperationsLocked(sheet, operations); err != nil {
		return err
	}
	conflictRows := make([]int, 0, len(conflictCells))
	for row := range conflictCells {
		conflictRows = append(conflictRows, row)
	}
	sort.Ints(conflictRows)
	if kind == "copy-row" {
		for _, row := range conflictRows {
			s.recordResolutionLocked(RowResolution{
				Sheet: sheet, SourceRow: row, TargetRow: row, Kind: ResolutionOverwriteRow,
			})
		}
	} else {
		for _, row := range conflictRows {
			s.recordResolutionLocked(RowResolution{
				Sheet: sheet, SourceRow: row, TargetRow: row,
				Kind: ResolutionOverwriteCells, CellCount: conflictCells[row],
			})
		}
	}
	return nil
}

func (s *Session) applyOperationsLocked(sheet string, operations []merge.Operation) error {
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
	if err := s.reclassifyRowsLocked(sheet); err != nil {
		s.rollbackCopiesLocked(operations[:appliedCount])
		return err
	}
	s.recordHistoryLocked(operations)
	return nil
}

func (s *Session) CopyRowsRightToLeft(sheet string, rows []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(rows) == 0 {
		return errors.New("no rows selected")
	}
	if s.right == nil || s.rightFile == nil {
		return errors.New("当前没有可用于合并的右侧工作簿")
	}
	leftSheet, rightSheet := s.left.ByName[sheet], s.right.ByName[sheet]
	if leftSheet == nil || rightSheet == nil {
		return fmt.Errorf("worksheet %q must exist on both sides", sheet)
	}
	unique, err := normalizeRows(rows)
	if err != nil {
		return err
	}
	maxCol := max(leftSheet.MaxCol, rightSheet.MaxCol)
	if len(unique)*maxCol > 10000 {
		return errors.New("too many cells selected (maximum 10000)")
	}
	cells := make([]CellCoordinate, 0, len(unique)*maxCol)
	for _, row := range unique {
		for col := 1; col <= maxCol; col++ {
			cells = append(cells, CellCoordinate{Row: row, Col: col})
		}
	}
	return s.copyRightToLeftManyLocked(sheet, cells, "copy-row")
}

func (s *Session) AppendRowsRightToLeft(
	sheet string,
	rows []int,
	requestedIDs []string,
) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.options.ReadonlyLeft {
		return nil, errors.New("left workbook is read-only")
	}
	if s.right == nil || s.rightFile == nil {
		return nil, errors.New("当前没有可用于合并的右侧工作簿")
	}
	leftSheet, rightSheet := s.left.ByName[sheet], s.right.ByName[sheet]
	if leftSheet == nil || rightSheet == nil {
		return nil, fmt.Errorf("worksheet %q must exist on both sides", sheet)
	}
	unique, err := normalizeRows(rows)
	if err != nil {
		return nil, err
	}
	if len(requestedIDs) != 0 && len(requestedIDs) != len(unique) {
		return nil, errors.New("指定 ID 数量必须与所选行数一致")
	}
	sheetDiff := s.sheetDiffLocked(sheet)
	if sheetDiff == nil || sheetDiff.IDColumn == 0 {
		return nil, errors.New("当前工作表首行没有 id 列")
	}
	maxCol := max(leftSheet.MaxCol, rightSheet.MaxCol)
	if len(unique)*maxCol > 10000 {
		return nil, errors.New("too many cells selected (maximum 10000)")
	}
	existingIDs := make(map[string]struct{})
	for row := 2; row <= leftSheet.MaxRow; row++ {
		id := strings.TrimSpace(leftSheet.Cell(row, sheetDiff.IDColumn).Raw)
		if id != "" {
			existingIDs[id] = struct{}{}
		}
	}
	assigned := make([]string, len(unique))
	if len(requestedIDs) == 0 {
		if sheetDiff.NextID <= 0 {
			return nil, errors.New("无法从左侧 id 列计算下一个整数 ID")
		}
		for index := range assigned {
			assigned[index] = strconv.FormatInt(sheetDiff.NextID+int64(index), 10)
		}
	} else {
		for index, id := range requestedIDs {
			assigned[index] = strings.TrimSpace(id)
			if assigned[index] == "" {
				return nil, fmt.Errorf("第 %d 行的指定 ID 不能为空", index+1)
			}
		}
	}
	for _, id := range assigned {
		if _, exists := existingIDs[id]; exists {
			return nil, fmt.Errorf("左侧已存在 ID %s", id)
		}
		existingIDs[id] = struct{}{}
	}

	operations := make([]merge.Operation, 0, len(unique)*maxCol)
	targetStart := leftSheet.MaxRow + 1
	for index, sourceRow := range unique {
		hasSource := false
		for col := 1; col <= maxCol; col++ {
			if rightSheet.Cell(sourceRow, col).Present {
				hasSource = true
				break
			}
		}
		if !hasSource {
			return nil, fmt.Errorf("右侧第 %d 行没有可新增的数据", sourceRow)
		}
		targetRow := targetStart + index
		for col := 1; col <= maxCol; col++ {
			targetRef := workbook.CellRef{Sheet: sheet, Row: targetRow, Col: col}
			before, captureErr := merge.Capture(s.leftFile, targetRef)
			if captureErr != nil {
				return nil, captureErr
			}
			after, captureErr := s.captureRightLocked(workbook.CellRef{Sheet: sheet, Row: sourceRow, Col: col})
			if captureErr != nil {
				return nil, captureErr
			}
			if col == sheetDiff.IDColumn {
				after.Value.Present = true
				after.Value.Raw = assigned[index]
				after.Value.Display = assigned[index]
				after.Value.Formula = ""
				if after.Value.Type == "number" {
					if _, parseErr := strconv.ParseFloat(assigned[index], 64); parseErr != nil {
						after.Value.Type = "string"
					}
				} else {
					after.Value.Type = "string"
				}
			}
			operations = append(operations, merge.Operation{
				Ref: targetRef, Before: before, After: after, Kind: "append-row",
			})
		}
	}
	if err := s.applyOperationsLocked(sheet, operations); err != nil {
		return nil, err
	}
	resolutionKind := ResolutionAppendAuto
	if len(requestedIDs) > 0 {
		resolutionKind = ResolutionAppendSpecified
	}
	for index, sourceRow := range unique {
		s.recordResolutionLocked(RowResolution{
			Sheet: sheet, SourceRow: sourceRow, TargetRow: targetStart + index,
			TargetID: assigned[index], Kind: resolutionKind,
		})
	}
	return assigned, nil
}

func normalizeRows(rows []int) ([]int, error) {
	result := make([]int, 0, len(rows))
	seen := make(map[int]struct{}, len(rows))
	for _, row := range rows {
		if row < 1 {
			return nil, fmt.Errorf("invalid row %d", row)
		}
		if _, exists := seen[row]; exists {
			continue
		}
		seen[row] = struct{}{}
		result = append(result, row)
	}
	return result, nil
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
	if err := s.reclassifyRowsLocked(ref.Sheet); err != nil {
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
	changedSheets := make(map[string]struct{})
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
		changedSheets[command.Ref.Sheet] = struct{}{}
	}
	for sheet := range changedSheets {
		if err := s.reclassifyRowsLocked(sheet); err != nil {
			s.restoreUndoneLocked(undone)
			s.history.PushEntry(entry)
			return err
		}
	}
	s.removeResolutionsForStateLocked(entry.AfterState)
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

// Export writes the current left workbook state to a separate file without
// changing the session save target or marking worktree edits as saved.
func (s *Session) Export(target string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.options.ReadonlyLeft {
		return fmt.Errorf("left workbook is read-only")
	}
	if strings.TrimSpace(target) == "" {
		return errors.New("导出路径不能为空")
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	leftAbs, _ := filepath.Abs(s.left.Path)
	if filepath.Clean(abs) == filepath.Clean(leftAbs) {
		return errors.New("导出路径与当前工作区文件相同，请使用保存到当前工作区")
	}
	_, err = (storage.SafeWriter{}).Save(s.leftFile, abs, nil)
	return err
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

func (s *Session) sheetDiffLocked(name string) *diff.SheetDiff {
	if s.currentDiff == nil {
		return nil
	}
	for _, sheet := range s.currentDiff.Sheets {
		if sheet.Name == name {
			return sheet
		}
	}
	return nil
}

func (s *Session) captureRightLocked(ref workbook.CellRef) (merge.CellState, error) {
	if rows := s.rightRows[ref.Sheet]; rows != nil {
		if physicalRow := rows[ref.Row]; physicalRow > 0 {
			ref.Row = physicalRow
			return merge.Capture(s.rightFile, ref)
		}
		if s.options.GitMerge {
			return merge.CellState{}, nil
		}
	}
	return merge.Capture(s.rightFile, ref)
}

func (s *Session) rowStatusLocked(sheet string, row int) diff.RowStatus {
	item := s.sheetDiffLocked(sheet)
	if item == nil {
		return diff.RowUnchanged
	}
	for _, candidate := range item.Rows {
		if candidate.Row == row {
			return candidate.Status
		}
	}
	return diff.RowUnchanged
}

func (s *Session) recordResolutionLocked(resolution RowResolution) {
	resolution.stateID = s.stateID
	s.resolutions = append(s.resolutions, resolution)
}

func (s *Session) removeResolutionsForStateLocked(stateID uint64) {
	filtered := s.resolutions[:0]
	for _, resolution := range s.resolutions {
		if resolution.stateID != stateID {
			filtered = append(filtered, resolution)
		}
	}
	s.resolutions = filtered
}

func (s *Session) reclassifyRowsLocked(name string) error {
	if s.currentDiff == nil || s.right == nil {
		return nil
	}
	return s.currentDiff.ReclassifyRows(name, s.left.ByName[name], s.right.ByName[name])
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
		if ref.Row == sheet.MaxRow || ref.Col == sheet.MaxCol {
			recalculateSheetBounds(sheet)
		}
	}
	if s.currentDiff == nil || s.right == nil {
		return nil
	}
	return s.currentDiff.UpdateCell(ref, state.Value, s.right.ByName[ref.Sheet].Cell(ref.Row, ref.Col))
}

func recalculateSheetBounds(sheet *workbook.SheetSnapshot) {
	sheet.MaxRow = 0
	sheet.MaxCol = 0
	for _, key := range sheet.CellList {
		sheet.MaxRow = max(sheet.MaxRow, key.Row)
		sheet.MaxCol = max(sheet.MaxCol, key.Col)
	}
}
