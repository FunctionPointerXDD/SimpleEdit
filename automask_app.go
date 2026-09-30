// AutoMask.Frames 박스는 [x, y, w, h, 트랙 ID], Tracks는 거르고 남긴 트랙 수다.
// AutoMask는 요청 구간이 아니라 쌓인 분석 범위 전체(Start~Start+Duration)를 준다.
// AnalyzeRequest.Fresh가 false면 이미 분석한 범위는 다시 쓰고 빠진 앞·뒤만 분석한다.

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

var errStaleAuto = errors.New("자동 분석이 저장 구간을 덮지 않음")

type AnalyzeRequest struct {
	Kind  string  `json:"kind"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Fresh bool    `json:"fresh"`
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
func toAutoMask(kind string, c *automask.Coverage) AutoMask {
	round := func(v float64) float32 { return float32(math.Round(v*1e4) / 1e4) }
	frames := make([][][5]float32, len(c.Frames))
	for n, boxes := range c.Frames {
		frames[n] = make([][5]float32, len(boxes))
		for i, b := range boxes {
			frames[n][i] = [5]float32{round(b.Rect.X), round(b.Rect.Y), round(b.Rect.W), round(b.Rect.H), float32(b.ID)}
		}
	}
	return AutoMask{
		Kind: kind, Start: float64(c.First) / c.FPS, Duration: float64(len(c.Frames)) / c.FPS, FPS: c.FPS,
		Found: c.Found, Tracks: c.Kept, Frames: frames,
	}
}

// 빠진 조각만 차례로 분석해 기존 범위에 붙인다. 빠진 곳이 없으면 바로 준다. 그사이 영상이 바뀌면 버린다.
func (a *App) Analyze(req AnalyzeRequest) (AutoMask, error) {
	if req.Kind != kindFace {
		return AutoMask{}, userError(msgAnalyzeFail, fmt.Errorf("지원하지 않는 종류: %s", req.Kind))
	}
	a.mu.Lock()
	source, info, prev := a.source, a.info, a.auto[req.Kind]
	a.mu.Unlock()
	if source == "" {
		return AutoMask{}, userError(msgNoVideo, nil)
	}
	start, end, err := clipRange(info, req.Start, req.End)
	if err != nil {
		return AutoMask{}, err
	}
	cov := &automask.Coverage{FPS: info.FPS}
	if prev != nil && !req.Fresh {
		c := *prev
		cov = &c
	}
	spans, restart := cov.Missing(automask.FrameSpan{First: automask.FrameAt(start, info.FPS), End: automask.FrameAt(end, info.FPS)})
	if restart {
		cov = &automask.Coverage{FPS: info.FPS}
	}
	if len(spans) == 0 {
		return toAutoMask(req.Kind, cov), nil
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
	total, finished := 0, 0
	for _, sp := range spans {
		total += sp.End - sp.First
	}
	for _, sp := range spans {
		s, e := float64(sp.First)/info.FPS, float64(sp.End)/info.FPS
		applog.Printf("얼굴 분석: %.2f~%.2f초, %.3ffps", s, e, info.FPS)
		n := sp.End - sp.First
		res, err := automask.Analyze(ctx, pipeSpec(source, info, s, e), det, automask.OptionsFor(info.FPS), func(f float64) {
			emit((float64(finished)+f*float64(n))/float64(total), "analyze")
		})
		if err != nil {
			return AutoMask{}, analyzeError(err)
		}
		applog.Printf("얼굴 분석 완료: 프레임 %d개, 검출 %d건, 트랙 %d개 남김·%d개 버림",
			len(res.Frames), res.Found, res.Stats.Kept, res.Stats.Dropped)
		cov.Merge(sp, res)
		finished += n
	}
	a.mu.Lock()
	if a.source == source {
		a.auto[req.Kind] = cov
	}
	a.mu.Unlock()
	return toAutoMask(req.Kind, cov), nil
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

// 저장 구간의 프레임을 쌓인 범위에서 꺼내 뺀 트랙을 제외해 합친다. 박스가 없으면 nil
func (a *App) autoFrames(kinds []string, exclude map[string][]int, start, end float64) ([][]media.Rect, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	count := pipeSpec(a.source, a.info, start, end).FrameCount() + 1
	merged := make([][]media.Rect, count)
	boxes := 0
	for _, k := range kinds {
		cov := a.auto[k]
		win, ok := cov.Window(automask.FrameAt(start, a.info.FPS), count)
		if !ok {
			return nil, errStaleAuto
		}
		skip := map[int]bool{}
		for _, id := range exclude[k] {
			skip[id] = true
		}
		for n, rs := range automask.Rects(win, skip) {
			merged[n] = append(merged[n], rs...)
			boxes += len(rs)
		}
	}
	if boxes == 0 {
		return nil, nil
	}
	return merged, nil
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
