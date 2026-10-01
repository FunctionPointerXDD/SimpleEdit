package automask

import (
	"testing"

	"simpleEdit/internal/frame"
	"simpleEdit/internal/media"
)

func TestDetectSize(t *testing.T) {
	tests := []struct{ w, h, limit, ww, wh int }{
		{1920, 1080, 640, 640, 360},
		{1080, 1920, 640, 360, 640},
		{320, 240, 640, 320, 240},
		{1281, 721, 0, 1280, 720},
	}
	for _, tt := range tests {
		if w, h := detectSize(tt.w, tt.h, tt.limit); w != tt.ww || h != tt.wh {
			t.Errorf("detectSize(%d,%d,%d) = %dx%d, want %dx%d", tt.w, tt.h, tt.limit, w, h, tt.ww, tt.wh)
		}
	}
}

func TestBlurFuncUsesFrameBoxes(t *testing.T) {
	auto := [][]media.Rect{nil, {{X: 0.5, Y: 0, W: 0.5, H: 1}}}
	fn := BlurFunc([]ManualRect{{Rect: media.Rect{X: 0, Y: 0, W: 0.25, H: 1}, First: 0, End: 3}}, auto)
	for n := 0; n < 3; n++ {
		f := frame.New(64, 16)
		for i := range f.Pix {
			f.Pix[i] = byte(i)
		}
		orig := append([]byte(nil), f.Pix...)
		if err := fn(n, f); err != nil {
			t.Fatal(err)
		}
		changed := func(x int) bool {
			o := (8*64 + x) * 3
			return f.Pix[o] != orig[o]
		}
		if !changed(3) {
			t.Errorf("%d번: 수동 사각형이 안 가려짐", n)
		}
		if changed(24) {
			t.Errorf("%d번: 가리지 않을 곳이 바뀜", n)
		}
		if got := changed(40); got != (n == 1) {
			t.Errorf("%d번: 자동 박스 적용 = %v", n, got)
		}
	}
}

func TestBlurFuncManualOnlyInSpan(t *testing.T) {
	fn := BlurFunc([]ManualRect{{Rect: media.Rect{X: 0, Y: 0, W: 0.5, H: 1}, First: 2, End: 4}}, nil)
	for n := 0; n < 6; n++ {
		f := frame.New(32, 8)
		for i := range f.Pix {
			f.Pix[i] = byte(i * 7)
		}
		orig := append([]byte(nil), f.Pix...)
		if err := fn(n, f); err != nil {
			t.Fatal(err)
		}
		o := (4*32 + 4) * 3
		if got := f.Pix[o] != orig[o]; got != (n >= 2 && n < 4) {
			t.Errorf("%d번 프레임 가림 = %v, [2,4)에서만 가려야 함", n, got)
		}
	}
}

func TestOptionsFor(t *testing.T) {
	o30, o60 := OptionsFor(30), OptionsFor(59.94)
	if o30.Step != 1 || o30.Track.MaxGap != 15 || o30.Track.Hold != 9 {
		t.Errorf("30fps: %+v", o30)
	}
	if o60.Step != 2 || o60.Track.MaxGap != 30 || o60.Track.Hold != 18 {
		t.Errorf("60fps: %+v", o60)
	}
}
