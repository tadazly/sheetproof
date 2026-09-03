package fileopener

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateWorkbook(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("workbook path is empty")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve workbook path: %w", err)
	}
	absolute = filepath.Clean(absolute)
	stat, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("access workbook %s: %w", absolute, err)
	}
	if stat.IsDir() || !strings.EqualFold(filepath.Ext(absolute), ".xlsx") {
		return "", fmt.Errorf("workbook must be an existing .xlsx file: %s", absolute)
	}
	return absolute, nil
}
