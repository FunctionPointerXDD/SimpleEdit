// Box는 원본 프레임 픽셀 좌표와 점수다.
// 모델 입력은 긴 변을 limit 이하로 줄이고 32의 배수로 늘린 캔버스다(OpenCV FaceDetectorYN과 같음).

// Package detect는 ONNX 모델로 얼굴·번호판을 찾는다.
package detect

import (
	"math"

	"simpleEdit/internal/frame"
)

type Box struct {
	X, Y, W, H float32
	Score      float32
}

const inputAlign = 32

// 긴 변이 limit를 넘으면 비율대로 줄이고, 캔버스는 두 변을 inputAlign 배수로 올린다.
func InputSize(w, h, limit int) (scale float32, nw, nh, pw, ph int) {
	scale = min(1, float32(limit)/float32(max(w, h)))
	nw = int(math.Round(float64(float32(w) * scale)))
	nh = int(math.Round(float64(float32(h) * scale)))
	align := func(v int) int { return (v + inputAlign - 1) / inputAlign * inputAlign }
	return scale, nw, nh, align(nw), align(nh)
}

// InputSize 캔버스 왼쪽 위에 그림을 두고 나머지는 pad로 채운 CHW(BGR)를 dst에 쓴다.
func Letterbox(f frame.Frame, limit int, pad float32, dst []float32) (scale float32, pw, ph int) {
	scale, nw, nh, pw, ph := InputSize(f.W, f.H, limit)
	for i := range dst[:3*pw*ph] {
		dst[i] = pad
	}
	if scale == 1 {
		copyPlanes(f, pw, ph, dst)
		return 1, pw, ph
	}
	resizeBilinear(f, nw, nh, pw, ph, dst)
	return scale, pw, ph
}

// 줄이지 않을 때 HWC 바이트를 채널별 평면(한 줄 pw칸)으로 옮긴다.
func copyPlanes(f frame.Frame, pw, ph int, dst []float32) {
	plane := pw * ph
	for y := 0; y < f.H; y++ {
		row := f.Pix[y*f.W*3:]
		for x := 0; x < f.W; x++ {
			o := y*pw + x
			dst[o] = float32(row[x*3])
			dst[plane+o] = float32(row[x*3+1])
			dst[2*plane+o] = float32(row[x*3+2])
		}
	}
}

// OpenCV INTER_LINEAR처럼 픽셀 중심 좌표로 쌍선형 보간한다.
func resizeBilinear(f frame.Frame, nw, nh, pw, ph int, dst []float32) {
	plane := pw * ph
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
			o := y*pw + x
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

// 음수는 0, 끝을 넘으면 n-1에 붙이고 정수부와 소수부로 나눈다.
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
