// ortVersion은 onnxruntime_go 헤더 버전과 맞춘다.

package tools

import (
	"path/filepath"
	"runtime"
)

const ortVersion = "1.29.0"

const (
	FaceModel  = "face_yunet_2026may.onnx"
	PlateModel = "plate_yolox_tiny.onnx"
)

// Windows는 bin\onnxruntime.dll, 그 밖은 bin/libonnxruntime.so.버전
func ORTLib() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(BinDir(), "onnxruntime.dll")
	}
	return filepath.Join(BinDir(), "libonnxruntime.so."+ortVersion)
}

func ModelPath(name string) string {
	return filepath.Join(BinDir(), "models", name)
}
