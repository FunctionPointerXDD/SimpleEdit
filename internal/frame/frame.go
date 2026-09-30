// Frame.Pix는 Format이 BGR이면 bgr24(W*H*3), YUV420이면 yuv420p 평면 Y·U·V(W*H*3/2)다.
// YUV420은 W·H가 짝수여야 하고 U·V는 가로세로 절반이다. 저장 파이프가 영상 원래 형식 그대로
// 주고받아 색 변환 두 번을 덜려고 쓴다(분석은 YuNet 입력 때문에 BGR).
// 흐림은 평면(채널)마다 따로 한다. 절반 크기 평면은 사각형을 바깥쪽으로 반 내리고, 반지름도 반으로.
// Blur의 상자 평균 3회는 가우시안에 가깝고, 시그마는 대략 radius다.
// 반지름을 blurCell 안팎이 되게 사각형을 줄여 흐린 뒤 키운다. 모양은 같고 계산은 작다.
// BlurEllipse는 사각형 전체를 흐린 뒤 그 안에 꼭 맞는 타원 안쪽만 쓴다.
// 타원 반지름의 바깥 EllipseFeather 비율 구간은 smoothstep으로 원본과 섞어 주변과 이어지게 한다.

// Package frame은 bgr24·yuv420p 프레임과 블러를 다룬다.
package frame

import (
	"image"
	"math"
)

type Format int

const (
	BGR Format = iota
	YUV420
)

type Frame struct {
	W, H   int
	Format Format
	Pix    []byte
}

func New(w, h int) Frame {
	return Frame{W: w, H: h, Pix: make([]byte, w*h*3)}
}

func NewYUV(w, h int) Frame {
	return Frame{W: w, H: h, Format: YUV420, Pix: make([]byte, w*h*3/2)}
}

type plane struct {
	pix        []byte
	w, h, step int
	sub        int
}

// BGR은 한 버퍼를 3바이트 간격으로 세 번, YUV420은 Y 전체와 절반 크기 U·V로 나눈다.
func (f Frame) planes() []plane {
	if f.Format == YUV420 {
		n, cw, ch := f.W*f.H, f.W/2, f.H/2
		return []plane{
			{f.Pix[:n], f.W, f.H, 1, 1},
			{f.Pix[n : n+cw*ch], cw, ch, 1, 2},
			{f.Pix[n+cw*ch:], cw, ch, 1, 2},
		}
	}
	return []plane{{f.Pix, f.W, f.H, 3, 1}, {f.Pix[1:], f.W, f.H, 3, 1}, {f.Pix[2:], f.W, f.H, 3, 1}}
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

const EllipseFeather = 0.25

// 사각형 안 전체를 흐린다.
func (f Frame) Blur(r image.Rectangle, radius int) { f.blur(r, radius, false) }

// 사각형에 꼭 맞는 타원 안쪽만 흐린다.
func (f Frame) BlurEllipse(r image.Rectangle, radius int) { f.blur(r, radius, true) }

// 평면마다 사각형·반지름을 sub로 나눠 흐린다. 타원 가중치는 sub별로 한 번만 구한다.
func (f Frame) blur(shape image.Rectangle, radius int, ellipse bool) {
	if radius < 1 {
		return
	}
	var alpha [3][]float32
	for _, p := range f.planes() {
		r := p.rect(shape)
		if r.Empty() {
			continue
		}
		if ellipse && alpha[p.sub] == nil {
			alpha[p.sub] = ellipseWeights(shape, r, p.sub)
		}
		p.blur(r, max(1, (radius+p.sub/2)/p.sub), alpha[p.sub])
	}
}

// shape를 sub로 나눠 바깥쪽으로 반올림하고 평면 안으로 자른다.
func (p plane) rect(shape image.Rectangle) image.Rectangle {
	s := p.sub
	r := image.Rect(floorDiv(shape.Min.X, s), floorDiv(shape.Min.Y, s),
		-floorDiv(-shape.Max.X, s), -floorDiv(-shape.Max.Y, s))
	return r.Intersect(image.Rect(0, 0, p.w, p.h))
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && a < 0 {
		q--
	}
	return q
}

// 칸 평균으로 줄여 상자 평균 3회로 흐린 뒤 쌍선형으로 키워 쓴다. alpha가 있으면 원본과 섞는다.
func (p plane) blur(r image.Rectangle, radius int, alpha []float32) {
	k := max(1, radius/blurCell)
	w, h := r.Dx(), r.Dy()
	sw, sh := (w+k-1)/k, (h+k-1)/k
	rad := max(1, (radius+k/2)/k)
	small := make([]int32, sw*sh)
	line := make([]int32, max(sw, sh))
	out := make([]int32, max(sw, sh))
	xs, ys := upscaleTaps(w, sw, k), upscaleTaps(h, sh, k)
	p.shrink(r, k, small, sw, sh)
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
		row := p.pix[((r.Min.Y+y)*p.w+r.Min.X)*p.step:]
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
				o := float32(row[x*p.step])
				v = o + (v-o)*wt
			}
			row[x*p.step] = byte(v + 0.5)
		}
	}
}

// 평면 좌표(shape/sub)의 정규화 반지름 q로 (1-q)/EllipseFeather를 smoothstep. 경계는 0
func ellipseWeights(shape, r image.Rectangle, sub int) []float32 {
	s := float64(sub)
	cx, cy := float64(shape.Min.X+shape.Max.X)/2/s, float64(shape.Min.Y+shape.Max.Y)/2/s
	ax, ay := float64(shape.Dx())/2/s, float64(shape.Dy())/2/s
	out := make([]float32, r.Dx()*r.Dy())
	for y := 0; y < r.Dy(); y++ {
		ny := (float64(r.Min.Y+y) + 0.5 - cy) / ay
		for x := 0; x < r.Dx(); x++ {
			nx := (float64(r.Min.X+x) + 0.5 - cx) / ax
			t := math.Max(0, math.Min(1, (1-math.Hypot(nx, ny))/EllipseFeather))
			out[y*r.Dx()+x] = float32(t * t * (3 - 2*t))
		}
	}
	return out
}

// 한 칸(k×k, 끝은 남은 만큼)의 평균을 small에 채운다.
func (p plane) shrink(r image.Rectangle, k int, small []int32, sw, sh int) {
	w, h := r.Dx(), r.Dy()
	for j := 0; j < sh; j++ {
		y0, y1 := j*k, min((j+1)*k, h)
		for i := 0; i < sw; i++ {
			x0, x1 := i*k, min((i+1)*k, w)
			var sum int32
			for y := y0; y < y1; y++ {
				row := p.pix[((r.Min.Y+y)*p.w+r.Min.X)*p.step:]
				for x := x0; x < x1; x++ {
					sum += int32(row[x*p.step])
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
