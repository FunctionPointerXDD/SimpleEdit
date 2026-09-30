// NotifyShell: 바탕화면(특히 OneDrive 동기화 폴더)은 ffmpeg가 만든 파일이나 이름 바꿔 넣은 파일을
// 늦게 보이거나 안 보일 때가 있어, 저장 뒤 탐색기에 직접 알린다(2026-09-30 사용자 신고).

package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const createNoWindow = 0x08000000

const (
	shcneCreate     = 0x00000002
	shcneUpdateItem = 0x00002000
	shcneUpdateDir  = 0x00001000
	shcnfPathW      = 0x0005
	shcnfFlush      = 0x1000
)

var shChangeNotify = windows.NewLazySystemDLL("shell32.dll").NewProc("SHChangeNotify")

// CREATE_NO_WINDOW로 콘솔 창이 뜨지 않게 한다.
func hideWindow(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}

type processTree struct {
	job windows.Handle
}

// KILL_ON_JOB_CLOSE Job Object에 넣어 자식까지 함께 끝나게 한다. 실패하면 nil
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

// 파일이면 /select로 선택해 탐색기를 연다. 종료는 기다리지 않는다.
func Reveal(path string) error {
	cmd := exec.Command("explorer.exe")
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
	} else {
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe "` + path + `"`}
	}
	return cmd.Start()
}

// GetLongPathName으로 ARTTAT~1 같은 8.3 이름을 긴 이름으로 바꾼다.
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

// 새 파일이면 만들어짐, 아니면 바뀜으로 알리고 들어 있는 폴더도 다시 읽게 한다.
func NotifyShell(path string, created bool) {
	event := uintptr(shcneUpdateItem)
	if created {
		event = shcneCreate
	}
	for _, n := range []struct {
		event uintptr
		path  string
	}{{event, path}, {shcneUpdateDir, filepath.Dir(path)}} {
		p, err := windows.UTF16PtrFromString(n.path)
		if err != nil {
			return
		}
		shChangeNotify.Call(n.event, shcnfPathW|shcnfFlush, uintptr(unsafe.Pointer(p)), 0)
	}
}
