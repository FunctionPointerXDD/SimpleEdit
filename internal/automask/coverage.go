// Coverage는 영상 전체 프레임 번호(시각 × fps 반올림) 기준 자동 분석 결과이고, 떨어진 조각(Pieces)을 담는다.
// 구간을 바꾸거나 여러 구간을 이어 붙여도 이미 분석한 프레임과 트랙 번호(뺀 트랙)를 그대로 쓰고,
// 빠진 곳만 더 분석해 붙인다. 조각은 시작 순서로 두고, 맞닿으면 하나로 합친다.
// 붙인 조각의 트랙 번호는 NextID부터 이어 매겨 다른 조각 번호와 겹치지 않는다.
// FrameSpan은 [First, End) 프레임 번호 구간이다.
// 저장 프레임 수 round(길이×fps)와 분석 끝 round(끝×fps)는 반올림 때문에 1프레임 어긋날 수 있어
// Window는 끝이 windowSlack 프레임까지 모자라도 마지막 박스를 이어 쓴다.

package automask

import (
	"math"
	"sort"
)

type Piece struct {
	First  int
	Frames [][]TrackBox
}

type Coverage struct {
	FPS    float64
	Pieces []Piece
	NextID int
	Found  int
	Kept   int
}

type FrameSpan struct{ First, End int }

const windowSlack = 2

// 시각에 fps를 곱해 반올림한다.
func FrameAt(t, fps float64) int { return int(math.Round(t * fps)) }

func (p Piece) End() int { return p.First + len(p.Frames) }

// 요청 구간에서 아직 어느 조각에도 없는 부분을 앞에서부터 준다.
func (c *Coverage) Missing(want FrameSpan) []FrameSpan {
	var spans []FrameSpan
	at := want.First
	if c != nil {
		for _, p := range c.Pieces {
			if p.End() <= at || p.First >= want.End {
				continue
			}
			if p.First > at {
				spans = append(spans, FrameSpan{at, p.First})
			}
			at = p.End()
		}
	}
	if at < want.End {
		spans = append(spans, FrameSpan{at, want.End})
	}
	return spans
}

// 조각 결과의 트랙 번호를 NextID 뒤로 옮겨 넣고 맞닿은 조각과 합친다. 조각은 Missing이 준 것이어야 한다.
func (c *Coverage) Merge(span FrameSpan, r Result) {
	shift := c.NextID
	frames := make([][]TrackBox, span.End-span.First)
	for n := range frames {
		if n < len(r.Frames) {
			for _, b := range r.Frames[n] {
				frames[n] = append(frames[n], TrackBox{Rect: b.Rect, ID: b.ID + shift})
			}
		}
	}
	c.NextID += r.Stats.Kept
	c.Found += r.Found
	c.Kept += r.Stats.Kept
	c.Pieces = append(c.Pieces, Piece{First: span.First, Frames: frames})
	sort.Slice(c.Pieces, func(i, j int) bool { return c.Pieces[i].First < c.Pieces[j].First })
	merged := c.Pieces[:1]
	for _, p := range c.Pieces[1:] {
		last := &merged[len(merged)-1]
		if p.First == last.End() {
			last.Frames = append(last.Frames, p.Frames...)
			continue
		}
		merged = append(merged, p)
	}
	c.Pieces = merged
}

// first부터 count개를 한 조각에서 꺼낸다(끝 windowSlack 모자람 허용). 없으면 false
func (c *Coverage) Window(first, count int) ([][]TrackBox, bool) {
	if c == nil {
		return nil, false
	}
	for _, p := range c.Pieces {
		if first < p.First || first >= p.End() {
			continue
		}
		if first+count > p.End()+windowSlack {
			return nil, false
		}
		out := make([][]TrackBox, count)
		for n := range out {
			out[n] = p.Frames[min(first+n-p.First, len(p.Frames)-1)]
		}
		return out, true
	}
	return nil, false
}
