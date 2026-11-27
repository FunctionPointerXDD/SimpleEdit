package automask

import "testing"

func boxesWithID(n, id int) Result {
	r := Result{Frames: make([][]TrackBox, n), Stats: TrackStats{Kept: 1}, Found: n}
	for i := range r.Frames {
		r.Frames[i] = []TrackBox{{Rect: rect(0.1, 0.1, 0.1, 0.1), ID: id}}
	}
	return r
}

func TestCoverageMissingAndMerge(t *testing.T) {
	c := &Coverage{FPS: 30}
	spans := c.Missing(FrameSpan{90, 150})
	if len(spans) != 1 || spans[0] != (FrameSpan{90, 150}) {
		t.Fatalf("빈 범위는 전체를: %v", spans)
	}
	c.Merge(spans[0], boxesWithID(60, 1))

	if spans := c.Missing(FrameSpan{100, 140}); len(spans) != 0 {
		t.Fatalf("안쪽 구간은 다시 분석하지 않아야 함: %v", spans)
	}
	spans = c.Missing(FrameSpan{60, 180})
	if len(spans) != 2 || spans[0] != (FrameSpan{60, 90}) || spans[1] != (FrameSpan{150, 180}) {
		t.Fatalf("앞·뒤 조각만: %v", spans)
	}
	c.Merge(spans[0], boxesWithID(30, 1))
	c.Merge(spans[1], boxesWithID(29, 1))
	if len(c.Pieces) != 1 || c.Pieces[0].First != 60 || c.Pieces[0].End() != 180 {
		t.Fatalf("맞닿은 조각은 하나로 60~180: %+v", c.Pieces)
	}
	f := c.Pieces[0].Frames
	if id := f[0][0].ID; id != 2 {
		t.Errorf("앞 조각 트랙 번호 %d, want 2 (기존 1 뒤)", id)
	}
	if id := f[30][0].ID; id != 1 {
		t.Errorf("기존 트랙 번호가 바뀜: %d", id)
	}
	if id := f[90][0].ID; id != 3 {
		t.Errorf("뒤 조각 트랙 번호 %d, want 3", id)
	}
	if len(f[119]) != 0 {
		t.Error("모자란 프레임은 빈칸이어야 함")
	}
}

func TestCoverageSeparatePieces(t *testing.T) {
	c := &Coverage{FPS: 30}
	c.Merge(FrameSpan{300, 330}, boxesWithID(30, 1))
	c.Merge(FrameSpan{90, 150}, boxesWithID(60, 1))
	if len(c.Pieces) != 2 || c.Pieces[0].First != 90 || c.Pieces[1].First != 300 {
		t.Fatalf("떨어진 조각은 시작 순서로 따로: %+v", c.Pieces)
	}
	spans := c.Missing(FrameSpan{120, 360})
	if len(spans) != 2 || spans[0] != (FrameSpan{150, 300}) || spans[1] != (FrameSpan{330, 360}) {
		t.Fatalf("조각 사이와 뒤만: %v", spans)
	}
	if _, ok := c.Window(140, 20); ok {
		t.Error("조각 사이를 걸치면 false")
	}
	if win, ok := c.Window(300, 30); !ok || win[0][0].ID != 1 {
		t.Errorf("뒤 조각 창: ok=%v", ok)
	}
}

func TestCoverageWindow(t *testing.T) {
	c := &Coverage{FPS: 30}
	c.Merge(FrameSpan{90, 150}, boxesWithID(60, 1))
	c.Pieces[0].Frames[59] = []TrackBox{{ID: 9}}
	win, ok := c.Window(140, 12)
	if !ok || len(win) != 12 || win[10][0].ID != 9 || win[11][0].ID != 9 {
		t.Fatalf("끝 2프레임 모자람은 마지막 프레임으로: ok=%v len=%d", ok, len(win))
	}
	if _, ok := c.Window(140, 13); ok {
		t.Error("3프레임 이상 모자라면 false")
	}
	if _, ok := c.Window(80, 10); ok {
		t.Error("앞이 모자라면 false")
	}
	var none *Coverage
	if _, ok := none.Window(0, 1); ok {
		t.Error("분석이 없으면 false")
	}
}

func TestCoverageWindowFitsAnalyzedRange(t *testing.T) {
	const fps = 30000.0 / 1001
	for k := 1; k < 3000; k++ {
		start, end := float64(k)/10, float64(k)/10+10.5
		c := &Coverage{FPS: fps}
		span := FrameSpan{FrameAt(start, fps), FrameAt(end, fps)}
		c.Merge(span, Result{Frames: make([][]TrackBox, span.End-span.First)})
		count := max(1, int((end-start)*fps+0.5)) + 1
		if _, ok := c.Window(FrameAt(start, fps), count); !ok {
			t.Fatalf("%.1f~%.1f초를 분석한 직후 저장 창이 안 맞음 (범위 %d~%d, 필요 %d)", start, end, span.First, span.End, count)
		}
	}
}
