// Coverage는 영상 전체 프레임 번호(시각 × fps 반올림) 기준으로 이어진 자동 분석 결과다.
// 구간을 바꿔도 이미 분석한 프레임과 트랙 번호(뺀 트랙)를 그대로 쓰고, 빠진 앞·뒤만 더 분석해 붙인다.
// 붙인 조각의 트랙 번호는 NextID부터 이어 매겨 앞 조각 번호와 겹치지 않는다.
// FrameSpan은 [First, End) 프레임 번호 구간이다.
// 저장 프레임 수 round(길이×fps)와 분석 끝 round(끝×fps)는 반올림 때문에 1프레임 어긋날 수 있어
// Window는 끝이 windowSlack 프레임까지 모자라도 마지막 박스를 이어 쓴다.

package automask

import "math"

type Coverage struct {
	FPS    float64
	First  int
	Frames [][]TrackBox
	NextID int
	Found  int
	Kept   int
}

type FrameSpan struct{ First, End int }

const windowSlack = 2

// 시각에 fps를 곱해 반올림한다.
func FrameAt(t, fps float64) int { return int(math.Round(t * fps)) }

func (c *Coverage) End() int { return c.First + len(c.Frames) }

// 요청 구간에서 아직 없는 앞·뒤 조각. 겹치지 않으면 요청 구간 전체와 true(새로 시작)를 준다.
func (c *Coverage) Missing(want FrameSpan) (spans []FrameSpan, restart bool) {
	if c == nil || len(c.Frames) == 0 || want.End <= c.First || want.First >= c.End() {
		return []FrameSpan{want}, true
	}
	if want.First < c.First {
		spans = append(spans, FrameSpan{want.First, c.First})
	}
	if want.End > c.End() {
		spans = append(spans, FrameSpan{c.End(), want.End})
	}
	return spans, false
}

// 조각 결과의 트랙 번호를 NextID 뒤로 옮기고 앞이나 뒤에 붙인다. 조각은 Missing이 준 것이어야 한다.
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
	switch {
	case len(c.Frames) == 0:
		c.First, c.Frames = span.First, frames
	case span.First < c.First:
		c.First, c.Frames = span.First, append(frames, c.Frames...)
	default:
		c.Frames = append(c.Frames, frames...)
	}
}

// 시작 프레임부터 count개. 끝에서 windowSlack까지 모자라면 마지막 프레임을 다시 쓰고, 더 모자라면 false
func (c *Coverage) Window(first, count int) ([][]TrackBox, bool) {
	if c == nil || first < c.First || first+count > c.End()+windowSlack || len(c.Frames) == 0 {
		return nil, false
	}
	out := make([][]TrackBox, count)
	for n := range out {
		out[n] = c.Frames[min(first+n-c.First, len(c.Frames)-1)]
	}
	return out, true
}
