package workbook

import (
	"fmt"
	"strconv"
)

type CellRef struct {
	Sheet string `json:"sheet"`
	Row   int    `json:"row"`
	Col   int    `json:"col"`
}

func (r CellRef) Axis() string {
	col := r.Col
	name := ""
	for col > 0 {
		col--
		name = string(rune('A'+col%26)) + name
		col /= 26
	}
	return name + strconv.Itoa(r.Row)
}

type CellValue struct {
	Present bool   `json:"present"`
	Raw     string `json:"raw"`
	Display string `json:"display"`
	Formula string `json:"formula,omitempty"`
	Type    string `json:"type"`
	StyleID int    `json:"styleId,omitempty"`
}

func (v CellValue) Equal(other CellValue) bool {
	return v.Present == other.Present &&
		v.Raw == other.Raw &&
		v.Formula == other.Formula &&
		v.Type == other.Type
}

type CellKey struct {
	Row int
	Col int
}

type SheetSnapshot struct {
	Name     string
	Index    int
	MaxRow   int
	MaxCol   int
	Cells    map[CellKey]CellValue
	CellList []CellKey
}

type FileIdentity struct {
	Size    int64
	ModTime int64
	SHA256  string
}

type WorkbookSnapshot struct {
	Path     string
	Sheets   []*SheetSnapshot
	ByName   map[string]*SheetSnapshot
	Identity FileIdentity
}

func (s *SheetSnapshot) Cell(row, col int) CellValue {
	if s == nil {
		return CellValue{}
	}
	return s.Cells[CellKey{Row: row, Col: col}]
}

func ValidateRef(ref CellRef) error {
	if ref.Sheet == "" || ref.Row < 1 || ref.Col < 1 {
		return fmt.Errorf("invalid cell reference: sheet=%q row=%d col=%d", ref.Sheet, ref.Row, ref.Col)
	}
	return nil
}
