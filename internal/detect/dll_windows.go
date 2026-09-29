package detect

import "golang.org/x/sys/windows"

// SetDllDirectory로 onnxruntime.dll 옆의 VC++ DLL을 먼저 찾게 한다.
func addDLLDir(dir string) {
	_ = windows.SetDllDirectory(dir)
}
