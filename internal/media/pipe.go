// FrameFunc는 n번째 프레임을 제자리에서 고친다. 반환 뒤 버퍼는 다시 쓰인다.

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

type PipeSpec struct {
	Source   string
	Start    float64
	Duration float64
	FPS      float64
	Width    int
	Height   int
	Mute     bool
}

type FrameFunc func(n int, f frame.Frame) error

const pipeBuffers = 3

// 길이 × fps를 반올림한다. 최소 1
func (s PipeSpec) FrameCount() int {
	return max(1, int(s.Duration*s.FPS+0.5))
}

// yuv420p 인코딩을 위해 짝수로 내린다. 최소 2
func (s PipeSpec) evenSize() (int, int) {
	return max(2, s.Width&^1), max(2, s.Height&^1)
}

// 구간을 fps 필터로 고정 fps로 만들고 짝수 크기 bgr24로 stdout에 낸다.
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

// stdin 원시 프레임을 H.264로 인코딩하고, 무음이 아니면 원본 구간 소리를 붙인다.
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

// 디코딩 → fn → 인코더 stdin. 실패하면 인코더를 멈추고 output을 지운다.
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

// 읽기 고루틴과 버퍼 pipeBuffers개를 돌려 쓰고, fn이 실패하면 디코더를 취소한다.
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

func BlurRects(rects []PixelRect) FrameFunc {
	return func(_ int, f frame.Frame) error {
		for _, r := range rects {
			f.Blur(image.Rect(r.X, r.Y, r.X+r.W, r.Y+r.H), BlurSigma(r.W, r.H))
		}
		return nil
	}
}
