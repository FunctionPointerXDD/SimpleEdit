package tools

import "testing"

func TestIsArchiveRun(t *testing.T) {
	const temp = `C:\Users\강사\AppData\Local\Temp`
	tests := []struct {
		name   string
		exeDir string
		want   bool
	}{
		{"탐색기에서 zip 안 실행", `C:\Users\강사\AppData\Local\Temp\Temp1_운전영상편집기.zip\운전영상편집기`, true},
		{"임시 폴더 아래 다른 압축 프로그램", `C:\Users\강사\AppData\Local\Temp\7zO8A1B\운전영상편집기`, true},
		{"대소문자 다름", `c:\users\강사\appdata\local\TEMP\Bandizip\a`, true},
		{"임시 폴더 밖 .zip 경로", `D:\받은 파일\운전영상편집기.ZIP\운전영상편집기`, true},
		{"바탕화면에 압축 풂", `C:\Users\강사\Desktop\운전영상편집기`, false},
		{"이름만 Temp로 시작", `C:\Users\강사\AppData\Local\Temp2\운전영상편집기`, false},
		{"폴더 이름에 zip 글자", `C:\Users\강사\Documents\zip파일\운전영상편집기`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isArchiveRun(tt.exeDir, temp); got != tt.want {
				t.Errorf("isArchiveRun(%q) = %v, want %v", tt.exeDir, got, tt.want)
			}
		})
	}
	if isArchiveRun(`C:\a`, "") {
		t.Error("임시 폴더를 모르면 .zip 경로만 봐야 함")
	}
}
