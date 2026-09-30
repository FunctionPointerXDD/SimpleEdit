// Rect는 레터박스를 뺀 영상 영역 기준 0~1 좌표, PixelRect는 원본 px(짝수)다.
// 블러 세기는 저장 방식(ffmpeg 필터·Go 프레임)과 미리보기가 같은 규칙을 쓴다. 사각형은 BlurSigma,
// 자동 얼굴 타원은 FaceBlurSigma다.
// 긴 변의 20%면 얼굴 박스 안이 뭉개져 알아볼 수 없고, 작은 박스도 minBlurSigma를 지킨다.
// 자동 얼굴(타원)은 FaceBlurSigma로 8%다. 20% → 14%(너무 진함) → 8%(사용자 판단, 2026-09-30).
// 8%는 방송 블랙박스 영상(hx_4EDQ3ky4)의 얼굴 블러 5~7%보다 조금 진하게 잡은 값이다.
// 필터도 frame.Blur처럼 시그마가 blurCell 안팎이 되게 줄여 흐린 뒤 키운다(속도 2배 차).
// 줄인 작은 그림에서 gblur는 가장자리가 깨져(초록·검은 줄) frame.Blur와 같은 boxblur 3회를 쓴다.
// 작은 yuv420p는 색 평면이 너무 작아 흐릴 때만 gbrp로 바꾼다.
// boxblur는 2×반지름+1이 짧은 변을 넘으면 값이 틀어져, 짧은 변은 minBlurSide 이상으로 줄이고
// 반지름은 (짧은 변-1)/2 이하로 둔다.
// ExportSpec.Start·Duration은 초 단위다.

package media

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const MaxRects = 5

const (
	blurShare     = 0.2
	faceBlurShare = 0.08
	minBlurSigma  = 4
	blurCell      = 4
	minBlurSide   = 12
)

// max(minBlurSigma, 긴 변 × blurShare)를 정수 px로 반올림한다.
func BlurSigma(w, h int) int {
	return max(minBlurSigma, int(math.Round(float64(max(w, h))*blurShare)))
}

// BlurSigma와 같되 비율이 faceBlurShare다.
func FaceBlurSigma(w, h int) int {
	return max(minBlurSigma, int(math.Round(float64(max(w, h))*faceBlurShare)))
}

type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

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

// 사각형마다 crop→축소→상자 평균 3회→확대 후 차례로 overlay한다. 출력은 [v]
func BlurFilter(rects []PixelRect) string {
	if len(rects) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("[0:v]split=" + strconv.Itoa(len(rects)+1) + "[base]")
	for i := range rects {
		fmt.Fprintf(&b, "[m%d]", i)
	}
	for i, r := range rects {
		sigma := BlurSigma(r.W, r.H)
		k := max(1, min(sigma/blurCell, min(r.W, r.H)/minBlurSide))
		sw, sh := evenUp((r.W+k-1)/k), evenUp((r.H+k-1)/k)
		rad := max(1, min((sigma+k/2)/k, (min(sw, sh)-1)/2))
		fmt.Fprintf(&b, ";[m%d]crop=%d:%d:%d:%d,scale=%d:%d:flags=area,format=gbrp,boxblur=%d:3,format=yuv420p,scale=%d:%d:flags=bilinear[p%d]",
			i, r.W, r.H, r.X, r.Y, sw, sh, rad, r.W, r.H, i)
	}
	prev := "base"
	for i, r := range rects {
		out := "o" + strconv.Itoa(i)
		if i == len(rects)-1 {
			out = "v"
		}
		fmt.Fprintf(&b, ";[%s][p%d]overlay=%d:%d[%s]", prev, i, r.X, r.Y, out)
		prev = out
	}
	return b.String()
}

type ExportSpec struct {
	Source   string
	Output   string
	Start    float64
	Duration float64
	Rects    []PixelRect
	Mute     bool
}

// 사각형이 있으면 filter_complex로 가리고, H.264 yuv420p로 인코딩한다.
func ExportArgs(s ExportSpec) []string {
	args := []string{
		"-hide_banner", "-y",
		"-ss", secs(s.Start),
		"-i", s.Source,
		"-t", secs(s.Duration),
	}
	if f := BlurFilter(s.Rects); f != "" {
		args = append(args, "-filter_complex", f, "-map", "[v]")
	} else {
		args = append(args, "-map", "0:v:0")
	}
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

func evenUp(v int) int { return max(2, (v+1)&^1) }
