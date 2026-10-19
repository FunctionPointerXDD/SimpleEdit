// Tracker는 OpenCV Zoo VitTrack(object_tracking_vittrack_2023sep.onnx)을
// OpenCV TrackerVit 원본 코드(tracker_vit.cpp)와 같은 앞뒤 처리로 돌린다.
// 같은 처리를 Python으로 옮긴 것과 BGR 프레임에서 상자 1px·점수 0.03 안으로 같다. pip opencv 5.0의 내장 추적기와는
// 점수가 0.1~0.2 다른데 원인은 못 찾았다(모델 실행 결과는 같음). 낮 번호판은 오히려 더 오래 따라갔다(10-06).
// Init: 상자 주변을 √(w·h)×2 정사각형으로 잘라(밖은 0) 128×128로 줄여 "template"으로 둔다(이후 고정).
// Update: 직전 상자 주변 √(w·h)×4를 256×256으로 "search"에 넣고, 16×16 확신도에 Hanning 창을
// 곱한 최댓값 칸(행 우선 첫 칸)의 크기·칸 안 위치로 새 상자를 정한다. 최댓값이 LostScore 미만이면 놓침.
// 정규화는 OpenCV처럼 BGR 순서 그대로 채널마다 (값 − mean×255) / (std×255)다.
// 자를 때 오른쪽·아래 끝 한 줄을 0으로 두는 OpenCV 계산(x2_pad = x2 − W + 1)도 그대로 따른다.
// 놓쳐도 점수가 LostScore 위에 남은 채 상자가 커지면, 탐색 범위(상자의 4배)도 커져 화면 전체를
// 덮은 채 끝까지 "따라가는" 일이 있었다(10-06 사용자 신고, 밤 영상에서 한 번에 면적 147배).
// 그래서 sane이 앞 상자 대비 면적 maxStepGrow배 초과·minStepShrink배 미만, 처음 대비 maxGrow배 초과·
// minShrink배 미만, 중심이 화면 밖이면 점수와 상관없이 놓침으로 본다.
// 프레임은 BGR이나 YUV420 모두 받고, YUV420이면 자를 영역만 BGR로 바꾼다(화면 전체 변환을 피함).
// 변환식은 ffmpeg의 yuv420p→bgr24 기본과 같은 BT.601 제한 범위이고, 색은 2×2 칸 하나를 그대로 쓴다.
// 줄이기는 쌍선형(픽셀 중심 기준) 뒤 OpenCV처럼 8비트로 반올림한다. 고정소수점 계산과는 아주 조금 다를 수 있다.

package detect

import (
	"errors"
	"fmt"
	"math"

	ort "github.com/yalue/onnxruntime_go"

	"simpleEdit/internal/frame"
)

const (
	trackTemplate = 128
	trackSearch   = 256
	trackGrid     = 16
	LostScore     = 0.2

	maxStepGrow   = 1.6
	minStepShrink = 0.6
	maxGrow       = 9.0
	minShrink     = 1.0 / 16
)

var (
	trackMean = [3]float32{0.485, 0.456, 0.406}
	trackStd  = [3]float32{0.229, 0.224, 0.225}
)

type PixelBox struct{ X, Y, W, H int }

type Tracker struct {
	sess   *ort.DynamicAdvancedSession
	tmpl   *ort.Tensor[float32]
	search *ort.Tensor[float32]
	outs   [3]*ort.Tensor[float32]
	hann   [trackGrid * trackGrid]float32
	last   PixelBox
	first  PixelBox
	ready  bool
}

// Init 뒤에 부른다. 입력·출력 크기가 고정이라 텐서를 처음에 만든다.
func NewTracker(modelPath string) (*Tracker, error) {
	opt, err := sessionOptions()
	if err != nil {
		return nil, err
	}
	defer opt.Destroy()
	sess, err := ort.NewDynamicAdvancedSession(modelPath, []string{"template", "search"},
		[]string{"output1", "output2", "output3"}, opt)
	if err != nil {
		return nil, fmt.Errorf("추적 세션: %w", err)
	}
	t := &Tracker{sess: sess}
	if err := t.tensors(); err != nil {
		t.Close()
		return nil, err
	}
	t.hann = hann2d()
	return t, nil
}

