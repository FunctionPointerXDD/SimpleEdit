// FrameFunc는 n번째 프레임을 제자리에서 고친다. 반환 뒤 버퍼는 다시 쓰인다.
// 분석(DecodeFrames)은 bgr24, 저장(ProcessFrames)은 yuv420p로 주고받는다. 저장 쪽은 원본·출력이
// 모두 yuv420p라 bgr24를 거치면 색 변환만 두 번 늘어난다(2026-09-30: 1080p 60초 저장 21.5→15.0초).
// 여러 구간을 이어 붙일 때(ProcessSegments)는 구간마다 디코더를 차례로 열어 인코더 하나에 넣는다.
// 구간마다 정확히 FrameCount장을 넣고(모자라면 마지막 장 반복, 넘치면 버림) 소리도 구간마다
// FrameCount/fps초로 맞춰(모자라면 무음으로 채움) concat으로 이어, 영상과 소리가 어긋나지 않는다.
// 소리가 없는 원본은 Mute로 불러야 한다(concat 필터는 없는 소리 입력을 건너뛰지 못함).

package media

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"strconv"
	"strings"

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

// 구간을 fps 필터로 고정 fps로 만들고 짝수 크기 원시 프레임(format)으로 stdout에 낸다.
func DecodeArgs(s PipeSpec, format frame.Format) []string {
	w, h := s.evenSize()
	return []string{
		"-hide_banner", "-nostdin", "-loglevel", "error",
		"-ss", secs(s.Start),
		"-i", s.Source,
		"-t", secs(s.Duration),
		"-map", "0:v:0",
		"-vf", fmt.Sprintf("fps=%s,scale=%d:%d:flags=area", rate(s.FPS), w, h),
		"-f", "rawvideo", "-pix_fmt", pixFmt(format),
		"pipe:1",
	}
}

// stdin yuv420p 원시 프레임을 H.264로 인코딩하고, 무음이 아니면 구간마다 원본 소리를 이어 붙인다.
func EncodeArgs(parts []PipeSpec, output string) []string {
	s := parts[0]
	w, h := s.evenSize()
	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error", "-y",
		"-f", "rawvideo", "-pix_fmt", pixFmt(frame.YUV420),
		"-s", fmt.Sprintf("%dx%d", w, h),
		"-framerate", rate(s.FPS),
		"-i", "pipe:0",
	}
	total := 0.0
	for _, p := range parts {
		total += p.frameSeconds()
		if !s.Mute {
			args = append(args, "-ss", secs(p.Start), "-t", secs(p.frameSeconds()), "-i", p.Source)
		}
	}
	args = append(args, "-map", "0:v:0")
	switch {
	case s.Mute || len(parts) == 1:
		args = append(args, audioArgs(s.Mute, 1)...)
	default:
		var graph, labels strings.Builder
		for i, p := range parts {
			fmt.Fprintf(&graph, "[%d:a:0]apad,atrim=duration=%s,asetpts=PTS-STARTPTS[a%d];", i+1, secs(p.frameSeconds()), i)
			fmt.Fprintf(&labels, "[a%d]", i)
		}
		args = append(args, "-filter_complex", fmt.Sprintf("%s%sconcat=n=%d:v=0:a=1[a]", graph.String(), labels.String(), len(parts)),
			"-map", "[a]", "-c:a", "aac", "-b:a", "160k")
	}
	return append(args,
		"-t", secs(total),
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "20",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		output,
	)
}

// FrameCount장의 길이(초)
func (s PipeSpec) frameSeconds() float64 { return float64(s.FrameCount()) / s.FPS }

func pixFmt(f frame.Format) string {
	if f == frame.YUV420 {
		return "yuv420p"
	}
	return "bgr24"
}

func rate(fps float64) string { return strconv.FormatFloat(fps, 'f', 6, 64) }

// 한 구간만 ProcessSegments
func ProcessFrames(parent context.Context, s PipeSpec, output string, fn FrameFunc, onProgress ProgressFunc) error {
	return ProcessSegments(parent, []PipeSpec{s}, output, []FrameFunc{fn}, onProgress)
}

// 구간마다 디코딩 → fns[i](nil이면 그대로) → 인코더 stdin. 실패하면 인코더를 멈추고 output을 지운다.
func ProcessSegments(parent context.Context, parts []PipeSpec, output string, fns []FrameFunc, onProgress ProgressFunc) (err error) {
	if len(parts) == 0 || len(parts) != len(fns) {
		return errors.New("저장할 구간 없음")
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	enc := tools.Command(ctx, tools.FFmpeg, EncodeArgs(parts, output)...)
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

	total, written := 0, 0
	for _, p := range parts {
		total += p.FrameCount()
	}
	write := func(f frame.Frame) error {
		if _, err := encIn.Write(f.Pix); err != nil {
			return fmt.Errorf("인코더 쓰기: %w", err)
		}
		written++
		if onProgress != nil {
			onProgress(min(0.999, float64(written)/float64(total)))
		}
		return nil
	}
	for i, p := range parts {
		if err = encodePart(ctx, p, fns[i], write); err != nil {
			cancel()
			encIn.Close()
			enc.Wait()
			return err
		}
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

var errPartFull = errors.New("구간 프레임 다 참")

// p를 디코딩해 fn을 거쳐 write에 FrameCount장을 넘긴다. 모자라면 마지막 장을 반복한다.
func encodePart(ctx context.Context, p PipeSpec, fn FrameFunc, write func(frame.Frame) error) error {
	want, n := p.FrameCount(), 0
	var last []byte
	err := decodeFrames(ctx, p, frame.YUV420, func(i int, f frame.Frame) error {
		if fn != nil {
			if err := fn(i, f); err != nil {
				return err
			}
		}
		if err := write(f); err != nil {
			return err
		}
		n++
		if n >= want {
			return errPartFull
		}
		last = append(last[:0], f.Pix...)
		return nil
	})
	if errors.Is(err, errPartFull) {
		return nil
	}
	if err != nil {
		return err
	}
	w, h := p.evenSize()
	pad := frame.NewYUV(w, h)
	copy(pad.Pix, last)
	for ; n < want; n++ {
		if err := write(pad); err != nil {
			return err
		}
	}
	return nil
}

// bgr24로 decodeFrames
func DecodeFrames(ctx context.Context, s PipeSpec, fn FrameFunc) error {
	return decodeFrames(ctx, s, frame.BGR, fn)
}

// yuv420p로 decodeFrames
func DecodeYUV(ctx context.Context, s PipeSpec, fn FrameFunc) error {
	return decodeFrames(ctx, s, frame.YUV420, fn)
}

// 읽기 고루틴과 버퍼 pipeBuffers개를 돌려 쓰고, fn이 실패하면 디코더를 취소한다.
func decodeFrames(ctx context.Context, s PipeSpec, format frame.Format, fn FrameFunc) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	dec := tools.Command(ctx, tools.FFmpeg, DecodeArgs(s, format)...)
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
		if format == frame.YUV420 {
			free <- frame.NewYUV(w, h)
		} else {
			free <- frame.New(w, h)
		}
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
