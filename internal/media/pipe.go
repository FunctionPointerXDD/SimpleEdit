package media

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"strconv"

	"simpleEdit/internal/frame"
	"simpleEdit/internal/tools"
)

// PipeSpec은 프레임 단위 처리에 필요한 값이다.
type PipeSpec struct {
	Source   string
	Start    float64
	Duration float64
	FPS      float64
	Width    int
	Height   int
	Mute     bool
}

// FrameFunc는 n번째 프레임을 제자리에서 고친다.
type FrameFunc func(n int, f frame.Frame) error

// 디코더와 처리 사이에 돌려 쓰는 프레임 버퍼 수
const pipeBuffers = 3

// FrameCount는 구간의 예상 프레임 수다.
func (s PipeSpec) FrameCount() int {
	return max(1, int(s.Duration*s.FPS+0.5))
}

// evenSize는 yuv420p 인코딩용으로 짝수로 내린다.
func (s PipeSpec) evenSize() (int, int) {
	return max(2, s.Width&^1), max(2, s.Height&^1)
}

// DecodeArgs는 고정 fps bgr24 원시 프레임을 stdout으로 낸다.
func DecodeArgs(s PipeSpec) []string {
	w, h := s.evenSize()
	return []string{
		"-hide_banner", "-nostdin", "-loglevel", "error",
		"-ss", secs(s.Start),
		"-i", s.Source,
		"-t", secs(s.Duration),
		"-map", "0:v:0",
		"-vf", fmt.Sprintf("fps=%s,scale=%d:%d:flags=area", rate(s.FPS), w, h),
		"-f", "rawvideo", "-pix_fmt", "bgr24",
		"pipe:1",
	}
}

// EncodeArgs는 stdin 원시 프레임과 원본 소리로 MP4를 만든다.
func EncodeArgs(s PipeSpec, output string) []string {
	w, h := s.evenSize()
	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error", "-y",
		"-f", "rawvideo", "-pix_fmt", "bgr24",
		"-s", fmt.Sprintf("%dx%d", w, h),
		"-framerate", rate(s.FPS),
		"-i", "pipe:0",
	}
	if !s.Mute {
		args = append(args, "-ss", secs(s.Start), "-t", secs(s.Duration), "-i", s.Source)
	}
	args = append(args, "-map", "0:v:0")
	args = append(args, audioArgs(s.Mute, 1)...)
	return append(args,
		"-t", secs(s.Duration),
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "20",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		output,
	)
}

func rate(fps float64) string { return strconv.FormatFloat(fps, 'f', 6, 64) }

// ProcessFrames는 디코딩→fn→인코딩. 실패 시 output 삭제
func ProcessFrames(parent context.Context, s PipeSpec, output string, fn FrameFunc, onProgress ProgressFunc) (err error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	enc := tools.Command(ctx, tools.FFmpeg, EncodeArgs(s, output)...)
	encIn, err := enc.StdinPipe()
	if err != nil {
		return err
	}
	if err := enc.Start(); err != nil {
		return err
	}
	defer func() {
		if err == nil {
			return
		}
		os.Remove(output)
		switch {
		case parent.Err() != nil:
			err = parent.Err()
		case isNoSpace(enc.Stderr()):
			err = ErrNoSpace
		}
	}()

	total := s.FrameCount()
	err = DecodeFrames(ctx, s, func(n int, f frame.Frame) error {
		if err := fn(n, f); err != nil {
			return err
		}
		if _, err := encIn.Write(f.Pix); err != nil {
			return fmt.Errorf("인코더 쓰기: %w", err)
		}
		if onProgress != nil {
			onProgress(min(0.999, float64(n+1)/float64(total)))
		}
		return nil
	})
	if err != nil {
		cancel()
		encIn.Close()
		enc.Wait()
		return err
	}
	encIn.Close()
	if err := enc.Wait(); err != nil {
		return fmt.Errorf("인코더: %w", err)
	}
	if onProgress != nil {
		onProgress(1)
	}
	return nil
}

// DecodeFrames는 버퍼를 돌려 쓰며 프레임을 fn에 넘긴다.
func DecodeFrames(ctx context.Context, s PipeSpec, fn FrameFunc) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	dec := tools.Command(ctx, tools.FFmpeg, DecodeArgs(s)...)
	out, err := dec.StdoutPipe()
	if err != nil {
		return err
	}
	if err := dec.Start(); err != nil {
		return err
	}

	w, h := s.evenSize()
	free := make(chan frame.Frame, pipeBuffers)
	for range pipeBuffers {
		free <- frame.New(w, h)
	}
	full := make(chan frame.Frame, pipeBuffers)
	readErr := make(chan error, 1)
	go func() {
		defer close(full)
		for {
			var f frame.Frame
			select {
			case f = <-free:
			case <-ctx.Done():
				readErr <- ctx.Err()
				return
			}
			if _, err := io.ReadFull(out, f.Pix); err != nil {
				if errors.Is(err, io.EOF) {
					err = nil
				}
				readErr <- err
				return
			}
			full <- f
		}
	}()

	n := 0
	var fnErr error
	for f := range full {
		if fnErr == nil {
			fnErr = fn(n, f)
			if fnErr != nil {
				cancel()
			}
			n++
		}
		free <- f
	}
	rerr := <-readErr
	werr := dec.Wait()
	switch {
	case fnErr != nil:
		return fnErr
	case ctx.Err() != nil:
		return ctx.Err()
	case rerr != nil:
		return fmt.Errorf("디코더 읽기: %w", rerr)
	case werr != nil:
		return fmt.Errorf("디코더: %w", werr)
	case n == 0:
		return errors.New("디코더가 프레임을 내지 않음")
	}
	return nil
}

// PixelateRects는 고정 사각형들을 모자이크하는 FrameFunc다.
func PixelateRects(rects []PixelRect) FrameFunc {
	return func(_ int, f frame.Frame) error {
		for _, r := range rects {
			f.Pixelate(image.Rect(r.X, r.Y, r.X+r.W, r.Y+r.H), mosaicBlock)
		}
		return nil
	}
}
