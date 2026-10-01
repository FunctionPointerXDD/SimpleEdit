// Package download는 유튜브 주소를 검사하고 yt-dlp로 영상을 받는다.
package download

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

var ErrBadURL = errors.New("유튜브 주소 아님")

var videoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// youtu.be, watch?v=, shorts/, live/에서 11자 ID를 꺼내 watch 주소로 바꾼다.
func NormalizeURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", ErrBadURL
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", ErrBadURL
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	host = strings.TrimPrefix(host, "m.")
	path := strings.Trim(u.Path, "/")

	var id string
	switch host {
	case "youtu.be":
		id, _, _ = strings.Cut(path, "/")
	case "youtube.com", "music.youtube.com":
		switch {
		case path == "watch":
			id = u.Query().Get("v")
		case strings.HasPrefix(path, "shorts/"):
			id, _, _ = strings.Cut(strings.TrimPrefix(path, "shorts/"), "/")
		case strings.HasPrefix(path, "live/"):
			id, _, _ = strings.Cut(strings.TrimPrefix(path, "live/"), "/")
		}
	}
	if !videoID.MatchString(id) {
		return "", ErrBadURL
	}
	return "https://www.youtube.com/watch?v=" + id, nil
}
