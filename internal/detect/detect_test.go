package detect

import (
	"math"
	"testing"

	"simpleEdit/internal/frame"
)

func TestInputSizeAligns(t *testing.T) {
	tests := []struct{ w, h, pw, ph int }{{640, 360, 640, 384}, {1920, 1080, 640, 384}, {360, 640, 384, 640}, {640, 640, 640, 640}, {2, 1, 32, 32}}
	for _, tt := range tests {
		if _, _, _, pw, ph := InputSize(tt.w, tt.h, 640); pw != tt.pw || ph != tt.ph {
			t.Errorf("%dx%d → %dx%d, want %dx%d", tt.w, tt.h, pw, ph, tt.pw, tt.ph)
		}
	}
}

func TestLetterboxCopyAndPad(t *testing.T) {
	f := frame.Frame{W: 2, H: 1, Pix: []byte{1, 2, 3, 4, 5, 6}}
	dst := make([]float32, 3*32*32)
	s, pw, ph := Letterbox(f, 640, 114, dst)
	if s != 1 || pw != 32 || ph != 32 {
		t.Fatalf("scale = %v, 캔버스 %dx%d", s, pw, ph)
	}
	plane := 32 * 32
	want := map[int]float32{0: 1, 1: 4, plane: 2, plane + 1: 5, 2 * plane: 3, 2*plane + 1: 6}
	for i, v := range dst {
		w, ok := want[i]
		if !ok {
			w = 114
		}
		if v != w {
			t.Fatalf("dst[%d] = %v, want %v", i, v, w)
		}
	}
}

func TestLetterboxDownscale(t *testing.T) {
	f := frame.Frame{W: 128, H: 64, Pix: make([]byte, 128*64*3)}
	for i := range f.Pix {
		f.Pix[i] = 200
	}
	dst := make([]float32, 3*64*32)
	s, pw, ph := Letterbox(f, 64, 0, dst)
	if s != 0.5 || pw != 64 || ph != 32 {
		t.Fatalf("scale = %v, 캔버스 %dx%d", s, pw, ph)
	}
	for i, v := range dst {
		if v != 200 {
			t.Fatalf("dst[%d] = %v", i, v)
		}
	}
}

func TestNMS(t *testing.T) {
	boxes := []Box{
		{X: 0, Y: 0, W: 10, H: 10, Score: 0.8},
		{X: 1, Y: 1, W: 10, H: 10, Score: 0.9},
		{X: 50, Y: 50, W: 10, H: 10, Score: 0.7},
	}
	got := NMS(boxes, 0.3)
	if len(got) != 2 || got[0].Score != 0.9 || got[1].Score != 0.7 {
		t.Fatalf("NMS = %+v", got)
	}
	if v := IoU(boxes[0], Box{X: 20, Y: 20, W: 5, H: 5}); v != 0 {
		t.Errorf("떨어진 박스 IoU = %v", v)
	}
}

func TestDecodeYuNet(t *testing.T) {
	var o yunetOutputs
	const pw, ph = 640, 384
	for i, s := range yunetStrides {
		n := (pw / s) * (ph / s)
		o.cls[i] = make([]float32, n)
		o.obj[i] = make([]float32, n)
		o.bbox[i] = make([]float32, n*4)
	}
	cols := pw / 16
	idx := 3*cols + 5
	o.cls[1][idx], o.obj[1][idx] = 0.81, 1.2
	copy(o.bbox[1][idx*4:], []float32{0.5, 0.25, float32(math.Log(2)), 0})
	got := decodeYuNet(o, pw, 0.5)
	if len(got) != 1 {
		t.Fatalf("박스 %d개", len(got))
	}
	b := got[0]
	want := Box{X: (5.5)*16 - 16, Y: (3.25)*16 - 8, W: 32, H: 16, Score: 0.9}
	if !near(b.X, want.X) || !near(b.Y, want.Y) || !near(b.W, want.W) || !near(b.H, want.H) || !near(b.Score, want.Score) {
		t.Fatalf("got %+v want %+v", b, want)
	}
}

func TestToFrameScalesAndClips(t *testing.T) {
	got := toFrame([]Box{{X: -10, Y: 10, W: 40, H: 20, Score: 1}, {X: 700, Y: 0, W: 5, H: 5}}, 0.5, 100, 100)
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	if b := got[0]; b.X != 0 || b.Y != 20 || b.W != 60 || b.H != 40 {
		t.Fatalf("got %+v", b)
	}
}

