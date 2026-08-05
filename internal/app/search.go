package app

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tadazly/sheetproof/internal/diff"
	"github.com/tadazly/sheetproof/internal/workbook"
)

type SearchSide string

const (
	SearchLeft  SearchSide = "left"
	SearchRight SearchSide = "right"
)

type SearchRef struct {
	Row       int    `json:"row"`
	SourceRow int    `json:"sourceRow"`
	Col       int    `json:"col"`
	Axis      string `json:"axis"`
}

type SearchSummary struct {
	Count        int        `json:"count"`
	CurrentIndex int        `json:"currentIndex"`
	CurrentRef   *SearchRef `json:"currentRef,omitempty"`
	Error        string     `json:"error,omitempty"`
}

type searchKey struct {
	Sheet         string
	Query         string
	Filters       string
	CaseSensitive bool
	WholeWord     bool
	Regex         bool
	Generation    uint64
}

type searchMatch struct {
	row       int
	sourceRow int
	col       int
}

type searchCache struct {
	key     searchKey
	matches []searchMatch
	current int
}

type cellMatcher struct {
	expression *regexp.Regexp
	wholeWord  bool
}

func compileCellMatcher(query string, caseSensitive, wholeWord, useRegex bool) (*cellMatcher, error) {
	pattern := query
	if !useRegex {
		pattern = regexp.QuoteMeta(pattern)
	}
	if !caseSensitive {
		pattern = "(?i:" + pattern + ")"
	}
	expression, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &cellMatcher{expression: expression, wholeWord: wholeWord}, nil
}

func (m *cellMatcher) matches(text string) bool {
	if !m.wholeWord {
		return m.expression.MatchString(text)
	}
	for _, location := range m.expression.FindAllStringIndex(text, -1) {
		if unicodeWordBoundary(text, location[0], location[1]) {
			return true
		}
	}
	return false
}

func unicodeWordBoundary(text string, start, end int) bool {
	if start > 0 {
		before, _ := utf8.DecodeLastRuneInString(text[:start])
		if unicodeWordRune(before) {
			return false
		}
	}
	if end < len(text) {
		after, _ := utf8.DecodeRuneInString(text[end:])
		if unicodeWordRune(after) {
			return false
		}
	}
	return true
}

func unicodeWordRune(value rune) bool {
	return value == '_' || unicode.IsLetter(value) || unicode.IsDigit(value)
}

func searchCellText(value workbook.CellValue) string {
	if value.Formula != "" {
		return "=" + value.Formula
	}
	return value.Raw
}

