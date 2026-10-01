package appdir

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// KnownFolderPath로 OneDrive 이동까지 반영하고, 실패하면 ~/Desktop
func Desktop() string {
	if p, err := windows.KnownFolderPath(windows.FOLDERID_Desktop, 0); err == nil && p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop")
}

// GetDiskFreeSpaceEx의 사용자 가용 바이트. 실패하면 -1
func FreeBytes(path string) int64 {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return -1
	}
	var free uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, nil, nil); err != nil {
		return -1
	}
	return int64(free)
}
