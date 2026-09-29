// maxNameLen은 확장자를 뺀 글자 수 기준이다.

package appdir

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrNameEmpty    = errors.New("파일 이름 없음")
	ErrNameBadChar  = errors.New("쓸 수 없는 글자")
	ErrNameReserved = errors.New("예약된 이름")
	ErrNameTooLong  = errors.New("이름이 너무 김")
)

const badNameChars = `\/:*?"<>|`

const maxNameLen = 100

func DefaultFileName(now time.Time) string {
	return "운전강의_" + now.Format("20060102_1504")
}

// 앞뒤 공백과 .mp4, 끝의 점·공백을 떼고 빈 값·금지 글자·장치 이름·길이를 본다.
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

// 첫 점 앞부분을 대문자로 바꿔 CON, PRN, AUX, NUL, COM1~9, LPT1~9와 비교
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

// dir\name.mp4가 이미 있으면 _2, _3을 붙여 빈 이름을 찾는다.
func OutputPath(dir, name string) string {
	p := filepath.Join(dir, name+".mp4")
	for i := 2; fileExists(p); i++ {
		p = filepath.Join(dir, name+"_"+strconv.Itoa(i)+".mp4")
	}
	return p
}