func BenchmarkLetterbox640x360(b *testing.B) {
	f := frame.New(640, 360)
	for i := range f.Pix {
		f.Pix[i] = byte(i * 7)
	}
	dst := make([]float32, 3*640*384)
	for b.Loop() {
		Letterbox(f, 640, 0, dst)
	}
}

func TestDecodeYOLOXGrid(t *testing.T) {
	const pw, ph = 64, 32
	out := make([]float32, plateAnchors(pw, ph)*plateOut)
	put := func(i int, v ...float32) { copy(out[i*plateOut:], v) }
	put(8+3, 0.5, 0.5, 0, float32(math.Log(2)), 0.9, 0.9)
	put(32+2, 0.25, 0, float32(math.Log(0.5)), 0, 0.5, 0.5)
	boxes := decodeYOLOX(out, pw, ph, 0.3)
	if len(boxes) != 1 {
		t.Fatalf("점수 0.81 하나만 남아야 함(0.25는 기준 미달): %+v", boxes)
	}
	b := boxes[0]
	if !near(b.X, 24) || !near(b.Y, 4) || !near(b.W, 8) || !near(b.H, 16) || !near(b.Score, 0.81) {
		t.Errorf("stride 8 칸(3,1): 중심 (28,12), 크기 8×16이어야 함: %+v", b)
	}
	if n := plateAnchors(960, 544); n != 120*68+60*34+30*17 {
		t.Errorf("960×544 칸 수 %d", n)
	}
}

func TestDropTallPlates(t *testing.T) {
	boxes := []Box{{H: 54, Score: 0.9}, {H: 55, Score: 0.8}, {H: 40, Score: 0.7}}
	got := dropTall(boxes, 540)
	if len(got) != 2 || got[0].Score != 0.9 || got[1].Score != 0.7 {
		t.Errorf("540px 화면에서 높이 54 이하만 남아야 함: %+v", got)
	}
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-4 }

func TestTrackCropAndBox(t *testing.T) {
	f := frame.New(100, 50)
	for i := range f.Pix {
		f.Pix[i] = 200
	}
	dst := make([]float32, 3*16*16)
	crop := cropResize(f, PixelBox{X: 90, Y: 40, W: 8, H: 8}, 2, 16, dst)
	if crop != 16 {
		t.Fatalf("√(8·8)×2 = 16칸이어야 함: %d", crop)
	}
	zero := (0 - trackMean[0]*255) / (trackStd[0] * 255)
	if !near(dst[0], (200-trackMean[0]*255)/(trackStd[0]*255)) || !near(dst[15], zero) || !near(dst[15*16], zero) {
		t.Errorf("그림 안은 값, 오른쪽·아래 밖(끝 한 줄 포함)은 0이어야 함: %v %v %v", dst[0], dst[15], dst[15*16])
	}
	got := nextBox(PixelBox{X: 100, Y: 100, W: 20, H: 10}, 0.5, 0.5, 0.25, 0.125, 64)
	if got != (PixelBox{X: 102, Y: 101, W: 16, H: 8}) {
		t.Errorf("자른 칸 기준 가운데 0.25×0.125 상자 → 원본 %+v", got)
	}
	h := hann2d()
	if h[0] >= h[8*16+8] || !near(h[8*16+7], h[7*16+8]) {
		t.Errorf("Hanning 창은 가운데가 크고 대칭이어야 함")
	}
}

func TestTrackSaneRejectsBlowUp(t *testing.T) {
	first := PixelBox{X: 400, Y: 700, W: 56, H: 24}
	last := PixelBox{X: 700, Y: 720, W: 30, H: 14}
	cases := []struct {
		next PixelBox
		ok   bool
	}{
		{PixelBox{X: 702, Y: 721, W: 31, H: 14}, true},
		{PixelBox{X: 630, Y: 582, W: 773, H: 255}, false},
		{PixelBox{X: 700, Y: 720, W: 20, H: 10}, false},
		{PixelBox{X: 1950, Y: 720, W: 30, H: 14}, false},
		{PixelBox{X: 700, Y: 720, W: 0, H: 14}, false},
	}
	for _, c := range cases {
		if got := sane(c.next, last, first, 1920, 1080); got != c.ok {
			t.Errorf("%+v: %v, want %v", c.next, got, c.ok)
		}
	}
	if sane(PixelBox{W: 170, H: 72}, PixelBox{W: 160, H: 70}, first, 1920, 1080) {
		t.Error("처음보다 9배 넘게 커지면 놓침")
	}
}
