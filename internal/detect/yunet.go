// DefaultFaceScore 0.4는 놓치지 않게 OpenCV 기본값(0.9)보다 낮춘 값이다.
// yunetOutputs는 stride(8·16·32)별 cls, obj, bbox 원시 출력이다.

package detect

import (
	"errors"
	"fmt"
	"math"

	ort "github.com/yalue/onnxruntime_go"

	"simpleEdit/internal/frame"
)

const yunetSize = 640

const DefaultFaceScore = 0.4

var yunetStrides = [3]int{8, 16, 32}

type FaceDetector struct {
	ScoreThreshold float32
	NMSThreshold   float32

	sess  *ort.AdvancedSession
	input *ort.Tensor[float32]
	outs  []*ort.Tensor[float32]
}

type yunetOutputs struct {
	cls, obj, bbox [3][]float32
}

// Init 뒤에 부른다. 입출력 텐서를 미리 만들어 세션에 묶어 둔다.
func NewFaceDetector(modelPath string) (*FaceDetector, error) {
	in, err := ort.NewEmptyTensor[float32](ort.NewShape(1, 3, yunetSize, yunetSize))
	if err != nil {
		return nil, err
	}
	d := &FaceDetector{ScoreThreshold: DefaultFaceScore, NMSThreshold: 0.3, input: in}
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

// letterbox → 추론 → 디코드 → NMS 뒤 원본 좌표로 되돌린다.
func (d *FaceDetector) Detect(f frame.Frame) ([]Box, error) {
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

// score=√(cls·obj), 중심=(칸+d)·stride, 크기=exp(d)·stride로 푼다.
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

// scale로 나눠 원본 좌표로 바꾸고 프레임 밖은 잘라낸다. 넓이 0이면 버림
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
