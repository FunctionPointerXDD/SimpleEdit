package detect

import "golang.org/x/sys/windows"

// addDLLDir는 의존 DLL도 같은 폴더에서 찾게 한다.
func addDLLDir(dir string) {
	_ = windows.SetDllDirectory(dir)
}
