//go:build !windows

package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// 창 대신 새 프로세스 그룹을 만들어 그룹째 끝낼 수 있게 한다.
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

// 파일이면 그 폴더를 xdg-open으로 연다.
func Reveal(path string) error {
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		path = filepath.Dir(path)
	}
	return exec.Command("xdg-open", path).Start()
}

func longPath(p string) string { return p }
