// Follow는 사용자가 그린 상자를 그린 프레임(at)에서 앞뒤로 추적해 프레임마다 상자를 준다(VitTrack).
// 원본 해상도로 디코딩한다. 960px로 줄이면 역광·밤 장면에서 훨씬 빨리 놓쳤다(10-06 시험).
// 디코딩은 yuv420p로 받아 추적기가 자를 영역만 BGR로 바꾼다(bgr24로 받으면 15초에 7.6초 → 3.1초).
// 초당 followRate장 정도만 추적하고(60fps면 2프레임마다) 사이 프레임은 앞뒤 상자를 선형으로 잇는다.
// 앞쪽은 at부터 구간 끝까지 흘려 보내며 따라가고, 놓치면 디코딩을 멈춘다.
// 뒤쪽은 거꾸로 디코딩할 수 없어 followChunk장씩 받아 뒤에서부터 따라간다. 조각마다 앞 키프레임부터
// 다시 풀어야 해 조각을 크게 잡는다(0.5초 조각은 실제 길이보다 2배 느렸다). 1080p 64장 약 200MB.
// 조각 경계에서 fps 필터 반올림으로 한 프레임 어긋날 수 있지만 추적 상자에는 영향이 작다.
// Path.First는 영상 전체 기준 프레임 번호(FrameAt), Boxes는 그 뒤로 이어진 0~1 비율 상자다.
// LostBefore·LostAfter는 놓쳐서 멈췄는지(true), 구간 끝에 닿아 멈췄는지(false)다.

package automask

import (
	"context"
	"errors"
	"math"

	"simpleEdit/internal/detect"
	"simpleEdit/internal/frame"
	"simpleEdit/internal/media"
)

const (
	followChunk = 64
	followRate  = 30
)

type FollowTracker interface {
	Init(f frame.Frame, box detect.PixelBox) error
	Update(f frame.Frame) (detect.PixelBox, float32, bool, error)
}

type Path struct {
	First      int
	Boxes      []media.Rect
	LostBefore bool
	LostAfter  bool
}

var errFollowStop = errors.New("추적 멈춤")

// s는 자른 구간 전체(원본 크기·fps), at은 상자를 그린 영상 전체 기준 프레임 번호다.
func Follow(ctx context.Context, s media.PipeSpec, tr FollowTracker, at int, box detect.PixelBox, onProgress media.ProgressFunc) (Path, error) {
	first := FrameAt(s.Start, s.FPS)
	end := first + s.FrameCount()
	at = min(max(at, first), end-1)
	step := max(1, int(math.Round(s.FPS/followRate)))
	w, h := s.Width&^1, s.Height&^1
	total := float64(end-first) / float64(step)
	done := 0
	tick := func() {
		done++
		if onProgress != nil {
			onProgress(min(0.99, float64(done)/total))
		}
	}
	toRect := func(b detect.PixelBox) media.Rect {
		return media.Rect{X: float64(b.X) / float64(w), Y: float64(b.Y) / float64(h),
			W: float64(b.W) / float64(w), H: float64(b.H) / float64(h)}
	}

	back, lostBefore, err := followBack(ctx, s, tr, first, at, step, box, toRect, tick)
	if err != nil {
		return Path{}, err
	}
	ahead, lostAfter, err := followAhead(ctx, s, tr, at, end, step, box, toRect, tick)
	if err != nil {
		return Path{}, err
	}
	keys := append(back, ahead...)
	if onProgress != nil {
		onProgress(1)
	}
	return Path{First: at - (len(back))*step, Boxes: fillSteps(keys, step), LostBefore: lostBefore, LostAfter: lostAfter}, nil
}

// step 간격 상자 사이를 선형으로 채워 프레임마다 하나씩 둔다. 마지막 상자 뒤는 채우지 않는다.
func fillSteps(keys []media.Rect, step int) []media.Rect {
	if step == 1 || len(keys) < 2 {
		return keys
	}
	out := make([]media.Rect, 0, (len(keys)-1)*step+1)
	for i := 0; i+1 < len(keys); i++ {
		for k := 0; k < step; k++ {
			out = append(out, lerp(keys[i], keys[i+1], float64(k)/float64(step)))
		}
	}
	return append(out, keys[len(keys)-1])
}

// at 프레임에서 Init하고 step마다 end 전까지 따라간다. 결과 첫 상자는 at의 상자다.
func followAhead(ctx context.Context, s media.PipeSpec, tr FollowTracker, at, end, step int, box detect.PixelBox,
	toRect func(detect.PixelBox) media.Rect, tick func()) ([]media.Rect, bool, error) {
	var out []media.Rect
	lost := false
	err := media.DecodeYUV(ctx, span(s, at, end, step), func(n int, f frame.Frame) error {
		tick()
		if n == 0 {
			if err := tr.Init(f, box); err != nil {
				return err
			}
			out = append(out, toRect(box))
			return nil
		}
		b, _, ok, err := tr.Update(f)
		if err != nil {
			return err
		}
		if !ok {
			lost = true
			return errFollowStop
		}
		out = append(out, toRect(b))
		return nil
	})
	if errors.Is(err, errFollowStop) {
		err = nil
	}
	return out, lost, err
}

// at에서 Init하고 at−step, at−2·step… 위치를 followChunk장씩 받아 뒤에서부터 따라간다(결과는 시간순).
func followBack(ctx context.Context, s media.PipeSpec, tr FollowTracker, first, at, step int, box detect.PixelBox,
	toRect func(detect.PixelBox) media.Rect, tick func()) ([]media.Rect, bool, error) {
	var rev []media.Rect
	hi := at
	started := false
	for hi >= first {
		lo := hi - (followChunk-1)*step
		for lo < first {
			lo += step
		}
		frames, err := collect(ctx, span(s, lo, hi+1, step))
		if err != nil {
			return nil, false, err
		}
		for i := len(frames) - 1; i >= 0; i-- {
			if !started {
				if err := tr.Init(frames[i], box); err != nil {
					return nil, false, err
				}
				started = true
				continue
			}
			tick()
			b, _, ok, err := tr.Update(frames[i])
			if err != nil {
				return nil, false, err
			}
			if !ok {
				return reverse(rev), true, nil
			}
			rev = append(rev, toRect(b))
		}
		if len(frames) == 0 {
			break
		}
		hi = lo - step
	}
	return reverse(rev), false, nil
}

// s에서 영상 전체 기준 [from, to) 프레임을 step마다 디코딩하도록 시작·길이·fps를 바꾼다.
func span(s media.PipeSpec, from, to, step int) media.PipeSpec {
	out := s
	out.Start = float64(from) / s.FPS
	out.Duration = float64(to-from) / s.FPS
	out.FPS = s.FPS / float64(step)
	return out
}

// 디코더 버퍼는 다시 쓰이므로 프레임마다 복사해 모은다.
func collect(ctx context.Context, s media.PipeSpec) ([]frame.Frame, error) {
	var out []frame.Frame
	err := media.DecodeYUV(ctx, s, func(_ int, f frame.Frame) error {
		out = append(out, frame.Frame{W: f.W, H: f.H, Format: f.Format, Pix: append([]byte(nil), f.Pix...)})
		return nil
	})
	return out, err
}

// 순서를 거꾸로 한 새 슬라이스
func reverse(rs []media.Rect) []media.Rect {
	out := make([]media.Rect, len(rs))
	for i, r := range rs {
		out[len(rs)-1-i] = r
	}
	return out
}
