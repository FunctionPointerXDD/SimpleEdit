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
}

const (
	detectPerSecond = 30
	maxGapSeconds   = 0.5
	holdSeconds     = 0.3
)

// 초 단위 상수에 fps를 곱해 프레임 수로 바꾼다. 최소 1프레임
func OptionsFor(fps float64) Options {
	frames := func(sec float64) int { return max(1, int(math.Round(sec*fps))) }
	t := DefaultTrackOptions
	t.MaxGap = frames(maxGapSeconds)
	t.Hold = frames(holdSeconds)
	return Options{
		Step:       max(1, int(math.Round(fps/detectPerSecond))),
		DetectSize: 640,
		Track:      t,
	}
}

type Result struct {
	Start    float64      `json:"start"`
	Duration float64      `json:"duration"`
	FPS      float64      `json:"fps"`
	Frames   [][]TrackBox `json:"frames"`
	Found    int          `json:"found"`
	Stats    TrackStats   `json:"-"`
}

// Collect로 검출을 모은 뒤 Track으로 잇고 거른다.
func Analyze(ctx context.Context, s media.PipeSpec, det Detector, o Options, onProgress media.ProgressFunc) (Result, error) {
	samples, frames, found, err := Collect(ctx, s, det, o, onProgress)
	if err != nil {
		return Result{}, err
	}
	boxes, stats := Track(samples, frames, o.Track)
	return Result{
		Start: s.Start, Duration: s.Duration, FPS: s.FPS,
		Frames: boxes, Found: found, Stats: stats,
	}, nil
}

// DetectSize로 줄여 디코딩하고 Step 프레임마다 검출해 비율 좌표로 모은다.
func Collect(ctx context.Context, s media.PipeSpec, det Detector, o Options, onProgress media.ProgressFunc) (samples []Sample, frames, found int, err error) {
	ds := s
	ds.Width, ds.Height = detectSize(s.Width, s.Height, o.DetectSize)
	total := s.FrameCount()
	err = media.DecodeFrames(ctx, ds, func(n int, f frame.Frame) error {
		frames = n + 1
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
		found += len(boxes)
		samples = append(samples, Sample{Frame: n, Dets: normalize(boxes, f.W, f.H)})
		return nil
	})
	if err == nil && onProgress != nil {
		onProgress(1)
	}
	return samples, frames, found, err
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

// 프레임마다 수동 사각형과 n번째 자동 박스를 차례로 모자이크한다.
func PixelateFunc(manual []media.Rect, auto [][]media.Rect) media.FrameFunc {
	return func(n int, f frame.Frame) error {
		for _, r := range manual {
			pixelate(f, r)
		}
		if n < len(auto) {
			for _, r := range auto[n] {
				pixelate(f, r)
			}
		}
		return nil
	}
}

const (
	maxCells     = 8
	minBlockSize = 16
)

// 비율 좌표를 바깥쪽으로 반올림해 픽셀로 바꾼 뒤 blockSize 칸으로 모자이크
func pixelate(f frame.Frame, r media.Rect) {
	px := image.Rect(
		int(math.Floor(r.X*float64(f.W))), int(math.Floor(r.Y*float64(f.H))),
		int(math.Ceil((r.X+r.W)*float64(f.W))), int(math.Ceil((r.Y+r.H)*float64(f.H))),
	)
	f.Pixelate(px, blockSize(px))
}

// 긴 변이 maxCells칸 이하가 되게 칸을 키우되 minBlockSize보다 작지 않게
func blockSize(r image.Rectangle) int {
	return max(minBlockSize, (max(r.Dx(), r.Dy())+maxCells-1)/maxCells)
}
