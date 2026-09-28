// Package detect는 ONNX 모델로 얼굴·번호판을 찾는다.
package detect

import (
	"math"

	"simpleEdit/internal/frame"
)

// Box는 원본 프레임 픽셀 기준 검출 결과다.
type Box struct {
	X, Y, W, H float32
	Score      float32
}

// Letterbox는 왼쪽 위 정렬·pad 채움 CHW(BGR)를 만든다.
func Letterbox(f frame.Frame, size int, pad float32, dst []float32) (scale float32) {
	scale = min(1, float32(size)/float32(max(f.W, f.H)))
	nw := int(math.Round(float64(float32(f.W) * scale)))
	nh := int(math.Round(float64(float32(f.H) * scale)))
	plane := size * size
	for i := range dst[:3*plane] {
		dst[i] = pad
	}
	if scale == 1 {
		copyPlanes(f, size, dst)
		return 1
	}
	resizeBilinear(f, nw, nh, size, dst)
	return scale
}

// copyPlanes는 크기 변경 없이 채널별로 옮긴다.
func copyPlanes(f frame.Frame, size int, dst []float32) {
	plane := size * size
	for y := 0; y < f.H; y++ {
		row := f.Pix[y*f.W*3:]
		for x := 0; x < f.W; x++ {
			o := y*size + x
			dst[o] = float32(row[x*3])
			dst[plane+o] = float32(row[x*3+1])
			dst[2*plane+o] = float32(row[x*3+2])
		}
	}
}

// resizeBilinear는 INTER_LINEAR식 픽셀 중심 보간이다.
func resizeBilinear(f frame.Frame, nw, nh, size int, dst []float32) {
	plane := size * size
	sx := float32(f.W) / float32(nw)
	sy := float32(f.H) / float32(nh)
	for y := 0; y < nh; y++ {
		fy := (float32(y)+0.5)*sy - 0.5
		y0, wy := splitCoord(fy, f.H)
		y1 := min(y0+1, f.H-1)
		for x := 0; x < nw; x++ {
			fx := (float32(x)+0.5)*sx - 0.5
			x0, wx := splitCoord(fx, f.W)
			x1 := min(x0+1, f.W-1)
			o := y*size + x
			for c := 0; c < 3; c++ {
				p00 := float32(f.Pix[(y0*f.W+x0)*3+c])
				p01 := float32(f.Pix[(y0*f.W+x1)*3+c])
				p10 := float32(f.Pix[(y1*f.W+x0)*3+c])
				p11 := float32(f.Pix[(y1*f.W+x1)*3+c])
				top := p00 + (p01-p00)*wx
				bot := p10 + (p11-p10)*wx
				dst[c*plane+o] = top + (bot-top)*wy
			}
		}
	}
}

// splitCoord는 좌표를 정수 칸과 가중치로 나눈다.
func splitCoord(v float32, n int) (int, float32) {
	if v <= 0 {
		return 0, 0
	}
	i := int(v)
	if i >= n-1 {
		return n - 1, 0
	}
	return i, v - float32(i)
}
