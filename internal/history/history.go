package history

import (
	"errors"
	"sync"

	"github.com/tadazly/sheetproof/internal/merge"
	"github.com/tadazly/sheetproof/internal/workbook"
)

var ErrEmpty = errors.New("nothing to undo")

type Entry struct {
	Operations      []merge.Operation
	InsertedColumns []ColumnInsertion
	RemovedColumns  []ColumnRemoval
	BeforeState     uint64
	AfterState      uint64
}

type ColumnInsertion struct {
	Sheet  string
	Column int
}

type ColumnRemoval struct {
	Sheet         string
	Column        int
	Cells         []merge.Operation
	OriginalCells []OriginalColumnCell
}

type OriginalColumnCell struct {
	Row   int
	Value workbook.CellValue
}

type Stack struct {
	mu       sync.Mutex
	commands []Entry
}

func (s *Stack) Push(command merge.Operation) {
	s.PushBatch([]merge.Operation{command})
}

func (s *Stack) PushBatch(commands []merge.Operation) {
	s.PushEntry(Entry{Operations: commands})
}

func (s *Stack) PushEntry(entry Entry) {
	if len(entry.Operations) == 0 && len(entry.InsertedColumns) == 0 && len(entry.RemovedColumns) == 0 {
		return
	}
	entry.Operations = append([]merge.Operation(nil), entry.Operations...)
	entry.InsertedColumns = append([]ColumnInsertion(nil), entry.InsertedColumns...)
	entry.RemovedColumns = cloneColumnRemovals(entry.RemovedColumns)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commands = append(s.commands, entry)
}

func (s *Stack) Pop() (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.commands) == 0 {
		return Entry{}, ErrEmpty
	}
	index := len(s.commands) - 1
	command := s.commands[index]
	command.Operations = append([]merge.Operation(nil), command.Operations...)
	command.InsertedColumns = append([]ColumnInsertion(nil), command.InsertedColumns...)
	command.RemovedColumns = cloneColumnRemovals(command.RemovedColumns)
	s.commands = s.commands[:index]
	return command, nil
}

func cloneColumnRemovals(removals []ColumnRemoval) []ColumnRemoval {
	result := make([]ColumnRemoval, len(removals))
	for index, removal := range removals {
		result[index] = removal
		result[index].Cells = append([]merge.Operation(nil), removal.Cells...)
		result[index].OriginalCells = append([]OriginalColumnCell(nil), removal.OriginalCells...)
	}
	return result
}

func (s *Stack) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.commands)
}

func (s *Stack) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commands = nil
}
