// fakeTracker는 Update마다 상자를 오른쪽으로 1px 옮기고, lostAfter번째 Update에서 놓친다(0이면 안 놓침).
// Init에 들어온 프레임 크기와 Update 횟수를 기록해 앞뒤 방향을 따로 확인한다.

package automask

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"simpleEdit/internal/detect"
	"simpleEdit/internal/frame"
	"simpleEdit/internal/media"
	"simpleEdit/internal/tools"
)

type fakeTracker struct {
	box       detect.PixelBox
	updates   int
	lostAfter int
	inits     int
}

func (f *fakeTracker) Init(_ frame.Frame, b detect.PixelBox) error {
	f.box, f.updates = b, 0
	f.inits++
	return nil
}

func (f *fakeTracker) Update(frame.Frame) (detect.PixelBox, float32, bool, error) {
	f.updates++
	if f.lostAfter > 0 && f.updates >= f.lostAfter {
		return f.box, 0, false, nil
	}
	f.box.X++
	return f.box, 1, true, nil
}

// tools/linux의 ffmpeg로 30fps testsrc 영상을 만든다. 없으면 건너뛴다.
func testVideo(t *testing.T, seconds int) media.PipeSpec {
	t.Helper()
	dir, _ := filepath.Abs(filepath.Join("..", "..", "tools", "linux"))
	if _, err := os.Stat(filepath.Join(dir, "ffmpeg")); err != nil {
		t.Skipf("ffmpeg 없음: %s", dir)
	}
	t.Setenv(tools.EnvBinDir, dir)
	p := filepath.Join(t.TempDir(), "v.mp4")
	out, err := exec.Command(tools.Path(tools.FFmpeg), "-v", "error", "-y", "-f", "lavfi",
		"-i", fmt.Sprintf("testsrc=size=160x90:rate=30:duration=%d", seconds),
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", p).CombinedOutput()
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	return media.PipeSpec{Source: p, Duration: float64(seconds), FPS: 30, Width: 160, Height: 90}
}

func TestFollowBothWaysUntilEdges(t *testing.T) {
	s := testVideo(t, 4)
	s.Start, s.Duration = 1, 2
	tr := &fakeTracker{}
	p, err := Follow(context.Background(), s, tr, 60, detect.PixelBox{X: 40, Y: 20, W: 16, H: 8}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.LostBefore || p.LostAfter || tr.inits != 2 {
		t.Errorf("구간 끝까지 갔으면 놓침 아님, Init은 앞뒤 한 번씩: %+v inits %d", p, tr.inits)
	}
	if p.First < 29 || p.First > 31 || p.First+len(p.Boxes) < 89 || p.First+len(p.Boxes) > 91 {
		t.Errorf("1~3초(프레임 30~90)를 덮어야 함: First %d, %d개", p.First, len(p.Boxes))
	}
	at := p.Boxes[60-p.First]
	if at.X != 40.0/160 || at.W != 16.0/160 {
		t.Errorf("그린 프레임 상자는 그대로: %+v", at)
	}
	if p.Boxes[0].X <= at.X || p.Boxes[len(p.Boxes)-1].X <= at.X {
		t.Errorf("앞뒤 모두 추적한 상자여야 함(가짜는 오른쪽으로 감): %v %v", p.Boxes[0], p.Boxes[len(p.Boxes)-1])
	}
}

func TestFollowStopsWhenLost(t *testing.T) {
	s := testVideo(t, 3)
	tr := &fakeTracker{lostAfter: 5}
	p, err := Follow(context.Background(), s, tr, 45, detect.PixelBox{X: 40, Y: 20, W: 16, H: 8}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !p.LostBefore || !p.LostAfter || len(p.Boxes) != 9 || p.First != 41 {
		t.Errorf("앞뒤 4프레임씩 따라간 뒤 놓쳐야 함: First %d, %d개, %+v", p.First, len(p.Boxes), p)
	}
}

func TestFollowCanceled(t *testing.T) {
	s := testVideo(t, 2)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Follow(ctx, s, &fakeTracker{}, 30, detect.PixelBox{X: 1, Y: 1, W: 8, H: 8}, nil); err == nil {
		t.Error("취소하면 오류여야 함")
	}
}

func TestFillStepsInterpolates(t *testing.T) {
	keys := []media.Rect{{X: 0, W: 0.1}, {X: 0.2, W: 0.1}, {X: 0.3, W: 0.1}}
	got := fillSteps(keys, 2)
	want := []float64{0, 0.1, 0.2, 0.25, 0.3}
	if len(got) != len(want) {
		t.Fatalf("2프레임마다 상자 3개 → 프레임 5개: %d", len(got))
	}
	for i, x := range want {
		if d := got[i].X - x; d > 1e-9 || d < -1e-9 {
			t.Errorf("%d번 X = %v, want %v", i, got[i].X, x)
		}
	}
}
