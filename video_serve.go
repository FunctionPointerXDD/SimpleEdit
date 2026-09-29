// Wails는 Windows에서 응답 본문을 전부 메모리에 모은 뒤 UI 스레드에서 복사한다.
// 그래서 <video>의 "bytes=0-" 요청에 파일 끝까지 주면 긴 영상에서 화면이 멈춘다.
// 응답은 한 번에 maxRangeChunk까지만 206으로 주고, 나머지는 브라우저가 이어 요청한다.

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"simpleEdit/internal/applog"
)

const maxRangeChunk = 4 << 20

// /video/current만 받고, 요청 범위를 maxRangeChunk로 잘라 206으로 보낸다.
func (a *App) serveVideo(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/video/current" {
		http.NotFound(w, r)
		return
	}
	a.mu.Lock()
	path := a.preview
	a.mu.Unlock()
	if path == "" {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		applog.Printf("미리보기 파일 열기 실패: %v", err)
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	size := st.Size()
	h := w.Header()
	h.Set("Content-Type", "video/mp4")
	h.Set("Cache-Control", "no-store")
	h.Set("Accept-Ranges", "bytes")
	start, end, ok := chunkRange(r.Header.Get("Range"), size, maxRangeChunk)
	if !ok {
		h.Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	h.Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	w.WriteHeader(http.StatusPartialContent)
	if r.Method == http.MethodHead {
		return
	}
	if _, err := io.Copy(w, io.NewSectionReader(f, start, end-start+1)); err != nil {
		applog.Printf("미리보기 전송 실패: %v", err)
	}
}

// 첫 범위만 본다. 없으면 0부터, "-N"은 끝 N바이트. 끝은 start+limit-1을 넘지 않는다.
func chunkRange(header string, size, limit int64) (start, end int64, ok bool) {
	if size <= 0 {
		return 0, 0, false
	}
	start, end = 0, size-1
	if spec, found := strings.CutPrefix(strings.TrimSpace(header), "bytes="); found {
		first, _, _ := strings.Cut(spec, ",")
		a, b, dash := strings.Cut(strings.TrimSpace(first), "-")
		if !dash {
			return 0, 0, false
		}
		switch {
		case a == "":
			n, err := strconv.ParseInt(b, 10, 64)
			if err != nil || n <= 0 {
				return 0, 0, false
			}
			start = max(0, size-n)
		default:
			v, err := strconv.ParseInt(a, 10, 64)
			if err != nil || v < 0 || v >= size {
				return 0, 0, false
			}
			start = v
			if b != "" {
				e, err := strconv.ParseInt(b, 10, 64)
				if err != nil || e < start {
					return 0, 0, false
				}
				end = min(e, size-1)
			}
		}
	}
	return start, min(end, start+limit-1), true
}
