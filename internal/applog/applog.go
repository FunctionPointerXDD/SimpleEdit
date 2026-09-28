// Package applog는 app.log 한 파일에 외부 명령과 stderr를 기록한다.
package applog

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

const maxSize = 5 << 20 // 5MB를 넘으면 app.old.log로 돌린다

var (
	mu     sync.Mutex
	logger = log.New(io.Discard, "", log.LstdFlags)
	file   *os.File
)

// Open은 dir\app.log를 열어 이후 기록을 그 파일로 보낸다.
func Open(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "app.log")
	if st, err := os.Stat(path); err == nil && st.Size() > maxSize {
		_ = os.Rename(path, filepath.Join(dir, "app.old.log"))
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		file.Close()
	}
	file = f
	logger.SetOutput(f)
	return nil
}

// SetOutput은 테스트나 CLI에서 기록 대상을 바꿀 때 쓴다.
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	logger.SetOutput(w)
}

func Printf(format string, args ...any) {
	logger.Printf(format, args...)
}

func Close() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		file.Close()
		file = nil
	}
	logger.SetOutput(io.Discard)
}
