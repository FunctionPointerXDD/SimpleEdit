// facecheck: 긴 변 gtDetectSize로 모든 프레임을 검출한 결과를 유사 정답으로 삼아
// filterCandidates(짧은 트랙 검출 수, 짧은 트랙 기준 점수)별 놓침을 비교한다. 폭 기준은 기본값
// 덮임은 저장과 같은 타원으로 재고, 가장자리 섞임 구간은 원본이 반 넘게 남으면 덮이지 않은 것으로 본다.

package main

import (
	"context"
	"fmt"
	"math"
	"sort"

	"simpleEdit/internal/automask"
	"simpleEdit/internal/detect"
	"simpleEdit/internal/frame"
	"simpleEdit/internal/media"
	"simpleEdit/internal/tools"
)

const gtDetectSize = 1280

const halfBlend = 1 - frame.EllipseFeather/2

var filterCandidates = []struct {
	shortDet   int
	shortScore float32
}{
	{0, 0}, {5, 0.75}, {10, 0.75}, {10, 0.8}, {15, 0.8}, {20, 0.85},
}

// 검출은 한 번만 모으고 후보마다 Track만 다시 돌려 놓침·표시 박스를 센다.
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
		o.Track.ShortDetections, o.Track.ShortScore = c.shortDet, c.shortScore
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
		fmt.Printf("검출 %2d회 미만은 %.2f | 놓침 %d/%d (%.2f%%) | 표시 박스 %d | 트랙 남김 %d, 버림 %d\n",
			c.shortDet, c.shortScore, miss, total, 100*float64(miss)/float64(max(1, total)), shown, st.Kept, st.Dropped)
		if verbose && c.shortDet == automask.DefaultTrackOptions.ShortDetections && c.shortScore == automask.DefaultTrackOptions.ShortScore {
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

// 기준 박스 안 9×9 점 중 80% 이상이 어느 마스크 타원의 반 이상 섞이는 안쪽(halfBlend)에 들면 true
func covered(g media.Rect, masks []media.Rect) bool {
	const n = 9
	in := 0
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			px, py := g.X+g.W*(float64(i)+0.5)/n, g.Y+g.H*(float64(j)+0.5)/n
			for _, m := range masks {
				dx, dy := (px-(m.X+m.W/2))/(m.W/2), (py-(m.Y+m.H/2))/(m.H/2)
				if math.Hypot(dx, dy) <= halfBlend {
					in++
					break
				}
			}
		}
	}
	return in*10 >= n*n*8
}

// 검출 수가 많은 트랙부터 30개까지 출력한다.
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
