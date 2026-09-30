// 좌표는 모두 화면 대비 0~1 비율이다. MaxGap·Hold는 프레임 수다.
// MatchIoU는 같은 대상으로 볼 최소 IoU, Expand 0.4는 폭·높이를 40% 키운다는 뜻이다.
// 평균 폭이 Large 이상인 트랙은 최고 점수가 Strong, 아니면 Weak 이상이어야 남는다.
// 기본 Weak 0은 작은 트랙(멀리 있는 얼굴)을 거르지 않으려는 것이다.
// ShortDetections보다 적게 검출된 짧은 트랙은 최고 점수가 ShortScore 이상이어야 남는다.
// 한두 프레임 스친 낮은 점수 오탐이 Hold로 깜빡이는 것을 막고, 드문드문 잡힌 작은 실제 얼굴은 살린다.
// TrackBox.ID는 남긴 트랙 순서대로 1부터 매긴다.

// Package automask는 자동 검출 박스를 프레임마다 이어 준다.
package automask

import (
	"sort"

	"simpleEdit/internal/detect"
	"simpleEdit/internal/media"
)

type TrackOptions struct {
	MatchIoU float32
	MaxGap   int
	Hold     int
	Expand   float64
	Strong   float32
	Weak     float32
	Large    float64

	ShortDetections int
	ShortScore      float32
}

var DefaultTrackOptions = TrackOptions{
	MatchIoU: 0.2, MaxGap: 15, Hold: 8, Expand: 0.4,
	Strong: 0.7, Weak: 0, Large: 0.08,
	ShortDetections: 10, ShortScore: 0.8,
}

type TrackBox struct {
	Rect media.Rect
	ID   int
}

type Det struct {
	Box   media.Rect
	Score float32
}

type Sample struct {
	Frame int
	Dets  []Det
}

type TrackStats struct {
	Kept, Dropped int
	Tracks        []TrackInfo
}

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

// 정렬한 복사본을 link로 잇고, 점수가 폭 기준에 못 미치거나 검출이 적은 트랙은 버린다.
func Track(samples []Sample, frames int, o TrackOptions) ([][]TrackBox, TrackStats) {
	sorted := append([]Sample(nil), samples...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Frame < sorted[j].Frame })
	out := make([][]TrackBox, frames)
	var st TrackStats
	for _, tr := range link(sorted, o) {
		info := summarize(tr)
		info.Kept = info.MaxScore >= o.required(info)
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

// 폭 기준(Large 이상이면 Strong, 아니면 Weak)과 짧은 트랙 기준 중 큰 값
func (o TrackOptions) required(t TrackInfo) float32 {
	need := o.Weak
	if t.Width >= o.Large {
		need = o.Strong
	}
	if t.Detections < o.ShortDetections {
		need = max(need, o.ShortScore)
	}
	return need
}

// 검출마다 MaxGap 안의 트랙 중 IoU가 가장 큰 곳에 붙이고, 없으면 새 트랙을 연다.
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

// 첫 검출 앞 Hold, 검출 사이는 선형 보간, 끝 검출 뒤 Hold 프레임까지 확대해 넣는다.
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

// exclude에 든 ID의 박스만 빼고 Rect를 꺼낸다.
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

// 중심을 유지한 채 폭·높이를 k배만큼 키우고 0~1로 자른다.
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
