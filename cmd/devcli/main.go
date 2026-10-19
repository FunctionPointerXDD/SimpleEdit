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
		var res download.Result
		start := time.Now()
		res, err = download.Download(ctx, os.Args[2], os.Args[3], func(p download.Progress) {
			fmt.Printf("\r%-8s %5.1f%%   ", p.Stage, p.Frac*100)
		})
		fmt.Println()
		if err == nil {
			fmt.Printf("완료 %s [%s] (%.1f초)\n", res.Path, res.Title, time.Since(start).Seconds())
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
	case "analyze":
		need(6)
		kind := "face"
		if len(os.Args) > 6 {
			kind = os.Args[6]
		}
		err = analyze(ctx, os.Args[2], os.Args[3], num(os.Args[4]), num(os.Args[5]), kind)
	case "track":
		need(8)
		err = track(ctx, os.Args[2], os.Args[3], num(os.Args[4]), num(os.Args[5]), num(os.Args[6]), os.Args[7])
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
	fmt.Printf("원본 %dx%d %.3ffps, 사각형 %v\n", info.Width, info.Height, info.FPS, px)
	t0 := time.Now()
	progress := func(f float64) { fmt.Printf("\r저장 %5.1f%%   ", f*100) }
	if len(px) > 0 {
		spec := media.PipeSpec{Source: src, Start: start, Duration: end - start, FPS: info.FPS, Width: info.Width, Height: info.Height}
		err = media.ProcessFrames(ctx, spec, out, media.BlurRects(px), progress)
	} else {
		err = media.Export(ctx, media.ExportSpec{Source: src, Output: out, Start: start, Duration: end - start}, progress)
	}
	fmt.Println()
	if err == nil {
		fmt.Printf("완료 %s (%.1f초)\n", out, time.Since(t0).Seconds())
	}
	return err
}

func analyze(ctx context.Context, src, out string, start, end float64, kind string) error {
	info, err := media.Probe(ctx, src)
	if err != nil {
		return err
	}
	if err := detect.Init(tools.ORTLib()); err != nil {
		return err
	}
	det, opts, err := detectorFor(kind, info.FPS)
	if err != nil {
		return err
	}
	defer det.Close()
	spec := media.PipeSpec{Source: src, Start: start, Duration: end - start, FPS: info.FPS, Width: info.Width, Height: info.Height}
	t0 := time.Now()
	res, err := automask.Analyze(ctx, spec, det, opts, func(f float64) { fmt.Printf("\r분석 %5.1f%%   ", f*100) })
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
	err = media.ProcessFrames(ctx, spec, out, automask.BlurFunc(nil, []automask.AutoLayer{{Frames: automask.Rects(res.Frames, nil), Ellipse: kind == "face"}}), func(f float64) { fmt.Printf("\r저장 %5.1f%%   ", f*100) })
	fmt.Println()
	if err == nil {
		fmt.Printf("저장 %.1f초: %s\n", time.Since(t1).Seconds(), out)
	}
	return err
}

// 그린 시각의 상자를 앞뒤로 따라가 그 경로로만 흐린 영상을 저장한다.
func track(ctx context.Context, src, out string, start, end, at float64, rectArg string) error {
	info, err := media.Probe(ctx, src)
	if err != nil {
		return err
	}
	rects, err := parseRects([]string{rectArg}, info)
	if err != nil || len(rects) == 0 {
		return fmt.Errorf("상자가 너무 작거나 형식이 틀림: %q", rectArg)
	}
	if err := detect.Init(tools.ORTLib()); err != nil {
		return err
	}
	tr, err := detect.NewTracker(tools.ModelPath(tools.TrackModel))
	if err != nil {
		return err
	}
	defer tr.Close()
	r := rects[0]
	spec := media.PipeSpec{Source: src, Start: start, Duration: end - start, FPS: info.FPS, Width: info.Width, Height: info.Height}
	t0 := time.Now()
	p, err := automask.Follow(ctx, spec, tr, automask.FrameAt(at, info.FPS), detect.PixelBox{X: r.X, Y: r.Y, W: r.W, H: r.H},
		func(f float64) { fmt.Printf("\r추적 %5.1f%%   ", f*100) })
	fmt.Println()
	if err != nil {
		return err
	}
	base := automask.FrameAt(start, info.FPS)
	fmt.Printf("추적 %.1f초: %.2f~%.2f초(%d프레임), 앞 놓침 %v, 뒤 놓침 %v\n", time.Since(t0).Seconds(),
		float64(p.First)/info.FPS, float64(p.First+len(p.Boxes))/info.FPS, len(p.Boxes), p.LostBefore, p.LostAfter)
	m := automask.ManualRect{First: p.First - base, End: p.First - base + len(p.Boxes), Path: p.Boxes}
	return media.ProcessFrames(ctx, spec, out, automask.BlurFunc([]automask.ManualRect{m}, nil), nil)
}

type closingDetector interface {
	automask.Detector
	Close()
}

// face면 YuNet과 OptionsFor, plate면 YOLOX와 PlateOptionsFor
func detectorFor(kind string, fps float64) (closingDetector, automask.Options, error) {
	switch kind {
	case "face":
		d, err := detect.NewFaceDetector(tools.ModelPath(tools.FaceModel))
		return d, automask.OptionsFor(fps), err
	case "plate":
		d, err := detect.NewPlateDetector(tools.ModelPath(tools.PlateModel))
		return d, automask.PlateOptionsFor(fps), err
	}
	return nil, automask.Options{}, fmt.Errorf("종류는 face 또는 plate: %q", kind)
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
  devcli analyze <원본> <출력> <시작초> <끝초> [face|plate]
  devcli track <원본> <출력> <시작초> <끝초> <그린초> x,y,w,h
  devcli facecheck <원본> <시작초> <끝초> [정답 점수=0.6]
  devcli cancel <원본> <출력> <초>
  devcli update`)
	os.Exit(2)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "오류:", err)
	os.Exit(1)
}
