// PlateDetector는 번호판 1종류로 학습한 YOLOX-Tiny(scripts/plate)를 정적 INT8(QDQ)로 바꾼 모델을 쓴다.
// INT8도 입력·출력은 float32라 코드는 FP32와 같다(scripts/plate/quantize.py, Go·Python 결과 일치).
// 입력 "images" [1,3,H,W]는 BGR 0~255 그대로, 그림을 왼쪽 위에 두고 나머지를 114로 채운
// 32의 배수 캔버스다(학습과 같음).
// 출력 "output" [1,N,6]은 stride 8·16·32 격자 순서의 (cx,cy,w,h,obj,cls)이고
// obj·cls는 sigmoid 뒤 값이다. 중심 = (값 + 격자) × stride, 크기 = exp(값) × stride,
// 점수 = obj × cls (scripts/plate/detect.py와 같음, 같은 프레임에서 박스·점수 일치 확인 10-05).
// DefaultPlateScore 0.3은 트랙을 잇는 데만 쓰는 낮은 기준이고, 새 번호판 인정(0.7)은 automask가 한다.
// 입력 텐서 2개로 Prepare·Infer를 겹치는 방식은 FaceDetector와 같다.

package detect

import (
	"errors"
	"fmt"
	"math"

	ort "github.com/yalue/onnxruntime_go"

	"simpleEdit/internal/frame"
)

const (
	plateLimit = 960
	platePad   = 114
	plateOut   = 6
)

const DefaultPlateScore = 0.3

var plateStrides = [3]int{8, 16, 32}

type PlateDetector struct {
	ScoreThreshold float32
	NMSThreshold   float32

	sess   *ort.DynamicAdvancedSession
	pw, ph int
	inputs [2]*ort.Tensor[float32]
	out    *ort.Tensor[float32]
}

// Init 뒤에 부른다. 입력 크기가 정해지지 않은 세션만 열고 텐서는 Prepare에서 만든다.
func NewPlateDetector(modelPath string) (*PlateDetector, error) {
	opt, err := sessionOptions()
	if err != nil {
		return nil, err
	}
	defer opt.Destroy()
	sess, err := ort.NewDynamicAdvancedSession(modelPath, []string{"images"}, []string{"output"}, opt)
	if err != nil {
		return nil, fmt.Errorf("번호판 세션: %w", err)
	}
	return &PlateDetector{ScoreThreshold: DefaultPlateScore, NMSThreshold: 0.45, sess: sess}, nil
}

// 캔버스 크기가 바뀔 때만 입력 2개·출력 텐서를 새로 만든다.
func (d *PlateDetector) tensors(pw, ph int) error {
	if d.inputs[0] != nil && d.pw == pw && d.ph == ph {
		return nil
	}
	d.freeTensors()
	for i := range d.inputs {
		in, err := ort.NewEmptyTensor[float32](ort.NewShape(1, 3, int64(ph), int64(pw)))
		if err != nil {
			d.freeTensors()
			return err
		}
		d.inputs[i] = in
	}
	out, err := ort.NewEmptyTensor[float32](ort.NewShape(1, int64(plateAnchors(pw, ph)), plateOut))
	if err != nil {
		d.freeTensors()
		return err
	}
	d.out, d.pw, d.ph = out, pw, ph
	return nil
}

func (d *PlateDetector) freeTensors() {
	if d.out != nil {
		d.out.Destroy()
		d.out = nil
	}
	for i, in := range d.inputs {
		if in != nil {
			in.Destroy()
			d.inputs[i] = nil
		}
	}
}

// stride마다 (pw/s)×(ph/s) 칸을 더한다.
func plateAnchors(pw, ph int) int {
	n := 0
	for _, s := range plateStrides {
		n += (pw / s) * (ph / s)
	}
	return n
}

// 슬롯 0에 Prepare한 뒤 바로 Infer한다.
func (d *PlateDetector) Detect(f frame.Frame) ([]Box, error) {
	p, err := d.Prepare(0, f)
	if err != nil {
		return nil, err
	}
	return d.Infer(p)
}

// 입력 slot(0·1)에 114로 채운 letterbox를 쓰고 Infer에 넘길 비율·크기를 돌려준다.
func (d *PlateDetector) Prepare(slot int, f frame.Frame) (Prepared, error) {
	if d.sess == nil {
		return Prepared{}, errors.New("닫힌 검출기")
	}
	_, _, _, pw, ph := InputSize(f.W, f.H, plateLimit)
	if err := d.tensors(pw, ph); err != nil {
		return Prepared{}, err
	}
	scale, _, _ := Letterbox(f, plateLimit, platePad, d.inputs[slot].GetData())
	return Prepared{slot: slot, scale: scale, pw: pw, ph: ph, w: f.W, h: f.H}, nil
}

// p의 슬롯으로 추론 → 디코드 → NMS 뒤 원본 좌표로 되돌린다. 한 번에 하나씩만 부른다.
func (d *PlateDetector) Infer(p Prepared) ([]Box, error) {
	if d.sess == nil {
		return nil, errors.New("닫힌 검출기")
	}
	if err := d.sess.Run([]ort.Value{d.inputs[p.slot]}, []ort.Value{d.out}); err != nil {
		return nil, fmt.Errorf("번호판 추론: %w", err)
	}
	boxes := NMS(decodeYOLOX(d.out.GetData(), p.pw, p.ph, d.ScoreThreshold), d.NMSThreshold)
	return toFrame(boxes, p.scale, p.w, p.h), nil
}

func (d *PlateDetector) Close() {
	if d.sess != nil {
		d.sess.Destroy()
		d.sess = nil
	}
	d.freeTensors()
}

// 격자 순서대로 점수 obj×cls가 기준 이상인 칸만 왼쪽 위 좌표 상자로 푼다.
func decodeYOLOX(out []float32, pw, ph int, scoreTh float32) []Box {
	var boxes []Box
	i := 0
	for _, s := range plateStrides {
		gw, gh := pw/s, ph/s
		for gy := 0; gy < gh; gy++ {
			for gx := 0; gx < gw; gx++ {
				v := out[i*plateOut : i*plateOut+plateOut]
				i++
				score := v[4] * v[5]
				if score < scoreTh {
					continue
				}
				fs := float32(s)
				w := float32(math.Exp(float64(v[2]))) * fs
				h := float32(math.Exp(float64(v[3]))) * fs
				cx, cy := (v[0]+float32(gx))*fs, (v[1]+float32(gy))*fs
				boxes = append(boxes, Box{X: cx - w/2, Y: cy - h/2, W: w, H: h, Score: score})
			}
		}
	}
	return boxes
}
