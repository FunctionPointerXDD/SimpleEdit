package frame

import (
	"image"
	"image/color"
	"testing"
)

func TestFromImageIsBGR(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	img.Set(1, 0, color.RGBA{R: 40, G: 50, B: 60, A: 255})
	f := FromImage(img)
	want := []byte{30, 20, 10, 60, 50, 40}
	for i, v := range want {
		if f.Pix[i] != v {
			t.Fatalf("Pix = %v, want %v", f.Pix, want)
		}
	}
}

func checker(w, h int) Frame {
	f := New(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := byte(0)
			if (x/2+y/2)%2 == 0 {
				v = 255
			}
			o := (y*w + x) * 3
			f.Pix[o], f.Pix[o+1], f.Pix[o+2] = v, v/2, 255-v
		}
	}
	return f
}

func TestBlurSmoothsOnlyInside(t *testing.T) {
	f := checker(40, 20)
	orig := append([]byte(nil), f.Pix...)
	r := image.Rect(4, 2, 36, 18)
	f.Blur(r, 6)
	lo, hi := byte(255), byte(0)
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			o := (y*f.W + x) * 3
			if !image.Pt(x, y).In(r) {
				if f.Pix[o] != orig[o] || f.Pix[o+1] != orig[o+1] || f.Pix[o+2] != orig[o+2] {
					t.Fatalf("사각형 밖 (%d,%d)이 바뀜", x, y)
				}
				continue
			}
			lo, hi = min(lo, f.Pix[o]), max(hi, f.Pix[o])
		}
	}
	if hi-lo > 40 {
		t.Errorf("체커 무늬가 남음: 안쪽 값 %d~%d", lo, hi)
	}
}

func TestBlurKeepsFlatColorAndBounds(t *testing.T) {
	f := New(10, 10)
	for i := range f.Pix {
		f.Pix[i] = 77
	}
	f.Blur(image.Rect(0, 0, 10, 10), 30)
	for i, v := range f.Pix {
		if v != 77 {
			t.Fatalf("단색이 바뀜: Pix[%d] = %d", i, v)
		}
	}
	f.Blur(image.Rect(20, 20, 30, 30), 4)
	f.Blur(image.Rect(8, 8, 30, 30), 4)
	f.Blur(image.Rect(0, 0, 1, 1), 4)
	f.Blur(image.Rect(0, 0, 5, 5), 0)
}

func TestBlurLargeRadiusSmooths(t *testing.T) {
	f := checker(300, 200)
	orig := append([]byte(nil), f.Pix...)
	r := image.Rect(50, 40, 250, 160)
	f.Blur(r, 40)
	lo, hi := byte(255), byte(0)
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			o := (y*f.W + x) * 3
			if !image.Pt(x, y).In(r) {
				if f.Pix[o] != orig[o] {
					t.Fatalf("사각형 밖 (%d,%d)이 바뀜", x, y)
				}
				continue
			}
			lo, hi = min(lo, f.Pix[o]), max(hi, f.Pix[o])
		}
	}
	if hi-lo > 20 {
		t.Errorf("큰 반지름인데 무늬가 남음: %d~%d", lo, hi)
	}
}

func TestBlurEllipseKeepsCorners(t *testing.T) {
	f := checker(100, 60)
	orig := append([]byte(nil), f.Pix...)
	r := image.Rect(10, 10, 90, 50)
	f.BlurEllipse(r, 8)
	at := func(p []byte, x, y int) byte { return p[(y*f.W+x)*3] }
	for _, c := range [][2]int{{10, 10}, {89, 10}, {10, 49}, {89, 49}, {12, 12}} {
		if at(f.Pix, c[0], c[1]) != at(orig, c[0], c[1]) {
			t.Errorf("타원 밖 귀퉁이 (%d,%d)이 바뀜", c[0], c[1])
		}
	}
	lo, hi := byte(255), byte(0)
	for y := 22; y < 38; y++ {
		for x := 35; x < 65; x++ {
			lo, hi = min(lo, at(f.Pix, x, y)), max(hi, at(f.Pix, x, y))
		}
	}
	if hi-lo > 40 {
		t.Errorf("타원 가운데가 흐려지지 않음: %d~%d", lo, hi)
	}
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			if !image.Pt(x, y).In(r) && at(f.Pix, x, y) != at(orig, x, y) {
				t.Fatalf("사각형 밖 (%d,%d)이 바뀜", x, y)
			}
		}
	}
}
