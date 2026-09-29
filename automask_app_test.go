package main

import (
	"testing"

	"simpleEdit/internal/automask"
	"simpleEdit/internal/media"
)

func appWithFace(frames [][]automask.TrackBox) *App {
	return &App{auto: map[string]automask.Result{
		kindFace: {Start: 1, Duration: 2, FPS: 30, Frames: frames},
	}}
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
	if len(got) != 2 || len(got[0]) != 1 || got[0][0].X != 0.5 || len(got[1]) != 0 {
		t.Errorf("1번 트랙만 빠져야 함: %v", got)
	}
}

func TestAutoFramesNilWhenNothingLeft(t *testing.T) {
	a := appWithFace([][]automask.TrackBox{{{Rect: media.Rect{W: 0.1, H: 0.1}, ID: 1}}, nil})
	got, err := a.autoFrames([]string{kindFace}, map[string][]int{kindFace: {1}}, 1, 3)
	if err != nil || got != nil {
		t.Errorf("남은 박스가 없으면 nil이어야 함: %v, %v", got, err)
	}
}

func TestAutoFramesStaleRange(t *testing.T) {
	a := appWithFace(nil)
	if _, err := a.autoFrames([]string{kindFace}, nil, 1, 3.5); err != errStaleAuto {
		t.Errorf("구간이 다르면 errStaleAuto, got %v", err)
	}
}
