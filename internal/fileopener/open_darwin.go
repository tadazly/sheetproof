package fileopener

import (
	"fmt"
	"os/exec"
)

func OpenDefault(path string) error {
	absolute, err := validateWorkbook(path)
	if err != nil {
		return err
	}
	if err := exec.Command("open", absolute).Start(); err != nil {
		return fmt.Errorf("open workbook with default application: %w", err)
	}
	return nil
}

func Reveal(path string) error {
	absolute, err := validateWorkbook(path)
	if err != nil {
		return err
	}
	if err := exec.Command("open", "-R", absolute).Start(); err != nil {
		return fmt.Errorf("show workbook in Finder: %w", err)
	}
	return nil
}
