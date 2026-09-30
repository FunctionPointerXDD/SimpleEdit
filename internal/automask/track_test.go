package automask

import (
	"math"
	"testing"

	"simpleEdit/internal/media"
)

var noExpand = TrackOptions{MatchIoU: 0.2, MaxGap: 6, Hold: 2, Expand: 0, Strong: 0.7, Weak: 0.7}

func rect(x, y, w, h float64) media.Rect { return media.Rect{X: x, Y: y, W: w, H: h} }

func strong(rs ...media.Rect) []Det { return withScore(0.9, rs...) }

func withScore(score float32, rs ...media.Rect) []Det {
	out := make([]Det, len(rs))
	for i, r := range rs {
		out[i] = Det{Box: r, Score: score}
	}
	return out
}

func trackRects(samples []Sample, frames int, o TrackOptions) ([][]media.Rect, TrackStats) {
	boxes, st := Track(samples, frames, o)
	return Rects(boxes, nil), st
}

func nearRect(a, b media.Rect) bool {
	return math.Abs(a.X-b.X) < 1e-9 && math.Abs(a.Y-b.Y) < 1e-9 &&
		math.Abs(a.W-b.W) < 1e-9 && math.Abs(a.H-b.H) < 1e-9
}

func TestTrackInterpolatesBetweenSamples(t *testing.T) {
	samples := []Sample{
		{Frame: 2, Dets: strong(rect(0.10, 0.10, 0.2, 0.2))},
		{Frame: 4, Dets: strong(rect(0.14, 0.10, 0.2, 0.2))},
	}
	out, _ := trackRects(samples, 10, noExpand)
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
		{Frame: 0, Dets: strong(a, b)},
		{Frame: 2, Dets: strong(rect(0.72, 0.1, 0.1, 0.1), rect(0.12, 0.1, 0.1, 0.1))},
	}
	out, _ := trackRects(samples, 3, noExpand)
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
		{Frame: 0, Dets: strong(rect(0.1, 0.1, 0.2, 0.2))},
		{Frame: 20, Dets: strong(rect(0.1, 0.1, 0.2, 0.2))},
	}
	out, _ := trackRects(samples, 25, noExpand)
	if len(out[10]) != 0 {
		t.Errorf("MaxGap보다 긴 공백을 이으면 안 됨: %v", out[10])
	}
	if len(out[2]) != 1 || len(out[18]) != 1 {
		t.Errorf("앞뒤 Hold 프레임은 유지돼야 함: %v %v", out[2], out[18])
	}
}

func TestTrackExpandsAndClips(t *testing.T) {
	out, _ := trackRects([]Sample{{Frame: 0, Dets: strong(rect(0, 0.4, 0.2, 0.2))}}, 1,
		TrackOptions{MatchIoU: 0.2, MaxGap: 6, Hold: 0, Expand: 0.5, Strong: 0.7, Weak: 0.7})
	if got := out[0][0]; !nearRect(got, rect(0, 0.35, 0.25, 0.3)) {
		t.Errorf("확대·자르기 = %v", got)
	}
}

func TestTrackDoesNotReorderInput(t *testing.T) {
	samples := []Sample{
		{Frame: 4, Dets: strong(rect(0.1, 0.1, 0.2, 0.2))},
		{Frame: 0, Dets: strong(rect(0.1, 0.1, 0.2, 0.2))},
	}
	out, _ := trackRects(samples, 6, noExpand)
	if samples[0].Frame != 4 {
		t.Error("호출한 쪽 슬라이스 순서가 바뀜")
	}
	if len(out[2]) != 1 {
		t.Errorf("정렬 후 보간되어야 함: %v", out[2])
	}
}

func TestTrackDropsWeakOnlyTrack(t *testing.T) {
	var samples []Sample
	for n := 0; n < 30; n++ {
		samples = append(samples, Sample{Frame: n, Dets: withScore(0.55, rect(0.1, 0.1, 0.1, 0.1))})
	}
	out, st := trackRects(samples, 30, noExpand)
	if st.Kept != 0 || st.Dropped != 1 {
		t.Errorf("약한 검출만 있는 트랙은 버려야 함: %+v", st)
	}
	for n, boxes := range out {
		if len(boxes) != 0 {
			t.Fatalf("%d번 프레임에 박스가 남음", n)
		}
	}
}

func TestTrackWeakDetectionsExtendStrongTrack(t *testing.T) {
	samples := []Sample{{Frame: 0, Dets: strong(rect(0.1, 0.1, 0.1, 0.1))}}
	for n := 1; n <= 10; n++ {
		samples = append(samples, Sample{Frame: n, Dets: withScore(0.45, rect(0.1+0.005*float64(n), 0.1, 0.1, 0.1))})
	}
	out, st := trackRects(samples, 11, noExpand)
	if st.Kept != 1 || st.Dropped != 0 {
		t.Fatalf("강한 검출이 있는 트랙 하나여야 함: %+v", st)
	}
	if len(out[10]) != 1 || !nearRect(out[10][0], rect(0.15, 0.1, 0.1, 0.1)) {
		t.Errorf("약한 검출로 이어진 위치를 따라가야 함: %v", out[10])
	}
}

