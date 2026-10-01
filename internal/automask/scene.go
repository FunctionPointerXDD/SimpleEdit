// 장면 전환(컷)은 프레임을 sceneGrid 칸 밝기 평균으로 줄여 바로 앞 프레임과의 칸 차이 평균으로 찾는다.
// 차이가 CutDiff 이상이고, 앞뒤 sceneWindow 프레임 차이의 중앙값(최소 sceneFloor)의 CutRatio배
// 이상이면 컷이다. 블랙박스처럼 계속 움직이는 영상은 주변 차이도 커서 컷으로 보지 않는다.
// 앞 프레임을 그대로 반복한 프레임(차이 sceneDup 미만)은 중앙값 계산에서 뺀다.
// 2026-10-01 측정(눈으로 확인한 컷): 블랙박스 18개 중 17개, 뉴스 19개 중 18개, 잘못 잡음 1개.
// 놓친 것은 여러 프레임에 걸친 화면 전환 효과다. 히스토그램 차이는 비슷한 색 장면끼리의 컷을 못 잡아 안 쓴다.
// 컷 프레임 번호는 새 장면의 첫 프레임이다.

package automask

import (
	"sort"

	"simpleEdit/internal/frame"
)

const (
	sceneGridW  = 32
	sceneGridH  = 18
	sceneWindow = 6
	sceneDup    = 0.5
	sceneFloor  = 2
)

type SceneOptions struct {
	CutDiff  float64
	CutRatio float64
}

var DefaultSceneOptions = SceneOptions{CutDiff: 25, CutRatio: 4}

type sceneDiffs struct {
	prev  []float64
	diffs []float64
}

// 격자 밝기를 앞 프레임과 비교해 차이를 쌓는다. 첫 프레임은 0
func (s *sceneDiffs) add(f frame.Frame) {
	cur := lumaGrid(f)
	var d float64
	if s.prev != nil {
		for i := range cur {
			d += abs(cur[i] - s.prev[i])
		}
		d /= float64(len(cur))
	}
	s.diffs = append(s.diffs, d)
	s.prev = cur
}

// 차이가 CutDiff 이상이고 앞뒤 반복 아닌 프레임 중앙값의 CutRatio배 이상인 프레임들
func (o SceneOptions) cuts(diffs []float64) []int {
	var out []int
	var near []float64
	for n, d := range diffs {
		if n == 0 || d < o.CutDiff {
			continue
		}
		near = near[:0]
		for i := max(1, n-sceneWindow); i <= min(len(diffs)-1, n+sceneWindow); i++ {
			if i != n && diffs[i] >= sceneDup {
				near = append(near, diffs[i])
			}
		}
		if d >= o.CutRatio*max(median(near), sceneFloor) {
			out = append(out, n)
		}
	}
	return out
}

// 정렬한 뒤 가운데 값. 비어 있으면 0
func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

// 칸마다 (B+2G+R)/4 평균. 칸 경계는 좌표 × 격자 수 / 변 길이를 내림
func lumaGrid(f frame.Frame) []float64 {
	out := make([]float64, sceneGridW*sceneGridH)
	cnt := make([]int, len(out))
	for y := 0; y < f.H; y++ {
		gy := y * sceneGridH / f.H
		row := f.Pix[y*f.W*3:]
		for x := 0; x < f.W; x++ {
			g := gy*sceneGridW + x*sceneGridW/f.W
			out[g] += float64(int(row[x*3])+2*int(row[x*3+1])+int(row[x*3+2])) / 4
			cnt[g]++
		}
	}
	for i := range out {
		out[i] /= float64(max(1, cnt[i]))
	}
	return out
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// 정렬된 cuts에서 n이 속한 장면의 [시작, 끝) 프레임. 앞뒤 컷이 없으면 0, limit
func sceneOf(cuts []int, n, limit int) (int, int) {
	i := sort.SearchInts(cuts, n+1)
	first, end := 0, limit
	if i > 0 {
		first = cuts[i-1]
	}
	if i < len(cuts) {
		end = cuts[i]
	}
	return first, end
}
