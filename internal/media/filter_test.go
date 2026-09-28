package media

import (
	"slices"
	"testing"
)

func TestToPixels(t *testing.T) {
	tests := []struct {
		name   string
		r      Rect
		w, h   int
		want   PixelRect
		wantOK bool
	}{
		{"가운데 1080p", Rect{0.25, 0.25, 0.5, 0.5}, 1920, 1080, PixelRect{480, 270, 960, 540}, true},
		{"홀수 좌표는 짝수로", Rect{0.1, 0.1, 0.1, 0.1}, 1280, 720, PixelRect{128, 72, 128, 72}, true},
		{"오른쪽 아래 밖으로 나감", Rect{0.9, 0.9, 0.5, 0.5}, 1920, 1080, PixelRect{1728, 972, 192, 108}, true},
		{"왼쪽 위 밖에서 시작", Rect{-0.1, -0.1, 0.2, 0.2}, 1000, 1000, PixelRect{0, 0, 100, 100}, true},
		{"세로 영상", Rect{0.5, 0.5, 0.5, 0.5}, 1080, 1920, PixelRect{540, 960, 540, 960}, true},
		{"홀수 해상도 끝", Rect{0, 0, 1, 1}, 853, 481, PixelRect{0, 0, 852, 480}, true},
		{"너무 작음", Rect{0.5, 0.5, 0.001, 0.001}, 1920, 1080, PixelRect{}, false},
		{"완전히 밖", Rect{1.2, 0.2, 0.1, 0.1}, 1920, 1080, PixelRect{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ToPixels(tt.r, tt.w, tt.h)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("ToPixels = %+v, %v; want %+v, %v", got, ok, tt.want, tt.wantOK)
			}
			if ok {
				if got.W%2 != 0 || got.H%2 != 0 || got.X%2 != 0 || got.Y%2 != 0 {
					t.Errorf("짝수가 아님: %+v", got)
				}
				if got.X+got.W > tt.w || got.Y+got.H > tt.h {
					t.Errorf("영상 밖으로 나감: %+v", got)
				}
			}
		})
	}
}

func TestMosaicFilter(t *testing.T) {
	if got := MosaicFilter(nil); got != "" {
		t.Errorf("빈 목록 = %q", got)
	}

	one := MosaicFilter([]PixelRect{{X: 100, Y: 50, W: 320, H: 160}})
	wantOne := "[0:v]split=2[base][m0]" +
		";[m0]crop=320:160:100:50,scale=20:10,scale=320:160:flags=neighbor[p0]" +
		";[base][p0]overlay=100:50[v]"
	if one != wantOne {
		t.Errorf("1개:\n got %s\nwant %s", one, wantOne)
	}

	three := MosaicFilter([]PixelRect{{0, 0, 32, 32}, {100, 100, 8, 8}, {200, 0, 64, 16}})
	wantThree := "[0:v]split=4[base][m0][m1][m2]" +
		";[m0]crop=32:32:0:0,scale=2:2,scale=32:32:flags=neighbor[p0]" +
		";[m1]crop=8:8:100:100,scale=1:1,scale=8:8:flags=neighbor[p1]" +
		";[m2]crop=64:16:200:0,scale=4:1,scale=64:16:flags=neighbor[p2]" +
		";[base][p0]overlay=0:0[o0];[o0][p1]overlay=100:100[o1];[o1][p2]overlay=200:0[v]"
	if three != wantThree {
		t.Errorf("3개:\n got %s\nwant %s", three, wantThree)
	}
}

func TestExportArgs(t *testing.T) {
	plain := ExportArgs(ExportSpec{Source: "in.mp4", Output: "out.mp4", Start: 12.3, Duration: 30})
	if slices.Contains(plain, "-filter_complex") {
		t.Error("가릴 영역이 없는데 -filter_complex가 있음")
	}
	for _, want := range []string{"12.300", "30.000", "libx264", "aac", "yuv420p", "+faststart", "pipe:1"} {
		if !slices.Contains(plain, want) {
			t.Errorf("인자에 %q 없음: %v", want, plain)
		}
	}
	if plain[len(plain)-1] != "out.mp4" {
		t.Errorf("출력 파일이 마지막이 아님: %v", plain)
	}

	masked := ExportArgs(ExportSpec{Source: "in.mp4", Output: "out.mp4", Duration: 5,
		Rects: []PixelRect{{0, 0, 32, 32}}})
	i := slices.Index(masked, "-filter_complex")
	if i < 0 || masked[i+2] != "-map" || masked[i+3] != "[v]" {
		t.Errorf("모자이크 인자 이상: %v", masked)
	}
}
