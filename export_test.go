package main

import (
	"os"
	"path/filepath"
	"testing"

	"simpleEdit/internal/media"
)

func TestExportTargetOverwritesOnlyLastSave(t *testing.T) {
	dir := t.TempDir()
	last := filepath.Join(dir, "강의_2.mp4")
	for _, p := range []string{filepath.Join(dir, "강의.mp4"), last, filepath.Join(dir, "다른.mp4")} {
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := &App{lastOutput: last, lastName: "강의", lastDir: dir}

	if out, replace := a.exportTarget(dir, "강의", true); out != last || !replace {
		t.Errorf("같은 이름·폴더는 마지막 파일을 바꿔야 함: %s %v", out, replace)
	}
	if out, replace := a.exportTarget(dir+string(filepath.Separator), "강의", true); out != last || !replace {
		t.Errorf("폴더 끝 구분자는 같은 폴더로 봐야 함: %s %v", out, replace)
	}
	if out, replace := a.exportTarget(dir, "강의", false); replace || out != filepath.Join(dir, "강의_3.mp4") {
		t.Errorf("덮어쓰기를 원하지 않으면 새 이름: %s %v", out, replace)
	}
	if out, replace := a.exportTarget(dir, "다른", true); replace || out != filepath.Join(dir, "다른_2.mp4") {
		t.Errorf("마지막 저장이 아닌 기존 파일은 덮어쓰면 안 됨: %s %v", out, replace)
	}
	if out, replace := a.exportTarget(t.TempDir(), "강의", true); replace || out == last {
		t.Errorf("폴더가 다르면 새 파일: %s %v", out, replace)
	}
	if out, replace := (&App{}).exportTarget(dir, "강의", true); replace || out == last {
		t.Errorf("저장한 적 없으면 덮어쓰지 않음: %s %v", out, replace)
	}
}

func TestSavingPath(t *testing.T) {
	if got := savingPath(filepath.Join("d", "강의.mp4")); got != filepath.Join("d", "강의.saving.mp4") {
		t.Errorf("임시 경로 %s", got)
	}
}

func TestManualRectsFrames(t *testing.T) {
	box := media.MaskRect{X: 0.1, Y: 0.1, W: 0.2, H: 0.2}
	whole, part, outside, unset := box, box, box, box
	whole.Start, whole.End = 10, 20
	part.Start, part.End = 12, 13.5
	outside.Start, outside.End = 21, 25
	manual := manualRects([]media.MaskRect{whole, part, outside, unset}, 10, 20, 30)
	if len(manual) != 3 {
		t.Fatalf("구간 밖 사각형만 빠져야 함: %d", len(manual))
	}
	if m := manual[0]; m.First != 0 || m.End != 300 {
		t.Errorf("구간 전체: %+v", m)
	}
	if m := manual[1]; m.First != 60 || m.End != 105 {
		t.Errorf("2~3.5초 → 60~105번 프레임: %+v", m)
	}
	if m := manual[2]; m.First != 0 || m.End != 300 {
		t.Errorf("시각이 없으면 구간 전체: %+v", m)
	}
}
