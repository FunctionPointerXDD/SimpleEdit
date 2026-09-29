package media

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"simpleEdit/internal/frame"
	"simpleEdit/internal/tools"
)

// useTools는 ffmpeg가 있는 bin 폴더를 쓰게 하고, 없으면 건너뛴다.
func useTools(t *testing.T) {
	t.Helper()
	dir := os.Getenv(tools.EnvBinDir)
	if dir == "" {
		dir, _ = filepath.Abs(filepath.Join("..", "..", "tools", "linux"))
	}
	if _, err := os.Stat(filepath.Join(dir, "ffmpeg")); err != nil {
		if _, err := os.Stat(filepath.Join(dir, "ffmpeg.exe")); err != nil {
			t.Skipf("ffmpeg 없음: %s", dir)
		}
	}
	t.Setenv(tools.EnvBinDir, dir)
}

// makeSource는 잡음 없는 움직이는 테스트 영상을 만든다.
func makeSource(t *testing.T, w, h int, seconds float64) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "src.mp4")
	args := []string{"-v", "error", "-y",
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc=size=%dx%d:rate=30:duration=%g", w, h, seconds),
		"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:duration=%g", seconds),
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", p}
	if out, err := exec.Command(tools.Path(tools.FFmpeg), args...).CombinedOutput(); err != nil {
		t.Fatalf("테스트 영상 생성 실패: %v\n%s", err, out)
	}
	return p
}

