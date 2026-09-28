//go:build !windows

package appdir

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Desktop은 개발 환경용 대체 경로다. ~/Desktop이 없으면 홈 폴더.
func Desktop() string {
	home, _ := os.UserHomeDir()
	d := filepath.Join(home, "Desktop")
	if fileExists(d) {
		return d
	}
	return home
}

// FreeBytes는 path가 있는 파일시스템의 남은 용량이다. 알 수 없으면 -1.
func FreeBytes(path string) int64 {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return -1
	}
	return int64(st.Bavail) * int64(st.Bsize)
}
