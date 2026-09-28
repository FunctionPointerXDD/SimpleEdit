package download

import (
	"errors"
	"math"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	const want = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	ok := []string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"  https://youtube.com/watch?v=dQw4w9WgXcQ&list=PL123&t=42s \n",
		"http://m.youtube.com/watch?feature=share&v=dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ?si=abcdef",
		"youtu.be/dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ",
		"https://youtube.com/shorts/dQw4w9WgXcQ?feature=share",
		"https://www.youtube.com/live/dQw4w9WgXcQ",
	}
	for _, in := range ok {
		got, err := NormalizeURL(in)
		if err != nil || got != want {
			t.Errorf("NormalizeURL(%q) = %q, %v", in, got, err)
		}
	}
	bad := []string{
		"", "hello", "https://www.youtube.com/", "https://www.youtube.com/watch?v=short",
		"https://www.youtube.com/playlist?list=PL123", "https://vimeo.com/12345",
		"https://evil.com/watch?v=dQw4w9WgXcQ", "https://youtube.com.evil.com/watch?v=dQw4w9WgXcQ",
		"C:\\Users\\a.mp4",
	}
	for _, in := range bad {
		if got, err := NormalizeURL(in); !errors.Is(err, ErrBadURL) {
			t.Errorf("NormalizeURL(%q) = %q, %v; want ErrBadURL", in, got, err)
		}
	}
}

func TestParserTwoStreams(t *testing.T) {
	var got []Progress
	p := newParser(func(pr Progress) { got = append(got, pr) })
	lines := []string{
		"SEINFO 140 900 100",
		"SEPROG 0 900",
		"SEPROG 450 900",
		"SEPROG 900 900",
		"SEPROG 10 100",
		"SEPROG 100 100",
		"SEFILE C:\\work\\source.mp4",
	}
	for _, l := range lines {
		p.line(l)
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	k := 1 - mergeShare
	wantFrac := []float64{0, 0.45 * k, 0.9 * k, 0.91 * k, 1.0 * k}
	if len(got) != len(wantFrac) {
		t.Fatalf("이벤트 %d개, want %d: %+v", len(got), len(wantFrac), got)
	}
	for i, w := range wantFrac {
		if !near(got[i].Frac, w) {
			t.Errorf("이벤트 %d = %v, want %v", i, got[i].Frac, w)
		}
	}
	if got[len(got)-1].Stage != StageMerge {
		t.Errorf("마지막 단계 = %s", got[len(got)-1].Stage)
	}
	if p.file != `C:\work\source.mp4` {
		t.Errorf("파일 경로 = %q", p.file)
	}
}

func TestParserSingleStreamUnknownSize(t *testing.T) {
	var last Progress
	p := newParser(func(pr Progress) { last = pr })
	p.line("SEINFO - 0 0")
	p.line("SEPROG 50 NA")
	if last.Frac != 0 {
		t.Errorf("전체 크기를 모르면 0이어야 함: %v", last.Frac)
	}
	p.line("SEPROG 50 100")
	if math.Abs(last.Frac-0.5*(1-mergeShare)) > 1e-9 {
		t.Errorf("단일 스트림 50%% = %v", last.Frac)
	}
}

func TestClassify(t *testing.T) {
	tests := map[string]error{
		"ERROR: [youtube] abc: Private video. Sign in if you've been granted access":           ErrPrivate,
		"ERROR: [youtube] abc: Video unavailable. This video has been removed by the uploader": ErrUnavailable,
		"ERROR: [youtube] abc: Sign in to confirm your age. This video may be inappropriate":   ErrAgeLimit,
		"ERROR: [youtube] abc: Sign in to confirm you’re not a bot":                            ErrBlocked,
		"ERROR: Unable to download webpage: <urlopen error [Errno 11001] getaddrinfo failed>":  ErrNetwork,
		"ERROR: unable to write data: [Errno 28] No space left on device":                      ErrNoSpace,
		"WARNING: something harmless": nil,
	}
	for in, want := range tests {
		if got := Classify(in); !errors.Is(got, want) || (want == nil && got != nil) {
			t.Errorf("Classify(%q) = %v, want %v", in, got, want)
		}
	}
}
