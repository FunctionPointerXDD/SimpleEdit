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
		{"가운데 1080p", Rect{0.25, 0.25, 0.5, 0.5}, 1920, 1080, PixelRect{X: 480, Y: 270, W: 960, H: 540}, true},
		{"홀수 좌표는 짝수로", Rect{0.1, 0.1, 0.1, 0.1}, 1280, 720, PixelRect{X: 128, Y: 72, W: 128, H: 72}, true},
		{"오른쪽 아래 밖으로 나감", Rect{0.9, 0.9, 0.5, 0.5}, 1920, 1080, PixelRect{X: 1728, Y: 972, W: 192, H: 108}, true},
		{"왼쪽 위 밖에서 시작", Rect{-0.1, -0.1, 0.2, 0.2}, 1000, 1000, PixelRect{X: 0, Y: 0, W: 100, H: 100}, true},
		{"세로 영상", Rect{0.5, 0.5, 0.5, 0.5}, 1080, 1920, PixelRect{X: 540, Y: 960, W: 540, H: 960}, true},
		{"홀수 해상도 끝", Rect{X: 0, Y: 0, W: 1, H: 1}, 853, 481, PixelRect{X: 0, Y: 0, W: 852, H: 480}, true},
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

func TestBlurSigma(t *testing.T) {
	tests := []struct{ w, h, want int }{{10, 8, 4}, {40, 20, 4}, {100, 200, 16}, {320, 160, 26}}
	for _, tt := range tests {
		if got := BlurSigma(tt.w, tt.h); got != tt.want {
			t.Errorf("BlurSigma(%d,%d) = %d, want %d", tt.w, tt.h, got, tt.want)
		}
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
}

func TestExportArgsMute(t *testing.T) {
	args := ExportArgs(ExportSpec{Source: "in.mp4", Output: "out.mp4", Duration: 5, Mute: true})
	if !slices.Contains(args, "-an") || slices.Contains(args, "aac") || slices.Contains(args, "0:a:0?") {
		t.Errorf("무음 인자 이상: %v", args)
	}
	enc := EncodeArgs(PipeSpec{Source: "in.mp4", Duration: 5, FPS: 30, Width: 64, Height: 64, Mute: true}, "out.mp4")
	if !slices.Contains(enc, "-an") || slices.Contains(enc, "in.mp4") {
		t.Errorf("무음 파이프는 원본을 입력으로 열지 않아야 함: %v", enc)
	}
}
