package detect

import (
	"math"
	"testing"

	"simpleEdit/internal/frame"
)

func TestLetterboxCopyAndPad(t *testing.T) {
	f := frame.Frame{W: 2, H: 1, Pix: []byte{1, 2, 3, 4, 5, 6}}
	dst := make([]float32, 3*4*4)
	if s := Letterbox(f, 4, 114, dst); s != 1 {
		t.Fatalf("scale = %v", s)
	}
	plane := 16
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
	f := frame.Frame{W: 8, H: 4, Pix: make([]byte, 8*4*3)}
	for i := range f.Pix {
		f.Pix[i] = 200
	}
	dst := make([]float32, 3*4*4)
	s := Letterbox(f, 4, 0, dst)
	if s != 0.5 {
		t.Fatalf("scale = %v", s)
	}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			v := dst[y*4+x]
			if y < 2 && v != 200 || y >= 2 && v != 0 {
				t.Fatalf("(%d,%d) = %v", x, y, v)
			}
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
	for i, s := range yunetStrides {
		n := (yunetSize / s) * (yunetSize / s)
		o.cls[i] = make([]float32, n)
		o.obj[i] = make([]float32, n)
		o.bbox[i] = make([]float32, n*4)
	}
	cols := yunetSize / 16
	idx := 3*cols + 5
	o.cls[1][idx], o.obj[1][idx] = 0.81, 1.2
	copy(o.bbox[1][idx*4:], []float32{0.5, 0.25, float32(math.Log(2)), 0})
	got := decodeYuNet(o, 0.5)
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

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-4 }
