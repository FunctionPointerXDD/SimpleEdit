// Package frame은 bgr24 프레임과 모자이크를 다룬다.
package frame

import "image"

// Frame은 bgr24 한 장이다. len(Pix) = W*H*3
type Frame struct {
	W, H int
	Pix  []byte
}

// New는 검은 프레임을 만든다.
func New(w, h int) Frame {
	return Frame{W: w, H: h, Pix: make([]byte, w*h*3)}
}

// Size는 프레임 한 장의 바이트 수다.
func Size(w, h int) int { return w * h * 3 }

// FromImage는 image.Image를 bgr24 Frame으로 바꾼다.
func FromImage(img image.Image) Frame {
	b := img.Bounds()
	f := New(b.Dx(), b.Dy())
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			f.Pix[i], f.Pix[i+1], f.Pix[i+2] = byte(bl>>8), byte(g>>8), byte(r>>8)
			i += 3
		}
	}
	return f
}

// Pixelate는 사각형을 block 크기 칸 평균색으로 채운다.
func (f Frame) Pixelate(r image.Rectangle, block int) {
	r = r.Intersect(image.Rect(0, 0, f.W, f.H))
	if r.Empty() {
		return
	}
	cols := max(1, r.Dx()/block)
	rows := max(1, r.Dy()/block)
	for j := 0; j < rows; j++ {
		y0 := r.Min.Y + j*r.Dy()/rows
		y1 := r.Min.Y + (j+1)*r.Dy()/rows
		for i := 0; i < cols; i++ {
			x0 := r.Min.X + i*r.Dx()/cols
			x1 := r.Min.X + (i+1)*r.Dx()/cols
			f.fillMean(x0, y0, x1, y1)
		}
	}
}

// fillMean은 칸 안을 채널별 평균값으로 채운다.
func (f Frame) fillMean(x0, y0, x1, y1 int) {
	var sum [3]int
	for y := y0; y < y1; y++ {
		row := f.Pix[(y*f.W+x0)*3 : (y*f.W+x1)*3]
		for i := 0; i < len(row); i += 3 {
			sum[0] += int(row[i])
			sum[1] += int(row[i+1])
			sum[2] += int(row[i+2])
		}
	}
	n := (x1 - x0) * (y1 - y0)
	c := [3]byte{byte(sum[0] / n), byte(sum[1] / n), byte(sum[2] / n)}
	for y := y0; y < y1; y++ {
		row := f.Pix[(y*f.W+x0)*3 : (y*f.W+x1)*3]
		for i := 0; i < len(row); i += 3 {
			row[i], row[i+1], row[i+2] = c[0], c[1], c[2]
		}
	}
}
