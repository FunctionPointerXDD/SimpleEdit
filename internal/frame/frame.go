// Frame.Pix는 bgr24이고 len(Pix) = W*H*3이다.
// Blur의 상자 평균 3회는 가우시안에 가깝고, 시그마는 대략 radius다.
// 반지름을 blurCell 안팎이 되게 사각형을 줄여 흐린 뒤 키운다. 모양은 같고 계산은 작다.
// BlurEllipse는 사각형 전체를 흐린 뒤 그 안에 꼭 맞는 타원 안쪽만 쓴다. 경계 1px은 섞어 계단을 없앤다.

// Package frame은 bgr24 프레임과 블러를 다룬다.
package frame

import (
	"image"
	"math"
)

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

const blurCell = 4

// 사각형 안 전체를 흐린다.
func (f Frame) Blur(r image.Rectangle, radius int) { f.blur(r, radius, false) }

// 사각형에 꼭 맞는 타원 안쪽만 흐린다.
func (f Frame) BlurEllipse(r image.Rectangle, radius int) { f.blur(r, radius, true) }

// 칸 평균으로 줄여 상자 평균 3회로 흐린 뒤 쌍선형으로 키워 쓴다. 타원이면 가중치로 원본과 섞는다.
func (f Frame) blur(shape image.Rectangle, radius int, ellipse bool) {
	r := shape.Intersect(image.Rect(0, 0, f.W, f.H))
	if r.Empty() || radius < 1 {
		return
	}
	var alpha []float32
	if ellipse {
		alpha = ellipseWeights(shape, r)
	}
	k := max(1, radius/blurCell)
	w, h := r.Dx(), r.Dy()
	sw, sh := (w+k-1)/k, (h+k-1)/k
	rad := max(1, (radius+k/2)/k)
	small := make([]int32, sw*sh)
	line := make([]int32, max(sw, sh))
	out := make([]int32, max(sw, sh))
	xs, ys := upscaleTaps(w, sw, k), upscaleTaps(h, sh, k)
	for c := 0; c < 3; c++ {
		f.shrink(r, k, c, small, sw, sh)
		for range 3 {
			for y := 0; y < sh; y++ {
				boxLine(small[y*sw:y*sw+sw], 1, line[:sw], out[:sw], rad)
			}
			for x := 0; x < sw; x++ {
				boxLine(small[x:], sw, line[:sh], out[:sh], rad)
			}
		}
		for y := 0; y < h; y++ {
			ty := ys[y]
			row := f.Pix[((r.Min.Y+y)*f.W+r.Min.X)*3:]
			for x := 0; x < w; x++ {
				tx := xs[x]
				p00, p01 := small[ty.i*sw+tx.i], small[ty.i*sw+tx.j]
				p10, p11 := small[ty.j*sw+tx.i], small[ty.j*sw+tx.j]
				top := float32(p00) + float32(p01-p00)*tx.t
				bot := float32(p10) + float32(p11-p10)*tx.t
				v := top + (bot-top)*ty.t
				if alpha != nil {
					wt := alpha[y*w+x]
					if wt <= 0 {
						continue
					}
					o := float32(row[x*3+c])
					v = o + (v-o)*wt
				}
				row[x*3+c] = byte(v + 0.5)
			}
		}
	}
}

// 픽셀 중심이 shape 내접 타원 경계에서 안쪽으로 몇 px인지 재어 +0.5를 0~1로 자른 값
func ellipseWeights(shape, r image.Rectangle) []float32 {
	cx, cy := float64(shape.Min.X+shape.Max.X)/2, float64(shape.Min.Y+shape.Max.Y)/2
	ax, ay := float64(shape.Dx())/2, float64(shape.Dy())/2
	scale := math.Min(ax, ay)
	out := make([]float32, r.Dx()*r.Dy())
	for y := 0; y < r.Dy(); y++ {
		ny := (float64(r.Min.Y+y) + 0.5 - cy) / ay
		for x := 0; x < r.Dx(); x++ {
			nx := (float64(r.Min.X+x) + 0.5 - cx) / ax
			inside := (1 - math.Hypot(nx, ny)) * scale
			out[y*r.Dx()+x] = float32(math.Max(0, math.Min(1, inside+0.5)))
		}
	}
	return out
}

// 한 칸(k×k, 끝은 남은 만큼)의 c 채널 평균을 small에 채운다.
func (f Frame) shrink(r image.Rectangle, k, c int, small []int32, sw, sh int) {
	w, h := r.Dx(), r.Dy()
	for j := 0; j < sh; j++ {
		y0, y1 := j*k, min((j+1)*k, h)
		for i := 0; i < sw; i++ {
			x0, x1 := i*k, min((i+1)*k, w)
			var sum int32
			for y := y0; y < y1; y++ {
				row := f.Pix[((r.Min.Y+y)*f.W+r.Min.X)*3:]
				for x := x0; x < x1; x++ {
					sum += int32(row[x*3+c])
				}
			}
			n := int32((x1 - x0) * (y1 - y0))
			small[j*sw+i] = (sum + n/2) / n
		}
	}
}

type tap struct {
	i, j int
	t    float32
}

// 픽셀 중심을 작은 격자 좌표로 옮겨 이웃 두 칸과 가중치를 미리 구한다.
func upscaleTaps(n, sn, k int) []tap {
	taps := make([]tap, n)
	for x := range taps {
		v := (float32(x)+0.5)/float32(k) - 0.5
		i := int(max(0, v))
		t := max(0, v) - float32(i)
		if i >= sn-1 {
			i, t = sn-1, 0
		}
		taps[x] = tap{i: i, j: min(i+1, sn-1), t: t}
	}
	return taps
}

// stride 간격 값을 line에 모아 끝을 늘린 이동 합으로 평균 내고 제자리에 쓴다.
func boxLine(p []int32, stride int, line, out []int32, radius int) {
	n := len(line)
	for i := range line {
		line[i] = p[i*stride]
	}
	at := func(i int) int32 { return line[min(max(i, 0), n-1)] }
	k := int32(2*radius + 1)
	var sum int32
	for i := -radius; i <= radius; i++ {
		sum += at(i)
	}
	for i := 0; i < n; i++ {
		out[i] = (sum + k/2) / k
		sum += at(i+radius+1) - at(i-radius)
	}
	for i, v := range out {
		p[i*stride] = v
	}
}
