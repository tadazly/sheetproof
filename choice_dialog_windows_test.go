//go:build windows

package main

import (
	"reflect"
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
