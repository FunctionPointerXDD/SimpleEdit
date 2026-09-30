// facecheck: 긴 변 gtDetectSize로 모든 프레임을 검출한 결과를 유사 정답으로 삼아
// smoothCandidates(위치·크기를 부드럽게 하는 시간 σ(초), 박스 확대율)별 놓침과 흔들림을 비교한다.
// 흔들림: 같은 트랙의 연속 세 프레임 중심 2차 차분(가속도), 연속 두 프레임 폭 변화를 박스 폭 대비 %로 평균
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

var smoothCandidates = []struct{ pos, size, expand float64 }{
	{0, 0, 0.4}, {0.05, 0.1, 0.4}, {0.05, 0.1, 0.5}, {0.1, 0.2, 0.4}, {0.1, 0.2, 0.5}, {0.1, 0.2, 0.6},
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
	for _, c := range smoothCandidates {
		o.Track.SmoothPos, o.Track.SmoothSize, o.Track.Expand = c.pos*info.FPS, c.size*info.FPS, c.expand
		tracked, st := automask.Track(samples, frames, o.Track)
		boxes := automask.Rects(tracked, nil)
		miss := 0
		for n, g := range gt {
			for _, b := range g {
				if n >= len(boxes) || !covered(b, boxes[n]) {
					miss++
				}
			}
		}
		acc, size := jitter(tracked)
		fmt.Printf("부드럽게 위치 %.2f초·크기 %.2f초·확대 %.0f%% | 놓침 %d/%d (%.2f%%) | 흔들림: 중심 가속 %.2f%%, 폭 변화 %.2f%% | 트랙 %d\n",
			c.pos, c.size, 100*c.expand, miss, total, 100*float64(miss)/float64(max(1, total)), acc, size, st.Kept)
		if verbose && c == smoothCandidates[0] {
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

// 트랙마다 프레임 순으로 모아 중심 2차 차분과 폭 변화를 폭 대비 %로 평균한다.
func jitter(tracked [][]automask.TrackBox) (acc, size float64) {
	byID := map[int][]struct {
		n int
		r media.Rect
	}{}
	for n, boxes := range tracked {
		for _, b := range boxes {
			byID[b.ID] = append(byID[b.ID], struct {
				n int
				r media.Rect
			}{n, b.Rect})
		}
	}
	var sa, ss float64
	var na, ns int
	for _, seq := range byID {
		for i := 1; i < len(seq); i++ {
			a, b := seq[i-1], seq[i]
			if b.n != a.n+1 || b.r.W == 0 {
				continue
			}
			ss += math.Abs(b.r.W-a.r.W) / b.r.W
			ns++
			if i+1 < len(seq) && seq[i+1].n == b.n+1 {
				c := seq[i+1]
				ax := (c.r.X + c.r.W/2) - 2*(b.r.X+b.r.W/2) + (a.r.X + a.r.W/2)
				ay := (c.r.Y + c.r.H/2) - 2*(b.r.Y+b.r.H/2) + (a.r.Y + a.r.H/2)
				sa += math.Hypot(ax, ay) / b.r.W
				na++
			}
		}
	}
	return 100 * sa / float64(max(1, na)), 100 * ss / float64(max(1, ns))
}
