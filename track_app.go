// Track은 3단계 직접 가림 하나를 그린 시각(Time)에서 자른 구간 [Start, End) 안으로 앞뒤 추적한다.
// 결과 경로는 App.paths에 번호로 두고(영상을 닫거나 바꾸면 비움), 화면에는 같은 번호와 상자들을 준다.
// TrackResult.Frames는 Start초부터 1/FPS 간격의 [x, y, w, h](0~1, 소수 4자리)다.
// LostStart·LostEnd는 앞·뒤에서 놓쳐 멈췄는지다(false면 자른 구간 끝까지 따라감).
// 저장할 때 MaskRect.Track으로 이 번호를 보내면 프레임마다 경로 상자로 가린다.

package main

import (
	"math"

	"simpleEdit/internal/applog"
	"simpleEdit/internal/automask"
	"simpleEdit/internal/detect"
	"simpleEdit/internal/media"
	"simpleEdit/internal/tools"
)

const evTrackProgress = "track:progress"

type TrackRequest struct {
	Time  float64    `json:"time"`
	Start float64    `json:"start"`
	End   float64    `json:"end"`
	Rect  media.Rect `json:"rect"`
}

type TrackResult struct {
	ID        int          `json:"id"`
	Start     float64      `json:"start"`
	FPS       float64      `json:"fps"`
	Frames    [][4]float32 `json:"frames"`
	LostStart bool         `json:"lostStart"`
	LostEnd   bool         `json:"lostEnd"`
}

// 상자를 원본 픽셀로 바꿔 Follow로 앞뒤 추적하고, 그사이 영상이 바뀌지 않았으면 경로를 저장한다.
func (a *App) Track(req TrackRequest) (TrackResult, error) {
	a.mu.Lock()
	source, info := a.source, a.info
	a.mu.Unlock()
	if source == "" {
		return TrackResult{}, userError(msgNoVideo, nil)
	}
	start, end, err := clipRange(info, req.Start, req.End)
	if err != nil {
		return TrackResult{}, err
	}
	w, h := float64(info.Width&^1), float64(info.Height&^1)
	box := detect.PixelBox{
		X: int(math.Round(req.Rect.X * w)), Y: int(math.Round(req.Rect.Y * h)),
		W: max(1, int(math.Round(req.Rect.W*w))), H: max(1, int(math.Round(req.Rect.H*h))),
	}

	ctx, done, err := a.jobs.Begin(a.ctx)
	if err != nil {
		return TrackResult{}, analyzeError(err)
	}
	defer done()
	tr, err := a.followTracker()
	if err != nil {
		return TrackResult{}, analyzeError(err)
	}
	emit := a.progressEmitter(evTrackProgress)
	emit(0, "track")
	at := automask.FrameAt(req.Time, info.FPS)
	applog.Printf("움직임 따라가기: %.2f초(프레임 %d), 구간 %.2f~%.2f초, 상자 %+v", req.Time, at, start, end, box)
	p, err := automask.Follow(ctx, pipeSpec(source, info, start, end), tr, at, box, func(f float64) { emit(f, "track") })
	if err != nil {
		return TrackResult{}, analyzeError(err)
	}
	applog.Printf("따라가기 끝: 프레임 %d부터 %d개, 앞 놓침 %v, 뒤 놓침 %v", p.First, len(p.Boxes), p.LostBefore, p.LostAfter)

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.source != source {
		return TrackResult{}, userError(msgNoVideo, nil)
	}
	a.nextPath++
	a.paths[a.nextPath] = &p
	return toTrackResult(a.nextPath, p, info.FPS), nil
}

// 추적기는 처음 한 번만 올리고 이후엔 같은 것을 쓴다.
func (a *App) followTracker() (*detect.Tracker, error) {
	a.detMu.Lock()
	defer a.detMu.Unlock()
	if a.tracker == nil {
		t, err := loadDetector(tools.TrackModel, detect.NewTracker)
		if err != nil {
			return nil, err
		}
		a.tracker = t
	}
	return a.tracker, nil
}

// 좌표를 소수 4자리로 줄이고 시작 프레임을 초로 바꾼다.
func toTrackResult(id int, p automask.Path, fps float64) TrackResult {
	round := func(v float64) float32 { return float32(math.Round(v*1e4) / 1e4) }
	frames := make([][4]float32, len(p.Boxes))
	for i, b := range p.Boxes {
		frames[i] = [4]float32{round(b.X), round(b.Y), round(b.W), round(b.H)}
	}
	return TrackResult{ID: id, Start: float64(p.First) / fps, FPS: fps, Frames: frames,
		LostStart: p.LostBefore, LostEnd: p.LostAfter}
}
