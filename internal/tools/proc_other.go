//go:build !windows

package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// hideWindow는 트리 종료용 프로세스 그룹을 만든다.
func hideWindow(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

type processTree struct{ pgid int }

func newProcessTree(p *os.Process) *processTree { return &processTree{pgid: p.Pid} }

func (t *processTree) kill() {
	if t != nil {
		_ = syscall.Kill(-t.pgid, syscall.SIGKILL)
	}
}

func (t *processTree) close() {}

// Reveal은 개발 환경에서 path가 있는 폴더를 연다.
func Reveal(path string) error {
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		path = filepath.Dir(path)
	}
	return exec.Command("xdg-open", path).Start()
}

// longPath는 Windows 밖에서는 그대로 돌려준다.
func longPath(p string) string { return p }
