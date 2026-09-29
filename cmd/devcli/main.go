// devcli는 UI 없이 기술 검증하는 개발용 명령이다.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"simpleEdit/internal/applog"
	"simpleEdit/internal/automask"
	"simpleEdit/internal/detect"
	"simpleEdit/internal/download"
	"simpleEdit/internal/media"
	"simpleEdit/internal/tools"
)

func main() {
	applog.SetOutput(os.Stderr)
	if len(os.Args) < 2 {
		usage()
	}
	ctx := context.Background()
	var err error
	switch os.Args[1] {
	case "download":
		need(4)
		if len(os.Args) > 4 {
			c, cancel := context.WithTimeout(ctx, time.Duration(num(os.Args[4])*float64(time.Second)))
			defer cancel()
			ctx = c
		}
		var path string
		start := time.Now()
		path, err = download.Download(ctx, os.Args[2], os.Args[3], func(p download.Progress) {
			fmt.Printf("\r%-8s %5.1f%%   ", p.Stage, p.Frac*100)
		})
		fmt.Println()
		if err == nil {
			fmt.Printf("완료 %s (%.1f초)\n", path, time.Since(start).Seconds())
		}
	case "probe":
		need(3)
		var info media.Info
		info, err = media.Probe(ctx, os.Args[2])
		if err == nil {
			fmt.Printf("%+v\n재생 가능: %v\n", info, info.PlayableInWebView())
		}
	case "export":
		need(6)
		err = export(ctx, os.Args[2], os.Args[3], num(os.Args[4]), num(os.Args[5]), os.Args[6:])
	case "pipe":
		need(6)
		err = exportPipe(ctx, os.Args[2], os.Args[3], num(os.Args[4]), num(os.Args[5]), os.Args[6:])
	case "analyze":
		need(6)
		err = analyze(ctx, os.Args[2], os.Args[3], num(os.Args[4]), num(os.Args[5]))
	case "facecheck":
		need(5)
		gtScore := float32(0.6)
		if len(os.Args) > 5 {
			gtScore = float32(num(os.Args[5]))
		}
		err = facecheck(ctx, os.Args[2], num(os.Args[3]), num(os.Args[4]), gtScore, os.Getenv("VERBOSE") != "")
	case "cancel":
		need(5)
		c, cancel := context.WithTimeout(ctx, time.Duration(num(os.Args[4])*float64(time.Second)))
		defer cancel()
		info, perr := media.Probe(ctx, os.Args[2])
		if perr != nil {
			fail(perr)
		}
		err = export(c, os.Args[2], os.Args[3], 0, info.Duration, nil)
		_, statErr := os.Stat(os.Args[3])
		fmt.Printf("취소 결과: err=%v, 출력 파일 남음=%v\n", err, statErr == nil)
		if statErr == nil {
			os.Exit(1)
		}
		err = nil
	case "update":
		err = download.Update(ctx)
	default:
		usage()
	}
	if err != nil {
		fail(err)
	}
}

func export(ctx context.Context, src, out string, start, end float64, rectArgs []string) error {
	info, err := media.Probe(ctx, src)
	if err != nil {
		return err
	}
	px, err := parseRects(rectArgs, info)
	if err != nil {
		return err
	}
	fmt.Printf("원본 %dx%d, 사각형 %v\n", info.Width, info.Height, px)
	t0 := time.Now()
	err = media.Export(ctx, media.ExportSpec{
		Source: src, Output: out, Start: start, Duration: end - start, Rects: px,
	}, func(f float64) { fmt.Printf("\r저장 %5.1f%%   ", f*100) })
	fmt.Println()
	if err == nil {
		fmt.Printf("완료 %s (%.1f초)\n", out, time.Since(t0).Seconds())
	}
	return err
}

func exportPipe(ctx context.Context, src, out string, start, end float64, rectArgs []string) error {
	info, err := media.Probe(ctx, src)
	if err != nil {
		return err
	}
	px, err := parseRects(rectArgs, info)
	if err != nil {
		return err
	}
	spec := media.PipeSpec{Source: src, Start: start, Duration: end - start, FPS: info.FPS, Width: info.Width, Height: info.Height}
	fmt.Printf("원본 %dx%d %.3ffps, 사각형 %v\n", info.Width, info.Height, info.FPS, px)
	t0 := time.Now()
	err = media.ProcessFrames(ctx, spec, out, media.PixelateRects(px), func(f float64) { fmt.Printf("\r파이프 %5.1f%%   ", f*100) })
	fmt.Println()
	if err == nil {
		fmt.Printf("완료 %s (%.1f초)\n", out, time.Since(t0).Seconds())
	}
	return err
}

func analyze(ctx context.Context, src, out string, start, end float64) error {
	info, err := media.Probe(ctx, src)
	if err != nil {
		return err
	}
	if err := detect.Init(tools.ORTLib()); err != nil {
		return err
	}
	det, err := detect.NewFaceDetector(tools.ModelPath(tools.FaceModel))
	if err != nil {
		return err
	}
	defer det.Close()
	spec := media.PipeSpec{Source: src, Start: start, Duration: end - start, FPS: info.FPS, Width: info.Width, Height: info.Height}
	t0 := time.Now()
	res, err := automask.Analyze(ctx, spec, det, automask.OptionsFor(info.FPS), func(f float64) { fmt.Printf("\r분석 %5.1f%%   ", f*100) })
	fmt.Println()
	if err != nil {
		return err
	}
	covered := 0
	for _, fr := range res.Frames {
		if len(fr) > 0 {
			covered++
		}
	}
	fmt.Printf("분석 %.1f초: 프레임 %d개, 검출 %d건, 박스 있는 프레임 %d개\n", time.Since(t0).Seconds(), len(res.Frames), res.Found, covered)
	data, _ := json.Marshal(res)
	if err := os.WriteFile(out+".json", data, 0o644); err != nil {
		return err
	}
	t1 := time.Now()
	err = media.ProcessFrames(ctx, spec, out, automask.PixelateFunc(nil, automask.Rects(res.Frames, nil)), func(f float64) { fmt.Printf("\r저장 %5.1f%%   ", f*100) })
	fmt.Println()
	if err == nil {
		fmt.Printf("저장 %.1f초: %s\n", time.Since(t1).Seconds(), out)
	}
	return err
}

func parseRects(args []string, info media.Info) ([]media.PixelRect, error) {
	var rects []media.Rect
	for _, a := range args {
		f := strings.Split(a, ",")
		if len(f) != 4 {
			return nil, fmt.Errorf("사각형 형식은 x,y,w,h: %q", a)
		}
		rects = append(rects, media.Rect{X: num(f[0]), Y: num(f[1]), W: num(f[2]), H: num(f[3])})
	}
	return media.RectsToPixels(rects, info.Width, info.Height), nil
}

func num(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		fail(fmt.Errorf("숫자가 아님: %q", s))
	}
	return v
}

func need(n int) {
	if len(os.Args) < n {
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `사용법:
  devcli download <URL> <작업폴더> [취소초]
  devcli probe <파일>
  devcli export <원본> <출력> <시작초> <끝초> [x,y,w,h ...]
  devcli pipe <원본> <출력> <시작초> <끝초> [x,y,w,h ...]
  devcli analyze <원본> <출력> <시작초> <끝초>
  devcli facecheck <원본> <시작초> <끝초> [정답 점수=0.6]
  devcli cancel <원본> <출력> <초>
  devcli update`)
	os.Exit(2)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "오류:", err)
	os.Exit(1)
}
