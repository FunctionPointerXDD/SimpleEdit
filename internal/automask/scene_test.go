package automask

import (
	"reflect"
	"testing"

	"simpleEdit/internal/frame"
)

func flat(v byte) frame.Frame {
	f := frame.New(64, 36)
	for i := range f.Pix {
		f.Pix[i] = v
	}
	return f
}

func TestSceneCutsOnJumpNotSteadyMotion(t *testing.T) {
	var s sceneDiffs
	for n := 0; n < 30; n++ {
		v := byte(40)
		if n >= 15 {
			v = 200
		}
		s.add(flat(v))
	}
	if got := DefaultSceneOptions.cuts(s.diffs); !reflect.DeepEqual(got, []int{15}) {
		t.Errorf("밝기가 한 번 크게 바뀐 곳만 컷: %v", got)
	}
	moving := []float64{0, 28, 30, 27, 29, 31, 28, 30, 29, 27, 30}
	if got := DefaultSceneOptions.cuts(moving); len(got) != 0 {
		t.Errorf("계속 크게 움직이면 컷이 아님: %v", got)
	}
	dup := []float64{0, 0.1, 0, 0.2, 0, 40, 0, 0.1, 0, 0.2, 0}
	if got := DefaultSceneOptions.cuts(dup); !reflect.DeepEqual(got, []int{5}) {
		t.Errorf("반복 프레임(0)이 섞여도 컷: %v", got)
	}
}

func TestSceneOf(t *testing.T) {
	cuts := []int{10, 20}
	for _, c := range []struct{ n, first, end int }{{0, 0, 10}, {9, 0, 10}, {10, 10, 20}, {25, 20, 30}} {
		if f, e := sceneOf(cuts, c.n, 30); f != c.first || e != c.end {
			t.Errorf("sceneOf(%d) = %d,%d", c.n, f, e)
		}
	}
}
