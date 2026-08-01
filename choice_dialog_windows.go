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
	messageBoxOK                      = 0x00000000
	messageBoxOKCancel                = 0x00000001
	messageBoxYesNoCancel             = 0x00000003
	messageBoxIconError               = 0x00000010
	messageBoxIconQuestion            = 0x00000020
	messageBoxIconWarning             = 0x00000030
	messageBoxIconInformation         = 0x00000040
	messageBoxDefaultButton2          = 0x00000100
	messageBoxDefaultButton3          = 0x00000200
	messageBoxSetForeground           = 0x00010000
	standardOKButtonID          int32 = 1
	standardCancelButtonID      int32 = 2
	standardYesButtonID         int32 = 6
	standardNoButtonID          int32 = 7
)

var (
	taskDialogProc               = windows.NewLazySystemDLL("comctl32.dll").NewProc("TaskDialogIndirect")
	coInitializeExProc           = windows.NewLazySystemDLL("ole32.dll").NewProc("CoInitializeEx")
	coUninitializeProc           = windows.NewLazySystemDLL("ole32.dll").NewProc("CoUninitialize")
	getForegroundWindowProc      = windows.NewLazySystemDLL("user32.dll").NewProc("GetForegroundWindow")
	getWindowThreadProcessIDProc = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowThreadProcessId")
	isWindowEnabledProc          = windows.NewLazySystemDLL("user32.dll").NewProc("IsWindowEnabled")
	isWindowVisibleProc          = windows.NewLazySystemDLL("user32.dll").NewProc("IsWindowVisible")
	messageBoxProc               = windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")
	findTaskDialogProc           = taskDialogProc.Find
	taskDialogNative             = nativeTaskDialogCalls{
		initializeSTA: func() uintptr {
			result, _, _ := coInitializeExProc.Call(
				0,
				windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE,
			)
			return result
		},
		uninitialize: func() {
			coUninitializeProc.Call()
		},
		show: func(config *taskDialogConfig, selected *int32) uintptr {
			result, _, _ := taskDialogProc.Call(
				uintptr(unsafe.Pointer(config)),
				uintptr(unsafe.Pointer(selected)),
				0,
				0,
			)
			return result
		},
	}
	messageBoxNative = func(parent windows.HWND, message, title *uint16, flags uint32) int32 {
		result, _, _ := messageBoxProc.Call(
			uintptr(parent),
			uintptr(unsafe.Pointer(message)),
			uintptr(unsafe.Pointer(title)),
			uintptr(flags),
		)
		return int32(result)
	}
)

type nativeTaskDialogCalls struct {
	initializeSTA func() uintptr
	uninitialize  func()
	show          func(config *taskDialogConfig, selected *int32) uintptr
}

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

type preparedMessageBox struct {
	parent  windows.HWND
	title   *uint16
	message *uint16
	flags   uint32
	answers map[int32]string
}

