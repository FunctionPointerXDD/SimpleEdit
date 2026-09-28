package media

import (
	"strings"
	"testing"
)

func TestParseProbe(t *testing.T) {
	data := `{
	  "streams": [
	    {"codec_type":"video","codec_name":"h264","width":1920,"height":1080,"pix_fmt":"yuv420p","duration":"300.2"},
	    {"codec_type":"audio","codec_name":"aac"}
	  ],
	  "format": {"duration":"300.500","bit_rate":"4000000"}
	}`
	info, err := ParseProbe([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	want := Info{Duration: 300.5, Width: 1920, Height: 1080, VideoCodec: "h264", PixFmt: "yuv420p", HasAudio: true, BitRate: 4000000}
	if info != want {
		t.Errorf("got %+v\nwant %+v", info, want)
	}
	if !info.PlayableInWebView() {
		t.Error("h264 yuv420p는 재생 가능해야 함")
	}
}

func TestParseProbeRotation(t *testing.T) {
	data := `{
	  "streams": [
	    {"codec_type":"video","codec_name":"hevc","width":1920,"height":1080,"pix_fmt":"yuv420p10le",
	     "side_data_list":[{"side_data_type":"Display Matrix","rotation":-90}]}
	  ],
	  "format": {"duration":"10.0"}
	}`
	info, err := ParseProbe([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 1080 || info.Height != 1920 {
		t.Errorf("회전 반영 안 됨: %dx%d", info.Width, info.Height)
	}
	if info.HasAudio {
		t.Error("소리 없음이어야 함")
	}
	if info.PlayableInWebView() {
		t.Error("HEVC 10비트는 미리보기 변환이 필요해야 함")
	}
}

func TestParseProbeRotateTagAndCover(t *testing.T) {
	data := `{
	  "streams": [
	    {"codec_type":"video","codec_name":"mjpeg","width":300,"height":300,"disposition":{"attached_pic":1}},
	    {"codec_type":"video","codec_name":"h264","width":640,"height":360,"pix_fmt":"yuv420p","tags":{"rotate":"270"}}
	  ],
	  "format": {"duration":"5"}
	}`
	info, err := ParseProbe([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 360 || info.Height != 640 || info.VideoCodec != "h264" {
		t.Errorf("got %+v", info)
	}
}

func TestParseProbeNoVideo(t *testing.T) {
	_, err := ParseProbe([]byte(`{"streams":[{"codec_type":"audio"}],"format":{"duration":"3"}}`))
	if err == nil {
		t.Fatal("영상 없는 파일은 오류여야 함")
	}
}

func TestReadProgress(t *testing.T) {
	out := strings.Join([]string{
		"frame=10", "out_time_us=N/A", "progress=continue",
		"out_time_us=5000000", "progress=continue",
		"out_time_us=10000000", "progress=end",
	}, "\n")
	var got []float64
	ReadProgress(strings.NewReader(out), 10, func(f float64) { got = append(got, f) })
	want := []float64{0.5, 1, 1}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}
