// Options.Step은 몇 프레임마다 검출할지, DetectSize는 검출용 긴 변(px)이다.
// Result.Found는 거르기 전 검출 수, Stats.Kept는 남긴 트랙 수다.
// 번호판(PlateOptionsFor)은 YOLOX가 얼굴 모델보다 훨씬 무거워(INT8로도 960px 한 장 41ms, YuNet 2.4ms)
// 초당 10번, 긴 변 960px로 검출한다.
// 640px은 확실한 번호판(0.7 이상)을 14장면 중 4개만 잡았고 960px은 1280px과 같은 11개였다(10-05).
// 검출은 0.3까지 받아 트랙을 잇고, 트랙이 남으려면 최고 점수가 크기와 상관없이 0.7 이상이어야 한다.
// 짧은 트랙(검출 3회 미만, 약 0.3초)은 0.8이 필요하다. 번호판 상자는 꼭 맞아 30%만 키우고 올리지 않는다.
// ManualRect.Path는 "움직임 따라가기" 상자(First부터 프레임마다)이고, 추적 상자는 번호판에 꼭 맞게
// 나올 때가 많아 followExpand(20%)만큼 넓혀 흐린다(화면 state.js TRACK_EXPAND와 같음).
// AutoLayer는 종류별 프레임 박스이고, Ellipse면 내접 타원(얼굴), 아니면 사각형(번호판)으로 흐린다.
// 검출기가 Prepare·Infer를 지원하면(pipelined) 다음 프레임 입력 변환을 이번 프레임 추론과 겹친다.
// 계산 고루틴 하나가 Infer를 차례로 부르고, 넘기는 채널은 버퍼가 없어 아직 계산 중인 슬롯을
// 덮어쓰지 않는다(job k를 넘길 수 있으면 같은 슬롯을 쓴 k-2는 이미 끝남). 결과와 순서는 같다.
// 2026-10-02 측정: 변환 1.0ms가 추론 2.4ms 앞에 차례로 붙어 검출이 그만큼 느렸다.

package automask

import (
	"context"
	"errors"
	"image"
	"math"
	"sync/atomic"

	"simpleEdit/internal/detect"
	"simpleEdit/internal/frame"
	"simpleEdit/internal/media"
)

type Detector interface {
	Detect(f frame.Frame) ([]detect.Box, error)
}

type pipelined interface {
	Prepare(slot int, f frame.Frame) (detect.Prepared, error)
	Infer(p detect.Prepared) ([]detect.Box, error)
}

type Options struct {
	Step       int
	DetectSize int
	Track      TrackOptions
	Scene      SceneOptions
}

const (
	detectPerSecond      = 30
	plateDetectPerSecond = 10
	plateDetectSize      = 960
	maxGapSeconds        = 0.5
	holdSeconds          = 0.3
	smoothPosSec         = 0.10
	smoothSizeSec        = 0.20
)

// 초 단위 상수에 fps를 곱해 프레임 수로 바꾼다. 최소 1프레임
func OptionsFor(fps float64) Options {
	frames := func(sec float64) int { return max(1, int(math.Round(sec*fps))) }
	t := DefaultTrackOptions
	t.MaxGap = frames(maxGapSeconds)
	t.Hold = frames(holdSeconds)
	t.SmoothPos, t.SmoothSize = smoothPosSec*fps, smoothSizeSec*fps
	return Options{
		Step:       max(1, int(math.Round(fps/detectPerSecond))),
		DetectSize: 640,
		Track:      t,
		Scene:      DefaultSceneOptions,
	}
}

// OptionsFor를 번호판용 검출 간격·크기·트랙 기준으로 바꾼다.
func PlateOptionsFor(fps float64) Options {
	o := OptionsFor(fps)
	o.Step = max(1, int(math.Round(fps/plateDetectPerSecond)))
	o.DetectSize = plateDetectSize
	o.Track.Strong, o.Track.Weak = 0.7, 0.7
	o.Track.ShortDetections, o.Track.ShortScore = 3, 0.8
	o.Track.Expand, o.Track.Lift = 0.3, 0
	return o
}

type Result struct {
	Frames [][]TrackBox
	Found  int
	Stats  TrackStats
}

type Collection struct {
	Samples []Sample
	Cuts    []int
	Frames  int
	Found   int
}

// Collect로 검출과 컷을 모은 뒤 Track으로 잇고 거른다.
func Analyze(ctx context.Context, s media.PipeSpec, det Detector, o Options, onProgress media.ProgressFunc) (Result, error) {
	c, err := Collect(ctx, s, det, o, onProgress)
	if err != nil {
		return Result{}, err
	}
	boxes, stats := Track(c.Samples, c.Cuts, c.Frames, o.Track)
	return Result{Frames: boxes, Found: c.Found, Stats: stats}, nil
}

// DetectSize로 줄여 디코딩하고, 프레임마다 컷 차이를 재고 Step 프레임마다 검출해 모은다.
func Collect(ctx context.Context, s media.PipeSpec, det Detector, o Options, onProgress media.ProgressFunc) (Collection, error) {
	ds := s
	ds.Width, ds.Height = detectSize(s.Width, s.Height, o.DetectSize)
	total := s.FrameCount()
	var c Collection
	var scene sceneDiffs
	q := newDetectQueue(det, &c)
	err := media.DecodeFrames(ctx, ds, func(n int, f frame.Frame) error {
		c.Frames = n + 1
		scene.add(f)
		if onProgress != nil {
			onProgress(min(0.99, float64(n+1)/float64(total)))
		}
		if n%max(1, o.Step) != 0 {
			return nil
		}
		return q.push(n, f)
	})
	if qerr := q.close(); qerr != nil {
		return Collection{}, qerr
	}
	if err != nil {
		return Collection{}, err
	}
	c.Cuts = o.Scene.cuts(scene.diffs)
	if onProgress != nil {
		onProgress(1)
	}
	return c, nil
}

