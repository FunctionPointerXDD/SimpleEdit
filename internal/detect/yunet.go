// DefaultFaceScore 0.6: 0.4는 오탐이 많다는 사용자 판단(2026-09-29)으로 올렸다. OpenCV 기본은 0.9
// yunetOutputs는 stride(8·16·32)별 cls, obj, bbox 원시 출력이다. 랜드마크(kps)는 쓰지 않아 받지 않는다.
// 모델은 2026may(2023mar 가중치를 가변 입력으로 다시 내보낸 것)라 640×360 영상은 640×384로 추론한다.
// 추론 스레드를 논리 코어 전부로 두면 함께 도는 ffmpeg 디코더와 다퉈 느려진다.
// 물리 코어 수(논리 ÷ 2)로 두면 2분 1080p 분석이 37초 → 28초였다(Ryzen 5 5625U).

package detect

import (
	"errors"
	"fmt"
	"math"
	"runtime"

	ort "github.com/yalue/onnxruntime_go"

	"simpleEdit/internal/frame"
)

const yunetLimit = 640

const DefaultFaceScore = 0.6

var yunetStrides = [3]int{8, 16, 32}

var yunetKinds = [3]string{"cls", "obj", "bbox"}

type FaceDetector struct {
	ScoreThreshold float32
	NMSThreshold   float32

	sess   *ort.DynamicAdvancedSession
	pw, ph int
	input  *ort.Tensor[float32]
	outs   []*ort.Tensor[float32]
	vals   []ort.Value
}

type yunetOutputs struct {
	cls, obj, bbox [3][]float32
}

// Init 뒤에 부른다. 입력 크기가 정해지지 않은 세션만 열고 텐서는 Detect에서 만든다.
func NewFaceDetector(modelPath string) (*FaceDetector, error) {
	var names []string
	for _, kind := range yunetKinds {
		for _, s := range yunetStrides {
			names = append(names, fmt.Sprintf("%s_%d", kind, s))
		}
	}
	opt, err := sessionOptions()
	if err != nil {
		return nil, err
	}
	defer opt.Destroy()
	sess, err := ort.NewDynamicAdvancedSession(modelPath, []string{"input"}, names, opt)
	if err != nil {
		return nil, fmt.Errorf("YuNet 세션: %w", err)
	}
	return &FaceDetector{ScoreThreshold: DefaultFaceScore, NMSThreshold: 0.3, sess: sess}, nil
}

// 캔버스 크기가 바뀔 때만 입력·출력 텐서를 새로 만든다.
func (d *FaceDetector) tensors(pw, ph int) error {
	if d.input != nil && d.pw == pw && d.ph == ph {
		return nil
	}
	d.freeTensors()
	in, err := ort.NewEmptyTensor[float32](ort.NewShape(1, 3, int64(ph), int64(pw)))
	if err != nil {
		return err
	}
	d.input, d.pw, d.ph = in, pw, ph
	for _, kind := range yunetKinds {
		for _, s := range yunetStrides {
			n := (pw / s) * (ph / s)
			t, err := ort.NewEmptyTensor[float32](ort.NewShape(1, int64(n), yunetChannels(kind)))
			if err != nil {
				d.freeTensors()
				return err
			}
			d.outs = append(d.outs, t)
			d.vals = append(d.vals, t)
		}
	}
	return nil
}

func (d *FaceDetector) freeTensors() {
	for _, t := range d.outs {
		t.Destroy()
	}
	d.outs, d.vals = nil, nil
	if d.input != nil {
		d.input.Destroy()
		d.input = nil
	}
}

// 연산 내부 스레드는 논리 코어의 절반(최소 1), 연산 사이 병렬은 1개로 둔다.
func sessionOptions() (*ort.SessionOptions, error) {
	opt, err := ort.NewSessionOptions()
	if err != nil {
		return nil, err
	}
	if err := opt.SetIntraOpNumThreads(max(1, runtime.NumCPU()/2)); err != nil {
		opt.Destroy()
		return nil, err
	}
	if err := opt.SetInterOpNumThreads(1); err != nil {
		opt.Destroy()
		return nil, err
	}
	return opt, nil
}

func yunetChannels(kind string) int64 {
	if kind == "bbox" {
		return 4
	}
	return 1
}

// letterbox → 추론 → 디코드 → NMS 뒤 원본 좌표로 되돌린다.
func (d *FaceDetector) Detect(f frame.Frame) ([]Box, error) {
	if d.sess == nil {
		return nil, errors.New("닫힌 검출기")
	}
	_, _, _, pw, ph := InputSize(f.W, f.H, yunetLimit)
	if err := d.tensors(pw, ph); err != nil {
		return nil, err
	}
	scale, _, _ := Letterbox(f, yunetLimit, 0, d.input.GetData())
	if err := d.sess.Run([]ort.Value{d.input}, d.vals); err != nil {
		return nil, fmt.Errorf("YuNet 추론: %w", err)
	}
	var o yunetOutputs
	for i := range yunetStrides {
		o.cls[i] = d.outs[i].GetData()
		o.obj[i] = d.outs[3+i].GetData()
		o.bbox[i] = d.outs[6+i].GetData()
	}
	boxes := NMS(decodeYuNet(o, pw, d.ScoreThreshold), d.NMSThreshold)
	return toFrame(boxes, scale, f.W, f.H), nil
}

func (d *FaceDetector) Close() {
	if d.sess != nil {
		d.sess.Destroy()
		d.sess = nil
	}
	d.freeTensors()
}

// score=√(cls·obj), 중심=(칸+d)·stride, 크기=exp(d)·stride로 푼다.
func decodeYuNet(o yunetOutputs, pw int, scoreTh float32) []Box {
	var boxes []Box
	for i, s := range yunetStrides {
		cols := pw / s
		sf := float32(s)
		for idx := range o.cls[i] {
			cls := clamp01(o.cls[i][idx])
			obj := clamp01(o.obj[i][idx])
			score := float32(math.Sqrt(float64(cls * obj)))
			if score < scoreTh {
				continue
			}
			r, c := float32(idx/cols), float32(idx%cols)
			bb := o.bbox[i][idx*4 : idx*4+4]
			cx := (c + bb[0]) * sf
			cy := (r + bb[1]) * sf
			w := float32(math.Exp(float64(bb[2]))) * sf
			h := float32(math.Exp(float64(bb[3]))) * sf
			boxes = append(boxes, Box{X: cx - w/2, Y: cy - h/2, W: w, H: h, Score: score})
		}
	}
	return boxes
}

// scale로 나눠 원본 좌표로 바꾸고 박스는 프레임 안으로 자른다. 넓이 0이면 버림
func toFrame(boxes []Box, scale float32, w, h int) []Box {
	out := make([]Box, 0, len(boxes))
	for _, b := range boxes {
		x0 := max(0, b.X/scale)
		y0 := max(0, b.Y/scale)
		x1 := min(float32(w), (b.X+b.W)/scale)
		y1 := min(float32(h), (b.Y+b.H)/scale)
		if x1 <= x0 || y1 <= y0 {
			continue
		}
		out = append(out, Box{X: x0, Y: y0, W: x1 - x0, H: y1 - y0, Score: b.Score})
	}
	return out
}

func clamp01(v float32) float32 { return max(0, min(1, v)) }
