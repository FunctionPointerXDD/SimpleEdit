// Package appdir는 로그·작업 폴더와 바탕화면 경로를 정한다.
package appdir

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const AppName = "운전영상편집기"

// LOCALAPPDATA → 사용자 캐시 → 임시 폴더 순으로 앞의 것을 쓴다.
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

// work\날짜-시각 폴더를 만들고, 이미 있으면 -2, -3을 붙인다.
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

// dir 바로 아래 항목 중 수정 시각이 maxAge를 넘은 것을 지운 개수
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
