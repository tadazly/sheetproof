package app

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/tadazly/sheetproof/internal/diff"
	"github.com/tadazly/sheetproof/internal/history"
	"github.com/tadazly/sheetproof/internal/localization"
	"github.com/tadazly/sheetproof/internal/merge"
	"github.com/tadazly/sheetproof/internal/storage"
	"github.com/tadazly/sheetproof/internal/workbook"
	"github.com/xuri/excelize/v2"
)

type Options struct {
	Locale         string `json:"locale,omitempty"`
	LocaleExplicit bool   `json:"-"`
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
	RowAlignment  RowAlignment       `json:"rowAlignment"`
	MergeNotice   string             `json:"mergeNotice"`
	SelectedSheet string             `json:"selectedSheet"`
}

type RowAlignmentMode string

const (
	RowAlignmentAuto     RowAlignmentMode = "auto"
	RowAlignmentPosition RowAlignmentMode = "position"
)

type RowAlignment struct {
	Mode      RowAlignmentMode             `json:"mode"`
	Available bool                         `json:"available"`
	Applied   bool                         `json:"applied"`
	Moved     int                          `json:"moved"`
	Sheets    map[string]SheetRowAlignment `json:"sheets"`
}

type SheetRowAlignment struct {
	Available bool `json:"available"`
	Applied   bool `json:"applied"`
	Moved     int  `json:"moved"`
	KeyColumn int  `json:"keyColumn"`
}

type ExternalFileChange struct {
	Changed   bool   `json:"changed"`
	Path      string `json:"path"`
	Signature string `json:"signature"`
	Writable  bool   `json:"writable"`
}

type ExternalChanges struct {
	Left  ExternalFileChange `json:"left"`
	Right ExternalFileChange `json:"right"`
}

type ResolutionKind string

const (
	ResolutionOverwriteCells  ResolutionKind = "overwrite-cells"
	ResolutionOverwriteRow    ResolutionKind = "overwrite-row"
	ResolutionAppendRow       ResolutionKind = "append-row"
	ResolutionAppendAuto      ResolutionKind = "append-auto"
	ResolutionAppendSpecified ResolutionKind = "append-specified"
)

func isAppendResolution(kind ResolutionKind) bool {
	return kind == ResolutionAppendRow || kind == ResolutionAppendAuto || kind == ResolutionAppendSpecified
}

type RowResolution struct {
	Sheet           string         `json:"sheet"`
	SourceRow       int            `json:"sourceRow"`
	TargetRow       int            `json:"targetRow,omitempty"`
	TargetSourceRow int            `json:"targetSourceRow,omitempty"`
	TargetID        string         `json:"targetId,omitempty"`
	Kind            ResolutionKind `json:"kind"`
	CellCount       int            `json:"cellCount,omitempty"`
	stateID         uint64
}

type RegionCell struct {
	Row          int                `json:"row"`
	SourceRow    int                `json:"sourceRow"`
	LeftRow      int                `json:"leftRow"`
	RightRow     int                `json:"rightRow"`
	Col          int                `json:"col"`
	LeftCol      int                `json:"leftCol"`
	RightCol     int                `json:"rightCol"`
	Axis         string             `json:"axis"`
	Left         workbook.CellValue `json:"left"`
	OriginalLeft workbook.CellValue `json:"originalLeft"`
	Right        workbook.CellValue `json:"right"`
	Status       diff.CellStatus    `json:"status"`
	RowStatus    diff.RowStatus     `json:"rowStatus"`
	LeftMatch    bool               `json:"leftMatch,omitempty"`
	RightMatch   bool               `json:"rightMatch,omitempty"`
}

type RegionColumn struct {
	Col      int `json:"col"`
	LeftCol  int `json:"leftCol"`
	RightCol int `json:"rightCol"`
}

type Region struct {
	Sheet     string         `json:"sheet"`
	FromRow   int            `json:"fromRow"`
	ToRow     int            `json:"toRow"`
	FromCol   int            `json:"fromCol"`
	ToCol     int            `json:"toCol"`
	Filtered  bool           `json:"filtered"`
	TotalRows int            `json:"totalRows"`
	Columns   []RegionColumn `json:"columns"`
	Cells     []RegionCell   `json:"cells"`
}

type regionRow struct {
	display int
	source  int
	left    int
	right   int
}

type CellCoordinate struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

const maxClipboardCells = 10000

