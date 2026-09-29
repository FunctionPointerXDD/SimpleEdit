// Frame.Pix는 bgr24이고 len(Pix) = W*H*3이다.

// Package frame은 bgr24 프레임과 모자이크를 다룬다.
package frame

import "image"

type Frame struct {
	W, H int
	Pix  []byte
}

func New(w, h int) Frame {
	return Frame{W: w, H: h, Pix: make([]byte, w*h*3)}
}

// At으로 한 픽셀씩 읽어 16비트 RGBA의 상위 바이트를 BGR 순으로 넣는다.
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

// 화면 안으로 자른 사각형을 block 크기에 가깝게 균등 분할해 칸마다 평균색으로 칠한다.
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

// 채널별 합을 넓이로 나눈 평균으로 칸 전체를 덮는다.
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
