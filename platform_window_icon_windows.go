//go:build windows

package main

import (
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wailsAppIconResourceID  = 3
	imageIcon               = 1
	windowSetIconMessage    = 0x0080
	windowIconSmall         = 0
	windowIconBig           = 1
	windowIconSmall2        = 2
	systemMetricIconWidth   = 11
	systemMetricIconHeight  = 12
	systemMetricSmallWidth  = 49
	systemMetricSmallHeight = 50
)

var (
	iconUser32              = windows.NewLazySystemDLL("user32.dll")
	iconKernel32            = windows.NewLazySystemDLL("kernel32.dll")
	loadImageProc           = iconUser32.NewProc("LoadImageW")
	enumWindowsProc         = iconUser32.NewProc("EnumWindows")
	getWindowProcessIDProc  = iconUser32.NewProc("GetWindowThreadProcessId")
	sendWindowMessageProc   = iconUser32.NewProc("SendMessageW")
	getSystemMetricsProc    = iconUser32.NewProc("GetSystemMetrics")
	getModuleHandleProc     = iconKernel32.NewProc("GetModuleHandleW")
	platformWindowBigIcon   uintptr
	platformWindowSmallIcon uintptr
)

type windowMessageSender func(window uintptr, message uint32, parameter, value uintptr) uintptr

// applyPlatformWindowIcon fills both WM_GETICON sizes. Wails 2.10.2 sets only
// ICON_SMALL, which lets the taskbar and window switcher fall back to a generic
// class icon even when the executable resource itself is correct.
func applyPlatformWindowIcon() {
	module, _, _ := getModuleHandleProc.Call(0)
	if module == 0 {
		return
	}
	bigWidth, _, _ := getSystemMetricsProc.Call(systemMetricIconWidth)
	bigHeight, _, _ := getSystemMetricsProc.Call(systemMetricIconHeight)
	smallWidth, _, _ := getSystemMetricsProc.Call(systemMetricSmallWidth)
	smallHeight, _, _ := getSystemMetricsProc.Call(systemMetricSmallHeight)
	big, _, _ := loadImageProc.Call(
		module, wailsAppIconResourceID, imageIcon, bigWidth, bigHeight, 0,
	)
	small, _, _ := loadImageProc.Call(
		module, wailsAppIconResourceID, imageIcon, smallWidth, smallHeight, 0,
	)
	if big == 0 || small == 0 {
		return
	}
	platformWindowBigIcon = big
	platformWindowSmallIcon = small
	currentProcessID := windows.GetCurrentProcessId()
	callback := windows.NewCallback(func(window uintptr, _ uintptr) uintptr {
		var processID uint32
		getWindowProcessIDProc.Call(window, uintptr(unsafe.Pointer(&processID)))
		if processID == currentProcessID {
			applyIconHandlesToWindow(window, big, small, sendWindowMessage)
		}
		return 1
	})
	enumWindowsProc.Call(callback, 0)
	runtime.KeepAlive(callback)
}

func sendWindowMessage(window uintptr, message uint32, parameter, value uintptr) uintptr {
	result, _, _ := sendWindowMessageProc.Call(window, uintptr(message), parameter, value)
	return result
}

func applyIconHandlesToWindow(window, big, small uintptr, send windowMessageSender) {
	send(window, windowSetIconMessage, windowIconSmall, small)
	send(window, windowSetIconMessage, windowIconSmall2, small)
	send(window, windowSetIconMessage, windowIconBig, big)
}
