package main

import (
	"context"
	"fmt"
	"sort"

	"simpleEdit/internal/automask"
	"simpleEdit/internal/detect"
	"simpleEdit/internal/frame"
	"simpleEdit/internal/media"
	"simpleEdit/internal/tools"
)

// 유사 정답: 모든 프레임을 이 크기로 검출
const gtDetectSize = 1280

// 비교할 거르기 설정: 큰 박스 기준, 작은 박스 기준, 큰 박스 폭
var filterCandidates = []struct {
	strong, weak float32
	large        float64
}{
	{0, 0, 0}, {0.7, 0, 0.06}, {0.7, 0, 0.07}, {0.7, 0, 0.08},
}

// facecheck는 강한 점수 기준별 놓침·트랙을 비교한다.
func facecheck(ctx context.Context, src string, start, end float64, gtScore float32, verbose bool) error {
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

	det.ScoreThreshold = gtScore
	gt, err := groundTruth(ctx, spec, det)
	if err != nil {
		return err
	}
	det.ScoreThreshold = detect.DefaultFaceScore
	o := automask.OptionsFor(info.FPS)
	samples, frames, _, err := automask.Collect(ctx, spec, det, o, nil)
	if err != nil {
		return err
	}
	total := 0
	for _, g := range gt {
		total += len(g)
	}
	fmt.Printf("유사 정답: %d프레임, 얼굴 %d건 (점수 %.2f 이상, %dpx)\n", len(gt), total, gtScore, gtDetectSize)
	for _, c := range filterCandidates {
		o.Track.Strong, o.Track.Weak, o.Track.Large = c.strong, c.weak, c.large
		tracked, st := automask.Track(samples, frames, o.Track)
		boxes := automask.Rects(tracked, nil)
		miss, shown := 0, 0
		for n, g := range gt {
			for _, b := range g {
				if n >= len(boxes) || !covered(b, boxes[n]) {
					miss++
				}
			}
		}
		for _, b := range boxes {
			shown += len(b)
		}
		fmt.Printf("폭≥%.2f는 %.2f, 그 외 %.2f | 놓침 %d/%d (%.2f%%) | 표시 박스 %d | 트랙 남김 %d, 버림 %d\n",
			c.large, c.strong, c.weak, miss, total, 100*float64(miss)/float64(max(1, total)), shown, st.Kept, st.Dropped)
		if verbose && c.large == automask.DefaultTrackOptions.Large && c.strong == automask.DefaultTrackOptions.Strong {
			printTracks(st, info.FPS, start)
		}
	}
	return nil
}

func groundTruth(ctx context.Context, spec media.PipeSpec, det *detect.FaceDetector) ([][]media.Rect, error) {
	gs := spec
	k := float64(gtDetectSize) / float64(max(spec.Width, spec.Height))
	if k < 1 {
		gs.Width, gs.Height = int(float64(spec.Width)*k)&^1, int(float64(spec.Height)*k)&^1
	}
	var gt [][]media.Rect
	err := media.DecodeFrames(ctx, gs, func(_ int, f frame.Frame) error {
		boxes, err := det.Detect(f)
		if err != nil {
			return err
		}
		rs := make([]media.Rect, len(boxes))
		for i, b := range boxes {
			rs[i] = media.Rect{X: float64(b.X) / float64(f.W), Y: float64(b.Y) / float64(f.H),
				W: float64(b.W) / float64(f.W), H: float64(b.H) / float64(f.H)}
		}
		gt = append(gt, rs)
		return nil
	})
	return gt, err
}

// covered는 기준 박스 면적의 80% 이상이 덮였는지 본다.
func covered(g media.Rect, masks []media.Rect) bool {
	for _, m := range masks {
		x0, y0 := max(g.X, m.X), max(g.Y, m.Y)
		x1, y1 := min(g.X+g.W, m.X+m.W), min(g.Y+g.H, m.Y+m.H)
		if x1 > x0 && y1 > y0 && (x1-x0)*(y1-y0) >= 0.8*g.W*g.H {
			return true
		}
	}
	return false
}

// printTracks는 검출이 많은 트랙부터 몇 개 보여 준다.
func printTracks(st automask.TrackStats, fps, start float64) {
	tr := append([]automask.TrackInfo(nil), st.Tracks...)
	sort.Slice(tr, func(i, j int) bool { return tr[i].Detections > tr[j].Detections })
	for i, t := range tr[:min(30, len(tr))] {
		state := "버림"
		if t.Kept {
			state = "남김"
		}
		fmt.Printf("  %d. %s | %.1f~%.1f초 | 검출 %d | 최고 점수 %.2f | 중심 (%.2f, %.2f) | 폭 %.3f\n", i+1, state,
			start+float64(t.First)/fps, start+float64(t.Last)/fps, t.Detections, t.MaxScore, t.Center[0], t.Center[1], t.Width)
	}
}
