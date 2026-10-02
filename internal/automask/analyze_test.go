// fakePipeline은 Prepare한 슬롯을 차례로 쌓고 Infer가 앞에서 꺼낸다.
// 계산 중인 슬롯에 Prepare하면 overwrite로 기록해 겹치기가 슬롯을 덮어쓰는지 본다.

package automask

import (
	"errors"
	"sync"
	"testing"
	"time"

	"simpleEdit/internal/detect"
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

type fakePipeline struct {
	mu        sync.Mutex
	queue     []int
	pixels    []byte
	busy      int
	overwrite bool
	failAt    int
	infers    int
}

func (f *fakePipeline) Detect(fr frame.Frame) ([]detect.Box, error) { return nil, errors.New("쓰지 않음") }

func (f *fakePipeline) Prepare(slot int, fr frame.Frame) (detect.Prepared, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if slot == f.busy {
		f.overwrite = true
	}
	f.queue = append(f.queue, slot)
	f.pixels = append(f.pixels, fr.Pix[0])
	return detect.Prepared{}, nil
}

func (f *fakePipeline) Infer(detect.Prepared) ([]detect.Box, error) {
	f.mu.Lock()
	f.busy, f.queue = f.queue[0], f.queue[1:]
	v := f.pixels[0]
	f.pixels = f.pixels[1:]
	f.infers++
	n := f.infers
	f.mu.Unlock()
	time.Sleep(2 * time.Millisecond)
	f.mu.Lock()
	f.busy = -1
	f.mu.Unlock()
	if n == f.failAt {
		return nil, errors.New("추론 실패")
	}
	return []detect.Box{{X: float32(v), W: 1, H: 1, Score: 1}}, nil
}

func TestDetectQueuePipelinesInOrder(t *testing.T) {
	fp := &fakePipeline{busy: -1}
	var c Collection
	q := newDetectQueue(fp, &c)
	for n := 0; n < 20; n++ {
		f := frame.New(10, 10)
		f.Pix[0] = byte(n)
		if err := q.push(n, f); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.close(); err != nil {
		t.Fatal(err)
	}
	if fp.overwrite {
		t.Error("계산 중인 슬롯에 다음 프레임을 덮어씀")
	}
	if len(c.Samples) != 20 || c.Found != 20 {
		t.Fatalf("샘플 %d개, 검출 %d건", len(c.Samples), c.Found)
	}
	for n, s := range c.Samples {
		if s.Frame != n || s.Dets[0].Box.X*10 != float64(n) {
			t.Fatalf("%d번 순서·내용이 바뀜: %+v", n, s)
		}
	}
}

func TestDetectQueueStopsOnInferError(t *testing.T) {
	fp := &fakePipeline{busy: -1, failAt: 3}
	var c Collection
	q := newDetectQueue(fp, &c)
	var pushErr error
	for n := 0; n < 50 && pushErr == nil; n++ {
		pushErr = q.push(n, frame.New(4, 4))
	}
	if err := q.close(); err == nil || err.Error() != "추론 실패" {
		t.Fatalf("추론 오류를 돌려줘야 함: %v", err)
	}
	if !errors.Is(pushErr, errDetectStopped) {
		t.Errorf("실패 뒤 push는 멈춰야 함: %v", pushErr)
	}
}
