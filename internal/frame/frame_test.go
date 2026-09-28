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

func TestPixelateBlocksAndBounds(t *testing.T) {
	f := New(40, 20)
	for i := range f.Pix {
		f.Pix[i] = byte(i % 251)
	}
	orig := append([]byte(nil), f.Pix...)
	f.Pixelate(image.Rect(4, 2, 36, 18), 16)

	at := func(x, y int) [3]byte {
		o := (y*f.W + x) * 3
		return [3]byte{f.Pix[o], f.Pix[o+1], f.Pix[o+2]}
	}
	if at(4, 2) != at(19, 17) || at(20, 2) != at(35, 17) {
		t.Error("칸 안이 한 색이 아님")
	}
	if at(4, 2) == at(20, 2) {
		t.Error("두 칸이 같은 색 (나뉘지 않음)")
	}
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			inside := x >= 4 && x < 36 && y >= 2 && y < 18
			o := (y*f.W + x) * 3
			if !inside && (f.Pix[o] != orig[o] || f.Pix[o+2] != orig[o+2]) {
				t.Fatalf("사각형 밖 (%d,%d)이 바뀜", x, y)
			}
		}
	}
}

func TestPixelateSmallAndOutside(t *testing.T) {
	f := New(10, 10)
	f.Pix[0] = 90
	f.Pixelate(image.Rect(0, 0, 3, 3), 16)
	if f.Pix[0] != 10 || f.Pix[(2*10+2)*3] != 10 {
		t.Fatalf("작은 사각형은 한 칸 평균이어야 함: %v", f.Pix[0])
	}
	f.Pixelate(image.Rect(20, 20, 30, 30), 16)
	f.Pixelate(image.Rect(8, 8, 30, 30), 16)
}
