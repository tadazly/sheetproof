//go:build windows

package backgroundcmd

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func configure(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}
