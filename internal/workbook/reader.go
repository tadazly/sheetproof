package workbook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"
)

type Reader struct{}

func (Reader) Validate(path string) error {
	if err := ValidateXLSXPath(path); err != nil {
		return err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return &Error{Code: ErrUnreadable, Path: path, Err: err}
	}
	if _, err := Identity(abs); err != nil {
		return err
	}
	f, err := excelize.OpenFile(abs, excelize.Options{RawCellValue: true})
	if err != nil {
		return &Error{Code: ErrCorrupt, Path: abs, Err: err}
	}
	defer f.Close()
	if len(f.GetSheetList()) == 0 {
		return &Error{Code: ErrNoSheets, Path: abs, Err: fmt.Errorf("workbook contains no worksheets")}
	}
	return nil
}

func ValidateXLSXPath(path string) error {
	if strings.ToLower(filepath.Ext(path)) != ".xlsx" {
		return &Error{Code: ErrUnsupported, Path: path, Err: fmt.Errorf("only .xlsx files are supported")}
	}
	return nil
}

func Identity(path string) (FileIdentity, error) {
	return IdentityContext(context.Background(), path)
}

// IdentityContext calculates the file identity while allowing background
// callers such as repository indexing to stop promptly during application
// shutdown.
func IdentityContext(ctx context.Context, path string) (FileIdentity, error) {
	if err := contextError(ctx); err != nil {
		return FileIdentity{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		code := ErrUnreadable
		if os.IsNotExist(err) {
			code = ErrNotFound
		}
		return FileIdentity{}, &Error{Code: code, Path: path, Err: err}
	}
	f, err := os.Open(path)
	if err != nil {
		return FileIdentity{}, &Error{Code: ErrUnreadable, Path: path, Err: err}
	}
	defer f.Close()
	hash := sha256.New()
	var source io.Reader = f
	if ctx.Done() != nil {
		source = &contextReader{ctx: ctx, reader: f}
	}
	if _, err = io.Copy(hash, source); err != nil {
		return FileIdentity{}, &Error{Code: ErrUnreadable, Path: path, Err: err}
	}
	return FileIdentity{
		Size:    info.Size(),
		ModTime: info.ModTime().UnixNano(),
		SHA256:  hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

func (Reader) Open(path string) (*excelize.File, *WorkbookSnapshot, error) {
	return (Reader{}).OpenContext(context.Background(), path)
}

// OpenContext opens and snapshots a workbook while periodically checking ctx.
// Interactive opens use Open; background semantic indexing uses this method so
// closing the application does not wait for the entire workbook scan.
func (Reader) OpenContext(ctx context.Context, path string) (*excelize.File, *WorkbookSnapshot, error) {
	if err := contextError(ctx); err != nil {
		return nil, nil, err
	}
	if err := ValidateXLSXPath(path); err != nil {
		return nil, nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, &Error{Code: ErrUnreadable, Path: path, Err: err}
	}
	id, err := IdentityContext(ctx, abs)
	if err != nil {
		return nil, nil, err
	}
	if err := contextError(ctx); err != nil {
		return nil, nil, err
	}
	f, err := excelize.OpenFile(abs, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, nil, &Error{Code: ErrCorrupt, Path: abs, Err: err}
	}
	snapshot, err := snapshotContext(ctx, f, abs, id)
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	return f, snapshot, nil
}

func Snapshot(f *excelize.File, path string, id FileIdentity) (*WorkbookSnapshot, error) {
	return snapshotContext(context.Background(), f, path, id)
}

func snapshotContext(ctx context.Context, f *excelize.File, path string, id FileIdentity) (*WorkbookSnapshot, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	names := f.GetSheetList()
	if len(names) == 0 {
		return nil, &Error{Code: ErrNoSheets, Path: path, Err: fmt.Errorf("workbook contains no worksheets")}
	}
	result := &WorkbookSnapshot{
		Path:     path,
		Sheets:   make([]*SheetSnapshot, 0, len(names)),
		ByName:   make(map[string]*SheetSnapshot, len(names)),
		Identity: id,
	}
	for index, name := range names {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		sheet, err := snapshotSheet(ctx, f, name, index)
		if err != nil {
			return nil, &Error{Code: ErrCorrupt, Path: path, Err: fmt.Errorf("read sheet %q: %w", name, err)}
		}
		result.Sheets = append(result.Sheets, sheet)
		result.ByName[name] = sheet
	}
	return result, nil
}

func snapshotSheet(ctx context.Context, f *excelize.File, name string, index int) (*SheetSnapshot, error) {
	rows, err := f.Rows(name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sheet := &SheetSnapshot{
		Name:  name,
		Index: index,
		Cells: make(map[CellKey]CellValue),
	}
	rowIndex := 0
	for rows.Next() {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		rowIndex++
		columns, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return nil, err
		}
		for colIndex := 1; colIndex <= len(columns); colIndex++ {
			if err := contextError(ctx); err != nil {
				return nil, err
			}
			axis, _ := excelize.CoordinatesToCellName(colIndex, rowIndex)
			cellType, err := f.GetCellType(name, axis)
			if err != nil {
				return nil, err
			}
			formula, err := f.GetCellFormula(name, axis)
			if err != nil {
				return nil, err
			}
			raw := columns[colIndex-1]
			present := CellPresent(cellType, raw, formula)
			if !present {
				continue
			}
			display, err := f.GetCellValue(name, axis, excelize.Options{RawCellValue: false})
			if err != nil {
				return nil, err
			}
			styleID, _ := f.GetCellStyle(name, axis)
			key := CellKey{Row: rowIndex, Col: colIndex}
			sheet.Cells[key] = CellValue{
				Present: true,
				Raw:     raw,
				Display: display,
				Formula: formula,
				Type:    ClassifyCellType(f, cellType, formula, raw, styleID),
				StyleID: styleID,
			}
			sheet.CellList = append(sheet.CellList, key)
			if colIndex > sheet.MaxCol {
				sheet.MaxCol = colIndex
			}
		}
		if len(columns) > 0 {
			sheet.MaxRow = rowIndex
		}
	}
	if err := rows.Error(); err != nil {
		return nil, err
	}
	sort.Slice(sheet.CellList, func(i, j int) bool {
		if sheet.CellList[i].Row == sheet.CellList[j].Row {
			return sheet.CellList[i].Col < sheet.CellList[j].Col
		}
		return sheet.CellList[i].Row < sheet.CellList[j].Row
	})
	return sheet, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	if err := contextError(r.ctx); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

func contextError(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// CellPresent keeps snapshot reads and incremental merge captures consistent.
// A style-only/default-typed empty cell is visually and semantically absent,
// while an explicitly stored empty string remains present.
func CellPresent(cellType excelize.CellType, raw, formula string) bool {
	if raw != "" || formula != "" {
		return true
	}
	return cellType == excelize.CellTypeInlineString ||
		cellType == excelize.CellTypeSharedString
}

func ClassifyCellType(f *excelize.File, t excelize.CellType, formula, raw string, styleID int) string {
	if formula != "" {
		return "formula"
	}
	switch t {
	case excelize.CellTypeBool:
		return "bool"
	case excelize.CellTypeDate:
		return "date"
	case excelize.CellTypeError:
		return "error"
	case excelize.CellTypeFormula:
		return "formula"
	case excelize.CellTypeInlineString:
		return "inline-string"
	case excelize.CellTypeNumber:
		return "number"
	case excelize.CellTypeSharedString:
		return "string"
	default:
		if raw != "" {
			if styleID != 0 {
				if style, err := f.GetStyle(styleID); err == nil && styleUsesDateFormat(style) {
					return "date"
				}
			}
			return "number"
		}
		return "unset"
	}
}

func styleUsesDateFormat(style *excelize.Style) bool {
	if style == nil {
		return false
	}
	switch style.NumFmt {
	case 14, 15, 16, 17, 18, 19, 20, 21, 22,
		27, 28, 29, 30, 31, 32, 33, 34, 35, 36,
		45, 46, 47, 50, 51, 52, 53, 54, 55, 56, 57, 58:
		return true
	}
	if style.CustomNumFmt == nil {
		return false
	}
	format := strings.ToLower(*style.CustomNumFmt)
	format = strings.ReplaceAll(format, `\`, "")
	return strings.ContainsAny(format, "yd") ||
		(strings.Contains(format, "m") && (strings.Contains(format, "h") || strings.Contains(format, "s")))
}

func SamePath(left, right string) (bool, error) {
	l, err := filepath.Abs(left)
	if err != nil {
		return false, err
	}
	r, err := filepath.Abs(right)
	if err != nil {
		return false, err
	}
	if filepath.Clean(l) == filepath.Clean(r) {
		return true, nil
	}
	li, lerr := os.Stat(l)
	ri, rerr := os.Stat(r)
	return lerr == nil && rerr == nil && os.SameFile(li, ri), nil
}