func (t *Tracker) tensors() error {
	var err error
	if t.tmpl, err = ort.NewEmptyTensor[float32](ort.NewShape(1, 3, trackTemplate, trackTemplate)); err != nil {
		return err
	}
	if t.search, err = ort.NewEmptyTensor[float32](ort.NewShape(1, 3, trackSearch, trackSearch)); err != nil {
		return err
	}
	for i, ch := range []int64{1, 2, 2} {
		if t.outs[i], err = ort.NewEmptyTensor[float32](ort.NewShape(1, ch, trackGrid, trackGrid)); err != nil {
			return err
		}
	}
	return nil
}

// 상자 주변을 2배로 잘라 template에 넣고 직전 상자로 둔다. 상자가 비면 오류
func (t *Tracker) Init(f frame.Frame, box PixelBox) error {
	if t.sess == nil {
		return errors.New("닫힌 추적기")
	}
	if box.W < 1 || box.H < 1 {
		return errors.New("빈 상자")
	}
	cropResize(f, box, 2, trackTemplate, t.tmpl.GetData())
	t.last, t.first, t.ready = box, box, true
	return nil
}

// 직전 상자 주변을 4배로 잘라 추론하고, 점수가 LostScore 이상이면 새 상자와 true를 준다.
func (t *Tracker) Update(f frame.Frame) (PixelBox, float32, bool, error) {
	if t.sess == nil || !t.ready {
		return PixelBox{}, 0, false, errors.New("시작하지 않은 추적기(Init 전)")
	}
	crop := cropResize(f, t.last, 4, trackSearch, t.search.GetData())
	if err := t.sess.Run([]ort.Value{t.tmpl, t.search}, []ort.Value{t.outs[0], t.outs[1], t.outs[2]}); err != nil {
		return PixelBox{}, 0, false, fmt.Errorf("추적 추론: %w", err)
	}
	conf, size, off := t.outs[0].GetData(), t.outs[1].GetData(), t.outs[2].GetData()
	best, score := 0, float32(math.Inf(-1))
	for i, c := range conf {
		if v := c * t.hann[i]; v > score {
			best, score = i, v
		}
	}
	if score < LostScore {
		return t.last, score, false, nil
	}
	const n = trackGrid * trackGrid
	gx, gy := best%trackGrid, best/trackGrid
	cx := (float32(gx) + off[best]) / trackGrid
	cy := (float32(gy) + off[n+best]) / trackGrid
	w, h := size[best], size[n+best]
	next := nextBox(t.last, cx, cy, w, h, crop)
	if !sane(next, t.last, t.first, f.W, f.H) {
		return t.last, score, false, nil
	}
	t.last = next
	return next, score, true, nil
}

func (t *Tracker) Close() {
	if t.sess != nil {
		t.sess.Destroy()
		t.sess = nil
	}
	for _, x := range append([]*ort.Tensor[float32]{t.tmpl, t.search}, t.outs[:]...) {
		if x != nil {
			x.Destroy()
		}
	}
	t.tmpl, t.search, t.outs = nil, nil, [3]*ort.Tensor[float32]{}
}

// 크기가 있고, 앞·처음 상자 대비 면적 변화가 한도 안이며, 중심이 화면 안이면 true
func sane(next, last, first PixelBox, w, h int) bool {
	if next.W < 1 || next.H < 1 {
		return false
	}
	area := func(b PixelBox) float64 { return float64(b.W) * float64(b.H) }
	step, total := area(next)/area(last), area(next)/area(first)
	cx, cy := next.X+next.W/2, next.Y+next.H/2
	return step <= maxStepGrow && step >= minStepShrink && total <= maxGrow && total >= minShrink &&
		cx >= 0 && cx < w && cy >= 0 && cy < h
}

