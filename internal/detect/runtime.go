// ErrRuntime은 주로 VC++ 런타임이 없어 onnxruntime.dll을 못 올릴 때 난다.

package detect

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

var ErrRuntime = errors.New("onnxruntime을 불러오지 못함")

var (
	initMu  sync.Mutex
	initLib string
)

// 같은 경로로 이미 올렸으면 넘어가고, DLL 폴더를 검색 경로에 넣은 뒤 환경을 만든다.
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

// 초기화된 경우에만 환경을 해제하고 경로 기록을 지운다.
func Shutdown() error {
	initMu.Lock()
	defer initMu.Unlock()
	if !ort.IsInitialized() {
		return nil
	}
	initLib = ""
	return ort.DestroyEnvironment()
}
