// Package automask는 자동 검출 박스를 프레임마다 이어 준다.
package automask

import (
	"sort"

	"simpleEdit/internal/detect"
	"simpleEdit/internal/media"
)

// TrackOptions는 놓치지 않는 쪽으로 기울인 추적 설정이다.
type TrackOptions struct {
	MatchIoU float32 // 같은 대상으로 볼 최소 IoU
	MaxGap   int     // 이 프레임 수까지 끊겨도 보간
	Hold     int     // 처음·마지막 검출 앞뒤로 유지할 프레임 수
	Expand   float64 // 박스 폭·높이 확대 비율 (0.25 = 25%)
	Strong   float32 // 큰 트랙이 남으려면 한 번은 넘을 점수
	Weak     float32 // 작은 트랙이 남으려면 한 번은 넘을 점수
	Large    float64 // 평균 폭(화면 비율)이 이 이상이면 큰 트랙
}

// DefaultTrackOptions는 30fps 기준값. 보통 OptionsFor 사용
var DefaultTrackOptions = TrackOptions{
	MatchIoU: 0.2, MaxGap: 15, Hold: 8, Expand: 0.4,
	Strong: 0.7, Weak: 0, Large: 0.08,
}

// TrackBox는 프레임에 그릴 박스와 트랙 번호(1부터)다.
type TrackBox struct {
	Rect media.Rect
	ID   int
}

// Det는 비율 좌표 박스와 검출 점수다.
type Det struct {
	Box   media.Rect
	Score float32
}

// Sample은 한 프레임의 검출 결과다.
type Sample struct {
	Frame int
	Dets  []Det
}

// TrackStats는 남긴·버린 트랙 수와 트랙별 요약이다.
type TrackStats struct {
	Kept, Dropped int
	Tracks        []TrackInfo
}

// TrackInfo는 트랙 하나의 구간, 최고 점수, 평균 위치다.
type TrackInfo struct {
	First, Last int
	Detections  int
	MaxScore    float32
	Center      [2]float64
	Width       float64
	Kept        bool
}

type obs struct {
	frame int
	box   media.Rect
	score float32
}

// Track은 샘플을 잇고 강한 검출 없는 트랙은 버린다.
func Track(samples []Sample, frames int, o TrackOptions) ([][]TrackBox, TrackStats) {
	sorted := append([]Sample(nil), samples...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Frame < sorted[j].Frame })
	out := make([][]TrackBox, frames)
	var st TrackStats
	for _, tr := range link(sorted, o) {
		info := summarize(tr)
		info.Kept = info.MaxScore >= o.required(info.Width)
		st.Tracks = append(st.Tracks, info)
		if !info.Kept {
			st.Dropped++
			continue
		}
		st.Kept++
		fill(out, tr, st.Kept, o)
	}
	return out, st
}

func summarize(tr []obs) TrackInfo {
	info := TrackInfo{First: tr[0].frame, Last: tr[len(tr)-1].frame, Detections: len(tr)}
	for _, ob := range tr {
		info.MaxScore = max(info.MaxScore, ob.score)
		info.Center[0] += (ob.box.X + ob.box.W/2) / float64(len(tr))
		info.Center[1] += (ob.box.Y + ob.box.H/2) / float64(len(tr))
		info.Width += ob.box.W / float64(len(tr))
	}
	return info
}

// required는 트랙 크기에 따라 넘어야 할 점수다.
func (o TrackOptions) required(width float64) float32 {
	if width >= o.Large {
		return o.Strong
	}
	return o.Weak
}

// link는 IoU가 가장 큰 기존 트랙에 탐욕적으로 붙인다.
func link(samples []Sample, o TrackOptions) [][]obs {
	var tracks [][]obs
	for _, s := range samples {
		used := make([]bool, len(tracks))
		for _, d := range s.Dets {
			b := d.Box
			best, bestIoU := -1, o.MatchIoU
			for ti, tr := range tracks {
				last := tr[len(tr)-1]
				if used[ti] || s.Frame-last.frame > o.MaxGap {
					continue
				}
				if v := iou(last.box, b); v >= bestIoU {
					best, bestIoU = ti, v
				}
			}
			if best < 0 {
				tracks = append(tracks, []obs{{s.Frame, b, d.Score}})
				used = append(used, true)
				continue
			}
			tracks[best] = append(tracks[best], obs{s.Frame, b, d.Score})
			used[best] = true
		}
	}
	return tracks
}

// fill은 한 트랙을 보간·유지·확대해 프레임별로 넣는다.
func fill(out [][]TrackBox, tr []obs, id int, o TrackOptions) {
	put := func(n int, r media.Rect) {
		if n >= 0 && n < len(out) {
			out[n] = append(out[n], TrackBox{Rect: expand(r, o.Expand), ID: id})
		}
	}
	first, last := tr[0], tr[len(tr)-1]
	for n := first.frame - o.Hold; n < first.frame; n++ {
		put(n, first.box)
	}
	for i, cur := range tr {
		put(cur.frame, cur.box)
		if i+1 == len(tr) {
			break
		}
		next := tr[i+1]
		for n := cur.frame + 1; n < next.frame; n++ {
			t := float64(n-cur.frame) / float64(next.frame-cur.frame)
			put(n, lerp(cur.box, next.box, t))
		}
	}
	for n := last.frame + 1; n <= last.frame+o.Hold; n++ {
		put(n, last.box)
	}
}

// Rects는 뺀 트랙을 제외한 프레임별 사각형이다.
func Rects(frames [][]TrackBox, exclude map[int]bool) [][]media.Rect {
	out := make([][]media.Rect, len(frames))
	for n, boxes := range frames {
		for _, b := range boxes {
			if !exclude[b.ID] {
				out[n] = append(out[n], b.Rect)
			}
		}
	}
	return out
}

func lerp(a, b media.Rect, t float64) media.Rect {
	return media.Rect{
		X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t,
		W: a.W + (b.W-a.W)*t, H: a.H + (b.H-a.H)*t,
	}
}

// expand는 중심을 유지한 채 키우고 0~1로 자른다.
func expand(r media.Rect, k float64) media.Rect {
	dx, dy := r.W*k/2, r.H*k/2
	x0, y0 := max(0, r.X-dx), max(0, r.Y-dy)
	x1, y1 := min(1, r.X+r.W+dx), min(1, r.Y+r.H+dy)
	return media.Rect{X: x0, Y: y0, W: max(0, x1-x0), H: max(0, y1-y0)}
}

func iou(a, b media.Rect) float32 {
	return detect.IoU(
		detect.Box{X: float32(a.X), Y: float32(a.Y), W: float32(a.W), H: float32(a.H)},
		detect.Box{X: float32(b.X), Y: float32(b.Y), W: float32(b.W), H: float32(b.H)},
	)
}
