package main

import (
	"testing"

	"simpleEdit/internal/automask"
	"simpleEdit/internal/media"
)

// 30fps, 1~3초(프레임 30~90)를 분석해 둔 App
func appWithFace(frames [][]automask.TrackBox) *App {
	full := make([][]automask.TrackBox, 60)
	copy(full, frames)
	return &App{
		info: media.Info{FPS: 30, Duration: 10, Width: 64, Height: 64},
		auto: map[string]*automask.Coverage{kindFace: {FPS: 30, First: 30, Frames: full}},
	}
}

func TestAutoFramesSkipsExcluded(t *testing.T) {
	a := appWithFace([][]automask.TrackBox{
		{{Rect: media.Rect{W: 0.1, H: 0.1}, ID: 1}, {Rect: media.Rect{X: 0.5, W: 0.1, H: 0.1}, ID: 2}},
		{{Rect: media.Rect{W: 0.1, H: 0.1}, ID: 1}},
	})
	got, err := a.autoFrames([]string{kindFace}, map[string][]int{kindFace: {1}}, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 || len(got[0]) != 1 || got[0][0].X != 0.5 || len(got[1]) != 0 {
		t.Errorf("1번 트랙만 빠져야 함: %v", got[:2])
	}
}

func TestAutoFramesInsideAnalyzedRange(t *testing.T) {
	frames := make([][]automask.TrackBox, 60)
	frames[30] = []automask.TrackBox{{Rect: media.Rect{X: 0.3, W: 0.1, H: 0.1}, ID: 4}}
	a := appWithFace(frames)
	got, err := a.autoFrames([]string{kindFace}, nil, 2, 2.5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got[0]) != 1 || got[0][0].X != 0.3 {
		t.Errorf("2초(프레임 60)의 박스가 저장 구간 첫 프레임이어야 함: %v", got[0])
	}
}

func TestAutoFramesNilWhenNothingLeft(t *testing.T) {
	a := appWithFace([][]automask.TrackBox{{{Rect: media.Rect{W: 0.1, H: 0.1}, ID: 1}}, nil})
	got, err := a.autoFrames([]string{kindFace}, map[string][]int{kindFace: {1}}, 1, 3)
	if err != nil || got != nil {
		t.Errorf("남은 박스가 없으면 nil이어야 함: %v, %v", got, err)
	}
}

func TestAutoFramesOutsideRange(t *testing.T) {
	a := appWithFace(nil)
	for _, r := range [][2]float64{{0.5, 2}, {2, 3.5}} {
		if _, err := a.autoFrames([]string{kindFace}, nil, r[0], r[1]); err != errStaleAuto {
			t.Errorf("%v: 분석 범위 밖이면 errStaleAuto, got %v", r, err)
		}
	}
}