type Session struct {
	mu                     sync.RWMutex
	searchMu               sync.Mutex
	leftFile               *excelize.File
	rightFile              *excelize.File
	left                   *workbook.WorkbookSnapshot
	comparisonLeft         *workbook.WorkbookSnapshot
	originalLeft           *workbook.WorkbookSnapshot
	right                  *workbook.WorkbookSnapshot
	rightSource            *workbook.WorkbookSnapshot
	leftRows               map[string]map[int]int
	rightRows              map[string]map[int]int
	leftCols               map[string]map[int]int
	rightCols              map[string]map[int]int
	columnAligned          map[string]bool
	keyColumns             map[string]int
	alignedSheets          map[string]bool
	currentDiff            *diff.WorkbookDiff
	history                history.Stack
	options                Options
	dirty                  bool
	warnings               []string
	alignmentNotice        string
	alignmentMode          RowAlignmentMode
	alignmentMoved         int
	alignmentAvailable     bool
	alignmentSheetMoved    map[string]int
	alignmentSheetEligible map[string]bool
	mergeNotice            string
	stateID                uint64
	savedState             uint64
	nextState              uint64
	resolutions            []RowResolution
	dataGeneration         uint64
	searches               map[SearchSide]*searchCache
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
	alignment := prepareRightForComparison(left, rightSource, options.GitMerge, RowAlignmentAuto, nil)
	right, rightRows := alignment.right, alignment.rowSources
	currentDiff, err := compareContextWithKeyColumns(ctx, alignment.left, right, alignment.keyColumns)
	if err != nil {
		_ = rightFile.Close()
		_ = leftFile.Close()
		return nil, err
	}
	annotateDiffRowSources(currentDiff, alignment.leftSources, alignment.rowSources)
	mergeNotice := ""
	if options.GitMerge && strings.TrimSpace(options.MergeBase) != "" {
		baseFile, base, baseErr := reader.OpenContext(ctx, options.MergeBase)
		if baseErr != nil {
			_ = rightFile.Close()
			_ = leftFile.Close()
			return nil, baseErr
		}
		_ = baseFile.Close()
		mergeNotice = mergeSemanticNotice(base, left, rightSource, options.Locale)
	}
	options = defaultOptions(options)
	return &Session{
		leftFile: leftFile, rightFile: rightFile,
		left: left, comparisonLeft: alignment.left, originalLeft: cloneWorkbookSnapshot(left),
		right: right, rightSource: rightSource,
		leftRows: alignment.leftSources, rightRows: rightRows,
		leftCols: alignment.leftColumns, rightCols: alignment.rightColumns,
		columnAligned: alignment.columnAligned, keyColumns: make(map[string]int),
		alignedSheets: alignment.alignedSheets,
		currentDiff:   currentDiff, alignmentNotice: alignmentWarning(options.Locale, alignment.moved),
		alignmentMode: RowAlignmentAuto, alignmentMoved: alignment.moved,
		alignmentAvailable:  alignment.available,
		alignmentSheetMoved: alignment.sheetMoved, alignmentSheetEligible: alignment.sheetEligible,
		mergeNotice: mergeNotice,
		options:     options, stateID: 1, savedState: 1, nextState: 2,
		dataGeneration: 1, searches: make(map[SearchSide]*searchCache),
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

func compareContextWithKeyColumns(
	ctx context.Context,
	left, right *workbook.WorkbookSnapshot,
	keyColumns map[string]int,
) (*diff.WorkbookDiff, error) {
	if ctx.Done() == nil {
		return diff.CompareWithKeyColumns(left, right, keyColumns), nil
	}
	result := make(chan *diff.WorkbookDiff, 1)
	go func() {
		result <- diff.CompareWithKeyColumns(left, right, keyColumns)
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
		leftFile: leftFile, left: left, comparisonLeft: left,
		originalLeft: cloneWorkbookSnapshot(left), options: options,
		leftRows: identityWorkbookRows(left), leftCols: identityWorkbookColumns(left), keyColumns: make(map[string]int),
		alignmentMode: RowAlignmentAuto,
		stateID:       1, savedState: 1, nextState: 2,
		dataGeneration: 1, searches: make(map[SearchSide]*searchCache),
	}, nil
}

func defaultOptions(options Options) Options {
	locale := localization.Normalize(options.Locale)
	if options.LeftLabel == "" {
		switch locale {
		case localization.SimplifiedChinese:
			options.LeftLabel = "本地（可编辑）"
		case localization.Japanese:
			options.LeftLabel = "ローカル（編集可能）"
		default:
			options.LeftLabel = "Local (editable)"
		}
	}
	if options.RightLabel == "" {
		switch locale {
		case localization.SimplifiedChinese:
			options.RightLabel = "对比来源（只读）"
		case localization.Japanese:
			options.RightLabel = "比較元（読み取り専用）"
		default:
			options.RightLabel = "Comparison source (read-only)"
		}
	}
	return options
}

func alignmentWarning(locale string, moved int) string {
	if moved == 0 {
		return ""
	}
	switch localization.Normalize(locale) {
	case localization.SimplifiedChinese:
		return fmt.Sprintf("检测到 %d 条同 ID 记录位于不同物理行。已按唯一 ID 对齐，避免将插入或删除显示为连续修改。", moved)
	case localization.Japanese:
		return fmt.Sprintf("同じ ID のレコード %d 件が異なる物理行にあります。挿入や削除を連続した変更として扱わないよう、一意の ID で揃えて表示しています。", moved)
	default:
		return fmt.Sprintf("%d records with the same ID are on different physical rows. They are aligned by unique ID so an insertion or deletion does not appear as a series of changes.", moved)
	}
}

func sessionText(locale, english, chinese, japanese string) string {
	switch localization.Normalize(locale) {
	case localization.SimplifiedChinese:
		return chinese
	case localization.Japanese:
		return japanese
	default:
		return english
	}
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
	alignment := prepareRightForComparison(
		s.left, rightSource, s.options.GitMerge, s.alignmentMode, s.keyColumns,
	)
	s.rightFile = rightFile
	s.comparisonLeft = alignment.left
	s.right = alignment.right
	s.rightSource = rightSource
	s.leftRows = alignment.leftSources
	s.rightRows = alignment.rowSources
	s.leftCols = alignment.leftColumns
	s.rightCols = alignment.rightColumns
	s.columnAligned = alignment.columnAligned
	s.alignedSheets = alignment.alignedSheets
	s.alignmentNotice = alignmentWarning(s.options.Locale, alignment.moved)
	s.alignmentMoved = alignment.moved
	s.alignmentAvailable = alignment.available
	s.alignmentSheetMoved = alignment.sheetMoved
	s.alignmentSheetEligible = alignment.sheetEligible
	s.currentDiff = diff.CompareWithKeyColumns(alignment.left, alignment.right, alignment.keyColumns)
	annotateDiffRowSources(s.currentDiff, alignment.leftSources, alignment.rowSources)
	s.resolutions = nil
	s.dataGeneration++
	if rightLabel != "" {
		s.options.RightLabel = rightLabel
	}
	s.mu.Unlock()
	if old != nil {
		return old.Close()
	}
	return nil
}

// ExternalChanges checks whether either source file has changed since it was
// loaded. A quick size/mtime comparison avoids hashing unchanged workbooks.
func (s *Session) ExternalChanges() (ExternalChanges, error) {
	s.mu.RLock()
	leftPath, leftExpected := s.left.Path, s.left.Identity
	readonlyLeft := s.options.ReadonlyLeft
	rightPath := ""
	rightExpected := workbook.FileIdentity{}
	if s.rightSource != nil {
		rightPath, rightExpected = s.rightSource.Path, s.rightSource.Identity
	}
	s.mu.RUnlock()

	leftChanged, leftCurrent, err := externalFileChanged(leftPath, leftExpected)
	if err != nil {
		return ExternalChanges{}, err
	}
	result := ExternalChanges{
		Left: ExternalFileChange{
			Changed: leftChanged, Path: leftPath,
			Signature: externalIdentitySignature(leftCurrent), Writable: !readonlyLeft,
		},
	}
	if rightPath != "" {
		rightChanged, rightCurrent, rightErr := externalFileChanged(rightPath, rightExpected)
		if rightErr != nil {
			return ExternalChanges{}, rightErr
		}
		result.Right = ExternalFileChange{
			Changed: rightChanged, Path: rightPath,
			Signature: externalIdentitySignature(rightCurrent), Writable: false,
		}
	}
	return result, nil
}

func externalFileChanged(path string, expected workbook.FileIdentity) (bool, workbook.FileIdentity, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, workbook.FileIdentity{}, err
	}
	if info.Size() == expected.Size && info.ModTime().UnixNano() == expected.ModTime {
		return false, expected, nil
	}
	current, err := workbook.Identity(path)
	if err != nil {
		return false, workbook.FileIdentity{}, err
	}
	return current.SHA256 != expected.SHA256, current, nil
}

func externalIdentitySignature(identity workbook.FileIdentity) string {
	if identity.SHA256 == "" {
		return ""
	}
	return fmt.Sprintf("%d:%d:%s", identity.Size, identity.ModTime, identity.SHA256)
}

// ReloadLeft replaces the in-memory editable source with the latest file on
// disk. It intentionally resets edit history because the previous operations
// were based on a different workbook identity.
func (s *Session) ReloadLeft() error {
	s.mu.Lock()
	reader := workbook.Reader{}
	leftFile, left, err := reader.Open(s.left.Path)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	alignment := prepareRightForComparison(
		left, s.rightSource, s.options.GitMerge, s.alignmentMode, s.keyColumns,
	)
	right := alignment.right
	var currentDiff *diff.WorkbookDiff
	if right != nil {
		currentDiff = diff.CompareWithKeyColumns(alignment.left, right, alignment.keyColumns)
		annotateDiffRowSources(currentDiff, alignment.leftSources, alignment.rowSources)
	}
	mergeNotice := s.mergeNotice
	if s.options.GitMerge && strings.TrimSpace(s.options.MergeBase) != "" && s.rightSource != nil {
		baseFile, base, baseErr := reader.Open(s.options.MergeBase)
		if baseErr != nil {
			_ = leftFile.Close()
			s.mu.Unlock()
			return baseErr
		}
		_ = baseFile.Close()
		mergeNotice = mergeSemanticNotice(base, left, s.rightSource, s.options.Locale)
	}
	old := s.leftFile
	s.leftFile = leftFile
	s.left = left
	s.comparisonLeft = alignment.left
	s.originalLeft = cloneWorkbookSnapshot(left)
	s.right = right
	s.leftRows = alignment.leftSources
	s.rightRows = alignment.rowSources
	s.leftCols = alignment.leftColumns
	s.rightCols = alignment.rightColumns
	s.columnAligned = alignment.columnAligned
	s.alignedSheets = alignment.alignedSheets
	s.currentDiff = currentDiff
	s.history.Clear()
	s.dirty = false
	s.warnings = nil
	s.alignmentNotice = alignmentWarning(s.options.Locale, alignment.moved)
	s.alignmentMoved = alignment.moved
	s.alignmentAvailable = alignment.available
	s.alignmentSheetMoved = alignment.sheetMoved
	s.alignmentSheetEligible = alignment.sheetEligible
	s.mergeNotice = mergeNotice
	s.stateID = 1
	s.savedState = 1
	s.nextState = 2
	s.resolutions = nil
	s.dataGeneration++
	s.mu.Unlock()
	if old != nil {
		return old.Close()
	}
	return nil
}

func (s *Session) ReloadRight() error {
	s.mu.RLock()
	if s.rightSource == nil {
		locale := s.options.Locale
		s.mu.RUnlock()
		return errors.New(sessionText(locale, "There is no workbook to reload on the right.", "当前没有可重新载入的右侧工作簿。", "右側に再読み込みできるブックがありません。"))
	}
	path, label := s.rightSource.Path, s.options.RightLabel
	s.mu.RUnlock()
	return s.ReplaceRight(path, label)
}

// DetachRight keeps the editable workbook open while explicitly representing
// that there is no right-side workbook to compare.
func (s *Session) DetachRight(rightLabel string) error {
	s.mu.Lock()
	old := s.rightFile
	s.rightFile = nil
	s.comparisonLeft = s.left
	s.right = nil
	s.rightSource = nil
	s.leftRows = identityWorkbookRows(s.left)
	s.rightRows = nil
	s.leftCols = identityWorkbookColumns(s.left)
	s.rightCols = nil
	s.columnAligned = nil
	s.alignedSheets = nil
	s.currentDiff = nil
	s.alignmentNotice = ""
	s.alignmentMoved = 0
	s.alignmentAvailable = false
	s.alignmentSheetMoved = nil
	s.alignmentSheetEligible = nil
	s.resolutions = nil
	s.dataGeneration++
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
	warnings := make([]string, 0, len(s.warnings)+1)
	if s.alignmentNotice != "" {
		warnings = append(warnings, s.alignmentNotice)
	}
	warnings = append(warnings, s.warnings...)
	sheetAlignments := make(map[string]SheetRowAlignment, len(presentation.Sheets))
	for _, sheet := range presentation.Sheets {
		sheetAlignments[sheet.Name] = SheetRowAlignment{
			Available: s.alignmentSheetEligible[sheet.Name],
			Applied:   s.alignedSheets[sheet.Name],
			Moved:     s.alignmentSheetMoved[sheet.Name],
			KeyColumn: sheet.IDColumn,
		}
	}
	return Summary{
		Options: s.options, Diff: compactDiff(presentation),
		Resolutions: append([]RowResolution{}, s.resolutions...),
		Dirty:       s.dirty, UndoCount: s.history.Len(),
		Warnings: warnings,
		RowAlignment: RowAlignment{
			Mode: s.alignmentMode, Available: s.alignmentAvailable,
			Applied: rowAlignmentApplied(s.alignedSheets), Moved: s.alignmentMoved,
			Sheets: sheetAlignments,
		},
		MergeNotice: s.mergeNotice, SelectedSheet: selected,
	}
}

func rowAlignmentApplied(sheets map[string]bool) bool {
	for _, applied := range sheets {
		if applied {
			return true
		}
	}
	return false
}

func (s *Session) SetRowAlignment(mode RowAlignmentMode) error {
	if mode != RowAlignmentAuto && mode != RowAlignmentPosition {
		return fmt.Errorf("%s: %s", sessionText(s.options.Locale, "Unknown row alignment mode", "未知的行对齐方式", "不明な行の比較方法"), mode)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.options.GitMerge {
		return errors.New(sessionText(s.options.Locale, "Git merge sessions always use ID alignment.", "Git 合并会话固定使用 ID 对齐。", "Git マージセッションでは常に ID で揃えて比較します。"))
	}
	if s.history.Len() > 0 {
		return errors.New(sessionText(s.options.Locale, "Undo all edits and merge actions before changing row alignment.", "请先撤销所有编辑和合并操作，再切换行对齐方式。", "行の比較方法を変える前に、編集と反映操作をすべて元に戻してください。"))
	}
	if s.alignmentMode == mode {
		return nil
	}
	s.alignmentMode = mode
	s.rebuildComparisonLocked()
	return nil
}

// SetKeyColumn overrides automatic record-key detection for one worksheet.
// Column zero explicitly disables key alignment. Changing the key can remap
// logical rows, so it is allowed only before edit/merge history exists.
func (s *Session) SetKeyColumn(sheet string, column int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	leftSheet := s.left.ByName[sheet]
	var rightSheet *workbook.SheetSnapshot
	if s.rightSource != nil {
		rightSheet = s.rightSource.ByName[sheet]
	}
	if leftSheet == nil && rightSheet == nil {
		return fmt.Errorf("worksheet %q not found", sheet)
	}
	maxCol := max(sheetColumnCount(leftSheet), sheetColumnCount(rightSheet))
	if column < 0 || column > maxCol {
		return fmt.Errorf("invalid key column %d for worksheet %q", column, sheet)
	}
	if s.history.Len() > 0 {
		return errors.New(sessionText(s.options.Locale, "Undo all edits and merge actions before changing the key column.", "请先撤销所有编辑和合并操作，再更改主键列。", "キー列を変える前に、編集と反映操作をすべて元に戻してください。"))
	}
	if s.keyColumns == nil {
		s.keyColumns = make(map[string]int)
	}
	s.keyColumns[sheet] = column
	if column > 0 && !s.options.GitMerge {
		s.alignmentMode = RowAlignmentAuto
	}
	s.rebuildComparisonLocked()
	return nil
}

func sheetColumnCount(sheet *workbook.SheetSnapshot) int {
	if sheet == nil {
		return 0
	}
	return sheet.MaxCol
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
	rows := make([]regionRow, 0, rowCount)
	leftMax, rightMax := 0, 0
	if leftSheet := s.left.ByName[sheet]; leftSheet != nil {
		leftMax = leftSheet.MaxRow
	}
	if s.rightSource != nil {
		if rightSheet := s.rightSource.ByName[sheet]; rightSheet != nil {
			rightMax = rightSheet.MaxRow
		}
	}
	for row := fromRow; row < fromRow+rowCount; row++ {
		rows = append(rows, regionRow{
			display: row, source: row,
			left:  physicalRowForLogical(s.leftRows[sheet], row, leftMax),
			right: physicalRowForLogical(s.rightRows[sheet], row, rightMax),
		})
	}
	region, err := s.regionForRowsLocked(sheet, rows, fromCol, colCount)
	if err != nil {
		return Region{}, err
	}
	region.FromRow = fromRow
	region.ToRow = fromRow + rowCount - 1
	return region, nil
}

// PinnedRegion reads arbitrary logical rows and columns for the frontend's
// pinned row/column overlays without widening the normal viewport Region.
func (s *Session) PinnedRegion(sheet string, rows, columns []int) (Region, error) {
	if len(rows) == 0 || len(columns) == 0 {
		return Region{Sheet: sheet}, nil
	}
	if len(rows)*len(columns) > maxClipboardCells {
		return Region{}, fmt.Errorf("invalid pinned region (maximum %d cells)", maxClipboardCells)
	}
	rows = sortedPositiveCoordinates(rows)
	columns = sortedPositiveCoordinates(columns)
	s.mu.RLock()
	defer s.mu.RUnlock()
	leftSheet := s.left.ByName[sheet]
	var rightSheet *workbook.SheetSnapshot
	if s.rightSource != nil {
		rightSheet = s.rightSource.ByName[sheet]
	}
	if leftSheet == nil && rightSheet == nil {
		return Region{}, fmt.Errorf("worksheet %q not found", sheet)
	}
	leftMax, rightMax := 0, 0
	if leftSheet != nil {
		leftMax = leftSheet.MaxRow
	}
	if rightSheet != nil {
		rightMax = rightSheet.MaxRow
	}
	mapped := make([]regionRow, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, regionRow{
			display: row,
			source:  row,
			left:    physicalRowForLogical(s.leftRows[sheet], row, leftMax),
			right:   physicalRowForLogical(s.rightRows[sheet], row, rightMax),
		})
	}
	return s.regionForRowsAndColumnsLocked(sheet, mapped, columns)
}

func sortedPositiveCoordinates(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value < 1 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Ints(result)
	return result
}

// FilteredRegion returns a packed viewport containing only rows whose semantic
// classification matches one of statuses. Display row numbers are dense while
// SourceRow/LeftRow/RightRow retain the physical coordinates used by edits and
// merge operations. When a conflict source row was appended with a new ID, the
// left side is paired with that appended target row so the filtered view shows
// the actual merge result beside its right-side source.
func (s *Session) FilteredRegion(
	sheet string,
	statuses []diff.RowStatus,
	fromRow, rowCount, fromCol, colCount int,
) (Region, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if fromRow < 1 || fromCol < 1 || rowCount < 1 || colCount < 1 || rowCount > 300 || colCount > 100 {
		return Region{}, fmt.Errorf("invalid filtered region (maximum 300 rows by 100 columns)")
	}
	allRows, err := s.filteredRowsLocked(sheet, statuses)
	if err != nil {
		return Region{}, err
	}
	windowStart := fromRow - 1
	windowRows := make([]regionRow, 0, rowCount)
	if windowStart < len(allRows) {
		end := min(windowStart+rowCount, len(allRows))
		windowRows = append(windowRows, allRows[windowStart:end]...)
	}

	region, err := s.regionForRowsLocked(sheet, windowRows, fromCol, colCount, statuses)
	if err != nil {
		return Region{}, err
	}
	region.FromRow = fromRow
	region.ToRow = fromRow + len(windowRows) - 1
	region.Filtered = true
	region.TotalRows = len(allRows)
	return region, nil
}

func physicalRowForLogical(rows map[int]int, logical, physicalMax int) int {
	if physical := rows[logical]; physical > 0 {
		return physical
	}
	maxLogical := 0
	for candidate := range rows {
		maxLogical = max(maxLogical, candidate)
	}
	if logical > maxLogical {
		return physicalMax + logical - maxLogical
	}
	return 0
}

func (s *Session) regionForRowsLocked(
	sheet string,
	rows []regionRow,
	fromCol, colCount int,
	filters ...[]diff.RowStatus,
) (Region, error) {
	columns := make([]int, 0, colCount)
	for col := fromCol; col < fromCol+colCount; col++ {
		columns = append(columns, col)
	}
	return s.regionForRowsAndColumnsLocked(sheet, rows, columns, filters...)
}

func (s *Session) regionForRowsAndColumnsLocked(
	sheet string,
	rows []regionRow,
	columns []int,
	filters ...[]diff.RowStatus,
) (Region, error) {
	leftSheet := s.left.ByName[sheet]
	originalLeftSheet := s.originalLeft.ByName[sheet]
	var rightSheet *workbook.SheetSnapshot
	if s.rightSource != nil {
		rightSheet = s.rightSource.ByName[sheet]
	}
	if leftSheet == nil && rightSheet == nil {
		return Region{}, fmt.Errorf("worksheet %q not found", sheet)
	}
	statuses := make(map[workbook.CellKey]diff.CellStatus, len(rows)*len(columns))
	rowStatuses := make(map[int]diff.RowStatus, len(rows))
	fromCol, toCol := 0, 0
	if len(columns) > 0 {
		fromCol, toCol = columns[0], columns[0]
		for _, col := range columns[1:] {
			fromCol = min(fromCol, col)
			toCol = max(toCol, col)
		}
	}
	if s.currentDiff != nil {
		for _, sheetDiff := range s.currentDiff.Sheets {
			if sheetDiff.Name == sheet {
				collectRegionStatuses(sheetDiff, rows, fromCol, toCol, statuses, rowStatuses)
				break
			}
		}
	}
	region := Region{
		Sheet:   sheet,
		FromCol: fromCol, ToCol: toCol,
		Columns: make([]RegionColumn, 0, len(columns)),
		Cells:   make([]RegionCell, 0, len(rows)*len(columns)),
	}
	for _, col := range columns {
		region.Columns = append(region.Columns, RegionColumn{
			Col: col, LeftCol: physicalColumnForLogical(s.leftCols[sheet], col),
			RightCol: physicalColumnForLogical(s.rightCols[sheet], col),
		})
	}
	var activeFilters []diff.RowStatus
	if len(filters) > 0 {
		activeFilters = filters[0]
	}
	leftMatches := s.searchMatchesLocked(SearchLeft, sheet, activeFilters)
	rightMatches := s.searchMatchesLocked(SearchRight, sheet, activeFilters)
	for _, row := range rows {
		for _, col := range columns {
			leftCol := physicalColumnForLogical(s.leftCols[sheet], col)
			rightCol := physicalColumnForLogical(s.rightCols[sheet], col)
			ref := workbook.CellRef{Sheet: sheet, Row: row.source, Col: col}
			key := workbook.CellKey{Row: row.source, Col: col}
			status := statuses[key]
			if status == "" {
				status = diff.Unchanged
			}
			rowStatus := rowStatuses[row.source]
			if rowStatus == "" {
				rowStatus = diff.RowUnchanged
			}
			cell := RegionCell{
				Row: row.display, SourceRow: row.source, LeftRow: row.left, RightRow: row.right,
				Col: col, LeftCol: leftCol, RightCol: rightCol, Axis: ref.Axis(),
				Left: leftSheet.Cell(row.left, leftCol), OriginalLeft: originalLeftSheet.Cell(row.left, leftCol),
				Right:  rightSheet.Cell(row.right, rightCol),
				Status: status, RowStatus: rowStatus,
			}
			cell.LeftMatch = searchMatchAt(leftMatches, row.source, col)
			cell.RightMatch = searchMatchAt(rightMatches, row.source, col)
			region.Cells = append(region.Cells, cell)
		}
	}
	if len(rows) > 0 {
		region.FromRow = rows[0].display
		region.ToRow = rows[len(rows)-1].display
	}
	return region, nil
}

func physicalColumnForLogical(columns map[int]int, logical int) int {
	if physical := columns[logical]; physical > 0 {
		return physical
	}
	return 0
}

// collectRegionStatuses reads only the sparse diff entries that intersect the
// requested viewport. Region requests run continuously while scrolling, so
// rebuilding maps from every difference in a large sheet on each request would
// make viewport cost proportional to the entire diff instead of its 48×20
// window.
func collectRegionStatuses(
	sheetDiff *diff.SheetDiff,
	rows []regionRow,
	fromCol, toCol int,
	statuses map[workbook.CellKey]diff.CellStatus,
	rowStatuses map[int]diff.RowStatus,
) {
	for _, row := range rows {
		cellIndex := sort.Search(len(sheetDiff.Differences), func(index int) bool {
			ref := sheetDiff.Differences[index].Ref
			return ref.Row > row.source || (ref.Row == row.source && ref.Col >= fromCol)
		})
		for cellIndex < len(sheetDiff.Differences) {
			cellDiff := sheetDiff.Differences[cellIndex]
			if cellDiff.Ref.Row != row.source || cellDiff.Ref.Col > toCol {
				break
			}
			statuses[workbook.CellKey{Row: cellDiff.Ref.Row, Col: cellDiff.Ref.Col}] = cellDiff.Status
			cellIndex++
		}

		rowIndex := sort.Search(len(sheetDiff.Rows), func(index int) bool {
			return sheetDiff.Rows[index].Row >= row.source
		})
		if rowIndex < len(sheetDiff.Rows) && sheetDiff.Rows[rowIndex].Row == row.source {
			rowStatuses[row.source] = sheetDiff.Rows[rowIndex].Status
		}
	}
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
		return fmt.Errorf("%s", sessionText(s.options.Locale, "The workbook on the left is read-only.", "左侧工作簿为只读，无法修改。", "左側のブックは読み取り専用のため編集できません。"))
	}
	if s.right == nil || s.rightFile == nil {
		return fmt.Errorf("%s", sessionText(s.options.Locale, "There is no workbook on the right to copy from.", "当前没有可供复制的右侧工作簿。", "右側に反映元となるブックがありません。"))
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
		if kind != "copy-row" {
			leftValue := s.comparisonLeft.ByName[sheet].Cell(cell.Row, cell.Col)
			rightValue := s.right.ByName[sheet].Cell(cell.Row, cell.Col)
			if leftValue.Equal(rightValue) {
				continue
			}
		}
		if s.rowStatusLocked(sheet, cell.Row) == diff.RowConflict {
			conflictCells[cell.Row]++
		}
		sourceRef := workbook.CellRef{Sheet: sheet, Row: cell.Row, Col: cell.Col}
		targetRef := sourceRef
		targetRef.Row = s.ensureLeftPhysicalRowLocked(sheet, cell.Row)
		targetRef.Col = physicalColumnForLogical(s.leftCols[sheet], cell.Col)
		if targetRef.Col == 0 {
			return errors.New(sessionText(s.options.Locale,
				"This column does not exist on the left. Copy the whole column first.",
				"左侧尚无此列，请先复制整列到左侧。",
				"左側にこの列がありません。先に列全体を左側へ反映してください。"))
		}
		before, err := merge.Capture(s.leftFile, targetRef)
		if err != nil {
			return err
		}
		after, err := s.captureRightLocked(sourceRef)
		if err != nil {
			return err
		}
		operations = append(operations, merge.Operation{Ref: targetRef, Before: before, After: after, Kind: kind})
	}
	if len(operations) == 0 {
		return nil
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
				Sheet: sheet, SourceRow: row,
				TargetRow: s.leftRows[sheet][row], Kind: ResolutionOverwriteRow,
			})
		}
	} else {
		for _, row := range conflictRows {
			s.recordResolutionLocked(RowResolution{
				Sheet: sheet, SourceRow: row, TargetRow: s.leftRows[sheet][row],
				Kind: ResolutionOverwriteCells, CellCount: conflictCells[row],
			})
		}
	}
	return nil
}

