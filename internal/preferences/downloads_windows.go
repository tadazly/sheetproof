//go:build windows

package preferences

import "golang.org/x/sys/windows"

func platformDownloadsDirectory() string {
	path, err := windows.KnownFolderPath(windows.FOLDERID_Downloads, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return ""
	}
	return path
}
