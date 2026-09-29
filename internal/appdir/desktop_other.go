//go:build !windows

package appdir

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// ~/Desktop이 없으면 홈 폴더를 쓴다.
func Desktop() string {
	home, _ := os.UserHomeDir()
	d := filepath.Join(home, "Desktop")
	if fileExists(d) {
		return d
	}
	return home
}

// statfs로 남은 블록 × 블록 크기. 실패하면 -1
func FreeBytes(path string) int64 {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return -1
	}
	return int64(st.Bavail) * int64(st.Bsize)
}
