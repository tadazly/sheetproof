package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/xuri/excelize/v2"
)

type Pair struct {
	Left  string
	Right string
}

func CreatePair(dir string) (Pair, error) {
	left := filepath.Join(dir, "左侧 文件.xlsx")
	right := filepath.Join(dir, "右侧 文件.xlsx")
	if err := createWorkbook(left, false); err != nil {
		return Pair{}, err
	}
	if err := createWorkbook(right, true); err != nil {
		return Pair{}, err
	}
	return Pair{Left: left, Right: right}, nil
}

func createWorkbook(path string, right bool) error {
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName("Sheet1", "数据 表")
	values := map[string]any{
		"A1": "旧文本", "B1": 0, "C1": true, "G1": "中文内容", "H1": "保留",
		"A2": "special <>&\"'", "B2": time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
		"J1": "合并内容",
	}
	if right {
		values["A1"] = "新文本"
		values["B1"] = 42
		values["C1"] = false
		values["G1"] = "另一内容"
	}
	for axis, value := range values {
		if err := f.SetCellValue("数据 表", axis, value); err != nil {
			return err
		}
	}
	if err := f.SetCellStr("数据 表", "E1", ""); err != nil {
		return err
	}
	formula := "SUM(B1,1)"
	if right {
		formula = "SUM(B1,2)"
	}
	if err := f.SetCellFormula("数据 表", "D1", formula); err != nil {
		return err
	}
	style, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"2F75B5"}, Pattern: 1},
	})
	if err != nil {
		return err
	}
	if err := f.SetCellStyle("数据 表", "A1", "A1", style); err != nil {
		return err
	}
	if err := f.MergeCell("数据 表", "J1", "K1"); err != nil {
		return err
	}
	if err := f.SetColWidth("数据 表", "A", "A", 24); err != nil {
		return err
	}
	if err := f.SetRowHeight("数据 表", 1, 28); err != nil {
		return err
	}
	hyperlink := "https://example.com"
	if right {
		hyperlink = "https://example.org/right"
	}
	if err := f.SetCellHyperLink("数据 表", "A2", hyperlink, "External"); err != nil {
		return err
	}
	comment := "测试批注"
	if right {
		comment = "右侧批注"
	}
	if err := f.AddComment("数据 表", excelize.Comment{Cell: "G1", Author: "ugxlsx-test", Text: comment}); err != nil {
		return err
	}
	if _, err := f.NewSheet("共同表"); err != nil {
		return err
	}
	if err := f.SetCellValue("共同表", "A1", "same"); err != nil {
		return err
	}
	if right {
		if _, err := f.NewSheet("右侧新增"); err != nil {
			return err
		}
		if err := f.SetCellValue("右侧新增", "A1", "only right"); err != nil {
			return err
		}
	} else {
		if _, err := f.NewSheet("左侧新增"); err != nil {
			return err
		}
		if err := f.SetCellValue("左侧新增", "A1", "only left"); err != nil {
			return err
		}
	}
	return f.SaveAs(path)
}

func CorruptFile(dir string) (string, error) {
	path := filepath.Join(dir, "损坏.xlsx")
	return path, os.WriteFile(path, []byte("not a zip workbook"), 0o644)
}

func CreateLarge(path string, sheets, cellsPerSheet int) error {
	f := excelize.NewFile()
	defer f.Close()
	for sheetIndex := 0; sheetIndex < sheets; sheetIndex++ {
		name := fmt.Sprintf("Sheet%d", sheetIndex+1)
		if sheetIndex == 0 {
			_ = f.SetSheetName("Sheet1", name)
		} else if _, err := f.NewSheet(name); err != nil {
			return err
		}
		writer, err := f.NewStreamWriter(name)
		if err != nil {
			return err
		}
		const columns = 20
		rows := (cellsPerSheet + columns - 1) / columns
		for row := 1; row <= rows; row++ {
			values := make([]any, columns)
			for col := range values {
				index := (row-1)*columns + col
				if index < cellsPerSheet {
					values[col] = index
				}
			}
			axis, _ := excelize.CoordinatesToCellName(1, row)
			if err := writer.SetRow(axis, values); err != nil {
				return err
			}
		}
		if err := writer.Flush(); err != nil {
			return err
		}
	}
	return f.SaveAs(path)
}
