package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tadazly/sheetproof/internal/workbook"
	"github.com/xuri/excelize/v2"
)

type SafeWriter struct{}

func (SafeWriter) Save(file *excelize.File, target string, expected *workbook.FileIdentity) (workbook.FileIdentity, error) {
	if err := workbook.ValidateXLSXPath(target); err != nil {
		return workbook.FileIdentity{}, err
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return workbook.FileIdentity{}, &workbook.Error{Code: workbook.ErrSave, Path: target, Err: err}
	}
	dir := filepath.Dir(abs)
	info, statErr := os.Stat(dir)
	if statErr != nil || !info.IsDir() {
		if statErr == nil {
			statErr = fmt.Errorf("not a directory")
		}
		return workbook.FileIdentity{}, &workbook.Error{Code: workbook.ErrSave, Path: dir, Err: statErr}
	}
	if info.Mode().Perm()&0o222 == 0 {
		return workbook.FileIdentity{}, &workbook.Error{Code: workbook.ErrUnwritable, Path: dir, Err: fmt.Errorf("target directory is not writable")}
	}
	if targetInfo, err := os.Stat(abs); err == nil && targetInfo.Mode().Perm()&0o222 == 0 {
		return workbook.FileIdentity{}, &workbook.Error{Code: workbook.ErrUnwritable, Path: abs, Err: fmt.Errorf("target file is read-only")}
	}
	if expected != nil {
		current, err := workbook.Identity(abs)
		if err != nil {
			return workbook.FileIdentity{}, err
		}
		if current != *expected {
			return workbook.FileIdentity{}, &workbook.Error{
				Code: workbook.ErrExternalEdit, Path: abs,
				Err: fmt.Errorf("file changed since it was loaded"),
			}
		}
	}
	temp, err := os.CreateTemp(dir, ".sheetproof-*.xlsx")
	if err != nil {
		return workbook.FileIdentity{}, &workbook.Error{Code: workbook.ErrSave, Path: abs, Err: err}
	}
	tempPath := temp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temp.Close()
		}
		_ = os.Remove(tempPath)
	}()
	if err := file.Write(temp); err != nil {
		return workbook.FileIdentity{}, &workbook.Error{Code: workbook.ErrSave, Path: abs, Err: err}
	}
	if err := temp.Sync(); err != nil {
		return workbook.FileIdentity{}, &workbook.Error{Code: workbook.ErrSave, Path: abs, Err: err}
	}
	if err := temp.Close(); err != nil {
		return workbook.FileIdentity{}, &workbook.Error{Code: workbook.ErrSave, Path: abs, Err: err}
	}
	closed = true
	check, err := excelize.OpenFile(tempPath)
	if err != nil {
		return workbook.FileIdentity{}, &workbook.Error{Code: workbook.ErrSave, Path: abs, Err: fmt.Errorf("temporary workbook validation: %w", err)}
	}
	if len(check.GetSheetList()) == 0 {
		_ = check.Close()
		return workbook.FileIdentity{}, &workbook.Error{Code: workbook.ErrSave, Path: abs, Err: fmt.Errorf("temporary workbook has no worksheets")}
	}
	if err := check.Close(); err != nil {
		return workbook.FileIdentity{}, &workbook.Error{Code: workbook.ErrSave, Path: abs, Err: err}
	}
	if expected != nil {
		current, err := workbook.Identity(abs)
		if err != nil {
			return workbook.FileIdentity{}, err
		}
		if current != *expected {
			return workbook.FileIdentity{}, &workbook.Error{
				Code: workbook.ErrExternalEdit, Path: abs,
				Err: fmt.Errorf("file changed while saving"),
			}
		}
	}
	mode := os.FileMode(0o644)
	if existing, err := os.Stat(abs); err == nil {
		mode = existing.Mode().Perm()
	}
	if err := os.Chmod(tempPath, mode); err != nil {
		return workbook.FileIdentity{}, &workbook.Error{Code: workbook.ErrSave, Path: abs, Err: err}
	}
	if err := replaceFile(tempPath, abs); err != nil {
		return workbook.FileIdentity{}, &workbook.Error{Code: workbook.ErrSave, Path: abs, Err: err}
	}
	if dirHandle, err := os.Open(dir); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}
	id, err := workbook.Identity(abs)
	if err != nil {
		return workbook.FileIdentity{}, err
	}
	reopened, err := excelize.OpenFile(abs)
	if err != nil {
		return workbook.FileIdentity{}, &workbook.Error{Code: workbook.ErrSave, Path: abs, Err: fmt.Errorf("saved workbook validation: %w", err)}
	}
	_ = reopened.Close()
	return id, nil
}

func CopyFile(source, target string) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(target)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}
