//go:build !windows && !darwin

package fileopener

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

func OpenDefault(path string) error {
	absolute, err := validateWorkbook(path)
	if err != nil {
		return err
	}
	if err := exec.Command("xdg-open", absolute).Start(); err != nil {
		return fmt.Errorf("open workbook with default application: %w", err)
	}
	return nil
}

func Reveal(path string) error {
	absolute, err := validateWorkbook(path)
	if err != nil {
		return err
	}
	if err := exec.Command("xdg-open", filepath.Dir(absolute)).Start(); err != nil {
		return fmt.Errorf("show workbook in file manager: %w", err)
	}
	return nil
}
