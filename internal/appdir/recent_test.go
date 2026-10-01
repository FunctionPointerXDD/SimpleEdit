package appdir

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecentDownloads(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	mk := func(name, url string, at time.Time, withFile bool) {
		dir := filepath.Join(Work(), name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if withFile {
			os.WriteFile(filepath.Join(dir, "source.mp4"), []byte("x"), 0o644)
		}
		if url != "" {
			if err := WriteMeta(dir, Meta{URL: url, Title: name, Duration: 60, File: filepath.Join(dir, "source.mp4"), SavedAt: at}); err != nil {
				t.Fatal(err)
			}
		}
	}
	now := time.Now()
	mk("a", "https://www.youtube.com/watch?v=aaaaaaaaaaa", now.Add(-2*time.Hour), true)
	mk("b", "https://www.youtube.com/watch?v=bbbbbbbbbbb", now.Add(-1*time.Hour), true)
	mk("c", "https://www.youtube.com/watch?v=aaaaaaaaaaa", now.Add(-3*time.Hour), true)
	mk("gone", "https://www.youtube.com/watch?v=ggggggggggg", now, false)
	mk("local", "", now, true)

	got := RecentDownloads()
	if len(got) != 3 || got[0].ID != "b" || got[1].ID != "a" || got[2].ID != "c" {
		t.Fatalf("원본이 있는 meta 폴더만 최신순이어야 함: %+v", got)
	}
	if got[0].Path != filepath.Join(Work(), "b", "source.mp4") {
		t.Errorf("경로 %s", got[0].Path)
	}
	if r, ok := FindDownload("https://www.youtube.com/watch?v=aaaaaaaaaaa"); !ok || r.ID != "a" {
		t.Errorf("같은 주소는 가장 최근 폴더: %+v %v", r, ok)
	}
	if _, ok := FindDownload("https://www.youtube.com/watch?v=ggggggggggg"); ok {
		t.Error("원본이 지워진 폴더는 찾으면 안 됨")
	}
	for _, id := range []string{"local", "..", "../b", filepath.Join(Work(), "b"), ""} {
		if _, ok := RecentByID(id); ok {
			t.Errorf("목록에 없는 id %q를 받으면 안 됨", id)
		}
	}
	if r, ok := RecentByID("b"); !ok || r.Meta.Title != "b" {
		t.Errorf("목록의 id는 찾아야 함: %+v", r)
	}
}
