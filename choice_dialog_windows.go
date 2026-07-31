//go:build windows

package main

import (
	"context"
	"fmt"
	goruntime "runtime"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

const (
	taskDialogAllowCancellation       = 0x0008
	taskDialogPositionRelative        = 0x1000
	taskDialogSizeToContent           = 0x01000000
	taskDialogFirstButtonID     int32 = 1000
	standardCancelButtonID      int32 = 2
)

var (
	taskDialogProc               = windows.NewLazySystemDLL("comctl32.dll").NewProc("TaskDialogIndirect")
	getForegroundWindowProc      = windows.NewLazySystemDLL("user32.dll").NewProc("GetForegroundWindow")
	getWindowThreadProcessIDProc = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowThreadProcessId")
)

type taskDialogButton struct {
	id   int32
	text *uint16
}

type taskDialogConfig struct {
	size                 uint32
	parent               windows.HWND
	instance             windows.Handle
	flags                uint32
	commonButtons        uint32
	windowTitle          *uint16
	mainIcon             uintptr
	mainInstruction      *uint16
	content              *uint16
	buttonCount          uint32
	buttons              *taskDialogButton
	defaultButton        int32
	radioButtonCount     uint32
	radioButtons         uintptr
	defaultRadioButton   int32
	verificationText     *uint16
	expandedInformation  *uint16
	expandedControlText  *uint16
	collapsedControlText *uint16
	footerIcon           uintptr
	footer               *uint16
	callback             uintptr
	callbackData         uintptr
	width                uint32
}

type preparedTaskDialog struct {
	config       taskDialogConfig
	buttons      []taskDialogButton
	labels       []*uint16
	answers      map[int32]string
	cancelAnswer string
}

func showChoiceDialog(ctx context.Context, options runtime.MessageDialogOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	prepared, err := prepareTaskDialog(options)
	if err != nil {
		return "", err
	}
	if err := taskDialogProc.Find(); err != nil {
		return "", fmt.Errorf("Windows 自定义选择对话框不可用: %w", err)
	}
	selected := int32(0)
	result, _, _ := taskDialogProc.Call(
		uintptr(unsafe.Pointer(&prepared.config)),
		uintptr(unsafe.Pointer(&selected)),
		0,
		0,
	)
	goruntime.KeepAlive(prepared)
	if result != 0 {
		return "", fmt.Errorf("显示 Windows 选择对话框失败: HRESULT 0x%08X", uint32(result))
	}
	if selected == standardCancelButtonID && prepared.cancelAnswer != "" {
		return prepared.cancelAnswer, nil
	}
	answer, ok := prepared.answers[selected]
	if !ok {
		return "", fmt.Errorf("Windows 选择对话框返回未知按钮: %d", selected)
	}
	return answer, nil
}

func prepareTaskDialog(options runtime.MessageDialogOptions) (preparedTaskDialog, error) {
	if len(options.Buttons) == 0 {
		return preparedTaskDialog{}, fmt.Errorf("Windows 选择对话框至少需要一个按钮")
	}
	title, err := windows.UTF16PtrFromString(options.Title)
	if err != nil {
		return preparedTaskDialog{}, err
	}
	message, err := windows.UTF16PtrFromString(options.Message)
	if err != nil {
		return preparedTaskDialog{}, err
	}
	prepared := preparedTaskDialog{
		buttons:      make([]taskDialogButton, 0, len(options.Buttons)),
		labels:       make([]*uint16, 0, len(options.Buttons)),
		answers:      make(map[int32]string, len(options.Buttons)),
		cancelAnswer: options.CancelButton,
	}
	defaultID := taskDialogFirstButtonID
	for index, answer := range options.Buttons {
		label, labelErr := windows.UTF16PtrFromString(answer)
		if labelErr != nil {
			return preparedTaskDialog{}, labelErr
		}
		id := taskDialogFirstButtonID + int32(index)
		prepared.labels = append(prepared.labels, label)
		prepared.buttons = append(prepared.buttons, taskDialogButton{id: id, text: label})
		prepared.answers[id] = answer
		if answer == options.DefaultButton {
			defaultID = id
		}
	}
	parent := currentProcessForegroundWindow()
	flags := uint32(taskDialogAllowCancellation | taskDialogSizeToContent)
	if parent != 0 {
		flags |= taskDialogPositionRelative
	}
	prepared.config = taskDialogConfig{
		size:          uint32(unsafe.Sizeof(taskDialogConfig{})),
		parent:        parent,
		flags:         flags,
		windowTitle:   title,
		content:       message,
		buttonCount:   uint32(len(prepared.buttons)),
		buttons:       &prepared.buttons[0],
		defaultButton: defaultID,
	}
	return prepared, nil
}

func currentProcessForegroundWindow() windows.HWND {
	window, _, _ := getForegroundWindowProc.Call()
	if window == 0 {
		return 0
	}
	var processID uint32
	getWindowThreadProcessIDProc.Call(window, uintptr(unsafe.Pointer(&processID)))
	if processID != windows.GetCurrentProcessId() {
		return 0
	}
	return windows.HWND(window)
}
