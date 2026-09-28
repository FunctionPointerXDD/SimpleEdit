package media

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"simpleEdit/internal/tools"
)

// ProgressFunc는 0~1 사이 진행 비율을 받는다.
type ProgressFunc func(frac float64)

// ErrNoSpace는 디스크 공간 부족으로 ffmpeg가 실패했을 때 반환된다.
var ErrNoSpace = errors.New("디스크 공간 부족")

// Export는 ffmpeg로 저장한다. 실패하거나 취소되면 미완성 파일을 지운다.
func Export(ctx context.Context, s ExportSpec, onProgress ProgressFunc) error {
	return runFFmpeg(ctx, ExportArgs(s), s.Output, s.Duration, onProgress)
}

// MakePreview는 미리보기용 H.264 파일을 만든다.
func MakePreview(ctx context.Context, source, output string, duration float64, onProgress ProgressFunc) error {
	return runFFmpeg(ctx, PreviewArgs(source, output), output, duration, onProgress)
}

func runFFmpeg(ctx context.Context, args []string, output string, duration float64, onProgress ProgressFunc) (err error) {
	cmd := tools.Command(ctx, tools.FFmpeg, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(output)
		}
	}()

	ReadProgress(stdout, duration, onProgress)
	err = cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		if isNoSpace(cmd.Stderr()) {
			return ErrNoSpace
		}
		return fmt.Errorf("ffmpeg: %w", err)
	}
	return nil
}

// ReadProgress는 out_time_us로 진행률을 알린다.
func ReadProgress(r io.Reader, duration float64, onProgress ProgressFunc) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		key, val, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if !ok || onProgress == nil {
			continue
		}
		switch key {
		case "out_time_us", "out_time_ms":
			us, err := strconv.ParseInt(val, 10, 64)
			if err != nil || us < 0 || duration <= 0 {
				continue
			}
			onProgress(min(1, float64(us)/1e6/duration))
		case "progress":
			if val == "end" {
				onProgress(1)
			}
		}
	}
	_, _ = io.Copy(io.Discard, r)
}

func isNoSpace(stderr string) bool {
	s := strings.ToLower(stderr)
	return strings.Contains(s, "no space left") || strings.Contains(s, "not enough space") ||
		strings.Contains(s, "disk full")
}
