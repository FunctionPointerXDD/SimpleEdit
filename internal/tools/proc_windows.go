package tools

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const createNoWindow = 0x08000000

// hideWindow는 외부 도구 실행 시 검은 콘솔 창이 뜨지 않게 한다.
func hideWindow(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}

// processTree는 자식까지 Job Object로 묶어 함께 끝낸다.
type processTree struct {
	job windows.Handle
}

func newProcessTree(p *os.Process) *processTree {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return nil
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(p.Pid))
	if err != nil {
		windows.CloseHandle(job)
		return nil
	}
	defer windows.CloseHandle(h)
	if err := windows.AssignProcessToJobObject(job, h); err != nil {
		windows.CloseHandle(job)
		return nil
	}
	return &processTree{job: job}
}

func (t *processTree) kill() {
	if t != nil {
		_ = windows.TerminateJobObject(t.job, 1)
	}
}

func (t *processTree) close() {
	if t != nil && t.job != 0 {
		windows.CloseHandle(t.job)
		t.job = 0
	}
}

// Reveal은 탐색기로 path를 선택해 연다. 종료는 안 기다림
func Reveal(path string) error {
	cmd := exec.Command("explorer.exe")
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
	} else {
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe "` + path + `"`}
	}
	return cmd.Start()
}

// longPath는 ARTTAT~1 같은 8.3 이름을 긴 이름으로 바꾼다.
func longPath(p string) string {
	src, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return p
	}
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetLongPathName(src, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) > len(buf) {
		return p
	}
	return windows.UTF16ToString(buf[:n])
}