func showChoiceDialog(ctx context.Context, options runtime.MessageDialogOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	prepared, err := prepareTaskDialog(options)
	if err != nil {
		return "", err
	}
	if err := findTaskDialogProc(); err != nil {
		return fallbackMessageBoxChoice(options, fmt.Errorf("Windows 自定义选择对话框不可用: %w", err))
	}
	selected, result, err := runTaskDialog(&prepared.config)
	if err != nil {
		return fallbackMessageBoxChoice(options, err)
	}
	// TaskDialogIndirect rejects a hidden or disabled owner with E_INVALIDARG.
	// The Wails foreground window can change between preparing and showing the
	// dialog, so retry without an owner before using the standard Win32 fallback.
	if result != 0 && prepared.config.parent != 0 {
		detached := prepared.config
		detached.parent = 0
		detached.flags &^= taskDialogPositionRelative
		selected, result, err = runTaskDialog(&detached)
		if err != nil {
			return fallbackMessageBoxChoice(options, err)
		}
	}
	goruntime.KeepAlive(prepared)
	if result != 0 {
		return fallbackMessageBoxChoice(
			options,
			fmt.Errorf("显示 Windows 选择对话框失败: HRESULT 0x%08X", uint32(result)),
		)
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

func fallbackMessageBoxChoice(options runtime.MessageDialogOptions, taskDialogErr error) (string, error) {
	answer, fallbackErr := showMessageBoxChoice(options)
	if fallbackErr == nil {
		return answer, nil
	}
	return "", fmt.Errorf("%v；标准对话框回退也失败: %w", taskDialogErr, fallbackErr)
}

func showMessageBoxChoice(options runtime.MessageDialogOptions) (string, error) {
	prepared, err := prepareMessageBoxChoice(options)
	if err != nil {
		return "", err
	}
	selected := messageBoxNative(prepared.parent, prepared.message, prepared.title, prepared.flags)
	goruntime.KeepAlive(prepared)
	if selected == 0 {
		return "", fmt.Errorf("显示 Windows 标准选择对话框失败")
	}
	answer, ok := prepared.answers[selected]
	if !ok {
		return "", fmt.Errorf("Windows 标准选择对话框返回未知按钮: %d", selected)
	}
	return answer, nil
}

func prepareMessageBoxChoice(options runtime.MessageDialogOptions) (preparedMessageBox, error) {
	if len(options.Buttons) == 0 || len(options.Buttons) > 3 {
		return preparedMessageBox{}, fmt.Errorf("Windows 标准选择对话框只支持一到三个按钮")
	}
	title, err := windows.UTF16PtrFromString(options.Title)
	if err != nil {
		return preparedMessageBox{}, err
	}
	message := options.Message
	answers := make(map[int32]string, len(options.Buttons))
	flags := uint32(messageBoxSetForeground)
	switch options.Type {
	case runtime.ErrorDialog:
		flags |= messageBoxIconError
	case runtime.WarningDialog:
		flags |= messageBoxIconWarning
	case runtime.QuestionDialog:
		flags |= messageBoxIconQuestion
	default:
		flags |= messageBoxIconInformation
	}
	switch len(options.Buttons) {
	case 1:
		flags |= messageBoxOK
		answers[standardOKButtonID] = options.Buttons[0]
	case 2:
		confirmAnswer := options.Buttons[0]
		cancelAnswer := options.Buttons[1]
		if options.CancelButton != "" {
			for _, answer := range options.Buttons {
				if answer == options.CancelButton {
					cancelAnswer = answer
				} else {
					confirmAnswer = answer
				}
			}
		}
		message += fmt.Sprintf("\n\n点击“确定”：%s\n点击“取消”：%s", confirmAnswer, cancelAnswer)
		flags |= messageBoxOKCancel
		answers[standardOKButtonID] = confirmAnswer
		answers[standardCancelButtonID] = cancelAnswer
		if options.DefaultButton == cancelAnswer {
			flags |= messageBoxDefaultButton2
		}
	case 3:
		first, second, cancelAnswer := options.Buttons[0], options.Buttons[1], options.Buttons[2]
		if options.CancelButton != "" {
			remaining := make([]string, 0, 2)
			for _, answer := range options.Buttons {
				if answer == options.CancelButton {
					cancelAnswer = answer
				} else {
					remaining = append(remaining, answer)
				}
			}
			if len(remaining) == 2 {
				first, second = remaining[0], remaining[1]
			}
		}
		message += fmt.Sprintf("\n\n选择“是”：%s\n选择“否”：%s\n选择“取消”：%s", first, second, cancelAnswer)
		flags |= messageBoxYesNoCancel
		answers[standardYesButtonID] = first
		answers[standardNoButtonID] = second
		answers[standardCancelButtonID] = cancelAnswer
		switch options.DefaultButton {
		case second:
			flags |= messageBoxDefaultButton2
		case cancelAnswer:
			flags |= messageBoxDefaultButton3
		}
	}
	messageText, err := windows.UTF16PtrFromString(message)
	if err != nil {
		return preparedMessageBox{}, err
	}
	return preparedMessageBox{
		parent: currentProcessForegroundWindow(), title: title, message: messageText,
		flags: flags, answers: answers,
	}, nil
}

func runTaskDialog(config *taskDialogConfig) (int32, uintptr, error) {
	// TaskDialogIndirect requires a single-threaded apartment. Wails binding
	// calls are not guaranteed to stay on an initialized OS thread, so keep the
	// whole native dialog lifetime on one thread and balance COM initialization.
	goruntime.LockOSThread()
	defer goruntime.UnlockOSThread()

	initialization := taskDialogNative.initializeSTA()
	if initialization != 0 && initialization != 1 { // S_OK or S_FALSE
		return 0, 0, fmt.Errorf(
			"初始化 Windows 选择对话框线程失败: HRESULT 0x%08X",
			uint32(initialization),
		)
	}
	defer taskDialogNative.uninitialize()

	selected := int32(0)
	result := taskDialogNative.show(config, &selected)
	return selected, result, nil
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
	visible, _, _ := isWindowVisibleProc.Call(window)
	enabled, _, _ := isWindowEnabledProc.Call(window)
	if visible == 0 || enabled == 0 {
		return 0
	}
	return windows.HWND(window)
}
