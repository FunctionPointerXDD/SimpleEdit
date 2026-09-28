package automask

import (
	"math"
	"testing"

	"simpleEdit/internal/media"
)

var noExpand = TrackOptions{MatchIoU: 0.2, MaxGap: 6, Hold: 2, Expand: 0}

func rect(x, y, w, h float64) media.Rect { return media.Rect{X: x, Y: y, W: w, H: h} }

func nearRect(a, b media.Rect) bool {
	return math.Abs(a.X-b.X) < 1e-9 && math.Abs(a.Y-b.Y) < 1e-9 &&
		math.Abs(a.W-b.W) < 1e-9 && math.Abs(a.H-b.H) < 1e-9
}

func TestTrackInterpolatesBetweenSamples(t *testing.T) {
	samples := []Sample{
		{Frame: 2, Boxes: []media.Rect{rect(0.10, 0.10, 0.2, 0.2)}},
		{Frame: 4, Boxes: []media.Rect{rect(0.14, 0.10, 0.2, 0.2)}},
	}
	out := Track(samples, 10, noExpand)
	want := map[int]media.Rect{
		0: rect(0.10, 0.10, 0.2, 0.2), 1: rect(0.10, 0.10, 0.2, 0.2),
		2: rect(0.10, 0.10, 0.2, 0.2), 3: rect(0.12, 0.10, 0.2, 0.2),
		4: rect(0.14, 0.10, 0.2, 0.2), 5: rect(0.14, 0.10, 0.2, 0.2),
		6: rect(0.14, 0.10, 0.2, 0.2),
	}
	for n := 0; n < 10; n++ {
		w, ok := want[n]
		if !ok {
			if len(out[n]) != 0 {
				t.Errorf("%d번 프레임에 박스가 있으면 안 됨: %v", n, out[n])
			}
			continue
		}
		if len(out[n]) != 1 || !nearRect(out[n][0], w) {
			t.Errorf("%d번 = %v, want %v", n, out[n], w)
		}
	}
}

func TestTrackKeepsTwoFacesSeparate(t *testing.T) {
	a, b := rect(0.1, 0.1, 0.1, 0.1), rect(0.7, 0.1, 0.1, 0.1)
	samples := []Sample{
		{Frame: 0, Boxes: []media.Rect{a, b}},
		{Frame: 2, Boxes: []media.Rect{rect(0.72, 0.1, 0.1, 0.1), rect(0.12, 0.1, 0.1, 0.1)}},
	}
	out := Track(samples, 3, noExpand)
	if len(out[1]) != 2 {
		t.Fatalf("1번 프레임 박스 %d개: %v", len(out[1]), out[1])
	}
	for _, r := range out[1] {
		if !nearRect(r, rect(0.11, 0.1, 0.1, 0.1)) && !nearRect(r, rect(0.71, 0.1, 0.1, 0.1)) {
			t.Errorf("잘못 섞인 보간: %v", r)
		}
	}
}

func TestTrackLongGapDoesNotBridge(t *testing.T) {
	samples := []Sample{
		{Frame: 0, Boxes: []media.Rect{rect(0.1, 0.1, 0.2, 0.2)}},
		{Frame: 20, Boxes: []media.Rect{rect(0.1, 0.1, 0.2, 0.2)}},
	}
	out := Track(samples, 25, noExpand)
	if len(out[10]) != 0 {
		t.Errorf("MaxGap보다 긴 공백을 이으면 안 됨: %v", out[10])
	}
	if len(out[2]) != 1 || len(out[18]) != 1 {
		t.Errorf("앞뒤 Hold 프레임은 유지돼야 함: %v %v", out[2], out[18])
	}
}

func TestTrackExpandsAndClips(t *testing.T) {
	out := Track([]Sample{{Frame: 0, Boxes: []media.Rect{rect(0, 0.4, 0.2, 0.2)}}}, 1,
		TrackOptions{MatchIoU: 0.2, MaxGap: 6, Hold: 0, Expand: 0.5})
	got := out[0][0]
	if !nearRect(got, rect(0, 0.35, 0.25, 0.3)) {
		t.Errorf("확대·자르기 = %v", got)
	}
}

func TestTrackDoesNotReorderInput(t *testing.T) {
	samples := []Sample{
		{Frame: 4, Boxes: []media.Rect{rect(0.1, 0.1, 0.2, 0.2)}},
		{Frame: 0, Boxes: []media.Rect{rect(0.1, 0.1, 0.2, 0.2)}},
	}
	out := Track(samples, 6, noExpand)
	if samples[0].Frame != 4 {
		t.Error("호출한 쪽 슬라이스 순서가 바뀜")
	}
	if len(out[2]) != 1 {
		t.Errorf("정렬 후 보간되어야 함: %v", out[2])
	}
}
