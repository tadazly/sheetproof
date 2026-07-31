//go:build windows

package main

import "testing"

func TestApplyIconHandlesSetsSmallSmall2AndBig(t *testing.T) {
	type message struct {
		window    uintptr
		parameter uintptr
		value     uintptr
	}
	var messages []message
	applyIconHandlesToWindow(10, 20, 30, func(window uintptr, messageID uint32, parameter, value uintptr) uintptr {
		if messageID != windowSetIconMessage {
			t.Fatalf("message = %#x", messageID)
		}
		messages = append(messages, message{window: window, parameter: parameter, value: value})
		return 0
	})
	want := []message{
		{window: 10, parameter: windowIconSmall, value: 30},
		{window: 10, parameter: windowIconSmall2, value: 30},
		{window: 10, parameter: windowIconBig, value: 20},
	}
	if len(messages) != len(want) {
		t.Fatalf("messages = %v", messages)
	}
	for index := range want {
		if messages[index] != want[index] {
			t.Fatalf("message %d = %+v, want %+v", index, messages[index], want[index])
		}
	}
}
