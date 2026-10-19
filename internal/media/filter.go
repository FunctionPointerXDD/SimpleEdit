// Rect는 레터박스를 뺀 영상 영역 기준 0~1 좌표, PixelRect는 원본 px(짝수)다.
// MaskRect.Start·End는 직접 가림을 켤 원본 기준 초다(화면이 자른 구간 전체로 채워 보냄).
// MaskRect.Track이 0이 아니면 App.Track이 돌려준 추적 번호이고, 상자는 프레임마다 그 경로를 따른다.
// 블러 세기는 저장(Go 프레임)과 미리보기가 같은 BlurSigma를 쓴다. 자동 얼굴·직접 가림 모두 긴 변의 8%.
// 기록: 사각형 20%·얼굴 20% → 얼굴 14%(너무 진함) → 얼굴 8%(09-30, 방송 블랙박스 5~7% 참고)
// → 사각형도 8%(10-01 사용자 요청). 작은 박스도 minBlurSigma를 지킨다.
// 가림이 있으면 저장은 모두 Go 파이프(frame.Blur)로 한다. ffmpeg 필터 블러는 미리보기·저장
// 블러를 한 곳(frame)에서 맞추려고 10-01에 뺐다. ExportSpec은 가림 없는 저장(자르기·소리)만 다룬다.
// ExportSpec.Start·Duration은 초 단위다.

package media

import (
	"math"
	"strconv"
)

const MaxRects = 10

const (
	blurShare    = 0.08
	minBlurSigma = 4
)

// max(minBlurSigma, 긴 변 × blurShare)를 정수 px로 반올림한다.
func BlurSigma(w, h int) int {
	return max(minBlurSigma, int(math.Round(float64(max(w, h))*blurShare)))
}

type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type MaskRect struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Track int     `json:"track"`
}

func (r MaskRect) Box() Rect { return Rect{X: r.X, Y: r.Y, W: r.W, H: r.H} }

type PixelRect struct {
	X, Y, W, H int
}

const minRectPx = 4

// 0~1로 자른 뒤 px로 바꿔 위치는 짝수 내림, 크기는 짝수 올림. minRectPx 미만은 false
func ToPixels(r Rect, width, height int) (PixelRect, bool) {
	x0 := clamp(r.X, 0, 1) * float64(width)
	y0 := clamp(r.Y, 0, 1) * float64(height)
	x1 := clamp(r.X+r.W, 0, 1) * float64(width)
	y1 := clamp(r.Y+r.H, 0, 1) * float64(height)

	x := evenFloor(x0)
	y := evenFloor(y0)
	w := evenCeil(x1 - float64(x))
	h := evenCeil(y1 - float64(y))
	if x+w > width {
		w = (width - x) &^ 1
	}
	if y+h > height {
		h = (height - y) &^ 1
	}
	if w < minRectPx || h < minRectPx {
		return PixelRect{}, false
	}
	return PixelRect{X: x, Y: y, W: w, H: h}, true
}

func RectsToPixels(rs []Rect, width, height int) []PixelRect {
	out := make([]PixelRect, 0, len(rs))
	for _, r := range rs {
		if p, ok := ToPixels(r, width, height); ok {
			out = append(out, p)
		}
	}
	return out
}

type ExportSpec struct {
	Source   string
	Output   string
	Start    float64
	Duration float64
	Mute     bool
}

// 구간을 잘라 H.264 yuv420p로 인코딩한다.
func ExportArgs(s ExportSpec) []string {
	args := []string{
		"-hide_banner", "-y",
		"-ss", secs(s.Start),
		"-i", s.Source,
		"-t", secs(s.Duration),
	}
	args = append(args, "-map", "0:v:0")
	args = append(args, audioArgs(s.Mute, 0)...)
	args = append(args,
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "20",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		"-progress", "pipe:1", "-nostats",
		s.Output,
	)
	return args
}

// 높이 720 이하로 줄여 ultrafast H.264 + AAC로 만든다.
func PreviewArgs(source, output string) []string {
	return []string{
		"-hide_banner", "-y",
		"-i", source,
		"-map", "0:v:0", "-map", "0:a:0?",
		"-vf", "scale=-2:'min(720,ih)'",
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "26",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "128k",
		"-movflags", "+faststart",
		"-progress", "pipe:1", "-nostats",
		output,
	}
}

// mute면 -an, 아니면 input번 입력의 첫 소리(없어도 됨)를 AAC로 넣는다.
func audioArgs(mute bool, input int) []string {
	if mute {
		return []string{"-an"}
	}
	return []string{"-map", strconv.Itoa(input) + ":a:0?", "-c:a", "aac", "-b:a", "160k"}
}

func secs(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) }

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func evenFloor(v float64) int { return int(math.Floor(v)) &^ 1 }

func evenCeil(v float64) int {
	n := int(math.Ceil(v - 1e-9))
	return (n + 1) &^ 1
}
