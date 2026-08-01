//go:build windows

package main

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

func TestRunTaskDialogInitializesSTAOnOneThread(t *testing.T) {
	original := taskDialogNative
	t.Cleanup(func() { taskDialogNative = original })

	events := []string{}
	threadIDs := []uint32{}
	taskDialogNative = nativeTaskDialogCalls{
		initializeSTA: func() uintptr {
			events = append(events, "initialize")
			threadIDs = append(threadIDs, windows.GetCurrentThreadId())
			return 0
		},
		show: func(_ *taskDialogConfig, selected *int32) uintptr {
			events = append(events, "show")
			threadIDs = append(threadIDs, windows.GetCurrentThreadId())
			*selected = taskDialogFirstButtonID
			return 0
		},
		uninitialize: func() {
			events = append(events, "uninitialize")
			threadIDs = append(threadIDs, windows.GetCurrentThreadId())
		},
	}

	selected, result, err := runTaskDialog(&taskDialogConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if selected != taskDialogFirstButtonID || result != 0 {
		t.Fatalf("selected = %d, result = %#x", selected, result)
	}
	if !reflect.DeepEqual(events, []string{"initialize", "show", "uninitialize"}) {
		t.Fatalf("events = %v", events)
	}
	if threadIDs[0] != threadIDs[1] || threadIDs[1] != threadIDs[2] {
		t.Fatalf("native calls used different threads: %v", threadIDs)
	}
}

func TestRunTaskDialogAcceptsAlreadyInitializedSTA(t *testing.T) {
	original := taskDialogNative
	t.Cleanup(func() { taskDialogNative = original })

	uninitialized := false
	shown := false
	taskDialogNative = nativeTaskDialogCalls{
		initializeSTA: func() uintptr { return 1 }, // S_FALSE
		show: func(_ *taskDialogConfig, _ *int32) uintptr {
			shown = true
			return 0
		},
		uninitialize: func() { uninitialized = true },
	}

	if _, _, err := runTaskDialog(&taskDialogConfig{}); err != nil {
		t.Fatal(err)
	}
	if !shown || !uninitialized {
		t.Fatalf("shown = %t, uninitialized = %t", shown, uninitialized)
	}
}

func TestRunTaskDialogRejectsIncompatibleApartment(t *testing.T) {
	original := taskDialogNative
	t.Cleanup(func() { taskDialogNative = original })

	shown := false
	taskDialogNative = nativeTaskDialogCalls{
		initializeSTA: func() uintptr { return 0x80010106 }, // RPC_E_CHANGED_MODE
		show: func(_ *taskDialogConfig, _ *int32) uintptr {
			shown = true
			return 0
		},
		uninitialize: func() {},
	}

	if _, _, err := runTaskDialog(&taskDialogConfig{}); err == nil {
		t.Fatal("expected COM apartment error")
	}
	if shown {
		t.Fatal("dialog was shown without an STA")
	}
}

func TestPrepareTaskDialogPreservesThreeCustomChoices(t *testing.T) {
	options := runtime.MessageDialogOptions{
		Title:         "当前表格存在未保存的修改",
		Message:       "请选择后续操作。",
		Buttons:       []string{"保存并继续", "不保存并继续", "取消"},
		DefaultButton: "保存并继续",
		CancelButton:  "取消",
	}
	prepared, err := prepareTaskDialog(options)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.config.buttonCount != 3 {
		t.Fatalf("button count = %d", prepared.config.buttonCount)
	}
	if prepared.config.defaultButton != taskDialogFirstButtonID {
		t.Fatalf("default button = %d", prepared.config.defaultButton)
	}
	if prepared.config.parent == 0 && prepared.config.flags&taskDialogPositionRelative != 0 {
		t.Fatal("relative-position flag set without a parent window")
	}
	for index, want := range options.Buttons {
		button := prepared.buttons[index]
		if got := windows.UTF16PtrToString(button.text); got != want {
			t.Fatalf("button %d = %q, want %q", index, got, want)
		}
		if got := prepared.answers[button.id]; got != want {
			t.Fatalf("answer %d = %q, want %q", button.id, got, want)
		}
	}
	if prepared.cancelAnswer != "取消" {
		t.Fatalf("cancel answer = %q", prepared.cancelAnswer)
	}
}

func TestPrepareMessageBoxMapsConfigureAndCancel(t *testing.T) {
	prepared, err := prepareMessageBoxChoice(runtime.MessageDialogOptions{
		Type: runtime.QuestionDialog, Title: "配置 UGit", Message: "只替换 *.xlsx。",
		Buttons: []string{"取消", "配置 UGit"}, DefaultButton: "配置 UGit", CancelButton: "取消",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := prepared.answers[standardOKButtonID]; got != "配置 UGit" {
		t.Fatalf("OK answer = %q", got)
	}
	if got := prepared.answers[standardCancelButtonID]; got != "取消" {
		t.Fatalf("Cancel answer = %q", got)
	}
	if prepared.flags&messageBoxOKCancel == 0 {
		t.Fatalf("flags = %#x", prepared.flags)
	}
	if prepared.flags&messageBoxDefaultButton2 != 0 {
		t.Fatalf("configure should be the default button: flags = %#x", prepared.flags)
	}
	message := windows.UTF16PtrToString(prepared.message)
	if want := "点击“确定”：配置 UGit"; !strings.Contains(message, want) {
		t.Fatalf("message = %q, want it to contain %q", message, want)
	}
}

func TestPrepareMessageBoxMapsUnsavedChoices(t *testing.T) {
	prepared, err := prepareMessageBoxChoice(runtime.MessageDialogOptions{
		Type: runtime.QuestionDialog, Title: "存在未保存修改", Message: "请选择后续操作。",
		Buttons:       []string{"保存并继续", "不保存并继续", "取消"},
		DefaultButton: "保存并继续", CancelButton: "取消",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[int32]string{
		standardYesButtonID: "保存并继续", standardNoButtonID: "不保存并继续", standardCancelButtonID: "取消",
	}
	if !reflect.DeepEqual(prepared.answers, want) {
		t.Fatalf("answers = %#v, want %#v", prepared.answers, want)
	}
	if prepared.flags&messageBoxYesNoCancel != messageBoxYesNoCancel {
		t.Fatalf("flags = %#x", prepared.flags)
	}
}

func TestShowMessageBoxChoiceReturnsMappedAnswer(t *testing.T) {
	original := messageBoxNative
	t.Cleanup(func() { messageBoxNative = original })
	messageBoxNative = func(_ windows.HWND, _, _ *uint16, _ uint32) int32 {
		return standardOKButtonID
	}
	answer, err := showMessageBoxChoice(runtime.MessageDialogOptions{
		Title: "配置 UGit", Message: "确认配置。", Buttons: []string{"取消", "配置 UGit"},
		DefaultButton: "配置 UGit", CancelButton: "取消",
	})
	if err != nil {
		t.Fatal(err)
	}
	if answer != "配置 UGit" {
		t.Fatalf("answer = %q", answer)
	}
}

func TestShowChoiceDialogFallsBackAfterTaskDialogInvalidArgument(t *testing.T) {
	originalTaskDialog := taskDialogNative
	originalMessageBox := messageBoxNative
	originalFind := findTaskDialogProc
	t.Cleanup(func() {
		taskDialogNative = originalTaskDialog
		messageBoxNative = originalMessageBox
		findTaskDialogProc = originalFind
	})
	findTaskDialogProc = func() error { return nil }
	taskDialogNative = nativeTaskDialogCalls{
		initializeSTA: func() uintptr { return 0 },
		uninitialize:  func() {},
		show: func(_ *taskDialogConfig, _ *int32) uintptr {
			return 0x80070057
		},
	}
	messageBoxNative = func(_ windows.HWND, _, _ *uint16, _ uint32) int32 {
		return standardOKButtonID
	}
	answer, err := showChoiceDialog(context.Background(), runtime.MessageDialogOptions{
		Type: runtime.QuestionDialog, Title: "配置 UGit", Message: "确认配置。",
		Buttons: []string{"取消", "配置 UGit"}, DefaultButton: "配置 UGit", CancelButton: "取消",
	})
	if err != nil {
		t.Fatal(err)
	}
	if answer != "配置 UGit" {
		t.Fatalf("answer = %q", answer)
	}
}

func TestShowChoiceDialogFallsBackWhenSTAInitializationIsUnavailable(t *testing.T) {
	originalTaskDialog := taskDialogNative
	originalMessageBox := messageBoxNative
	originalFind := findTaskDialogProc
	t.Cleanup(func() {
		taskDialogNative = originalTaskDialog
		messageBoxNative = originalMessageBox
		findTaskDialogProc = originalFind
	})
	findTaskDialogProc = func() error { return nil }
	taskDialogNative = nativeTaskDialogCalls{
		initializeSTA: func() uintptr { return 0x80010106 },
		uninitialize:  func() {},
		show: func(_ *taskDialogConfig, _ *int32) uintptr {
			t.Fatal("TaskDialog must not be shown without an STA")
			return 0
		},
	}
	messageBoxNative = func(_ windows.HWND, _, _ *uint16, _ uint32) int32 {
		return standardCancelButtonID
	}
	answer, err := showChoiceDialog(context.Background(), runtime.MessageDialogOptions{
		Type: runtime.QuestionDialog, Title: "配置 UGit", Message: "确认配置。",
		Buttons: []string{"取消", "配置 UGit"}, DefaultButton: "配置 UGit", CancelButton: "取消",
	})
	if err != nil {
		t.Fatal(err)
	}
	if answer != "取消" {
		t.Fatalf("answer = %q", answer)
	}
}
