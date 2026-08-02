package history

import (
	"testing"

	"github.com/tadazly/sheetproof/internal/merge"
	"github.com/tadazly/sheetproof/internal/workbook"
)

func TestStackLIFOAndClear(t *testing.T) {
	var stack Stack
	stack.Push(merge.Operation{Ref: workbook.CellRef{Sheet: "S", Row: 1, Col: 1}})
	stack.Push(merge.Operation{Ref: workbook.CellRef{Sheet: "S", Row: 2, Col: 1}})
	if stack.Len() != 2 {
		t.Fatalf("length = %d", stack.Len())
	}
	command, err := stack.Pop()
	if err != nil || len(command.Operations) != 1 || command.Operations[0].Ref.Row != 2 {
		t.Fatalf("pop = %+v, %v", command, err)
	}
	stack.PushEntry(Entry{
		Operations: []merge.Operation{
			{Ref: workbook.CellRef{Sheet: "S", Row: 3, Col: 1}},
			{Ref: workbook.CellRef{Sheet: "S", Row: 4, Col: 1}},
		},
		BeforeState: 10,
		AfterState:  11,
	})
	command, err = stack.Pop()
	if err != nil || len(command.Operations) != 2 || command.Operations[1].Ref.Row != 4 ||
		command.BeforeState != 10 || command.AfterState != 11 {
		t.Fatalf("batch pop = %+v, %v", command, err)
	}
	stack.Clear()
	if _, err := stack.Pop(); err != ErrEmpty {
		t.Fatalf("empty pop error = %v", err)
	}
}