func probeSpec(t *testing.T, path string, start, dur float64) PipeSpec {
	t.Helper()
	info, err := Probe(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return PipeSpec{Source: path, Start: start, Duration: dur, FPS: info.FPS, Width: info.Width, Height: info.Height}
}

// grabFrame은 n번째 프레임을 복사해 돌려준다.
func grabFrame(t *testing.T, s PipeSpec, n int) frame.Frame {
	t.Helper()
	var got frame.Frame
	err := DecodeFrames(context.Background(), s, func(i int, f frame.Frame) error {
		if i == n {
			got = frame.Frame{W: f.W, H: f.H, Pix: append([]byte(nil), f.Pix...)}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Pix == nil {
		t.Fatalf("%d번 프레임 없음", n)
	}
	return got
}

// changedBounds는 8×8 칸 평균 차가 큰 영역의 경계다.
func changedBounds(a, b frame.Frame, threshold float64) image.Rectangle {
	const cell = 8
	var r image.Rectangle
	for cy := 0; cy+cell <= a.H; cy += cell {
		for cx := 0; cx+cell <= a.W; cx += cell {
			sum := 0
			for y := cy; y < cy+cell; y++ {
				for x := cx; x < cx+cell; x++ {
					o := (y*a.W + x) * 3
					for c := 0; c < 3; c++ {
						sum += abs(int(a.Pix[o+c]) - int(b.Pix[o+c]))
					}
				}
			}
			if float64(sum)/(cell*cell*3) > threshold {
				r = r.Union(image.Rect(cx, cy, cx+cell, cy+cell))
			}
		}
	}
	return r
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func TestProcessFramesLengthAndAudio(t *testing.T) {
	useTools(t)
	src := makeSource(t, 320, 240, 4)
	s := probeSpec(t, src, 1, 2)
	out := filepath.Join(t.TempDir(), "out.mp4")
	var frames, lastPct float64
	err := ProcessFrames(context.Background(), s, out, func(int, frame.Frame) error {
		frames++
		return nil
	}, func(f float64) { lastPct = f })
	if err != nil {
		t.Fatal(err)
	}
	info, err := Probe(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if d := info.Duration - 2; d < -0.2 || d > 0.2 {
		t.Errorf("길이 %.3f초, want 2±0.2", info.Duration)
	}
	if frames != 60 || lastPct != 1 {
		t.Errorf("프레임 %v개, 마지막 진행률 %v", frames, lastPct)
	}
	if !info.HasAudio || info.VideoCodec != "h264" || info.PixFmt != "yuv420p" {
		t.Errorf("출력 형식: %+v", info)
	}
}

func TestPipeMosaicMatchesFilterPosition(t *testing.T) {
	useTools(t)
	src := makeSource(t, 320, 240, 3)
	s := probeSpec(t, src, 0.5, 1.5)
	rect := PixelRect{X: 64, Y: 48, W: 128, H: 96}
	dir := t.TempDir()
	pipeOut := filepath.Join(dir, "pipe.mp4")
	filterOut := filepath.Join(dir, "filter.mp4")
	if err := ProcessFrames(context.Background(), s, pipeOut, PixelateRects([]PixelRect{rect}), nil); err != nil {
		t.Fatal(err)
	}
	if err := Export(context.Background(), ExportSpec{Source: src, Output: filterOut, Start: 0.5, Duration: 1.5, Rects: []PixelRect{rect}}, nil); err != nil {
		t.Fatal(err)
	}
	want := image.Rect(rect.X, rect.Y, rect.X+rect.W, rect.Y+rect.H)
	orig := grabFrame(t, s, 20)
	for name, path := range map[string]string{"pipe": pipeOut, "filter": filterOut} {
		got := grabFrame(t, PipeSpec{Source: path, Duration: 1.5, FPS: s.FPS, Width: s.Width, Height: s.Height}, 20)
		b := changedBounds(orig, got, 12)
		if b.Empty() || !b.In(want.Inset(-8)) {
			t.Errorf("%s: 바뀐 영역 %v, 사각형 %v 밖으로 나감", name, b, want)
		}
		t.Logf("%s: 바뀐 영역 %v (사각형 %v)", name, b, want)
	}
}

func TestProcessFramesCleansUpOnError(t *testing.T) {
	useTools(t)
	src := makeSource(t, 160, 120, 3)
	s := probeSpec(t, src, 0, 3)
	dir := t.TempDir()

	boom := errors.New("처리 실패")
	out := filepath.Join(dir, "fail.mp4")
	err := ProcessFrames(context.Background(), s, out, func(n int, _ frame.Frame) error {
		if n == 10 {
			return boom
		}
		return nil
	}, nil)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want 처리 실패", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Error("실패 후 출력 파일이 남음")
	}

	ctx, cancel := context.WithCancel(context.Background())
	out = filepath.Join(dir, "cancel.mp4")
	err = ProcessFrames(ctx, s, out, func(n int, _ frame.Frame) error {
		if n == 5 {
			cancel()
		}
		return nil
	}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want Canceled", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Error("취소 후 출력 파일이 남음")
	}
}

func TestProcessFramesOddSize(t *testing.T) {
	useTools(t)
	src := makeSource(t, 322, 242, 1)
	s := probeSpec(t, src, 0, 1)
	s.Width, s.Height = 321, 241
	out := filepath.Join(t.TempDir(), "odd.mp4")
	if err := ProcessFrames(context.Background(), s, out, func(int, frame.Frame) error { return nil }, nil); err != nil {
		t.Fatal(err)
	}
	info, err := Probe(context.Background(), out)
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 320 || info.Height != 240 {
		t.Errorf("크기 %dx%d, want 320x240", info.Width, info.Height)
	}
}

func TestMuteRemovesAudio(t *testing.T) {
	useTools(t)
	src := makeSource(t, 160, 120, 3)
	s := probeSpec(t, src, 0.5, 2)
	s.Mute = true
	dir := t.TempDir()
	pipeOut := filepath.Join(dir, "pipe.mp4")
	filterOut := filepath.Join(dir, "filter.mp4")
	if err := ProcessFrames(context.Background(), s, pipeOut, func(int, frame.Frame) error { return nil }, nil); err != nil {
		t.Fatal(err)
	}
	if err := Export(context.Background(), ExportSpec{Source: src, Output: filterOut, Start: 0.5, Duration: 2, Mute: true}, nil); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{pipeOut, filterOut} {
		info, err := Probe(context.Background(), p)
		if err != nil {
			t.Fatal(err)
		}
		if info.HasAudio {
			t.Errorf("%s: 소리 트랙이 남음", filepath.Base(p))
		}
		if d := info.Duration - 2; d < -0.2 || d > 0.2 {
			t.Errorf("%s: 길이 %.3f초", filepath.Base(p), info.Duration)
		}
	}
}
