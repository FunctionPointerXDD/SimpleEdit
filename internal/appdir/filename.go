package appdir

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// 파일 이름 오류. app에서 안내 문장으로 바꾼다.
var (
	ErrNameEmpty    = errors.New("파일 이름 없음")
	ErrNameBadChar  = errors.New("쓸 수 없는 글자")
	ErrNameReserved = errors.New("예약된 이름")
	ErrNameTooLong  = errors.New("이름이 너무 김")
)

// Windows 파일 이름에 쓸 수 없는 글자
const badNameChars = `\/:*?"<>|`

// 이름 길이 상한 (확장자 제외, 글자 수)
const maxNameLen = 100

// DefaultFileName은 확장자 없는 기본 파일 이름이다.
func DefaultFileName(now time.Time) string {
	return "운전강의_" + now.Format("20060102_1504")
}

// CleanFileName은 이름을 검사하고 .mp4를 뗀 값을 준다.
func CleanFileName(name string) (string, error) {
	s := strings.TrimSpace(name)
	if strings.EqualFold(filepath.Ext(s), ".mp4") {
		s = strings.TrimSpace(s[:len(s)-4])
	}
	s = strings.TrimRight(s, ". ")
	switch {
	case s == "":
		return "", ErrNameEmpty
	case strings.ContainsAny(s, badNameChars) || strings.ContainsFunc(s, isControl):
		return "", ErrNameBadChar
	case isReserved(s):
		return "", ErrNameReserved
	case utf8.RuneCountInString(s) > maxNameLen:
		return "", ErrNameTooLong
	}
	return s, nil
}

func isControl(r rune) bool { return r < 32 }

// isReserved는 CON, NUL, COM1 같은 장치 이름인지 본다.
func isReserved(s string) bool {
	base, _, _ := strings.Cut(strings.ToUpper(s), ".")
	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) {
		return base[3] >= '1' && base[3] <= '9'
	}
	return false
}

// OutputPath는 dir\name.mp4. 이미 있으면 _2, _3을 붙인다.
func OutputPath(dir, name string) string {
	p := filepath.Join(dir, name+".mp4")
	for i := 2; fileExists(p); i++ {
		p = filepath.Join(dir, name+"_"+strconv.Itoa(i)+".mp4")
	}
	return p
}
