// AutoMask.Frames 박스는 [x, y, w, h, 트랙 ID], Tracks는 거르고 남긴 트랙 수다.

package main

import (
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

const kindFace = "face"

const evAnalyzeProgress = "analyze:progress"

var errStaleAuto = errors.New("자동 분석 구간 불일치")

type AnalyzeRequest struct {
	Kind  string  `json:"kind"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type AutoMask struct {
	Kind     string         `json:"kind"`
	Start    float64        `json:"start"`
	Duration float64        `json:"duration"`
	FPS      float64        `json:"fps"`
	Found    int            `json:"found"`
	Tracks   int            `json:"tracks"`
	Frames   [][][5]float32 `json:"frames"`
}

// 좌표를 소수 4자리로 반올림하고 트랙 ID를 다섯째 값으로 붙인다.
func toAutoMask(kind string, r automask.Result) AutoMask {
	round := func(v float64) float32 { return float32(math.Round(v*1e4) / 1e4) }
	frames := make([][][5]float32, len(r.Frames))
	for n, boxes := range r.Frames {
		frames[n] = make([][5]float32, len(boxes))
		for i, b := range boxes {
			frames[n][i] = [5]float32{round(b.Rect.X), round(b.Rect.Y), round(b.Rect.W), round(b.Rect.H), float32(b.ID)}
		}
	}
	return AutoMask{Kind: kind, Start: r.Start, Duration: r.Duration, FPS: r.FPS, Found: r.Found, Tracks: r.Stats.Kept, Frames: frames}
}

// 구간 검사 → 작업 시작 → 검출기 준비 → 분석. 그사이 영상이 바뀌면 결과를 버린다.
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
	applog.Printf("얼굴 분석 완료: 프레임 %d개, 검출 %d건, 트랙 %d개 남김·%d개 버림",
		len(res.Frames), res.Found, res.Stats.Kept, res.Stats.Dropped)
	a.mu.Lock()
	if a.source == source {
		a.auto[req.Kind] = res
	}
	a.mu.Unlock()
	return toAutoMask(req.Kind, res), nil
}

// 처음 한 번 파일을 확인해 런타임·모델을 올려 두고, 이후엔 같은 검출기를 쓴다.
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

// 검출기가 있을 때만 닫고 ONNX 환경까지 해제한다.
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

// 결과가 저장 구간과 같은지 보고 뺀 트랙을 제외해 합친다. 박스가 없으면 nil
func (a *App) autoFrames(kinds []string, exclude map[string][]int, start, end float64) ([][]media.Rect, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var merged [][]media.Rect
	boxes := 0
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
		for n, rs := range automask.Rects(res.Frames, skip) {
			merged[n] = append(merged[n], rs...)
			boxes += len(rs)
		}
	}
	if boxes == 0 {
		return nil, nil
	}
	return merged, nil
}

const rangeTolerance = 1e-3

func sameRange(r automask.Result, start, end float64) bool {
	return math.Abs(r.Start-start) < rangeTolerance && math.Abs(r.Start+r.Duration-end) < rangeTolerance
}

// 시작은 0, 끝은 영상 길이로 자른 뒤 순서와 최소 길이를 검사한다.
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
