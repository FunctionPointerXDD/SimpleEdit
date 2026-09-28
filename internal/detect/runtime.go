package detect

import (
	"fmt"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

var (
	initMu  sync.Mutex
	initLib string
)

// Init은 ONNX Runtime 라이브러리를 한 번만 불러온다.
func Init(libPath string) error {
	initMu.Lock()
	defer initMu.Unlock()
	if ort.IsInitialized() {
		if libPath != initLib {
			return fmt.Errorf("이미 다른 경로로 초기화됨: %s", initLib)
		}
		return nil
	}
	ort.SetSharedLibraryPath(libPath)
	if err := ort.InitializeEnvironment(); err != nil {
		return fmt.Errorf("onnxruntime 초기화: %w", err)
	}
	initLib = libPath
	return nil
}

// Shutdown은 ONNX Runtime 환경을 정리한다.
func Shutdown() error {
	initMu.Lock()
	defer initMu.Unlock()
	if !ort.IsInitialized() {
		return nil
	}
	initLib = ""
	return ort.DestroyEnvironment()
}
