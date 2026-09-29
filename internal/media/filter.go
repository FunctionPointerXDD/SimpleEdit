package media

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// MaxRects는 한 번에 가릴 수 있는 사각형 개수다.
const MaxRects = 5

// mosaicBlock은 모자이크 한 칸의 크기(원본 픽셀)다.
const mosaicBlock = 16

// Rect는 레터박스를 뺀 영상 영역 기준 0~1 좌표다.
type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// PixelRect는 원본 해상도 기준 정수 좌표다. 폭·높이는 짝수.
type PixelRect struct {
	X, Y, W, H int
}

// minRectPx보다 작은 사각형은 실수로 클릭한 것으로 보고 버린다.
const minRectPx = 4

// ToPixels는 원본 좌표로 바꾼다. 쓸 수 없으면 ok=false.
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

// RectsToPixels는 여러 사각형을 변환하고, 쓸 수 없는 것은 뺀다.
func RectsToPixels(rs []Rect, width, height int) []PixelRect {
	out := make([]PixelRect, 0, len(rs))
	for _, r := range rs {
		if p, ok := ToPixels(r, width, height); ok {
			out = append(out, p)
		}
	}
	return out
}

// MosaicFilter는 모자이크 필터를 만든다. 출력은 [v].
func MosaicFilter(rects []PixelRect) string {
	if len(rects) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("[0:v]split=" + strconv.Itoa(len(rects)+1) + "[base]")
	for i := range rects {
		fmt.Fprintf(&b, "[m%d]", i)
	}
	for i, r := range rects {
		sw := max(1, r.W/mosaicBlock)
		sh := max(1, r.H/mosaicBlock)
		fmt.Fprintf(&b, ";[m%d]crop=%d:%d:%d:%d,scale=%d:%d,scale=%d:%d:flags=neighbor[p%d]",
			i, r.W, r.H, r.X, r.Y, sw, sh, r.W, r.H, i)
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

// ExportSpec은 저장 명령을 만드는 데 필요한 값이다.
type ExportSpec struct {
	Source   string
	Output   string
	Start    float64 // 초
	Duration float64 // 초
	Rects    []PixelRect
	Mute     bool
}

// ExportArgs는 저장용 ffmpeg 인자. yuv420p로 호환성 확보
func ExportArgs(s ExportSpec) []string {
	args := []string{
		"-hide_banner", "-y",
		"-ss", secs(s.Start),
		"-i", s.Source,
		"-t", secs(s.Duration),
	}
	if f := MosaicFilter(s.Rects); f != "" {
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

// PreviewArgs는 720p H.264 미리보기용 인자다.
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

// audioArgs는 input번 입력의 소리를 AAC로 넣거나 뺀다.
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
