package tools

import (
	"os"
	"path/filepath"
	"strings"
)

// RunningFromArchive는 zip 안에서 바로 실행했는지 추정한다.
func RunningFromArchive() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return isArchiveRun(longPath(filepath.Dir(exe)), longPath(os.TempDir()))
}

// isArchiveRun은 exe 폴더가 임시 폴더나 .zip 경로 안인지 본다.
func isArchiveRun(exeDir, tempDir string) bool {
	dir := normPath(exeDir)
	if strings.Contains(dir+"/", ".zip/") {
		return true
	}
	tmp := strings.TrimSuffix(normPath(tempDir), "/")
	return tmp != "" && strings.HasPrefix(dir+"/", tmp+"/")
}

// normPath는 구분자를 /로, 글자를 소문자로 맞춘다.
func normPath(p string) string {
	return strings.ToLower(strings.ReplaceAll(p, `\`, "/"))
}
