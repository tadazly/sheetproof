//go:build !windows

package preferences

import (
	"os"
	"path/filepath"
)

func platformDownloadsDirectory() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Downloads")
}
