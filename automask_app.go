package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"

	"simpleEdit/internal/applog"
	"simpleEdit/internal/automask"
	"simpleEdit/internal/detect"
	"simpleEdit/internal/media"
	"simpleEdit/internal/tools"
)

// 자동 가리기 종류 (번호판은 P5에서 추가)
const kindFace = "face"

// 분석 진행률 이벤트 이름
const evAnalyzeProgress = "analyze:progress"

// errStaleAuto는 분석한 구간과 저장 구간이 다를 때 쓴다.
var errStaleAuto = errors.New("자동 분석 구간 불일치")

// AnalyzeRequest는 자동으로 찾을 종류와 구간이다.
type AnalyzeRequest struct {
	Kind  string  `json:"kind"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// AutoMask는 프론트 표시용 박스 [x,y,w,h,트랙 번호]다.
type AutoMask struct {
	Kind     string         `json:"kind"`
	Start    float64        `json:"start"`
	Duration float64        `json:"duration"`
	FPS      float64        `json:"fps"`
	Found    int            `json:"found"`
	Frames   [][][5]float32 `json:"frames"`
}

// toAutoMask는 JSON을 줄이려고 소수 4자리 배열로 바꾼다.
func toAutoMask(kind string, r automask.Result) AutoMask {
	round := func(v float64) float32 { return float32(math.Round(v*1e4) / 1e4) }
	frames := make([][][5]float32, len(r.Frames))
	for n, boxes := range r.Frames {
		frames[n] = make([][5]float32, len(boxes))
		for i, b := range boxes {
			frames[n][i] = [5]float32{round(b.Rect.X), round(b.Rect.Y), round(b.Rect.W), round(b.Rect.H), float32(b.ID)}
		}
	}
	return AutoMask{Kind: kind, Start: r.Start, Duration: r.Duration, FPS: r.FPS, Found: r.Found, Frames: frames}
}

// Analyze는 구간에서 얼굴 등을 찾아 프레임별 박스를 준다.
func (a *App) Analyze(req AnalyzeRequest) (AutoMask, error) {
	if req.Kind != kindFace {
		return AutoMask{}, userError(msgAnalyzeFail, fmt.Errorf("지원하지 않는 종류: %s", req.Kind))
	}
	a.mu.Lock()
	source, info := a.source, a.info
	a.mu.Unlock()
	if source == "" {
		return AutoMask{}, userError(msgNoVideo, nil)
	}
	start, end, err := clipRange(info, req.Start, req.End)
	if err != nil {
		return AutoMask{}, err
	}
	ctx, done, err := a.jobs.Begin(a.ctx)
	if err != nil {
		return AutoMask{}, analyzeError(err)
	}
	defer done()

	det, err := a.faceDetector()
	if err != nil {
		return AutoMask{}, analyzeError(err)
	}
	emit := a.progressEmitter(evAnalyzeProgress)
	emit(0, "analyze")
	spec := pipeSpec(source, info, start, end)
	applog.Printf("얼굴 분석: %.2f~%.2f초, %.3ffps", start, end, info.FPS)
	res, err := automask.Analyze(ctx, spec, det, automask.OptionsFor(info.FPS), func(f float64) { emit(f, "analyze") })
	if err != nil {
		return AutoMask{}, analyzeError(err)
	}
	applog.Printf("얼굴 분석 완료: 프레임 %d개, 검출 %d건", len(res.Frames), res.Found)
	a.mu.Lock()
	if a.source == source {
		a.auto[req.Kind] = res
	}
	a.mu.Unlock()
	return toAutoMask(req.Kind, res), nil
}

// faceDetector는 런타임과 모델을 처음 한 번만 불러온다.
func (a *App) faceDetector() (*detect.FaceDetector, error) {
	a.detMu.Lock()
	defer a.detMu.Unlock()
	if a.faceDet != nil {
		return a.faceDet, nil
	}
	lib, model := tools.ORTLib(), tools.ModelPath(tools.FaceModel)
	for _, p := range []string{lib, model} {
		if _, err := os.Stat(p); err != nil {
			return nil, fmt.Errorf("%w: %s", tools.ErrMissing, p)
		}
	}
	if err := detect.Init(lib); err != nil {
		return nil, err
	}
	det, err := detect.NewFaceDetector(model)
	if err != nil {
		return nil, err
	}
	a.faceDet = det
	return det, nil
}

// closeDetectors는 종료할 때 검출기와 런타임을 정리한다.
func (a *App) closeDetectors() {
	a.detMu.Lock()
	defer a.detMu.Unlock()
	if a.faceDet != nil {
		a.faceDet.Close()
		a.faceDet = nil
		if err := detect.Shutdown(); err != nil {
			applog.Printf("onnxruntime 정리 실패: %v", err)
		}
	}
}

// autoFrames는 켜진 종류에서 뺀 트랙을 빼고 합친다.
func (a *App) autoFrames(kinds []string, exclude map[string][]int, start, end float64) ([][]media.Rect, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var merged [][]media.Rect
	for _, k := range kinds {
		res, ok := a.auto[k]
		if !ok || !sameRange(res, start, end) {
			return nil, errStaleAuto
		}
		skip := map[int]bool{}
		for _, id := range exclude[k] {
			skip[id] = true
		}
		for len(merged) < len(res.Frames) {
			merged = append(merged, nil)
		}
		for n, boxes := range automask.Rects(res.Frames, skip) {
			merged[n] = append(merged[n], boxes...)
		}
	}
	return merged, nil
}

// 분석 구간과 저장 구간을 같은 것으로 볼 허용 오차(초)
const rangeTolerance = 1e-3

func sameRange(r automask.Result, start, end float64) bool {
	return math.Abs(r.Start-start) < rangeTolerance && math.Abs(r.Start+r.Duration-end) < rangeTolerance
}

// clipRange는 저장·분석에 같이 쓰는 구간 검사다.
func clipRange(info media.Info, reqStart, reqEnd float64) (float64, float64, error) {
	start := max(0, reqStart)
	end := min(info.Duration, reqEnd)
	if end <= start {
		return 0, 0, userError(msgBadRange, nil)
	}
	if end-start < minClipSeconds {
		return 0, 0, userError(msgTooShort, nil)
	}
	return start, end, nil
}

func pipeSpec(source string, info media.Info, start, end float64) media.PipeSpec {
	return media.PipeSpec{
		Source: source, Start: start, Duration: end - start,
		FPS: info.FPS, Width: info.Width, Height: info.Height,
	}
}

// exportAuto는 자동 박스와 수동 사각형을 프레임마다 가려 저장한다.
func exportAuto(ctx context.Context, spec media.PipeSpec, out string, manual []media.Rect, auto [][]media.Rect, onProgress media.ProgressFunc) error {
	return media.ProcessFrames(ctx, spec, out, automask.PixelateFunc(manual, auto), onProgress)
}