// OpenCV updateLastRect: 자른 칸 기준 0~1 중심·크기를 원본 픽셀로 내림해 바꾼다.
func nextBox(last PixelBox, cx, cy, w, h float32, crop int) PixelBox {
	x0 := last.X + (last.W-crop)/2
	y0 := last.Y + (last.H-crop)/2
	c := float32(crop)
	return PixelBox{
		X: int(math.Floor(float64((cx-w/2)*c + float32(x0)))),
		Y: int(math.Floor(float64((cy-h/2)*c + float32(y0)))),
		W: int(math.Floor(float64(w * c))),
		H: int(math.Floor(float64(h * c))),
	}
}

// OpenCV crop_image처럼 √(w·h)×factor 정사각형(밖은 0)을 BGR로 떼어 size로 쌍선형 축소해 정규화 CHW로 쓴다.
func cropResize(f frame.Frame, b PixelBox, factor, size int, dst []float32) int {
	crop := int(math.Ceil(math.Sqrt(float64(b.W*b.H)) * float64(factor)))
	src := cropBGR(f, b.X+(b.W-crop)/2, b.Y+(b.H-crop)/2, crop)
	plane := size * size
	scale := float32(crop) / float32(size)
	at := func(x, y, c int) float32 { return float32(src[(y*crop+x)*3+c]) }
	for y := 0; y < size; y++ {
		sy0, wy := splitCoord((float32(y)+0.5)*scale-0.5, crop)
		sy1 := min(sy0+1, crop-1)
		for x := 0; x < size; x++ {
			sx0, wx := splitCoord((float32(x)+0.5)*scale-0.5, crop)
			sx1 := min(sx0+1, crop-1)
			for c := 0; c < 3; c++ {
				top := at(sx0, sy0, c) + (at(sx1, sy0, c)-at(sx0, sy0, c))*wx
				bot := at(sx0, sy1, c) + (at(sx1, sy1, c)-at(sx0, sy1, c))*wx
				v := float32(math.Round(float64(top + (bot-top)*wy)))
				dst[c*plane+y*size+x] = (v - trackMean[c]*255) / (trackStd[c] * 255)
			}
		}
	}
	return crop
}

// (x1, y1)에서 crop×crop을 BGR 바이트로 뗀다. 밖과 오른쪽·아래 끝 한 줄은 0, YUV420은 이 영역만 변환
func cropBGR(f frame.Frame, x1, y1, crop int) []byte {
	out := make([]byte, crop*crop*3)
	x2, y2 := x1+crop, y1+crop
	vx0, vy0 := max(x1, 0), max(y1, 0)
	vx1, vy1 := x2-max(x2-f.W+1, 0), y2-max(y2-f.H+1, 0)
	cw, n := f.W/2, f.W*f.H
	for ay := vy0; ay < vy1; ay++ {
		row := out[((ay-y1)*crop+(vx0-x1))*3:]
		for ax := vx0; ax < vx1; ax++ {
			o := (ax - vx0) * 3
			if f.Format != frame.YUV420 {
				copy(row[o:o+3], f.Pix[(ay*f.W+ax)*3:])
				continue
			}
			ci := (ay/2)*cw + ax/2
			row[o], row[o+1], row[o+2] = yuvToBGR(f.Pix[ay*f.W+ax], f.Pix[n+ci], f.Pix[n+n/4+ci])
		}
	}
	return out
}

func yuvToBGR(y, u, v byte) (byte, byte, byte) {
	c := 1.164 * (float32(y) - 16)
	d, e := float32(u)-128, float32(v)-128
	clip := func(x float32) byte { return byte(max(0, min(255, x+0.5))) }
	return clip(c + 2.018*d), clip(c - 0.391*d - 0.813*e), clip(c + 1.596*e)
}

// OpenCV hann2d(centered): 0.5·(1 − cos(2π/(n+1)·(i+1)))의 행·열 곱
func hann2d() [trackGrid * trackGrid]float32 {
	var h1 [trackGrid]float32
	for i := range h1 {
		h1[i] = 0.5 * (1 - float32(math.Cos(2*math.Pi/float64(trackGrid+1)*float64(i+1))))
	}
	var out [trackGrid * trackGrid]float32
	for y := range h1 {
		for x := range h1 {
			out[y*trackGrid+x] = h1[y] * h1[x]
		}
	}
	return out
}
