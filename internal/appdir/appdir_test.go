package appdir

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOutputPathAvoidsOverwrite(t *testing.T) {
	dir := t.TempDir()
	name := DefaultFileName(time.Date(2026, 9, 28, 14, 5, 0, 0, time.Local))
	p1 := OutputPath(dir, name)
	if filepath.Base(p1) != "운전강의_20260928_1405.mp4" {
		t.Fatalf("이름 = %s", filepath.Base(p1))
	}
	os.WriteFile(p1, nil, 0o644)
	if p2 := OutputPath(dir, name); filepath.Base(p2) != "운전강의_20260928_1405_2.mp4" {
		t.Fatalf("두 번째 이름 = %s", filepath.Base(p2))
	}
}

func TestCleanFileName(t *testing.T) {
	ok := map[string]string{
		"사고 영상":          "사고 영상",
		"  교차로 우회전.mp4 ": "교차로 우회전",
		"강의.MP4":         "강의",
		"1강. ":           "1강",
		"CONSOLE":        "CONSOLE",
		"a.b":            "a.b",
	}
	for in, want := range ok {
		if got, err := CleanFileName(in); err != nil || got != want {
			t.Errorf("CleanFileName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	bad := map[string]error{
		"":                       ErrNameEmpty,
		"  .mp4":                 ErrNameEmpty,
		"...":                    ErrNameEmpty,
		"사고/영상":                  ErrNameBadChar,
		"질문?":                    ErrNameBadChar,
		"탭\t이름":                  ErrNameBadChar,
		"con":                    ErrNameReserved,
		"COM1.mp4":               ErrNameReserved,
		"lpt9.txt":               ErrNameReserved,
		strings.Repeat("가", 101): ErrNameTooLong,
	}
	for in, want := range bad {
		if _, err := CleanFileName(in); !errors.Is(err, want) {
			t.Errorf("CleanFileName(%q) err = %v, want %v", in, err, want)
		}
	}
}

func TestCleanOld(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old := filepath.Join(dir, "old")
	fresh := filepath.Join(dir, "fresh")
	os.MkdirAll(filepath.Join(old, "x"), 0o755)
	os.Mkdir(fresh, 0o755)
	os.Chtimes(old, now.Add(-8*24*time.Hour), now.Add(-8*24*time.Hour))

	if n := CleanOld(dir, 7*24*time.Hour, now); n != 1 {
		t.Fatalf("지운 개수 = %d", n)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("오래된 폴더가 남음")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("새 폴더가 지워짐")
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if got := LoadSettings(); got.OutputDirOrDesktop() != Desktop() {
		t.Fatalf("처음에는 바탕화면이어야 함: %+v", got)
	}
	dir := t.TempDir()
	if err := SaveSettings(Settings{OutputDir: dir}); err != nil {
		t.Fatal(err)
	}
	s := LoadSettings()
	if s.OutputDirOrDesktop() != dir {
		t.Errorf("저장 폴더 = %q, want %q", s.OutputDirOrDesktop(), dir)
	}
	os.RemoveAll(dir)
	if s.OutputDirOrDesktop() != Desktop() {
		t.Error("폴더가 사라지면 바탕화면이어야 함")
	}
}