func (s *Session) Search(
	sheet string,
	side SearchSide,
	query string,
	caseSensitive, wholeWord, useRegex bool,
	statuses []diff.RowStatus,
	anchor CellCoordinate,
) (SearchSummary, error) {
	if side != SearchLeft && side != SearchRight {
		return SearchSummary{}, fmt.Errorf("invalid search side %q", side)
	}
	canonicalFilters, err := canonicalRowFilters(statuses)
	if err != nil {
		return SearchSummary{}, err
	}
	if query == "" {
		s.ClearSearch(side)
		return SearchSummary{}, nil
	}
	matcher, err := compileCellMatcher(query, caseSensitive, wholeWord, useRegex)
	if err != nil {
		return SearchSummary{Error: err.Error()}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.left.ByName[sheet] == nil && (s.rightSource == nil || s.rightSource.ByName[sheet] == nil) {
		return SearchSummary{}, fmt.Errorf("worksheet %q not found", sheet)
	}
	key := searchKey{
		Sheet: sheet, Query: query, Filters: canonicalFilters,
		CaseSensitive: caseSensitive, WholeWord: wholeWord, Regex: useRegex,
		Generation: s.dataGeneration,
	}
	s.searchMu.Lock()
	defer s.searchMu.Unlock()
	cache := s.searches[side]
	if cache == nil || cache.key != key {
		matches, scanErr := s.scanMatchesLocked(sheet, side, statuses, matcher)
		if scanErr != nil {
			return SearchSummary{}, scanErr
		}
		cache = &searchCache{key: key, matches: matches, current: -1}
		s.searches[side] = cache
	}
	cache.current = initialSearchIndex(cache.matches, anchor)
	return searchSummary(cache), nil
}

func (s *Session) NavigateSearch(side SearchSide, direction int) (SearchSummary, error) {
	if side != SearchLeft && side != SearchRight {
		return SearchSummary{}, fmt.Errorf("invalid search side %q", side)
	}
	if direction != -1 && direction != 1 {
		return SearchSummary{}, fmt.Errorf("invalid search direction %d", direction)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.searchMu.Lock()
	defer s.searchMu.Unlock()
	cache := s.searches[side]
	if cache == nil || cache.key.Generation != s.dataGeneration || len(cache.matches) == 0 {
		return SearchSummary{}, nil
	}
	cache.current = (cache.current + direction + len(cache.matches)) % len(cache.matches)
	return searchSummary(cache), nil
}

func (s *Session) ClearSearch(side SearchSide) {
	s.searchMu.Lock()
	delete(s.searches, side)
	s.searchMu.Unlock()
}

func searchSummary(cache *searchCache) SearchSummary {
	if cache == nil || len(cache.matches) == 0 || cache.current < 0 {
		return SearchSummary{}
	}
	match := cache.matches[cache.current]
	ref := workbook.CellRef{Sheet: cache.key.Sheet, Row: match.sourceRow, Col: match.col}
	return SearchSummary{
		Count: len(cache.matches), CurrentIndex: cache.current + 1,
		CurrentRef: &SearchRef{
			Row: match.row, SourceRow: match.sourceRow, Col: match.col, Axis: ref.Axis(),
		},
	}
}

func initialSearchIndex(matches []searchMatch, anchor CellCoordinate) int {
	if len(matches) == 0 {
		return -1
	}
	if anchor.Row < 1 {
		anchor.Row = 1
	}
	if anchor.Col < 1 {
		anchor.Col = 1
	}
	index := sort.Search(len(matches), func(index int) bool {
		match := matches[index]
		return match.row > anchor.Row || (match.row == anchor.Row && match.col >= anchor.Col)
	})
	if index == len(matches) {
		return 0
	}
	return index
}

func (s *Session) scanMatchesLocked(
	sheet string,
	side SearchSide,
	statuses []diff.RowStatus,
	matcher *cellMatcher,
) ([]searchMatch, error) {
	if len(statuses) == 0 {
		var current *workbook.SheetSnapshot
		if side == SearchLeft {
			current = s.comparisonLeft.ByName[sheet]
		} else if s.right != nil {
			current = s.right.ByName[sheet]
		}
		return scanSheetMatches(current, nil, matcher), nil
	}
	rows, err := s.filteredRowsLocked(sheet, statuses)
	if err != nil {
		return nil, err
	}
	physicalRows := make(map[int]regionRow, len(rows))
	var current *workbook.SheetSnapshot
	if side == SearchLeft {
		current = s.left.ByName[sheet]
		for _, row := range rows {
			if row.left > 0 {
				physicalRows[row.left] = row
			}
		}
	} else {
		if s.rightSource != nil {
			current = s.rightSource.ByName[sheet]
		}
		for _, row := range rows {
			if row.right > 0 {
				physicalRows[row.right] = row
			}
		}
	}
	return scanSheetMatches(current, physicalRows, matcher), nil
}

func scanSheetMatches(
	sheet *workbook.SheetSnapshot,
	physicalRows map[int]regionRow,
	matcher *cellMatcher,
) []searchMatch {
	if sheet == nil {
		return nil
	}
	matches := make([]searchMatch, 0)
	for _, key := range sheet.CellList {
		row := regionRow{display: key.Row, source: key.Row}
		if physicalRows != nil {
			mapped, visible := physicalRows[key.Row]
			if !visible {
				continue
			}
			row = mapped
		}
		value := sheet.Cells[key]
		if !value.Present || !matcher.matches(searchCellText(value)) {
			continue
		}
		matches = append(matches, searchMatch{row: row.display, sourceRow: row.source, col: key.Col})
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].row < matches[j].row ||
			(matches[i].row == matches[j].row && matches[i].col < matches[j].col)
	})
	return matches
}