func TestTrackSizeAwareThreshold(t *testing.T) {
	o := noExpand
	o.Strong, o.Weak, o.Large = 0.7, 0, 0.08
	var samples []Sample
	for n := 0; n < 10; n++ {
		samples = append(samples, Sample{Frame: n, Dets: append(
			withScore(0.55, rect(0.1, 0.1, 0.10, 0.12)),
			withScore(0.55, rect(0.6, 0.6, 0.03, 0.04))...)})
	}
	out, st := trackRects(samples, 10, o)
	if st.Kept != 1 || st.Dropped != 1 {
		t.Fatalf("큰 약한 트랙만 버려야 함: %+v", st)
	}
	if len(out[5]) != 1 || out[5][0].X != 0.6 {
		t.Errorf("작은 트랙이 남아야 함: %v", out[5])
	}
}

func TestRectsExcludesTrack(t *testing.T) {
	samples := []Sample{
		{Frame: 0, Dets: strong(rect(0.1, 0.1, 0.1, 0.1), rect(0.7, 0.1, 0.1, 0.1))},
		{Frame: 1, Dets: strong(rect(0.1, 0.1, 0.1, 0.1), rect(0.7, 0.1, 0.1, 0.1))},
	}
	boxes, _ := Track(samples, 2, noExpand)
	ids := map[int]bool{}
	for _, b := range boxes[0] {
		ids[b.ID] = true
	}
	if len(ids) != 2 || boxes[0][0].ID != boxes[1][0].ID {
		t.Fatalf("트랙마다 번호가 다르고 프레임 사이에 같아야 함: %+v", boxes)
	}
	drop := boxes[0][0].ID
	for n, rs := range Rects(boxes, map[int]bool{drop: true}) {
		if len(rs) != 1 {
			t.Errorf("%d번 프레임: 뺀 트랙이 남음 %v", n, rs)
		}
	}
}

func TestTrackShortNeedsHighScore(t *testing.T) {
	o := noExpand
	o.Weak, o.Large, o.ShortDetections, o.ShortScore = 0, 0.08, 10, 0.75
	var samples []Sample
	for n := 0; n < 12; n++ {
		dets := withScore(0.65, rect(0.1, 0.1, 0.03, 0.04))
		if n < 3 {
			dets = append(dets, withScore(0.7, rect(0.6, 0.1, 0.03, 0.04))...)
		}
		if n >= 5 && n < 8 {
			dets = append(dets, withScore(0.8, rect(0.6, 0.6, 0.03, 0.04))...)
		}
		samples = append(samples, Sample{Frame: n, Dets: dets})
	}
	_, st := Track(samples, 12, o)
	if st.Kept != 2 || st.Dropped != 1 {
		t.Fatalf("긴 트랙과 점수 높은 짧은 트랙만 남아야 함: %+v", st)
	}
	for _, tr := range st.Tracks {
		if tr.Detections == 3 && tr.MaxScore < 0.75 && tr.Kept {
			t.Errorf("점수 낮은 짧은 트랙이 남음: %+v", tr)
		}
	}
}

func TestSmoothKeepsMotionRemovesJitter(t *testing.T) {
	var line, shaky []obs
	for n := 0; n < 40; n++ {
		x := 0.1 + 0.005*float64(n)
		line = append(line, obs{frame: n, box: rect(x, 0.2, 0.1, 0.1)})
		d := 0.004 * float64(n%2*2-1)
		shaky = append(shaky, obs{frame: n, box: rect(0.3+d, 0.2, 0.1+d, 0.1)})
	}
	sl := smooth(line, 3, 6)
	for i := 10; i < 30; i++ {
		if !nearRect(sl[i].box, line[i].box) {
			t.Fatalf("일정하게 움직이면 가운데 구간은 그대로여야 함(늦게 따라가면 안 됨): %d %v → %v", i, line[i].box, sl[i].box)
		}
	}
	ss := smooth(shaky, 3, 6)
	for i := 10; i < 30; i++ {
		if math.Abs(ss[i].box.X+ss[i].box.W/2-0.35) > 0.0005 || math.Abs(ss[i].box.W-0.1) > 0.0005 {
			t.Fatalf("떨림이 남음: %d %+v", i, ss[i].box)
		}
	}
	if got := smooth(line, 0, 0); &got[0] != &line[0] {
		t.Error("시그마 0이면 그대로 돌려줘야 함")
	}
}
