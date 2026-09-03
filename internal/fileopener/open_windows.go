package fileopener

import (
	"fmt"
	"os/exec"

	"golang.org/x/sys/windows"
)

func OpenDefault(path string) error {
	absolute, err := validateWorkbook(path)
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(absolute)
	if err != nil {
		return err
	}
	if err := windows.ShellExecute(0, nil, file, nil, nil, 1); err != nil {
		return fmt.Errorf("open workbook with default application: %w", err)
	}
	return nil
}

func Reveal(path string) error {
	absolute, err := validateWorkbook(path)
	if err != nil {
		return err
	}
	if err := exec.Command("explorer.exe", "/select,", absolute).Start(); err != nil {
		return fmt.Errorf("show workbook in File Explorer: %w", err)
	}
	return nil
}
