// AutoMask.Frames 박스는 [x, y, w, h, 트랙 ID], Tracks는 거르고 남긴 트랙 수다.
// AutoMask는 요청 구간이 아니라 쌓인 분석 범위 전체(Start~Start+Duration)를 준다.
// AnalyzeRequest.Fresh가 false면 이미 분석한 범위는 다시 쓰고 빠진 앞·뒤만 분석한다.
// Kind는 face(YuNet, 타원) 또는 plate(YOLOX, 사각형)다. 진행 단계 이름도 종류마다 다르다.
// 번호판 모델(bin\models\plate_yolox_tiny.onnx)은 CC BY 4.0 데이터로 학습해 출처를 앱과 함께 둔다.

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

const (
	kindFace  = "face"
	kindPlate = "plate"
)

var kindNames = map[string]string{kindFace: "얼굴", kindPlate: "번호판"}

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
	name, ok := kindNames[req.Kind]
	if !ok {
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
	det, opts, err := a.detector(req.Kind, info.FPS)
	if err != nil {
		return AutoMask{}, analyzeError(err)
	}
	stage := "analyze_" + req.Kind
	emit := a.progressEmitter(evAnalyzeProgress)
	emit(0, stage)
	total, finished := 0, 0
	for _, sp := range spans {
		total += sp.End - sp.First
	}
	for _, sp := range spans {
		s, e := float64(sp.First)/info.FPS, float64(sp.End)/info.FPS
		applog.Printf("%s 분석: %.2f~%.2f초, %.3ffps", name, s, e, info.FPS)
		n := sp.End - sp.First
		res, err := automask.Analyze(ctx, pipeSpec(source, info, s, e), det, opts, func(f float64) {
			emit((float64(finished)+f*float64(n))/float64(total), stage)
		})
		if err != nil {
			return AutoMask{}, analyzeError(err)
		}
		applog.Printf("%s 분석 완료: 프레임 %d개, 검출 %d건, 트랙 %d개 남김·%d개 버림",
			name, len(res.Frames), res.Found, res.Stats.Kept, res.Stats.Dropped)
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

// 종류에 맞는 검출기와 분석 설정. 검출기는 처음 한 번만 올리고 이후엔 같은 것을 쓴다.
func (a *App) detector(kind string, fps float64) (automask.Detector, automask.Options, error) {
	a.detMu.Lock()
	defer a.detMu.Unlock()
	if kind == kindPlate {
		if a.plateDet == nil {
			d, err := loadDetector(tools.PlateModel, detect.NewPlateDetector)
			if err != nil {
				return nil, automask.Options{}, err
			}
			a.plateDet = d
		}
		return a.plateDet, automask.PlateOptionsFor(fps), nil
	}
	if a.faceDet == nil {
		d, err := loadDetector(tools.FaceModel, detect.NewFaceDetector)
		if err != nil {
			return nil, automask.Options{}, err
		}
		a.faceDet = d
	}
	return a.faceDet, automask.OptionsFor(fps), nil
}

// 런타임·모델 파일이 있는지 보고 ONNX 환경을 올린 뒤 open으로 연다.
func loadDetector[T any](model string, open func(string) (T, error)) (T, error) {
	var zero T
	lib, path := tools.ORTLib(), tools.ModelPath(model)
	for _, p := range []string{lib, path} {
		if _, err := os.Stat(p); err != nil {
			return zero, fmt.Errorf("%w: %s", tools.ErrMissing, p)
		}
	}
	if err := detect.Init(lib); err != nil {
		return zero, err
	}
	return open(path)
}

// 열린 검출기를 모두 닫고, 하나라도 있었으면 ONNX 환경까지 해제한다.
func (a *App) closeDetectors() {
	a.detMu.Lock()
	defer a.detMu.Unlock()
	if a.faceDet == nil && a.plateDet == nil {
		return
	}
	if a.faceDet != nil {
		a.faceDet.Close()
		a.faceDet = nil
	}
	if a.plateDet != nil {
		a.plateDet.Close()
		a.plateDet = nil
	}
	if err := detect.Shutdown(); err != nil {
		applog.Printf("onnxruntime 정리 실패: %v", err)
	}
}

// 종류마다 저장 구간 프레임을 쌓인 범위에서 꺼내 뺀 트랙을 빼고 층으로 준다. 박스가 없으면 nil
func (a *App) autoFrames(kinds []string, exclude map[string][]int, start, end float64) ([]automask.AutoLayer, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	count := pipeSpec(a.source, a.info, start, end).FrameCount() + 1
	var layers []automask.AutoLayer
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
		frames := automask.Rects(win, skip)
		for _, rs := range frames {
			boxes += len(rs)
		}
		layers = append(layers, automask.AutoLayer{Frames: frames, Ellipse: k == kindFace})
	}
	if boxes == 0 {
		return nil, nil
	}
	return layers, nil
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
