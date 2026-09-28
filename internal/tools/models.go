package tools

import (
	"path/filepath"
	"runtime"
)

// ORT 버전 (onnxruntime_go 헤더와 맞춤)
const ortVersion = "1.29.0"

// FaceModel은 bin\models 안 얼굴 모델 파일 이름이다.
const FaceModel = "face_yunet_2023mar.onnx"

// ORTLib는 bin 폴더의 onnxruntime 라이브러리 경로다.
func ORTLib() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(BinDir(), "onnxruntime.dll")
	}
	return filepath.Join(BinDir(), "libonnxruntime.so."+ortVersion)
}

// ModelPath는 bin\models 안 모델 경로다.
func ModelPath(name string) string {
	return filepath.Join(BinDir(), "models", name)
}
