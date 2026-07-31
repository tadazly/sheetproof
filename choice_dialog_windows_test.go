//go:build windows

package main

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

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