// 긴 변이 limit를 넘으면 비율대로 줄이고, 두 변을 짝수로 내린다.
func detectSize(w, h, limit int) (int, int) {
	if limit <= 0 || max(w, h) <= limit {
		return w &^ 1, h &^ 1
	}
	k := float64(limit) / float64(max(w, h))
	return int(math.Round(float64(w)*k)) &^ 1, int(math.Round(float64(h)*k)) &^ 1
}

func normalize(boxes []detect.Box, w, h int) []Det {
	out := make([]Det, len(boxes))
	for i, b := range boxes {
		out[i] = Det{Score: b.Score, Box: media.Rect{
			X: float64(b.X) / float64(w), Y: float64(b.Y) / float64(h),
			W: float64(b.W) / float64(w), H: float64(b.H) / float64(h),
		}}
	}
	return out
}

type ManualRect struct {
	Rect       media.Rect
	First, End int
	Path       []media.Rect
}

const followExpand = 0.2

type AutoLayer struct {
	Frames  [][]media.Rect
	Ellipse bool
}

// [First, End) 안의 수동 사각형(Path가 있으면 그 프레임 상자를 넓혀)과 층별 자동 박스를 흐린다.
func BlurFunc(manual []ManualRect, auto []AutoLayer) media.FrameFunc {
	return func(n int, f frame.Frame) error {
		for _, m := range manual {
			if n < m.First || n >= m.End {
				continue
			}
			if m.Path != nil {
				blur(f, expand(m.Path[n-m.First], followExpand, 0), false)
			} else {
				blur(f, m.Rect, false)
			}
		}
		for _, l := range auto {
			if n < len(l.Frames) {
				for _, r := range l.Frames[n] {
					blur(f, r, l.Ellipse)
				}
			}
		}
		return nil
	}
}

// 비율 좌표를 픽셀로 바꿔 BlurSigma로 흐린다. ellipse면 내접 타원만
func blur(f frame.Frame, r media.Rect, ellipse bool) {
	px := image.Rect(
		int(math.Floor(r.X*float64(f.W))), int(math.Floor(r.Y*float64(f.H))),
		int(math.Ceil((r.X+r.W)*float64(f.W))), int(math.Ceil((r.Y+r.H)*float64(f.H))),
	)
	if ellipse {
		f.BlurEllipse(px, media.BlurSigma(px.Dx(), px.Dy()))
		return
	}
	f.Blur(px, media.BlurSigma(px.Dx(), px.Dy()))
}

var errDetectStopped = errors.New("검출 중단")

type detectJob struct {
	n    int
	p    detect.Prepared
	w, h int
}

type detectQueue struct {
	det    Detector
	pd     pipelined
	c      *Collection
	jobs   chan detectJob
	done   chan struct{}
	err    error
	failed atomic.Bool
	next   int
}

// pipelined면 계산 고루틴을 띄우고, 아니면 push가 바로 Detect한다.
func newDetectQueue(det Detector, c *Collection) *detectQueue {
	q := &detectQueue{det: det, c: c}
	if pd, ok := det.(pipelined); ok {
		q.pd, q.jobs, q.done = pd, make(chan detectJob), make(chan struct{})
		go q.run()
	}
	return q
}

// 다음 슬롯(0·1 번갈아)에 Prepare해 계산 고루틴에 넘긴다. 계산이 실패했으면 멈추게 한다.
func (q *detectQueue) push(n int, f frame.Frame) error {
	if q.pd == nil {
		boxes, err := q.det.Detect(f)
		if err != nil {
			return err
		}
		q.add(n, boxes, f.W, f.H)
		return nil
	}
	if q.failed.Load() {
		return errDetectStopped
	}
	p, err := q.pd.Prepare(q.next%2, f)
	if err != nil {
		return err
	}
	q.next++
	q.jobs <- detectJob{n: n, p: p, w: f.W, h: f.H}
	return nil
}

// 받은 순서대로 Infer한다. 실패한 뒤로는 받기만 해 push가 막히지 않게 한다.
func (q *detectQueue) run() {
	defer close(q.done)
	for j := range q.jobs {
		if q.err != nil {
			continue
		}
		boxes, err := q.pd.Infer(j.p)
		if err != nil {
			q.err = err
			q.failed.Store(true)
			continue
		}
		q.add(j.n, boxes, j.w, j.h)
	}
}

// 남은 계산을 기다리고 계산 오류를 돌려준다.
func (q *detectQueue) close() error {
	if q.pd == nil {
		return nil
	}
	close(q.jobs)
	<-q.done
	return q.err
}

func (q *detectQueue) add(n int, boxes []detect.Box, w, h int) {
	q.c.Found += len(boxes)
	q.c.Samples = append(q.c.Samples, Sample{Frame: n, Dets: normalize(boxes, w, h)})
}
