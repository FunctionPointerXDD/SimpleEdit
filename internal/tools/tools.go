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

// 외부 도구 이름 (확장자 제외)
const (
	YtDlp   = "yt-dlp"
	FFmpeg  = "ffmpeg"
	FFprobe = "ffprobe"
)

// EnvBinDir가 있으면 bin\ 대신 그 폴더를 쓴다.
const EnvBinDir = "SIMPLEEDIT_BIN"

// ErrMissing은 도구 파일이 없을 때 반환된다.
var ErrMissing = errors.New("도구 파일 없음")

// 손자가 파이프를 잡아도 Wait가 끝나게 하는 여유
const waitDelay = 3 * time.Second

// BinDir는 외부 도구가 있는 폴더다. 기본값은 실행 파일 옆 bin\.
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

// Path는 도구 이름에 해당하는 실행 파일 경로다.
func Path(name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(BinDir(), name)
}

// Cmd는 콘솔 창 없이 실행되고, 취소하면 자식 프로세스까지 함께 종료되는 외부 명령이다.
type Cmd struct {
	*exec.Cmd
	name   string
	stderr tailBuffer
	tree   *processTree
	mu     sync.Mutex
}

// Command는 취소 시 트리 종료, PATH에 bin\ 추가
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

// Start는 프로세스를 시작하고 Job Object에 묶는다.
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

// Wait는 종료를 기다리고 stderr 끝부분을 로그에 남긴다.
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

// Run은 Start와 Wait를 이어서 부른다.
func (c *Cmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

// Output은 stdout 전체를 받아 반환한다.
func (c *Cmd) Output() ([]byte, error) {
	var out bytes.Buffer
	c.Cmd.Stdout = &out
	err := c.Run()
	return out.Bytes(), err
}

// Stderr는 stderr의 마지막 부분(최대 64KB)이다. 오류 원인 분류에 쓴다.
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

// tailBuffer는 마지막 64KB만 보관한다.
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
