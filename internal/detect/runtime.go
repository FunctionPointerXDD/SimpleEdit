package detect

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// ErrRuntime은 런타임 로드 실패(주로 VC++ 런타임 없음)다.
var ErrRuntime = errors.New("onnxruntime을 불러오지 못함")

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
	addDLLDir(filepath.Dir(libPath))
	ort.SetSharedLibraryPath(libPath)
	if err := ort.InitializeEnvironment(); err != nil {
		return fmt.Errorf("%w: %v", ErrRuntime, err)
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
