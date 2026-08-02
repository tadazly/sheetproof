package merge

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ug-tools/ugxlsx/internal/workbook"
	"github.com/xuri/excelize/v2"
)

type CellState struct {
	Value     workbook.CellValue
	Style     *excelize.Style
	Hyperlink string
	LinkType  string
	Comment   *excelize.Comment
}

type Operation struct {
	Ref    workbook.CellRef
	Before CellState
	After  CellState
	Kind   string
}

func Capture(f *excelize.File, ref workbook.CellRef) (CellState, error) {
	if err := workbook.ValidateRef(ref); err != nil {
		return CellState{}, err
	}
	axis := ref.Axis()
	cellType, err := f.GetCellType(ref.Sheet, axis)
	if err != nil {
		return CellState{}, err
	}
	raw, err := f.GetCellValue(ref.Sheet, axis, excelize.Options{RawCellValue: true})
	if err != nil {
		return CellState{}, err
	}
	display, err := f.GetCellValue(ref.Sheet, axis, excelize.Options{RawCellValue: false})
	if err != nil {
		return CellState{}, err
	}
	formula, err := f.GetCellFormula(ref.Sheet, axis)
	if err != nil {
		return CellState{}, err
	}
	styleID, err := f.GetCellStyle(ref.Sheet, axis)
	if err != nil {
		return CellState{}, err
	}
	var style *excelize.Style
	if styleID != 0 {
		style, err = f.GetStyle(styleID)
		if err != nil {
			return CellState{}, err
		}
	}
	hasLink, link, err := f.GetCellHyperLink(ref.Sheet, axis)
	if err != nil {
		return CellState{}, err
	}
	if !hasLink {
		link = ""
	}
	linkType := ""
	if link != "" {
		linkType = "External"
		lower := strings.ToLower(link)
		if !strings.Contains(lower, "://") && !strings.HasPrefix(lower, "mailto:") {
			linkType = "Location"
		}
	}
	comments, err := f.GetComments(ref.Sheet)
	if err != nil {
		return CellState{}, err
	}
	var comment *excelize.Comment
	for i := range comments {
		if comments[i].Cell == axis {
			copy := comments[i]
			comment = &copy
			break
		}
	}
	present := workbook.CellPresent(cellType, raw, formula)
	value := workbook.CellValue{}
	if present {
		value = workbook.CellValue{
			Present: true,
			Raw:     raw, Display: display, Formula: formula,
			Type: workbook.ClassifyCellType(f, cellType, formula, raw, styleID), StyleID: styleID,
		}
	}
	return CellState{
		Value: value,
		Style: style, Hyperlink: link, LinkType: linkType, Comment: comment,
	}, nil
}

func Apply(f *excelize.File, ref workbook.CellRef, state CellState) ([]string, error) {
	if err := workbook.ValidateRef(ref); err != nil {
		return nil, err
	}
	axis := ref.Axis()
	warnings := make([]string, 0, 1)
	if state.Value.Formula != "" {
		if err := f.SetCellFormula(ref.Sheet, axis, state.Value.Formula); err != nil {
			return warnings, err
		}
	} else if !state.Value.Present {
		if err := f.SetCellValue(ref.Sheet, axis, nil); err != nil {
			return warnings, err
		}
	} else {
		if err := setRawValue(f, ref.Sheet, axis, state.Value); err != nil {
			return warnings, err
		}
	}
	if state.Style != nil {
		style := *state.Style
		if style.Fill.Pattern < 0 || style.Fill.Pattern > 18 {
			warnings = append(warnings, fmt.Sprintf(
				"%s 的填充样式编号 %d 超出支持范围，已保留内容并忽略该填充",
				axis, style.Fill.Pattern,
			))
			style.Fill = excelize.Fill{}
		}
		styleID, err := f.NewStyle(&style)
		if err != nil {
			return warnings, err
		}
		if err := f.SetCellStyle(ref.Sheet, axis, axis, styleID); err != nil {
			return warnings, err
		}
	} else if state.Value.StyleID == 0 {
		if err := f.SetCellStyle(ref.Sheet, axis, axis, 0); err != nil {
			return warnings, err
		}
	}
	if err := f.DeleteComment(ref.Sheet, axis); err != nil {
		return warnings, err
	}
	if state.Comment != nil {
		comment := *state.Comment
		comment.Cell = axis
		if err := f.AddComment(ref.Sheet, comment); err != nil {
			return warnings, err
		}
	}
	if state.Hyperlink != "" {
		linkType := state.LinkType
		if linkType == "" {
			linkType = "External"
		}
		if err := f.SetCellHyperLink(ref.Sheet, axis, state.Hyperlink, linkType); err != nil {
			return warnings, err
		}
	} else {
		hasLink, _, err := f.GetCellHyperLink(ref.Sheet, axis)
		if err != nil {
			return warnings, err
		}
		if hasLink {
			if err := f.SetCellHyperLink(ref.Sheet, axis, "", "None"); err != nil {
				return warnings, err
			}
		}
	}
	return warnings, nil
}

func setRawValue(f *excelize.File, sheet, axis string, value workbook.CellValue) error {
	switch value.Type {
	case "bool":
		parsed, err := strconv.ParseBool(value.Raw)
		if err != nil {
			return fmt.Errorf("parse bool %q: %w", value.Raw, err)
		}
		return f.SetCellBool(sheet, axis, parsed)
	case "number", "date":
		if value.Raw == "" {
			return f.SetCellDefault(sheet, axis, "")
		}
		parsed, err := strconv.ParseFloat(value.Raw, 64)
		if err != nil {
			return f.SetCellDefault(sheet, axis, value.Raw)
		}
		return f.SetCellFloat(sheet, axis, parsed, -1, 64)
	case "inline-string", "string":
		return f.SetCellStr(sheet, axis, value.Raw)
	default:
		return f.SetCellDefault(sheet, axis, value.Raw)
	}
}
