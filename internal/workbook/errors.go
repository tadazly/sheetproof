package workbook

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	ErrUnsupported  ErrorCode = "unsupported_format"
	ErrNotFound     ErrorCode = "file_not_found"
	ErrUnreadable   ErrorCode = "file_unreadable"
	ErrCorrupt      ErrorCode = "corrupt_workbook"
	ErrNoSheets     ErrorCode = "no_worksheets"
	ErrSameFile     ErrorCode = "same_file"
	ErrExternalEdit ErrorCode = "external_modification"
	ErrUnwritable   ErrorCode = "file_unwritable"
	ErrSave         ErrorCode = "save_failed"
)

type Error struct {
	Code ErrorCode
	Path string
	Err  error
}

func (e *Error) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("%s: %v", e.Code, e.Err)
	}
	return fmt.Sprintf("%s (%s): %v", e.Code, e.Path, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

func HasCode(err error, code ErrorCode) bool {
	var target *Error
	return errors.As(err, &target) && target.Code == code
}
