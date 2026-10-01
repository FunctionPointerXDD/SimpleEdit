package tools

import (
	"os"
	"path/filepath"
	"strings"
)

// exe 폴더와 임시 폴더를 8.3 이름 없이 긴 경로로 바꿔 isArchiveRun에 넘긴다.
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

// 경로에 .zip/이 있거나 임시 폴더 아래면 true
func isArchiveRun(exeDir, tempDir string) bool {
	dir := normPath(exeDir)
	if strings.Contains(dir+"/", ".zip/") {
		return true
	}
	tmp := strings.TrimSuffix(normPath(tempDir), "/")
	return tmp != "" && strings.HasPrefix(dir+"/", tmp+"/")
}

func normPath(p string) string {
	return strings.ToLower(strings.ReplaceAll(p, `\`, "/"))
}
