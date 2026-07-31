package backgroundcmd

import (
	"context"
	"os/exec"
)

// CommandContext creates a background CLI command with platform-appropriate
// process attributes. It is intentionally used only for child CLI processes,
// never for the desktop application itself.
func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, name, args...)
	configure(command)
	return command
}