func (s *Session) ensureLeftPhysicalRowLocked(sheet string, logical int) int {
	if physical := s.leftRows[sheet][logical]; physical > 0 {
		return physical
	}
	leftSheet := s.left.ByName[sheet]
	target := leftSheet.MaxRow + 1
	for _, physical := range s.leftRows[sheet] {
		target = max(target, physical+1)
	}
	if s.leftRows[sheet] == nil {
		s.leftRows[sheet] = make(map[int]int)
	}
	s.leftRows[sheet][logical] = target
	return target
}

func (s *Session) applyOperationsLocked(sheet string, operations []merge.Operation) error {
	appliedCount := 0
	for index, operation := range operations {
		warnings, err := merge.Apply(s.leftFile, operation.Ref, operation.After)
		if err != nil {
			// Apply can fail after changing the cell value but before finishing
			// style/comment/link metadata. Include the current operation in the
			// rollback so one malformed source cell cannot poison later actions.
			s.rollbackCopiesLocked(operations[:index+1])
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
	rebuild := false
	for _, operation := range operations {
		if operation.Ref.Row == 1 {
			rebuild = true
			break
		}
	}
	if rebuild {
		s.rebuildComparisonLocked()
	} else if err := s.reclassifyRowsLocked(sheet); err != nil {
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
		return errors.New(sessionText(s.options.Locale, "There is no workbook on the right to copy from.", "当前没有可供复制的右侧工作簿。", "右側に反映元となるブックがありません。"))
	}
	leftSheet, rightSheet := s.left.ByName[sheet], s.right.ByName[sheet]
	if leftSheet == nil || rightSheet == nil {
		return fmt.Errorf("worksheet %q must exist on both sides", sheet)
	}
	unique, err := normalizeRows(rows)
	if err != nil {
		return err
	}
	maxCol := max(s.comparisonLeft.ByName[sheet].MaxCol, s.right.ByName[sheet].MaxCol)
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

// CopyColumnsRightToLeft copies complete logical comparison columns. If a
// selected source column only exists on the right, a real column is inserted
// into the editable workbook at the corresponding neighboring position. The
// insertion and all copied cells form one undo step.
func (s *Session) CopyColumnsRightToLeft(sheet string, columns []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.options.ReadonlyLeft {
		return errors.New(sessionText(s.options.Locale, "The workbook on the left is read-only.", "左侧工作簿为只读，无法修改。", "左側のブックは読み取り専用のため編集できません。"))
	}
	if s.rightSource == nil || s.rightFile == nil {
		return errors.New(sessionText(s.options.Locale, "There is no workbook on the right to copy from.", "当前没有可供复制的右侧工作簿。", "右側に反映元となるブックがありません。"))
	}
	leftSheet, rightSheet := s.left.ByName[sheet], s.rightSource.ByName[sheet]
	if leftSheet == nil || rightSheet == nil {
		return fmt.Errorf("worksheet %q must exist on both sides", sheet)
	}
	unique := make([]int, 0, len(columns))
	seen := make(map[int]struct{}, len(columns))
	logicalMaxCol := max(s.comparisonLeft.ByName[sheet].MaxCol, s.right.ByName[sheet].MaxCol)
	for _, column := range columns {
		if column < 1 || column > logicalMaxCol {
			return fmt.Errorf("invalid column %d", column)
		}
		if _, exists := seen[column]; exists {
			continue
		}
		seen[column] = struct{}{}
		unique = append(unique, column)
	}
	if len(unique) == 0 {
		return errors.New("no columns selected")
	}
	sort.Ints(unique)
	maxRow := max(s.comparisonLeft.ByName[sheet].MaxRow, s.right.ByName[sheet].MaxRow)
	if len(unique)*maxRow > 10000 {
		return errors.New(sessionText(s.options.Locale,
			"Too many cells selected for a whole-column copy (maximum 10000).",
			"整列复制的单元格过多（最多 10000 个）。",
			"列全体の反映対象が多すぎます（最大 10000 セル）。"))
	}

	insertions := make([]history.ColumnInsertion, 0)
	operations := make([]merge.Operation, 0, len(unique)*maxRow)
	for _, logicalCol := range unique {
		if physicalColumnForLogical(s.rightCols[sheet], logicalCol) == 0 {
			continue
		}
		leftCol := physicalColumnForLogical(s.leftCols[sheet], logicalCol)
		if leftCol == 0 {
			leftCol = s.insertionColumnForLogicalLocked(sheet, logicalCol)
			name, err := excelize.ColumnNumberToName(leftCol)
			if err != nil {
				s.rollbackColumnCopyLocked(operations, insertions)
				return err
			}
			if err := s.leftFile.InsertCols(sheet, name, 1); err != nil {
				s.rollbackColumnCopyLocked(operations, insertions)
				return err
			}
			insertSnapshotColumn(s.left.ByName[sheet], leftCol)
			insertSnapshotColumn(s.originalLeft.ByName[sheet], leftCol)
			shiftColumnSourcesForInsert(s.leftCols[sheet], leftCol)
			s.leftCols[sheet][logicalCol] = leftCol
			insertions = append(insertions, history.ColumnInsertion{Sheet: sheet, Column: leftCol})
		}
		for logicalRow := 1; logicalRow <= maxRow; logicalRow++ {
			targetRow := s.ensureLeftPhysicalRowLocked(sheet, logicalRow)
			targetRef := workbook.CellRef{Sheet: sheet, Row: targetRow, Col: leftCol}
			before, err := merge.Capture(s.leftFile, targetRef)
			if err != nil {
				s.rollbackColumnCopyLocked(operations, insertions)
				return err
			}
			after, err := s.captureRightLocked(workbook.CellRef{Sheet: sheet, Row: logicalRow, Col: logicalCol})
			if err != nil {
				s.rollbackColumnCopyLocked(operations, insertions)
				return err
			}
			operations = append(operations, merge.Operation{
				Ref: targetRef, Before: before, After: after, Kind: "copy-column",
			})
		}
	}
	if len(operations) == 0 {
		return errors.New(sessionText(s.options.Locale, "The selected columns do not exist on the right.", "右侧不存在所选列。", "選択した列は右側にありません。"))
	}
	for index, operation := range operations {
		warnings, err := merge.Apply(s.leftFile, operation.Ref, operation.After)
		if err != nil {
			s.rollbackColumnCopyLocked(operations[:index+1], insertions)
			return fmt.Errorf("copy column %s failed: %w", operation.Ref.Axis(), err)
		}
		applied, err := merge.Capture(s.leftFile, operation.Ref)
		if err != nil {
			s.rollbackColumnCopyLocked(operations[:index+1], insertions)
			return err
		}
		s.warnings = append(s.warnings, warnings...)
		updateSnapshotCell(s.left.ByName[sheet], workbook.CellKey{Row: operation.Ref.Row, Col: operation.Ref.Col}, applied.Value)
	}
	s.rebuildComparisonLocked()
	s.recordHistoryEntryLocked(operations, insertions, nil)
	return nil
}

// DeleteLeftColumns removes complete columns that exist only on the editable
// left side. The removed workbook structure and cell states form one undo step.
func (s *Session) DeleteLeftColumns(sheet string, columns []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.options.ReadonlyLeft {
		return errors.New(sessionText(s.options.Locale, "The workbook on the left is read-only.", "左侧工作簿为只读，无法修改。", "左側のブックは読み取り専用のため編集できません。"))
	}
	if s.rightSource == nil {
		return errors.New(sessionText(s.options.Locale, "There is no workbook on the right to reconcile with.", "当前没有可用于恢复结构的右侧工作簿。", "列構成の復元に使える右側のブックがありません。"))
	}
	leftSheet, rightSheet := s.left.ByName[sheet], s.rightSource.ByName[sheet]
	if leftSheet == nil || rightSheet == nil {
		return fmt.Errorf("worksheet %q must exist on both sides", sheet)
	}
	seen := make(map[int]struct{}, len(columns))
	removals := make([]history.ColumnRemoval, 0, len(columns))
	for _, logicalCol := range columns {
		if logicalCol < 1 {
			return fmt.Errorf("invalid column %d", logicalCol)
		}
		if _, exists := seen[logicalCol]; exists {
			continue
		}
		seen[logicalCol] = struct{}{}
		leftCol := physicalColumnForLogical(s.leftCols[sheet], logicalCol)
		rightCol := physicalColumnForLogical(s.rightCols[sheet], logicalCol)
		if leftCol == 0 || rightCol != 0 {
			continue
		}
		removal := history.ColumnRemoval{Sheet: sheet, Column: leftCol}
		for row := 1; row <= leftSheet.MaxRow; row++ {
			ref := workbook.CellRef{Sheet: sheet, Row: row, Col: leftCol}
			before, err := merge.Capture(s.leftFile, ref)
			if err != nil {
				return err
			}
			removal.Cells = append(removal.Cells, merge.Operation{Ref: ref, Before: before, Kind: "delete-column"})
		}
		if original := s.originalLeft.ByName[sheet]; original != nil {
			for _, key := range original.CellList {
				if key.Col == leftCol {
					removal.OriginalCells = append(removal.OriginalCells, history.OriginalColumnCell{Row: key.Row, Value: original.Cells[key]})
				}
			}
		}
		removals = append(removals, removal)
	}
	if len(removals) == 0 {
		return errors.New(sessionText(s.options.Locale,
			"The selected columns are not left-only columns.",
			"所选列不是仅存在于左侧的列。",
			"選択した列は左側だけにある列ではありません。"))
	}
	if len(removals)*leftSheet.MaxRow > 10000 {
		return errors.New(sessionText(s.options.Locale,
			"Too many cells selected for whole-column removal (maximum 10000).",
			"整列删除的单元格过多（最多 10000 个）。",
			"列全体の削除対象が多すぎます（最大 10000 セル）。"))
	}
	sort.Slice(removals, func(i, j int) bool { return removals[i].Column > removals[j].Column })
	applied := make([]history.ColumnRemoval, 0, len(removals))
	for _, removal := range removals {
		name, err := excelize.ColumnNumberToName(removal.Column)
		if err != nil {
			_ = s.restoreRemovedColumnsLocked(applied)
			return err
		}
		if err := s.leftFile.RemoveCol(removal.Sheet, name); err != nil {
			_ = s.restoreRemovedColumnsLocked(applied)
			return err
		}
		removeSnapshotColumn(s.left.ByName[removal.Sheet], removal.Column)
		removeSnapshotColumn(s.originalLeft.ByName[removal.Sheet], removal.Column)
		applied = append(applied, removal)
	}
	s.rebuildComparisonLocked()
	s.recordHistoryEntryLocked(nil, nil, removals)
	return nil
}

func (s *Session) insertionColumnForLogicalLocked(sheet string, logical int) int {
	target := s.left.ByName[sheet].MaxCol + 1
	for candidate := logical + 1; candidate <= max(logical, s.comparisonLeft.ByName[sheet].MaxCol+1); candidate++ {
		if physical := physicalColumnForLogical(s.leftCols[sheet], candidate); physical > 0 {
			target = physical
			break
		}
	}
	return target
}

func shiftColumnSourcesForInsert(columns map[int]int, inserted int) {
	for logical, physical := range columns {
		if physical >= inserted {
			columns[logical] = physical + 1
		}
	}
}

func insertSnapshotColumn(sheet *workbook.SheetSnapshot, inserted int) {
	if sheet == nil {
		return
	}
	cells := make(map[workbook.CellKey]workbook.CellValue, len(sheet.Cells))
	list := make([]workbook.CellKey, 0, len(sheet.CellList))
	for _, original := range sheet.CellList {
		key := original
		if key.Col >= inserted {
			key.Col++
		}
		cells[key] = sheet.Cells[original]
		list = append(list, key)
	}
	sheet.Cells, sheet.CellList = cells, list
	sheet.MaxCol++
}

func removeSnapshotColumn(sheet *workbook.SheetSnapshot, removed int) {
	if sheet == nil {
		return
	}
	cells := make(map[workbook.CellKey]workbook.CellValue, len(sheet.Cells))
	list := make([]workbook.CellKey, 0, len(sheet.CellList))
	for _, original := range sheet.CellList {
		if original.Col == removed {
			continue
		}
		key := original
		if key.Col > removed {
			key.Col--
		}
		cells[key] = sheet.Cells[original]
		list = append(list, key)
	}
	sheet.Cells, sheet.CellList = cells, list
	recalculateSheetBounds(sheet)
}

func (s *Session) rollbackColumnCopyLocked(operations []merge.Operation, insertions []history.ColumnInsertion) {
	for index := len(operations) - 1; index >= 0; index-- {
		operation := operations[index]
		if _, err := merge.Apply(s.leftFile, operation.Ref, operation.Before); err == nil {
			updateSnapshotCell(s.left.ByName[operation.Ref.Sheet], workbook.CellKey{Row: operation.Ref.Row, Col: operation.Ref.Col}, operation.Before.Value)
		}
	}
	for index := len(insertions) - 1; index >= 0; index-- {
		insertion := insertions[index]
		name, err := excelize.ColumnNumberToName(insertion.Column)
		if err == nil {
			_ = s.leftFile.RemoveCol(insertion.Sheet, name)
		}
		removeSnapshotColumn(s.left.ByName[insertion.Sheet], insertion.Column)
		removeSnapshotColumn(s.originalLeft.ByName[insertion.Sheet], insertion.Column)
	}
	s.rebuildComparisonLocked()
}

func (s *Session) restoreRemovedColumnsLocked(removals []history.ColumnRemoval) error {
	sort.Slice(removals, func(i, j int) bool { return removals[i].Column < removals[j].Column })
	for _, removal := range removals {
		name, err := excelize.ColumnNumberToName(removal.Column)
		if err != nil {
			return err
		}
		if err := s.leftFile.InsertCols(removal.Sheet, name, 1); err != nil {
			return err
		}
		insertSnapshotColumn(s.left.ByName[removal.Sheet], removal.Column)
		insertSnapshotColumn(s.originalLeft.ByName[removal.Sheet], removal.Column)
		for _, original := range removal.OriginalCells {
			updateSnapshotCell(s.originalLeft.ByName[removal.Sheet], workbook.CellKey{Row: original.Row, Col: removal.Column}, original.Value)
		}
		for _, cell := range removal.Cells {
			if _, applyErr := merge.Apply(s.leftFile, cell.Ref, cell.Before); applyErr != nil {
				return applyErr
			}
			if restored, captureErr := merge.Capture(s.leftFile, cell.Ref); captureErr == nil {
				updateSnapshotCell(s.left.ByName[removal.Sheet], workbook.CellKey{Row: cell.Ref.Row, Col: removal.Column}, restored.Value)
			} else {
				return captureErr
			}
		}
	}
	s.rebuildComparisonLocked()
	return nil
}

func (s *Session) AppendRowsRightToLeft(
	sheet string,
	rows []int,
	requestedIDs []string,
) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.options.ReadonlyLeft {
		return nil, errors.New(sessionText(s.options.Locale, "The workbook on the left is read-only.", "左侧工作簿为只读，无法修改。", "左側のブックは読み取り専用のため編集できません。"))
	}
	if s.right == nil || s.rightFile == nil {
		return nil, errors.New(sessionText(s.options.Locale, "There is no workbook on the right to append from.", "当前没有可供追加的右侧工作簿。", "右側に追加元となるブックがありません。"))
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
		return nil, errors.New(sessionText(s.options.Locale, "Provide one ID for each selected row.", "每个选中行都需要指定一个 ID。", "選択した各行に 1 つずつ ID を指定してください。"))
	}
	sheetDiff := s.sheetDiffLocked(sheet)
	for _, row := range unique {
		if s.rowStatusLocked(sheet, row) != diff.RowConflict {
			return nil, fmt.Errorf(sessionText(s.options.Locale, "Row %d on the right is not a conflict and cannot be appended.", "右侧第 %d 行不是冲突行，不能追加到左侧。", "右側の %d 行目は競合行ではないため追加できません。"), row)
		}
	}
	idColumn := 0
	if sheetDiff != nil && sheetDiff.NextID > 0 {
		idColumn = sheetDiff.IDColumn
	}
	if idColumn == 0 && len(requestedIDs) != 0 {
		return nil, errors.New(sessionText(s.options.Locale, "This sheet has no integer ID column for assigned IDs.", "当前工作表没有可用的整数 ID 列，无法指定 ID。", "このシートには整数の ID 列がないため、ID を指定できません。"))
	}
	maxCol := max(s.comparisonLeft.ByName[sheet].MaxCol, s.right.ByName[sheet].MaxCol)
	if len(unique)*maxCol > 10000 {
		return nil, errors.New("too many cells selected (maximum 10000)")
	}
	assigned := []string{}
	if idColumn > 0 {
		existingIDs := make(map[string]struct{})
		physicalIDColumn := physicalColumnForLogical(s.leftCols[sheet], idColumn)
		for row := 2; row <= leftSheet.MaxRow; row++ {
			id := strings.TrimSpace(leftSheet.Cell(row, physicalIDColumn).Raw)
			if id != "" {
				existingIDs[id] = struct{}{}
			}
		}
		assigned = make([]string, len(unique))
		if len(requestedIDs) == 0 {
			if sheetDiff.NextID <= 0 {
				return nil, errors.New(sessionText(s.options.Locale, "The next integer ID could not be calculated from the left ID column.", "无法根据左侧 ID 列计算下一个整数 ID。", "左側の ID 列から次の整数 ID を計算できません。"))
			}
			for index := range assigned {
				assigned[index] = strconv.FormatInt(sheetDiff.NextID+int64(index), 10)
			}
		} else {
			for index, id := range requestedIDs {
				assigned[index] = strings.TrimSpace(id)
				if assigned[index] == "" {
					return nil, fmt.Errorf(sessionText(s.options.Locale, "The ID for selected row %d cannot be empty.", "第 %d 个选中行的指定 ID 不能为空。", "選択行 %d の ID は空にできません。"), index+1)
				}
			}
		}
		for _, id := range assigned {
			if _, exists := existingIDs[id]; exists {
				return nil, fmt.Errorf(sessionText(s.options.Locale, "ID %s already exists on the left.", "左侧已存在 ID %s。", "左側には ID %s がすでに存在します。"), id)
			}
			existingIDs[id] = struct{}{}
		}
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
			return nil, fmt.Errorf(sessionText(s.options.Locale, "Row %d on the right has no data to append.", "右侧第 %d 行没有可追加的数据。", "右側の %d 行目には追加できるデータがありません。"), sourceRow)
		}
		targetRow := targetStart + index
		for col := 1; col <= maxCol; col++ {
			physicalCol := physicalColumnForLogical(s.leftCols[sheet], col)
			if physicalCol == 0 {
				return nil, errors.New(sessionText(s.options.Locale,
					"A source column does not exist on the left. Copy that whole column first.",
					"左侧缺少来源列，请先复制对应整列。",
					"左側に元の列がありません。先にその列全体を反映してください。"))
			}
			targetRef := workbook.CellRef{Sheet: sheet, Row: targetRow, Col: physicalCol}
			before, captureErr := merge.Capture(s.leftFile, targetRef)
			if captureErr != nil {
				return nil, captureErr
			}
			after, captureErr := s.captureRightLocked(workbook.CellRef{Sheet: sheet, Row: sourceRow, Col: col})
			if captureErr != nil {
				return nil, captureErr
			}
			if idColumn > 0 && col == idColumn {
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
	resolutionKind := ResolutionAppendRow
	if idColumn > 0 && len(requestedIDs) == 0 {
		resolutionKind = ResolutionAppendAuto
	} else if idColumn > 0 {
		resolutionKind = ResolutionAppendSpecified
	}
	for index, sourceRow := range unique {
		targetID := ""
		if idColumn > 0 {
			targetID = assigned[index]
		}
		s.recordResolutionLocked(RowResolution{
			Sheet: sheet, SourceRow: sourceRow, TargetRow: targetStart + index,
			TargetID: targetID, Kind: resolutionKind,
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
			s.warnings = append(s.warnings, fmt.Sprintf(sessionText(s.options.Locale, "Could not roll back %s: %v", "回滚 %s 失败：%v", "%s を元に戻せませんでした：%v"), operation.Ref.Axis(), err))
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
	logicalRef := ref
	ref.Col = physicalColumnForLogical(s.leftCols[ref.Sheet], logicalRef.Col)
	if ref.Col == 0 {
		return errors.New(sessionText(s.options.Locale,
			"This column does not exist on the editable workbook.",
			"可编辑工作簿中不存在此列。",
			"編集可能なブックにこの列はありません。"))
	}
	before, err := merge.Capture(s.leftFile, ref)
	if err != nil {
		return err
	}
	after, err := editedCellState(before, value, valueType)
	if err != nil {
		return err
	}
	if editValuesEqual(before.Value, after.Value) {
		return nil
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
	if ref.Row == 1 {
		s.rebuildComparisonLocked()
	} else if err := s.reclassifyRowsLocked(ref.Sheet); err != nil {
		return err
	}
	s.recordHistoryLocked([]merge.Operation{{Ref: ref, Before: before, After: after, Kind: "edit"}})
	return nil
}

func editedCellState(before merge.CellState, value, valueType string) (merge.CellState, error) {
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
			return merge.CellState{}, fmt.Errorf("invalid number %q", value)
		}
		after.Value.Raw, after.Value.Display, after.Value.Type = value, value, "number"
	case "text", "":
		after.Value.Raw, after.Value.Display, after.Value.Type = value, value, "string"
	default:
		return merge.CellState{}, fmt.Errorf("unsupported edit type %q", valueType)
	}
	return after, nil
}

// ClipboardText serializes the displayed logical selection as Excel-compatible
// tab-separated text. Explicit rows are logical source rows supplied by the
// packed filtered view and retain their display order.
func (s *Session) ClipboardText(
	sheetName, side string,
	startRow, endRow, startCol, endCol int,
	rows []int,
) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if startCol < 1 || endCol < startCol {
		return "", fmt.Errorf("invalid column range %d:%d", startCol, endCol)
	}
	logicalRows, err := clipboardRows(startRow, endRow, rows)
	if err != nil {
		return "", err
	}
	if len(logicalRows)*(endCol-startCol+1) > maxClipboardCells {
		return "", fmt.Errorf("too many cells selected (maximum %d)", maxClipboardCells)
	}
	var source *workbook.WorkbookSnapshot
	switch side {
	case "left":
		source = s.comparisonLeft
	case "right":
		source = s.right
	default:
		return "", fmt.Errorf("unsupported clipboard side %q", side)
	}
	if source == nil {
		return "", fmt.Errorf("no %s workbook is available", side)
	}
	sheet := source.ByName[sheetName]
	if sheet == nil {
		return "", fmt.Errorf("worksheet %q not found", sheetName)
	}
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	writer.Comma = '\t'
	writer.UseCRLF = true
	for _, row := range logicalRows {
		record := make([]string, 0, endCol-startCol+1)
		for col := startCol; col <= endCol; col++ {
			value := sheet.Cell(row, col)
			switch {
			case value.Formula != "":
				record = append(record, "="+value.Formula)
			case value.Raw != "":
				record = append(record, value.Raw)
			default:
				record = append(record, value.Display)
			}
		}
		if err := writer.Write(record); err != nil {
			return "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func clipboardRows(startRow, endRow int, rows []int) ([]int, error) {
	if len(rows) > 0 {
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
	if startRow < 1 || endRow < startRow {
		return nil, fmt.Errorf("invalid row range %d:%d", startRow, endRow)
	}
	result := make([]int, 0, endRow-startRow+1)
	for row := startRow; row <= endRow; row++ {
		result = append(result, row)
	}
	return result, nil
}

// PasteLeftText parses spreadsheet TSV and applies the complete block as one
// undoable Session operation. It never creates columns outside the editable
// workbook's existing logical column mapping.
func (s *Session) PasteLeftText(sheetName string, startRow, startCol int, text string) error {
	rows, err := parseClipboardText(text)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.options.ReadonlyLeft {
		return errors.New("left workbook is read-only")
	}
	if startRow < 1 || startCol < 1 {
		return fmt.Errorf("invalid paste origin %d:%d", startRow, startCol)
	}
	if s.left.ByName[sheetName] == nil {
		return fmt.Errorf("left worksheet %q not found", sheetName)
	}
	cellCount := 0
	for _, row := range rows {
		cellCount += len(row)
	}
	if cellCount == 0 || cellCount > maxClipboardCells {
		return fmt.Errorf("invalid clipboard cell count %d (maximum %d)", cellCount, maxClipboardCells)
	}
	for _, values := range rows {
		for colOffset := range values {
			logicalCol := startCol + colOffset
			if physicalColumnForLogical(s.leftCols[sheetName], logicalCol) == 0 {
				return errors.New(sessionText(s.options.Locale,
					"The pasted range extends beyond the editable workbook's columns.",
					"粘贴范围超出了可编辑工作簿的现有列。",
					"貼り付け範囲が編集可能なブックの既存列を超えています。"))
			}
		}
	}
	operations := make([]merge.Operation, 0, cellCount)
	for rowOffset, values := range rows {
		logicalRow := startRow + rowOffset
		physicalRow := s.ensureLeftPhysicalRowLocked(sheetName, logicalRow)
		for colOffset, value := range values {
			logicalCol := startCol + colOffset
			physicalCol := physicalColumnForLogical(s.leftCols[sheetName], logicalCol)
			ref := workbook.CellRef{Sheet: sheetName, Row: physicalRow, Col: physicalCol}
			before, captureErr := merge.Capture(s.leftFile, ref)
			if captureErr != nil {
				return captureErr
			}
			after, editErr := editedCellState(before, value, clipboardValueType(value))
			if editErr != nil {
				return editErr
			}
			if editValuesEqual(before.Value, after.Value) {
				continue
			}
			operations = append(operations, merge.Operation{
				Ref: ref, Before: before, After: after, Kind: "paste",
			})
		}
	}
	if len(operations) == 0 {
		return nil
	}
	return s.applyOperationsLocked(sheetName, operations)
}

func clipboardValueType(value string) string {
	if value == "" {
		return "clear"
	}
	if strings.HasPrefix(value, "=") {
		return "formula"
	}
	if _, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
		return "number"
	}
	return "text"
}

func parseClipboardText(text string) ([][]string, error) {
	if text == "" {
		return [][]string{{""}}, nil
	}
	reader := csv.NewReader(strings.NewReader(text))
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("invalid clipboard table: %w", err)
	}
	if len(records) == 0 {
		return [][]string{{""}}, nil
	}
	return records, nil
}

func editValuesEqual(before, after workbook.CellValue) bool {
	if before.Present && after.Present && before.Formula != "" && before.Formula == after.Formula && before.Type == after.Type {
		return true
	}
	return before.Equal(after)
}

// ClearLeftSelection removes the contents of every present left-side cell in
// the selected rectangle or explicit row set. Styles and other cell metadata
// are preserved, and the whole selection is recorded as one undo step.
func (s *Session) ClearLeftSelection(
	sheetName string,
	startRow, endRow, startCol, endCol int,
	rows []int,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.options.ReadonlyLeft {
		return errors.New("left workbook is read-only")
	}
	sheet := s.left.ByName[sheetName]
	if sheet == nil {
		return fmt.Errorf("left worksheet %q not found", sheetName)
	}
	if startCol < 1 || endCol < startCol {
		return fmt.Errorf("invalid column range %d:%d", startCol, endCol)
	}

	var selectedRows map[int]struct{}
	if len(rows) > 0 {
		unique, err := normalizeRows(rows)
		if err != nil {
			return err
		}
		selectedRows = make(map[int]struct{}, len(unique))
		for _, row := range unique {
			selectedRows[row] = struct{}{}
		}
	} else {
		if startRow < 1 || endRow < startRow {
			return fmt.Errorf("invalid row range %d:%d", startRow, endRow)
		}
		// Grid coordinates are logical comparison rows. Translate the range
		// back to the editable workbook's physical rows before mutating it.
		// Explicit rows (used by the packed filtered view) are already physical.
		selectedRows = make(map[int]struct{}, endRow-startRow+1)
		for logical := startRow; logical <= endRow; logical++ {
			if physical := s.leftRows[sheetName][logical]; physical > 0 {
				selectedRows[physical] = struct{}{}
			}
		}
	}
	selectedCols := make(map[int]struct{}, endCol-startCol+1)
	for logicalCol := startCol; logicalCol <= endCol; logicalCol++ {
		if physicalCol := physicalColumnForLogical(s.leftCols[sheetName], logicalCol); physicalCol > 0 {
			selectedCols[physicalCol] = struct{}{}
		}
	}

	operations := make([]merge.Operation, 0)
	for _, key := range sheet.CellList {
		if _, selected := selectedCols[key.Col]; !selected {
			continue
		}
		if _, selected := selectedRows[key.Row]; !selected {
			continue
		}
		ref := workbook.CellRef{Sheet: sheetName, Row: key.Row, Col: key.Col}
		before, err := merge.Capture(s.leftFile, ref)
		if err != nil {
			return err
		}
		if !before.Value.Present {
			continue
		}
		after := before
		after.Value = workbook.CellValue{}
		operations = append(operations, merge.Operation{
			Ref: ref, Before: before, After: after, Kind: "clear",
		})
	}
	if len(operations) == 0 {
		return nil
	}
	return s.applyOperationsLocked(sheetName, operations)
}

func (s *Session) Undo() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, err := s.history.Pop()
	if err != nil {
		return err
	}
	commands := entry.Operations
	if len(entry.RemovedColumns) > 0 {
		if err := s.restoreRemovedColumnsLocked(entry.RemovedColumns); err != nil {
			s.history.PushEntry(entry)
			return err
		}
		s.removeResolutionsForStateLocked(entry.AfterState)
		s.stateID = entry.BeforeState
		s.dirty = s.stateID != s.savedState
		return nil
	}
	if len(entry.InsertedColumns) > 0 {
		for index := len(commands) - 1; index >= 0; index-- {
			command := commands[index]
			warnings, applyErr := merge.Apply(s.leftFile, command.Ref, command.Before)
			if applyErr != nil {
				s.history.PushEntry(entry)
				return applyErr
			}
			s.warnings = append(s.warnings, warnings...)
			updateSnapshotCell(s.left.ByName[command.Ref.Sheet], workbook.CellKey{Row: command.Ref.Row, Col: command.Ref.Col}, command.Before.Value)
		}
		for index := len(entry.InsertedColumns) - 1; index >= 0; index-- {
			insertion := entry.InsertedColumns[index]
			name, nameErr := excelize.ColumnNumberToName(insertion.Column)
			if nameErr != nil {
				s.history.PushEntry(entry)
				return nameErr
			}
			if removeErr := s.leftFile.RemoveCol(insertion.Sheet, name); removeErr != nil {
				s.history.PushEntry(entry)
				return removeErr
			}
			removeSnapshotColumn(s.left.ByName[insertion.Sheet], insertion.Column)
			removeSnapshotColumn(s.originalLeft.ByName[insertion.Sheet], insertion.Column)
		}
		s.rebuildComparisonLocked()
		s.removeResolutionsForStateLocked(entry.AfterState)
		s.stateID = entry.BeforeState
		s.dirty = s.stateID != s.savedState
		return nil
	}
	undone := make([]merge.Operation, 0, len(commands))
	changedSheets := make(map[string]struct{})
	rebuildComparison := false
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
		rebuildComparison = rebuildComparison || command.Ref.Row == 1
	}
	if rebuildComparison {
		s.rebuildComparisonLocked()
	} else {
		for sheet := range changedSheets {
			if err := s.reclassifyRowsLocked(sheet); err != nil {
				s.restoreUndoneLocked(undone)
				s.history.PushEntry(entry)
				return err
			}
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
			s.warnings = append(s.warnings, fmt.Sprintf(sessionText(s.options.Locale, "Could not restore %s: %v", "恢复 %s 失败：%v", "%s を復元できませんでした：%v"), operation.Ref.Axis(), err))
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
		return fmt.Errorf("%s", sessionText(s.options.Locale, "The workbook on the left is read-only.", "左侧工作簿为只读，无法修改。", "左側のブックは読み取り専用のため編集できません。"))
	}
	if strings.TrimSpace(target) == "" {
		return errors.New(sessionText(s.options.Locale, "Choose a destination for the exported copy.", "请选择导出副本的保存位置。", "書き出すコピーの保存先を選択してください。"))
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	leftAbs, _ := filepath.Abs(s.left.Path)
	if filepath.Clean(abs) == filepath.Clean(leftAbs) {
		return errors.New(sessionText(s.options.Locale, "The export destination is the current worktree file. Use Save to worktree instead.", "导出位置就是当前工作区文件，请改用“保存到工作区”。", "書き出し先が現在のワークツリーファイルと同じです。「ワークツリーに保存」を使用してください。"))
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
	s.recordHistoryEntryLocked(operations, nil, nil)
}

func (s *Session) recordHistoryEntryLocked(
	operations []merge.Operation,
	insertions []history.ColumnInsertion,
	removals []history.ColumnRemoval,
) {
	after := s.nextState
	s.nextState++
	s.history.PushEntry(history.Entry{
		Operations: operations, InsertedColumns: insertions, RemovedColumns: removals,
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
	if columns := s.rightCols[ref.Sheet]; columns != nil {
		if physicalCol := columns[ref.Col]; physicalCol > 0 {
			ref.Col = physicalCol
		} else if s.columnAligned[ref.Sheet] {
			return merge.CellState{}, nil
		}
	}
	if rows := s.rightRows[ref.Sheet]; rows != nil {
		if physicalRow := rows[ref.Row]; physicalRow > 0 {
			ref.Row = physicalRow
			return merge.Capture(s.rightFile, ref)
		}
		if s.alignedSheets[ref.Sheet] {
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

func logicalRowForPhysical(rows map[int]int, physical int) int {
	for logical, candidate := range rows {
		if candidate == physical {
			return logical
		}
	}
	return 0
}

func annotateDiffRowSources(
	result *diff.WorkbookDiff,
	leftRows, rightRows map[string]map[int]int,
) {
	if result == nil {
		return
	}
	for _, sheet := range result.Sheets {
		for index := range sheet.Rows {
			logical := sheet.Rows[index].Row
			sheet.Rows[index].LeftRow = leftRows[sheet.Name][logical]
			sheet.Rows[index].RightRow = rightRows[sheet.Name][logical]
		}
	}
}

func (s *Session) recordResolutionLocked(resolution RowResolution) {
	if resolution.TargetRow > 0 && resolution.TargetSourceRow == 0 {
		resolution.TargetSourceRow = logicalRowForPhysical(s.leftRows[resolution.Sheet], resolution.TargetRow)
	}
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
	err := s.currentDiff.ReclassifyRows(name, s.comparisonLeft.ByName[name], s.right.ByName[name])
	if err == nil {
		annotateDiffRowSources(s.currentDiff, s.leftRows, s.rightRows)
	}
	return err
}

func (s *Session) rebuildComparisonLocked() {
	s.dataGeneration++
	if s.rightSource == nil {
		s.comparisonLeft = s.left
		s.right = nil
		s.leftRows = identityWorkbookRows(s.left)
		s.rightRows = nil
		s.leftCols = identityWorkbookColumns(s.left)
		s.rightCols = nil
		s.columnAligned = nil
		s.alignedSheets = nil
		s.currentDiff = nil
		s.alignmentNotice = ""
		s.alignmentMoved = 0
		s.alignmentAvailable = false
		s.alignmentSheetMoved = nil
		s.alignmentSheetEligible = nil
		return
	}
	alignment := prepareRightForComparison(
		s.left, s.rightSource, s.options.GitMerge, s.alignmentMode, s.keyColumns,
	)
	s.comparisonLeft = alignment.left
	s.right = alignment.right
	s.leftRows = alignment.leftSources
	s.rightRows = alignment.rowSources
	s.leftCols = alignment.leftColumns
	s.rightCols = alignment.rightColumns
	s.columnAligned = alignment.columnAligned
	s.alignedSheets = alignment.alignedSheets
	s.currentDiff = diff.CompareWithKeyColumns(alignment.left, alignment.right, alignment.keyColumns)
	annotateDiffRowSources(s.currentDiff, alignment.leftSources, alignment.rowSources)
	s.alignmentNotice = alignmentWarning(s.options.Locale, alignment.moved)
	s.alignmentMoved = alignment.moved
	s.alignmentAvailable = alignment.available
	s.alignmentSheetMoved = alignment.sheetMoved
	s.alignmentSheetEligible = alignment.sheetEligible
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
	s.dataGeneration++
	if s.currentDiff == nil || s.right == nil {
		return nil
	}
	logicalRow := logicalRowForPhysical(s.leftRows[ref.Sheet], ref.Row)
	if logicalRow == 0 {
		logicalRow = nextLogicalRow(s.leftRows[ref.Sheet])
		if s.leftRows[ref.Sheet] == nil {
			s.leftRows[ref.Sheet] = make(map[int]int)
		}
		s.leftRows[ref.Sheet][logicalRow] = ref.Row
	}
	logicalRef := ref
	logicalRef.Row = logicalRow
	logicalCol := logicalColumnForPhysical(s.leftCols[ref.Sheet], ref.Col)
	if logicalCol == 0 {
		logicalCol = ref.Col
	}
	logicalRef.Col = logicalCol
	comparisonSheet := s.comparisonLeft.ByName[ref.Sheet]
	updateSnapshotCell(comparisonSheet, workbook.CellKey{Row: logicalRow, Col: logicalCol}, state.Value)
	return s.currentDiff.UpdateCell(
		logicalRef, state.Value, s.right.ByName[ref.Sheet].Cell(logicalRow, logicalCol),
	)
}

func logicalColumnForPhysical(columns map[int]int, physical int) int {
	for logical, candidate := range columns {
		if candidate == physical {
			return logical
		}
	}
	return 0
}

func nextLogicalRow(rows map[int]int) int {
	next := 1
	for logical := range rows {
		next = max(next, logical+1)
	}
	return next
}

func updateSnapshotCell(sheet *workbook.SheetSnapshot, key workbook.CellKey, value workbook.CellValue) {
	if sheet == nil {
		return
	}
	_, existed := sheet.Cells[key]
	if value.Present {
		sheet.Cells[key] = value
		if !existed {
			index := sort.Search(len(sheet.CellList), func(i int) bool {
				item := sheet.CellList[i]
				return item.Row > key.Row || (item.Row == key.Row && item.Col >= key.Col)
			})
			sheet.CellList = append(sheet.CellList, workbook.CellKey{})
			copy(sheet.CellList[index+1:], sheet.CellList[index:])
			sheet.CellList[index] = key
		}
		sheet.MaxRow = max(sheet.MaxRow, key.Row)
		sheet.MaxCol = max(sheet.MaxCol, key.Col)
		return
	}
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
	if key.Row == sheet.MaxRow || key.Col == sheet.MaxCol {
		recalculateSheetBounds(sheet)
	}
}

func recalculateSheetBounds(sheet *workbook.SheetSnapshot) {
	sheet.MaxRow = 0
	sheet.MaxCol = 0
	for _, key := range sheet.CellList {
		sheet.MaxRow = max(sheet.MaxRow, key.Row)
		sheet.MaxCol = max(sheet.MaxCol, key.Col)
	}
}
