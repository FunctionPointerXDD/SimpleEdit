// EnvBinDir 환경 변수가 있으면 bin\ 대신 그 폴더를 쓴다.
// waitDelay는 손자 프로세스가 파이프를 잡고 있어도 Wait가 끝나게 하는 여유다.
// Cmd는 콘솔 창 없이 실행되고, 취소하면 자식 프로세스까지 함께 끝난다.

// Package tools는 bin\ 도구를 콘솔 창 없이 실행한다.
package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"simpleEdit/internal/applog"
)

const (
	YtDlp   = "yt-dlp"
	FFmpeg  = "ffmpeg"
	FFprobe = "ffprobe"
)

const EnvBinDir = "SIMPLEEDIT_BIN"

var ErrMissing = errors.New("도구 파일 없음")

const waitDelay = 3 * time.Second

// SIMPLEEDIT_BIN이 있으면 그 폴더, 없으면 실행 파일(링크 해석) 옆 bin
func BinDir() string {
	if d := os.Getenv(EnvBinDir); d != "" {
		return d
	}
	exe, err := os.Executable()
	if err != nil {
		return "bin"
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return filepath.Join(filepath.Dir(exe), "bin")
}

// Windows에서는 .exe를 붙인다.
func Path(name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(BinDir(), name)
}

type Cmd struct {
	*exec.Cmd
	name   string
	stderr tailBuffer
	tree   *processTree
	mu     sync.Mutex
}

// PATH 앞에 bin을 넣고, 취소되면 Job Object(또는 프로세스 그룹)째 끝낸다.
func Command(ctx context.Context, name string, args ...string) *Cmd {
	c := &Cmd{name: name}
	c.Cmd = exec.CommandContext(ctx, Path(name), args...)
	c.Cmd.Env = append(os.Environ(),
		"PATH="+BinDir()+string(os.PathListSeparator)+os.Getenv("PATH"),
		"PYTHONIOENCODING=utf-8",
	)
	c.Cmd.Stderr = &c.stderr
	c.Cmd.Cancel = func() error {
		c.mu.Lock()
		t := c.tree
		c.mu.Unlock()
		t.kill()
		return c.Cmd.Process.Kill()
	}
	c.Cmd.WaitDelay = waitDelay
	hideWindow(c.Cmd)
	return c
}

// 실행 파일이 없으면 ErrMissing. 시작하면 프로세스 트리에 묶는다.
func (c *Cmd) Start() error {
	if _, err := os.Stat(c.Path); err != nil {
		applog.Printf("[%s] 실행 파일 없음: %s", c.name, c.Path)
		return fmt.Errorf("%w: %s", ErrMissing, c.Path)
	}
	applog.Printf("[%s] 실행: %s", c.name, quoteArgs(c.Args))
	if err := c.Cmd.Start(); err != nil {
		applog.Printf("[%s] 시작 실패: %v", c.name, err)
		return err
	}
	t := newProcessTree(c.Process)
	c.mu.Lock()
	c.tree = t
	c.mu.Unlock()
	return nil
}

// 종료를 기다린 뒤 트리를 닫고 stderr 끝부분과 종료 코드를 로그에 남긴다.
func (c *Cmd) Wait() error {
	err := c.Cmd.Wait()
	c.mu.Lock()
	c.tree.close()
	c.mu.Unlock()
	if s := strings.TrimSpace(c.Stderr()); s != "" {
		applog.Printf("[%s] stderr:\n%s", c.name, s)
	}
	if err != nil {
		applog.Printf("[%s] 종료: %v", c.name, err)
	} else {
		applog.Printf("[%s] 정상 종료", c.name)
	}
	return err
}

func (c *Cmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

func (c *Cmd) Output() ([]byte, error) {
	var out bytes.Buffer
	c.Cmd.Stdout = &out
	err := c.Run()
	return out.Bytes(), err
}

// 마지막 64KB만 남긴 stderr
func (c *Cmd) Stderr() string { return c.stderr.String() }

func quoteArgs(args []string) string {
	q := make([]string, len(args))
	for i, a := range args {
		if a == "" || strings.ContainsAny(a, " \t\"") {
			a = `"` + strings.ReplaceAll(a, `"`, `\"`) + `"`
		}
		q[i] = a
	}
	return strings.Join(q, " ")
}

type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
}

const tailMax = 64 << 10

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > tailMax {
		t.buf = append(t.buf[:0], t.buf[len(t.buf)-tailMax:]...)
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}
