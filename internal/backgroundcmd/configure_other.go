//go:build !windows

package backgroundcmd

import "os/exec"

func configure(_ *exec.Cmd) {}
