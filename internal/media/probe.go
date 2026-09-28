// Package media는 영상 정보 조회와 ffmpeg 명령을 다룬다.
package media

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"simpleEdit/internal/tools"
)

// Info의 Width·Height는 회전을 반영한 크기다.
type Info struct {
	Duration   float64 // 초
	Width      int
	Height     int
	VideoCodec string
	PixFmt     string
	HasAudio   bool
	BitRate    int64 // bps, 모르면 0
	FPS        float64
}

type probeJSON struct {
	Streams []struct {
		CodecType string            `json:"codec_type"`
		CodecName string            `json:"codec_name"`
		Width     int               `json:"width"`
		Height    int               `json:"height"`
		PixFmt    string            `json:"pix_fmt"`
		AvgRate   string            `json:"avg_frame_rate"`
		RealRate  string            `json:"r_frame_rate"`
		Duration  string            `json:"duration"`
		Tags      map[string]string `json:"tags"`
		SideData  []struct {
			Rotation float64 `json:"rotation"`
		} `json:"side_data_list"`
		Disposition struct {
			AttachedPic int `json:"attached_pic"`
		} `json:"disposition"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		BitRate  string `json:"bit_rate"`
	} `json:"format"`
}

// Probe는 ffprobe를 실행해 Info를 반환한다.
func Probe(ctx context.Context, path string) (Info, error) {
	out, err := tools.Command(ctx, tools.FFprobe,
		"-v", "error", "-print_format", "json", "-show_format", "-show_streams", path,
	).Output()
	if err != nil {
		return Info{}, fmt.Errorf("ffprobe: %w", err)
	}
	return ParseProbe(out)
}

// ParseProbe는 ffprobe JSON 출력을 해석한다.
func ParseProbe(data []byte) (Info, error) {
	var p probeJSON
	if err := json.Unmarshal(data, &p); err != nil {
		return Info{}, fmt.Errorf("ffprobe JSON: %w", err)
	}
	var info Info
	foundVideo := false
	for _, s := range p.Streams {
		switch s.CodecType {
		case "video":
			if foundVideo || s.Disposition.AttachedPic == 1 {
				continue
			}
			foundVideo = true
			info.VideoCodec = s.CodecName
			info.PixFmt = s.PixFmt
			info.FPS = pickFPS(s.AvgRate, s.RealRate)
			info.Width, info.Height = s.Width, s.Height
			if isQuarterTurn(rotation(s.Tags, s.SideData)) {
				info.Width, info.Height = info.Height, info.Width
			}
			if d, err := strconv.ParseFloat(s.Duration, 64); err == nil {
				info.Duration = d
			}
		case "audio":
			info.HasAudio = true
		}
	}
	if !foundVideo || info.Width <= 0 || info.Height <= 0 {
		return Info{}, fmt.Errorf("영상 스트림 없음")
	}
	if d, err := strconv.ParseFloat(p.Format.Duration, 64); err == nil && d > 0 {
		info.Duration = d
	}
	if info.Duration <= 0 {
		return Info{}, fmt.Errorf("영상 길이를 알 수 없음")
	}
	info.BitRate, _ = strconv.ParseInt(p.Format.BitRate, 10, 64)
	return info, nil
}

func rotation(tags map[string]string, side []struct {
	Rotation float64 `json:"rotation"`
}) float64 {
	for _, sd := range side {
		if sd.Rotation != 0 {
			return sd.Rotation
		}
	}
	if r, err := strconv.ParseFloat(tags["rotate"], 64); err == nil {
		return r
	}
	return 0
}

// 고정 fps 변환에 쓸 기본값과 상한
const (
	defaultFPS = 30
	maxFPS     = 60
)

// pickFPS는 평균 fps를 우선하고 이상하면 기본값을 쓴다.
func pickFPS(rates ...string) float64 {
	for _, r := range rates {
		if v := parseRate(r); v >= 1 && v <= 240 {
			return min(v, maxFPS)
		}
	}
	return defaultFPS
}

// parseRate는 "30000/1001" 같은 분수를 푼다.
func parseRate(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		v, _ := strconv.ParseFloat(s, 64)
		return v
	}
	n, err1 := strconv.ParseFloat(num, 64)
	d, err2 := strconv.ParseFloat(den, 64)
	if err1 != nil || err2 != nil || d == 0 {
		return 0
	}
	return n / d
}

func isQuarterTurn(deg float64) bool {
	r := math.Mod(math.Abs(math.Round(deg)), 180)
	return r == 90
}

// PlayableInWebView는 H.264 8비트 4:2:0인지 본다.
func (i Info) PlayableInWebView() bool {
	if i.VideoCodec != "h264" {
		return false
	}
	switch i.PixFmt {
	case "yuv420p", "yuvj420p":
		return true
	}
	return false
}
