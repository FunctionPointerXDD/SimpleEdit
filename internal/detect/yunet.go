package detect

import (
	"errors"
	"fmt"
	"math"

	ort "github.com/yalue/onnxruntime_go"
)

// YuNet 2023mar 모델의 고정 입력 크기
const yunetSize = 640

// YuNet 출력의 stride 단계
var yunetStrides = [3]int{8, 16, 32}

// FaceDetector는 YuNet으로 얼굴을 찾는다.
type FaceDetector struct {
	ScoreThreshold float32
	NMSThreshold   float32

	sess  *ort.AdvancedSession
	input *ort.Tensor[float32]
	outs  []*ort.Tensor[float32]
}

// yunetOutputs는 stride별 cls, obj, bbox 원시 값이다.
type yunetOutputs struct {
	cls, obj, bbox [3][]float32
}

// NewFaceDetector는 모델을 열어 검출기를 만든다. Init 먼저
func NewFaceDetector(modelPath string) (*FaceDetector, error) {
	in, err := ort.NewEmptyTensor[float32](ort.NewShape(1, 3, yunetSize, yunetSize))
	if err != nil {
		return nil, err
	}
	d := &FaceDetector{ScoreThreshold: 0.5, NMSThreshold: 0.3, input: in}
	var names []string
	var outs []ort.Value
	for _, kind := range []string{"cls", "obj", "bbox", "kps"} {
		for _, s := range yunetStrides {
			n := (yunetSize / s) * (yunetSize / s)
			t, err := ort.NewEmptyTensor[float32](ort.NewShape(1, int64(n), yunetChannels(kind)))
			if err != nil {
				d.Close()
				return nil, err
			}
			d.outs = append(d.outs, t)
			names = append(names, fmt.Sprintf("%s_%d", kind, s))
			outs = append(outs, t)
		}
	}
	d.sess, err = ort.NewAdvancedSession(modelPath, []string{"input"}, names, []ort.Value{in}, outs, nil)
	if err != nil {
		d.Close()
		return nil, fmt.Errorf("YuNet 세션: %w", err)
	}
	return d, nil
}

func yunetChannels(kind string) int64 {
	switch kind {
	case "bbox":
		return 4
	case "kps":
		return 10
	}
	return 1
}

// Detect는 프레임에서 얼굴 박스를 원본 좌표로 돌려준다.
func (d *FaceDetector) Detect(f Frame) ([]Box, error) {
	if d.sess == nil {
		return nil, errors.New("닫힌 검출기")
	}
	scale := Letterbox(f, yunetSize, 0, d.input.GetData())
	if err := d.sess.Run(); err != nil {
		return nil, fmt.Errorf("YuNet 추론: %w", err)
	}
	var o yunetOutputs
	for i := range yunetStrides {
		o.cls[i] = d.outs[i].GetData()
		o.obj[i] = d.outs[3+i].GetData()
		o.bbox[i] = d.outs[6+i].GetData()
	}
	boxes := NMS(decodeYuNet(o, d.ScoreThreshold), d.NMSThreshold)
	return toFrame(boxes, scale, f.W, f.H), nil
}

// Close는 세션과 텐서를 해제한다.
func (d *FaceDetector) Close() {
	if d.sess != nil {
		d.sess.Destroy()
		d.sess = nil
	}
	for _, t := range d.outs {
		t.Destroy()
	}
	d.outs = nil
	if d.input != nil {
		d.input.Destroy()
		d.input = nil
	}
}

// decodeYuNet은 FaceDetectorYN과 같이 점수·박스를 푼다.
func decodeYuNet(o yunetOutputs, scoreTh float32) []Box {
	var boxes []Box
	for i, s := range yunetStrides {
		cols := yunetSize / s
		sf := float32(s)
		for idx := range o.cls[i] {
			cls := clamp01(o.cls[i][idx])
			obj := clamp01(o.obj[i][idx])
			score := float32(math.Sqrt(float64(cls * obj)))
			if score < scoreTh {
				continue
			}
			r, c := idx/cols, idx%cols
			bb := o.bbox[i][idx*4 : idx*4+4]
			cx := (float32(c) + bb[0]) * sf
			cy := (float32(r) + bb[1]) * sf
			w := float32(math.Exp(float64(bb[2]))) * sf
			h := float32(math.Exp(float64(bb[3]))) * sf
			boxes = append(boxes, Box{X: cx - w/2, Y: cy - h/2, W: w, H: h, Score: score})
		}
	}
	return boxes
}

// toFrame은 모델 좌표를 원본 프레임 좌표로 바꾸고 잘라낸다.
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
