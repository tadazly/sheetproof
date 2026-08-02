package history

import (
	"errors"
	"sync"

	"github.com/tadazly/sheetproof/internal/merge"
)

var ErrEmpty = errors.New("nothing to undo")

type Entry struct {
	Operations  []merge.Operation
	BeforeState uint64
	AfterState  uint64
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
	if len(entry.Operations) == 0 {
		return
	}
	entry.Operations = append([]merge.Operation(nil), entry.Operations...)
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
	s.commands = s.commands[:index]
	return command, nil
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
