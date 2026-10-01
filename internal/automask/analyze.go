// Options.Step은 몇 프레임마다 검출할지, DetectSize는 검출용 긴 변(px)이다.
// Result.Found는 거르기 전 검출 수, Stats.Kept는 남긴 트랙 수다.

package automask

import (
	"context"
	"image"
	"math"

	"simpleEdit/internal/detect"
	"simpleEdit/internal/frame"
	"simpleEdit/internal/media"
)

type Detector interface {
	Detect(f frame.Frame) ([]detect.Box, error)
}

type Options struct {
	Step       int
	DetectSize int
	Track      TrackOptions
	Scene      SceneOptions
}

const (
	detectPerSecond = 30
	maxGapSeconds   = 0.5
	holdSeconds     = 0.3
	smoothPosSec    = 0.10
	smoothSizeSec   = 0.20
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
	err := media.DecodeFrames(ctx, ds, func(n int, f frame.Frame) error {
		c.Frames = n + 1
		scene.add(f)
		if onProgress != nil {
			onProgress(min(0.99, float64(n+1)/float64(total)))
		}
		if n%max(1, o.Step) != 0 {
			return nil
		}
		boxes, err := det.Detect(f)
		if err != nil {
			return err
		}
		c.Found += len(boxes)
		c.Samples = append(c.Samples, Sample{Frame: n, Dets: normalize(boxes, f.W, f.H)})
		return nil
	})
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
}

// n번째 프레임에 [First, End) 안인 수동 사각형은 사각형으로, 자동 박스는 타원으로 흐린다.
func BlurFunc(manual []ManualRect, auto [][]media.Rect) media.FrameFunc {
	return func(n int, f frame.Frame) error {
		for _, m := range manual {
			if n >= m.First && n < m.End {
				blur(f, m.Rect, false)
			}
		}
		if n < len(auto) {
			for _, r := range auto[n] {
				blur(f, r, true)
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
