package appdir

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Desktop은 OneDrive 이동까지 반영한 바탕화면 경로다.
func Desktop() string {
	if p, err := windows.KnownFolderPath(windows.FOLDERID_Desktop, 0); err == nil && p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Desktop")
}

// FreeBytes는 path가 있는 드라이브의 남은 용량이다. 알 수 없으면 -1.
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
