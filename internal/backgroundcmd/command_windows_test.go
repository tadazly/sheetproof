//go:build windows

package backgroundcmd

import (
	"context"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCommandContextPreventsBackgroundConsoleWindows(t *testing.T) {
	command := CommandContext(context.Background(), "git", "--version")
	if command.SysProcAttr == nil {
		t.Fatal("Windows process attributes are missing")
	}
	if command.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatalf("creation flags = %#x, want CREATE_NO_WINDOW", command.SysProcAttr.CreationFlags)
	}
}