func canonicalRowFilters(statuses []diff.RowStatus) (string, error) {
	selected := make(map[diff.RowStatus]bool, len(statuses))
	for _, status := range statuses {
		switch status {
		case diff.RowAdded, diff.RowDeleted, diff.RowModified, diff.RowConflict:
			selected[status] = true
		default:
			return "", fmt.Errorf("invalid row filter %q", status)
		}
	}
	ordered := []diff.RowStatus{diff.RowAdded, diff.RowDeleted, diff.RowModified, diff.RowConflict}
	values := make([]string, 0, len(selected))
	for _, status := range ordered {
		if selected[status] {
			values = append(values, string(status))
		}
	}
	return strings.Join(values, ","), nil
}

func (s *Session) filteredRowsLocked(sheet string, statuses []diff.RowStatus) ([]regionRow, error) {
	if len(statuses) == 0 {
		return nil, fmt.Errorf("at least one row filter is required")
	}
	if _, err := canonicalRowFilters(statuses); err != nil {
		return nil, err
	}
	selected := make(map[diff.RowStatus]bool, len(statuses))
	for _, status := range statuses {
		selected[status] = true
	}
	sheetDiff := s.sheetDiffLocked(sheet)
	if sheetDiff == nil {
		if s.left.ByName[sheet] == nil && (s.right == nil || s.right.ByName[sheet] == nil) {
			return nil, fmt.Errorf("worksheet %q not found", sheet)
		}
		return []regionRow{}, nil
	}
	appendTargets := make(map[int]bool)
	appendSources := make(map[int]int)
	for _, resolution := range s.resolutions {
		if resolution.Sheet == sheet && resolution.TargetRow > 0 && isAppendResolution(resolution.Kind) {
			appendTargets[resolution.TargetSourceRow] = true
			appendSources[resolution.SourceRow] = resolution.TargetRow
		}
	}
	rows := make([]regionRow, 0, len(sheetDiff.Rows))
	for _, rowDiff := range sheetDiff.Rows {
		if appendTargets[rowDiff.Row] || !selected[rowDiff.Status] {
			continue
		}
		mapping := regionRow{
			display: len(rows) + 1,
			source:  rowDiff.Row, left: rowDiff.LeftRow, right: rowDiff.RightRow,
		}
		if targetRow := appendSources[rowDiff.Row]; targetRow > 0 {
			mapping.left = targetRow
		}
		rows = append(rows, mapping)
	}
	return rows, nil
}

func (s *Session) searchMatchesLocked(
	side SearchSide,
	sheet string,
	statuses []diff.RowStatus,

) []searchMatch {
	filters, err := canonicalRowFilters(statuses)
	if err != nil {
		return nil
	}
	s.searchMu.Lock()
	defer s.searchMu.Unlock()
	cache := s.searches[side]
	if cache == nil || cache.key.Generation != s.dataGeneration ||
		cache.key.Sheet != sheet || cache.key.Filters != filters {
		return nil
	}
	return cache.matches
}

func searchMatchAt(matches []searchMatch, sourceRow, col int) bool {
	index := sort.Search(len(matches), func(index int) bool {
		match := matches[index]
		return match.sourceRow > sourceRow || (match.sourceRow == sourceRow && match.col >= col)
	})
	return index < len(matches) &&
		matches[index].sourceRow == sourceRow && matches[index].col == col
}
