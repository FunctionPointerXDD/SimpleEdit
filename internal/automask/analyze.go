package automask

import (
	"context"
	"image"
	"math"

	"simpleEdit/internal/detect"
	"simpleEdit/internal/frame"
	"simpleEdit/internal/media"
)

// Detector는 프레임에서 박스를 찾는다. (FaceDetector 등)
type Detector interface {
	Detect(f frame.Frame) ([]detect.Box, error)
}

// Options는 분석 간격·크기와 추적 설정이다.
type Options struct {
	Step       int // 몇 프레임마다 검출할지
	DetectSize int // 검출용으로 줄일 긴 변 픽셀
	Track      TrackOptions
}

// 시간 기준 설정: 초당 검출 횟수, 이어 줄 공백, 앞뒤 유지
const (
	detectPerSecond = 30
	maxGapSeconds   = 0.5
	holdSeconds     = 0.3
)

// OptionsFor는 fps에 맞춰 프레임 단위 설정을 만든다.
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

// Result는 구간의 프레임별 자동 박스다. 좌표는 0~1 비율
type Result struct {
	Start    float64      `json:"start"`
	Duration float64      `json:"duration"`
	FPS      float64      `json:"fps"`
	Frames   [][]TrackBox `json:"frames"`
	Found    int          `json:"found"`
	Stats    TrackStats   `json:"-"`
}

// Analyze는 줄인 프레임에서 검출하고 추적 결과를 만든다.
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

// Collect는 검출만 모은다. 추적 설정 비교용으로도 쓴다.
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

// detectSize는 긴 변을 limit 이하로, 짝수로 줄인다.
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

// PixelateFunc는 수동 사각형과 n번째 자동 박스를 함께 가린다.
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

// 모자이크 한 변의 최대 칸 수와 최소 칸 크기
const (
	maxCells     = 8
	minBlockSize = 16
)

// pixelate는 비율 좌표를 픽셀로 바꿔 모자이크한다.
func pixelate(f frame.Frame, r media.Rect) {
	px := image.Rect(
		int(math.Floor(r.X*float64(f.W))), int(math.Floor(r.Y*float64(f.H))),
		int(math.Ceil((r.X+r.W)*float64(f.W))), int(math.Ceil((r.Y+r.H)*float64(f.H))),
	)
	f.Pixelate(px, blockSize(px))
}

// blockSize는 한 변이 maxCells칸 이하가 되게 칸을 키운다.
func blockSize(r image.Rectangle) int {
	return max(minBlockSize, (max(r.Dx(), r.Dy())+maxCells-1)/maxCells)
}
