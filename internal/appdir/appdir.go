// Package appdir는 로그·작업 폴더와 바탕화면 경로를 정한다.
package appdir

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// AppName은 %LOCALAPPDATA% 아래 폴더 이름이다.
const AppName = "운전영상편집기"

// Base는 %LOCALAPPDATA%\운전영상편집기 경로다.
func Base() string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return filepath.Join(d, AppName)
	}
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, AppName)
	}
	return filepath.Join(os.TempDir(), AppName)
}

func Logs() string { return filepath.Join(Base(), "logs") }
func Work() string { return filepath.Join(Base(), "work") }

// NewWorkDir는 work 아래 이번 작업 폴더를 만든다.
func NewWorkDir(now time.Time) (string, error) {
	if err := os.MkdirAll(Work(), 0o755); err != nil {
		return "", err
	}
	base := filepath.Join(Work(), now.Format("20060102-150405"))
	dir := base
	for i := 2; ; i++ {
		err := os.Mkdir(dir, 0o755)
		if err == nil {
			return dir, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
		dir = base + "-" + strconv.Itoa(i)
	}
}

// CleanOld는 maxAge보다 오래된 항목을 지운 개수를 준다.
func CleanOld(dir string, maxAge time.Duration, now time.Time) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) < maxAge {
			continue
		}
		if os.RemoveAll(filepath.Join(dir, e.Name())) == nil {
			n++
		}
	}
	return n
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
